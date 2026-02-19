#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

usage() {
  cat <<'USAGE'
Usage: ./scripts/verify.sh [--viewer]

Runs repository verification checks in a consistent order.

Options:
  --viewer  Force viewer lint/test checks even when no viewer changes are detected.
  -h, --help  Show this help.
USAGE
}

force_viewer_checks=false

while (($# > 0)); do
  case "$1" in
    --viewer)
      force_viewer_checks=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
  shift
done

run_step() {
  local label="$1"
  shift

  echo
  echo "==> $label"
  "$@"
}

viewer_changes_present() {
  git rev-parse --is-inside-work-tree >/dev/null 2>&1 || return 1

  local status_output
  status_output="$(git status --porcelain -- web/viewer)"
  [[ -n "$status_output" ]]
}

should_run_viewer_checks() {
  if [[ "$force_viewer_checks" == true ]]; then
    return 0
  fi

  viewer_changes_present
}

run_step "Go tests" \
  env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s

run_step "Architecture dependency direction check" ./scripts/check-architecture-deps.sh
run_step "No internal/models imports outside internal/models" ./scripts/check-no-models-imports.sh
run_step "Dependency source check" ./scripts/check-dependency-source.sh

if command -v docker >/dev/null 2>&1; then
  run_step "Docker Compose config validation" docker compose -f deploy/docker-compose.yml config
else
  echo
  echo "==> Docker Compose config validation"
  echo "Skipping: docker is not installed or not on PATH."
fi

if should_run_viewer_checks; then
  run_step "Viewer lint" npm --prefix web/viewer run lint
  run_step "Viewer tests" npm --prefix web/viewer run test
else
  echo
  echo "==> Viewer lint/test"
  echo "Skipping: no changes detected under web/viewer (use --viewer to force)."
fi

echo

echo "All requested verification checks completed successfully."
