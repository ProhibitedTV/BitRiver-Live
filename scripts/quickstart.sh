#!/usr/bin/env bash
set -euo pipefail

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: $1 is required but was not found in PATH." >&2
    return 1
  fi
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$REPO_ROOT/.env"
COMPOSE_FILE="$REPO_ROOT/deploy/docker-compose.yml"

require_command docker || exit 1
require_command curl || exit 1

if ! docker compose version >/dev/null 2>&1; then
  echo "Error: Docker Compose V2 is required. Install the docker compose plugin for Docker and try again." >&2
  exit 1
fi

if ! docker_info_output=$(docker info 2>&1); then
  status=$?
  echo "$docker_info_output" >&2
  if printf '%s' "$docker_info_output" | grep -qi "permission denied"; then
    cat <<'EOF' >&2

Hint: Add your account to the docker group so you can talk to the daemon without sudo:

  sudo usermod -aG docker $USER
  newgrp docker  # or log out and back in

You can rerun this script with sudo ./scripts/quickstart.sh, but that will create root-owned files like .env, so fixing the group membership first is strongly recommended.
EOF
  fi
  exit "$status"
fi

echo "Docker and Docker Compose detected."

declare -A env_defaults=(
  [BITRIVER_LIVE_PORT]='8080'
  [BITRIVER_LIVE_STORAGE_DRIVER]='postgres'
  [BITRIVER_LIVE_MODE]='production'
  [BITRIVER_LIVE_ADDR]=':8080'
  [BITRIVER_LIVE_POSTGRES_DSN]='postgres://bitriver:bitriver@postgres:5432/bitriver?sslmode=disable'
  [BITRIVER_LIVE_POSTGRES_MAX_CONNS]='15'
  [BITRIVER_LIVE_POSTGRES_MIN_CONNS]='5'
  [BITRIVER_LIVE_POSTGRES_ACQUIRE_TIMEOUT]='5s'
  [BITRIVER_LIVE_POSTGRES_MAX_CONN_LIFETIME]='30m'
  [BITRIVER_POSTGRES_PORT]='5432'
  [BITRIVER_VIEWER_ORIGIN]='http://viewer:3000'
  [BITRIVER_REDIS_PORT]='6379'
  [BITRIVER_SRS_API]='http://srs:1985'
  [BITRIVER_SRS_TOKEN]='local-dev-token'
  [BITRIVER_SRS_API_PORT]='1985'
  [BITRIVER_SRS_RTMP_PORT]='1935'
  [BITRIVER_OME_API]='http://ome:8081'
  [BITRIVER_OME_USERNAME]='admin'
  [BITRIVER_OME_PASSWORD]='local-dev-password'
  [BITRIVER_OME_HTTP_PORT]='8081'
  [BITRIVER_OME_SIGNALLING_PORT]='9000'
  [BITRIVER_TRANSCODER_API]='http://transcoder:9000'
  [BITRIVER_TRANSCODER_TOKEN]='local-dev-token'
  [BITRIVER_TRANSCODER_HOST_PORT]='9001'
  [BITRIVER_INGEST_HEALTH]='/healthz'
  [NEXT_PUBLIC_API_BASE_URL]=''
  [NEXT_VIEWER_BASE_PATH]='/viewer'
  [NEXT_PUBLIC_VIEWER_URL]='http://localhost:8080/viewer'
  [BITRIVER_LIVE_ADMIN_EMAIL]='admin@example.com'
  [BITRIVER_LIVE_ADMIN_PASSWORD]='change-me-now'
)

read_env_value() {
  local key=$1
  local value=""
  if [[ -f "$ENV_FILE" ]]; then
    value=$(grep -E "^${key}=" "$ENV_FILE" | tail -n1 | cut -d= -f2-)
  fi
  if [[ -z $value ]]; then
    value="${env_defaults[$key]:-}"
  fi
  printf '%s' "$value"
}

