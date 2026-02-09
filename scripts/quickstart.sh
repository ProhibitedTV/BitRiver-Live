#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/quickstart.sh [-h|--help]

Runs the Go-based BitRiver Live CLI quickstart command to run doctor, initialize
the environment, render OME configuration, start Docker Compose (including
waiting for dependency health checks such as srs-controller/transcoder), wait
for the API readiness probe, and seed the admin user. Override ENV_FILE or
COMPOSE_FILE to point at custom locations.

Options:
  -h, --help   Show this help message.
USAGE
}

while (($# > 0)); do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if ! command -v go >/dev/null 2>&1; then
  cat <<'NEEDGO'
Error: Go is required to run the BitRiver Live CLI.
Install Go 1.21+ from https://go.dev/dl/ and ensure it is available in your PATH.
NEEDGO
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CODE_ROOT="${BITRIVER_QUICKSTART_REPO_ROOT:-$REPO_ROOT}"
DEFAULT_ENV_FILE="$CODE_ROOT/.env"
DEFAULT_COMPOSE_FILE="$CODE_ROOT/deploy/docker-compose.yml"
ENV_FILE_PATH="${ENV_FILE:-$DEFAULT_ENV_FILE}"
COMPOSE_FILE_PATH="${COMPOSE_FILE:-$DEFAULT_COMPOSE_FILE}"
VERIFY_OME_TOKEN_SCRIPT="$CODE_ROOT/scripts/verify-ome-health-token.sh"

read_env_value() {
  local key="$1"
  awk -v key="$key" '
    /^[[:space:]]*#/ || /^[[:space:]]*$/ { next }
    {
      line = $0
      sub(/^[[:space:]]*export[[:space:]]+/, "", line)
      eq = index(line, "=")
      if (eq == 0) next
      name = substr(line, 1, eq - 1)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", name)
      if (name != key) next
      val = substr(line, eq + 1)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", val)
      if (val ~ /^".*"$/ || val ~ /^\047.*\047$/) {
        val = substr(val, 2, length(val) - 2)
      }
      out = val
    }
    END {
      if (out != "") {
        print out
      }
    }
  ' "$ENV_FILE_PATH"
}

run_ome_auth_preflight() {
  if [[ ! -f "$ENV_FILE_PATH" ]]; then
    echo "OME auth preflight failed: env file not found at $ENV_FILE_PATH" >&2
    return 1
  fi
  if [[ ! -x "$VERIFY_OME_TOKEN_SCRIPT" ]]; then
    echo "OME auth preflight failed: helper script is missing or not executable at $VERIFY_OME_TOKEN_SCRIPT" >&2
    return 1
  fi

  local auth_mode raw_auth_mode shell_auth_mode
  raw_auth_mode="$(read_env_value BITRIVER_OME_HEALTHCHECK_AUTH_MODE || true)"
  auth_mode="${raw_auth_mode,,}"
  if [[ -z "$auth_mode" ]]; then
    auth_mode="accesstoken"
  fi

  case "$auth_mode" in
    accesstoken|basic|none|off|disabled)
      ;;
    *)
      cat >&2 <<EOF
OME auth preflight failed: BITRIVER_OME_HEALTHCHECK_AUTH_MODE must be accesstoken, basic, or none/off/disabled (current: ${raw_auth_mode:-<empty>}).
Set BITRIVER_OME_HEALTHCHECK_AUTH_MODE=accesstoken for token probes, or:
  BITRIVER_OME_HEALTHCHECK_AUTH_MODE=basic
  BITRIVER_OME_USERNAME=ome-operator
  BITRIVER_OME_PASSWORD=replace-with-strong-password
in $ENV_FILE_PATH before running scripts/quickstart.sh.
EOF
      return 1
      ;;
  esac

  shell_auth_mode="${BITRIVER_OME_HEALTHCHECK_AUTH_MODE:-}"
  if [[ -n "$shell_auth_mode" && "${shell_auth_mode,,}" != "$auth_mode" ]]; then
    echo "OME auth preflight notice: shell BITRIVER_OME_HEALTHCHECK_AUTH_MODE=$shell_auth_mode differs from $ENV_FILE_PATH ($raw_auth_mode); using env-file value for validation." >&2
  fi

  local api_token access_token healthcheck_token
  api_token="$(read_env_value BITRIVER_OME_API_TOKEN || true)"
  access_token="$(read_env_value BITRIVER_OME_ACCESS_TOKEN || true)"
  healthcheck_token="$(read_env_value BITRIVER_OME_HEALTHCHECK_TOKEN || true)"

  if [[ -z "$api_token" ]]; then
    cat >&2 <<EOF
OME auth preflight failed: BITRIVER_OME_API_TOKEN is empty in $ENV_FILE_PATH.
Expected BITRIVER_OME_API_TOKEN=<non-empty token> so OME render can populate <Managers><API><AccessToken>.
EOF
    return 1
  fi

  if [[ "$auth_mode" == "basic" ]]; then
    local ome_username ome_password
    ome_username="$(read_env_value BITRIVER_OME_USERNAME || true)"
    ome_password="$(read_env_value BITRIVER_OME_PASSWORD || true)"
    if [[ -z "$ome_username" || -z "$ome_password" ]]; then
      cat >&2 <<EOF
OME auth preflight failed: BITRIVER_OME_HEALTHCHECK_AUTH_MODE=basic requires BITRIVER_OME_USERNAME and BITRIVER_OME_PASSWORD in $ENV_FILE_PATH.
Example:
  BITRIVER_OME_HEALTHCHECK_AUTH_MODE=basic
  BITRIVER_OME_USERNAME=ome-operator
  BITRIVER_OME_PASSWORD=replace-with-strong-password
EOF
      return 1
    fi
  elif [[ "$auth_mode" == "accesstoken" ]]; then
    if [[ -z "$healthcheck_token" && -z "$access_token" && -z "$api_token" ]]; then
      cat >&2 <<EOF
OME auth preflight failed: BITRIVER_OME_HEALTHCHECK_AUTH_MODE=accesstoken requires a non-empty token in canonical order:
  BITRIVER_OME_HEALTHCHECK_TOKEN -> BITRIVER_OME_ACCESS_TOKEN -> BITRIVER_OME_API_TOKEN
Example:
  BITRIVER_OME_HEALTHCHECK_AUTH_MODE=accesstoken
  BITRIVER_OME_API_TOKEN=replace-with-non-empty-token
  # Optional overrides:
  # BITRIVER_OME_ACCESS_TOKEN=replace-with-probe-token
  # BITRIVER_OME_HEALTHCHECK_TOKEN=replace-with-healthcheck-token
EOF
      return 1
    fi
  fi

  echo "Running OME auth preflight: rendering config and validating token consistency ..."
  run_cli ome render --force --env-file "$ENV_FILE_PATH"
  "$VERIFY_OME_TOKEN_SCRIPT" --env-file "$ENV_FILE_PATH" --config "$CODE_ROOT/deploy/ome/Server.generated.xml"
}

run_cli() {
  (cd "$CODE_ROOT" && GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver "$@")
}

quickstart_args=("--env-file" "$ENV_FILE_PATH" "--compose-file" "$COMPOSE_FILE_PATH")

echo "Running BitRiver Live quickstart ..."
run_ome_auth_preflight
run_cli quickstart "${quickstart_args[@]}"
