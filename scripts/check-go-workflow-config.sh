#!/usr/bin/env bash
set -euo pipefail

workflows=(
  ".github/workflows/go-unit-tests.yml"
  ".github/workflows/postgres-tests.yml"
  ".github/workflows/ingest-e2e.yml"
  ".github/workflows/release.yml"
)

approved_local_setup_go="./.github/actions/setup-go"
approved_local_setup_go_action=".github/actions/setup-go/action.yml"

if ! grep -Eq "uses:[[:space:]]+actions/setup-go@[0-9a-f]{40}" "$approved_local_setup_go_action"; then
  echo "$approved_local_setup_go_action must pin actions/setup-go to a 40-character SHA" >&2
  exit 1
fi

if ! grep -Eq "default:[[:space:]]*'\\.go-version'|default:[[:space:]]*\\.go-version" "$approved_local_setup_go_action"; then
  echo "$approved_local_setup_go_action must default go-version-file to .go-version" >&2
  exit 1
fi

for workflow in "${workflows[@]}"; do
  uses_direct_sha_setup_go=false
  uses_approved_local_setup_go=false

  if grep -Eq "uses:[[:space:]]+actions/setup-go@[0-9a-f]{40}" "$workflow"; then
    uses_direct_sha_setup_go=true
  fi

  if grep -Eq "uses:[[:space:]]+\./\.github/actions/setup-go" "$workflow"; then
    uses_approved_local_setup_go=true
  fi

  if [[ "$uses_direct_sha_setup_go" == false && "$uses_approved_local_setup_go" == false ]]; then
    echo "$workflow must use actions/setup-go pinned to a 40-character SHA or $approved_local_setup_go" >&2
    exit 1
  fi

  if ! grep -Eq "go-version-file:[[:space:]]*'\.go-version'|go-version-file:[[:space:]]*\.go-version" "$workflow" && [[ "$uses_approved_local_setup_go" == false ]]; then
    echo "$workflow must set go-version-file: .go-version" >&2
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
