# PLAN

## Current scope - branch hygiene and release CI consolidation (2026-07-26)

- Reduce the remote branch inventory without risking active or unincorporated
  work. Delete only branches whose tips are ancestors of `origin/main`; retain
  every non-ancestor branch until its pull-request history and remaining diff
  are classified separately.
- Keep `ci.yml` as the only automatic pull-request/main orchestrator. Make the
  targeted manual workflows reusable, then have CI call those definitions
  instead of maintaining inline copies for viewer, image scan, shell, docs,
  monitoring, workflow-policy, wizard, and quickstart-entrypoint checks.
- Keep intentionally distinct heavyweight paths separate: the unified Ubuntu
  `test-all` gate owns changed-path integration/Compose work, the production
  golden-path workflow owns full media acceptance, and the standalone Go gate
  owns manual full-matrix/govulncheck coverage.
- Make the tag workflow call the reusable Postgres workflow rather than owning a
  second service definition. Keep host Go verification offline, but restore the
  public Go proxy/checksum database only for the release verification step so
  clean Compose image builds can resolve `go.production.mod`.
- Centralize runtime setup in the local setup actions without a hidden second
  checkout. Every workflow remains responsible for one explicit, SHA-pinned
  checkout before invoking a setup action.
- Do not create `v1.2.3-rc.3` until focused workflow regressions, the complete
  local gate, the pull-request matrix, and a post-merge targeted workflow run
  are green. Failed `rc.1` and `rc.2` tags remain immutable.

### Audit evidence and assumptions

- The fetched remote contains exactly 1,000 branches: 943 non-default branch
  tips are ancestors of `origin/main`, while 57 are not. GitHub reports no open
  pull requests and no protected branches. `main` and every non-ancestor branch
  are excluded from the first cleanup pass.
- The repository registers 13 workflow files plus GitHub's Dependabot workflow.
  Only `ci.yml` automatically runs for pull requests/main; the other CI
  workflows are manual and/or reusable, and `release.yml` is tag-only.
- Proven drift exists today: `ci.yml` embeds Trivy 0.70.0 with bounded download
  retries while the standalone image workflow embeds 0.50.1 without them.
  Release also duplicates the reusable Postgres service, which already caused
  `rc.1` to diverge, and both setup composite actions perform a second checkout
  using a different action pin than their callers.
- The `rc.2` release failure is bounded to the `go-tests` verification step:
  job-level `GOPROXY=off` is inherited by Compose build arguments, while
  `verify.sh` already applies offline variables directly to host Go tests.
- User-owned untracked deployment notes/helpers/data and the private root
  `.env` remain outside this change and must not be staged, rewritten, or
  included in branch-cleanup evidence.

### Risks

- Squash merges do not make a historical head an ancestor of `main`. This is why
  the 57 non-ancestor branches are retained during the ancestry-safe cleanup
  even when their work may already be represented on `main`.
- Remote deletion of all 943 ancestry-merged branches is intentionally pending
  a separate explicit confirmation after the execution safety gate rejected the
  broad mutation. No branch was deleted; CI/release work can proceed
  independently from the preserved cleanup classification.
- Reusable-workflow calls change the displayed check hierarchy. There are no
  protected-branch required-check rules today, but the complete PR run must
  prove all path-gated jobs still execute or skip as intended before merge.
- A called workflow cannot elevate caller permissions. The image scan caller
  must explicitly retain `contents: read` and `packages: read`.
- Consolidation must not make CI repeat the same Docker lifecycle. The CI call
  to quickstart smoke will run entrypoint checks only because the unified Ubuntu
  gate already owns Compose smoke for relevant changes.
- A green source/CI candidate still does not prove clean Ubuntu/XOA install,
  Nginx Proxy Manager browser access, reboot recovery, or OME recovery. Those
  remain stable-promotion evidence after tagged artifacts exist.

### Test and publication plan

- Add workflow-contract tests requiring CI reusable calls, one checkout per
  workflow path, release reuse of Postgres, explicit migrations in the reusable
  service, and release verification proxy/checksum restoration.
