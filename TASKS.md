# TASKS

## Scoped change: supported production runtime baselines (#1295)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Establish supported baselines and migration boundaries
  - Acceptance criteria:
    - `PLAN.md` records selected versions, dependency-source policy, risks, tests, and boundaries.
    - `SPEC.md` identifies supported runtimes and production dependency provenance as the current target.
    - Existing Docker, CI, installer, release, viewer, and local replacement paths are inventoried before implementation.
  - Check:
    - Read-only Docker, CI, installer, release, viewer, and replacement-path inventory completed.
    - Official support policy and registry metadata reviewed for the selected runtime and tool versions.
    - `git diff --check -- PLAN.md TASKS.md SPEC.md` passed.

- [x] Task 2 - Align Go toolchains and production dependency provenance
  - Acceptance criteria:
    - `go.mod`, CI, Dockerfiles, installers, and contributor docs agree on Go 1.26.
    - One tested helper creates the production modfile by removing every local `third_party` replacement.
    - Release binaries and Postgres-capable images fail inspection if pgx is stubbed or any local replacement is linked.
    - Direct upstream Go dependencies and checksums are refreshed intentionally.
  - Check:
    - `go test ./... -count=1 -timeout=120s` passed under the pinned `golang:1.26.5` container with offline module settings.
    - A Go 1.26.5 upstream-graph build of `cmd/server` passed and `verify-production-binary --require-module github.com/jackc/pgx/v5` confirmed real pgx with no local replacements.
    - Production-module, binary-inspection, runtime-alignment, Redis chat/session, and scripts suites passed.
    - Shell syntax, Go workflow consistency, checksum refresh, and `git diff --check` passed.

- [x] Task 3 - Upgrade the viewer runtime and toolchain
  - Acceptance criteria:
    - Viewer uses Node 24, Next.js 16, React 19, TypeScript 6, ESLint 9 flat config, and compatible maintained test tooling.
    - Removed commands and deprecated configuration are replaced.
    - Lockfile review and a blocking high-severity npm audit pass.
  - Check:
    - Clean `npm ci` completed under Node 24 and the reviewed lockfile resolves Next.js 16.2.10 with React 19.2.7.
    - `npm run lint` passed with zero warnings and all 214 Jest tests passed.
    - `npm audit --audit-level=high` passed; only two moderate PostCSS findings remain and npm offers no non-breaking fix for the current Next.js line.
    - `npm run build` passed compilation, TypeScript 6 validation, static generation, and route generation after the Next.js 16 migrations.

- [x] Task 4 - Preserve viewer routing and standalone deployment
  - Acceptance criteria:
    - Next.js request API migrations compile without unsafe casts or duplicate route state.
    - `/viewer` base-path behavior, proxy navigation, and standalone Docker packaging remain functional.
    - Playwright coverage exercises critical public and authenticated shell paths.
  - Check:
    - `npm run build` passed Next.js 16 compilation, TypeScript validation, route generation, and static prerendering.
    - All 36 Playwright tests passed with Chromium 149, including desktop/mobile navigation, HLS/native playback, chat, auth, creator, upload, and accessibility paths.
    - The Node 24 viewer Dockerfile built successfully with a 7.78 kB incremental context after adding `.dockerignore` (the first build sent 530 MB of local artifacts).
    - A temporary container reported Node v24.18.0 and returned HTTP 200 with BitRiver content from `/viewer`.

- [x] Task 5 - Enforce dependency maintenance policy
  - Acceptance criteria:
    - Dependabot creates grouped, reviewable Go and viewer updates.
    - High/critical dependency findings block CI and release unless a complete time-bounded exception exists.
    - Support windows, update cadence, provenance rules, and operator upgrade notes are documented.
  - Check:
    - Workflow policy, runtime-alignment, Dependabot grouping, and exception-policy tests passed under Go 1.26.5.
    - Changed Dependabot and GitHub Actions YAML parsed successfully; shell syntax checks passed.
    - `npm audit --audit-level=high` passed with only the two documented moderate PostCSS findings and no valid non-breaking fix.
    - Pinned `govulncheck v1.6.0` completed every root/offline-mirror scan with zero reachable findings and an empty exception baseline.
    - The live scan exposed a v1.6 pretty-printed JSON stream/schema change; the parser now decodes concatenated objects and supports both module and package identities.

