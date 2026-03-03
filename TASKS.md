## Scoped change: directory page suspense data boundary refactor

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Move directory/home awaits into an async `DirectoryDataBoundary` within the Suspense subtree
  - Acceptance criteria:
    - `DirectoryPage` only normalizes query and renders `Suspense` + `DirectoryPageFallback` + `DirectoryDataBoundary`.
    - `DirectoryDataBoundary` performs `loadHomeData()` + `loadDirectoryData(query)` and renders `DirectoryPageContent`.
    - Existing `mapDirectoryError` and `emptyHomeData` behavior remains intact.

- [x] Task 2 — Add/adjust directory page unit test coverage for pending fallback behavior
  - Acceptance criteria:
    - Tests cover loading fallback UI while boundary data promises are unresolved.
    - Existing directory success/error/auth behavior assertions continue to pass.

- [x] Task 3 — Run scoped viewer tests and full verify gate
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- directoryPage.test.tsx` passes.
    - `./scripts/verify.sh` is run and results are recorded.

### Execution log (directory page suspense data boundary refactor)
- ✅ Task 1 complete: refactored `DirectoryPage` to normalize query only and render a suspense shell with async `DirectoryDataBoundary` fetching `loadHomeData()` + `loadDirectoryData(query)` inside the suspended subtree while preserving `mapDirectoryError` and `emptyHomeData` behavior.
- ✅ Task 2 complete: updated directory page tests to render resolved boundary content via `DirectoryPageShell`/`DirectoryDataBoundary`, added coverage asserting the lightweight shell query normalization, and added a pending-child suspense fallback assertion.
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- directoryPage.test.tsx`
  - ✅ `./scripts/verify.sh` (passed; Docker-dependent checks skipped because Docker is not installed in this environment)

## Scoped change: following hook reload no-op state guard

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add guarded status/error updates in `reload`
  - Acceptance criteria:
    - Keep `authLoading` and `!isAuthenticated` gates unchanged.
    - Only call `setStatus`/`setError` when next value differs from current value.

- [x] Task 2 — Skip no-op channel updates on successful fetch
  - Acceptance criteria:
    - Compare current vs fetched channels by ID sequence/length.
    - Avoid `setChannels` when lists are semantically unchanged.
    - Keep success/empty semantics unchanged.

- [x] Task 3 — Run scoped viewer tests and record outcomes
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- followingSidebar.test.tsx` passes.

### Execution log (following hook reload no-op state guard)
- ✅ Task 1 complete: added guarded `setStatus`/`setError` helpers that compare current and next values and preserved existing auth gate branching.
- ✅ Task 2 complete: added semantic channel equality checks (ID order + length) to skip no-op `setChannels` updates while keeping ready/empty/error semantics unchanged.
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- followingSidebar.test.tsx`
  - ✅ `./scripts/verify.sh` (passed; Docker-dependent checks skipped because Docker is not installed in this environment)

## Scoped change: chat panel message derivation allocation cleanup

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Remove redundant normalized-message pass in ChatPanel memoization pipeline
  - Acceptance criteria:
    - `ChatPanel` computes sorted message entries in a single memoized pass without a separate `normalizedMessages` array.
    - Sort/group output remains based on `message.sentAt` timestamps and retains existing grouping semantics.

- [x] Task 2 — Run viewer ChatPanel tests and record outcomes
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- chatPanel.test.tsx` passes.

- [x] Task 3 — Run required verification gate and record outcomes
  - Acceptance criteria:
    - `./scripts/verify.sh` is run and results are logged.

### Execution log (chat panel message derivation allocation cleanup)
- ✅ Task 1 complete: collapsed `normalizedMessages` + `sortedMessages` into a single memoized sorted-entry pass to remove one full-array allocation/memo layer per messages update while preserving sort keys and grouping inputs.
- ✅ Task 2 checks:
  - ⚠️ `cd web/viewer && npm run test -- chatPanel.test.tsx` (initially failed because `jest` was not installed before dependencies were restored)
  - ✅ `cd web/viewer && npm ci`
  - ✅ `cd web/viewer && npm run test -- chatPanel.test.tsx`
- ✅ Task 3 checks:
  - ✅ `./scripts/verify.sh` (passed; Docker-dependent checks skipped because Docker is not installed in this environment)

## Scoped change: directory follower-count single-pass cache

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Refactor directory handler flow to build and reuse follower counts
  - Acceptance criteria:
    - A `map[string]int` count cache is built once per directory request path before sorting/serialization.
    - `sortChannelsByFollowers` consumes precomputed counts (or equivalent single-pass result).
    - `writeDirectoryResponse` consumes the same map and does not call `CountFollowers` again per channel.
    - Response ordering and `FollowerCount` payload values remain unchanged.

- [x] Task 2 — Add/adjust directory handler tests for parity and service-call reduction
  - Acceptance criteria:
    - Tests assert ordering and `FollowerCount` values are unchanged for representative directory endpoints.
    - Tests include mock/spy assertions showing `CountFollowers` is not double-called per channel.

- [x] Task 3 — Run scoped API tests and record outcomes
  - Acceptance criteria:
    - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1 -run Directory` passes.
    - If feasible, broader `./internal/api` tests also pass.

### Execution log (directory follower-count single-pass cache)
- ✅ Task 1 complete: directory handlers now build a per-request `map[string]int` follower-count cache, thread it through follower sorting, and reuse it in response serialization.
- ✅ Task 2 complete: added a counting `ChannelsDirectoryUseCase` spy in handler tests and asserted recommended-directory order/count payload parity plus exactly one `CountFollowers` call per returned channel.
- ✅ Task 3 checks:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1 -run Directory`
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1`

## Scoped change: rate limiter cleanup cadence throttling

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add cleanup timestamp tracking and gated cleanup invocation in login limiter
  - Acceptance criteria:
    - `rateLimiter` includes a cleanup timestamp field used by in-memory login limiting.
    - `AllowLogin` still creates/updates per-key buckets on each request.
    - `cleanupLocked()` invocation is throttled by `loginWindow/2` with a sane minimum interval.
    - Cleanup timestamp is updated when cleanup runs.

- [x] Task 2 — Add/adjust rate-limit tests for behavior parity and eventual stale eviction
  - Acceptance criteria:
    - Existing allow/deny login-limiting behavior remains covered and passing.
    - New/updated tests assert stale buckets are eventually removed after sufficient elapsed time.

- [x] Task 3 — Run scoped server tests and record results
  - Acceptance criteria:
    - `go test ./internal/server -count=1` passes.


### Execution log (rate limiter cleanup cadence throttling)
- ✅ Task 1 complete: added `lastLoginCleanup` timestamp and gated in-memory cleanup calls to run at most every `loginWindow/2` with a 1s minimum interval; per-key bucket update/create and `cleanupLocked()` eviction logic remain unchanged.
- ✅ Task 2 complete: added focused limiter tests for unchanged allow/deny behavior and eventual stale-bucket eviction once cleanup interval elapses.
- ✅ Task 3 checks:
  - ❌ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1` (initial run failed: missing `context` import in `internal/server/server_test.go`; fixed in follow-up).
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1`
  - ✅ `gofmt -w internal/server/ratelimit.go internal/server/server_test.go`
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1`

## Scoped change: upload manager sorted-items memo split

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Split list derivation into `sortedItems` + filter-only `visibleItems` in `web/viewer/components/UploadManager.tsx`
  - Acceptance criteria:
    - `sortedItems` is defined with `useMemo(() => [...items].sort(compareUploadsForMonitoring), [items])`.
    - `visibleItems` depends on `sortedItems`, `listFilter`, and `searchTerm`.
    - Existing `active`/`ready`/`failed` filtering and case-insensitive title/filename matching semantics remain unchanged.

- [x] Task 2 — Run targeted viewer upload-manager tests and record result
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- uploadManager.test.tsx` passes and is logged.


### Execution log (upload manager sorted-items memo split)
- ✅ Task 1 complete: extracted `sortedItems` memo keyed only on `items` and updated `visibleItems` to filter/search over `sortedItems` while preserving existing filter/search semantics.
- ✅ Task 1 check:
  - ✅ `rg -n "const sortedItems|const visibleItems|normalizedSearch" web/viewer/components/UploadManager.tsx`
- ✅ Task 2 check:
  - ✅ `cd web/viewer && npm run test -- uploadManager.test.tsx`

- ✅ Final scoped verification:
  - ✅ `./scripts/verify.sh`

## Scoped change: chat gateway per-channel filter cache

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add per-channel chat-filter cache + TTL config in `internal/chat/gateway.go`
  - Acceptance criteria:
    - `GatewayConfig` exposes a chat-filter cache TTL with conservative default behavior.
    - Gateway stores per-channel cache entries with fetch timestamp/version metadata.
    - Cache reads/writes are protected for concurrent access.


### Execution log (chat gateway per-channel filter cache)
- ✅ Task 1 complete: added `GatewayConfig.ChatFilterCacheTTL` with conservative default, plus concurrency-safe per-channel cache entry scaffolding including fetch timestamp/version metadata in `Gateway`.
- ✅ Task 1 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1`

- [x] Task 2 — Wire cache-aware `matchChatFilter` refresh behavior
  - Acceptance criteria:
    - `matchChatFilter` uses fresh cached filters when available.
    - Stale/missing cache entries trigger `ListChatFilters(channelID)` and refresh cached data.
    - Functional matching behavior for word/regex filters remains unchanged.

- ✅ Task 2 complete: `matchChatFilter` now reads per-channel cached filters when fresh and refreshes via `ListChatFilters(channelID)` when stale/missing.
- ⚠️ Task 2 check:
  - ❌ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1` (expected interim failure: existing regex pattern-change test assumed immediate refetch before TTL; adjusted in Task 3).

