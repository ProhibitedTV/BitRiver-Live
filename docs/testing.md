# Testing BitRiver Live

This document collects the commands the project uses in CI so contributors can
run the same suites locally before opening a pull request. See
`docs/testing-status.md` for a living summary of flaky suites and gaps that need
coverage.

## Verify prerequisites

`./scripts/verify.sh` requires these tools on `PATH`:

- `go` (for `go test ./...`)
- Python 3 (`python3`, `python`, or `py -3`) used by the contract/doc validation helpers
- `docker` (optional; Docker-dependent verify phases (`docker compose ... config`, `./scripts/test-quickstart.sh`) are skipped when Docker is unavailable)
- `node` + `npm` (optional; required only when viewer lint/test checks are selected)

If no usable Python 3 interpreter is available, `./scripts/verify.sh` fails fast with a clear prerequisite error before running the verify sequence.

## Pull-request merge enforcement

The CI orchestrator keeps expensive checks path-selective, then always runs one
stable `Merge gate`. That job compares every child result with the complete
changed-file classification. A relevant job that failed, was cancelled, or was
unexpectedly skipped fails the aggregate; an unrelated skipped job is reported
as an expected skip. `Merge gate` is the required branch-protection context, so
contributors do not need to infer safety from a changing list of child jobs.

The gate also validates the pull-request release scorecard. Warnings remain
advisory for docs/planning-only paths. They become blocking when the scorecard
selects medium/high risk or the diff touches code, CI, dependencies,
deployment, packaging, or operator workflow paths.

Run the focused aggregate fixtures locally with:

```bash
bash ./scripts/test-ci-merge-gate.sh
```

GitHub writes the result table to the job summary and retains
`merge-gate-<run>-<attempt>` for 14 days. This merge enforcement does not prove
or authorize stable release promotion; immutable promotion remains a separate
release gate.

## Test taxonomy and single entrypoints

Use these category entrypoints from the repository root:

- **Unit:** `./scripts/test-unit.sh`
  - Runs `go test ./... -count=1 -timeout=120s` with offline Go env defaults (`GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off`).
  - CI: the manual [`.github/workflows/go-unit-tests.yml`](../.github/workflows/go-unit-tests.yml) full matrix and the path-gated Ubuntu/cross-platform jobs in [`.github/workflows/ci.yml`](../.github/workflows/ci.yml).
- **Integration umbrella:** `./scripts/test-integration.sh`
  - Wraps `./scripts/test-postgres.sh` and `./scripts/test-quickstart.sh`.
  - Docker-dependent checks are skipped with explicit messages when Docker is unavailable.
  - The production golden path remains opt-in (`--production-golden-path` or `BITRIVER_TEST_ALL_PRODUCTION_GOLDEN_PATH=1`). When enabled it owns the canonical quickstart lifecycle instead of starting the same stack twice.
- **Postgres integration (tagged):** `./scripts/test-postgres.sh`
  - Runs storage integration tests behind the `postgres` tag using Docker or `BITRIVER_TEST_POSTGRES_DSN`.
  - CI: the reusable/manual [`.github/workflows/postgres-tests.yml`](../.github/workflows/postgres-tests.yml), the changed-path Ubuntu umbrella gate in [`.github/workflows/ci.yml`](../.github/workflows/ci.yml), and the same reusable Postgres gate called by [`.github/workflows/release.yml`](../.github/workflows/release.yml).
- **Postgres migration lifecycle:** `./scripts/test-postgres-migrations.sh`
  - Uses a disposable Postgres 15 container to prove fresh apply, previous-schema upgrade, no-op rerun, checksum drift refusal, failed retry, interrupted-state acknowledgment, and non-sensitive status output.
  - Runs automatically inside `./scripts/verify.sh` when Docker is available.
- **Quickstart smoke:** `./scripts/test-quickstart.sh`
  - Validates compose rendering/healthcheck wiring and boots the quickstart stack.
  - CI: [`.github/workflows/quickstart-smoke.yml`](../.github/workflows/quickstart-smoke.yml) is the reusable/manual source for cross-platform entrypoint checks and targeted Compose smoke; the CI orchestrator calls its entrypoint matrix while the unified Ubuntu gate owns changed-path Compose smoke.
