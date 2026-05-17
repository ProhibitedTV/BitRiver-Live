## Scoped change: issue #1227 following sidebar focus

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the route-aware following plan
  - Acceptance criteria:
    - `PLAN.md` captures issue #1227 scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists ordered implementation and validation tasks before source edits begin.
    - The read-only pass identifies shell placement, following state components, CSS grid behavior, and existing unit/Playwright coverage.

- [x] Task 2 - Make following placement route-aware
  - Acceptance criteria:
    - Discovery routes keep a desktop following sidebar and mobile following entry.
    - Channel watch and creator routes render without following side chrome.
    - Wide no-sidebar routes use a one-column content layout instead of the desktop shell grid.

- [x] Task 3 - Keep following states useful but lightweight
  - Acceptance criteria:
    - Guest, empty, error, loading, and populated states continue to render through shared following components.
    - Empty/unauthenticated following states do not add extra watch-page chrome.
    - Existing sign-in and retry behavior remains intact.

- [x] Task 4 - Update tests and browser coverage
  - Acceptance criteria:
    - Unit tests cover sidebar visibility by route and focus management when the drawer is available.
    - Following state tests still cover guest, empty, error, and populated states.
    - Playwright covers desktop discovery rail alignment and mobile watch pages without the following drawer entry.

- [x] Task 5 - Run validation and record results
  - Acceptance criteria:
    - Focused Jest, lint, build, targeted Playwright, and the repo viewer gate are run or blockers are recorded.
    - `TASKS.md` records command results and any residual warnings.

- [x] Task 6 - Fix PR #1234 Ubuntu smoke follow-up
  - Acceptance criteria:
    - The quickstart smoke prepares the API data bind mount for Linux CI.
    - The test-only Compose override lets `bitriver-live` write repo-backed smoke data without changing the deployment contract.
    - Syntax/diff checks pass locally and a fresh CI run confirms the Ubuntu gate.

- [x] Task 7 - Avoid CI host port collision in quickstart smoke
  - Acceptance criteria:
    - The latest PR #1234 failure annotation is recorded with its root cause.
    - The generated smoke `.env` publishes the API/viewer on a less collision-prone host port while preserving container port `8080` and the real deployment contract.
    - Regression, syntax, and script checks pass locally before pushing a follow-up.

- [x] Task 8 - Fix remaining quickstart entrypoint sanity failures
  - Acceptance criteria:
    - ShellCheck no longer reports the indirectly invoked `deploy-smoke.sh` callbacks.
    - `scripts/quickstart.ps1` keeps the static quickstart contract expected by CI without taking over Docker orchestration from the Go CLI.
    - Syntax/static/script tests pass locally.

- [x] Task 9 - Fix local composite-action checkout ordering
  - Acceptance criteria:
    - CI jobs that use `.github/actions/setup-go` or `.github/actions/setup-node-viewer` check out the repository first.
    - Existing Ubuntu verify and quickstart smoke behavior stays unchanged.
    - Workflow edits remain limited to setup ordering.

- [-] Task 10 - Validate and push the CI follow-up
  - Acceptance criteria:
    - Focused syntax, PowerShell static, Go script tests, and diff checks pass locally or blockers are recorded.
    - Changes are committed/pushed to PR #1234.
    - A fresh PR #1234 CI run is checked.

### Execution log (issue #1227 following sidebar focus)
- Task 1 complete: confirmed PR #1233 merged and issue #1220 was already closed by GitHub, fast-forwarded local `main` to merge commit `7fa76a18`, selected open issue #1227, and created branch `codex/issue-1227-following-focus`.
- Task 1 checks:
  - `git status --short --branch`
  - GitHub connector: fetched PR #1233 and issue #1220 state.
  - GitHub connector: listed recent issues and fetched issues #1224, #1225, and #1227.
  - `git fetch origin --prune`
  - `git pull --ff-only origin main`
  - `git checkout -b codex/issue-1227-following-focus`
  - `rg --files -g AGENTS.md`
  - `Get-Content web/viewer/AGENTS.md`
  - `Get-Content web/viewer/components/ViewerShell.tsx`
  - `Get-Content web/viewer/components/FollowingSidebar.tsx`
  - `Get-Content web/viewer/components/following/FollowingState.tsx`
  - `Get-Content web/viewer/components/following/useFollowingChannels.ts`
  - `Get-Content web/viewer/__tests__/followingSidebar.test.tsx`
  - `Get-Content web/viewer/__tests__/viewerShell.test.tsx`
  - `rg -n "viewer-shell|viewer-sidebar|following-sidebar|following-state|following" web/viewer/styles/globals.css`
- Task 2 complete: `ViewerShell` now enables the following sidebar/drawer only on discovery routes (`/`, `/browse`, and `/videos`) and applies a `viewer-shell--following-disabled` layout on watch/creator-style routes. The wide CSS grid now has a one-column override when the sidebar is disabled.
- Task 3 complete: following state rendering still flows through the shared loading, guest, empty, error, and populated components; signed-out and empty states no longer add any following chrome to channel watch routes because the shell does not mount the drawer there.
- Task 2/3 checks:
  - `npm.cmd --prefix web/viewer run test -- viewerShell.test.tsx followingSidebar.test.tsx followingStatePresentation.test.tsx --silent` - passed, 3 suites and 12 tests.
  - `npm.cmd --prefix web/viewer run test -- viewerShell.test.tsx followingSidebar.test.tsx followingStatePresentation.test.tsx --silent` - passed after adding creator-route coverage, 3 suites and 13 tests.
- Task 4 checks so far:
  - `npm.cmd --prefix web/viewer run test -- channelPage.test.tsx --silent` - passed, 1 suite and 19 tests.
- Task 4 complete: unit coverage now asserts the shell hides following side chrome on channel watch and creator workflow routes, the channel mobile browser spec asserts watch pages no longer expose `Show following`, and homepage browser coverage covers guest, empty, and populated following sidebar/drawer states.
- Task 5 complete: focused unit, channel, lint, build, targeted Playwright, diff-check, and repo viewer gate were run. The repo viewer gate still stops on this host's missing Python prerequisite before viewer checks.
- Task 5 checks:
  - `npm.cmd --prefix web/viewer run lint` - passed.
  - `npm.cmd --prefix web/viewer run build` - passed with existing Browserslist and Next.js client-render deopt warnings.
  - `npm.cmd --prefix web/viewer run test:playwright -- tests/navbar-mobile.spec.ts tests/homepage-layout.spec.ts tests/channel.spec.ts` - first run passed 10/11 and exposed a stale homepage selector; follow-up homepage-only run exposed stale subheader alignment; after aligning the spec to the current hero DOM and shell behavior, the combined run passed 13 tests.
  - `git diff --check` - passed with expected CRLF normalization warnings.
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'cd /c/Users/RhythmicCarnage/Desktop/BitRiver-Live && ./scripts/verify.sh --viewer'` - stopped at the host prerequisite gate because no required Python interpreter is installed.
- Task 6 diagnosis: PR #1234 CI run `25944748726` passed the image vulnerability scan but failed the Ubuntu gate during quickstart smoke after dependency services were healthy and application services began starting. The diagnostics show the app layer still not fully up, consistent with the API service hitting the same Linux bind-mount ownership class as the earlier transcoder smoke fix.
- Task 6 follow-up diagnosis: CI run `25945047019` still failed in the same app-start window after shell lint and image scan passed; diagnostics showed `viewer` created immediately after the grouped application start, so the smoke should start the API first, wait for API health, then start viewer/proxy sidecars.
- Task 6 second follow-up diagnosis: CI run `25945237008` still failed before the API container appeared in diagnostics, consistent with `--no-deps` short-circuiting API creation after explicit dependency waits; the API-only start should let Compose evaluate its already-healthy dependency graph, then sidecars can still start with `--no-deps`.
- Task 6 third follow-up diagnosis: CI run `25945489025` still failed in the unified Ubuntu gate, while shell lint and the image scan passed. GitHub raw logs require an authenticated/admin token, so the smoke now emits GitHub annotations for Compose-start, health-wait, and completion-wait failures to expose the next exact failure through the public check-run annotations API.
- Task 6 complete: `scripts/test-quickstart.sh` now prepares `deploy/data` for the smoke, adds `bitriver-live` to the existing Linux-only host UID/GID override, and phases app startup as API-first then viewer/public sidecars. `deploy/docker-compose.yml` is unchanged.
- Task 6 checks:
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'cd /c/Users/RhythmicCarnage/Desktop/BitRiver-Live && bash -n scripts/test-quickstart.sh'` - passed.
  - `$env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; $env:GOCACHE=(Resolve-Path .codex-tmp\go-cache).Path; go test ./scripts -count=1` - passed.
  - `git diff --check` - passed with expected CRLF normalization warnings.
  - Follow-up after phasing app startup: `bash -n scripts/test-quickstart.sh`, `go test ./scripts -count=1`, and `git diff --check` - passed.
  - Follow-up after switching the API-only start back to Compose dependency evaluation: `bash -n scripts/test-quickstart.sh`, `go test ./scripts -count=1`, and `git diff --check` - passed.
  - Follow-up after adding CI smoke annotations: `bash -n scripts/test-quickstart.sh`, `go test ./scripts -count=1`, and `git diff --check` - passed.
- Task 7 diagnosis: CI run `25945739050` still failed in `Ubuntu test-all gate`; the new GitHub annotation shows the API service start reached Docker networking and then failed with `Bind for 0.0.0.0:8080 failed: port is already allocated`. This is isolated to the smoke-created temporary `.env`, not the real deployment contract.
- Task 7 complete: the smoke-created temporary `.env` now publishes the API and viewer through host port `18080` while keeping `BITRIVER_LIVE_ADDR=:8080` and the in-container healthcheck contract on `localhost:8080`.
- Task 7 checks:
  - `bash -n scripts/test-quickstart.sh` - passed.
  - `go test ./scripts -run TestQuickstartSmokeGeneratedEnvUsesNonDefaultHostPort -count=1` - passed.
  - `go test ./scripts -count=1` - passed.
  - `git diff --check` - passed with expected CRLF normalization warnings for touched files.
- Task 8 diagnosis: CI run `25980878280` confirmed the prior Ubuntu test-all gate and image vulnerability scan are green. Remaining failures are `Quickstart entrypoint sanity` ShellCheck notes for `scripts/deploy-smoke.sh` callback functions, Windows static validation expecting `Ensure-DockerDesktopRunning`, and local composite setup actions being referenced before checkout in viewer/Go follow-up jobs.
- Task 8 complete: `scripts/deploy-smoke.sh` now suppresses the current ShellCheck callback note (`SC2329`) only on the trap/polling callback functions, and the duplicated quickstart PowerShell static checks now validate the current wrapper contract instead of stale direct Docker-helper snippets.
- Task 8 checks:
  - `bash -n scripts/deploy-smoke.sh scripts/quickstart.sh scripts/test-quickstart.sh` - passed.
  - PowerShell static snippet check against `scripts/quickstart.ps1` - passed.
  - PowerShell static snippet check against `.github/workflows/ci.yml` and `.github/workflows/quickstart-smoke.yml` - passed after fixing the local check's escaped literals.
- Task 9 complete: every workflow job that calls `.github/actions/setup-go` or `.github/actions/setup-node-viewer` now checks out the repository first so the local composite action metadata exists before Actions resolves it. Existing composite action setup behavior remains unchanged.
- Task 9 checks:
  - `rg -n -B 3 "uses: \./\.github/actions/setup-(go|node-viewer)" .github\workflows` - confirmed each local action call is preceded by `actions/checkout`.
  - `./scripts/check-ci-contract.sh` - passed.
  - `./scripts/check-go-workflow-config.sh` - passed.
- Task 10 validation so far:
  - `go test ./scripts -count=1` with workspace-local `GOCACHE` - passed.
  - `git diff --check` - passed with expected CRLF normalization warnings for touched files.

## Scoped change: PR #1233 CI readiness

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Diagnose failing PR checks
  - Acceptance criteria:
    - Fetch PR #1233 status and failing job logs where available.
    - Record the specific failing commands and root causes in `PLAN.md`/`TASKS.md`.
    - Distinguish code failures from transient or workflow-level failures.

- [x] Task 2 - Fix Ubuntu test-all gate failures
  - Acceptance criteria:
    - OME render tests use the current renderer helper/API correctly.
    - Default server security config applies the default CSP to non-viewer routes.
    - Viewer route CSP behavior remains covered by existing tests.

- [-] Task 3 - Validate locally and update the PR branch
  - Acceptance criteria:
    - Focused Go tests pass.
    - Repo viewer gate passes locally or records any remaining blocker.
    - Changes are committed, pushed, and PR #1233 is updated for merge readiness.

- [x] Task 4 - Fix CI contract env fallback
  - Acceptance criteria:
    - Contract invariants Compose validation works in CI when root `.env` is absent.
    - The fallback does not overwrite a real root `.env`.
    - Temporary fallback `.env` files are cleaned up.

- [x] Task 5 - Fix remaining shell lint and verify Compose fallback failures
  - Acceptance criteria:
    - Shell scripts touched by the failing lint output no longer emit the reported ShellCheck findings.
    - `scripts/verify.sh` validates Docker Compose with a temporary root `.env` when CI has no committed root `.env`.
    - The verify fallback does not overwrite a real root `.env` and cleans temporary files on exit.

- [x] Task 6 - Fix clean-runner quickstart smoke image pulls
  - Acceptance criteria:
    - `scripts/test-quickstart.sh` can start Compose on a clean runner without assuming third-party images are already cached.
    - First-party services still build from the source checkout during the quickstart smoke.
    - The change is validated with syntax/unit checks and confirmed in the next CI run.

- [-] Task 7 - Fix final quickstart smoke and image scan failures
  - Acceptance criteria:
    - The quickstart smoke prepares the host transcoder bind mount and test-only Unix user override so the transcoder can become healthy on Linux CI.
    - The smoke fixture uses the same non-loopback transcoder public URL used by the CI image build fixture.
    - The final API and proxied viewer checks poll until ready instead of racing service startup.
    - Compose startup is phased so dependency health/completion is checked before starting the API/viewer layer.
    - Trivy installation in CI retries and extracts a complete archive before scanning.
    - The one-shot `postgres-migrations` completion waiter can inspect stopped containers after Compose starts dependencies.
    - The Trivy install pin targets an available official release asset.
    - First-party Go Docker images build binaries with a Go patch line that includes the `CVE-2025-68121` stdlib fix.
    - Application services start after explicit dependency validation without Compose reprocessing the dependency graph.
    - The API runtime image and pgx dependency no longer produce blocking critical Trivy findings.
    - Viewer Next.js packages are on the fixed 13.5 patch line for `CVE-2025-29927`.
    - The change is validated with syntax/unit checks, pushed, and confirmed in the next CI run.

### Execution log (PR #1233 CI readiness)
- Task 1 complete: `gh auth status` is still blocked by an invalid local GitHub token, so Actions inspection used the GitHub connector. PR #1233's CI run failed in `Ubuntu test-all gate` and `Image vulnerability scan`. The Ubuntu gate failed on `cmd/bitriver/env_validation_test.go` calling `renderOMEFromEnv` with the old five-argument shape and `internal/server/security_headers_test.go` receiving an empty default CSP header. The image scan failed while installing Trivy with `gzip: stdin: unexpected end of file`; no workflow edit is planned unless it recurs after a fresh push.
- Task 1 checks:
  - `gh auth status` - failed due invalid local token for `ProhibitedTV`.
  - GitHub connector: fetched PR #1233 metadata and workflow run `25826344573`.
  - GitHub connector: fetched failing job logs for `Ubuntu test-all gate` and `Image vulnerability scan`.
  - `rg -n "renderOMEFromEnv|Content-Security-Policy|SecurityHeaders|securityHeaders|security header|CSP|default-src" cmd internal PLAN.md TASKS.md`
  - `Get-Content cmd/bitriver/env_validation_test.go | Select-Object -Skip 500 -First 120`
  - `Get-Content cmd/bitriver/ome_render.go | Select-Object -First 180`
  - `Get-Content internal/server/security.go | Select-Object -First 180`
  - `Get-Content internal/server/server.go | Select-Object -Skip 250 -First 100`
  - `Get-Content internal/server/security_headers_test.go`
