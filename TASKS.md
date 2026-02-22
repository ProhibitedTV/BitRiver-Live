# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add shared default HTTP client for nil-client paths
  - Acceptance criteria:
    - `internal/ingest/adapters.go` defines a private package-level shared `http.Client` default.
    - Shared client keeps timeout aligned with `defaultHTTPTimeout`.
  - Relevant checks:
    - ✅ `go test ./internal/ingest -count=1`
  - Result:
    - Passed.

- [x] Task 2 — Switch helper nil-client fallback to shared client
  - Acceptance criteria:
    - `postJSON` and `deleteRequest` use the shared default client when `client == nil`.
    - Existing retry behavior and function signatures remain unchanged.
  - Relevant checks:
    - ✅ `go test ./internal/ingest -count=1`
  - Result:
    - Passed.

- [x] Task 3 — Add/adjust nil-client tests
  - Acceptance criteria:
    - Tests cover nil-client behavior for helper operations.
    - Targeted ingest tests pass and results are recorded.
  - Relevant checks:
    - ✅ `go test ./internal/ingest -count=1`
    - ⚠️ `./scripts/verify.sh` (passes; Docker-dependent steps skipped because Docker is unavailable in this environment)
  - Result:
    - Passed.

## Execution log
- Task 1 check: `go test ./internal/ingest -count=1` (pass).
- Task 2 check: `go test ./internal/ingest -count=1` (pass).
- Task 3 checks:
  - `go test ./internal/ingest -count=1` (pass).
  - `./scripts/verify.sh` (pass with expected Docker-unavailable skips).
