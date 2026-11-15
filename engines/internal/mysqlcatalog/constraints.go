package mysqlcatalog

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmeiracorbal/schema-guard/schema"
)

func (r *reader) loadConstraints(ctx context.Context) error {
	rows, err := r.query(ctx, "constraints.sql")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ns, tableName, conName, kind string
		var enforced, expr sql.NullString
		if err := rows.Scan(&ns, &tableName, &conName, &kind, &enforced, &expr); err != nil {
			return fmt.Errorf("scan constraint: %w", err)
		}
		con := schema.Constraint{Name: conName, Columns: []string{}}
		switch kind {
		case "PRIMARY KEY":
			con.Type = schema.PrimaryKey
		case "UNIQUE":
			con.Type = schema.UniqueKey
		case "FOREIGN KEY":
			con.Type = schema.ForeignKey
		case "CHECK":
			con.Type = schema.Check
			con.Expression = optional(expr)
			if con.Expression == nil || *con.Expression == "" {
				return fmt.Errorf("check constraint %q missing expression", conName)
			}
		default:
			return fmt.Errorf("%s: unsupported constraint %q of type %q", r.engine, conName, kind)
		}
		if enforced.Valid {
			switch enforced.String {
			case "YES":
				v := true
				con.Enforced = &v
			case "NO":
				v := false
				con.Enforced = &v
			default:
				return fmt.Errorf("constraint %q: invalid enforced flag %q", conName, enforced.String)
			}
		}
		ref := constraintRef{Table: schema.TableRef{Schema: ns, Name: tableName}, Name: conName}
		t, err := r.table(ref.Table)
		if err != nil {
			return err
		}
		if _, ok := r.constraints[ref]; ok {
			return fmt.Errorf("duplicate constraint %q on %s.%s", conName, ns, tableName)
		}
		r.constraints[ref] = len(t.Constraints)
		t.Constraints = append(t.Constraints, con)
	}
	return rows.Err()
}

func (r *reader) constraint(ref constraintRef) (*schema.Constraint, error) {
	t, err := r.table(ref.Table)
	if err != nil {
		return nil, err
	}
	idx, ok := r.constraints[ref]
	if !ok {
		return nil, fmt.Errorf("%s: unknown constraint %s on %s.%s", r.engine, ref.Name, ref.Table.Schema, ref.Table.Name)
	}
	return &t.Constraints[idx], nil
}

func (r *reader) loadConstraintColumns(ctx context.Context) error {
	rows, err := r.query(ctx, "constraint_columns.sql")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ns, tableName, conName, colName string
		var ordinal int
		var refSchema, refTable, refColumn sql.NullString
		if err := rows.Scan(&ns, &tableName, &conName, &ordinal, &colName, &refSchema, &refTable, &refColumn); err != nil {
			return fmt.Errorf("scan constraint column: %w", err)
		}
		ref := constraintRef{Table: schema.TableRef{Schema: ns, Name: tableName}, Name: conName}
		con, err := r.constraint(ref)
		if err != nil {
			return err
		}
		if con.Type == schema.Check {
			return fmt.Errorf("check constraint %q has key columns", conName)
		}
		if ordinal != len(con.Columns)+1 {
			return fmt.Errorf("constraint %q: non-sequential key columns", conName)
		}
		con.Columns = append(con.Columns, colName)
		if con.Type == schema.ForeignKey {
			if !refSchema.Valid || !refTable.Valid || !refColumn.Valid {
				return fmt.Errorf("foreign key %q missing referenced column", conName)
			}
			if con.References == nil {
				con.References = &schema.Reference{Schema: refSchema.String, Table: refTable.String, Columns: []string{}}
			}
			if con.References.Schema != refSchema.String || con.References.Table != refTable.String {
				return fmt.Errorf("foreign key %q has inconsistent referenced table", conName)
			}
			con.References.Columns = append(con.References.Columns, refColumn.String)
		} else if refSchema.Valid || refTable.Valid || refColumn.Valid {
			return fmt.Errorf("non-FK constraint %q unexpectedly references a table", conName)
		}
	}
	return rows.Err()
}

func (r *reader) loadReferences(ctx context.Context) error {
	rows, err := r.query(ctx, "references.sql")
	if err != nil {
		return err
	}
	defer rows.Close()
	seen := make(map[constraintRef]bool)
	for rows.Next() {
		var ns, tableName, conName, update, delete, match string
		if err := rows.Scan(&ns, &tableName, &conName, &update, &delete, &match); err != nil {
			return fmt.Errorf("scan foreign key action: %w", err)
		}
		ref := constraintRef{Table: schema.TableRef{Schema: ns, Name: tableName}, Name: conName}
		if seen[ref] {
			return fmt.Errorf("duplicate reference rule for %q", conName)
		}
		seen[ref] = true
		con, err := r.constraint(ref)
		if err != nil {
			return err
		}
		if con.Type != schema.ForeignKey || con.References == nil {
			return fmt.Errorf("constraint %q: missing foreign key columns", conName)
		}
		con.References.OnUpdate = update
		con.References.OnDelete = delete
		con.References.Match = match
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for ref := range r.constraints {
		con, err := r.constraint(ref)
		if err != nil {
			return err
		}
		if con.Type == schema.ForeignKey && !seen[ref] {
			return fmt.Errorf("foreign key %q missing referential actions", con.Name)
		}
	}
	return nil
}
