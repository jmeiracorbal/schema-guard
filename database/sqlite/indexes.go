package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmeiracorbal/schema-guard/schema"
)

func inspectIndexes(ctx context.Context, tx *sql.Tx, table *schema.Table, defs map[string]string) error {
	rows, err := tx.QueryContext(ctx, query("indexes.sql"), table.Name, table.Schema)
	if err != nil {
		return fmt.Errorf("indexes %s: %w", table.Name, err)
	}
	type entry struct {
		name, origin    string
		unique, partial bool
	}
	var list []entry
	for rows.Next() {
		var item entry
		var unique, partial int
		if err = rows.Scan(&item.name, &unique, &item.origin, &partial); err != nil {
			break
		}
		item.unique = unique != 0
		item.partial = partial != 0
		list = append(list, item)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return fmt.Errorf("scan indexes %s: %w", table.Name, err)
	}
	for _, item := range list {
		idx := schema.Index{Name: item.name, Unique: item.unique, Method: "btree", Columns: []schema.IndexColumn{}, IncludedColumns: []string{}}
		if item.origin != "c" && item.origin != "u" && item.origin != "pk" {
			return fmt.Errorf("unsupported SQLite index origin %q", item.origin)
		}
		if ddl, ok := defs["index\x00"+item.name]; ok {
			idx.Definition = &ddl
		} else if item.origin == "c" {
			return fmt.Errorf("index %q has no definition", item.name)
		}
		if item.partial && idx.Definition == nil {
			return fmt.Errorf("partial index %q has no definition", item.name)
		}
		keys, err := indexKeys(ctx, tx, table.Schema, item.name)
		if err != nil {
			return err
		}
		idx.Columns = keys
		table.Indexes = append(table.Indexes, idx)
		if item.origin == "u" {
			cols := make([]string, 0, len(keys))
			for _, key := range keys {
				if key.Name == "" {
					return fmt.Errorf("UNIQUE constraint %q references an expression", item.name)
				}
				cols = append(cols, key.Name)
			}
			table.Constraints = append(table.Constraints, schema.Constraint{Name: item.name, Type: schema.UniqueKey, Columns: cols})
		}
	}
	return nil
}

func indexKeys(ctx context.Context, tx *sql.Tx, schemaName, indexName string) ([]schema.IndexColumn, error) {
	rows, err := tx.QueryContext(ctx, query("index_columns.sql"), indexName, schemaName)
	if err != nil {
		return nil, fmt.Errorf("index %q: %w", indexName, err)
	}
	var keys []schema.IndexColumn
	for rows.Next() {
		var seq, cid, descending, isKey int
		var name, coll sql.NullString
		if err = rows.Scan(&seq, &cid, &name, &descending, &coll, &isKey); err != nil {
			break
		}
		if isKey == 0 {
			continue
		}
		if seq != len(keys) {
			err = fmt.Errorf("non-contiguous key ordinals in index %q", indexName)
			break
		}
		key := schema.IndexColumn{Descending: descending != 0}
		if coll.Valid {
			val := coll.String
			key.Collation = &val
		}
		switch cid {
		case -1:
			key.Expression = "ROWID"
		case -2:
			key.Expression = fmt.Sprintf("EXPRESSION_KEY_%d", seq+1) // Native CREATE INDEX SQL preserves its expression.
		default:
			if !name.Valid || name.String == "" {
				err = fmt.Errorf("index %q key has no name", indexName)
				break
			}
			key.Name = name.String
		}
		keys = append(keys, key)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, fmt.Errorf("index %q: %w", indexName, err)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("index %q has no keys", indexName)
	}
	return keys, nil
}
