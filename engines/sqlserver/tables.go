package sqlserver

import (
	"context"
	"fmt"

	"github.com/jmeiracorbal/schema-guard/schema"
)

func (r *reader) loadTables(ctx context.Context) error {
	rows, err := r.rows(ctx, "tables.sql")
	if err != nil {
		return err
	}
	for rows.Next() {
		var oid int64
		var name, namespace string
		var temporal, ledger int
		var memory, filetable, node, edge bool
		if err = rows.Scan(&oid, &namespace, &name, &temporal, &memory, &filetable, &node, &edge, &ledger); err != nil {
			break
		}
		if temporal != 0 || memory || filetable || node || edge || ledger != 0 {
			err = fmt.Errorf("unsupported specialized table %s.%s (temporal=%d memory=%t filetable=%t graph=%t/%t ledger=%d)", namespace, name, temporal, memory, filetable, node, edge, ledger)
			break
		}
		if _, exists := r.tables[oid]; exists {
			err = fmt.Errorf("duplicate table id %d", oid)
			break
		}
		r.tables[oid] = len(r.snapshot.Tables)
		r.snapshot.Tables = append(r.snapshot.Tables, schema.Table{Schema: namespace, Name: name, Kind: "table", Columns: []schema.Column{}, Indexes: []schema.Index{}, Constraints: []schema.Constraint{}})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	return err
}
