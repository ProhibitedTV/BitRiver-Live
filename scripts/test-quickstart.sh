#!/usr/bin/env bash
set -euo pipefail

# Git Bash rewrites POSIX-looking environment values before launching native
# Windows executables. Compose must receive container paths such as /healthz
# unchanged, while normal argument conversion remains enabled for temp files.
if [[ "${OSTYPE:-}" == msys* || "${OSTYPE:-}" == cygwin* ]]; then
  export MSYS2_ENV_CONV_EXCL="*"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/polling.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ROOT_ENV_FILE="$REPO_ROOT/.env"
ENV_FILE="${BITRIVER_SMOKE_ENV_FILE:-$REPO_ROOT/.env}"
COMPOSE_FILE="$REPO_ROOT/deploy/docker-compose.yml"
COMPOSE_CONFIG_OUTPUT="$(mktemp)"
COMPOSE_SMOKE_OVERRIDE=""
GOLDEN_PATH_METRICS_BEARER_FILE=""
OME_CONFIG="$REPO_ROOT/deploy/ome/Server.generated.xml"
OME_CONFIG_BACKUP=""
OME_CONFIG_EXISTED=false
BITRIVER_DATA_DIR="$REPO_ROOT/deploy/data"
TRANSCODER_DATA_DIR="$REPO_ROOT/deploy/transcoder-data"
CREATED_ENV_FILE=false
CREATED_ROOT_ENV_BRIDGE=false
CREATED_BITRIVER_DATA_DIR=false
CREATED_TRANSCODER_DATA_DIR=false
PYTHON_RUNNER=()
COMPOSE_RUNTIME_ARGS=("-f" "$COMPOSE_FILE")

SMOKE_IMAGE_SOURCE="${BITRIVER_SMOKE_IMAGE_SOURCE:-build}"
SMOKE_LIVE_MODE="${BITRIVER_SMOKE_LIVE_MODE:-development}"

case "$SMOKE_IMAGE_SOURCE" in
  build|pull)
    ;;
  *)
    echo "error: BITRIVER_SMOKE_IMAGE_SOURCE must be build or pull" >&2
    exit 2
    ;;
esac

apply_smoke_mode_overrides() {
  export BITRIVER_DEPLOY_IMAGE_SOURCE="$SMOKE_IMAGE_SOURCE"
  export BITRIVER_LIVE_MODE="$SMOKE_LIVE_MODE"
  if [[ "$SMOKE_IMAGE_SOURCE" == "build" ]]; then
    export BITRIVER_LIVE_IMAGE_DIGEST=""
    export BITRIVER_VIEWER_IMAGE_DIGEST=""
    export BITRIVER_SRS_CONTROLLER_IMAGE_DIGEST=""
    export BITRIVER_TRANSCODER_IMAGE_DIGEST=""
    export BITRIVER_OME_CONFIG_IMAGE_DIGEST=""
    export BITRIVER_SRS_IMAGE_DIGEST=""
  fi
}
apply_smoke_mode_overrides
export BITRIVER_SRS_PUBLIC_RTMP_BASE_URL="${BITRIVER_SRS_PUBLIC_RTMP_BASE_URL:-rtmp://localhost:1935/live}"
export BITRIVER_OME_PUBLIC_LLHLS_BASE_URL="${BITRIVER_OME_PUBLIC_LLHLS_BASE_URL:-http://localhost:18080/live}"
export BITRIVER_TRANSCODER_PUBLIC_BASE_URL="${BITRIVER_TRANSCODER_PUBLIC_BASE_URL:-http://localhost:9080/hls}"
DOCKER_BUILD_GOPROXY="${BITRIVER_DOCKER_GOPROXY:-https://proxy.golang.org,direct}"
DOCKER_BUILD_GOSUMDB="${BITRIVER_DOCKER_GOSUMDB:-sum.golang.org}"

