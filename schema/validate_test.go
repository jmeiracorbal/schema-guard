package schema_test

import (
	"strings"
	"testing"

	"github.com/jmeiracorbal/schema-guard/schema"
)

func TestValidationRejectsAmbiguousMetadata(t *testing.T) {
	cases := []struct {
		name string
		snap schema.Snapshot
		want string
	}{
		{"unknown engine", schema.Snapshot{Version: 1, Engine: "oracle"}, "unsupported database engine"},
		{"missing type", schema.Snapshot{Version: 1, Engine: schema.SQLite, Tables: []schema.Table{{Name: "a", Columns: []schema.Column{{Name: "id", Position: 1}}}}}, "data type"},
		{"duplicate table", schema.Snapshot{Version: 1, Engine: schema.MySQL, Tables: []schema.Table{{Name: "a"}, {Name: "a"}}}, "duplicate table"},
		{"duplicate column ordinal", schema.Snapshot{Version: 1, Engine: schema.SQLite, Tables: []schema.Table{{Name: "a", Columns: []schema.Column{{Name: "id", Position: 1, DataType: "int"}, {Name: "x", Position: 1, DataType: "text"}}}}}, "duplicate column position"},
		{"invalid index key", schema.Snapshot{Version: 1, Engine: schema.SQLServer, Tables: []schema.Table{{Name: "a", Indexes: []schema.Index{{Name: "idx", Columns: []schema.IndexColumn{{Name: "x", Expression: "lower(x)"}}}}}}}, "exactly one"},
		{"invalid foreign key", schema.Snapshot{Version: 1, Engine: schema.PostgreSQL, Tables: []schema.Table{{Name: "a", Constraints: []schema.Constraint{{Name: "fk", Type: schema.ForeignKey, Columns: []string{"id"}}}}}}, "foreign key"},
		{"unknown constraint", schema.Snapshot{Version: 1, Engine: schema.PostgreSQL, Tables: []schema.Table{{Name: "a", Constraints: []schema.Constraint{{Name: "unknown", Type: "mystery"}}}}}, "unsupported kind"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.snap.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted error containing %q, got %v", tc.want, err)
			}
		})
	}
}
