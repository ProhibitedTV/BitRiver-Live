# `internal/ingest` Guidance

Adapters for RTMP/SRS/OvenMediaEngine control live here. See the root `AGENTS.md` for shared expectations.

## Adapters
- Keep adapters pure HTTP clients (see `adapters.go`). They should accept interfaces or configs, construct requests, and return typed responses/errors. No global state.
- When adding endpoints, respect existing timeout/retry semantics so rate limiting and backoff remain predictable.
- Use `metrics.Default()` for instrumentation so ingest metrics align with the rest of the stack.

## Testing
- Stub remote services via `internal/testsupport` helpers. Avoid real network calls in unit tests.
- Document new control-plane interactions in `docs/advanced-deployments.md` when applicable.

## Before opening a PR
- Update `deploy/` and scripts if new ingest endpoints require env vars or ports.
- Run `go test ./internal/ingest -count=1` and integration suites that touch ingest flows.
