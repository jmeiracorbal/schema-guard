package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmeiracorbal/schema-guard/schema"
)

func (r *reader) loadKeyConstraints(ctx context.Context) error {
	rows, err := r.rows(ctx, "key_constraints.sql")
	if err != nil {
		return err
	}
	for rows.Next() {
		var oid int64
		var name, kind, col string
		var ordinal int
		if err = rows.Scan(&oid, &name, &kind, &ordinal, &col); err != nil {
			break
		}
		t, e := r.table(oid)
		if e != nil {
			err = e
			break
		}
		ref := constraintRef{oid, name}
		idx, ok := r.constraints[ref]
		if !ok {
			c := schema.Constraint{Name: name, Columns: []string{}}
			switch kind {
			case "PK":
				c.Type = schema.PrimaryKey
			case "UQ":
				c.Type = schema.UniqueKey
			default:
				err = fmt.Errorf("unknown key constraint type %q", kind)
			}
			if err != nil {
				break
			}
			idx = len(t.Constraints)
			t.Constraints = append(t.Constraints, c)
			r.constraints[ref] = idx
		}
		c := &t.Constraints[idx]
		if ordinal != len(c.Columns)+1 {
			err = fmt.Errorf("key %q has non-contiguous ordinals", name)
			break
		}
		c.Columns = append(c.Columns, col)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	return err
}

func (r *reader) loadForeignKeys(ctx context.Context) error {
	rows, err := r.rows(ctx, "foreign_keys.sql")
	if err != nil {
		return err
	}
	for rows.Next() {
		var oid int64
		var name, col, refSchema, refTable, refColumn, update, del string
		var ordinal int
		var disabled, untrusted, notForReplication bool
		if err = rows.Scan(&oid, &name, &ordinal, &col, &refSchema, &refTable, &refColumn, &update, &del, &disabled, &untrusted, &notForReplication); err != nil {
			break
		}
		if notForReplication {
			err = fmt.Errorf("foreign key %q uses unsupported NOT FOR REPLICATION", name)
			break
		}
		table, e := r.table(oid)
		if e != nil {
			err = e
			break
		}
		ref := constraintRef{oid, name}
		idx, ok := r.constraints[ref]
		if !ok {
			enforced, validated := !disabled, !untrusted
			con := schema.Constraint{Name: name, Type: schema.ForeignKey, Columns: []string{}, Enforced: &enforced, Validated: &validated,
				References: &schema.Reference{Schema: refSchema, Table: refTable, Columns: []string{}, OnUpdate: fkAction(update), OnDelete: fkAction(del), Match: "SIMPLE"}}
			idx = len(table.Constraints)
			table.Constraints = append(table.Constraints, con)
			r.constraints[ref] = idx
		}
		c := &table.Constraints[idx]
		if c.Type != schema.ForeignKey || c.References.Schema != refSchema || c.References.Table != refTable || c.References.OnUpdate != fkAction(update) || c.References.OnDelete != fkAction(del) {
			err = fmt.Errorf("inconsistent foreign key %q", name)
			break
		}
		if ordinal != len(c.Columns)+1 {
			err = fmt.Errorf("foreign key %q has invalid ordinal", name)
			break
		}
		c.Columns = append(c.Columns, col)
		c.References.Columns = append(c.References.Columns, refColumn)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	return err
}

func fkAction(raw string) string { return strings.ReplaceAll(raw, "_", " ") }

func (r *reader) loadChecks(ctx context.Context) error {
	rows, err := r.rows(ctx, "checks.sql")
	if err != nil {
		return err
	}
	for rows.Next() {
		var oid int64
		var name, expression string
		var disabled, untrusted, notForReplication bool
		var colID int
		var column sql.NullString
		if err = rows.Scan(&oid, &name, &expression, &disabled, &untrusted, &notForReplication, &colID, &column); err != nil {
			break
		}
		if notForReplication {
			err = fmt.Errorf("CHECK constraint %q uses unsupported NOT FOR REPLICATION", name)
			break
		}
		table, e := r.table(oid)
		if e != nil {
			err = e
			break
		}
		enforced, validated := !disabled, !untrusted
		check := schema.Constraint{Name: name, Type: schema.Check, Expression: &expression, Definition: &expression, Enforced: &enforced, Validated: &validated}
		if colID > 0 {
			if !column.Valid {
				return fmt.Errorf("CHECK %q missing column name", name)
			}
			check.Columns = []string{column.String}
		}
		table.Constraints = append(table.Constraints, check)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	return err
}
