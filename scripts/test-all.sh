#!/usr/bin/env bash
set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

usage() {
  cat <<'USAGE'
Usage: ./scripts/test-all.sh [--production-golden-path]

Runs local validation entrypoints in one command:
  - ./scripts/test-unit.sh
  - ./scripts/verify.sh
  - ./scripts/test-postgres.sh (opt-in)
  - ./scripts/test-quickstart.sh (opt-in)
  - ./scripts/test-production-golden-path.sh via canonical quickstart (opt-in)
  - viewer integration tests (when node + npm + playwright are available)

Integration controls:
  BITRIVER_TEST_POSTGRES=1       Run Postgres integration tests.
  BITRIVER_TEST_QUICKSTART=1     Run quickstart smoke test.

Production golden-path controls:
  --production-golden-path                    Run the real product gate.
  BITRIVER_TEST_ALL_PRODUCTION_GOLDEN_PATH=1  Run it by environment override.

Compatibility:
  --ingest-e2e and BITRIVER_TEST_ALL_INGEST_E2E=1 remain accepted aliases.
USAGE
}

run_production_golden_path=false
run_postgres=false
run_quickstart=false

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

if [[ "${BITRIVER_TEST_POSTGRES:-}" == "1" ]]; then
  run_postgres=true
fi

if [[ "${BITRIVER_TEST_QUICKSTART:-}" == "1" ]]; then
  run_quickstart=true
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

run_step "Unit tests" ./scripts/test-unit.sh

run_step "Verification gate" ./scripts/verify.sh

if [[ "$run_postgres" == true ]]; then
  if command -v docker >/dev/null 2>&1 || [[ -n "${BITRIVER_TEST_POSTGRES_DSN:-}" ]]; then
    run_step "Postgres integration tests" ./scripts/test-postgres.sh
  else
    skip_step "Postgres integration tests" "docker is not installed or not on PATH, and BITRIVER_TEST_POSTGRES_DSN is unset."
  fi
else
  skip_step "Postgres integration tests" "disabled by default (set BITRIVER_TEST_POSTGRES=1 to enable)."
fi

if [[ "$run_quickstart" == true ]]; then
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    run_step "Release smoke gate fast tier" ./scripts/release-gate-smoke.sh \
      --tier fast \
      --artifact-dir "${BITRIVER_RELEASE_GATE_ARTIFACT_DIR:-.artifacts/release-gate-fast}" \
      --target "${BITRIVER_RELEASE_GATE_TARGET:-v0.0.0-ci}"
    if [[ "$run_production_golden_path" == true ]]; then
      skip_step "Quickstart smoke" "the production golden path owns the same canonical quickstart lifecycle."
    else
      run_step "Quickstart smoke" ./scripts/test-quickstart.sh
    fi
  else
    skip_step "Release smoke gate fast tier" "docker compose is not installed, not on PATH, or unavailable."
    skip_step "Quickstart smoke" "docker compose is not installed, not on PATH, or unavailable."
  fi
else
  skip_step "Quickstart smoke" "disabled by default (set BITRIVER_TEST_QUICKSTART=1 to enable)."
fi

if [[ "$run_production_golden_path" == true ]]; then
  run_step "Production golden path" ./scripts/test-production-golden-path.sh \
    --stack quickstart \
    --client "${BITRIVER_GOLDEN_PATH_CLIENT:-docker}"
else
  skip_step "Production golden path" "disabled by default (use --production-golden-path or BITRIVER_TEST_ALL_PRODUCTION_GOLDEN_PATH=1)."
fi

if [ ! -d web/viewer ]; then
  skip_step "Viewer integration/playwright" "web/viewer does not exist in this checkout."
elif ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
  skip_step "Viewer integration/playwright" "node and/or npm are not installed or not on PATH."
elif [ ! -d web/viewer/node_modules ]; then
  skip_step "Viewer integration/playwright" "web/viewer/node_modules is missing; run npm install in web/viewer first."
elif [ ! -x web/viewer/node_modules/.bin/playwright ]; then
  skip_step "Viewer integration/playwright" "Playwright is not installed in web/viewer dependencies."
else
  run_step "Viewer integration/playwright" npm --prefix web/viewer run test:integration
fi

echo

echo "test-all sequence completed."