wait_for_api() {
  local url=$1
  local attempts=${2:-60}
  local sleep_seconds=${3:-2}
  echo "Waiting for BitRiver Live API at $url ..."
  for ((i=1; i<=attempts; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      echo "API is reachable."
      return 0
    fi
    sleep "$sleep_seconds"
  done
  echo "Timed out waiting for API readiness after $((attempts * sleep_seconds)) seconds." >&2
  return 1
}

bootstrap_admin() {
  local storage_driver=$(read_env_value BITRIVER_LIVE_STORAGE_DRIVER)
  storage_driver=${storage_driver:-postgres}
  local email=$(read_env_value BITRIVER_LIVE_ADMIN_EMAIL)
  local password=$(read_env_value BITRIVER_LIVE_ADMIN_PASSWORD)
  if [[ -z $email || -z $password ]]; then
    echo "Skipping admin bootstrap (email or password missing)."
    return 1
  fi

  local display_name="Administrator"
  local name_flag=(--name "$display_name")

  if [[ ${storage_driver,,} == "postgres" ]]; then
    local container_dsn="postgres://bitriver:bitriver@postgres:5432/bitriver?sslmode=disable"
    local host_port=$(read_env_value BITRIVER_POSTGRES_PORT)
    host_port=${host_port:-5432}
    local host_dsn="postgres://bitriver:bitriver@localhost:${host_port}/bitriver?sslmode=disable"
    if docker compose exec -T bitriver-live /app/bootstrap-admin --postgres-dsn "$container_dsn" --email "$email" --password "$password" "${name_flag[@]}" >/dev/null; then
      return 0
    fi
    if command -v go >/dev/null 2>&1; then
      if go run ./cmd/tools/bootstrap-admin --postgres-dsn "$host_dsn" --email "$email" --password "$password" "${name_flag[@]}" >/dev/null; then
        return 0
      fi
    fi
    echo "Failed to run bootstrap helper automatically. Use the following command after ensuring the API is running:" >&2
    echo "  docker compose exec bitriver-live /app/bootstrap-admin --postgres-dsn '$container_dsn' --email '$email' --password '$password' --name '$display_name'" >&2
    return 2
  fi

  local data_path=$(read_env_value BITRIVER_LIVE_DATA)
  if [[ -z $data_path ]]; then
    echo "JSON datastore path not configured; unable to bootstrap admin automatically." >&2
    return 2
  fi
  if docker compose exec -T bitriver-live /app/bootstrap-admin --json "$data_path" --email "$email" --password "$password" "${name_flag[@]}" >/dev/null; then
    return 0
  fi
  if command -v go >/dev/null 2>&1; then
    if go run ./cmd/tools/bootstrap-admin --json "$data_path" --email "$email" --password "$password" "${name_flag[@]}" >/dev/null; then
      return 0
    fi
  fi
  echo "Failed to run bootstrap helper automatically. Configure the admin account manually." >&2
  return 2
}

if [ -f "$ENV_FILE" ]; then
  echo "Existing .env file detected at $ENV_FILE. Skipping regeneration."
else
  {
    printf '# Generated by scripts/quickstart.sh on %s\n' "$(date -u +'%%Y-%%m-%%dT%%H:%%M:%%SZ')"
    echo "# Update the admin email, password, and viewer URL before inviting real users."
    cat <<'EOF'
BITRIVER_INGEST_HEALTH=/healthz
BITRIVER_LIVE_ADDR=:8080
BITRIVER_LIVE_ADMIN_EMAIL=admin@example.com
BITRIVER_LIVE_ADMIN_PASSWORD=change-me-now
BITRIVER_LIVE_MODE=production
BITRIVER_LIVE_PORT=8080
BITRIVER_LIVE_POSTGRES_ACQUIRE_TIMEOUT=5s
BITRIVER_LIVE_POSTGRES_DSN=postgres://bitriver:bitriver@postgres:5432/bitriver?sslmode=disable
BITRIVER_LIVE_POSTGRES_MAX_CONN_LIFETIME=30m
BITRIVER_LIVE_POSTGRES_MAX_CONNS=15
BITRIVER_LIVE_POSTGRES_MIN_CONNS=5
BITRIVER_LIVE_STORAGE_DRIVER=postgres
BITRIVER_OME_API=http://ome:8081
BITRIVER_OME_HTTP_PORT=8081
BITRIVER_OME_PASSWORD=local-dev-password
BITRIVER_OME_SIGNALLING_PORT=9000
BITRIVER_OME_USERNAME=admin
BITRIVER_POSTGRES_PORT=5432
BITRIVER_REDIS_PORT=6379
BITRIVER_SRS_API=http://srs:1985
BITRIVER_SRS_API_PORT=1985
BITRIVER_SRS_RTMP_PORT=1935
BITRIVER_SRS_TOKEN=local-dev-token
BITRIVER_TRANSCODER_API=http://transcoder:9000
BITRIVER_TRANSCODER_HOST_PORT=9001
BITRIVER_TRANSCODER_TOKEN=local-dev-token
BITRIVER_VIEWER_ORIGIN=http://viewer:3000
NEXT_PUBLIC_API_BASE_URL=
NEXT_PUBLIC_VIEWER_URL=http://localhost:8080/viewer
NEXT_VIEWER_BASE_PATH=/viewer
EOF
  } > "$ENV_FILE"
  echo "Wrote default environment configuration to $ENV_FILE."
fi

cd "$REPO_ROOT"
export COMPOSE_FILE="$COMPOSE_FILE"

echo "Starting BitRiver Live stack..."
docker compose up -d

echo "Stack is starting. Use 'docker compose logs -f' to follow service output."

API_PORT=$(read_env_value BITRIVER_LIVE_PORT)
API_PORT=${API_PORT:-8080}
API_HEALTH_URL="http://localhost:${API_PORT}/healthz"
if wait_for_api "$API_HEALTH_URL"; then
  if bootstrap_admin; then
    viewer_url=$(read_env_value NEXT_PUBLIC_VIEWER_URL)
    echo ""
    echo "Administrator credentials:"
    echo "  Email:    $(read_env_value BITRIVER_LIVE_ADMIN_EMAIL)"
    echo "  Password: $(read_env_value BITRIVER_LIVE_ADMIN_PASSWORD)"
    if [[ -n $viewer_url ]]; then
      echo "Log in via $viewer_url (or the mapped host) and change the password immediately."
    else
      echo "Log in through the control center and change the password immediately."
    fi
  else
    echo "Administrator bootstrap requires manual follow-up." >&2
  fi
else
  echo "API did not become ready in time; skipping admin bootstrap." >&2
fi
