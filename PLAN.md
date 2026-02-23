# PLAN

## Scope (current change)
- Remove unsynchronized lazy writes in `internal/api/handlers.go` accessor methods:
  - `sessionManager`
  - `mfaChallengeManager`
  - `logger`
  - `tracer`
  - `srsTracker`
- Preserve existing nil-safe behavior expected by current tests (accessors still return usable defaults when fields are unset).
- Add a concurrency-focused test in `internal/api` that exercises these accessors from many goroutines, then validate with the race detector.

## Assumptions
- Accessors are expected to be safe to call on a zero-value/partially-initialized `Handler` and should continue to return non-nil defaults.
- `NewHandler` should keep current defaults for explicit dependency injection while improving concurrency safety.
- Existing API/runtime behavior is unchanged; this is an internal safety fix plus test coverage.

## Risks
- Changing initialization timing could alter pointer identity assumptions in tests if not done carefully.
- Concurrency test could become flaky if it depends on timing rather than deterministic synchronization.
- Race-detector runs are slower and may expose unrelated package races; scope test selection to `internal/api` accessor tests first.

## Test plan
- `gofmt -w internal/api/handlers.go internal/api/handlers_concurrency_test.go`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run TestHandlerAccessorsConcurrentInitialization -count=1 -race -timeout=120s`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1 -timeout=120s`
