// Package sqlite inspects SQLite database schemas through sqlite_schema and
// table-valued PRAGMA functions. It never parses DDL or executes migrations.
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jmeiracorbal/schema-guard/internal/domain"
	"github.com/jmeiracorbal/schema-guard/schema"
	"github.com/jmeiracorbal/schema-guard/snapshot"
)

//go:embed queries/*.sql
var queries embed.FS

// Options selects SQLite databases. The default is the main database; an
// attached database must be selected explicitly by its database name.
type Options struct{ Schemas []string }

type Inspector struct {
	db      *sql.DB
	schemas []string
}

var _ domain.Inspector = (*Inspector)(nil)

func New(db *sql.DB, opts Options) (*Inspector, error) {
	if db == nil {
		return nil, fmt.Errorf("sqlite: database connection must not be nil")
	}
	schemas := opts.Schemas
	if len(schemas) == 0 {
		schemas = []string{"main"}
	}
	unique := make(map[string]bool)
	for _, name := range schemas {
		if name == "" || strings.IndexByte(name, 0) >= 0 {
			return nil, fmt.Errorf("sqlite: schema names must be nonempty and exclude NUL")
		}
		if unique[name] {
			return nil, fmt.Errorf("sqlite: duplicate schema %q", name)
		}
		unique[name] = true
	}
	selected := append([]string(nil), schemas...)
	sort.Strings(selected)
	return &Inspector{db: db, schemas: selected}, nil
}

func (*Inspector) Engine() schema.Engine { return schema.SQLite }

// Inspect acquires one connection and a transaction so that all PRAGMA reads
// share the same consistent SQLite read snapshot. The supplied database/sql
// driver and connection determine whether the database is opened read-only.
func (i *Inspector) Inspect(ctx context.Context) (schema.Snapshot, error) {
	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return schema.Snapshot{}, fmt.Errorf("sqlite: begin read transaction: %w", err)
	}
	defer tx.Rollback()
	discovered, err := listSchemas(ctx, tx)
	if err != nil {
		return schema.Snapshot{}, err
	}
	result := schema.Snapshot{Version: schema.FormatVersion, Engine: schema.SQLite, Tables: []schema.Table{}}
	for _, name := range i.schemas {
		if !discovered[name] {
			return schema.Snapshot{}, fmt.Errorf("sqlite: schema %q is not attached", name)
		}
		if err = inspectSchema(ctx, tx, name, &result); err != nil {
			return schema.Snapshot{}, fmt.Errorf("sqlite: schema %q: %w", name, err)
		}
	}
	canonical, err := snapshot.Canonical(result)
	if err != nil {
		return schema.Snapshot{}, fmt.Errorf("sqlite: invalid catalog metadata: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return schema.Snapshot{}, fmt.Errorf("sqlite: commit read transaction: %w", err)
	}
	return canonical, nil
}

func query(name string) string {
	b, err := queries.ReadFile("queries/" + name)
	if err != nil {
		panic(fmt.Sprintf("sqlite: missing embedded query %q: %v", name, err))
	}
	return string(b)
}

func listSchemas(ctx context.Context, tx *sql.Tx) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, query("schemas.sql"))
	if err != nil {
		return nil, fmt.Errorf("sqlite: query schemas: %w", err)
	}
	defer rows.Close()
	found := make(map[string]bool)
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, err
		}
		found[name] = true
	}
	return found, rows.Err()
}

func inspectSchema(ctx context.Context, tx *sql.Tx, name string, result *schema.Snapshot) error {
	defs, err := loadDefinitions(ctx, tx, name)
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, query("tables.sql"), name)
	if err != nil {
		return fmt.Errorf("list tables: %w", err)
	}
	var tables []schema.Table
	for rows.Next() {
		var table schema.Table
		var kind string
		var withoutRowID, strict int
		if err = rows.Scan(&table.Name, &kind, &withoutRowID, &strict); err != nil {
			break
		}
		if kind != "table" {
			err = fmt.Errorf("unsupported SQLite relation %q (%s)", table.Name, kind)
			break
		}
		ddl, ok := defs["table\x00"+table.Name]
		if !ok || ddl == "" {
			err = fmt.Errorf("missing CREATE TABLE definition for %q", table.Name)
			break
		}
		table.Schema = name
		table.Kind = "table"
		if withoutRowID != 0 {
			table.Kind = "without_rowid"
		}
		if strict != 0 {
			table.Persistence = "strict"
		}
		table.Definition = &ddl
		table.Columns = []schema.Column{}
		table.Indexes = []schema.Index{}
		table.Constraints = []schema.Constraint{}
		tables = append(tables, table)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return fmt.Errorf("scan tables: %w", err)
	}
	for j := range tables {
		t := &tables[j]
		if err = inspectColumns(ctx, tx, t); err != nil {
			return err
		}
		if err = inspectIndexes(ctx, tx, t, defs); err != nil {
			return err
		}
		if err = inspectForeignKeys(ctx, tx, t); err != nil {
			return err
		}
	}
	result.Tables = append(result.Tables, tables...)
	return nil
}

// Only schema-qualified identifiers are interpolated, after strict validation
// against the active sqlite database list. Values are always bound parameters.
func loadDefinitions(ctx context.Context, tx *sql.Tx, name string) (map[string]string, error) {
	statement := strings.Replace(query("definitions.sql"), "{{schema}}", quoteIdentifier(name), 1)
	rows, err := tx.QueryContext(ctx, statement)
	if err != nil {
		return nil, fmt.Errorf("load definitions: %w", err)
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var kind, n string
		var ddl sql.NullString
		if err = rows.Scan(&kind, &n, &ddl); err != nil {
			return nil, err
		}
		if ddl.Valid {
			out[kind+"\x00"+n] = ddl.String
		}
	}
	return out, rows.Err()
}

func quoteIdentifier(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }
