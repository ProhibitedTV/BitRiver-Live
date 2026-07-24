# TASKS

## Scoped change: clean-host Ubuntu Compose installer foundation (#1297)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Inventory release packages and define the clean-host contract
  - Acceptance criteria:
    - `PLAN.md` records supported-host claims, asset/config/data layout, systemd lifecycle, OME readiness boundary, Nginx Proxy Manager topology, risks, and evidence plan.
    - Existing launcher/archive/package contents, Compose bind mounts, pull-only image behavior, installers, and docs are inspected before implementation.
    - Unrelated working-tree files and unproved Debian/ARM64/reboot/playback claims are explicit boundaries.
  - Check:
    - The Linux launcher package currently ships only `deploy/docker-compose.yml` and `deploy/.env.example`; canonical Compose also binds migrations, the migration runner, SRS/OME generated configs, Nginx config, and transcoder data.
    - Pull-only Compose still names `bitriver-live/ome-config:local`, but release automation publishes only API, viewer, SRS controller, and transcoder images.
    - Packaged CLI root discovery falls back to the current directory when no `go.mod` exists, so OME rendering cannot reliably locate installed templates.
    - The historical Ubuntu/systemd installer deploys native API binaries rather than the canonical full Compose stack.

- [x] Task 2 - Make release bundles self-contained and pull-only
  - Acceptance criteria:
    - One canonical asset manifest/staging helper builds the launcher layout used by archives and Linux packages.
    - Installed-root discovery is explicit and works outside a source checkout and from paths containing spaces.
    - Compose uses a published, version-matched OME config image in pull mode; release automation publishes/scans multi-architecture output.
    - Every Compose bind mount and render dependency required by the release-shaped stack is present without source files.
  - Check:
    - Added `deploy/install/release-assets.txt` plus `scripts/stage-release-assets.sh`; both release binary archives and launcher/package jobs now consume the same source-free asset set.
    - Staging passed in a Linux container with an output path containing spaces and included Compose/env, all migrations, the canonical migration runner, SRS/OME render inputs, Nginx config, installer/systemd assets, scripts, and operator docs.
    - Packaged root discovery now honors `BITRIVER_ROOT` and launcher/package layouts outside a Go checkout; focused CLI and wrapper tests passed with Go 1.26.5.
    - Compose now pulls `ghcr.io/bitriver-live/bitriver-ome-config` by release tag/digest; GHCR preflight and required production digest validation include it.
    - Release automation now publishes the OME helper for amd64/arm64 and emits Linux amd64/arm64 CLI, launcher, `.deb`, and `.rpm` artifacts. Compose config, release workflow tests, shell syntax, and `git diff --check` passed.

- [x] Task 3 - Add the Ubuntu host installer and safe lifecycle commands
  - Acceptance criteria:
    - Installer supports archive and package layouts, separates assets/config/data, creates a bounded systemd unit, and never starts with sample credentials.
    - Install is rerunnable; status/log/upgrade commands are actionable; non-root operation uses explicit sudo boundaries.
    - Uninstall disables/removes program integration while retaining config/data by default; destructive purge requires an explicit flag and warning.
    - OME failure leaves the unit failed with redacted service diagnostics and retry commands.
  - Check:
    - Added `deploy/install/compose-host.sh` with install/upgrade/configure/activate/doctor/status/logs/uninstall commands and a 15-minute bounded systemd unit.
    - Program assets stage under `/opt/bitriver-live`; root-owned source assets are separated from `/etc/bitriver-live` configuration and `/var/lib/bitriver-live` application/transcoder data through explicit symlinks.
    - First install runs non-interactive env initialization to rotate sample credentials but leaves the service disabled until the guided wizard, doctor, production env validation, Docker access, and bounded quickstart pass.
    - Activation failure reports systemd plus Compose status and exact targeted OME/retry commands without automatically dumping credential-bearing environment or raw logs.
    - The isolated lifecycle test passed twice from a source path and target path containing spaces; configuration survived the rerun, ordinary uninstall retained config/data, unconfirmed purge failed, and confirmed purge removed them.

- [x] Task 4 - Add artifact-only and package lifecycle evidence
  - Acceptance criteria:
    - Tests assemble and execute the bundle outside the checkout in a path containing spaces.
    - Tests cover complete contents, rerunnable install, configuration gate, systemd/service shape, restart behavior, upgrade staging, safe uninstall, and explicit purge.
    - `.deb`/`.rpm` payload generation and canonical asset parity are checked for amd64/arm64 inputs.
    - Relevant focused checks pass before documentation work proceeds.
  - Check:
    - `scripts/test-release-bundle.sh` staged the canonical payload outside the checkout in a path containing spaces, verified manifest parity, and rejected generated credential-bearing OME/SRS files.
    - `scripts/test-compose-host-installer.sh` covered rerunnable install, rotated configuration, rendered systemd shape, upgrade-safe state retention, ordinary uninstall, rejected purge, and confirmed purge without touching the host.
    - `scripts/test-linux-packages.sh` used nFPM v2.47.0 to build and inspect amd64/arm64 `.deb` and `.rpm` payloads from the staged release bundle.
    - Real package generation exposed and fixed unsupported nFPM template syntax and an extra asset-directory nesting level; package paths now resolve to `/usr/local/share/bitriver-live/deploy/...`.

