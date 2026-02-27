# Testing BitRiver Live

This document collects the commands the project uses in CI so contributors can
run the same suites locally before opening a pull request. See
`docs/testing-status.md` for a living summary of flaky suites and gaps that need
coverage.

## Verify prerequisites

`./scripts/verify.sh` requires these tools on `PATH`:

- `go` (for `go test ./...`)
- `python3` (used by `./scripts/check-contract-invariants.sh` to validate generated artifact references in `docs/contract.md`)
- `docker` (optional; Docker-dependent verify phases (`docker compose ... config`, `./scripts/test-quickstart.sh`) are skipped when Docker is unavailable)
- `node` + `npm` (optional; required only when viewer lint/test checks are selected)

If `python3` is missing, `./scripts/verify.sh` now fails fast with a clear prerequisite error before running the verify sequence.

## Test taxonomy and single entrypoints

Use these category entrypoints from the repository root:

- **Unit:** `./scripts/test-unit.sh`
  - Runs `go test ./... -count=1 -timeout=120s` with offline Go env defaults (`GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off`).
  - CI: [`.github/workflows/go-unit-tests.yml`](../.github/workflows/go-unit-tests.yml) and the `go-tests` job in [`.github/workflows/ci.yml`](../.github/workflows/ci.yml).
- **Integration umbrella:** `./scripts/test-integration.sh`
  - Wraps `./scripts/test-postgres.sh` and `./scripts/test-quickstart.sh`.
  - Docker-dependent checks are skipped with explicit messages when Docker is unavailable.
  - Ingest e2e remains opt-in (`--ingest-e2e` or `BITRIVER_TEST_ALL_INGEST_E2E=1`) and runs `./scripts/test-ingest-e2e.sh` when enabled.
- **Postgres integration (tagged):** `./scripts/test-postgres.sh`
  - Runs storage integration tests behind the `postgres` tag using Docker or `BITRIVER_TEST_POSTGRES_DSN`.
  - CI: [`.github/workflows/postgres-tests.yml`](../.github/workflows/postgres-tests.yml), plus `postgres-tests` in [`.github/workflows/ci.yml`](../.github/workflows/ci.yml) and release validation in [`.github/workflows/release.yml`](../.github/workflows/release.yml).
- **Quickstart smoke:** `./scripts/test-quickstart.sh`
  - Validates compose rendering/healthcheck wiring and boots the quickstart stack.
  - CI: [`.github/workflows/quickstart-smoke.yml`](../.github/workflows/quickstart-smoke.yml) and `quickstart-smoke` in [`.github/workflows/ci.yml`](../.github/workflows/ci.yml).
- **Deploy smoke:** `./scripts/deploy-smoke.sh`
  - Boots the compose stack with an isolated temporary project name, waits for API `/readyz`, prints a short PASS/FAIL summary, and always tears down.
  - Operator-focused one-command confidence check before/after deploy changes.
- **Ingest e2e:** `./scripts/test-ingest-e2e.sh`
  - Exercises the ingest control-plane/storage lifecycle guard.
  - CI: [`.github/workflows/ingest-e2e.yml`](../.github/workflows/ingest-e2e.yml) and `ingest-e2e` in [`.github/workflows/ci.yml`](../.github/workflows/ci.yml).
- **Viewer integration / Playwright:** `npm --prefix web/viewer run test:integration`
  - Runs viewer lint + Jest + Playwright integration checks.
  - CI: [`.github/workflows/viewer-ci.yml`](../.github/workflows/viewer-ci.yml), `viewer-tests` in [`.github/workflows/ci.yml`](../.github/workflows/ci.yml), and release viewer validation in [`.github/workflows/release.yml`](../.github/workflows/release.yml).

Run everything with the umbrella entrypoint:

```bash
./scripts/test-all.sh
```

