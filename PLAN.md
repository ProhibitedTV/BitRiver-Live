# PLAN

## Scope (current change)
- Stabilize `TestAuthSessionIdleRefresh` in `internal/api/auth_integration_test.go` by removing fixed sleep and waiting for observable expiry advancement with bounded polling.
- Stabilize `TestHTTPControllerHealthChecksRunConcurrentlyAndDeterministically` in `internal/ingest/http_controller_test.go` by replacing server-side fixed delay with synchronization via atomics/channels and bounded waits.
- Reuse/add a small shared `internal/testsupport` helper for eventual assertions within timeouts.
- Validate determinism by re-running affected package tests multiple times.

## Assumptions
- Existing helper support may already exist in `internal/testsupport`; if sufficient, reuse it rather than introducing duplicate helpers.
- These updates are test-only and should not affect runtime behavior or deployment contract files.
- Repeated `go test -count` runs are enough to catch timing-related flakiness for this scope.

## Risks
- Polling conditions that are too strict may still be flaky under slow CI scheduling.
- New synchronization in ingest health-check test could deadlock if not carefully bounded.
- Overly short timeouts may cause false negatives in loaded environments.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run TestAuthSessionIdleRefresh -count=20 -timeout=120s`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/ingest -run TestHTTPControllerHealthChecksRunConcurrentlyAndDeterministically -count=20 -timeout=120s`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api ./internal/ingest ./internal/testsupport -count=1 -timeout=120s`