- Update existing image/viewer/release tests to inspect the single source of
  truth instead of requiring duplicated commands in `ci.yml`.
- Parse every workflow/action YAML file, run `check-ci-contract.sh`,
  `check-go-workflow-config.sh`, focused Go script tests, shell syntax, and
  `git diff --check`.
- Run literal `./scripts/verify.sh --viewer` while preserving/restoring the
  operator's private root `.env`, then push a small PR and require the complete
  remote CI result.
- After merge, manually dispatch the reusable Postgres and other release-relevant
  targeted gates as needed. Only then tag `v1.2.3-rc.3`, monitor every release
  job, inspect checksums/packages, prove anonymous GHCR pulls, and run the
  Docker Desktop pull-only product gate before clean Ubuntu/XOA acceptance.

## Current scope - first public release-candidate publication gate (#1297, 2026-07-24)

- Make the tag-triggered release workflow runnable from this public repository without preloading deployment credentials. Generate strong job-local validation credentials, retain only sentinel-scanned status evidence, and keep real operator secrets entirely outside GitHub Actions.
- Publish first-party images to the repository owner's real GHCR namespace, exposed through one deployment-contract variable so official installs default to `ghcr.io/prohibitedtv` while forks and mirrors can override it.
- Validate release tags as SemVer, normalize package/MSI versions separately from the human tag, mark hyphenated tags as GitHub prereleases, and prevent prerelease tags from moving `latest`.
- After all five tagged images publish, run the canonical Compose stack in production/pull mode and execute the same 1080p media/API golden path. The GitHub Release job must depend on this scanner-approved pull-only evidence.
- Repair the Windows MSI staging/version seam so the release matrix cannot be blocked by paths that disagree with the canonical release asset manifest.
- Only after the workflow change is merged and its PR gates pass, create the first immutable `v1.2.3-rc.1` tag. Treat it as a public candidate for clean Ubuntu/XOA acceptance, not as the stable v1.2.3 announcement.

### Release-candidate design

- Add a deterministic release-env preparation helper that copies `deploy/.env.example`, replaces sample credentials with cryptographically random job-local values, applies the exact release tag/official namespace, resolves current third-party registry digests, and writes a separate temporary sentinel file. It must never print values.
- Extend the quickstart smoke through explicit `BITRIVER_SMOKE_*` controls. Existing local/CI callers keep build/development defaults; the release job supplies an external env file and selects pull/production without rewriting a checkout-owned `.env`.
- In pull mode, skip every Compose build, pull all rendered image references, enforce production dependency digests, render OME with the tagged helper image, run the existing service checks, then run `test-production-golden-path.sh --stack running`.
- Upload only `production-golden-path.json` after the existing evidence scan. Raw Compose logs, generated OME/SRS files, env files, cookies, stream keys, and registry credentials remain runner-local and are removed on every exit path.
- Derive `release_version` and `prerelease` once from the tag. Use the normalized numeric core for MSI, SemVer components for Linux packaging, the original tag for filenames/image tags, and the prerelease flag for GitHub Release metadata.
- Stage the Windows launcher from `deploy/install/release-assets.txt`, and make WiX source paths match the staged `share/bitriver-live` layout rather than maintaining a second incomplete asset list.

### Release-candidate risks

- `ghcr.io/bitriver-live` is not an owned GitHub account namespace, while the repository owner is `ProhibitedTV`; the current workflow cannot publish the references Compose names. Changing the official default is a deployment-contract change and must update Compose, env, CLI preflight, Helm/docs, and generated contract evidence together.
- GHCR packages may require a one-time public-visibility action after their first push. The workflow must prove anonymous manifest access before creating a GitHub Release; if visibility cannot be changed with the repository token, stop with the exact external action rather than publishing unusable assets.
- Tag workflows publish immutable external state. A failed `v1.2.3-rc.1` is never force-moved or overwritten; corrections use `rc.2`.
- The immutable `v1.2.3-rc.1` run reached release validation but stopped
  before artifact/image publication because its fresh Postgres service database
  was not migrated. `test-postgres.sh` intentionally requires
  `BITRIVER_TEST_POSTGRES_RUN_MIGRATIONS=1` for an externally supplied DSN;
  both the tagged and reusable Postgres workflows must opt in explicitly.
