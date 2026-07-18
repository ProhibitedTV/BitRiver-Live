# PLAN

## Scope
- Make Windows 11 with Docker Desktop and native PowerShell a first-class, reproducible source-checkout evaluation path.
- Replace the concept-art README banner with current screenshots captured from the real Docker Compose application.
- Rewrite the README around what exists today: public viewer, admin/control-plane workflow, ingest-to-playback path, local evaluation, and production boundaries.
- State release availability accurately. The remote currently has no published GitHub Releases; source checkout is the supported evaluation path until tagged artifacts exist.

## Assumptions
- The canonical runtime remains root `.env` plus `deploy/docker-compose.yml`; this change should not alter that contract unless a Windows proof exposes a genuine blocker.
- Docker Desktop uses its Linux/WSL 2 backend. Native PowerShell is the primary Windows shell; Git Bash remains useful for repository-wide Bash verification.
- Screenshots must be reproducible product evidence, not mockups. Any example account/channel state must be created through supported application or test interfaces and contain no private data.
- Ubuntu 24.04/XOA behind Nginx Proxy Manager remains the production target, but unpublished installer/package links must not appear as if users can download them today.
- OvenMediaEngine process health alone is not playback proof. The README must keep real ingest/playback and recovery acceptance separate until those gates pass.
- The Docker Desktop proof exposed a deployment-contract defect: SRS calls `/api/ingest/srs/{connect,publish,unpublish,play,stop}`, while the control plane currently exposes only the legacy `/api/ingest/srs-hook`. Align those machine-to-machine routes, authentication/CSRF exemptions, and action handling before treating ingest as proven.
- The first real publish then exposed a second contract defect: the control plane calls a nonexistent OME `/v1/applications` facade even though the configured `default/live` application is declared in `Server.xml` and therefore immutable through OME's REST API. Use the declared application, validate it through the supported manager route, forward accepted SRS streams to OME under the public channel ID, and derive the LL-HLS URL from an explicit public base.
- Browser playback is part of the OME acceptance gate. The LL-HLS publisher must emit a compatible `Access-Control-Allow-Origin` response when the viewer and OME use different origins; a successful manifest probe without that browser contract is insufficient.
- Serve OME LL-HLS through a same-origin `/live/` reverse-proxy route in the Go edge. This matches the intended Nginx Proxy Manager deployment, avoids exposing a second browser origin in the golden path, and keeps direct OME ports available only for diagnostics or advanced deployments.
- OME's top-level `<Server><IP>` is a local listener address, not a public-advertisement field. In Docker it must be allowed to remain `0.0.0.0`/`*`; public WebRTC addresses belong in the relay/ICE candidate settings, and public LL-HLS belongs in `BITRIVER_OME_PUBLIC_LLHLS_BASE_URL`. Remove the production validation rule that can force OME to bind a host address unavailable inside the container.
- An explicit transcoder stop cancels FFmpeg and therefore returns a process error such as `signal: killed`; this is expected shutdown, not component failure. Mark the job stopped before cancellation so the exit monitor can distinguish operator/control-plane stops from genuine unexpected exits and keep `/healthz` recoverable.
- Windows directory mirrors fall back from symlinks to `mklink /J`. Resolve the Windows command interpreter from `ComSpec`/`SystemRoot` instead of relying on `cmd.exe` being present in a caller-modified `PATH`; the release gate intentionally narrows `PATH` and exposed that hidden dependency.
- The Windows proof deletes its temporary evaluation env after startup, so its printed cleanup command must provide the two evaluation-only public media URL overrides explicitly; older root `.env` files should not strand the proof stack because they predate those required keys.
- Remote CI showed that the inline and standalone image-scan Compose fixtures, plus the future tagged-release validation input, must evolve with required deployment variables. Add the public SRS RTMP and OME LL-HLS URLs to all three workflow contracts and lock them with a static regression.

