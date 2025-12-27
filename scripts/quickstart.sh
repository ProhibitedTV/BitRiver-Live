#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/quickstart.sh [-h|--help]

Runs the Go-based BitRiver Live CLI to initialize the environment, render OME
configuration, and start Docker Compose. Override ENV_FILE or COMPOSE_FILE to
point at custom locations.

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
ENV_FILE_PATH="${ENV_FILE:-$DEFAULT_ENV_FILE}"
COMPOSE_FILE_PATH="${COMPOSE_FILE:-$CODE_ROOT/deploy/docker-compose.yml}"

reconcile_env_file() {
  local template="$CODE_ROOT/deploy/.env.example"

  ensure_kv() {
    local key="$1" value="$2"

    if grep -q "^${key}=" "$ENV_FILE_PATH"; then
      sed -i "s/^${key}=.*/${key}=${value}/" "$ENV_FILE_PATH"
    else
      echo "${key}=${value}" >>"$ENV_FILE_PATH"
    fi
  }

  if [[ ! -f "$ENV_FILE_PATH" ]]; then
    echo "Environment file missing at $ENV_FILE_PATH" >&2
    exit 1
  fi

  if [[ ! -f "$template" ]]; then
    echo "Template missing at $template" >&2
    exit 1
  fi

  while IFS= read -r line; do
    if [[ -z "$line" || "${line:0:1}" == "#" ]]; then
      continue
    fi

    local key=${line%%=*}
    if ! grep -q "^${key}=" "$ENV_FILE_PATH"; then
      echo "$line" >>"$ENV_FILE_PATH"
    fi
  done <"$template"

  ensure_kv "BITRIVER_LIVE_IMAGE_TAG" "latest"
  ensure_kv "BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD" "bitriver"
}

run_cli() {
  (cd "$CODE_ROOT" && GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver "$@")
}

echo "Initializing environment file via Go CLI ..."
run_cli env init --env-file "$ENV_FILE_PATH"
reconcile_env_file

echo "Rendering OME configuration ..."
run_cli ome render --env-file "$ENV_FILE_PATH"

echo "Starting Docker Compose ..."
run_cli compose --file "$COMPOSE_FILE_PATH" up
