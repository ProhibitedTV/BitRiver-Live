## Scoped change: local redeploy for homepage QA

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the local redeploy scope before runtime actions
  - Acceptance criteria:
    - `PLAN.md` captures the redeploy scope, assumptions, risks, and verification commands.
    - `TASKS.md` lists the ordered rebuild and route-check tasks before Docker actions begin.

- [x] Task 2 - Rebuild and recreate the local app services from the current checkout
  - Acceptance criteria:
    - `bitriver-live` and `viewer` are rebuilt from the current source tree using the canonical Compose contract.
    - Compose status/log output is captured after the rebuild attempt.
    - The action stays limited to the app services unless a hard blocker requires wider intervention.

- [x] Task 3 - Verify the redeployed local routes reflect the latest viewer code
  - Acceptance criteria:
    - `http://localhost:8080/` responds after the rebuild.
    - `http://localhost:8080/viewer` serves the updated homepage rescue content.
    - `TASKS.md` records the post-redeploy status or any host blocker precisely.

### Execution log (local redeploy for homepage QA)
- Task 1 complete: recorded the local redeploy scope in `PLAN.md` and `TASKS.md` before touching the running stack so the rebuild and route checks were explicit.
- Task 1 checks:
  - `Get-Content PLAN.md | Select-Object -Last 30`
  - `Get-Content TASKS.md -TotalCount 30`
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live status --short`
- Task 2 complete: checked the current Compose status, attempted a targeted `up -d --build` for `bitriver-live` and `viewer`, then forced a no-deps recreate to make sure the app containers actually restarted on the current checkout. The recreate succeeded and the app services now show fresh container ages.
- Task 2 checks:
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer` (timed out before returning cleanly, so a follow-up status check plus explicit recreate was used)
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --force-recreate --no-deps bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=40 bitriver-live viewer` (still blocked on this host by Docker config/pipe access denial)
- Task 3 complete: verified the redeployed app is live locally. `/` still returns the expected `307` redirect, `/viewer` now serves the updated homepage rescue content including `Find the streams worth opening now`, `Watch live now`, and `Full directory`, and `/signup` still serves the sign-in-first auth page with `Back to viewer`.
- Task 3 checks:
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode,Headers } catch { $_.Exception.Response | Select-Object StatusCode,Headers }`
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer).Content | Select-String -Pattern 'Find the streams worth opening now|Watch live now|Full directory|Browse BitRiver Live' -AllMatches`
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/signup).Content | Select-String -Pattern 'Sign in to continue|Back to viewer' -AllMatches`

## Scoped change: homepage UI/UX rescue pass

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the homepage rescue diagnosis and implementation plan before editing
  - Acceptance criteria:
    - `PLAN.md` captures the homepage rescue scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered header/shell/homepage/styling/validation tasks before viewer edits begin.
    - The read-only diagnosis identifies the main hierarchy, spacing, and layout-system conflicts driving the rescue pass.

- [x] Task 2 - Simplify the global shell and header priorities
  - Acceptance criteria:
    - The navbar presents a clearer single-priority structure on desktop and mobile without breaking existing auth/creator/admin entry points.
    - The viewer shell and following sidebar feel like one product frame instead of disconnected panes.
    - Shared layout/tokens used by the current page are consolidated enough that the touched surfaces are easier to reason about.

- [x] Task 3 - Rebuild the homepage information hierarchy around one coherent discovery layout
  - Acceptance criteria:
    - The hero has a clear headline, primary action, search role, and compact supporting proof points.
    - Featured, categories, trending, following, live-now, and directory sections use consistent spacing, header structure, and surface treatment.
    - The page feels balanced on desktop and scales down cleanly on smaller screens without lopsided dead space or overflow.

- [x] Task 4 - Standardize shared component sizing and state presentation on the homepage
  - Acceptance criteria:
    - Card, button, pill, and input sizing is visibly more consistent across the current page.
    - Loading/empty/error states on the touched discovery surfaces are more intentional than plain text blocks.
    - Existing routes, links, and viewer data flows remain functional.

- [x] Task 5 - Run focused viewer validation and record the results
  - Acceptance criteria:
    - The targeted homepage/navigation Jest coverage passes.
    - Viewer lint and production build are rerun and logged.
    - Any remaining follow-ups or host-specific blockers are captured explicitly in the execution log.

### Execution log (homepage UI/UX rescue pass)
- Task 1 complete: recorded the scoped homepage rescue plan in `PLAN.md`/`TASKS.md` before editing and captured the read-only diagnosis that the current page was suffering from duplicated shell/navbar systems, muddled header priorities, a hero with weak hierarchy, redundant discovery patterns, and inconsistent state/card sizing.
- Task 1 checks:
  - `Get-Content PLAN.md | Select-Object -Last 40`
  - `Get-Content TASKS.md -TotalCount 40`
  - `Get-Content web/viewer/app/directory-view.tsx`
  - `Get-Content web/viewer/components/Navbar.tsx`
  - `Get-Content web/viewer/styles/globals.css`
  - `Get-Content web/viewer/styles/home.css`
- Task 2 complete: simplified the shared viewer frame by trimming the navbar down to a clearer single-row hierarchy, removing the extra desktop context strip, tightening CTA/search sizing, and refreshing the following shell/sidebar presentation so the current page reads like one product surface instead of separate panes.
- Task 2 checks:
  - `npm.cmd --prefix web/viewer run test -- navbar.test.tsx viewerShell.test.tsx`
- Task 3 complete: rebuilt the homepage into one stronger discovery flow with a more decisive hero, a dedicated featured panel, a cleaner recommended/categories/trending/live/directory section order, and clearer desktop-first balance between the main content and persistent following sidebar.
- Task 3 checks:
  - `npm.cmd --prefix web/viewer run test -- directoryPage.test.tsx`
- Task 4 complete: standardized the touched discovery components around shared panel/state/search treatments, aligned card/button/input sizing, refreshed empty/loading/error messaging, and updated featured/directory card actions so the page feels intentional instead of pieced together.
- Task 4 checks:
  - `npm.cmd --prefix web/viewer run test -- channelDisplayPrimitives.test.tsx -u`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
- Task 5 complete: reran the focused homepage/navigation tests plus lint/build successfully and attempted the repo viewer verification gate; `./scripts/verify.sh --viewer` stopped at the existing host blocker where `python3` is unavailable on this Windows machine, so the verify path was invoked and recorded but not completed end-to-end here.
- Task 5 checks:
  - `npm.cmd --prefix web/viewer run test -- navbar.test.tsx viewerShell.test.tsx directoryPage.test.tsx channelDisplayPrimitives.test.tsx -u`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh --viewer` (blocked at the env-placeholder hygiene step because `python3` is not installed/callable on this host)

## Scoped change: root README banner polish

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Record the docs-only plan for the README refresh
  - Acceptance criteria:
    - `PLAN.md` captures the README banner scope, assumptions, risks, and verification commands.
    - `TASKS.md` lists a small implementation and verification sequence before editing `README.md`.

- [x] Task 2 — Add the banner image and tighten the README opening copy
  - Acceptance criteria:
    - `README.md` includes `bitriver-live-banner-text.png` near the top of the file.
    - The opening copy is slightly sharper for public readers without changing technical meaning.
    - The image reference uses a valid repo-relative path.

- [x] Task 3 — Verify the README asset path and record the result
  - Acceptance criteria:
    - `Test-Path bitriver-live-banner-text.png` confirms the banner asset exists.
    - `Get-Content README.md -TotalCount 20` shows the new banner reference near the top.
    - `./scripts/verify.sh` is run and recorded.

### Execution log (root README banner polish)
- ✅ Task 1 complete: captured the docs-only README scope in `PLAN.md` and queued a single README implementation task plus verification in `TASKS.md`.
- ✅ Task 1 checks:
  - ✅ `Get-Item bitriver-live-banner-text.png | Select-Object Name,Length,LastWriteTime`
  - ✅ `Get-Content README.md -TotalCount 120`
- ✅ Task 2 complete: added the new banner directly under the `README.md` title and refreshed the opening two paragraphs so the project pitch reads a little more cleanly for public visitors without changing any technical claims.
- ✅ Task 2 checks:
  - ✅ `Get-Content README.md -TotalCount 20`
- ✅ Task 3 complete: confirmed the banner asset path exists, verified the new banner reference appears at the top of `README.md`, and reran the repository verification gate successfully.
- ✅ Task 3 checks:
  - ✅ `Test-Path bitriver-live-banner-text.png`
  - ✅ `Get-Content README.md -TotalCount 20`
  - ✅ `./scripts/verify.sh`

## Scoped change: postgres-tagged `puddle/v2` build fix

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Record the reproduced failure and scoped fix plan
  - Acceptance criteria:
    - `PLAN.md` captures the postgres-tagged `puddle/v2` build failure, assumptions, risks, and verification commands.
    - `TASKS.md` lists a small implementation and verification sequence before code edits.

- [x] Task 2 — Make the local `puddle/v2` replacement buildable under `-tags postgres`
  - Acceptance criteria:
    - `third_party/github.com/jackc/puddle/v2` contains at least one buildable Go file when the `postgres` tag is enabled.
    - Default non-postgres builds remain unaffected.
    - The diff stays narrowly scoped to the vendored package.

- [x] Task 3 — Verify the reproduced failure is resolved
  - Acceptance criteria:
    - `go test -tags postgres ./internal/auth -count=1 -timeout=120s` passes.
    - `go test ./internal/auth -count=1 -timeout=120s` passes.
    - `./scripts/verify.sh` is run and recorded.

### Execution log (postgres-tagged `puddle/v2` build fix)
- ✅ Task 1 complete: confirmed that `github.com/jackc/puddle/v2` is locally replaced to `./third_party/github.com/jackc/puddle/v2`, reproduced the exact failure with `go test -tags postgres ./internal/auth`, and narrowed the issue to `third_party/github.com/jackc/puddle/v2/puddle.go` being gated behind `//go:build !postgres`.
- ✅ Task 1 checks:
  - ✅ `Get-Content go.mod -TotalCount 200`
  - ✅ `Get-ChildItem -Recurse third_party\\github.com\\jackc\\puddle\\v2 | Select-Object FullName,Length,LastWriteTime`
  - ✅ `Get-Content third_party\\github.com\\jackc\\puddle\\v2\\puddle.go -TotalCount 80`
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test -tags postgres ./internal/auth -count=1 -timeout=120s`
- ✅ Task 2 complete: removed the `!postgres` build constraint from `third_party/github.com/jackc/puddle/v2/puddle.go`, which keeps the existing local replacement visible to both default and postgres-tagged builds without changing dependency sourcing.
- ✅ Task 2 checks:
  - ✅ `Get-Content third_party\\github.com\\jackc\\puddle\\v2\\puddle.go -TotalCount 80`
- ✅ Task 3 complete: reran the reproduced postgres-tagged auth build, confirmed the default auth tests still pass, and reran the repo verification gate. An additional postgres-tagged storage run no longer hits the `puddle` compile error and instead stops at a Docker access blocker on this host.
- ✅ Task 3 checks:
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test -tags postgres ./internal/auth -count=1 -timeout=120s`
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/auth -count=1 -timeout=120s`
  - ⚠️ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test -tags postgres ./internal/storage -count=1 -timeout=120s` (compile proceeds past `puddle`; current blocker is Docker access for postgres integration tests on this host)
  - ✅ `./scripts/verify.sh`

## Scoped change: cleanup audit and `internal/executil` task 1

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Audit hotspots and write `docs/cleanup-plan.md`
  - Acceptance criteria:
    - `docs/cleanup-plan.md` captures the top brittle/inefficient hotspots with evidence, risk, proposed fixes, and verification notes.
    - `docs/cleanup-plan.md` lists 5–8 ordered cleanup tasks.

- [x] Task 2 — Execute `docs/cleanup-plan.md` task 1 in `internal/executil`
  - Acceptance criteria:
    - `internal/executil/executil_test.go` no longer depends on POSIX shell tools.
    - Runtime `executil` behavior is unchanged.
    - `go test ./internal/executil -count=1 -timeout=120s` passes on this host.

- [x] Task 3 — Run verification and record outcomes
  - Acceptance criteria:
    - The targeted `internal/executil` test command is logged as passing.
    - `go test ./... -count=1 -timeout=120s` is rerun and any remaining unrelated failures are recorded.
    - `./scripts/verify.sh` is attempted and its result or host blocker is recorded.

### Execution log (cleanup audit and `internal/executil` task 1)
- ✅ Task 1 complete: audited the current brittle/inefficient hotspots, captured the top 10 findings in `docs/cleanup-plan.md`, and ordered 7 reviewable cleanup tasks with task 1 focused on `internal/executil`.
- ✅ Task 1 checks:
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./... -count=1 -timeout=120s` (fails today in `cmd/transcoder`, `internal/executil`, and `scripts`, which seeded the cleanup ordering)
- ✅ Task 2 complete: replaced the POSIX-only `internal/executil` test harness with a helper subprocess launched through the Go test binary, preserving the same `CommandError` assertions without relying on `sh`, `head`, or `/dev/zero`.
- ✅ Task 2 checks:
  - ✅ `gofmt -w internal/executil/executil_test.go`
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/executil -count=1 -timeout=120s`
- ✅ Task 3 complete: reran the broad verification commands after the `internal/executil` fix; the package dropped out of the failure list, `cmd/transcoder` and `scripts` still fail for their already-audited reasons, and `./scripts/verify.sh` completed successfully on this host.
- ✅ Task 3 checks:
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./... -count=1 -timeout=120s` (`internal/executil` now passes; remaining failures are limited to `cmd/transcoder` and `scripts`)
  - ✅ `./scripts/verify.sh`

## Scoped change: repo git hygiene and generated-artifact cleanup

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Update plan docs for repo hygiene cleanup
  - Acceptance criteria:
    - `PLAN.md` records the Git-hygiene scope, assumptions, risks, and verification commands.
    - `TASKS.md` contains an ordered cleanup checklist before repo files are edited.

- [x] Task 2 — Ignore generated local cache/build/test artifacts
  - Acceptance criteria:
    - `.gitignore` ignores `.gocache`, `.npm-cache`, `web/viewer/.next`, `web/viewer/playwright-report`, and `web/viewer/test-results`.
    - Ignore rules stay path-specific so tracked source paths are unaffected.

- [x] Task 3 — Stabilize `deploy/ome/Server.generated.xml` line endings
  - Acceptance criteria:
    - `.gitattributes` includes a targeted entry that keeps `deploy/ome/Server.generated.xml` normalized consistently.
    - The generated OME file can be restored without a content diff after verification runs.

- [x] Task 4 — Remove generated artifacts from the Git index
  - Acceptance criteria:
    - Generated cache/build/test directories are no longer staged.
    - Working-tree source/doc changes remain intact.

- [x] Task 5 — Verify development-friendly Git status and record the result
  - Acceptance criteria:
    - `git status --short` no longer floods with generated-artifact noise.
    - `TASKS.md` records the resulting clean/intended status set and any residual caveats.

### Execution log (repo git hygiene and generated-artifact cleanup)
- ✅ Task 1 complete: added a focused repo-hygiene scope to `PLAN.md` and `TASKS.md` covering generated-artifact ignores, OME XML line-ending stabilization, and Git index cleanup.
- ✅ Task 1 check:
  - ✅ `rg -n "repo git hygiene and generated-artifact cleanup|Git-hygiene scope|generated OME file" PLAN.md TASKS.md`
- ✅ Task 2 complete: added path-specific ignore rules for local Go/npm caches plus the viewer build and Playwright/Jest result directories so routine verification output no longer pollutes staging.
- ✅ Task 2 checks:
  - ✅ `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live check-ignore -v --no-index .gocache/example .npm-cache/example web/viewer/.next/example web/viewer/playwright-report/index.html web/viewer/test-results/.last-run.json`
  - ✅ `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live diff -- .gitignore`
- ✅ Task 3 complete: added a narrow `.gitattributes` policy for `deploy/ome/Server.generated.xml` and refreshed the worktree copy so the generated OME contract file no longer shows a false-positive content modification on Windows.
- ✅ Task 3 checks:
  - ✅ `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live restore --worktree deploy/ome/Server.generated.xml`
  - ✅ `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live diff --numstat -- deploy/ome/Server.generated.xml`
  - ✅ `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live status --short -- deploy/ome/Server.generated.xml .gitattributes`
- ✅ Task 4 complete: removed the generated cache/build/test directories from the Git index so local verification output no longer pollutes the staged change set.
- ✅ Task 4 checks:
  - ✅ `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live restore --staged .gocache .npm-cache web/viewer/.next web/viewer/playwright-report web/viewer/test-results`
  - ✅ `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live status --short`
  - ✅ `(git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live diff --cached --name-only -- .gocache .npm-cache web/viewer/.next web/viewer/playwright-report web/viewer/test-results | Measure-Object -Line).Lines`
- ✅ Task 5 complete: restaged the intentional repo-hygiene files and verified that Git status now shows only the intended source/doc changes, with generated output hidden by ignore policy.
- ✅ Task 5 checks:
  - ✅ `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live add .gitignore .gitattributes PLAN.md TASKS.md`
  - ✅ `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live status --short`
  - ✅ `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live diff --cached --name-only`
  - ✅ `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live check-ignore -v --no-index .gocache/example .npm-cache/example web/viewer/.next/example web/viewer/playwright-report/index.html web/viewer/test-results/.last-run.json`
- ℹ️ Resulting staged set:
  - `.gitattributes`
  - `.gitignore`
  - `PLAN.md`
  - `TASKS.md`
  - `web/viewer/__tests__/creatorLiveStreamStatus.test.ts`
  - `web/viewer/__tests__/directoryPage.test.tsx`
  - `web/viewer/app/browse/page.tsx`
  - `web/viewer/app/creator/live/[channelId]/page.tsx`
  - `web/viewer/app/creator/live/[channelId]/stream-status.ts`
  - `web/viewer/app/directory-page-shell.tsx`
  - `web/viewer/app/page.tsx`
  - `web/viewer/components/UploadManager.tsx`
  - `web/viewer/components/following/useFollowingChannels.ts`

## Scoped change: viewer route export release-build unblock

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Update plan docs for the creator live release blocker
  - Acceptance criteria:
    - `PLAN.md` records the viewer release-gate scope, assumptions, risks, and test plan.
    - `TASKS.md` contains an ordered implementation/check list before code changes begin.

- [x] Task 2 — Extract creator live stream-status logic out of the Next.js page module
  - Acceptance criteria:
    - `deriveControlCentreStatus` no longer exports from `web/viewer/app/creator/live/[channelId]/page.tsx`.
    - The creator live page imports the extracted helper from a non-page module.
    - Existing creator live UI behavior remains unchanged.

- [x] Task 3 — Keep targeted creator live status tests aligned with the extracted helper
  - Acceptance criteria:
    - `web/viewer/__tests__/creatorLiveStreamStatus.test.ts` imports the helper from its new module path.
    - Targeted unit coverage still validates the current live/offline/error status mapping.

- [x] Task 4 — Extract directory page shell/boundary helpers out of the Next.js home page module
  - Acceptance criteria:
    - `DirectoryPageShell` and `DirectoryDataBoundary` no longer export from `web/viewer/app/page.tsx`.
    - The default route page imports those helpers from a non-page module.
    - Existing directory/homepage behavior remains unchanged.

- [x] Task 5 — Keep directory page tests aligned with the extracted shell/boundary helpers
  - Acceptance criteria:
    - `web/viewer/__tests__/directoryPage.test.tsx` imports the extracted helper components from their new module path.
    - Existing directory page tests still cover the normalized shell + pending-boundary behavior.

- [x] Task 6 — Fix browse page query typing for build-safe directory loading
  - Acceptance criteria:
    - `web/viewer/app/browse/page.tsx` no longer passes `string | undefined` into `loadDirectoryChannels`.
    - Browse page behavior remains unchanged for empty and populated search queries.

- [x] Task 7 — Fix following hook channel identity typing for production builds
  - Acceptance criteria:
    - `web/viewer/components/following/useFollowingChannels.ts` compares `DirectoryChannel` entries using valid typed channel identifiers.
    - Existing no-op reload semantics remain unchanged.

- [x] Task 8 — Fix UploadManager optional metadata typing for production builds
  - Acceptance criteria:
    - `web/viewer/components/UploadManager.tsx` no longer calls `Object.keys` on possibly undefined metadata.
    - Existing upload payload behavior remains unchanged for empty and populated metadata.

- [x] Task 9 — Fix UploadManager playback URL typing for production builds
  - Acceptance criteria:
    - `web/viewer/components/UploadManager.tsx` treats optional `playbackUrl` values safely when computing playback availability.
    - Existing upload card action behavior remains unchanged.

- [-] Task 10 — Run release-facing viewer validation and record results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- creatorLiveStreamStatus.test.ts` passes.
    - `cd web/viewer && npm run test -- directoryPage.test.tsx` passes.
    - `cd web/viewer && npm run test -- browsePage.test.tsx` passes.
    - `cd web/viewer && npm run test -- followingSidebar.test.tsx` passes.
    - `cd web/viewer && npm run test -- uploadManager.test.tsx` passes.
    - `cd web/viewer && npm run build` passes.
    - `cd web/viewer && npm run test:playwright -- tests/creator-live-setup.spec.ts` is run and logged.
    - `./scripts/verify.sh --viewer` is run and logged.

### Execution log (viewer route export release-build unblock)
- ✅ Task 1 complete: updated `PLAN.md` and `TASKS.md` with a scoped release-readiness pass focused on the current viewer production-build blocker in the creator live page.
- ✅ Task 1 check:
  - ✅ `rg -n "creator live page release-build unblock|viewer release-blocking `next build` failure" PLAN.md TASKS.md`
- ✅ Task 2 complete: moved creator live stream-status labels/types/derivation into a sibling `stream-status.ts` module and rewired both the route page and unit test to import it, removing the named helper export from the Next.js page file.
- ✅ Task 2 check:
  - ✅ `rg -n "stream-status|export function deriveControlCentreStatus|export const CREATOR_STATUS_LABELS" web/viewer/app/creator/live/[channelId]/page.tsx web/viewer/app/creator/live/[channelId]/stream-status.ts web/viewer/__tests__/creatorLiveStreamStatus.test.ts`
- ✅ Task 3 complete: updated the targeted creator-live status unit test to import the extracted helper and confirmed the existing live/offline/error mappings still pass unchanged.
- ✅ Task 3 checks:
  - ❌ `npm --prefix web/viewer run test -- creatorLiveStreamStatus.test.ts` (PowerShell execution policy blocks `npm.ps1`; switched to `npm.cmd`)
  - ❌ `npm.cmd --prefix web/viewer run test -- creatorLiveStreamStatus.test.ts` (initially failed because viewer dependencies were not installed in this checkout)
  - ✅ `npm.cmd --prefix web/viewer ci --cache .npm-cache`
  - ✅ `npm.cmd --prefix web/viewer run test -- creatorLiveStreamStatus.test.ts`
- ✅ Task 4 complete: moved the directory page suspense shell and async data boundary out of `web/viewer/app/page.tsx` into `web/viewer/app/directory-page-shell.tsx`, leaving the route file with only its default page export.
- ✅ Task 4 checks:
  - ✅ `rg -n "DirectoryPageShell|DirectoryDataBoundary|directory-page-shell" web/viewer/app/page.tsx web/viewer/app/directory-page-shell.tsx web/viewer/__tests__/directoryPage.test.tsx`
- ✅ Task 5 complete: updated the directory page tests to import the extracted shell/boundary helpers from the new module path and confirmed the normalized-shell + suspense fallback coverage still passes.
- ✅ Task 5 checks:
  - ✅ `npm.cmd --prefix web/viewer run test -- directoryPage.test.tsx` (passes; emits pre-existing `act(...)` warnings from `SearchBar` interactions but no test failures)
- ✅ Task 6 complete: defaulted the browse page's optional `search` parameter before calling `loadDirectoryChannels`, preserving empty-query behavior while satisfying the production build type checker.
- ✅ Task 6 checks:
  - ✅ `rg -n 'search = ""|loadDirectoryChannels\(search\)' web/viewer/app/browse/page.tsx`
  - ✅ `npm.cmd --prefix web/viewer run test -- browsePage.test.tsx`
- ✅ Task 7 complete: corrected the following hook's channel identity comparison to use `DirectoryChannel.channel.id`, preserving the no-op reload optimization while satisfying TypeScript.
- ✅ Task 7 checks:
  - ✅ `rg -n 'channel\.id' web/viewer/components/following/useFollowingChannels.ts`
  - ✅ `npm.cmd --prefix web/viewer run test -- followingSidebar.test.tsx`
- ✅ Task 8 complete: guarded the upload payload metadata shape so `UploadManager` only calls `Object.keys` when metadata exists, preserving empty-metadata behavior while satisfying the build.
- ✅ Task 8 checks:
  - ✅ `rg -n 'metadata && Object\.keys\(metadata\)\.length > 0' web/viewer/components/UploadManager.tsx`
  - ✅ `npm.cmd --prefix web/viewer run test -- uploadManager.test.tsx`
