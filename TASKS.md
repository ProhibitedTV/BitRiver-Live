# TASKS

## Scoped change: Windows Docker Desktop proof and evidence-led README

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Inventory current claims and reproducible proof paths
  - Acceptance criteria:
    - README art, release/package links, Windows entrypoints, runtime routes, demo-data options, and screenshot locations are mapped before implementation.
    - The GitHub remote release state and local Docker Desktop client/server state are checked without exposing credentials.
    - Existing unrelated work and the Ubuntu installer PR remain explicit boundaries.
  - Check:
    - Confirmed the README uses the 2.4 MB concept-art `bitriver-live-banner-text.png` and claims packaged launcher availability despite zero published GitHub Releases.
    - Confirmed native `scripts/quickstart.ps1`, the canonical CLI, `/viewer`, `/admin`, `/healthz`, `/readyz`, self-signup, first-channel creation, and authenticated admin seeding provide supported proof/demo paths.
    - Confirmed Docker Desktop 4.82.0 is running on Windows with client/engine 29.6.1, Compose 5.3.0, the `desktop-linux` context, and a Linux/amd64 server.
    - Updated `SPEC.md` and `PLAN.md` before runtime/code changes; kept PR #1325, root `.env`, and the user's untracked deployment helpers out of scope.

- [x] Task 2 - Make the native Windows Docker Desktop proof reproducible
  - Acceptance criteria:
    - A documented PowerShell command validates Docker Desktop Linux-container mode, Compose, canonical config, source-build startup, and bounded health checks.
    - Failures distinguish Docker-not-running/permission problems from repository defects.
    - Focused parser and behavior tests pass.
  - Check:
    - Added `scripts/verify-windows-docker.ps1`; its lightweight path proved Docker Desktop 4.82.0, Linux/amd64 engine 29.6.1, Compose 5.3.0, `desktop-linux`, and canonical config rendering.
    - The `-Start` path uses an isolated temporary evaluation env, delegates startup to the canonical CLI, and passed migrations, all critical service health checks, admin bootstrap, and `200` responses for `/healthz`, `/readyz`, `/viewer`, and `/admin` on this Windows host.
    - Real execution exposed and fixed two PowerShell wrapper defects: case-insensitive `Path`/`PATH` handling hid Docker from the CLI, and offline Go build variables leaked into Docker build args. The wrapper now compiles the CLI under isolated Go settings, restores the operator environment, and then runs it.
    - Aligned quickstart/env/Compose validation with the documented inline `BITRIVER_LIVE_MODE=development` override while keeping the saved env required to remain production mode.
    - PowerShell parsing, validate-only execution, focused `cmd/bitriver` and `scripts` Go tests under Go 1.26.5, workflow static contracts, and `git diff --check` passed.

- [x] Task 3 - Prove the running product and prepare safe demo state
  - Acceptance criteria:
    - Canonical Compose boots from this Windows source checkout and core HTTP endpoints pass.
    - Any screenshot data is deterministic, non-sensitive, and created through supported interfaces.
    - The canonical SRS callback routes bypass browser-session middleware but still require the independent shared hook token, and a synthetic RTMP publish no longer fails at `on_connect`.
    - OME evidence is described at the level actually exercised, including LL-HLS CORS, the same-origin `/live/` route, and browser playback rather than process health alone.
  - Check:
    - The canonical Windows proof completed migrations, started the source-built Compose stack, reported every critical service healthy, bootstrapped an administrator, and returned `200` from `/healthz`, `/readyz`, `/viewer`, and `/admin`.
    - Created a local-only channel through supported authenticated interfaces, published a synthetic audio/video RTMP source through SRS, and fixed the callback, dynamic-forward, static OME application, transcoder job, and public playback contracts exposed by that run.
    - Verified all three transcoder manifests (`1080p`, `720p`, and `480p`) returned `200`; verified OME LL-HLS returned `Access-Control-Allow-Origin: *`; and decoded five seconds of video and audio from the same-origin `/live/<channel>/llhls.m3u8` route with FFmpeg.
    - Added focused callback-auth, orchestration, OME application, transcoder recovery, proxy, CSP, and player-selection tests. The controlled browser backend blocked `.m3u8` requests with `ERR_BLOCKED_BY_CLIENT` before they reached the application, so automated visual playback is not claimed; the direct decode and hls.js path are the bounded evidence.

