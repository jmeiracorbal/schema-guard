// Package mysqlcatalog contains the shared metadata reader for the MySQL and
// MariaDB adapters. Vendor-specific SQL is supplied by those adapters.
package mysqlcatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jmeiracorbal/schema-guard/schema"
	"github.com/jmeiracorbal/schema-guard/snapshot"
)

type Options struct{ Schemas []string }

type Inspector struct {
	db      *sql.DB
	engine  schema.Engine
	schemas []string
	queries map[string]string
}

var queryNames = []string{"server.sql", "schemas.sql", "tables.sql", "columns.sql", "partitions.sql", "indexes.sql", "constraints.sql", "constraint_columns.sql", "references.sql"}

func New(db *sql.DB, engine schema.Engine, source fs.FS, opts Options) (*Inspector, error) {
	if db == nil {
		return nil, fmt.Errorf("%s: database connection must not be nil", engine)
	}
	if engine != schema.MySQL && engine != schema.MariaDB {
		return nil, fmt.Errorf("unsupported catalog engine %q", engine)
	}
	names := make(map[string]bool, len(opts.Schemas))
	for _, s := range opts.Schemas {
		if s == "" {
			return nil, fmt.Errorf("%s: schema must not be empty", engine)
		}
		if names[s] {
			return nil, fmt.Errorf("%s: duplicate schema %q", engine, s)
		}
		names[s] = true
	}
	queries := make(map[string]string, len(queryNames))
	for _, file := range queryNames {
		data, err := fs.ReadFile(source, "queries/"+file)
		if err != nil {
			return nil, fmt.Errorf("%s: load %s: %w", engine, file, err)
		}
		queries[file] = string(data)
	}
	selection := append([]string(nil), opts.Schemas...)
	sort.Strings(selection)
	return &Inspector{db: db, engine: engine, schemas: selection, queries: queries}, nil
}

func (i *Inspector) Engine() schema.Engine { return i.engine }

// Inspect reads the catalogs with a read-only transaction and rejects unknown
// schema selections or unsupported metadata, rather than returning partial data.
// DDL concurrent with inspection is not guaranteed to be isolated by a MySQL-
// family engine: callers should avoid schema changes during a capture.
func (i *Inspector) Inspect(ctx context.Context) (schema.Snapshot, error) {
	tx, err := i.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return schema.Snapshot{}, fmt.Errorf("%s: begin read-only transaction: %w", i.engine, err)
	}
	defer tx.Rollback()
	var version, comment string
	if err := tx.QueryRowContext(ctx, i.queries["server.sql"]).Scan(&version, &comment); err != nil {
		return schema.Snapshot{}, fmt.Errorf("%s: identify server: %w", i.engine, err)
	}
	isMariaDB := strings.Contains(strings.ToLower(version+" "+comment), "mariadb")
	if (i.engine == schema.MariaDB) != isMariaDB {
		return schema.Snapshot{}, fmt.Errorf("%s: incompatible server %q (%q)", i.engine, version, comment)
	}
	var filter any
	if len(i.schemas) > 0 {
		data, err := json.Marshal(i.schemas)
		if err != nil {
			return schema.Snapshot{}, err
		}
		filter = string(data)
	}
	catalog := newReader(tx, i.queries, filter, i.engine)
	if err := catalog.validateSchemas(ctx, i.schemas); err != nil {
		return schema.Snapshot{}, err
	}
	if err := catalog.loadTables(ctx); err != nil {
		return schema.Snapshot{}, err
	}
	if err := catalog.loadColumns(ctx); err != nil {
		return schema.Snapshot{}, err
	}
	if err := catalog.loadPartitions(ctx); err != nil {
		return schema.Snapshot{}, err
	}
	if err := catalog.loadIndexes(ctx); err != nil {
		return schema.Snapshot{}, err
	}
	if err := catalog.loadConstraints(ctx); err != nil {
		return schema.Snapshot{}, err
	}
	if err := catalog.loadConstraintColumns(ctx); err != nil {
		return schema.Snapshot{}, err
	}
	if err := catalog.loadReferences(ctx); err != nil {
		return schema.Snapshot{}, err
	}
	result, err := snapshot.Canonical(catalog.snapshot)
	if err != nil {
		return schema.Snapshot{}, fmt.Errorf("%s: invalid catalog metadata: %w", i.engine, err)
	}
	if err := tx.Commit(); err != nil {
		return schema.Snapshot{}, fmt.Errorf("%s: commit read-only transaction: %w", i.engine, err)
	}
	return result, nil
}

type reader struct {
	tx          *sql.Tx
	queries     map[string]string
	filter      any
	engine      schema.Engine
	snapshot    schema.Snapshot
	tables      map[schema.TableRef]int
	indexes     map[indexRef]int
	constraints map[constraintRef]int
}
type indexRef struct {
	Table schema.TableRef
	Name  string
}
type constraintRef struct {
	Table schema.TableRef
	Name  string
}

func newReader(tx *sql.Tx, queries map[string]string, filter any, engine schema.Engine) *reader {
	return &reader{tx: tx, queries: queries, filter: filter, engine: engine,
		snapshot: schema.Snapshot{Version: schema.FormatVersion, Engine: engine, Tables: []schema.Table{}},
		tables:   make(map[schema.TableRef]int), indexes: make(map[indexRef]int), constraints: make(map[constraintRef]int)}
}
func (r *reader) query(ctx context.Context, file string) (*sql.Rows, error) {
	rows, err := r.tx.QueryContext(ctx, r.queries[file], r.filter, r.filter)
	if err != nil {
		return nil, fmt.Errorf("%s: query %s: %w", r.engine, file, err)
	}
	return rows, nil
}
func (r *reader) table(ref schema.TableRef) (*schema.Table, error) {
	i, ok := r.tables[ref]
	if !ok {
		return nil, fmt.Errorf("%s: catalog references unknown table %s.%s", r.engine, ref.Schema, ref.Name)
	}
	return &r.snapshot.Tables[i], nil
}
func optional(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}
func (r *reader) validateSchemas(ctx context.Context, selected []string) error {
	if len(selected) == 0 {
		return nil
	}
	rows, err := r.query(ctx, "schemas.sql")
	if err != nil {
		return err
	}
	defer rows.Close()
	found := make(map[string]bool)
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return fmt.Errorf("scan schema: %w", err)
		}
		found[name] = true
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, name := range selected {
		if !found[name] {
			return fmt.Errorf("%s: requested schema %q unavailable or excluded", r.engine, name)
		}
	}
	return nil
}
