#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
GATE="$ROOT_DIR/scripts/check-ci-merge-gate.sh"
SCORECARD="$ROOT_DIR/scripts/check-pr-release-scorecard.sh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

base_env=(
  SECRET_GUARD_RESULT=success
  CHANGED_FILES_RESULT=success
  VERIFY_CHANGED=false
  GO_CHANGED=false
  DEPLOY_CHANGED=false
  VIEWER_CHANGED=false
  MONITORING_CHANGED=false
  DOCS_CHANGED=false
  SHELL_CHANGED=false
  IMAGE_SCAN_CHANGED=false
  QUICKSTART_CHANGED=false
  WIZARD_RELEASE_CHANGED=false
  GO_WORKFLOW_CHANGED=false
  VERIFY_RESULT=skipped
  GO_VERIFY_RESULT=skipped
  QUICKSTART_RESULT=skipped
  VIEWER_RESULT=skipped
  SHELLCHECK_RESULT=skipped
  DOCS_RESULT=skipped
  MONITORING_RESULT=skipped
  GO_WORKFLOW_RESULT=skipped
  WIZARD_RESULT=skipped
  IMAGE_SCAN_RESULT=skipped
  SCORECARD_REQUIRED=false
  SCORECARD_RESULT=skipped
)

expect_pass() {
  local name="$1"
  shift
  if ! env "${base_env[@]}" "$@" "$GATE" >"$TMP_DIR/$name.out" 2>&1; then
    echo "FAIL: $name should pass" >&2
    cat "$TMP_DIR/$name.out" >&2
    exit 1
  fi
  grep -Fq '**Result: PASS**' "$TMP_DIR/$name.out"
}

expect_fail() {
  local name="$1"
  local expected="$2"
  shift 2
  if env "${base_env[@]}" "$@" "$GATE" >"$TMP_DIR/$name.out" 2>&1; then
    echo "FAIL: $name should fail" >&2
    cat "$TMP_DIR/$name.out" >&2
    exit 1
  fi
  grep -Fq "$expected" "$TMP_DIR/$name.out"
  grep -Fq '**Result: FAIL**' "$TMP_DIR/$name.out"
}

expect_pass docs-only
expect_pass selected-success \
  VERIFY_CHANGED=true VERIFY_RESULT=success \
  GO_CHANGED=true GO_VERIFY_RESULT=success \
  QUICKSTART_CHANGED=true QUICKSTART_RESULT=success \
  SHELL_CHANGED=true SHELLCHECK_RESULT=success
expect_fail required-skip 'Ubuntu test-all gate was required but result was skipped' \
  VERIFY_CHANGED=true VERIFY_RESULT=skipped
expect_fail required-cancel 'Viewer CI was required but result was cancelled' \
  VIEWER_CHANGED=true VIEWER_RESULT=cancelled
expect_fail unexpected-failure 'Docs consistency was not required but result was failure' \
  DOCS_RESULT=failure
expect_fail scorecard-failure 'PR release scorecard was required but result was failure' \
  SCORECARD_REQUIRED=true SCORECARD_RESULT=failure

cat >"$TMP_DIR/incomplete-body.md" <<'EOF'
## Summary

Incomplete scorecard fixture.
EOF
printf '%s\n' 'docs/testing.md' >"$TMP_DIR/docs-files.txt"
printf '%s\n' '.github/workflows/ci.yml' >"$TMP_DIR/risky-files.txt"

"$SCORECARD" \
  --body "$TMP_DIR/incomplete-body.md" \
  --changed-files "$TMP_DIR/docs-files.txt" \
  --strict-if-risky >"$TMP_DIR/docs-scorecard.out"
grep -Fq 'Advisory mode: warnings do not fail this command.' "$TMP_DIR/docs-scorecard.out"

if "$SCORECARD" \
  --body "$TMP_DIR/incomplete-body.md" \
  --changed-files "$TMP_DIR/risky-files.txt" \
  --strict-if-risky >"$TMP_DIR/risky-scorecard.out" 2>&1; then
  echo "FAIL: risky changed paths must make scorecard warnings blocking" >&2
  exit 1
fi
grep -Fq 'Risk-triggered strict mode: warnings fail this command.' "$TMP_DIR/risky-scorecard.out"

cat >"$TMP_DIR/complete-body.md" <<'EOF'
## Release scorecard

### Change classification

- [x] build/CI

### Risk level

- [x] medium - runtime behavior

### Evidence map

- [x] Unit/focused tests: fixture

### Operator/release impact

- [x] No operator-facing impact

### Medium/high-risk review prompts

- The supported deployment boundary is unchanged.
EOF

"$SCORECARD" \
  --body "$TMP_DIR/complete-body.md" \
  --changed-files "$TMP_DIR/risky-files.txt" \
  --strict-if-risky >"$TMP_DIR/complete-scorecard.out"
grep -Fq 'PR release scorecard check passed.' "$TMP_DIR/complete-scorecard.out"

echo "PASS: merge gate and risk-aware scorecard fixtures"
