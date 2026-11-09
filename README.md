# schema-guard

`schema-guard` is a Go library for detecting structural drift between database schemas. It compares deterministic snapshots of tables, columns, indexes, and constraints without generating or executing migrations.

## Status

Foundation implemented: typed schema model, snapshot validation and canonicalization, SHA-256 hashing, and deterministic structural differences. Database introspection adapters and CLI commands are not yet implemented.

## Planned database engines

PostgreSQL, MySQL, MariaDB, SQLite, and SQL Server. Each engine will be inspected through its metadata catalogs, rather than by parsing DDL with regular expressions.

## Usage

The following example compares snapshots assembled in Go. Introspection from a live database is not available yet.

```go
expected := schema.Snapshot{Version: schema.FormatVersion, Engine: schema.PostgreSQL, Tables: []schema.Table{}}
actual := schema.Snapshot{Version: schema.FormatVersion, Engine: schema.PostgreSQL, Tables: []schema.Table{}}
result, err := diff.Compare(expected, actual)
if err != nil {
    // Handle invalid or incompatible snapshots.
}
_ = result
```

Imports: `github.com/jmeiracorbal/schema-guard/schema` and `github.com/jmeiracorbal/schema-guard/diff`.

## Development

Requires Go 1.22 or later.

```sh
go test ./...
```

Canonical JSON is independent of catalog query order. Column positions and index/constraint key order are preserved because their order is meaningful. Snapshots from different database engines are not directly comparable. Native SQL expressions are compared as captured; no semantic equivalence between differently formatted expressions is implied.
