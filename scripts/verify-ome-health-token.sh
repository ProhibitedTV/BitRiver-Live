#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH=; cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH=; cd -- "$SCRIPT_DIR/.." && pwd)
ENV_FILE="${ENV_FILE:-$REPO_ROOT/.env}"
CONFIG_FILE="${CONFIG_FILE:-$REPO_ROOT/deploy/ome/Server.generated.xml}"

usage() {
  cat <<'USAGE'
Usage: scripts/verify-ome-health-token.sh [--env-file PATH] [--config PATH]

Checks that deploy/ome/Server.generated.xml contains a non-empty
<Managers><API><AccessToken> value and that it matches the runtime
health token source resolved with canonical precedence:
BITRIVER_OME_HEALTHCHECK_TOKEN -> BITRIVER_OME_ACCESS_TOKEN -> BITRIVER_OME_API_TOKEN
from the same .env file Docker Compose uses.
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --env-file)
      shift
      ENV_FILE="${1:-}"
      [ -n "$ENV_FILE" ] || {
        echo "--env-file requires a path" >&2
        exit 1
      }
      ;;
    --config)
      shift
      CONFIG_FILE="${1:-}"
      [ -n "$CONFIG_FILE" ] || {
        echo "--config requires a path" >&2
        exit 1
      }
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
  shift
done

(
  cd "$REPO_ROOT"
  GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off \
    go run ./cmd/bitriver ome verify-health-token --env-file "$ENV_FILE" --config "$CONFIG_FILE"
)
