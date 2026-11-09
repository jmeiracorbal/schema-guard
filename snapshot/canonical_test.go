package snapshot_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/jmeiracorbal/schema-guard/schema"
	"github.com/jmeiracorbal/schema-guard/snapshot"
)

func fixture() schema.Snapshot {
	return schema.Snapshot{
		Version: schema.FormatVersion, Engine: schema.PostgreSQL,
		Tables: []schema.Table{
			{Schema: "public", Name: "users", Columns: []schema.Column{
				{Name: "email", Position: 2, DataType: "text", Nullable: true},
				{Name: "id", Position: 1, DataType: "int8"},
			}, Indexes: []schema.Index{
				{Name: "z_idx", Method: "btree", Columns: []schema.IndexColumn{{Name: "email"}}, IncludedColumns: []string{"id", "email"}},
				{Name: "a_idx", Method: "btree", Columns: []schema.IndexColumn{{Name: "id"}}},
			}, Constraints: []schema.Constraint{
				{Name: "z_unique", Type: schema.UniqueKey, Columns: []string{"email"}},
				{Name: "pk", Type: schema.PrimaryKey, Columns: []string{"id"}},
			}},
			{Schema: "other", Name: "events"},
		},
	}
}

func TestCanonicalIndependentOfQueryOrder(t *testing.T) {
	first := fixture()
	second := fixture()
	second.Tables[0], second.Tables[1] = second.Tables[1], second.Tables[0]
	u := &second.Tables[1]
	u.Columns[0], u.Columns[1] = u.Columns[1], u.Columns[0]
	u.Indexes[0], u.Indexes[1] = u.Indexes[1], u.Indexes[0]
	u.Constraints[0], u.Constraints[1] = u.Constraints[1], u.Constraints[0]
	u.Indexes[1].IncludedColumns[0], u.Indexes[1].IncludedColumns[1] = u.Indexes[1].IncludedColumns[1], u.Indexes[1].IncludedColumns[0]

	firstJSON, err := snapshot.CanonicalJSON(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := snapshot.CanonicalJSON(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("different canonical JSON for equivalent schemas:\n%s\n%s", firstJSON, secondJSON)
	}
	firstHash, err := snapshot.Hash(first)
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := snapshot.Hash(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstHash != secondHash || len(firstHash) != 64 {
		t.Fatalf("hash mismatch %q vs %q", firstHash, secondHash)
	}
	if first.Tables[0].Columns[0].Name != "email" {
		t.Fatal("canonicalization mutated the original snapshot")
	}
}

func TestPreservesMeaningfulKeyOrdering(t *testing.T) {
	first := fixture()
	first.Tables[0].Indexes[0].Columns = []schema.IndexColumn{{Name: "id"}, {Name: "email"}}
	second := fixture()
	second.Tables[0].Indexes[0].Columns = []schema.IndexColumn{{Name: "email"}, {Name: "id"}}
	firstHash, err := snapshot.Hash(first)
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := snapshot.Hash(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstHash == secondHash {
		t.Fatal("index key order must affect the hash")
	}
}

func TestRoundTripAndEmptyCollectionNormalization(t *testing.T) {
	input := schema.Snapshot{Version: schema.FormatVersion, Engine: schema.SQLite, Tables: nil}
	data, err := snapshot.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := snapshot.Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := snapshot.Canonical(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parsed, expected) {
		t.Fatalf("round trip changed snapshot: %#v", parsed)
	}
	if !strings.Contains(string(data), `"tables": []`) {
		t.Fatalf("expected empty array, got %s", data)
	}
}

func TestRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	for _, raw := range []string{
		`{"version":1,"engine":"sqlite","tables":[],"unexpected":true}`,
		`{"version":1,"engine":"sqlite","tables":[]} {"version":1}`,
		`{"version":999,"engine":"sqlite","tables":[]}`,
	} {
		if _, err := snapshot.Unmarshal([]byte(raw)); err == nil {
			t.Errorf("accepted invalid snapshot %s", raw)
		}
	}
}
