# Testing BitRiver Live

This document collects the commands the project uses in CI so contributors can
run the same suites locally before opening a pull request. See
`docs/testing-status.md` for a living summary of flaky suites and gaps that need
coverage.

## Go API

Run the fast unit suite (JSON datastore, REST handlers, chat flows) from the
repository root with the same environment guardrails CI enforces. Setting
`GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off` ensures the local Go toolchain is
used without reaching out to the network, which keeps results reproducible and
matches the locked-down CI runners. The `-count=1 -timeout=10s` flags prevent
test caching and match CI's deadline for each package:

```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=10s
```

OME quickstart drift is guarded by an ingest test that reads the pinned image
in `deploy/docker-compose.yml` and compares `deploy/ome/Server.xml` to the
expected template for that tag. It also enforces required fields such as
`<Type>origin</Type>` and the `<Bind>`/`<IP>` listener pairs. When updating the
OME image, refresh the template map in
`internal/ingest/ome_config_test.go` and rerun:

```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/ingest -count=1
```

The same package run now exercises the ingest stream lifecycle with
`internal/testsupport/ingeststub`, simulating channel provision, application
creation, transcoder retries, and teardown without external services. To focus
on the lifecycle path while iterating, scope the tests with `-run`:

```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/ingest -count=1 -run HTTPControllerStreamLifecycleIntegration
```

## Quickstart/Compose smoke

Run the compose smoke guard to ensure the default `.env` and `deploy/docker-compose.yml` still render and that the tracked health probes stay wired:

```bash
./scripts/test-quickstart.sh
```

When no `.env` exists in the repository root, the helper seeds one with the same local defaults baked into the quickstart script, renders `docker compose config`, and verifies that the API, transcoder, OME, SRS, Postgres, and Redis healthchecks still point at their expected endpoints. It also calls `scripts/render-ome-config.sh` against the seeded `.env` and fails fast when `deploy/ome/Server.generated.xml` is stale or missing required `<Bind>`, `<IP>`, or control credential values so the tracked compose mount stays fresh. It cleans up the temporary `.env` after the run.

## Postgres storage layer

Storage integration tests live behind the `postgres` build tag. They expect an
empty database that matches the schema in `deploy/migrations/`. Point
`BITRIVER_TEST_POSTGRES_DSN` at the database before launching `go test`:

```bash
BITRIVER_TEST_POSTGRES_DSN="postgres://bitriver:bitriver@127.0.0.1:5432/bitriver_test?sslmode=disable" \
  go test -count=1 -tags postgres ./internal/storage/...
```

When `BITRIVER_TEST_POSTGRES_DSN` is unset, the test harness now spins up a
disposable Postgres container (using the same defaults as
`scripts/test-postgres.sh`) and skips cleanly when Docker is unavailable so
`go test -tags postgres ./...` stays deterministic. For local development, run
the helper script instead of managing the database by hand. It starts a
disposable Postgres container, applies the tracked migrations, and executes the
storage suite in one step (Docker required). The script forces an offline
module mode (`GOPROXY=off GOSUMDB=off GOFLAGS=-mod=readonly`) so vendored
replacements stay intact and `go.mod`/`go.sum` remain untouched:

```bash
./scripts/test-postgres.sh
```

## Web viewer

Install dependencies once and execute the lint and integration harnesses:

```bash
cd web/viewer
npm install
npm run lint
npm run test:integration
```

The Playwright-powered integration suite downloads its browsers on first run.
Use `npx playwright install --with-deps` when you need an offline-friendly
preinstall.
