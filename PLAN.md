# PLAN

## Scope (current change)
- Clarify Following surface summary counts by making labels explicit in `web/viewer/components/FollowingSidebar.tsx` and `web/viewer/components/FollowingRail.tsx`.
- Decide and encode count semantics per surface (`followed creators` vs `live now`) based on existing data shown in each component.
- Reuse shared copy constants from `web/viewer/components/following/FollowingState.tsx` for summary labels and state text to prevent wording drift.
- Keep empty and unauthenticated terminology consistent across rail/sidebar prompts (specifically “channels you follow” vs “creators you follow”).

## Assumptions
- `FollowingSidebar` count reflects total followed channels returned by `fetchFollowingChannels()` for the signed-in user.
- `FollowingRail` presents currently live followed channels, so its ready-state summary should use a live-now label.
- This is viewer copy/label behavior; no backend/API contract changes are needed.

## Risks
- Tests that assert old inline summary strings may fail until updated.
- Introducing new shared constants could be partially adopted, leaving mixed terminology if not fully wired into both components.

## Test plan
- `cd web/viewer && npm run test -- followingStatePresentation.test.tsx followingSidebar.test.tsx`

## Scope (current change)
- Standardize viewer-facing VOD terminology around the existing heading term “Past broadcasts” in `web/viewer/components/VodGallery.tsx` and adjacent channel page copy.
- Update VOD gallery empty, loading, and error states to consistently reference “past broadcasts”.
- Align nearby channel page fallback error text and replay CTA wording so users do not see mixed terms like “VODs”/“replays”.

## Assumptions
- This is copy-only behavior; no API shape or data model changes are required.
- Existing channel page tests in `web/viewer/__tests__/channelPage.test.tsx` cover affected loading/error/empty strings and will be updated for the new wording.

## Risks
- String assertion drift in viewer tests if any old wording remains.
- Minor UX ambiguity risk for singular CTA wording (“Watch past broadcast”) if not matched exactly in assertions/content review.

## Test plan
- `cd web/viewer && npm run test -- channelPage.test.tsx`

## Scope (current change)
- Update the channel page error panel in `web/viewer/app/channels/[id]/page.tsx` to keep one consistent user-facing message style.
- Replace inline raw error rendering with friendly guidance text, while exposing raw diagnostics only in non-production environments.
- Align fallback copy in `setError(...)` and `setVodError(...)` with the existing “We couldn’t…” wording.
- Keep retry actions and current headline/body structure unchanged.

## Assumptions
- Existing channel page tests under `web/viewer/__tests__/channelPage.test.tsx` cover the alert surface and can be extended if needed.
- `process.env.NODE_ENV` checks in this client component are acceptable for dev-only diagnostic blocks.

## Risks
- Changing fallback copy may require test-string updates where assertions currently look for “Unable to…”.
- Dev-only diagnostics could accidentally leak to production if the environment guard is implemented incorrectly.

## Test plan
- `cd web/viewer && npm run test -- channelPage.test.tsx`

## Scope (current change)
- Add an internal `internal/app` helper that maps `storage.ObjectStorageConfig` to `api.UploadSourceStorageConfig`.
- Replace duplicated field-by-field mapping in `internal/app/http.go` (`NewHandler`) and `internal/app/server_runtime.go` (upload processor setup) with the new helper.
- Add a focused unit test in `internal/app` asserting every mapped field is preserved exactly.
- Keep runtime behavior unchanged aside from deduplicating the mapping logic.

## Assumptions
- The mapping surface remains exactly: `Endpoint`, `Bucket`, `Prefix`, `PublicEndpoint`, `UseSSL`, and `RequestTimeout`.
- No call-sites outside `internal/app` need this helper for this scope.