COMPOSE_SMOKE_OVERRIDE="$(mktemp)"
if [[ "${OSTYPE:-}" != msys* && "${OSTYPE:-}" != cygwin* ]]; then
  host_uid="$(id -u)"
  host_gid="$(id -g)"
  cat >"$COMPOSE_SMOKE_OVERRIDE" <<YAML
services:
  bitriver-live:
    user: "${host_uid}:${host_gid}"
  srs-config:
    user: "${host_uid}:${host_gid}"
  ome-config:
    user: "${host_uid}:${host_gid}"
  ome-health-token-check:
    user: "${host_uid}:${host_gid}"
  # The transcoder image owns /work as its non-root image user. Do not replace
  # that UID when /work is an isolated named volume.
  transcoder:
    volumes:
      - bitriver-smoke-transcoder:/work
  transcoder-public:
    volumes:
      - bitriver-smoke-transcoder:/work:ro
volumes:
  bitriver-smoke-transcoder:
YAML
else
  cat >"$COMPOSE_SMOKE_OVERRIDE" <<'YAML'
services:
  transcoder:
    volumes:
      - bitriver-smoke-transcoder:/work
  transcoder-public:
    volumes:
      - bitriver-smoke-transcoder:/work:ro
volumes:
  bitriver-smoke-transcoder:
YAML
fi
COMPOSE_RUNTIME_ARGS+=("-f" "$COMPOSE_SMOKE_OVERRIDE")