- Task 2 complete: updated the OME render tests to call `renderOMEFromEnvOutput` when they need a temp output path, and restored default CSP population in `SecurityConfig.withDefaults()` after frame-ancestor defaults are resolved. Viewer routes still skip the API/admin default CSP in middleware and keep the dedicated viewer proxy CSP coverage.
- Task 2 checks:
  - `$env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./cmd/bitriver ./internal/server -count=1` - blocked by a pre-existing Windows Go cache path issue.
  - `New-Item -ItemType Directory -Force -Path .codex-tmp\go-cache; $env:GOCACHE=(Resolve-Path .codex-tmp\go-cache).Path; go test ./cmd/bitriver ./internal/server -count=1` - passed.
- Task 3 validation so far:
  - `$env:GOCACHE=(Resolve-Path .codex-tmp\go-cache).Path; & 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh --viewer` - did not reach project checks because Git Bash could not find `dirname` when launched without login PATH setup.
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'GOCACHE="$PWD/.codex-tmp/go-cache" ./scripts/verify.sh --viewer'` - reached project checks and stopped at the contract stage because this workstation has no usable Python interpreter (`py -0p` reports no installed Pythons).
  - `$env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; $env:GOCACHE=(Resolve-Path .codex-tmp\go-cache).Path; go test ./... -count=1 -timeout=120s` - passed.
  - `git diff --check` - passed; Git reported expected CRLF normalization warnings for touched files.
- Task 4 in progress: the fresh PR run showed the previous Go failures fixed, then failed in `scripts/check-contract-invariants.sh` because `docker compose --env-file deploy/.env.example -f deploy/docker-compose.yml config` still loads the Compose service-level `env_file: ../.env`, which is absent in CI.
- Task 4 complete: `scripts/check-contract-invariants.sh` now creates a temporary root `.env` from `deploy/.env.example` only when `.env` is missing, uses that file for Compose config validation, and removes it on exit. The script does not overwrite a real root `.env`.
- Task 4 checks:
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'bash -n scripts/check-contract-invariants.sh'` - passed.
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc './scripts/check-contract-invariants.sh'` - Compose validation and generated docs check passed locally with real root `.env`; stopped at missing local Python interpreter.
  - Temporary no-`.env` fixture running `./scripts/check-contract-invariants.sh` - Compose validation reached the later Python step and the temporary root `.env` was cleaned up.
  - `$env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; $env:GOCACHE=(Resolve-Path .codex-tmp\go-cache).Path; go test ./scripts -count=1` - passed.
- Task 5 complete: normalized the reported `CDPATH` assignments, narrowed `deploy-smoke.sh` ShellCheck suppressions to trap/polling callback functions, removed the unused smoke result variable, and changed `scripts/verify.sh` to create a temporary root `.env` from `deploy/.env.example` only around Docker Compose config validation when the root `.env` is absent.
- Task 5 checks:
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'bash -n scripts/verify.sh scripts/check-go-sum-not-empty.sh scripts/refresh-go-sum.sh scripts/require-image-digests.sh scripts/deploy-smoke.sh'` - passed.
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'command -v shellcheck || true'` - shellcheck is not installed locally, so CI must perform the actual lint confirmation.
  - Temporary no-`.env` fixture sourcing `scripts/verify.sh` and running `run_docker_compose_config_validation` - Compose config rendered, fallback message printed, and the temporary root `.env` was cleaned up.
  - `$env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; $env:GOCACHE=(Resolve-Path .codex-tmp\go-cache).Path; go test ./scripts -count=1` - passed.
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'GOCACHE="$PWD/.codex-tmp/go-cache" ./scripts/verify.sh --viewer'` - stopped at the host prerequisite gate because this workstation has no usable Python interpreter.
  - `git diff --check` - passed; Git reported expected CRLF normalization warnings for touched Markdown files.
- Task 6 complete: `scripts/test-quickstart.sh` now builds local Compose images first, pulls missing non-buildable runtime images, and then starts the stack with `--no-build --pull never`. Building before pulling avoids fetching helper services such as `bitriver-live/ome-config:local`, which are reused by non-buildable healthcheck jobs after the local image is built.
- Task 6 checks:
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'bash -n scripts/test-quickstart.sh scripts/verify.sh scripts/check-go-sum-not-empty.sh scripts/refresh-go-sum.sh scripts/require-image-digests.sh scripts/deploy-smoke.sh'` - passed.
  - `docker compose --env-file .env -f deploy/docker-compose.yml pull --ignore-buildable --policy missing --dry-run` - blocked on this workstation by Docker config/buildx access permissions before validating the Compose pull plan.
  - `$env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; $env:GOCACHE=(Resolve-Path .codex-tmp\go-cache).Path; go test ./scripts -count=1` - passed.
  - `git diff --check` - passed; Git reported expected CRLF normalization warnings for touched Markdown files.
  - Fresh CI run `25828160170` confirmed the first pull ordering was wrong because `ome-health-token-check` reused the locally built `bitriver-live/ome-config:local` image without its own `build:` block. Follow-up syntax, scripts test, and diff-check commands passed after reordering build/pull/start.
  - Fresh CI run `25828368236` confirmed the build/pull/start order was correct and then failed in Linux bind-mount writes for the one-shot `srs-config` and `ome-config` jobs. The smoke now adds a temporary Unix-only Compose override so those repo-writing helper jobs run as the host UID/GID while leaving `deploy/docker-compose.yml` unchanged.
  - Follow-up `bash -n scripts/test-quickstart.sh`, `go test ./scripts -count=1`, and `git diff --check` - passed.
- Task 7 in progress: CI run `25828715591` advanced through build/pull/config jobs and then failed because `bitriver-transcoder` became unhealthy during quickstart smoke startup. The same run still failed `Image vulnerability scan` while streaming the Trivy archive into `tar` (`gzip: stdin: unexpected end of file`), so the recurring scanner install failure is now in scope with the user's approval.
- Task 7 follow-up: CI run `25829197707` moved past shell lint and guards, but the Ubuntu gate still failed late in quickstart smoke after OME and core services were starting. The smoke's final API/viewer curls are one-shot checks, so they can race the viewer sidecar even after dependency healthchecks are green.
- Task 7 second follow-up: CI run `25829744630` still failed during `docker compose up` before the script's own health waits could isolate the service. The smoke should start dependency services first, wait for health and migration completion explicitly, and then start the API/viewer services.
- Task 7 local checks:
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'bash -n scripts/test-quickstart.sh'` - passed.
  - `$env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; $env:GOCACHE=(Resolve-Path .codex-tmp\go-cache).Path; go test ./scripts -count=1` - passed.
  - `git diff --check` - passed; Git reported expected CRLF normalization warnings for touched Markdown/workflow files.
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'GOCACHE="$PWD/.codex-tmp/go-cache" ./scripts/verify.sh --viewer'` - stopped at the host prerequisite gate because this workstation has no usable Python runtime.
  - Follow-up after endpoint polling: `bash -n scripts/test-quickstart.sh`, `go test ./scripts -count=1`, and `git diff --check` - passed.
  - Follow-up after phased Compose startup: `bash -n scripts/test-quickstart.sh`, `go test ./scripts -count=1`, and `git diff --check` - passed.
- Task 7 latest diagnosis: CI run `25830088634` got all dependency health checks green, then failed at `postgres-migrations` completion with `error: no container found for service postgres-migrations`; the script was querying only running containers even though the migration job is expected to be stopped/exited. The same run's image scan failed in `Install Trivy` because `https://github.com/aquasecurity/trivy/releases/download/v0.50.1/trivy_0.50.1_Linux-64bit.tar.gz` returned 404 across retries. The official Trivy releases page shows `v0.70.0` as the current immutable release with Linux tarball assets.
- Task 7 latest fixes: `wait_for_completed_check` now queries stopped containers with `docker compose ps -a -q` before inspecting one-shot exit status, and the CI Trivy install pin now targets `v0.70.0`.
- Task 7 latest local checks:
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'bash -n scripts/test-quickstart.sh'` - passed.
  - `$env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; $env:GOCACHE=(Resolve-Path .codex-tmp\go-cache).Path; go test ./scripts -count=1` - passed.
  - `git diff --check` - passed; Git reported expected CRLF normalization warnings for touched Markdown/workflow files.
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'GOCACHE="$PWD/.codex-tmp/go-cache" ./scripts/verify.sh --viewer'` - stopped at the host prerequisite gate because this workstation has no usable Python runtime.
- Task 7 image-scan follow-up: CI run `25830437975` confirmed Trivy `v0.70.0` installs and the separate `ome-config` OS-package scan passes, then failed in the blocking first-party scan because the `ome-config` Go binary was built with Go `v1.21.13` and Trivy reported stdlib `CVE-2025-68121` fixed by Go `1.24.13`, `1.25.7`, and newer. The fix should update all first-party Go Docker builder images from `golang:1.21*` to a fixed patch line while keeping `go.mod` at `go 1.21`.
- Task 7 Ubuntu follow-up: CI run `25830437975` moved past the `postgres-migrations` lookup and showed the migration job completing successfully in diagnostics. The remaining Ubuntu failure happens after dependencies are already validated, so application startup should use `--no-deps` to avoid Compose reprocessing the dependency graph and one-shot job during the second phase.
- Task 7 second follow-up fixes: all first-party Go Docker builders now use Go `1.25.7` builder images, and application service startup now uses `docker compose up --no-deps` after the script has explicitly validated dependency health and migration completion.
- Task 7 second follow-up local checks:
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'bash -n scripts/test-quickstart.sh'` - passed.
  - `$env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; $env:GOCACHE=(Resolve-Path .codex-tmp\go-cache).Path; go test ./scripts -count=1` - passed.
  - `git diff --check` - passed; Git reported expected CRLF normalization warnings for touched files.
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'GOCACHE="$PWD/.codex-tmp/go-cache" ./scripts/verify.sh --viewer'` - stopped at the host prerequisite gate because this workstation has no usable Python runtime.
- Task 7 image-scan second follow-up: CI run `25830760564` cleared the Go stdlib CVE and then failed on `ghcr.io/bitriver-live/bitriver-live:ci` because the Debian runtime layer contains critical package findings and the real Postgres binary embeds `github.com/jackc/pgx/v5 v5.7.4` with `CVE-2026-33816` fixed in `v5.9.0`.
- Task 7 third follow-up fixes: the API runtime image now uses Alpine `3.23` with `curl` preserved for healthchecks, SRS/transcoder Alpine runtime stages now use Alpine `3.23`, and the real pgx module requirement plus checksum are updated to `github.com/jackc/pgx/v5 v5.9.0`.
- Task 7 third follow-up local checks:
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'bash -n scripts/test-quickstart.sh'` - passed.
  - `$env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; $env:GOCACHE=(Resolve-Path .codex-tmp\go-cache).Path; go test ./... -count=1 -timeout=120s` - passed.
  - `git diff --check` - passed; Git reported expected CRLF normalization warnings for touched files.
  - `& 'C:\Program Files\Git\bin\bash.exe' -lc 'GOCACHE="$PWD/.codex-tmp/go-cache" ./scripts/verify.sh --viewer'` - stopped at the host prerequisite gate because this workstation has no usable Python runtime.
- Task 7 image-scan third follow-up: CI run `25831129289` moved past the API runtime and pgx blockers, then failed on viewer `next 13.5.6` with `CVE-2025-29927`; the viewer now uses the patched `13.5.11` release from the same 13.5 line.

## Scoped change: issue #1220 chat controls and signed-out chat UX

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the chat-control diagnosis and scoped implementation plan
  - Acceptance criteria:
    - `PLAN.md` captures issue #1220 scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists ordered implementation and validation tasks before chat source edits begin.
    - The read-only pass identifies current header controls, settings/popout dialogs, inline report form, signed-out composer, tests, and CSS surfaces.

- [x] Task 2 - Consolidate secondary chat controls into a compact options menu
  - Acceptance criteria:
    - Chat header shows title plus only viewer-relevant counts by default.
    - Pop-out and display settings are discoverable from one compact options control.
    - The options menu supports outside-click and Escape dismissal.
    - Pop-out opens directly without a confirmation modal.

- [x] Task 3 - Move reporting and signed-out chat into calmer surfaces
  - Acceptance criteria:
    - Report actions open a modal/sheet instead of expanding inline inside message bubbles.
    - Report submit/cancel/error behavior remains intact.
    - Signed-out chat shows one clear sign-in CTA instead of a disabled composer.

- [x] Task 4 - Update focused tests and docs
  - Acceptance criteria:
    - ChatPanel tests cover the options menu, direct pop-out action, settings toggles, report modal, and signed-out CTA.
    - Playwright chat auth coverage reflects the new signed-out state.
    - User-visible viewer behavior is documented where appropriate.

- [x] Task 5 - Run validation and record results
  - Acceptance criteria:
    - Focused Jest, lint, build, targeted Playwright, and the repo viewer gate are run or explicitly recorded with unrelated blockers.
    - `TASKS.md` records command results and any residual warnings.

### Execution log (issue #1220 chat controls and signed-out chat UX)
- Task 1 complete: closed issue #1217 as completed after confirming merged PR #1232, selected the next oldest open product issue (#1220), created branch `codex/issue-1220-chat-controls`, and reviewed `web/viewer/components/ChatPanel.tsx`, `web/viewer/__tests__/chatPanel.test.tsx`, `web/viewer/tests/channel-chat-playback.spec.ts`, channel auth smoke coverage, chat CSS, and viewer docs before editing.
- Task 1 checks:
  - `git pull --ff-only origin main`
  - GitHub connector: fetched PR #1232 and closed issue #1217 with `state_reason=completed`
  - GitHub connector: searched open issues and selected issue #1220
  - `git checkout -b codex/issue-1220-chat-controls`
  - `Get-Content web/viewer/components/ChatPanel.tsx`
  - `Get-Content web/viewer/__tests__/chatPanel.test.tsx`
  - `Get-Content web/viewer/styles/globals.css | Select-Object -Skip 2400 -First 230`
  - `Get-Content web/viewer/tests/channel-chat-playback.spec.ts`
  - `rg -n "chat-panel|chat-header|chat-actions|chat-settings|chat-popout|report|composer|settings|popout|viewer count|Live sync|Refreshing" web/viewer/styles web/viewer/__tests__ web/viewer/tests web/viewer/components/ChatPanel.tsx`
- Task 2 complete: replaced permanent header controls with a compact chat options menu, kept counts visible in the header, moved connection status into the menu, preserved avatar/timestamp toggles, and made pop-out open directly from the menu.
- Task 2 checks:
  - `npm.cmd --prefix web/viewer run test -- chatPanel.test.tsx` - passed 16 tests; existing fake-timer `act(...)` warnings were still emitted around typed input/report interactions.
- Task 3 complete: moved message reporting into a dedicated dialog with Escape/focus handling and replaced the signed-out disabled composer with a sign-in CTA card.
- Task 3 checks:
  - `npm.cmd --prefix web/viewer run test -- chatPanel.test.tsx` - passed 16 tests; existing fake-timer `act(...)` warnings were still emitted around typed input/report interactions.
- Task 4 complete: updated ChatPanel, ChannelPage, and Playwright chat/auth coverage for the compact options menu, direct pop-out action, report dialog, and signed-out sign-in CTA; documented the chat control contract in `web/viewer/README.md`.
- Task 4 checks:
  - `npm.cmd --prefix web/viewer run test -- chatPanel.test.tsx` - passed 16 tests; existing fake-timer `act(...)` warnings were still emitted around typed input/report interactions.
  - `npm.cmd --prefix web/viewer run test -- channelPage.test.tsx` - passed 19 tests.
  - `npm.cmd --prefix web/viewer run lint` - passed with no ESLint warnings or errors.
  - `npm.cmd --prefix web/viewer run build` - passed; existing Browserslist/deopt warnings were emitted by Next.js.
  - `npx.cmd playwright test tests/channel-chat-playback.spec.ts` against the standalone build - passed 4 tests.
  - `npx.cmd playwright test tests/channel.spec.ts` against the standalone build - passed 8 tests.
- Task 5 complete: ran the viewer-focused gate commands and recorded the remaining repository gate blockers.
- Task 5 checks:
  - `npm.cmd --prefix web/viewer run test -- chatPanel.test.tsx` - passed 16 tests; existing fake-timer `act(...)` warnings were still emitted around typed input/report interactions.
  - `npm.cmd --prefix web/viewer run test -- channelPage.test.tsx` - passed 19 tests.
  - `npm.cmd --prefix web/viewer run lint` - passed with no ESLint warnings or errors.
  - `npm.cmd --prefix web/viewer run build` - passed; existing Browserslist/deopt warnings were emitted by Next.js.
  - `npx.cmd playwright test tests/channel-chat-playback.spec.ts` against the standalone build - passed 4 tests.
  - `npx.cmd playwright test tests/channel.spec.ts` against the standalone build - passed 8 tests.
  - `npm.cmd --prefix web/viewer run lint` after final cleanup - passed with no ESLint warnings or errors.
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh --viewer` - failed before viewer checks on existing backend blockers: `cmd/bitriver/env_validation_test.go` still calls `renderOMEFromEnv` with the old five-argument signature, and `internal/server/security_headers_test.go` still expects CSP headers that the current server responses do not set.
  - `git diff --check` - passed; Git reported expected CRLF normalization warnings for touched files.

