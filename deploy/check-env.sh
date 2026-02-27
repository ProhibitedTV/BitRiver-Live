#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

SKIP_DOCTOR=0
ENV_FILE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-doctor)
      SKIP_DOCTOR=1
      shift
      ;;
    -h|--help)
      cat <<USAGE
Usage: deploy/check-env.sh [--skip-doctor] [ENV_FILE]

Runs preflight doctor checks (unless --skip-doctor) and then validates the env file.
USAGE
      exit 0
      ;;
    *)
      if [[ -n "$ENV_FILE" ]]; then
        echo "error: unexpected extra argument: $1" >&2
        exit 1
      fi
      ENV_FILE="$1"
      shift
      ;;
  esac
done

if [[ -z "$ENV_FILE" ]]; then
  ENV_FILE="$REPO_ROOT/.env"
fi

if [[ "$SKIP_DOCTOR" -eq 0 ]]; then
  echo "Running BitRiver doctor preflight..."
  if ! (
    cd "$REPO_ROOT" &&
    GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver doctor --env-file "$ENV_FILE"
  ); then
    cat >&2 <<EOFMSG
error: preflight doctor checks failed.

What to do next:
  1) Run: GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver doctor --env-file "$ENV_FILE"
  2) Fix each FAIL item and rerun deploy/check-env.sh.
  3) Advanced override: deploy/check-env.sh --skip-doctor "$ENV_FILE"
EOFMSG
    exit 1
  fi
fi

(
  cd "$REPO_ROOT" &&
  GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver env validate --env-file "$ENV_FILE"
)
