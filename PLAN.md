## Scope (current change)
- Improve the public-facing root `README.md` by featuring the new `bitriver-live-banner-text.png` asset near the top of the file.
- Tighten the opening README copy so the project pitch reads more clearly to first-time visitors without changing any technical guidance or deployment contracts.
- Keep this as a docs-only change and verify the asset path plus the standard repository verification gate.

## Assumptions
- Leaving `bitriver-live-banner-text.png` in the repo root and referencing it directly from `README.md` is the smallest, clearest way to publish the banner.
- The rest of the README structure remains accurate; only the top presentation copy needs polishing.
- A docs-only change should not require contract or runtime documentation updates outside `README.md`.

## Risks
- Moving or renaming the image unnecessarily could create a broken public README image link, so the asset path should stay simple.
- Rewording the intro too aggressively could drift from the repo’s existing deployment framing, so the copy change should stay tight and factual.
- `./scripts/verify.sh` may surface unrelated repository issues, so verification results should distinguish README work from ambient failures if any appear.

## Test plan
- `Test-Path bitriver-live-banner-text.png`
- `Get-Content README.md -TotalCount 20`
- `./scripts/verify.sh`

## Scope (current change)
- Fix the `github.com/jackc/puddle/v2` local replacement so postgres-tagged builds do not fail with `build constraints exclude all Go files`.
- Keep the change narrowly scoped to vendored dependency buildability; do not change runtime behavior, module contracts, or dependency sources.
- Verify the exact failing path with postgres-tagged tests and the standard repository verification gate.

## Assumptions
- The failure is caused by the lone file in `third_party/github.com/jackc/puddle/v2` being gated behind `//go:build !postgres`, while `go.mod` always replaces `github.com/jackc/puddle/v2` with that local directory.
- Existing tests already define the expected behavior; we only need the vendored package to remain buildable when `-tags postgres` is used.
- We should not run `go mod tidy` or change dependency sourcing unless the local replacement is actually missing or corrupt, which it is not in this checkout.

## Risks
- Relaxing the build constraint could allow the local puddle stub to compile in postgres-tagged builds where a future workflow expected a different implementation.
- A too-broad change in `third_party` could affect non-postgres builds, so the diff should stay minimal and isolated to this package.
- Other postgres-tagged issues may still exist after this fix; we should verify the reproduced failing command directly before inferring broader success.

## Test plan
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test -tags postgres ./internal/auth -count=1 -timeout=120s`
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/auth -count=1 -timeout=120s`
- `./scripts/verify.sh`

## Scope (current change)
- Audit the repository for stale, brittle, or inefficient hotspots and record an ordered remediation plan in `docs/cleanup-plan.md`.
- Complete only `docs/cleanup-plan.md` task 1 in this change: make `internal/executil` tests platform-neutral without changing runtime behavior or public APIs.
- Re-run targeted verification plus the repo verification entrypoint, and record any host-specific blockers explicitly.

## Assumptions
- The current `internal/executil` failures come from POSIX-only test fixtures (`sh`, `head`, `/dev/zero`), not from a required product behavior on Windows.
- A helper subprocess launched via the Go test binary can preserve the existing `CommandError` assertions while removing shell dependencies.
- This scoped cleanup does not change runtime behavior, routes, CLI output, or deployment contracts.

## Risks
- Helper-process tests can recurse indefinitely if the env guard is too loose.
- Changing how stderr is generated could accidentally weaken the truncation coverage if the helper output is too small.
- `./scripts/verify.sh` may still be blocked by this host's Bash/WSL setup even after the targeted package fix lands; the blocker needs to be captured rather than masked.

## Test plan
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/executil -count=1 -timeout=120s`
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./... -count=1 -timeout=120s`
- `./scripts/verify.sh`

## Scope (current change)
- Clean up repo Git hygiene so local build/test/cache artifacts do not flood staging during normal development or Codex-assisted work.
- Ignore the known generated directories created by the current Go/viewer verification flow on this Windows host (`.gocache`, `.npm-cache`, `web/viewer/.next`, `web/viewer/playwright-report`, `web/viewer/test-results`).
- Pin the generated OME contract file to a stable line-ending policy so `deploy/ome/Server.generated.xml` does not appear as a false-positive modified file after validation runs.
- Restore a development-friendly Git working tree by un-staging generated artifacts and verifying status only reports intentional source/doc changes.

## Assumptions
- The generated directories above are local development outputs and should never be committed from this repository.
- Adding targeted ignore rules and a narrow `.gitattributes` entry is sufficient; no deployment contract or runtime behavior changes are required.
- Cleaning the Git index for generated artifacts is safe because these files were produced by local verification commands, not authored source changes.

## Risks
- Ignore rules that are too broad could accidentally hide a file the repo intends to track, so changes should stay path-specific.
- Updating `.gitattributes` for the generated OME XML could require a worktree refresh to fully clear existing line-ending noise on Windows.
- The current index may still contain staged generated paths until we explicitly unstage them after updating ignore policy.

## Test plan
- `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live check-ignore -v .gocache/example .npm-cache/example web/viewer/.next/example web/viewer/playwright-report/index.html web/viewer/test-results/.last-run.json`
- `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live restore --staged .gocache .npm-cache web/viewer/.next web/viewer/playwright-report web/viewer/test-results`
- `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live restore --worktree deploy/ome/Server.generated.xml`
- `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live status --short`

## Scope (current change)
- Clear the current viewer release-blocking `next build` failures caused by route-page export violations and immediate type-safety issues surfaced by rerunning the production build.
- Preserve the creator live control-centre and directory/homepage behavior while moving reusable status/shell/boundary logic into build-safe modules that both page code and unit tests can import.
- Keep browse-directory behavior unchanged while tightening query typing so production builds can complete.
- Re-run viewer production validation so the self-hosted creator go-live and directory experience can ship through the documented release path again.

## Assumptions
- The fastest production-readiness win is to remove known release-gate blockers before expanding product scope.
- `deriveControlCentreStatus` is pure presentation logic and can be extracted without changing backend contracts or runtime behavior.
- Viewer-only source/test changes do not require deployment contract edits.

## Risks
- Refactoring the helper out of the page could introduce import/type drift between the page and its tests.
- Once this export issue is fixed, `next build` may surface additional latent viewer typing or routing errors.
- Browser-spec execution may still depend on local Playwright/browser availability even after the build is fixed.

## Test plan
- `cd web/viewer && npm run test -- creatorLiveStreamStatus.test.ts`
- `cd web/viewer && npm run test -- directoryPage.test.tsx`
- `cd web/viewer && npm run test -- browsePage.test.tsx`
- `cd web/viewer && npm run build`
- `cd web/viewer && npm run test:playwright -- tests/creator-live-setup.spec.ts`
- `./scripts/verify.sh --viewer`

## Scope (current change)
- Validate the canonical deployment path on this Windows host by exercising the BitRiver quickstart/Compose flow against `deploy/docker-compose.yml`.
- Prefer repo-documented PowerShell or direct Go entrypoints when Bash wrappers are blocked in this environment, without changing the deployment contract.
- If quickstart or compose smoke exposes a repo issue, implement the smallest fix needed to make the canonical deployment path succeed and re-run validation.
- Record any environment-specific blockers precisely when they prevent a full successful boot (for example shell permissions, missing tools, host port conflicts, or unhealthy containers).

## Assumptions
- Using `scripts/quickstart.ps1` or direct `go run ./cmd/bitriver quickstart` is operationally equivalent to `./scripts/quickstart.sh` for this validation, per repo docs.
- A temporary/generated root `.env` may be needed because this checkout currently does not include one.
- Any code, config, or docs updates triggered by quickstart failures should stay narrowly scoped to the issue blocking deployment validation.

## Risks
- Local Windows shell and permissions issues (Git Bash startup, Go build cache location, git safe-directory detection) may block scripted validation even when the repo logic is healthy.
- Docker image pulls/builds and container startup can fail for host-specific reasons unrelated to the repo, so failures need careful attribution.
- Quickstart may rewrite generated artifacts such as the root `.env` and `deploy/ome/Server.generated.xml`; we need to distinguish expected generated drift from real contract regressions.

## Test plan
- `powershell -File scripts/quickstart.ps1 -ValidateOnly` or `go run ./cmd/bitriver quickstart --help`
- `docker compose --env-file deploy/.env.example -f deploy/docker-compose.yml config`
- `go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml`
- If quickstart fails: `docker compose --env-file .env -f deploy/docker-compose.yml ps` and `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120`
- `./scripts/verify.sh` or the closest documented equivalent subset possible on this host when Bash remains blocked

## Scope (current change)
- Refine the quick-actions notifications affordance in `web/viewer/components/Navbar.tsx` so it is no longer a permanently disabled dead control.
- Keep the bell affordance visible as an enabled control that opens a lightweight dismissible popover with roadmap context.
- Add keyboard and assistive semantics for this control (clear accessible label/description, escape dismissal, and outside-click dismissal).
- Update `web/viewer/__tests__/navbar.test.tsx` to assert the interactive coming-soon behavior and dismiss flows.

## Assumptions
- Keeping the notifications icon visible is preferable to removal for roadmap discoverability, as long as it behaves interactively.
- Existing navbar layout can absorb a small anchored popover without requiring wider responsive refactors.
- Viewer-only component/test updates do not require deployment contract edits.

## Risks
- Popover open/close behavior can become flaky if dismissal listeners are not cleaned up correctly.
- Accessibility naming can regress if icon-only button text and aria attributes conflict.

## Test plan
- `cd web/viewer && npm run test -- navbar.test.tsx`
- `./scripts/verify.sh`

## Scope (current change)
- Implement durable viewer theme preference in `web/viewer/components/Navbar.tsx` by persisting explicit user theme selections in `localStorage` (key: `viewer-theme`).
- On mount, initialize theme from stored preference when present; only use `prefers-color-scheme` when no stored preference exists.
- Gate the `matchMedia("(prefers-color-scheme: light)")` change listener so OS-theme updates only apply for users without an explicit saved preference.
- Centralize body `data-theme` synchronization through a single deterministic effect tied to resolved theme state.
- Add/adjust navbar component tests for initial load with stored preference, initial load without preference, and persisted manual toggle across remount.

## Assumptions
- `localStorage` is available in normal browser execution; SSR and non-browser contexts must no-op safely.
- Existing theme UI contract remains unchanged (same toggle button and aria-label semantics), with behavior updates limited to preference source and persistence.
- Viewer-only changes do not require deployment contract updates.

## Risks
- Theme initialization could race with media-query listeners if not sequenced carefully, causing non-deterministic initial theme in tests.
- Test mocks for `window.matchMedia`/`localStorage` may become brittle if shared helpers do not expose listener behavior.

## Test plan
- `cd web/viewer && npm run test -- navbar.test.tsx`
- `./scripts/verify.sh`

## Scope (current change)
- Strengthen navbar search keyboard focus visibility in `web/viewer/styles/globals.css` without regressing existing navbar visual design.
- Add a high-contrast `.nav-search` focus treatment that is clearly visible for both desktop inline and mobile drawer search variants.
- Define/verify theme variables so the focus treatment remains visible in dark (`:root`) and light (`[data-theme="light"]`) modes.
- Add viewer test coverage that exercises keyboard focus in the navbar search path and asserts the visual focus contract is present.

