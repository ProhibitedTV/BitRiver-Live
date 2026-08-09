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

selected_env_file="$CODE_ROOT/.env"
quickstart_args=("$@")
for ((i = 0; i < ${#quickstart_args[@]}; i++)); do
  case "${quickstart_args[$i]}" in
    --env-file)
      if ((i + 1 < ${#quickstart_args[@]})); then
        selected_env_file="${quickstart_args[$((i + 1))]}"
      fi
      ;;
    --env-file=*)
      selected_env_file="${quickstart_args[$i]#--env-file=}"
      ;;
  esac
done
if [[ "$selected_env_file" != /* ]]; then
  selected_env_file="$CODE_ROOT/$selected_env_file"
fi

env_file_declares_host_identity() {
  local env_file="$1"
  local line value

  [[ -f "$env_file" ]] || return 1
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%$'\r'}"
    if [[ "$line" =~ ^[[:space:]]*(export[[:space:]]+)?(BITRIVER_HOST_UID|BITRIVER_HOST_GID)[[:space:]]*=(.*)$ ]]; then
      value="${BASH_REMATCH[3]}"
      value="${value#"${value%%[![:space:]]*}"}"
      value="${value%"${value##*[![:space:]]}"}"
      if [[ -n "$value" ]]; then
        return 0
      fi
    fi
  done <"$env_file"
  return 1
}

case "$(uname -s 2>/dev/null || true)" in
  Linux|Darwin)
    if [[ -z "${BITRIVER_HOST_UID:-}" && -z "${BITRIVER_HOST_GID:-}" ]] &&
      ! env_file_declares_host_identity "$selected_env_file"; then
      BITRIVER_HOST_UID="$(id -u)"
      BITRIVER_HOST_GID="$(id -g)"
      export BITRIVER_HOST_UID BITRIVER_HOST_GID
    fi
    ;;
esac

echo "Running BitRiver Live quickstart ..."
(cd "$CODE_ROOT" && GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver quickstart "$@")
