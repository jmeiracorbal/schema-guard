package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jmeiracorbal/schema-guard/schema"
)

func inspectConstraints(ctx context.Context, tx *sql.Tx, filter any, result *schema.Snapshot, byOID map[int64]int) error {
	rows, err := tx.QueryContext(ctx, query("constraints.sql"), filter)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var oid int64
		var constraint schema.Constraint
		var kind, definition, columnsJSON, refColumnsJSON string
		var deferrable, initiallyDeferred, validated, noInherit bool
		var expr, refSchema, refName sql.NullString
		var onUpdate, onDelete, match string
		if err := rows.Scan(&oid, &constraint.Name, &kind, &deferrable, &initiallyDeferred,
			&validated, &noInherit, &definition, &expr, &columnsJSON,
			&refSchema, &refName, &refColumnsJSON, &onUpdate, &onDelete, &match); err != nil {
			return fmt.Errorf("scan constraint: %w", err)
		}
		constraint.Definition = &definition
		constraint.Deferrable = &deferrable
		constraint.InitiallyDeferred = &initiallyDeferred
		constraint.Validated = &validated
		constraint.NoInherit = noInherit
		if err := json.Unmarshal([]byte(columnsJSON), &constraint.Columns); err != nil {
			return fmt.Errorf("decode constraint %q columns: %w", constraint.Name, err)
		}
		switch kind {
		case "p":
			constraint.Type = schema.PrimaryKey
		case "u":
			constraint.Type = schema.UniqueKey
		case "f":
			constraint.Type = schema.ForeignKey
			if !refSchema.Valid || !refName.Valid {
				return fmt.Errorf("foreign key %q has no referenced table", constraint.Name)
			}
			ref := &schema.Reference{Schema: refSchema.String, Table: refName.String}
			if err := json.Unmarshal([]byte(refColumnsJSON), &ref.Columns); err != nil {
				return fmt.Errorf("decode foreign key %q reference columns: %w", constraint.Name, err)
			}
			var err error
			if ref.OnUpdate, err = action(onUpdate); err != nil {
				return fmt.Errorf("foreign key %q: %w", constraint.Name, err)
			}
			if ref.OnDelete, err = action(onDelete); err != nil {
				return fmt.Errorf("foreign key %q: %w", constraint.Name, err)
			}
			switch match {
			case "s":
				ref.Match = "SIMPLE"
			case "f":
				ref.Match = "FULL"
			case "p":
				ref.Match = "PARTIAL"
			default:
				return fmt.Errorf("foreign key %q: unknown match type %q", constraint.Name, match)
			}
			constraint.References = ref
		case "c":
			constraint.Type = schema.Check
			constraint.Expression = optional(expr)
		case "x":
			constraint.Type = schema.Exclusion
			constraint.Expression = &definition // expressions/operators are preserved in the native definition
		case "n":
			constraint.Type = schema.NotNull
		default:
			return fmt.Errorf("constraint %q: unsupported PostgreSQL kind %q", constraint.Name, kind)
		}
		table, err := tableAt(result, byOID, oid)
		if err != nil {
			return err
		}
		table.Constraints = append(table.Constraints, constraint)
	}
	return rows.Err()
}

func action(code string) (string, error) {
	switch code {
	case "a":
		return "NO ACTION", nil
	case "r":
		return "RESTRICT", nil
	case "c":
		return "CASCADE", nil
	case "n":
		return "SET NULL", nil
	case "d":
		return "SET DEFAULT", nil
	default:
		return "", fmt.Errorf("unknown foreign key action %q", code)
	}
}
