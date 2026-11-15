package postgres

import (
	"database/sql"
	"testing"

	"github.com/jmeiracorbal/schema-guard/schema"
)

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	if _, err := New(nil, Options{}); err == nil {
		t.Fatal("expected nil connection to be rejected")
	}
	db := new(sql.DB)
	for _, opts := range []Options{
		{Schemas: []string{""}},
		{Schemas: []string{"production", "production"}},
	} {
		if _, err := New(db, opts); err == nil {
			t.Fatalf("expected invalid config %+v to fail", opts)
		}
	}
	original := []string{"b", "a"}
	inspector, err := New(db, Options{Schemas: original})
	if err != nil {
		t.Fatal(err)
	}
	original[0] = "tampered"
	if inspector.schemas[0] != "a" || inspector.schemas[1] != "b" {
		t.Fatalf("schema options not copied and sorted: %v", inspector.schemas)
	}
	if got := inspector.Engine(); got != schema.PostgreSQL {
		t.Fatalf("expected PostgreSQL engine, got %s", got)
	}
}

func TestForeignKeyActions(t *testing.T) {
	for input, want := range map[string]string{
		"a": "NO ACTION", "r": "RESTRICT", "c": "CASCADE", "n": "SET NULL", "d": "SET DEFAULT",
	} {
		got, err := action(input)
		if err != nil || got != want {
			t.Fatalf("action(%q) = %q, %v, want %q", input, got, err, want)
		}
	}
	if _, err := action("unknown"); err == nil {
		t.Fatal("unknown action must fail explicitly")
	}
}