## Scoped change: issue #1217 navbar density and action hierarchy

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the navbar-density diagnosis and scoped implementation plan
  - Acceptance criteria:
    - `PLAN.md` captures issue #1217 scope, assumptions, risks, and the focused viewer validation plan.
    - `TASKS.md` lists the ordered implementation and validation sequence before navbar source edits begin.
    - The read-only pass identifies current primary nav, drawer, account, CSS, and test surfaces.

- [x] Task 2 - Simplify the persistent navbar route/action hierarchy
  - Acceptance criteria:
    - Persistent primary nav emphasizes viewer discovery routes instead of creator/account utilities.
    - Creator/admin/profile/theme utilities remain reachable through account/site menu surfaces.
    - Guest sign-in and create-account flows remain available and keep their existing auth behavior.

- [x] Task 3 - Tighten mobile drawer and navbar styling around the new hierarchy
  - Acceptance criteria:
    - Drawer links do not duplicate primary route destinations or utility CTAs confusingly.
    - Effective CSS rules reduce desktop header crowding and preserve mobile no-overflow behavior.
    - Menu buttons keep clear accessible labels and keyboard/outside-click dismissal behavior.

- [x] Task 4 - Update viewer docs and focused tests
  - Acceptance criteria:
    - `web/viewer/README.md` reflects the updated navigation contract.
    - Navigation/navbar unit tests cover the final guest/member/creator/admin route visibility and utility placement.
    - Playwright mobile navbar coverage still asserts drawer behavior and no horizontal overflow.

- [x] Task 5 - Run validation and record results
  - Acceptance criteria:
    - Focused Jest checks, lint, build, targeted Playwright, and the repo viewer gate are run or explicitly recorded with host blockers.
    - `TASKS.md` records all command results and any residual gaps.

### Execution log (issue #1217 navbar density and action hierarchy)
- Task 1 complete: closed issue #1219 as completed after confirming merged PR #1231, selected the oldest remaining open product issue (#1217), created branch `codex/issue-1217-nav-density`, reviewed `web/viewer/components/Navbar.tsx`, `web/viewer/lib/navigation.ts`, `web/viewer/README.md`, navbar unit tests, mobile Playwright coverage, and effective navbar CSS override sections before editing.
- Task 1 checks:
  - `git pull --ff-only origin main`
  - `git checkout -b codex/issue-1217-nav-density`
  - GitHub connector: fetched PR #1231 and closed issue #1219 with `state_reason=completed`
  - GitHub connector: searched open issues and selected issue #1217
  - `Get-Content web/viewer/components/Navbar.tsx`
  - `Get-Content web/viewer/lib/navigation.ts`
  - `Get-Content web/viewer/README.md -TotalCount 160`
  - `Get-Content web/viewer/__tests__/navbar.test.tsx`
  - `Get-Content web/viewer/__tests__/navigation.test.ts`
  - `Get-Content web/viewer/tests/navbar-mobile.spec.ts`
  - `rg -n "Navbar|navigation|nav-|navbar|drawer|theme|Control center|Start stream|Create account|Menu|Close|Light|Dark" web/viewer/__tests__ web/viewer/tests web/viewer/styles web/viewer/components web/viewer/lib`
- Task 2 complete: removed `Go Live` and `Profile` from the persistent primary route config, moved creator/admin/profile/setup/theme utilities into the compact account/site menu, and preserved signed-out sign-in/create-account CTAs with their existing redirect/overlay behavior.
- Task 2 checks:
  - `npm.cmd --prefix web/viewer run test -- navigation.test.ts navbar.test.tsx` (passed)
- Task 3 complete: updated the mobile drawer utility section to expose creator setup/go-live outside the primary route list, added profile/sign-out grouping for signed-in drawer accounts, and added small menu styling hooks for the site menu and drawer account actions.
- Task 3 checks:
  - `npm.cmd --prefix web/viewer run test -- navigation.test.ts navbar.test.tsx` (passed)
- Task 4 complete: updated `web/viewer/README.md` with the viewer-first navigation contract, adjusted navigation/navbar unit tests for the final route matrix and account-menu utility placement, and updated the theme Playwright flow to open the site menu before toggling theme.
- Task 4 checks:
  - `npm.cmd --prefix web/viewer run test -- navigation.test.ts navbar.test.tsx` (passed)
- Task 5 complete: focused navbar/navigation Jest coverage, adjacent shell coverage, viewer lint, production build, and targeted mobile Playwright coverage passed. The repo viewer gate was invoked and recorded; it failed in unrelated Go checks before reaching viewer checks.
- Task 5 checks:
  - `npm.cmd --prefix web/viewer run test -- navigation.test.ts navbar.test.tsx` (passed)
  - `npm.cmd --prefix web/viewer run test -- viewerShell.test.tsx` (passed)
  - `npm.cmd --prefix web/viewer run lint` (passed)
  - `npm.cmd --prefix web/viewer run build` (passed; existing Next client-rendering and Browserslist warnings were emitted)
  - `npx.cmd playwright install chromium` (completed; local project browser cache was already present)
  - `Copy-Item -Path web\viewer\.next\static -Destination web\viewer\.next\standalone\.next\static -Recurse -Force` (prepared the standalone server for hydrated Playwright smoke testing)
  - `PLAYWRIGHT_BASE_URL=http://127.0.0.1:3000 PLAYWRIGHT_BROWSERS_PATH=0 npx.cmd playwright test tests/navbar-mobile.spec.ts` with a temporary standalone server (passed)
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh --viewer` (failed outside this viewer/navbar scope: `cmd/bitriver` tests still call `renderOMEFromEnv` with the old signature, and `internal/server` security header tests expected CSP headers that were absent)

## Scoped change: restore overlay auth flow from `codex/signin-polish`

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the selective overlay-restore plan before editing viewer auth
  - Acceptance criteria:
    - `PLAN.md` captures the scoped overlay-auth restore, assumptions, risks, and focused viewer test commands.
    - `TASKS.md` lists a small ordered implementation and validation sequence before auth/viewer files change.
    - The read-only analysis identifies which branch pieces are being ported and which current-checkout behaviors must be preserved.

- [x] Task 2 - Restore the mounted overlay auth state and dialog wiring
  - Acceptance criteria:
    - `useAuth` supports in-viewer dialog state, signup/MFA flow handling, and current-route redirect preservation.
    - External `loginUrl` redirects still work when provided by the runtime.
    - `Providers` mounts the overlay dialog so viewer pages can trigger it again.

- [x] Task 3 - Rewire signed-out viewer CTAs back to overlay actions
  - Acceptance criteria:
    - Navbar sign-in/join actions open the restored overlay for local auth installs.
    - Following signed-out prompts use auth actions instead of hard-linking to `/signup#login-form`.
    - `/signup` remains available as the standalone auth fallback and is not removed.

- [x] Task 4 - Restore overlay styling and align viewer auth tests
  - Acceptance criteria:
    - The overlay dialog has the required `auth-overlay` styling hooks in viewer CSS.
    - Viewer auth test helpers cover the expanded `useAuth` context shape.
    - Focused Jest coverage is updated for `useAuth`, `AuthDialog`, navbar auth CTAs, and following CTA behavior.

- [x] Task 5 - Run focused viewer validation and record the results
  - Acceptance criteria:
    - The targeted auth/viewer Jest commands are run and logged.
    - Any residual warnings or gaps are recorded explicitly in the execution log.

### Execution log (restore overlay auth flow from `codex/signin-polish`)
- Task 1 complete: reviewed the current checkout against `codex/signin-polish`, confirmed the overlay flow lives primarily in `web/viewer/hooks/useAuth.tsx`, `web/viewer/components/auth/AuthDialog.tsx`, `web/viewer/components/Providers.tsx`, `web/viewer/components/Navbar.tsx`, `web/viewer/components/following/FollowingState.tsx`, viewer auth tests, and the `auth-overlay` CSS block, and recorded the selective-port plan here before editing.
- Task 1 checks:
  - `git show codex/signin-polish:web/viewer/hooks/useAuth.tsx`
  - `git show codex/signin-polish:web/viewer/components/auth/AuthDialog.tsx`
  - `git show codex/signin-polish:web/viewer/components/Providers.tsx`
  - `git show codex/signin-polish:web/viewer/components/Navbar.tsx`
  - `git show codex/signin-polish:web/viewer/components/following/FollowingState.tsx`
  - `git show codex/signin-polish:web/viewer/styles/globals.css | Select-Object -Skip 2520 -First 190`
  - `Get-Content web/viewer/hooks/useAuth.tsx`
  - `Get-Content web/viewer/components/auth/AuthDialog.tsx`
  - `Get-Content web/viewer/components/Providers.tsx`
  - `Get-Content web/viewer/components/Navbar.tsx`
  - `Get-Content web/viewer/components/following/FollowingState.tsx`
- Task 2 complete: replaced the current redirect-only `useAuth` state with the selective overlay flow from `codex/signin-polish`, kept `/signup` out of the primary local sign-in path, preserved current-route redirect tracking plus MFA/signup handling, and retained redirect compatibility when `/api/viewer/me` returns a `loginUrl`. `Providers` now mounts the restored overlay dialog globally so viewer routes can open it again.
- Task 2 checks:
  - `Get-Content web/viewer/hooks/useAuth.tsx`
  - `Get-Content web/viewer/components/auth/AuthDialog.tsx`
  - `Get-Content web/viewer/components/Providers.tsx`
- Task 3 complete: rewired the signed-out navbar join/sign-in actions and the following unauthenticated prompt back through `useAuth`, so local installs open the overlay in place while explicitly external signup/login destinations still redirect when configured.
- Task 3 checks:
  - `Get-Content web/viewer/components/Navbar.tsx`
  - `Get-Content web/viewer/components/following/FollowingState.tsx`
- Task 4 complete: restored the `auth-overlay` CSS block in viewer globals, expanded the shared auth mock helpers to cover the richer `useAuth` context shape, replaced the old redirect-centric `useAuth` tests with overlay-focused coverage, added a dedicated `authDialog.test.tsx` suite plus CTA expectation updates in navbar/following tests, and documented the restored viewer-overlay-vs-`/signup` behavior in `docs/quickstart.md`.
- Task 4 checks:
  - `Get-Content web/viewer/styles/globals.css | Select-String -Pattern "auth-overlay" -Context 0,2`
  - `Get-Content web/viewer/test/auth.ts`
  - `Get-Content web/viewer/__tests__/useAuth.test.tsx`
  - `Get-Content web/viewer/__tests__/authDialog.test.tsx`
  - `Get-Content web/viewer/__tests__/navbar.test.tsx`
  - `Get-Content web/viewer/__tests__/followingStatePresentation.test.tsx`
  - `Get-Content web/viewer/__tests__/followingSidebar.test.tsx`
  - `Get-Content docs/quickstart.md | Select-String -Pattern "in-viewer auth dialog|The standalone /signup page remains available" -Context 0,0`
- Task 5 complete: the focused auth/viewer Jest coverage passed, the neighboring auth-adjacent regression sweep passed, viewer lint passed, and the production viewer build compiled successfully after the overlay restore.
- Task 5 checks:
  - `npm.cmd --prefix web/viewer run test -- __tests__/useAuth.test.tsx __tests__/authDialog.test.tsx __tests__/navbar.test.tsx __tests__/followingStatePresentation.test.tsx`
  - `npm.cmd --prefix web/viewer run test -- __tests__/chatPanel.test.tsx __tests__/channelHero.test.tsx __tests__/uploadManager.test.tsx __tests__/followingSidebar.test.tsx`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
- Remaining validation notes:
  - `navbar.test.tsx` still emits the existing jsdom "Not implemented: navigation" warning when real anchor navigation is exercised; the assertions pass and this predates the overlay restore.
  - `chatPanel.test.tsx` still emits the existing non-failing React `act(...)` warnings from chat composer/dialog interactions; the suite passes and this change did not modify `ChatPanel`.

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

## Scoped change: local redeploy and credential handoff

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the scoped redeploy plan before runtime actions
  - Acceptance criteria:
    - `PLAN.md` captures the local redeploy scope, assumptions, risks, and verification commands.
    - `TASKS.md` lists the ordered deployment and credential-verification tasks before Docker actions begin.

- [x] Task 2 - Run the documented local deployment workflow
  - Acceptance criteria:
    - The Windows quickstart entrypoint is exercised against the canonical repo contract.
    - Compose service state/logs are captured after the deployment attempt.
    - Any repo-side deployment blocker is isolated clearly from host-only issues.

- [x] Task 3 - Verify login surfaces and capture the credentials actually used
  - Acceptance criteria:
    - The live auth/admin entry points are checked locally after deployment.
    - The admin credentials used for this run are identified from quickstart output or the active `.env`.
    - The final response can hand the user a concrete login path plus username/password.

- [x] Task 4 - Run closing verification and record the outcome
  - Acceptance criteria:
    - Relevant quickstart/Compose/HTTP checks are rerun or summarized after the deployment attempt.
    - `TASKS.md` records whether the stack reached a healthy running state and any remaining blockers.

### Execution log (local redeploy and credential handoff)
- Task 1 complete: appended this scoped redeploy-and-credential plan to `PLAN.md` and `TASKS.md` before rerunning Docker actions, using the existing repo-root `.env` and the canonical Windows quickstart entrypoint as the initial deployment target.
- Task 1 checks:
  - `rg -n "local redeploy and credential handoff|Redeploy the local BitRiver Live stack on this Windows host" PLAN.md TASKS.md`
  - `rg -n "^(BITRIVER_LIVE_MODE|BITRIVER_DEPLOY_IMAGE_SOURCE|BITRIVER_LIVE_ADMIN_EMAIL|BITRIVER_LIVE_ADMIN_PASSWORD|NEXT_PUBLIC_VIEWER_URL|BITRIVER_TRANSCODER_PUBLIC_BASE_URL|BITRIVER_OME_BIND|BITRIVER_OME_IP)=" .env`
