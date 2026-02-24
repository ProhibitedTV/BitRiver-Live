# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Wire `require_tool go` before Go tests
  - Acceptance criteria:
    - `scripts/verify.sh` checks for `go` via `require_tool` before the Go test step.
    - Go test command remains unchanged otherwise.
  - Relevant checks:
    - `bash -n scripts/verify.sh`
  - Result:
    - Passed.

- [x] Task 2 — Wire `require_tool python3` before contract invariants check
  - Acceptance criteria:
    - `scripts/verify.sh` checks for `python3` via `require_tool` before `./scripts/check-contract-invariants.sh`.
    - Existing contract invariants command remains in-place.
  - Relevant checks:
    - `bash -n scripts/verify.sh`
  - Result:
    - Passed.

- [x] Task 3 — Add optional `git` gate for local viewer-change detection with clear fallback message
  - Acceptance criteria:
    - Missing `git` does not hard-fail verify in local mode.
    - Viewer skip message explicitly explains fallback/force option when `git` is unavailable.
    - Docker/node/npm remain conditional skips.
  - Relevant checks:
    - `./scripts/verify.sh --go-packages ./cmd/bitriver`
  - Result:
    - Passed (local skip path still works and conditional docker/node/npm behavior remains unchanged).

## Execution log

- ✅ `bash -n scripts/verify.sh`
- ✅ `./scripts/verify.sh --go-packages ./cmd/bitriver`
