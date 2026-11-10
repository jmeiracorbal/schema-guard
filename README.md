# schema-guard

`schema-guard` is a Go library for detecting structural drift between database schemas. It inspects database metadata, produces deterministic snapshots, and compares their structural state without generating or executing migrations.

## Database engines

- **PostgreSQL 14+**: catalog-based, read-only inspection of ordinary and partitioned tables, columns, indexes, and constraints.
- **Planned**: MySQL, MariaDB, SQLite, and SQL Server. The typed domain and diff engine are independent of database drivers.

PostgreSQL inspection preserves native column types, defaults, generated and identity attributes, collations, table partitions, index expressions, predicates, included columns, index definitions, and native constraint definitions. Foreign keys retain referenced columns and actions. All catalog queries live in separate `.sql` files, with parameterized schema selection.

## Use as a Go library

Pass an already configured `*sql.DB` with a registered PostgreSQL driver:

```go
import (
    "context"
    "database/sql"

    _ "github.com/lib/pq"

    "github.com/jmeiracorbal/schema-guard/postgres"
    "github.com/jmeiracorbal/schema-guard/snapshot"
)

func inspect(ctx context.Context, db *sql.DB) (string, error) {
    inspector, err := postgres.New(db, postgres.Options{Schemas: []string{"public"}})
    if err != nil {
        return "", err
    }
    current, err := inspector.Inspect(ctx)
    if err != nil {
        return "", err
    }
    return snapshot.Hash(current)
}
```

Use `diff.Compare(expected, actual)` to detect added, removed, or modified tables, columns, indexes, and constraints. Snapshots from different engines cannot be compared. The PostgreSQL adapter uses a read-only `REPEATABLE READ` transaction for a consistent capture.

## Scope and limitations

- Inspection is read-only. No DDL or migration operations are performed.
- System schemas are excluded. Set `Options.Schemas` to inspect exact application schemas; missing schemas fail explicitly.
- Views, materialized views, foreign tables, functions, triggers, policies, extensions, standalone sequences, and domain/type definitions are outside the current model. Standalone sequence drift remains a separate concern.
- Multiple inheritance parents and invalid or in-progress indexes cause an explicit error. PostgreSQL-native index and constraint definitions preserve features that lack a common cross-engine field; they are not parsed for semantic equivalence.
- The adapter uses PostgreSQL's deparsed expressions. Textually different but semantically equivalent expressions are not considered equal.
- Only PostgreSQL 14 and 17 are currently covered by container-backed integration tests.

## Development

Requires Go 1.22 or later.

```sh
go test ./...
go vet ./...
```

The `CI` workflow also starts PostgreSQL 14 and 17 service containers and runs the `integration`-tagged tests against each database.
