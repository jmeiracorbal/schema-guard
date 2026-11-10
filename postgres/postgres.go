// Package postgres reads PostgreSQL structural metadata from pg_catalog.
// The caller supplies a database/sql connection and registers its own SQL driver.
// No data definition or migration statements are executed by the inspector.
package postgres

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jmeiracorbal/schema-guard/internal/domain"
	"github.com/jmeiracorbal/schema-guard/schema"
	"github.com/jmeiracorbal/schema-guard/snapshot"
)

//go:embed queries/*.sql
var queries embed.FS

// Options limits inspection to the exact schema names supplied. If Schemas is
// empty, all non-system schemas are inspected. Schema names are passed as a
// parameter, never interpolated into SQL identifiers.
type Options struct {
	Schemas []string
}

// Inspector returns deterministic, validated PostgreSQL snapshots.
type Inspector struct {
	db      *sql.DB
	schemas []string
}

var _ domain.Inspector = (*Inspector)(nil)

// New validates configuration and constructs an inspector without opening any
// new connections. A non-nil, initialized sql.DB must be provided.
func New(db *sql.DB, opts Options) (*Inspector, error) {
	if db == nil {
		return nil, fmt.Errorf("postgres: database connection must not be nil")
	}
	seen := make(map[string]struct{}, len(opts.Schemas))
	for _, name := range opts.Schemas {
		if name == "" {
			return nil, fmt.Errorf("postgres: schema name must not be empty")
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("postgres: duplicate schema name %q", name)
		}
		seen[name] = struct{}{}
	}
	selected := append([]string(nil), opts.Schemas...)
	sort.Strings(selected)
	return &Inspector{db: db, schemas: selected}, nil
}

func (*Inspector) Engine() schema.Engine { return schema.PostgreSQL }

// Inspect reads all metadata within a single read-only REPEATABLE READ
// transaction. It fails instead of returning a partial or ambiguous snapshot.
func (i *Inspector) Inspect(ctx context.Context) (schema.Snapshot, error) {
	transaction, err := i.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return schema.Snapshot{}, fmt.Errorf("postgres: begin read-only transaction: %w", err)
	}
	defer transaction.Rollback()

	if _, err := transaction.ExecContext(ctx, query("session.sql")); err != nil {
		return schema.Snapshot{}, fmt.Errorf("postgres: configure catalog search path: %w", err)
	}

	filter, err := i.schemaFilter()
	if err != nil {
		return schema.Snapshot{}, err
	}
	if err := i.validateSchemas(ctx, transaction, filter); err != nil {
		return schema.Snapshot{}, err
	}

	result := schema.Snapshot{Version: schema.FormatVersion, Engine: schema.PostgreSQL, Tables: []schema.Table{}}
	byOID, err := inspectTables(ctx, transaction, filter, &result)
	if err != nil {
		return schema.Snapshot{}, err
	}
	for _, stage := range []struct {
		name string
		load func(context.Context, *sql.Tx, any, *schema.Snapshot, map[int64]int) error
	}{
		{"columns", inspectColumns},
		{"indexes", inspectIndexes},
		{"constraints", inspectConstraints},
	} {
		if err := stage.load(ctx, transaction, filter, &result, byOID); err != nil {
			return schema.Snapshot{}, fmt.Errorf("postgres: inspect %s: %w", stage.name, err)
		}
	}

	canonical, err := snapshot.Canonical(result)
	if err != nil {
		return schema.Snapshot{}, fmt.Errorf("postgres: invalid metadata: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return schema.Snapshot{}, fmt.Errorf("postgres: commit read-only transaction: %w", err)
	}
	return canonical, nil
}

func query(name string) string {
	data, err := queries.ReadFile("queries/" + name)
	if err != nil {
		// Embedded, compile-time constant paths only; a missing asset is a
		// programmer error rather than a recoverable database failure.
		panic(fmt.Sprintf("postgres: embedded query %q: %v", name, err))
	}
	return string(data)
}

func (i *Inspector) schemaFilter() (any, error) {
	if len(i.schemas) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(i.schemas)
	if err != nil {
		return nil, fmt.Errorf("postgres: encode requested schemas: %w", err)
	}
	return string(data), nil
}

func (i *Inspector) validateSchemas(ctx context.Context, tx *sql.Tx, filter any) error {
	if len(i.schemas) == 0 {
		return nil
	}
	rows, err := tx.QueryContext(ctx, query("schemas.sql"), filter)
	if err != nil {
		return fmt.Errorf("postgres: list schemas: %w", err)
	}
	defer rows.Close()

	found := make(map[string]struct{}, len(i.schemas))
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("postgres: scan schema: %w", err)
		}
		found[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("postgres: list schemas: %w", err)
	}
	for _, name := range i.schemas {
		if _, ok := found[name]; !ok {
			return fmt.Errorf("postgres: requested schema %q not found or is a system schema", name)
		}
	}
	return nil
}

func tableAt(result *schema.Snapshot, byOID map[int64]int, oid int64) (*schema.Table, error) {
	index, ok := byOID[oid]
	if !ok {
		return nil, fmt.Errorf("catalog referenced an unknown table oid %d", oid)
	}
	return &result.Tables[index], nil
}

func optional(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	value := s.String
	return &value
}
