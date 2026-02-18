#!/usr/bin/env bash
set -euo pipefail

workflows=(
  ".github/workflows/go-unit-tests.yml"
  ".github/workflows/postgres-tests.yml"
  ".github/workflows/ingest-e2e.yml"
  ".github/workflows/release.yml"
)

for workflow in "${workflows[@]}"; do
  if ! rg -q "uses:\\s+actions/setup-go@v5" "$workflow"; then
    echo "$workflow must use actions/setup-go@v5" >&2
    exit 1
  fi

  if ! rg -q "go-version-file:\\s*'go\\.mod'|go-version-file:\\s*go\\.mod" "$workflow"; then
    echo "$workflow must set go-version-file: go.mod" >&2
    exit 1
  fi

  if ! rg -q "GOTOOLCHAIN:\\s*local" "$workflow"; then
    echo "$workflow must define GOTOOLCHAIN=local" >&2
    exit 1
  fi

  if ! rg -q "GOPROXY:\\s*off" "$workflow"; then
    echo "$workflow must define GOPROXY=off" >&2
    exit 1
  fi

  if ! rg -q "GOSUMDB:\\s*off" "$workflow"; then
    echo "$workflow must define GOSUMDB=off" >&2
    exit 1
  fi
done

echo "Go workflow configuration checks passed."
