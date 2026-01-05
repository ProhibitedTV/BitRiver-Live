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
read_env_value() {
  local env_var="$1"
  local raw_line

  raw_line=$(grep -E "^[[:space:]]*${env_var}=" "$ENV_FILE" | tail -n 1 || true)
  raw_line=${raw_line#*=}
  raw_line=${raw_line%""""}
  raw_line=${raw_line%\"}
  raw_line=${raw_line#\"}
  echo "$raw_line" | xargs
}

check_insecure_dsn() {
  local env_var="$1"
  local raw_line value lowered

  raw_line=$(grep -E "^[[:space:]]*${env_var}=" "$ENV_FILE" | tail -n 1 || true)
  value=$(echo "${raw_line#*=}" | xargs)
  if [ -z "$value" ]; then
    return
  fi

  lowered=$(echo "$value" | tr '[:upper:]' '[:lower:]')
  if [[ "$lowered" == *"sslmode=disable"* ]]; then
    echo "${env_var} must enable TLS in production (set sslmode=require or supply a CA for verify-full)" >&2
    exit 1
  fi
}

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

if [ "${mode_value,,}" = "production" ]; then
  check_insecure_dsn "BITRIVER_LIVE_POSTGRES_DSN"
  check_insecure_dsn "BITRIVER_LIVE_SESSION_POSTGRES_DSN"
fi

tls_cert=$(read_env_value "BITRIVER_LIVE_TLS_CERT")
tls_key=$(read_env_value "BITRIVER_LIVE_TLS_KEY")
api_url=$(read_env_value "NEXT_PUBLIC_API_BASE_URL")
viewer_url=$(read_env_value "NEXT_PUBLIC_VIEWER_URL")

https_requested=false
for val in "$api_url" "$viewer_url"; do
  lower=$(echo "$val" | tr '[:upper:]' '[:lower:]')
  if [[ "$lower" == https://* ]]; then
    https_requested=true
    break
  fi
done

if [ -n "$tls_cert" ] || [ -n "$tls_key" ]; then
  if [ -z "$tls_cert" ] || [ -z "$tls_key" ]; then
    echo "BITRIVER_LIVE_TLS_CERT and BITRIVER_LIVE_TLS_KEY must both be set to enable HTTPS" >&2
    exit 1
  fi
  if [ ! -r "$tls_cert" ]; then
    echo "BITRIVER_LIVE_TLS_CERT points at $tls_cert but the file is not readable" >&2
    exit 1
  fi
  if [ ! -r "$tls_key" ]; then
    echo "BITRIVER_LIVE_TLS_KEY points at $tls_key but the file is not readable" >&2
    exit 1
  fi
elif [ "$https_requested" = true ]; then
  echo "HTTPS URLs are configured for the API/viewer, but BITRIVER_LIVE_TLS_CERT/BITRIVER_LIVE_TLS_KEY are empty." >&2
  echo "Provide TLS files or terminate HTTPS upstream and update NEXT_PUBLIC_API_BASE_URL/NEXT_PUBLIC_VIEWER_URL accordingly." >&2
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