- ✅ Task 9 complete: tightened upload-card playback URL detection to treat the optional `playbackUrl` as a boolean presence check, preserving card-action behavior while satisfying TypeScript.
- ✅ Task 9 checks:
  - ✅ `rg -n 'Boolean\(item\.playbackUrl\?\.trim\(\)\)' web/viewer/components/UploadManager.tsx`
  - ✅ `npm.cmd --prefix web/viewer run test -- uploadManager.test.tsx`
- ⚠️ Task 10 validation progress:
  - ✅ `npm.cmd --prefix web/viewer run test -- creatorLiveStreamStatus.test.ts`
  - ✅ `npm.cmd --prefix web/viewer run test -- directoryPage.test.tsx` (passes; emits pre-existing `act(...)` warnings from `SearchBar` interactions)
  - ✅ `npm.cmd --prefix web/viewer run test -- browsePage.test.tsx`
  - ✅ `npm.cmd --prefix web/viewer run test -- followingSidebar.test.tsx`
  - ✅ `npm.cmd --prefix web/viewer run test -- uploadManager.test.tsx`
  - ✅ `npm.cmd --prefix web/viewer run lint`
  - ✅ `npm.cmd --prefix web/viewer run build`
  - ❌ `npm.cmd --prefix web/viewer run test:playwright -- tests/creator-live-setup.spec.ts` (initially blocked because Playwright Chromium was not installed on this machine)
  - ✅ `npx.cmd playwright install chromium`
  - ✅ `npm.cmd --prefix web/viewer run test:playwright -- tests/creator-live-setup.spec.ts`
  - ❌ `./scripts/verify.sh --viewer` (wrapper blocked on this host: Git Bash crashes with MSYS signal-pipe error, WSL `bash.exe` returns `E_ACCESSDENIED`, and Python is not installed for the script's `python3` dependency)
  - ⚠️ Closest documented equivalent subset run directly in PowerShell instead:
  - ✅ `npm.cmd --prefix web/viewer run lint`
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/ingest -run TestRenderOMEConfigRequiresManagersAuth -count=1 -v`
  - ❌ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./... -count=1 -timeout=120s` (remaining blockers in `cmd/transcoder`, `internal/executil`, `scripts`, plus a likely cross-package race involving `internal/ingest`/OME generated config when the full suite runs concurrently)

## Scoped change: deployment validation and quickstart smoke on Windows host

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Update plan docs for deployment validation
  - Acceptance criteria:
    - `PLAN.md` records deployment-validation scope, assumptions, risks, and test plan for this host.
    - `TASKS.md` contains an ordered task list for the deployment check before runtime changes begin.

- [x] Task 2 — Validate local deployment prerequisites and compose rendering
  - Acceptance criteria:
    - Confirm which documented deployment entrypoint is usable on this host (`scripts/quickstart.ps1`, direct Go CLI, or Bash wrapper).
    - Run a compose render against canonical `deploy/docker-compose.yml` with an appropriate env source.
    - Record any prerequisite/tooling blockers or render failures in the execution log.

- [x] Task 3 — Run the canonical quickstart deployment flow and inspect readiness
  - Acceptance criteria:
    - Execute `cmd/bitriver quickstart` (via wrapper or direct Go entrypoint) against `deploy/docker-compose.yml`.
    - Confirm the stack reaches expected healthy/ready state, or capture the exact failure stage and logs.

- [x] Task 4 — Fix any repo issue blocking quickstart and rerun deployment validation
  - Acceptance criteria:
    - Any implementation change is limited to the concrete issue blocking the canonical deployment path.
    - Relevant targeted checks run after each fix and results are recorded before proceeding.

- [x] Task 5 — Run final verification and summarize deployment status
  - Acceptance criteria:
    - Run `./scripts/verify.sh` or the closest documented equivalent possible on this host.
    - Record what passed, what was skipped/blocked, and whether deployment is confirmed working.

### Execution log (deployment validation and quickstart smoke on Windows host)
- ✅ Task 1 complete: updated `PLAN.md` and `TASKS.md` with a deployment-validation scope focused on the canonical quickstart/Compose contract for this Windows environment.
- ✅ Task 1 check:
  - ✅ `rg -n "deployment validation and quickstart smoke on Windows host|Validate the canonical deployment path on this Windows host" PLAN.md TASKS.md`
- ✅ Task 2 complete: confirmed the PowerShell wrapper is the intended Windows quickstart entrypoint here, Bash wrappers are blocked in this environment, and the canonical deployment flow is currently stopped by a Windows compile error before Docker orchestration begins.
- ✅ Task 2 checks:
  - ❌ `docker compose --env-file deploy/.env.example -f deploy/docker-compose.yml config` (fails because `deploy/docker-compose.yml` still references the repo-root `.env` via `env_file`, and this checkout does not have one yet)
  - ❌ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE = (Resolve-Path .gocache).Path; powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 -ValidateOnly` (fails while compiling `cmd/bitriver`: `doctor.go` references Unix-only `syscall.Statfs` on Windows)
  - ❌ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE = (Resolve-Path .gocache).Path; go run ./cmd/bitriver quickstart --help` (same compile failure on Windows)
- ✅ Task 3 complete: executed the canonical quickstart path on Windows and traced the current runtime stop point to the host Docker service rather than additional BitRiver code failures.
- ✅ Task 3 checks:
  - ❌ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE = (Resolve-Path .gocache).Path; powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 --compose-file deploy/docker-compose.yml` (fails during `Doctor` because `docker version` cannot connect to `//./pipe/docker_engine`)
  - ❌ `docker version` (Docker CLI installed, but `com.docker.service` is stopped and the engine pipe is missing)
  - ❌ `Start-Service com.docker.service` (this session could not start the Docker Desktop service)
- ✅ Task 4 complete: fixed the Windows-specific repo blockers by moving the doctor disk probe behind OS-specific files and making `scripts/quickstart.ps1` propagate real CLI failures while still allowing the validate-only help path.
- ✅ Task 4 checks:
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE = (Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./cmd/bitriver -count=1`
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE = (Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./scripts -run TestPowerShellQuickstartPropagatesCliExitCodes -count=1`
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE = (Resolve-Path .gocache).Path; powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 -ValidateOnly`
  - ❌ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE = (Resolve-Path .gocache).Path; powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 --compose-file deploy/docker-compose.yml; exit $LASTEXITCODE` (now correctly returns non-zero when quickstart fails)
- ✅ Task 5 complete: ran the closest documented verification subset available on this host and recorded both deployment-contract passes and remaining blockers.
- ✅ Task 5 checks:
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE = (Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go run ./cmd/bitriver env init --env-file .env`
  - ❌ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE = (Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go run ./cmd/bitriver env validate --env-file .env` (expected strict `quickstart-prod` rejection until routable/public values replace generated defaults for `BITRIVER_TRANSCODER_PUBLIC_BASE_URL`, `BITRIVER_OME_BIND`, `BITRIVER_OME_IP`, and `NEXT_PUBLIC_VIEWER_URL`)
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE = (Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go run ./cmd/bitriver ome render --force --env-file .env --quiet`
  - ✅ `docker compose --env-file .env -f deploy/docker-compose.yml config`
  - ❌ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE = (Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./... -count=1 -timeout=120s` (host-specific failures observed in `cmd/transcoder`, `internal/executil`, and `scripts`)
  - ℹ️ Deployment status: the repo-side Windows quickstart blockers found during this run are fixed, the compose contract renders with a generated root `.env`, but end-to-end deployment is still not confirmed on this machine because Docker Desktop is stopped and strict production networking values still need operator-provided updates before `quickstart`/`env validate` can fully pass.

## Scoped change: navbar notifications coming-soon interactive affordance

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Replace disabled notifications icon button with interactive coming-soon affordance
  - Acceptance criteria:
    - `web/viewer/components/Navbar.tsx` no longer renders a permanently disabled notifications button.
    - Notifications quick-action remains keyboard focusable and opens a lightweight roadmap popover/toast.
    - Control has explicit screen-reader labeling for intent beyond icon semantics.

- [x] Task 2 — Implement dismiss and accessibility behavior for the notifications affordance
  - Acceptance criteria:
    - Popover/toast can be dismissed via Escape and outside click.
    - Open/closed state is exposed with appropriate ARIA attributes (`aria-expanded`, `aria-controls`, descriptive region labeling).

- [x] Task 3 — Update navbar tests for notifications affordance interactions
  - Acceptance criteria:
    - `web/viewer/__tests__/navbar.test.tsx` verifies button is enabled and toggles roadmap content.
    - Tests cover dismiss behavior (Escape and outside click).

- [x] Task 4 — Run scoped viewer tests and record results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- navbar.test.tsx` passes and is logged.

### Execution log (navbar notifications coming-soon interactive affordance)
- ✅ Task 1 complete: replaced the disabled notifications quick-action with an enabled button that toggles a lightweight roadmap popover and exposes explicit intent labeling for assistive tech.
- ✅ Task 1 check:
  - ✅ `rg -n "NOTIFICATIONS_POPOVER_ID|notificationsPopoverOpen|aria-label="Notifications roadmap details"|nav-notifications__popover" web/viewer/components/Navbar.tsx web/viewer/styles/globals.css`
- ✅ Task 2 complete: added Escape and outside-click dismissal for the notifications popover with focus return to the triggering button and ARIA state wiring (`aria-expanded` + `aria-controls`).
- ✅ Task 2 check:
  - ✅ `rg -n "handleEscapeKey|handleOutsideInteraction|notificationsButtonRef|notificationsPopoverRef" web/viewer/components/Navbar.tsx`
- ✅ Task 3 complete: replaced disabled-control assertions with interactive notifications popover tests, including open/toggle semantics and dismiss paths.
- ✅ Task 3 check:
  - ✅ `rg -n "notifications roadmap popover|notifications feature roadmap|toHaveFocus|toBeEnabled" web/viewer/__tests__/navbar.test.tsx`
- ✅ Task 4 checks:
  - ✅ `cd web/viewer && npm run test -- navbar.test.tsx`
  - ❌ `./scripts/verify.sh` (fails due to pre-existing unrelated test failure: `internal/auth` `TestValidateHonorsAbsoluteTTL`)

## Scoped change: navbar search visible keyboard focus treatment

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add strong theme-aware navbar search focus styling in globals CSS
  - Acceptance criteria:
    - `web/viewer/styles/globals.css` defines focus-ring variables in both `:root` and `[data-theme="light"]`.
    - `.nav-search` gets a clear high-contrast `:focus-within` treatment that keeps existing visual style.
    - Focus treatment applies for both `.nav-search--inline` and `.nav-search--drawer` via shared/base selector coverage.

- [x] Task 2 — Add/adjust navbar test coverage for keyboard focus visibility contract
  - Acceptance criteria:
    - `web/viewer/__tests__/navbar.test.tsx` includes keyboard-focus coverage for navbar search path.
    - Test asserts style contract for visible focus treatment is present (focus selector + theme variables).

- [x] Task 3 — Run scoped viewer tests and record results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- navbar.test.tsx` passes and is logged.


### Execution log (navbar search visible keyboard focus treatment)
- ✅ Task 1 complete: added theme-aware navbar search focus tokens for dark/light themes and a shared `.nav-search:focus-within` high-contrast ring/border treatment that applies to both inline and drawer variants.
- ✅ Task 1 check:
  - ✅ `rg -n "navbar-search-focus-ring|navbar-search-focus-border|\.nav-search:focus-within|nav-search__input:focus-visible" web/viewer/styles/globals.css`
- ✅ Task 2 complete: added navbar search keyboard-focus path coverage and CSS focus-visibility contract assertions in viewer tests.
- ✅ Task 2 check:
  - ✅ `rg -n "keyboard focus path available|visible nav-search focus-within style contract|readFileSync|resolve\(" web/viewer/__tests__/navbar.test.tsx`
- ✅ Task 3 checks:
  - ❌ `cd web/viewer && npm run test -- navbar.test.tsx` (initial attempt failed due to a brittle tab-order assumption in the new test; corrected by directly focusing the intended search inputs for deterministic assertions)
  - ✅ `cd web/viewer && npm run test -- navbar.test.tsx`
  - ✅ `./scripts/verify.sh` (passed; Docker-dependent checks skipped because Docker is not installed in this environment)

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

## Scoped change: channel page tab roving-focus keyboard navigation

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Implement roving-focus + keyboard tab navigation in channel page
  - Acceptance criteria:
    - `web/viewer/app/channels/[id]/page.tsx` sets `tabIndex=0` for active tab and `tabIndex=-1` for inactive tabs.
    - Tab triggers handle `ArrowLeft`/`ArrowRight` (and `ArrowUp`/`ArrowDown`), `Home`, and `End` to move focus and switch active tab.
    - `aria-selected` state remains synchronized with focused/active tab selection.

- [x] Task 2 — Add/update channel page tests for keyboard-only tab traversal
  - Acceptance criteria:
    - `web/viewer/__tests__/channelPage.test.tsx` includes coverage proving keyboard navigation across About/Schedule/Videos tabs.
    - Test asserts active tab and visible `tabpanel` stay in sync after keyboard navigation.

- [x] Task 3 — Run scoped viewer checks and required verify gate
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- channelPage.test.tsx` passes.
    - `./scripts/verify.sh` is run and result logged.

### Execution log (channel page tab roving-focus keyboard navigation)
- ✅ Task 1 complete: added tab roving-focus semantics and keyboard handling (`ArrowLeft`/`ArrowRight`/`ArrowUp`/`ArrowDown`, `Home`, `End`) while preserving existing ARIA tab-to-tabpanel wiring.
- ✅ Task 1 check:
  - ✅ `cd web/viewer && npm run lint -- --file app/channels/[id]/page.tsx`
- ✅ Task 2 complete: extended `channelPage.test.tsx` with keyboard-only navigation coverage asserting focus movement, active tab selection, and panel visibility synchronization.
- ✅ Task 2 check:
  - ✅ `cd web/viewer && npm run test -- channelPage.test.tsx`
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- channelPage.test.tsx`
  - ✅ `./scripts/verify.sh`


## Scoped change: durable viewer theme preference in navbar

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Refactor navbar theme initialization/persistence flow
  - Acceptance criteria:
    - `web/viewer/components/Navbar.tsx` reads `viewer-theme` from `localStorage` on client mount.
    - Stored preference is prioritized over `prefers-color-scheme`; media query is fallback-only.
    - Theme toggle writes the new value to `localStorage` and body `data-theme` sync remains deterministic in one place.

- [x] Task 2 — Add navbar theme persistence component coverage
  - Acceptance criteria:
    - `web/viewer/__tests__/navbar.test.tsx` covers initial load with stored preference.
    - `web/viewer/__tests__/navbar.test.tsx` covers initial load without stored preference.
    - `web/viewer/__tests__/navbar.test.tsx` covers manual toggle persistence across remount.

- [x] Task 3 — Run scoped viewer validation and record results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- navbar.test.tsx` passes and is logged.
    - `./scripts/verify.sh` is run and result logged.

### Execution log (durable viewer theme preference in navbar)
- ✅ Task 1 complete: navbar theme bootstrapping now prioritizes persisted `viewer-theme` preference, falls back to `prefers-color-scheme` only when unset, gates media-query change handling to non-explicit users, and keeps body `data-theme` sync centralized in one effect.
- ✅ Task 1 check:
  - ✅ `cd web/viewer && npm run test -- navbar.test.tsx`
- ✅ Task 2 complete: added navbar tests for stored preference initialization, media-query fallback initialization, and manual-toggle persistence across remount with ignored media-query updates after explicit selection.
- ✅ Task 2 check:
  - ✅ `cd web/viewer && npm run test -- navbar.test.tsx`
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- navbar.test.tsx`
  - ✅ `./scripts/verify.sh`

## Scoped change: channel page tab URL-state synchronization

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Implement URL-backed tab state in channel page
  - Acceptance criteria:
    - `web/viewer/app/channels/[id]/page.tsx` derives a validated tab id from URL state (`?tab=` and/or hash fallback) using `CHANNEL_TABS`.
    - Initial render hydrates `activeTab` from URL when present; invalid values fall back to `about`.
    - Tab-switch actions update URL state through router/history integration with expected back/forward behavior and no keyboard-regression to tablist controls.

- [x] Task 2 — Add channel page URL-state tab tests
  - Acceptance criteria:
    - `web/viewer/__tests__/channelPage.test.tsx` covers loading from deep-linked tab.
    - Coverage asserts tab switching updates URL state.
    - Coverage asserts browser navigation restores prior tab selection.

- [x] Task 3 — Run scoped checks and required gate
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- channelPage.test.tsx` passes.
    - `./scripts/verify.sh` is run and result logged.

### Execution log (channel page tab URL-state synchronization)
- ✅ Task 1 complete: channel tab state now validates URL tab values from query param (with hash fallback), initializes active tab from URL state, syncs tab changes to router query updates, and preserves existing ARIA keyboard tablist behavior.
- ✅ Task 2 complete: added tests for deep-linked tab initialization, URL updates on tab switch, and browser back navigation restoring the prior tab.
- ✅ Task 3 checks:
  - ✅ `cd web/viewer && npm run test -- channelPage.test.tsx`
  - ✅ `./scripts/verify.sh` (Docker-dependent checks skipped because Docker is unavailable in this environment)

## Scoped change: creator-first guided go-live flow

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the scoped creator live plan before implementation
  - Acceptance criteria:
    - `PLAN.md` captures the guided go-live scope, assumptions, risks, and verification commands.
    - `TASKS.md` lists ordered implementation and verification tasks before viewer code is edited.

- [x] Task 2 - Refine creator live status helpers for guided test-stream messaging
  - Acceptance criteria:
    - Existing creator-live helper logic supports the requested status-card states and calm instruction copy using current playback/session signals only.
    - `web/viewer/__tests__/creatorLiveStreamStatus.test.ts` covers the main state mappings used by the new guided flow.

- [x] Task 3 - Reshape the creator live page into the requested top-to-bottom flow
  - Acceptance criteria:
    - `web/viewer/app/creator/live/[channelId]/page.tsx` renders the sections in this order: Channel, OBS Setup, Test Stream, Preview, Share.
    - The page clearly shows the active channel and supports switching when multiple channels exist.
    - OBS setup exposes copyable read-only ingest URL + masked stream key with reveal/hide, copy URL/key, and copy OBS settings actions.
    - Test Stream polls existing signals every 3-5 seconds, shows last checked time, and presents calm state-specific guidance.
    - Preview and Share sections explain preview readiness clearly and provide copy/open viewer affordances.

- [x] Task 4 - Add happy-path coverage for setup through first successful live signal
  - Acceptance criteria:
    - Playwright coverage in `web/viewer/tests/creator-live-setup.spec.ts` exercises the guided flow sections and the main copy/reveal/live-preview/share happy path.
    - Supporting unit coverage in `web/viewer/__tests__/creatorLivePage.test.tsx` or equivalent asserts the guided section order and key affordances.

- [x] Task 5 - Run scoped viewer validation and record results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- creatorLiveStreamStatus.test.ts` passes.
    - `cd web/viewer && npm run test -- creatorLivePage.test.tsx` passes.
    - `cd web/viewer && npm run test:playwright -- tests/creator-live-setup.spec.ts` is run and logged.
    - `./scripts/verify.sh --viewer` is run and logged.

### Execution log (creator-first guided go-live flow)
- ✅ Task 1 complete: appended the scoped creator-live plan and ordered implementation/verification tasks to `PLAN.md` and `TASKS.md` before touching viewer code.
- ✅ Task 1 check:
  - ✅ `rg -n -e 'creator-first guided go-live flow' -e 'Test Stream polls existing signals every 3-5 seconds' TASKS.md`
  - ✅ `rg -n -e 'creator-first guided \"Go Live\" flow' -e 'Copy affordances for stream key, ingest URL, OBS settings, and viewer link' PLAN.md`
- ✅ Task 2 complete: simplified the creator-live status helper around the requested setup states (`Waiting for stream`, `Live`, `Reconnecting`, `Offline / Unknown`) and added calm instruction copy driven by the existing playback/session signals only.
- ✅ Task 2 check:
  - ✅ `npm.cmd --prefix web/viewer run test -- creatorLiveStreamStatus.test.ts` (passes; Jest also reports a pre-existing `.next/standalone/package.json` haste-name collision warning in this checkout)
- ✅ Task 3 complete: replaced the old two-column control-room layout with a single guided stack in the requested order, kept channel switching/title editing intact, added a visible OBS settings block plus share link actions, and wired the preview to the existing live/liveState signals so "preview warming up" is explained without new APIs.
- ✅ Task 3 check:
  - ✅ `npm.cmd --prefix web/viewer run build` (passes; Next.js also reports pre-existing client-render deopt warnings for several routes in this checkout)
- ✅ Task 4 complete: added `creatorLivePage.test.tsx` to assert the guided section order and key affordances, extended the shared viewer API Jest mocks for creator-live page coverage, and upgraded the existing Playwright happy path to cover setup, live-status transition, preview messaging, and sharing.
- ✅ Task 4 checks:
  - ✅ `npm.cmd --prefix web/viewer run test -- creatorLivePage.test.tsx`
  - ✅ `npm.cmd --prefix web/viewer run test:playwright -- tests/creator-live-setup.spec.ts`
- ✅ Task 5 complete: reran the targeted creator-live Jest coverage after the last test edits and executed the repo viewer verification gate.
- ✅ Task 5 checks:
  - ✅ `npm.cmd --prefix web/viewer run test -- creatorLiveStreamStatus.test.ts creatorLivePage.test.tsx` (passes; `creatorLivePage.test.tsx` still emits React `act(...)` warnings from async polling/state-reset effects in the test harness)
  - ✅ `npm.cmd --prefix web/viewer run test:playwright -- tests/creator-live-setup.spec.ts` (passes; Playwright logs pre-existing Next.js client-render deopt warnings plus the existing `next start` standalone warning)
  - ✅ `./scripts/verify.sh --viewer`

## Scoped change: creator share-link basePath fix

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the scoped base-path regression plan before implementation
  - Acceptance criteria:
    - `PLAN.md` captures the share-link base-path regression, assumptions, risks, and verification commands.
    - `TASKS.md` lists the ordered implementation and validation tasks before viewer code is edited.

- [x] Task 2 - Centralize viewer-link construction with base-path awareness
  - Acceptance criteria:
    - Viewer URL construction includes the active Next.js base path instead of hard-coding root-relative `/channels/...`.
    - The creator live Share step uses the shared base-path-aware logic for both copy and open actions.

- [x] Task 3 - Reuse the fix in getting-started and add regression coverage
  - Acceptance criteria:
    - The creator getting-started viewer-link copy flow also uses the shared base-path-aware viewer URL logic.
    - Viewer tests cover creator-live and/or creator-getting-started URL construction under a non-root viewer mount such as `/viewer`.

- [x] Task 4 - Run scoped viewer validation and record results
  - Acceptance criteria:
    - `cd web/viewer && npm run test -- creatorLivePage.test.tsx` passes.
    - `cd web/viewer && npm run test -- creatorGettingStartedPage.test.tsx` passes.
    - `cd web/viewer && npm run build` passes.
    - `./scripts/verify.sh --viewer` is run and logged.

### Execution log (creator share-link basePath fix)
- ✅ Task 1 complete: appended the scoped share-link base-path fix plan and ordered implementation/validation tasks before editing viewer code.
- ✅ Task 1 checks:
  - ✅ `rg -n -e 'creator share-link basePath fix' -e 'Centralize viewer-link construction with base-path awareness' TASKS.md`
  - ✅ `rg -n -e 'Fix creator-facing viewer/share links so they include the active Next.js basePath' -e 'The active viewer base path can be derived safely on the client' PLAN.md`
- ✅ Task 2 complete: added a shared `viewer-links` helper that prefixes viewer URLs and paths with the configured Next.js base path, exposed that base path to client code via `NEXT_PUBLIC_VIEWER_BASE_PATH`, and rewired the creator-live Share step to use the helper for both copy and open actions.
- ✅ Task 2 checks:
  - ✅ `npm.cmd --prefix web/viewer run test -- creatorLivePage.test.tsx creatorGettingStartedPage.test.tsx viewerLinks.test.ts`
- ✅ Task 3 complete: reused the same base-path-aware viewer-link helper in creator getting-started and added regression coverage for both page flows and the helper itself under a `/viewer` mount.
- ✅ Task 3 checks:
  - ✅ `npm.cmd --prefix web/viewer run test -- creatorLivePage.test.tsx creatorGettingStartedPage.test.tsx viewerLinks.test.ts` (passes; Jest still logs the pre-existing React `act(...)` warnings from creator-live/getting-started async effects)
- ✅ Task 4 complete: reran the scoped viewer tests and production build successfully, then ran the required `verify.sh --viewer` gate; it entered the repo checks and stopped on this Windows host because `python3` is not installed, which blocks the existing env-placeholder step before viewer checks.
- ✅ Task 4 checks:
  - ✅ `npm.cmd --prefix web/viewer run test -- creatorLivePage.test.tsx creatorGettingStartedPage.test.tsx viewerLinks.test.ts`
  - ✅ `npm.cmd --prefix web/viewer run build`
  - ⚠️ `bash ./scripts/verify.sh --viewer` (sandbox/WSL host failure: access denied and path translation failure)
  - ⚠️ `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh --viewer` (ran outside sandbox, then failed on host prerequisite: `python3` missing during the existing env example placeholder hygiene step)
## Scoped change: real local shakedown deployment on this Windows host

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the scoped shakedown plan before deployment
  - Acceptance criteria:
    - `PLAN.md` captures the local shakedown scope, assumptions, risks, and verification commands.
    - `TASKS.md` lists the ordered deployment-validation tasks before runtime actions begin.

- [x] Task 2 - Confirm host prerequisites and deployment inputs
  - Acceptance criteria:
    - Docker availability/state is checked on this host and recorded.
    - The canonical compose contract renders with the current repo-root `.env`, or the exact render failure is captured.
    - The deployment entrypoint used for the shakedown is recorded.

- [-] Task 3 - Run the real quickstart deployment and inspect readiness
  - Acceptance criteria:
    - The canonical quickstart flow is executed against `deploy/docker-compose.yml` and `.env`.
    - Post-run service state is inspected with `docker compose ps` and relevant logs.
    - Basic runtime reachability is checked when the stack starts far enough to do so.

- [-] Task 4 - Fix repo-side deployment blockers if any are found
  - Acceptance criteria:
    - Any repo defect surfaced by the shakedown is fixed with the smallest practical change.
    - The affected deployment checks are rerun after the fix.
    - Pure host/operator blockers are documented clearly if no repo change can address them.

- [ ] Task 5 - Run closing verification and record the final issue list
  - Acceptance criteria:
    - Relevant verification commands are rerun and logged after the shakedown/fixes.
    - `TASKS.md` records the final deployment outcome, including unresolved issues and whether the stack actually reached a healthy running state on this machine.

### Execution log (real local shakedown deployment on this Windows host)
- ✅ Task 1 complete: appended a fresh shakedown-deployment scope to `PLAN.md` and an ordered host-validation checklist to `TASKS.md` before running any runtime actions.
- ✅ Task 1 checks:
  - ✅ `rg -n "real local shakedown deployment on this Windows host|Run a real local shakedown deployment of BitRiver Live" PLAN.md TASKS.md`
  - ✅ `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live diff -- PLAN.md TASKS.md`
- ✅ Task 2 complete: confirmed the current repo-root `.env` renders the canonical compose contract, the PowerShell quickstart wrapper is the intended Windows entrypoint, and Docker daemon access requires elevated execution on this host.
- ✅ Task 2 checks:
  - ⚠️ `docker version` (sandboxed shell can see the Docker client but daemon access is denied with "must be run with elevated privileges" and `C:\Users\RhythmicCarnage\.docker\config.json` access warnings)
  - ✅ `docker compose --env-file .env -f deploy/docker-compose.yml config`
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 -ValidateOnly`
  - ✅ `rg -n "^(BITRIVER_LIVE_MODE|NEXT_PUBLIC_VIEWER_URL|BITRIVER_TRANSCODER_PUBLIC_BASE_URL|BITRIVER_OME_BIND|BITRIVER_OME_IP)=" .env`
- ⚠️ Task 3 in progress: the first elevated quickstart attempt reached the Windows wrapper and failed before any containers were created because `scripts/quickstart.ps1` calls `Start-Process -FilePath 'go'` after clearing process `PATH`, which prevented `go.exe` from being found in that context.
- ⚠️ Task 3 checks so far:
  - ❌ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 --env-file .env --compose-file deploy/docker-compose.yml; docker compose --env-file .env -f deploy/docker-compose.yml ps; docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120` (`Start-Process` could not find `go`; `docker compose ps` showed no containers)
- ✅ Task 4 partial progress: fixed the first repo-side Windows wrapper blocker in `scripts/quickstart.ps1` by resolving `go.exe` explicitly and normalizing duplicate `Path`/`PATH` keys before `Start-Process`, then added regression coverage for the wrapper and release Dockerfile pgx replacement handling.
- ✅ Task 4 checks so far:
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./scripts -run 'TestPowerShellQuickstartPropagatesCliExitCodes|TestDockerfileDropsStubbedPuddleInRealPgxMode' -count=1 -timeout=120s`
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 -ValidateOnly`
- ⚠️ Task 3 deeper shakedown result: the direct Go quickstart with elevated Docker access proved the canonical path still stops before container startup because the saved `.env` is production-mode and contains non-routable local placeholders for `BITRIVER_TRANSCODER_PUBLIC_BASE_URL`, `NEXT_PUBLIC_VIEWER_URL`, `BITRIVER_OME_BIND`, and `BITRIVER_OME_IP`.
- ⚠️ Additional shakedown finding: the documented quickstart-dev shell override (`BITRIVER_LIVE_MODE=development`) did not bypass the quickstart production contract in practice; direct quickstart still evaluated `.env` as production and rejected `--image-source build`, so the docs/behavior need follow-up.
- ✅ Task 4 additional progress: fixing the direct Compose bring-up exposed and resolved three more repo-side release-build blockers before runtime startup:
  - Dockerfile real-pgx mode now drops the stub `github.com/jackc/puddle/v2` replacement.
  - `go.mod` now requires the upstream-available `github.com/jackc/puddle/v2 v2.2.2`.
  - Postgres storage now detects stubbed pgx via the portable `pgx.ErrNoRows` heuristic instead of the stub-only `pgx.IsStub` symbol.
- ✅ Task 4 additional checks:
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test -tags postgres ./internal/auth -count=1 -timeout=120s`
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/storage -run TestNewPostgresRepositoryStubbedIncludesCause -count=1 -timeout=120s`
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test -tags postgres ./internal/storage -run '^$' -count=1 -timeout=120s`
- ⚠️ Task 3 current stop point: a full direct `docker compose up -d --build` with the documented development-mode override now builds the API, viewer, transcoder, SRS controller, and OME helper images successfully and starts part of the stack, but deployment still fails because:
  - `bitriver-postgres` becomes unhealthy with `chmod ... Operation not permitted` / `find ... Permission denied`.
  - `deploy-redis-1` crash-loops with `failed switching to "redis": operation not permitted`.
  - `bitriver-transcoder-public` crash-loops because nginx cannot `chown("/var/cache/nginx/client_temp", 101)`.
- ⚠️ Task 3 latest elevated inspection confirms the partial stack shape behind that failure:
  - Only `bitriver-srs` and `bitriver-transcoder` are healthy and reachable from Compose.
  - `bitriver-live`, `bitriver-viewer`, `bitriver-ome`, `bitriver-srs-controller`, and `deploy-postgres-migrations-1` remain stuck in `Created` because their dependencies never become healthy.
  - Host probes to `http://localhost:8080/healthz` and `http://localhost:8080/viewer` currently fail because nothing is listening on `BITRIVER_LIVE_PORT` yet.
- ⚠️ Current assessment: these remaining failures point at compose-level hardening on vendor images (`cap_drop: ["ALL"]`, plus related startup privilege assumptions), which means the next fix would change `deploy/docker-compose.yml` and must be confirmed with the user before proceeding.
- ⚠️ Task 3 additional checks so far:
  - ✅ `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - ✅ `docker compose --env-file .env -f deploy/docker-compose.yml ps -a`
  - ✅ `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 postgres redis transcoder-public`
  - ⚠️ `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/healthz -TimeoutSec 10 } catch { $_.Exception.Message }` (returns "Unable to connect to the remote server" because the API container never starts)
  - ⚠️ `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer -TimeoutSec 10 } catch { $_.Exception.Message }` (returns "Unable to connect to the remote server" because the API proxy/viewer path never binds)

## Scoped change: viewer-first public entry and explicit admin route

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the scoped public-entry UX plan before implementation
  - Acceptance criteria:
    - `PLAN.md` captures the viewer-first root-entry scope, assumptions, risks, and verification commands.
    - `TASKS.md` lists the ordered route/auth/navbar implementation tasks before runtime code is edited.

- [x] Task 2 - Move the public root flow to the viewer and expose the control center at `/admin`
  - Acceptance criteria:
    - When a viewer origin is configured, requests to `/` redirect to `/viewer`.
    - The control-center SPA remains reachable from `/admin` without changing its existing internal behavior.
    - `internal/server` coverage validates the new root/admin behavior and preserves the existing signup asset handling.

- [x] Task 3 - Make the auth screen sign-in-first when self-signup is disabled
  - Acceptance criteria:
    - The signup page no longer presents self-signup as the default primary action when `AllowSelfSignup` is false.
    - A public auth-config path or equivalent wiring lets the static auth page react to `AllowSelfSignup` safely.
    - Existing sign-in and MFA flows remain unchanged.

- [x] Task 4 - Add a simple admin entrypoint in the viewer UI for signed-in admins
  - Acceptance criteria:
    - Signed-in admins can reach `/admin` from the viewer navigation without typing the URL manually.
    - Non-admin viewer users do not see the new control-center entrypoint.
    - Viewer tests cover the new admin affordance.

- [x] Task 5 - Update docs for the new public and admin entry paths
  - Acceptance criteria:
    - Operator-facing docs explain that the public root now lands in the viewer flow when the viewer proxy is configured.
    - Docs point administrators to `/admin` for the control center.
    - The docs update stays narrowly scoped to the workflow/entrypoint guidance impacted by this route change.

- [x] Task 6 - Run targeted verification and record final results
  - Acceptance criteria:
    - `go test ./internal/server -count=1 -timeout=120s` passes.
    - `npm.cmd --prefix web/viewer run test -- navbar.test.tsx` passes.
    - `./scripts/verify.sh` is run and logged.

### Execution log (viewer-first public entry and explicit admin route)
- ✅ Task 1 complete: appended the scoped public-entry UX plan and ordered implementation/verification tasks to `PLAN.md` and `TASKS.md` before editing runtime code.
- ✅ Task 1 checks:
  - ✅ `rg -n "viewer-first public entry|explicit admin route|public-entry UX" PLAN.md TASKS.md`
- ✅ Task 2 complete: updated the HTTP server so `/` redirects into `/viewer` when the viewer proxy is configured, preserved the old root control-center fallback for non-viewer deployments, and mounted the existing control-center SPA explicitly at `/admin`.
- ✅ Task 2 checks:
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
- ✅ Task 3 complete: reworked the auth page into a sign-in-first layout, hid self-signup by default, and used server-rendered config on `/signup` so the page only reveals the create-account card when `AllowSelfSignup` is enabled.
- ✅ Task 3 checks:
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
- ✅ Task 4 complete: removed the fake viewer-local admin dashboard item from the shared viewer nav config and replaced it with a real same-origin `Control center` entry at `/admin` in the navbar, drawer, and account menu for signed-in admins only.
- ✅ Task 4 checks:
  - ✅ `npm.cmd --prefix web/viewer run test -- navbar.test.tsx navigation.test.ts` (passes; jsdom still logs the expected "navigation not implemented" console warning when the mobile drawer test clicks a real anchor)
- ✅ Task 5 complete: updated the README, quickstart guide, and Ubuntu install guide so operators now see the viewer-first root behavior and the explicit `/admin` control-center path in the same places they already use for bootstrap guidance.
- ✅ Task 5 checks:
  - ✅ `rg -n "/admin|viewer flow|public root|root now lands|control center" README.md docs/quickstart.md docs/installing-on-ubuntu.md`
- ✅ Task 6 complete: reran the focused server and viewer checks successfully, then ran the repo verification entrypoint and recorded the current host blocker precisely.
- ✅ Task 6 checks:
  - ✅ `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
  - ✅ `npm.cmd --prefix web/viewer run test -- navbar.test.tsx navigation.test.ts`
  - ⚠️ `bash ./scripts/verify.sh` (sandboxed Bash blocked on this Windows host with `Bash/Service/CreateInstance/E_ACCESSDENIED`)
  - ⚠️ `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh` (ran outside the sandbox and failed on the existing host prerequisite: `python3` missing during the env-example placeholder hygiene step)

## Scoped change: rebuild local stack onto the merged viewer-first routing changes

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the scoped rebuild/validation plan before runtime actions
  - Acceptance criteria:
    - `PLAN.md` captures the rebuild scope, assumptions, risks, and verification commands.
    - `TASKS.md` lists the ordered runtime rebuild and post-restart validation steps before Docker actions begin.

- [x] Task 2 - Rebuild the local API/viewer services from the merged source tree
  - Acceptance criteria:
    - Compose config confirms the expected services are present.
    - `bitriver-live` and `viewer` are recreated from the local build context rather than continuing to serve the older pulled release containers.
    - Service status/log output is recorded after the rebuild attempt.

- [x] Task 3 - Verify the live root/admin/auth behavior after the rebuild
  - Acceptance criteria:
    - `http://localhost:8080/` no longer serves the old control-center root when the merged API is active.
    - `http://localhost:8080/admin` serves the control center explicitly.
    - `http://localhost:8080/signup` reflects the new sign-in-first page and/or clearly reports the intentional self-signup-disabled state.
    - Any remaining auth denial is attributed precisely to config versus runtime bug.

### Execution log (rebuild local stack onto the merged viewer-first routing changes)
- ✅ Task 1 complete: captured the rebuild-specific runtime scope in `PLAN.md` and queued the local compose rebuild plus post-restart route/auth checks here before touching the running stack.
- ✅ Task 1 checks:
  - ✅ `git status --short`
  - ✅ `rg -n "rebuild the running local BitRiver Live stack|Rebuild the local API/viewer services from the merged source tree" PLAN.md TASKS.md`
- ✅ Task 2 complete: confirmed the compose service list, built fresh local images for the API/viewer stack, then forced a no-deps recreate of `bitriver-live` and `viewer` after the first `up -d --build` run hit a Windows Docker/Compose failure before it actually replaced the running containers.
- ✅ Task 2 checks:
  - ✅ `docker compose --env-file .env -f deploy/docker-compose.yml config --services`
  - ⚠️ `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer` (local images built successfully, but Docker Compose exited with a Windows-side stack trace before the old containers were replaced)
  - ✅ `docker compose --env-file .env -f deploy/docker-compose.yml up -d --force-recreate --no-deps bitriver-live viewer`
  - ✅ `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - ✅ `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 bitriver-live viewer`
- ✅ Task 3 complete: verified that the rebuilt API now redirects `/` to `/viewer`, keeps the control center explicitly at `/admin`, serves the new sign-in-first `/signup` page with `data-allow-self-signup="false"`, and still rejects public signup POSTs with the intentional `signup_disabled` response from the current `.env`.
- ✅ Task 3 checks:
  - ✅ `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0 -ErrorAction Stop } catch { $_.Exception.Response }`
  - ✅ `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/admin -MaximumRedirection 0 -ErrorAction Stop } catch { $_.Exception.Response }`
  - ✅ `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/signup -MaximumRedirection 0 -ErrorAction Stop } catch { $_.Exception.Response }`
- ✅ `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/admin).Content | Select-String -Pattern "BitRiver Live Control Center|Control Center|Control centre"`
- ✅ `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/signup).Content | Select-String -Pattern "Sign in to continue|Back to viewer|data-allow-self-signup"`
- ✅ `Invoke-RestMethod http://localhost:8080/api/auth/signup -Method Post -ContentType 'application/json' -Body ...` (`{"error":{"code":"signup_disabled","message":"public self-signup is disabled"}}`)

