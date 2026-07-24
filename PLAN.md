# PLAN

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

## Boundaries
- The user explicitly authorized installer, deployment-contract, and roadmap work for the Ubuntu/XOA/Nginx Proxy Manager target, including the necessary release workflow changes.
- Do not edit root `.env`, stage generated OME credentials/config, or include the user's untracked deployment helper files/runtime data.
- Do not expose PostgreSQL, Redis, SRS control, OME Managers API, or transcoder control ports through Nginx Proxy Manager.
- Do not claim Debian, ARM64, host reboot, or real playback acceptance until direct evidence exists.
- Do not automatically delete operator configuration, database volumes, recordings, or transcoder data during package removal.

## Completion
- Local implementation and acceptance are complete: pinned-toolchain repository verification, package generation, PostgreSQL migration acceptance, release-shaped Compose/OME smoke, and clean teardown passed.
- Draft PR #1325 is published. Its ShellCheck failure, stale local-only OME Trivy target, and sibling-service registry pull were repaired with focused Linux bundle/workflow/Go checks plus a full OME/API/viewer host smoke and clean teardown passing. Required CI is green on implementation head `de869492`, including the Ubuntu test-all gate, image vulnerability scan, viewer integration, cross-platform Go, and entrypoint checks.
- Tagged Ubuntu/XOA reboot, authenticated OME control-plane access, and real ingest/playback/recovery remain required external release evidence for #1297/#1300/#1304; this local candidate does not claim them.
