# Testing Status

This page tracks suites that still need attention and critical gaps in
coverage. Keep it updated as tests are hardened or new flakes are discovered.

## Known flaky or failing areas

- **Postgres-tagged storage suite (`internal/storage`, `-tags postgres`):**
  - Requires either `BITRIVER_TEST_POSTGRES_DSN` or Docker to launch a clean
    database. The harness now fails fast when neither is available instead of
    skipping.
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
- **Transcoder lifecycle tests (`cmd/transcoder/main_test.go`):** The FFmpeg
  integration paths now exercise a bundled stub at
  `cmd/transcoder/testdata/ffmpeg`, so they no longer depend on a host
  installation. If assertions fail, verify the stub remains on `PATH` and that
  the temporary output roots created by the tests are writable.

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
- CI merges must run (not skip) the Postgres-tagged storage suite; provision
  Docker or set `BITRIVER_TEST_POSTGRES_DSN` so `go test -tags postgres ./...`
  executes end-to-end.
- For Postgres-tagged tests, verify Docker is available or set
  `BITRIVER_TEST_POSTGRES_DSN` to a prepared database before invoking
  `go test -tags postgres ./...`. The harness will now fail fast when neither
  is available.
- Keep fake helpers in `internal/testsupport` deterministic; adding new hooks is
  preferable to introducing sleeps.
