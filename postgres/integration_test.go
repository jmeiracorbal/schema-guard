//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"embed"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/jmeiracorbal/schema-guard/diff"
	"github.com/jmeiracorbal/schema-guard/postgres"
	"github.com/jmeiracorbal/schema-guard/schema"
	"github.com/jmeiracorbal/schema-guard/snapshot"
)

//go:embed testdata/*.sql
var fixtures embed.FS

func TestPostgreSQLIntegration(t *testing.T) {
	dsn := os.Getenv("SCHEMA_GUARD_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("SCHEMA_GUARD_POSTGRES_DSN must point to an isolated PostgreSQL test database")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	executeFixture(t, ctx, db, "drop.sql")
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		executeFixture(t, cleanupCtx, db, "drop.sql")
	})
	for _, name := range []string{
		"schema.sql", "accounts.sql", "orders.sql", "account_index.sql",
		"events.sql", "events_partition.sql",
	} {
		executeFixture(t, ctx, db, name)
	}

	inspector, err := postgres.New(db, postgres.Options{Schemas: []string{"schema_guard_integration"}})
	if err != nil {
		t.Fatal(err)
	}
	first, err := inspector.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Tables) != 4 {
		t.Fatalf("expected 4 tables, got %d", len(first.Tables))
	}
	accounts := findTable(t, first, "accounts")
	if len(accounts.Columns) != 5 {
		t.Fatalf("expected five account columns, got %d", len(accounts.Columns))
	}
	id := findColumn(t, accounts, "id")
	if id.Generated == nil || *id.Generated != "IDENTITY ALWAYS" {
		t.Fatalf("identity metadata not preserved: %+v", id)
	}
	generated := findColumn(t, accounts, "normalized_email")
	if generated.Generated == nil || !strings.HasPrefix(*generated.Generated, "STORED:") {
		t.Fatalf("generated expression not preserved: %+v", generated)
	}
	var expressionIndex *schema.Index
	for n := range accounts.Indexes {
		if accounts.Indexes[n].Name == "accounts_normalized_email_idx" {
			expressionIndex = &accounts.Indexes[n]
			break
		}
	}
	if expressionIndex == nil || len(expressionIndex.Columns) != 1 ||
		expressionIndex.Columns[0].Expression == "" || len(expressionIndex.IncludedColumns) != 1 ||
		expressionIndex.IncludedColumns[0] != "state" || expressionIndex.Predicate == nil ||
		expressionIndex.Definition == nil {
		t.Fatalf("expression/partial index metadata incomplete: %+v", expressionIndex)
	}
	orders := findTable(t, first, "orders")
	var foundFK bool
	for _, constraint := range orders.Constraints {
		if constraint.Type == schema.ForeignKey {
			foundFK = true
			if constraint.References == nil || constraint.References.Table != "accounts" ||
				constraint.References.OnDelete != "CASCADE" || constraint.References.OnUpdate != "RESTRICT" ||
				constraint.Deferrable == nil || !*constraint.Deferrable ||
				constraint.InitiallyDeferred == nil || !*constraint.InitiallyDeferred {
				t.Fatalf("foreign key metadata incomplete: %+v", constraint)
			}
		}
	}
	if !foundFK {
		t.Fatal("foreign key not detected")
	}
	partition := findTable(t, first, "events_2026")
	if partition.Kind != "table_partition" || partition.Parent == nil || partition.Parent.Name != "events" || partition.PartitionBound == nil {
		t.Fatalf("partition definition not captured: %+v", partition)
	}

	h1, err := snapshot.Hash(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := inspector.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := snapshot.Hash(second)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("snapshots must be identical, got %s and %s", h1, h2)
	}
	self, err := diff.Compare(first, first)
	if err != nil || !self.Empty() {
		t.Fatalf("self comparison must be empty: %+v, %v", self, err)
	}

	executeFixture(t, ctx, db, "add_column.sql")
	after, err := inspector.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := diff.Compare(first, after)
	if err != nil {
		t.Fatal(err)
	}
	var added bool
	for _, change := range changes.Columns {
		if change.Name == "observed_only" && change.Kind == diff.Added {
			added = true
		}
	}
	if !added {
		t.Fatalf("new column not detected: %+v", changes)
	}

	invalid, err := postgres.New(db, postgres.Options{Schemas: []string{"schema_guard_integration'; DROP SCHEMA schema_guard_integration; --"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := invalid.Inspect(ctx); err == nil {
		t.Fatal("unknown schema must fail, not return a partial/empty snapshot")
	}
	if _, err := inspector.Inspect(ctx); err != nil {
		t.Fatalf("requested schema changed after hostile name: %v", err)
	}
}

func executeFixture(t *testing.T, ctx context.Context, db *sql.DB, name string) {
	t.Helper()
	data, err := fixtures.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(data)); err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
}

func findTable(t *testing.T, s schema.Snapshot, name string) schema.Table {
	t.Helper()
	for _, table := range s.Tables {
		if table.Name == name {
			return table
		}
	}
	t.Fatalf("table %s missing", name)
	return schema.Table{}
}

func findColumn(t *testing.T, table schema.Table, name string) schema.Column {
	t.Helper()
	for _, column := range table.Columns {
		if column.Name == name {
			return column
		}
	}
	t.Fatalf("column %s missing", name)
	return schema.Column{}
}