## Assumptions
- Both `.nav-search--inline` and `.nav-search--drawer` already share the `.nav-search` base class and should inherit one focus-within treatment.
- A CSS-contract assertion (plus navbar keyboard-focus path test) is acceptable for verifying visible focus representation in Jest/jsdom.
- No deployment contract or runtime backend behavior changes are required for this viewer-only CSS/test update.

## Risks
- Focus styling could be too subtle in one theme if ring/border colors are not theme-tuned.
- Drawer/inline search focus tests can be brittle if assertions depend on CSS visibility instead of semantic structure.

## Test plan
- `cd web/viewer && npm run test -- navbar.test.tsx`

## Scope (current change)
- Update `web/viewer/components/ChatPanel.tsx` accessibility live-region semantics so only the message stream is announced as incremental updates.
- Remove the outer chat panel live region and apply `role="log"`, `aria-live="polite"`, `aria-relevant="additions text"`, and `aria-atomic="false"` on the chat-entry stream container only.
- Keep existing `role="alert"` and `role="status"` blocks scoped outside any broad live region to avoid duplicate/overly verbose announcements.
- Add/adjust viewer tests to assert live-region attributes are attached only to incoming chat entries.

## Assumptions
- Chat error and sign-in status affordances should remain accessible via their existing ARIA roles without being part of the chat log live stream.
- Existing chat fetch/send behavior is unchanged; this is an accessibility semantics update only.

## Risks
- Moving live attributes could regress assistive announcement behavior if the log region no longer wraps rendered chat messages.
- Test assertions tied to DOM structure may need small updates to avoid brittle selector coupling.

## Test plan
- `cd web/viewer && npm run test -- chatPanel.test.tsx`

## Scope (current change)
- Implement focus restoration for `TipDrawer` so keyboard focus reliably returns to the `ChannelHero` “Send a tip” trigger after any drawer close path.
- Add a trigger button ref in `web/viewer/components/ChannelHero.tsx` and thread it into `TipDrawer` via a new optional focus-return prop.
- Ensure all existing close entry points (`Escape`, backdrop click, close icon button, cancel button, and successful submit flow) restore focus to that trigger.
- Extend viewer tests to verify focus returns correctly when the drawer closes.

## Assumptions
- The tip drawer trigger remains rendered while the drawer is open, so focusing it on close is valid.
- A nullable ref prop keeps `TipDrawer` reusable in other contexts without requiring focus-return wiring.
- Existing tip submission behavior and status messaging remain unchanged.

## Risks
- Focus restoration could fire before unmount in a way that creates flaky tests if not coordinated with close handlers.
- Centralizing close behavior may accidentally skip one existing close path if any handler bypasses the shared close routine.

## Test plan
- `cd web/viewer && npm run test -- tipDrawer.test.tsx channelHero.test.tsx`

## Scope (current change)
- Refactor `web/viewer/app/page.tsx` so `Suspense` wraps a child async server boundary that performs data loading inside the suspended subtree.
- Keep `DirectoryPage` lightweight by normalizing `searchParams.q` and rendering `<Suspense fallback={<DirectoryPageFallback .../>}>` with a dedicated `DirectoryDataBoundary` child.
- Preserve existing `mapDirectoryError` mapping and `emptyHomeData` fallback semantics while moving home/directory awaits into the boundary component.
- Add/adjust viewer unit coverage to assert fallback loading UI appears while the boundary promise is pending.

## Assumptions
- Existing directory/home fetch helper behavior and API call patterns remain unchanged; only the async boundary placement is refactored.
- Rendering fallback assertions can be validated in Jest by forcing unresolved fetch promises during initial render.
- No deployment contract/docs updates are needed because this is viewer rendering-structure work only.

## Risks
- Async server component testing in Jest can be brittle if fallback timing is not controlled.
- Refactor could accidentally bypass existing error/empty-state mapping if helper return paths drift.

## Test plan
- `cd web/viewer && npm run test -- directoryPage.test.tsx`
- `./scripts/verify.sh`

## Scope (current change)
- Update `web/viewer/components/following/useFollowingChannels.ts` reload behavior so interval refreshes do not force transient `"loading"` state or clear errors when values are unchanged.
- Preserve auth gates and existing unauthenticated/empty/error semantics while reducing no-op state writes.
- Avoid no-op channel state updates when fetched channel IDs/order/length are unchanged.

## Assumptions
- Polling refreshes should still fetch on schedule; only redundant React state writes are removed.
- Semantic channel equality for this optimization is defined as identical channel ID sequence and length.
- Public hook API (`channels`, `status`, `error`, `reload`) remains unchanged.

## Risks
- Incorrect equality checks could suppress legitimate UI updates.
- Ref-based comparison bookkeeping could become stale if not synchronized with state updates.

## Test plan
- `cd web/viewer && npm run test -- followingSidebar.test.tsx`

## Scope (current change)
- Audit `web/viewer/components/ChatPanel.tsx` message derivation for repeated per-render work in the hot chat render path.
- Remove redundant intermediate message normalization allocation while preserving identical sort/group output semantics.
- Keep polling, auth, and UI behavior unchanged; this is a behavior-preserving efficiency refactor.

## Assumptions
- Chat ordering remains based on `sentAt` timestamp ascending, exactly as current behavior.
- Grouping by user and 2-minute window must remain unchanged.
- Existing ChatPanel tests cover user-visible behavior sufficiently for this scoped optimization.

## Risks
- Refactoring memoization boundaries could accidentally change ordering/grouping if comparator inputs drift.
- Removing an intermediate structure could affect types used by grouping logic if not kept equivalent.

## Test plan
- `cd web/viewer && npm run test -- chatPanel.test.tsx`
- `./scripts/verify.sh`

## Scope (current change)
- Add internal analytics bulk/grouped store accessors to fetch follower counts, current sessions, recent sessions, and chat message counts grouped by channel ID.
- Refactor `computeAnalyticsOverview` in `internal/service/usecases.go` to prefetch grouped datasets once and compute per-channel analytics in a single pass.
- Preserve existing analytics calculations, summary behavior, and per-channel tie-break sorting semantics.
- Add regression tests that compare legacy per-channel fetch behavior with the new grouped-data path on representative multi-channel fixtures.

## Assumptions
- New bulk accessors are internal-only and do not alter external API contracts.
- Grouped chat message counts must preserve the prior "messages since UTC day start" semantics.
- Existing watch-time and stream-live calculations remain unchanged.

## Risks
- A mismatch in grouped aggregation ordering/counting could subtly alter existing outputs.
- Introducing new interface methods could break compile-time conformance for storage stubs/fakes.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/service -count=1`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -count=1`

## Scope (current change)
- Refactor directory handlers in `internal/api/channels_directory_handlers.go` to compute follower counts once per request and reuse them for sorting plus response serialization.
- Thread a `map[string]int` follower-count cache through directory request paths so `CountFollowers` is not called redundantly for the same channel.
- Update directory handler tests to verify response ordering and `FollowerCount` values remain unchanged while asserting fewer `CountFollowers` invocations.

## Assumptions
- Directory endpoints should preserve current response ordering semantics, including live-first tie-breaking and `CreatedAt` fallback ordering.
- Missing map entries should behave equivalently to prior behavior (effectively zero followers when absent).
- This is an internal performance refactor with no API contract or docs changes required.

## Risks
- Accidentally changing sort order if follower map wiring is inconsistent between sort and response layers.
- Test spy wiring could miss some paths and under-assert `CountFollowers` call counts.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1 -run Directory`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1`

## Scope (current change)
- Optimize in-memory login limiter cleanup cadence in `internal/server/ratelimit.go` by tracking last cleanup timestamp on `rateLimiter`.
- Keep per-key bucket creation/update behavior unchanged in `AllowLogin`, but gate `cleanupLocked()` calls behind a bounded interval (`loginWindow/2` with a minimum duration).
- Preserve existing stale-bucket eviction semantics by leaving `cleanupLocked()` implementation logic unchanged.
- Extend server rate-limit tests to verify allow/deny behavior is unchanged and stale buckets are eventually evicted when cleanup interval elapses.

## Assumptions
- Login limiter behavior should remain functionally identical from caller perspective aside from reduced cleanup frequency.
- Cleanup throttle interval must stay >0 even for very small windows to avoid per-request cleanup.
- No deployment contract/docs updates are required because this is internal limiter maintenance behavior.

## Risks
- Incorrect cleanup interval gating could allow stale buckets to accumulate too long.
- Time-based test assertions can become flaky if they rely on tight sleeps.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server -count=1`

# PLAN

## Scope (current change)
- Refactor upload list derivation in `web/viewer/components/UploadManager.tsx` into two memoized stages.
- Add `sortedItems` as a dedicated `useMemo` keyed only on `items` that applies `compareUploadsForMonitoring`.
- Update `visibleItems` to consume `sortedItems` and only apply existing filter/search matching semantics.

## Assumptions
- Current upload filter semantics for `active`, `ready`, and `failed` remain unchanged.
- Search matching must stay case-insensitive and continue checking both title and filename.
- This is a behavior-preserving performance refactor; no API/data contract changes are needed.

## Risks
- Accidental dependency changes could cause stale filtering or missed re-sorts.
- Subtle search/filter logic drift could alter currently expected list visibility behavior.

## Test plan
- `cd web/viewer && npm run test -- uploadManager.test.tsx`

## Scope (current change)
- Add an internal per-channel chat-filter cache in `internal/chat/gateway.go` that stores fetched filters with freshness metadata (timestamp/version token).
- Update `matchChatFilter` to prefer cached filters when fresh and fall back to `Store.ListChatFilters(channelID)` when stale/missing, refreshing cache entries after fetch.
- Add a small configurable TTL to `GatewayConfig` with a conservative default so moderation behavior remains effectively real-time.
- Ensure cache access is concurrency-safe for simultaneous chat message moderation checks.
- Extend gateway chat-filter tests to prove matching behavior remains unchanged while validating cache reuse and refresh after TTL expiry.

## Assumptions
- Cache staleness is based on elapsed time since last fetch; callers should never observe errors hidden by stale cache refresh failures.
- A conservative default TTL should be short enough (seconds) to avoid operator-visible moderation drift while still reducing repeated store calls.
- Existing regex compilation cache behavior should remain unchanged and continue to be exercised by existing tests.

## Risks
- Overly long default TTL could delay newly added/disabled filter enforcement.
- Cache synchronization bugs could cause data races or inconsistent filter snapshots under concurrent message creation.
- Tests relying on real time can flake if TTL windows are too tight.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1`


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

## Scope (current change)
- Replace rune-length checks in chat message creation paths with `utf8.RuneCountInString` in `internal/chat/gateway.go` and `internal/storage/chat.go`.
- Keep the existing maximum thresholds and error message text unchanged.
- Add/adjust chat message length tests to confirm multibyte character behavior remains unchanged.

## Assumptions
- `utf8.RuneCountInString` is behaviorally equivalent to `len([]rune(...))` for valid UTF-8 message content while avoiding unnecessary allocations.
- Existing chat/store tests can be extended with focused length-validation coverage.

