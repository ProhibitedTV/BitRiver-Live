#!/bin/sh
set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd -P)
assets_default="$script_dir/../share/bitriver-live"
assets_dir=${BITRIVER_LAUNCHER_ROOT:-$assets_default}
compose_file="$assets_dir/deploy/docker-compose.yml"
example_env="$assets_dir/deploy/.env.example"
env_file=${BITRIVER_ENV_FILE:-$assets_dir/deploy/.env}
binary_path=${BITRIVER_BINARY:-"$script_dir/bitriver"}

fatal() {
  printf '%s\n' "$1" >&2
  exit 1
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fatal "$1 is required but was not found in PATH"
  fi
}

ensure_assets() {
  if [ ! -f "$compose_file" ]; then
    fatal "docker-compose.yml not found at $compose_file; reinstall BitRiver Live installer"
  fi

  if [ ! -f "$env_file" ]; then
    if [ -f "$example_env" ]; then
      mkdir -p "$(dirname "$env_file")"
      cp "$example_env" "$env_file"
      printf 'Created default env file at %s. Please review secrets before continuing.\n' "$env_file"
    else
      fatal "No env file found and no example to copy from"
    fi
  fi
}

check_prereqs() {
  echo "Checking Docker and Compose prerequisites..."
  require_cmd docker
  if ! docker version >/dev/null 2>&1; then
    fatal "docker version failed; ensure Docker is running"
  fi

  if ! docker compose version >/dev/null 2>&1; then
    fatal "docker compose version failed; install Docker Compose plugin"
  fi
}

pull_images() {
  echo "Pulling BitRiver Live images..."
  docker compose -f "$compose_file" --env-file "$env_file" pull
}

bring_up() {
  echo "Starting BitRiver Live stack..."
  docker compose -f "$compose_file" --env-file "$env_file" up -d
}

main() {
  check_prereqs
  ensure_assets
  if [ -x "$binary_path" ]; then
    echo "Running bitriver doctor for sanity checks..."
    if ! "$binary_path" doctor; then
      echo "warning: bitriver doctor reported issues; continuing because Docker is available" >&2
    fi
  fi
  pull_images
  bring_up
  echo "BitRiver Live is starting. Use 'docker compose -f $compose_file logs -f' to follow logs."
}

main "$@"
