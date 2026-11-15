package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/jmeiracorbal/schema-guard/schema"
)

func (r *reader) loadIndexes(ctx context.Context) error {
	rows, err := r.rows(ctx, "indexes.sql")
	if err != nil {
		return err
	}
	for rows.Next() {
		var oid int64
		var id, kind, fillFactor int
		var idx schema.Index
		var kindName string
		var filter sql.NullString
		var disabled, hypothetical, padded, ignoreDuplicates, allowRows, allowPages bool
		if err = rows.Scan(&oid, &id, &idx.Name, &idx.Unique, &kind, &kindName, &filter, &disabled, &hypothetical, &fillFactor, &padded, &ignoreDuplicates, &allowRows, &allowPages); err != nil {
			break
		}
		if kind != 1 && kind != 2 {
			err = fmt.Errorf("index %s has unsupported type %q", idx.Name, kindName)
			break
		}
		if idx.Name == "" {
			err = fmt.Errorf("unnamed index on object %d", oid)
			break
		}
		if disabled || hypothetical || fillFactor != 0 || padded || ignoreDuplicates || !allowRows || !allowPages {
			err = fmt.Errorf("index %s has unsupported physical options or state", idx.Name)
			break
		}
		idx.Method = strings.ToLower(kindName)
		idx.Predicate = nullable(filter)
		idx.Columns = []schema.IndexColumn{}
		idx.IncludedColumns = []string{}
		table, e := r.table(oid)
		if e != nil {
			err = e
			break
		}
		key := indexRef{oid, id}
		if _, exists := r.indexes[key]; exists {
			err = fmt.Errorf("duplicate index %s", idx.Name)
			break
		}
		r.indexes[key] = len(table.Indexes)
		table.Indexes = append(table.Indexes, idx)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	return err
}

func (r *reader) loadIndexColumns(ctx context.Context) error {
	rows, err := r.rows(ctx, "index_columns.sql")
	if err != nil {
		return err
	}
	for rows.Next() {
		var oid int64
		var id, ordinal, keyOrdinal int
		var name string
		var descending, included bool
		if err = rows.Scan(&oid, &id, &ordinal, &keyOrdinal, &name, &descending, &included); err != nil {
			break
		}
		table, e := r.table(oid)
		if e != nil {
			err = e
			break
		}
		ix, found := r.indexes[indexRef{oid, id}]
		if !found {
			err = fmt.Errorf("index %d on table %d not loaded", id, oid)
			break
		}
		index := &table.Indexes[ix]
		if included {
			if keyOrdinal != 0 {
				err = fmt.Errorf("included column %q has nonzero key ordinal", name)
				break
			}
			index.IncludedColumns = append(index.IncludedColumns, name)
		} else {
			if keyOrdinal != len(index.Columns)+1 {
				err = fmt.Errorf("index %q key ordinals are not contiguous", index.Name)
				break
			}
			index.Columns = append(index.Columns, schema.IndexColumn{Name: name, Descending: descending})
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	for _, table := range r.snapshot.Tables {
		for _, idx := range table.Indexes {
			if len(idx.Columns) == 0 {
				return fmt.Errorf("index %q has no keys", idx.Name)
			}
			sort.Strings(idx.IncludedColumns)
		}
	}
	return nil
}