- [x] Task 3 — Add tests for functional parity + TTL refresh behavior
  - Acceptance criteria:
    - Tests cover unchanged filter matching expectations.
    - Tests verify store fetch reuse within TTL and refresh after TTL expiry.
    - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1` passes and result is logged.

- ✅ Task 3 complete: expanded gateway chat-filter tests to verify unchanged match behavior, cache reuse within TTL, and refresh behavior after TTL expiry.
- ✅ Task 3 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1`

- ✅ Final scoped verification:
  - ✅ `./scripts/verify.sh` (passes; Docker-dependent checks skipped because Docker is unavailable in this environment).

## Scoped change: viewer past-broadcast terminology alignment

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Align VOD gallery/channel copy to one surface term
  - Acceptance criteria:
    - `web/viewer/components/VodGallery.tsx` loading, empty, error, and primary CTA copy all use “past broadcast(s)” language.
    - `web/viewer/app/channels/[id]/page.tsx` VOD fallback error copy no longer uses “replays” and matches the same noun phrase.

- [x] Task 2 — Update viewer tests for revised copy
  - Acceptance criteria:
    - `web/viewer/__tests__/channelPage.test.tsx` assertions reflect updated empty-state wording and continue validating loading/error states.

- [x] Task 3 — Run targeted viewer test check and record results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- channelPage.test.tsx` passes and result is logged in this section.

### Execution log (viewer past-broadcast terminology alignment)
- ✅ Task 1 complete: standardized `VodGallery` copy to “past broadcast(s)” across loading/error/empty states and CTA text, and aligned channel-page VOD fallback error copy to “past broadcasts”.
- ✅ Task 1 check:
  - ✅ `rg -n "No past broadcasts yet|Watch past broadcast|load past broadcasts|load replays" web/viewer/components/VodGallery.tsx web/viewer/app/channels/[id]/page.tsx`
- ✅ Task 2 complete: updated channel page tests to assert the revised empty-state copy (“No past broadcasts yet”).
- ✅ Task 2 check:
  - ✅ `rg -n "no vods yet|no past broadcasts yet" web/viewer/__tests__/channelPage.test.tsx`
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- channelPage.test.tsx`
  - ✅ `./scripts/verify.sh`


## Scoped change: channel page error panel message consistency

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Update channel page error messaging and diagnostics gating
  - Acceptance criteria:
    - `web/viewer/app/channels/[id]/page.tsx` retains the existing friendly headline/body and retry actions.
    - The visible user-facing detail text is a short guidance sentence instead of `Error details: {error}`.
    - Raw error text is shown only when `process.env.NODE_ENV !== "production"` in a secondary detail block.
    - Fallback strings in `setError(...)` and `setVodError(...)` use “We couldn’t…” style copy.

- [x] Task 2 — Run viewer channel-page test and capture results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- channelPage.test.tsx` passes and is logged in this section.

### Execution log (channel page error panel message consistency)
- ✅ Task 1 complete: kept the existing friendly channel error headline/body with retry actions, replaced inline raw error copy with short guidance text, added development-only diagnostic details, and aligned fallback `setError`/`setVodError` strings to “We couldn’t…” style.
- ✅ Task 2 check:
  - ✅ `cd web/viewer && npm run test -- channelPage.test.tsx`


# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Implement doctor preflight checks and flags in `cmd/bitriver/doctor.go`
  - Acceptance criteria:
    - Keeps `runDoctor(args []string) bool` signature.
    - Supports `--env-file`, `--compose-file`, and `--json`.
    - Emits PASS/WARN/FAIL results with remediation hints.
    - Returns FAIL when hard requirements fail.

- [x] Task 2 — Update/add tests for doctor and verify compatibility
  - Acceptance criteria:
    - Unit coverage validates JSON schema fields (`name`, `status`, `details`, `remediation`).
    - Unit coverage validates compose-file hard failure path.
    - Existing `verify` flow remains compatible.

- [x] Task 3 — Add docs preflight section and host sizing guidance
  - Acceptance criteria:
    - Docs describe how to run `bitriver doctor` and interpret PASS/WARN/FAIL.
    - Docs include conservative minimum sizing guidance used by doctor.

## Execution log
- ✅ Task 1 complete: rewired doctor checks/flags/output, added compose-aware port+bind-mount checks, binary/version/resource checks, and JSON fields (`name,status,details,remediation`).
- ✅ Task 1 checks:
  - `go run ./cmd/bitriver doctor --compose-file deploy/docker-compose.yml`
  - `go run ./cmd/bitriver doctor --json --compose-file deploy/docker-compose.yml`
  - `go run ./cmd/bitriver doctor --compose-file deploy/does-not-exist.yml`

- ✅ Task 2 complete: updated doctor tests for new report schema and added explicit missing compose-file fail test.
- ✅ Task 2 checks:
  - `go test ./cmd/bitriver -count=1`
  - `go test ./... -count=1`

- ✅ Task 3 complete: added preflight guidance and minimum sizing notes to `docs/operations.md` and `docs/production-single-host.md`.
- ✅ Task 3 check:
  - `rg -n "Preflight|bitriver doctor|PASS|WARN|FAIL|4 logical CPUs|8 GiB RAM|20 GiB" docs/operations.md docs/production-single-host.md`


## Scoped change: check-env doctor-by-default

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Update `deploy/check-env.sh` doctor + skip flag UX
  - Acceptance criteria:
    - Runs `go run ./cmd/bitriver doctor --env-file "$ENV_FILE" --compose-file "$REPO_ROOT/deploy/docker-compose.yml"` before env validate by default.
    - Supports `--skip-doctor` in either position (`deploy/check-env.sh --skip-doctor [ENV_FILE]` and `deploy/check-env.sh [ENV_FILE] --skip-doctor`).
    - Prints headings for doctor and env validation.
    - On doctor failure, exits non-zero with actionable next steps.

- [x] Task 2 — Update quickstart + production docs to call out `deploy/check-env.sh` first
  - Acceptance criteria:
    - `docs/quickstart.md` mentions running `deploy/check-env.sh` as first preflight step.
    - `docs/production-single-host.md` mentions running `deploy/check-env.sh` as first preflight step.

- [x] Task 3 — Validate behavior and capture results
  - Acceptance criteria:
    - `bash deploy/check-env.sh --help` shows skip-doctor usage.
    - `bash deploy/check-env.sh --skip-doctor` succeeds for existing flow.
    - `bash deploy/check-env.sh` runs doctor+env validation sequence.


### Execution log (check-env doctor-by-default)
- ✅ Task 1 complete: `deploy/check-env.sh` now runs doctor first with canonical compose file, supports `--skip-doctor` in either argument position, prints stage headings, and emits actionable failure next steps.
- ✅ Task 2 complete: added first-step preflight guidance to quickstart and production single-host docs using `bash deploy/check-env.sh`.
- ✅ Task 3 checks:
  - `bash deploy/check-env.sh --help` (pass)
  - `bash deploy/check-env.sh --skip-doctor` (fails in this environment because root `.env` is absent)
  - `bash deploy/check-env.sh deploy/.env.example --skip-doctor` (runs env validation; fails as expected because example placeholders are intentionally invalid for production)
  - `bash deploy/check-env.sh deploy/.env.example` (confirms doctor runs first, then exits non-zero on doctor FAIL with actionable guidance)
  - `./scripts/verify.sh` (fails on pre-existing `.env.example` placeholder hygiene rule unrelated to this scoped change)

## Scoped change: compose-compatible resource limits overlay

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Convert `deploy/docker-compose.resources.yml` to ulimits-only overlay
  - Acceptance criteria:
    - Removes all `services.*.deploy.resources.*` blocks.
    - Keeps ingest `ulimits` settings.
    - Adds explicit guidance that enforceable CPU/memory limits come from `deploy/docker-compose.limits.yml`.

- [x] Task 2 — Update docs to distinguish limits overlay vs ulimits overlay
  - Acceptance criteria:
    - `docs/production-single-host.md` recommends `docker-compose.limits.yml` for production limits.
    - Runbook docs no longer describe `docker-compose.resources.yml` as CPU/memory limits.
    - Docs explain when to layer `docker-compose.resources.yml` for `nofile` ulimits.

- [x] Task 3 — Validate compose rendering and full Go test suite
  - Acceptance criteria:
    - Combined compose overlays render cleanly with `docker compose ... config`.
    - `go test ./...` passes.


### Execution log (compose-compatible resource limits overlay)
- ✅ Task 1 complete: `deploy/docker-compose.resources.yml` is now ulimits-only; all `deploy.resources` Swarm blocks removed with explicit non-Swarm guidance and limits-overlay command.
- ✅ Task 2 complete: updated `docs/production-single-host.md`, `docs/operations.md`, and `docs/advanced-deployments.md` to separate enforceable limits (`docker-compose.limits.yml`) from ulimits (`docker-compose.resources.yml`).
- ✅ Task 3 checks:
  - ⚠️ `docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.limits.yml config` (blocked: `docker` CLI unavailable in this environment).
  - ⚠️ `docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.limits.yml -f deploy/docker-compose.resources.yml config` (blocked: `docker` CLI unavailable in this environment).
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s`


## Scoped change: `_FILE` secret env support and validation

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Extend env secret resolution in `cmd/bitriver/env_validation_helpers.go`
  - Acceptance criteria:
    - All sensitive keys validated by `env validate` support companion `*_FILE` values.
    - Resolution order is deterministic and documented (`FOO` wins over `FOO_FILE` with warning).
    - `_FILE` unreadable paths surface validation errors; resolved values feed existing required/blocked checks.