## Risks
- Missing a validation path could leave mixed length-counting implementations.
- New tests could become brittle if they assert full error strings beyond the existing contract.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat ./internal/storage -count=1`

## Scope (current change)
- Reduce allocations in chat filter cache reads by returning cached filter slices directly from `cachedChatFilters`.
- Preserve cache-write defensive copying in `chatFiltersForChannel` so datastore-returned slices are not aliased into cache storage.
- Document Gateway invariant that cached filter slices are treated as immutable within Gateway internals.
- Add/adjust gateway tests to confirm moderation match outcomes are unchanged and matching logic does not mutate cached filter entries.

## Assumptions
- Gateway internals never mutate `entry.filters` after cache insertion.
- `matchChatFilter` iterates filters read-only, so returning cached slices directly is safe.
- This is an internal performance change with no deployment contract or user-facing behavior impact.

## Risks
- Any future mutation of cached slices would now affect shared cached state.
- Tests must explicitly guard against accidental mutation regression to preserve correctness.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1`
- `./scripts/verify.sh`
## Scope (current change)
- Reduce per-channel user/profile lookup churn in `internal/api/channels_directory_handlers.go` by preloading users/profiles once and doing map lookups in `writeDirectoryResponse`.
- Extend `service.ChannelsDirectoryUseCase` with bulk user listing support so directory handlers can use `ListUsers` + `ListProfiles` instead of repeated `GetUser`/`GetProfile` calls.
- Extend directory handler tests to assert response JSON parity while verifying fewer per-channel lookup calls in multi-channel responses.

## Assumptions
- `ListUsers` and `ListProfiles` represent the same backing data currently accessed by `GetUser`/`GetProfile` and can be used to preserve response semantics.
- Missing owner behavior must remain a skip (`continue`) and missing profile behavior must remain optional/zero-value.
- This is an internal performance refactor with no API contract or docs updates required.

## Risks
- Interface changes to `ChannelsDirectoryUseCase` could break compile-time conformance if any implementation is missed.
- Incorrect map-keying could alter owner/profile joins or accidentally include channels with missing owners.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1 -run 'TestDirectory(RecommendedSortsByFollowers|ResponseUsesBulkUserProfileLookups)$'`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1`

## Scope (current change)
- Refactor `web/viewer/components/ChatPanel.tsx` so message normalization/sort ordering is handled when `applyMessages` updates state instead of per-render memo recomputation.
- Preserve visible chat behavior: chronological ordering, user grouping by display-name fallback logic, and 2-minute merge window semantics.
- Keep both polling replacement (`fetchChannelChat`) and optimistic append (`sendChatMessage`) paths producing identical message order while preserving `MAX_MESSAGES` truncation.

## Assumptions
- Polling payloads are usually monotonic by `sentAt`; we can short-circuit sort work when that invariant holds.
- UI grouping should continue to operate on the same sorted message sequence and same user label derivation.

## Risks
- Incorrect monotonic detection could skip needed sorting and change ordering.
- Moving timestamp parsing into update-time state handling could drift from prior behavior if not applied consistently across replace/append flows.

## Test plan
- `cd web/viewer && npm run test -- chatPanel.test.tsx`
- `./scripts/verify.sh`

## Scope (current change)
- Optimize Redis stream payload extraction in `internal/chat/redis_queue.go` to avoid `[]byte -> string -> []byte` roundtrips when Redis already returns raw bytes.
- Keep payload key detection logic unchanged (`strings.EqualFold(key, "payload")`) and preserve downstream decode behavior.
- Add/adjust targeted chat queue tests for helper behavior across string/byte inputs.

## Assumptions
- Redis stream field keys may arrive as either `string` or `[]byte`, and values may also be either type.
- Returning raw `[]byte` for byte-backed payload values is behaviorally equivalent for `json.Unmarshal(entry.Payload, &event)` callers.
- This is an internal performance refactor with no user-facing runtime contract change.

## Risks
- Helper changes could accidentally alter empty-payload filtering semantics.
- Key/value decoding paths must continue to tolerate mixed `string`/`[]byte` field tuples.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/chat -count=1 -run 'Test(ExtractPayload|AsBytes)'`
- `./scripts/verify.sh`

## Scope (current change)
- Update `web/viewer/components/Navbar.tsx` mobile drawer behavior to act like a modal dialog when open on small screens.
- Add drawer focus management: initial focus into drawer, keyboard Tab/Shift+Tab trapping, backdrop click-to-close, and focus restore to the toggle on close.
- Prevent page scroll while the drawer is open via `document.body.style.overflow = "hidden"` with reliable cleanup.
- Add mobile-only dialog semantics (`aria-modal`, `role="dialog"`) while preserving existing desktop navigation semantics.
- Extend `web/viewer/__tests__/navbar.test.tsx` with coverage for drawer focus trap and focus restoration.

## Assumptions
- Drawer modal semantics should apply only to the mobile presentation (`(max-width: 800px)`), not desktop/tablet layouts.
- Existing route/navigation behavior stays unchanged; only accessibility and interaction behavior for open drawer is adjusted.
- JSDOM focus behavior can validate tab order when focusable elements are explicitly queried.

## Risks
- Overriding `document.body.style.overflow` could conflict with other overlays if cleanup is incomplete.
- Keyboard event handling for Tab trapping could regress if focusable selector misses expected controls.
- Mobile-only semantics in tests may require deterministic `matchMedia` stubs to avoid flaky expectations.

## Test plan
- `cd web/viewer && npm run test -- navbar.test.tsx`
- `./scripts/verify.sh`

## Scope (current change)
- Align directory search submit copy by changing `DirectorySearchBar` to use `Search` label (matching shared `SearchBar` default semantics) instead of `Apply`.
- Update viewer tests under `web/viewer/__tests__/` that assert the search submit button text.
- Review `web/viewer/README.md` for stale `Apply` wording and update only if needed.

## Assumptions
- `SearchBar` already defaults submit text to `Search`; this change only removes an overriding label in directory usage.
- Existing search behavior and routing remain unchanged; only button text/copy assertions are affected.
- No deployment contract/docs outside viewer README are impacted.

## Risks
- Tests may assert old `/apply/i` accessible-name matching and fail until updated.
- If a snapshot indirectly captures the old label, it may need regeneration.

## Test plan
- `cd web/viewer && npm run test -- directoryPage.test.tsx`
- `./scripts/verify.sh`

## Scope (current change)
- Update `web/viewer/app/directory-view.tsx` quick-links wrapper from a generic `<div>` to semantic `<nav aria-label="Quick jump links" className="home-hero__quick-links">`.
- Preserve all existing quick-jump anchors and targets (`#top-categories`, `#trending-now`, `#live-now`).
- Re-run browse/directory accessibility coverage and adjust tests if role semantics changed.

## Assumptions
- This is a semantic-only markup change; styling and navigation behavior should remain identical.
- Existing browse/directory Playwright accessibility coverage is in `web/viewer/tests/accessibility.spec.ts`.
- Viewer unit tests may need expectation updates only if they assert previous non-landmark semantics.

## Risks
- Role-semantic changes can break tests that previously queried generic containers.
- E2E accessibility specs may be slower/flakier in CI-like environments.

## Test plan
- `cd web/viewer && npm run test -- directoryPage.test.tsx browsePage.test.tsx`
- `cd web/viewer && npm run test:e2e -- accessibility.spec.ts`
- `./scripts/verify.sh`

## Scope (current change)
- Improve `web/viewer/components/ViewerShell.tsx` mobile sidebar accessibility behavior: focus entry, focus trapping, focus restore, and body scroll locking while open.
- Treat the mobile sidebar overlay as a modal dialog context (`aria-modal`) with a clear close affordance and keyboard-inert background behavior.
- Extend viewer tests to cover focus movement and close behaviors (Escape and backdrop).

## Assumptions
- The existing sidebar toggle button remains mounted while the sidebar is open, so restoring focus to it on close is always valid.
- The sidebar contains at least one focusable control in normal operation; if none exist, focusing the sidebar heading/container is acceptable fallback behavior.
- Desktop layout can retain current non-modal behavior while mobile open state enforces modal semantics for keyboard users.

## Risks
- Incorrect focus trap logic can strand keyboard users or block close interactions.
- Scroll lock management can leak global `document.body.style.overflow` changes if cleanup is missed.
- E2E tests that depend on viewport/mobile behavior may be flaky if assertions race transitions.

## Test plan
- `cd web/viewer && npm run test -- viewerShell.test.tsx`
- `cd web/viewer && npm run test -- navbar-mobile.spec.ts`
- `./scripts/verify.sh`

## Scope (current change)
- Add roving-focus keyboard semantics to channel info tabs in `web/viewer/app/channels/[id]/page.tsx`.
- Ensure tab buttons use `tabIndex=0` only for the active tab and `tabIndex=-1` for inactive tabs while preserving existing ARIA wiring.
- Add keyboard handling for `ArrowLeft`/`ArrowRight` (plus `ArrowUp`/`ArrowDown`), `Home`, and `End` so focus movement and active panel selection stay synchronized.
- Add/update viewer tests to verify keyboard-only navigation across About, Schedule, and Videos tabs.

## Assumptions
- Horizontal tabs should wrap at ends for arrow-key navigation, matching common WAI-ARIA tabs behavior.
- Existing tabpanel IDs/labels and panel visibility behavior should remain unchanged apart from active-tab updates.
- Jest unit coverage in `web/viewer/__tests__/channelPage.test.tsx` is sufficient for this keyboard interaction update.

## Risks
- Focus-ref bookkeeping can break if tab order/index mapping drifts from the rendered tab list.
- Keyboard handlers could interfere with native button behavior if default action suppression is too broad.

## Test plan
- `cd web/viewer && npm run test -- channelPage.test.tsx`
- `./scripts/verify.sh`

## Scope (current change)
- Sync the local checkout to the merged `origin/main` state that now contains the platform-grade viewer redesign.
- Rebuild or recreate the local Docker-backed runtime from that merged source so manual QA hits the merged viewer code instead of an older container/image.
- Confirm the locally running app is reachable on the expected routes and record the local inspection entrypoints for the user.

## Assumptions
- The merged redesign is already contained in `origin/main`, and local manual inspection should happen against that merged branch rather than the older feature-branch checkout.
- A clean worktree means it is safe to switch to `main` and fast-forward it to `origin/main` without risking uncommitted local work.
- Rebuilding the local compose stack from repo root is sufficient for manual UI inspection without changing deployment-contract files.

## Risks
- Switching branches can leave the user inspecting stale containers if the rebuild step does not recreate the viewer/API services from the new source tree.
- If supporting services are stopped or unhealthy, the viewer may load with misleading empty/error states that look like UI regressions instead of local runtime issues.
- Docker/Compose on this Windows host has shown intermittent environment-specific issues before, so the deployment path needs explicit status/log verification rather than assuming success.