- Task 2 complete: exercised the documented Windows quickstart entrypoint and isolated two repo-side workflow defects plus one host-only blocker before completing a local bring-up with the documented development-mode Compose override flow on an alternate port.
- Task 2 findings:
  - Host-only blocker: `TCP/8080` was already occupied by an unrelated `bitriverd.exe` from `C:\Users\RhythmicCarnage\Desktop\BitRiver-Studio-OS`, so the local shakedown had to move to `http://localhost:18080`.
  - Repo blocker 1: `scripts/quickstart.ps1 --env-file .env --compose-file deploy/docker-compose.yml` fails `Doctor` under elevated execution because the spawned CLI process cannot find `docker` on `PATH`.
  - Repo blocker 2: `deploy/docker-compose.yml` miswires `postgres-migrations` with `entrypoint: ["/bin/sh","-c"]` plus a scalar `command`, which Compose/tokenization turns into `["set","-e","if",...]`, so the migration job exits successfully without applying schema files.
  - Workflow note: the canonical quickstart path also enforces the production contract in the saved `.env`, so the checked-in localhost/demo values are intentionally not quickstart-runnable end to end without production-safe overrides.
- Task 2 checks:
  - `powershell -ExecutionPolicy Bypass -File scripts/quickstart.ps1 --env-file .env --compose-file deploy/docker-compose.yml`
  - `$env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go run ./cmd/bitriver quickstart --env-file .env --compose-file deploy/docker-compose.yml`
  - `docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"`
  - `Get-NetTCPConnection -LocalPort 8080 -State Listen | Select-Object LocalAddress,LocalPort,OwningProcess | Format-List`
  - `Get-Process -Id (Get-NetTCPConnection -LocalPort 8080 -State Listen | Select-Object -ExpandProperty OwningProcess -Unique) | Select-Object Id,ProcessName,Path | Format-List`
  - `docker pull docker/dockerfile:1`
  - `docker pull ossrs/srs:v5.0.185`
  - `$env:BITRIVER_LIVE_MODE='development'; $env:BITRIVER_LIVE_PORT='18080'; $env:NEXT_PUBLIC_VIEWER_URL='http://localhost:18080/viewer'; docker compose --env-file .env -f deploy/docker-compose.yml up -d --build`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps -a`
  - `docker inspect deploy-postgres-migrations-1 --format '{{json .Config.Entrypoint}} {{json .Config.Cmd}}'`
  - `docker compose --env-file .env -f deploy/docker-compose.yml run --rm --entrypoint psql postgres-migrations -h postgres -U brlive_app -d brlive_app -v ON_ERROR_STOP=1 -f /migrations/0001_initial.sql -f /migrations/0002_auth_sessions.sql -f /migrations/0002_chat_filters.sql -f /migrations/0003_oauth_accounts.sql -f /migrations/0004_auth_session_hashes.sql -f /migrations/0005_auth_session_absolute_expiry.sql -f /migrations/0006_profile_social_links.sql -f /migrations/0007_auth_mfa.sql -f /migrations/0008_legal_cases.sql -f /migrations/0009_appeals.sql -f /migrations/0010_payments_webhooks.sql`
- Task 3 complete: verified the live local routes on `http://localhost:18080`, seeded the admin account inside the running stack, and confirmed the active `.env` admin email/password are accepted by the login API.
- Task 3 findings:
  - `http://localhost:18080/` returns `307` to `/viewer`.
  - `http://localhost:18080/viewer` serves the public viewer shell.
  - `http://localhost:18080/signup` serves the sign-in page with `login-form`.
  - `http://localhost:18080/admin` returns `200`.
  - After seeding the admin account, `POST /api/auth/login` with the active `.env` admin email/password returns `200` and an MFA enrollment payload (`mfaRequired: true`), confirming the credentials are valid for this deployment.
- Task 3 checks:
  - `docker compose --env-file .env -f deploy/docker-compose.yml exec -T postgres psql -U brlive_app -d brlive_app -c "\dt"`
  - `docker compose --env-file .env -f deploy/docker-compose.yml exec -T bitriver-live /app/bootstrap-admin --postgres-dsn "postgres://brlive_app:***@postgres:5432/brlive_app?sslmode=disable" --email <active .env admin email> --password <active .env admin password>`
  - `Invoke-WebRequest -UseBasicParsing http://localhost:18080/ -MaximumRedirection 0`
  - `Invoke-WebRequest -UseBasicParsing http://localhost:18080/viewer`
  - `Invoke-WebRequest -UseBasicParsing http://localhost:18080/signup`
  - `Invoke-WebRequest -UseBasicParsing http://localhost:18080/admin -MaximumRedirection 0`
  - `Invoke-WebRequest -UseBasicParsing -Uri http://localhost:18080/api/auth/login -Method Post -ContentType 'application/json' -Body <active .env admin creds JSON>`
- Task 4 complete: recorded the final deployment state and remaining workflow blockers after the local stack reached a usable running state on the alternate port.
- Task 4 result:
  - Local deployment is up and usable on `http://localhost:18080`, with healthy API/postgres/redis/OME/SRS/transcoder services and a verified admin login path.
  - The deployment workflow is not fully healthy yet because the Windows quickstart wrapper loses Docker from `PATH`, and the Compose migration job silently no-ops due to the `postgres-migrations` command wiring.
  - `bitriver-transcoder-public` still logs `setgid(101) failed (1: Operation not permitted)` under the current hardened container settings, so the local shakedown did not treat the HLS nginx sidecar as fully healthy.
- Task 4 checks:
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps -a`
  - `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=80 bitriver-live viewer postgres redis srs-controller ome transcoder transcoder-public`

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

## Scoped change: viewer discovery journey polish

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the scoped discovery-journey plan before editing
  - Acceptance criteria:
    - `PLAN.md` captures the discovery-navigation scope, assumptions, risks, and focused viewer validation commands.
    - `TASKS.md` lists the ordered implementation/validation steps before viewer files change.
    - The read-only diagnosis identifies the concrete dead-end controls or passive recovery states to fix.

- [x] Task 2 - Make homepage and browse discovery controls actionable
  - Acceptance criteria:
    - Homepage category chips take the user into Browse with relevant context instead of acting like dead buttons.
    - Browse featured highlights open the corresponding channel or otherwise provide a direct watch action.
    - Any new routing/query behavior stays consistent with the current viewer `basePath`.

- [x] Task 3 - Persist browse topic drill-ins and improve recovery states
  - Acceptance criteria:
    - Browse can hydrate a selected topic/category filter from URL state and keep it stable through navigation updates.
    - The touched empty states include explicit next actions grounded in existing routes/behaviors.
    - Viewer tests are updated where the interaction contract changes.

- [x] Task 4 - Run focused validation and record the results
  - Acceptance criteria:
    - The targeted browse/homepage Jest coverage passes.
    - Viewer lint and production build are rerun and logged.
    - Any residual warnings or gaps are captured explicitly in the execution log.

### Execution log (viewer discovery journey polish)
- Task 1 complete: recorded the scoped discovery-journey polish plan in `PLAN.md` and `TASKS.md` before editing, based on the read-only diagnosis that the homepage category chips are dead buttons, the browse "Featured right now" cards are non-actionable visual blocks, and several discovery empty states still strand first-time visitors with passive copy instead of clear recovery paths.
- Task 1 checks:
  - `Get-Content web/viewer/components/CategoryRail.tsx`
  - `Get-Content web/viewer/app/browse/page.tsx`
  - `Get-Content web/viewer/components/ChannelRail.tsx`
  - `Get-Content web/viewer/components/LiveNowGrid.tsx`
  - `Get-Content web/viewer/components/DirectoryGrid.tsx`
- Task 2 complete: converted homepage category chips into real browse drill-ins, made browse featured highlights open their corresponding channels, and kept the new discovery navigation on internal viewer routes so `basePath` handling stays consistent.
- Task 2 checks:
  - `Get-Content web/viewer/components/CategoryRail.tsx`
  - `Get-Content web/viewer/app/browse/page.tsx`
  - `npm.cmd --prefix web/viewer run test -- __tests__/browsePage.test.tsx __tests__/channelDisplayPrimitives.test.tsx`
- Task 3 complete: added URL-backed browse topic hydration/synchronization, kept the selected topic visible through browse summaries/headings, upgraded the touched discovery empty states with explicit recovery actions instead of passive copy, and documented the new discovery drill-in behavior in `docs/quickstart.md`.
- Task 3 checks:
  - `Get-Content web/viewer/lib/directory-state.ts`
  - `Get-Content web/viewer/hooks/useDirectorySearch.ts`
  - `Get-Content web/viewer/components/FeaturedChannel.tsx`
  - `Get-Content web/viewer/components/DirectoryGrid.tsx`
  - `Get-Content docs/quickstart.md | Select-String -Pattern "homepage category chips drill into|browse workspace preserves the active topic" -Context 0,0`
  - `npm.cmd --prefix web/viewer run test -- __tests__/browsePage.test.tsx __tests__/channelDisplayPrimitives.test.tsx`
- Task 4 complete: reran the planned browse/homepage Jest coverage, passed viewer lint and production build, and attempted the repo-native viewer verification script. The UX pass is green at the targeted test/build level; the only failing closing check was the existing Windows host blocker where `verify.sh` cannot find Python for the env-placeholder hygiene step.
- Task 4 checks:
  - `npm.cmd --prefix web/viewer run test -- __tests__/browsePage.test.tsx __tests__/directoryPage.test.tsx __tests__/channelDisplayPrimitives.test.tsx`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh --viewer` (blocked by missing `python`/`python3` on this Windows host during the env example placeholder hygiene step)
- Task 4 notes:
  - The focused Jest suites passed with existing non-failing console noise from older `act(...)` warnings in search interactions and the longstanding `next/image` mock warnings about `fill`/`priority` props in jsdom.
  - `next build` completed successfully and surfaced the existing viewer/app warnings about pages being deopted into client-side rendering plus an outdated `caniuse-lite` database; neither warning blocked the production build.

## Scoped change: Git push repair

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the scoped push-repair plan before changing Git state
  - Acceptance criteria:
    - `PLAN.md` captures the push-repair scope, assumptions, risks, and diagnostic commands.
    - `TASKS.md` lists the ordered diagnosis/repair/verification steps before branch or push actions change repo state.
    - The read-only diagnosis identifies whether divergence, auth, or remote policy is the likely blocker.

- [x] Task 2 - Reproduce the push failure and identify the precise remote rejection
  - Acceptance criteria:
    - The exact `git push` failure mode is captured.
    - The repo state needed to choose a safe repair path is documented in the execution log.

- [x] Task 3 - Repair the push path without rewriting shared `main` history
  - Acceptance criteria:
    - The current commits are placed on a safe remote branch or otherwise pushed successfully.
    - No force-push to `main` is used unless the user explicitly asked for it.
    - Local Git state remains clean and understandable afterward.

- [x] Task 4 - Verify the repaired remote state and record the outcome
  - Acceptance criteria:
    - The successful push target is logged.
    - Any remaining auth/policy blockers are recorded precisely if they still prevent publication.

### Execution log (Git push repair)
- Task 1 complete: recorded the push-repair scope in `PLAN.md` and `TASKS.md` before changing Git state, based on the read-only diagnosis that local `main` is currently `ahead 2, behind 22` relative to `origin/main`, which makes a non-fast-forward rejection the most likely push blocker.
- Task 1 checks:
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live status --short --branch`
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live remote -v`
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live branch -vv`
- Task 2 complete: reproduced the actual push against GitHub and confirmed the blocker is the expected non-fast-forward rejection on `main` (`[rejected] main -> main (fetch first)`), not a bad credential or branch-name issue. The local branch is ahead by the two unpublished commits `b4617f75 restored signin ui` and `4f34a498 Polish viewer discovery navigation and recovery states`.
- Task 2 checks:
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live push origin main` (sandbox blocked on outbound network)
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live push origin main` (with host network access; rejected as non-fast-forward)
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live log --oneline --decorate origin/main..HEAD`
- Task 3 complete: created a safe repair branch from the current `HEAD` and pushed that branch to GitHub with upstream tracking instead of rebasing or force-pushing shared `main`. The local environment refused the repo-preferred `fix/...` nested ref path inside `.git`, so the repair used the flat branch name `fix-viewer-discovery-polish`.
- Task 3 checks:
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live switch -c fix/viewer-discovery-polish` (blocked locally by ref creation/permission issues)
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live switch -c fix-viewer-discovery-polish`
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live push -u origin fix-viewer-discovery-polish`
- Task 4 complete: verified that `fix-viewer-discovery-polish` now tracks `origin/fix-viewer-discovery-polish` successfully. The original `main` branch still remains locally diverged from `origin/main` (`ahead 2, behind 22`), but the unpublished commits are now safely available on GitHub and ready for PR flow.
- Task 4 checks:
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live status --short --branch`
  - `git -c safe.directory=C:/Users/RhythmicCarnage/Desktop/BitRiver-Live branch -vv`
- Task 4 notes:
  - The working tree still shows modified `PLAN.md` and `TASKS.md` because this repair followed the repo's spec-driven workflow and recorded the diagnosis/results there; no source-code files were changed as part of the push repair itself.
  - GitHub reported the new-branch PR URL: `https://github.com/ProhibitedTV/BitRiver-Live/pull/new/fix-viewer-discovery-polish`

## Scoped change: production-like local OBS rehearsal

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the production-like rehearsal plan before editing contract files
  - Acceptance criteria:
    - `PLAN.md` captures the LAN rehearsal scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered env/digest/boot tasks before changing `.env` or generated config files.
    - The read-only diagnosis identifies whether the saved first-party image tags are real release artifacts or placeholders.

- [x] Task 2 - Authenticate registry access and verify release-image availability
  - Acceptance criteria:
    - GHCR login is verified from this host.
    - The configured first-party release tag is checked directly in GHCR.
    - The repo's upstream Git release-tag state is checked so missing images can be attributed correctly.

- [ ] Task 3 - If release images exist, update the LAN rehearsal contract files and pin digests
  - Acceptance criteria:
    - Root `.env` uses `10.0.0.108` for the OME/viewer/transcoder public URLs needed by this rehearsal.
    - `deploy/ome/Server.generated.xml` is re-rendered from the updated env file.
    - All required `*_IMAGE_DIGEST` values are pinned in `.env`.

- [ ] Task 4 - Run production-path validation and record the result
  - Acceptance criteria:
    - `go run ./cmd/bitriver env validate --env-file ./.env` passes.
    - `docker compose --env-file .env -f deploy/docker-compose.yml config` passes.
    - `./scripts/require-image-digests.sh` passes.
    - `go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml` either boots successfully or yields a precisely documented next blocker.

### Execution log (production-like local OBS rehearsal)
- Task 1 complete: recorded the requested production-like LAN rehearsal scope in `PLAN.md`/`TASKS.md` before mutating the deployment contract, based on the read-only diagnosis that the current `.env` still uses placeholder/loopback networking values and empty image digests.
- Task 1 checks:
  - `git diff --stat`
  - `Get-Content .env | Select-String -Pattern '^(BITRIVER_LIVE_MODE|BITRIVER_DEPLOY_IMAGE_SOURCE|BITRIVER_LIVE_PORT|BITRIVER_OME_BIND|BITRIVER_OME_IP|BITRIVER_TRANSCODER_PUBLIC_BASE_URL|BITRIVER_TRANSCODER_HOST_PORT|BITRIVER_TRANSCODER_PUBLIC_PORT|NEXT_PUBLIC_VIEWER_URL|BITRIVER_LIVE_IMAGE_TAG|BITRIVER_VIEWER_IMAGE_TAG|BITRIVER_SRS_CONTROLLER_IMAGE_TAG|BITRIVER_TRANSCODER_IMAGE_TAG|BITRIVER_LIVE_IMAGE_DIGEST|BITRIVER_VIEWER_IMAGE_DIGEST|BITRIVER_SRS_CONTROLLER_IMAGE_DIGEST|BITRIVER_TRANSCODER_IMAGE_DIGEST|BITRIVER_SRS_IMAGE_DIGEST|BITRIVER_OME_IMAGE_DIGEST|BITRIVER_REDIS_IMAGE_DIGEST|BITRIVER_POSTGRES_IMAGE_DIGEST|BITRIVER_NGINX_IMAGE_DIGEST|BITRIVER_ALPINE_3_IMAGE_DIGEST|BITRIVER_ALPINE_3_19_IMAGE_DIGEST|BITRIVER_DEBIAN_IMAGE_DIGEST)='`
  - `Get-Content docs/contract-change-recipe.md`
