// Package sqlserver reads SQL Server 2022+ catalog metadata from sys.* views.
// It never writes database state or generates migrations.
package sqlserver

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

type Options struct{ Schemas []string }

type Inspector struct {
	db      *sql.DB
	schemas []string
}

var _ domain.Inspector = (*Inspector)(nil)

func New(db *sql.DB, opts Options) (*Inspector, error) {
	if db == nil {
		return nil, fmt.Errorf("sqlserver: database connection must not be nil")
	}
	selected := append([]string(nil), opts.Schemas...)
	seen := make(map[string]bool)
	for _, name := range selected {
		if name == "" {
			return nil, fmt.Errorf("sqlserver: schema name must not be empty")
		}
		if seen[name] {
			return nil, fmt.Errorf("sqlserver: duplicate schema name %q", name)
		}
		seen[name] = true
	}
	sort.Strings(selected)
	return &Inspector{db: db, schemas: selected}, nil
}
func (*Inspector) Engine() schema.Engine { return schema.SQLServer }

// Inspect uses REPEATABLE READ to keep a stable catalog view across queries.
// The inspector issues SELECT statements only; database accounts should be
// configured with read-only permissions and VIEW DEFINITION on target schemas.
func (i *Inspector) Inspect(ctx context.Context) (schema.Snapshot, error) {
	tx, err := i.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return schema.Snapshot{}, fmt.Errorf("sqlserver: begin inspection: %w", err)
	}
	defer tx.Rollback()
	var version int
	if err = tx.QueryRowContext(ctx, query("server.sql")).Scan(&version); err != nil {
		return schema.Snapshot{}, fmt.Errorf("sqlserver: server version: %w", err)
	}
	if version < 16 {
		return schema.Snapshot{}, fmt.Errorf("sqlserver: requires SQL Server 2022 or later, got major %d", version)
	}
	if err = i.validateSchemas(ctx, tx); err != nil {
		return schema.Snapshot{}, err
	}
	var filter any
	if len(i.schemas) > 0 {
		var data []byte
		data, err = json.Marshal(i.schemas)
		if err != nil {
			return schema.Snapshot{}, err
		}
		filter = string(data)
	}
	r := reader{tx: tx, filter: filter, snapshot: schema.Snapshot{Version: schema.FormatVersion, Engine: schema.SQLServer, Tables: []schema.Table{}}, tables: make(map[int64]int), indexes: make(map[indexRef]int), constraints: make(map[constraintRef]int)}
	for _, stage := range []struct {
		label string
		load  func(context.Context) error
	}{
		{"tables", r.loadTables}, {"columns", r.loadColumns}, {"indexes", r.loadIndexes}, {"index keys", r.loadIndexColumns},
		{"key constraints", r.loadKeyConstraints}, {"foreign keys", r.loadForeignKeys}, {"check constraints", r.loadChecks},
	} {
		if err = stage.load(ctx); err != nil {
			return schema.Snapshot{}, fmt.Errorf("sqlserver: %s: %w", stage.label, err)
		}
	}
	canonical, err := snapshot.Canonical(r.snapshot)
	if err != nil {
		return schema.Snapshot{}, fmt.Errorf("sqlserver: invalid catalog metadata: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return schema.Snapshot{}, fmt.Errorf("sqlserver: commit inspection: %w", err)
	}
	return canonical, nil
}

func query(name string) string {
	b, err := queries.ReadFile("queries/" + name)
	if err != nil {
		panic(fmt.Sprintf("sqlserver: missing embedded query %q: %v", name, err))
	}
	return string(b)
}

func (i *Inspector) validateSchemas(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, query("schemas.sql"))
	if err != nil {
		return fmt.Errorf("sqlserver: list schemas: %w", err)
	}
	found := make(map[string]bool)
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			break
		}
		found[name] = true
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return fmt.Errorf("sqlserver: scan schemas: %w", err)
	}
	for _, name := range i.schemas {
		if !found[name] {
			return fmt.Errorf("sqlserver: requested schema %q not found or invisible", name)
		}
	}
	return nil
}

type indexRef struct {
	oid int64
	id  int
}
type constraintRef struct {
	oid  int64
	name string
}
type reader struct {
	tx          *sql.Tx
	filter      any
	snapshot    schema.Snapshot
	tables      map[int64]int
	indexes     map[indexRef]int
	constraints map[constraintRef]int
}

func (r *reader) rows(ctx context.Context, filename string) (*sql.Rows, error) {
	rows, err := r.tx.QueryContext(ctx, query(filename), r.filter)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", filename, err)
	}
	return rows, nil
}
func (r *reader) table(oid int64) (*schema.Table, error) {
	id, found := r.tables[oid]
	if !found {
		return nil, fmt.Errorf("unknown SQL Server table id %d", oid)
	}
	return &r.snapshot.Tables[id], nil
}
func nullable(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}
