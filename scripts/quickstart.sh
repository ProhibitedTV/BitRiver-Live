#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/quickstart.sh [quickstart flags...]

Thin wrapper around the Go BitRiver CLI quickstart flow.
It performs minimal local checks, then forwards all arguments to:
  go run ./cmd/bitriver quickstart ...

Examples:
  scripts/quickstart.sh
  scripts/quickstart.sh --env-file .env.prod --compose-file deploy/docker-compose.yml
USAGE
}

if (($# > 0)); then
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
  esac
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CODE_ROOT="${BITRIVER_QUICKSTART_REPO_ROOT:-$REPO_ROOT}"

if ! "$SCRIPT_DIR/check-go-toolchain.sh"; then
  echo "Install a supported toolchain from https://go.dev/dl/ and ensure it is available in your PATH." >&2
  exit 1
fi

if [[ ! -d "$CODE_ROOT/cmd/bitriver" ]]; then
  echo "Error: expected Go CLI sources at $CODE_ROOT/cmd/bitriver" >&2
  exit 1
fi

echo "Running BitRiver Live quickstart ..."
(cd "$CODE_ROOT" && GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver quickstart "$@")