- [x] Task 4 - Capture real product screenshots and remove concept art
  - Acceptance criteria:
    - A small screenshot set is captured from the live application at a stable desktop viewport and visually inspected; responsive behavior is inspected separately.
    - Images contain no secrets, broken states, browser chrome, or private host details.
    - The old banner is removed from the README and repository if no longer referenced.
  - Check:
    - Captured the running viewer home and live directory without browser chrome at a stable desktop viewport, then visually inspected both files and the responsive route state.
    - Confirmed the images contain only intentional local demo content and no credentials, private hosts, error states, or operator-only details.
    - Added the two product screenshots under `docs/assets/screenshots/`, replaced README concept art with those images, and removed the unreferenced 2.4 MB banner from the repository.

- [x] Task 5 - Rewrite README and Windows quickstart guidance
  - Acceptance criteria:
    - README leads with the real product, shows accurate operator/viewer workflows, and gives a copyable native PowerShell path.
    - Current source-checkout availability is clearly separated from planned tagged installers/packages.
    - Ubuntu/XOA/Nginx Proxy Manager and OME production gates remain accurate and linked to deeper docs.
    - Production validation treats OME's server IP as a container-local listener and does not require the VM's public address inside the container.
  - Check:
    - Replaced the README with an evidence-led product/operator narrative using the two real application screenshots, an accurate media-flow diagram, source-checkout quickstarts, native Windows proof, first-stream workflow, support boundary, and operational acceptance gate.
    - Removed dead package/installer download instructions from the quickstart and replaced the Ubuntu guide with the current Ubuntu 24.04 source-build path for XOA/NPM home hosting; added explicit same-origin `/live/`, `/hls/`, websocket, firewall, RTMP/TCP, backup, reboot, and recovery guidance.
    - Corrected contract, lifecycle, failure-model, and advanced-deployment docs to reflect authenticated SRS callbacks/forwarding, static OME `default/live` validation, LL-HLS CORS, and public/private URL boundaries.
    - Fixed the production OME listener validator discovered during documentation: `BITRIVER_OME_BIND`/`BITRIVER_OME_IP` may remain wildcard inside Docker, while public LL-HLS, RTMP, transcoder, and viewer URLs remain strict. Focused `cmd/bitriver` and `internal/ingest` tests passed.
    - Generated contract index, installer-language guard, README image existence checks, and `git diff --check` passed.

- [-] Task 6 - Run release-quality validation and publish separately
  - Acceptance criteria:
    - Focused Windows/runtime/viewer/docs checks and `./scripts/verify.sh --viewer` pass or exact blockers are recorded.
    - Diff and staged-file review exclude `.env`, credentials, runtime output, and unrelated untracked helpers.
    - Work is committed and proposed separately from PR #1325 with remote CI status reported accurately.
  - Check:
    - The first full gate exposed a Windows junction-fallback defect: the transcoder depended on `cmd.exe` being discoverable through `PATH`, while the release gate intentionally narrows `PATH`. It now resolves the interpreter through `ComSpec`/`SystemRoot`; the interpreter-selection and complete job lifecycle/health regressions pass under that narrowed environment.
    - Final `./scripts/verify.sh --viewer` passed all Go packages, architecture/dependency/contract checks, the Postgres migration lifecycle, Compose rendering, clean quickstart smoke, viewer lint, and 25 Jest suites / 215 tests / 4 snapshots.
    - Final `.\scripts\verify-windows-docker.ps1 -Start` passed on Docker Desktop 4.82.0 with Linux/amd64 engine 29.6.1 and Compose 5.3.0; migrations, critical service health, admin bootstrap, and `200` responses from `/healthz`, `/readyz`, `/viewer`, and `/admin` all passed. Its printed cleanup command was executed successfully.
    - The staged audit included 64 intended files with no root `.env`, runtime data, unrelated deployment helpers, or detected secret patterns; `git diff --cached --check` passed and the generated OME config retained its placeholder token.
    - Committed and pushed on `feat/windows-docker-readme`; opened draft PR [#1326](https://github.com/ProhibitedTV/BitRiver-Live/pull/1326) against `main`, separate from Ubuntu installer PR #1325.
    - Remote CI exposed a missing-fixture failure before the vulnerability scan ran: the inline `.ci.env` did not include the two new required public media URLs, so the dependent Ubuntu gate failed and its remaining jobs skipped. Workflow-fixture repair and final remote revalidation are in progress.
