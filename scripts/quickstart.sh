#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/quickstart.sh [-h|--help]

Runs the Go-based BitRiver Live CLI quickstart command to run doctor, initialize
the environment, render OME configuration, start Docker Compose, wait for the
API readiness probe, and seed the admin user. Override ENV_FILE or COMPOSE_FILE
to point at custom locations.

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
  shift
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

  local api_token access_token
  api_token="$(read_env_value BITRIVER_OME_API_TOKEN || true)"
  access_token="$(read_env_value BITRIVER_OME_ACCESS_TOKEN || true)"

  if [[ -z "$api_token" && -z "$access_token" ]]; then
    cat >&2 <<EOF
OME auth preflight failed: missing required token values.
Expected BITRIVER_OME_API_TOKEN=<non-empty token> in $ENV_FILE_PATH.
Optional override: BITRIVER_OME_ACCESS_TOKEN=<same token used by OME health probes>.
EOF
    return 1
  fi

  if [[ -z "$api_token" ]]; then
    cat >&2 <<EOF
OME auth preflight failed: BITRIVER_OME_API_TOKEN is empty in $ENV_FILE_PATH.
Expected BITRIVER_OME_API_TOKEN=<non-empty token> so OME render can populate <Managers><API><AccessToken>.
EOF
    return 1
  fi

  if [[ -n "$access_token" && "$access_token" != "$api_token" ]]; then
    cat >&2 <<EOF
OME auth preflight failed: token mismatch detected in $ENV_FILE_PATH.
BITRIVER_OME_ACCESS_TOKEN and BITRIVER_OME_API_TOKEN must match for quickstart health probes.
Set BITRIVER_OME_ACCESS_TOKEN=$api_token (or clear BITRIVER_OME_ACCESS_TOKEN to fall back to BITRIVER_OME_API_TOKEN).
EOF
    return 1
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
