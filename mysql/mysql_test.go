package mysql_test

import (
	"database/sql"
	"testing"

	"github.com/jmeiracorbal/schema-guard/mysql"
	"github.com/jmeiracorbal/schema-guard/schema"
)

func TestNew(t *testing.T) {
	if _, err := mysql.New(nil, mysql.Options{}); err == nil {
		t.Fatal("expected nil database error")
	}
	db := new(sql.DB)
	for _, tc := range []struct {
		name string
		opts mysql.Options
	}{
		{"empty", mysql.Options{Schemas: []string{""}}},
		{"duplicate", mysql.Options{Schemas: []string{"sample", "sample"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := mysql.New(db, tc.opts); err == nil {
				t.Fatal("expected invalid schema selection error")
			}
		})
	}
	inspect, err := mysql.New(db, mysql.Options{Schemas: []string{"sample"}})
	if err != nil {
		t.Fatal(err)
	}
	if inspect.Engine() != schema.MySQL {
		t.Fatalf("unexpected engine %s", inspect.Engine())
	}
}