## Risks
- An attractive README can overstate runtime maturity; pair every workflow claim with a command, route, or supporting document and label planned release paths clearly.
- Local `.env` and browser state may contain credentials or host-specific values; never commit `.env`, tokens, terminal output, or screenshots containing secrets.
- Empty viewer state is technically real but weak product evidence; prefer deterministic demo data created through supported APIs without changing production defaults.
- Docker Desktop access may require elevated host permission even when the CLI is installed. Record host-versus-repository blockers precisely.
- Real screenshots drift when the UI changes; keep a small set, use stable viewports, document how they were captured, and validate README image links.

## Tasks
1. Inventory the current README, release/package claims, Windows entrypoints, Docker Desktop state, and reproducible viewer data paths.
2. Add or tighten a native PowerShell proof path for Docker Desktop without changing the deployment contract unnecessarily.
3. Boot the canonical source-build stack on Windows, seed only non-sensitive demo state, repair the verified SRS/OME callback and media-routing mismatches, and exercise bounded RTMP ingest through SRS, the transcoder, and OME before recording the exact playback level reached.
4. Capture and inspect current product screenshots from the running application; remove the concept-art asset.
5. Rewrite README and supporting quickstart documentation with accurate workflows, support boundaries, and release status.
6. Run focused checks plus `./scripts/verify.sh --viewer`, review the diff for secrets/runtime data, and publish this work separately from the Ubuntu installer PR.

## Test Plan
- PowerShell parser/entrypoint tests and focused Go tests for any Windows wrapper changes.
- `docker version`, `docker compose version`, Compose config rendering, and the native PowerShell proof command on this Windows Docker Desktop host.
- HTTP checks for `/healthz`, `/readyz`, `/viewer`, and `/admin`; use aggregate smoke plus bounded OME diagnostics without claiming real playback unless exercised.
- Server/API regression tests for canonical SRS event routes: no browser session required, the shared hook token remains mandatory, `connect` succeeds without a stream key, and publish/unpublish retain lifecycle handling.
- Adapter/controller tests for explicit public/internal SRS RTMP bases, the supported OME `default/live` manager route, immutable application cleanup, dynamic SRS-to-OME forwarding, and derived public LL-HLS URLs.
- A bounded synthetic RTMP publish from inside Compose, followed by SRS/control-plane/transcoder/OME logs and directory/playback probes with credentials redacted.
- A real browser playback check that distinguishes manifest availability from CORS/player readiness, plus a direct bounded FFmpeg decode of the OME LL-HLS output.
- Proxy tests that preserve the `/live/<channel>/llhls.m3u8` path, stream upstream responses, and return a bounded gateway error when OME is unavailable.
- Production env/quickstart tests that accept OME wildcard listener values while continuing to reject loopback public playback, ingest, viewer, and transcoder URLs; OME render tests must keep the wildcard in `<Server><IP>`.
- A transcoder lifecycle regression that deletes an active job, waits for process/metadata cleanup, and requires `/healthz` to remain `200` rather than retaining the cancellation signal as a permanent FFmpeg error.
- A Windows command-interpreter regression that proves the junction fallback honors `ComSpec`, followed by the full transcoder lifecycle test under the same narrowed `PATH` used by the repository gate.
- PowerShell parser/static-contract checks for the Windows proof's self-contained cleanup command, plus live execution of that command against the proof stack.
- Extract each image-scan `.ci.env` fixture and require `docker compose config --quiet` to render with it; statically require the tagged-release workflow to forward both public media URLs before pushing the CI repair.
- Viewer lint, Jest, production build, and browser inspection at desktop and mobile widths for changed README-visible surfaces.
- Markdown/image-link validation, `git diff --check`, and full `./scripts/verify.sh --viewer` before publication.

## Boundaries
- Do not edit or stage root `.env`, generated credentials, runtime volumes, recordings, or the user's untracked deployment helper files.
- Keep PR #1325 and its Ubuntu installer/package changes separate; do not copy unpublished artifact claims into this branch.
- Do not claim a tagged release, downloadable package, clean Ubuntu install, real OME ingest/playback, or OME recovery result without corresponding remote or runtime evidence.
- Preserve the single-host operator support boundary and the existing deployment contract unless verified Windows execution requires a scoped fix with matching contract documentation.