cleanup() {
  rm -f "$COMPOSE_CONFIG_OUTPUT"
  if [[ -n "$GOLDEN_PATH_METRICS_BEARER_FILE" ]]; then
    rm -f "$GOLDEN_PATH_METRICS_BEARER_FILE"
  fi
  if docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" ps >/dev/null 2>&1; then
    docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  fi

  if [[ -n "$COMPOSE_SMOKE_OVERRIDE" ]]; then
    rm -f "$COMPOSE_SMOKE_OVERRIDE"
  fi

  if [[ -n "$OME_CONFIG_BACKUP" ]]; then
    if [[ "$OME_CONFIG_EXISTED" == true ]]; then
      cp "$OME_CONFIG_BACKUP" "$OME_CONFIG"
    else
      rm -f "$OME_CONFIG"
    fi
    rm -f "$OME_CONFIG_BACKUP"
  fi

  if [ "$CREATED_ENV_FILE" = true ]; then
    rm -f "$ENV_FILE"
  fi

  if [ "$CREATED_ROOT_ENV_BRIDGE" = true ]; then
    rm -f "$ROOT_ENV_FILE"
  fi

  if [ "$CREATED_BITRIVER_DATA_DIR" = true ]; then
    rmdir "$BITRIVER_DATA_DIR" 2>/dev/null || true
  fi

  if [ "$CREATED_TRANSCODER_DATA_DIR" = true ]; then
    rmdir "$TRANSCODER_DATA_DIR/public/live" "$TRANSCODER_DATA_DIR/public/uploads" 2>/dev/null || true
    rmdir "$TRANSCODER_DATA_DIR/public" "$TRANSCODER_DATA_DIR/live" "$TRANSCODER_DATA_DIR/uploads" 2>/dev/null || true
    rmdir "$TRANSCODER_DATA_DIR" 2>/dev/null || true
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

if [ ! -f "$ENV_FILE" ] && [[ "$SMOKE_IMAGE_SOURCE" == "pull" ]]; then
  echo "error: pull-mode quickstart smoke requires BITRIVER_SMOKE_ENV_FILE to name an existing release environment" >&2
  exit 2
fi

if [ ! -f "$ENV_FILE" ]; then
  CREATED_ENV_FILE=true
  cat >"$ENV_FILE" <<'ENV'
BITRIVER_DEPLOY_IMAGE_SOURCE=build
BITRIVER_LIVE_IMAGE_TAG=latest
BITRIVER_VIEWER_IMAGE_TAG=latest
BITRIVER_SRS_CONTROLLER_IMAGE_TAG=latest
BITRIVER_TRANSCODER_IMAGE_TAG=latest
BITRIVER_SRS_IMAGE_TAG=v5.0.185
BITRIVER_OME_IMAGE_TAG=0.16.0
BITRIVER_LIVE_PORT=18080
BITRIVER_LIVE_STORAGE_DRIVER=postgres
BITRIVER_LIVE_MODE=development
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
BITRIVER_SRS_PUBLIC_RTMP_BASE_URL=rtmp://localhost:1935/live
BITRIVER_OME_API=http://ome:8081
BITRIVER_OME_HTTP_PORT=8081
BITRIVER_OME_SIGNALLING_PORT=9000
BITRIVER_OME_PUBLIC_LLHLS_BASE_URL=http://localhost:18080/live
BITRIVER_TRANSCODER_API=http://transcoder:9000
BITRIVER_TRANSCODER_PUBLIC_BASE_URL=https://example.com/hls
BITRIVER_TRANSCODER_HOST_PORT=9001
BITRIVER_INGEST_HEALTH=/healthz
BITRIVER_SRS_RTMP_PORT=1935
BITRIVER_SRS_CONTROLLER_PORT=1986
SRS_CONTROLLER_UPSTREAM=http://srs:1985/api/
NEXT_PUBLIC_API_BASE_URL=
NEXT_VIEWER_BASE_PATH=/viewer
NEXT_PUBLIC_VIEWER_URL=http://localhost:18080/viewer
BITRIVER_LIVE_ADMIN_EMAIL=admin@example.com
BITRIVER_LIVE_ADMIN_PASSWORD=local-dev-password
BITRIVER_SRS_TOKEN=local-dev-token
BITRIVER_OME_USERNAME=admin
BITRIVER_OME_PASSWORD=local-dev-password
BITRIVER_OME_API_TOKEN=local-dev-access-token
BITRIVER_OME_ACCESS_TOKEN=local-dev-access-token
BITRIVER_TRANSCODER_TOKEN=local-dev-token
BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD=bitriver
ENV
fi

if [[ "$ENV_FILE" != "$ROOT_ENV_FILE" ]]; then
  if [ -e "$ROOT_ENV_FILE" ]; then
    if ! cmp -s "$ENV_FILE" "$ROOT_ENV_FILE"; then
      echo "error: explicit smoke env cannot replace an existing operator-owned root .env" >&2
      exit 2
    fi
  else
    cp "$ENV_FILE" "$ROOT_ENV_FILE"
    chmod 600 "$ROOT_ENV_FILE"
    CREATED_ROOT_ENV_BRIDGE=true
  fi
fi

if [[ "$SMOKE_IMAGE_SOURCE" == "pull" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
  export BITRIVER_DEPLOY_IMAGE_SOURCE="$SMOKE_IMAGE_SOURCE"
  export BITRIVER_LIVE_MODE="$SMOKE_LIVE_MODE"
  "$SCRIPT_DIR/require-image-digests.sh" --env-file "$ENV_FILE"
fi

if [ ! -d "$BITRIVER_DATA_DIR" ]; then
  CREATED_BITRIVER_DATA_DIR=true
fi
mkdir -p "$BITRIVER_DATA_DIR"

if [ ! -d "$TRANSCODER_DATA_DIR" ]; then
  CREATED_TRANSCODER_DATA_DIR=true
fi
mkdir -p "$TRANSCODER_DATA_DIR/live" "$TRANSCODER_DATA_DIR/uploads" "$TRANSCODER_DATA_DIR/public/live" "$TRANSCODER_DATA_DIR/public/uploads"

echo "Rendering docker compose config..."
docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" config >"$COMPOSE_CONFIG_OUTPUT"

if python3 -c 'import sys' >/dev/null 2>&1; then
  PYTHON_RUNNER=(python3)
elif py -3 -c 'import sys' >/dev/null 2>&1; then
  PYTHON_RUNNER=(py -3)
elif python -c 'import sys' >/dev/null 2>&1; then
  PYTHON_RUNNER=(python)
else
  echo "error: python3, python, or py -3 is required for quickstart smoke checks" >&2
  exit 1
fi

if [[ "$SMOKE_IMAGE_SOURCE" == "build" ]]; then
  echo "Building the canonical OME config helper image..."
  env \
    GOPROXY="$DOCKER_BUILD_GOPROXY" \
    GOSUMDB="$DOCKER_BUILD_GOSUMDB" \
    docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" build ome-config
else
  echo "Using the published OME config helper image from the release environment."
  docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" pull ome-config
fi

OME_CONFIG_BACKUP="$(mktemp)"
if [[ -f "$OME_CONFIG" ]]; then
  OME_CONFIG_EXISTED=true
  cp "$OME_CONFIG" "$OME_CONFIG_BACKUP"
fi

echo "Rendering OME config from template..."
docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" run --rm --no-deps -T ome-config >/dev/null

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
grep_healthcheck "srs" "/api/v1/versions"
grep_healthcheck "ome" "http://localhost:${BITRIVER_OME_HTTP_PORT:-8081}/"
grep_healthcheck "transcoder" "http://localhost:9000/healthz"
grep_healthcheck "postgres" "pg_isready"
grep_healthcheck "redis" "redis-cli"

if ! grep -q "Server.generated.xml" "$COMPOSE_CONFIG_OUTPUT" || ! grep -q "target: /opt/ovenmediaengine/bin/origin_conf/Server.xml" "$COMPOSE_CONFIG_OUTPUT"; then
  echo "error: expected OME to mount deploy/ome/Server.generated.xml into origin_conf" >&2
  exit 1
fi

if ! grep -q "Server.generated.xml" "$COMPOSE_CONFIG_OUTPUT" || ! grep -q "target: /opt/ovenmediaengine/bin/edge_conf/Server.xml" "$COMPOSE_CONFIG_OUTPUT"; then
  echo "error: expected OME to mount deploy/ome/Server.generated.xml into edge_conf" >&2
  exit 1
fi

"${PYTHON_RUNNER[@]}" - "$ENV_FILE" "$OME_CONFIG" <<'PY'
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

required = ["BITRIVER_OME_API_TOKEN"]
missing = [key for key in required if not env_values.get(key)]
if missing:
    sys.exit(f"error: missing required OME credentials in {env_path}: {', '.join(missing)}")

bind_default = "0.0.0.0"
expected_bind = env_values.get("BITRIVER_OME_BIND", bind_default)
expected_port = env_values.get("BITRIVER_OME_HTTP_PORT", "8081")
expected_tls_port = env_values.get("BITRIVER_OME_HTTP_TLS_PORT", "8082")

tree = ET.parse(config_path)
root = tree.getroot()

root_ip = root.findtext("./IP")
legacy_root_bind_ip = root.findtext("./Bind/IP")
legacy_root_bind_address = root.findtext("./Bind/Address")
if (legacy_root_bind_ip and legacy_root_bind_ip.strip()) or (legacy_root_bind_address and legacy_root_bind_address.strip()):
    sys.exit("error: expected root <Bind> to omit unsupported host child tags (<IP>/<Address>)")
listener_port = root.findtext("./Bind/Managers/API/Port")
listener_tls_port = root.findtext("./Bind/Managers/API/TLSPort")
listener_worker_count = root.findtext("./Bind/Managers/API/WorkerCount")
api_access_token = root.findtext("./Managers/API/AccessToken")
legacy_access_tokens = root.find("./Managers/API/AccessTokens")
outputs_wrapper = root.find(".//Application/Outputs")
output_profiles = root.find(".//Application/OutputProfiles")
application_llhls = root.find(".//Application/LLHLS")
output_streams = root.find(".//Application/OutputProfiles/OutputProfile/OutputStreams")
output_stream_name = root.findtext(".//Application/OutputProfiles/OutputProfile/OutputStreamName")
misplaced_listener_token = root.find("./Bind/Managers/API/AccessToken")
misplaced_auth_listener = [
    tag for tag in ("Port", "TLSPort", "WorkerCount")
    if root.find(f"./Managers/API/{tag}") is not None
]

values = {
    "ListenerPort": listener_port,
    "ListenerTLSPort": listener_tls_port,
    "ListenerWorkerCount": listener_worker_count,
    "AccessToken": api_access_token,
}

empty = [tag for tag, value in values.items() if value is None or not value.strip()]
if empty:
    sys.exit(
        "error: OME config is missing required tags: " + ", ".join(f"<{tag}>" for tag in empty)
    )

if listener_port != expected_port or listener_tls_port != expected_tls_port:
    sys.exit(
        "error: expected <Bind><Managers><API> listener ports to match env values: "
        f"port={listener_port}, tlsPort={listener_tls_port}, "
        f"expected port={expected_port}, tlsPort={expected_tls_port}"
    )

if (root_ip or "").strip() != expected_bind:
    sys.exit(
        "error: expected root <IP> to match BITRIVER_OME_BIND: "
        f"ip={root_ip}, expected={expected_bind}"
    )

if legacy_access_tokens is not None:
    sys.exit("error: rendered OME config still uses deprecated <AccessTokens> wrapper")

if misplaced_listener_token is not None:
    sys.exit("error: rendered OME config placed <AccessToken> under <Bind><Managers><API>")

if outputs_wrapper is not None:
    sys.exit("error: rendered OME config still wraps output profiles in deprecated <Application><Outputs>")

if output_profiles is None:
    sys.exit("error: rendered OME config is missing direct <Application><OutputProfiles>")

if application_llhls is not None:
    sys.exit("error: rendered OME config still contains deprecated application-level <LLHLS> node")

if output_streams is not None:
    sys.exit("error: rendered OME config still uses deprecated <OutputStreams> wrapper")

if (output_stream_name or "").strip() != "${OriginStreamName}":
    sys.exit("error: rendered OME config must set <OutputStreamName>${OriginStreamName}</OutputStreamName>")

if misplaced_auth_listener:
    sys.exit(
        "error: rendered OME config placed listener field(s) under top-level <Managers><API>: "
        + ", ".join(misplaced_auth_listener)
    )

if api_access_token != env_values["BITRIVER_OME_API_TOKEN"]:
    sys.exit("error: rendered OME API <AccessToken> does not match .env defaults")

healthcheck_token = env_values.get("BITRIVER_OME_HEALTHCHECK_TOKEN")
legacy_token = env_values.get("BITRIVER_OME_ACCESS_TOKEN")
api_token = env_values["BITRIVER_OME_API_TOKEN"]
if healthcheck_token and healthcheck_token != api_token:
    sys.exit(
        "error: BITRIVER_OME_HEALTHCHECK_TOKEN should match BITRIVER_OME_API_TOKEN for health checks"
    )
if legacy_token and legacy_token != api_token:
    sys.exit(
        "error: BITRIVER_OME_ACCESS_TOKEN should match BITRIVER_OME_API_TOKEN for health checks"
    )
if healthcheck_token and legacy_token and healthcheck_token != legacy_token:
    sys.exit(
        "error: BITRIVER_OME_HEALTHCHECK_TOKEN should match BITRIVER_OME_ACCESS_TOKEN for health checks"
    )

print("OME config validation passed.")
PY

if [[ "$SMOKE_IMAGE_SOURCE" == "build" ]]; then
  echo "Building local docker compose images..."
  env \
    GOPROXY="$DOCKER_BUILD_GOPROXY" \
    GOSUMDB="$DOCKER_BUILD_GOSUMDB" \
    docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" build
else
  echo "Pull-only smoke selected; no Compose image builds are permitted."
fi

echo "Pulling missing runtime images..."
mapfile -t runtime_images < <(
  docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" config --images | sort -u
)
for image in "${runtime_images[@]}"; do
  if [[ -z "$image" ]]; then
    continue
  fi
  if docker image inspect "$image" >/dev/null 2>&1; then
    echo "Using local runtime image: $image"
    continue
  fi
  echo "Pulling missing runtime image: $image"
  docker pull "$image"
done

dump_compose_diagnostics() {
  docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" ps -a >&2 || true
  docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" logs --tail=60 bitriver-live viewer srs-controller transcoder postgres-migrations srs-config ome-config ome-health-token-check postgres redis >&2 || true
}

escape_github_annotation() {
  local value="$1"

  value="${value//'%'/'%25'}"
  value="${value//$'\r'/'%0D'}"
  value="${value//$'\n'/'%0A'}"
  printf '%s' "$value"
}

emit_ci_error() {
  local title="$1"
  local message="$2"

  if [[ "${GITHUB_ACTIONS:-}" != "true" ]]; then
    return 0
  fi

  printf '::error title=%s::%s\n' \
    "$(escape_github_annotation "$title")" \
    "$(escape_github_annotation "$message")" >&2
}

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a
apply_smoke_mode_overrides

if [[ "${BITRIVER_TEST_GOLDEN_PATH:-}" == "1" ]]; then
  # The production product exercise needs public account creation and media URLs
  # that are reachable from the same host that runs ffmpeg/ffprobe. These
  # release-gate overrides never rewrite the operator-owned .env file.
  export BITRIVER_LIVE_MODE="$SMOKE_LIVE_MODE"
  export BITRIVER_LIVE_ALLOW_SELF_SIGNUP=true
  export BITRIVER_SRS_PUBLIC_RTMP_BASE_URL="rtmp://localhost:${BITRIVER_SRS_RTMP_PORT:-1935}/live"
  export BITRIVER_OME_PUBLIC_LLHLS_BASE_URL="http://localhost:${BITRIVER_LIVE_PORT:-8080}/live"
  export BITRIVER_TRANSCODER_PUBLIC_BASE_URL="http://localhost:${BITRIVER_TRANSCODER_PUBLIC_PORT:-9080}/hls"
  # Keep the content gate deterministic on two-core CI and Docker Desktop
  # while retaining the required 1080p encode/decode path. Full ladder policy
  # remains covered by transcoder contract tests; browser quality selection is
  # a separate release gate.
  export BITRIVER_TRANSCODE_LADDER="1080p:2500"
  if [[ -n "${BITRIVER_LIVE_METRICS_TOKEN:-}" ]]; then
    GOLDEN_PATH_METRICS_BEARER_FILE="$(mktemp)"
    printf '%s\n' "$BITRIVER_LIVE_METRICS_TOKEN" >"$GOLDEN_PATH_METRICS_BEARER_FILE"
    export BITRIVER_GOLDEN_PATH_METRICS_BEARER_FILE="$GOLDEN_PATH_METRICS_BEARER_FILE"
  fi
fi

WAIT_TIMEOUT=${WAIT_TIMEOUT:-300}

start_compose_services() {
  local label="$1"
  shift
  local compose_output
  local compose_tail

  echo "Starting $label..."
  compose_output="$(mktemp)"
  if ! docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" up -d --no-build --pull never "$@" >"$compose_output" 2>&1; then
    cat "$compose_output" >&2
    compose_tail="$(tail -n 30 "$compose_output")"
    rm -f "$compose_output"
    emit_ci_error "docker compose $label failed" "$compose_tail"
    echo "error: docker compose $label failed to start" >&2
    dump_compose_diagnostics
    exit 1
  fi
  cat "$compose_output"
  rm -f "$compose_output"
}

start_compose_services_without_deps() {
  local label="$1"
  shift
  local compose_output
  local compose_tail

  echo "Starting $label..."
  compose_output="$(mktemp)"
  if ! docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" up -d --no-build --pull never --no-deps "$@" >"$compose_output" 2>&1; then
    cat "$compose_output" >&2
    compose_tail="$(tail -n 30 "$compose_output")"
    rm -f "$compose_output"
    emit_ci_error "docker compose $label failed" "$compose_tail"
    echo "error: docker compose $label failed to start" >&2
    dump_compose_diagnostics
    exit 1
  fi
  cat "$compose_output"
  rm -f "$compose_output"
}

wait_for_health_check() {
  local service_name="$1"

  container_id=$(docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" ps -q "$service_name")
  if [ -z "$container_id" ]; then
    echo "error: no container found for service $service_name" >&2
    return 2
  fi

  status=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container_id")
  WAIT_FOR_HEALTH_LAST_STATUS="$status"

  case "$status" in
  healthy)
    echo "Service $service_name is healthy."
    return 0
    ;;
  unhealthy)
    echo "error: service $service_name reported unhealthy" >&2
    docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" logs --tail=80 "$service_name" >&2 || true
    docker inspect \
      --format 'state={{.State.Status}} exit_code={{.State.ExitCode}} error={{json .State.Error}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' \
      "$container_id" >&2 || true
    return 2
    ;;
  *)
    return 1
    ;;
  esac
}

wait_for_health() {
  local service_name="$1"
  WAIT_FOR_HEALTH_LAST_STATUS="unknown"

  if bounded_poll "$WAIT_TIMEOUT" 5 wait_for_health_check "$service_name"; then
    return 0
  fi

  poll_rc=$?
  if [ "$poll_rc" -eq 124 ]; then
    echo "error: timed out waiting for $service_name to become healthy (last status: $WAIT_FOR_HEALTH_LAST_STATUS)" >&2
    emit_ci_error "service health timeout" "$service_name last status: $WAIT_FOR_HEALTH_LAST_STATUS"
  else
    echo "error: failed waiting for $service_name to become healthy (poll result: $poll_rc, last status: $WAIT_FOR_HEALTH_LAST_STATUS)" >&2
    emit_ci_error "service health failure" "$service_name poll result: $poll_rc, last status: $WAIT_FOR_HEALTH_LAST_STATUS"
  fi
  exit 1
}

wait_for_completed_check() {
  local service_name="$1"

  container_id=$(docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" ps -a -q "$service_name")
  if [ -z "$container_id" ]; then
    echo "error: no container found for service $service_name" >&2
    return 2
  fi

  status=$(docker inspect -f '{{.State.Status}}' "$container_id")
  exit_code=$(docker inspect -f '{{.State.ExitCode}}' "$container_id")
  WAIT_FOR_COMPLETED_LAST_STATUS="$status"

  case "$status" in
  exited)
    if [ "$exit_code" = "0" ]; then
      echo "Service $service_name completed successfully."
      return 0
    fi
    echo "error: service $service_name exited with code $exit_code" >&2
    docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" logs --tail=80 "$service_name" >&2 || true
    return 2
    ;;
  *)
    return 1
    ;;
  esac
}

