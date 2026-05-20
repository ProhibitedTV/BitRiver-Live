## Scope (current change)
- Address GitHub issue #1222 by making the Following side chrome state-aware and discovery-focused.
- Keep watch/creator routes focused on stream, chat, and owner workflows with no following drawer/sidebar.
- On discovery routes, keep the persistent desktop sidebar and mobile drawer only when a signed-in viewer actually has followed channels.
- Let guests and empty-following users use the normal `Following` nav/page path instead of reserving full sidebar or drawer space.
- Keep the change viewer-only: `ViewerShell`, following state/sidebar helpers, CSS only if needed, and focused unit/Playwright tests.

## Assumptions
- The dedicated `/following` page remains the right full-state surface for guest, empty, error, and followed-channel experiences.
- Persistent side chrome is useful only after the viewer has a followed-channel list; otherwise it competes with discovery content.
- The existing route gating for watch/creator pages is the correct direction and should be preserved.
- Following refreshes should not collapse the sidebar/drawer or shift layout once populated data is already visible.

## Risks
- Moving following state into `ViewerShell` can duplicate fetches unless the sidebar is made presentational when data is already loaded.
- Hiding guest/empty side chrome will break tests that currently expect `Show following` on every discovery route.
- If the refresh hook sets `loading` during background refreshes, populated following chrome may disappear every 30 seconds.
- Desktop grid classes currently depend on `viewer-shell--following-disabled`; route-eligible but unpopulated following states must use the disabled layout.

## Test plan
- `npm.cmd --prefix web/viewer run test -- viewerShell.test.tsx followingSidebar.test.tsx followingStatePresentation.test.tsx --silent`
- `npm.cmd --prefix web/viewer run test:playwright -- tests/homepage-layout.spec.ts tests/navbar-mobile.spec.ts tests/mobile-layout.spec.ts tests/channel.spec.ts`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `git diff --check`
- `./scripts/verify.sh --viewer`

## Scope (current change)
- Address GitHub issue #1221 by simplifying the standalone `/browse` discovery page so search, sort, filters, and channel cards appear faster on desktop and mobile.
- Keep the change viewer-only and focused on `web/viewer/app/browse/page.tsx`, existing Browse/Directory tests, and narrow CSS additions only if needed.
- Preserve URL state for search query (`q`) and topic/category (`topic`) through `useDirectorySearch`.
- Remove or collapse duplicated explanatory layers, especially the stats header and featured shortlist that repeats the same channel set.
- Keep empty/error states helpful but shorter.

## Assumptions
- The homepage already carries the richer discovery narrative; `/browse` should behave like a focused directory tool.
- The current API can only derive category/tag chips from loaded directory data, so "stable" chip behavior means preserving the active URL topic even when a search result set does not include that topic.
- `DirectoryGrid` already emphasizes status, viewer/follower counts, category, creator name, and card actions, so this pass should not rewrite card internals unless tests show a scanning regression.
- Existing `browsePage.test.tsx`, `accessibility.spec.ts`, and `mobile-layout.spec.ts` are the right coverage anchors for Browse behavior.
- PR CI exposed an unrelated full-suite Viewer CI flake in `channel.spec.ts`; if it blocks this PR, keep any stabilization test-only and document the rerun evidence in `TASKS.md`.

## Risks
- Removing featured shortlist markup will break tests that currently assert highlight links; tests should move to the new primary grid behavior.
- Tightening copy and moving results up can accidentally hide reset/category state if status text becomes too terse.
- Search/category URL updates rely on router push/replace semantics in `useDirectorySearch`; tests need to cover both query and topic paths.
- CSS for Browse shares generic `.surface`, `.section-heading`, `.chip`, and `.search-bar` rules with other pages, so style edits must be class-scoped.
- The existing theme-toggle Playwright test can read `body[data-theme]` before Navbar hydration applies the stored/preferred theme, so a deterministic localStorage setup is safer than deriving the test branch from a transient initial attribute.

## Test plan
- `npm.cmd --prefix web/viewer run test -- browsePage.test.tsx directoryPage.test.tsx --silent`
- `npm.cmd --prefix web/viewer run test:playwright -- tests/accessibility.spec.ts tests/mobile-layout.spec.ts`
- `npm.cmd --prefix web/viewer run test:playwright -- tests/channel.spec.ts -g "theme toggle updates the rendered document"`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `git diff --check`
- `./scripts/verify.sh --viewer`

## Scope (current change)
- Address GitHub issue #1225 by hardening the viewer's small-screen layout across discovery, watch, chat, following, auth, and creator live setup surfaces.
- Keep the change viewer-only: global viewer CSS and Playwright coverage, with source component changes only if CSS cannot make existing markup safe.
- Target the acceptance widths called out by the issue: 320, 360, 390, 430, tablet, and desktop.
- Prevent document-level horizontal scrolling, make action rows thumb-friendly on narrow screens, and make long titles, tags, URLs, stream keys, tabs, and chat text wrap or truncate safely.
- Avoid deployment contract, API contract, auth behavior, and CI/workflow changes.

## Assumptions
- The next issue after merged PR #1234 is the oldest remaining open product ticket, issue #1225.
- Most breakage is caused by accumulated CSS overrides, minimum widths, wide action rows, and long tokens rather than missing data or API behavior.
- The existing route-aware following change from issue #1227 is the baseline: following chrome exists on discovery routes and is absent on watch/creator routes.
- Playwright browser coverage is the right regression guard because the issue is layout-specific and depends on real viewport widths.
- Visual before/after proof can be recorded as PR notes plus automated viewport checks instead of committing screenshot artifacts.

## Risks
- `globals.css` has several late override sections, so new mobile rules must be placed after the effective rules or they can silently lose to older declarations.
- Reducing mobile chrome can accidentally hide important navigation, search, account, chat, or creator actions if rules are too broad.
- Full Playwright validation may expose existing flaky viewer tests from `main`; fixes should stay limited to mobile layout coverage unless a blocker prevents the viewer gate from running.
- Long read-only inputs and chips can overflow even when their parent grid is responsive, so both containers and text nodes need `min-width: 0` and wrapping/truncation safeguards.
- Touching shared button/action-row CSS can affect desktop spacing, so narrow-width overrides should be explicit.

## Test plan
- `npm.cmd --prefix web/viewer run test:playwright -- tests/mobile-layout.spec.ts tests/navbar-mobile.spec.ts tests/homepage-layout.spec.ts tests/channel.spec.ts tests/creator-live-setup.spec.ts`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `git diff --check`
- `./scripts/verify.sh --viewer`

## Scope (current change)
- Finish the remaining PR #1234 CI follow-up after the quickstart smoke port fix.
- Keep the already-green Ubuntu test-all gate and image vulnerability scan untouched.
- Fix the quickstart entrypoint sanity failures by restoring ShellCheck suppressions for indirectly invoked deploy-smoke callbacks and aligning the PowerShell quickstart wrapper with its static contract.
- Fix viewer/Go CI setup failures where local composite actions are referenced before checkout on jobs that do not already fetch the repository.
- Fix the now-unmasked Viewer CI integration failure by starting Playwright against the standalone Next.js server that matches `output: "standalone"` instead of `next start`.
- Avoid deployment contract changes.

## Assumptions
- The latest CI run `25980878280` is the source of truth for this pass.
- The local composite actions in `.github/actions/` are valid; the failing jobs simply need checkout before using them.
- `scripts/deploy-smoke.sh` callbacks are intentionally invoked indirectly through traps/polling helpers, so ShellCheck suppressions should stay narrow and local.
- The PowerShell quickstart wrapper should keep delegating orchestration to the Go CLI while preserving helper names/static snippets that CI validates.
- The broad Playwright failures are caused by the test server using `next start` with standalone output; matching the Docker/README standalone server path should restore client hydration for static/CSR viewer routes without changing runtime app behavior.
- After switching the test server to standalone, the remaining Viewer CI failures are stale Playwright expectations for current creator/profile/directory copy plus upload mocks that compare Playwright's `request.method` function instead of calling `request.method()`.

## Risks
- CI workflow edits can accidentally affect required-check behavior, so changes should be limited to repository checkout ordering before local actions.
- Adding PowerShell helper compatibility should not reintroduce Docker orchestration into the wrapper's validate-only path.
- ShellCheck suppressions should not hide broad script issues beyond the two callback functions reported by CI.
- The standalone server requires `.next/static` and `public/` beside `.next/standalone/server.js`; the test helper must copy those assets without disturbing production Docker behavior.
- Test refreshes should stay limited to current UI contracts and mocked API behavior; no runtime component behavior should change for this CI follow-up.

## Test plan
- `bash -n scripts/deploy-smoke.sh scripts/quickstart.sh scripts/test-quickstart.sh`
- PowerShell static snippet check mirroring `.github/workflows/ci.yml`
- `go test ./scripts -count=1`
- `npm.cmd --prefix web/viewer run test:playwright`
- `git diff --check`
- Recheck PR #1234 GitHub Actions after pushing.

## Scope (current change)
- Address GitHub issue #1227 by making the following surface route-aware instead of globally persistent.
- Keep following discovery available from home/browse/videos while removing persistent side chrome from channel watch and creator workflows.
- Make the mobile following entry less dominant by showing it only on discovery surfaces; watch pages should prioritize video, chat, details, and creator actions.
- Fix the PR #1234 Ubuntu smoke failure uncovered after push by preparing the API data mount, starting the API before viewer/proxy sidecars, letting Compose evaluate the already-healthy API dependency graph during the API-only start, surfacing CI smoke failures through GitHub annotations when raw logs are unavailable, and avoiding the generated smoke env's host-port collision on `8080`.
- Keep API contracts, auth behavior, and deployment configuration unchanged.
- Add focused unit and Playwright coverage for route placement and guest/empty/populated following states.

## Assumptions
- The navbar's primary `Following` route remains the durable entry point when the shell does not render a sidebar.
- Discovery pages still benefit from a following rail because it can personalize browsing without competing with playback.
- Channel watch and creator routes should not reserve desktop width or mobile vertical space for following.
- The existing following data hook and state components are sufficient; the change should mostly reshape shell placement and tests.
- The smoke-created `.env` can use a non-default host port because container-to-container API traffic and in-container health checks remain on port `8080`.

## Risks
- `ViewerShell` is mounted around every viewer route, so route matching must avoid hiding following on discovery routes accidentally.
- Removing the sidebar from watch pages can break tests that assumed the mobile `Show following` button always exists.
- Desktop CSS currently defines a two-column grid when the viewport is wide; no-sidebar routes need an explicit one-column override.
- Playwright coverage depends on the standalone viewer fixtures exposing enough following API responses to distinguish guest, empty, and populated states.
- The smoke-only API user override and phased app startup must not change `deploy/docker-compose.yml`; production still runs the API with the configured non-root runtime UID and declared Compose dependency graph.
- The API-first smoke start should rely on the existing Compose dependency graph only after explicit dependency health/completion waits, then start viewer/proxy sidecars without reprocessing dependencies.
- CI-facing failure annotations should stay limited to smoke diagnostics and avoid changing local command behavior.
- The high-port smoke default must stay limited to the generated test env; operators' real root `.env` and Compose default host port remain unchanged.

## Test plan
- `npm.cmd --prefix web/viewer run test -- viewerShell.test.tsx followingSidebar.test.tsx followingStatePresentation.test.tsx`
- `npm.cmd --prefix web/viewer run test -- channelPage.test.tsx`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `npm.cmd --prefix web/viewer run test:playwright -- tests/navbar-mobile.spec.ts tests/homepage-layout.spec.ts`
- `bash -n scripts/test-quickstart.sh`
- `go test ./scripts -count=1`
- `./scripts/verify.sh --viewer`

## Scope (current change)
- Get PR #1233 ready for merge by fixing the current failing CI checks without widening the chat-control feature scope.
- Address the Ubuntu test-all gate failures reproduced locally and in Actions logs:
  - `cmd/bitriver/env_validation_test.go` still calls `renderOMEFromEnv` with an output path even though that helper now delegates to the default generated OME path.
  - `internal/server/security_headers_test.go` expects the default CSP header, but `SecurityConfig.withDefaults()` currently fills every default security header except `ContentSecurityPolicy`.
- Address the follow-up CI contract failure where `scripts/check-contract-invariants.sh` selects `deploy/.env.example` for Compose substitution but Compose still tries to load service-level `env_file: ../.env`.
- Address the next follow-up CI failures from run `25827555245`:
  - Shell lint now fails on existing scripts that use `CDPATH= cd` and on callback/trap-only functions that ShellCheck cannot prove reachable.
  - The later `scripts/verify.sh` Docker Compose validation still falls back to `deploy/.env.example`, which does not satisfy Compose's service-level `env_file: ../.env` requirement when the root `.env` is absent in CI.
- Address the subsequent quickstart smoke failure where a clean GitHub runner starts Compose with `--pull never` before third-party runtime images such as `redis:7-alpine` and `debian:12-slim` exist locally.
- Address the latest quickstart smoke failure where the Linux runner's host bind mount can leave `deploy/transcoder-data` unwritable for the `transcoder` container's fixed UID.
- Address the follow-up quickstart smoke race where the final proxied viewer curl can run before the viewer sidecar is ready, even after container health dependencies have turned green.
- Address the remaining quickstart startup race by booting Compose dependencies first, waiting for their health/completion, and only then starting the API/viewer layer.
- Address the recurring image vulnerability scan failure by downloading the Trivy archive with retries to a file before extraction instead of streaming a possibly truncated response into `tar`.
- Address the latest CI run `25830088634` failures:
  - The phased smoke now reaches healthy dependencies but misses the exited `postgres-migrations` one-shot because `docker compose ps -q` only reports running containers in this context.
  - The hardened Trivy download now fails deterministically because the pinned `v0.50.1` GitHub release asset returns 404; update to the current official immutable `v0.70.0` Linux archive.
  - With Trivy `v0.70.0`, the scanner reaches first-party Go binaries and blocks on Go stdlib `CVE-2025-68121` from the `golang:1.21` builder images; rebuild all first-party Go binaries from a fixed Go patch line while leaving the module target at `go 1.21`.
  - After explicit dependency health/completion waits, application startup should not ask Compose to re-evaluate or restart the one-shot dependency chain.
  - The refreshed scan then reports Debian runtime package CVEs in the API image and `github.com/jackc/pgx/v5` `CVE-2026-33816`; move the static Go runtime image to Alpine and bump the real pgx module requirement to the fixed version.
  - The next image scan blocker is `next` `CVE-2025-29927`; bump the viewer Next.js packages within the existing 13.5 patch line to `13.5.11`.