- The immutable `v1.2.3-rc.2` run crossed the Postgres gate, then stopped
  before builds because the release Go job's host-only `GOPROXY=off` policy
  leaked through Compose into clean Docker builds. Keep host Go tests offline,
  but give the release verification step the real upstream proxy/checksum
  settings already used by artifact builders.
- Multi-architecture image publication is slower than registry index propagation. Use bounded manifest retries before the pull-only gate, not unbounded sleeps.
- Production mode currently requires third-party digest pins but not first-party pins. Resolve and record first-party manifest digests in candidate evidence/release notes; do not claim digest-pinned clean-host proof from tag-only pulls.
- The existing Windows workflow passes `v...` directly to WiX and stages files under paths WiX does not read. Static checks are insufficient; the remote Windows MSI job remains required before candidate publication.
- The Jul 24 GitHub Advisory Database update for
  `GHSA-mh99-v99m-4gvg` marks every `brace-expansion` release through 5.0.7
  vulnerable to attacker-controlled memory exhaustion and names 5.0.8 as the
  patched release. Viewer CI installs older transitive majors through
  ESLint/Jest tooling, and ordinary `npm audit fix` cannot update those parent
  ranges without a breaking ESLint major. Use one explicit npm override to
  5.0.8 only if clean `npm ci`, lint, unit, browser, build, and audit all pass;
  do not accept `--force` or suppress the advisory.
- GitHub-hosted pull-only proof still is not a clean Ubuntu/XOA install, Nginx Proxy Manager/browser test, or host reboot. Those remain explicit #1297/#1304 promotion gates.

### Release-candidate test plan

- Unit-test tag parsing, prerelease/latest behavior, env replacement, secret uniqueness, sentinel separation, digest formatting, no-value output, and failure on malformed tags or unresolved images.
- Add workflow-contract tests requiring job-local credentials, the official/overridable GHCR namespace, the post-publish pull-only product job, scanner-approved artifact upload, stable-only `latest`, prerelease metadata, and release-job dependency ordering.
- Add a workflow-contract regression requiring every CI-owned fresh Postgres
  service DSN to opt into repository migrations, then run the real
  Postgres-tagged suite against a disposable service database before `rc.2`.
- Add a release-workflow regression requiring the verification step to restore
  production dependency network settings before Docker builds while
  `verify.sh` continues to force host Go tests offline.
- Add quickstart regression tests proving build/development remains the default and pull/production performs no build while enforcing the supplied external env/digest contract.
- Run shell syntax/ShellCheck, generated contract checks, focused Go/Python tests, Compose rendering in both build and pull shapes, release-bundle/package tests, and `git diff --check`.
- Run `./scripts/verify.sh --viewer` and the full PR matrix. After merge, tag the RC, monitor every release job, inspect/download the published assets/checksums, verify anonymous GHCR access and image digests, and run a Docker Desktop pull-only golden path before handing the candidate to the clean Ubuntu/XOA gate.
- For `GHSA-mh99-v99m-4gvg`, require the lock graph to contain only
  `brace-expansion@5.0.8`, `npm audit --audit-level=high` to report zero
  vulnerabilities, and the complete viewer lint/unit/Playwright/build sequence
  to pass after a clean install. Build the real viewer container too, because
  its dependency stage must copy the local compatibility hook before `npm ci`.
  Keep the override registry-backed so nested consumers receive ordinary
  package copies and `npm ls` reports a valid dependency graph.

## Current scope - production golden-path E2E (#1300, 2026-07-24)