- Task 2 complete: GHCR access is now authenticated from this host, but the requested pull-mode rehearsal is blocked upstream because the configured first-party image tag does not exist and the remote repo has no Git release tags.
- Task 2 findings:
  - `gh auth status` reports an active `ProhibitedTV` GitHub CLI session on this machine.
  - `gh auth token | docker login ghcr.io -u ProhibitedTV --password-stdin` succeeded, so registry auth is no longer the blocker.
  - `docker buildx imagetools inspect ghcr.io/bitriver-live/bitriver-live:v1.2.3 --format "{{.Manifest.Digest}}"` returned `not found`, which means the configured first-party release tag is not currently published in GHCR.
  - `git ls-remote --tags origin` returned no tags, which strongly suggests the saved `v1.2.3` values in `.env` / `deploy/.env.example` / docs are placeholders rather than release artifacts that this checkout can pull today.
  - `docker buildx imagetools inspect redis:7-alpine --format "{{.Manifest.Digest}}"` succeeded, so the third-party digest path is healthy and only the unpublished first-party images are blocking pull mode.
- Task 2 checks:
  - `gh auth status`
  - `gh auth token | docker login ghcr.io -u ProhibitedTV --password-stdin`
  - `docker buildx imagetools inspect ghcr.io/bitriver-live/bitriver-live:v1.2.3 --format "{{.Manifest.Digest}}"`
  - `docker buildx imagetools inspect redis:7-alpine --format "{{.Manifest.Digest}}"`
  - `git ls-remote --tags origin`

## Scoped change: build-mode local OBS rehearsal fix

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the build-mode rehearsal plan before editing contract files
  - Acceptance criteria:
    - `PLAN.md` captures the build-mode fallback scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered compose/script/docs/runtime steps before contract edits begin.
    - The read-only diagnosis identifies whether the repo needs real contract fixes or only local workarounds.

- [x] Task 2 - Fix the Compose contract blockers for source-build rehearsal
  - Acceptance criteria:
    - `deploy/docker-compose.yml` runs `postgres-migrations` through one intact shell command instead of the broken `entrypoint + scalar command` combination.
    - `deploy/docker-compose.yml` grants `transcoder-public` the capabilities required by the bundled nginx config under the current hardening model.
    - `docs/contract.md` documents the updated hardening exception accurately.

- [x] Task 3 - Make quickstart smoke and operator docs source-checkout-safe
  - Acceptance criteria:
    - `scripts/test-quickstart.sh` no longer assumes unpublished first-party GHCR images exist.
    - The smoke path uses local build mode for first-party images while keeping the rest of the guardrails intact.
    - `docs/quickstart.md` and `deploy/README.md` clearly separate release-image pull mode from source-checkout local rehearsal/build mode.
    - The repo smoke/verify wrappers accept the working Python launcher on this Windows host and their OME compose assertions match the current `docker compose config` output.

- [x] Task 4 - Prepare the local rehearsal runtime state
  - Acceptance criteria:
    - Root `.env` uses the planned development/build/LAN viewer/HLS values for this host rehearsal while keeping OME on container-safe bind/public settings that actually boot under Compose.
    - `deploy/ome/Server.generated.xml` is re-rendered from the updated env file.
    - The resulting env/config pairing is suitable for local compose boot on the default ports.

- [x] Task 5 - Run validation, bring the stack up, and record the rehearsal result
  - Acceptance criteria:
    - The targeted Go/storage test and Compose config validation pass.
    - Local build-mode Compose startup is exercised with status/log inspection.
    - `scripts/test-quickstart.sh` is rerun after the fixes.
    - `./scripts/verify.sh` is attempted and any remaining host blocker is recorded precisely.

### Execution log (build-mode local OBS rehearsal fix)
- Task 1 complete: recorded the build-mode fallback scope in `PLAN.md`/`TASKS.md` before mutating the deployment contract, based on the read-only diagnosis that `production + pull` cannot work from this checkout because first-party GHCR images do not exist, while the source-build fallback still needs real contract fixes rather than ad-hoc manual workarounds.
- Task 1 findings:
  - `docs/quickstart.md` already frames `--image-source build` as development-only and separate from strict production pull mode.
  - `deploy/docker-compose.yml` still has the broken `postgres-migrations` `entrypoint: ["/bin/sh","-c"]` plus scalar `command` combination that Compose tokenizes incorrectly.
  - `deploy/nginx/transcoder-public.conf` still declares `user nginx;`, while `transcoder-public` currently keeps only `CHOWN`, which matches the earlier `setgid(101) failed` runtime failure.
  - `scripts/test-quickstart.sh` seeds `BITRIVER_*_IMAGE_TAG=latest` and starts plain `docker compose up -d`, which cannot succeed from a source checkout because `ghcr.io/bitriver-live/bitriver-live:latest` is also unpublished.
- Task 1 checks:
  - `Get-Content PLAN.md | Select-Object -Last 60`
  - `Get-Content TASKS.md | Select-Object -Last 80`
  - `Get-Content docs/quickstart.md | Select-Object -Skip 120 -First 80`
  - `Get-Content cmd/bitriver/commands_env_compose.go | Select-Object -Skip 520 -First 140`
  - `Get-Content deploy/docker-compose.yml | Select-Object -Skip 182 -First 52`
  - `Get-Content deploy/docker-compose.yml | Select-Object -Skip 533 -First 23`
  - `Get-Content scripts/test-quickstart.sh | Select-Object -First 220`
  - `docker manifest inspect ghcr.io/bitriver-live/bitriver-live:latest` (confirmed unpublished)
- Task 2 complete: rewired `postgres-migrations` to run through one intact `/bin/sh -c` command array, expanded `transcoder-public` capabilities to include `SETGID` and `SETUID` for the bundled `user nginx;` config, and updated `docs/contract.md` so the hardening exception matches the runtime requirement.
- Task 2 checks:
  - `docker compose --env-file .env -f deploy/docker-compose.yml config`
  - `Get-Content deploy/docker-compose.yml | Select-Object -Skip 182 -First 52`
  - `Get-Content deploy/docker-compose.yml | Select-Object -Skip 533 -First 23`
  - `Get-Content docs/contract.md | Select-String -Pattern 'transcoder-public retains CHOWN, SETGID, and SETUID' -Context 0,0`
- Task 3 complete: switched the repo smoke path to explicit source-checkout build mode by exporting build-mode overrides, clearing first-party digests for direct Compose interpolation, generating a temporary development/build `.env` when needed, and starting the stack with `docker compose ... up -d --build --pull never`. Updated `docs/quickstart.md` and `deploy/README.md` to separate release-image pull mode from local source-build rehearsals and to stop implying a saved production `.env` can simply be paired with a CLI `--build` flag. While validating the wrapper on this Windows host, also fixed the stale OME mount/API-port assertions in `scripts/test-quickstart.sh` and added Python launcher fallbacks in `scripts/test-quickstart.sh`, `scripts/check-env-example-placeholders.sh`, `scripts/check-contract-invariants.sh`, and `scripts/verify.sh`.
- Task 3 checks:
  - `Get-Content scripts/test-quickstart.sh | Select-String -Pattern 'BITRIVER_DEPLOY_IMAGE_SOURCE=build|BITRIVER_LIVE_MODE=development|up -d --build --pull never|BITRIVER_LIVE_IMAGE_DIGEST=""|BITRIVER_VIEWER_IMAGE_DIGEST=""|BITRIVER_SRS_CONTROLLER_IMAGE_DIGEST=""|BITRIVER_TRANSCODER_IMAGE_DIGEST=""' -Context 0,0`
  - `Get-Content docs/quickstart.md | Select-String -Pattern 'source-checkout local rehearsals|docker compose --env-file \.env -f deploy/docker-compose.yml up -d --build --pull never|saved \.env to local build settings|published-image pull contract' -Context 0,0`
  - `Get-Content deploy/README.md | Select-String -Pattern 'source checkout|docker compose --env-file \.env -f \"\$COMPOSE_FILE\" up -d --build --pull never|first-party digest variables empty' -Context 0,0`
  - `Get-Content scripts/check-env-example-placeholders.sh`
  - `Get-Content scripts/check-contract-invariants.sh`
  - `Get-Content scripts/verify.sh | Select-Object -First 80`
  - `& 'C:\Program Files\Git\bin\bash.exe' -n ./scripts/test-quickstart.sh` (host-local MSYS signal-pipe failure on this machine; the real smoke execution remains part of Task 5)
- Task 4 complete: updated the working-copy root `.env` to development/build mode with LAN viewer/HLS URLs (`http://10.0.0.108:8080/viewer` and `http://10.0.0.108:9080/hls`) and kept the default service ports intact. Live boot then proved that containerized OME cannot bind its listeners to the host LAN IP, so the final working Compose rehearsal state keeps `BITRIVER_OME_BIND=0.0.0.0` and `BITRIVER_OME_IP=0.0.0.0` while still exposing viewer/HLS traffic on the LAN host. Re-rendered `deploy/ome/Server.generated.xml` after each env correction so the generated config stayed aligned with the live compose inputs.
- Task 4 checks:
  - `Get-Content .env | Select-String -Pattern '^(BITRIVER_DEPLOY_IMAGE_SOURCE|BITRIVER_LIVE_MODE|BITRIVER_OME_BIND|BITRIVER_OME_IP|BITRIVER_TRANSCODER_PUBLIC_BASE_URL|NEXT_PUBLIC_VIEWER_URL)=' -Context 0,0`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go run ./cmd/bitriver ome render --force --env-file ./.env --quiet`
  - `Get-Content deploy/ome/Server.generated.xml | Select-String -Pattern '<IP>0\.0\.0\.0</IP>|<Port>8081</Port>|<AccessToken>' -Context 0,0`
- Task 5 complete: the focused storage ingest test passed, Compose config rendered successfully, the source-build stack built and booted on the default ports after correcting the OME bind/IP assumption, `/readyz`, `/admin`, and `/viewer` all returned `200`, the repo-native quickstart smoke wrapper passed end-to-end after the Windows portability fixes, and the stack was brought back up with an explicitly bootstrapped admin account ready for manual sign-in. `./scripts/verify.sh` was also rerun; it no longer fails on missing `python3`, but it still reports unrelated existing Go-suite failures in `cmd/transcoder` and `internal/auth`.
- Task 5 checks:
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/storage -count=1 -timeout=120s -run TestIngestPipelineEndToEnd`
  - `docker compose --env-file .env -f deploy/docker-compose.yml config`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build --pull never`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps -a`
  - `docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=120 postgres-migrations bitriver-live transcoder-public`
  - `Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8080/readyz`
  - `Invoke-WebRequest -UseBasicParsing http://10.0.0.108:8080/admin`
  - `Invoke-WebRequest -UseBasicParsing http://10.0.0.108:8080/viewer`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/test-quickstart.sh`
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d`
  - `docker compose --env-file .env -f deploy/docker-compose.yml exec -T bitriver-live /app/bootstrap-admin --postgres-dsn <compose-network-dsn> --email <env admin email> --password <env admin password>`
- Task 5 notes:
  - The original rehearsal plan’s `BITRIVER_OME_BIND=10.0.0.108` / `BITRIVER_OME_IP=10.0.0.108` assumption is not compatible with the current containerized OME runtime on this host; OME treats that address as a real listener bind and fails with `Cannot assign requested address`. The working local Compose rehearsal therefore keeps OME bound to `0.0.0.0`.
  - `./scripts/verify.sh` is no longer blocked by the Windows Python shim name, but it still fails in pre-existing suites unrelated to this rehearsal work: `cmd/transcoder` (`unexpected ffmpeg path` / health recovery assertions) and `internal/auth` (`TestValidateHonorsAbsoluteTTL`).
  - The final stack was deliberately brought back up after `verify.sh` cleaned it down so manual browser/OBS testing can start immediately from the current checkout.

## Scoped change: viewer chrome fixes for theme/auth/top spacing

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Reproduce and document the viewer chrome failures in the current runtime/code path
  - Acceptance criteria:
    - The read-only notes identify where the theme toggle writes state, where the auth dialog mounts, and which final CSS rules control the shell top spacing.
    - `PLAN.md` and `TASKS.md` are updated for this scope before product code changes begin.
    - The likely runtime failure mode is specific enough to guide a minimal patch rather than a blind redesign.

- [x] Task 2 - Repair the theme toggle and auth CTA behavior
  - Acceptance criteria:
    - The light/dark toggle updates the active document theme reliably in the live viewer.
    - Signed-out sign-in and join/sign-up CTAs visibly trigger the intended auth flow.
    - Existing auth-link behavior for optional external signup destinations remains intact.

- [x] Task 3 - Remove the unintended shell gap and tighten the header/content alignment
  - Acceptance criteria:
    - The main viewer shell no longer adds redundant top spacing above the left rail or main content.
    - Desktop and mobile layouts still clear the fixed navbar cleanly.
    - The spacing fix stays inside viewer CSS/components and does not alter deployment/runtime docs.

- [x] Task 4 - Add or adjust focused viewer coverage and validate the result
  - Acceptance criteria:
    - Jest coverage reflects the theme/auth/runtime fixes.
    - Viewer lint and build pass.
    - Any residual issue or unverified browser-only behavior is recorded explicitly.

### Execution log (viewer chrome fixes for theme/auth/top spacing)
- Task 1 complete: finished the read-only diagnosis and then reproduced the live viewer failure path with targeted Playwright coverage. The actual blocker was narrower than the initial guess: the hidden mobile nav drawer still occupied the right edge of the viewport and intercepted pointer events, which made the visible theme and auth controls appear dead. The layout read also confirmed that the final rescue-pass `.viewer-shell` padding was applied at the whole-shell level, which explains the extra gap above the left rail.
- Task 1 checks:
  - `rg -n "theme|dark|light|toggle|sign in|sign up|signup|login|auth|gap|layout|shell|Navbar|Providers|useAuth|theme" web/viewer -g "!*node_modules*"`
  - `Get-Content web/viewer/components/Navbar.tsx`
  - `Get-Content web/viewer/hooks/useAuth.tsx`
  - `Get-Content web/viewer/components/auth/AuthDialog.tsx`
  - `Get-Content web/viewer/components/Providers.tsx`
  - `Get-Content web/viewer/components/ViewerShell.tsx`
  - `Get-Content web/viewer/app/layout.tsx`
  - `Get-Content web/viewer/styles/home.css`
  - `Get-Content web/viewer/styles/globals.css`
  - `Get-Content web/viewer/__tests__/navbar.test.tsx`
  - `npm.cmd --prefix web/viewer run test:playwright -- tests/channel.spec.ts --grep "theme toggle updates the rendered document|navbar sign-in button redirects to the configured login URL"` (first run reproduced the live failures; hidden `.nav-drawer` intercepted pointer events on the theme button, and the sign-in test exposed selector ambiguity because there were two visible sign-in buttons)
- Task 2 complete: fixed the click-dead navbar controls by making the hidden nav drawer truly non-interactive when `hidden` is present, then tightened the Playwright coverage so the top-right signed-out auth actions are exercised directly. The viewer now both redirects correctly when `loginUrl` is configured and opens the in-viewer sign-in/sign-up dialog flow when it is not.
- Task 2 checks:
  - `npm.cmd --prefix web/viewer run test -- __tests__/navbar.test.tsx`
  - `npm.cmd --prefix web/viewer run test:playwright -- tests/channel.spec.ts --grep "signed-out navbar actions open the in-viewer auth flow when no external login URL is configured|navbar sign-in button redirects to the configured login URL|theme toggle updates the rendered document"`