## Test plan
- `git fetch origin --prune`
- `git checkout main`
- `git pull --ff-only origin main`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build`
- `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 bitriver-live viewer`
- `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0 -ErrorAction Stop } catch { $_.Exception.Response }`
- `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer -MaximumRedirection 0 -ErrorAction Stop } catch { $_.Exception.Response }`

## Scope (current change)
- Turn the viewer-home shell interactions into real working entry points instead of dead-end buttons, focusing on auth entry, search/discovery continuity, and visibly actionable navigation.
- Fix the desktop shell/layout issues surfaced in manual QA: the following column should behave like a true sidebar on large screens, and the home page should stop overflowing horizontally on a 1080p desktop.
- Improve the responsive/mobile behavior for the same shell and home surfaces so the layout remains usable on smaller screens.
- Fix the broken server-rendered directory fetch on the home page so the merged viewer actually loads its discovery data in local deployment.

## Assumptions
- The intended public auth entry surface is `/signup`, which already contains the sign-in experience and conditionally reveals self-signup based on server config.
- We should not change the deployment contract or root `.env` in this pass; if public self-signup remains disabled, the UI should still route correctly to the auth surface without pretending account creation is available.
- The screenshot’s “sidebar overlap” is primarily caused by the home hero breakout/width strategy and shell breakpoint behavior, not by the following data component itself.
- Mobile friendliness here means the same routes/components should remain navigable and readable on smaller viewports without introducing a separate mobile-only product surface.

## Risks
- Changing auth-entry behavior in the navbar and following prompts can regress existing tests or flows that currently assume a `/login` fallback, so link generation needs to be centralized and covered.
- Fixing the viewer shell width and sidebar behavior touches global CSS that many pages share, so breakpoint changes must be scoped carefully to avoid new regressions in creator/channel/profile layouts.
- Patching the server-side API-base handling for the discovery surface can accidentally break client-side requests if the server/client resolution logic diverges.
- Mobile-friendly reductions in nav density can hide critical actions if drawer, auth CTA, and search access are not preserved thoughtfully.

## Test plan
- `npm.cmd --prefix web/viewer run test -- navbar.test.tsx useAuth.test.tsx followingSidebar.test.tsx viewerShell.test.tsx directoryPage.test.tsx viewer-api.test.ts`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0 -ErrorAction Stop } catch { $_.Exception.Response }`
- `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer).Content | Select-String -Pattern "Sign in|Join|Browse|Following|Failed to parse URL" -AllMatches`

## Scope (current change)
- Redesign the Next.js viewer shell into a clearer platform-style experience with stronger global navigation, consistent layout scaffolding, and repaired design-token gaps that currently leave some UI states styled inconsistently.
- Rework the main discovery journeys (`/`, `/browse`, `/following`) so each page has a distinct role, shared section patterns, and intentional empty/loading/error states without changing backend API contracts.
- Refactor the highest-friction detail and management surfaces (`/channels/[id]`, `/creator/*`, `/profile`) onto shared page patterns so browsing, viewing, onboarding, and creator workflows feel connected instead of page-by-page one-offs.
- Keep the change viewer-only unless a small UI logic fix is required to preserve current behavior during the refactor; do not change deployment-contract files.

## Assumptions
- The current viewer routes and API responses are the right functional backbone, so the redesign can focus on information architecture, interaction clarity, and layout consistency rather than introducing new backend features.
- Control-center access remains an explicit `/admin` handoff, while the viewer experience owns public browsing, watching, profile editing, and creator workflow guidance.
- Updating viewer styling, structure, and component composition does not require operator-doc updates because the deployment/runtime contract is unchanged.

## Risks
- Refactoring the shared shell/navigation can regress mobile drawer, focus-management, auth-action, or creator-entry behavior if existing state and accessibility hooks are disturbed.
- Discovery-page consolidation can blur the distinction between home and browse if section hierarchy and CTA placement are not kept purpose-specific.
- Creator/profile/channel cleanup touches many UI files at once, so inconsistent test updates or missed inline-style dependencies could leave subtle regressions behind.

## Test plan
- `npm.cmd --prefix web/viewer run test -- navbar.test.tsx viewerShell.test.tsx navigation.test.ts`
- `npm.cmd --prefix web/viewer run test -- directoryPage.test.tsx browsePage.test.tsx followingStatePresentation.test.tsx channelDisplayPrimitives.test.tsx`
- `npm.cmd --prefix web/viewer run test -- channelPage.test.tsx creatorGettingStartedPage.test.tsx creatorLivePage.test.tsx profilePage.test.tsx uploadManager.test.tsx`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run test`
- `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh --viewer`

## Scope (current change)
- Rebuild the running local BitRiver Live stack so `localhost:8080` serves the merged viewer-first routing/auth UX changes instead of the previously pulled release images.
- Reuse the existing healthy Compose deployment and rebuild only the application services that own the changed behavior (`bitriver-live` and `viewer`) unless runtime evidence shows another local service must also be restarted.
- Validate the live listener after rebuild by checking `/`, `/admin`, and `/signup`, then confirm whether the remaining signup denial is the intended `BITRIVER_LIVE_ALLOW_SELF_SIGNUP=false` behavior or a fresh runtime defect.

## Assumptions
- The current mismatch is deployment-state drift, not a new code regression: the running containers are still serving the older published UI while the merged repo contains the new routes/templates.
- Rebuilding the application services with `docker compose ... up -d --build bitriver-live viewer` is sufficient to pick up the merged server + viewer changes without disturbing the healthy vendor/stateful services more than necessary.
- The current `.env` should remain unchanged for this shakedown, so a post-rebuild signup denial is expected unless the new sign-in-first page is now correctly hiding public account creation.

## Risks
- A local rebuild can fail for host-specific Docker/build reasons even when the code is sound, so we need to separate build/runtime failures from the original UX bug.
- Restarting the API/viewer services can briefly interrupt the current local stack while the new containers come up.
- If the rebuilt API still serves the old root/signup pages, there may be image-cache or compose-targeting issues that require a narrower inspection of the running container/image IDs.

## Test plan
- `docker compose --env-file .env -f deploy/docker-compose.yml config --services`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 bitriver-live viewer`
- `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0 -ErrorAction Stop } catch { $_.Exception.Response }`
- `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/admin -MaximumRedirection 0 -ErrorAction Stop } catch { $_.Exception.Response }`
- `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/signup -MaximumRedirection 0 -ErrorAction Stop } catch { $_.Exception.Response }`

## Scope (current change)
- Run a real local shakedown deployment of BitRiver Live on this Windows host using the canonical quickstart/Compose contract (`deploy/docker-compose.yml` plus the repo-root `.env`).
- Capture the true host-side behavior during bring-up: Docker availability, compose render validity, container health, and basic service reachability after the stack starts.
- If the shakedown surfaces a repo-side blocker, apply the smallest fix that unblocks deployment validation and rerun the affected checks; otherwise, document host/operator blockers precisely without mutating the deployment contract.
- If the shakedown confirms compose hardening is blocking vendor/root-based services (for example Postgres, Redis, or the nginx transcoder-public sidecar), narrow the fix to the minimum service/capability exception set and confirm the contract change with the user before editing `deploy/docker-compose.yml`.

## Assumptions
- The current repo-root `.env` is the intended local deployment input for this checkout, so we should prefer using it as-is rather than generating or rewriting contract values unless the user asks for contract changes.
- `scripts/quickstart.ps1` remains the best host-native entrypoint on this Windows system, with direct `go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml --env-file .env` available as a fallback for comparison/debugging.
- A successful shakedown should include more than a config render: we should observe running containers and at least basic API/viewer reachability, or record the exact point where the host prevents that.

## Risks
- Docker Desktop / Engine may be installed but not running, which would stop the shakedown before container startup and needs to be separated from repo defects.
- The existing `.env` may contain site-specific or production-mode values that fail strict validation on this host; because `.env` is part of the deployment contract, we should report those gaps rather than silently rewriting them.
- A full quickstart may build/pull large images and leave containers/volumes behind, so cleanup and final state reporting need to be explicit.
- Relaxing compose hardening on vendor images could widen the runtime privilege envelope if we remove restrictions too broadly, so any contract edit should stay limited to the exact services/startup capabilities proven necessary.

## Test plan
- `docker version`
- `docker compose --env-file .env -f deploy/docker-compose.yml config`
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 --env-file .env --compose-file deploy/docker-compose.yml`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps -a`
- `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120`
- `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 postgres redis transcoder-public`
- `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/healthz -TimeoutSec 10 } catch { $_.Exception.Message }`
- `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer -TimeoutSec 10 } catch { $_.Exception.Message }`
- `./scripts/verify.sh`

## Scope (current change)
- Make the default public entry on the BitRiver Live HTTP listener viewer-first instead of auth-first, so anonymous visitors who open the host root land in the browsing/watch flow rather than the isolated signup screen.
- Move the existing control-center SPA off `/` onto an explicit `/admin` entrypoint while preserving the current control-center behavior and auth requirements once an administrator chooses that path.
- Make the auth page match the default config by treating sign-in as the primary action and only presenting self-signup affordances when `BITRIVER_LIVE_ALLOW_SELF_SIGNUP` is enabled.
- Add a simple in-product admin route affordance for signed-in admins inside the viewer UI so switching between the public viewer and control center stays easy after the root path change.
- Update the operator-facing quickstart docs so the new root-to-viewer behavior and explicit `/admin` control-center entrypoint are discoverable without reading code.

## Assumptions
- Redirecting `/` to `/viewer` only when the viewer proxy is configured is the smallest safe behavioral change for the default compose/install shape; environments without a viewer origin should continue to fall back to the existing root control-center page.
- `/admin` is the clearest explicit home for the existing control-center SPA and does not require changing the app's internal section navigation model.
- A small public auth-config endpoint is acceptable for letting the static signup page hide or reframe the signup form when self-signup is disabled.
- This change affects runtime UX and routes, but it does not change the deployment contract files themselves unless a docs update is needed to describe the new public/admin paths.

## Risks
- Root-route changes can surprise existing operators or tests that implicitly expect the control center at `/`, so the old experience needs a stable explicit replacement path and coverage.
- If `/` redirects unconditionally without a viewer proxy configured, some non-compose deployments could end up with a broken landing page.
- The auth page could briefly flash the signup card before config loads if the client-side gating is not handled carefully.
- Adding an admin entry link in the viewer navbar could create role-based UI clutter if it is shown too broadly instead of only to admins.

## Test plan
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
- `npm.cmd --prefix web/viewer run test -- navbar.test.tsx`
- `./scripts/verify.sh`

## Scope (current change)
- Upgrade `web/viewer/app/creator/live/[channelId]/page.tsx` into a creator-first guided "Go Live" flow with a clear top-to-bottom sequence: Channel, OBS Setup, Test Stream, Preview, and Share.
- Reuse existing viewer signals only (`fetchManagedChannels`, `fetchChannelPlayback` via `useCreatorChannel`, and `fetchChannelSessions`) to drive channel selection, ingest details, live-status polling, preview readiness messaging, and viewer-link sharing.
- Keep secrets hidden by default, preserve the existing creator route/layout, and extend Playwright coverage for the main setup-to-live happy path.

## Assumptions
- The current creator live route is the right surface to upgrade; the getting-started page can continue linking into it without a broader dashboard redesign.
- Existing signals are sufficient to infer the required status card states:
  - `liveState === "live"` maps to `Live`
  - `liveState === "starting"` or active ingest without playback maps to `Reconnecting`
  - `liveState === "offline"` with no active ingest maps to `Waiting for stream`
  - missing/unexpected state maps to `Offline / Unknown`
- Passing existing `live` and `liveState` props into `Player` is enough to explain "preview not ready yet" without adding new preview APIs.
- Viewer-only changes do not affect deployment-contract files or require docs updates outside the workflow artifacts unless runtime/operator guidance changes materially.

## Risks
- Status messaging can become confusing if `playback.channel.liveState`, `playback.live`, and session data drift temporarily; helper logic needs stable precedence and conservative copy.
- Polling plus manual refresh can cause noisy UI state if timestamps/copy messages are reset too aggressively on every cycle.
- Copy affordances for stream key, ingest URL, OBS settings, and viewer link can become brittle in tests if labels/test IDs are not stable.
- Reordering the page into a strict guided flow could accidentally bury existing capabilities like title editing or channel switching if they are not integrated carefully into the new sections.

## Test plan
- `cd web/viewer && npm run test -- creatorLiveStreamStatus.test.ts`
- `cd web/viewer && npm run test -- creatorLivePage.test.tsx`
- `cd web/viewer && npm run test:playwright -- tests/creator-live-setup.spec.ts`
- `./scripts/verify.sh --viewer`

