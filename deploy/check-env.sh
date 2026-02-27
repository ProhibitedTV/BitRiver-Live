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
Usage:
  deploy/check-env.sh [ENV_FILE] [--skip-doctor]
  deploy/check-env.sh --skip-doctor [ENV_FILE]

Runs the enhanced doctor preflight first (unless --skip-doctor), then runs env validation.
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
  echo "Running doctor..."
  if ! (
    cd "$REPO_ROOT" &&
    GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver doctor --env-file "$ENV_FILE" --compose-file "$REPO_ROOT/deploy/docker-compose.yml"
  ); then
    cat >&2 <<EOFMSG
error: doctor preflight reported blocking failures.

Next steps:
  1) Re-run doctor directly:
     GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver doctor --env-file "$ENV_FILE" --compose-file "$REPO_ROOT/deploy/docker-compose.yml"
  2) Resolve each FAIL item in the doctor output.
  3) Re-run: bash deploy/check-env.sh
  4) Temporary bypass (not recommended for production): bash deploy/check-env.sh "$ENV_FILE" --skip-doctor
EOFMSG
    exit 1
  fi
fi

echo "Running env validation..."
(
  cd "$REPO_ROOT" &&
  GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver env validate --env-file "$ENV_FILE"
)
