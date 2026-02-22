# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Refactor `waitFor` helper to ticker + context timeout
  - Acceptance criteria:
    - `waitFor` uses `context.WithTimeout` and `time.Ticker`.
    - Helper no longer uses `time.Sleep` busy-wait polling.
    - Failure path keeps explicit timeout context in message.
  - Relevant checks:
    - ✅ `go test ./cmd/transcoder -count=1`
  - Result:
    - Passed.

- [x] Task 2 — Update transcoder tests for new `waitFor` signature
  - Acceptance criteria:
    - All `waitFor` call sites compile with new signature.
    - Failure reason strings are explicit per wait condition.
  - Relevant checks:
    - ✅ `go test ./cmd/transcoder -count=1`
  - Result:
    - Passed.

- [x] Task 3 — Final verification and status log
  - Acceptance criteria:
    - Targeted transcoder package test passes with no assertion behavior changes.
    - Task results are recorded below.
  - Relevant checks:
    - ✅ `go test ./cmd/transcoder -count=1`
  - Result:
    - Passed.

## Execution log
- Task 1 check: `go test ./cmd/transcoder -count=1` (pass).
- Task 2 check: `go test ./cmd/transcoder -count=1` (pass).
- Task 3 check: `go test ./cmd/transcoder -count=1` (pass; no behavior assertion changes).