## Scope (current change)
- Fix creator-facing viewer/share links so they include the active Next.js `basePath` instead of always assuming the viewer is mounted at `/`.
- Apply the fix to the new creator live Share step and the older creator getting-started viewer-link copy flow so viewer URLs stay consistent across creator entrypoints.
- Keep the change client-side and reuse current runtime context rather than adding new APIs or changing deployment-contract files.

## Assumptions
- The active viewer base path can be derived safely on the client from the current browser pathname because both affected surfaces are client components rendered under the viewer app.
- The standard deployment shape continues to serve the viewer from the same origin as the current page, so we only need to prepend the correct base path before building absolute URLs.
- Updating share/copy URL generation in viewer code does not require contract docs changes because the deployment contract itself is unchanged.

## Risks
- Base-path inference can over-trim or double-prefix URLs if path normalization is sloppy, especially when the viewer is mounted at `/`.
- Fixing only the new live page would leave the older getting-started viewer-link copy flow inconsistent and still broken under `/viewer`.
- URL-construction tests can be brittle if they rely on the default jsdom pathname instead of explicitly setting the mounted viewer path.

## Test plan
- `cd web/viewer && npm run test -- creatorLivePage.test.tsx`
- `cd web/viewer && npm run test -- creatorGettingStartedPage.test.tsx`
- `cd web/viewer && npm run build`
- `./scripts/verify.sh --viewer`

## Scope (current change)
- Enhance `web/viewer/app/channels/[id]/page.tsx` tab state management so the selected channel tab is encoded in URL state and deep-linkable.
- Read initial tab selection from URL (`?tab=` first, hash fallback), validate against `CHANNEL_TABS`, and default to `about` when invalid/missing.
- Keep tab selection and URL synchronized via router/history updates while preserving current ARIA tablist + keyboard roving behavior.
- Extend `web/viewer/__tests__/channelPage.test.tsx` for deep-link initialization, URL updates on tab switch, and browser back/forward restoration.

## Assumptions
- Query-param tab encoding (`?tab=<id>`) is preferred for deterministic router integration; hash can remain read-compatible for backward links.
- Viewer page remains a client component and can safely read `window.location` and use Next.js `useRouter`/`useSearchParams` APIs in effects.
- Viewer-only change does not alter deployment contract files.

## Risks
- URL/state sync can create update loops if route-change listeners and state setters are not guarded against no-op transitions.
- Test reliability can regress if mocked router/history behavior diverges from browser popstate semantics.

## Test plan
- `cd web/viewer && npm run test -- channelPage.test.tsx`
- `./scripts/verify.sh`

## Scope (current change)
- Run a focused viewer usability shakedown against the current BitRiver Live interface so the primary product journeys feel sensible and actionable instead of looking polished-but-brittle.
- Validate the highest-friction flows first: anonymous landing/discovery, auth entry, following recovery states, and shell/layout behavior across desktop and mobile breakpoints.
- Build on the existing uncommitted viewer work already present in this checkout; keep the pass narrowly scoped to viewer UX/usability fixes unless runtime smoke proves a small supporting API/viewer integration fix is required.
- Avoid deployment-contract edits in this pass unless a true usability blocker is impossible to resolve without them.

## Assumptions
- The most important “actually usable” journeys for this pass are: open the product, understand where to go, browse or search channels, reach the real auth surface, and recover cleanly from empty/error/following states.
- Existing viewer Jest/Playwright coverage plus local HTTP smoke checks are enough to identify the top remaining usability regressions without inventing new product requirements.
- The uncommitted viewer changes already in this worktree are intentional in-flight fixes, so the safest path is to extend and verify them rather than trying to reset to a cleaner baseline.

## Risks
- Shared shell/CSS changes can improve one route while subtly regressing another, so touched navigation and layout code needs targeted regression coverage.
- Local runtime/config issues can masquerade as UI breakage during smoke checks, especially around auth availability and directory data loading, so findings need careful attribution.
- Because `PLAN.md`, `TASKS.md`, and several viewer files are already dirty, any edits need to stay additive and narrowly scoped to avoid stomping collaborator work.

## Test plan
- `npm.cmd --prefix web/viewer run test -- navbar.test.tsx useAuth.test.tsx followingSidebar.test.tsx viewerShell.test.tsx directoryPage.test.tsx viewer-api.test.ts`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `npm.cmd --prefix web/viewer run test:playwright -- accessibility.spec.ts navbar-mobile.spec.ts`
- `Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0`
- `Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer`
- `Invoke-WebRequest -UseBasicParsing http://localhost:8080/signup`

## Scope (current change)
- Ensure the locally running BitRiver Live instance is rebuilt from the current checkout so the app on `localhost:8080` reflects the latest source instead of any older container build.
- Keep this refresh narrowly scoped to the application services that present the current product changes: `bitriver-live` and `viewer`.
- Verify the refreshed runtime with Compose status plus public route checks after the rebuild.

## Assumptions
- The current clean checkout is the desired source of truth for the local runtime.
- Rebuilding `bitriver-live` and `viewer` is sufficient to pick up the latest application changes without restarting unrelated healthy stateful/vendor services.
- Successful route checks after the rebuild are enough to confirm the local instance reflects the latest code in this checkout.

## Risks
- Docker layer caching can make a rebuild look successful while still leaving uncertainty, so route-level verification is required after recreation.
- Recreating the API/viewer services will briefly interrupt the local app while the containers restart.
- Host/runtime issues could block startup even when the source tree is healthy, so service status and logs need to be captured if anything fails.

## Test plan
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 bitriver-live viewer`
- `Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0`
- `Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer`
- `Invoke-WebRequest -UseBasicParsing http://localhost:8080/signup`

## Scope (current change)
- Rescue the public viewer homepage so the current `/` experience reads as a coherent streaming-platform entry point instead of several unrelated sections competing for attention.
- Keep the existing product direction intact: dark premium viewer UI, desktop-first responsiveness, existing routes/data wiring, and the same discovery/following/channel primitives where they still fit.
- Focus the implementation on the highest-impact UX surfaces visible on the current page: navbar prioritization, shell/sidebar alignment, homepage hero hierarchy, section rhythm, card consistency, and shared loading/empty states.
- Keep this pass viewer-only unless a tiny supporting test or presentation-logic adjustment is required to preserve current behavior.

## Assumptions
- The user is asking for a targeted rescue pass on the current homepage experience, not a broad rebrand or a backend/product-scope expansion.
- The current discovery data contracts are sufficient; the main problem is information architecture, spacing, and inconsistent visual systems rather than missing content.
- Reusing the existing homepage/discovery components is preferable to a wholesale rewrite as long as we standardize their hierarchy and styling around one clear layout system.
- Viewer-only layout/style improvements do not require deployment-contract or operator-doc updates because runtime behavior and routes remain unchanged.

## Risks
- `globals.css` already contains duplicated navbar/shell/token sections, so touching shared styles can accidentally improve the homepage while regressing another viewer surface if overrides are not consolidated carefully.
- Simplifying the header too aggressively could hide important creator/admin or auth entry points, so role-aware actions still need a clear home after the cleanup.
- Reworking the homepage hierarchy may shift DOM structure enough to break viewer tests that currently target old labels/sections, so test updates need to stay aligned with the new information architecture.
- Because `PLAN.md`, `TASKS.md`, and existing viewer files are already dirty in this checkout, edits need to remain additive and avoid disturbing unrelated work.

## Test plan
- `npm.cmd --prefix web/viewer run test -- navbar.test.tsx viewerShell.test.tsx directoryPage.test.tsx channelDisplayPrimitives.test.tsx`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `./scripts/verify.sh --viewer`

## Scope (current change)
- Redeploy the locally running BitRiver Live app so `localhost:8080` serves the latest homepage rescue code from the current checkout instead of any older container build.
- Keep the runtime action narrowly scoped to the application services that own the changed viewer experience: `bitriver-live` and `viewer`.
- Confirm the redeployed routes are reachable and reflect the updated viewer shell/homepage so the user can inspect the changes locally right away.

## Assumptions
- The current dirty checkout is the intended source of truth and should be what gets rebuilt into the local runtime.
- Rebuilding `bitriver-live` and `viewer` is sufficient to surface the homepage/UI rescue changes without restarting unrelated healthy stateful services.
- Route checks against `/`, `/viewer`, and optionally `/signup` are enough to confirm the redeploy succeeded for local visual QA.

## Risks
- Docker layer caching can make a rebuild appear successful while still leaving stale behavior in the running app, so explicit route verification is required after recreation.
- Recreating the app services will briefly interrupt the current local instance while the new containers start.
- Host-side Docker or shell issues may block the redeploy even when the source tree is healthy, so service status/logs need to be captured if anything fails.

## Test plan
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 bitriver-live viewer`
- `Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0`
- `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer).Content | Select-String -Pattern "Find the streams worth opening now|Watch live now|Full directory" -AllMatches`
- `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/signup).Content | Select-String -Pattern "Sign in to continue|Back to viewer" -AllMatches`

## Scope (current change)
- Audit the current viewer UI with actual interaction checks so the most visible dead or misleading controls are fixed instead of only restyled.
- Reproduce the reported navbar theme-toggle failure and identify other broken high-traffic controls on the current discovery shell and homepage before editing implementation code.
- Prioritize fixes that make the shipped viewer materially usable right now: controls should either perform a real action, navigate somewhere meaningful, or be removed/reframed so they are no longer dead ends.
- Keep the scope viewer-only unless a tiny supporting test/helper change is required to validate the repaired controls.

## Assumptions
- The user's report is grounded in the current runtime, so even controls with existing unit coverage still need browser-level validation.
- The highest-impact broken controls are likely in the signed-out discovery experience (`Navbar`, homepage hero, category/discovery controls, mobile shell) because that is the first surface users hit.
- Existing viewer tests are not yet sufficient to prove usability; we need a combination of static button audit, targeted Jest coverage, and at least one browser-level pass where the host allows it.
- Viewer-only control/UX fixes do not require deployment-contract changes as long as routes, env contract, and backend APIs stay the same.

## Risks
- A broad "fix all buttons" pass could sprawl into unrelated redesign work, so the task list needs to stay tied to reproduced breakage on high-traffic controls.
- Some controls may appear broken because their visual feedback is too subtle rather than because state never changes; we need to distinguish no-op behavior from weak affordance.
- Browser automation may still hit the existing Windows Playwright/permission friction on this host, so validation needs a fallback path rather than assuming the first runner will work.
- `PLAN.md`, `TASKS.md`, and multiple viewer files are already dirty in this checkout, so edits need to stay additive and avoid trampling unrelated in-progress work.

## Test plan
- `Get-Content web/viewer/components/Navbar.tsx`
- `Get-Content web/viewer/components/CategoryRail.tsx`
- `Get-Content web/viewer/app/directory-view.tsx`
- `npm.cmd --prefix web/viewer run test -- navbar.test.tsx directoryPage.test.tsx`
- `npm.cmd --prefix web/viewer run build`
- `npm.cmd --prefix web/viewer run test:playwright -- tests/channel.spec.ts tests/navbar-mobile.spec.ts`
- If Playwright remains blocked: `npx.cmd playwright test tests/channel.spec.ts tests/navbar-mobile.spec.ts --reporter=list`

## Scope (current change)
- Rebuild and redeploy the locally running `bitriver-live` and `viewer` services from the current branch so the user can review the latest viewer-control fixes in the real app.
- Verify the specific acceptance check the user named: the light/dark mode toggle must work on the deployed app at `localhost:8080`, not only in the isolated viewer test server.
- Keep this as a runtime refresh and validation pass only; do not broaden into additional product changes unless the deployment/verification uncovers a new blocker in the current branch.

## Assumptions
- The current checked-out branch and worktree contents are the intended source of truth for the review deployment.
- Rebuilding `bitriver-live` and `viewer` is sufficient for this review pass because the recent changes are viewer-focused and do not require stateful service contract changes.
- A focused browser check against the deployed app is the most meaningful proof for the user's review gate because the theme toggle is the named manual test.

## Risks
- Docker may reuse stale layers or leave an older container running unless we confirm service recreation and route-level behavior after the rebuild.
- The deployed app may differ slightly from the local Playwright/Jest harness, so the light/dark toggle needs an explicit deployed-instance verification rather than inference from prior test runs.
- Host-specific issues such as Docker pipe access, Playwright browser launch quirks, or route timing could block the verification even if the current branch is healthy, so results need to distinguish repo issues from host issues.

## Test plan
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `npx.cmd playwright test tests/channel.spec.ts --grep "theme toggle updates the rendered document" --reporter=list` with `PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080`
- `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer).Content | Select-String -Pattern "Switch to (light|dark) theme|BitRiver Live" -AllMatches`
- `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode,Headers } catch { $_.Exception.Response | Select-Object StatusCode,Headers }`

## Scope (current change)
- Fix the deployed viewer hydration blocker uncovered during review deployment: the Go server's default CSP currently blocks the proxied Next.js viewer boot scripts on `/viewer`, leaving interactive controls inert on the live app.
- Keep the fix narrowly scoped to security-header behavior for the proxied viewer path, preserving the stricter existing defaults for admin/API routes unless the viewer runtime specifically requires a relaxation.
- Rebuild the local review deployment after the change and re-run a real browser check against `http://localhost:8080` so the light/dark toggle is proven working on the deployed app, not only in isolated viewer tests.
- Update the operator-facing security-header docs so the new default viewer-path behavior is documented alongside the existing override flags and environment variables.