wait_for_completed() {
  local service_name="$1"
  WAIT_FOR_COMPLETED_LAST_STATUS="unknown"

  if bounded_poll "$WAIT_TIMEOUT" 5 wait_for_completed_check "$service_name"; then
    return 0
  fi

  poll_rc=$?
  if [ "$poll_rc" -eq 124 ]; then
    echo "error: timed out waiting for $service_name to complete (last status: $WAIT_FOR_COMPLETED_LAST_STATUS)" >&2
    emit_ci_error "service completion timeout" "$service_name last status: $WAIT_FOR_COMPLETED_LAST_STATUS"
  else
    echo "error: failed waiting for $service_name to complete (poll result: $poll_rc, last status: $WAIT_FOR_COMPLETED_LAST_STATUS)" >&2
    emit_ci_error "service completion failure" "$service_name poll result: $poll_rc, last status: $WAIT_FOR_COMPLETED_LAST_STATUS"
  fi
  exit 1
}

curl_endpoint_check() {
  local label="$1"
  local url="$2"

  if curl -fsSL "$url" >/dev/null; then
    echo "$label endpoint is reachable."
    return 0
  fi

  return 1
}

wait_for_endpoint() {
  local label="$1"
  local url="$2"

  if bounded_poll "$WAIT_TIMEOUT" 5 curl_endpoint_check "$label" "$url"; then
    return 0
  fi

  echo "error: timed out waiting for $label endpoint at $url" >&2
  dump_compose_diagnostics
  exit 1
}