## Assumptions
- The backend gate fixes are acceptable in this PR because they are blocking PR #1233's merge readiness and are narrowly scoped to test/API security-header contract drift.
- Restoring the default CSP in `withDefaults()` preserves the existing viewer proxy behavior because `/viewer` route responses still receive the viewer-specific inline-script CSP in the proxy path.
- No deployment contract changes are needed.
- The approved script change should only create a temporary root `.env` from `deploy/.env.example` when `.env` is absent, and it must clean that temporary file up.
- The user's approval covers the required script/check behavior changes needed to get the PR ready, including the narrow `scripts/verify.sh` fallback repair.
- The quickstart smoke should still build first-party images from the working tree before pulling missing non-buildable runtime images, because some helper services reuse locally built images without declaring their own `build:` block.
- The user's follow-up approval covers the workflow hardening needed for the recurring Trivy install failure, including moving off the broken old Trivy release asset.

## Risks
- Security header middleware ordering is broad; the CSP default fix must keep viewer routes exempt from the API/admin default CSP so the dedicated viewer CSP can be set by proxy handling.
- OME renderer tests should keep using the temp workspace output rather than accidentally writing to the real repository generated config.
- The contract check must not overwrite an operator's real root `.env`.
- Shell lint fixes should stay mechanical and narrow so they do not change deploy smoke semantics.
- The verify fallback must clean up only a temporary `.env` it created and must leave real operator `.env` files untouched.
- Pulling missing third-party images in `scripts/test-quickstart.sh` adds network dependency to the Docker smoke, but the CI runner already needs network to build/pull base layers and scan images.
- The temporary quickstart smoke Compose override must remain test-only; it should not alter `deploy/docker-compose.yml` or the production helper-service user contract.
- Running the `transcoder` service as the host UID/GID in the smoke override must stay limited to the smoke harness so the deployment contract's fixed runtime UID remains unchanged.
- Polling the final viewer/API curls should not hide persistent routing failures; diagnostics must include Compose status and relevant service logs when the endpoint never becomes reachable.
- Phased quickstart startup should preserve the deployed service graph while avoiding Compose's early dependency-failure short-circuit on slow hosted runners.
- Trivy install hardening touches CI workflow behavior; the version bump should stay limited to a known official GitHub release asset and avoid changing scanner policy.
- One-shot completion waiting should include stopped containers without changing application-service health behavior.
- Bumping Docker builder images should not imply raising the source checkout's minimum Go version or changing `go.mod`.
- Starting application services with `--no-deps` depends on the preceding explicit health/completion waits staying complete and ordered.
- Moving the API runtime image from Debian to Alpine must preserve `curl` for the existing Compose healthcheck and the non-root runtime user.
- Bumping pgx affects real Postgres builds because Docker drops the local stub replacement; local stubbed tests should remain unaffected.
- Next.js patch updates must keep the viewer on the existing major/minor line and preserve lint/build behavior.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./cmd/bitriver ./internal/server -count=1`
- `& 'C:\Program Files\Git\bin\bash.exe' -lc 'PATH="/c/Program Files/Docker/Docker/resources/bin:$PATH" ./scripts/check-contract-invariants.sh'`
- `& 'C:\Program Files\Git\bin\bash.exe' -lc 'bash -n scripts/verify.sh scripts/check-go-sum-not-empty.sh scripts/refresh-go-sum.sh scripts/require-image-digests.sh scripts/deploy-smoke.sh'`
- `& 'C:\Program Files\Git\bin\bash.exe' -lc 'bash -n scripts/test-quickstart.sh'`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./scripts -count=1`
- `git diff --check`
- `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh --viewer`
- Recheck PR #1233 GitHub Actions after pushing.

## Scope (current change)
- Address GitHub issue #1220 by reducing live chat chrome and making secondary chat actions less dominant.
- Keep chat data contracts unchanged while updating only the viewer `ChatPanel` presentation and tests.
- Consolidate settings and pop-out into a compact chat options menu, make pop-out a direct action, and keep connection status visible without using a permanent header pill.
- Move message reporting out of the message bubble flow into a modal/sheet so the thread does not jump when a report form opens.
- Replace the signed-out disabled composer with a clear sign-in CTA state.

## Assumptions
- The user wants the next oldest open product issue after merged PR #1232, which is issue #1220.
- A focused viewer-only implementation is preferable to touching `internal/chat/websocket.go`, since the issue is about UI clutter and current backend contracts already cover chat/report submission.
- Current chat tests already provide enough mocked API/WebSocket coverage to validate the intended behavior without adding backend tests.
- The repo viewer gate still has unrelated Go/backend failures from the prior run; this change should record that separately if it persists.

## Risks
- Refactoring chat controls can break keyboard/focus behavior if the new options menu is not dismissible by outside click and Escape.
- Moving reports into a modal can break report submission tests unless the selected message state is handled carefully.
- Removing the disabled signed-out composer changes existing assertions in Jest and Playwright, so auth-required tests must be updated together.
- Chat panel CSS is global and shared with channel layouts, so style changes should stay scoped to `chat-panel`/`chat-message` selectors.

## Test plan
- `npm.cmd --prefix web/viewer run test -- chatPanel.test.tsx`
- `npm.cmd --prefix web/viewer run test -- channelPage.test.tsx`
- `PLAYWRIGHT_BASE_URL=http://127.0.0.1:3000 PLAYWRIGHT_BROWSERS_PATH=0 npx.cmd playwright test tests/channel-chat-playback.spec.ts` with a temporary standalone viewer server
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `./scripts/verify.sh --viewer`

## Scope (current change)
- Address GitHub issue #1217 by reducing persistent viewer navbar density and clarifying the primary navigation/action hierarchy.
- Keep the default header viewer-first: public discovery routes and search stay prominent, while creator/admin/account/preferences actions move into compact account/site menu surfaces.
- Keep route behavior stable for sign-in, create-account, profile, creator setup/go-live, admin control center, and theme switching.
- Update the viewer navigation contract and focused navbar tests, with mobile drawer coverage preserved.
- Avoid deployment contract edits.

## Assumptions
- The merged watch-page work on `main` is the current baseline, and issue #1219 can be closed as completed because PR #1231 landed.
- The next issue should be the oldest remaining open product issue, #1217.
- Viewer-facing discovery routes should remain quickly reachable from the persistent header, but `Go Live`, `Profile`, `Control center`, and theme switching are secondary utilities rather than default primary tabs.
- The existing `Navbar` menu and drawer state can be extended without adding dependencies or changing auth contracts.