## Assumptions
- The deployed theme-toggle failure is caused by the confirmed CSP console errors blocking Next.js hydration, not by additional bugs in `Navbar.tsx`.
- Allowing the proxied viewer route to execute the inline bootstrap scripts required by Next.js is the minimal runtime fix needed to restore interactivity.
- Existing security-header override flags/env vars should keep their current meaning; only the default behavior for proxied viewer responses needs adjustment.

## Risks
- Relaxing CSP too broadly could weaken protections for admin or API surfaces that do not need inline scripts, so the change should stay path-aware and as narrow as possible.
- Viewer proxy responses and locally served admin pages currently share middleware, so the implementation must avoid accidentally changing non-viewer routes.
- Runtime verification may still hit host-specific Docker or Playwright friction, so the execution log needs to distinguish repo fixes from host blockers clearly.

## Test plan
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `@'...playwright probe...'@ | node -` against `http://127.0.0.1:8080/`
- `npx.cmd playwright test tests/channel.spec.ts --grep "theme toggle updates the rendered document" --reporter=list` with `PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080`

## Scope (current change)
- Fix the misleading homepage category-chip browse links so they land on a real exact category filter instead of overloading the free-text `q` search parameter.
- Add category-aware directory handling across the viewer and API so `/browse?category=...` loads exact category results, while existing free-text search keeps working and also starts matching `channel.category` to align with current UI copy.
- Keep the change narrowly scoped to directory browsing behavior, category-link wiring, and the minimum test coverage needed to prove both the API and viewer paths.

## Assumptions
- The user is correct that `?q=<category>` is currently semantically wrong for category chips because the existing backend search is not an exact category filter.
- Introducing a dedicated `category` query parameter is the clearest user-facing fix for homepage category chips, while also extending free-text search to include `channel.category` keeps the rest of the browse/search copy honest.
- Directory result sets are still small enough that an exact category filter can be applied in the API layer without introducing pagination or contract changes elsewhere.

## Risks
- Query-param synchronization on the browse page can become confusing if `q` and `category` are not normalized and preserved consistently through search/reset flows.
- Changing viewer API helper signatures could ripple through existing tests if the update is not kept backward-compatible.
- Broadening free-text search to include category could subtly change result sets for existing browse queries, so API coverage needs to assert the intended matching behavior explicitly.

## Test plan
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/api -count=1 -timeout=120s`
- `npm.cmd --prefix web/viewer run test -- directoryPage.test.tsx browsePage.test.tsx viewer-api.test.ts`
- `npm.cmd --prefix web/viewer run build`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --force-recreate --no-deps bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`

## Scope (current change)
- Revisit the initial admin-access UX so operators can reliably find the bootstrap control-center credentials after install/quickstart, especially when public self-signup is disabled by default.
- Add a small operator-facing CLI recovery path that reads the deployment `.env` and prints the admin access summary on demand instead of relying on a one-time terminal flash during bootstrap.
- Update quickstart/install/auth messaging so the operator sees where the bootstrap creds live, where to sign in (`/admin`), and what caveat applies if the password was rotated later.

## Assumptions
- The current install/quickstart flows already persist the bootstrap admin credentials in the deployment `.env`, but that storage location is not obvious enough to operators after the initial run.
- A targeted CLI helper plus clearer summaries is safer than introducing a second secret store or exposing recovery internals too broadly in the public UI.
- The public auth page can safely include a concise operator hint that points to `/admin` and the deployment environment file without exposing any actual secrets.

## Risks
- Printing bootstrap password values too casually could leak secrets into shell history or shared terminals, so any recovery command should default to redacting the password unless the operator opts in.
- Operators who already rotated the admin password in the control center could misread the env-backed bootstrap password as the current live credential, so the output must warn about that distinction.
- Updating the public signup copy too aggressively could make the page feel operator-centric for normal viewers, so the added guidance should stay short and secondary.

## Test plan
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./cmd/bitriver -count=1 -timeout=120s`
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
- `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh`

## Scope (current change)
- Make the `/signup` auth surface feel like a continuation of the `/viewer` experience instead of a disconnected static utility page.
- Keep the existing API/auth flow intact while tightening the visual language, return-path context, and navigation cues around sign-in/sign-up.
- Refresh the local review deployment after the change so the updated auth surface is available on the running app for direct UX review.

## Assumptions
- The main problem is cohesion and continuity, not the underlying login/signup mechanics; the existing auth handlers and redirects can stay intact.
- The fastest high-impact improvement is to restyle and restructure the static auth page to mirror the current viewer product language, then thread the existing `next` destination more visibly through the page.
- A small doc update in the viewer README is appropriate because the auth landing behavior is part of the viewer navigation contract even though the page is served by the Go binary.

## Risks
- The static auth page lives outside the Next.js viewer app, so copying too much viewer styling directly could create a brittle parallel design system unless the scope stays focused on the highest-value shared cues.
- Dynamic “return to where you left off” messaging can become misleading if the `next` parameter is not sanitized or displayed carefully.
- Route-level UX validation requires rebuilding the running `bitriver-live` service after the embed changes, so host Docker issues could block final live review even if the code/tests pass.

## Test plan
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `@'...playwright auth probe...'@ | node -` against `http://127.0.0.1:8080/signup?next=%2Fviewer%2Fbrowse%3Fq%3Dmusic`

## Scope (current change)
- Rework the `/viewer` homepage so the above-the-fold experience is video-centric and creator-centric instead of leading with long-form discovery copy.
- Keep the existing discovery data sources, routes, and viewer shell, but shift the hierarchy toward featured/live previews, creator cards, and faster visual entry into content.
- Limit the scope to the signed-out/signed-in homepage experience plus the minimum supporting tests and runtime refresh needed for local review.

## Assumptions
- The gap the user is calling out is primarily information architecture and visual emphasis, not missing backend data or a need for autoplay video playback.
- Existing components such as `FeaturedChannel`, `LiveNowGrid`, `ChannelRail`, and `CategoryRail` already provide enough content primitives to build a stronger video-first homepage without changing APIs.
- Viewer-only composition/styling changes do not require deployment-contract or operator-doc updates as long as routes and backend behavior stay the same.

## Risks
- Moving too much content above the fold can make the homepage feel busier rather than more engaging, so the new hierarchy needs to stay decisive and avoid turning every section into a hero.
- Rebalancing the viewer shell around larger media modules could regress mobile layout or the current following-sidebar affordances if shared styles are changed too broadly.
- Updating homepage copy, headings, and section order will likely require aligned Jest assertions and a live redeploy check so we do not ship a visually improved page with stale tests or stale containers.

## Test plan
- `npm.cmd --prefix web/viewer run test -- directoryPage.test.tsx viewerShell.test.tsx`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`

## Scope (current change)
- Rework the `/signup` auth presentation so it feels like a centered overlay/modal on top of the viewer instead of a separate full-page destination.
- Keep the existing auth API flow, `next`-based return behavior, MFA/signup handling, and static-route serving intact while shifting the information architecture and styling toward a Twitch-like overlay pattern.
- Refresh the local app after the change so the updated auth route is reviewable on `localhost:8080` in the real running install.

## Assumptions
- The user is asking for a presentation and interaction-framing change, not for a new SPA modal mounted inside the Next.js viewer runtime; a static auth page that visually behaves like an overlay is the right scope.
- Existing auth JS already has the needed return-path behavior, so the main work is collapsing the page into a modal card and demoting the surrounding explanatory content into backdrop/context treatment.
- This route-level UX change does not require deployment-contract updates as long as auth endpoints, redirects, and operator hints remain unchanged.

## Risks
- If the fake viewer backdrop is too decorative or too interactive-looking, users may think the background is usable while the auth card is open, so the backdrop needs to read clearly as dimmed context.
- Compressing too much auth/help context into a modal-sized surface could hurt clarity around self-signup-disabled and MFA states if spacing or hierarchy gets too tight.
- Because the route is static HTML/CSS/JS served by the Go binary, server tests that assert specific signup copy/scaffold will need coordinated updates when the modal contract changes.

## Test plan
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `@'...auth route probe...'@ | node -` against `http://127.0.0.1:8080/signup?next=%2Fviewer`

## Scope (current change)
- Replace the viewer's auth redirects with a real in-app overlay mounted inside the Next.js `/viewer` experience for sign-in, sign-up, and MFA continuation.
- Add a small viewer-auth API contract so the viewer can learn session state plus auth affordances (`allowSelfSignup`, logout path, and similar) without relying on the old static `/signup` page bootstrap.
- Demote `/signup` from the primary UX to a compatibility path that forwards into the viewer overlay when the viewer is configured, while preserving a safe fallback for non-viewer deployments.

## Assumptions
- The user's main complaint is architectural, not just visual: auth should happen inside the viewer shell, so the right fix is to move the form flow into the viewer runtime instead of polishing the static route again.
- `useAuth` is already the natural place to centralize session loading plus modal open/close state, and the global viewer layout can host the overlay once that hook exposes the needed controls.
- Keeping `/signup` as a compatibility redirect when `ViewerOrigin` is configured is safer than deleting the route outright because existing OAuth/MFA/legacy links may still target it.
- A viewer-specific `/api/viewer/me` response is the cleanest minimal contract because the viewer already expects that path and it can return guest-safe auth config with or without a live session.

