#!/usr/bin/env bash
set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

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

if command -v docker >/dev/null 2>&1 || [ -n "${BITRIVER_TEST_POSTGRES_DSN:-}" ]; then
  run_step "Postgres integration tests" ./scripts/test-postgres.sh
else
  skip_step "Postgres integration tests" "docker is not installed and BITRIVER_TEST_POSTGRES_DSN is not set."
fi

if command -v docker >/dev/null 2>&1; then
  run_step "Quickstart smoke" ./scripts/test-quickstart.sh
else
  skip_step "Quickstart smoke" "docker is not installed."
fi

run_step "Ingest end-to-end tests" ./scripts/test-ingest-e2e.sh

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