## Scoped change: platform-grade viewer UX/system redesign

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the scoped redesign plan and ordered implementation work before editing
  - Acceptance criteria:
    - `PLAN.md` captures redesign scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered shell/discovery/detail validation tasks before viewer code changes begin.
    - A quick read-only diagnosis identifies the product journeys and the main UX fragmentation points driving the redesign.

- [x] Task 2 - Rebuild the shared viewer shell, navigation, and design-token foundation
  - Acceptance criteria:
    - Global navigation communicates primary destinations and role-aware actions clearly on desktop and mobile.
    - Shared shell/layout classes replace the most fragile one-off scaffolding for top-level viewer pages.
    - Broken or missing style tokens referenced by the viewer are normalized so touched surfaces render consistently.
    - Navbar/shell/navigation tests are updated for the new structure and still pass.

- [x] Task 3 - Unify the public discovery experience across Home, Browse, and Following
  - Acceptance criteria:
    - Home feels like an editorial discovery surface, Browse feels like the full searchable directory, and Following has a clearer recovery/next-step flow.
    - Shared card, section-heading, search/filter, and state patterns are reused instead of diverging by page.
    - Empty/loading/error states remain functional and more intentional across the public discovery routes.
    - Discovery-surface tests are updated and pass.

- [x] Task 4 - Refactor channel, creator, and profile surfaces onto consistent page patterns
  - Acceptance criteria:
    - Channel detail, creator onboarding/live/uploads, and profile management all use clearer hierarchy and stronger next-step guidance.
    - Inline layout/styling is reduced materially on touched pages in favor of shared classes/components.
    - Existing creator/profile/channel logic continues to work, with any UI bugs fixed as part of the refactor.
    - Touched detail/creator/profile tests are updated and pass.

- [x] Task 5 - Run viewer validation and record the final results plus remaining UX follow-ups
  - Acceptance criteria:
    - Viewer lint and test commands are run and logged.
    - The required repo verification path for viewer work is run and logged.
    - Remaining product ambiguities, tradeoffs, or follow-up improvements are captured in the execution log for handoff.

- Task 5 complete: reran lint plus the full viewer Jest suite after the last channel/hero fixes, attempted the documented `./scripts/verify.sh --viewer` gate, and recorded the remaining host-environment blockers plus the highest-priority UX follow-ups for handoff.
- Task 5 checks:
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run test -- channelHero.test.tsx`
  - `npm.cmd --prefix web/viewer run test`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh --viewer` was attempted but stopped at the env-placeholder hygiene step because `python3` is not callable on this Windows host.
  - Follow-up attempts to re-run the bash-based verify flow were blocked by Git Bash returning `couldn't create signal pipe, Win32 error 5`, so the required viewer verify path could be invoked but not completed on this machine.
  - `docker compose --env-file .env -f deploy/docker-compose.yml config`
  - `$env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./... -count=1 -timeout=120s` initially failed because the default Windows Go build cache path was access-denied.
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./... -count=1 -timeout=120s` progressed but still failed in existing non-viewer areas (`cmd/transcoder`, `scripts`) due host-specific ffmpeg/WSL/bash assumptions unrelated to the viewer redesign.
- Remaining UX/product follow-ups captured during validation:
  - The creator live/getting-started and profile tests still emit non-failing React `act(...)` warnings from async refresh/reset behavior; the product behavior works, but the harness should be tightened.
  - Notifications are now intentionally removed as a dead-end shell affordance, but a real inbox/alerts product decision is still needed before that surface returns.
  - Channel schedule remains a placeholder state because the current product contract does not expose structured schedule data; once the backend model exists, that tab should become a true programming/calendar surface.

## Scoped change: sync to merged main and redeploy locally for manual viewer QA

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the sync/deploy plan before switching branches or touching the running stack
  - Acceptance criteria:
    - `PLAN.md` captures the merged-main sync scope, assumptions, risks, and local deployment commands.
    - `TASKS.md` lists the ordered sync, rebuild, and smoke-check tasks before runtime actions begin.

- [x] Task 2 - Sync the local checkout onto merged `main`
  - Acceptance criteria:
    - Local refs are refreshed from `origin`.
    - The checkout is moved to local `main` and fast-forwarded to `origin/main`.
    - The resulting local HEAD is logged so the deployed source revision is unambiguous.

- [x] Task 3 - Rebuild the local stack from the merged source and verify inspection entrypoints
  - Acceptance criteria:
    - Compose status is checked before and after the rebuild.
    - The local stack is recreated from the merged source tree.
    - Logs plus HTTP checks confirm the viewer/API routes are reachable for manual inspection.

### Execution log (sync to merged main and redeploy locally for manual viewer QA)
- Task 1 complete: recorded the merged-main sync and local redeploy scope in `PLAN.md`/`TASKS.md` before switching branches or touching the local runtime.
- Task 1 checks:
  - `git status --short --branch`
  - `git fetch origin --prune`
  - `rg -n "sync to merged main and redeploy locally for manual viewer QA|Sync the local checkout to the merged|Record the sync/deploy plan" PLAN.md TASKS.md`
- Task 2 complete: refreshed refs from `origin`, switched the checkout from the older feature branch onto local `main`, fast-forwarded `main` to `origin/main`, and reapplied the scoped workflow-note stash so the deployment run still stays documented on the merged branch.
- Task 2 checks:
  - `git branch --show-current`
  - `git checkout main`
  - `git pull --ff-only origin main`
  - `git log --oneline --decorate -n 3`
  - `git status --short --branch`
- Task 3 complete: rebuilt the local runtime from merged `main`, confirmed fresh `bitriver-live`/`viewer` containers are serving on localhost, and verified the main inspection entrypoints are reachable for manual QA.
- Task 3 checks:
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `docker compose --env-file .env -f deploy/docker-compose.yml config --services`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 bitriver-live viewer`
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0 -ErrorAction Stop } catch { $_.Exception.Response }`
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer).Content | Select-String -Pattern "Browse|Following|Creator|Profile|BitRiver" -AllMatches`
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/admin -MaximumRedirection 0 -ErrorAction Stop } catch { $_.Exception.Response }`
- Task 3 notes:
  - `http://localhost:8080/` returns `307 Temporary Redirect` to `/viewer`.
  - `http://localhost:8080/viewer` serves the redesigned shell/home experience from the rebuilt viewer container.
  - `http://localhost:8080/admin` is reachable and still serves the control center.
  - The local home page currently surfaces `We couldn't load the directory right now: Failed to parse URL from ""/api/directory`, so manual QA can proceed, but that issue is already visible on the freshly deployed merged build.

## Scoped change: fix viewer-shell interaction dead ends, desktop overflow, and mobile responsiveness

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the scoped UX-fix plan before editing
  - Acceptance criteria:
    - `PLAN.md` captures the auth-entry, sidebar/layout, mobile, and server-fetch scope plus risks/tests.
    - `TASKS.md` lists the ordered implementation/validation tasks before viewer files change.
    - The read-only diagnosis captures the concrete problems from manual QA: dead-end auth entry, pseudo-sidebar behavior, horizontal overflow, and mobile-readiness gaps.

- [x] Task 2 - Make homepage auth/discovery entry points behave like real product flows
  - Acceptance criteria:
    - Top-level signed-out entry points route to the actual auth surface instead of dead or mismatched paths.
    - Following/guest prompts use the same auth entry contract.
    - The home directory/discovery surface no longer fails on local server render because of a missing API base.

- [x] Task 3 - Rework the shell and home layout so desktop behaves like a real two-column app and small screens stay usable
  - Acceptance criteria:
    - The following region renders as a real sidebar on large screens and as a modal/drawer only when space is limited.
    - The home page no longer overflows horizontally on a standard 1080p desktop viewport.
    - Navbar/shell/home spacing and breakpoints are tightened so critical controls remain reachable on mobile.

- [x] Task 4 - Run focused viewer validation, redeploy locally, and record inspection notes
  - Acceptance criteria:
    - Relevant viewer tests/lint/build commands are run and logged.
    - The local `bitriver-live` and `viewer` services are rebuilt from the updated source.
    - Post-redeploy HTTP checks record the local URLs plus any remaining inspection caveats.