## Risks
- Expanding `useAuth` from "session lookup" into full auth-flow state management could ripple through many signed-out CTAs and existing tests if the API is not kept tidy.
- Redirect/refresh behavior after successful auth can feel jarring if the modal does not preserve the intended `next` route or if it reloads more of the viewer than necessary.
- MFA and self-signup-disabled states currently live in the static route script, so porting them into React carries a real risk of regressions unless we keep the same API semantics and cover them explicitly.
- Converting `/signup` into a compatibility redirect must not break installs that do not proxy the Next.js viewer; the server needs to branch cleanly on whether `ViewerOrigin` is configured.

## Test plan
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
- `npm.cmd --prefix web/viewer run test -- useAuth.test.tsx navbar.test.tsx followingStatePresentation.test.tsx`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `@'...viewer auth overlay probe...'@ | node -` against `http://127.0.0.1:8080/viewer?auth=signup`

## Scope (current change)
- Tighten the `/viewer` desktop shell so the Following rail and Featured Live hero share a cleaner top alignment and the sidebar no longer burns vertical space on explanatory copy.
- Keep the implementation narrowly scoped to viewer shell/homepage composition, focused tests, and a local redeploy for review.
- Evaluate renaming the Docker Compose project from the default `deploy` label to a clearer stack name, but treat that as a deployment-contract change that requires explicit user approval before implementation.

## Assumptions
- The visible mismatch in the screenshot is being driven by the desktop shell/sidebar framing, especially the extra intro copy above `FollowingSidebar`, rather than by missing homepage data.
- A more compact signed-out guidance treatment inside the following rail can preserve the onboarding message without pushing the sidebar out of rhythm with the hero.
- Docker Desktop is showing `deploy` because Compose is inheriting its default project name from the compose-file directory; we should not change that contract until the user opts in.

## Risks
- Shared shell CSS changes could accidentally regress mobile sidebar behavior or other viewer pages if they are broader than necessary.
- Compressing the following rail too aggressively could make signed-out guidance less clear if the CTA and explanation are not still obvious.
- Renaming the Compose project will rename Docker resources and can surprise operators looking for the old stack name, so it must stay approval-gated.

## Test plan
- `Get-Content web/viewer/components/ViewerShell.tsx | Select-Object -Skip 130 -First 90`
- `Get-Content web/viewer/styles/globals.css | Select-Object -Skip 4260 -First 150`
- `Get-Content web/viewer/styles/home.css | Select-Object -Skip 970 -First 280`
- `npm.cmd --prefix web/viewer run test -- directoryPage.test.tsx viewerShell.test.tsx followingStatePresentation.test.tsx`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`

## Scope (current change)
- Audit the GitHub Actions workflows defined under `.github/workflows` and run the closest feasible local equivalents from this Windows host.
- Use the workflow definitions themselves as the source of truth for which checks matter to cross-platform confidence for a self-hosted streaming deployment.
- Treat this as a verification/reporting pass only unless a currently reproducible repository failure demands a narrowly scoped fix.

## Assumptions
- We cannot literally reproduce GitHub-hosted `ubuntu-latest` and `macos-latest` runner environments from this one Windows workstation, but we can still execute the workflow entrypoints and scripts that are platform-agnostic or Windows-compatible.
- The most meaningful local evidence will come from the canonical gates and workflow scripts: `scripts/verify.sh`, `scripts/test-all.sh`, `scripts/test-postgres.sh`, `scripts/test-quickstart.sh`, viewer CI commands, and workflow consistency/doc checks.
- Some workflow steps may remain blocked by host-tool differences already known in this environment, especially `python3` inside Bash-based scripts and Linux-only tools such as `shellcheck`.

## Risks
- Running broad validation on a dirty worktree can mix current uncommitted viewer changes into the results, so the report needs to distinguish “current checkout passes locally” from “clean mainline CI status”.
- Several workflows are Linux-specific or depend on GitHub-hosted service containers/actions, so local results alone cannot prove full cross-platform coverage for Ubuntu and macOS.
- Long-running Docker and Playwright checks can consume time and resources; if one host prerequisite is missing, we should record the blocker precisely instead of forcing partial or misleading results.

## Test plan
- `Get-Content .github/workflows/ci.yml`
- `Get-Content .github/workflows/go-unit-tests.yml`
- `Get-Content .github/workflows/viewer-ci.yml`
- `Get-Content .github/workflows/quickstart-smoke.yml`
- `Get-Content .github/workflows/postgres-tests.yml`
- `Get-Command bash, python3, py, go, docker, node, npm, psql, shellcheck, gh, act -ErrorAction SilentlyContinue | Select-Object Name,Source`
- `pwsh -File scripts/quickstart.ps1 -help`
- `pwsh -File scripts/quickstart.ps1 -ValidateOnly`
- `go test ./... -count=1 -timeout=120s`
- `npm.cmd --prefix web/viewer run test:integration`
- `npm.cmd --prefix web/viewer run build`
- `docker compose --env-file .env -f deploy/docker-compose.yml config`
- `./scripts/test-postgres.sh`
- `./scripts/test-quickstart.sh`
- `./scripts/check-go-workflow-config.sh`
- `./scripts/check-doc-installer-language.sh`
- `./scripts/generate-contract-doc.sh --check`
- `./scripts/check-monitoring-config.sh`

## Scope (current change)
- Close the Windows portability gaps uncovered by the CI verification pass so the repo's defined cross-platform Go and quickstart checks can run meaningfully on a self-hosted Windows workstation.
- Keep runtime behavior unchanged; the fix should stay inside test harnesses unless a production path is truly required.
- Continue to use the existing GitHub Actions workflows as the validation target, and dispatch safe hosted workflows only after the local Windows failures are addressed.

## Assumptions
- The failing `cmd/transcoder` tests are partly caused by the test helper assuming a Unix-style `ffmpeg` stub is executable on Windows, but the direct symlink probe shows there is also a real Windows runtime incompatibility in the transcoder's live publish path.
- The failing `scripts` tests are caused by choosing an unusable `bash` binary (`C:\\WINDOWS\\system32\\bash.exe`) and by Unix-only PATH assumptions inside the test harness, not by a broken quickstart wrapper.
- GitHub-hosted Ubuntu/macOS workflows remain the source of truth for non-Windows coverage, but getting the Windows-local equivalents green is necessary before dispatching them with confidence.

## Risks
- A too-clever Windows stub could diverge from the existing Unix `ffmpeg` stub semantics and mask real transcoder regressions if it does not produce the same playlist and segment shape.
- Shell-resolution changes in the quickstart tests could accidentally weaken the assertions if they silently skip real wrapper execution instead of selecting a usable shell.
- Changing the live publish path from "symlink only" to a Windows-capable fallback must preserve the current Unix behavior and still keep live manifests visible to the viewer while a stream is active.
- Triggering hosted workflows before the local Windows failures are corrected would create noisy, avoidable red CI and make the cross-platform report less credible.

## Test plan
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./cmd/transcoder -count=1 -timeout=120s`
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./scripts -count=1 -timeout=120s`
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./... -count=1 -timeout=120s`
- `powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 -help`
- `powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 -ValidateOnly`
- `$tmp = Join-Path (Resolve-Path .).Path '.codex-symlink-check'; if (Test-Path $tmp) { Remove-Item -Recurse -Force $tmp }; New-Item -ItemType Directory -Path $tmp | Out-Null; New-Item -ItemType Directory -Path (Join-Path $tmp 'target') | Out-Null; try { New-Item -ItemType SymbolicLink -Path (Join-Path $tmp 'link') -Target (Join-Path $tmp 'target') -ErrorAction Stop | Out-Null; 'symlink-ok' } catch { 'symlink-failed: ' + $_.Exception.Message }`
- `docker compose --env-file .env -f deploy/docker-compose.yml config`
- `gh workflow run "CI" --ref codex/UI_UX_repair`
- `gh workflow run "Quickstart compose smoke" --ref codex/UI_UX_repair`

## Scope (current change)
- Remove the redundant destination-explanation sentence from the in-viewer auth overlay's route context card in `web/viewer/components/auth/AuthDialog.tsx`.
- Keep the compact "Continue where you left off" header plus destination path visible so users still understand where auth returns them without restating automatic behavior.
- Add focused viewer coverage for the trimmed auth-overlay copy contract.

## Assumptions
- The user wants the explanatory route paragraph removed from the auth overlay generally, not just for the `/viewer` root case, because the auto-return behavior is already implicit in the flow.
- Existing auth redirect, MFA, and self-signup behavior should remain unchanged; this is a copy and hierarchy cleanup only.
- This viewer-only adjustment does not change deployment contracts or require operator-doc updates.

## Risks
- Removing the sentence could make the route card feel too sparse if the destination path is not still visually obvious.
- A narrowly scoped copy test can become brittle if it asserts too much surrounding markup instead of the user-visible contract.

## Test plan
- `Get-Content web/viewer/components/auth/AuthDialog.tsx`
- `npm.cmd --prefix web/viewer run test -- authDialog.test.tsx`

## Scope (current change)
- Redeploy the local app stack so `http://localhost:8080` reflects the latest checkout, including the most recent viewer/auth updates.
- Use the canonical Compose contract (`.env` plus `deploy/docker-compose.yml`) without changing deployment settings.
- Confirm the live HTTP routes after the rebuild/recreate instead of assuming local source edits were already picked up.

## Assumptions
- The required root `.env` already exists and is valid for the local review stack.
- Rebuilding and recreating `bitriver-live` and `viewer` should be sufficient to surface the latest app changes on `localhost:8080`.
- A successful route check on `/`, `/viewer`, and the current auth entry path is enough to confirm the redeploy worked for this request.

## Risks
- Docker build/recreate commands can time out or hit local daemon/config permission issues on this Windows host.
- If the running app depends on stale supporting services or cached assets, a targeted app-service rebuild may not be enough and we may need to record that blocker.
- HTTP verification can pass while still missing the intended UI update if we probe the wrong route, so the post-redeploy check needs to target the current viewer/auth surface.