`./scripts/test-all.sh` runs `./scripts/test-unit.sh` + `./scripts/test-integration.sh` and then viewer integration when Node/Playwright tooling is available. It skips unavailable Docker/Node/Playwright-dependent steps with explicit skip messages.

Ingest e2e is intentionally opt-in in the umbrella script. Enable it with either:

```bash
./scripts/test-all.sh --ingest-e2e
```

or:

```bash
BITRIVER_TEST_ALL_INGEST_E2E=1 ./scripts/test-all.sh
```

## Dependency source of truth

Offline Go builds in this repository use `go.mod` `replace` directives that point
at checked-in modules under `third_party/`. Do **not** use `vendor/` as a second
copy of the same modules. Keep `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off` for
offline runs, and use `./scripts/check-dependency-source.sh` to verify the tree
contains no duplicate third-party modules across `third_party/` and `vendor/`.

## pgx sourcing modes

BitRiver Live now treats pgx wiring as an explicit two-mode contract:

- **Stub mode (`stub`)**: default local/offline mode using `third_party/github.com/jackc/pgx/v5`. This keeps unit and JSON-driver workflows reproducible without reaching external module sources.
- **Release mode (`real`)**: required for Postgres-capable binaries/images. Release/build jobs must point `github.com/jackc/pgx/v5` at a non-stub module source before compiling Postgres artifacts (for example, a maintained vendored real pgx mirror under `third_party/` or a controlled CI-only replace strategy), and must also unpin stubbed transitive replacements (for example `golang.org/x/text`) before running `go mod download`.

Use the guard below whenever `BITRIVER_LIVE_STORAGE_DRIVER=postgres` is expected:

```bash
./scripts/check-postgres-pgx.sh postgres
```

The check fails if `pgx.IsStub` is `true`, which prevents publishing binaries/images that would boot with Postgres configured but only have stubbed driver wiring.

## Go API

Run the fast unit suite (storage unit tests, REST handlers, chat flows) from the
repository root with the same environment guardrails CI enforces. Setting
`GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off` ensures the local Go toolchain is
used without reaching out to the network, which keeps results reproducible and
matches the locked-down CI runners. The `-count=1 -timeout=120s` flags prevent
test caching and match CI's 120-second deadline for each package. Use the same
timeout locally to avoid flakes on slower machines:

```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s
```

Validate architecture-layer import direction (as defined in
`docs/architecture.md`) with:

```bash
./scripts/check-architecture-deps.sh
```


Monitoring config syntax is validated in CI before release packaging. Run the same guard locally after editing
`deploy/monitoring/` assets:

