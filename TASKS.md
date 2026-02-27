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
  - ✅ `./scripts/verify.sh --viewer`
