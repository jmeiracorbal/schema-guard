package snapshot_test

import (
	"testing"

	"github.com/jmeiracorbal/schema-guard/diff"
	"github.com/jmeiracorbal/schema-guard/schema"
	"github.com/jmeiracorbal/schema-guard/snapshot"
)

func TestMySQLMetadataCanonicalization(t *testing.T) {
	base := schema.Snapshot{Version: schema.FormatVersion, Engine: schema.MySQL, Tables: []schema.Table{{
		Schema: "app", Name: "events", Kind: "table", StorageEngine: "InnoDB",
		Columns:    []schema.Column{{Name: "id", Position: 1, DataType: "int", Nullable: false}},
		Indexes:    []schema.Index{{Name: "PRIMARY", Method: "BTREE", Unique: true, Columns: []schema.IndexColumn{{Name: "id"}}}},
		Partitions: []schema.Partition{{Name: "pmax", Position: 2, Method: "RANGE", Expression: "id"}, {Name: "p0", Position: 1, Method: "RANGE", Expression: "id"}},
	}}}
	h1, err := snapshot.Hash(base)
	if err != nil {
		t.Fatal(err)
	}
	reordered := base
	reordered.Tables = append([]schema.Table(nil), base.Tables...)
	reordered.Tables[0].Partitions = []schema.Partition{base.Tables[0].Partitions[1], base.Tables[0].Partitions[0]}
	h2, err := snapshot.Hash(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("unordered catalog partitions changed hash: %s vs %s", h1, h2)
	}
	result, err := diff.Compare(base, reordered)
	if err != nil || !result.Empty() {
		t.Fatalf("unexpected drift %+v, %v", result, err)
	}
	reordered.Tables[0].StorageEngine = "MyISAM"
	h3, err := snapshot.Hash(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if h3 == h1 {
		t.Fatal("storage engine change must affect hash")
	}
	result, err = diff.Compare(base, reordered)
	if err != nil {
		t.Fatal(err)
	}
	if result.Empty() {
		t.Fatal("storage engine change must be reported")
	}
}
