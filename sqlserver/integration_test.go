//go:build integration

package sqlserver_test

import (
	"context"
	"database/sql"
	"embed"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/microsoft/go-mssqldb"

	"github.com/jmeiracorbal/schema-guard/diff"
	"github.com/jmeiracorbal/schema-guard/schema"
	"github.com/jmeiracorbal/schema-guard/snapshot"
	"github.com/jmeiracorbal/schema-guard/sqlserver"
)

//go:embed testdata/*.sql
var fixtures embed.FS

func TestSQLServerIntegration(t *testing.T) {
	dsn := os.Getenv("SCHEMA_GUARD_SQLSERVER_DSN")
	if dsn == "" {
		t.Fatal("SCHEMA_GUARD_SQLSERVER_DSN must point to an isolated SQL Server 2022 test instance")
	}
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	// SQL Server images can report 'started' before the database is accepting logins.
	var connected bool
	for n := 0; n < 50; n++ {
		probe, stop := context.WithTimeout(ctx, 5*time.Second)
		err = db.PingContext(probe)
		stop()
		if err == nil {
			connected = true
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("SQL Server not ready: %v", ctx.Err())
		case <-time.After(time.Second):
		}
	}
	if !connected {
		t.Fatalf("SQL Server connection unavailable: %v", err)
	}
	for _, name := range []string{"drop_orders.sql", "drop_accounts.sql", "drop_schema.sql"} {
		run(t, ctx, db, name)
	}
	t.Cleanup(func() {
		cleanupCtx, finish := context.WithTimeout(context.Background(), 20*time.Second)
		defer finish()
		for _, name := range []string{"drop_orders.sql", "drop_accounts.sql", "drop_schema.sql"} {
			run(t, cleanupCtx, db, name)
		}
	})
	for _, name := range []string{"create_schema.sql", "create_accounts.sql", "create_orders.sql", "create_index.sql"} {
		run(t, ctx, db, name)
	}
	inspector, err := sqlserver.New(db, sqlserver.Options{Schemas: []string{"schema_guard_integration"}})
	if err != nil {
		t.Fatal(err)
	}
	first, err := inspector.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Tables) != 2 {
		t.Fatalf("expected two tables, got %d", len(first.Tables))
	}
	accounts := find(t, first, "accounts")
	if id := findCol(t, accounts, "id"); id.Generated == nil || !strings.HasPrefix(*id.Generated, "IDENTITY(") {
		t.Fatalf("identity not preserved: %+v", id)
	}
	if gen := findCol(t, accounts, "normalized_email"); gen.Generated == nil || !strings.Contains(*gen.Generated, "PERSISTED") {
		t.Fatalf("computed column not preserved: %+v", gen)
	}
	var hasCheck, hasPK, hasIndex bool
	for _, c := range accounts.Constraints {
		if c.Type == schema.Check && c.Name == "ck_accounts_state" && c.Expression != nil {
			hasCheck = true
		}
		if c.Type == schema.PrimaryKey && c.Name == "pk_accounts" {
			hasPK = true
		}
	}
	for _, idx := range accounts.Indexes {
		if idx.Name == "idx_accounts_state" && len(idx.Columns) == 1 && idx.Columns[0].Descending && len(idx.IncludedColumns) == 1 {
			hasIndex = true
		}
	}
	if !hasCheck || !hasPK || !hasIndex {
		t.Fatalf("catalog metadata missing: check=%t pk=%t index=%t", hasCheck, hasPK, hasIndex)
	}
	orders := find(t, first, "orders")
	var foundFK bool
	for _, c := range orders.Constraints {
		if c.Type == schema.ForeignKey && c.References != nil && c.References.OnDelete == "CASCADE" {
			foundFK = true
		}
	}
	if !foundFK {
		t.Fatal("foreign key not detected")
	}
	hash1, err := snapshot.Hash(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := inspector.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	hash2, err := snapshot.Hash(second)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 != hash2 {
		t.Fatalf("non-deterministic snapshot %s != %s", hash1, hash2)
	}
	equal, err := diff.Compare(first, first)
	if err != nil || !equal.Empty() {
		t.Fatalf("self diff is not empty: %+v %v", equal, err)
	}
	run(t, ctx, db, "add_column.sql")
	after, err := inspector.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := diff.Compare(first, after)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, c := range changes.Columns {
		if c.Name == "observed_only" && c.Kind == diff.Added {
			found = true
		}
	}
	if !found {
		t.Fatalf("column drift missed: %+v", changes)
	}
	untrusted, _ := sqlserver.New(db, sqlserver.Options{Schemas: []string{"schema_guard_integration'; DROP TABLE schema_guard_integration.accounts; --"}})
	if _, err = untrusted.Inspect(ctx); err == nil {
		t.Fatal("unknown schema accepted")
	}
	if _, err = inspector.Inspect(ctx); err != nil {
		t.Fatalf("hostile schema input affected source: %v", err)
	}
}
func run(t *testing.T, ctx context.Context, db *sql.DB, name string) {
	t.Helper()
	b, err := fixtures.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, string(b)); err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
}
func find(t *testing.T, s schema.Snapshot, name string) schema.Table {
	t.Helper()
	for _, v := range s.Tables {
		if v.Name == name {
			return v
		}
	}
	t.Fatalf("missing table %s", name)
	return schema.Table{}
}
func findCol(t *testing.T, v schema.Table, name string) schema.Column {
	t.Helper()
	for _, c := range v.Columns {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("missing column %s", name)
	return schema.Column{}
}
