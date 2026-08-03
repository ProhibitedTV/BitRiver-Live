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
  - python3, python, or py -3 (required by ./scripts/check-contract-invariants.sh)

Options:
  --viewer  Force viewer lint/test checks even when no viewer changes are detected.
  --ci-viewer  In CI, force viewer lint/test checks for non-viewer workflows.
  --go-packages  Optional single Go package pattern for targeted tests.
                 Default: ./cmd/... ./internal/... ./scripts/... ./web
  -h, --help  Show this help.
USAGE
}

force_viewer_checks=false
force_ci_viewer_checks=false
go_test_packages=("./cmd/..." "./internal/..." "./scripts/..." "./web")

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
      go_test_packages=("$1")
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

PYTHON_RUNNER=()

require_python_runner() {
  if python3 -c 'import sys' >/dev/null 2>&1; then
    PYTHON_RUNNER=(python3)
    return 0
  fi
  if py -3 -c 'import sys' >/dev/null 2>&1; then
    PYTHON_RUNNER=(py -3)
    return 0
  fi
  if python -c 'import sys' >/dev/null 2>&1; then
    PYTHON_RUNNER=(python)
    return 0
  fi

  echo
  echo "Missing required Python interpreter" >&2
  echo "Install python3, python, or the Windows py launcher to run repository contract checks." >&2
  exit 1
}

viewer_changes_present() {
  if ! command -v git >/dev/null 2>&1; then
    return 2
  fi

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

  local local_base_ref=""
  if git show-ref --verify --quiet refs/remotes/origin/main; then
    local_base_ref="$(git merge-base HEAD refs/remotes/origin/main 2>/dev/null || true)"
  fi

  if [[ -z "$local_base_ref" ]] && git rev-parse --verify --quiet HEAD~1 >/dev/null 2>&1; then
    local_base_ref="HEAD~1"
  fi

  if [[ -n "$local_base_ref" ]]; then
    local local_changed_files
    if local_changed_files="$(git diff --name-only "$local_base_ref" HEAD -- web/viewer 2>/dev/null)"; then
      if [[ -n "$local_changed_files" ]]; then
        return 0
      fi
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

redact_sensitive_output() {
  sed -E \
    -e 's/^([[:space:]]*[A-Za-z0-9_]*(PASSWORD|TOKEN|SECRET|DSN|CREDENTIAL)[A-Za-z0-9_]*:[[:space:]]*).*/\1<redacted>/g' \
    -e 's#(postgres://[^:[:space:]]+:)[^@[:space:]]+(@)#\1<redacted>\2#g'
}

run_docker_compose_config_validation() {
  local compose_env_file=".env"
  local created_temp_env_file=false
  local compose_output
  local compose_rc

  if [[ ! -f "$compose_env_file" ]]; then
    if [[ ! -f deploy/.env.example ]]; then
      echo "Missing env file: expected .env or deploy/.env.example" >&2
      return 1
    fi

    echo "Root .env missing; using temporary copy of deploy/.env.example for compose config validation"
    cp deploy/.env.example "$compose_env_file"
    created_temp_env_file=true
  fi

  compose_output="$(mktemp)"

  set +e
  docker compose --env-file "$compose_env_file" -f deploy/docker-compose.yml config >"$compose_output" 2>&1
  compose_rc=$?
  set -e

  if [[ "$created_temp_env_file" == true ]]; then
    rm -f "$compose_env_file"
  fi

  if [[ "$compose_rc" -ne 0 ]]; then
    echo "Docker Compose config validation failed. Redacted output:" >&2
    redact_sensitive_output <"$compose_output" >&2
    rm -f "$compose_output"
    return "$compose_rc"
  fi

  rm -f "$compose_output"
  echo "Docker Compose config rendered successfully."
  return "$compose_rc"
}

if [[ "${BITRIVER_VERIFY_SOURCE_ONLY:-0}" == "1" ]]; then
  if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then
    return 0
  fi
  exit 0
fi

run_step "go.sum non-empty guard" ./scripts/check-go-sum-not-empty.sh
run_step "CI workflow contract check" ./scripts/check-ci-contract.sh
run_step "CI merge gate tests" bash ./scripts/test-ci-merge-gate.sh
run_step "Repository hygiene guard tests" bash ./scripts/test-repository-hygiene.sh
run_step "Repository hygiene guard" ./scripts/check-repository-hygiene.sh
run_step "Env example placeholder hygiene" ./scripts/check-env-example-placeholders.sh
run_step "Release bundle contents" bash ./scripts/test-release-bundle.sh
if [[ "$(uname -s)" == "Linux" ]]; then
  run_step "Compose host installer lifecycle" bash ./scripts/test-compose-host-installer.sh
else
  echo
  echo "==> Compose host installer lifecycle"
  echo "Skipping: Linux filesystem ownership and symlink semantics are required."
fi

require_tool "go" "Install Go to run repository Go tests."
verify_go_flags=${GOFLAGS:-}
verify_go_flags="${verify_go_flags:+$verify_go_flags }-buildvcs=false"
run_step "Go tests" \
  env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS="$verify_go_flags" \
  go test "${go_test_packages[@]}" -count=1 -timeout=120s

run_step "Architecture dependency direction check" ./scripts/check-architecture-deps.sh
run_step "No internal/models imports outside internal/models" ./scripts/check-no-models-imports.sh
run_step "Dependency source check" ./scripts/check-dependency-source.sh
require_python_runner
run_step "Markdown local-link checker tests" "${PYTHON_RUNNER[@]}" -m unittest scripts/check_doc_links_test.py
run_step "Markdown local-link check" "${PYTHON_RUNNER[@]}" scripts/check_doc_links.py
run_step "Contract invariants check" ./scripts/check-contract-invariants.sh
run_step "Production third-party digest gate" ./scripts/require-image-digests.sh --env-file .env

docker_available=false
if command -v docker >/dev/null 2>&1; then
  docker_available=true
fi

if [[ "$docker_available" == true ]]; then
  run_step "Postgres migration lifecycle" ./scripts/test-postgres-migrations.sh
  run_step "Docker Compose config validation" run_docker_compose_config_validation
  run_step "Quickstart smoke" ./scripts/test-quickstart.sh
else
  echo
  echo "==> Docker Compose config validation"
  echo "Skipping: docker is not installed or not on PATH."
  echo
  echo "==> Quickstart smoke"
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
    if command -v git >/dev/null 2>&1; then
      echo "Skipping: no changes detected under web/viewer (use --viewer to force)."
    else
      echo "Skipping: git is unavailable, so local viewer-change detection cannot run (use --viewer to force)."
    fi
  fi
fi

echo

echo "All requested verification checks completed successfully."
