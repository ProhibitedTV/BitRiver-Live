#!/bin/sh
set -eu

script_dir=$(CDPATH=; cd -- "$(dirname "$0")" && pwd -P)
assets_default="$script_dir/../share/bitriver-live"
assets_dir=${BITRIVER_LAUNCHER_ROOT:-$assets_default}
compose_file="$assets_dir/deploy/docker-compose.yml"
example_env="$assets_dir/deploy/.env.example"
env_file=${BITRIVER_ENV_FILE:-$assets_dir/.env}
binary_path=${BITRIVER_BINARY:-"$script_dir/bitriver"}
export BITRIVER_ROOT="$assets_dir"

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

  if [ -z "${BITRIVER_CONFIG_ROOT:-}" ]; then
    BITRIVER_CONFIG_ROOT=$(CDPATH=; cd -- "$(dirname "$env_file")" && pwd -P)
    export BITRIVER_CONFIG_ROOT
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

compose_cmd() {
  docker compose -f "$compose_file" --env-file "$env_file" "$@"
}

run_quickstart() {
  if [ ! -x "$binary_path" ]; then
    fatal "bitriver binary not found at $binary_path; reinstall or rebuild the launcher"
  fi

  echo "Starting BitRiver Live stack..."
  "$binary_path" quickstart --compose-file "$compose_file" --env-file "$env_file"
}

stop_stack() {
  echo "Stopping BitRiver Live stack..."
  compose_cmd stop
}

restart_stack() {
  echo "Restarting BitRiver Live stack..."
  compose_cmd restart
}

follow_logs() {
  echo "Tailing BitRiver Live logs..."
  compose_cmd logs -f
}

open_desktop() {
  if [ ! -x "$binary_path" ]; then
    fatal "bitriver binary not found at $binary_path; reinstall or rebuild the launcher"
  fi

  echo "Launching BitRiver Live control panel..."
  "$binary_path" desktop --compose-file "$compose_file" --env-file "$env_file"
}

usage() {
  cat <<EOF
Usage: $(basename "$0") [start|stop|restart|logs|ui]

Commands:
  start    Pull images and start the BitRiver Live stack (default)
  stop     Stop running containers without removing volumes
  restart  Restart running containers
  logs     Follow docker compose logs
  ui       Open the desktop control panel
EOF
}

main() {
  command=${1:-start}
  case "$command" in
    start)
      check_prereqs
      ensure_assets
      if [ -x "$binary_path" ]; then
        echo "Running bitriver doctor for sanity checks..."
        if ! "$binary_path" doctor; then
          echo "warning: bitriver doctor reported issues; continuing because Docker is available" >&2
        fi
      fi
      run_quickstart
      echo "BitRiver Live is starting. Use '$(basename "$0") logs' to follow logs or '$(basename "$0") ui' for the tray."
      ;;
    stop)
      check_prereqs
      ensure_assets
      stop_stack
      ;;
    restart)
      check_prereqs
      ensure_assets
      restart_stack
      ;;
    logs)
      check_prereqs
      ensure_assets
      follow_logs
      ;;
    ui)
      check_prereqs
      ensure_assets
      open_desktop
      ;;
    *)
      usage
      exit 1
      ;;
  esac
}

main "$@"
