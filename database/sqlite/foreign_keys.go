package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"github.com/jmeiracorbal/schema-guard/schema"
)

type foreignPart struct {
	seq      int
	from, to string
}
type foreignGroup struct {
	table, update, delete, match string
	parts                        []foreignPart
}

func inspectForeignKeys(ctx context.Context, tx *sql.Tx, table *schema.Table) error {
	rows, err := tx.QueryContext(ctx, query("foreign_keys.sql"), table.Name, table.Schema)
	if err != nil {
		return fmt.Errorf("foreign keys %s: %w", table.Name, err)
	}
	groups := make(map[int]*foreignGroup)
	for rows.Next() {
		var id, seq int
		var referenced, from, update, del, match string
		var to sql.NullString
		if err = rows.Scan(&id, &seq, &referenced, &from, &to, &update, &del, &match); err != nil {
			break
		}
		g := groups[id]
		if g == nil {
			g = &foreignGroup{table: referenced, update: update, delete: del, match: match}
			groups[id] = g
		}
		if g.table != referenced || g.update != update || g.delete != del || g.match != match {
			err = fmt.Errorf("inconsistent foreign key %d", id)
			break
		}
		toName := ""
		if to.Valid {
			toName = to.String
		}
		g.parts = append(g.parts, foreignPart{seq, from, toName})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return fmt.Errorf("scan foreign keys %s: %w", table.Name, err)
	}
	ids := make([]int, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		g := groups[id]
		sort.Slice(g.parts, func(a, b int) bool { return g.parts[a].seq < g.parts[b].seq })
		key := schema.Constraint{Name: fmt.Sprintf("fk_%d", id), Type: schema.ForeignKey, Columns: []string{}, References: &schema.Reference{Schema: table.Schema, Table: g.table, Columns: []string{}, OnUpdate: g.update, OnDelete: g.delete, Match: g.match}}
		for n, p := range g.parts {
			if p.seq != n {
				return fmt.Errorf("non-contiguous foreign key %d columns", id)
			}
			key.Columns = append(key.Columns, p.from)
			if p.to != "" {
				key.References.Columns = append(key.References.Columns, p.to)
			}
		}
		if len(key.References.Columns) == 0 {
			pk, err := referencedPrimaryKey(ctx, tx, table.Schema, g.table)
			if err != nil {
				return fmt.Errorf("foreign key %d: %w", id, err)
			}
			if len(pk) != len(key.Columns) {
				return fmt.Errorf("foreign key %d refers to incompatible primary key", id)
			}
			key.References.Columns = pk
		} else if len(key.References.Columns) != len(key.Columns) {
			return fmt.Errorf("foreign key %d mixes implicit and explicit reference columns", id)
		}
		table.Constraints = append(table.Constraints, key)
	}
	return nil
}

func referencedPrimaryKey(ctx context.Context, tx *sql.Tx, namespace, name string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query("columns.sql"), name, namespace)
	if err != nil {
		return nil, err
	}
	cols := make(map[int]string)
	for rows.Next() {
		var cid, notNull, pk, hidden int
		var column, typ string
		var defaultValue sql.NullString
		if err = rows.Scan(&cid, &column, &typ, &notNull, &defaultValue, &pk, &hidden); err != nil {
			break
		}
		if pk > 0 {
			cols[pk] = column
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(cols))
	for i := range keys {
		col, ok := cols[i+1]
		if !ok {
			return nil, fmt.Errorf("non-contiguous referenced PK")
		}
		keys[i] = col
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("reference %s has no primary key", name)
	}
	return keys, nil
}