- Replace the misleading ingest "E2E" boundary, which currently runs only a storage package test, with one reusable acceptance harness against the real canonical Compose stack: Postgres, Redis, SRS, controller, transcoder, OvenMediaEngine, API, and viewer.
- Generate deterministic 1080p video plus audio at runtime; publish it over the creator-facing RTMP path; require the channel to transition live and back offline; and prove both OME LL-HLS and transcoder HLS are decodable and advancing rather than checking only HTTP status.
- Exercise real self-signup/session cookies, first-channel creator bootstrap, chat send/history, an owner moderation action, multipart VOD upload/transcode/publication, viewer metadata, and health/readiness/status surfaces.
- Emit a versioned machine-readable stage report, media probe evidence, endpoint summaries, timing, and failure context without retaining credentials. Scan retained evidence against per-run sentinel values before accepting it.
- Make the live-stack tier reusable from source quickstart, release workflows, and an already-running clean-host installation. Keep the cheap storage integration test separate so local unit/integration commands do not accidentally claim production coverage.

### Golden-path design

- Implement the product exercise as a standard-library Python harness plus a small shell entrypoint. The harness talks only through public HTTP/RTMP surfaces and invokes host `ffmpeg`/`ffprobe`; it must not reach directly into Postgres or mutate containers to manufacture success.
- The shell entrypoint supports two modes: `--stack running` validates an already-running deployment, while `--stack quickstart` delegates lifecycle to the canonical quickstart smoke and runs the same product assertions before teardown.
- Test credentials and stream keys exist only in process memory or a temporary sentinel file outside the evidence directory. Reports store stable labels, IDs, status, durations, URLs with query/userinfo removed, and redacted command descriptions.
- A phase fails with a bounded, stage-specific error. The wrapper then captures Compose state and selected recent logs through the existing redaction/scanning boundary; success and failure both leave a report that names the first failed stage.
- Keep the first PR tier build-based for deterministic source validation. Tagged RC/stable promotion must call the same running-stack harness after pull-only immutable images are installed; build-mode success alone is not publication approval.

### Golden-path risks

- LL-HLS manifests can exist before media is decodable. Require a media probe and advancing segment/timestamp evidence, not a single successful manifest GET.
- RTMP publication is asynchronous across SRS callbacks, OME forwarding, and transcoder startup. Use explicit per-stage deadlines with last-observed state; do not use unbounded sleeps or retries that hide hangs.
- The first real VOD run exposed that the runtime passed `storeUseCases` into the upload-processing adapter even though that facade does not preserve the repository's upload-recording method. A successful transcode was therefore followed by `upload recording store unavailable`; the unbudgeted persistence retry re-enqueued the whole upload and submitted thousands of duplicate jobs. Wire the concrete repository through a compile-time-complete narrow adapter, retry persistence operations in place with a fixed budget, and never resubmit an accepted transcode merely because recording/update persistence failed.
- The next VOD run exposed two additional contract gaps hidden by handler-only tests: global API authentication rejected the signed `GET /api/uploads/{id}/media` request before its constant-time media-token check, and Compose did not pass the supported `BITRIVER_TRANSCODE_LADDER` setting into the API container. Exempt only the exact signed-media GET route from session auth, keep all neighboring upload routes protected, and add the ladder variable to Compose/env/documentation together.
- Upload FFmpeg failures were also silent because the shared launcher selected log context only from the live-job map. Resolve live or upload metadata under the server lock before constructing the process logger so a failed VOD job remains diagnosable without placing signed source URLs in retained release evidence.
- The authenticated source then proved that `POST /v1/uploads` is asynchronous while `UploadProcessor` treated acceptance as completion: it marked the upload ready and deleted the source before FFmpeg opened it, producing a 404. Add an authenticated upload-job status resource, persist success/failure state, and make the HTTP ingest adapter wait under the processor's existing bounded context. Source cleanup and public readiness may occur only after FFmpeg plus publication complete.
- The final operator probe identified a Docker Desktop/Git Bash harness distortion rather than unhealthy services: MSYS rewrote the exported `/healthz` value to `C:/Program Files/Git/healthz` before Compose passed it into the Linux API container. Disable only MSYS environment conversion for native Docker invocations in the Windows smoke path; retain argument conversion so temporary Compose paths still resolve, then require the aggregate status to pass against the unmodified container endpoint.
- The first Ubuntu CI run exposed an ownership conflict in the new isolated media volume: the image creates `/work` for UID 10001, but the legacy Linux smoke override forced the host runner UID 1001, so the transcoder failed on `mkdir /work/live: permission denied`. Keep host UID overrides only for services that write bind-mounted checkout paths; let the transcoder use its image user with the named volume. Failure diagnostics must also report only sanitized container state rather than dumping `docker inspect` configuration and environment values.
- After Linux quickstart passed, the tagged Postgres tier exposed stale test-only ingest-stub usage: `postgres_ingest_e2e_test.go` still supplied the removed `PlaybackURL` option and expected the superseded OME application create/delete sequence. Align the Postgres repository scenario with the current `OMEPlaybackBaseURL` and application-validation contract, then run the actual `-tags postgres` suite before relying on the remote rerun.
- Multipart VOD source URLs generated from a host request must also be reachable from the transcoder container. Set the request Host to the canonical internal API origin while connecting through the published host port, then verify the returned playback URL externally.
- Live and VOD transcoding are CPU-heavy on hosted runners and Docker Desktop. Use a deterministic short fixture, a release-grade timeout, and measured phase durations; do not lower the 1080p content assertion.
- Raw Compose logs and generated configuration can contain operator credentials. Never put `.env`, generated OME/SRS config, request cookies, authorization headers, or unredacted command lines in the evidence directory.
- Browser playback proof may need a separate Playwright phase after the media/API harness is stable. Do not claim browser recovery/quality behavior from FFprobe evidence alone.
- The current workflow may invoke quickstart and the legacy ingest test in one job. Rewire it so the expensive real-stack path runs once, while the cheap storage test remains independently callable.

