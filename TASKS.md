# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Implement CI-aware viewer diff detection in `scripts/verify.sh`
  - Acceptance criteria:
    - `viewer_changes_present()` uses commit range diff when CI base/head metadata is available.
    - Local fallback remains `git status --porcelain -- web/viewer`.
    - Existing `--viewer` and `--ci-viewer` behavior is unchanged.
  - Relevant checks:
    - `bash scripts/test-verify-viewer-detection.sh`
  - Result:
    - Passed.

- [x] Task 2 — Add script-level detection contract tests under `scripts/`
  - Acceptance criteria:
    - Tests cover CI base/head range with viewer changes.
    - Tests cover CI base/head range without viewer changes.
    - Tests cover local uncommitted viewer changes fallback behavior.
  - Relevant checks:
    - `bash scripts/test-verify-viewer-detection.sh`
  - Result:
    - Passed.

- [x] Task 3 — Validate integration behavior for viewer skip/run conditions
  - Acceptance criteria:
    - Confirm viewer checks are skipped only when out of scope.
    - Confirm force flags still trigger viewer checks.
    - Record executed commands and outcomes.
  - Relevant checks:
    - `./scripts/verify.sh --go-packages ./cmd/bitriver`
    - `bash scripts/test-verify-viewer-detection.sh`
  - Result:
    - Passed (`./scripts/verify.sh` skipped viewer in out-of-scope local run; contract test asserted force-flag run paths).

## Execution log

- ✅ `bash scripts/test-verify-viewer-detection.sh`
- ✅ `./scripts/verify.sh --go-packages ./cmd/bitriver`
- ✅ `bash scripts/test-verify-viewer-detection.sh`
