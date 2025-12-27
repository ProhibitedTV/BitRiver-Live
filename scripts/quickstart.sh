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
DEFAULT_ENV_FILE="$REPO_ROOT/.env"
ENV_FILE_PATH="${ENV_FILE:-$DEFAULT_ENV_FILE}"
COMPOSE_FILE_PATH="${COMPOSE_FILE:-$REPO_ROOT/deploy/docker-compose.yml}"

run_cli() {
  (cd "$REPO_ROOT" && GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver "$@")
}

echo "Initializing environment file via Go CLI ..."
run_cli env init

if [[ "$ENV_FILE_PATH" != "$DEFAULT_ENV_FILE" ]]; then
  mkdir -p "$(dirname "$ENV_FILE_PATH")"
  cp "$DEFAULT_ENV_FILE" "$ENV_FILE_PATH"
  echo "Copied generated .env to $ENV_FILE_PATH"
fi

echo "Rendering OME configuration ..."
run_cli ome render

echo "Starting Docker Compose ..."
run_cli compose --file "$COMPOSE_FILE_PATH" up