### Golden-path test plan

- Add static/unit coverage for report redaction, URL sanitization, timeout/failure stage reporting, deterministic fixture/probe parsing, and workflow wiring.
- Add a runtime-wiring regression plus processor tests that force recording and ready-state persistence failures; require a bounded terminal state and exactly one ingest submission.
- Run the cheap storage ingest integration test separately and prove its name/docs no longer describe it as the canonical product E2E.
- Run the new harness against Docker Desktop from a clean Compose teardown, require real 1080p RTMP publish, SRS live state, OME and transcoder playback probes, chat/moderation, VOD publication/playback, health surfaces, offline transition, evidence scan, and clean teardown.
- Deliberately break at least one dependency input in a focused test and require a failure at the named stage with no secret echoed.
- Run `./scripts/verify.sh`, viewer checks where browser evidence changes, Compose config validation, the upgraded ingest workflow contract tests, and required remote CI before merge.
- Leave tagged pull-only Ubuntu/XOA repetition, repeated-run flake measurements, and browser player recovery/quality evidence explicitly pending until their direct runs exist.

## Scope
- Advance production blocker #1297 with a clean-host Linux Compose installer that consumes release artifacts only and targets Ubuntu 24.04 LTS x86_64 first.
- Make launcher archives plus `.deb`/`.rpm` packages self-contained for the canonical pull-only stack: CLI/wrapper, Compose/env contract, migrations, renderers/templates, proxy config, systemd integration, and operator docs.
- Install immutable program assets separately from operator-owned configuration and data; provide idempotent install/status/log/upgrade entrypoints plus a safe uninstall that retains data unless destruction is explicitly requested.
- Publish the OME config helper as a multi-architecture release image so a source-free host never needs `ome-config:local` or a Go toolchain.
- Add repeatable artifact-only acceptance that proves package contents, non-root/sudo operation, paths containing spaces, restart semantics, diagnostics, and data-preserving uninstall.
- Document the XOA/XCP-ng VM and Nginx Proxy Manager topology, including public HTTP(S), WebSocket forwarding, trusted proxies, media/firewall ports, and internal-only control services.

