#!/usr/bin/env bash
set -Eeuo pipefail

failures=()
rows=()

add_failure() {
  failures+=("$1")
}

is_true() {
  [[ "$1" == "true" ]]
}

require_boolean() {
  local name="$1"
  local value="$2"
  if [[ "$value" != "true" && "$value" != "false" ]]; then
    add_failure "$name expectation is ${value:-missing}; expected true or false"
  fi
}

check_result() {
  local label="$1"
  local expected="$2"
  local result="$3"
  local verdict

  if is_true "$expected"; then
    if [[ "$result" == "success" ]]; then
      verdict="pass"
    else
      verdict="FAIL: required check did not succeed"
      add_failure "$label was required but result was ${result:-missing}"
    fi
  else
    case "$result" in
      skipped)
        verdict="expected skip"
        ;;
      success)
        verdict="extra pass"
        ;;
      *)
        verdict="FAIL: unexpected non-success"
        add_failure "$label was not required but result was ${result:-missing}"
        ;;
    esac
  fi

  rows+=("| $label | $expected | ${result:-missing} | $verdict |")
}

verify_changed="${VERIFY_CHANGED:-}"
go_changed="${GO_CHANGED:-}"
deploy_changed="${DEPLOY_CHANGED:-}"
viewer_changed="${VIEWER_CHANGED:-}"
monitoring_changed="${MONITORING_CHANGED:-}"
docs_changed="${DOCS_CHANGED:-}"
shell_changed="${SHELL_CHANGED:-}"
image_scan_changed="${IMAGE_SCAN_CHANGED:-}"
quickstart_changed="${QUICKSTART_CHANGED:-}"
wizard_release_changed="${WIZARD_RELEASE_CHANGED:-}"
go_workflow_changed="${GO_WORKFLOW_CHANGED:-}"
scorecard_required="${SCORECARD_REQUIRED:-false}"

for pair in \
  "VERIFY_CHANGED:$verify_changed" \
  "GO_CHANGED:$go_changed" \
  "DEPLOY_CHANGED:$deploy_changed" \
  "VIEWER_CHANGED:$viewer_changed" \
  "MONITORING_CHANGED:$monitoring_changed" \
  "DOCS_CHANGED:$docs_changed" \
  "SHELL_CHANGED:$shell_changed" \
  "IMAGE_SCAN_CHANGED:$image_scan_changed" \
  "QUICKSTART_CHANGED:$quickstart_changed" \
  "WIZARD_RELEASE_CHANGED:$wizard_release_changed" \
  "GO_WORKFLOW_CHANGED:$go_workflow_changed" \
  "SCORECARD_REQUIRED:$scorecard_required"; do
  require_boolean "${pair%%:*}" "${pair#*:}"
done

go_verify_expected="false"
if is_true "$go_changed" || is_true "$deploy_changed"; then
  go_verify_expected="true"
fi

check_result "Committed secret file guard" "true" "${SECRET_GUARD_RESULT:-}"
check_result "Detect changed files" "true" "${CHANGED_FILES_RESULT:-}"
check_result "Ubuntu test-all gate" "$verify_changed" "${VERIFY_RESULT:-}"
check_result "Go tests (macOS/Windows)" "$go_verify_expected" "${GO_VERIFY_RESULT:-}"
check_result "Quickstart entrypoints" "$quickstart_changed" "${QUICKSTART_RESULT:-}"
check_result "Viewer CI" "$viewer_changed" "${VIEWER_RESULT:-}"
check_result "ShellCheck" "$shell_changed" "${SHELLCHECK_RESULT:-}"
check_result "Docs consistency" "$docs_changed" "${DOCS_RESULT:-}"
check_result "Monitoring config" "$monitoring_changed" "${MONITORING_RESULT:-}"
check_result "Go workflow consistency" "$go_workflow_changed" "${GO_WORKFLOW_RESULT:-}"
check_result "Wizard release" "$wizard_release_changed" "${WIZARD_RESULT:-}"
check_result "Image scan" "$image_scan_changed" "${IMAGE_SCAN_RESULT:-}"
check_result "PR release scorecard" "$scorecard_required" "${SCORECARD_RESULT:-skipped}"

summary="# Merge gate summary

| Check | Required | Result | Verdict |
| --- | --- | --- | --- |
$(printf '%s\n' "${rows[@]}")"

if ((${#failures[@]} == 0)); then
  summary+=$'\n\n**Result: PASS**'
else
  summary+=$'\n\n**Result: FAIL**\n'
  for failure in "${failures[@]}"; do
    summary+="\n- $failure"
  done
fi

printf '%s\n' "$summary"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  printf '%s\n' "$summary" >>"$GITHUB_STEP_SUMMARY"
fi
if [[ -n "${MERGE_GATE_REPORT:-}" ]]; then
  printf '%s\n' "$summary" >"$MERGE_GATE_REPORT"
fi

if ((${#failures[@]} > 0)); then
  exit 1
fi