### Execution log (fix viewer-shell interaction dead ends, desktop overflow, and mobile responsiveness)
- Task 1 complete: recorded the scoped feedback pass in `PLAN.md`/`TASKS.md` before editing and confirmed the concrete QA issues in code: auth fallbacks still point at `/login` while the real public auth surface is `/signup`, the home page is server-failing on `"/api/directory"` with no absolute base, the home hero still uses a `100vw` breakout that can spill horizontally, and the shell/sidebar breakpoint behavior leaves the following panel feeling more like an overlap than a stable app column.
- Task 1 checks:
  - `Get-Content -Path 'web/viewer/components/Navbar.tsx' | Select-Object -First 260`
  - `Get-Content -Path 'web/viewer/components/ViewerShell.tsx' | Select-Object -First 260`
  - `Get-Content -Path 'web/viewer/styles/globals.css' | Select-Object -Skip 2980 -First 440`
  - `Get-Content -Path 'web/viewer/styles/home.css' | Select-Object -First 320`
  - `Invoke-RestMethod http://localhost:8080/api/viewer/me -Method Get` (captured `401 missing session token`)
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/signup).Content | Select-String -Pattern "Sign in to continue|Create account|data-allow-self-signup" -AllMatches`
- Task 2 complete: redirected signed-out viewer entry points onto the real `/signup` auth surface, aligned guest/following prompts with the same contract, restored server-rendered discovery fetches by resolving relative API requests against the internal API origin, and added auth-surface focus behavior so sign-in and join land on the correct section instead of feeling dead.
- Task 2 checks:
  - `npm.cmd --prefix web/viewer run test -- navbar.test.tsx useAuth.test.tsx followingSidebar.test.tsx viewer-api.test.ts`
  - `rg -n "/login|/signup|login-form|signup-card|bitriver-live:8080" web/viewer/components web/viewer/hooks web/viewer/lib web/static web/viewer/__tests__`
- Task 3 complete: tightened the active shell/home CSS so the viewer shell is width-safe, lowered the desktop sidebar breakpoint so large-but-constrained desktop windows get a real sidebar, removed the home hero breakout that was forcing horizontal overflow, and improved navbar/search/shell spacing for smaller screens.
- Task 3 checks:
  - `npm.cmd --prefix web/viewer run test -- viewerShell.test.tsx directoryPage.test.tsx navbar.test.tsx`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
- Task 4 complete: reran the focused viewer/unit checks, passed lint and production build, rebuilt the local `bitriver-live` and `viewer` services, and confirmed the rebuilt `/viewer` HTML no longer contains the old parse-URL directory error while `/signup` now exposes the real sign-in-first auth surface with explicit section targeting for sign-in vs join.
- Task 4 checks:
  - `npm.cmd --prefix web/viewer run test -- navbar.test.tsx useAuth.test.tsx followingSidebar.test.tsx viewerShell.test.tsx directoryPage.test.tsx viewer-api.test.ts`
  - `npm.cmd --prefix web/viewer run test -- viewerShell.test.tsx directoryPage.test.tsx navbar.test.tsx`
  - `npm.cmd --prefix web/viewer run test -- viewer-api.test.ts navbar.test.tsx useAuth.test.tsx`
  - `npm.cmd --prefix web/viewer run test`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 bitriver-live viewer`
  - `Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0`
  - `Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer`
  - `Invoke-WebRequest -UseBasicParsing http://localhost:8080/signup`
- Task 4 notes:
  - `/` still redirects to `/viewer` as intended.
  - The rebuilt `/viewer` HTML contains the new shell/auth CTAs and no longer contains `Failed to parse URL` or the previous `Invalid URL` directory alert.
  - `/signup` contains both `login-form` and `signup-card`, but local self-signup is still disabled by the current deployment config (`data-allow-self-signup="false"`), so true account creation remains gated on an approved `.env` contract change.
  - The full viewer Jest suite now passes, with existing non-failing warnings still present from jsdom navigation limitations and older `act(...)` gaps in profile/creator/directory tests.

## Scoped change: viewer usability shakedown and polish

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the scoped usability-pass plan before editing
  - Acceptance criteria:
    - `PLAN.md` captures the usability-shakedown scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered shakedown, fix, and validation steps before additional viewer edits begin.
    - The read-only diagnosis identifies the primary journeys and the highest-likelihood usability friction points to validate.

- [x] Task 2 - Run a focused usability shakedown across the primary viewer journeys
  - Acceptance criteria:
    - The current behavior of root/viewer/signup plus the main discovery shell is checked via code inspection and local/runtime validation.
    - Concrete blockers are recorded in the execution log instead of working from vague UX goals.
    - The top-fix list stays small and tied to actual friction found during the shakedown.

- [x] Task 3 - Implement the smallest fixes needed to remove the confirmed usability blockers
  - Acceptance criteria:
    - High-friction actions remain actionable and route to the correct product surfaces.
    - Layout/shell changes keep desktop and mobile flows usable without reintroducing overflow or dead-end behavior.
    - Viewer tests are updated when the behavior contract changes.

- [x] Task 4 - Run focused validation and record the final usability status
  - Acceptance criteria:
    - The targeted viewer Jest, lint, and build commands are run and logged.
    - Relevant Playwright/local smoke checks are attempted and their outcomes are recorded.
    - Remaining gaps, if any, are described precisely for follow-up instead of hand-wavy “needs polish” notes.

### Execution log (viewer usability shakedown and polish)
- Task 1 complete: appended the scoped viewer-usability plan to `PLAN.md` and this task list before making additional viewer edits, using the current read-only diagnosis that the highest-risk friction remains around primary entry/discovery/auth flows and shared shell responsiveness.
- Task 2 complete: validated the current root/viewer/signup flow with targeted viewer tests plus live HTTP inspection and found one concrete release-grade blocker still visible in rendered HTML: the signed-out following CTA was rendering as `/viewer/signup#login-form` instead of the real auth surface at `/signup#login-form` because the shared prompt used a Next.js `Link` under the viewer `basePath`. The same sweep also surfaced a remaining separator-copy inconsistency in `FollowingRail`.
- Task 2 checks:
  - `npm.cmd --prefix web/viewer run test -- navbar.test.tsx useAuth.test.tsx followingSidebar.test.tsx viewerShell.test.tsx directoryPage.test.tsx viewer-api.test.ts`
  - `Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0`
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer).Content | Select-String -Pattern "/signup#login-form|/viewer/signup|Sign in to see channels you follow|Browse directory|Failed to parse URL|Invalid URL" -AllMatches`
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/signup).Content | Select-String -Pattern "Sign in to continue|Create account|data-allow-self-signup|login-form|signup-card" -AllMatches`
- Task 3 complete: switched the shared unauthenticated-following CTA to a plain anchor so auth handoff stays outside the viewer `basePath`, and normalized the following-rail category/live-state separator to a stable ASCII ` - ` string.
- Task 3 checks:
  - `Get-Content web/viewer/components/following/FollowingState.tsx`
  - `Get-Content web/viewer/components/FollowingRail.tsx`
- Task 4 complete: reran the focused viewer regression suite, lint, and production build successfully; attempted browser-level Playwright smoke twice but hit the current Windows `spawn EPERM` restriction; then rebuilt the live `bitriver-live` and `viewer` services and confirmed the real `/viewer` HTML now emits `/signup#login-form` rather than `/viewer/signup`.
- Task 4 checks:
  - `npm.cmd --prefix web/viewer run test -- followingStatePresentation.test.tsx followingSidebar.test.tsx directoryPage.test.tsx navbar.test.tsx useAuth.test.tsx viewer-api.test.ts`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
  - `npm.cmd --prefix web/viewer run test:playwright -- tests/accessibility.spec.ts tests/navbar-mobile.spec.ts` (blocked by Windows `spawn EPERM` during the package pretest build hook)
  - `npx.cmd playwright test tests/accessibility.spec.ts tests/navbar-mobile.spec.ts` (blocked by the same Windows `spawn EPERM` when Playwright tried to launch its configured process)
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer).Content | Select-String -Pattern "/signup#login-form|/viewer/signup|Sign in to see channels you follow|Browse directory|Failed to parse URL|Invalid URL" -AllMatches`
  - `Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0`
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/signup).Content | Select-String -Pattern "Sign in to continue|data-allow-self-signup|Back to viewer" -AllMatches`
- Remaining gaps noted during validation:
  - Browser-level Playwright smoke is still blocked on this Windows host by `spawn EPERM`, so accessibility/mobile assertions remain covered by existing code-level tests plus live HTTP inspection rather than a completed headless-browser pass.
  - `directoryPage.test.tsx` still emits pre-existing non-failing React `act(...)` warnings from `SearchBar` interactions, and `navbar.test.tsx` still emits the known jsdom “navigation not implemented” warning when real anchor navigation is exercised.

## Scoped change: local runtime refresh to latest checkout

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the scoped runtime-refresh plan before rebuilding services
  - Acceptance criteria:
    - `PLAN.md` captures the local runtime-refresh scope, assumptions, risks, and verification commands.
    - `TASKS.md` lists the ordered rebuild and verification steps before Docker actions begin.

- [x] Task 2 - Rebuild and recreate the local application services from the current checkout
  - Acceptance criteria:
    - `bitriver-live` and `viewer` are rebuilt from the current source tree using the canonical Compose contract.
    - Compose status/log output is recorded after the refresh attempt.
    - Healthy infrastructure services are left alone unless they are required dependencies of the rebuilt services.

- [x] Task 3 - Verify the live local routes reflect the refreshed services
  - Acceptance criteria:
    - `http://localhost:8080/` responds as expected after the rebuild.
    - `http://localhost:8080/viewer` and `http://localhost:8080/signup` load successfully after the refresh.
    - `TASKS.md` records the post-refresh status so a new contributor can see that the local instance was synced.

### Execution log (local runtime refresh to latest checkout)
- Task 1 complete: appended a focused local runtime-refresh scope to `PLAN.md` and `TASKS.md` before rebuilding services, using the clean checkout and currently healthy stack as the baseline.
- Task 2 complete: ran the canonical `docker compose ... up -d --build bitriver-live viewer` refresh from the current clean checkout, then force-recreated `bitriver-live` and `viewer` so the running app containers were restarted explicitly against the latest built images.
- Task 2 checks:
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --force-recreate --no-deps bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 bitriver-live viewer` (initial log snapshot succeeded during the rebuild pass)
  - `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=40 bitriver-live viewer` (follow-up retry hit a host-side Docker access-denied error against `//./pipe/docker_engine`)
- Task 3 complete: verified the refreshed containers are now newly created and healthy (`bitriver-live`/`viewer` both restarted within seconds of the check), confirmed `/` still returns `307` to the viewer flow, confirmed `/viewer` serves the latest viewer shell markup with the corrected `/signup#login-form` auth CTA, and confirmed `/signup` still serves the current sign-in-first auth page.
- Task 3 checks:
  - `Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0`
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer).Content | Select-String -Pattern 'BitRiver Live|/signup#login-form|/viewer/signup|Failed to parse URL|Invalid URL' -AllMatches`
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/signup).Content | Select-String -Pattern 'Sign in to continue|data-allow-self-signup|Back to viewer' -AllMatches`

### Execution log (platform-grade viewer UX/system redesign)
- ✅ Task 1 complete: appended the scoped viewer-platform redesign plan to `PLAN.md`/`TASKS.md` before editing and captured the read-only diagnosis that currently the product works as a self-hosted creator/viewer platform, but the UX is fragmented by duplicated discovery entry points, ad-hoc page scaffolding, heavy inline styling, and shell/state tokens that are missing or inconsistent.
- ✅ Task 1 findings:
  - ✅ Main user journeys found in code: public discovery (`/`, `/browse`, `/following`), channel watching (`/channels/[id]`), profile management (`/profile`), and creator setup/live/uploads (`/creator/*`).
  - ✅ Main fragmentation points found in code: duplicated Home vs Browse discovery patterns, overlay-first following navigation that obscures primary IA, creator/profile pages built with many one-off inline layouts, and several CSS variables referenced without definitions (`--surface-2`, `--surface-3`, `--success`, `--radius-lg`, `--shadow-md`, `--shadow-lg`, `--accent-500`, `--accent-600`, `--accent-soft`, `--space-3`).
- ✅ Task 1 checks:
  - ✅ `rg -n "platform-grade viewer UX/system redesign|viewer shell into a clearer platform-style experience|Record the scoped redesign plan" PLAN.md TASKS.md`
- ✅ Task 2 complete: rebuilt the viewer shell around a clearer platform frame with stronger desktop/mobile navigation, removed the dead-end notifications affordance, promoted creator access into a studio CTA, made the following rail persistent on desktop but modal on smaller screens, and normalized the missing token/utility foundation that several viewer surfaces were already referencing.
- ✅ Task 2 checks:
  - ✅ `npm.cmd --prefix web/viewer run test -- navbar.test.tsx viewerShell.test.tsx navigation.test.ts`
  - ⚠️ The navbar test run still emits the existing jsdom console warning for real-anchor navigation (`Not implemented: navigation`) when a drawer link is clicked, but the assertions now pass and the behavior is unchanged from earlier viewer tests.
- ✅ Task 3 complete: split discovery into clearer roles by turning Home into an editorial launch surface, Browse into a search-and-filter workspace, and Following into a recovery-focused live feed with summary cards, while keeping the same directory/follow data wiring underneath.
- ✅ Task 3 checks:
  - ✅ `npm.cmd --prefix web/viewer run test -- directoryPage.test.tsx browsePage.test.tsx followingStatePresentation.test.tsx channelDisplayPrimitives.test.tsx`
  - ⚠️ Focused discovery tests still emit existing non-failing warnings from the local test harness: mocked `next/image` props (`fill`/`priority`) and older SearchBar act warnings in directory-page tests.
- ✅ Task 4 complete: refactored the creator studio, onboarding, live control room, uploads manager, profile management, and channel detail surfaces onto a shared workspace/page system with stronger headers, summary cards, clearer next-step CTAs, fewer one-off inline layouts, and small UX fixes like removing duplicate or dead-end actions and cleaning up broken string rendering/state copy.
- ✅ Task 4 checks:
  - ✅ `npm.cmd --prefix web/viewer run test -- channelPage.test.tsx creatorGettingStartedPage.test.tsx creatorLivePage.test.tsx profilePage.test.tsx uploadManager.test.tsx`
  - ⚠️ The targeted suite passes, but the existing test harness still emits non-failing React `act(...)` warnings from async polling/state-reset effects in the creator getting-started/live pages.
## Scoped change: viewer control usability repair pass

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the control-repair scope and initial read-only diagnosis
  - Acceptance criteria:
    - `PLAN.md` captures the viewer control-usability scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered reproduce/fix/validate steps before viewer code is edited.
    - The initial diagnosis names at least the reported theme-toggle issue plus any additional confirmed or strongly suspected dead controls found during read-only inspection.

- [x] Task 2 - Reproduce the broken controls with a focused interaction audit
  - Acceptance criteria:
    - The current navbar/homepage/mobile-shell controls are checked with code inspection plus browser/test interaction where available.
    - Concrete broken or misleading controls are recorded in the execution log with enough specificity to fix them.
    - The confirmed fix list stays scoped to the highest-traffic usability blockers instead of drifting into broad redesign work.

- [x] Task 3 - Implement the smallest viewer changes needed to make the confirmed controls usable
  - Acceptance criteria:
    - Broken controls either perform a meaningful action, navigate somewhere real, or are reframed so they are no longer dead UI.
    - The light/dark toggle has observable, testable behavior instead of a no-op-feeling interaction.
    - Any touched homepage/discovery controls remain keyboard-accessible and consistent across desktop/mobile layouts.

- [x] Task 4 - Add or adjust coverage for the repaired controls
  - Acceptance criteria:
    - Jest and/or Playwright coverage asserts the repaired behavior for the theme toggle and any other fixed controls.
    - Tests focus on user-observable outcomes rather than implementation details.
    - New coverage is stable on this host as far as the environment allows.

- [x] Task 5 - Run focused validation and record the usability status
  - Acceptance criteria:
    - Targeted viewer tests and build validation are run and logged after the fixes.
    - Browser-level interaction checks are attempted and their outcome is recorded precisely, including any host blocker.
    - Remaining UI gaps, if any, are written down concretely for follow-up.

### Execution log (viewer control usability repair pass)
- Task 1 complete: appended the scoped control-repair plan to `PLAN.md` and `TASKS.md` before touching viewer code, with the initial read-only diagnosis that the reported theme toggle needs browser-level reproduction and the homepage category chips are already a confirmed dead control because they render as buttons with no action.
- Task 1 checks:
  - `Get-Content PLAN.md | Select-Object -Last 40`
  - `Get-Content TASKS.md -TotalCount 60`
  - `git status --short PLAN.md TASKS.md`
- Task 2 complete: reproduced the breakage with a real browser pass and a static control audit. The closed `#viewer-nav-menu` drawer is still rendered and intercepting pointer events on desktop, which blocks clicks on the theme toggle, sign-in button, and account menu even when the drawer is supposedly closed. The same audit confirmed the homepage `CategoryRail` chips are dead buttons with no `onClick`, route, or filter behavior at all.
- Task 2 findings:
  - `npx.cmd playwright test tests/channel.spec.ts tests/navbar-mobile.spec.ts --reporter=list` failed in the browser with `#viewer-nav-menu` intercepting pointer events for the theme toggle, sign-in, and account-menu clicks; the mobile nav hidden-state assertion also failed because the drawer remained visible despite `hidden`.
  - `web/viewer/styles/globals.css` contains a later `.nav-drawer { display: grid; }` rule that overrides the earlier closed-state `display: none`, which explains why the hidden drawer still occupies the page.
  - `web/viewer/components/CategoryRail.tsx` renders category controls as plain `<button type="button">` elements without any action, so they are guaranteed no-ops on the homepage today.
  - The older follow-button Playwright expectations in `tests/channel.spec.ts` also no longer match the current accessible label formatting (`Follow - 10 supporters` in the component versus the old middle-dot string in the spec), but that is test drift rather than the main usability blocker the user reported.
- Task 2 checks:
  - `npm.cmd --prefix web/viewer run test -- navbar.test.tsx directoryPage.test.tsx`
  - `npm.cmd --prefix web/viewer run build`
  - `npx.cmd playwright test tests/channel.spec.ts tests/navbar-mobile.spec.ts --reporter=list`
  - `Get-Content web/viewer/styles/globals.css | Select-Object -Skip 1568 -First 60`
  - `Get-Content web/viewer/styles/globals.css | Select-Object -Skip 3196 -First 70`
  - `Get-Content web/viewer/components/CategoryRail.tsx`
- Task 3 complete: added a global `[hidden] { display: none !important; }` rule so hidden UI cannot remain clickable/visible when later display rules try to override it, which restores real click access to the navbar theme toggle and other right-side controls. I also turned the homepage category chips into real browse links that carry the category name into the existing directory search flow instead of rendering inert buttons.
- Task 3 checks:
  - `git diff -- web/viewer/styles/globals.css web/viewer/components/CategoryRail.tsx`
  - `npm.cmd --prefix web/viewer run test -- navbar.test.tsx directoryPage.test.tsx`
  - `npm.cmd --prefix web/viewer run build`
  - `npx.cmd playwright test tests/navbar-mobile.spec.ts tests/channel.spec.ts --grep "collapses into a toggle on small viewports|theme toggle updates the rendered document" --reporter=list`
- Task 3 notes:
  - The rebuilt-browser theme-toggle Playwright check now passes, confirming the user-reported dead control was being blocked by the hidden drawer overlay rather than by the theme-state logic itself.
  - The focused mobile-navbar Playwright check progressed past the hidden/blocked state and now only fails on an outdated ambiguous locator (`Browse` versus `Browse all channels`), which is a test issue to clean up in Task 4 rather than a remaining product bug.
- Task 4 complete: updated the shared viewer test helper to mock the homepage recommendation/category endpoints explicitly, added a homepage unit assertion that category chips now link into `/browse?q=...`, and tightened the mobile Playwright navbar spec so it targets the drawer's `Browse` link precisely instead of colliding with the hero CTA copy.
- Task 4 checks:
  - `npm.cmd --prefix web/viewer run test -- directoryPage.test.tsx navbar.test.tsx`
  - `npx.cmd playwright test tests/navbar-mobile.spec.ts tests/channel.spec.ts --grep "collapses into a toggle on small viewports|theme toggle updates the rendered document" --reporter=list`
- Task 4 notes:
  - The Jest suite now covers the repaired category-chip behavior directly.
  - The focused Playwright suite now passes for both the mobile drawer interaction and the navbar theme toggle.
- Task 5 complete: reran the focused viewer validation suite, refreshed the local `bitriver-live` and `viewer` services, and confirmed `localhost:8080` is serving the rebuilt viewer shell. The only remaining validation blocker is the existing host setup issue where `./scripts/verify.sh --viewer` stops immediately because Python is not installed/aliased for the env-placeholder hygiene step on this Windows machine.
- Task 5 checks:
  - `npm.cmd --prefix web/viewer run test -- directoryPage.test.tsx navbar.test.tsx`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
  - `npx.cmd playwright test tests/navbar-mobile.spec.ts tests/channel.spec.ts --grep "collapses into a toggle on small viewports|theme toggle updates the rendered document" --reporter=list`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh --viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer).Content | Select-String -Pattern 'Switch to (light|dark) theme|Browse by category|BitRiver Live' -AllMatches`
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode,Headers } catch { $_.Exception.Response | Select-Object StatusCode,Headers }`
- Task 5 notes:
  - Focused Jest and Playwright coverage now pass for the repaired controls.
  - Viewer lint and build both pass.
  - The local app services were rebuilt and `http://localhost:8080/` still returns the expected `307` into the viewer flow while `/viewer` serves the updated shell markup.
  - `./scripts/verify.sh --viewer` remains blocked on this host by the missing Python executable before it reaches the viewer-specific checks.

## Scoped change: deploy current branch for viewer review

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the review-deploy scope before runtime actions
  - Acceptance criteria:
    - `PLAN.md` captures the deploy/verification scope, assumptions, risks, and commands.
    - `TASKS.md` lists the rebuild and deployed-toggle verification steps before Docker actions begin.

- [x] Task 2 - Rebuild and refresh the local app services from the current branch
  - Acceptance criteria:
    - `bitriver-live` and `viewer` are rebuilt from the current checkout using the canonical Compose contract.
    - Compose status confirms the refreshed app services are up after the rebuild.
    - The action stays scoped to the review deployment.

- [x] Task 3 - Verify the deployed light/dark toggle on the running app
  - Acceptance criteria:
    - The deployed app at `localhost:8080` serves the current viewer shell from this branch.
    - A focused browser check confirms the light/dark mode toggle works on the deployed instance.
    - `TASKS.md` records any host blocker precisely if deployed verification cannot complete.

### Execution log (deploy current branch for viewer review)
- Task 1 complete: appended a focused deploy-for-review scope to `PLAN.md` and `TASKS.md` before touching Docker, centered on rebuilding the current branch and proving the deployed light/dark toggle works on `localhost:8080`.
- Task 1 checks:
  - `Get-Content PLAN.md | Select-Object -Last 40`
  - `Get-Content TASKS.md | Select-Object -Last 60`
  - `git status --short`
- Task 2 complete: rebuilt the current checkout with `docker compose ... up -d --build bitriver-live viewer` and confirmed the review deployment is up. Compose status shows `bitriver-live` healthy on `0.0.0.0:8080` and the deployed `/viewer` HTML includes the current navbar shell with the `Switch to light theme` toggle button.
- Task 2 checks:
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer).Content | Select-String -Pattern 'Switch to (light|dark) theme|BitRiver Live' -AllMatches`
- Task 3 complete: the first deployed browser check failed and revealed the real blocker: proxied `/viewer` pages were not hydrating because the default server CSP blocked the inline Next.js bootstrap scripts. After the follow-up CSP fix and a targeted no-deps recreate of `bitriver-live`/`viewer`, the deployed app now hydrates correctly and the light/dark toggle updates the rendered document on `localhost:8080`.
- Task 3 checks:
  - `@'...playwright probe...'@ | node -` against `http://127.0.0.1:8080/`
  - `$env:PLAYWRIGHT_BASE_URL='http://127.0.0.1:8080'; npx.cmd playwright test tests/channel.spec.ts --grep "theme toggle updates the rendered document" --reporter=list`
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode,Headers } catch { $_.Exception.Response | Select-Object StatusCode,Headers }`

## Scoped change: viewer proxy CSP hydration fix

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the viewer-hydration blocker and scoped fix plan
  - Acceptance criteria:
    - `PLAN.md` captures the CSP/hydration scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered security-header, test, redeploy, and deployed-browser validation steps before code edits begin.

- [x] Task 2 - Adjust default security-header behavior so proxied viewer pages can hydrate
  - Acceptance criteria:
    - The default CSP applied to proxied `/viewer` responses permits the inline Next.js bootstrap scripts required for hydration.
    - Existing non-viewer routes keep the stricter current default unless explicitly configured otherwise.
    - The implementation remains compatible with existing security-header override flags/env vars.

- [x] Task 3 - Add regression coverage for viewer-path CSP behavior
  - Acceptance criteria:
    - `internal/server` tests assert the default viewer-path CSP behavior separately from the stricter non-viewer default.
    - Tests also cover that custom security-header overrides still take precedence when configured.

- [x] Task 4 - Update the security-header docs for proxied viewer defaults
  - Acceptance criteria:
    - `docs/advanced-deployments.md` describes the stricter default CSP for admin/API routes and the relaxed default applied to proxied viewer responses.
    - Existing flag/env override guidance remains accurate.

- [x] Task 5 - Rebuild the review deployment and prove the live theme toggle works
  - Acceptance criteria:
    - `bitriver-live` and `viewer` are rebuilt from the current checkout after the CSP fix.
    - A real browser check against `http://localhost:8080` confirms the deployed theme toggle changes document theme state.
    - `TASKS.md` records any remaining host blocker precisely if end-to-end deployed verification still cannot complete.