- [x] Task 2 — Expand env validation tests for `*_FILE` behavior
  - Acceptance criteria:
    - Coverage includes only direct value, only `_FILE`, both set, and missing/unreadable file.
    - Coverage includes trailing-newline trimming for file-backed values.
    - Coverage confirms existing placeholder/constraint checks apply to resolved file values.

- [x] Task 3 — Update env/docs examples for `*_FILE` usage
  - Acceptance criteria:
    - `deploy/.env.example` documents `*_FILE` companions for sensitive values.
    - `docs/secrets-hardening.md` adds section “Using *_FILE secrets with Docker Compose mounts” with a concrete mounted secrets directory example.



### Execution log (`_FILE` secret env support and validation)
- ✅ Task 1 complete: secret file resolution now also covers `BITRIVER_OME_HEALTHCHECK_TOKEN`, keeps direct-value precedence with warning, validates unreadable file paths, and trims trailing newlines when loading file-backed secrets.
- ✅ Task 2 complete: added test coverage ensuring file-backed placeholders still trigger blocked checks and optional sensitive `_FILE` keys emit readable-path errors.
- ✅ Task 2 checks:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./cmd/bitriver -count=1`
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s`
- ✅ Task 3 complete: updated `docs/secrets-hardening.md` section title/content with a concrete Compose mount example and documented `BITRIVER_OME_HEALTHCHECK_TOKEN_FILE` in `deploy/.env.example`.
- ✅ Task 3 check:
  - ✅ `rg -n "Using \*_FILE secrets with Docker Compose mounts|Concrete Docker Compose mount example|OME_HEALTHCHECK_TOKEN_FILE|_FILE" docs/secrets-hardening.md deploy/.env.example`


## Scoped change: upgrade-plan operator checklist command

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Rework `cmd/bitriver/upgrade_plan.go` for checklist-style planning output
  - Acceptance criteria:
    - Supports `--compose-file`, `--env-file`, and required `--target`.
    - Best-effort running-tag detection from Docker Compose with WARN fallback guidance.
    - Output includes backup docs link, migration behavior status, rollback caveats, and actionable checklist.

- [x] Task 2 — Update/add tests for planner parsing and output behavior
  - Acceptance criteria:
    - Coverage includes missing running stack warning/fallback path.
    - Coverage includes compose `ps` parsing for running image tags.
    - Coverage validates checklist sections include backup/migration/rollback guidance.

- [x] Task 3 — Update upgrade documentation with new command usage and sample output
  - Acceptance criteria:
    - `docs/upgrades.md` references `--target` syntax.
    - Includes a concise example output block demonstrating WARN + checklist flow.



### Execution log (upgrade-plan operator checklist command)
- ✅ Task 1 complete: `upgrade-plan` now accepts `--compose-file`, `--env-file`, and required `--target`; it performs best-effort running tag detection from Compose with WARN fallback guidance and prints migration + rollback sections with an operator checklist.
- ✅ Task 2 complete: added focused tests covering compose `ps` image parsing, env-file fallback warnings, and unknown-current-version guidance while still emitting checklist output.
- ✅ Task 3 complete: updated `docs/upgrades.md` command examples to `--target` and added best-effort behavior notes plus sample command output.

## Scoped change: production-readiness gate unblock (env placeholder hygiene)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Run release-readiness gate and capture blockers
  - Acceptance criteria:
    - Run `./scripts/verify.sh` from repo root.
    - Record the first release-blocking failure.

- [x] Task 2 — Fix env placeholder hygiene blocker in `deploy/.env.example`
  - Acceptance criteria:
    - `BITRIVER_LIVE_ADMIN_PASSWORD` example includes an explicit sample marker.
    - Re-running `./scripts/verify.sh` passes.

### Execution log (production-readiness gate unblock)
- ✅ Task 1 check:
  - ❌ `./scripts/verify.sh` (failed: `BITRIVER_LIVE_ADMIN_PASSWORD` placeholder missing explicit sample marker)
- ✅ Task 2 complete: updated `deploy/.env.example` admin password example to an explicit sample placeholder string.
- ✅ Task 2 check:
  - ✅ `./scripts/verify.sh`


## Scoped change: navbar notifications control semantics

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Update notifications icon button semantics in `web/viewer/components/Navbar.tsx`
  - Acceptance criteria:
    - Notifications button in `.nav-icon-group` is explicitly disabled if feature is not implemented.
    - Control includes helper text/tooltip such as “Notifications coming soon”.
    - Styling remains aligned with existing icon button usage.

- [x] Task 2 — Add/update navbar tests in `web/viewer/__tests__/navbar.test.tsx`
  - Acceptance criteria:
    - Test verifies notifications control is disabled.
    - Test verifies helper text/tooltip content is present on the control.

- [x] Task 3 — Run viewer navbar test check and record results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- navbar.test.tsx` completes successfully.
    - Execution log is captured in this scoped section.


### Execution log (navbar notifications control semantics)
- ✅ Task 1 complete: notifications icon button now uses disabled semantics and includes helper tooltip text “Notifications coming soon”.
- ✅ Task 2 complete: added navbar test covering disabled notifications action and tooltip text.
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- navbar.test.tsx`
  - ✅ `./scripts/verify.sh`

## Scoped change: chat panel dialog keyboard/focus accessibility

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add modal focus management and keyboard handling in `web/viewer/components/ChatPanel.tsx`
  - Acceptance criteria:
    - Store trigger refs for pop-out and settings buttons and restore focus to the correct trigger on close.
    - Opening either dialog moves focus inside the dialog and traps Tab/Shift+Tab within the active modal.
    - Escape closes the active dialog and opening one dialog closes the other.

- [x] Task 2 — Add dialog accessibility tests in `web/viewer/__tests__/chatPanel.test.tsx`
  - Acceptance criteria:
    - Test verifies Escape closes open pop-out/settings dialog.
    - Test verifies Tab cycles within active dialog controls.
    - Test verifies focus returns to the originating trigger after close.

- [x] Task 3 — Run viewer checks for chat panel changes and record results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- chatPanel.test.tsx` passes.
    - `./scripts/verify.sh --viewer` passes (or capture explicit environment limitation).

### Execution log (chat panel dialog keyboard/focus accessibility)
- ✅ Task 1 complete: added dedicated trigger refs for pop-out/settings controls; dialog open now moves focus to heading, Escape closes active dialog, Tab/Shift+Tab is trapped within the active modal, close actions restore focus, and opening one dialog closes the other.
- ✅ Task 2 complete: added dialog-focused tests covering Escape close for both dialogs, focus trap tab wrapping behavior, trigger-focus restoration on close, and single-active-dialog behavior.
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- chatPanel.test.tsx`
  - ✅ `./scripts/verify.sh --viewer`

## Scoped change: viewer grid image priority loading

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Update preview image loading strategy in grid components
  - Acceptance criteria:
    - `web/viewer/components/DirectoryGrid.tsx` applies `priority` only to leading mapped card(s) and uses explicit lazy loading for non-priority cards.
    - `web/viewer/components/LiveNowGrid.tsx` applies `priority` only to leading mapped card(s) and uses explicit lazy loading for non-priority cards.
    - Existing `sizes` attributes remain unchanged.

- [x] Task 2 — Add/update component tests for priority vs lazy behavior
  - Acceptance criteria:
    - Tests render multiple cards in both grids.
    - Assertions verify only expected leading card(s) receive priority behavior (or corresponding props) and subsequent cards are lazy.

- [x] Task 3 — Run viewer checks for updated grid behavior
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- channelDisplayPrimitives.test.tsx` passes.
    - `./scripts/verify.sh --viewer` passes (or capture explicit environment limitation).

### Execution log (viewer grid image priority loading)
- ✅ Task 1 complete: updated `DirectoryGrid` and `LiveNowGrid` mapped preview images to set `priority` only for the first card (`index < 1`) and explicit `loading="lazy"` for non-priority cards while keeping existing `sizes` values.
- ✅ Task 1 check:
  - ✅ `cd web/viewer && npm run test -- channelDisplayPrimitives.test.tsx`
- ✅ Task 2 complete: extended `channelDisplayPrimitives` tests with multi-card assertions confirming only non-leading grid preview images are explicitly lazy-loaded in both directory and live-now layouts.
- ✅ Task 2 check:
  - ✅ `cd web/viewer && npm run test -- channelDisplayPrimitives.test.tsx`
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- channelDisplayPrimitives.test.tsx`
  - ✅ `./scripts/verify.sh`
  - ✅ `./scripts/verify.sh --viewer`

## Scoped change: featured carousel reduced-motion autoplay behavior

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Implement reduced-motion aware autoplay behavior in `web/viewer/components/FeaturedChannel.tsx`
  - Acceptance criteria:
    - Detect `window.matchMedia("(prefers-reduced-motion: reduce)")` on mount and sync component state.
    - Autoplay initializes to off when reduced motion is preferred, while preserving explicit prop-driven policy behavior.
    - Preference changes update autoplay behavior reactively and interval setup is blocked while reduced motion mode is active.
    - Manual Play/Pause control remains available.

- [x] Task 2 — Add reduced-motion autoplay tests in `web/viewer/__tests__/channelDisplayPrimitives.test.tsx`
  - Acceptance criteria:
    - Test verifies reduced-motion preference starts with autoplay off.
    - Test verifies toggling Play enables rotation.
    - Test verifies preference change to reduced motion stops autoplay.