- [x] Task 5 - Document Ubuntu/XOA/Nginx Proxy Manager installation and support boundaries
  - Acceptance criteria:
    - README, quickstart, Ubuntu install guide, deployment contract, release notes, and production release guide describe the artifact-only path.
    - VM sizing, Docker/Compose setup, non-root/sudo workflow, boot recovery, firewall/NAT ports, WebSockets, trusted proxies, TLS, backup/upgrade/uninstall, and diagnostics are explicit.
    - Ubuntu 24.04 amd64 is the only production claim unless additional direct evidence passes; real ingest/playback and OME restart remain assigned to #1300/#1304.
  - Check:
    - Replaced the stale native Ubuntu service guide with the artifact-only Compose host path for Ubuntu 24.04 amd64, including XOA VM sizing, Docker/Compose prerequisites, archive/package checksums, two-phase activation, paths, backup, upgrade, uninstall, and reboot evidence.
    - Documented NPM app/media proxy hosts, WebSockets, exact trusted-proxy CIDR, TLS/public URL values, and the direct RTMP/WebRTC firewall/NAT boundary that an HTTP reverse proxy cannot satisfy.
    - README, quickstart, deployment contract, release guide/notes, deploy map, testing, upgrades, and the NPM/Cloudflare guide now agree on the release asset and systemd lifecycle.
    - OME language explicitly requires authenticated control plus real ingest/playback/recovery against the tagged VM; an unauthenticated root health probe is not release approval.
    - Generated contract environment index and installer-language consistency checks passed.

- [x] Task 6 - Run full verification and prepare publication evidence
  - Acceptance criteria:
    - Full repository verification, release/package tests, Compose rendering, and quickstart smoke pass or exact environment blockers are recorded.
    - Diff review excludes credentials, generated runtime output, and unrelated deployment helpers/data.
    - Final task evidence distinguishes implementation proof from tagged-release VM reboot and playback evidence.
  - Check:
    - Literal `./scripts/verify.sh` passed in the pinned Go 1.26.5 plus Python container, including release-bundle, installer-lifecycle, all first-party Go package, architecture, models-import, dependency-source, contract-invariant, and generated-contract checks. Docker and viewer phases reported explicit container-tooling skips.
    - The verification entrypoint now disables VCS stamping and bounds default Go/filesystem traversal to first-party Go roots; focused regression tests prevent the Windows-mounted-workspace livelocks exposed by this final run.
    - `scripts/test-postgres-migrations.sh` passed the real PostgreSQL migration lifecycle. `scripts/test-quickstart.sh` then rebuilt the release-shaped stack and passed OME helper rendering/validation, OME health-token preflight, service health, migrations, API health, and retried viewer health.
    - `scripts/test-linux-packages.sh` generated and inspected amd64/arm64 `.deb` and `.rpm` payloads with nFPM v2.47.0. Compose rendering, YAML parsing, PowerShell parsing, shell syntax, installer-language consistency, and `git diff --check` also passed.
    - Post-smoke `docker compose --env-file .env -f deploy/docker-compose.yml ps --all` returned an empty service table; generated OME/SRS config and root `.env` have no diff.
    - GitHub authentication was restored; commit `c3dd9c65` was pushed and draft PR #1325 opened without closing #1297. The first CI pass caught ShellCheck SC2016 in two intentional literal workflow assertions; escaped fixed-string patterns now preserve the contract without suppressions, and Linux `bash -n` plus `scripts/test-release-bundle.sh` pass locally.
    - The repaired CI run then exposed a stale `bitriver-live/ome-config:local` Trivy target duplicated in `ci.yml`, even though Compose built `ghcr.io/bitriver-live/bitriver-ome-config:ci`. Both scan workflows now select exactly one OME helper from the collected Compose image list; YAML parsing, CI contract validation, rendered image selection, and the focused Go 1.26.5 regression test pass.
    - The next image scan passed, but the Ubuntu gate exposed `compose pull --ignore-buildable` contacting GHCR for the non-buildable `ome-health-token-check` sibling after its shared helper image had already been built locally. Quickstart now enumerates rendered image references, retains locally inspectable images, and pulls only genuinely absent images.
    - Linux syntax and the focused Go 1.26.5 quickstart regression passed. A full host smoke then reused the local OME helper, passed OME render/token/process health, migrations, API health, and viewer retry, and cleaned down to an empty Compose project.
    - Final CI run 29628820507 passed on implementation head `de869492`: Ubuntu test-all, image vulnerability scan, ShellCheck, docs/workflow consistency, committed-secret guard, viewer integration, Windows/macOS Go tests, and Ubuntu/macOS/Windows entrypoint checks are green.
    - Draft PR #1325 remains unmerged. No production-release claim is permitted until the external release gates below pass.
    - Unrelated deployment helpers/data remain untracked and are explicitly excluded from the intended change set; the temporary `.gomodcache/` was removed after verification.
    - Tagged Ubuntu/XOA reboot, authenticated OME control-plane, and real ingest/playback acceptance remain external release evidence owned by #1297/#1300/#1304 and are not claimed by this local candidate.

