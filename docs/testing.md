# Testing BitRiver Live

This document collects the commands the project uses in CI so contributors can
run the same suites locally before opening a pull request.

## Go API

Run the fast unit suite (JSON datastore, REST handlers, chat flows) from the
repository root:

```bash
go test ./...
```

## Postgres storage layer

Storage integration tests live behind the `postgres` build tag. They expect an
empty database that matches the schema in `deploy/migrations/`. Point
`BITRIVER_TEST_POSTGRES_DSN` at the database before launching `go test`:

```bash
BITRIVER_TEST_POSTGRES_DSN="postgres://bitriver:bitriver@127.0.0.1:5432/bitriver_test?sslmode=disable" \
  go test -count=1 -tags postgres ./internal/storage/...
```

For local development, run the helper script instead of managing the database by
hand. It starts a disposable Postgres container, applies the tracked migrations,
and executes the storage suite in one step (Docker required):

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