## Test plan
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode,Headers } catch { $_.Exception.Response | Select-Object StatusCode,Headers }`
- `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer).Content | Select-String -Pattern 'Browse BitRiver Live|Watch live now|Full directory' -AllMatches`
- `(Invoke-WebRequest -UseBasicParsing 'http://localhost:8080/viewer?auth=signup').Content | Select-String -Pattern 'Continue where you left off|Create your BitRiver account|/viewer' -AllMatches`

## Scope (current change)
- Audit the repository from a first-time open-source adopter's perspective and tighten the public-facing structure before a credible tagged release.
- Focus on low-risk, high-signal improvements: repo hygiene, onboarding clarity, community/security metadata, release docs, and small docs/config fixes that improve trust.
- Avoid speculative product rewrites; only make code/config changes when they clearly support safer setup or better release ergonomics.

## Assumptions
- The existing deployment/docs direction is broadly correct; the main gap is public presentation, consistency, and release readiness rather than missing core functionality.
- Durable release/runbook docs under `docs/` should stay, but ad-hoc internal artifacts (temporary files, dated evidence logs, speculative release notes) can be removed or replaced with cleaner public equivalents.
- Public contributors will expect standard repository signals (`CONTRIBUTING.md`, `SECURITY.md`, issue templates, changelog, release checklist) at the repo root or in `.github/`.

## Risks
- Rewriting the README too aggressively could drift from the actual deployment contract or overstate current support; copy changes need to stay precise and source-backed.
- Removing tracked artifacts must not break any scripts or docs that still assume those files exist, so references need to be updated in the same pass.
- Public release docs need to align with the repo's existing semver/release workflow; introducing a contradictory version story would create more confusion than it solves.

## Test plan
- `git ls-files artifacts .tmp docs/releases`
- `./scripts/check-no-committed-secrets.sh`
- `./scripts/check-doc-installer-language.sh`
- `./scripts/generate-contract-doc.sh --check`
- `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh`

## Scope (current change)
- Do a second public-release polish pass focused only on outsider trust and adoption signals, not on product behavior or new features.
- Tighten the first 30 seconds of the README, clarify install/onboarding surfaces, and remove stale or internally scoped wording from public docs and packaging metadata.
- Add or refine low-risk legitimacy/support surfaces where they materially improve newcomer confidence.

## Assumptions
- The main remaining gaps are presentation and consistency issues rather than missing runtime capability.
- Public adopters benefit more from a smaller number of crisp, trustworthy docs than from adding more sprawling internal-detail pages.
- The current repository remote (`github.com/ProhibitedTV/BitRiver-Live`) is the right public source of truth for repository URLs shown in docs and package metadata.

## Risks
- Tightening quickstart/install docs too aggressively could accidentally hide important constraints that operators still need before production use.
- Updating public repo URLs and support paths must stay aligned across docs, issue templates, and packaging metadata so the repo does not feel split-brained.
- Release-stage wording needs to stay honest about the supported single-host baseline without creating a contradictory version story.

## Test plan
- `Get-Content README.md -TotalCount 160`
- `Get-Content docs/quickstart.md -TotalCount 160`
- `Get-Content CONTRIBUTING.md`
- `Get-Content SUPPORT.md`
- `Get-Content .github/ISSUE_TEMPLATE/config.yml`
- `Get-Content docs/production-status.md`
- `rg -n "github.com/bitriver-live/bitriver-live|docs/cross-platform-plan.md|v1.0|SUPPORT.md" README.md docs CONTRIBUTING.md .github deploy scripts`
- `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/check-doc-installer-language.sh`
- `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/check-no-committed-secrets.sh`

## Scope (current change)
- Fast-forward the local checkout to the merged `origin/main` commit and redeploy the app services so `localhost:8080` reflects the latest merged work.
- Polish the release-facing viewer UI/UX with one goal in mind: help average people understand and launch a self-hosted Twitch-like experience without extra controls, dense copy, or maintainer-centric phrasing.
- Focus on high-traffic/product surfaces first: viewer homepage, primary navigation/shell, and creator onboarding/live setup flows. Keep the deployment contract unchanged.

## Assumptions
- The merged public-release PR is already on `origin/main` and the current local feature branch can fast-forward cleanly to it.
- The most valuable UX improvements are subtractive: fewer controls, shorter explanations, clearer primary actions, and a more obvious "watch or go live" path.
- The core product behavior already works; this pass should simplify presentation and task flow rather than add new features.

## Risks
- Simplifying creator flows too aggressively could hide information people still need when copying ingest settings or validating a stream.
- UI copy and layout changes can easily break existing viewer tests if text contracts or element hierarchy move more than intended.
- Redeploying before aligning the local checkout to `origin/main` would risk rebuilding stale code and create confusion about what is actually live on `localhost:8080`.

## Test plan
- `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live rev-list --left-right --count HEAD...origin/main`
- `git pull --ff-only origin main`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `npm.cmd --prefix web/viewer run test -- authDialog.test.tsx navigation.test.ts viewerShell.test.tsx creatorGettingStartedPage.test.tsx creatorLivePage.test.tsx`
- `npx.cmd playwright test tests/homepage-layout.spec.ts tests/channel.spec.ts tests/creator-live-setup.spec.ts --reporter=list`
- `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode } catch { $_.Exception.Response | Select-Object StatusCode }`

## Scope (current change)
- Clean up the in-viewer sign-in overlay so it no longer exposes the raw redirect route (for example `/viewer`) as visible UI copy.
- Remove the redundant duplicate "Sign in" controls that appear when self-signup is disabled, while keeping the auth flow and redirect behavior unchanged.
- Add focused viewer test coverage for the cleaned-up auth-dialog presentation.

## Assumptions
- The reported issue is confined to `web/viewer/components/auth/AuthDialog.tsx`; backend auth APIs and redirect plumbing in `useAuth` should remain unchanged.
- Replacing the raw route chip with friendlier return-context copy is acceptable as long as successful auth still sends the viewer back to the same route.
- Hiding the single-mode tab control when sign-up is unavailable is the smallest fix for the duplicate-button complaint.

## Risks
- If the replacement return copy is too generic, the dialog could feel less grounded than the current route-specific presentation.
- Tab visibility logic must still preserve the full sign-in/sign-up switcher when self-signup is allowed.
- Tightening auth-dialog tests around copy and button counts can become brittle if selectors depend on exact phrasing unnecessarily.

## Test plan
- `npm.cmd --prefix web/viewer run test -- authDialog.test.tsx useAuth.test.tsx`
- `npm.cmd --prefix web/viewer run lint`
- `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh --viewer`

## Scope (current change)
- Remove the auth-dialog return-summary panel entirely so the sign-in window only shows auth actions and not any "continue where you left off" text box.
- Keep the previous duplicate-sign-in cleanup intact while tightening the dialog to a simpler signed-out presentation.
- Re-run focused viewer verification and refresh the local viewer stack so `localhost:8080/viewer` reflects the trimmed dialog.

## Assumptions
- The return-summary box is purely presentational and can be removed without affecting redirect behavior, because the actual post-auth redirect still lives in `useAuth`.
- No backend, route, or auth-hook changes are needed; this follow-up should stay confined to the dialog component and its focused tests.
- Viewer-only UI cleanup still does not require deployment-contract documentation changes.

## Risks
- Removing the panel changes dialog spacing, so the auth overlay could feel top-heavy if the remaining content does not flow cleanly.
- Tests that currently assert the return-summary copy must be updated or they will fail for the wrong reason.
- A local redeploy is still needed after the code change, otherwise the running viewer will continue showing stale overlay markup.

## Test plan
- `npm.cmd --prefix web/viewer run test -- authDialog.test.tsx useAuth.test.tsx`
- `npm.cmd --prefix web/viewer run lint`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode } catch { $_.Exception.Response | Select-Object StatusCode }`

## Scope (current change)
- Analyze the current local post-install Docker Compose logs for runtime errors or warning patterns that appear repo-owned and actionable from this codebase.
- Correlate the most important findings to the owning service, source file, script, or contract surface before deciding whether a narrow code/doc fix is warranted.
- If a high-confidence repo issue is reproducible from the logs, implement the smallest fix and re-run the relevant verification; otherwise keep this pass analysis-only with precise findings.

## Assumptions
- The repo-root `.env` plus `deploy/docker-compose.yml` describe the local install the user wants analyzed.
- Recent container logs still contain enough startup/runtime signal to identify post-install issues without tearing the stack down and reinstalling.
- Some log noise may come from third-party services or host Docker behavior, so only findings with clear repo ownership should drive code changes.

## Risks
- Startup logs can contain benign one-time warnings, so we need to separate ambient noise from real user-facing defects before editing code.
- A too-broad fix driven by logs could accidentally change the deployment contract or healthy service behavior, so any remediation should stay minimal and evidence-backed.
- Docker access or missing historical logs on this host could limit what we can verify directly, in which case the investigation should report the evidence gap instead of guessing.

## Test plan
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps -a`
- `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=200`
- `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=200 bitriver-live viewer postgres redis ome srs transcoder-public`
- `rg -n "<finding-specific-pattern>" cmd internal web deploy scripts docs`
- If a repo fix is made: run the narrowest focused test(s) plus `./scripts/verify.sh` when the affected surface warrants it

## Scope (current change)
- Add a guided first-run wizard for the source-based `cmd/bitriver` flow so operators can set key quickstart controls instead of editing `.env` manually after the fact.
- Support the wizard in both `go run ./cmd/bitriver env init` and `go run ./cmd/bitriver quickstart`, while preserving the existing non-interactive defaults for scripts/CI users who do not opt in.
- Cover the new wizard prompts with focused `cmd/bitriver` tests and document the guided path in the quickstart docs and README.

## Assumptions
- The user pain is primarily about missing first-run controls, not about replacing the underlying Compose-based deployment contract.
- A small guided set of prompts is enough for now: admin email, viewer/API URLs, API port, OME host settings, transcoder public URL, and self-signup.
- Existing secret generation should remain automatic; the wizard should collect operator-facing deployment values without forcing users to hand-enter every secret up front.

## Risks
- Interactive prompt code can easily break non-interactive automation if the wizard ever runs unexpectedly, so the opt-in/TTY gating needs to stay explicit and well tested.
- Writing wizard choices back into `.env` must stay aligned with the current validation and quickstart contract or users will get a friendlier prompt followed by the same validation failure.
- The quickstart success path currently prints generated credentials based on first-run env state; adding wizard support could accidentally hide or misreport those generated values if the comparison logic is wrong.

## Test plan
- `go test ./cmd/bitriver -count=1`
- `go test ./... -count=1 -timeout=120s`
- `docker compose -f deploy/docker-compose.yml config`
- `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh`

## Scope (current change)
- Redeploy the local Docker Compose stack from the current workspace so the operator environment is refreshed against the latest checked-out repo state.
- Keep the deployment contract unchanged; this is an operational local-stack refresh, not a code or config refactor.
- Confirm the relevant local services are recreated successfully and report any runtime blocker clearly.

## Assumptions
- The user wants the current local Compose environment refreshed from this checkout, even though the most recent code changes were in the CLI/docs path rather than the long-running API/viewer runtime.
- The existing root `.env` remains the intended local deployment input for this machine.
- Docker Desktop/Engine is available locally because Compose config validation succeeded earlier in this session when `--env-file .env` was supplied.

## Risks
- Local redeploy may rebuild or recreate containers unexpectedly if the current stack has drifted or if images are stale.
- Because the current `.env` still uses local loopback/demo-style values in some places, the redeploy only proves local operability, not production readiness.
- If Docker or the local daemon state changed since the earlier checks, the redeploy could fail for host reasons unrelated to the repository changes.

## Test plan
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode } catch { $_.Exception.Response | Select-Object StatusCode }`

## Scope (current change)
- Simplify the signed-out auth dialog further by removing the extra sign-in reassurance heading and paragraph inside the sign-in form.
- Keep the dialog focused on the title, fields, and actions only; do not change redirect behavior, auth APIs, or the prior duplicate-sign-in cleanup.
- Re-run focused viewer checks and refresh the local viewer container so the simplified dialog is live at `localhost:8080/viewer`.

## Assumptions
- The top-level dialog title already provides enough context, so the inner `Sign in without losing your place` block is redundant.
- This follow-up remains confined to `web/viewer/components/auth/AuthDialog.tsx` and focused auth-dialog tests.
- Viewer-only copy removal still does not require deployment-contract documentation changes.

## Risks
- Removing the inner heading/subcopy changes the dialog rhythm slightly, so we should confirm the remaining fields/actions still read cleanly.
- Focused tests need to assert the copy is gone so the change does not quietly regress.
- A local rebuild is still required after the viewer edit, otherwise the running stack will keep serving stale dialog markup.

## Test plan
- `npm.cmd --prefix web/viewer run test -- authDialog.test.tsx useAuth.test.tsx`
- `npm.cmd --prefix web/viewer run lint`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode } catch { $_.Exception.Response | Select-Object StatusCode }`