## Risks
- Moving nav items out of the primary list can break tests or user expectations if the drawer/account menu does not keep those routes discoverable.
- Admin and creator links depend on runtime API base URL and managed channel loading, so the menu wiring must preserve configured destinations.
- Navbar CSS has accumulated multiple override sections; edits need to land in the effective rules without broad layout churn.
- Playwright coverage may be host-sensitive because it builds and starts the Next.js viewer.

## Test plan
- `npm.cmd --prefix web/viewer run test -- navigation.test.ts navbar.test.tsx`
- `npm.cmd --prefix web/viewer run test -- viewerShell.test.tsx`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `npm.cmd --prefix web/viewer run test:playwright -- tests/navbar-mobile.spec.ts`
- `./scripts/verify.sh --viewer`

## Scope (current change)
- Restore the in-viewer overlay auth flow from `codex/signin-polish` onto the current checkout without wholesale branch checkout or unrelated viewer drift.
- Re-enable the mounted overlay dialog, route signed-out viewer CTAs back through `useAuth`, and keep `/signup` available as the existing standalone fallback surface.
- Preserve current external-auth compatibility where `/api/viewer/me` provides a `loginUrl`, so installs that still rely on redirect-based sign-in do not regress.
- Keep the change viewer-only and avoid deployment-contract edits.

## Assumptions
- The user wants the old overlay sign-in/join experience back inside the viewer shell, not a full rollback of every auth-related branch change from `codex/signin-polish`.
- The minimal selective port is the auth state/dialog wiring plus the CTA surfaces that currently send users to `/signup#login-form`.
- Existing `/signup` behavior should remain intact for admin/MFA/direct-entry flows even after the overlay is restored.
- Current viewer tests and auth mocks can be updated in place without touching backend contracts or operator docs.

## Risks
- Porting the branch `useAuth` logic too literally could break current external `loginUrl` redirect deployments, so redirect-based sign-in needs to remain supported when configured.
- The current checkout may still return `401/403` for anonymous `/api/viewer/me` requests, so the overlay state layer must not regress guest loading/error behavior while adding dialog state.
- Rewiring shared auth mocks can ripple across many viewer tests if the context shape changes are not kept backward-compatible.
- Restoring the overlay without its CSS would produce a technically mounted but visually broken dialog, so style hooks need to be carried over with the component wiring.

## Test plan
- `npm.cmd --prefix web/viewer run test -- __tests__/useAuth.test.tsx`
- `npm.cmd --prefix web/viewer run test -- __tests__/authDialog.test.tsx`
- `npm.cmd --prefix web/viewer run test -- __tests__/navbar.test.tsx`
- `npm.cmd --prefix web/viewer run test -- __tests__/followingStatePresentation.test.tsx`

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
- Redeploy the local BitRiver Live stack on this Windows host and confirm the currently checked-in deployment workflow still works in practice from the canonical repo contract.
- Use the existing repo-root `.env` as the source of truth for credentials and local listener values unless the quickstart path regenerates a different admin secret during this run.
- Prefer the documented Windows quickstart entrypoint first, then inspect the resulting Compose state, logs, and reachable auth/admin surfaces so the user gets both a running stack and the exact credentials that were exercised.

## Assumptions
- The current repo-root `.env` is intentionally provisioned for this workstation and already contains the admin email/password that local login should use if bootstrap does not rotate them.
- Docker daemon access on this machine still requires elevated execution, so the meaningful deployment checks need to run with host-level privileges.
- For a localhost shakedown, a successful deployment can be validated by healthy Compose services plus reachable `http://localhost:8080/`, `/viewer`, `/signup`, and `/admin`.

## Risks
- The quickstart path may still reject parts of the saved production `.env` or hit Compose/runtime issues before the stack is fully healthy, in which case we need to distinguish repo defects from host-only blockers.
- Because deployment-contract edits require confirmation in this repo, any fix that touches `deploy/docker-compose.yml`, root `.env`, or generated OME expectations may need a pause before implementation.
- Even when the stack starts, auth verification can still fail if bootstrap credentials drifted from the saved `.env`, so we need to confirm the actual login surface and account state before handing credentials back.

## Test plan
- `powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 -ValidateOnly`
- `powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 --env-file .env --compose-file deploy/docker-compose.yml`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120`
- `Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0`
- `Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer`
- `Invoke-WebRequest -UseBasicParsing http://localhost:8080/signup`
- `Invoke-WebRequest -UseBasicParsing http://localhost:8080/admin -MaximumRedirection 0`

## Scope (current change)
- Tighten the viewer discovery journey so homepage and browse surfaces never present dead-end controls to first-time visitors.
- Make homepage category chips actionable, make browse-page featured highlights open the relevant channel, and persist browse topic filters in the URL so drill-ins survive reload/back-forward navigation.
- Improve the touched discovery empty states with explicit recovery actions instead of passive "nothing here yet" copy.
- Keep the change viewer-only and avoid deployment-contract edits.

## Assumptions
- The highest-value UI/UX win for launch-readiness is to remove discovery dead ends rather than start another broad visual redesign.
- Reusing the existing browse search/filter model is preferable to introducing a new backend category-filter API; a URL-level `topic` filter on top of the current client-side filtering is enough for now.
- Homepage category drill-ins can safely route through `/browse` because that page already owns directory exploration and filtering.
- Viewer-only route/query/state updates do not require operator-doc updates because runtime contracts and deployment workflows stay unchanged.

## Risks
- Adding a new browse URL parameter can create router-state drift or reset loops if query hydration and local filter state are not synchronized carefully.
- Converting non-actionable cards/chips into links could break existing tests or styling if the DOM structure changes more than expected.
- Empty-state CTA copy can become misleading if it promises behavior the current product cannot fulfill, so the actions need to stay grounded in existing routes.