- **Release bundle:** `./scripts/test-release-bundle.sh`
  - Stages the source-free release allowlist outside the checkout in a path
    containing spaces, checks asset parity, rejects deployment-generated
    credential files, and proves an exact candidate tag is written to all five
    staged first-party image defaults without changing the source env.
- **Ubuntu host lifecycle:** `./scripts/test-compose-host-installer.sh`
  - Exercises rerunnable install/upgrade, separated configuration/data, bounded unit rendering, safe uninstall, rejected purge, and confirmed purge under an isolated root prefix.
  - Runs automatically from `./scripts/verify.sh` on Linux.
- **Linux package generation:** `BITRIVER_INSTALL_NFPM=1 ./scripts/test-linux-packages.sh`
  - Installs the pinned nFPM version when opted in, then builds stable and
    separately tag-stamped prerelease amd64/arm64 `.deb`/`.rpm` packages from
    the canonical bundle. On Linux it extracts the prerelease `.deb` and checks
    all five installed image tags.
  - The release workflow separately installs/removes the amd64 package in Ubuntu 24.04, Debian 12, and Rocky Linux 9 containers before publication.
- **Deploy smoke:** `./scripts/deploy-smoke.sh`
  - Boots the compose stack with an isolated temporary project name, waits for API `/readyz`, prints a short PASS/FAIL summary, and always tears down.
  - Operator-focused one-command confidence check before/after deploy changes.
- **Ingest storage/controller integration:** `./scripts/test-ingest-storage.sh`
  - Runs the cheap focused storage and HTTP-controller lifecycle guard without claiming real media-service coverage.
- **Production golden path:** `./scripts/test-production-golden-path.sh --stack quickstart --client docker`
  - Boots the canonical Compose stack, creates real account/channel state, publishes deterministic 1080p RTMP with audio, requires advancing and decodable OME and transcoder HLS, observes offline state, exercises chat/moderation, uploads and decodes a published VOD, checks aggregate health, scans retained evidence, and tears down.
  - `scripts/test-ingest-e2e.sh` remains a compatibility alias for branch protection and existing callers; it now runs this real product gate.
  - CI: [`.github/workflows/ingest-e2e.yml`](../.github/workflows/ingest-e2e.yml) and the path-gated Ubuntu job in [`.github/workflows/ci.yml`](../.github/workflows/ci.yml).
- **Viewer integration / Playwright:** `npm --prefix web/viewer run test:integration`
  - Runs viewer lint + Jest + Playwright integration checks.
  - CI: [`.github/workflows/viewer-ci.yml`](../.github/workflows/viewer-ci.yml) is the reusable/manual source called by [`.github/workflows/ci.yml`](../.github/workflows/ci.yml); release viewer validation remains in [`.github/workflows/release.yml`](../.github/workflows/release.yml).

Run everything with the umbrella entrypoint:

```bash
./scripts/test-all.sh
```

`./scripts/test-all.sh` runs `./scripts/test-unit.sh`, the repository verifier, selected Docker integration gates, and viewer integration when Node/Playwright tooling is available. It skips unavailable Docker/Node/Playwright-dependent steps with explicit skip messages.

The production golden path is intentionally opt-in in the umbrella script. Enable it with either:

```bash
./scripts/test-all.sh --production-golden-path
```

or:

```bash
BITRIVER_TEST_ALL_PRODUCTION_GOLDEN_PATH=1 ./scripts/test-all.sh
```

The old `--ingest-e2e` and `BITRIVER_TEST_ALL_INGEST_E2E=1` names remain
accepted as compatibility aliases. New automation should use the accurate
production-golden-path names.

## Self-hosted product acceptance checklist

Before calling a release candidate a working self-hosted live streaming website,
pair the automated gates with a real happy-path rehearsal against the Compose
stack:

