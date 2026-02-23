# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Update auth idle refresh test to avoid fixed sleep
  - Acceptance criteria:
    - `TestAuthSessionIdleRefresh` no longer uses `time.Sleep` to wait for refreshability.
    - Test waits with bounded polling and fails with clear timeout context if expiry does not advance.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run TestAuthSessionIdleRefresh -count=20 -timeout=120s`

- [x] Task 2 — Update ingest concurrency test to use synchronization instead of fixed server sleep
  - Acceptance criteria:
    - `TestHTTPControllerHealthChecksRunConcurrentlyAndDeterministically` no longer uses fixed server-side sleeps.
    - Concurrency observation uses atomics/channels with bounded waits and deterministic assertions.
  - Relevant checks:
    - ❌ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/ingest -run TestHTTPControllerHealthChecksRunConcurrentlyAndDeterministically -count=20 -timeout=120s` (first attempt exposed deadlock in test synchronization; fixed by releasing probes via channel close and waiting for bounded concurrent arrival)
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/ingest -run TestHTTPControllerHealthChecksRunConcurrentlyAndDeterministically -count=20 -timeout=120s` (after fix)

- [x] Task 3 — Add/reuse shared eventually-with-timeout helper in `internal/testsupport`
  - Acceptance criteria:
    - A small shared helper supports polling conditions until timeout with useful failure output.
    - Updated tests use the shared helper where applicable.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/testsupport ./internal/api ./internal/ingest -count=1 -timeout=120s`

- [x] Task 4 — Re-run affected package tests multiple times for determinism validation
  - Acceptance criteria:
    - Repeated runs for targeted tests complete successfully.
    - Task log records commands and outcomes.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run TestAuthSessionIdleRefresh -count=20 -timeout=120s`
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/ingest -run TestHTTPControllerHealthChecksRunConcurrentlyAndDeterministically -count=20 -timeout=120s`

## Execution log
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run TestAuthSessionIdleRefresh -count=20 -timeout=120s`.
- ❌ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/ingest -run TestHTTPControllerHealthChecksRunConcurrentlyAndDeterministically -count=20 -timeout=120s` (initial deadlock surfaced).
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/ingest -run TestHTTPControllerHealthChecksRunConcurrentlyAndDeterministically -count=20 -timeout=120s` (after synchronization fix).
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/testsupport ./internal/api ./internal/ingest -count=1 -timeout=120s`.
- ✅ `./scripts/verify.sh` (passes; docker-related checks skipped because docker is not installed in this environment).
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api ./internal/ingest ./internal/testsupport -count=1 -timeout=120s`.
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run TestAuthSessionIdleRefresh -count=20 -timeout=120s && GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/ingest -run TestHTTPControllerHealthChecksRunConcurrentlyAndDeterministically -count=20 -timeout=120s`.
