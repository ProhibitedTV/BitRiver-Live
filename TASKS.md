# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add reusable async wait helper in `internal/testsupport`
  - Acceptance criteria:
    - New helper supports timeout-based polling for async test conditions.
    - Helper exposes improved timeout diagnostics with caller-provided context.
  - Relevant checks:
    - ✅ `go test ./internal/chat -count=1`
  - Result:
    - Passed.

- [x] Task 2 — Replace `waitUntil` in `internal/chat/gateway_test.go`
  - Acceptance criteria:
    - Local `waitUntil` helper is removed from `gateway_test.go`.
    - Existing waits call the shared testsupport helper.
    - Assertion semantics remain unchanged.
  - Relevant checks:
    - ✅ `go test ./internal/chat -count=1`
  - Result:
    - Passed.

- [x] Task 3 — Confirm parity with chat tests
  - Acceptance criteria:
    - Targeted chat package tests pass after refactor.
    - Task statuses and check results are recorded.
  - Relevant checks:
    - ✅ `go test ./internal/chat -count=1`
  - Result:
    - Passed.

## Execution log
- Task 1 check: `go test ./internal/chat -count=1` (pass).
- Task 2 check: `go test ./internal/chat -count=1` (pass).
- Task 3 check: `go test ./internal/chat -count=1` (pass).