- [x] Task 6 - Verify, publish, and close #1295
  - Acceptance criteria:
    - Full repository, Postgres, viewer, Compose, quickstart, and release smoke gates pass or blockers are recorded precisely.
    - Pull-request checks pass before squash merge.
    - The issue is closed by the merged pull request and local `main` is synchronized.
  - Check:
    - Full offline Go 1.26.5 suite passed, including the offline mirrors and new runtime/dependency policy tests.
    - Full upstream production module suite passed with Postgres tags against the migrated canonical Postgres service.
    - Viewer lint, all 214 Jest tests, blocking high-severity audit, production build, all 36 Playwright tests, and standalone Node 24 container smoke passed.
    - Canonical Compose config, all five custom image builds, service health, API/controller/transcoder probes, and `/viewer` proxy smoke passed; the final rebuilt API image remained healthy.
    - Pinned `govulncheck v1.6.0` completed with zero reachable findings; generated contract documentation and CRLF-safe `--check` passed.
    - Fresh continuation checks on 2026-07-16 passed: full offline Go 1.26.5 suite and architecture graph in the pinned container; viewer lint, 214 Jest tests, high-severity audit, and production build under Node 24; Compose config and contract invariants; and an isolated full quickstart image/build/health/endpoint smoke.
    - The one-line `./scripts/verify.sh --viewer` wrapper could not run directly because the Windows `bash` shim targets an uninstalled WSL distribution and the host Go 1.25.6 binary is older than the module baseline. Its constituent checks were run with Git Bash, the bundled Python runtime, Node 24, Docker, and the pinned Go 1.26.5 container; the isolated quickstart avoided touching the root `.env` or persistent volumes.
    - PR #1312's first CI run exposed a staged-release fixture drift: the wizard requires `bitriver-live` plus `bootstrap-admin`, while the regression test created `server` plus `bootstrap-admin`. The fixture now uses the packaged binary name and `scripts/test-wizard-release.sh` passes locally.
    - PR #1312's second CI run passed unit, verification, Compose, quickstart, and image-vulnerability gates before the Postgres-tagged offline lane reached a new real-driver-only acquire-timeout test. The test now follows the existing pgx-stub guard: it skips explicitly on the offline graph and passes against isolated Postgres with the generated upstream production modfile.
    - PR #1312's third CI run passed the Ubuntu unified gate, image scan, and macOS/Linux quickstart checks before Windows exposed that its no-op Go entrypoint ran without the pinned toolchain. The quickstart matrix now installs `.go-version` on Windows before invoking the intentionally offline `GOTOOLCHAIN=local` wrapper; CI-contract checks, the scripts Go tests, and the exact `quickstart.ps1 -ValidateOnly` path passed locally with Go 1.26.5.
    - The next Windows Go matrix run confirmed the pinned quickstart fix, then exposed LF-only matching in the new module-baseline assertions. The checks now normalize CRLF before matching the exact Go directive, include LF/CRLF regression cases, and pass in the full `scripts` package with the pinned Windows Go 1.26.5 toolchain.
    - The subsequent run passed every backend, image, quickstart, and policy job before one homepage Playwright assertion observed both the streamed `Syncing` fallback and settled `Relay ready` hero during Suspense reveal. Waiting for settled text alone still matched hidden and visible transition copies on the hosted runner, so the assertion now waits for the visible settled relay state before requiring one total hero. The refined regression passed 20/20 repeated Chromium runs under eight workers, all four homepage layout cases in parallel, and the full 36-test Playwright suite.
    - CI run 29552998448 passed every applicable job. PR #1312 was marked ready and squash-merged as `385b0c9e559a4162e79b060a25d77ac29fbdd790`; issue #1295 closed automatically, and local `main` was fast-forwarded to the same commit with unrelated worktree changes preserved and unstaged.

