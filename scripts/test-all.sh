#!/usr/bin/env bash
set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

usage() {
  cat <<'USAGE'
Usage: ./scripts/test-all.sh [--ingest-e2e]

Runs local validation entrypoints in one command:
  - ./scripts/verify.sh
  - ./scripts/test-postgres.sh (requires docker)
  - ./scripts/test-quickstart.sh (requires docker)
  - ./scripts/test-ingest-e2e.sh (only when explicitly enabled)
  - viewer integration tests (when node + npm + playwright are available)

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

run_step "Repository verification" ./scripts/verify.sh

if command -v docker >/dev/null 2>&1; then
  run_step "Postgres integration tests" ./scripts/test-postgres.sh
else
  skip_step "Postgres integration tests" "docker is not installed or not on PATH."
fi

if command -v docker >/dev/null 2>&1; then
  run_step "Quickstart smoke" ./scripts/test-quickstart.sh
else
  skip_step "Quickstart smoke" "docker is not installed or not on PATH."
fi

if [[ "$run_ingest_e2e" == true ]]; then
  run_step "Ingest end-to-end tests" ./scripts/test-ingest-e2e.sh
else
  skip_step "Ingest end-to-end tests" "disabled by default (use --ingest-e2e or BITRIVER_TEST_ALL_INGEST_E2E=1)."
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