- Render and boot the canonical Compose stack from the documented quickstart.
- Sign up as a viewer when self-signup is enabled, then sign out and sign back in.
- Create or manage a creator channel and copy the displayed RTMP ingest settings.
- Add an upcoming stream schedule in the creator Go Live dashboard and confirm it appears on the public channel Schedule tab.
- Start a stream from a real RTMP encoder, confirm the channel moves to live, and watch playback from a separate viewer session.
- Send chat messages as a signed-in viewer, confirm live updates arrive, and submit a report against another user's message.
- Stop the stream and confirm the channel returns offline without leaving an active session behind.
- Confirm a completed recording stays unpublished until the creator publishes it, then verify the VOD appears on the channel and Videos pages.
- Run backup/restore or deploy smoke guidance when the change touches operations, persistence, or release packaging.

Record the exact environment, commands, encoder settings, and observed URLs in
the release or PR notes so the rehearsal is reproducible.

## Dependency source of truth

Offline Go builds in this repository use `go.mod` `replace` directives that point
at checked-in modules under `third_party/`. Do **not** use `vendor/` as a second
copy of the same modules. Keep `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off` for
offline runs, and use `./scripts/check-dependency-source.sh` to verify the tree
contains no duplicate third-party modules across `third_party/` and `vendor/`.

## pgx sourcing modes

BitRiver Live treats Go dependency wiring as an explicit two-mode contract:

- **Offline mode**: the default local verification path uses every checked-in `third_party` replacement with `GOPROXY=off`.
- **Production mode**: `go run ./cmd/tools/production-module --output go.production.mod` creates an isolated module file that removes every local replacement. Production builds download that complete graph with `go mod download -modfile=go.production.mod all` and compile with `-modfile=go.production.mod`.

Use both guards whenever `BITRIVER_LIVE_STORAGE_DRIVER=postgres` is expected:

```bash
GOFLAGS="-modfile=$PWD/go.production.mod" ./scripts/check-postgres-pgx.sh postgres
go run ./cmd/tools/verify-production-binary --require-module github.com/jackc/pgx/v5 ./bitriver-live
```

The runtime guard rejects stub mode before compilation. The artifact guard reads Go build metadata and rejects any local replacement or a missing pgx module. Release Dockerfiles and workflows run these checks automatically.

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

The CI orchestrator calls the reusable viewer workflow for `web/viewer/**` and
immediate contract/runtime paths (`deploy/.env.example`, `internal/api/**`, and
`internal/server/**`) because the viewer consumes those payloads and settings
directly. Treat backend contract changes as cross-surface updates and run the
manual viewer workflow when validating a branch outside a pull request.

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
CI (`v1.6.0`, matching `.github/workflows/go-unit-tests.yml`) instead of
`@latest`; pinning keeps local and CI evidence comparable on Go 1.26.

`./scripts/run-govulncheck.sh` enforces the current vulnerability policy for the
pinned Go 1.26.5 toolchain and writes structured artifacts under
`.artifacts/govulncheck/<timestamp>/`:

- `raw/*.jsonl`: full per-scan govulncheck JSON output for audit/history.
- `findings.json`: normalized findings with module + scan + platform metadata.
- `new-findings.json`: only new disallowed findings (compared to baseline).
- `summary.json`: counts plus categorized finding lists.

Execution policy:

- Reachable vulnerabilities in modules or the Go standard library are disallowed, but only
  fail the run when they are **not** listed in
  `scripts/govulncheck-baseline.json`.
- Baseline matching includes platform (`goos`/`goarch`) so OS-specific
  advisories (for example Windows-only findings) report meaningful matrix
  deltas.

Use the helper script to run the root module scan plus checks for each replaced
third-party module:

