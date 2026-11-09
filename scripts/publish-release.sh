#!/usr/bin/env bash
set -euo pipefail

if [[ "${GITHUB_EVENT_NAME:-}" != "push" || "${GITHUB_REF:-}" != refs/tags/* ]]; then
  printf 'Release publication requires a tag push event.\n' >&2
  exit 1
fi

tag="${GITHUB_REF_NAME:?missing tag name}"
identifier='(0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)'
semver_regex="^v(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)(-${identifier}(\\.${identifier})*)?$"
if [[ ! "$tag" =~ $semver_regex ]]; then
  printf 'Invalid SemVer tag: %s (expected vMAJOR.MINOR.PATCH[-PRERELEASE]).\n' "$tag" >&2
  exit 1
fi

if ! git merge-base --is-ancestor HEAD refs/remotes/origin/main; then
  printf 'Tagged commit %s is not reachable from main.\n' "$(git rev-parse HEAD)" >&2
  exit 1
fi

if gh release view "$tag" --json tagName >/dev/null 2>&1; then
  printf 'GitHub Release for %s already exists; no changes made.\n' "$tag"
  exit 0
fi

args=(--verify-tag --generate-notes --title "$tag")
if [[ "$tag" == *-* ]]; then
  args+=(--prerelease --latest=false)
fi

gh release create "$tag" "${args[@]}"