### Execution log
- Task 1 analysis:
  - Confirmed the repository declares Go 1.21 while production Go containers already drifted to 1.25.7 and the host uses 1.25.6.
  - Confirmed Next.js 13.5.11 is unsupported, Node 20 is used in viewer CI/containers, and `next lint` must be replaced for Next.js 16.
  - Found three App Router request-API call sites requiring migration and no custom webpack surface.
  - Found eight local Go replacements while the API Dockerfile drops only pgx, puddle, and x/text; release binary jobs currently use the local module graph with `GOPROXY=off`.
  - Selected supported Go 1.26.5, Node 24 LTS, Next.js 16.2.10, React 19.2.7, TypeScript 6.0.3, and ESLint 9.39.5 baselines from official support policy and package metadata.
- Task 2 implementation:
  - Added an exact `.go-version`, aligned all Go builder images and offline mirror modules, refreshed direct upstream dependencies and checksums, and centralized CI setup on the pinned toolchain.
  - Replaced partial Docker `-dropreplace` lists with a tested production-module generator that removes every local mirror without mutating `go.mod`.
  - Added build-metadata inspection to release binaries, source installers, and all Go Dockerfiles; Postgres artifacts additionally require an upstream pgx module.
  - The first real-module compile exposed that both Redis queue and session-store code had been written against a divergent local stub API. Aligned application code, the mirror, and test doubles with upstream `go-redis` command semantics.
  - Added a shared Unix Go minimum-version guard, equivalent PowerShell enforcement, runtime drift tests, and production dependency provenance documentation.
- Task 3 implementation:
  - Upgraded the viewer to Node 24, Next.js 16.2.10, React 19.2.7, TypeScript 6.0.3, ESLint 9 flat config, Jest 30, Playwright 1.61.1, and compatible current types/testing libraries.
  - Kept the React Compiler-specific `set-state-in-effect` rule disabled until compiler adoption is scoped, while fixing stricter ref, immutability, memoization, and dependency findings.
  - Replaced browser-global mutation in tests with a small navigation boundary, enabled the React 19 act environment, isolated server API tests under the Node Jest environment, and ignored generated standalone output in Jest discovery.
  - Migrated dynamic client routes to `useParams`, made server `searchParams` asynchronous, and added stable Suspense boundaries for search-parameter consumers.
- Task 4 implementation:
  - Added stable route-level Suspense boundaries for every `useSearchParams` consumer and preserved a branded navigation fallback during hydration.
  - Fixed a real Browse reset race by treating refs as last-observed URL state and using Next.js 16's native History API integration for same-page query navigation.
  - Updated playback browser assertions to accept both native HLS URLs and hls.js blob sources, which are both valid under current Chromium.
  - Added a viewer `.dockerignore`, reducing the Docker context from 530 MB of local dependencies/build output to a minimal source context.
- Task 5 implementation:
  - Added grouped weekly Dependabot updates for Go production dependencies and separate viewer runtime/tooling review lanes.
  - Made high-severity npm audits blocking in viewer, main CI, and release artifact jobs; critical findings remain unconditionally unreleasable.
  - Upgraded the pinned Go vulnerability scanner, removed the obsolete Go 1.21 exception, and made baseline entries complete, owner-assigned, tracked, and expiry-validated.
  - Hardened the scanner parser against the actual v1.6 JSON stream and OSV package schema after exercising the live tool, not only workflow fixtures.
- Task 6 verification and hardening:
  - Replaced the chat queue's oversized Redis dependency with a queue-owned command interface and made its RESP2 wire contract explicit.
  - Corrected the Redis test server to return unsupported-command errors without closing healthy connections, with a raw protocol regression test.
  - Ran real-pgx storage tests and fixed cleanup ordering, lazy-row context lifetime, nested appeal query row ownership, subscription status/column parity, and stub-only test isolation.
  - Fixed contract-document generation so CRLF templates do not pollute generated values or fail checks solely because of checkout line endings.