- Task 3 complete: moved the navbar-clearance spacing off the whole `.viewer-shell` and onto `.viewer-shell__content-inner`, which removes the unexplained gap above the left rail/main content while keeping the fixed navbar clearance where the content actually needs it.
- Task 3 checks:
  - `Get-Content web/viewer/styles/globals.css | Select-String -Pattern '\.nav-drawer\[hidden\]|\.viewer-shell \{|padding-top: var\(--viewer-top-offset\);' -Context 0,2`
  - `npm.cmd --prefix web/viewer run test:playwright -- tests/channel.spec.ts --grep "theme toggle updates the rendered document"`
- Task 4 complete: added runtime-focused Playwright coverage for the local in-viewer auth flow, reran the focused navbar Jest suite, and finished with clean viewer lint/build passes.
- Task 4 checks:
  - `npm.cmd --prefix web/viewer run test -- __tests__/navbar.test.tsx`
  - `npm.cmd --prefix web/viewer run test:playwright -- tests/channel.spec.ts --grep "signed-out navbar actions open the in-viewer auth flow when no external login URL is configured|navbar sign-in button redirects to the configured login URL|theme toggle updates the rendered document"`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`

## Scoped change: BitRiver Live v1 DLive-replacement overhaul

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Add self-serve first-channel creation support across backend and viewer client
  - Acceptance criteria:
    - Authenticated self-signup users can create their own first channel through `POST /api/channels` without already holding the creator role.
    - Channel creation remains self-only for non-admins, and creating channels for another owner is still admin-only.
    - Successful first-channel creation upgrades the account to creator-capable immediately.
    - The viewer web client exposes a typed create-channel helper that the onboarding flow can use directly.

- [x] Task 2 - Rebuild creator onboarding around first-channel setup and go-live actions
  - Acceptance criteria:
    - The creator entry flow clearly guides a new account through channel creation, OBS setup, preview readiness, and sharing the viewer link.
    - Signed-in users without a channel land in a visible recovery/setup path instead of dead-end or permission-error states.
    - The creator onboarding experience works cleanly on mobile and desktop.

- [x] Task 3 - Refresh public navigation and information architecture
  - Acceptance criteria:
    - The top-level viewer IA is reoriented around `Home`, `Browse`, `Following`, `Go Live`, `Videos`, and `Profile`.
    - Primary CTAs either navigate, open auth, or perform an action immediately; no silent dead buttons remain in the touched surfaces.
    - The signed-in and signed-out navigation states both feel intentional and consistent across viewport sizes.

- [x] Task 4 - Upgrade the home, browse, and channel experiences to feel productized
  - Acceptance criteria:
    - Home and browse prioritize live discovery and clear category browsing over setup/demo copy.
    - The channel page is explicitly video-first with clear follow, tip, share, chat, and VOD actions above the fold.
    - Subscriptions remain supported but are visually secondary to tips and live watching.

- [x] Task 5 - Add focused coverage and validate the overhaul
  - Acceptance criteria:
    - Focused backend or viewer tests cover the creator bootstrap path and the touched navigation/channel flows.
    - Viewer lint and production build pass.
    - Any remaining gaps, especially around full manual OBS playback, are recorded explicitly.

### Execution log (BitRiver Live v1 DLive-replacement overhaul)
- Task 1 started: the read-only analysis confirmed that the current product already has live discovery, playback, chat, follow, tip, creator dashboard, and VOD primitives, so the right first move is enabling self-serve creator bootstrap rather than attempting a greenfield rewrite. The concrete backend gap is that `POST /api/channels` currently requires `creator` or `admin`, while self-signup users are created as `viewer` by default. The viewer-side gap is that `web/viewer/lib/viewer-api-channel.ts` has no typed `createChannel` helper yet, which is why the current onboarding flow cannot create a first channel directly.
- Task 1 checks:
  - `Get-Content web/viewer/lib/viewer-api-channel.ts`
  - `Get-Content web/viewer/lib/viewer-api-types.ts`
  - `Get-Content web/viewer/lib/navigation.ts`
  - `Get-Content web/viewer/app/page.tsx`
  - `Get-Content web/viewer/app/following/page.tsx`
  - `Get-Content web/viewer/app/channels/[id]/page.tsx`
  - `Get-Content web/viewer/app/creator/page.tsx`
  - `Get-Content web/viewer/app/creator/getting-started/page.tsx`
  - `Get-Content web/viewer/app/creator/live/[channelId]/page.tsx`
  - `Get-Content web/viewer/components/ChannelHero.tsx`
  - `Get-Content web/viewer/components/ChatPanel.tsx`
  - `Get-Content web/viewer/components/TipDrawer.tsx`
  - `Get-Content internal/api/auth_users_handlers.go`
  - `Get-Content internal/api/channels_directory_handlers.go`
  - `Get-Content internal/service/usecases.go`
  - `Get-Content internal/storage/types.go`
  - `Get-Content internal/storage/storage.go`
  - `rg -n "create channel|POST /api/channels|roleCreator|SelfSignup|follow|tip|vod|creator" internal web/viewer -g "!*node_modules*"`
- Task 1 complete: relaxed `POST /api/channels` to authenticate any signed-in user, reload the latest persisted actor roles, allow self-only first-channel creation for non-admin/non-creator users, and upgrade the owner to include the `creator` role after a successful first channel. Added a typed `createChannel` helper and payload export to the viewer API layer so the creator onboarding UI can create channels directly without inventing its own fetch wrapper. If the owner-role upgrade ever fails after channel creation, the handler now rolls the new channel back so onboarding does not leave half-created accounts behind.
- Task 1 follow-up checks:
  - `gofmt -w internal/service/usecases.go internal/api/channels_directory_handlers.go internal/api/handlers_test.go`
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/api -count=1 -timeout=120s -run "Test(SelfSignupUserCanCreateFirstChannelAndBecomeCreator|ViewerCannotCreateChannelForAnotherOwner|ChannelStreamLifecycle)$"`
- Task 2 complete: rebuilt the creator entry flow around actual first-channel bootstrap instead of passive guidance. Signed-in viewers without channels now land on a visible setup path, can create their first channel directly from the getting-started page, refresh into creator-capable state immediately, and then continue into the same OBS, preview, share-link, and uploads flow used by existing creators. The creator overview page now distinguishes guest, first-channel, and active-creator states so the top-level studio route always gives the user a real next action instead of a dead-end.
- Task 2 checks:
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run test -- __tests__/useAuth.test.tsx`
  - `npm.cmd --prefix web/viewer run test -- __tests__/creatorGettingStartedPage.test.tsx -t "lets a signed-in viewer create a first channel and unlock live setup"`
  - `npm.cmd --prefix web/viewer run test -- __tests__/creatorGettingStartedPage.test.tsx __tests__/creatorPage.test.tsx __tests__/creatorLivePage.test.tsx`
- Task 3 complete: refreshed the public viewer IA around the literal destinations `Home`, `Browse`, `Following`, `Go Live`, `Videos`, and signed-in `Profile`, then tightened the navbar copy and CTA behavior so the main actions either navigate immediately, open auth, or send a creator into a real setup/live destination. Added a dedicated `/videos` surface so replay discovery is a first-class destination instead of being buried only inside channel tabs.
- Task 3 checks:
  - `Get-Content web/viewer/lib/navigation.ts`
  - `Get-Content web/viewer/components/Navbar.tsx`
  - `Get-Content web/viewer/components/ViewerShell.tsx`
  - `Get-Content web/viewer/app/layout.tsx`
  - `Get-Content web/viewer/app/videos/page.tsx`
  - `Get-Content web/viewer/components/auth/AuthDialog.tsx`
- Task 4 complete: reworked the home, browse, and channel surfaces so the product reads like a real live platform instead of a prototype. Home now leads with live discovery and stronger recovery CTAs, browse has clearer live/category framing, the channel view is more video-first with follow/tip/share above the fold and replay access called out when offline, and the shared styling now supports the new replay/cards/navigation surfaces across desktop and mobile.
- Task 4 checks:
  - `Get-Content web/viewer/app/directory-view.tsx`
  - `Get-Content web/viewer/app/browse/page.tsx`
  - `Get-Content web/viewer/app/channels/[id]/page.tsx`
  - `Get-Content web/viewer/components/ChannelHero.tsx`
  - `Get-Content web/viewer/components/DirectoryGrid.tsx`
  - `Get-Content web/viewer/components/CategoryRail.tsx`
  - `Get-Content web/viewer/components/FeaturedChannel.tsx`
  - `Get-Content web/viewer/styles/globals.css`
- Task 5 complete: added focused browser/unit coverage for the new creator/live and replay navigation paths, updated the Playwright specs to the refreshed IA, and fixed one real runtime regression uncovered during validation: changing the tip currency was re-triggering the drawer reset logic and wiping the form before submit. After the fix, the targeted Jest suites, targeted Playwright suite, viewer lint, and production build all passed.
- Task 5 checks:
  - `npm.cmd --prefix web/viewer run test -- __tests__/navigation.test.ts __tests__/navbar.test.tsx __tests__/directoryPage.test.tsx __tests__/channelHero.test.tsx __tests__/channelPage.test.tsx __tests__/videosPage.test.tsx __tests__/creatorGettingStartedPage.test.tsx __tests__/creatorPage.test.tsx`
  - `npm.cmd --prefix web/viewer run test -- __tests__/tipDrawer.test.tsx __tests__/channelHero.test.tsx`
  - `npm.cmd --prefix web/viewer run test:playwright -- tests/channel.spec.ts tests/creator-live-setup.spec.ts tests/navbar-mobile.spec.ts`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
- Task 5 notes:
  - Targeted Jest coverage still emits existing non-failing React `act(...)` warnings in some creator/search/tip tests, but the suites pass and the new overhaul behavior is covered.
  - Viewer production builds still report the existing Next client-render deopt warnings for several routes (`/`, `/browse`, `/following`, `/videos`, `/creator`, `/creator/getting-started`, `/profile`, `/getting-started`, `/404`); those warnings predate this pass and do not fail the build.
  - This overhaul pass did not rerun a live manual OBS broadcast after the final UI changes, so the remaining manual acceptance item is still a human ingest/playback rehearsal against the running stack.

## Scoped change: redeploy viewer overhaul and live functionality check

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the redeploy/runtime-check scope before touching the running stack
  - Acceptance criteria:
    - `PLAN.md` captures the redeploy scope, assumptions, risks, and live validation commands.
    - `TASKS.md` lists the ordered redeploy and runtime-check steps before Docker commands begin.
    - The scope stays limited to redeploying and validating the current checkout unless runtime checks reveal a concrete regression.

- [ ] Task 2 - Redeploy the current checkout onto the local Compose stack
  - Acceptance criteria:
    - The stack is rebuilt and relaunched from the current checkout with the existing local source-build workflow.
    - `docker compose ps -a` shows the expected long-lived services running and `postgres-migrations` completed.
    - If the redeploy fails, the blocking service/log is captured explicitly in the execution log.

- [x] Task 3 - Run live functionality checks against the redeployed stack and repair any runtime blocker they expose
  - Acceptance criteria:
    - API readiness and the key public viewer routes respond successfully on the live deployment host.
    - The redeployed viewer hydrates in a real browser so the shipped theme/auth/nav controls respond to input.
    - The creator-facing route needed for the new go-live flow is reachable.
    - Any remaining manual-only gap, such as authenticated interaction or real OBS ingest, is recorded explicitly.

### Execution log (redeploy viewer overhaul and live functionality check)
- Task 1 complete: recorded the redeploy/runtime-check scope in `PLAN.md` and queued the runtime steps here before reusing the local Docker stack. The read-only check also confirmed that the repo is already in a dirty worktree from earlier release/runtime work, so this pass must avoid destructive cleanup and focus on rebuilding the current checkout in place.
- Task 1 checks:
  - `Get-Content PLAN.md | Select-Object -Last 80`
  - `Get-Content TASKS.md | Select-Object -Last 120`
  - `git status --short`
- Task 2 complete: rebuilt and relaunched the local source-build stack with `docker compose ... up -d --build --pull never`, then verified the expected service state with `docker compose ps -a`. The redeploy succeeded cleanly: `bitriver-live` came back healthy, `bitriver-viewer` restarted from the current checkout, and `postgres-migrations` exited successfully once. A follow-up `docker compose logs` attempt hit a host Docker permission issue, but that did not block the actual rebuild or service restart.
- Task 2 checks:
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build --pull never`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps -a`
- Task 3 in progress: the live HTTP checks show `/readyz` and the public `/viewer` host route responding, and the rendered navbar links already resolve to `/viewer/...` paths. The real blocker turned out to be deeper: a headless browser smoke reproduced the user's “buttons do nothing” report because the API server's default CSP rejects Next.js inline bootstrap scripts on the proxied viewer pages. With hydration blocked, the theme toggle never updates `data-theme`, sign-in never opens the auth dialog, and route bodies such as `/viewer/browse` stay inert even though the HTML shell loads. The next step is a minimal server-side CSP fix plus a fresh redeploy and rerun of the live smoke.
- Task 3 checks so far:
  - `Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8080/readyz`
  - `Invoke-WebRequest -UseBasicParsing http://10.0.0.108:8080/viewer`
  - `Invoke-WebRequest -UseBasicParsing http://10.0.0.108:8080/viewer/browse`
  - `Invoke-WebRequest -UseBasicParsing http://10.0.0.108:8080/viewer/videos`
  - `Invoke-WebRequest -UseBasicParsing http://10.0.0.108:8080/viewer/creator`
  - Headless Playwright smoke against `http://10.0.0.108:8080/viewer` (confirmed correct `/viewer/...` link hrefs, but no theme/auth interaction and no hydrated route content)
  - Headless Playwright console capture against `http://10.0.0.108:8080/viewer` (captured repeated CSP `script-src 'self'` violations for inline Next.js bootstrap scripts)
- Task 3 complete: fixed the live runtime blocker by moving the viewer-specific CSP handling onto the proxied `/viewer` responses themselves and relaxing only that route family to `script-src 'self' 'unsafe-inline'`, which lets the shipped Next.js bootstrap scripts hydrate without weakening the API/admin defaults. Added focused `internal/server` coverage for both the middleware and proxied viewer route behavior, rebuilt the `bitriver-live` container, and reran the live browser smoke. The redeployed viewer now hydrates and responds: the theme toggle changes state again, the sign-in CTA opens the in-viewer auth dialog with the expected `?auth=signin&next=%2Fviewer` state, and `/viewer/browse` plus `/viewer/creator` render real page content instead of inert shells. The remaining gap is still manual/authenticated broadcast validation, especially a real OBS ingest and public playback pass.
- Task 3 finishing checks:
  - `New-Item -ItemType Directory -Force .gocache | Out-Null; $env:GOCACHE=(Resolve-Path .gocache).Path; $env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'; go test ./internal/server -count=1 -timeout=120s`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build --pull never bitriver-live`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps -a`
  - `Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8080/readyz`
  - `Invoke-WebRequest -UseBasicParsing http://10.0.0.108:8080/viewer` (confirmed `Content-Security-Policy` now includes `script-src 'self' 'unsafe-inline'`)
  - Headless Playwright smoke against `http://10.0.0.108:8080/viewer` (confirmed theme toggle state change, sign-in dialog open, and hydrated browse/creator content)

## Scoped change: re-enable public signup and redesign homepage discovery

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the signup + homepage scope before changing runtime or viewer files
  - Acceptance criteria:
    - `PLAN.md` captures the signup unblock, homepage redesign goals, risks, and validation commands.
    - `TASKS.md` lists the runtime and viewer work in execution order before product edits begin.
    - The scope stays limited to self-signup availability plus the homepage/discovery experience.

- [x] Task 2 - Re-enable public self-signup on the local deployment
  - Acceptance criteria:
    - The local runtime no longer reports `allowSelfSignup=false` for guest viewer auth state.
    - The live `bitriver-live` service is restarted cleanly after the env change.
    - The signup CTA becomes available again in the shipped viewer.

- [x] Task 3 - Redesign the homepage into a more Twitch-style discovery surface
  - Acceptance criteria:
    - The homepage leads with live discovery and featured content instead of a marketing-first hero.
    - The layout clearly separates featured content, recommended/live shelves, and browse-by-topic entry points.
    - The signed-out experience makes account creation and watching feel immediate and intentional.

