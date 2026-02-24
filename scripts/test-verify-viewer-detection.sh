#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
VERIFY_SCRIPT="$ROOT_DIR/scripts/verify.sh"

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT

create_repo() {
  local repo_dir="$1"
  mkdir -p "$repo_dir"
  git init -q "$repo_dir"
  git -C "$repo_dir" config user.name "BitRiver Test"
  git -C "$repo_dir" config user.email "bitriver-test@example.com"
  mkdir -p "$repo_dir/web/viewer" "$repo_dir/internal/api"
  echo "base" > "$repo_dir/web/viewer/app.tsx"
  echo "base" > "$repo_dir/internal/api/handler.go"
  git -C "$repo_dir" add .
  git -C "$repo_dir" commit -qm "base"
}

run_viewer_changes_present() {
  local repo_dir="$1"
  local base_sha="${2:-}"
  local head_sha="${3:-}"

  if [[ -n "$base_sha" && -n "$head_sha" ]]; then
    BITRIVER_VERIFY_SOURCE_ONLY=1 TEST_REPO="$repo_dir" GITHUB_BASE_SHA="$base_sha" GITHUB_SHA="$head_sha" \
      bash -c 'script_path="$1"; set --; source "$script_path"; cd "$TEST_REPO"; set +e; viewer_changes_present; rc=$?; set -e; exit "$rc"' _ "$VERIFY_SCRIPT"
  else
    BITRIVER_VERIFY_SOURCE_ONLY=1 TEST_REPO="$repo_dir" \
      bash -c 'script_path="$1"; set --; source "$script_path"; cd "$TEST_REPO"; set +e; viewer_changes_present; rc=$?; set -e; exit "$rc"' _ "$VERIFY_SCRIPT"
  fi
}

assert_detection() {
  local expected="$1"
  local description="$2"
  local repo_dir="$3"
  local base_sha="${4:-}"
  local head_sha="${5:-}"

  local rc=0
  set +e
  run_viewer_changes_present "$repo_dir" "$base_sha" "$head_sha"
  rc=$?
  set -e

  if [[ "$expected" == "present" && "$rc" -ne 0 ]]; then
    echo "FAIL: $description (expected changes)" >&2
    return 1
  fi

  if [[ "$expected" == "absent" && "$rc" -eq 0 ]]; then
    echo "FAIL: $description (expected no changes)" >&2
    return 1
  fi

  echo "PASS: $description"
}

assert_force_logic() {
  local description="$1"
  local command_snippet="$2"

  if BITRIVER_VERIFY_SOURCE_ONLY=1 TEST_REPO="$ROOT_DIR" \
    bash -c 'script_path="$1"; snippet="$2"; set --; source "$script_path"; cd "$TEST_REPO"; eval "$snippet"' _ "$VERIFY_SCRIPT" "$command_snippet"; then
    echo "PASS: $description"
  else
    echo "FAIL: $description" >&2
    return 1
  fi
}

repo_with_viewer_diff="$TMP_ROOT/repo-viewer-diff"
create_repo "$repo_with_viewer_diff"
base_sha="$(git -C "$repo_with_viewer_diff" rev-parse HEAD)"
echo "viewer change" >> "$repo_with_viewer_diff/web/viewer/app.tsx"
git -C "$repo_with_viewer_diff" add web/viewer/app.tsx
git -C "$repo_with_viewer_diff" commit -qm "viewer change"
head_sha="$(git -C "$repo_with_viewer_diff" rev-parse HEAD)"
assert_detection "present" "CI metadata detects viewer changes in base/head diff" "$repo_with_viewer_diff" "$base_sha" "$head_sha"

repo_without_viewer_diff="$TMP_ROOT/repo-no-viewer-diff"
create_repo "$repo_without_viewer_diff"
base_sha="$(git -C "$repo_without_viewer_diff" rev-parse HEAD)"
echo "api change" >> "$repo_without_viewer_diff/internal/api/handler.go"
git -C "$repo_without_viewer_diff" add internal/api/handler.go
git -C "$repo_without_viewer_diff" commit -qm "api change"
head_sha="$(git -C "$repo_without_viewer_diff" rev-parse HEAD)"
assert_detection "absent" "CI metadata skips viewer when diff has no viewer changes" "$repo_without_viewer_diff" "$base_sha" "$head_sha"

repo_with_local_status="$TMP_ROOT/repo-local-status"
create_repo "$repo_with_local_status"
echo "local viewer edit" >> "$repo_with_local_status/web/viewer/app.tsx"
assert_detection "present" "Local fallback detects uncommitted viewer changes" "$repo_with_local_status"

assert_force_logic "--viewer force path keeps viewer checks enabled" 'force_viewer_checks=true; if should_run_viewer_checks; then true; else false; fi'
assert_force_logic "--ci-viewer force path keeps CI viewer checks enabled" 'force_ci_viewer_checks=true; CI=true; GITHUB_WORKFLOW=api-ci; if should_run_viewer_checks; then true; else false; fi'

echo "viewer detection contract checks passed."