### Execution log (viewer proxy CSP hydration fix)
- Task 1 complete: the deployed `/viewer` HTML was current, but an escalated Playwright probe showed repeated browser console errors refusing inline scripts under the API's default `Content-Security-Policy`. Because those Next.js bootstrap scripts never execute, the proxied viewer does not hydrate and controls like the theme toggle remain inert on the live app even though the same component works in the isolated viewer test server.
- Task 1 checks:
  - `rg -n "Content-Security-Policy|script-src" internal/server cmd/server docs/advanced-deployments.md`
  - `Get-Content internal/server/security.go`
  - `Get-Content internal/server/security_headers_test.go`
  - `@'...playwright probe...'@ | node -` against `http://127.0.0.1:8080/`
- Task 2 complete: updated the security-header middleware so default CSP is now selected per request path. Proxied `/viewer` responses use a viewer-specific default that permits the inline Next.js bootstrap scripts needed for hydration, while non-viewer routes still receive the stricter existing default and explicit custom CSP overrides still pass through unchanged.
- Task 2 checks:
  - `git diff -- internal/server/security.go`
  - `gofmt -w internal/server/security.go internal/server/security_headers_test.go`
- Task 3 complete: added targeted middleware/server coverage for the new viewer-path default plus the custom-override path, then reran the focused `internal/server` suite successfully.
- Task 3 checks:
  - `git diff -- internal/server/security_headers_test.go`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
- Task 4 complete: updated `docs/advanced-deployments.md` so the default hardening section now distinguishes the stricter admin/API CSP from the proxied viewer default that allows the inline Next.js bootstrap required for hydration. The existing override flag/env guidance remains unchanged.
- Task 4 checks:
  - `rg -n "Proxied \`/viewer\` responses|Admin/API and other non-viewer routes" docs/advanced-deployments.md`
  - `Get-Content docs/advanced-deployments.md | Select-Object -Skip 336 -First 16`
- Task 5 complete: rebuilt the `bitriver-live` and `viewer` images from the current checkout, used a targeted `--force-recreate --no-deps` refresh to land the new images after Compose hit an unrelated container-reconciliation error elsewhere in the stack, and then reran real browser validation against `http://localhost:8080`. The deployed viewer now hydrates, the browser probe shows the theme toggle changing `body[data-theme]` plus `localStorage`, and the focused Playwright theme test passes against the live deployment.
- Task 5 checks:
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer` (build completed, then hit an unrelated container reconciliation error while touching non-target services)
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --force-recreate --no-deps bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer).Headers['Content-Security-Policy']`
  - `@'...playwright probe...'@ | node -` against `http://127.0.0.1:8080/`
  - `$env:PLAYWRIGHT_BASE_URL='http://127.0.0.1:8080'; npx.cmd playwright test tests/channel.spec.ts --grep "theme toggle updates the rendered document" --reporter=list`
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode,Headers } catch { $_.Exception.Response | Select-Object StatusCode,Headers }`

## Scoped change: exact category browse links

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the category-link bug and the scoped fix plan
  - Acceptance criteria:
    - `PLAN.md` captures the exact category-link scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered API/viewer/test steps before code edits begin.
    - The read-only diagnosis identifies both the misleading `?q=` category links and the current free-text directory search gap around `channel.category`.

- [x] Task 2 - Add category-aware directory handling on the API/storage path
  - Acceptance criteria:
    - `/api/directory` accepts a dedicated `category` query parameter for exact category filtering.
    - Free-text directory search also matches `channel.category` so existing search copy remains accurate.
    - The implementation stays backward-compatible for callers that only use `q` today.

- [x] Task 3 - Wire the viewer browse flow to the real category filter
  - Acceptance criteria:
    - Homepage category chips navigate with an exact category filter instead of `?q=...`.
    - The browse page preserves and resets `q`/`category` state coherently.
    - Existing browse search flows without a category continue to behave the same.

- [x] Task 4 - Add regression coverage for the API and viewer category flows
  - Acceptance criteria:
    - API tests assert exact category filtering and free-text category matching.
    - Viewer tests assert homepage category links use the dedicated param and the browse page loads the right API helper path for `?category=...`.

- [x] Task 5 - Run focused validation and record the result
  - Acceptance criteria:
    - Targeted Go and viewer tests pass.
    - Viewer production build passes.
    - The local `bitriver-live` and `viewer` services are refreshed so the fix is reviewable on the running app.
    - `TASKS.md` captures any remaining blocker precisely.

### Execution log (exact category browse links)
- Task 1 complete: confirmed the user-reported mismatch in read-only analysis. `CategoryRail` currently links to `/browse?q=<category>`, but `/api/directory` forwards only `q` into `ListChannels`, and the PostgreSQL search predicate currently matches title, owner display name, and tags only, not `c.category`. That means the homepage chips are currently semantically wrong even before considering result quality, and the browse/search copy is also overstating current search coverage.
- Task 1 checks:
  - `Get-Content internal/api/channels_directory_handlers.go`
  - `Get-Content internal/storage/postgres_channels.go`
  - `Get-Content web/viewer/components/CategoryRail.tsx`
  - `Get-Content web/viewer/app/browse/page.tsx`
  - `Get-Content web/viewer/hooks/useDirectorySearch.ts`
  - `Get-Content web/viewer/lib/directory-state.ts`
- Task 2 complete: added a dedicated `category` query path to the directory handler and exact-match category filtering there, while also extending shared free-text channel matching to include `channel.Category` in both the Postgres and in-memory storage paths. That keeps existing `q` callers backward-compatible while making category search true everywhere the current UI says it is.
- Task 2 checks:
  - `git diff -- internal/api/channels_directory_handlers.go internal/storage/postgres_channels.go internal/storage/storage_channel_helpers.go`
  - `gofmt -w internal/api/channels_directory_handlers.go internal/api/handlers_test.go internal/storage/storage_channel_helpers.go`
- Task 3 complete: rewired the homepage category rail to `/browse?category=...`, extended the viewer directory helpers to preserve both `q` and `category`, and updated the browse page so category-linked entries load the exact category view while reset/search flows keep the URL state coherent.
- Task 3 checks:
  - `git diff -- web/viewer/components/CategoryRail.tsx web/viewer/app/browse/page.tsx web/viewer/hooks/useDirectorySearch.ts web/viewer/lib/directory-state.ts web/viewer/lib/viewer-api-directory.ts`
- Task 4 complete: added API coverage for exact category filtering plus free-text category matching, updated the homepage category-link assertions to use the dedicated param, added browse-page coverage for `?category=Music`, and added viewer API request assertions for category-aware directory calls.
- Task 4 checks:
  - `npm.cmd --prefix web/viewer run test -- directoryPage.test.tsx browsePage.test.tsx viewer-api.test.ts`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/api -count=1 -timeout=120s`
- Task 5 complete: the targeted Go and viewer tests passed, the viewer production build passed, and the local `bitriver-live` plus `viewer` services were rebuilt from the current checkout. A live browser probe against `http://127.0.0.1:8080/viewer/browse?category=Music` confirmed the rebuilt route hydrates and requests `GET /api/directory?category=Music`; the local dataset simply does not have enough seeded category data to render a visible `Music` filter chip in that runtime check. I also updated `web/viewer/README.md` with the browse URL contract so the dedicated `category=` behavior is documented where viewer route semantics already live.
- Task 5 checks:
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/api -count=1 -timeout=120s`
  - `npm.cmd --prefix web/viewer run test -- directoryPage.test.tsx browsePage.test.tsx viewer-api.test.ts`
  - `npm.cmd --prefix web/viewer run build`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `@'...playwright browse probe...'@ | node -` against `http://127.0.0.1:8080/viewer/browse?category=Music`
  - `Get-Content web/viewer/README.md | Select-Object -Skip 68 -First 8`

## Scoped change: bootstrap admin access recovery UX

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the admin-access UX gap and recovery plan
  - Acceptance criteria:
    - `PLAN.md` captures the operator admin-access scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered CLI, messaging, docs, and validation steps before code edits begin.
    - The read-only diagnosis confirms where bootstrap admin creds are generated, persisted, and currently surfaced.

- [x] Task 2 - Add an operator-facing CLI command to recover bootstrap admin access details
  - Acceptance criteria:
    - The CLI can read the deployment `.env` and print the admin sign-in route plus bootstrap email on demand.
    - Password output is opt-in so routine usage does not dump secrets by default.
    - The output warns operators that env-backed bootstrap passwords may be stale if they already rotated the admin password later.

- [x] Task 3 - Improve quickstart/install/admin auth messaging
  - Acceptance criteria:
    - Quickstart success output calls out the `/admin` route and where bootstrap creds live.
    - Install output calls out the same recovery path instead of only flashing a generated password line.
    - The closed-signup auth page gives operators a concise pointer to the bootstrap admin path without exposing secrets.

- [x] Task 4 - Update docs and coverage for the new operator path
  - Acceptance criteria:
    - Tests cover the new CLI/admin summary behavior and the updated signup-copy contract.
    - Quickstart/install docs mention the new recovery flow in the right operator-facing places.

- [x] Task 5 - Run focused validation and record the result
  - Acceptance criteria:
    - Targeted `cmd/bitriver` and `internal/server` tests pass.
    - `TASKS.md` records any remaining host blocker precisely.

### Execution log (bootstrap admin access recovery UX)
- Task 1 complete: confirmed the current bootstrap flow does persist the admin credentials, but the operator handoff is easy to miss. `quickstart` prints generated secrets once and then a success summary that shows the admin email plus env file path, while `install systemd` writes `BITRIVER_LIVE_ADMIN_EMAIL` and `BITRIVER_LIVE_ADMIN_PASSWORD` into the staged `.env` and only prints the generated password once when it auto-created one. The public `/signup` page, meanwhile, only tells viewers to ask an administrator for access, which leaves operators with no obvious pointer back to the bootstrap admin account when self-signup is disabled.
- Task 1 checks:
  - `Get-Content cmd/bitriver/install.go`
  - `Get-Content cmd/bitriver/commands_env_compose.go`
  - `Get-Content docs/quickstart.md`
  - `Get-Content web/static/signup.js`
  - `Get-Content web/static/signup.html`
- Task 2 complete: added `env admin` to the BitRiver CLI so operators can reprint the bootstrap admin summary on demand from the deployment `.env`. The new subcommand derives the admin sign-in URL, prints the bootstrap email plus env file path, hides the password by default, reveals it only with `--show-password`, and always warns that the env-backed password may no longer match the live credential after a later rotation in `/admin`. I also wired the shared admin URL helper into quickstart summary generation so the CLI now has one canonical way to describe where operators should sign in.
- Task 2 checks:
  - `gofmt -w cmd/bitriver/main.go cmd/bitriver/commands_env_compose.go`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./cmd/bitriver -count=1 -timeout=120s`
- Task 3 complete: extended the quickstart success summary to call out the `/admin` sign-in URL, the env-backed bootstrap credential source, and the `env admin` recovery path; updated `install systemd` to print the same sign-in and recovery guidance instead of relying only on a one-time generated-password line; and refreshed the closed-signup auth copy so operators get a concise `/admin` + deployment `.env` pointer without exposing any secrets in the public UI.
- Task 3 checks:
  - `gofmt -w cmd/bitriver/install.go`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./cmd/bitriver -count=1 -timeout=120s`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
- Task 4 complete: added focused `cmd/bitriver` coverage for the new `env admin` recovery flow, the expanded quickstart summary, and the installer recovery messaging; added `internal/server` coverage for the new `/signup` operator hint contract; and updated the quickstart plus Ubuntu install docs to explain how to revisit bootstrap admin access later with the new CLI helper.
- Task 4 checks:
  - `gofmt -w cmd/bitriver/main_test.go cmd/bitriver/install_test.go internal/server/server_test.go`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./cmd/bitriver -count=1 -timeout=120s`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
- Task 5 complete: reran the focused `cmd/bitriver` and `internal/server` suites successfully and re-attempted the repo verification entrypoint. The targeted validation now passes for this scope; the only remaining blocker is unchanged from earlier work on this Windows host, where `./scripts/verify.sh` stops immediately at the env-example placeholder hygiene step because Python is not installed or aliased.
- Task 5 checks:
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./cmd/bitriver -count=1 -timeout=120s`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh` (blocked immediately at env example placeholder hygiene because Python is not installed/aliased on this host)

## Scoped change: viewer-auth UX cohesion

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the auth/viewer cohesion gap and scoped plan
  - Acceptance criteria:
    - `PLAN.md` captures the auth/viewer cohesion scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered auth-markup, styling, docs/tests, and runtime validation steps before edits begin.
    - The read-only diagnosis identifies why the current `/signup` page feels visually and structurally separate from `/viewer`.

- [x] Task 2 - Rebuild the auth page structure around viewer continuity
  - Acceptance criteria:
    - The `/signup` page presents clear viewer-connected branding, hierarchy, and return-path context instead of a standalone utility form layout.
    - Existing sign-in, sign-up, MFA, and self-signup-disabled behaviors remain intact.
    - The page more clearly communicates how sign-in returns the user to the viewer flow they came from.

- [x] Task 3 - Align auth styling and client-side context with the viewer experience
  - Acceptance criteria:
    - Static auth styling uses the same visual direction as the current viewer shell rather than the older static-admin look.
    - The page updates its “back” and “return after auth” cues from the safe `next` destination when present.
    - Mobile and desktop layouts both feel intentional and connected to the viewer product.

- [x] Task 4 - Add docs/coverage for the new auth cohesion contract
  - Acceptance criteria:
    - Tests assert the new viewer-connected auth markup/copy contract.
    - Relevant viewer-facing docs mention the auth landing contract where route/navigation behavior is already documented.

- [x] Task 5 - Refresh the local review deployment and validate the live route
  - Acceptance criteria:
    - The local `bitriver-live` service is rebuilt from the current checkout so `/signup` serves the updated auth experience.
    - Focused validation confirms the running auth page shows the new continuity cues and honors the `next`-based return link.
    - `TASKS.md` records any host/runtime blocker precisely.

### Execution log (viewer-auth UX cohesion)
- Task 1 complete: read-only analysis confirmed that the auth route is functionally wired into the viewer flow through `next` and the navbar CTAs, but the page itself still uses an older standalone static design system (`web/static/styles.css`) that does not resemble the current `/viewer` shell hierarchy, spacing, or visual direction. The existing page also underplays the return destination even though the JS already resolves a safe `next` path, which is a big part of why the sign-in/sign-up flow feels detached from the viewer journey.
- Task 1 checks:
  - `Get-Content web/static/styles.css`
  - `Get-Content web/static/signup.html`
  - `Get-Content web/static/signup.js`
  - `Get-Content web/viewer/styles/globals.css`
  - `Get-Content web/viewer/components/Navbar.tsx`
  - `Get-Content web/viewer/lib/auth-links.ts`
  - `Get-Content internal/server/server_test.go`
- Task 2 complete: rebuilt `web/static/signup.html` into a two-column auth landing that reads like a viewer continuation instead of a standalone form page. The page now has viewer-branded framing, a “return after auth” context panel, a clearer sign-in-first hierarchy, and the same sign-in/sign-up/MFA structure anchored inside one cohesive experience.
- Task 2 checks:
  - `Get-Content web/static/signup.html | Select-Object -First 140`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
- Task 3 complete: aligned the auth page styling and lightweight client-side context with the current viewer product language. `web/static/styles.css` now uses a viewer-like dark/teal shell, richer hierarchy, and responsive layout, while `web/static/signup.js` now rewrites return links plus the destination context card from the safe `next` route and hides the sign-up jump affordance when self-signup is disabled.
- Task 3 checks:
  - `Get-Content web/static/styles.css | Select-Object -First 320`
  - `Get-Content web/static/signup.js | Select-Object -First 180`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
- Task 4 complete: added targeted server coverage for the new viewer-continuity auth scaffold and documented the auth landing contract in `web/viewer/README.md`. The tests now assert that `/signup` keeps its sign-in-first structure while also shipping the new viewer-framed stage, return-path context, and return-link hooks that the page JS rewrites from the safe `next` route.
- Task 4 checks:
  - `gofmt -w internal/server/server_test.go`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
- Task 5 complete: rebuilt the local app stack and verified the live `/signup` route against the running deployment. The deployed page now shows the new viewer-connected hero, the continuity panel text, and the `next`-aware return path/link. A browser probe against `http://127.0.0.1:8080/signup?next=%2Fviewer%2Fbrowse%3Fq%3Dmusic` confirmed the page renders `Sign in without dropping out of the viewer experience.`, shows `/viewer/browse?q=music` in the return context, points the top return link at that same route, and hides the sign-up jump affordance when self-signup is disabled on the local install.
- Task 5 checks:
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - escalated browser probe from `web/viewer` against `http://127.0.0.1:8080/signup?next=%2Fviewer%2Fbrowse%3Fq%3Dmusic`

## Scoped change: video-first homepage discovery pass

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the video-first homepage diagnosis and plan
  - Acceptance criteria:
    - `PLAN.md` captures the new homepage scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered homepage composition, styling, test, and redeploy tasks before edits begin.
    - The read-only diagnosis captures why the current homepage still feels copy-first instead of content-first.

- [x] Task 2 - Recompose the homepage hero around live content and creator previews
  - Acceptance criteria:
    - The first screen gives visual priority to featured/live content, not just explanatory copy.
    - Users can immediately choose a highlighted broadcast or a live rail without scrolling deep into the page.
    - Existing browse and search entry points remain present but no longer dominate the layout.

- [x] Task 3 - Rebalance the surrounding shell and discovery sections to support the new hierarchy
  - Acceptance criteria:
    - The following/sidebar frame supports the homepage instead of competing with it for attention.
    - Supporting sections below the fold feel like continuation rows under a media-first hero.
    - Desktop and mobile layouts remain intentional and usable.

- [x] Task 4 - Update homepage/viewer coverage for the new UX contract
  - Acceptance criteria:
    - Tests assert the new hero hierarchy and key homepage controls/sections.
    - Any changed shell expectations stay covered by focused Jest tests.

- [x] Task 5 - Run focused validation, redeploy locally, and record the outcome
  - Acceptance criteria:
    - Targeted viewer tests, lint, and build pass.
    - The local `bitriver-live` and `viewer` services are refreshed from the current checkout.
    - `TASKS.md` records any remaining host/runtime blocker precisely.

### Execution log (video-first homepage discovery pass)
- Task 1 complete: read-only inspection of `directory-view.tsx`, `home.css`, `ViewerShell.tsx`, `FollowingSidebar.tsx`, and the current homepage tests confirmed the user’s core critique. The page was opening with a large explanatory hero and a permanently weighty following rail, while the most compelling creator/video surfaces (`FeaturedChannel`, `LiveNowGrid`, and the rails) appeared as secondary sections instead of driving the first impression. The scoped plan kept the same discovery data and routes but shifted the hierarchy toward featured/live media.
- Task 1 checks:
  - `Get-Content web/viewer/app/directory-view.tsx`
  - `Get-Content web/viewer/styles/home.css`
  - `Get-Content web/viewer/components/ViewerShell.tsx`
  - `Get-Content web/viewer/components/FollowingSidebar.tsx`
  - `Get-Content web/viewer/__tests__/directoryPage.test.tsx`
- Task 2 complete: rebuilt the homepage hero around a large featured-live stage plus a companion quick-picks stack instead of a text-dominant intro block. The new layout keeps browse/search available, but they now support the media modules instead of leading them. I also moved the `Live now` section ahead of the rest of the discovery rows so viewers hit active streams earlier.
- Task 2 checks:
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live diff -- web/viewer/app/directory-view.tsx`
- Task 3 complete: rebalanced the supporting shell and styling so the homepage reads more like a content destination. The desktop following rail is narrower and lighter, the hero now uses smaller support panels on the right, and the discovery sections below the fold read as continuation rows under the featured/live stage instead of competing hero blocks.
- Task 3 checks:
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live diff -- web/viewer/styles/home.css web/viewer/styles/globals.css`
- Task 4 complete: updated homepage coverage to assert the new quick-link set, the revised hero heading, and the presence of the live quick-picks stack beside the featured stage. Existing `ViewerShell` coverage still passes against the slimmer shell treatment.
- Task 4 checks:
  - `npm.cmd --prefix web/viewer run test -- directoryPage.test.tsx viewerShell.test.tsx`
- Task 5 complete: focused viewer tests, lint, and production build all passed. I then rebuilt/recreated `bitriver-live` and `viewer` locally and verified that `http://localhost:8080/viewer` serves the new homepage copy (`Start with creators already on air`, `Video first, browse second`, `Find a creator, category, or tag`). The initial targeted `docker compose up -d --build` timed out before returning cleanly, so I followed it with Compose status checks and an explicit `--force-recreate --no-deps` refresh of the app services. The remaining practical caveat is content-related, not code-related: this local runtime currently has little or no live/featured seed data, so the new video-first shell is visible but mostly shows its loading/empty states until creators are active.
- Task 5 checks:
  - `npm.cmd --prefix web/viewer run test -- directoryPage.test.tsx viewerShell.test.tsx`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh --viewer` (blocked at env-example placeholder hygiene because Python is not installed/aliased on this Windows host)
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer` (timed out before returning cleanly)
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --force-recreate --no-deps bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer -ErrorAction Stop | Select-Object StatusCode } catch { $_.Exception.Response | Select-Object StatusCode }`
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer).Content | Select-String -Pattern 'Start with creators already on air|Video first, browse second|Find a creator, category, or tag' -AllMatches`

## Scoped change: auth overlay modal treatment

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the auth-overlay diagnosis and plan
  - Acceptance criteria:
    - `PLAN.md` captures the modal-overlay auth scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered auth markup, styling, coverage, and runtime validation tasks before edits begin.
    - The read-only diagnosis explains why the current `/signup` route still reads like a separate page instead of an overlay.

- [x] Task 2 - Rebuild the auth markup into a centered overlay/dialog presentation
  - Acceptance criteria:
    - The auth route visually reads as a modal card over dimmed viewer context instead of a split-page destination.
    - Existing sign-in, sign-up, MFA, return-link, and self-signup-disabled flows remain intact.
    - The primary sign-in action stays immediately visible and easy to understand.

- [x] Task 3 - Restyle the auth surface and backdrop for a viewer-overlay feel
  - Acceptance criteria:
    - The page background clearly resembles viewer context while remaining non-interactive and subordinate to the auth modal.
    - The modal uses tighter hierarchy and spacing closer to streaming-platform auth overlays.
    - Mobile and desktop both remain usable.

- [x] Task 4 - Update auth-route coverage for the new modal contract
  - Acceptance criteria:
    - `internal/server` coverage asserts the new auth scaffold/copy hooks that define the overlay treatment.
    - Existing operator/admin hint expectations remain intact.

- [x] Task 5 - Run focused validation, refresh the local app, and record the outcome
  - Acceptance criteria:
    - Targeted `internal/server` tests pass.
    - The local `bitriver-live` and `viewer` services are refreshed so the updated auth route is reviewable at `localhost:8080`.
    - `TASKS.md` records any remaining host/runtime blocker precisely.

### Execution log (auth overlay modal treatment)
- Task 1 complete: read-only review of `web/static/signup.html`, `web/static/signup.js`, `web/static/styles.css`, and `internal/server/server_test.go` confirmed that the current auth route was still structured like a two-column destination page. Even though it shared viewer branding and return-context copy, it still devoted half the screen to explanatory content instead of presenting auth as a centered checkpoint over viewer context, which is the exact gap the user called out via the Twitch comparison.
- Task 1 checks:
  - `Get-Content web/static/signup.html`
  - `Get-Content web/static/signup.js`
  - `Get-Content web/static/styles.css`
  - `Get-Content internal/server/server_test.go | Select-Object -Skip 300 -First 120`
- Task 2 complete: rebuilt `web/static/signup.html` into a modal-first auth route. The page now centers a compact login dialog over a dimmed viewer-style backdrop, keeps the return link in a modal close position, and collapses sign-up into the same modal flow instead of presenting auth as a separate split-page destination. I also added lightweight sign-in/sign-up mode tabs tied to `#login-card` and `#signup-card` so the flow stays compact and more overlay-like.
- Task 2 checks:
  - `Get-Content web/static/signup.html`
  - `node --check web/static/signup.js`
- Task 3 complete: restyled `web/static/styles.css` so `/signup` reads like a streaming-platform overlay: blurred viewer context behind the modal, tighter modal spacing, full-width primary auth actions, and a compact return-context card instead of a large left-column explainer. The auth JS now hides or reveals the sign-up view based on hash mode while preserving the existing self-signup-disabled behavior and return-path rewrites.
- Task 3 checks:
  - `Get-Content web/static/styles.css`
  - `Get-Content web/static/signup.js`
- Task 4 complete: updated `internal/server/server_test.go` so the static signup route contract now asserts the modal scaffold (`auth-stage--modal`, `auth-modal__surface`), the simpler auth copy (`Log in to BitRiver Live`), the preserved return-path hook, and the auth-mode toggle hook. The existing operator/admin hint coverage stayed intact.
- Task 4 checks:
  - `gofmt -w internal/server/server_test.go`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
