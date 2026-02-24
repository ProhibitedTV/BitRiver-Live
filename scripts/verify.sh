#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

usage() {
  cat <<'USAGE'
Usage: ./scripts/verify.sh [--viewer] [--ci-viewer] [--go-packages <pattern>]

Runs repository verification checks in a consistent order.

Prerequisites:
  - go
  - python3 (required by ./scripts/check-contract-invariants.sh)

Options:
  --viewer  Force viewer lint/test checks even when no viewer changes are detected.
  --ci-viewer  In CI, force viewer lint/test checks for non-viewer workflows.
  --go-packages  Optional Go package pattern for targeted tests (default: ./...).
  -h, --help  Show this help.
USAGE
}

force_viewer_checks=false
force_ci_viewer_checks=false
go_test_packages="./..."

while (($# > 0)); do
  case "$1" in
    --viewer)
      force_viewer_checks=true
      ;;
    --ci-viewer)
      force_ci_viewer_checks=true
      ;;
    --go-packages)
      shift
      if (($# == 0)); then
        echo "Missing value for --go-packages" >&2
        usage >&2
        exit 1
      fi
      go_test_packages="$1"
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

require_tool() {
  local tool_name="$1"
  local reason="$2"

  if command -v "$tool_name" >/dev/null 2>&1; then
    return 0
  fi

  echo
  echo "Missing required tool: $tool_name" >&2
  echo "$reason" >&2
  exit 1
}

viewer_changes_present() {
  git rev-parse --is-inside-work-tree >/dev/null 2>&1 || return 1

  local ci_base_ref="${GITHUB_BASE_REF:-${GITHUB_BASE_SHA:-${CI_MERGE_REQUEST_TARGET_BRANCH_SHA:-}}}"
  local ci_head_ref="${GITHUB_HEAD_REF:-${GITHUB_SHA:-${CI_COMMIT_SHA:-}}}"

  if [[ -n "$ci_base_ref" && -n "$ci_head_ref" ]]; then
    local changed_files
    if changed_files="$(git diff --name-only "$ci_base_ref" "$ci_head_ref" -- web/viewer 2>/dev/null)"; then
      [[ -n "$changed_files" ]]
      return
    fi
  fi

  local status_output
  status_output="$(git status --porcelain -- web/viewer)"
  [[ -n "$status_output" ]]
}

is_ci_environment() {
  case "${CI:-}" in
    1|true|TRUE)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

is_viewer_related_workflow() {
  local workflow_context="${GITHUB_WORKFLOW:-} ${GITHUB_WORKFLOW_REF:-}"
  [[ "$workflow_context" =~ [Vv]iewer ]]
}

viewer_checks_supported() {
  [[ -d web/viewer ]] || return 1
  command -v node >/dev/null 2>&1 || return 1
  command -v npm >/dev/null 2>&1 || return 1
}

should_run_viewer_checks() {
  if ! viewer_checks_supported; then
    return 1
  fi

  if is_ci_environment; then
    if is_viewer_related_workflow || [[ "$force_ci_viewer_checks" == true ]]; then
      return 0
    fi
    return 1
  fi

  if [[ "$force_viewer_checks" == true ]]; then
    return 0
  fi

  viewer_changes_present
}

if [[ "${BITRIVER_VERIFY_SOURCE_ONLY:-0}" == "1" ]]; then
  if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then
    return 0
  fi
  exit 0
fi

run_step "go.sum non-empty guard" ./scripts/check-go-sum-not-empty.sh
run_step "CI workflow contract check" ./scripts/check-ci-contract.sh

run_step "Go tests" \
  env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test "$go_test_packages" -count=1 -timeout=120s

run_step "Architecture dependency direction check" ./scripts/check-architecture-deps.sh
run_step "No internal/models imports outside internal/models" ./scripts/check-no-models-imports.sh
run_step "Dependency source check" ./scripts/check-dependency-source.sh
run_step "Contract invariants check" ./scripts/check-contract-invariants.sh
run_step "Production third-party digest gate" ./scripts/require-image-digests.sh

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
  if ! [[ -d web/viewer ]]; then
    echo "Skipping: web/viewer does not exist in this checkout."
  elif ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
    echo "Skipping: node and/or npm are not installed or not on PATH."
  elif is_ci_environment; then
    echo "Skipping: CI viewer checks run only for viewer-related workflows or with --ci-viewer."
  else
    echo "Skipping: no changes detected under web/viewer (use --viewer to force)."
  fi
fi

echo

echo "All requested verification checks completed successfully."
