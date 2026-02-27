# PLAN

## Scope (current change)
- Update the navbar notifications icon button in `web/viewer/components/Navbar.tsx` to reflect that notifications are not yet implemented by using proper disabled semantics and helper copy.
- Preserve existing icon-button styling while making assistive technology state explicit (`disabled`) and exposing a short tooltip/helper string.
- Add/adjust navbar tests to verify the notifications control is disabled with the expected helper text.

## Assumptions
- There is no existing notifications route/panel in the viewer app, so a disabled “coming soon” control is the correct concrete implementation for now.
- A `title` attribute is acceptable helper text/tooltip for this icon-only button in current UI patterns.

## Risks
- Minor copy drift risk if a dedicated tooltip system is introduced later and this string is not centralized.

## Test plan
- `cd web/viewer && npm run test -- navbar.test.tsx`

## Scope (current change)
- Expand env validation secret handling to treat `*_FILE` as first-class companions for all sensitive environment values validated by `cmd/bitriver env validate`.
- Keep deterministic precedence when both direct and file-based values are provided: direct value wins and validator emits a warning.
- Ensure file-backed secret values flow through existing missing/blocked checks so placeholder and required-value rules still apply.
- Update deployment/docs examples to show `*_FILE` usage, including Docker Compose secret-directory mount patterns.

## Assumptions
- The existing validator behavior (warnings are non-fatal, errors are fatal) should remain unchanged.
- Secret-file support is scoped to env validation and documentation; runtime service config remains env-driven.

## Risks
- Adding new sensitive keys to file-resolution may surface new warnings/errors for existing env files that set both direct and `_FILE` values.
- Documentation examples may drift from compose reality if mount paths are inconsistent across files.

## Test plan
- `go test ./cmd/bitriver -count=1`
- `go test ./... -count=1 -timeout=120s`

## Scope (current change)
- Update `deploy/check-env.sh` so doctor preflight runs by default with both `--env-file` and canonical `--compose-file deploy/docker-compose.yml` before env validation.
- Preserve CI/operator compatibility by keeping default invocation argument-free and adding an explicit escape hatch `--skip-doctor` with documented usage.
- Improve script UX with explicit phase headings and actionable failure guidance for doctor failures.
- Update `docs/quickstart.md` and `docs/production-single-host.md` so `deploy/check-env.sh` is called out as the first environment preflight step.

## Assumptions
- `bitriver doctor` already encodes WARN vs FAIL semantics (WARN should return success; FAIL should return non-zero).
- Existing CI usage calls `bash deploy/check-env.sh` (or equivalent) without positional changes.

## Risks
- Parsing logic for optional `--skip-doctor` could regress if argument handling becomes order-sensitive.
- Doc updates may drift if they duplicate command examples inconsistently between quickstart and production docs.

## Test plan
- `bash deploy/check-env.sh --help`
- `bash deploy/check-env.sh --skip-doctor`
- `bash deploy/check-env.sh`

## Scope (previous change)
- Upgrade `bitriver doctor` into a production preflight with actionable PASS/WARN/FAIL checks while preserving `func runDoctor(args []string) bool` compatibility used by `verify` and `main`.
- Add flags `--env-file`, `--compose-file`, and `--json` to support environment-aware checks and machine-readable output.
- Expand checks to include host sizing, required/optional binaries, Docker/Compose minimum versions, port conflicts, and compose bind-mount readability/writability.
- Document the preflight workflow and minimum host guidance in operations/production docs.

## Assumptions
- Existing quickstart port requirement helpers remain authoritative for env-driven service ports.
- Compose file parsing should be best-effort using stdlib only (no heavy YAML dependency).
- `verify` must continue to call `runDoctor(nil)` unchanged.

## Risks
- Compose parsing false positives/negatives if lines use uncommon YAML shapes.
- OS-specific host resource probes may be incomplete outside Linux and should degrade to WARN.
- Version parsing may fail on unusual Docker output; should warn with manual remediation.

