package mysqlcatalog

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmeiracorbal/schema-guard/schema"
)

func (r *reader) loadIndexes(ctx context.Context) error {
	rows, err := r.query(ctx, "indexes.sql")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ns, tableName, indexName, method, visibility string
		var seq, nonUnique int
		var col, expr, collation, comment sql.NullString
		var prefix sql.NullInt64
		if err := rows.Scan(&ns, &tableName, &indexName, &seq, &nonUnique, &method, &col, &expr, &collation, &prefix, &visibility, &comment); err != nil {
			return fmt.Errorf("scan index: %w", err)
		}
		if seq <= 0 || (nonUnique != 0 && nonUnique != 1) {
			return fmt.Errorf("index %s.%s.%s: invalid sequence or uniqueness", ns, tableName, indexName)
		}
		key := schema.IndexColumn{}
		if col.Valid {
			key.Name = col.String
		}
		if expr.Valid {
			key.Expression = expr.String
		}
		if (key.Name == "") == (key.Expression == "") {
			return fmt.Errorf("index %s.%s.%s: missing or ambiguous key", ns, tableName, indexName)
		}
		switch {
		case !collation.Valid:
		case collation.String == "A":
		case collation.String == "D":
			key.Descending = true
		default:
			return fmt.Errorf("index %s.%s.%s: unknown collation %q", ns, tableName, indexName, collation.String)
		}
		if prefix.Valid {
			if prefix.Int64 <= 0 || prefix.Int64 > 2147483647 {
				return fmt.Errorf("index %s.%s.%s: invalid prefix size", ns, tableName, indexName)
			}
			n := int(prefix.Int64)
			key.PrefixLength = &n
		}
		ref := indexRef{Table: schema.TableRef{Schema: ns, Name: tableName}, Name: indexName}
		t, err := r.table(ref.Table)
		if err != nil {
			return err
		}
		if index, ok := r.indexes[ref]; ok {
			existing := &t.Indexes[index]
			if seq != len(existing.Columns)+1 || existing.Unique != (nonUnique == 0) || existing.Method != method || existing.Visibility != visibility || existing.Comment != comment.String {
				return fmt.Errorf("index %s.%s.%s: inconsistent key metadata", ns, tableName, indexName)
			}
			existing.Columns = append(existing.Columns, key)
		} else {
			if seq != 1 {
				return fmt.Errorf("index %s.%s.%s: first key sequence is %d", ns, tableName, indexName, seq)
			}
			i := schema.Index{Name: indexName, Unique: nonUnique == 0, Method: method, Visibility: visibility, Columns: []schema.IndexColumn{key}, IncludedColumns: []string{}}
			if comment.Valid {
				i.Comment = comment.String
			}
			r.indexes[ref] = len(t.Indexes)
			t.Indexes = append(t.Indexes, i)
		}
	}
	return rows.Err()
}

func (r *reader) loadPartitions(ctx context.Context) error {
	rows, err := r.query(ctx, "partitions.sql")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ns, tableName string
		var method, expression sql.NullString
		var partName, subName, description, subMethod, subExpr sql.NullString
		var position, subPosition sql.NullInt64
		if err := rows.Scan(&ns, &tableName, &partName, &position, &method, &expression, &description, &subName, &subPosition, &subMethod, &subExpr); err != nil {
			return fmt.Errorf("scan partition: %w", err)
		}
		if !partName.Valid {
			continue
		} // Non-partitioned tables have a sentinel metadata row.
		if !position.Valid {
			return fmt.Errorf("partition %s.%s.%s missing position", ns, tableName, partName.String)
		}
		p := schema.Partition{Name: partName.String, Position: int(position.Int64), Method: method.String, Expression: expression.String, Description: optional(description)}
		if subName.Valid {
			p.Subpartition = subName.String
		}
		if subPosition.Valid {
			p.SubpartitionPosition = int(subPosition.Int64)
		}
		if subMethod.Valid {
			p.SubpartitionMethod = subMethod.String
		}
		if subExpr.Valid {
			p.SubpartitionExpression = subExpr.String
		}
		t, err := r.table(schema.TableRef{Schema: ns, Name: tableName})
		if err != nil {
			return err
		}
		t.Partitions = append(t.Partitions, p)
	}
	return rows.Err()
}
