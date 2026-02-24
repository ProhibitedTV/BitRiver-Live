# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Thread caller context through rate-limit interfaces
  - Acceptance criteria:
    - `tokenStore.Allow` and `rateLimiter.AllowLogin` accept `context.Context`.
    - Middleware passes `r.Context()` into `AllowLogin`.
    - Code compiles with updated signatures.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1 -timeout=120s`

- [x] Task 2 — Refactor Redis store allow context handling
  - Acceptance criteria:
    - `redisStore.Allow` uses normalized caller context (not `context.Background()`).
    - Redis operations are wrapped with configured timeout while preserving caller cancellation/deadline behavior.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -run 'TestRedisStoreAllow(Plain|TLS)$|TestRateLimitMiddlewareAuthPaths' -count=1 -timeout=120s`

- [x] Task 3 — Add tests for cancellation and timeout handling
  - Acceptance criteria:
    - Tests cover canceled context propagation and timeout behavior for Redis-backed `Allow` path.
    - Tests pass reliably in `internal/server` package.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -run 'TestRedisStoreAllow(CanceledContext|Timeout)$' -count=1 -timeout=120s`

- [x] Task 4 — Run full verification
  - Acceptance criteria:
    - Repo verification script executed and result recorded.
  - Relevant checks:
    - ✅ `./scripts/verify.sh` (passed; docker-dependent checks skipped because docker is not installed)

## Execution log
- ✅ `gofmt -w internal/server/ratelimit.go internal/server/server.go internal/server/redis_store.go internal/server/redis_store_integration_test.go internal/server/redis_store_test.go`.
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1 -timeout=120s`.
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -run 'TestRedisStoreAllow(Plain|TLS)$|TestRateLimitMiddlewareAuthPaths' -count=1 -timeout=120s`.
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -run 'TestRedisStoreAllow(CanceledContext|Timeout)$' -count=1 -timeout=120s`.
- ✅ `./scripts/verify.sh` (passed; docker compose validation steps skipped because docker is not installed in this environment).