- Task 5 complete: rebuilt the local app stack and verified the live auth route from the running install. `http://localhost:8080/signup?next=%2Fviewer#signup-card` now serves the new modal scaffold and, in a real browser pass, correctly keeps the login card visible, hides the sign-up tab on this self-signup-disabled local install, points the close/return link back to `/viewer`, and surfaces the operator/admin feedback when someone lands on the sign-up hash. The canonical repo verify entrypoint is still blocked on this Windows host at the env-example placeholder hygiene step because Python is not installed/aliased.
- Task 5 checks:
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `node --check web/static/signup.js`
  - `(Invoke-WebRequest -UseBasicParsing 'http://localhost:8080/signup?next=%2Fviewer#signup-card').Content | Select-String -Pattern 'Log in to BitRiver Live|auth-stage auth-stage--modal|data-auth-mode="signin"|Need an account\? Sign up' -AllMatches`
  - escalated Playwright probe against `http://127.0.0.1:8080/signup?next=%2Fviewer#signup-card`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh` (blocked at env-example placeholder hygiene because Python is not installed/aliased on this host)

## Scoped change: real in-viewer auth overlay

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the in-viewer auth-overlay diagnosis and implementation plan
  - Acceptance criteria:
    - `PLAN.md` captures the new in-viewer overlay scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered backend, viewer-overlay, CTA migration, and validation tasks before edits begin.
    - The read-only diagnosis identifies why the current `/signup` route is still architecturally separate from `/viewer`.

- [x] Task 2 - Add a viewer-auth contract and compatibility redirect on the Go server
  - Acceptance criteria:
    - The backend exposes a guest-safe viewer-auth response the viewer can call without treating anonymous users as hard failures.
    - `/signup` forwards into the viewer overlay when the viewer is configured, while preserving a safe fallback when it is not.
    - Focused server coverage locks the new route behavior and compatibility contract.

- [x] Task 3 - Build the real auth overlay inside the Next.js viewer runtime
  - Acceptance criteria:
    - A global viewer-owned modal handles sign-in, sign-up, feedback, and MFA continuation without navigating away from `/viewer`.
    - `useAuth` exposes the state/actions needed for opening the overlay, submitting auth requests, and refreshing the session after success.
    - Successful auth keeps users in the viewer experience and returns them to the intended route.

- [x] Task 4 - Retarget signed-out viewer controls to the in-app overlay
  - Acceptance criteria:
    - Navbar, following-sidebar, and other existing sign-in/join entry points open the in-viewer overlay instead of treating `/signup` as the primary path.
    - Updated viewer tests cover the new auth CTA contract and any changed auth-state helpers.
    - Viewer-facing docs mention the new overlay-first auth flow if the navigation/runtime contract changes.

- [x] Task 5 - Run focused validation, redeploy locally, and record the outcome
  - Acceptance criteria:
    - Touched Go and viewer test suites pass, plus viewer lint/build.
    - The local `bitriver-live` and `viewer` services are refreshed from the current checkout.
    - A live browser probe confirms auth opens as an overlay on `/viewer`, and `TASKS.md` records any remaining blocker precisely.

### Execution log (real in-viewer auth overlay)
- Task 1 complete: read-only analysis confirmed that the current setup is still architecturally split. `useAuth` loads `/api/viewer/me` but falls back to redirecting out to `/signup`; the navbar `Join` CTA hard-navigates to `/signup#signup-card`; the following sidebar exposes a static `/signup` link; and the only existing viewer-side `AuthDialog` is just a thin "sign in/sign out" shell that still redirects. On the backend, `/api/viewer/me` is not explicitly registered even though the viewer expects it, and the self-signup flag currently only gets bootstrapped into the static `signup.html` route. That combination is why the auth flow still feels separate even after the prior modal styling pass.
- Task 1 checks:
  - `Get-Content web/viewer/hooks/useAuth.tsx`
  - `Get-Content web/viewer/components/Navbar.tsx`
  - `Get-Content web/viewer/components/following/FollowingState.tsx`
  - `Get-Content web/viewer/components/auth/AuthDialog.tsx`
  - `Get-Content web/viewer/components/Providers.tsx`
  - `Get-Content web/viewer/app/layout.tsx`
  - `Get-Content internal/api/auth_users_handlers.go`
  - `Get-Content internal/server/server.go`
  - `Get-Content web/static/signup.js`
- Task 2 complete: added a real guest-safe `/api/viewer/me` handler on the Go API, registered it with optional auth semantics, and made `DELETE /api/viewer/me` an idempotent sign-out path that clears both session and MFA challenge cookies. I also changed `/signup` so viewer-enabled installs now redirect into the appropriate viewer route with `auth`/`mfa` state in the query string, while non-viewer installs still keep the embedded static auth page as a fallback.
- Task 2 checks:
  - `gofmt -w internal/api/auth_users_handlers.go internal/api/auth_users_handlers_test.go internal/server/server.go internal/server/server_test.go`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/api ./internal/server -count=1 -timeout=120s`
- Task 3 complete: moved auth into the actual viewer runtime. `useAuth` now owns overlay state, guest-safe session/config loading, sign-in/sign-up/MFA submissions, and same-route auth completion. `Providers` now mounts a global `AuthDialog`, and the modal itself carries the real sign-in, sign-up, MFA, and “continue where you left off” flow inside the Next.js viewer shell instead of redirecting users to a separate page.
- Task 3 checks:
  - `npm.cmd --prefix web/viewer run test -- useAuth.test.tsx`
  - `npm.cmd --prefix web/viewer run build`
- Task 4 complete: retargeted the signed-out viewer entry points to the new overlay-first flow. The navbar now opens the in-viewer modal for internal auth and only falls back to hard navigation for truly external signup destinations; the following unauthenticated prompt now triggers `signIn()` directly instead of exposing a dead-feeling `/signup` link; and the viewer README now documents the overlay-first contract with `/signup` as compatibility-only behavior.
- Task 4 checks:
  - `npm.cmd --prefix web/viewer run test -- navbar.test.tsx followingStatePresentation.test.tsx`
  - `npm.cmd --prefix web/viewer run lint`
- Task 5 complete: reran the focused viewer and Go coverage, lint, and production build; rebuilt the running `bitriver-live` and `viewer` services; and then validated the live app with route-level and browser-level probes. The first `docker compose up -d --build` attempt timed out before the viewer image finished rebuilding, which initially left the old auth behavior live; rerunning the same build with a longer timeout resolved that, and the final browser probe against `http://127.0.0.1:8080/viewer` confirmed that the navbar Sign in button now opens the in-viewer auth dialog. I also verified that `http://localhost:8080/signup?next=%2Fviewer%2Fbrowse%3Fq%3Dmusic&mfa=verify` now returns a `307` redirect into `/viewer/browse?auth=signin&mfa=verify...` instead of serving the old auth page on this viewer-enabled install.
- Task 5 checks:
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/api ./internal/server -count=1 -timeout=120s`
  - `npm.cmd --prefix web/viewer run test -- useAuth.test.tsx navbar.test.tsx followingStatePresentation.test.tsx`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer` (first attempt timed out before the viewer image finished rebuilding; reran with a longer timeout and it completed)
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - escalated headless browser probe against `http://127.0.0.1:8080/viewer`
  - `Invoke-WebRequest -UseBasicParsing 'http://localhost:8080/signup?next=%2Fviewer%2Fbrowse%3Fq%3Dmusic&mfa=verify' -MaximumRedirection 0`

## Scoped change: viewer shell alignment and Docker project naming review

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the new shell-alignment scope and Docker naming diagnosis before editing
  - Acceptance criteria:
    - `PLAN.md` captures the viewer-shell alignment scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered UI tasks plus the approval-gated Docker naming follow-up before code edits begin.
    - Read-only diagnosis identifies whether the `deploy` label is coming from Compose project naming and whether the homepage mismatch is a shell/layout issue.

- [x] Task 2 - Tighten the desktop viewer shell and following rail hierarchy
  - Acceptance criteria:
    - The Following rail feels aligned with the Featured Live hero on desktop instead of starting on an awkwardly different rhythm.
    - Signed-out guidance remains present, but extra explanatory copy no longer dominates the rail or pushes the layout off balance.
    - Existing viewer shell/sidebar behavior stays usable on mobile and desktop.

- [x] Task 3 - Update focused viewer coverage for the revised shell contract
  - Acceptance criteria:
    - Focused viewer tests cover the updated sidebar/header treatment and homepage structure where expectations changed.
    - Existing auth-overlay and homepage behaviors remain covered after the shell cleanup.

- [x] Task 4 - Run focused viewer validation, redeploy locally, and record the outcome
  - Acceptance criteria:
    - The targeted viewer test suites, lint, and production build pass.
    - The local `bitriver-live` and `viewer` services are refreshed from the current checkout for review.
    - `TASKS.md` records any remaining runtime or host blocker precisely.

- [ ] Task 5 - Rename the Compose project to a clearer Docker Desktop stack name if approved
  - Acceptance criteria:
    - The Compose project name changes from the default `deploy` label to an explicit BitRiver-specific name.
    - The deployment contract/docs are updated together if the change is approved and implemented.
    - Compose config/runtime validation records the new stack naming without breaking the canonical workflow.

### Execution log (viewer shell alignment and Docker project naming review)
- Task 1 complete: read-only inspection confirmed the two user-reported issues come from different layers. Docker Desktop is showing the stack as `deploy` because the Compose project name is currently implicit and is inheriting from the compose-file directory, which makes any rename a deployment-contract/workflow change that needs approval first. The homepage mismatch is shell-driven: the desktop `viewer-sidebar` currently renders its own eyebrow, heading, and intro copy above `FollowingSidebar`, which makes the left rail start on a different visual rhythm than the hero block.
- Task 1 checks:
  - `Get-Content PLAN.md | Select-Object -Last 30`
  - `Get-Content TASKS.md | Select-Object -Last 40`
  - `Get-Content deploy/docker-compose.yml -TotalCount 40`
  - `Get-Content .env -TotalCount 40`
  - `Get-Content web/viewer/components/ViewerShell.tsx | Select-Object -Skip 130 -First 90`
  - `Get-Content web/viewer/styles/globals.css | Select-Object -Skip 4260 -First 150`
  - `Get-Content web/viewer/styles/home.css | Select-Object -Skip 970 -First 280`
- Task 2 complete: removed the duplicate shell-owned sidebar intro/header on desktop, kept only the compact mobile close header, and tightened the `FollowingSidebar` header so the rail now carries one concise title plus one state card instead of layering explanatory copy above the actual following UI.
- Task 2 checks:
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live diff -- web/viewer/components/ViewerShell.tsx web/viewer/components/FollowingSidebar.tsx web/viewer/styles/globals.css`
- Task 3 complete: updated focused viewer coverage to lock in the cleaner contract. `ViewerShell` now asserts the old duplicate intro copy is gone, and `followingStatePresentation` now checks for the new `Following` heading plus the single guest prompt in the sidebar.
- Task 3 checks:
  - `npm.cmd --prefix web/viewer run test -- viewerShell.test.tsx followingStatePresentation.test.tsx directoryPage.test.tsx`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
- Task 4 complete: reran the focused viewer validation after stripping the stale inner-column padding, added a browser-level Playwright regression for the desktop homepage shell, rebuilt the local `bitriver-live`/`viewer` stack, and then rechecked the deployed page on `http://127.0.0.1:8080/viewer`. The final browser probe showed the desktop shell in the correct mode, the following rail and featured hero aligned at the same top offset (`88px` each), the header rows within `6px` of each other, the old shell intro copy gone, and the guest sign-in message present only once.
- Task 4 checks:
  - `npm.cmd --prefix web/viewer run test -- viewerShell.test.tsx followingStatePresentation.test.tsx directoryPage.test.tsx`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
  - `npx.cmd playwright test tests/homepage-layout.spec.ts --reporter=list`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - escalated headless browser probe against `http://127.0.0.1:8080/viewer`

## Scoped change: workflow and CI verification pass

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the workflow-validation scope and inspect the defined CI jobs before running checks
  - Acceptance criteria:
    - `PLAN.md` captures the CI/workflow validation scope, assumptions, risks, and candidate commands.
    - `TASKS.md` lists the ordered workflow-inspection, execution, and reporting tasks before running broad checks.
    - The read-only pass identifies which workflows can be mirrored locally on this Windows host and which inherently depend on GitHub-hosted Linux/macOS runners or external services.

- [x] Task 2 - Run the feasible local equivalents of the defined workflow gates
  - Acceptance criteria:
    - Local commands cover the main workflow categories: Go/unit gates, viewer CI, quickstart sanity, postgres integration, docs/workflow consistency, and monitoring config.
    - Each command result is recorded clearly as pass, fail, or host/tooling blocker.
    - No product-code edits are made unless a currently reproducible repository issue demands it.

- [x] Task 3 - Summarize cross-platform confidence and remaining hosted-runner gaps
  - Acceptance criteria:
    - The final report distinguishes what was actually verified on this Windows host from what still requires Ubuntu/macOS GitHub Actions.
    - Any blockers are tied back to specific workflow steps or missing host tools.
    - The result is actionable for shipping a self-hosted live-streaming stack, not just a raw command dump.

### Execution log (workflow and CI verification pass)
- Task 1 complete: inspected the current workflow set under `.github/workflows` and mapped the important gates. The repo currently defines a top-level `ci.yml` with an Ubuntu `test-all` gate plus separate macOS/Windows Go coverage, a cross-platform quickstart sanity matrix, Ubuntu-only viewer CI, postgres integration, shell/docs/workflow consistency checks, monitoring validation, image scan, ingest e2e, and release flows. Tool availability on this host is mixed: `bash`, `py`, `go`, `docker`, `node`, `npm`, and `gh` are available, while `python3`, `psql`, `shellcheck`, and `act` are not currently on PATH.
- Task 1 checks:
  - `Get-Content .github/workflows/ci.yml`
  - `Get-Content .github/workflows/go-unit-tests.yml`
  - `Get-Content .github/workflows/viewer-ci.yml`
  - `Get-Content .github/workflows/quickstart-smoke.yml`
  - `Get-Content .github/workflows/postgres-tests.yml`
  - `Get-Content scripts/verify.sh`
  - `Get-Content scripts/test-all.sh`
  - `Get-Command bash, python3, py, go, docker, node, npm, psql, shellcheck, gh, act -ErrorAction SilentlyContinue | Select-Object Name,Source`
- Task 2 complete: after the Windows parity fixes below, the feasible local CI equivalents now run cleanly on this workstation. `go test ./... -count=1 -timeout=120s` passes with a workspace-local `GOCACHE`, `powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 -help` passes, `powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 -ValidateOnly` now passes after the PowerShell wrapper gained a writable fallback `GOCACHE`, and `docker compose --env-file .env -f deploy/docker-compose.yml config` still renders the deployment contract successfully (with a local Docker config permission warning from this host, but an exit code of 0).
- Task 2 checks:
  - `git branch --show-current`
  - `pwsh -File scripts/quickstart.ps1 -help`
  - `pwsh -File scripts/quickstart.ps1 -ValidateOnly`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./... -count=1 -timeout=120s`
  - `gh auth status`
  - `gh workflow list`
- Task 3 complete: the hosted-runner picture is now clearer. This Windows host verified the core Go/runtime path and PowerShell quickstart entrypoint, but the manually dispatched hosted workflows exposed a second layer of issues: several standalone workflows (`go-unit-tests.yml`, `viewer-ci.yml`, `postgres-tests.yml`, `ingest-e2e.yml`) fail immediately under `workflow_dispatch` because they invoke local composite actions before checking out the repo, while other workflows surfaced real repo/script issues such as ShellCheck warnings, a brittle OME mount assertion in `test-quickstart.sh`, a monitoring config validation command problem, and a staged wizard-release regression.

## Scoped change: Windows CI parity fixes for transcoder and quickstart tests

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Capture the concrete Windows test failures before implementation
  - Acceptance criteria:
    - `PLAN.md` records the narrowed Windows parity scope, assumptions, risks, and validation commands.
    - `TASKS.md` records the exact failing packages and the current root-cause hypothesis.
    - The implementation stays scoped to the failing test harnesses unless runtime code is proven necessary.

- [x] Task 2 - Make the transcoder FFmpeg stub harness work on Windows without changing runtime behavior
  - Acceptance criteria:
    - `cmd/transcoder` tests use a deterministic FFmpeg stub on Windows instead of resolving a real system `ffmpeg.exe`.
    - The Windows stub produces the same observable playlist/segment outputs the existing Unix stub-based tests expect.
    - The previously failing transcoder package tests pass on this host.

- [x] Task 3 - Make the quickstart shell-wrapper tests select a usable shell on Windows
  - Acceptance criteria:
    - The tests prefer a runnable Git Bash style shell over unusable WSL `bash.exe` on Windows.
    - PATH manipulation in the harness uses platform-correct separators.
    - The previously failing `scripts` package tests pass on this host without weakening their assertions.

- [x] Task 4 - Rerun the cross-platform verification commands and dispatch safe hosted workflows
  - Acceptance criteria:
    - `go test ./...` passes on this Windows host from the current checkout.
    - Windows quickstart PowerShell entrypoint checks complete locally with the available shell tooling.
    - Safe validation workflows are dispatched from GitHub CLI against the current branch, and their run status is recorded.

### Execution log (Windows CI parity fixes for transcoder and quickstart tests)
- Task 1 complete: confirmed the Windows-local failures are test-harness issues in two isolated areas. `cmd/transcoder/main_test.go` currently assumes `testdata/ffmpeg` is directly executable and therefore fails to shadow a real `ffmpeg.exe` on Windows, which cascades into the health recovery assertions. `scripts/quickstart_test.go` assumes any `bash` on PATH is usable, but on this host the default resolution lands on WSL `bash.exe`, while usable Git Bash binaries exist separately under `C:\Program Files\Git\...`.
- Task 1 checks:
  - `Get-Content cmd/transcoder/main_test.go | Select-Object -First 220`
  - `Get-Content cmd/transcoder/testdata/ffmpeg | Select-Object -First 200`
  - `Get-Content scripts/quickstart_test.go | Select-Object -First 220`
  - `Get-Content scripts/quickstart.sh | Select-Object -First 200`
  - `Get-Content .github/workflows/quickstart-smoke.yml | Select-Object -First 220`
  - `$candidates = @('C:\Program Files\Git\bin\bash.exe','C:\Program Files\Git\usr\bin\bash.exe','C:\WINDOWS\system32\bash.exe'); foreach ($p in $candidates) { '{0}`t{1}' -f $p, (Test-Path $p) }`
- Task 2 in progress: the first round of fixes proved the quickstart harness issue was isolated, but also uncovered a real Windows runtime limitation in the transcoder live publish path. `go test ./scripts` now passes on this host after selecting a usable shell and fixing PATH handling. `go test ./cmd/transcoder` improved enough to expose the next blocker: Windows on this machine cannot create directory symlinks without administrator privileges (`New-Item -ItemType SymbolicLink ...` fails with "Administrator privilege required"), while `publishLive` currently relies on `os.Symlink`, so live manifest publication and the related health checks stay degraded even after the FFmpeg stub issue is addressed.
- Task 2 checks:
  - `gofmt -w cmd/transcoder/main_test.go scripts/quickstart_test.go`
  - `New-Item -ItemType Directory -Force .gocache-transcoder | Out-Null; $env:GOCACHE=(Resolve-Path .gocache-transcoder).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./cmd/transcoder -count=1 -timeout=120s`
  - `New-Item -ItemType Directory -Force .gocache-scripts | Out-Null; $env:GOCACHE=(Resolve-Path .gocache-scripts).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./scripts -count=1 -timeout=120s`
  - `$tmp = Join-Path (Resolve-Path .).Path '.codex-symlink-check'; if (Test-Path $tmp) { Remove-Item -Recurse -Force $tmp }; New-Item -ItemType Directory -Path $tmp | Out-Null; New-Item -ItemType Directory -Path (Join-Path $tmp 'target') | Out-Null; try { New-Item -ItemType SymbolicLink -Path (Join-Path $tmp 'link') -Target (Join-Path $tmp 'target') -ErrorAction Stop | Out-Null; 'symlink-ok' } catch { 'symlink-failed: ' + $_.Exception.Message }`
- Task 2 complete: replaced the Windows-only FFmpeg stub path with a native `ffmpeg.exe` test shim so the transcoder suite no longer falls through to a real system `ffmpeg.exe`. The suite also now uses a Windows-capable live mirror path: `publishLive` still prefers `os.Symlink`, but on Windows it falls back to a directory junction created via `mklink /J`, and the lifecycle test now tolerates Windows junction semantics plus slower mirror cleanup timing. With those changes in place, `go test ./cmd/transcoder -count=1 -timeout=120s` passes on this host.
- Task 2 checks:
  - `cmd /c mklink /J "<workspace>\\.codex-junction-check\\link" "<workspace>\\.codex-junction-check\\target"`
  - `New-Item -ItemType Directory -Force .gocache-transcoder | Out-Null; $env:GOCACHE=(Resolve-Path .gocache-transcoder).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./cmd/transcoder -count=1 -timeout=120s`
- Task 3 complete: the quickstart test harness now resolves a usable Git Bash-style shell on Windows instead of assuming the first `bash.exe` is runnable, uses the correct PATH separator on Windows, and stages a Windows-friendly `go.cmd` stub for the wrapper test. I also hardened `scripts/quickstart.ps1` itself so `-ValidateOnly` sets a writable fallback `GOCACHE` before launching `go run`, which removed the local `Access is denied` failure against the default Windows build cache path. `go test ./scripts -count=1 -timeout=120s` and the real `powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 -ValidateOnly` path both pass now.
- Task 3 checks:
  - `gofmt -w scripts/quickstart_test.go`
  - `New-Item -ItemType Directory -Force .gocache-scripts | Out-Null; $env:GOCACHE=(Resolve-Path .gocache-scripts).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./scripts -count=1 -timeout=120s`
  - `New-Item -ItemType Directory -Force .gocache-quickstart | Out-Null; $env:GOCACHE=(Resolve-Path .gocache-quickstart).Path; powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 -ValidateOnly`
- Task 4 complete: reran the broad Windows-local gate and then dispatched the safe hosted validation workflows for branch `codex/UI_UX_repair`. The local gate now passes end-to-end (`go test ./...` plus `docker compose ... config`). Hosted workflow dispatch succeeded for `ci.yml`, `docs-consistency.yml`, `go-unit-tests.yml`, `go-workflow-consistency.yml`, `image-scan.yml`, `ingest-e2e.yml`, `monitoring-config.yml`, `postgres-tests.yml`, `quickstart-smoke.yml`, `shellcheck.yml`, `viewer-ci.yml`, and `wizard-release.yml`. The resulting run list shows the current hosted blockers clearly: `CI` and `Docs consistency` passed, `Go workflow consistency` passed, while several workflow-dispatch-only jobs fail immediately because they use local composite actions before checkout, and several others fail on real repo/script issues (`shellcheck`, `monitoring-config`, `quickstart-smoke`, `wizard-release`, and an external Trivy install failure in `image-scan`).
- Task 4 checks:
  - `New-Item -ItemType Directory -Force .gocache-all | Out-Null; $env:GOCACHE=(Resolve-Path .gocache-all).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./... -count=1 -timeout=120s`
  - `powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 -help`
  - `powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 -ValidateOnly`
  - `docker compose --env-file .env -f deploy/docker-compose.yml config`
  - `gh workflow run ci.yml --ref codex/UI_UX_repair`
  - `gh workflow run docs-consistency.yml --ref codex/UI_UX_repair`
  - `gh workflow run go-unit-tests.yml --ref codex/UI_UX_repair`
  - `gh workflow run go-workflow-consistency.yml --ref codex/UI_UX_repair`
  - `gh workflow run image-scan.yml --ref codex/UI_UX_repair`
  - `gh workflow run ingest-e2e.yml --ref codex/UI_UX_repair`
  - `gh workflow run monitoring-config.yml --ref codex/UI_UX_repair`
  - `gh workflow run postgres-tests.yml --ref codex/UI_UX_repair`
  - `gh workflow run quickstart-smoke.yml --ref codex/UI_UX_repair`
  - `gh workflow run shellcheck.yml --ref codex/UI_UX_repair`
  - `gh workflow run viewer-ci.yml --ref codex/UI_UX_repair`
  - `gh workflow run wizard-release.yml --ref codex/UI_UX_repair`
  - `gh run list --branch codex/UI_UX_repair --limit 20`
  - `gh run view 23512035395 --log-failed`
  - `gh run view 23512040120 --log-failed`
  - `gh run view 23512041661 --log-failed`
  - `gh run view 23512043069 --log-failed`
  - `gh run view 23512044850 --log-failed`
  - `gh run view 23512046165 --log-failed`
  - `gh run view 23512047607 --log-failed`
  - `gh run view 23512049060 --log-failed`

## Scoped change: auth overlay route-copy trim

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the scoped auth-overlay copy cleanup before editing
  - Acceptance criteria:
    - `PLAN.md` captures the narrowed overlay-copy scope, assumptions, risks, and focused validation command.
    - `TASKS.md` lists the ordered implementation and verification steps before `AuthDialog` is edited.
    - Read-only inspection identifies the exact redundant sentence currently rendered in the route context card.

- [x] Task 2 - Remove the redundant destination explanation from the auth overlay
  - Acceptance criteria:
    - The auth overlay still shows "Continue where you left off" and the destination path.
    - The extra sentence describing the automatic return behavior is removed from the dialog.
    - Existing sign-in/sign-up/MFA behavior remains unchanged.

- [x] Task 3 - Add focused viewer coverage and run verification
  - Acceptance criteria:
    - A focused auth-dialog test locks in the trimmed route-context contract.
    - The targeted viewer test command passes.
    - `TASKS.md` records the executed command(s) and result.

### Execution log (auth overlay route-copy trim)
- Task 1 complete: captured the scoped copy-cleanup plan in `PLAN.md`/`TASKS.md` before touching the viewer component and confirmed the redundant sentence currently comes from `describeDestination()` inside `web/viewer/components/auth/AuthDialog.tsx`.
- Task 1 checks:
  - `Get-Content PLAN.md | Select-Object -Last 20`
  - `Get-Content TASKS.md | Select-Object -Last 30`
  - `Get-Content web/viewer/components/auth/AuthDialog.tsx`
- Task 2 complete: removed the route-context paragraph from `AuthDialog` and deleted the now-unused `describeDestination()` helper, leaving the overlay card focused on the return header plus destination path only.
- Task 2 checks:
  - `Get-Content web/viewer/components/auth/AuthDialog.tsx | Select-Object -First 120`
  - `git diff -- web/viewer/components/auth/AuthDialog.tsx web/viewer/__tests__/authDialog.test.tsx`
- Task 3 complete: added focused `web/viewer/__tests__/authDialog.test.tsx` coverage for the trimmed route-context contract and ran the targeted viewer Jest command successfully. Jest still emitted the existing `.next/standalone/package.json` haste-collision warning from the worktree, but the suite passed cleanly.
- Task 3 checks:
  - `npm.cmd --prefix web/viewer run test -- authDialog.test.tsx`

## Scoped change: local redeploy for latest localhost review

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the local redeploy scope before runtime actions
  - Acceptance criteria:
    - `PLAN.md` captures the redeploy scope, assumptions, risks, and verification commands.
    - `TASKS.md` lists the ordered rebuild and route-check tasks before Docker actions begin.

- [x] Task 2 - Rebuild and recreate the local app services from the current checkout
  - Acceptance criteria:
    - `bitriver-live` and `viewer` are rebuilt from the current source tree using the canonical Compose contract.
    - Compose status output is captured after the rebuild attempt.
    - The action stays limited to the app services unless a hard blocker requires wider intervention.

- [x] Task 3 - Verify the redeployed local routes reflect the latest app update
  - Acceptance criteria:
    - `http://localhost:8080/` responds after the rebuild.
    - `http://localhost:8080/viewer` serves the current homepage content from the updated app.
    - The live viewer auth entry path reflects the latest auth-overlay update.
    - `TASKS.md` records the post-redeploy status or any host blocker precisely.