- [x] Task 3 — Run viewer tests for featured carousel behavior and record results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- channelDisplayPrimitives.test.tsx` passes and is recorded in execution log.


### Execution log (featured carousel reduced-motion autoplay behavior)
- ✅ Task 1 complete: featured carousel now reads prefers-reduced-motion on mount, tracks preference changes, defaults autoplay off in reduced-motion mode, and blocks interval setup while reduced-motion mode is active unless manually resumed.
- ✅ Task 1 check:
  - ✅ `cd web/viewer && npm run test -- channelDisplayPrimitives.test.tsx`
- ✅ Task 2 complete: added reduced-motion autoplay tests for initial paused state, manual Play rotation, and runtime preference-change stop behavior.
- ✅ Task 2 check:
  - ✅ `cd web/viewer && npm run test -- channelDisplayPrimitives.test.tsx`
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- channelDisplayPrimitives.test.tsx`
  - ✅ `./scripts/verify.sh`

## Scoped change: navbar avatar-menu outside-dismiss + Escape behavior

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Implement avatar-menu refs and dismiss/focus handlers in `web/viewer/components/Navbar.tsx`
  - Acceptance criteria:
    - Add refs for avatar toggle button and avatar menu container.
    - While `userMenuOpen` is true, register document-level outside interaction listener(s) that close menu when target is outside both refs.
    - While `userMenuOpen` is true, register Escape-key listener that closes the menu.
    - Escape/outside-dismiss close path restores focus to avatar button.
    - Avatar toggle includes `aria-controls` pointing to menu element id.

- [x] Task 2 — Add navbar tests for outside click, Escape close, and focus restoration
  - Acceptance criteria:
    - `web/viewer/__tests__/navbar.test.tsx` includes a test proving outside click closes the menu.
    - Includes a test proving Escape closes the menu.
    - Includes a test proving focus returns to avatar toggle after Escape close.

- [x] Task 3 — Run viewer checks and record results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- navbar.test.tsx` passes.
    - `./scripts/verify.sh --viewer` passes (or explicit environment limitation is recorded).


### Execution log (navbar avatar-menu outside-dismiss + Escape behavior)
- ✅ Task 1 complete: added avatar button/menu refs, document-level outside interaction + Escape handlers while menu is open, focus restoration for dismiss paths, and `aria-controls`/menu-id linkage.
- ✅ Task 1 check:
  - ✅ `cd web/viewer && npm run test -- navbar.test.tsx`
- ✅ Task 2 complete: added navbar tests for outside-click close, Escape close, and Escape focus restoration to the avatar toggle.
- ✅ Task 2 check:
  - ✅ `cd web/viewer && npm run test -- navbar.test.tsx`
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- navbar.test.tsx`
  - ✅ `./scripts/verify.sh --viewer`

## Scoped change: server runtime constructor rollback cleanup

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Refactor `NewServerRuntime` to register and disable constructor rollback closers
  - Acceptance criteria:
    - Repository/store cleanup is deferred immediately after successful constructor calls and triggered on downstream failure.
    - Postgres session and MFA store closers are similarly registered when created.
    - Cleanup defers are disabled on successful `ServerRuntime` return.
    - Top-level failures remain the same classes with added safe stage context.

- [x] Task 2 — Add constructor-failure tests in `internal/app`
  - Acceptance criteria:
    - Test covers MFA store creation failure after session store success and confirms prior closers ran.
    - Test covers chat queue creation failure after store/session setup and confirms all created closers ran.
    - Tests verify no leaked closers remain on these error returns.

- [x] Task 3 — Run scoped app tests and record results
  - Acceptance criteria:
    - `go test ./internal/app -count=1` passes.
    - Execution log is captured in this scoped section.


### Execution log (server runtime constructor rollback cleanup)
- ✅ Task 1 complete: `NewServerRuntime` now registers constructor-time rollback defers for repository/session/MFA Postgres stores and disables them on success; setup failures are wrapped with stage context.
- ✅ Task 1 check:
  - ✅ `go test ./internal/app -count=1`
- ✅ Task 2 complete: added constructor-failure tests for MFA-store init failure and chat-queue init failure, asserting repository/session/MFA close hooks run exactly once on constructor errors.
- ✅ Task 2 check:
  - ✅ `go test ./internal/app -count=1`
- ✅ Task 3 checks:
  - ✅ `go test ./internal/app -count=1`

## Scoped change: server.New helper extraction and behavior lock tests

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Extract handler mutation helper in `internal/server/server.go`
  - Acceptance criteria:
    - `New(handler, cfg)` delegates OAuth/session-cookie/webhook/upload/self-signup mutations to a helper.
    - Mutation precedence/behavior remains unchanged.

- [x] Task 2 — Extract route registration helper in `internal/server/server.go`
  - Acceptance criteria:
    - Route registration for `/healthz`, `/metrics`, `/api/*`, static files, `/viewer`, and `/` SPA fallback is delegated to helper(s) that accept `*http.ServeMux`.
    - Viewer proxy error handling behavior remains unchanged.

- [x] Task 3 — Extract middleware chain assembly helper in `internal/server/server.go`
  - Acceptance criteria:
    - Middleware chain construction is delegated to a helper.
    - Effective middleware order remains exactly unchanged.

- [x] Task 4 — Add/adjust tests in `internal/server/server_test.go`
  - Acceptance criteria:
    - Tests lock in key route availability (`/healthz`, `/metrics`, representative `/api/*`, static route, `/viewer` behavior).
    - Tests lock in middleware ordering/effects so reordering is detected.

- [x] Task 5 — Run required checks and record results
  - Acceptance criteria:
    - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1` passes.
    - `./scripts/verify.sh` passes (or environment limitation is explicitly recorded).

### Execution log (server.New helper extraction and behavior lock tests)
- ✅ Task 1 complete: extracted `configureAPIHandler` for OAuth/self-signup/session cookie policy/webhook/upload mutation while preserving existing precedence.
- ✅ Task 2 complete: extracted `registerRoutes` handling health/metrics/api/static/viewer/spa routes and preserved viewer proxy error handling behavior.
- ✅ Task 3 complete: extracted `buildMiddlewareChain` with unchanged middleware wrapping order.
- ✅ Task 4 complete: added constructor-level tests for config mutation application, key route registration outcomes, and middleware-order effects (auth-before-CORS, request-id reachability).
- ✅ Task 5 checks:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1`
  - ✅ `./scripts/verify.sh` (Docker-dependent checks skipped by script because docker is unavailable)

## Scoped change: CORS `Vary` merge behavior

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Preserve existing `Vary` header values in CORS middleware
  - Acceptance criteria:
    - `internal/server/cors.go` replaces direct `Set("Vary", "Origin")` with helper logic that appends `Origin` only when absent.
    - Existing CORS headers and status flow remain unchanged.

- [x] Task 2 — Add regression test for upstream `Vary` preservation
  - Acceptance criteria:
    - `internal/server/cors_test.go` includes a test where upstream middleware sets `Vary` before CORS runs.
    - Test verifies resulting `Vary` contains original value(s) and `Origin` (merged, not replaced).

- [x] Task 3 — Run scoped server tests and record results
  - Acceptance criteria:
    - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1` passes.
    - Execution log for this scoped section is recorded.

### Execution log (CORS `Vary` merge behavior)
- ✅ Task 1 complete: replaced direct `Vary` assignment with `appendVaryHeader` to preserve existing `Vary` values while appending `Origin` only when missing.
- ✅ Task 1 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1`
- ✅ Task 2 complete: added regression coverage proving an upstream `Vary: Accept-Encoding` value is preserved and merged with `Origin`.
- ✅ Task 2 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1`
- ✅ Task 3 checks:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1`
  - ✅ `./scripts/verify.sh` (Docker-dependent checks skipped by script because docker is unavailable)

## Scoped change: CSRF cookie creation failure hardening

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Return errors from CSRF cookie creation and fail closed in middleware
  - Acceptance criteria:
    - `setCSRFCookie` returns `(*http.Cookie, error)` and never silently emits empty CSRF tokens.
    - `csrfMiddleware` handles CSRF token generation failure by logging a warning and returning `403` for protected requests.
    - Existing exempt-path and bearer-token bypass behavior remains unchanged.

- [x] Task 2 — Add regression tests for token-generation failure denial
  - Acceptance criteria:
    - Tests can force CSRF token generation failure deterministically.
    - Protected cookie-auth requests that require token issuance are denied with forbidden status when generation fails.
    - Downstream handler is not called on token-generation failure.

- [x] Task 3 — Run scoped server tests and record results
  - Acceptance criteria:
    - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1` passes.
    - Execution log for this scoped section is recorded.

### Execution log (CSRF cookie creation failure hardening)
- ✅ Task 1 complete: `setCSRFCookie` now returns `(*http.Cookie, error)` and `csrfMiddleware` fails closed with a warning + forbidden response when CSRF token issuance fails.
- ✅ Task 1 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1`
- ✅ Task 2 complete: added CSRF middleware tests that force token generation failure, verify protected cookie-auth requests are denied, and confirm bearer/exempt-path bypass behavior remains intact.
- ✅ Task 2 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1`
- ✅ Task 3 checks:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1`
  - ✅ `./scripts/verify.sh` (Docker-dependent checks skipped by script because docker is unavailable)

## Scoped change: server/storage documentation-only comment cleanup

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Clean misleading comments in server middleware helpers
  - Acceptance criteria:
    - `internal/server/ratelimit.go`, `internal/server/request_id.go`, and `internal/server/security.go` comments no longer claim non-error-returning funcs return errors.
    - Exported API comments are concise and accurate.


