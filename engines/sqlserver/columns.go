package sqlserver

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmeiracorbal/schema-guard/schema"
)

func (r *reader) loadColumns(ctx context.Context) error {
	rows, err := r.rows(ctx, "columns.sql")
	if err != nil {
		return err
	}
	for rows.Next() {
		var oid int64
		var col schema.Column
		var def, computed, collation sql.NullString
		var persisted, notForReplication sql.NullBool
		var seed, increment sql.NullString
		var userDefined, assembly, hidden, sparse, columnSet, fileStream bool
		var generated int
		var encryption sql.NullInt64
		if err = rows.Scan(&oid, &col.Name, &col.Position, &col.DataType, &col.Nullable,
			&def, &computed, &persisted, &seed, &increment, &notForReplication,
			&collation, &userDefined, &assembly, &hidden, &sparse, &columnSet, &fileStream, &generated, &encryption); err != nil {
			break
		}
		if userDefined || assembly || hidden || sparse || columnSet || fileStream || generated != 0 || encryption.Valid {
			err = fmt.Errorf("column %q has unsupported user-defined, sparse, hidden, generated-always, filestream, or encrypted metadata", col.Name)
			break
		}
		col.Default = nullable(def)
		col.Collation = nullable(collation)
		if computed.Valid && seed.Valid {
			err = fmt.Errorf("column %q is both computed and identity", col.Name)
			break
		}
		if computed.Valid {
			value := "COMPUTED: " + computed.String
			if persisted.Valid && persisted.Bool {
				value = "COMPUTED PERSISTED: " + computed.String
			}
			col.Generated = &value
		}
		if seed.Valid {
			if !increment.Valid {
				err = fmt.Errorf("identity column %q lacks increment", col.Name)
				break
			}
			if notForReplication.Valid && notForReplication.Bool {
				err = fmt.Errorf("identity column %q has unsupported NOT FOR REPLICATION", col.Name)
				break
			}
			val := fmt.Sprintf("IDENTITY(%s,%s)", seed.String, increment.String)
			col.Generated = &val
		}
		table, e := r.table(oid)
		if e != nil {
			err = e
			break
		}
		table.Columns = append(table.Columns, col)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	return err
}
