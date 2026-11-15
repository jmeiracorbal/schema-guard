package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmeiracorbal/schema-guard/schema"
)

func inspectColumns(ctx context.Context, tx *sql.Tx, filter any, result *schema.Snapshot, byOID map[int64]int) error {
	rows, err := tx.QueryContext(ctx, query("columns.sql"), filter)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var oid int64
		var col schema.Column
		var expression, collation sql.NullString
		var generated, identity string
		if err := rows.Scan(&oid, &col.Name, &col.Position, &col.DataType, &col.Nullable,
			&expression, &generated, &identity, &collation); err != nil {
			return fmt.Errorf("scan column: %w", err)
		}
		col.Collation = optional(collation)
		switch {
		case generated != "" && identity != "":
			return fmt.Errorf("column %q has both identity and generated attributes", col.Name)
		case generated != "":
			if !expression.Valid {
				return fmt.Errorf("generated column %q has no expression", col.Name)
			}
			switch generated {
			case "s":
				col.Generated = stringPointer("STORED: " + expression.String)
			case "v":
				col.Generated = stringPointer("VIRTUAL: " + expression.String)
			default:
				return fmt.Errorf("column %q has unknown generation kind %q", col.Name, generated)
			}
		case identity != "":
			switch identity {
			case "a":
				col.Generated = stringPointer("IDENTITY ALWAYS")
			case "d":
				col.Generated = stringPointer("IDENTITY BY DEFAULT")
			default:
				return fmt.Errorf("column %q has unknown identity kind %q", col.Name, identity)
			}
		default:
			col.Default = optional(expression)
		}
		table, err := tableAt(result, byOID, oid)
		if err != nil {
			return err
		}
		table.Columns = append(table.Columns, col)
	}
	return rows.Err()
}

func stringPointer(s string) *string { return &s }
