# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add focused retry-behavior tests for ingest boot helper in `internal/storage/stream_test.go`
  - Acceptance criteria:
    - Add a test where fake ingest boot returns `context.DeadlineExceeded` or `context.Canceled` on first call.
    - Invoke `runIngestBootWithRetry` with attempts > 1 and non-zero retry interval.
    - Assert only one boot call is made and elapsed time confirms no retry sleep delay is applied.
    - Add a complementary test where a transient non-context error on first call triggers retry and succeeds/fails on subsequent call as expected.
  - Relevant checks:
    - `go test ./internal/storage -run 'TestRunIngestBootWithRetry' -count=1`

- [x] Task 2 — Make retry helper behavior explicit if needed in `internal/storage/ingest_boot_helpers.go`
  - Acceptance criteria:
    - If tests reveal retries continue after terminal context errors, update helper minimally to stop retrying on context cancellation/deadline errors.
    - Keep behavior unchanged for non-context transient errors.
  - Relevant checks:
    - `go test ./internal/storage -run 'TestRunIngestBootWithRetry' -count=1`

- [x] Task 3 — Run repository verification gate
  - Acceptance criteria:
    - `./scripts/verify.sh` runs successfully, or environment limitation/failure is recorded.
  - Relevant checks:
    - `./scripts/verify.sh`

## Execution log

- ✅ `gofmt -w internal/storage/ingest_boot_helpers.go internal/storage/stream_test.go`
- ✅ `go test ./internal/storage -run 'TestRunIngestBootWithRetry' -count=1`

- ⚠️ `./scripts/verify.sh` (passes overall; docker-dependent compose checks skipped because docker is not installed in this environment)
