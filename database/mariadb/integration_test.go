//go:build integration

package mariadb_test

import (
	"context"
	"database/sql"
	"embed"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmeiracorbal/schema-guard/diff"
	"github.com/jmeiracorbal/schema-guard/database/mariadb"
	"github.com/jmeiracorbal/schema-guard/schema"
	"github.com/jmeiracorbal/schema-guard/snapshot"
)

//go:embed testdata/*.sql
var fixtures embed.FS

func TestMariaDBIntegration(t *testing.T) {
	dsn := os.Getenv("SCHEMA_GUARD_MARIADB_DSN")
	if dsn == "" {
		t.Fatal("SCHEMA_GUARD_MARIADB_DSN must target an isolated MariaDB database")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name); err != nil {
		t.Fatal(err)
	}
	cleanup := func(ctx context.Context) {
		for _, f := range []string{"drop_orders.sql", "drop_partition.sql", "drop_accounts.sql"} {
			fixture(t, ctx, db, f)
		}
	}
	cleanup(ctx)
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cleanup(c)
	})
	for _, f := range []string{"accounts.sql", "orders.sql", "partition.sql", "special_index.sql", "expression_index.sql"} {
		fixture(t, ctx, db, f)
	}
	inspect, err := mariadb.New(db, mariadb.Options{Schemas: []string{name}})
	if err != nil {
		t.Fatal(err)
	}
	first, err := inspect.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first.Engine != schema.MariaDB || len(first.Tables) != 3 {
		t.Fatalf("invalid MySQL snapshot: %+v", first)
	}
	accounts := table(t, first, "sg_accounts")
	if len(accounts.Columns) != 4 {
		t.Fatalf("unexpected columns: %+v", accounts.Columns)
	}
	if col := column(t, accounts, "normalized_email"); col.Generated == nil {
		t.Fatalf("generated metadata lost: %+v", col)
	}
	var hasPrefix, hasIgnored bool
	for _, idx := range accounts.Indexes {
		switch idx.Name {
		case "idx_account_prefix":
			hasPrefix = len(idx.Columns) == 1 && idx.Columns[0].PrefixLength != nil && *idx.Columns[0].PrefixLength == 12
		case "idx_account_state":
			hasIgnored = idx.Visibility == "IGNORED"

		}
	}
	if !hasPrefix || !hasIgnored {
		t.Fatalf("missing index metadata: %+v", accounts.Indexes)
	}
	verifyShared(t, ctx, db, inspect, first, name)
}

func verifyShared(t *testing.T, ctx context.Context, db *sql.DB, inspect *mariadb.Inspector, first schema.Snapshot, name string) {
	t.Helper()
	orders := table(t, first, "sg_orders")
	var fk bool
	for _, c := range orders.Constraints {
		if c.Type == schema.ForeignKey {
			fk = true
			if c.References == nil || c.References.Table != "sg_accounts" || c.References.OnUpdate != "CASCADE" || c.References.OnDelete != "RESTRICT" {
				t.Fatalf("incomplete FK: %+v", c)
			}
		}
	}
	if !fk {
		t.Fatal("foreign key missing")
	}
	if parts := table(t, first, "sg_partitioned").Partitions; len(parts) != 2 || parts[0].Name != "p0" {
		t.Fatalf("incomplete partitions: %+v", parts)
	}
	a, err := snapshot.Hash(first)
	if err != nil {
		t.Fatal(err)
	}
	next, err := inspect.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := snapshot.Hash(next)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("repeated snapshot hash changed %s != %s", a, b)
	}
	same, err := diff.Compare(first, next)
	if err != nil || !same.Empty() {
		t.Fatalf("unexpected self drift: %+v, %v", same, err)
	}
	fixture(t, ctx, db, "add_column.sql")
	after, err := inspect.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	change, err := diff.Compare(first, after)
	if err != nil {
		t.Fatal(err)
	}
	var added bool
	for _, c := range change.Columns {
		if c.Name == "observed_only" && c.Kind == diff.Added {
			added = true
		}
	}
	if !added {
		t.Fatalf("new column not detected: %+v", change)
	}
	bad, err := mariadb.New(db, mariadb.Options{Schemas: []string{name + "' OR 1=1 --"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bad.Inspect(ctx); err == nil {
		t.Fatal("invalid schema selection did not fail")
	}
}

func fixture(t *testing.T, ctx context.Context, db *sql.DB, name string) {
	t.Helper()
	data, err := fixtures.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(data)); err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
}
func table(t *testing.T, s schema.Snapshot, name string) schema.Table {
	t.Helper()
	for _, v := range s.Tables {
		if v.Name == name {
			return v
		}
	}
	t.Fatalf("table %q missing", name)
	return schema.Table{}
}
func column(t *testing.T, s schema.Table, name string) schema.Column {
	t.Helper()
	for _, v := range s.Columns {
		if v.Name == name {
			return v
		}
	}
	t.Fatalf("column %q missing", name)
	return schema.Column{}
}