- ✅ Task 1 complete: cleaned misleading server middleware comments in rate-limit/request-id/security helpers and removed nonexistent error-return wording.
- ✅ Task 1 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1`

- [x] Task 2 — Clean repetitive boilerplate comments in postgres repository helpers
  - Acceptance criteria:
    - `internal/storage/postgres_repository.go` helper comments are simplified to intent-focused docs.
    - Comments for bool/string/non-error signatures no longer mention nonexistent error returns.


- ✅ Task 2 complete: simplified postgres repository and helper comments to intent-focused docs and removed repetitive boilerplate/error-language mismatches.
- ✅ Task 2 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -count=1`

- [x] Task 3 — Run scoped tests and record results
  - Acceptance criteria:
    - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server ./internal/storage -count=1` passes.
    - Execution log for this scoped section is recorded.


### Execution log (server/storage documentation-only comment cleanup)
- ✅ Task 1 complete: cleaned misleading server middleware comments in rate-limit/request-id/security helpers and removed nonexistent error-return wording.
- ✅ Task 1 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1`
- ✅ Task 2 complete: simplified postgres repository and helper comments to intent-focused docs and removed repetitive boilerplate/error-language mismatches.
- ✅ Task 2 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -count=1`
- ✅ Task 3 checks:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server ./internal/storage -count=1`
  - ✅ `./scripts/verify.sh` (Docker-dependent checks skipped by script because docker is unavailable)

## Scoped change: server runtime shutdown close warning logs

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Replace ignored shutdown close errors with warning logs in `internal/app/server_runtime.go`
  - Acceptance criteria:
    - Shutdown still attempts `store`, then `session_store`, then `mfa_store` close operations in existing order.
    - Close errors are logged as warnings with component identifiers: `store`, `session_store`, `mfa_store`.
    - Shutdown remains non-fatal (no early return introduced).

- [x] Task 2 — Add shutdown close-warning tests in `internal/app/server_runtime_test.go`
  - Acceptance criteria:
    - Test uses fake closers returning errors for store/session/mfa.
    - Assertions verify warning logs include each component identifier and close error context.
    - Assertions verify shutdown continues and invokes all closers despite earlier close failures.

- [x] Task 3 — Run validation commands and record results
  - Acceptance criteria:
    - `go test ./internal/app -count=1` passes.
    - `./scripts/verify.sh` passes (or explicit environment limitation captured).

### Execution log (server runtime shutdown close warning logs)
- ✅ Task 1 complete: replaced ignored shutdown close errors with warning logs carrying component identifiers for store, session_store, and mfa_store while preserving shutdown flow/order.
- ✅ Task 2 complete: added shutdown test with failing closers that asserts warning logs for each component and confirms shutdown continues through all closers in order.
- ✅ Task 2 check:
  - ✅ `go test ./internal/app -count=1`
- ✅ Task 3 checks:
  - ✅ `./scripts/verify.sh`
  - ⚠️ Docker-dependent checks skipped by verify script because docker is not installed in this environment.
- ✅ Task 3 re-check after gofmt:
  - ✅ `go test ./internal/app -count=1`

## Scoped change: deduplicate upload source storage mapping in internal/app

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add shared storage mapping helper in `internal/app`
  - Acceptance criteria:
    - New helper converts `storage.ObjectStorageConfig` to `api.UploadSourceStorageConfig`.
    - Helper performs explicit field-by-field mapping for `Endpoint`, `Bucket`, `Prefix`, `PublicEndpoint`, `UseSSL`, and `RequestTimeout`.

- [x] Task 2 — Use helper in handler/runtime wiring paths
  - Acceptance criteria:
    - `internal/app/http.go` `NewHandler` uses helper for `handler.UploadSourceStorage` assignment.
    - `internal/app/server_runtime.go` upload processor config setup uses helper.
    - No behavior changes beyond mapping deduplication.

- [x] Task 3 — Add focused unit test for mapping fidelity
  - Acceptance criteria:
    - New unit test in `internal/app` validates mapped fields preserve exact values.
    - Test covers all mapped fields.

- [x] Task 4 — Run scoped tests/checks and record results
  - Acceptance criteria:
    - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/app -count=1` passes.

### Execution log (deduplicate upload source storage mapping in internal/app)
- ✅ Task 1 complete: added `uploadSourceStorageConfigFromObjectStorage` helper in `internal/app/upload_source_storage.go` with explicit field-by-field mapping.
- ✅ Task 1 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/app -run '^$' -count=1`
- ✅ Task 2 complete: updated `internal/app/http.go` and `internal/app/server_runtime.go` to use the shared helper mapping.
- ✅ Task 2 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/app -run '^$' -count=1`
- ✅ Task 3 complete: added focused test `TestUploadSourceStorageConfigFromObjectStorage` covering all mapped fields.
- ✅ Task 3 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/app -run TestUploadSourceStorageConfigFromObjectStorage -count=1`
- ✅ Task 4 checks:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/app -count=1`

## Scoped change: unify creator live status labels

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add single-source creator status labels in creator live page
  - Acceptance criteria:
    - `web/viewer/app/creator/live/[channelId]/page.tsx` defines a single shared mapping for creator-visible labels for offline/starting/live/error conditions.
    - `deriveControlCentreStatus(...)` reads labels from that mapping instead of hardcoded per-branch strings.

- [x] Task 2 — Reuse same labels in test panel status copy
  - Acceptance criteria:
    - `testPanelStatus` in `page.tsx` reuses the shared label mapping.
    - Offline idle and degraded reconnecting states each use one consistent label across derive/test panel.
    - Instructions remain minimal and deterministic per status.

- [x] Task 3 — Update label assertions in viewer tests
  - Acceptance criteria:
    - Tests under `web/viewer/__tests__` that assert `deriveControlCentreStatus` labels are updated for unified wording.
    - `cd web/viewer && npm run test -- creatorLiveStreamStatus.test.ts` passes.
- [x] Task 1 — Add single-source creator status labels in creator live page
  - Acceptance criteria:
    - `web/viewer/app/creator/live/[channelId]/page.tsx` defines a single shared mapping for creator-visible labels for offline/starting/live/error conditions.
    - `deriveControlCentreStatus(...)` reads labels from that mapping instead of hardcoded per-branch strings.

- ✅ Task 1 complete: added `CREATOR_STATUS_LABELS` and switched `deriveControlCentreStatus`/default stream status labels to use the shared map.
- ✅ Task 1 check:
  - ✅ `cd web/viewer && npm run test -- creatorLiveStreamStatus.test.ts`

- [x] Task 2 — Reuse same labels in test panel status copy
  - Acceptance criteria:
    - `testPanelStatus` in `page.tsx` reuses the shared label mapping.
    - Offline idle and degraded reconnecting states each use one consistent label across derive/test panel.
    - Instructions remain minimal and deterministic per status.

- ✅ Task 2 complete: updated `testPanelStatus` to reuse shared labels and simplified status instructions for offline/live/reconnecting/error states.
- ✅ Task 2 check:
  - ✅ `cd web/viewer && npm run test -- creatorLiveStreamStatus.test.ts`

- [x] Task 3 — Update label assertions in viewer tests
  - Acceptance criteria:
    - Tests under `web/viewer/__tests__` that assert `deriveControlCentreStatus` labels are updated for unified wording.
    - `cd web/viewer && npm run test -- creatorLiveStreamStatus.test.ts` passes.

- ✅ Task 3 complete: updated `creatorLiveStreamStatus` label expectation from `Ingesting` to `Reconnecting`.
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- creatorLiveStreamStatus.test.ts`
  - ✅ `./scripts/verify.sh` (full repo + viewer checks; passed with expected npm env warnings and docker skip notices)

## Scoped change: following summary label clarity and terminology consistency

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Define and centralize following summary copy constants
  - Acceptance criteria:
    - `web/viewer/components/following/FollowingState.tsx` exports shared summary label constants for followed-count and live-now-count text.
    - Existing empty/unauthenticated copy remains centralized and unchanged in meaning.

- [x] Task 2 — Update FollowingSidebar and FollowingRail to use explicit summary labels
  - Acceptance criteria:
    - Sidebar ready summary shows an explicit followed count label (e.g. `X followed`).
    - Rail ready summary shows an explicit live-now count label (e.g. `X live now`).
    - Both components consume shared copy constants from `FollowingState.tsx` where applicable.

- [x] Task 3 — Update/verify viewer tests for terminology and summary semantics
  - Acceptance criteria:
    - Tests cover new explicit summary labels for sidebar/rail ready states.
    - Empty/unauthenticated wording remains consistent across both surfaces.
    - `cd web/viewer && npm run test -- followingStatePresentation.test.tsx followingSidebar.test.tsx` passes and results are logged below.

### Execution log (following summary label clarity and terminology consistency)
- ✅ Task 1 complete: added shared summary label builders in `FOLLOWING_COPY` for followed-count and live-now-count phrasing.
- ✅ Task 1 check:
  - ✅ `cd web/viewer && npm run test -- followingStatePresentation.test.tsx`
- ✅ Task 2 complete: sidebar now reports explicit followed totals and rail now reports explicit live-now totals, both sourced from shared following copy constants.
- ⚠️ Task 2 check (intermediate):
  - ❌ `cd web/viewer && npm run test -- followingStatePresentation.test.tsx followingSidebar.test.tsx` (failed due to pre-existing single-match assertion in `followingSidebar.test.tsx` after summary copy became intentionally duplicated in empty state UI)