## Test plan
- `npm.cmd --prefix web/viewer run test -- __tests__/browsePage.test.tsx __tests__/directoryPage.test.tsx __tests__/channelDisplayPrimitives.test.tsx`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`

## Scope (current change)
- Diagnose why the current checkout cannot be pushed to `origin` and repair the safest viable Git path so the current work is available remotely.
- Confirm whether the blocker is non-fast-forward branch divergence, auth, or remote-policy rejection before making any branch or history changes.
- Prefer a non-destructive fix that preserves the current commits exactly as they exist now; avoid force-pushing `main`.
- Keep the change limited to Git workflow metadata and repo planning artifacts unless a small supporting doc update is required.

## Assumptions
- The immediate goal is to get the current local commits onto GitHub safely, not to rewrite shared history on `origin/main`.
- Because local `main` is already behind `origin/main`, creating and pushing a dedicated branch is likely safer than rebasing/pushing `main` without explicit user direction.
- The working tree is clean enough that branch creation or a push repair will not trample unrelated uncommitted work.

## Risks
- Rebasing or merging `origin/main` into the current `main` without confirmation could create unnecessary conflict resolution or change the exact commits the user expects to publish.
- Pushing directly to `main` could still be blocked by branch protection or auth even after divergence is addressed, so the exact remote error needs to be captured first.
- Any networked Git operation may require host-level approval or credentials outside the sandbox, so repair work may pause on environment access rather than repository state.

## Test plan
- `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live status --short --branch`
- `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live remote -v`
- `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live branch -vv`
- `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live push origin main`
- If non-fast-forward on `main`: create/push a safe topic branch from the current `HEAD` and verify `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live status --short --branch`

## Scope (current change)
- Prepare this Windows host for a production-leaning local OBS rehearsal using the current deployment contract (`production` + `pull`) and the existing repo-root `.env`.
- Fix the known LAN-host contract mismatches first: loopback/demo public URLs, `0.0.0.0` OME values, and missing image digests.
- Authenticate container-registry access, verify that the configured first-party release images actually exist, and only then pin digests plus attempt the canonical quickstart/Compose boot path.
- Keep `deploy/docker-compose.yml` unchanged unless validation proves a contract bug is the active blocker after real first-party images are available.

## Assumptions
- The user wants the strict production-like pull workflow first, not a faster fallback to build mode.
- The current `.env` image tags (`v1.2.3`) may be example/default values rather than already-verified published release tags, so registry reality must be checked before mutating the env file further.
- This host can use a workspace-local Go build cache (`.gocache`) to avoid the existing Windows profile permission issue during Go-backed validators/renderers.
- Local/LAN HTTP on `10.0.0.108` is acceptable for this rehearsal even though the saved `.env` remains in `production` mode.

## Risks
- If the configured first-party release tag does not exist in GHCR, the requested `pull`-mode rehearsal cannot complete without either publishing release images or switching to build mode.
- Editing the tracked root `.env` for a machine-specific LAN rehearsal can create diff noise and must not be committed with live secrets.
- Production digest enforcement requires both first-party and third-party pins; resolving only the public third-party digests is not enough to satisfy quickstart.
- Rerendering `deploy/ome/Server.generated.xml` against the LAN rehearsal values will intentionally change a tracked generated contract file and must stay aligned with the actual `.env`.

## Test plan
- `gh auth status`
- `gh auth token | docker login ghcr.io -u ProhibitedTV --password-stdin`
- `git ls-remote --tags origin`
- `docker buildx imagetools inspect ghcr.io/bitriver-live/bitriver-live:v1.2.3 --format "{{.Manifest.Digest}}"`
- `docker buildx imagetools inspect redis:7-alpine --format "{{.Manifest.Digest}}"`
- If first-party release images exist: `go run ./cmd/bitriver env validate --env-file ./.env`, `docker compose --env-file .env -f deploy/docker-compose.yml config`, `./scripts/require-image-digests.sh`, `go run ./cmd/bitriver ome render --force --env-file ./.env`, `go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml`

## Scope (current change)
- Pivot the local OBS rehearsal from the impossible `production + pull` path to a supported source-checkout build path that uses the current repository contents.
- Fix the concrete deployment-contract blockers already observed on this host: broken `postgres-migrations` command wiring, insufficient `transcoder-public` capabilities for the bundled nginx config, and a quickstart smoke script that assumes unpublished first-party GHCR images exist.
- Keep public APIs, storage schemas, and viewer/backend contracts unchanged while making the documented local source-build path actually boot and stream.
- Treat root `.env` and `deploy/ome/Server.generated.xml` as local runtime state for the rehearsal: update them for the host run, keep them aligned, and avoid committing secrets or host-specific values.

## Assumptions
- The near-term goal is a reliable source-build rehearsal on this Windows host, not publishing release tags or first-party GHCR images.
- The repo should continue to support strict release-style `pull` mode later, but the source-checkout smoke and local rehearsal path must no longer depend on nonexistent `v1.2.3` or `latest` first-party images.
- Default host ports (`8080`, `9080`, `1935`, `9000`, `9001`) are currently free and should be used unless a fresh conflict appears during implementation.
- The safest local runtime path is direct `docker compose ... up -d --build --pull never` with a development-mode `.env`, rather than trying to force the strict production quickstart contract onto unpublished source artifacts.

## Risks
- `deploy/docker-compose.yml`, root `.env`, and `deploy/ome/Server.generated.xml` are contract files, so any change must stay synchronized with `docs/contract.md` and operator docs.
- Fixing `postgres-migrations` and `transcoder-public` only in Compose while leaving Helm/docs inconsistent would create contract drift.
- `scripts/test-quickstart.sh` and `./scripts/verify.sh` may still be partially blocked on this host by missing `python3`, even after the source-build smoke path itself is fixed.
- The local rehearsal may still surface additional runtime issues after the stack starts (for example creator bootstrap, ingest preview timing, or OBS publishing), so validation needs to include actual route and service checks, not just `docker compose up`.

## Test plan
- `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/storage -count=1 -timeout=120s -run TestIngestPipelineEndToEnd`
- `docker compose --env-file .env -f deploy/docker-compose.yml config`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build --pull never`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps -a`
- `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 postgres-migrations bitriver-live transcoder-public`
- `./scripts/test-quickstart.sh`
- `./scripts/verify.sh`
- Route/runtime rehearsal checks: `http://10.0.0.108:8080/admin`, `/viewer`, `/viewer/creator/live/<channelId>`, RTMP ingest on `:1935`, and HLS playback on `http://10.0.0.108:9080/hls`

## Scope (current change)
- Repair the viewer chrome so the visible light/dark toggle, sign-in CTA, and sign-up/join CTA all respond predictably in the live app.
- Remove the unnecessary top spacing in the homepage shell so the left rail and main content sit cleanly under the fixed navbar.
- Keep the work viewer-only, with no backend API or deployment-contract changes.
- Validate the fixes in the shipped viewer surface, not just isolated component logic, because the reported failures are runtime UX issues.

## Assumptions
- The broken auth/theme behavior is likely caused by viewer-side state wiring or CSS layering rather than a backend auth outage, because the stack is already up and the controls render.
- The visible top gap is caused by competing shell spacing rules in `web/viewer/styles/globals.css`, especially the later rescue-pass overrides that moved top spacing from the content wrapper to the whole shell.
- A robust theme fix should update the document-level theme attribute in a way that both existing CSS selectors and browser-native UI elements honor consistently.
- Sign-in and join should stay in-modal/in-viewer by default unless an explicit external auth URL is configured.

## Risks
- The viewer stylesheet contains multiple later override sections, so changing spacing or navbar layering in one block can accidentally regress another viewport if the final cascade is not checked carefully.
- Auth CTA fixes that only satisfy Jest mocks could still miss a real runtime issue such as a hidden overlay, incorrect dialog mounting, or pointer-event conflict.
- Moving theme state from body-only handling to document-level handling can affect tests and any selectors that assume a single attribute location, so the change needs targeted coverage.

