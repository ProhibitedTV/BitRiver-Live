#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="${ENV_FILE:-$REPO_ROOT/.env}"

usage() {
  cat <<'USAGE'
Usage: scripts/render-ome-config.sh [--check] [--force] [--env-file PATH] [--quiet]

Canonical OME AccessToken precedence during render/validation/healthcheck:
  BITRIVER_OME_HEALTHCHECK_TOKEN -> BITRIVER_OME_ACCESS_TOKEN -> BITRIVER_OME_API_TOKEN

Options:
  --check       Only verify that deploy/ome/Server.generated.xml exists.
  --force       Re-render even if the generated file already exists.
  --env-file    Path to the .env file to source (defaults to ./../.env).
  --quiet       Suppress informational output.
USAGE
}

args=()
env_file_set=0

while (($# > 0)); do
  case "$1" in
    --check|--force|--quiet)
      args+=("$1")
      ;;
    --env-file)
      shift
      ENV_FILE="${1:-}"
      if [[ -z "$ENV_FILE" ]]; then
        echo "--env-file requires a path" >&2
        exit 1
      fi
      args+=("--env-file" "$ENV_FILE")
      env_file_set=1
      ;;
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

if [[ $env_file_set -eq 0 ]]; then
  args+=("--env-file" "$ENV_FILE")
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Go is required to render OvenMediaEngine configuration." >&2
  exit 1
fi

(
  cd "$REPO_ROOT" &&
  GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver ome render "${args[@]}"
)