## Test plan
- `go test ./... -count=1`
- `go run ./cmd/bitriver doctor --compose-file deploy/docker-compose.yml`
- `go run ./cmd/bitriver doctor --json --compose-file deploy/docker-compose.yml`
- `go run ./cmd/bitriver doctor --compose-file deploy/does-not-exist.yml` (expect FAIL/non-zero)
- `go run ./cmd/bitriver verify`

## Scope (current change)
- Replace Swarm-style `deploy.resources` usage in `deploy/docker-compose.resources.yml` with a Docker Compose (non-Swarm) safe model.
- Keep `deploy/docker-compose.limits.yml` as the enforceable CPU/memory overlay (`cpus`, `mem_limit`, `mem_reservation`) and keep `deploy/docker-compose.resources.yml` focused on ingest `ulimits` only.
- Clarify operator docs so production commands recommend the limits overlay and explain when to layer the ulimits overlay.

## Assumptions
- `deploy/docker-compose.limits.yml` already contains env-driven knobs and compose-compatible fields for key services.
- `cmd/bitriver env validate` already validates `*_CPUS`, `*_MEM`, and `*_MEM_RESERVATION` formats.

## Risks
- Documentation drift if some runbooks continue to present `docker-compose.resources.yml` as a CPU/memory limits overlay.
- Operators may miss the optional ulimits layer unless commands/examples clearly show both overlays when needed.

## Test plan
- `docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.limits.yml config`
- `docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.limits.yml -f deploy/docker-compose.resources.yml config`
- `go test ./... -count=1 -timeout=120s`

## Scope (current change)
- Refine `bitriver upgrade-plan` into an operator checklist command with explicit `--compose-file`, `--env-file`, and required `--target` flags.
- Detect currently running Compose service image tags via `docker compose ps --format json` on a best-effort basis, with env-file fallback and WARN guidance when unavailable.
- Print upgrade planning guidance that references docs backup procedures, states migration behavior when detectable from compose contract, and includes rollback caveats.
- Update `docs/upgrades.md` with the new command syntax and a realistic sample checklist output.

## Assumptions
- Docker may be unavailable or the stack may be stopped; planner output should still be usable.
- Existing migration contract in docs (`postgres-migrations` for compose deployments) remains accurate for the default compose file.

## Risks
- Compose `ps --format json` schema can vary across Docker versions; parser must tolerate missing fields.
- Image references can include digest-only forms, so tag extraction should be defensive and may produce `unknown` entries.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./cmd/bitriver -count=1`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s`

## Scope (current change)
- Assess production-release readiness using the repository's required verification gate (`./scripts/verify.sh`).
- Address any release-blocking failures found in the default gate with the smallest safe change.

## Assumptions
- Passing `./scripts/verify.sh` is the project's baseline production-readiness signal for this scope.

## Risks
- Placeholder hygiene fixes in `deploy/.env.example` must remain clearly non-secret examples and aligned with docs.

## Test plan
- `./scripts/verify.sh`

## Scope (current change)
- Improve `ChatPanel` modal accessibility for both pop-out and settings dialogs in `web/viewer/components/ChatPanel.tsx`.
- Ensure dialog open/close focus lifecycle is deterministic: move focus into active dialog, trap tab navigation, support Escape close, and restore focus to the originating trigger.
- Enforce single-active-modal behavior so opening one dialog closes the other.
- Add viewer tests in `web/viewer/__tests__/chatPanel.test.tsx` for Escape close, in-dialog tab cycling, and trigger focus restoration.

## Assumptions
- Existing dialog headings/buttons are acceptable initial focus targets; we can focus the heading by making it programmatically focusable.
- `jsdom` focus behavior in existing viewer tests can validate focus trap + restoration semantics using `userEvent.tab()` and key events.

## Risks
- Focus trap selector scope could unintentionally skip controls if visibility filtering is too strict.
- Keyboard handler overlap between two dialogs could cause close/open race conditions without explicit single-active-modal state transitions.

## Test plan
- `cd web/viewer && npm run test -- chatPanel.test.tsx`
- `./scripts/verify.sh --viewer`
