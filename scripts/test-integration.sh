#!/usr/bin/env bash
set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

usage() {
  cat <<'USAGE'
Usage: ./scripts/test-integration.sh [--production-golden-path]

Runs integration entrypoints:
  - ./scripts/test-postgres.sh (requires docker or BITRIVER_TEST_POSTGRES_DSN)
  - ./scripts/test-quickstart.sh (requires docker)
  - ./scripts/test-production-golden-path.sh via canonical quickstart (opt-in)

Production golden-path controls:
  --production-golden-path                    Run the real product gate.
  BITRIVER_TEST_ALL_PRODUCTION_GOLDEN_PATH=1  Run it by environment override.

Compatibility:
  --ingest-e2e and BITRIVER_TEST_ALL_INGEST_E2E=1 remain accepted aliases.
USAGE
}

run_production_golden_path=false

while (($# > 0)); do
  case "$1" in
    --production-golden-path|--ingest-e2e)
      run_production_golden_path=true
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

if [[ "${BITRIVER_TEST_ALL_PRODUCTION_GOLDEN_PATH:-}" == "1" || "${BITRIVER_TEST_ALL_INGEST_E2E:-}" == "1" ]]; then
  run_production_golden_path=true
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
  if [[ "$run_production_golden_path" == true ]]; then
    skip_step "Quickstart smoke" "the production golden path owns the same canonical quickstart lifecycle."
  else
    run_step "Quickstart smoke" ./scripts/test-quickstart.sh
  fi
else
  skip_step "Postgres integration tests" "docker is not installed or not on PATH."
  skip_step "Quickstart smoke" "docker is not installed or not on PATH."
fi

if [[ "$run_production_golden_path" == true ]]; then
  run_step "Production golden path" ./scripts/test-production-golden-path.sh \
    --stack quickstart \
    --client "${BITRIVER_GOLDEN_PATH_CLIENT:-docker}"
else
  skip_step "Production golden path" "disabled by default (use --production-golden-path or BITRIVER_TEST_ALL_PRODUCTION_GOLDEN_PATH=1)."
fi
