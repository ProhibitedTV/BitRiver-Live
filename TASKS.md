# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Make handler accessors concurrency-safe
  - Acceptance criteria:
    - `sessionManager`, `mfaChallengeManager`, `logger`, `tracer`, and `srsTracker` no longer perform unsynchronized lazy writes.
    - Chosen approach is either eager initialization in `NewHandler` or `sync.Once` guards per field.
    - Nil-safe behavior is preserved (unset fields still yield usable defaults).
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run TestHandlerAccessorsConcurrentInitialization -count=1 -race -timeout=120s`

- [x] Task 2 — Add concurrent accessor test coverage
  - Acceptance criteria:
    - New test in `internal/api` calls accessor methods from multiple goroutines.
    - Test validates non-nil defaults and stable initialized instances under concurrent access.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run TestHandlerAccessorsConcurrentInitialization -count=1 -race -timeout=120s`

- [x] Task 3 — Run package verification and record outcomes
  - Acceptance criteria:
    - `internal/api` tests pass after changes.
    - Task log captures commands and pass/fail.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1 -timeout=120s`
    - ❌ `./scripts/verify.sh` (first run failed due unrelated flaky cleanup in `cmd/transcoder` test)
    - ✅ `./scripts/verify.sh` (rerun passed; docker checks skipped because docker is not installed)

## Execution log
- ✅ `gofmt -w internal/api/handlers.go internal/api/handlers_concurrency_test.go`.
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run TestHandlerAccessorsConcurrentInitialization -count=1 -race -timeout=120s`.
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1 -timeout=120s`.
- ❌ `./scripts/verify.sh` (first run failed due unrelated flaky cleanup in `cmd/transcoder` test).
- ✅ `./scripts/verify.sh` (rerun passed; docker-dependent checks skipped due missing docker binary).