## Main reconciliation scope (2026-07-24)
- Reconcile the installer foundation with current `main`, including merged PR #1326 and its proven SRS callback, public RTMP/LL-HLS, same-origin `/live/`, OME application, transcoder, Windows evaluation, and README contracts.
- Resolve workflow and release-asset conflicts by composing both requirements: the Ubuntu artifacts must include every new required media URL, and image scans/quickstart fixtures must continue to render the canonical contract.
- Keep the public documentation truthful before the first tag: installer/package code may be release-ready while GitHub Releases, `.deb`, `.rpm`, and launcher downloads remain unavailable.
- Re-run the installer lifecycle, release bundle/package checks, canonical Compose render, full repository verification, and remote PR gates on the reconciled head before marking #1325 ready.

## Design Decisions
- The production install root is `/opt/bitriver-live`; configuration lives under `/etc/bitriver-live`; durable application/transcoder data lives under `/var/lib/bitriver-live`. Release assets remain replaceable and operator data remains outside package ownership.
- A single systemd unit wraps the canonical Docker Compose stack. Docker retains container restart behavior; systemd provides boot ordering, status, bounded startup, and an operator-visible failure boundary.
- The installer stages but does not silently weaken or bypass production validation. First activation uses the existing `bitriver env init --wizard`, `doctor`, `env validate`, pull preflight, migration runner, quickstart, and health checks.
- Linux packages install the same bundle layout as the launcher archive. Package installation may create directories and the disabled unit, but it must not start with sample credentials.
- `bitriver-ome-config` is a version-matched GHCR image for `linux/amd64` and `linux/arm64`; Compose and image preflight use its release tag/digest just like other first-party services.
- Ubuntu 24.04 amd64 is the declared production target for this change. Debian 12 and Linux arm64 remain provisional until their clean-host evidence passes; release docs must not overclaim them.
- Installer completion means files and service integration are installed. Production readiness additionally requires successful quickstart, OME process health, authenticated OME control-plane access, aggregate API health, and the basic ingest/playback acceptance owned by #1300.

## Assumptions
- Docker Engine and the Compose v2 plugin are installed from Docker's supported repository, or the installer may install them only after explicit operator confirmation.
- The VM has at least 4 vCPU, 8 GiB RAM, 20 GiB free disk, working DNS, and outbound access to GHCR plus pinned third-party registries.
- Nginx Proxy Manager terminates TLS on a separate trusted host or VM and proxies the viewer/API HTTP origin to the BitRiver VM. RTMP, LL-HLS/WebRTC, TURN/relay, and ICE traffic are forwarded directly rather than tunneled through the HTTP proxy host.
- The tagged release publishes all first-party multi-architecture images before clean-host acceptance runs.