```bash
./scripts/check-monitoring-config.sh
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

Viewer CI (`.github/workflows/viewer-ci.yml`) intentionally triggers for both
`web/viewer/**` and backend contract-facing paths (`internal/api/**`,
`internal/domain/**`, and `internal/api/viewer_contract_test.go`) because the
viewer consumes those API payload shapes directly. Treat backend contract
changes as cross-surface updates and run viewer checks when touching those
paths.

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
the default module mode so the results track the pinned `third_party/`
replacements declared in `go.mod`. Install the same pinned tool version used in
CI (`v1.1.3`, matching `.github/workflows/go-unit-tests.yml`) instead of
`@latest`; pinning is required so CI's Go 1.21 runners always use a compatible
`govulncheck` release.

`./scripts/run-govulncheck.sh` enforces the current vulnerability policy for the
pinned Go 1.21 toolchain and now writes structured artifacts under
`.artifacts/govulncheck/<timestamp>/`:

- `raw/*.jsonl`: full per-scan govulncheck JSON output for audit/history.
- `findings.json`: normalized findings with module + scan + platform metadata.
- `new-findings.json`: only new disallowed findings (compared to baseline).
- `summary.json`: counts plus categorized finding lists.

Execution policy:

- Reachable vulnerabilities in **non-stdlib modules** are disallowed, but only
  fail the run when they are **not** listed in
  `scripts/govulncheck-baseline.json`.
- Reachable vulnerabilities that affect only the Go `stdlib` are logged as
  informational while the repository remains on Go 1.21.
- Baseline matching includes platform (`goos`/`goarch`) so OS-specific
  advisories (for example Windows-only findings) report meaningful matrix
  deltas.
- Once the toolchain target in `go.mod` is raised beyond 1.21, stdlib findings
  return to fail-closed behavior automatically.

Use the helper script to run the root module scan plus checks for each replaced
third-party module:

```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go install golang.org/x/vuln/cmd/govulncheck@v1.1.3
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off ./scripts/run-govulncheck.sh
```

Baseline entries are managed in `scripts/govulncheck-baseline.json` and support
wildcards (`"*"`) per field. Each entry can scope by advisory ID, affected
module, scan label, and platform:

```json
{
  "version": 1,
  "entries": [
    {
      "id": "GO-0000-0000",
      "module": "example.com/module",
      "scan": "root module",
      "goos": "windows",
      "goarch": "amd64"
    }
  ]
}
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

`./scripts/verify.sh` now validates compose config with an explicit env file, preferring root `.env` and falling back to `deploy/.env.example` when `.env` is absent, so missing environment variables surface during verification. When Docker is available, verify then runs `./scripts/test-quickstart.sh` as an integration/smoke phase immediately after compose validation. Both Docker-dependent phases emit explicit skip messages when Docker is unavailable.

```bash
./scripts/test-quickstart.sh
```

When no `.env` exists in the repository root, the helper seeds one with the same quickstart fixture defaults (including `BITRIVER_LIVE_MODE=production` to match `deploy/check-env.sh` validation), renders `docker compose config`, and verifies that the API, transcoder, OME, SRS, Postgres, and Redis healthchecks still point at their expected endpoints. It then boots the compose stack with the seeded `.env`, waits for all healthchecks to go green, curls the API health endpoint and viewer page, and tears the stack down via `docker compose down -v` so nothing is left behind. The script also invokes the Go renderer (`go run ./cmd/bitriver ome render`, or the `scripts/render-ome-config.sh` wrapper) against the seeded `.env` and fails fast when `deploy/ome/Server.generated.xml` is stale or missing required `<Bind>`, `<IP>`, or control credential values so the tracked compose mount stays fresh. It cleans up the temporary `.env` after the run.

CI enforces the same guardrails in [`.github/workflows/quickstart-smoke.yml`](../.github/workflows/quickstart-smoke.yml): keep both the `quickstart-entrypoints` matrix job (Ubuntu/macOS shell usage + static checks, Windows PowerShell help + `-ValidateOnly` no-op path) and the Ubuntu `quickstart-smoke` compose job enabled as required pull-request checks so script drift is blocked before merge.

For a fast operator confidence check outside the heavier quickstart suite, run:

```bash
./scripts/deploy-smoke.sh
```

This helper uses the same env-file selection order (`.env`, then `deploy/.env.example`), starts compose under a temporary project name, polls `http://localhost:${BITRIVER_LIVE_PORT:-8080}/readyz`, prints PASS/FAIL, and always tears down containers/volumes on exit.


## Docs installer consistency

Run the installer-language guard to keep shipped milestones consistent across release and deployment docs:

```bash
./scripts/check-doc-installer-language.sh
```

CI enforces the same check in [`.github/workflows/docs-consistency.yml`](../.github/workflows/docs-consistency.yml).

Go workflow reproducibility is guarded by [`.github/workflows/go-workflow-consistency.yml`](../.github/workflows/go-workflow-consistency.yml), which runs [`scripts/check-go-workflow-config.sh`](../scripts/check-go-workflow-config.sh) to enforce `actions/setup-go@v5`, `go-version-file: go.mod`, and offline Go env defaults (`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`) across the core Go workflows.

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
(`GOPROXY=off GOSUMDB=off`) so local `third_party/` replacements stay intact and
`go.mod`/`go.sum` remain untouched:

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