DEPENDENCY_SERVICES=(
  postgres
  redis
  srs
  srs-controller
  ome
  transcoder
  postgres-migrations
)

APPLICATION_CORE_SERVICES=(
  bitriver-live
)

APPLICATION_SIDECAR_SERVICES=(
  viewer
  transcoder-public
)

DEPENDENCY_SERVICES_WITH_HEALTHCHECKS=(
  postgres
  redis
  srs
  srs-controller
  ome
  transcoder
)

APPLICATION_SERVICES_WITH_HEALTHCHECKS=(
  bitriver-live
)

start_compose_services "dependency services" "${DEPENDENCY_SERVICES[@]}"

echo "Waiting for dependency services to report healthy..."
for service in "${DEPENDENCY_SERVICES_WITH_HEALTHCHECKS[@]}"; do
  wait_for_health "$service"
done
wait_for_completed "postgres-migrations"

start_compose_services "API service" "${APPLICATION_CORE_SERVICES[@]}"

echo "Waiting for API service to report healthy..."
for service in "${APPLICATION_SERVICES_WITH_HEALTHCHECKS[@]}"; do
  wait_for_health "$service"
done

start_compose_services_without_deps "viewer and public sidecars" "${APPLICATION_SIDECAR_SERVICES[@]}"

API_PORT=${BITRIVER_LIVE_PORT:-8080}
VIEWER_PATH=${NEXT_VIEWER_BASE_PATH:-/viewer}