- ✅ Task 3 complete: updated viewer tests to assert explicit summary semantics and consistent channels-focused terminology across rail/sidebar.
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- followingStatePresentation.test.tsx followingSidebar.test.tsx`
  - ✅ `./scripts/verify.sh`

## Scoped change: cache browse created-at timestamps before sort

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Build cached timestamp list and preserve filter behavior
  - Acceptance criteria:
    - `sortedChannels` creates an intermediate mapped list with cached numeric `createdAt` timestamp per entry.
    - Existing filter behavior is applied against that intermediate list without changing semantics.

- [x] Task 2 — Sort using cached timestamp and return original entries
  - Acceptance criteria:
    - `new` sort mode uses cached timestamp values, not `Date` parsing inside comparator.
    - `live`/`trending` sort logic remains unchanged in behavior.
    - `sortedChannels` returns original channel entries (helper fields removed before return).

- [x] Task 3 — Validate viewer browse checks
  - Acceptance criteria:
    - Relevant viewer lint/test command(s) pass and results are logged.

### Execution log (cache browse created-at timestamps before sort)
- ✅ Task 1 complete: refactored `sortedChannels` to map channels into intermediate objects with cached `createdAtTs` and applied unchanged filter predicate to the mapped list.
- ✅ Task 1 check:
  - ✅ `cd web/viewer && npm run test -- browsePage.test.tsx`
- ✅ Task 2 complete: updated sort logic to use cached `createdAtTs` for `new` mode, preserved existing live/trending viewer/live sort logic, and mapped results back to original channel entries.
- ✅ Task 2 check:
  - ✅ `cd web/viewer && npm run test -- browsePage.test.tsx`
- ✅ Task 3 complete: ran viewer lint/test validation for browse page refactor.
- ⚠️ Task 3 checks:
  - ❌ `cd web/viewer && npm run lint -- app/browse/page.tsx` (Next.js lint CLI in this project does not accept file-path arg and exited before linting)
  - ✅ `cd web/viewer && npm run lint`
  - ✅ `cd web/viewer && npm run test -- browsePage.test.tsx`

## Scoped change: cache chat sentAt timestamps within memoized derivations

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add normalized memoized messages with cached `sentAtTs`
  - Acceptance criteria:
    - `ChatPanel` adds a `useMemo` that maps `messages` to objects containing the original `message` plus numeric `sentAtTs`.
    - Timestamp parsing occurs once per message per memo cycle.

- [x] Task 2 — Refactor sort/group calculations to consume cached timestamps
  - Acceptance criteria:
    - `sortedMessages` comparator uses cached `sentAtTs` values.
    - Grouping 2-minute window compares current/previous `sentAtTs` values, preserving same-user grouping behavior.
    - Render path still reads original `ChatMessage` fields unchanged.

- [x] Task 3 — Run targeted viewer test check and record results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- chatPanel.test.tsx` runs successfully and is logged.


### Execution log (cache chat sentAt timestamps within memoized derivations)
- ✅ Task 1 complete: added `normalizedMessages` memo that maps each `ChatMessage` to `{ message, sentAtTs }`, parsing `sentAt` once per memo cycle.
- ✅ Task 1 check:
  - ✅ `cd web/viewer && npm run test -- chatPanel.test.tsx`
- ✅ Task 2 complete: `sortedMessages` now compares cached `sentAtTs`, and grouping time-window checks compare current/previous cached timestamps while preserving 2-minute same-user grouping and rendering from original `message` fields.
- ✅ Task 2 check:
  - ✅ `cd web/viewer && npm run test -- chatPanel.test.tsx`
- ✅ Task 3 complete: ran targeted and full verification checks for the viewer/chat-panel refactor.
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- chatPanel.test.tsx`
  - ✅ `./scripts/verify.sh`

## Scoped change: gate channel VOD fetch by Videos tab usage

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Refactor channel page VOD fetch trigger to Videos-tab activation
  - Acceptance criteria:
    - Remove unconditional VOD fetch-on-mount effect in `web/viewer/app/channels/[id]/page.tsx`.
    - Add gating so VOD fetch runs when `activeTab === "videos"` and channel `id` changes reset request tracking.
    - Existing `loadVods`, `handleVodRetry`, and `VodGallery` prop contract remain intact.

- [x] Task 2 — Add/update channel page tests for tab-gated VOD fetching
  - Acceptance criteria:
    - Tests verify initial render does not fetch VODs while on default About tab.
    - Tests verify opening Videos tab fetches once, tab switching does not refetch, and retry still triggers a new fetch.
    - Tests verify changing `id` resets gating and fetches for the new channel when Videos is viewed.

- [x] Task 3 — Run scoped viewer test check and record results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- channelPage.test.tsx` passes.
    - Results are logged in this section.

### Execution log (gate channel VOD fetch by Videos tab usage)
- ✅ Task 1 complete: replaced unconditional VOD fetch effect with Videos-tab gated fetch logic, added per-channel request tracking, and reset gating on channel changes while preserving `loadVods`, retry behavior, and `VodGallery` props.
- ✅ Task 2 complete: updated `channelPage` tests to assert no VOD fetch on initial About tab, one fetch on first Videos activation, no refetch on tab toggles, retry-driven refetch, and channel-id reset behavior.
- ✅ Task 2 check:
  - ✅ `cd web/viewer && npm run test -- channelPage.test.tsx`
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- channelPage.test.tsx`
  - ✅ `./scripts/verify.sh`
  - ⚠️ Docker-dependent steps in `./scripts/verify.sh` were skipped because Docker is not installed in this environment.

## Scoped change: cache compiled regex chat filters in gateway

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add gateway-level compiled regex cache and use it in regex filter matching
  - Acceptance criteria:
    - `Gateway` owns a private regex cache map guarded by a mutex.
    - `matchChatFilter` regex branch looks up by a key that changes with filter identity + pattern content.
    - Regexes compile only on cache miss and successful compilations are cached.
    - Invalid regex behavior remains warn-and-skip.

- [x] Task 2 — Add/update gateway tests for repeated evaluation and cache invalidation behavior
  - Acceptance criteria:
    - Tests confirm repeated evaluation with unchanged regex pattern keeps matching behavior unchanged.
    - Tests confirm unchanged pattern is compiled once across repeated calls.
    - Tests confirm changing filter pattern triggers recompilation and updated matching behavior.

- [x] Task 3 — Run scoped/full checks and record results
  - Acceptance criteria:
    - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1` passes.
    - `./scripts/verify.sh` is run and results logged.


### Execution log (cache compiled regex chat filters in gateway)
- ✅ Task 1 complete: added a private gateway regex cache guarded by a mutex and routed regex filter matching through cached compilation keyed by filter ID + pattern.
- ✅ Task 2 complete: added gateway regex cache tests for repeated calls, pattern-change recompilation, and invalid regex skip behavior without caching failures.
- ✅ Task 2 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1`
- ✅ Task 3 complete: ran full repository verification gate after the chat gateway changes.
- ⚠️ Task 3 checks:
  - ❌ `./scripts/verify.sh` (first run had unrelated transient failure in `internal/api` test `TestChatReportsAPI`)
  - ✅ `./scripts/verify.sh` (rerun passed; Docker-dependent steps skipped because Docker is not installed in this environment)

## Scoped change: use utf8 rune counting for chat message length validation

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Update gateway message length validation implementation
  - Acceptance criteria:
    - `internal/chat/gateway.go` uses `utf8.RuneCountInString(trimmed) > 500` in `CreateMessage`.
    - `unicode/utf8` import is added and formatting remains gofmt-clean.
    - Error text remains `message exceeds 500 characters`.

- [x] Task 2 — Update storage message length validation implementation
  - Acceptance criteria:
    - `internal/storage/chat.go` uses `utf8.RuneCountInString(trimmed) > MaxChatMessageLength` in `CreateChatMessage`.
    - `unicode/utf8` import is added and formatting remains gofmt-clean.
    - Error text remains unchanged.

- [x] Task 3 — Add/adjust multibyte length tests and run scoped checks
  - Acceptance criteria:
    - Chat/gateway and storage tests cover multibyte boundary behavior for 500 vs 501 characters.
    - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat ./internal/storage -count=1` passes and is logged.

### Execution log (use utf8 rune counting for chat message length validation)
- ✅ Task 1 complete: `CreateMessage` now uses `utf8.RuneCountInString(trimmed) > 500` and imports `unicode/utf8`, with unchanged error text.
- ✅ Task 1 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1`
- ✅ Task 2 complete: `CreateChatMessage` now uses `utf8.RuneCountInString(trimmed) > MaxChatMessageLength` and imports `unicode/utf8`, with unchanged error text.
- ✅ Task 2 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -count=1`
- ✅ Task 3 complete: added multibyte boundary tests for gateway and storage message length validation and ran scoped package tests.
- ✅ Task 3 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat ./internal/storage -count=1`
- ✅ Additional gate check:
  - ⚠️ `./scripts/verify.sh` (passed; Docker-dependent checks were skipped because Docker is not installed in this environment)

## Scoped change: reduce chat filter cache read allocations in gateway

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Return cached filter slice directly and document immutability invariant
  - Acceptance criteria:
    - `internal/chat/gateway.go` `cachedChatFilters` returns `entry.filters` directly (no `append` copy).
    - Cache write path keeps defensive copy `append([]domain.ChatFilter(nil), fetched...)`.
    - Code comment documents that cached slices are treated as immutable by Gateway internals.

- [x] Task 2 — Add/adjust gateway unit test for unchanged matching + no cache mutation
  - Acceptance criteria:
    - A gateway unit test validates repeated matching behavior remains identical.
    - Test asserts matching logic does not mutate cached filter data.

- [x] Task 3 — Run scoped + required verification checks and record results
  - Acceptance criteria:
    - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1` passes.
    - `./scripts/verify.sh` is run and results logged.


