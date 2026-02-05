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
matches the locked-down CI runners. The `-count=1 -timeout=120s` flags prevent
test caching and match CI's 120-second deadline for each package. Use the same
timeout locally to avoid flakes on slower machines:

```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s
```


Function comment coverage is enforced in CI with a lightweight guard script:

```bash
./scripts/check-function-comments.py --strict-unexported
```

The checker scans non-test Go files and requires immediate comments above `func`
declarations. It ignores generated files (`// Code generated ... DO NOT EDIT.`),
`vendor/`, `third_party/`, and `_test.go` files by default.

Coverage policy:

- **Exported functions:** must always have comments (100% required).
- **Unexported functions in `cmd/` and `internal/`:** use strict mode for 100%,
  or set a threshold when iterating locally:

```bash
./scripts/check-function-comments.py --unexported-threshold 90
```

CI runs in regression mode against changed files so existing historical debt can
be paid down incrementally without allowing new gaps:

```bash
./scripts/check-function-comments.py --strict-unexported --git-base origin/main
```

Acceptable comment examples:

```go
// Start launches the API server and blocks until shutdown is requested.
func Start(ctx context.Context) error { ... }

// normalizeTags trims whitespace and removes duplicate tag values.
func normalizeTags(tags []string) []string { ... }
```

Exceptions:

- Generated files are excluded automatically.
- `_test.go` helpers are excluded automatically.
- `vendor/` and `third_party/` are excluded automatically.

Authentication/session lifecycle coverage lives in
`internal/api/auth_integration_test.go`. These integration-style handlers use
`internal/testsupport.SessionStoreStub` to validate cookie issuance, refresh,
logout, and admin-only enforcement without external services. No additional
environment toggles are required beyond the standard offline Go flags above.

Viewer payload contracts live in `internal/api/viewer_contract_test.go`. Run
the suite with the same offline flags and cache-busting timeout CI expects:

```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1 -timeout=120s -run ViewerContractEndpoints
```

The harness spins an `httptest` server using the real API router wired to the
JSON storage backend, then asserts that directory, playback, profile,
following, and chat history payloads match the contracts consumed by
`web/viewer/lib/viewer-api.ts`.

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

Security scanning uses `govulncheck` with the same offline Go settings and
vendor mode to ensure the results track the pinned `third_party/` replacements
that are mirrored into `vendor/`. Install the same pinned tool version used in
CI (`v1.1.3`, matching `.github/workflows/go-unit-tests.yml`) instead of
`@latest`; pinning is required so CI's Go 1.21 runners always use a compatible
`govulncheck` release.

`./scripts/run-govulncheck.sh` enforces the current vulnerability policy for the
pinned Go 1.21 toolchain:

- Reachable vulnerabilities in **non-stdlib modules** fail the run.
- Reachable vulnerabilities that affect only the Go `stdlib` are logged as
  informational while the repository remains on Go 1.21.
- Once the toolchain target in `go.mod` is raised beyond 1.21, stdlib findings
  return to fail-closed behavior automatically.

Use the helper script to run the root module scan plus checks for each replaced
third-party module:

```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go install golang.org/x/vuln/cmd/govulncheck@v1.1.3
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off ./scripts/run-govulncheck.sh
```

## Container image vulnerability scan exceptions (Trivy)

Container image CVE scanning is enforced by
[`.github/workflows/image-scan.yml`](../.github/workflows/image-scan.yml). The
workflow keeps **CRITICAL** gating enabled for first-party images
(`ghcr.io/bitriver-live/*` and local `bitriver-live/*` builds) and runs a
separate informational scan for pinned third-party images.

When a third-party base image ships an unavoidable finding (for example, a
distro package marked `will_not_fix`), add a tightly scoped exception under
[`.trivyignore/`](../.trivyignore/) using these rules:

1. Use an image-specific file when possible (for example,
   `postgres-15-alpine.txt`).
2. Suppress by **CVE ID only** (never by severity, package wildcard, or broad
   image class).
