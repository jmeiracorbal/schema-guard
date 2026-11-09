#!/usr/bin/env bash
set -euo pipefail

unformatted="$(gofmt -l .)"
if [[ -n "$unformatted" ]]; then
  printf 'Go files require formatting:\n%s\n' "$unformatted" >&2
  exit 1
fi

go vet ./...
go test -race -count=1 ./...
go build ./...
