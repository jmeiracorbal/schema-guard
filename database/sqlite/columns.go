package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"github.com/jmeiracorbal/schema-guard/schema"
)

func inspectColumns(ctx context.Context, tx *sql.Tx, table *schema.Table) error {
	rows, err := tx.QueryContext(ctx, query("columns.sql"), table.Name, table.Schema)
	if err != nil {
		return fmt.Errorf("columns %s: %w", table.Name, err)
	}
	type pkColumn struct {
		pos  int
		name string
	}
	keys := []pkColumn{}
	for rows.Next() {
		var cid, notNull, pk, hidden int
		var col schema.Column
		var defaultValue sql.NullString
		if err = rows.Scan(&cid, &col.Name, &col.DataType, &notNull, &defaultValue, &pk, &hidden); err != nil {
			break
		}
		if hidden == 1 {
			err = fmt.Errorf("virtual/hidden column %q is unsupported", col.Name)
			break
		}
		if hidden != 0 && hidden != 2 && hidden != 3 {
			err = fmt.Errorf("unrecognized hidden flag %d on %q", hidden, col.Name)
			break
		}
		col.Position = cid + 1
		col.Nullable = notNull == 0
		if defaultValue.Valid {
			v := defaultValue.String
			col.Default = &v
		}
		if hidden == 2 {
			v := "VIRTUAL"
			col.Generated = &v
		}
		if hidden == 3 {
			v := "STORED"
			col.Generated = &v
		}
		if pk > 0 {
			keys = append(keys, pkColumn{pk, col.Name})
		}
		table.Columns = append(table.Columns, col)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return fmt.Errorf("scan columns %s: %w", table.Name, err)
	}
	if len(keys) > 0 {
		sort.Slice(keys, func(i, j int) bool { return keys[i].pos < keys[j].pos })
		names := make([]string, len(keys))
		for j, k := range keys {
			if k.pos != j+1 {
				return fmt.Errorf("invalid primary key position %d in %s", k.pos, table.Name)
			}
			names[j] = k.name
		}
		table.Constraints = append(table.Constraints, schema.Constraint{Name: "PRIMARY", Type: schema.PrimaryKey, Columns: names})
	}
	return nil
}
