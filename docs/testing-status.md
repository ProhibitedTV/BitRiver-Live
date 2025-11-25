# Testing Status

This page tracks suites that still need attention and critical gaps in
coverage. Keep it updated as tests are hardened or new flakes are discovered.

## Known flaky or failing areas

- **Postgres-tagged storage suite (`internal/storage`, `-tags postgres`):**
  - Depends on Docker when `BITRIVER_TEST_POSTGRES_DSN` is unset; tests skip if
    Docker is unavailable and otherwise start a disposable container.
  - Common failures come from containers that cannot start (ports in use,
    Docker daemon down) or schema drift between migrations and test helpers.
- **Chat Redis queue tests (`internal/chat/redis_queue_test.go`):** Previously
  relied on `time.Sleep` to force requeue timing, which was prone to races.
  Event-driven hooks now gate the assertions, but regressions in the Redis stub
  or queue back-pressure still need close watch.
- **Upload processor tests (`internal/api/uploads_processor_test.go`):** Now
  synchronized on fake ingest/store completions instead of polling. If the fake
  processors change, ensure completion channels continue to fire to avoid
  deadlocks.

## Coverage gaps to address

- **Ingest/stream lifecycle:** Live ingest orchestration now has deterministic
  integration coverage for stream start/stop and retry handling via the HTTP
  controller fakes, but export and recording flows still need end-to-end
  attention.
- **Authentication and session flows:** Need integration coverage for login,
  session refresh/expiry, and permission checks across admin and viewer APIs.
- **Viewer/client interactions:** Limited automated coverage for chat and
  playback from the Next.js viewer beyond unit tests.

## Reliability checklist

- Prefer channel- or hook-based synchronization in asynchronous tests to avoid
  timing-based flakes.
- For Postgres-tagged tests, verify Docker is available or set
  `BITRIVER_TEST_POSTGRES_DSN` to a prepared database before invoking
  `go test -tags postgres ./...`.
- Keep fake helpers in `internal/testsupport` deterministic; adding new hooks is
  preferable to introducing sleeps.