## Risks
- Existing launcher packages omit runtime assets and resolve repo-relative paths from the current directory; add an explicit installed asset root and verify the bundle outside the source checkout.
- Compose currently uses local-only OME render images in pull mode; publishing and pinning this helper changes the deployment contract and release workflow together.
- SRS/OME render jobs write generated files into the installed asset tree; use an operator-owned runtime workspace or deliberately writable generated paths without making immutable package files credential-bearing.
- A systemd timeout that is too short can kill a valid first image pull; make it bounded but generous and preserve redacted logs plus exact recovery commands.
- Nginx Proxy Manager handles HTTP/WebSocket traffic but not the full media port surface. Documentation must distinguish proxy routes from XOA/firewall/NAT rules.
- GitHub-hosted CI cannot prove an actual XOA VM reboot. Automate everything repeatable, then leave final tagged-release reboot evidence explicitly pending rather than claiming it.
- Read-only Go dependency and verification checks must stay inside first-party Go roots and must not request VCS build stamping: on Windows-mounted Linux workspaces, an implicit `git status`, `go test ./...`, or a blanket `find .` can livelock on frontend dependencies and media before test timeouts start. Set `GOFLAGS=-buildvcs=false`, scope default Go verification and models-import scans to `cmd`, `internal`, `scripts`, and `web`, and guard these contracts in tests.
- ShellCheck treats single-quoted workflow-contract patterns containing `$out_dir` or `$launcher_root` as suspicious non-expansion. Preserve the intended literal dollar signs with escaped variables in double-quoted fixed-string assertions so CI remains strict without suppressions.
- Image-scan logic is duplicated between `ci.yml` and the manual workflow; a hard-coded legacy `bitriver-live/ome-config:local` reference can diverge from the Compose-built GHCR tag and make Trivy scan a nonexistent image. Resolve the OME helper from the collected Compose image list in both workflows and reject missing or ambiguous matches.
- Compose classifies `ome-health-token-check` as non-buildable even though it shares the locally built `ome-config` image. A blanket `compose pull --ignore-buildable --policy missing` can still make a denied registry request for that sibling service. Enumerate rendered image references, retain any image already inspectable in the local daemon, and pull only genuinely absent runtime images.
- The installer branch and PR #1326 both changed release workflows, deployment variables, quickstart smoke, and operator documentation. A mechanical conflict choice could silently discard either the published OME helper or the verified media-routing contract; reconcile each overlapping file by rendered/runtime behavior, not by branch preference.
- Current `main` correctly says no tagged downloads exist. Do not replace that statement with live download commands until the release workflow has actually published a tag and assets.
- The quickstart smoke may reuse an operator's existing root `.env`, including files created before public RTMP/LL-HLS variables became required. Because the smoke already forces an isolated development/build posture and host ports, it must also supply non-secret smoke defaults for every required public media URL without rewriting the operator file.
- Current `main` merged three incompatible major development-tool bumps: TypeScript 7 is outside the latest `ts-jest` range (`typescript >=4.3 <7`), ESLint 10 is outside the Next.js-bundled lint plugins' declared ranges, and Node 26 types exceed the enforced Node 24 runtime. Restore the proven TypeScript 6.0.3, ESLint 9.39.5, and `@types/node` 24.13.3 pins until the complete toolchain supports the newer majors; do not bypass peer resolution with `--force` or `--legacy-peer-deps`.
- The official npm audit reports three high-severity runtime findings against Next 16.2.10 and its PostCSS/Sharp dependency chain. Upgrade `next` and `eslint-config-next` together to the non-major fixed 16.2.11 release and require the blocking production audit to report no high/critical findings.
- Next 16.2.11 fixes its direct advisories but still hard-pins vulnerable PostCSS 8.4.31 and allows only vulnerable Sharp 0.34.x; no newer stable Next release exists. Temporarily override those transitive packages to fixed PostCSS 8.5.22 and Sharp 0.35.3, lock the exception in the runtime-baseline test, and exercise the production image. The viewer already sets `images.unoptimized: true`, so it does not depend on Next's image optimizer; remove the override once an aligned stable Next release ships.
- The root Docker build context currently includes `deploy/transcoder-data/`; this checkout's local media made the context roughly 200 MB and could leak runtime artifacts to a builder/cache. Exclude that runtime directory explicitly and lock the boundary in a static regression.
- The reconciled CI image scan found CVE-2026-59873 in `tar@7.5.15` under the Node 24 Alpine image's global npm installation, not in the viewer application dependency tree. The production runner needs the Node executable but never invokes npm or npx; remove the package manager payload and entrypoints from the final runtime stage, retain npm in the build stages, and guard both the runtime capability and reduced attack surface in the Dockerfile baseline test.

