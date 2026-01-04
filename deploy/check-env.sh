#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="${1:-$REPO_ROOT/.env}"

if [ ! -f "$ENV_FILE" ]; then
  echo "Environment file not found at $ENV_FILE" >&2
  exit 1
fi

mode_line=$(grep -E '^[[:space:]]*BITRIVER_LIVE_MODE=' "$ENV_FILE" | tail -n 1 || true)
mode_value=$(echo "${mode_line#*=}" | xargs)

if [ -z "$mode_line" ] || [ -z "$mode_value" ]; then
  echo "BITRIVER_LIVE_MODE must be set to production in $ENV_FILE" >&2
  echo "Keep the primary .env at production and apply a one-off override (for example, BITRIVER_LIVE_MODE=development docker compose --env-file $ENV_FILE -f deploy/docker-compose.yml up) when you intentionally need development mode." >&2
  exit 1
fi

if [ "${mode_value,,}" = "development" ]; then
  echo "BITRIVER_LIVE_MODE=development is not allowed in $ENV_FILE; deployments must default to production." >&2
  echo "Use an explicit override (for example, BITRIVER_LIVE_MODE=development docker compose --env-file $ENV_FILE -f deploy/docker-compose.yml up) for local HTTP-only demos instead of changing the main env file." >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Go is required to run environment validation." >&2
  exit 1
fi

(
  cd "$REPO_ROOT" &&
  GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver env validate --env-file "$ENV_FILE"
)
