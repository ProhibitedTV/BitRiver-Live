#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/polling.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$REPO_ROOT/deploy/docker-compose.yml"

if [[ -f "$REPO_ROOT/.env" ]]; then
  ENV_FILE="$REPO_ROOT/.env"
elif [[ -f "$REPO_ROOT/deploy/.env.example" ]]; then
  ENV_FILE="$REPO_ROOT/deploy/.env.example"
else
  echo "FAIL: missing env file (.env or deploy/.env.example)" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "FAIL: docker is required" >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "FAIL: docker compose v2 is required" >&2
  exit 1
fi

PROJECT_NAME="bitriver-smoke-${RANDOM}-$$"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-180}"
WAIT_INTERVAL="${WAIT_INTERVAL:-3}"
READY_URL=""

# shellcheck disable=SC2317,SC2329
cleanup() {
  docker compose --project-name "$PROJECT_NAME" --env-file "$ENV_FILE" -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

# shellcheck disable=SC2317,SC2329
wait_for_readyz() {
  if curl -fsS "$READY_URL" >/dev/null; then
    return 0
  fi
  return 1
}

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

API_PORT="${BITRIVER_LIVE_PORT:-8080}"
READY_URL="http://localhost:${API_PORT}/readyz"

echo "Starting compose stack (project: $PROJECT_NAME)..."
docker compose --project-name "$PROJECT_NAME" --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d >/dev/null

echo "Waiting for API readiness at $READY_URL ..."
if bounded_poll "$WAIT_TIMEOUT" "$WAIT_INTERVAL" wait_for_readyz; then
  echo "PASS: deploy smoke succeeded ($READY_URL reachable)"
  exit 0
fi

poll_rc=$?
if [[ "$poll_rc" -eq 124 ]]; then
  echo "FAIL: deploy smoke timed out waiting for $READY_URL" >&2
else
  echo "FAIL: deploy smoke failed while checking $READY_URL (exit=$poll_rc)" >&2
fi

exit 1