## Test Plan
- Shell syntax, installer unit tests, and focused Go tests for installed-root discovery, image preflight, Compose invocation, and OME helper image selection.
- Assemble launcher bundles in temporary paths containing spaces and verify every Compose bind mount plus required executable/document exists without reading the checkout.
- Exercise install twice, disabled-before-configuration behavior, status/log commands, upgrade staging, service enablement, and safe uninstall/data purge using isolated filesystem roots.
- Extract `.deb` and `.rpm` payloads and compare them to the canonical bundle manifest; run package install checks in Ubuntu 24.04 and Debian 12 containers where possible.
- Render Compose in pull-only mode, run the existing quickstart smoke with the release-shaped bundle, and require bounded OME/aggregate health diagnostics.
- Run `./scripts/verify.sh`, release workflow contract tests, `git diff --check`, and pull-request CI before publication.
- Re-run default Go verification, architecture, and models-import checks in the pinned Linux toolchain; assert VCS stamping is disabled and package/filesystem traversal is limited to first-party Go roots so mounted-workspace verification remains bounded.
- Run ShellCheck on the corrected release-bundle assertion and rerun its focused test before pushing the CI repair.
- Add a workflow regression guard for Compose-derived OME scan selection, parse both workflow files, and reproduce the image-selection logic against the rendered CI Compose image list before republishing.
- Add a quickstart regression guard against blanket non-buildable pulls, run shell syntax plus the full quickstart smoke locally, and require the unified Ubuntu gate to pass on the same corrected path.
- After merging current `main`, run focused Go workflow/contract tests, `bash -n` on changed shell scripts, the release bundle and Compose-host lifecycle tests, `docker compose ... config --quiet` with a generated production-shaped env, `scripts/test-quickstart.sh`, and `./scripts/verify.sh`.
- Inspect the staged reconciliation for root `.env`, generated OME/SRS output, runtime data, secrets, and the user's unrelated untracked deployment helpers before publication.
- Run the quickstart smoke with a pre-existing env that omits the public RTMP/LL-HLS variables and require Compose interpolation to proceed through the runtime gate.
- Run a clean `npm ci`, viewer lint/test/build, and the viewer Docker build after restoring the supported Node 24 / TypeScript 6 / ESLint 9 development baseline; require the clean install to avoid peer-override warnings.
- Run `npm audit --omit=dev --audit-level=high` after the Next.js patch and record any remaining high/critical production finding as release-blocking.
- Start the clean production viewer image after the security overrides and require its public route to return successfully; do not accept a zero-count audit if the resulting image cannot boot.
- Rebuild the root OME/helper context after updating `.dockerignore` and confirm local transcoder output is neither transferred nor packaged.
- Rebuild the production viewer image, confirm `node` and the Next standalone server remain runnable while npm/npx and `/usr/local/lib/node_modules/npm` are absent, then rerun the blocking image scan and full remote CI.

## Boundaries
- The user explicitly authorized installer, deployment-contract, and roadmap work for the Ubuntu/XOA/Nginx Proxy Manager target, including the necessary release workflow changes.
- Do not edit root `.env`, stage generated OME credentials/config, or include the user's untracked deployment helper files/runtime data.
- Do not expose PostgreSQL, Redis, SRS control, OME Managers API, or transcoder control ports through Nginx Proxy Manager.
- Do not claim Debian, ARM64, host reboot, or real playback acceptance until direct evidence exists.
- Do not automatically delete operator configuration, database volumes, recordings, or transcoder data during package removal.

## Completion
- Local implementation and acceptance are complete: pinned-toolchain repository verification, package generation, PostgreSQL migration acceptance, release-shaped Compose/OME smoke, and clean teardown passed.
- Draft PR #1325 is published. Its ShellCheck failure, stale local-only OME Trivy target, and sibling-service registry pull were repaired with focused Linux bundle/workflow/Go checks plus a full OME/API/viewer host smoke and clean teardown passing. Required CI is green on implementation head `de869492`, including the Ubuntu test-all gate, image vulnerability scan, viewer integration, cross-platform Go, and entrypoint checks.
- The installer candidate is reconciled with current `main` on head `e3f96cb5`. CI run 30102433565 is green across the unified Ubuntu gate, first-party blocking image scan, viewer integration/build/audit, secret/docs/shell/workflow guards, cross-platform Go, and quickstart entrypoint matrix.
- Tagged Ubuntu/XOA reboot, authenticated OME control-plane access, and real ingest/playback/recovery remain required external release evidence for #1297/#1300/#1304; this local candidate does not claim them.
