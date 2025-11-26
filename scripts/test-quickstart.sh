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
  if docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps >/dev/null 2>&1; then
    docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
  fi

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
BITRIVER_LIVE_POSTGRES_DSN=postgres://bitriver:bitriver@postgres:5432/bitriver?sslmode=disable
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
BITRIVER_REDIS_PORT=6379
BITRIVER_VIEWER_ORIGIN=http://viewer:3000
BITRIVER_SRS_API=http://srs-controller:1985
BITRIVER_SRS_API_PORT=1985
BITRIVER_OME_API=http://ome:8081
BITRIVER_OME_HTTP_PORT=8081
BITRIVER_OME_SIGNALLING_PORT=9000
BITRIVER_TRANSCODER_API=http://transcoder:9000
BITRIVER_TRANSCODER_PUBLIC_BASE_URL=http://localhost:9080
BITRIVER_TRANSCODER_HOST_PORT=9001
BITRIVER_INGEST_HEALTH=/healthz
BITRIVER_SRS_RTMP_PORT=1935
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

echo "Rendering OME config from template..."
"$SCRIPT_DIR/render-ome-config.sh" --force --env-file "$ENV_FILE" --quiet

OME_CONFIG="$REPO_ROOT/deploy/ome/Server.generated.xml"

if [ ! -f "$OME_CONFIG" ]; then
  echo "error: OME config missing at $OME_CONFIG after render" >&2
  exit 1
fi

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
grep_healthcheck "ome" "curl -fsS -u admin:local-dev-password http://localhost:8081/healthz"
grep_healthcheck "transcoder" "http://localhost:9000/healthz"
grep_healthcheck "postgres" "pg_isready"
grep_healthcheck "redis" "redis-cli"

if ! grep -q "Server.generated.xml:/opt/ovenmediaengine/bin/origin_conf/Server.xml" "$COMPOSE_CONFIG_OUTPUT"; then
  echo "error: expected OME to mount deploy/ome/Server.generated.xml into origin_conf" >&2
  exit 1
fi

if ! grep -q "Server.generated.xml:/opt/ovenmediaengine/bin/edge_conf/Server.xml" "$COMPOSE_CONFIG_OUTPUT"; then
  echo "error: expected OME to mount deploy/ome/Server.generated.xml into edge_conf" >&2
  exit 1
fi

python3 - "$ENV_FILE" "$OME_CONFIG" <<'PY'
import sys
import xml.etree.ElementTree as ET
from pathlib import Path

env_path = Path(sys.argv[1])
config_path = Path(sys.argv[2])

env_values: dict[str, str] = {}
for line in env_path.read_text().splitlines():
    line = line.strip()
    if not line or line.startswith("#"):
        continue
    if "=" not in line:
        continue
    key, value = line.split("=", 1)
    env_values[key] = value

required = ["BITRIVER_OME_USERNAME", "BITRIVER_OME_PASSWORD"]
missing = [key for key in required if not env_values.get(key)]
if missing:
    sys.exit(f"error: missing required OME credentials in {env_path}: {', '.join(missing)}")

bind_default = "0.0.0.0"
expected_bind = env_values.get("BITRIVER_OME_BIND", bind_default)
expected_port = env_values.get("BITRIVER_OME_SERVER_PORT", "9000")
expected_tls_port = env_values.get("BITRIVER_OME_SERVER_TLS_PORT", "9443")

tree = ET.parse(config_path)
root = tree.getroot()

root_bind_ip = root.findtext("./Bind/IP")
root_port = root.findtext("./Bind/Port")
root_tls_port = root.findtext("./Bind/TLSPort")
bind = root.findtext(".//Control/Server/Listeners/TCP/Bind")
ip = root.findtext(".//Control/Server/Listeners/TCP/IP")
username = root.findtext(".//Control/Authentication/User/ID")
password = root.findtext(".//Control/Authentication/User/Password")

values = {
    "RootBind": root_bind_ip,
    "RootPort": root_port,
    "RootTLSPort": root_tls_port,
    "Bind": bind,
    "IP": ip,
    "ID": username,
    "Password": password,
}

empty = [tag for tag, value in values.items() if value is None or not value.strip()]
if empty:
    sys.exit(
        "error: OME config is missing required tags: " + ", ".join(f"<{tag}>" for tag in empty)
    )

if bind != expected_bind or ip != expected_bind:
    sys.exit(
        "error: expected <Bind> and <IP> to match BITRIVER_OME_BIND="
        f"{expected_bind}; got Bind='{bind}', IP='{ip}'"
    )

if root_bind_ip != expected_bind or root_port != expected_port or root_tls_port != expected_tls_port:
    sys.exit(
        "error: expected root <Bind> to match env values: "
        f"address={root_bind_ip}, port={root_port}, tlsPort={root_tls_port}, "
        f"expected address={expected_bind}, port={expected_port}, tlsPort={expected_tls_port}"
    )

if username != env_values["BITRIVER_OME_USERNAME"] or password != env_values["BITRIVER_OME_PASSWORD"]:
    sys.exit("error: rendered OME credentials do not match .env defaults")

print("OME config validation passed.")
PY

echo "Starting docker compose stack..."
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d

# shellcheck disable=SC1090
set -a
. "$ENV_FILE"
set +a

WAIT_TIMEOUT=${WAIT_TIMEOUT:-300}

wait_for_health() {
  local service_name="$1"
  local deadline=$((SECONDS + WAIT_TIMEOUT))

  while true; do
    container_id=$(docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps -q "$service_name")
    if [ -z "$container_id" ]; then
      echo "error: no container found for service $service_name" >&2
      exit 1
    fi

    status=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container_id")
    case "$status" in
    healthy)
      echo "Service $service_name is healthy."
      return 0
      ;;
    unhealthy)
      echo "error: service $service_name reported unhealthy" >&2
      docker inspect "$container_id"
      exit 1
      ;;
    esac

    if [ "$SECONDS" -ge "$deadline" ]; then
      echo "error: timed out waiting for $service_name to become healthy (last status: $status)" >&2
      exit 1
    fi

    sleep 5
  done
}

SERVICES_WITH_HEALTHCHECKS=(
  bitriver-live
  srs-controller
  srs
  ome
  transcoder
  postgres
  redis
)

echo "Waiting for services to report healthy..."
for service in "${SERVICES_WITH_HEALTHCHECKS[@]}"; do
  wait_for_health "$service"
done

API_PORT=${BITRIVER_LIVE_PORT:-8080}
VIEWER_PATH=${NEXT_VIEWER_BASE_PATH:-/viewer}

echo "CURLing API and viewer endpoints..."
curl -fsS "http://localhost:${API_PORT}/healthz" >/dev/null
curl -fsSL "http://localhost:${API_PORT}${VIEWER_PATH}" >/dev/null

echo "Quickstart compose smoke checks passed."
