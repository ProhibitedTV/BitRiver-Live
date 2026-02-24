# PLAN

## Scope (current change)
- Refactor server-side rate-limit token store contract so `Allow` accepts a caller-provided `context.Context`.
- Update `internal/server/redis_store.go` to normalize caller context and wrap Redis operations with the configured timeout instead of using `context.Background()`.
- Ensure request-scoped callers in rate-limiting middleware pass `r.Context()` through `AllowLogin` to the token store.
- Add/adjust tests to verify cancellation and timeout handling through the Redis-backed rate-limit path.

## Assumptions
- Existing behavior for nil contexts should still be handled via `ctxutil.Normalize`.
- `AllowLogin` is only called from HTTP middleware paths and can safely accept context without breaking external APIs.
- Redis client honors context cancellation/timeouts for `Do` operations, including with the local redis stub used in tests.

## Risks
- Signature changes across `tokenStore`/`rateLimiter` could break compilation in less obvious call sites.
- Timeout wrapping could accidentally override earlier caller deadlines if composed incorrectly.
- Cancellation/timeout tests may be flaky if they depend on real timing; keep assertions deterministic.

## Test plan
- `gofmt -w internal/server/ratelimit.go internal/server/server.go internal/server/redis_store.go internal/server/redis_store_integration_test.go internal/server/redis_store_test.go internal/server/server_test.go`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1 -timeout=120s`
- `./scripts/verify.sh`