## Test plan
- `npm.cmd --prefix web/viewer run test -- __tests__/navbar.test.tsx`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- If browser automation is needed to confirm live behavior: `npm.cmd --prefix web/viewer run test:playwright -- tests/navbar-mobile.spec.ts`

## Scope (current change)
- Rework the public viewer and creator web surfaces into a polished v1 live-platform experience centered on the core loop: discover, sign up, create a first channel, go live, watch, tip, and replay VOD.
- Keep the deployment model and backend stack intact while tightening the information architecture, removing silent no-op UX, and making the viewer/watch experience feel intentional instead of experimental.
- Add self-serve creator bootstrap so an authenticated self-signup user can create their own first channel without admin intervention and immediately enter the creator live flow.
- Keep the change focused on viewer + creator surfaces plus the minimal backend/channel API updates required to support first-channel onboarding.

## Assumptions
- The existing viewer already has enough live, chat, follow, tip, and VOD primitives that the best path is a guided overhaul rather than a wholesale rebuild.
- Open self-signup remains the default posture for this v1 launch, and the highest-friction missing capability is self-serve first-channel creation.
- Tips and wallet-style support should be visually primary in the channel/watch UX, while subscriptions remain available but de-emphasized.
- This pass should improve mobile responsiveness and shell consistency without trying to solve broader product areas like admin redesign, notifications, or a clips ecosystem.

## Risks
- Relaxing `POST /api/channels` for self-service onboarding can accidentally over-broaden channel creation if ownership checks and role upgrades are not kept self-only and first-channel-safe.
- Refreshing the homepage, browse page, navigation, and channel page together creates a larger CSS and interaction surface, so shared shell regressions are a real possibility.
- Creator onboarding will touch both backend permissions and viewer client state, which can produce misleading partial success if only one side is updated.
- The repo already has unrelated modified contract/runtime files in the working tree, so this pass must stay disciplined about not overwriting unrelated changes while still updating the planning artifacts.

## Test plan
- Backend/bootstrap:
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/api ./internal/auth ./internal/storage -count=1 -timeout=120s`
- Viewer targeted:
  - `npm.cmd --prefix web/viewer run test -- __tests__/navbar.test.tsx __tests__/browsePage.test.tsx __tests__/directoryPage.test.tsx __tests__/channelDisplayPrimitives.test.tsx`
  - `npm.cmd --prefix web/viewer run test:playwright -- tests/channel.spec.ts`
- Viewer validation:
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`

## Scope (current change)
- Redeploy the current checkout onto the existing local Compose stack so the viewer/creator overhaul is actually running in the live environment again.
- Validate the redeployed stack at the runtime level rather than only through mocked viewer tests: confirm service/container health, API readiness, public viewer routes, and the key creator/live setup pages.
- Fix any concrete runtime blocker exposed by the live redeploy when it prevents the shipped viewer from hydrating or responding to input, then redeploy and re-check the live surface.
- Record any remaining gap explicitly if a fully manual broadcast or authenticated browser flow still requires human interaction.

## Assumptions
- The repo should still be running in the local source-build workflow established earlier (`BITRIVER_DEPLOY_IMAGE_SOURCE=build` with `docker compose ... up -d --build --pull never`).
- The current `.env` and generated OME config are already aligned enough to let the stack rebuild from source without another contract change.
- The highest-signal runtime proof for this pass is a successful rebuild plus live route checks on `http://10.0.0.108:8080`, not another full repository-wide verification sweep.
- Because this machine shares the same working tree as earlier changes, redeploy commands must avoid resetting or cleaning anything.

## Risks
- Docker rebuilds can fail for host reasons unrelated to the app changes (daemon state, image cache, port collisions), so the first step must separate environment failure from product failure.
- The redeployed viewer could still differ from the mocked Playwright coverage in areas that require real auth/session bootstrap or live ingest state, so runtime checks need to include both anonymous and creator-facing routes.
- The API server's default CSP may be too strict for the Next.js viewer bootstrap, so a server-side header fix could be required even if the viewer build itself is healthy.
- If the stack comes up but the admin/bootstrap data is stale, functional checks may need to stop at route availability rather than full signed-in action flows.

## Test plan
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build --pull never`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps -a`
- `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 bitriver-live viewer postgres-migrations transcoder-public`
- `Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8080/readyz`
- `Invoke-WebRequest -UseBasicParsing http://10.0.0.108:8080/viewer`
- `Invoke-WebRequest -UseBasicParsing http://10.0.0.108:8080/browse`
- `Invoke-WebRequest -UseBasicParsing http://10.0.0.108:8080/videos`
- `Invoke-WebRequest -UseBasicParsing http://10.0.0.108:8080/creator`
- `go test ./internal/server -count=1 -timeout=120s`
- Headless browser smoke against `http://10.0.0.108:8080/viewer` to confirm the theme toggle, auth CTA, and hydrated route content all respond after redeploy

## Scope (current change)
- Re-enable public self-signup on this local deployment so guest users can actually create accounts from the shipped viewer experience.
- Redesign the homepage to be more like a modern Twitch-style discovery surface: live content first, tighter copy, strong shelves, and a clearer split between featured content, recommended channels, and browse-by-topic exploration.
- Keep the changes focused on the live local deployment plus the viewer homepage/auth entry points; avoid unrelated backend or creator-flow changes unless they are required to support signup or homepage behavior.
- Treat the root `.env` change as local runtime state for this machine and redeploy the affected service after updating it.

## Assumptions
- The immediate signup blocker is the current runtime flag `BITRIVER_LIVE_ALLOW_SELF_SIGNUP=false`, not a deeper auth/session failure.
- A Twitch-inspired homepage for this pass means borrowing the information hierarchy and content density, not cloning Twitch styling or copy.
- Existing discovery APIs (`featured`, `recommended`, `following`, `liveNow`, `trending`, `categories`) are sufficient to build a much stronger homepage without adding new backend endpoints.
- The current local stack should only require a `bitriver-live` restart for the signup flag and a viewer rebuild/redeploy for homepage code changes.

## Risks
- Editing the tracked root `.env` is a deployment-contract change, so the local runtime adjustment must stay intentional and should not leak secrets or unrelated config edits.
- Homepage changes touch a broad CSS surface and the most-trafficked viewer route, so regressions in spacing, mobile behavior, or inactive states are easy to introduce if the layout change is too aggressive.
- A homepage that becomes too visually close to Twitch without using our own copy, color choices, and interaction patterns could feel derivative instead of intentional.
- Redeploy validation needs both mocked viewer tests and a real browser smoke, because recent regressions only surfaced on the live `/viewer` mount.

## Test plan
- `npm.cmd --prefix web/viewer run test -- __tests__/directoryPage.test.tsx __tests__/navbar.test.tsx`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build --pull never viewer bitriver-live`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps -a`
- `Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8080/api/viewer/me`
- Headless browser smoke against `http://10.0.0.108:8080/viewer` to confirm the create-account flow is offered again and the redesigned homepage renders and hydrates on the live deployment