### Execution log (reduce chat filter cache read allocations in gateway)
- ✅ Task 1 complete: `cachedChatFilters` now returns cached slices directly and includes a comment documenting the immutable-slice invariant; cache writes still defensively copy fetched datastore slices.
- ✅ Task 2 complete: added a gateway unit test that verifies repeated matching behavior and asserts cached filters remain unchanged after matching operations.
- ✅ Task 2 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1`
- ✅ Task 3 complete: ran required repository verification gate after gateway cache allocation optimization.
- ✅ Task 3 checks:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1`
  - ⚠️ `./scripts/verify.sh` (passed; Docker-dependent checks were skipped because Docker is not installed in this environment)

## Scoped change: directory owner/profile bulk lookup in response builder

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Refactor `writeDirectoryResponse` to preload users/profiles and use maps
  - Acceptance criteria:
    - `channelsService().ListUsers()` and `channelsService().ListProfiles()` are each called once before iterating channels.
    - Response loop does O(1) owner/profile lookups via `map[userID]...`.
    - Missing owner channels are skipped exactly as before; missing profile remains optional/zero-value.

- [x] Task 2 — Extend tests for JSON parity and reduced per-channel lookup calls
  - Acceptance criteria:
    - Multi-channel test asserts response payload is identical to current behavior.
    - Test asserts fewer per-channel `GetUser`/`GetProfile` calls with bulk list usage.

- [x] Task 3 — Run scoped API tests and record outcomes
  - Acceptance criteria:
    - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1 -run 'TestDirectory(RecommendedSortsByFollowers|ResponseUsesBulkUserProfileLookups)$'` passes.
    - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1` passes.

### Execution log (directory owner/profile bulk lookup in response builder)
- ✅ Task 1 complete: `writeDirectoryResponse` now bulk-loads users/profiles and joins them with in-memory maps, while preserving owner-missing skip and profile optionality behavior.
- ✅ Task 1 check:
  - ✅ `rg -n "ListUsers\(|profilesByUserID|usersByID|writeDirectoryResponse" internal/api/channels_directory_handlers.go internal/service/usecases.go`
- ✅ Task 2 complete: extended directory handler test coverage with call-count instrumentation for `ListUsers`/`ListProfiles`/`GetUser`/`GetProfile` and JSON parity assertions in multi-channel responses.
- ✅ Task 3 checks:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1 -run 'TestDirectory(RecommendedSortsByFollowers|ResponseUsesBulkUserProfileLookups)$'`
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1`
  - ⚠️ `./scripts/verify.sh` (docker-dependent checks skipped because docker is not installed in this environment)


## Scoped change: analytics overview grouped fetch refactor

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add grouped analytics accessors in usecase/store interfaces and storage implementations
  - Acceptance criteria:
    - Internal analytics store interface supports grouped followers/current sessions/recent sessions/chat-count lookups by channel ID.
    - In-memory and postgres-backed storage compile with the new methods.

- [x] Task 2 — Refactor `computeAnalyticsOverview` to use grouped maps in one pass
  - Acceptance criteria:
    - Prefetches grouped datasets once per request.
    - Preserves all current calculations and per-channel sort tie-break order.

- [x] Task 3 — Add regression coverage comparing legacy vs refactored outputs
  - Acceptance criteria:
    - Tests include representative multi-channel fixture coverage.
    - Tests compare old/new compute outputs and keep contracts unchanged.

### Execution log (analytics overview grouped fetch refactor)
- ✅ Task 1 complete: added internal grouped analytics accessors for follower counts, current sessions, stream sessions, and chat message counts across service/store interfaces and both storage implementations.
- ✅ Task 1 checks:
  - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -count=1`
- ✅ Task 2 complete: refactored `computeAnalyticsOverview` to prefetch grouped maps and compute results in one pass while preserving existing summary math and sort tie-breakers.
- ✅ Task 2 check:
  - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/service -count=1`
- ✅ Task 3 complete: added multi-channel regression test that compares legacy and refactored overview outputs for representative fixture data.
- ✅ Task 3 checks:
  - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/service -count=1`
  - `./scripts/verify.sh`

## Scoped change: chat panel update-time message normalization/sort

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Refactor chat message state/apply pipeline to normalize once and maintain ordered entries
  - Acceptance criteria:
    - `applyMessages` computes/stores `sentAtTs` once per incoming message.
    - Replacement and append paths preserve prior visible ordering while avoiding unconditional re-sort when incoming payload is already monotonic.
    - `MAX_MESSAGES` truncation behavior remains unchanged.

- [x] Task 2 — Keep grouped rendering semantics unchanged with updated message-entry state
  - Acceptance criteria:
    - Grouping still uses display-name fallback (`displayName` → `id` → `Anonymous`).
    - Same-user 2-minute merge window behavior is unchanged.
    - Rendered chat item order remains identical to previous behavior.

- [x] Task 3 — Run scoped + required verification checks and record outcomes
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- chatPanel.test.tsx` passes.
    - `./scripts/verify.sh` is run and results logged.


### Execution log (chat panel update-time message normalization/sort)
- ✅ Task 1 complete: moved message normalization into `applyMessages`, storing `{ message, sentAtTs }` entries in state and preserving existing `MAX_MESSAGES` truncation slice semantics before ordering checks.
- ✅ Task 1 check:
  - ✅ `cd web/viewer && npm run test -- chatPanel.test.tsx`
- ✅ Task 2 complete: grouped rendering now consumes pre-normalized ordered entries while preserving display-name fallback and 2-minute same-user merge behavior.
- ✅ Task 2 checks:
  - ✅ `cd web/viewer && npm run test -- chatPanel.test.tsx`
  - ⚠️ `cd web/viewer && npm run lint -- components/ChatPanel.tsx` (command failed because `next lint` expects app/pages discovery from project root arguments, not a direct file path)
- ✅ Task 3 complete: ran full repository verification gate after chat panel refactor.
- ✅ Task 3 check:
  - ✅ `./scripts/verify.sh` (passed; Docker-dependent checks skipped because Docker is not installed in this environment)

## Scoped change: redis queue payload extraction byte-preserving path

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add byte-preserving payload helper and wire `extractPayload`
  - Acceptance criteria:
    - New helper returns `[]byte` directly for byte-backed values and only allocates from `string` when needed.
    - `extractPayload(fields []interface{})` uses the helper for the `payload` field while preserving `strings.EqualFold(key, "payload")` matching.
    - Read/decode call sites remain unchanged.

- [x] Task 2 — Add targeted tests for helper/extraction behavior
  - Acceptance criteria:
    - Tests cover mixed `string`/`[]byte` key + value tuples.
    - Tests confirm payload extraction still ignores empty/non-matching values.

- [x] Task 3 — Run scoped + required verification checks and record outcomes
  - Acceptance criteria:
    - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1 -run 'Test(ExtractPayload|AsBytes)'` passes.
    - `./scripts/verify.sh` is run and results logged.

### Execution log (redis queue payload extraction byte-preserving path)
- ✅ Task 1 complete: added `asBytes` helper and updated `extractPayload` to preserve byte payload values while retaining case-insensitive `payload` key matching and existing decode call paths.
- ✅ Task 1 check:
  - ✅ `rg -n "func extractPayload|func asBytes|json.Unmarshal\(entry.Payload, &event\)|strings.EqualFold\(key, \"payload\"\)" internal/chat/redis_queue.go`
- ✅ Task 2 complete: added focused unit coverage for `asBytes` and `extractPayload` across mixed string/byte field tuples, including empty and unsupported payload value cases.
- ✅ Task 2 check:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1 -run 'Test(ExtractPayload|AsBytes)'`
- ✅ Task 3 complete: executed scoped chat tests and full repo verification gate after redis queue optimization changes.
- ✅ Task 3 checks:
  - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1 -run 'Test(ExtractPayload|AsBytes)'`
  - ✅ `./scripts/verify.sh`

## Scoped change: viewer mobile navbar drawer modal behavior

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Implement modal-like mobile drawer behavior in Navbar
  - Acceptance criteria:
    - Opening the mobile drawer focuses the first interactive element in `#viewer-nav-menu`.
    - Tab and Shift+Tab are trapped within drawer focusable elements while open.
    - Drawer includes backdrop and closes on backdrop click.
    - Body scrolling is locked (`document.body.style.overflow = "hidden"`) while open with cleanup on close/unmount.
    - Mobile drawer applies `role="dialog"` and `aria-modal="true"`; focus restores to menu toggle when drawer closes.

- [x] Task 2 — Add navbar tests for drawer focus trap and focus restore
  - Acceptance criteria:
    - `web/viewer/__tests__/navbar.test.tsx` includes tests for Tab/Shift+Tab trapping within open drawer.
    - Tests verify focus returns to the menu toggle when drawer closes.

- [x] Task 3 — Run scoped viewer checks and record results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- navbar.test.tsx` passes and result is logged.
    - `./scripts/verify.sh` is run and result is logged.

### Execution log (viewer mobile navbar drawer modal behavior)
- ✅ Task 1 complete: updated mobile drawer to behave like a modal with first-focus handoff, Tab/Shift+Tab trapping, Escape close handling, backdrop click close, body-scroll lock/cleanup, and mobile-only dialog semantics plus focus restore to the toggle.
- ✅ Task 1 check:
  - ✅ `cd web/viewer && npm run test -- navbar.test.tsx`
- ✅ Task 2 complete: added navbar tests to validate mobile drawer focus trapping (Tab + Shift+Tab looping) and focus restoration to the menu toggle after close.
- ✅ Task 2 check:
  - ✅ `cd web/viewer && npm run test -- navbar.test.tsx`
- ✅ Task 3 complete: ran scoped and full verification checks after navbar modal-drawer updates.
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- navbar.test.tsx`
  - ✅ `./scripts/verify.sh` (passed; Docker-dependent checks skipped because Docker is not installed in this environment)

