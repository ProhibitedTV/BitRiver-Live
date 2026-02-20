#!/usr/bin/env bash
set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

usage() {
  cat <<'USAGE'
Usage: ./scripts/test-integration.sh [--ingest-e2e]

Runs integration entrypoints:
  - ./scripts/test-postgres.sh (requires docker or BITRIVER_TEST_POSTGRES_DSN)
  - ./scripts/test-quickstart.sh (requires docker)
  - ./scripts/test-ingest-e2e.sh (only when explicitly enabled)

Ingest e2e controls:
  --ingest-e2e                    Run ingest e2e in this invocation.
  BITRIVER_TEST_ALL_INGEST_E2E=1  Run ingest e2e by environment override.
USAGE
}

run_ingest_e2e=false

while (($# > 0)); do
  case "$1" in
    --ingest-e2e)
      run_ingest_e2e=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
  shift
done

if [[ "${BITRIVER_TEST_ALL_INGEST_E2E:-}" == "1" ]]; then
  run_ingest_e2e=true
fi

run_step() {
  local label="$1"
  shift

  echo
  echo "==> ${label}"
  "$@"
}

skip_step() {
  local label="$1"
  local reason="$2"

  echo
  echo "==> ${label}"
  echo "Skipping: ${reason}"
}

if command -v docker >/dev/null 2>&1; then
  run_step "Postgres integration tests" ./scripts/test-postgres.sh
  run_step "Quickstart smoke" ./scripts/test-quickstart.sh
else
  skip_step "Postgres integration tests" "docker is not installed or not on PATH."
  skip_step "Quickstart smoke" "docker is not installed or not on PATH."
fi

if [[ "$run_ingest_e2e" == true ]]; then
  run_step "Ingest end-to-end tests" ./scripts/test-ingest-e2e.sh
else
  skip_step "Ingest end-to-end tests" "disabled by default (use --ingest-e2e or BITRIVER_TEST_ALL_INGEST_E2E=1)."
fi