## Scope (current change)
- Repair the current verification blockers so the repository has a clean baseline before broader Twitch-style product work continues.
- Keep this pass focused on test/check reliability and functional gate health; do not start the local Compose stack or change runtime deployment settings.
- Avoid deployment-contract edits: leave `deploy/docker-compose.yml`, root `.env`, and `deploy/ome/Server.generated.xml` behaviorally unchanged.
- Add only the backward-compatible `bitriver ome render --output` flag needed to keep OME render tests from mutating the tracked generated contract file.

## Assumptions
- The first repair pass should prioritize the standard gates rather than a live-stack rehearsal or broader product-gap sweep.
- The local Compose stack being down is acceptable for this pass as long as Compose config validation still succeeds.
- The stale viewer snapshot should follow the current `Featured live` copy because that wording better matches the discovery-first live platform goal.
- Any Windows-specific test handling should preserve real coverage wherever possible and skip only host capabilities such as directory symlink creation when unavailable.

## Risks
- Transcoder health tests can hide real recovery bugs if the fix only widens timeouts, so recovery should be tied to successful state transitions or explicit component reset behavior.
- Adding an OME output path flag must not weaken the canonical generated-file path used by normal operators and Compose.
- Replacing wall-clock session sleeps with a test clock must not leak test-only API into public runtime surfaces.
- Updating snapshots can mask UI drift if the assertion coverage is too broad, so the copy change should stay narrowly targeted.

## Test plan
- `go test ./cmd/transcoder -count=1 -timeout=120s`
- `go test ./internal/auth -count=25 -run TestValidateHonorsAbsoluteTTL`
- `go test ./internal/ingest ./scripts -count=1 -timeout=120s`
- `npm.cmd --prefix web/viewer run test -- --silent`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `docker compose --env-file .env -f deploy/docker-compose.yml config`
- `./scripts/verify.sh`

## Scope (current change)
- Improve the core viewer functionality around channels, chat, and signup without changing the deployment contract.
- Upgrade the channel chat panel from REST-only polling to the existing authenticated `/api/chat/ws` gateway, while keeping REST history loading and polling as fallback for guests or failed sockets.
- Tighten the creator signup/onboarding path so creator-intent account creation points clearly toward first-channel setup instead of feeling like a viewer-only signup.
- Keep backend channel creation semantics unchanged for this pass because self-signup first-channel creation and role promotion already have API coverage.

## Assumptions
- The existing `POST /api/channels` self-service path is the correct first-channel bootstrap: a self-signup viewer creates their own first channel and is promoted to creator.
- The highest-value chat functionality gap is realtime delivery on the viewer channel page; moderation/report tooling can remain API-backed for this pass.
- A WebSocket failure should not break chat history or the composer; REST polling and send APIs remain the fallback path.
- Signup copy can adapt to creator intent using the existing auth redirect target, without adding new API payloads or signup roles.

## Risks
- WebSocket message envelopes use the gateway event shape, while the viewer renders REST chat DTOs; mapping and de-duplication must prevent duplicate messages when ack, broadcast, and polling overlap.
- jsdom does not exercise real browser WebSocket behavior by default, so tests need a small controllable socket fake.
- Auth dialog copy changes can accidentally regress generic viewer signup copy if creator-context detection is too broad.
- Chat changes touch the live channel page's most interactive surface, so focused component tests should cover both socket and fallback paths.

## Test plan
- `npm.cmd --prefix web/viewer run test -- __tests__/chatPanel.test.tsx __tests__/channelPage.test.tsx __tests__/authDialog.test.tsx __tests__/creatorGettingStartedPage.test.tsx --silent`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `go test ./internal/api ./internal/chat ./internal/auth -count=1 -timeout=120s`

## Scope: Product Readiness Closure - VOD Publish and Chat Reports

### Summary
Turn the latest product-readiness audit into a focused implementation pass without touching the deployment contract. This pass closes two user-visible functional gaps that block a Twitch-style self-hosted baseline: creators can publish upload-backed recordings into the public VOD surface, and signed-in viewers can report abusive chat messages from the live chat UI.

### Goals
- Refresh `SPEC.md` so the repo has product acceptance criteria for a self-hosted live streaming website, not only contributor workflow criteria.
- Add a creator upload action that calls the existing `POST /api/recordings/{id}/publish` API and refreshes the upload list/public VOD path.
- Add a chat-message report flow that calls the existing `POST /api/channels/{id}/chat/reports` API with `targetId`, `messageId`, and a viewer-supplied reason.
- Document a self-hosted acceptance checklist in existing testing docs so the final product bar includes real broadcast, playback, chat, VOD, and moderation proof.

### Assumptions
- The deployment contract remains unchanged: no edits to `deploy/docker-compose.yml`, root `.env`, or `deploy/ome/Server.generated.xml`.
- Existing backend APIs for recording publish and chat reports are the source of truth for this pass.
- A full stream schedule data model and real Docker/OBS-style broadcast rehearsal are follow-up product gates, not hidden inside this UI-focused patch.

### Risks
- Upload items do not expose a `publishedAt` field today, so the UI can confirm the publish request and refresh rather than rendering a durable published badge from the upload payload.
- Chat history groups consecutive messages by author; per-message report controls must stay attached to individual messages without making the thread noisy.
- Viewer test mocks must stay aligned with the public `viewer-api` barrel exports.

### Test Plan
- `npm.cmd --prefix web/viewer run test -- --silent UploadManager ChatPanel viewer-api`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `./scripts/verify.sh` if local prerequisites are available within the remaining pass.

## Scope: Product Readiness Closure - Schedule and Final Gates

### Summary
Finish the unresolved product-readiness items from the previous pass. Add a real channel schedule model instead of the public placeholder, expose creator editing in the live dashboard, render upcoming streams on public channel pages, and rerun the broad verification gates with enough time to complete.

### Goals
- Add a typed channel schedule model that works in JSON storage and Postgres-backed self-hosted installs.
- Extend existing channel create/update/read payloads with backward-compatible optional schedule entries.
- Let creators manage upcoming scheduled streams from the existing Go Live dashboard.
- Replace the public channel Schedule placeholder with actual upcoming stream entries.
- Re-run full verification and capture any remaining real-stack smoke limitations clearly.

### Assumptions
- A channel-level schedule array is the right minimal product shape for this pass: it supports one or more upcoming stream entries without introducing a separate moderation or calendar subsystem.
- Adding a nullable/defaulted Postgres column is a schema migration, not a Compose or `.env` deployment-contract change.
- A real OBS/manual broadcast rehearsal can be represented in docs and smoke gates here, but only automated if local Docker/encoder prerequisites are available.

### Risks
- Channel schedule JSON must be validated and normalized consistently across memory and Postgres repositories.
- Existing channel PATCH behavior must remain backward-compatible for title/category/tag-only updates.
- The public Schedule tab must stay useful when a creator has no upcoming entries.

