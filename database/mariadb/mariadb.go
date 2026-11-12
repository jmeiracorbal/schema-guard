// Package mariadb inspects MariaDB metadata through information_schema.
package mariadb

import (
	"database/sql"
	"embed"

	"github.com/jmeiracorbal/schema-guard/database/internal/mysqlcatalog"
	"github.com/jmeiracorbal/schema-guard/internal/domain"
	"github.com/jmeiracorbal/schema-guard/schema"
)

//go:embed queries/*.sql
var queries embed.FS

type Options = mysqlcatalog.Options

type Inspector struct{ *mysqlcatalog.Inspector }

var _ domain.Inspector = (*Inspector)(nil)

func New(db *sql.DB, opts Options) (*Inspector, error) {
	core, err := mysqlcatalog.New(db, schema.MariaDB, queries, opts)
	if err != nil {
		return nil, err
	}
	return &Inspector{Inspector: core}, nil
}