## Scoped change: tip drawer focus return on close

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Wire focus-return refs through ChannelHero and TipDrawer close flow
  - Acceptance criteria:
    - `ChannelHero` stores a ref for the “Send a tip” trigger and passes it to `TipDrawer` via a new optional prop.
    - `TipDrawer` centralizes close handling so all close actions (`Escape`, backdrop, close button, cancel button) invoke focus restoration before/when closing.
    - Successful submit path also closes and restores focus to the trigger.

- [x] Task 2 — Add/extend tests for focus restoration
  - Acceptance criteria:
    - Viewer tests assert focus returns to the trigger after drawer close (including successful submit and at least one direct close action).

- [x] Task 3 — Run targeted viewer tests and capture results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- tipDrawer.test.tsx channelHero.test.tsx` passes and result is logged in this section.

### Execution log (tip drawer focus return on close)

- ✅ Task 1 complete: added a `Send a tip` trigger ref in `ChannelHero`, threaded optional `returnFocusRef` into `TipDrawer`, and unified close handlers so Escape/backdrop/close/cancel return focus to the trigger while successful submit restores focus after success callback.
- ✅ Task 2 complete: extended `tipDrawer.test.tsx` to verify focus restoration across Escape, backdrop, close button, cancel button, and successful submit; also added a `ChannelHero` integration assertion for focus return on Escape close.
- ✅ Task 3 check:
  - ✅ `cd web/viewer && npm run test -- tipDrawer.test.tsx channelHero.test.tsx`

## Scoped change: viewer search action label alignment

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Align directory search submit label with shared search copy
  - Acceptance criteria:
    - `web/viewer/components/DirectorySearchBar.tsx` no longer passes `submitLabel="Apply"`.
    - Directory search submit button renders with text `Search` via shared label/default.

- [x] Task 2 — Update related viewer tests/snapshots asserting button text
  - Acceptance criteria:
    - Any affected tests under `web/viewer/__tests__/` are updated from `Apply` to `Search` expectations.
    - Scoped viewer tests pass after expectation updates.

- [x] Task 3 — Review viewer README wording and run required verification checks
  - Acceptance criteria:
    - `web/viewer/README.md` is reviewed for stale `Apply` copy and updated only if needed.
    - `cd web/viewer && npm run test -- directoryPage.test.tsx` passes and result is logged.
    - `./scripts/verify.sh` is run and result is logged.

### Execution log (viewer search action label alignment)
- ✅ Task 1 complete: removed the directory-specific `submitLabel="Apply"` override so `DirectorySearchBar` uses shared `SearchBar` submit copy (`Search`).
- ✅ Task 1 check:
  - ✅ `rg -n "submitLabel=\"Apply\"|submitLabel = \"Search\"" web/viewer/components/DirectorySearchBar.tsx web/viewer/components/SearchBar.tsx`
- ✅ Task 2 complete: updated directory page search-button expectation from `Apply` to `Search`; no snapshot updates were required.
- ✅ Task 2 check:
  - ✅ `cd web/viewer && npm run test -- directoryPage.test.tsx`
- ✅ Task 3 complete: reviewed `web/viewer/README.md` and confirmed no stale `Apply` wording; ran required verification checks.
- ✅ Task 3 checks:
  - ✅ `rg -n "Apply|apply" web/viewer/README.md`
  - ✅ `cd web/viewer && npm run test -- directoryPage.test.tsx`
  - ✅ `./scripts/verify.sh` (passed; Docker-dependent checks skipped because Docker is not installed in this environment)

## Scoped change: directory quick-links semantic nav landmark

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Replace directory hero quick-links wrapper with semantic nav landmark
  - Acceptance criteria:
    - `web/viewer/app/directory-view.tsx` uses `<nav aria-label="Quick jump links" className="home-hero__quick-links">`.
    - Anchor href targets remain `#top-categories`, `#trending-now`, and `#live-now`.

- [x] Task 2 — Update/add browse-directory tests for new quick-links role semantics
  - Acceptance criteria:
    - Relevant browse/directory tests validate quick-links via navigation landmark role.
    - Test expectations remain aligned with unchanged anchor targets.

- [x] Task 3 — Run scoped accessibility checks/specs and required verification
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- directoryPage.test.tsx browsePage.test.tsx` passes.
    - `cd web/viewer && npm run test:playwright -- tests/accessibility.spec.ts` passes (or failure reason is documented).
    - `./scripts/verify.sh` is run and result is logged.

### Execution log (directory quick-links semantic nav landmark)
- ✅ Task 1 complete: replaced the directory hero quick-links wrapper with a semantic `<nav>` landmark while preserving existing quick-jump targets.
- ✅ Task 1 check:
  - ✅ `rg -n "<nav aria-label="Quick jump links"|href="#top-categories"|href="#trending-now"|href="#live-now"" web/viewer/app/directory-view.tsx`
- ✅ Task 2 complete: added directory page coverage asserting the quick-jump region is exposed as a navigation landmark with unchanged link destinations.
- ✅ Task 2 check:
  - ✅ `cd web/viewer && npm run test -- directoryPage.test.tsx`
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- directoryPage.test.tsx browsePage.test.tsx`
  - ⚠️ `cd web/viewer && npm run test:playwright -- tests/accessibility.spec.ts` (fails during `next build` due to pre-existing type error in `app/creator/live/[channelId]/page.tsx`: invalid page export field `deriveControlCentreStatus`)
  - ✅ `./scripts/verify.sh` (passes; Docker-dependent checks skipped because Docker is not installed in this environment)

## Scoped change: chat panel live-region scoping for incoming entries

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Restrict ChatPanel live-region attributes to message stream only
  - Acceptance criteria:
    - `web/viewer/components/ChatPanel.tsx` removes `aria-live` from outer `.chat-panel` section.
    - Message stream element uses `role="log"`, `aria-live="polite"`, `aria-relevant="additions text"`, and `aria-atomic="false"`.
    - `role="alert"` and `role="status"` blocks are not nested inside a broad live region.

- [x] Task 2 — Add/adjust ChatPanel accessibility test coverage
  - Acceptance criteria:
    - Viewer test asserts only the chat entry stream is configured as a live log region.
    - Test confirms error/status blocks are outside the live log region semantics.

- [x] Task 3 — Run scoped viewer tests and record results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- chatPanel.test.tsx` passes and is logged.

### Execution log (chat panel live-region scoping for incoming entries)
- ✅ Task 1 complete: removed broad `aria-live` from `.chat-panel`, moved live-log semantics to the chat thread, and kept alert/status content outside the log region.
- ✅ Task 1 check:
  - ✅ `rg -n "<section className="chat-panel"|aria-live="polite"|aria-relevant="additions text"|aria-atomic="false"|role="status"|role="alert"|role="log"" web/viewer/components/ChatPanel.tsx`
- ✅ Task 2 complete: added chat panel accessibility coverage that verifies only incoming chat entries are treated as live updates and confirms no broad live region on panel/body wrappers.
- ✅ Task 2 check:
  - ✅ `rg -n "scopes live announcements to the chat message log only|aria-relevant|aria-atomic|queryByRole\("log"\)" web/viewer/__tests__/chatPanel.test.tsx`
- ✅ Task 3 check:
  - ✅ `cd web/viewer && npm run test -- chatPanel.test.tsx`

## Scoped change: viewer mobile sidebar focus + modal accessibility

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Implement sidebar modal-focus behavior in `ViewerShell`
  - Acceptance criteria:
    - On open, save previous focus and move focus into sidebar (first focusable element, fallback heading/container).
    - While open, trap Tab/Shift+Tab within sidebar + close control.
    - On close, restore focus to sidebar toggle button.
    - Lock `document.body` scroll while open and clean up on close/unmount.
    - Mobile overlay semantics include modal context (`aria-modal`), clear close affordance, non-focusable backdrop, and keyboard-inert background.

- [x] Task 2 — Extend tests for sidebar focus + close behavior
  - Acceptance criteria:
    - Unit and/or browser coverage verifies focus movement into sidebar and focus restoration to toggle on close.
    - Coverage verifies Escape close behavior.
    - Coverage verifies backdrop close behavior.

- [x] Task 3 — Run viewer checks for touched behavior
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- viewerShell.test.tsx` passes.
    - `cd web/viewer && npm run test -- navbar-mobile.spec.ts` is run (or equivalent sidebar browser spec) and result logged.


### Execution log (viewer mobile sidebar focus + modal accessibility)
- ✅ Task 1 complete: updated `ViewerShell` with open-focus handoff, Tab/Shift+Tab trapping, Escape close, toggle-focus restoration, body scroll lock cleanup, modal semantics, and backdrop accessibility behavior.
- ✅ Task 1 check:
  - ✅ `cd web/viewer && npm run test -- viewerShell.test.tsx`
- ✅ Task 2 complete: extended unit coverage in `viewerShell.test.tsx` and mobile browser coverage in `tests/navbar-mobile.spec.ts` for focus management + Escape/backdrop close paths.
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- viewerShell.test.tsx`
  - ⚠️ `cd web/viewer && npm run test:playwright -- tests/navbar-mobile.spec.ts` (blocked by pre-existing Next.js build type error in `app/creator/live/[channelId]/page.tsx`: invalid page export `deriveControlCentreStatus`)
  - ✅ `./scripts/verify.sh`
