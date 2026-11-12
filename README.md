# schema-guard

[![CI](https://github.com/jmeiracorbal/schema-guard/actions/workflows/ci.yml/badge.svg)](https://github.com/jmeiracorbal/schema-guard/actions/workflows/ci.yml)
[![Publish](https://github.com/jmeiracorbal/schema-guard/actions/workflows/release.yml/badge.svg?branch=main)](https://github.com/jmeiracorbal/schema-guard/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`schema-guard` is a read-only database schema drift detector written in Go. It captures deterministic schema snapshots and compares a live database against an expected snapshot committed to a repository. It does **not** generate or run database migrations.

## Supported databases

| Database | Metadata source | Tested version |
| --- | --- | --- |
| PostgreSQL | `pg_catalog` | 14, 17 |
| MySQL | `information_schema` | 8.4 |
| MariaDB | `information_schema` | 11.4 |
| SQLite | `sqlite_schema`, `PRAGMA` | 3.37+ |
| SQL Server | `sys.*` | 2022 |

All five adapters produce typed structural snapshots. Engines are compared only with snapshots from the **same** database engine. Queries are held in separate `.sql` files, with no regex DDL parser or cross-engine translation.

## Run in CI using the OCI image

Every successful push to `main` publishes a new OCI image to `ghcr.io/jmeiracorbal/schema-guard:latest` and an immutable `sha-<40-character commit SHA>` tag for `linux/amd64` and `linux/arm64`. Use `:latest` to track the current validated `main` build, or the SHA tag for reproducible deployments. SemVer tags such as `v1.0.0` additionally publish `:v1.0.0` and a GitHub Release. SemVer releases are not automatically incremented on each commit. The repository/package access settings determine who can pull the image; private packages require registry authentication.

Commit a canonical snapshot to your application repository and compare your running schema against it:

```sh
export SCHEMA_GUARD_DSN='postgres://user:password@database:5432/app?sslmode=verify-full'

docker run --rm --network host \
  -e SCHEMA_GUARD_DSN \
  -v "$PWD/schema:/workspace/schema:ro" \
  ghcr.io/jmeiracorbal/schema-guard:latest \
  check --engine postgresql --schema public \
  --expected /workspace/schema/expected.json --format json
```

In GitHub Actions, load `SCHEMA_GUARD_DSN` from a repository secret, expose the expected file with a read-only mount, and execute this command in a job. The image has no shell, runs as a non-root user, and includes certificates for TLS connections. Use a database account with only the metadata permissions the engine requires. For a remote database, omit `--network host` when normal container networking can reach it. On Linux runners, `--network host` permits connections to services bound on the runner's loopback interface.

### Exit codes

| Exit | Meaning |
| --- | --- |
| `0` | Inspection succeeded; no differences (or snapshot captured) |
| `1` | Invalid arguments, connection failure, invalid snapshot or other error |
| `2` | Structural drift found |

A workflow step normally fails on exit `2`. With `--format json`, `check` emits stable keys: `drift`, `change_count`, `expected_sha256`, `actual_sha256`, and typed `changes` (before/after values).

## CLI usage

The command-line tool is available inside the OCI image, or directly from source:

```sh
go run ./cmd/schema-guard version
```

Capture the approved state. Database credentials are read **only** from `SCHEMA_GUARD_DSN`, not from command-line arguments:

```sh
schema-guard snapshot --engine postgresql --schema public --output schema/expected.json
schema-guard check --engine postgresql --schema public --expected schema/expected.json --format text
```

Without `--output`, `snapshot` prints canonical JSON to standard output. `--schema` is repeatable; defaults depend on the engine (SQLite uses `main`). `check` accepts `--format text` (default) or `--format json`. Both commands accept `--timeout` (default `2m`). File snapshots are written atomically with restrictive permissions. For SQLite, use a read-only URI such as `file:/workspace/data/app.sqlite?mode=ro`; mount the database directory read-only.

For other engines, supply their native driver DSNs through `SCHEMA_GUARD_DSN`: MySQL/MariaDB (`go-sql-driver/mysql` format), SQLite (`modernc.org/sqlite` URI), SQL Server (`go-mssqldb` URL), or PostgreSQL (`lib/pq` URL). Never commit database credentials or embed them in snapshots.

## Go library

The adapters can also be used directly with an existing `*sql.DB` and a registered database driver:

```go
import (
    "context"
    "database/sql"

    "github.com/jmeiracorbal/schema-guard/diff"
    "github.com/jmeiracorbal/schema-guard/postgres"
    "github.com/jmeiracorbal/schema-guard/schema"
)

func compare(ctx context.Context, db *sql.DB, expected schema.Snapshot) (diff.Result, error) {
    inspector, err := postgres.New(db, postgres.Options{Schemas: []string{"public"}})
    if err != nil {
        return diff.Result{}, err
    }
    actual, err := inspector.Inspect(ctx)
    if err != nil {
        return diff.Result{}, err
    }
    return diff.Compare(expected, actual)
}
```

Packages: `postgres`, `mysql`, `mariadb`, `sqlite`, `sqlserver`, `schema`, `snapshot`, and `diff`.

## Limitations

- Only structural metadata is inspected; no row data, procedures, views, triggers, events, policies, or standalone sequences. Unsupported specialized structures fail explicitly where detected.
- Schema comparison is not a migration planner. Snapshot comparison does not establish semantic equivalence of different native SQL expressions.
- Concurrent DDL in MySQL and MariaDB can prevent a perfectly atomic metadata capture; avoid schema modifications during inspection.
- SQLite retains native SQL definitions to detect changes in CHECK constraints and expression indexes where PRAGMA does not expose individual definitions.
- SQL Server inspection requires sufficient catalog visibility (typically `VIEW DEFINITION`). Database roles and access rights remain the caller's responsibility.
- Pull requests only validate the code and container; they cannot publish images. A successful push to `main` publishes `:latest` and `:sha-<commit>`; SemVer tag pushes publish versioned images and a GitHub Release. Publishing is blocked when CI fails.

## Development

Requires Go 1.22 or later.

```sh
go test -race -count=1 ./...
go vet ./...
go build ./cmd/schema-guard
```

Pull requests trigger one CI workflow covering formatting, module integrity, `go vet`, Staticcheck, race-enabled tests, an OCI smoke test, and the reusable **Database engines** workflow. The database engines (PostgreSQL 14/17, MySQL 8.4, MariaDB 11.4, SQLite and SQL Server 2022) run in parallel. The CI workflow fails if **any** engine fails, and GHCR publishing can only start after the entire CI workflow passes. On each push to `main`, the publishing workflow runs the same checks before pushing a new `:latest` and immutable SHA-tagged image to GHCR. SemVer tags on commits reachable from `main` publish a version-tagged image and then create a GitHub Release; tagged releases do not overwrite the `:latest` build from `main`.
