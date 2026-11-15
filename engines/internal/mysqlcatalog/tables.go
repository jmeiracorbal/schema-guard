package mysqlcatalog

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmeiracorbal/schema-guard/schema"
)

func (r *reader) loadTables(ctx context.Context) error {
	rows, err := r.query(ctx, "tables.sql")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ns, name string
		var engine, collation, rowFormat, opts sql.NullString
		if err := rows.Scan(&ns, &name, &engine, &collation, &rowFormat, &opts); err != nil {
			return fmt.Errorf("scan table: %w", err)
		}
		if !engine.Valid || engine.String == "" {
			return fmt.Errorf("%s: table %s.%s has no storage engine", r.engine, ns, name)
		}
		t := schema.Table{Kind: "table", Schema: ns, Name: name, StorageEngine: engine.String, Collation: optional(collation),
			Columns: []schema.Column{}, Indexes: []schema.Index{}, Constraints: []schema.Constraint{}}
		if rowFormat.Valid {
			t.RowFormat = rowFormat.String
		}
		if opts.Valid {
			t.CreateOptions = opts.String
		}
		ref := t.Ref()
		if _, ok := r.tables[ref]; ok {
			return fmt.Errorf("duplicate table %s.%s", ns, name)
		}
		r.tables[ref] = len(r.snapshot.Tables)
		r.snapshot.Tables = append(r.snapshot.Tables, t)
	}
	return rows.Err()
}
func (r *reader) loadColumns(ctx context.Context) error {
	rows, err := r.query(ctx, "columns.sql")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ns, name, nullable, extra string
		var col schema.Column
		var def, generated, collation, charset, comment sql.NullString
		if err := rows.Scan(&ns, &name, &col.Name, &col.Position, &col.DataType, &nullable, &def, &generated, &extra, &collation, &charset, &comment); err != nil {
			return fmt.Errorf("scan column: %w", err)
		}
		switch nullable {
		case "YES":
			col.Nullable = true
		case "NO":
		default:
			return fmt.Errorf("column %s.%s.%s: unexpected nullability %q", ns, name, col.Name, nullable)
		}
		col.Extra = extra
		col.Default = optional(def)
		col.Collation = optional(collation)
		if charset.Valid {
			col.CharacterSet = charset.String
		}
		if comment.Valid {
			col.Comment = comment.String
		}
		if generated.Valid && generated.String != "" {
			// EXTRA is a catalog attribute, not DDL. Reject unknown forms.
			switch {
			case containsFold(extra, "VIRTUAL GENERATED"):
				val := "VIRTUAL: " + generated.String
				col.Generated = &val
			case containsFold(extra, "STORED GENERATED"), containsFold(extra, "PERSISTENT GENERATED"):
				val := "STORED: " + generated.String
				col.Generated = &val
			default:
				return fmt.Errorf("column %s.%s.%s: unrecognized generated mode %q", ns, name, col.Name, extra)
			}
		} else if containsFold(extra, " GENERATED") {
			return fmt.Errorf("column %s.%s.%s: generated column has no expression", ns, name, col.Name)
		}
		table, err := r.table(schema.TableRef{Schema: ns, Name: name})
		if err != nil {
			return err
		}
		table.Columns = append(table.Columns, col)
	}
	return rows.Err()
}
func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToUpper(haystack), needle)
}