## Risks
- A helper signature mismatch could accidentally alter zero-value handling if fields are omitted.
- Refactoring call-sites could unintentionally change behavior if either site had subtle differences.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/app -count=1`

## Scope (current change)
- Update `web/viewer/components/FeaturedChannel.tsx` to respect `prefers-reduced-motion` when deciding carousel autoplay behavior.
- Default autoplay to off when reduced motion is preferred while preserving a prop-based override path for explicit opt-in/out.
- React to runtime preference changes via media-query subscription and prevent autoplay interval setup whenever reduced-motion mode is active.
- Add viewer tests under `web/viewer/__tests__/` covering initial reduced-motion behavior, manual Play re-enable, and dynamic reduce-preference transitions.

## Assumptions
- `autoPlay` prop remains the explicit policy knob from parent components; reduced-motion handling should only change the initial/user-state behavior when no explicit opt-out is in effect.
- Existing Play/Pause button is the user override surface and should remain functional even when reduced-motion defaults autoplay off.

## Risks
- Media query listener compatibility differences (`addEventListener` vs `addListener`) can break updates in test/runtime if not handled defensively.
- New autoplay precedence rules could regress existing expectations where `autoPlay` prop changes are used to force state.

## Test plan
- `cd web/viewer && npm run test -- channelDisplayPrimitives.test.tsx`

## Scope (current change)
- Adjust image loading behavior in `web/viewer/components/DirectoryGrid.tsx` and `web/viewer/components/LiveNowGrid.tsx` so `next/image` `priority` is applied only to the leading visible card(s) instead of every mapped card.
- Keep existing `sizes` values unchanged to preserve responsive image selection behavior.
- Add/update component tests to validate that only expected leading cards receive priority behavior while subsequent cards are lazy-loaded.

## Assumptions
- For current layouts, prioritizing only the first card in each grid (`index < 1`) is the intended optimization baseline unless tests/layout indicate otherwise.
- In this test setup, `next/image` props can be asserted via attributes emitted by the existing test environment.

## Risks
- If the layout above-the-fold shows multiple cards on large screens, prioritizing only one card could under-fetch some initially visible media.
- Test assertions may be coupled to the current `next/image` test shim behavior and require updates if that shim changes.

## Test plan
- `cd web/viewer && npm run test -- channelDisplayPrimitives.test.tsx`

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


## Scope (current change)
- Improve authenticated avatar-menu accessibility/keyboard behavior in `web/viewer/components/Navbar.tsx` by closing on outside interaction and Escape.
- Add refs for the avatar toggle and menu container so outside-click logic can accurately detect whether pointer/click targets are outside the account-menu controls.
- Restore focus to the avatar toggle when the user menu closes because of Escape/outside interaction.
- Link the avatar button and popup menu with `aria-controls` and a stable menu id.
- Add navbar tests in `web/viewer/__tests__/navbar.test.tsx` validating outside click close, Escape close, and focus restoration.

## Assumptions
- The requested close behavior applies only when the user avatar menu is open and should not change normal toggle behavior.
- Focus restoration is required for Escape/outside-dismiss paths; standard button-toggle close behavior can remain unchanged.

## Risks
- Using document-level listeners without proper cleanup could leak handlers or cause duplicate close events across rerenders.
- Outside-click handling that listens to both pointer and click events can trigger duplicate close attempts if not idempotent.

## Test plan
- `cd web/viewer && npm run test -- navbar.test.tsx`
- `./scripts/verify.sh --viewer`


## Scope (current change)
- Refactor `internal/app/NewServerRuntime` to register rollback cleanup for each successfully created closeable dependency (`store`, Postgres session store, Postgres MFA store) and only disable those defers once runtime assembly succeeds.
- Preserve existing external error outcomes while adding safe stage context to returned errors.
- Add constructor-failure tests in `internal/app` that inject failures after partial setup and assert no resource closers leak on error paths.

## Assumptions
- Constructor behavior remains identical for successful initialization and shutdown paths; only constructor-time rollback cleanup is added.
- Test-time dependency injection can be done through package-level constructor function variables without affecting production behavior.

## Risks
- Over-wrapping errors could break existing exact-match assertions if messages change too aggressively.
- Newly introduced constructor indirection must avoid data races in tests (reset via `t.Cleanup`).

## Test plan
- `go test ./internal/app -count=1`

## Scope (current change)
- Refactor `internal/server/New` by extracting helper functions for handler mutation, route registration, and middleware chain assembly without changing constructor inputs/outputs or runtime behavior.
- Keep route registration coverage explicit for `/healthz`, `/metrics`, `/api/*`, static assets, and `/viewer` wiring.
- Preserve middleware composition order exactly as today and lock it with focused tests in `internal/server/server_test.go`.

## Assumptions
- Helper extraction is purely structural and must not alter route handlers, middleware order, or configuration precedence.
- Existing tests around auth/metrics/viewer remain valid and can be extended to verify route availability and middleware ordering.

## Risks
- Moving route registration into helpers may accidentally omit or reorder handlers if not copied verbatim.
- Middleware order regressions could silently alter auth/csrf/rate-limit behavior unless test assertions target the effective chain ordering.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1`
- `./scripts/verify.sh`

## Scope (current change)
- Update `internal/server/cors.go` so CORS middleware preserves existing `Vary` header values and only appends `Origin` when missing.
- Keep all existing CORS allow/expose headers and preflight/status behavior unchanged.
- Add server CORS test coverage proving upstream `Vary` values are merged with `Origin` rather than overwritten.

## Assumptions
- Go's header canonicalization and comma-separated `Vary` serialization are acceptable for assertion (`Accept-Encoding, Origin` ordering from append order).
- Existing middleware/header behavior should remain identical except for preserving previously-set `Vary` tokens.

## Risks
- Naive string matching on existing `Vary` values could miss case-insensitive duplicates unless normalized.
- Test could become flaky if it over-specifies formatting beyond token presence/order.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1`

## Scope (current change)
- Harden CSRF cookie issuance in `internal/server/csrf.go` so token-generation failures are surfaced as errors instead of silently setting empty cookie values.
- Update CSRF middleware flow to deny protected cookie-auth requests when CSRF token creation fails, while preserving exempt-path and bearer-token bypass behavior.
- Add server middleware tests that simulate CSRF token generation failure and assert protected requests return forbidden without invoking downstream handlers.

## Assumptions
- Protected requests that require CSRF and need a newly issued token should fail closed when token generation fails.
- Existing bypass paths (`csrfPathExempt`, bearer-auth skip, safe methods) must remain unchanged.

## Risks
- Introducing token-generator indirection for testing could leak test overrides if not restored per test.
- New denial path may alter expected error-body text for failure cases that previously produced generic invalid-token errors.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1`

## Scope (current change)
- Perform a documentation-only cleanup in `internal/server/ratelimit.go`, `internal/server/request_id.go`, `internal/server/security.go`, and `internal/storage/postgres_repository.go`.
- Remove inaccurate comment text that claims bool/string-returning helpers return errors.
- Simplify repetitive boilerplate comment blocks so function intent is clearer while keeping exported API docs accurate.
- Keep implementation logic unchanged.

## Assumptions
- This pass is strictly comment/docs cleanup and should not alter behavior, signatures, or tests.
- Scoped Go tests for touched packages are sufficient validation.

## Risks
- Broad comment replacement could accidentally touch code if edits are not constrained to comment lines.
- Over-pruning comments could remove useful exported-doc context.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server ./internal/storage -count=1`

## Scoped change: server runtime shutdown close warning logs

### Scope
- Update `ServerRuntime.Shutdown(ctx)` in `internal/app/server_runtime.go` to replace ignored close errors for `store`, `session_store`, and `mfa_store` with warning logs that include component identifiers.
- Preserve existing shutdown order and non-fatal behavior (do not return early on close errors).
- Add targeted tests in `internal/app` verifying warning emission and continued shutdown when closers fail.

### Assumptions
- Existing `Shutdown` behavior is intentionally best-effort and should keep proceeding after any close failure.
- Warning-level structured logs can be asserted via `slog` text handler output content.

### Risks
- Log message/key changes could make tests brittle; assertions should focus on stable substrings (message + component identifiers).
- Test setup must avoid introducing dependency on full runtime construction.

### Test plan
- Run focused Go tests for `internal/app` shutdown behavior additions.
- Run full required verification gate `./scripts/verify.sh` before finalizing.

## Scope (current change)
- Normalize creator live status labels in `web/viewer/app/creator/live/[channelId]/page.tsx` by introducing a single label map for offline/starting/live/error-facing states.
- Reuse the same label source in both `deriveControlCentreStatus(...)` and `testPanelStatus` so each label has one predictable meaning.
- Keep test-panel instructions concise while preserving current UX intent for idle/live/degraded/error situations.
- Update viewer tests that assert `deriveControlCentreStatus` label behavior.

## Assumptions
- `deriveControlCentreStatus` remains the canonical logic source for stream state derivation; this change centralizes labels, not state transitions.
- Existing `creatorLiveStreamStatus` tests are the primary assertions for label strings and should be updated in step with copy changes.

## Risks
- Label harmonization could unintentionally alter creator-facing wording relied on elsewhere if any UI snapshots/assertions depend on old strings.
- Over-centralizing labels without explicit semantic naming could reduce readability unless keys map clearly to meanings.

## Test plan
- `cd web/viewer && npm run test -- creatorLiveStreamStatus.test.ts`

## Scope (current change)
- Refactor browse directory `sortedChannels` computation in `web/viewer/app/browse/page.tsx` to cache parsed `createdAt` timestamps before sorting.
- Preserve current filter behavior and sort output ordering for `live`, `trending`, and `new` modes.
- Ensure memo result still returns original channel entries so rendering behavior stays unchanged.

## Assumptions
- `channel.createdAt` values are valid date strings currently parsed inline in the `new` comparator.
- This is a compute-only refactor with no UI copy or contract changes.

## Risks
- Accidentally changing tie-break behavior or mutation order during map/filter/sort refactor.
- Returning wrapped helper objects instead of original entries would alter downstream assumptions.

## Test plan
- `cd web/viewer && npm run lint -- app/browse/page.tsx`
- `cd web/viewer && npm run test -- browsePage.test.tsx`

## Scope (current change)
- Refactor `web/viewer/components/ChatPanel.tsx` to parse each message `sentAt` timestamp once per memo cycle and reuse the cached numeric value.
- Add a normalized `useMemo` projection that preserves the original `ChatMessage` object while storing `sentAtTs` for sort/group calculations.
- Keep `MAX_MESSAGES` truncation, sort order, and 2-minute same-user grouping semantics unchanged.
- Ensure rendering still uses the original message fields (`id`, `sentAt`, `message`, `user`, etc.).

## Assumptions
- `sentAt` remains ISO-compatible so `new Date(message.sentAt).getTime()` is still the canonical parse behavior; we are only caching its result.
- Existing ChatPanel tests (if any) are sufficient to catch regressions in grouping/order semantics.

## Risks
- Accidentally switching grouped message objects to a normalized shape could break JSX that expects raw `ChatMessage` fields.
- Comparator/window logic could subtly change if previous-message timestamp source is not aligned with existing behavior.

## Test plan
- `cd web/viewer && npm run test -- chatPanel.test.tsx`

## Scope (current change)
- Gate channel VOD fetching in `web/viewer/app/channels/[id]/page.tsx` so network requests happen only when the Videos tab is actually viewed.
- Track per-channel VOD request state and reset that state when `id` changes.
- Preserve existing VOD loading/error/retry UX and keep `VodGallery` prop wiring unchanged.
- Prevent repeated refetch on tab toggles after an initial successful/failed fetch unless the viewer explicitly retries.

## Assumptions
- Existing channel page tests in `web/viewer/__tests__/channelPage.test.tsx` can be extended to verify fetch timing without changing unrelated behavior.
- Keeping prior VOD data visible when leaving/returning to Videos on the same channel is acceptable and aligns with current state retention.

## Risks
- Effect dependency mistakes could reintroduce repeated requests or skip first fetch when opening Videos.
- Channel change reset logic could accidentally leak prior channel VOD request state if not keyed correctly.

## Test plan
- `cd web/viewer && npm run test -- channelPage.test.tsx`

## Scope (current change)
- Add a private compiled-regex cache to `internal/chat/Gateway` so regex chat filters are reused across repeated message evaluations.
- Update `matchChatFilter` regex handling to key cache entries by filter identity + pattern content so changed patterns trigger recompilation.
- Preserve invalid-regex behavior (warn and skip) while avoiding cache inserts for failed compilations.
- Add focused chat gateway tests validating unchanged moderation behavior over repeated calls and validating cache reuse/recompile semantics.

## Assumptions
- Regex cache scope can be process-local on each `Gateway` instance; no cross-instance sharing is required.
- Keying on `filter.ID` + trimmed `filter.Pattern` is sufficient to invalidate cache when content changes.

## Risks
- Incorrect cache locking could introduce races under concurrent chat evaluation.
- Returning cached regex for stale keys could mask pattern updates if key construction is too coarse.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1`
- `./scripts/verify.sh`
