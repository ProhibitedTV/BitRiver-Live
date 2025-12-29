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

run_cli() {
  (cd "$CODE_ROOT" && GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver "$@")
}

quickstart_args=("--env-file" "$ENV_FILE_PATH" "--compose-file" "$COMPOSE_FILE_PATH")

echo "Running BitRiver Live quickstart ..."
run_cli quickstart "${quickstart_args[@]}"
