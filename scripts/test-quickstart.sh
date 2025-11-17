#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$REPO_ROOT/.env"
COMPOSE_FILE="$REPO_ROOT/deploy/docker-compose.yml"
COMPOSE_CONFIG_OUTPUT="$(mktemp)"
CREATED_ENV_FILE=false

cleanup() {
  rm -f "$COMPOSE_CONFIG_OUTPUT"
  if [ "$CREATED_ENV_FILE" = true ]; then
    rm -f "$ENV_FILE"
  fi
}
trap cleanup EXIT

if ! command -v docker >/dev/null 2>&1; then
  echo "error: docker is required for quickstart smoke checks" >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "error: docker compose v2 is required for quickstart smoke checks" >&2
  exit 1
fi

if [ ! -f "$ENV_FILE" ]; then
  CREATED_ENV_FILE=true
  cat >"$ENV_FILE" <<'ENV'
BITRIVER_LIVE_IMAGE_TAG=latest
BITRIVER_VIEWER_IMAGE_TAG=latest
BITRIVER_SRS_CONTROLLER_IMAGE_TAG=latest
BITRIVER_TRANSCODER_IMAGE_TAG=latest
BITRIVER_SRS_IMAGE_TAG=v5.0.185
BITRIVER_OME_IMAGE_TAG=0.15.10
BITRIVER_LIVE_PORT=8080
BITRIVER_LIVE_STORAGE_DRIVER=postgres
BITRIVER_LIVE_MODE=production
BITRIVER_LIVE_ADDR=:8080
BITRIVER_POSTGRES_DB=bitriver
BITRIVER_POSTGRES_USER=bitriver
BITRIVER_POSTGRES_PASSWORD=bitriver
BITRIVER_REDIS_PASSWORD=bitriver
BITRIVER_LIVE_POSTGRES_MAX_CONNS=15
BITRIVER_LIVE_POSTGRES_MIN_CONNS=5
BITRIVER_LIVE_POSTGRES_ACQUIRE_TIMEOUT=5s
BITRIVER_LIVE_POSTGRES_MAX_CONN_LIFETIME=30m
BITRIVER_LIVE_SESSION_STORE=postgres
BITRIVER_LIVE_CHAT_QUEUE_DRIVER=redis
BITRIVER_LIVE_CHAT_QUEUE_REDIS_ADDR=redis:6379
BITRIVER_LIVE_CHAT_QUEUE_REDIS_STREAM=bitriver-live-chat
BITRIVER_LIVE_CHAT_QUEUE_REDIS_GROUP=bitriver-live-api
BITRIVER_POSTGRES_HOST_PORT=5432
BITRIVER_VIEWER_ORIGIN=http://viewer:3000
BITRIVER_SRS_API=http://srs-controller:1985
BITRIVER_OME_API=http://ome:8081
BITRIVER_TRANSCODER_API=http://transcoder:9000
BITRIVER_TRANSCODER_PUBLIC_BASE_URL=http://localhost:9080
BITRIVER_TRANSCODER_HOST_PORT=9001
BITRIVER_INGEST_HEALTH=/healthz
BITRIVER_SRS_CONTROLLER_PORT=1986
SRS_CONTROLLER_UPSTREAM=http://srs:1985/api/
NEXT_PUBLIC_API_BASE_URL=
NEXT_VIEWER_BASE_PATH=/viewer
NEXT_PUBLIC_VIEWER_URL=http://localhost:8080/viewer
BITRIVER_LIVE_ADMIN_EMAIL=admin@example.com
BITRIVER_LIVE_ADMIN_PASSWORD=local-dev-password
BITRIVER_SRS_TOKEN=local-dev-token
BITRIVER_OME_USERNAME=admin
BITRIVER_OME_PASSWORD=local-dev-password
BITRIVER_TRANSCODER_TOKEN=local-dev-token
BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD=bitriver
ENV
fi

echo "Rendering docker compose config..."
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" config >"$COMPOSE_CONFIG_OUTPUT"

grep_healthcheck() {
  local service_label="$1"
  local expected_snippet="$2"

  if ! grep -q "$expected_snippet" "$COMPOSE_CONFIG_OUTPUT"; then
    echo "error: expected healthcheck for ${service_label} containing '${expected_snippet}'" >&2
    exit 1
  fi
}

grep_healthcheck "bitriver-live" "http://localhost:8080/healthz"
grep_healthcheck "srs-controller" "http://localhost:1985/healthz"
grep_healthcheck "srs" "http://localhost:1985/healthz"
grep_healthcheck "ome" "http://localhost:8081/healthz"
grep_healthcheck "transcoder" "http://localhost:9000/healthz"
grep_healthcheck "postgres" "pg_isready"
grep_healthcheck "redis" "redis-cli"

echo "Quickstart compose smoke checks passed."
