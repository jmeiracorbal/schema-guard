package mariadb_test

import (
	"database/sql"
	"testing"

	"github.com/jmeiracorbal/schema-guard/mariadb"
	"github.com/jmeiracorbal/schema-guard/schema"
)

func TestNew(t *testing.T) {
	if _, err := mariadb.New(nil, mariadb.Options{}); err == nil {
		t.Fatal("expected nil database error")
	}
	db := new(sql.DB)
	for _, tc := range []struct {
		name string
		opts mariadb.Options
	}{
		{"empty", mariadb.Options{Schemas: []string{""}}},
		{"duplicate", mariadb.Options{Schemas: []string{"sample", "sample"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := mariadb.New(db, tc.opts); err == nil {
				t.Fatal("expected invalid schema selection error")
			}
		})
	}
	inspect, err := mariadb.New(db, mariadb.Options{Schemas: []string{"sample"}})
	if err != nil {
		t.Fatal(err)
	}
	if inspect.Engine() != schema.MariaDB {
		t.Fatalf("unexpected engine %s", inspect.Engine())
	}
}
