# PLAN

## Scope (current change)
- Add focused unit tests for `runIngestBootWithRetry` in `internal/storage/stream_test.go`.
- Verify retry loop stops immediately on terminal context errors (`context.DeadlineExceeded` / `context.Canceled`) without waiting for retry sleep.
- Verify retry loop still retries for non-context transient errors.
- Make helper behavior explicit in `internal/storage/ingest_boot_helpers.go` only if current implementation does not match expected behavior.

## Assumptions
- `runIngestBootWithRetry` should treat context cancellation/deadline errors as terminal for the current boot flow.
- Existing storage tests already expose a fake ingest controller that can be reused to count boot attempts.

## Risks
- Sleep-based retry checks can become flaky if assertions rely on wall-clock precision.
- Updating retry behavior could affect both file-backed and postgres-backed stream start paths since they share the helper.

## Test plan
- `go test ./internal/storage -run 'TestRunIngestBootWithRetry' -count=1`
- `./scripts/verify.sh`
