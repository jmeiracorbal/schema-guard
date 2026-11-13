//go:build integration

package sqlite_test

import (
	"context"
	"database/sql"
	"embed"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/jmeiracorbal/schema-guard/database/sqlite"
	"github.com/jmeiracorbal/schema-guard/diff"
	"github.com/jmeiracorbal/schema-guard/schema"
	"github.com/jmeiracorbal/schema-guard/snapshot"
)

//go:embed testdata/*.sql
var fixtures embed.FS

func TestSQLiteIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "integration.sqlite")
	writer, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	writer.SetMaxOpenConns(1)
	fixture(t, writer, "schema.sql")
	// The inspector uses a database opened read-only, separate from fixtures.
	reader, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	reader.SetMaxOpenConns(1)
	inspector, err := sqlite.New(reader, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	first, err := inspector.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Tables) != 4 {
		t.Fatalf("expected four tables, got %d", len(first.Tables))
	}
	accounts := find(t, first, "accounts")
	if accounts.Definition == nil || !strings.Contains(*accounts.Definition, "CHECK") {
		t.Fatal("CHECK constraint not preserved in native DDL")
	}
	if findColumn(t, accounts, "freeform").DataType != "" {
		t.Fatal("SQLite affinity-free column not preserved")
	}
	if got := findColumn(t, accounts, "lower_email").Generated; got == nil || *got != "STORED" {
		t.Fatalf("generated column missing: %v", got)
	}
	orders := find(t, first, "orders")
	if len(orders.Constraints) < 2 {
		t.Fatalf("foreign keys not detected: %+v", orders.Constraints)
	}
	var expression bool
	for _, idx := range orders.Indexes {
		if idx.Name == "idx_orders_lower_code" {
			expression = len(idx.Columns) == 1 && idx.Columns[0].Expression != "" && idx.Definition != nil && strings.Contains(*idx.Definition, "lower(code)")
		}
	}
	if !expression {
		t.Fatalf("expression index missing: %+v", orders.Indexes)
	}
	if got := find(t, first, "settings").Kind; got != "without_rowid" {
		t.Fatalf("missing WITHOUT ROWID metadata: %s", got)
	}
	if got := find(t, first, "strict_events").Persistence; got != "strict" {
		t.Fatalf("missing STRICT metadata: %s", got)
	}
	initialHash, err := snapshot.Hash(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := inspector.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	h, err := snapshot.Hash(second)
	if err != nil {
		t.Fatal(err)
	}
	if h != initialHash {
		t.Fatal("equivalent captures have different hashes")
	}
	equal, err := diff.Compare(first, first)
	if err != nil || !equal.Empty() {
		t.Fatalf("self-diff not empty: %+v %v", equal, err)
	}
	fixture(t, writer, "add_column.sql")
	updated, err := inspector.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := diff.Compare(first, updated)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, change := range changes.Columns {
		if change.Kind == diff.Added && change.Name == "observed_only" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing added-column drift: %+v", changes)
	}
	unsafe, err := sqlite.New(reader, sqlite.Options{Schemas: []string{`main"; DROP TABLE accounts; --`}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = unsafe.Inspect(ctx); err == nil {
		t.Fatal("unattached schema must fail")
	}
	if _, err = inspector.Inspect(ctx); err != nil {
		t.Fatalf("untrusted schema name affected the database: %v", err)
	}
}

func fixture(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	data, err := fixtures.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(data)); err != nil {
		t.Fatal(err)
	}
}
func find(t *testing.T, s schema.Snapshot, name string) schema.Table {
	t.Helper()
	for _, table := range s.Tables {
		if table.Name == name {
			return table
		}
	}
	t.Fatalf("missing table %s", name)
	return schema.Table{}
}
func findColumn(t *testing.T, table schema.Table, name string) schema.Column {
	t.Helper()
	for _, col := range table.Columns {
		if col.Name == name {
			return col
		}
	}
	t.Fatalf("missing column %s", name)
	return schema.Column{}
}

func TestSQLiteUnknownSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, schemas := range [][]string{{"main", "main"}, {""}} {
		if _, err = sqlite.New(db, sqlite.Options{Schemas: schemas}); err == nil {
			t.Fatalf("invalid selection %v accepted", schemas)
		}
	}
	inspector, _ := sqlite.New(db, sqlite.Options{Schemas: []string{"missing"}})
	if _, err = inspector.Inspect(context.Background()); err == nil {
		t.Fatal("unknown database accepted")
	}
}

func TestSQLiteFileScope(t *testing.T) {
	// Exercise native driver persistence: the inspector must not modify an opened file.
	path := filepath.Join(t.TempDir(), "data.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	read, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	inspector, _ := sqlite.New(read, sqlite.Options{})
	snap, err := inspector.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Tables) != 1 {
		t.Fatalf("unexpected schema: %+v", snap)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