### Execution log (local redeploy for latest localhost review)
- Task 1 complete: recorded the redeploy scope in `PLAN.md` and `TASKS.md` before touching Docker so the rebuild/recreate steps and live route checks were explicit.
- Task 1 checks:
  - `Get-Content PLAN.md | Select-Object -Last 30`
  - `Get-Content TASKS.md | Select-Object -Last 35`
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live status --short`
- Task 2 complete: captured the pre-redeploy Compose state, rebuilt `bitriver-live` and `viewer` from the current checkout with the canonical Compose contract, and confirmed the recreated app services came back on fresh container ages. Compose also refreshed the dependent app-path containers it needed during the targeted `up -d --build` flow, but no contract changes were made.
- Task 2 checks:
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- Task 3 complete: verified the redeployed stack is serving from the updated build. `/` still returns the expected `307` redirect, `/viewer` serves the current homepage copy including `Start with creators already on air`, `Watch live now`, and `Full directory`, and the live auth flow now resolves to `http://127.0.0.1:8080/viewer?auth=signin&next=%2Fviewer` on this stack (self-signup is currently disabled), where a browser-level check confirmed `CONTINUE WHERE YOU LEFT OFF`, `/viewer`, and the trimmed sign-in overlay copy are present while the removed auto-return sentence stays absent.
- Task 3 checks:
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/ -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode,Headers } catch { $_.Exception.Response | Select-Object StatusCode,Headers }`
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer).Content | Select-String -Pattern 'Browse BitRiver Live|Watch live now|Full directory' -AllMatches`
  - `(Invoke-WebRequest -UseBasicParsing 'http://localhost:8080/viewer?auth=signup').Content | Select-String -Pattern 'Continue where you left off|Create your BitRiver account|/viewer' -AllMatches` (route served, but the auth surface required a hydrated browser check on this self-signup-disabled stack)
  - escalated headless browser probe against `http://127.0.0.1:8080/viewer?auth=signup`
  - escalated headless browser probe against `http://127.0.0.1:8080/viewer` with a click on `Sign in`

## Scoped change: public release readiness and repo hygiene pass

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the public-release audit scope before editing
  - Acceptance criteria:
    - `PLAN.md` captures the audit/improvement scope, assumptions, risks, and verification commands.
    - `TASKS.md` lists the ordered hygiene/docs/release tasks before edits begin.
    - The read-only audit identifies concrete trust/onboarding/release blockers rather than vague cleanup ideas.

- [x] Task 2 - Remove internal-looking tracked clutter and harden ignore rules
  - Acceptance criteria:
    - Generated temp files and one-off release evidence logs are no longer tracked.
    - `.gitignore` covers the relevant local temp/output paths without hiding real source files.
    - Any docs that referenced removed artifact paths are updated or replaced with cleaner public guidance.

- [x] Task 3 - Add the missing public repo governance and contribution surfaces
  - Acceptance criteria:
    - Root/community files cover contributing, security reporting, code of conduct, and changelog/release expectations.
    - GitHub issue templates and a PR template exist for public collaboration.
    - Added docs stay honest about current support scope and maintainer expectations.

- [x] Task 4 - Rework onboarding docs so the repo lands well for first-time outsiders
  - Acceptance criteria:
    - `README.md` leads with a crisp value proposition and practical quickstart path.
    - The README clearly states what works today, what is planned, and what is not supported.
    - Broken/stale onboarding links or misleading internal-language sections are fixed in the touched docs.

- [x] Task 5 - Align release-facing artifacts for a credible first public tag
  - Acceptance criteria:
    - Release docs/changelog support a real first public release instead of internal placeholder history.
    - The repo has a clear release checklist and draft notes path for the recommended initial tag.
    - Remaining public-release blockers are documented precisely after verification.

- [x] Task 6 - Run relevant validation and record the final release-readiness status
  - Acceptance criteria:
    - Repo hygiene/docs checks are run and logged.
    - The closest feasible repo-wide verification gate is attempted and recorded.
    - `TASKS.md` captures any residual blockers that still prevent a confident public release.

### Execution log (public release readiness and repo hygiene pass)
- Task 1 complete: finished the read-only audit before editing. The strongest public-facing blockers are tracked temp/release-log artifacts under `.tmp/` and `artifacts/`, missing public collaboration/security files, missing issue/PR templates, a README that reads more like an internal maintainer memo than a first-release landing page, a broken quickstart anchor in `web/viewer/README.md`, and stale/speculative release docs (`docs/releases/release-checklist-report-*.md`, `docs/releases/v1.0.0.md`) that weaken trust.
- Task 1 checks:
  - `Get-ChildItem -Force | Select-Object Mode,Length,LastWriteTime,Name`
  - `Get-Content README.md`
  - `Get-Content deploy/.env.example`
  - `Get-ChildItem .github -Recurse | Select-Object FullName`
  - `git ls-files artifacts .tmp docs/releases`
  - `rg -n "quick-start-docker-one-command|v1\\.2\\.3|artifacts/release-checks|internal/service" README.md web/viewer/README.md docs .github deploy`
- Task 2 complete: removed tracked temp/release-evidence clutter (`.tmp/render-srs-config.sh`, `artifacts/release-checks-*`, and dated `docs/releases/release-checklist-report-*` files), deleted the stale speculative `docs/releases/v1.0.0.md`, and tightened `.gitignore` for repo-local scratch/output paths (`artifacts`, `.artifacts`, `.tmp`, `.codex-tmp`, `dist`, and targeted `.gocache-*` directories).
- Task 2 checks:
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live diff --name-status -- .gitignore .tmp artifacts docs/releases`
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live check-ignore -v .tmp/example artifacts/example .artifacts/example .codex-tmp/example dist/example`
  - `rg -n "artifacts/release-checks|\\.tmp/render-srs-config\\.sh" README.md docs deploy scripts .github`
- Task 3 complete: added public-facing contributor/community files (`CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`) plus GitHub issue templates and a pull-request template so the repo has the expected trust and collaboration surfaces for outside contributors.
- Task 3 checks:
  - `Get-Item CONTRIBUTING.md,SECURITY.md,CODE_OF_CONDUCT.md,.github/pull_request_template.md | Select-Object Name,Length`
  - `Get-ChildItem .github/ISSUE_TEMPLATE | Select-Object Name,Length`
  - `Get-Content SECURITY.md`
- Task 4 complete: rewrote the root `README.md` around a public-first pitch, a simpler quickstart, explicit support boundaries, and a small architecture overview. Also fixed the stale quickstart anchor in `web/viewer/README.md`.
- Task 4 checks:
  - `Get-Content README.md -TotalCount 120`
  - `rg -n "quick-start-docker-one-command|#quickstart" README.md web/viewer/README.md`
- Task 5 complete: added a root `CHANGELOG.md`, replaced the stale release-note story with `docs/releases/README.md` and `docs/releases/v1.2.3-draft.md`, and aligned the repo around the existing `v1.2.3` version lineage instead of the speculative checked-in `v1.0.0` note.
- Task 5 checks:
  - `Get-ChildItem docs/releases | Select-Object Name,Length`
  - `Get-Content CHANGELOG.md`
  - `rg -n "first public release|v1\\.0\\.0 release notes" CHANGELOG.md docs/releases docs`
- Task 6 complete: validation surfaced and then cleared several low-risk release blockers in the repo itself. The shell/doc checks now pass, `docs/contract.md` was regenerated from `deploy/.env.example`, `scripts/check-doc-installer-language.sh` now points at the correct `docs/labs/cross-platform-plan.md`, `verify.sh` now uses a small Python 3 resolver helper for clearer cross-platform prerequisite handling, the OME render tests now write to temp outputs instead of racing on `deploy/ome/Server.generated.xml`, the Windows bash quickstart test now stubs `go` correctly, and the stream/transcoder gauges now clamp at read/export time so concurrent stop-before-start races do not leak negative state or order-dependent positives.
- Task 6 result: the repo-wide `./scripts/verify.sh` gate now gets all the way through Go tests, architecture checks, dependency checks, and contract invariants. The remaining failure is precise and release-relevant: the local root `.env` used by this workstation does not yet pin real third-party image digests, so `scripts/require-image-digests.sh --env-file .env` stops the gate at the production digest-enforcement step. That is the main remaining blocker before a confident public release tag.
- Task 6 checks:
  - `& 'C:\Program Files\Git\bin\bash.exe' -n scripts/python.sh scripts/check-env-example-placeholders.sh scripts/check-contract-invariants.sh scripts/verify.sh`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/check-no-committed-secrets.sh`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/check-doc-installer-language.sh`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/generate-contract-doc.sh`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/generate-contract-doc.sh --check`
  - `New-Item -ItemType Directory -Force .gocache-ingest | Out-Null; $env:GOCACHE=(Resolve-Path .gocache-ingest).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/ingest -count=1 -timeout=120s`
  - `New-Item -ItemType Directory -Force .gocache-scripts-fix | Out-Null; $env:GOCACHE=(Resolve-Path .gocache-scripts-fix).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./scripts -count=1 -timeout=120s`
  - `New-Item -ItemType Directory -Force .gocache-bitriver-fix | Out-Null; $env:GOCACHE=(Resolve-Path .gocache-bitriver-fix).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./cmd/bitriver -count=1 -timeout=120s`
  - `New-Item -ItemType Directory -Force .gocache-metrics-fix | Out-Null; $env:GOCACHE=(Resolve-Path .gocache-metrics-fix).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/observability/metrics -count=1 -timeout=120s`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh` (current remaining blocker: local `.env` is missing required third-party digest pins for production pull mode)

## Scoped change: second-pass outsider trust and adoption polish

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [ ] Task 1 - Record the trust/adoption polish scope and confirm the remaining rough edges
  - Acceptance criteria:
    - `PLAN.md` captures the narrowed second-pass scope, assumptions, risks, and focused checks before edits.
    - `TASKS.md` lists the ordered public-surface tasks before files are changed.
    - Read-only inspection identifies concrete remaining trust issues (for example unclear README opening, install/support ambiguity, stale repo URLs, or inconsistent release-stage wording).

- [x] Task 2 - Tighten the README and install-facing docs for first-time trust
  - Acceptance criteria:
    - The README makes the project scope, intended operator, and quickest honest first success obvious in the first screenful.
    - Public install docs remove stale or confusing wording, fix repo/link inconsistencies, and feel more like a maintained project than private notes.
    - Public-facing packaging/docs metadata no longer point at the wrong GitHub repository path.

- [x] Task 3 - Strengthen contributor/support/release legitimacy signals
  - Acceptance criteria:
    - The repo has a clear public support path for newcomers and issue-template routing reflects it.
    - Contributor-facing docs are clearer about scope, expectations, and the best-supported path.
    - Release-stage/versioning wording avoids confusing or internally contradictory messaging.

- [x] Task 4 - Run focused validation and record the second-pass outcome
  - Acceptance criteria:
    - The touched docs/support/template surfaces are reviewed after edits.
    - Relevant repo-hygiene checks are rerun and logged.
    - `TASKS.md` records any remaining outsider-trust issues that should still block or temper a public release.

### Execution log (second-pass outsider trust and adoption polish)
- Task 1 complete: read-only review confirmed the remaining trust gaps are mostly presentation mismatches rather than missing mechanics. The top of `README.md` still made a newcomer infer too much about who the project is for and what success looks like, `docs/quickstart.md` still read like an internal operator notebook near the top, public package/docs metadata still referenced `github.com/bitriver-live/bitriver-live` even though this checkout's `origin` points at `github.com/ProhibitedTV/BitRiver-Live`, and the release-stage wording in `docs/production-status.md` still mixed a `v1.0` label into a repo that is otherwise aligned to `v1.2.3`.
- Task 1 checks:
  - `Get-Content README.md`
  - `Get-Content CONTRIBUTING.md`
  - `Get-Content CHANGELOG.md`
  - `Get-Content docs/releases/README.md`
  - `Get-Content docs/quickstart.md`
  - `Get-Content SECURITY.md`
  - `Get-Content docs/production-release.md`
  - `Get-Content docs/versioning.md`
  - `Get-Content .github/RELEASE_NOTES_TEMPLATE.md`
  - `Get-ChildItem .github -Recurse | Select-Object FullName`
  - `Test-Path bitriver-live-banner-text.png; Test-Path SUPPORT.md`
  - `Get-Content docs/releases/v1.2.3-draft.md`
  - `rg -n "â|�" README.md CONTRIBUTING.md docs web/viewer/README.md`
  - `Get-Content docs/testing.md`
  - `Get-Content web/viewer/README.md`
  - `git remote -v`
  - `Get-Content docs/architecture.md`
  - `Get-Content .github/dependabot.yml`
  - `rg -n "github.com/bitriver-live|ProhibitedTV/BitRiver-Live|cross-platform-plan" README.md docs web .github CONTRIBUTING.md SECURITY.md CHANGELOG.md`
  - `Get-Content docs/production-single-host.md`
  - `Get-Content LICENSE`
  - `Get-Content CODE_OF_CONDUCT.md`
  - `rg -n "releases/latest/download|ProhibitedTV/BitRiver-Live|github.com/bitriver-live/bitriver-live" README.md docs deploy .github web cmd scripts`
  - `Get-Content scripts/generate-brew-formula.sh`
  - `Get-Content deploy/installers/nfpm.yaml`
  - `Get-Content .github/ISSUE_TEMPLATE/config.yml`
  - `Get-Content .github/ISSUE_TEMPLATE/bug_report.yml`
  - `Get-Content .github/ISSUE_TEMPLATE/feature_request.yml`
- Task 2 complete: tightened the top of `README.md` around an honest operator fit, a clearer first-success quickstart, and a simpler support boundary, then reshaped the top of `docs/quickstart.md` so install paths, expected first success, and migration notes read like maintained public docs instead of internal notes. This pass also corrected the public GitHub repository path in install/package metadata (`docs/quickstart.md`, `scripts/generate-brew-formula.sh`, and `deploy/installers/nfpm.yaml`) and replaced the misleading architecture/quickstart references to the lab-only cross-platform plan with the supported production-status baseline.
- Task 2 checks:
  - `Get-Content README.md -TotalCount 180`
  - `Get-Content docs/quickstart.md -TotalCount 180`
  - `Get-Content docs/architecture.md -TotalCount 60`
  - `Get-Content deploy/installers/nfpm.yaml`
  - `Get-Content scripts/generate-brew-formula.sh`
- Task 3 complete: added a root `SUPPORT.md`, linked it from `README.md` and `CONTRIBUTING.md`, added GitHub issue-template contact links for support/security, clarified maintainer priorities in `CONTRIBUTING.md`, tightened release-note hygiene guidance in `docs/releases/README.md`, and changed the confusing `v1.0` release-stage label in `docs/production-status.md` to a clearer `stable` stage.
- Task 3 checks:
  - `Get-Content SUPPORT.md`
  - `Get-Content CONTRIBUTING.md`
  - `Get-Content .github/ISSUE_TEMPLATE/config.yml`
  - `Get-Content docs/production-status.md -TotalCount 120`
  - `Get-Content docs/releases/README.md`
- Task 4 complete: the second-pass trust/adoption surfaces were re-read after edits, the stale public repo URL and stale link grep returned no remaining matches, the docs installer-language check and committed-secret guard both passed, and the updated brew-formula shell script passed `bash -n` once rerun outside the sandbox after a Windows Git Bash signal-pipe permission failure in the sandboxed attempt.
- Task 4 result: the remaining public-release caution is still operational rather than presentational. The repo now reads more intentionally to outsiders, but the final release gate still depends on a production-grade `.env` with pinned third-party image digests so `./scripts/verify.sh` can go fully green end to end.
- Task 4 checks:
  - `Get-Content README.md -TotalCount 180`
  - `Get-Content docs/quickstart.md -TotalCount 180`
  - `Get-Content SUPPORT.md`
  - `Get-Content CONTRIBUTING.md`
  - `Get-Content .github/ISSUE_TEMPLATE/config.yml`
  - `Get-Content docs/production-status.md -TotalCount 120`
  - `Get-Content docs/releases/README.md`
  - `rg -n "github.com/bitriver-live/bitriver-live|docs/cross-platform-plan.md|### v1.0" README.md docs CONTRIBUTING.md .github deploy scripts` (no matches; `rg` exited 1)
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/check-doc-installer-language.sh`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/check-no-committed-secrets.sh`
  - `& 'C:\Program Files\Git\bin\bash.exe' -n scripts/generate-brew-formula.sh` (rerun outside sandbox after a Windows Git Bash permission failure in the sandboxed attempt)

## Scoped change: local sync and release-ready UI/UX polish

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the local-sync and UI/UX polish scope before changes
  - Acceptance criteria:
    - `PLAN.md` captures the redeploy + UI/UX scope, assumptions, risks, and focused validation commands.
    - `TASKS.md` lists the ordered sync, implementation, and verification tasks before edits or runtime changes begin.
    - Read-only inspection identifies the concrete UI clutter to remove instead of relying on generic polish language.

- [x] Task 2 - Fast-forward the local checkout to merged `main` and redeploy app services
  - Acceptance criteria:
    - The current branch is aligned to the merged `origin/main` commit without discarding local work.
    - `bitriver-live` and `viewer` are rebuilt/recreated from the updated checkout using the canonical Compose contract.
    - Post-redeploy checks confirm `localhost:8080` is serving the refreshed app.

- [x] Task 3 - Simplify the viewer homepage and navigation around watch-first adoption
  - Acceptance criteria:
    - The homepage hero and discovery rows prioritize "watch something now" and "browse" without redundant editorial copy or unnecessary controls.
    - Navigation keeps the primary actions obvious and removes low-value clutter for first-time users.
    - Any touched styles or components stay consistent across desktop and mobile.

- [x] Task 4 - Simplify creator onboarding/live setup around first success
  - Acceptance criteria:
    - Creator onboarding focuses on the shortest path to a first live stream and viewer link.
    - Manual checklist friction and verbose explanatory text are reduced where the UI can infer the next step.
    - Live setup still preserves the critical ingest/preview/share flow.

- [x] Task 5 - Run focused viewer validation and record the redeployed outcome
  - Acceptance criteria:
    - Targeted viewer/unit tests pass for the touched flows.
    - Focused Playwright coverage passes for homepage/channel/live-setup UX.
    - `TASKS.md` records the redeploy command(s), verification result, and any remaining UI risks.

### Execution log (local sync and release-ready UI/UX polish)
- Task 1 complete: read-only analysis confirmed two separate needs before coding. First, the local branch `codex/release_test` was clean but 12 commits behind `origin/main`, so a fast-forward sync is needed before redeploying. Second, the product UX is currently more complex than the target audience needs: the homepage stacks multiple editorial/info panels and carousel controls, the navbar keeps extra preference/setup affordances visible, and the creator onboarding/live pages read like internal operator workspaces with too many manual confirmations and explanatory paragraphs for a "get your Twitch-like stream running" flow.
- Task 1 checks:
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live status --short`
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live branch --show-current`
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live log -1 --oneline`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `Get-ChildItem web/viewer -Force`
  - `rg --files web/viewer | Select-Object -First 200`
  - `Get-Content web/viewer/AGENTS.md`
  - `Get-Content web/viewer/app/page.tsx`
  - `Get-Content web/viewer/components/ViewerShell.tsx`
  - `Get-Content web/viewer/components/Navbar.tsx`
  - `Get-Content web/viewer/app/creator/getting-started/page.tsx`
  - `Get-Content web/viewer/app/getting-started/page.tsx`
  - `git fetch origin --prune`
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live rev-parse HEAD`
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live rev-parse origin/main`
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live rev-list --left-right --count HEAD...origin/main`
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live log --oneline --decorate --graph --max-count 12 HEAD origin/main`
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live status --short --branch`
  - `Get-Content web/viewer/app/directory-page-shell.tsx`
  - `Get-Content web/viewer/app/directory-view.tsx`
  - `Get-Content web/viewer/components/FeaturedChannel.tsx`
  - `Get-Content web/viewer/components/LiveNowGrid.tsx`
  - `Get-Content web/viewer/components/CategoryRail.tsx`
  - `Get-Content web/viewer/styles/home.css`
  - `Get-Content -LiteralPath 'web/viewer/app/creator/live/[channelId]/page.tsx'`
  - `Get-Content -LiteralPath 'web/viewer/app/creator/live/[channelId]/layout.tsx'`
  - `Get-Content web/viewer/components/UploadManager.tsx`
  - `Get-Content web/viewer/app/creator/page.tsx`
  - `Get-Content web/viewer/components/ChannelRail.tsx`
- Task 2 complete: fast-forwarded the local checkout to `origin/main` with `git pull --ff-only origin main`, then rebuilt the app services with the canonical Compose contract. The first `docker compose ... up -d --build bitriver-live viewer` timed out before returning cleanly on this host, so follow-up status checks plus a final rebuild/recreate were used to confirm the viewer service picked up the new UI build now running on `localhost:8080`.
- Task 2 checks:
  - `git fetch origin --prune`
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live rev-list --left-right --count HEAD...origin/main`
  - `git pull --ff-only origin main`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer` (timed out before the CLI returned on this Windows host, but the rebuild progressed)
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer` (final viewer rebuild before live verification; timed out again before returning cleanly)
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
- Task 3 complete: simplified the viewer homepage around the two jobs an outsider understands immediately: watch what is on right now or start their own stream. The featured rail no longer exposes back/next/play controls, the following rail no longer repeats summary/error text or a footnote, homepage sections stay hidden when they would only show empty filler, and the hero now chooses between `Watch live now`, `Browse channels`, and `Start creator setup` based on what content the install actually has.
- Task 3 checks:
  - `Get-Content web/viewer/app/directory-view.tsx`
  - `Get-Content web/viewer/components/FeaturedChannel.tsx`
  - `Get-Content web/viewer/components/FollowingSidebar.tsx`
  - `Get-Content web/viewer/components/CategoryRail.tsx`
  - `Get-Content web/viewer/components/ChannelRail.tsx`
  - `Get-Content web/viewer/components/LiveNowGrid.tsx`