echo "CURLing API and viewer endpoints..."
wait_for_endpoint "API health" "http://localhost:${API_PORT}/healthz"
wait_for_endpoint "viewer" "http://localhost:${API_PORT}${VIEWER_PATH}"

if [[ "${BITRIVER_TEST_GOLDEN_PATH:-}" == "1" ]]; then
  echo "Running production golden-path product assertions..."
  set +e
  "$SCRIPT_DIR/test-production-golden-path.sh" \
    --stack running \
    --artifact-dir "${BITRIVER_GOLDEN_PATH_ARTIFACT_DIR:-$REPO_ROOT/.artifacts/production-golden-path}" \
    --base-url "http://localhost:${API_PORT}" \
    --rtmp-base-url "$BITRIVER_SRS_PUBLIC_RTMP_BASE_URL" \
    --viewer-path "$VIEWER_PATH"
  golden_path_status=$?
  set -e
  if ((golden_path_status != 0)); then
    emit_ci_error \
      "production golden path failed" \
      "Inspect the sanitized production-golden-path.json report for the first failed stage."
    # Container state is safe to retain; raw media-service logs can include the
    # per-run stream key and therefore remain on the operator host by default.
    docker compose --env-file "$ENV_FILE" "${COMPOSE_RUNTIME_ARGS[@]}" ps -a >&2 || true
    echo "error: production golden-path assertions failed; raw service logs were not exported because they can contain the per-run stream key" >&2
    exit "$golden_path_status"
  fi
fi

echo "Quickstart compose smoke checks passed."
