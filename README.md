# schema-guard

`schema-guard` is a Go library for detecting structural drift between database schemas. It reads database metadata, produces deterministic snapshots and compares structural state without generating or executing migrations.

## Supported engines

| Engine | Catalog source | Integration CI |
| --- | --- | --- |
| PostgreSQL 14+ | `pg_catalog` | PostgreSQL 14 and 17 |
| MySQL 8.4 | `information_schema` | MySQL 8.4 |
| MariaDB 11.4 | `information_schema` | MariaDB 11.4 |
| SQLite 3.37+ | `sqlite_schema` and table-valued PRAGMA functions | SQLite integration tests |
| SQL Server 2022+ | `sys.*` catalog views | SQL Server 2022 |

MySQL and MariaDB are separate adapters with vendor-specific SQL, while the shared metadata reader and diff engine remain database-independent.

Snapshots capture tables, columns (including native types, defaults, generated attributes, collations and character sets), indexes (including key order, prefix lengths and engine-specific visibility), constraints (including referenced keys and foreign-key actions), and partition metadata. Catalog queries reside in embedded `.sql` files; schema selection is parameterized, not interpolated into identifiers. Snapshots from different engines are intentionally not compared.

## Go library

Supply a configured `*sql.DB` and a registered database driver:

```go
import (
    "context"
    "database/sql"

    "github.com/jmeiracorbal/schema-guard/mysql"
    "github.com/jmeiracorbal/schema-guard/snapshot"
)

func capture(ctx context.Context, db *sql.DB) (string, error) {
    inspector, err := mysql.New(db, mysql.Options{Schemas: []string{"app"}})
    if err != nil { return "", err }
    current, err := inspector.Inspect(ctx)
    if err != nil { return "", err }
    return snapshot.Hash(current)
}
```

Import `github.com/jmeiracorbal/schema-guard/mariadb`, `/postgres`, `/sqlite` or `/sqlserver` to inspect those engines. SQLite defaults to the `main` database and supports named attached databases through `Options.Schemas`. SQL Server selects all application schemas by default, or those listed in `Options.Schemas`. Use `diff.Compare(expected, actual)` to report additions, removals and modifications. The adapters use read-only transactions; they never execute DDL or migrations.

## Use case: CI and containerized workflows

A target use case is running `schema-guard` as an OCI image in CI/CD workflows of other projects: capture the target database schema, compare it with an expected snapshot stored alongside application code, publish a structured drift report and fail the job when unexpected differences are found. This can detect manual schema edits outside the migration process before a deployment.

**This is a planned deployment mode, not yet a published image or runnable command.** A CLI, exit-code contract and image build/publishing pipeline will be implemented separately; the current deliverable is the reusable Go library.

## Limitations

- Only structural metadata is compared. Rows and estimated statistics are not part of snapshots.
- System schemas and views are excluded. Explicitly selected unknown schemas cause an error; users must have sufficient metadata visibility for their target schemas.
- A transaction provides a consistent connection and read-only access, but MySQL and MariaDB metadata reads are **not guaranteed to form an atomic snapshot during concurrent DDL**. Avoid schema changes while inspecting.
- Functions, triggers, policies, standalone sequences, routines, events and views are outside the current model.
- SQLite does not enumerate named CHECK constraints or expression-index keys through PRAGMA. Its adapter retains the native `sqlite_schema.sql` definitions in canonical snapshots to detect these changes, but does not report CHECK constraints as individually named objects. SQLite virtual/shadow tables are explicitly unsupported.
- SQL Server 2022 inspection covers ordinary tables, columns (identity and computed), rowstore indexes, PK/UNIQUE/CHECK/FK constraints and referential actions. Unsupported specialized structures (e.g., temporal, graph, memory-optimized or ledger tables, columnstore indexes and unusual physical index options) fail explicitly rather than silently losing metadata.
- SQL Server inspection reads catalog views in a `REPEATABLE READ` transaction, issuing no DDL; use a database principal with VIEW DEFINITION permission. SQLite inspection uses a single read transaction; open the database in read-only mode for enforcement.
- The engine-specific SQL and expressions are preserved without attempting to infer semantic equivalence. Complex vendor-specific features not represented by catalog fields are not silently translated into another engine's semantics.

## Development

Requires Go 1.22 or later.

```sh
go test ./...
go vet ./...
```

GitHub Actions runs race-enabled Go tests and database-backed integration tests against PostgreSQL 14/17, MySQL 8.4, MariaDB 11.4, SQLite, and SQL Server 2022. No Bash helper scripts are required.
