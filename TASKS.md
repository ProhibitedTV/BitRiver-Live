# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Refactor `doWithRetry` control flow
  - Acceptance criteria:
    - Remove the `attempt = attempts` control-flow pattern.
    - Introduce explicit branching for retryable vs non-retryable failures.
    - Preserve existing retry counts and error message content.
  - Relevant checks:
    - ✅ `go test ./internal/ingest -count=1`
  - Result:
    - Passed.

- [x] Task 2 — Add/adjust non-retryable 4xx coverage
  - Acceptance criteria:
    - A test proves non-429 4xx responses execute exactly one attempt (no retries).
    - Existing ingest adapter tests continue to pass.
  - Relevant checks:
    - ✅ `go test ./internal/ingest -count=1`
  - Result:
    - Passed.

## Execution log
- Task 1 check: `go test ./internal/ingest -count=1` (pass).
- Task 2 check: `go test ./internal/ingest -count=1` (pass).