### Test Plan
- `go test ./internal/storage ./internal/api -count=1 -timeout=120s`
- `npm.cmd --prefix web/viewer run test -- --silent creatorLivePage channelPage viewer-api`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s`
- `docker compose --env-file .env -f deploy/docker-compose.yml config --quiet`
- `./scripts/verify.sh`

### Status
- Implemented. See `TASKS.md` for per-task results and final gate output.

## Scope: Viewer Auth UI Refresh

### Summary
Fix the deployed viewer behavior where signing in from the normal site chrome updates the auth cookie but leaves route-level UI in its previous guest state until a navigation path forces a refresh. Keep this focused on client auth refresh behavior; do not change auth payloads, cookies, deployment config, or database schema.

### Goals
- Refresh Next.js route data after successful sign-in, sign-up, MFA completion, and sign-out when the user stays on the same route.
- Preserve the existing redirect behavior when auth was opened with a different safe destination.
- Add focused auth-provider coverage proving a same-page sign-in calls the route refresh path and updates the visible auth state.
- Rebuild/redeploy the viewer container so the running Docker stack reflects the fix.

### Assumptions
- The backend auth flow and cookies are working; the stale UX is caused by client route data not refreshing after the cookie changes.
- A client-side `router.refresh()` is the right Next.js primitive because it refreshes server components and cached route payloads without a full browser reload.
- No deployment contract change is needed.

### Risks
- Calling refresh before the viewer session has been reloaded could briefly preserve guest data, so refresh should happen after `/api/viewer/me` completes.
- Existing tests that render `AuthProvider` need a small router mock once the provider uses `useRouter`.

### Test Plan
- `npm.cmd --prefix web/viewer run test -- __tests__/useAuth.test.tsx --silent`
- `npm.cmd --prefix web/viewer run test -- __tests__/navbar.test.tsx __tests__/directoryPage.test.tsx __tests__/followingStatePresentation.test.tsx --silent`
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`

## Scope: OME Ingest Health Auth Repair

### Summary
Fix the deployed OME ingest health failure where BitRiver reports OvenMediaEngine as down with `401 Unauthorized` even though the OME container is reachable. Keep this pass focused on aligning BitRiver's ingest control-plane requests with the existing OME AccessToken contract; do not change Compose ports, `.env`, generated OME XML, or the vendor OME container health contract.

### Goals
- Load the canonical `BITRIVER_OME_API_TOKEN` into ingest config.
- Use OME's documented Basic auth credential form, where the raw rendered AccessToken is base64-encoded as the full Basic credential string.
- Treat authenticated OME non-5xx responses as reachable for the shared `/healthz` probe path, because OME returns `404` for that path after authentication rather than exposing a native `/healthz`.
- Preserve backward-compatible Basic auth fallback only when no OME API token is configured.
- Add focused tests that catch the runtime mismatch: OME health should send `AccessToken`, and missing ingest config should require `BITRIVER_OME_API_TOKEN`.
- Rebuild/redeploy the API service so `/healthz` and the viewer ingest status reflect the repaired OME health path.

### Assumptions
- The existing `deploy/ome/Server.generated.xml` already contains a rendered top-level `<Managers><API><AccessToken>` that matches `.env`.
- OME process liveness in Compose can remain unauthenticated and tolerant of 401 responses because it only verifies that the API listener is reachable.
- BitRiver's application-control API should follow the documented OME AccessToken-as-Basic-credential contract.

### Risks
- Tests and stubs currently expect Basic auth for OME adapter calls, so they need to be updated without weakening SRS/transcoder auth coverage.
- Runtime logs may continue to show 401 entries from OME's own unauthenticated container health probe; the success condition is BitRiver ingest status no longer marking OME down.

### Test Plan
- `go test ./internal/config ./internal/ingest -count=1 -timeout=120s`
- `go test ./internal/storage -run Ingest -count=1 -timeout=120s`
- `docker compose --env-file .env -f deploy/docker-compose.yml up --build -d bitriver-live`
- `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- `Invoke-WebRequest -UseBasicParsing http://localhost:8080/healthz -TimeoutSec 15`
- `Invoke-WebRequest -UseBasicParsing http://localhost:8080/readyz -TimeoutSec 15`

## Scope: Main Merge Conflict Resolution

### Summary
Merge the current `origin/main` into `fix-viewer-discovery-polish` and resolve any conflicts in favor of this branch. Keep this pass limited to conflict resolution and merge hygiene; do not introduce new product behavior beyond the merge result.

### Assumptions
- `origin/main` is the intended base branch for the PR conflict.
- "Our branch" means the currently checked out `fix-viewer-discovery-polish` branch.
- The right conflict policy is Git's `-X ours` behavior: keep incoming non-conflicting base changes, but choose this branch's hunks where both sides touch the same lines.

### Risks
- The base branch carries a large number of non-conflicting file additions, removals, and edits, so the resulting merge commit is broad even though conflict resolution itself is mechanical.
- Full verification may be expensive after a broad merge; run lightweight conflict hygiene unless a later pass asks for full gates.

### Test Plan
- `git merge --no-commit --no-ff -X ours origin/main`
- `git diff --name-only --diff-filter=U`
- `rg -n "<<<<<<<|=======|>>>>>>>" --glob "!web/viewer/node_modules/**" .`
- `git diff --cached --check`

## Scope: GitHub Issue #1219 Watch Page Priority

### Summary
Tighten the channel watch page hierarchy so live, offline-with-replays, offline-without-replays, and mobile states feel viewer-first instead of dashboard-like. Keep the pass viewer-only and avoid deployment contract changes.

### Goals
- Make the video/player the first visual priority, followed by chat and compact channel actions.
- Present one primary offline recovery/content action area instead of duplicating player and replay-card actions.
- Add a small mobile watch navigation affordance so video, chat, details, and videos are easy to reach without forcing users through a long stack.
- Keep creator-owner tools visually separate from the public watch flow.
- Add regression coverage that playback refreshes do not reset the active tab.

### Assumptions
- `main` already includes baseline player recovery, public schedules, VOD replay cards, and channel tab URL state.
- The channel page should decide whether player recovery or replay/schedule recovery is primary; `Player` should stay generic.
- Mobile smoke coverage can assert structural hierarchy and affordances without taking screenshot snapshots in this pass.

### Risks
- Removing player recovery actions from offline pages could hide a useful live-check button unless the offline action area explicitly exposes one.
- Mobile anchors can feel noisy if visible on desktop, so CSS should scope them to small screens.
- Timer-driven refresh coverage can become flaky if fake timers are not restored reliably.
- Full viewer validation may expose stale current-main tests or compile blockers outside #1219; repair only the narrow blockers needed to restore the viewer gate.

### Test Plan
- `npm.cmd --prefix web/viewer run test -- __tests__/channelPage.test.tsx __tests__/player.test.tsx`
- `PLAYWRIGHT_BASE_URL=http://127.0.0.1:3000 PLAYWRIGHT_BROWSERS_PATH=0 npx.cmd playwright test tests/channel-chat-playback.spec.ts tests/channel.spec.ts` with a local viewer server if the package Playwright wrapper remains blocked by standalone output.
- `npm.cmd --prefix web/viewer run lint`
- `npm.cmd --prefix web/viewer run build`
- `./scripts/verify.sh --viewer`
