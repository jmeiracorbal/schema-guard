package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jmeiracorbal/schema-guard/schema"
)

func inspectIndexes(ctx context.Context, tx *sql.Tx, filter any, result *schema.Snapshot, byOID map[int64]int) error {
	rows, err := tx.QueryContext(ctx, query("indexes.sql"), filter)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var oid int64
		var idx schema.Index
		var definition string
		var predicate sql.NullString
		var valid, ready, live bool
		var keysJSON, includedJSON string
		if err := rows.Scan(&oid, &idx.Name, &idx.Unique, &idx.Method, &definition, &predicate,
			&valid, &ready, &live, &keysJSON, &includedJSON); err != nil {
			return fmt.Errorf("scan index: %w", err)
		}
		if !valid || !ready || !live {
			return fmt.Errorf("index %q is invalid, not ready, or being dropped", idx.Name)
		}
		idx.Definition = &definition
		idx.Predicate = optional(predicate)
		if err := json.Unmarshal([]byte(keysJSON), &idx.Columns); err != nil {
			return fmt.Errorf("decode index %q keys: %w", idx.Name, err)
		}
		if err := json.Unmarshal([]byte(includedJSON), &idx.IncludedColumns); err != nil {
			return fmt.Errorf("decode index %q included columns: %w", idx.Name, err)
		}
		table, err := tableAt(result, byOID, oid)
		if err != nil {
			return err
		}
		table.Indexes = append(table.Indexes, idx)
	}
	return rows.Err()
}
