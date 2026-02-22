#!/usr/bin/env bash
set -euo pipefail

workflows=(
  ".github/workflows/go-unit-tests.yml"
  ".github/workflows/postgres-tests.yml"
  ".github/workflows/ingest-e2e.yml"
  ".github/workflows/release.yml"
)

for workflow in "${workflows[@]}"; do
  if ! grep -Eq "uses:[[:space:]]+actions/setup-go@v5" "$workflow"; then
    echo "$workflow must use actions/setup-go@v5" >&2
    exit 1
  fi

  if ! grep -Eq "go-version-file:[[:space:]]*'go\\.mod'|go-version-file:[[:space:]]*go\\.mod" "$workflow"; then
    echo "$workflow must set go-version-file: go.mod" >&2
    exit 1
  fi

  if ! grep -Eq "GOTOOLCHAIN:[[:space:]]*local" "$workflow"; then
    echo "$workflow must define GOTOOLCHAIN=local" >&2
    exit 1
  fi

  if ! grep -Eq "GOPROXY:[[:space:]]*off" "$workflow"; then
    echo "$workflow must define GOPROXY=off" >&2
    exit 1
  fi

  if ! grep -Eq "GOSUMDB:[[:space:]]*off" "$workflow"; then
    echo "$workflow must define GOSUMDB=off" >&2
    exit 1
  fi
done

echo "Go workflow configuration checks passed."
