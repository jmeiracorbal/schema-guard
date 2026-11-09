package diff_test

import (
	"reflect"
	"testing"

	"github.com/jmeiracorbal/schema-guard/diff"
	"github.com/jmeiracorbal/schema-guard/schema"
)

func TestIdenticalSnapshotHasNoDrift(t *testing.T) {
	input := schema.Snapshot{Version: 1, Engine: schema.SQLite, Tables: []schema.Table{
		{Name: "users", Columns: []schema.Column{{Name: "id", Position: 1, DataType: "INTEGER"}}},
	}}
	result, err := diff.Compare(input, input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Empty() || result.Count() != 0 {
		t.Fatalf("self-comparison unexpectedly has %d changes", result.Count())
	}
}

func TestDetectsTypedChanges(t *testing.T) {
	expected := schema.Snapshot{Version: 1, Engine: schema.PostgreSQL, Tables: []schema.Table{
		{Schema: "public", Name: "users", Columns: []schema.Column{
			{Name: "id", Position: 1, DataType: "int4"},
			{Name: "email", Position: 2, DataType: "varchar(100)"},
		}, Indexes: []schema.Index{{Name: "idx_email", Columns: []schema.IndexColumn{{Name: "email"}}}}, Constraints: []schema.Constraint{{Name: "pk", Type: schema.PrimaryKey, Columns: []string{"id"}}}},
		{Schema: "public", Name: "obsolete"},
	}}
	actual := schema.Snapshot{Version: 1, Engine: schema.PostgreSQL, Tables: []schema.Table{
		{Schema: "public", Name: "new_table"},
		{Schema: "public", Name: "users", Columns: []schema.Column{
			{Name: "id", Position: 1, DataType: "int8"},
			{Name: "email", Position: 2, DataType: "varchar(100)"},
			{Name: "created", Position: 3, DataType: "timestamp"},
		}, Indexes: []schema.Index{{Name: "idx_created", Columns: []schema.IndexColumn{{Name: "created"}}}}, Constraints: []schema.Constraint{{Name: "pk", Type: schema.PrimaryKey, Columns: []string{"id", "email"}}}},
	}}
	result, err := diff.Compare(expected, actual)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tables) != 2 || len(result.Columns) != 2 || len(result.Indexes) != 2 || len(result.Constraints) != 1 {
		t.Fatalf("wrong change totals: %+v", result)
	}
	if result.Tables[0].Kind != diff.Added || result.Tables[1].Kind != diff.Removed {
		t.Fatalf("unexpected table changes: %+v", result.Tables)
	}
	if result.Columns[0].Kind != diff.Added || result.Columns[1].Kind != diff.Modified || result.Columns[1].Before.DataType != "int4" || result.Columns[1].After.DataType != "int8" {
		t.Fatalf("typed columns not preserved: %+v", result.Columns)
	}
	if result.Constraints[0].Kind != diff.Modified {
		t.Fatalf("constraint change missing: %+v", result.Constraints)
	}
}

func TestQueryOrderDoesNotProduceDrift(t *testing.T) {
	col1 := schema.Column{Name: "id", DataType: "int", Position: 1}
	col2 := schema.Column{Name: "name", DataType: "text", Position: 2}
	a := schema.Snapshot{Version: 1, Engine: schema.MySQL, Tables: []schema.Table{{Name: "users", Columns: []schema.Column{col1, col2}}, {Name: "events"}}}
	b := schema.Snapshot{Version: 1, Engine: schema.MySQL, Tables: []schema.Table{{Name: "events"}, {Name: "users", Columns: []schema.Column{col2, col1}}}}
	changes, err := diff.Compare(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if !changes.Empty() {
		t.Fatalf("query order created drift: %+v", changes)
	}
	again, err := diff.Compare(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(changes, again) {
		t.Fatal("diff result is non-deterministic")
	}
}

func TestRejectsCrossEngineComparison(t *testing.T) {
	a := schema.Snapshot{Version: 1, Engine: schema.PostgreSQL}
	b := schema.Snapshot{Version: 1, Engine: schema.MySQL}
	if _, err := diff.Compare(a, b); err == nil {
		t.Fatal("cross-engine comparison should fail")
	}
}