```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go install golang.org/x/vuln/cmd/govulncheck@v1.6.0
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
CI orchestrator calls this same reusable workflow instead of embedding a second
scanner implementation. It keeps **CRITICAL** gating enabled for first-party images
(`ghcr.io/prohibitedtv/*` and local `bitriver-live/*` builds) and runs a
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

The cheap storage + HTTP controller integration guard remains independently
callable:

```bash
./scripts/test-ingest-storage.sh
```

It is not product E2E. For production acceptance, run the real Compose gate:

```bash
./scripts/test-production-golden-path.sh \
  --stack quickstart \
  --client docker
```

The gate retains only the versioned
`.artifacts/production-golden-path/production-golden-path.json` report after
scanning it against per-run account, password, token, and stream-key sentinels.
Passing requires content-level 1920x1080 decode and advancing playlists, not
only healthy containers or successful manifest responses. `--stack running`
reuses the same assertions against a deliberately prepared staging stack; it
does not own or tear down that deployment.

## Quickstart/Compose smoke

Run the compose smoke guard to ensure the default `.env` and `deploy/docker-compose.yml` still render and that the tracked health probes stay wired:

`./scripts/verify.sh` now validates compose config with an explicit env file, preferring root `.env` and falling back to `deploy/.env.example` when `.env` is absent, so missing environment variables surface during verification. When Docker is available, verify then runs `./scripts/test-quickstart.sh` as an integration/smoke phase immediately after compose validation. Both Docker-dependent phases emit explicit skip messages when Docker is unavailable.

Host Go tests remain offline (`GOPROXY=off GOSUMDB=off`). Clean production-module downloads inside quickstart container builds use `https://proxy.golang.org,direct` with `sum.golang.org` so a transient vanity-domain outage does not make Docker verification depend on a direct `gopkg.in` fetch. CI image scanning applies the same build-only policy. To test a different module mirror without changing runtime containers, set `BITRIVER_DOCKER_GOPROXY` and `BITRIVER_DOCKER_GOSUMDB` for `./scripts/test-quickstart.sh`.

```bash
./scripts/test-quickstart.sh
```

When no `.env` exists in the repository root, the helper seeds one with the same quickstart fixture defaults (including `BITRIVER_LIVE_MODE=production` to match `deploy/check-env.sh` validation), renders `docker compose config`, and verifies that the API, transcoder, OME, SRS, Postgres, and Redis healthchecks still point at their expected endpoints. It then boots the compose stack with the seeded `.env`, waits for all healthchecks to go green, curls the API health endpoint and viewer page, and tears the stack down via `docker compose down -v` so nothing is left behind. The script also invokes the Go renderer (`go run ./cmd/bitriver ome render`, or the `scripts/render-ome-config.sh` wrapper) against the seeded `.env` and fails fast when `deploy/ome/Server.generated.xml` is stale or missing required `<Bind>`, `<IP>`, or control credential values so the tracked compose mount stays fresh. It cleans up the temporary `.env` after the run.

CI calls [`.github/workflows/quickstart-smoke.yml`](../.github/workflows/quickstart-smoke.yml) with Compose smoke disabled so the reusable `quickstart-entrypoints` matrix covers Ubuntu/macOS shell usage plus Windows PowerShell help/`-ValidateOnly` without starting the same stack twice. Manual dispatch keeps Compose smoke enabled by default; changed-path pull requests run it through the unified Ubuntu gate.

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

CI calls the same reusable [`.github/workflows/docs-consistency.yml`](../.github/workflows/docs-consistency.yml) definition when documentation inputs change.

Go workflow reproducibility is guarded by [`.github/workflows/go-workflow-consistency.yml`](../.github/workflows/go-workflow-consistency.yml), which runs [`scripts/check-go-workflow-config.sh`](../scripts/check-go-workflow-config.sh) to enforce SHA-pinned `actions/setup-go@<40-hex-sha>` usage (either directly in workflows or through the approved `./.github/actions/setup-go` composite action that pins `actions/setup-go` by SHA), `go-version-file: .go-version`, and offline Go env defaults (`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`) across core verification workflows.

## Postgres storage layer

Before storage behavior, the ledger-aware deployment runner has its own real-Postgres lifecycle gate:

```bash
./scripts/test-postgres-migrations.sh
```

The test never uses the developer's database. It creates and removes a uniquely named `postgres:15-alpine` container, exercises `deploy/postgres-migrate.sh`, and fails if plan mutates an uninitialized database, applied files rerun, history edits pass, failed/interrupted state becomes healthy silently, or status output exposes the test credential.

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

Install dependencies with a lockfile-faithful install and execute the lint and integration harnesses:

```bash
cd web/viewer
npm ci
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