- [x] Task 4 - Add focused coverage, redeploy, and verify the live result
  - Acceptance criteria:
    - Focused viewer tests cover the updated homepage behavior and any signup-related CTA expectations touched by the change.
    - Viewer lint and production build pass.
    - The redeployed `/viewer` experience is verified in a live browser smoke, including homepage hydration and the return of public signup.

### Execution log (re-enable public signup and redesign homepage discovery)
- Task 1 complete: confirmed that the current signup failure is not a frontend no-op. The live API is intentionally returning `signup_disabled` because the local root `.env` currently sets `BITRIVER_LIVE_ALLOW_SELF_SIGNUP=false`, so guest registration is blocked before the auth dialog can succeed. The homepage read also confirmed that the current `/viewer` landing page still spends too much of its highest-value space on marketing copy and not enough on dense live discovery, which makes it feel less like Twitch and more like a polished prototype. The implementation path is therefore a small local runtime flip plus a viewer-only homepage restructure built on the existing discovery endpoints.
- Task 1 checks:
  - `git status --short`
  - `Get-Content PLAN.md | Select-Object -Last 120`
  - `Get-Content TASKS.md | Select-Object -Last 180`
  - `Get-Content .env | Select-String -Pattern "BITRIVER_LIVE_ALLOW_SELF_SIGNUP|NEXT_PUBLIC_VIEWER_URL|NEXT_VIEWER_BASE_PATH" -Context 0,0`
  - `Get-Content web/viewer/app/directory-view.tsx`
  - `Get-Content web/viewer/app/directory-page-shell.tsx`
  - `Get-Content web/viewer/styles/home.css`
  - `Get-Content web/viewer/components/FeaturedChannel.tsx`
  - `Get-Content web/viewer/components/ChannelRail.tsx`
  - `Get-Content web/viewer/__tests__/directoryPage.test.tsx`
- Task 2 complete: flipped the local runtime back to public signup by setting `BITRIVER_LIVE_ALLOW_SELF_SIGNUP=true` in the root `.env` and restarting `bitriver-live`. A real guest signup against the deployed API now succeeds again instead of returning `signup_disabled`; the smoke account created during validation came back with a real user id and viewer role on the live stack.
- Task 2 checks:
  - `Get-Content .env | Select-String -Pattern "BITRIVER_LIVE_ALLOW_SELF_SIGNUP" -Context 0,0`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d bitriver-live`
  - `Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/api/auth/signup -ContentType 'application/json' -Body ...` (returned `201` for a disposable smoke account after the restart)
- Task 3 complete: rebuilt the homepage into a denser, discovery-first landing experience with a featured live panel, recommendation stack, stronger browse/topic pivots, and a signed-out CTA hierarchy that feels closer to Twitch's information density without copying its visual language. The guest account-entry actions now use a real auth action component so the homepage `Create account` buttons open the in-viewer signup flow instead of silently only changing the query string.
- Task 3 checks:
  - `Get-Content web/viewer/app/directory-view.tsx`
  - `Get-Content web/viewer/styles/home.css`
  - `Get-Content web/viewer/components/auth/AuthActionLink.tsx`
  - `Get-Content web/viewer/__tests__/directoryPage.test.tsx`
- Task 4 complete: added homepage CTA regression coverage, reran the focused viewer validation, rebuilt the viewer image, restarted the live viewer container, and verified the deployed homepage in a headless browser. The live stack now shows the new `Live channels worth opening right now` hero, and clicking `Create account` on `http://10.0.0.108:8080/viewer` opens the `Create your BitRiver account` dialog on the running site. `docs/quickstart.md` now also documents the discovery-first homepage and the fact that homepage/navbar signup CTAs depend on `BITRIVER_LIVE_ALLOW_SELF_SIGNUP=true`.
- Task 4 checks:
  - `npm.cmd --prefix web/viewer run test -- __tests__/directoryPage.test.tsx`
  - `npm.cmd --prefix web/viewer run test -- __tests__/navbar.test.tsx`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
  - `docker compose --env-file .env -f deploy/docker-compose.yml build viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d viewer`
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps -a`
  - `node -e "const { chromium } = require('./web/viewer/node_modules/playwright'); ..."` (confirmed live homepage headline plus visible signup dialog on click)
- Task 4 notes:
  - Focused Jest still emits the existing non-failing `SearchBar` `act(...)` warnings, and `navbar.test.tsx` still logs the existing jsdom navigation warning; the suites pass and the updated homepage/signup path is covered.
  - Rebuilding the viewer required Docker access outside the sandbox on this host because Buildx needs `C:\Users\RhythmicCarnage\.docker\...`; once the command was rerun with approval, the image build and service restart succeeded.

## Scoped change: repair verification gates

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the gate-repair scope before implementation
  - Acceptance criteria:
    - `PLAN.md` captures the gate-repair scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered repair tasks before code/test fixture edits begin.
    - The scope explicitly excludes deployment contract changes and live stack startup.

- [x] Task 2 - Stabilize transcoder test fixtures and health recovery
  - Acceptance criteria:
    - FFmpeg test setup cannot accidentally resolve to a host `ffmpeg.exe` on Windows.
    - Health recovery tests verify that successful recovery clears degraded `ffmpeg` and publishing component state.
    - Live symlink assertions remain covered where the host supports them and are explicitly skipped where Windows permissions prevent directory symlink creation.

- [x] Task 3 - Render OME test configs outside the tracked contract file
  - Acceptance criteria:
    - `bitriver ome render --output <path>` writes to a caller-provided output path while preserving the default generated-file behavior.
    - OME render tests in `internal/ingest` and `scripts` use temp output files instead of touching `deploy/ome/Server.generated.xml`.

- [x] Task 4 - Replace auth session sleeps with deterministic time control
  - Acceptance criteria:
    - `internal/auth` session expiry tests use a package-private test clock option instead of short wall-clock sleeps.
    - Runtime session behavior and exported APIs remain unchanged.

- [x] Task 5 - Refresh the viewer snapshot baseline
  - Acceptance criteria:
    - `channelDisplayPrimitives` snapshots match the current `Featured live` copy.
    - The focused viewer test suite no longer fails on a stale snapshot.

- [x] Task 6 - Run the final gate and record results
  - Acceptance criteria:
    - The planned targeted Go, viewer, and Compose checks are recorded.
    - `./scripts/verify.sh` is run and any remaining blocker is documented clearly.

### Execution log (repair verification gates)
- Task 1 complete: recorded the scoped gate-repair plan and queued the ordered implementation tasks before touching code, tests, or snapshots. The check confirmed this pass excludes deployment contract edits and live stack startup.
- Task 1 checks:
  - `git status --short --untracked-files=all`
  - `Get-Content -Tail 50 PLAN.md`
  - `Get-Content -Tail 90 TASKS.md`
  - `git diff -- PLAN.md TASKS.md`
- Task 2 complete: replaced the FFmpeg test fixture lookup with a platform-specific shim that invokes a Go test-helper FFmpeg fake, so Windows no longer falls through to a host `ffmpeg.exe`. Tightened health recovery assertions so recovered `ffmpeg`/publishing components must be `ok` after fake jobs finish, and kept live symlink mirror checks conditional on host directory-symlink support while preserving upload publishing recovery coverage across platforms.
- Task 2 checks:
  - `gofmt -w cmd/transcoder/main_test.go`
  - `go test ./cmd/transcoder -count=1 -timeout=120s`
- Task 3 complete: added the backward-compatible `bitriver ome render --output <path>` flag while preserving the default `deploy/ome/Server.generated.xml` output path. Updated the OME render helpers in `internal/ingest` and `scripts` to render into temp files, and folded in the narrow quickstart Bash harness fix so `go test ./scripts` no longer fails on Windows by selecting a usable Git Bash path or skipping only when no shell works.
- Task 3 checks:
  - `gofmt -w cmd/bitriver/ome_render.go internal/ingest/ome_config_test.go scripts/quickstart_test.go`
  - `go test ./internal/ingest ./scripts -count=1 -timeout=120s`
  - `go test ./cmd/bitriver -count=1 -timeout=120s`
- Task 4 complete: added an internal clock hook to `SessionManager` and a package-private test clock option, then replaced the short expiry/idle/absolute-TTL sleeps in `internal/auth/session_test.go` with deterministic clock advancement. Exported session APIs are unchanged.
- Task 4 checks:
  - `gofmt -w internal/auth/session.go internal/auth/session_test.go`
  - `go test ./internal/auth -count=25 -run TestValidateHonorsAbsoluteTTL`
  - `go test ./internal/auth -count=1 -timeout=120s`
- Task 5 complete: updated the stale `channelDisplayPrimitives` snapshot so the featured card eyebrow matches the current `Featured live` copy.
- Task 5 checks:
  - `npm.cmd --prefix web/viewer run test -- __tests__/channelDisplayPrimitives.test.tsx --silent`
  - Note: Jest emitted the existing `.next/standalone/package.json` haste-map naming warning, but the focused suite and all snapshots passed.
- Task 6 complete: ran the planned targeted checks and the full verification gate. The first full-gate passes exposed a few additional Windows/full-suite timing blockers, which are now repaired: live transcoder metadata waits now read completed metadata instead of racing file writers, the `pollUntil` success unit test has enough scheduler headroom, chat tests start their worker subscriptions before publishing and avoid too-short moderation windows, the upload timeout test has a wider observation window without changing the processor timeout under test, and the contract invariant script now passes the same explicit env file to Compose that the rest of the verification path uses. No Compose, root `.env`, API payload, schema, or generated OME contract edits were made.
- Task 6 checks:
  - `go test ./cmd/transcoder -count=1 -timeout=120s`
  - `go test ./internal/auth -count=25 -run TestValidateHonorsAbsoluteTTL`
  - `go test ./internal/ingest ./scripts -count=1 -timeout=120s`
  - `npm.cmd --prefix web/viewer run test -- --silent`
  - `npm.cmd --prefix web/viewer run lint`
  - `docker compose --env-file .env -f deploy/docker-compose.yml config`
  - `npm.cmd --prefix web/viewer run build`
  - `go test ./cmd/transcoder -count=10 -run TestUploadPublishesHTTPPlayback -timeout=120s`
  - `go test ./cmd/bitriver -count=1 -run TestPollUntilSuccess -timeout=120s`
  - `go test ./cmd/transcoder -count=1 -run TestJobProducesSegmentsAndCanBeStopped -timeout=120s`
  - `go test ./internal/api -count=1 -run TestChatReportsAPI -timeout=120s`
  - `go test ./internal/chat -count=1 -run TestGatewayModerationFlow -timeout=120s`
  - `go test ./internal/service/uploads -count=1 -timeout=120s`
  - `./scripts/check-contract-invariants.sh`
  - `./scripts/verify.sh` (passed; run through Git Bash with workspace-local `GOCACHE` on Windows)
- Task 6 notes:
  - The full viewer/Jest gate still emits existing non-failing React `act(...)` warnings in directory search coverage.
  - `git diff --check` exits cleanly; Git only reports local line-ending warnings.
  - Git continues to warn that `C:\Users\RhythmicCarnage\.config\git\ignore` is not readable, but tracked status and diffs are still available.

## Scoped change: improve channel chat and signup onboarding

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Audit the current channel, chat, and signup flows
  - Acceptance criteria:
    - `PLAN.md` records this pass's scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists the ordered work before implementation begins.
    - The audit confirms which existing backend/channel behavior should remain unchanged.

- [x] Task 2 - Add realtime viewer chat over the existing WebSocket gateway
  - Acceptance criteria:
    - Signed-in channel viewers connect to `/api/chat/ws`, join the channel room, and render inbound message events without waiting for polling.
    - REST chat history/polling and REST send remain as fallback when WebSocket is unavailable or not yet open.
    - Duplicate message IDs are not rendered twice when history, WebSocket broadcasts, and acknowledgements overlap.

- [x] Task 3 - Polish creator-intent signup and first-channel onboarding copy
  - Acceptance criteria:
    - Auth dialog signup copy reflects creator onboarding when the redirect target is a creator route.
    - Generic viewer signup copy remains viewer-oriented.
    - Creator getting-started copy does not contain mojibake or broken apostrophe rendering.

- [x] Task 4 - Run focused validation and record results
  - Acceptance criteria:
    - Focused viewer tests for chat, channel page, auth dialog, and creator onboarding pass.
    - Focused backend channel/chat/auth tests pass.
    - Viewer lint/build results are recorded.

### Execution log (improve channel chat and signup onboarding)
- Task 1 complete: read the existing API, chat gateway, viewer channel page, chat panel, auth dialog, and creator onboarding code. Backend self-signup first-channel creation is already implemented and covered: a self-signup viewer can create their own first channel, gets a stream key, and is promoted to creator, while cross-owner channel creation is rejected. The clearest functionality gap is in the viewer chat panel, which still loads and sends through REST polling even though the backend exposes an authenticated `/api/chat/ws` live gateway. The signup flow is functional, but creator-intent signup still reads as a generic viewer account path.
- Task 1 checks:
  - `git status --short --untracked-files=all`
  - `Get-ChildItem -Recurse -Filter AGENTS.md`
  - `rg -n "signup|register|Create account|allowSelfSignup|ChatGateway|chat/reports|chat|channel|Channel" internal cmd web/viewer -S`
  - `go test ./internal/api ./internal/chat ./internal/auth -count=1 -timeout=120s`
  - `npm.cmd --prefix web/viewer run test -- __tests__/chatPanel.test.tsx __tests__/channelPage.test.tsx __tests__/creatorGettingStartedPage.test.tsx __tests__/creatorPage.test.tsx __tests__/useAuth.test.tsx --silent`
- Task 2 complete: added a viewer WebSocket URL helper and upgraded `ChatPanel` to open the authenticated `/api/chat/ws` gateway for signed-in viewers, send a `join` command, render inbound message events immediately, and submit chat messages over the socket while it is connected. REST history, polling, and REST send remain in place as fallback, and message IDs are de-duplicated so initial history, broadcasts, and acknowledgements do not double-render the same line.
- Task 2 checks:
  - `npm.cmd --prefix web/viewer run test -- __tests__/chatPanel.test.tsx --silent`
- Task 3 complete: adjusted the auth dialog so signup copy reflects creator onboarding when the auth redirect target is a creator route, while generic viewer signup remains unchanged. Cleaned the first-channel checklist copy to avoid non-ASCII apostrophe rendering in the creator onboarding path.
- Task 3 checks:
  - `npm.cmd --prefix web/viewer run test -- __tests__/authDialog.test.tsx __tests__/creatorGettingStartedPage.test.tsx --silent`
- Task 4 complete: ran the focused frontend and backend validation for the touched channel, chat, signup, and creator onboarding areas. Viewer lint and production build pass; the build still reports the existing Next.js client-rendering deopt warnings for several routes.
- Task 4 checks:
  - `npm.cmd --prefix web/viewer run test -- __tests__/chatPanel.test.tsx __tests__/channelPage.test.tsx __tests__/authDialog.test.tsx __tests__/creatorGettingStartedPage.test.tsx --silent`
  - `go test ./internal/api ./internal/chat ./internal/auth -count=1 -timeout=120s`
  - `npm.cmd --prefix web/viewer run lint`
  - `npm.cmd --prefix web/viewer run build`
- Task 4 notes:
  - The final focused Jest run emitted the existing `.next/standalone/package.json` haste-map naming collision warning after `next build` created `.next`; the suites passed.

## Product Readiness Closure - VOD Publish and Chat Reports

1. [x] Refresh product acceptance docs.
   - Acceptance: `SPEC.md` names the self-hosted live streaming product criteria and `docs/testing.md` includes a manual acceptance checklist for real broadcast readiness.
   - Checks: documentation review; no runtime contract changes.

2. [x] Add creator VOD publish action.
   - Acceptance: Ready upload cards with a `recordingId` expose a publish action that calls `POST /api/recordings/{id}/publish`, shows success/failure feedback, and refreshes uploads.
   - Checks: `npm.cmd --prefix web/viewer run test -- --silent UploadManager viewer-api` passed (16 tests).

3. [x] Add viewer chat report flow.
   - Acceptance: Signed-in viewers can report another user's chat message with a reason; the request includes `targetId` and `messageId`, and the UI confirms submission or shows errors.
   - Checks: `npm.cmd --prefix web/viewer run test -- --silent ChatPanel viewer-api` passed (20 tests).

4. [x] Run viewer gates and record results.
   - Acceptance: focused tests, lint, and build complete or any blocker is documented here.
   - Checks: `npm.cmd --prefix web/viewer run test -- --silent UploadManager ChatPanel viewer-api` passed (33 tests); `npm.cmd --prefix web/viewer run test -- --silent` passed (174 tests); `npm.cmd --prefix web/viewer run lint` passed; `npm.cmd --prefix web/viewer run build` passed with existing Next.js client-render deopt and Browserslist warnings; `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s` passed with local `.gocache`; `docker compose --env-file .env -f deploy/docker-compose.yml config --quiet` passed with the local Docker config permission warning.
   - Full verify: `./scripts/verify.sh` was attempted through Git Bash and timed out after 304 seconds before returning phase output.

## Product Readiness Closure - Schedule and Final Gates

1. [x] Add channel schedule storage and API contract.
   - Acceptance: Channel schedule entries are typed, normalized, persisted in JSON and Postgres storage, included in public/managed channel responses, and updateable through the existing channel PATCH endpoint.
   - Checks: focused storage/API tests.
   - Result: added typed schedule entries, additive JSONB migrations, snapshot import support, PATCH parsing, playback response coverage, and isolated in-memory return copies.
   - Check: `GOCACHE=.gocache go test ./internal/storage ./internal/api -count=1 -timeout=120s` passed.

2. [x] Add creator schedule editing.
   - Acceptance: The Go Live dashboard lets a channel owner edit at least one upcoming scheduled stream with title, start time, duration, and description, then saves it through the channel API.
   - Checks: focused creator live viewer tests.
   - Result: added an upcoming-stream form to the Go Live channel step with save/clear actions and API payload coverage.
   - Check: `npm.cmd --prefix web/viewer run test -- __tests__/creatorLivePage.test.tsx --silent` passed.

3. [x] Render the public schedule.
   - Acceptance: The public channel Schedule tab displays upcoming stream entries when present and a clear empty state when absent.
   - Checks: focused channel page viewer tests.
   - Result: replaced the placeholder-only Schedule tab with sorted schedule cards, formatted start times, durations, and descriptions.
   - Check: `npm.cmd --prefix web/viewer run test -- __tests__/channelPage.test.tsx --silent` passed.

4. [x] Run final gates and record results.
   - Acceptance: Viewer checks, Go checks, Compose config, and full verify either pass or the precise remaining blocker is recorded.
   - Checks: test plan from `PLAN.md`.
   - Result: all schedule/product-readiness gates passed. The first full Go run exposed a nondeterministic metrics concurrency test; the test now validates extra concurrent stops after starts are registered, and the full Go suite passes.
   - Checks:
     - `GOCACHE=.gocache GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage ./internal/api -count=1 -timeout=120s` passed.
     - `npm.cmd --prefix web/viewer run test -- __tests__/creatorLivePage.test.tsx __tests__/channelPage.test.tsx --silent` passed.
     - `npm.cmd --prefix web/viewer run test -- --silent` passed (175 tests).
     - `GOCACHE=.gocache GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s` passed after the metrics test repair.
     - `npm.cmd --prefix web/viewer run lint` passed.
     - `npm.cmd --prefix web/viewer run build` passed with existing Browserslist and Next.js client-render deopt warnings.
     - `docker compose --env-file .env -f deploy/docker-compose.yml config --quiet` passed with the local Docker config permission warning.
     - `./scripts/verify.sh` passed through Git Bash with LibreOffice Python on `PATH` and Docker access approved for quickstart smoke.
     - `docker compose --env-file .env -f deploy/docker-compose.yml ps` showed no running services after verify cleanup.

## Viewer Auth UI Refresh

1. [x] Refresh route data after auth state changes.
   - Acceptance: successful same-page sign-in/sign-up/MFA completion updates auth context and triggers a Next.js route refresh without forcing a full navigation.
   - Acceptance: sign-out also refreshes route data after `/api/viewer/me` settles.
   - Checks: focused `useAuth` tests.
   - Result: `AuthProvider` now calls `router.refresh()` after same-route auth success and after sign-out refresh settles; existing cross-route redirects remain full navigations.
   - Check: `npm.cmd --prefix web/viewer run test -- __tests__/useAuth.test.tsx --silent` passed.

2. [x] Validate affected viewer surfaces.
   - Acceptance: Navbar/auth, directory discovery, and following-state tests continue to pass.
   - Checks: focused viewer tests plus lint/build.
   - Result: focused auth-adjacent surfaces still pass; production viewer build compiles with the existing Next.js client-render deopt warnings.
   - Checks:
     - `npm.cmd --prefix web/viewer run test -- __tests__/navbar.test.tsx __tests__/directoryPage.test.tsx __tests__/followingStatePresentation.test.tsx --silent` passed.
     - `npm.cmd --prefix web/viewer run lint` passed.
     - `npm.cmd --prefix web/viewer run build` passed with existing deopt warnings.

3. [x] Rebuild the running Docker viewer.
   - Acceptance: Docker stack stays up and the rebuilt viewer/API routes serve successfully.
   - Checks: Compose build/up, `ps`, and basic HTTP checks.
   - Result: rebuilt and recreated `bitriver-viewer`; Compose stack remained up with core services healthy.
   - Checks:
     - `docker compose --env-file .env -f deploy/docker-compose.yml up --build -d viewer` passed.
     - `docker compose --env-file .env -f deploy/docker-compose.yml ps` passed; core services reported healthy/running.
     - `Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer -TimeoutSec 15` returned 200.
     - `Invoke-WebRequest -UseBasicParsing http://localhost:8080/readyz -TimeoutSec 15` returned 200.