- [-] Task 7 - Reconcile the installer candidate with current main
  - Acceptance criteria:
    - PR #1326's merged SRS/OME/transcoder/public media URL and Windows documentation contracts remain intact.
    - Ubuntu release assets, OME helper publication, package/systemd lifecycle, and pull-only behavior remain intact.
    - README and quickstart distinguish implemented future release paths from downloads that do not exist before the first tag.
    - Focused contract tests, release-bundle/installer checks, Compose rendering, full verification, and remote PR gates pass on the reconciled head.
  - Check:
    - Read-only merge analysis identified overlapping release workflows, Compose/env validation, quickstart smoke, and operator documentation; `PLAN.md` now records the reconciliation rules and validation plan.
    - Current `main` is `0f557e81`; PR #1325 remains draft at `5d2f3d11`, with its historical CI green but its head non-mergeable until this reconciliation is complete.

### Execution log
- Task 1 analysis:
  - Confirmed #1297 requires artifact-only install/restart/reboot/status/log/upgrade/uninstall behavior and an explicit Ubuntu/Debian/ARM64 support matrix.
  - Selected Ubuntu 24.04 amd64 as the first production target; Debian 12 and Linux arm64 remain provisional despite current cross-build/package jobs.
  - Selected `/opt/bitriver-live`, `/etc/bitriver-live`, and `/var/lib/bitriver-live` as separate program/config/data boundaries with data-preserving uninstall.
  - Identified local-only OME config image publication and packaged-root discovery as prerequisites to any truthful clean-host success claim.
- Task 2 implementation:
  - Replaced release-workflow copy fragments with a canonical manifest-driven asset staging step shared by binary archives and launcher/package payloads.
  - Added explicit installed asset-root resolution so the Go renderer, Compose defaults, doctor, migrations, and release commands resolve the release bundle rather than the invoking shell's directory.
  - Converted the OME helper from a local-only image name to a tagged/digest-pinnable GHCR contract and added it to multi-architecture publication plus vulnerability scanning.
  - Expanded launcher/package builds to Linux arm64 without declaring support until clean-host evidence passes.
- Task 3 implementation:
  - Added a release-layout-aware host manager and systemd unit rather than extending the historical native API-only installer.
  - Made package/archive installation safe-by-default: sample secrets are rotated, but no service is enabled before production network values and Docker prerequisites validate.
  - Kept immutable source/package payloads separate from the installed runtime workspace and operator-owned configuration/data so upgrades and package removal cannot silently erase state.
  - Added explicit OME failure/retry guidance while reserving real playback and restart acceptance for #1300/#1304.
- Task 4 verification:
  - Added permanent release-bundle, installer-lifecycle, and opt-in real Linux package acceptance scripts.
  - Added container package-install/remove acceptance to the release workflow for Ubuntu 24.04, Debian 12, and Rocky Linux 9 while keeping the production support claim limited to Ubuntu 24.04 amd64.
  - Proved nFPM emits all four Linux package variants and that the package payload is sourced from the same asset manifest as release archives.
- Task 5 documentation:
  - Established Ubuntu 24.04 amd64 as the production installation target while keeping Debian/RPM/arm64 claims provisional pending tagged clean-host evidence.
  - Added a concrete XOA plus Nginx Proxy Manager runbook that separates HTTP reverse proxying from RTMP/WebRTC L4 and UDP exposure.
  - Added clean-host/reboot evidence requirements to the production release gate and v1.2.3 draft notes instead of treating container health as end-to-end media proof.
- Task 6 verification/publication:
  - Passed literal repository verification in the pinned Go 1.26.5 environment, real PostgreSQL migration acceptance, real nFPM package generation, and a rebuilt Compose quickstart smoke through OME/API/viewer health; confirmed clean teardown afterward.
  - Hardened verification against implicit VCS stamping and unbounded mounted-workspace traversal after the complete gate exposed both portability defects.
  - Published the local candidate as draft PR #1325. The initial CI run's sole early failure was ShellCheck SC2016 on intentional literal variables; corrected it with escaped fixed-string assertions and passed the focused Linux syntax/bundle checks before republishing.
  - The next vulnerability job found the main CI workflow still scanned the retired local OME helper tag. Unified both scan workflows around the image rendered by Compose and added a regression guard against missing, ambiguous, or legacy targets.
  - After the image gate passed, the Ubuntu gate found quickstart's blanket non-buildable pull tried to fetch the locally built helper through its health-token sibling. Switched to local-first rendered-image inspection and passed the full host smoke plus clean teardown.
  - Completed the local/publication task with required PR CI green. Kept PR #1325 draft and #1297 open because tagged XOA/reboot/media-path evidence necessarily remains pending.
