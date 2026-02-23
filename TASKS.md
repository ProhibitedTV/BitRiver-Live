# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Extract shared ingest boot retry + mapping helpers
  - Acceptance criteria:
    - Shared helper(s) in `internal/storage` encapsulate common `BootStream` retry loop behavior and boot-result-to-session-field mapping.
    - Helper behavior preserves existing timeout, retry, and error propagation semantics.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -run 'TestStartStream(RetriesBootFailures|FailureRollsBackState|DefaultsAttemptsToOne|AppliesRetryInterval)' -count=1 -timeout=120s`

- [x] Task 2 — Refactor JSON and Postgres `StartStream` paths to use helpers
  - Acceptance criteria:
    - `internal/storage/storage.go` and `internal/storage/postgres_channels.go` both use the shared helper(s).
    - Persistence-specific writes/transactions stay local to each implementation.
    - Existing error text/fallback behavior (including rollback to offline + cleared session on boot failure) is preserved.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -run 'TestStartStream(RetriesBootFailures|FailureRollsBackState|DefaultsAttemptsToOne|AppliesRetryInterval)' -count=1 -timeout=120s`

- [x] Task 3 — Expand tests for retries/rollback guarantees
  - Acceptance criteria:
    - Tests assert attempts default to 1 when configured attempts are <= 0.
    - Tests assert retry interval is applied between failed boot attempts.
    - Tests assert rollback to `offline` and nil `current_session_id` on boot failure.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -run 'TestStartStream(RetriesBootFailures|FailureRollsBackState|DefaultsAttemptsToOne|AppliesRetryInterval)' -count=1 -timeout=120s`

- [x] Task 4 — Run storage verification tests and record results
  - Acceptance criteria:
    - Targeted and package-level storage tests pass.
    - Task log records commands and outcomes.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -count=1 -timeout=120s`
    - ❌ `./scripts/verify.sh` (first run failed in unrelated flaky test `TestStreamGaugeConcurrent`)
    - ✅ `./scripts/verify.sh` (second run passed; docker checks skipped because docker is not installed)

## Execution log
- ✅ `gofmt -w internal/storage/ingest_boot_helpers.go internal/storage/storage.go internal/storage/postgres_channels.go internal/storage/stream_test.go`.
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -run 'TestStartStream(RetriesBootFailures|FailureRollsBackState|DefaultsAttemptsToOne|AppliesRetryInterval)' -count=1 -timeout=120s`.
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -count=1 -timeout=120s`.
- ❌ `./scripts/verify.sh` (first run failed in unrelated flaky test `TestStreamGaugeConcurrent`).
- ✅ `./scripts/verify.sh` (rerun passed; docker-dependent checks skipped due missing docker binary).