## OME Ingest Health Auth Repair

1. [x] Align ingest OME auth with the AccessToken contract.
   - Acceptance: ingest config loads `BITRIVER_OME_API_TOKEN` and requires it for complete OME control configuration.
   - Acceptance: OME health and application control requests send `Authorization: Basic <base64(raw AccessToken)>` when the token is configured.
   - Acceptance: authenticated OME `404` responses from the shared health path count as reachable, while `401/403` still fail.
   - Acceptance: Basic auth remains a fallback for nonstandard legacy test/custom deployments with no token.
   - Checks: focused config/ingest tests.
   - Result: ingest config now resolves the OME access token from the rendered-token precedence; OME health/control calls use OME's raw-token Basic credential form after runtime probing showed `AccessToken:` headers still return 401.
   - Check: `go test ./internal/config ./internal/ingest -count=1 -timeout=120s` passed with a workspace-local `GOCACHE`.

2. [x] Update tests and local documentation comments.
   - Acceptance: tests catch the deployed 401 regression by asserting OME health uses the raw-token Basic credential and accepts authenticated `404` as reachable.
   - Acceptance: ingest package comments no longer claim OME uses Basic auth as the primary path.
   - Checks: `go test ./internal/config ./internal/ingest ./internal/app -count=1 -timeout=120s`.
   - Result: added health and adapter assertions for OME raw-token Basic auth, preserved legacy Basic fallback coverage, added runtime-config coverage so the API carries the token into `ingest.Config`, updated ingest package comments, and corrected the OME auth note in `docs/advanced-deployments.md`.
   - Checks:
     - `go test ./internal/config ./internal/ingest ./internal/app -count=1 -timeout=120s` passed with a workspace-local `GOCACHE`.
     - `go test ./internal/storage -run Ingest -count=1 -timeout=120s` passed with a workspace-local `GOCACHE`.

3. [x] Rebuild the running API service and verify deployed health.
   - Acceptance: Compose stack stays up and BitRiver health endpoints serve successfully after the API rebuild.
   - Acceptance: OME is no longer reported down in BitRiver ingest health after the rebuilt API starts.
   - Checks: Compose build/up, `ps`, `/healthz`, and `/readyz`.
   - Result: rebuilt `bitriver-live`; BitRiver `/healthz` now reports `status: ok` with `ovenmediaengine` status `ok`. Added `.gocache/` to `.dockerignore` after the local test cache inflated the Docker build context.
   - Checks:
     - `docker compose --env-file .env -f deploy/docker-compose.yml up --build -d bitriver-live` passed.
     - `docker compose --env-file .env -f deploy/docker-compose.yml ps` passed; core services reported healthy/running.
     - `Invoke-WebRequest -UseBasicParsing http://localhost:8080/healthz -TimeoutSec 15` returned 200 and `status: ok`.
     - `Invoke-WebRequest -UseBasicParsing http://localhost:8080/readyz -TimeoutSec 15` returned 200.
     - `Invoke-WebRequest -UseBasicParsing http://localhost:8080/viewer -TimeoutSec 15` returned 200.

## Main Merge Conflict Resolution

1. [x] Merge the current base into this branch using branch-preferred conflict policy.
   - Acceptance: `origin/main` is merged into `fix-viewer-discovery-polish` with conflicting hunks resolved in favor of this branch.
   - Checks: `git merge --no-commit --no-ff -X ours origin/main`.
   - Result: merge completed cleanly with no remaining unmerged paths; Git reported "All conflicts fixed but you are still merging."

2. [x] Run conflict hygiene checks.
   - Acceptance: no unmerged paths remain and no conflict markers remain in tracked source/docs.
   - Checks: unmerged-path scan, conflict-marker scan, staged diff whitespace check.
   - Result: no unmerged paths; no conflict markers found; staged diff whitespace check passed.
   - Checks:
     - `git diff --name-only --diff-filter=U` returned no paths.
     - `rg -n "^(<<<<<<<|=======|>>>>>>>)" --glob "!web/viewer/node_modules/**" .` returned no matches.
     - `git diff --cached --check` passed.

3. [x] Conclude the merge.
   - Acceptance: merge resolution is committed on `fix-viewer-discovery-polish`.
   - Checks: `git status` after commit.
   - Result: merge resolution committed on `fix-viewer-discovery-polish`.
   - Check: `git status` after commit.

## GitHub Issue #1219 Watch Page Priority

1. [x] Record the scoped #1219 plan before editing.
   - Acceptance: `PLAN.md` captures scope, assumptions, risks, and validation for #1219.
   - Acceptance: `TASKS.md` lists ordered tasks before channel page/CSS edits.
   - Result: read-only analysis found `main` already has player recovery, schedule tabs, and a replay card; the remaining gap is duplicate offline actions, missing mobile watch affordances, and explicit refresh-state regression coverage.
   - Checks:
     - `Get-Content -LiteralPath 'web\viewer\app\channels\[id]\page.tsx'`
     - `Get-Content web\viewer\components\ChatPanel.tsx`
     - `Get-Content web\viewer\components\ChannelHero.tsx`
     - `Get-Content web\viewer\components\VodGallery.tsx`
     - `Get-Content web\viewer\__tests__\channelPage.test.tsx`
     - `Get-Content web\viewer\tests\channel.spec.ts`
     - `rg -n "channel-page|channel-player|channel-replay|channel-owner|channel-tabs|channel-hero|chat-panel" web/viewer/styles/globals.css`

2. [x] Make live/offline watch hierarchy explicit.
   - Acceptance: live pages keep player/chat first, then actions/metadata.
   - Acceptance: offline pages expose exactly one primary action area, driven by VOD/schedule availability.
   - Acceptance: creator-owner management stays visually separated from the public watch flow.

3. [x] Add mobile watch-page affordances.
   - Acceptance: mobile users get compact links to chat, details, and videos/schedule near the player.
   - Acceptance: desktop layout remains two-column video/chat without extra mobile-only controls.
   - Acceptance: CSS avoids layout jumps during background playback refresh.

4. [x] Add/update focused channel coverage.
   - Acceptance: Jest covers offline-with-VOD, offline-without-VOD, and refresh preserving active tabs.
   - Acceptance: Playwright covers mobile channel hierarchy and the existing live/chat flow still passes.

5. [x] Run validation and record results.
   - Acceptance: focused Jest, focused Playwright, lint, build, and repo viewer gate are run or host blockers are recorded.

### Execution log (GitHub issue #1219 watch page priority)
- Task 1 complete: recorded the scoped #1219 plan before editing and confirmed the current baseline has player recovery, replay cards, public schedules, and channel tab URL state already present on `main`.
- Task 2 complete: made the channel page decide when player recovery actions are primary, removed player retry/browse actions from offline player states, and upgraded the offline replay card into the single primary offline action area for VOD, replay-error, schedule, and no-content states. Creator controls now read as creator-only tooling.
- Task 2 checks:
  - `npm.cmd --prefix web/viewer run test -- __tests__/channelPage.test.tsx __tests__/player.test.tsx` passed. Jest emitted a non-failing haste warning because the existing `.next/standalone/package.json` from a prior build is still present under `web/viewer/.next`.
- Task 3 complete: added a mobile-only watch navigation under the player with Chat, Details, Videos, and Schedule affordances, plus scroll targets for chat/details and CSS that keeps the desktop two-column watch layout unchanged.
- Task 3 checks:
  - `npm.cmd --prefix web/viewer run test -- __tests__/channelPage.test.tsx __tests__/player.test.tsx` passed.
- Task 4 complete: added Jest coverage for no-replay offline recovery, scheduled offline recovery, and background playback refresh preserving the active tab. Added Playwright mobile hierarchy coverage for player, chat, details, and tab switching.
- Task 4 validation also exposed stale current-main blockers outside the watch page: the viewer build had stale directory/category state references and several compile blockers in browse, creator setup, category rail, and viewer shell components; the channel Playwright auth redirect assertions expected the fallback modal despite a configured `loginUrl`. These were repaired narrowly so #1219 coverage could run against the current intended auth behavior.
- Task 4 checks:
  - `npm.cmd --prefix web/viewer run test -- __tests__/channelPage.test.tsx __tests__/player.test.tsx` passed.
  - `npm.cmd --prefix web/viewer run build` passed.
  - Standalone-server Playwright run for `tests/channel-chat-playback.spec.ts tests/channel.spec.ts` passed: 12 tests passed.
- Task 5 complete: ran focused and broad viewer validation. The first full Jest run exposed stale viewer gate drift on current main; repaired the broken auth test block, restored featured carousel controls, aligned stale creator/following/auth assertions with current UI behavior, imported `searchDirectory` in viewer API tests, and made the playback retry Playwright fixture deterministic.
- Task 5 checks:
  - `npm.cmd --prefix web/viewer run test -- __tests__/creatorGettingStartedPage.test.tsx __tests__/useAuth.test.tsx __tests__/channelDisplayPrimitives.test.tsx __tests__/followingStatePresentation.test.tsx __tests__/followingSidebar.test.tsx __tests__/viewer-api.test.ts --silent` passed.
  - `npm.cmd --prefix web/viewer run test -- --silent` passed: 24 suites, 187 tests.
  - `npm.cmd --prefix web/viewer run lint` passed.
  - `npm.cmd --prefix web/viewer run build` passed.
  - Standalone-server Playwright run for `tests/channel-chat-playback.spec.ts tests/channel.spec.ts` passed: 12 tests passed.
  - `docker compose --env-file .env -f deploy/docker-compose.yml config` passed; Docker emitted non-fatal warnings about denied access to the user Docker config file.
  - `& "C:\Program Files\Git\bin\bash.exe" -lc "cd /c/Users/RhythmicCarnage/Desktop/BitRiver-Live && ./scripts/verify.sh --viewer"` started and passed the go.sum and CI-contract checks, then stopped at the host prerequisite check because this machine has `C:\Windows\py.exe` but no installed Python runtime (`py -0p` reports no installed Pythons).
