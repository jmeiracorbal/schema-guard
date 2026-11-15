package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmeiracorbal/schema-guard/schema"
)

func inspectTables(ctx context.Context, tx *sql.Tx, filter any, result *schema.Snapshot) (map[int64]int, error) {
	rows, err := tx.QueryContext(ctx, query("tables.sql"), filter)
	if err != nil {
		return nil, fmt.Errorf("postgres: query tables: %w", err)
	}
	defer rows.Close()

	byOID := make(map[int64]int)
	for rows.Next() {
		var oid int64
		var name, namespace, kind, persistence string
		var isPartition bool
		var parentSchema, parentName, partitionKey, partitionBound sql.NullString
		if err := rows.Scan(&oid, &namespace, &name, &kind, &isPartition, &persistence,
			&parentSchema, &parentName, &partitionKey, &partitionBound); err != nil {
			return nil, fmt.Errorf("postgres: scan table: %w", err)
		}
		if _, exists := byOID[oid]; exists {
			return nil, fmt.Errorf("postgres: table %q has multiple inheritance parents (not supported)", name)
		}
		table := schema.Table{Schema: namespace, Name: name, Columns: []schema.Column{},
			Indexes: []schema.Index{}, Constraints: []schema.Constraint{},
			PartitionKey: optional(partitionKey), PartitionBound: optional(partitionBound)}
		switch kind {
		case "r":
			table.Kind = "table"
		case "p":
			table.Kind = "partitioned"
		default:
			return nil, fmt.Errorf("postgres: unsupported relation kind %q for %s.%s", kind, namespace, name)
		}
		if parentSchema.Valid != parentName.Valid {
			return nil, fmt.Errorf("postgres: incomplete parent for %s.%s", namespace, name)
		}
		if parentName.Valid {
			table.Parent = &schema.TableRef{Schema: parentSchema.String, Name: parentName.String}
			if isPartition {
				table.Kind += "_partition"
			} else {
				table.Kind += "_inherited"
			}
		} else if isPartition {
			return nil, fmt.Errorf("postgres: partition %s.%s has no parent", namespace, name)
		}
		switch persistence {
		case "p":
			table.Persistence = "permanent"
		case "u":
			table.Persistence = "unlogged"
		case "t":
			table.Persistence = "temporary"
		default:
			return nil, fmt.Errorf("postgres: unknown table persistence %q", persistence)
		}
		byOID[oid] = len(result.Tables)
		result.Tables = append(result.Tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: query tables: %w", err)
	}
	return byOID, nil
}
