## Scope (current change)
- Address GitHub issue #1243 by removing the remaining sleep-driven auth/server timing test coverage.
- Keep the change focused on `internal/server` rate-limiter testability, cleanup tracking docs, and working artifacts.
- Treat `internal/auth/session_test.go` as already resolved after the read-only pass because session expiration, idle refresh, and absolute TTL tests use a deterministic test clock.
- Preserve runtime rate-limit behavior by defaulting any clock hook to `time.Now`.

## Assumptions
- `SessionManager` already has a private test clock path (`withClock`) and does not need production API changes.
- The stale login-bucket cleanup test only needs deterministic control over the rate limiter's cleanup timestamps, not token-bucket refill behavior.
- A private `rateLimiter` clock function is sufficient because tests live in package `server`.
- Runtime behavior and deployment contracts are unchanged.

## Risks
- If cleanup and bucket last-seen timestamps use different clock sources, the test could pass while runtime cleanup semantics drift.
- Changing the token bucket clock is unnecessary for this issue and could broaden the behavioral surface.
- The full verification gate may be longer than the targeted package checks because Docker-dependent checks still run.

## Test plan
- `go test ./internal/auth -count=1 -timeout=120s`
- `go test ./internal/server -run TestRateLimiterCleanupStaleBucketsEventually -count=1 -timeout=120s`
- `go test ./internal/server -count=1 -timeout=120s`
- `go test ./... -count=1 -timeout=120s`
- `git diff --check`
- `./scripts/verify.sh`