- Task 4 complete: simplified the creator surfaces down to the shortest believable first-success path. `creator/getting-started` now focuses on three steps (`Choose your channel`, `Copy your stream settings`, `Go live and share the channel`) instead of a manual checklist, and the live setup page now keeps the flow to four sections (`Channel`, `Stream settings`, `Go live`, `Share`) with direct OBS copy actions, a single signal/preview area, and less maintainer-style explanatory text.
- Task 4 checks:
  - `Get-Content web/viewer/app/creator/getting-started/page.tsx`
  - `Get-Content -LiteralPath 'web/viewer/app/creator/live/[channelId]/page.tsx'`
  - `Get-Content web/viewer/__tests__/creatorGettingStartedPage.test.tsx`
  - `Get-Content web/viewer/__tests__/creatorLivePage.test.tsx`
  - `Get-Content web/viewer/tests/creator-live-setup.spec.ts`
- Task 5 complete: reran the focused viewer/unit suite, the browser smoke, and the viewer production build, then confirmed the rebuilt local viewer container is serving the updated homepage on `localhost:8080/viewer`. The live stack currently renders the cleaner empty-install homepage state because this local environment has no live directory content, which is the sensible outcome for the new watch-first/creator-first logic.
- Task 5 checks:
  - `npm.cmd --prefix web/viewer run test -- creatorGettingStartedPage.test.tsx creatorLivePage.test.tsx directoryPage.test.tsx navigation.test.ts viewerShell.test.tsx`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
  - `npx.cmd playwright test tests/homepage-layout.spec.ts tests/channel.spec.ts tests/creator-live-setup.spec.ts --reporter=list`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode } catch { $_.Exception.Response | Select-Object StatusCode }`
  - `(Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer).Content | Select-String -Pattern 'Launch your first self-hosted channel|Start creator setup|Browse directory|Full directory' -AllMatches`
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer/creator/getting-started -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode,Headers } catch { $_.Exception.Response | Select-Object StatusCode,Headers }`

## Scoped change: viewer auth-overlay cleanup

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the auth-overlay cleanup scope before editing
  - Acceptance criteria:
    - `PLAN.md` captures the auth-overlay scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered dialog cleanup and verification tasks before code edits begin.
    - Read-only inspection identifies where the raw `/viewer` copy and duplicate sign-in controls are rendered.

- [x] Task 2 - Clean up the sign-in overlay copy and mode controls
  - Acceptance criteria:
    - The auth dialog no longer shows raw redirect paths like `/viewer` as the visible return-context label.
    - When self-signup is unavailable, the dialog no longer renders a redundant standalone `Sign in` mode tab above the `Sign in` submit action.
    - Existing sign-in, sign-up, MFA, and redirect behavior remain intact.

- [x] Task 3 - Run focused viewer verification and record the result
  - Acceptance criteria:
    - Focused auth-dialog/auth-hook tests pass for the touched flow.
    - Viewer lint is rerun.
    - `TASKS.md` records the verification results and any remaining host blocker if the broader viewer gate cannot complete.

### Execution log (viewer auth-overlay cleanup)
- Task 1 complete: recorded the scope in `PLAN.md` and `TASKS.md` before editing, then confirmed in read-only inspection that `web/viewer/components/auth/AuthDialog.tsx` renders the raw `authRedirectTo` value in the route summary and always shows a `Sign in` tab even when sign-up is disabled, which is what creates the awkward `/viewer` label plus duplicate sign-in controls.
- Task 1 checks:
  - `Get-Content PLAN.md | Select-Object -Last 30`
  - `Get-Content TASKS.md | Select-Object -Last 40`
  - `Get-Content web/viewer/components/auth/AuthDialog.tsx`
  - `Get-Content web/viewer/hooks/useAuth.tsx`
  - `Get-Content web/viewer/__tests__/authDialog.test.tsx`
- Task 2 complete: replaced the raw route label with a friendlier return-summary in `AuthDialog`, removed the redundant auth-mode tab strip, and kept the existing in-form sign-in/sign-up switches plus redirect behavior intact. The sign-in overlay now shows human-facing labels like `Viewer home` instead of `/viewer`, and signed-out installs without public signup now surface only the real `Sign in` action.
- Task 2 checks:
  - `Get-Content web/viewer/components/auth/AuthDialog.tsx`
  - `Get-Content web/viewer/__tests__/authDialog.test.tsx`
- Task 3 complete: reran focused auth-dialog/auth-hook coverage and viewer lint successfully, then ran `./scripts/verify.sh --viewer`. The broader verify gate progressed through the Go and contract checks and stopped only at the existing production image-digest requirement in `.env`, which is outside this UI fix. After verification, the local `bitriver-live` and `viewer` services were explicitly recreated so the running stack picked up the auth-dialog update, and `http://localhost:8080/viewer` returned `200`.
- Task 3 checks:
  - `npm.cmd --prefix web/viewer run test -- authDialog.test.tsx useAuth.test.tsx`
  - `npm.cmd --prefix web/viewer run lint`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh --viewer` (blocked only by missing required `BITRIVER_*_IMAGE_DIGEST` values in `.env`)
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer` (timed out before the CLI returned on this host)
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --force-recreate --no-deps bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode } catch { $_.Exception.Response | Select-Object StatusCode }`

## Scoped change: viewer auth-dialog return-summary removal

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the follow-up auth-dialog cleanup scope before editing
  - Acceptance criteria:
    - `PLAN.md` captures the narrower follow-up scope, assumptions, risks, and verification commands.
    - `TASKS.md` lists the ordered remove-test-redeploy steps before code edits begin.
    - Read-only inspection confirms the unwanted text box is still the `auth-overlay__route` block in `AuthDialog`.

- [x] Task 2 - Remove the sign-in dialog return-summary box
  - Acceptance criteria:
    - The signed-out auth dialog no longer renders the `Continue where you left off` panel or any viewer-route summary text.
    - Existing sign-in, sign-up, MFA, and duplicate-sign-in cleanup behavior remain intact.
    - Focused auth-dialog tests are updated to assert the simpler presentation.

- [x] Task 3 - Verify and redeploy the local viewer stack
  - Acceptance criteria:
    - Focused auth-dialog/auth-hook tests pass.
    - Viewer lint is rerun.
    - The local `bitriver-live` and `viewer` services are rebuilt/recreated and `http://localhost:8080/viewer` responds afterward.

### Execution log (viewer auth-dialog return-summary removal)
- Task 1 complete: recorded the follow-up scope in `PLAN.md` and `TASKS.md`, then confirmed in read-only inspection that the extra text box the user still sees is the `auth-overlay__route` block in `web/viewer/components/auth/AuthDialog.tsx`, along with the focused test coverage added in the prior pass.
- Task 1 checks:
  - `Get-Content PLAN.md | Select-Object -Last 30`
  - `Get-Content TASKS.md | Select-Object -Last 40`
  - `Get-Content web/viewer/components/auth/AuthDialog.tsx`
  - `Get-Content web/viewer/__tests__/authDialog.test.tsx`
- Task 2 complete: removed the entire `auth-overlay__route` block and the helper code that generated its return-summary copy, leaving the sign-in dialog focused only on the auth form/actions. Updated the focused auth-dialog test to assert that the box and its text do not render anymore.
- Task 2 checks:
  - `Get-Content web/viewer/components/auth/AuthDialog.tsx`
  - `Get-Content web/viewer/__tests__/authDialog.test.tsx`
- Task 3 complete: reran the focused auth-dialog/auth-hook tests and viewer lint successfully, rebuilt the local `bitriver-live` and `viewer` services with the canonical Compose contract, and confirmed `http://localhost:8080/viewer` returned `200` after the new viewer container started.
- Task 3 checks:
  - `npm.cmd --prefix web/viewer run test -- authDialog.test.tsx useAuth.test.tsx`
  - `npm.cmd --prefix web/viewer run lint`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode } catch { $_.Exception.Response | Select-Object StatusCode }`

## Scoped change: OME container log triage

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the OME log-triage scope and capture current runtime evidence
  - Acceptance criteria:
    - `PLAN.md` captures the OME log-analysis scope, assumptions, risks, and verification commands.
    - `TASKS.md` lists the ordered capture, diagnosis, and follow-up tasks before log collection begins.
    - Current Compose status plus recent `ome`-related logs are captured from the local install.

- [x] Task 2 - Separate actionable repo-owned OME findings from upstream or host noise
  - Acceptance criteria:
    - The dominant `bitriver-ome` log patterns are grouped by severity and probable cause.
    - Each actionable finding is traced to the closest owning repo surface (generated config, compose/env input, script, or runtime code).
    - Non-actionable upstream/host noise is called out clearly.

- [x] Task 3 - Implement and verify the smallest high-confidence fix if warranted
  - Acceptance criteria:
    - A change is made only if the OME log evidence points to a narrow repo-owned issue.
    - Relevant focused verification is rerun after the change.
    - `TASKS.md` records either the validated fix or an explicit note that this pass remained diagnosis-only.

### Execution log (OME container log triage)
- Task 1 complete: recorded the scoped OME log-triage plan in `PLAN.md` and `TASKS.md`, then captured the current `ome`, `ome-config`, and `ome-health-token-check` runtime state from the local Compose install. `bitriver-ome` is healthy, `ome-config` rendered successfully, and `ome-health-token-check` confirmed the rendered `<Managers><API><AccessToken>` matches the canonical runtime token source.
- Task 1 checks:
  - `Get-Content PLAN.md | Select-Object -Last 30`
  - `Get-Content TASKS.md | Select-Object -Last 35`
-  - `docker compose --env-file .env -f deploy/docker-compose.yml ps ome ome-config ome-health-token-check`
-  - `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=200 ome ome-config ome-health-token-check`
- Task 2 complete: separated the two dominant `bitriver-ome` log loops. The `127.0.0.1 GET /` `401` entries come from the Compose OME liveness probe and are noisy but currently by design under the deployment contract. The `172.18.0.7 GET /healthz` `401 Invalid credential` entries map back to the BitRiver API container calling ingest `HealthChecks()` on each `/healthz` request, which is repo-owned runtime behavior and a good candidate for a narrow code fix.
- Task 2 checks:
-  - `Get-Content deploy/docker-compose.yml`
-  - `Get-Content deploy/ome/Server.generated.xml`
-  - `Get-Content internal/ingest/http_controller.go`
-  - `Get-Content internal/config/ingest.go`
-  - `Get-Content internal/api/handlers.go`
-  - `Get-Content internal/api/status_helpers.go`
-  - `Get-Content internal/storage/storage.go`
-  - `rg -n "HealthChecks|IngestHealth|LastIngestHealth|BITRIVER_INGEST_HEALTH|OME_API_TOKEN|AccessToken" internal cmd deploy docs scripts`
- Task 3 complete: updated the lightweight API `/healthz` path to reuse the last recorded ingest-health snapshot instead of live-probing OME on every container healthcheck, while keeping `/api/status` on the live refresh path for operator diagnostics. Added focused handler coverage for cached-vs-live ingest health behavior, rebuilt the local API stack, and rechecked fresh OME logs.
- Task 3 checks:
-  - `gofmt -w internal/api/handlers.go internal/api/status_helpers.go internal/api/handlers_test.go`
-  - `$env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; $env:GOCACHE=(Join-Path (Get-Location) '.gocache'); go test ./internal/api -count=1 -timeout=120s`
-  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
-  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/healthz -ErrorAction Stop | Select-Object StatusCode } catch { $_.Exception.Response | Select-Object StatusCode }`
-  - `Start-Sleep -Seconds 70; docker compose --env-file .env -f deploy/docker-compose.yml logs --since=120s ome | Select-String -Pattern '172\.18\.0\.7|127\.0\.0\.1|Invalid credential|Authorization header is required'`
- Result: the repeated API-owned `172.18.0.7 GET /healthz` `401 Invalid credential` lines stopped appearing after the rebuild. The remaining `127.0.0.1 GET /` `401 Authorization header is required` lines are still produced by the Compose OME liveness probe and remain a deployment-contract decision rather than an API runtime bug.

## Scoped change: post-install docker log triage

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the log-triage scope and capture current runtime evidence
  - Acceptance criteria:
    - `PLAN.md` captures the post-install log-analysis scope, assumptions, risks, and verification commands.
    - `TASKS.md` lists the ordered capture, triage, and follow-up tasks before Docker log collection begins.
    - Current Compose status and recent logs are captured from the local install.

- [x] Task 2 - Separate actionable repo-owned findings from host or third-party noise
  - Acceptance criteria:
    - The main error/warning patterns are grouped by service and severity.
    - Each actionable finding is traced to the closest owning code/config/docs surface in this repo.
    - Non-actionable host/vendor noise is called out clearly so it is not mistaken for a code defect.

- [x] Task 3 - Implement and verify the smallest high-confidence repo fix if warranted
  - Acceptance criteria:
    - A code/doc/config change is made only if the logs point to a reproducible repo-owned issue with a narrow fix.
    - Relevant focused verification is rerun after the change.
    - `TASKS.md` records either the validated fix or an explicit note that this pass remained analysis-only.

### Execution log (post-install docker log triage)
- Task 1 complete: recorded the scoped log-triage plan in `PLAN.md` and `TASKS.md`, then captured the current Compose status plus recent logs from `bitriver-live`, `viewer`, `postgres`, `redis`, and `transcoder-public`. The log pull also exposed a host quirk: a broader `docker compose ... logs ... ome srs ...` command hit a Windows Docker pipe access denial, but the already-approved targeted log commands succeeded and produced enough evidence to continue.
- Task 1 checks:
  - `Get-Content PLAN.md | Select-Object -Last 30`
  - `Get-Content TASKS.md | Select-Object -Last 40`
  - `git status --short PLAN.md TASKS.md`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps -a`
  - `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 postgres redis transcoder-public`
  - `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=200 bitriver-live viewer postgres redis ome srs transcoder-public` (blocked on this host by Windows Docker pipe access denial)
- Task 2 complete: grouped the captured messages into three buckets. The highest-confidence repo bug was `authMiddleware` treating only `/api/directory` as public while still 401-ing public subroutes like `/api/directory/featured`, `/api/directory/recommended`, `/api/directory/live`, `/api/directory/trending`, and `/api/directory/categories`. Separate findings remain for repeated missing-table errors against Postgres-backed runtime tables and a `transcoder-public` nginx capability mismatch (`setgid(101) failed`) that would require deployment-contract follow-up before changing Compose hardening.
- Task 2 checks:
  - `rg -n "authMiddleware|/api/directory|DirectoryFeatured|DirectoryRecommended|DirectoryLive|DirectoryTrending|DirectoryCategories" internal/server/server.go internal/api/channels_directory_handlers.go`
  - `Get-Content internal/server/server.go | Select-Object -Skip 680 -First 60`
  - `Get-Content deploy/nginx/transcoder-public.conf | Select-Object -First 40`
  - `Get-Content deploy/docker-compose.yml | Select-Object -Skip 532 -First 25`
- Task 3 complete: updated `authMiddleware` so public directory subroutes stay optional-auth while `/api/directory/following` remains protected, then added focused middleware coverage for anonymous and expired-session requests against those routes. The targeted `internal/server` test package passed after formatting the touched files, and a local app rebuild confirmed the live stack now returns `200` for anonymous `/api/directory/featured` while keeping anonymous `/api/directory/following` protected.
- Task 3 checks:
  - `gofmt -w internal/server/server.go internal/server/server_test.go`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/api/directory/featured -ErrorAction Stop | Select-Object StatusCode,Content } catch { $_.Exception.Response | Select-Object StatusCode }`
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/api/directory/following -ErrorAction Stop | Select-Object StatusCode,Content } catch { $_.Exception.Response | Select-Object StatusCode }`
  - `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=40 bitriver-live viewer` (blocked on this host by Windows Docker pipe access denial after the rebuild; route probes above were used as the runtime confirmation)

## Scoped change: quickstart first-run wizard controls

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the quickstart wizard scope before editing runtime code
  - Acceptance criteria:
    - `PLAN.md` captures the guided quickstart/env-init scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered implementation and verification tasks before code edits begin.
    - Read-only inspection identifies the current quickstart/env-init prompt gap and the closest reusable wizard/setup surfaces already in the repo.

- [x] Task 2 - Implement the guided quickstart/env-init wizard
  - Acceptance criteria:
    - `cmd/bitriver env init` supports an opt-in wizard mode that prompts for key first-run controls and writes them into `.env`.
    - `cmd/bitriver quickstart` can invoke the same wizard path before validation/orchestration without breaking the existing non-interactive flow.
    - Focused `cmd/bitriver` tests cover the wizard prompts, env writes, and quickstart handoff.

- [x] Task 3 - Document and verify the new guided flow
  - Acceptance criteria:
    - Operator docs mention the new wizard-driven quickstart path and explain what it configures.
    - Relevant CLI/backend tests are rerun and recorded here.
    - Broader verification (`go test ./...`, compose config, `./scripts/verify.sh`) is attempted and any host/environment blocker is captured explicitly.

### Execution log (quickstart first-run wizard controls)
- Task 1 complete: recorded the new scope in `PLAN.md`/`TASKS.md` after read-only inspection confirmed the current source quickstart only prompts for `BITRIVER_LIVE_ADMIN_EMAIL`, while the repo already has a richer Ubuntu installer wizard (`deploy/install/wizard.sh`) and a post-install control-centre setup wizard that can help shape the new first-run CLI flow.
- Task 1 checks:
  - `Get-ChildItem -Force`
  - `Get-ChildItem -Recurse -Filter AGENTS.md | Select-Object -ExpandProperty FullName`
  - `Get-Content SPEC.md`
  - `rg -n "quickstart|wizard|setup" SPEC.md PLAN.md TASKS.md README.md docs scripts cmd deploy web -g '!**/node_modules/**'`
  - `Get-Content deploy/install/wizard.sh`
  - `Get-Content cmd/bitriver/commands_env_compose.go`
  - `Get-Content cmd/bitriver/env_validation_helpers.go`
  - `Get-Content cmd/server/setup_manager.go`
- Task 2 complete: added a shared guided wizard path to `cmd/bitriver env init`, threaded the same opt-in flag through `quickstart`, and kept the default non-interactive flow unchanged for scripted use. The wizard now captures operator-facing first-run values (admin email, viewer/API URLs, API port, OME bind/public host, transcoder URL, and self-signup) while leaving secret generation automatic. Focused `cmd/bitriver` coverage now verifies the interactive env write, the non-interactive guardrail, and the quickstart-to-env-init handoff.
- Task 2 checks:
  - `gofmt -w cmd/bitriver/env_validation_helpers.go cmd/bitriver/commands_env_compose.go cmd/bitriver/main_test.go`
  - `go test ./cmd/bitriver -count=1` (initial host cache failure: `Access is denied` under `%LOCALAPPDATA%\\go-build`)
  - `$env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./cmd/bitriver -count=1`
- Task 3 complete: documented the new `--wizard` flow in the README and quickstart guide so operators can discover the guided path without changing the existing default examples. Reran the full Go suite with the repo-local cache, validated Compose config with the explicit root `.env`, and attempted the repo's `./scripts/verify.sh` gate. The broader verify run reached the production third-party digest enforcement step and failed only because the local `.env` still omits the required third-party `@sha256:` digest pins, which is an existing host/configuration issue outside this CLI/docs change.
- Task 3 checks:
  - `rg -n -- "--wizard|first-run wizard|guided quickstart settings|setup-wizard style control" README.md docs/quickstart.md cmd/bitriver/commands_env_compose.go cmd/bitriver/env_validation_helpers.go`
  - `$env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./... -count=1 -timeout=120s`
  - `docker compose -f deploy/docker-compose.yml config` (fails on this host unless `--env-file .env` is supplied explicitly; Compose did not pick up the root `.env` automatically here)
  - `docker compose --env-file .env -f deploy/docker-compose.yml config`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh` (blocked only by missing third-party image digest pins in `.env`: `BITRIVER_REDIS_IMAGE_DIGEST`, `BITRIVER_POSTGRES_IMAGE_DIGEST`, `BITRIVER_SRS_IMAGE_DIGEST`, `BITRIVER_OME_IMAGE_DIGEST`, `BITRIVER_NGINX_IMAGE_DIGEST`, `BITRIVER_ALPINE_3_IMAGE_DIGEST`, `BITRIVER_ALPINE_3_19_IMAGE_DIGEST`, `BITRIVER_DEBIAN_IMAGE_DIGEST`)
  - `git status --short`

## Scoped change: local redeploy refresh

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the local redeploy scope before touching the running stack
  - Acceptance criteria:
    - `PLAN.md` captures the local redeploy scope, assumptions, risks, and verification commands.
    - `TASKS.md` lists the ordered redeploy and verification steps before runtime actions begin.
    - Read-only context confirms the intended compose contract and current env file remain the repo-root defaults.

- [x] Task 2 - Redeploy the local Compose stack and verify service health
  - Acceptance criteria:
    - The relevant local services are recreated from the current checkout.
    - Post-redeploy Compose status is captured.
    - Local HTTP verification is attempted and any blocker is recorded explicitly.

### Execution log (local redeploy refresh)
- Task 1 complete: recorded the redeploy scope in `PLAN.md` and `TASKS.md` before running Docker commands. This refresh is operational only; the current checkout still points at the canonical root `.env` plus `deploy/docker-compose.yml` contract.
- Task 1 checks:
  - `Get-Content PLAN.md | Select-Object -Last 30`
  - `Get-Content TASKS.md | Select-Object -Last 40`
- Task 2 complete: ran a targeted local Compose refresh against `bitriver-live` and `viewer`, which also rebuilt/refreshed the dependent config and media services needed for the stack to settle cleanly. Post-redeploy Compose status shows the stack healthy, and `http://localhost:8080/viewer` returned `200`.
- Task 2 checks:
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `rg -n "^BITRIVER_LIVE_PORT=|^NEXT_PUBLIC_VIEWER_URL=" .env`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode } catch { $_.Exception.Response | Select-Object StatusCode }`

## Scoped change: viewer auth-dialog sign-in copy simplification

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the follow-up sign-in copy cleanup scope before editing
  - Acceptance criteria:
    - `PLAN.md` captures the narrower sign-in copy cleanup scope, assumptions, risks, and verification commands.
    - `TASKS.md` lists the ordered edit, verification, and redeploy steps before code edits begin.
    - Read-only inspection confirms the remaining unwanted text is the sign-in form's inner heading and muted paragraph in `AuthDialog`.

- [x] Task 2 - Remove the redundant sign-in reassurance copy
  - Acceptance criteria:
    - The signed-out sign-in form no longer renders `Sign in without losing your place` or the supporting reassurance paragraph.
    - Existing sign-in fields, actions, sign-up path, MFA flow, and redirect behavior remain intact.
    - Focused auth-dialog tests are updated to assert the simpler sign-in presentation.

- [x] Task 3 - Verify and redeploy the local viewer stack
  - Acceptance criteria:
    - Focused auth-dialog/auth-hook tests pass.
    - Viewer lint is rerun.
    - The local `bitriver-live` and `viewer` services are rebuilt/recreated and `http://localhost:8080/viewer` responds afterward.

### Execution log (viewer auth-dialog sign-in copy simplification)
- Task 1 complete: recorded the follow-up scope in `PLAN.md` and `TASKS.md`, then confirmed in read-only inspection that the remaining noisy copy the user called out is the sign-in form's inner heading (`Sign in without losing your place`) and muted reassurance paragraph in `web/viewer/components/auth/AuthDialog.tsx`.
- Task 1 checks:
  - `Get-Content PLAN.md | Select-Object -Last 30`
  - `Get-Content TASKS.md | Select-Object -Last 40`
  - `Get-Content web/viewer/components/auth/AuthDialog.tsx`
  - `Get-Content web/viewer/__tests__/authDialog.test.tsx`
- Task 2 complete: removed the sign-in form's inner reassurance heading and paragraph so the signed-out dialog now goes straight from the main title into the email/password fields and actions. Updated focused auth-dialog coverage to assert that the removed copy stays gone.
- Task 2 checks:
  - `Get-Content web/viewer/components/auth/AuthDialog.tsx`
  - `Get-Content web/viewer/__tests__/authDialog.test.tsx`
- Task 3 complete: reran the focused auth-dialog/auth-hook tests and viewer lint successfully, rebuilt the local `bitriver-live` and `viewer` services with the canonical Compose contract, and confirmed `http://localhost:8080/viewer` returned `200` after the new viewer container started.
- Task 3 checks:
  - `npm.cmd --prefix web/viewer run test -- authDialog.test.tsx useAuth.test.tsx`
  - `npm.cmd --prefix web/viewer run lint`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build bitriver-live viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps`
  - `try { Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer -MaximumRedirection 0 -ErrorAction Stop | Select-Object StatusCode } catch { $_.Exception.Response | Select-Object StatusCode }`