3. Add comments with rationale, review date, and expiry/review date.
4. Open a follow-up issue or planned image bump so the exception is removed
   promptly.

After adding or removing exceptions, rerun the scan workflow (or replicate its
Trivy commands locally in an environment with Docker) to confirm:

- first-party image scans still fail on unsuppressed CRITICAL findings, and
- any exception applies only to the intended image/CVE pair.

End-to-end ingest coverage (storage + HTTP controller + control-plane stub)
is packaged as a dedicated guard so release branches and tags keep exercising
the critical ingest → transcoder → playback path. Run the wrapper to boot the
ingest stub, drive the real HTTP controller, and verify manifests and teardown
calls are recorded:

```bash
./scripts/test-ingest-e2e.sh
```

## Quickstart/Compose smoke

Run the compose smoke guard to ensure the default `.env` and `deploy/docker-compose.yml` still render and that the tracked health probes stay wired:

```bash
./scripts/test-quickstart.sh
```

When no `.env` exists in the repository root, the helper seeds one with the same local defaults baked into the quickstart script, renders `docker compose config`, and verifies that the API, transcoder, OME, SRS, Postgres, and Redis healthchecks still point at their expected endpoints. It then boots the compose stack with the seeded `.env`, waits for all healthchecks to go green, curls the API health endpoint and viewer page, and tears the stack down via `docker compose down -v` so nothing is left behind. The script also invokes the Go renderer (`go run ./cmd/bitriver ome render`, or the `scripts/render-ome-config.sh` wrapper) against the seeded `.env` and fails fast when `deploy/ome/Server.generated.xml` is stale or missing required `<Bind>`, `<IP>`, or control credential values so the tracked compose mount stays fresh. It cleans up the temporary `.env` after the run.

## Postgres storage layer

Storage integration tests live behind the `postgres` build tag. They expect an
empty database that matches the schema in `deploy/migrations/`. The configured
user must be able to connect, create temporary tables, and read/write the
schema tables. Point `BITRIVER_TEST_POSTGRES_DSN` at the database before
launching `go test`:

```bash
BITRIVER_TEST_POSTGRES_DSN="postgres://bitriver:bitriver@127.0.0.1:5432/bitriver_test?sslmode=disable" \
  go test -count=1 -tags postgres ./internal/storage/...
```

When `BITRIVER_TEST_POSTGRES_DSN` is unset, the test harness spins up a
disposable Postgres container (using the same defaults as
`scripts/test-postgres.sh`). In CI, the suite must have either Docker available
or `BITRIVER_TEST_POSTGRES_DSN` pointing at a prepared database; otherwise the
postgres-tagged tests fail fast instead of skipping. For local development, run
the helper script instead of managing the database by hand. It uses a provided
`BITRIVER_TEST_POSTGRES_DSN` when set or starts a disposable Postgres
container, applies the tracked migrations, and executes the storage suite in
one step. When you supply `BITRIVER_TEST_POSTGRES_DSN`, the script runs a
connectivity/permissions preflight and verifies the schema is present; set
`BITRIVER_TEST_POSTGRES_RUN_MIGRATIONS=1` to have it apply the migrations
directly to that database when needed. If Docker is unavailable and
`BITRIVER_TEST_POSTGRES_DSN` is also unset, the harness exits with an error
explaining how to proceed. The script forces an offline module mode
(`GOPROXY=off GOSUMDB=off GOFLAGS=-mod=vendor`) so vendored replacements stay
intact and `go.mod`/`go.sum` remain untouched:

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
preinstall. `npm run test:playwright` builds the app and launches `npm run
start:test` unless you override the target host with `PLAYWRIGHT_BASE_URL`; in
either case, the specs mock the API to stay deterministic (for example,
`tests/channel-chat-playback.spec.ts` exercises chat authentication edge cases
and HLS player ready/error states, `tests/creator-uploads.spec.ts` walks the
upload manager through failed and recovered API calls, and
`tests/creator-schedule.spec.ts` validates ingest retries and stream title
updates alongside `tests/stream-playback.spec.ts`, which stubs playback
metadata and chat responses).
