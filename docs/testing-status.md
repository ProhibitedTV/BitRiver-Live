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

- **Ingest/stream lifecycle:** End-to-end coverage for stream start/stop across
  ingest controllers, transcoder coordination, and recording export is minimal.
- **Viewer/client interactions:** Limited automated coverage for chat and
  playback from the Next.js viewer beyond unit tests.

## Recent coverage additions

- **Authentication/session lifecycle:** Integration-style tests under
  `internal/api/auth_integration_test.go` exercise login, refresh, logout, and
  admin-only enforcement using the `internal/testsupport.SessionStoreStub`.
  They run with the standard Go test environment (offline module settings,
  no external services required).

## Reliability checklist

- Prefer channel- or hook-based synchronization in asynchronous tests to avoid
  timing-based flakes.
- For Postgres-tagged tests, verify Docker is available or set
  `BITRIVER_TEST_POSTGRES_DSN` to a prepared database before invoking
  `go test -tags postgres ./...`.
- Keep fake helpers in `internal/testsupport` deterministic; adding new hooks is
  preferable to introducing sleeps.
