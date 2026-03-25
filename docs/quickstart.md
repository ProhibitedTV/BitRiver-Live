# BitRiver Live Quickstart

This guide is the fastest honest path from a fresh checkout or release asset to a working BitRiver Live stack.

If you are evaluating the project, start here. If you are preparing a production rollout, get through one successful local run first, then continue with [`docs/production-single-host.md`](production-single-host.md), [`docs/security.md`](security.md), and [`docs/production-release.md`](production-release.md).

## Before you start

- Supported baseline: operator-managed single-host deployment using `deploy/docker-compose.yml` plus the repository-root `.env`.
- First-success goal: reach `http://localhost:8080/viewer`, sign in at `/admin`, and pass `go run ./cmd/bitriver smoke --env-file ./.env`.
- First run can take several minutes while Docker pulls images, renders config, runs migrations, and waits for readiness.
- Not the right fit: hands-off HA, Kubernetes-first deployment, or a managed-service expectation.

## Choose an install path

| Path | Use this when | Entry point |
| --- | --- | --- |
| Source checkout (recommended for evaluation and contribution) | You want the quickest path from a clone to a working local stack, or you expect to inspect/change code. | `go run ./cmd/bitriver quickstart` (PowerShell: `pwsh -c "go run ./cmd/bitriver quickstart"`) |
| macOS release launcher | You want to validate the packaged launcher experience from a tagged release. | `brew install --formula https://github.com/ProhibitedTV/BitRiver-Live/releases/latest/download/bitriver-live.rb && bitriver-live` |
| Linux release package | You want the packaged CLI/launcher on a Linux host. | Install the `.deb` or `.rpm` from the [latest release](https://github.com/ProhibitedTV/BitRiver-Live/releases/latest), then run `bitriver-live`. |
| Windows release installer | You want the packaged Windows entry point. | Install `bitriver-live-<version>.msi` from the [latest release](https://github.com/ProhibitedTV/BitRiver-Live/releases/latest), then launch **Start BitRiver Live** or run `bitriver-live.ps1`. |

## Shared backend pipeline (all launchers)

All entrypoints above execute one canonical deployment contract: `deploy/docker-compose.yml` plus the root `.env`.

| Stage | What runs |
| --- | --- |
| Doctor | Verify Docker + Compose prerequisites before mutating deployment state. |
| Env | Create/bootstrap root `.env` from `deploy/.env.example` when missing and validate required production settings. |
| Render | Generate `deploy/ome/Server.generated.xml` from template + `.env`. |
| Migrations | Apply database migrations via the same control-plane flow. |
| Compose up | Start services with `deploy/docker-compose.yml`. |
| Readiness | Poll `/readyz` until core services are healthy. |
| Bootstrap | Seed or print admin credentials when required. |

Choose the entry point based on packaging preference and operating system, not because you expect a different deployment model.

## First successful run

For the quickest source-based evaluation:

```bash
cp deploy/.env.example .env
go run ./cmd/bitriver env init --env-file ./.env
BITRIVER_LIVE_MODE=development go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml --image-source build
go run ./cmd/bitriver smoke --env-file ./.env
```

Success looks like:

- `http://localhost:8080/viewer` serves the viewer
- `http://localhost:8080/admin` serves the control centre
- `go run ./cmd/bitriver smoke --env-file ./.env` exits successfully

## Migration note for older local scripts

BitRiver Live no longer ships `deploy/docker-compose.ome-custom.yml`, and `BITRIVER_OME_CUSTOM_CONFIG` is no longer used. The default quickstart and Compose flow already renders and mounts `deploy/ome/Server.generated.xml` automatically.

If your local scripts still include that override filename or env toggle, remove them and use only `deploy/docker-compose.yml`.

## Platform prerequisites

| Platform | Docker runtime | Notes |
| --- | --- | --- |
| macOS 12+ | Docker Desktop with Compose V2 enabled | Start Docker Desktop first and leave at least 15GB free on Docker's data root. |
| Ubuntu 22.04+ / other Linux | Docker Engine + Compose plugin | Install the Compose plugin, add yourself to the `docker` group (or prefix commands with `sudo`), and confirm `docker compose` works without root. |
| Windows 11 (WSL 2 backend) | Docker Desktop | Run the quickstart inside WSL with the WSL 2 backend enabled; check `docker-desktop` has 15GB free. |
| Windows 11 (native PowerShell) | Docker Desktop (WSL 2 backend) | Same Docker Desktop install as above; native shells call the same Go CLI. |

Install Go 1.21+ to use the source-based quickstart (`go run ./cmd/bitriver quickstart`) or CLI tooling. Installer-backed
launchers ship a bundled CLI and do not require Go on the host.

### Minimum host requirements (safe defaults)

`bitriver doctor` now enforces conservative production defaults before startup:

- CPU: **2+ logical cores**
- RAM: **4+ GiB total host memory**
- Free disk: **10+ GiB** on the BitRiver workspace filesystem
- Docker: **24.0.0+**
- Docker Compose plugin: **2.20.0+**

These are baseline requirements for the default Compose stack. Real deployments
may need more CPU/RAM/disk when you increase concurrent streams, add heavier
transcoding profiles, retain more recordings, or enable extra compose profiles.

Run preflight explicitly at any time:

```bash
go run ./cmd/bitriver doctor
go run ./cmd/bitriver doctor --json
```

Result levels:

- `PASS`: check is healthy.
- `WARN`: not a hard blocker (or platform detection is limited), but review and
  apply mitigation guidance before production rollout.
- `FAIL`: blocking issue; command exits non-zero and startup wrappers should stop
  until you fix the listed item.

### Tier 1 coverage

The Go-based quickstart defines the canonical deployment contract across Tier 1 platforms: Windows 10/11 with Docker Desktop, macOS with Docker Desktop, and Ubuntu/Debian with Docker Engine plus the Compose plugin. Launcher wrappers and installers remain compatibility entrypoints that forward into the same Compose + `.env` pipeline. For the supported operator baseline, see [`docs/production-status.md`](production-status.md) and [`docs/production-single-host.md`](production-single-host.md).

## First step: run environment preflight

Before quickstart, run the canonical environment check wrapper:

```bash
bash deploy/check-env.sh
```

This runs `bitriver doctor` (with the canonical Compose contract) and then
`bitriver env validate` against your chosen env file. Use
`bash deploy/check-env.sh --skip-doctor` only as a temporary bypass when
you have already reviewed doctor output separately.

## Run the quickstart command

Installer path (recommended for operators):

```bash
bitriver-live
```

The launcher keeps its assets under `/usr/local/share/bitriver-live` (macOS/Linux) or `Program Files\BitRiver Live` (Windows), stores runtime settings in `<launcher-root>/.env`, and bootstraps that file from `deploy/.env.example` on first run. On first run, the Windows launcher fills in `BITRIVER_REDIS_PASSWORD` (and keeps `BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD` in sync) if the file still uses the sample placeholder so Docker Compose can interpolate the Redis credentials.

### Desktop/system-tray control panel

Run `bitriver-live ui` (or `./scripts/bitriver-live-wrapper.sh ui` / `./scripts/bitriver-live-wrapper.ps1 -Command ui` from a cloned repository) to open the desktop control panel. It keeps a tray icon handy, polls `docker compose ps` for service state/health, tails recent logs, and exposes Start/Stop/Restart/Refresh logs buttons that shell out to Docker Compose. The **Open health dashboard** link jumps straight to the control centre overview. Contributors can keep using the CLI directly when they need to rebuild images or run migrations from source.


```bash
# macOS, Linux, or Windows via WSL/bash
go run ./cmd/bitriver quickstart

# Windows PowerShell (same CLI, different shell)
pwsh -c "go run ./cmd/bitriver quickstart"
```

The Go CLI renders `deploy/ome/Server.generated.xml` directly (no Python dependency) before launching Compose. The quickstart waits for the API `/readyz` probe to succeed before seeding the admin user via the bundled `bootstrap-admin` binary, then prints a "Generated credentials" block for any secrets it auto-created so you can store them securely before logging in. The success summary now also points administrators at `/admin`, reminds you that the bootstrap credentials live in the deployment `.env`, and supports a recovery helper when you need to revisit them later:

```bash
bitriver-live env admin --env-file ./.env
# Source checkout equivalent:
go run ./cmd/bitriver env admin --env-file ./.env
```

Add `--show-password` only when you explicitly need to reveal the env-backed seed password. If you rotate the admin password later in `/admin`, treat the `.env` value as historical and use the newer live credential instead.

### Deployment image source mode (`BITRIVER_DEPLOY_IMAGE_SOURCE`)

Compose/quickstart now supports an explicit image-source switch:

- `BITRIVER_DEPLOY_IMAGE_SOURCE=pull` (default and recommended for production):
  - Runs a GHCR preflight before `docker compose up`.
  - Validates each required BitRiver image tag/digest exists (`docker manifest inspect`).
  - Fails early with precise guidance when registry access is denied (for example, missing `docker login ghcr.io`).
  - Enforces pull-only startup (`docker compose up --pull always --no-build`).
- `BITRIVER_DEPLOY_IMAGE_SOURCE=build`:
  - Skips GHCR manifest/auth preflight.
  - Requires local source build prerequisites and checks Dockerfiles before startup.
  - Runs compose in intentional build mode (`docker compose up --build --pull never`).

You can set the mode in `.env`, export it in the shell, or override per-run:

```bash
go run ./cmd/bitriver quickstart --image-source pull
go run ./cmd/bitriver compose up --image-source build --file deploy/docker-compose.yml
```

### OME auth preflight

Before `docker compose up -d` runs, quickstart now performs an OME auth preflight to avoid long healthcheck retry loops:

- Confirms `.env` contains a non-empty `BITRIVER_OME_API_TOKEN`.
- Validates `BITRIVER_OME_ACCESS_TOKEN` when set, and fails fast if it does not match `BITRIVER_OME_API_TOKEN`.
- Re-renders `deploy/ome/Server.generated.xml` and verifies `<Managers><API><AccessToken>` matches the runtime token source used by health checks.

If preflight fails, quickstart exits immediately with actionable guidance that names the exact variable to fix.

When stdin is not attached to a terminal (for example in CI, scripted deployments, or some Windows shells), the quickstart runs database migrations with `docker compose run -T` to disable TTY allocation and avoid interactive console errors.

Want a shim to handle shell-specific permissions? Use `./scripts/quickstart.sh` from POSIX shells or `./scripts/quickstart.ps1` from PowerShell. Both scripts are thin wrappers around `go run ./cmd/bitriver quickstart`, and all OME auth/env validation lives inside the Go CLI.

### Quickstart profiles

- **quickstart-dev (demo/local):** Run with a temporary shell override (`BITRIVER_LIVE_MODE=development`) when you intentionally want localhost/demo values. This keeps demo-only warnings available without mutating your committed `.env` defaults.
- **quickstart-prod (strict):** Keep `BITRIVER_LIVE_MODE=production` in `.env` and provide routable/public values for `BITRIVER_TRANSCODER_PUBLIC_BASE_URL`, `BITRIVER_OME_BIND`, `BITRIVER_OME_IP`, and `NEXT_PUBLIC_VIEWER_URL`. The quickstart now fails fast with one actionable error block when these are empty, localhost, or `0.0.0.0`.

### Production deployment contract

Before any containers are started (`deploy/check-env.sh`, `go run ./cmd/bitriver env validate`, `go run ./cmd/bitriver quickstart`, and `go run ./cmd/bitriver compose up`), production deployments must satisfy this contract:

- **Real pgx + Postgres storage only:** `BITRIVER_LIVE_STORAGE_DRIVER=postgres`, and release artifacts must be built with the non-stub pgx module path (verify with `./scripts/check-postgres-pgx.sh`).
- **Routable/public networking values:** `BITRIVER_TRANSCODER_PUBLIC_BASE_URL`, `NEXT_PUBLIC_VIEWER_URL`, `BITRIVER_OME_BIND`, and `BITRIVER_OME_IP` must be explicit non-loopback production values.
- **Authenticated registry pulls:** production bootstrap runs in pull mode and validates each required GHCR image reference with `docker manifest inspect`, which fails fast with `docker login ghcr.io` guidance when auth is missing.
- **Pinned images:** each production image must include both tag and digest in `.env` (`BITRIVER_*_IMAGE_TAG` + `BITRIVER_*_IMAGE_DIGEST`) so deployments are immutable and reproducible.

Use explicit development commands/overrides when intentionally running local-only workflows (`BITRIVER_LIVE_MODE=development` and `--image-source build`). Keep those out of committed production bootstrap runbooks.

### What the command configures

1. Verifies that both Docker and Docker Compose V2 are available and warns when disk space under the Docker data root is below 15GB.
2. Generates `.env` with the same defaults baked into `deploy/docker-compose.yml`, defaults `BITRIVER_LIVE_MODE` to `production` when the key is missing, empty, or still at the example placeholder, prompts for the administrator email when it is missing or still set to the example value, keeps `BITRIVER_LIVE_MODE=production` in the persisted file, and rotates required credentials (admin password, Postgres/Redis, SRS, OME, transcoder, metrics) to strong random values unless the file already exists. Fresh clones still need production-safe routable/public values for the quickstart-prod networking keys before `env validate`/`quickstart` will pass (`BITRIVER_TRANSCODER_PUBLIC_BASE_URL`, `BITRIVER_OME_BIND`, `BITRIVER_OME_IP`, and `NEXT_PUBLIC_VIEWER_URL`). When a pre-existing `.env` is missing required credentials, the helper backfills them (including the OME API username/password and the `BITRIVER_OME_BIND` listener address for the control listener) so Compose can start once production networking values are in place.
3. Launches the containers with `docker compose up --pull always --no-build -d` using the compose file in `deploy/`. Production quickstart expects authenticated GHCR access (`docker login ghcr.io`) and pinned tag+digest image references for API, viewer, SRS controller, and transcoder. The manifest enables `restart: unless-stopped` for each long-lived service so they come back online after crashes or host reboots. The `bitriver-live` API service intentionally waits for `srs-controller` and `transcoder` health checks (`/healthz`) before startup, so first boot can take slightly longer but avoids early dependency races.
4. Renders `deploy/ome/Server.generated.xml` from the bundled template via the Go renderer, mapping `BITRIVER_OME_BIND` to the control listener fields, stamping WebRTC signalling ports from `BITRIVER_OME_SERVER_PORT` / `BITRIVER_OME_SERVER_TLS_PORT`, and stamping Managers API listener ports from `BITRIVER_OME_HTTP_PORT` / `BITRIVER_OME_HTTP_TLS_PORT` while mirroring the OME API token from `.env` so health checks see the same `<Managers><API><AccessToken>` value the compose preflight expects. If the template or token drifts, the generator stops Compose before OME starts. The only supported render path is `go run ./cmd/bitriver ome render` (or the `./scripts/render-ome-config.sh` wrapper). Compose also renders `deploy/srs/conf/srs.generated.conf`, replacing `${BITRIVER_SRS_TOKEN}` from `.env` so SRS ingest hooks stay aligned with the API token.
5. Waits for the API readiness check to pass (`/readyz`), then invokes the `bootstrap-admin` helper to seed the admin account. After a successful run, the CLI prints any newly generated credentials once so you can store them securely. When the viewer proxy is configured (the default quickstart shape), opening the host root lands in the public viewer flow and administrators should head to `/admin` to sign in to the control centre. Once you sign in, use the control centre **System status** card (powered by `/api/status`) as the primary health view. The raw `/readyz` and `/healthz` endpoints remain available for automation and deep debugging.

Administrators and creators must complete multi-factor authentication (MFA) when signing in. The control centre prompts you to enroll the first time, provides recovery codes, and only issues a session after you verify a code. Keep the recovery codes somewhere safe in case you lose access to your authenticator.

Deployment `.env` files must keep `BITRIVER_LIVE_MODE=production`; the Go env validator (invoked by `deploy/check-env.sh`) fails fast when the mode is empty, still at the example placeholder, or still `development`. For local HTTP-only demos, leave `.env` at production and override the mode inline (for example, `BITRIVER_LIVE_MODE=development docker compose --env-file ./.env -f deploy/docker-compose.yml up -d`) or with a one-off Compose override that sets the API service's `BITRIVER_LIVE_MODE` to `development`. Drop the override after you enable HTTPS via `BITRIVER_LIVE_TLS_CERT`/`BITRIVER_LIVE_TLS_KEY` or a reverse proxy so cookies regain the `Secure` flag.

The bundled Compose Postgres runs without TLS, so the generated DSN uses `sslmode=disable` for the local `postgres` service. When you point `BITRIVER_LIVE_POSTGRES_DSN` or `BITRIVER_LIVE_SESSION_POSTGRES_DSN` at an external database, switch to `sslmode=require` or `sslmode=verify-full` and mount any private CA into `deploy/certs/` so you can append `sslrootcert=/certs/postgres-ca.pem` to the DSNs. The environment validator rejects `sslmode=disable` for any host other than the local Compose service.

The helper leaves `NEXT_PUBLIC_API_BASE_URL` empty so the viewer inherits the API origin when proxied through `NEXT_VIEWER_BASE_PATH` (default `/viewer`). Set `NEXT_PUBLIC_API_BASE_URL` to the publicly reachable API URL when serving the viewer from its own hostname and adjust `NEXT_PUBLIC_VIEWER_URL` to match before re-running `docker compose up -d`.

### Enable HTTPS with the Caddy reverse proxy

The TLS compose override adds a Caddy reverse proxy that terminates HTTPS and routes `/viewer` to the Next.js viewer while keeping `/` and `/api` on the BitRiver Live API service (websocket upgrades are proxied automatically). With the default viewer proxy enabled, the API now redirects `/` into the public viewer flow and keeps the control centre explicitly at `/admin`.

1. Update `.env` with your public hostname and ACME email:
   ```bash
   BITRIVER_PUBLIC_DOMAIN=stream.example.com
   BITRIVER_TLS_EMAIL=admin@stream.example.com
   NEXT_PUBLIC_VIEWER_URL=https://stream.example.com/viewer
   ```
2. Start the stack with the TLS override:
   ```bash
   docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.tls.yml up -d
   ```
3. Ensure ports 80 and 443 are reachable from the internet for ACME validation.

For production hosts, disable HTTP-only host ports once the proxy is handling traffic. The easiest approach is to create a local override (not checked in) that clears the published ports for services you do not want exposed, then include it after the TLS file:

```yaml
# deploy/docker-compose.no-http.yml
services:
  bitriver-live:
    ports: []
  srs-controller:
    ports: []
  ome:
    ports:
      - "${BITRIVER_OME_SIGNALLING_PORT:-${BITRIVER_OME_SERVER_PORT:-9000}}:${BITRIVER_OME_SERVER_PORT:-9000}"
      - "${BITRIVER_OME_SERVER_TLS_PORT:-9443}:${BITRIVER_OME_SERVER_TLS_PORT:-9443}"
      - "${BITRIVER_OME_RELAY_PORT:-3478}:3478/udp"
      - "${BITRIVER_OME_RELAY_PORT:-3478}:3478/${BITRIVER_OME_RELAY_PROTOCOL:-tcp}"
      - "${BITRIVER_OME_ICE_PORT_RANGE:-10000-10009}:10000-10009/udp"
  transcoder:
    ports: []
  transcoder-public:
    ports: []
```

Apply it with:

```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.tls.yml -f deploy/docker-compose.no-http.yml up -d
```

Alternatively, keep the existing port mappings and enforce the same restrictions using host firewall rules or security groups.

Running the Go env validator (`go run ./cmd/bitriver env validate --env-file ./.env`, or the `deploy/check-env.sh` wrapper) against the quickstart `.env` errors if `BITRIVER_LIVE_MODE` is missing or left at `development`; keep the saved file at production and rely on an inline override or Compose override file for HTTP-only demos. The validator still warns when loopback values remain for the viewer URL, OME bind/IP, or the transcoder public base URL so production deployments replace placeholders with routable hosts before re-running.

The control centre now exposes a **Setup wizard** under **Settings**. Use it as the default path for production-ready configuration instead of hand-editing `.env`: it prompts for the admin email, viewer domain, API port, TLS certificate paths, and required secrets (Postgres, Redis, SRS, OME, transcoder, metrics) then writes them to the environment file and schedules a safe restart. When you provide certificate paths the wizard copies them into `deploy/certs/` (next to the compose bundle) and updates `BITRIVER_LIVE_TLS_CERT`/`BITRIVER_LIVE_TLS_KEY` automatically so HTTPS is ready on the next restart. The wizard keeps Docker/systemd installs aligned without risking partial writes.

### Retention and cleanup defaults

The quickstart keeps retention settings unset, so the API and transcoder apply their built-in defaults until you configure overrides in `.env`:

- **Recordings + VODs:** `BITRIVER_LIVE_RECORDING_RETENTION_PUBLISHED` defaults to 90 days and `BITRIVER_LIVE_RECORDING_RETENTION_UNPUBLISHED` defaults to 14 days. Set either to `0` to keep recordings indefinitely, and pair it with object storage lifecycle rules when you archive recordings outside the host filesystem.
- **Chat logs:** `BITRIVER_LIVE_CHAT_RETENTION_MESSAGES` and `BITRIVER_LIVE_CHAT_RETENTION_MODERATION_LOGS` default to `0` (no automatic purge). Set durations like `720h` to prune messages and moderation reports.
- **Transcoder outputs:** `BITRIVER_TRANSCODER_RETENTION_LIVE` and `BITRIVER_TRANSCODER_RETENTION_UPLOADS` default to empty/disabled, so HLS output under `./transcoder-data` persists until you delete it. Set a duration (for example, `168h`) to enable the 30-minute cleanup sweep for stopped live sessions and finished uploads.
- **Stored upload source artifacts:** source files used for upload transcoding are cleaned by upload status. `ready`/`completed` uploads delete source artifacts immediately after handoff, while `failed` uploads retain source artifacts for 24 hours to support debugging and retries before cleanup runs. Deletion is idempotent (already-missing artifacts are treated as no-op) and cleanup actions are logged.

The health payload still expects the ingest services to be reachable from the API container:

- **SRS controller:** `BITRIVER_SRS_API` defaults to `http://srs-controller:1985` inside the Compose network. If you move SRS elsewhere, point this URL at a reachable host and keep the API token aligned with the controller's configuration. To expose the SRS HTTP API on the host for debugging, enable the `srs-api` profile (`docker compose --profile srs-api up -d`) so `BITRIVER_SRS_API_PORT` is published—leave it disabled in production and never expose the port publicly. The SRS webhook callbacks in `deploy/srs/conf/srs.conf` always read their token from `BITRIVER_SRS_TOKEN` in `.env`, so update the environment file instead of hardcoding query strings. The Compose stack runs the `srs-config` helper to render `deploy/srs/conf/srs.generated.conf` before SRS starts; if you operate SRS outside Compose or rotate `BITRIVER_SRS_TOKEN`, rerun `./scripts/render-srs-config.sh --force --env-file ./.env` so the mounted config stays current.
- **OvenMediaEngine:** `BITRIVER_OME_API` defaults to `http://ome:8081` and expects `BITRIVER_OME_API_TOKEN` from `.env`. A short-lived `ome-config` helper in the compose file renders `deploy/ome/Server.generated.xml` from `deploy/ome/Server.xml` before OME starts, keeping the API token aligned with `.env` for authenticated control-plane calls. When running OME outside Compose, keep this URL reachable from the API container so `/healthz` reports the correct status even though the HTTP status code remains 200 during degraded states, and mirror the same credentials in your OME configuration. The template writes API auth to top-level `<Managers><API><AccessToken>`, rewrites control-listener bind fields from `BITRIVER_OME_BIND` (default `0.0.0.0`), maps `<Bind><Managers><API><Port>/<TLSPort>` from `BITRIVER_OME_HTTP_PORT` / `BITRIVER_OME_HTTP_TLS_PORT`, maps WebRTC signalling `<Port>/<TLSPort>` from `BITRIVER_OME_SERVER_PORT` / `BITRIVER_OME_SERVER_TLS_PORT`, keeps the root `<Bind>` container focused on provider/publisher protocol sections (`<Providers>`, `<Publishers>`), omits unsupported root `<Bind><IP>`/`<Bind><Address>` host tags, and fills top-level `<Server><IP>` with `BITRIVER_OME_IP` (defaulting to `BITRIVER_OME_BIND`) as the canonical server bind host field—update both the template and `BITRIVER_OME_API` together if you customize the control API port. The liveness/readiness probe now targets OME's unauthenticated local root endpoint (`http://localhost:${BITRIVER_OME_HTTP_PORT:-8081}/`) so container health does not depend on control-plane auth headers. A probe is considered healthy when curl can connect and returns any non-`000` status below `500`; this tolerates auth redirects/`401`/`404` while still failing on transport errors and `5xx`. Render-time validation still fails fast when `${BITRIVER_OME_HEALTHCHECK_TOKEN:-${BITRIVER_OME_ACCESS_TOKEN:-$BITRIVER_OME_API_TOKEN}}` does not match the rendered top-level `<Managers><API><AccessToken>` value. For OME application outputs, keep profiles directly under `<Application><OutputProfiles>`; do not wrap them in deprecated `<Application><Outputs>`. Keep LL-HLS configuration at publisher scope (`<Bind><Publishers><LLHLS>`) only and avoid defining `<Application><LLHLS>`. Leave `BITRIVER_OME_SIGNALLING_PORT` empty to reuse `BITRIVER_OME_SERVER_PORT` for the host binding; set it explicitly only when the host-facing WebRTC port must differ from the value rendered into `Server.xml` so the API and browsers connect to the expected port.
  Edit `BITRIVER_OME_API_TOKEN`, `BITRIVER_OME_BIND`, `BITRIVER_OME_IP`, `BITRIVER_OME_HTTP_PORT`, `BITRIVER_OME_HTTP_TLS_PORT`, `BITRIVER_OME_SERVER_PORT`, or `BITRIVER_OME_SERVER_TLS_PORT` in `.env`? Re-render `deploy/ome/Server.generated.xml` with `go run ./cmd/bitriver ome render --force --env-file ./.env` (or the `./scripts/render-ome-config.sh` wrapper) before running `docker compose up -d` when you operate OME outside Compose so the API access token and canonical server host settings stay aligned with the health check. Override `BITRIVER_OME_ACCESS_TOKEN` only if your deployment needs a different health probe token; by default it mirrors `BITRIVER_OME_API_TOKEN`. The quickstart helper reruns the Go renderer automatically so template changes picked up via `git pull` land in the generated config before Compose starts.
- **Transcoder:** `BITRIVER_TRANSCODER_API` defaults to `http://transcoder:9000`; ensure the host and port resolve from the API container and that the token matches `BITRIVER_TRANSCODER_TOKEN`.

Update the generated `.env` before inviting real users—swap in a valid admin email, capture the generated credentials block (the quickstart rotates secrets only when needed and prints them once), rotate the `BITRIVER_POSTGRES_USER`/`BITRIVER_POSTGRES_PASSWORD` pair (and update `BITRIVER_LIVE_POSTGRES_DSN` to match), change the Redis credentials (`BITRIVER_REDIS_PASSWORD` and `BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD`), and point `BITRIVER_TRANSCODER_PUBLIC_BASE_URL` at the HTTP origin your viewers can actually reach instead of the default `http://localhost:9080`. Update the public viewer URL to match your domain or reverse proxy as well. Log in immediately at `/admin` and rotate the admin password from the control center settings page. Viewer self-registration stays disabled by default to keep new accounts admin-controlled; set `BITRIVER_LIVE_ALLOW_SELF_SIGNUP=true` in `.env` and rerun `docker compose up -d` to reopen public signups, or leave it false to require manual invites.

### Appendix: Health endpoints (advanced)

The Overview page in the control centre is the preferred health view. It calls `/api/status` to combine readiness, datastore/Redis checks, ingest probes, and remediation hints with links to relevant logs. Use the endpoints below when you need raw JSON for automation or troubleshooting:

- `/api/status` (aggregated dashboard payload with remediation guidance and log suggestions)
- `/readyz` (core dependency readiness for load balancers)
- `/healthz` (adds ingest information to `/readyz` while keeping HTTP 200 unless core dependencies fail)

## Common follow-up commands

Compose lives in `deploy/docker-compose.yml`. The Go CLI wraps Docker Compose so you can stay on the same command set across platforms:

- Inspect service health (Compose rerenders `deploy/ome/Server.generated.xml` via the `ome-config` preflight before OME starts, so stale configs block startup instead of leaving the container unhealthy):
  ```bash
  go run ./cmd/bitriver compose up --file deploy/docker-compose.yml
  docker compose ps
  ```
- Follow logs for every container:
  ```bash
  docker compose logs -f
  ```
- Stop the stack and keep data volumes intact:
  ```bash
  go run ./cmd/bitriver compose down --file deploy/docker-compose.yml
  ```
- Stop the stack and remove data volumes (destructive):
  ```bash
  go run ./cmd/bitriver compose down --file deploy/docker-compose.yml --volumes
  ```

All commands assume you are still in the repository root (where `.env` lives) so Docker Compose can locate the project name and compose file. Use `go run ./cmd/bitriver env validate --env-file ./.env` to double-check credentials before a restart.

## Updating your stack

- Pull upstream changes before restarting your containers so you pick up fixes and migrations:
  ```bash
  git pull --ff-only
  ```
- Re-run the quickstart to apply env/template changes and restart services with the latest configuration. Add `--build` when Dockerfiles or local source changes require a rebuild:
  ```bash
  go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml
  go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml --build
  ```
  The Go command reuses your existing `.env` and Docker volumes, so configuration, database data, and media files persist across updates. This is the canonical deployment path on Linux, macOS, and Windows.
`docker compose up` (including the quickstart wrapper) reruns the `ome-config` helper so OME consumes the credentials from `.env`, the current control-listener bind value from `BITRIVER_OME_BIND`, and the top-level `<Server><IP>` host from `BITRIVER_OME_IP` without requiring an extra compose override.
- Running `git pull` followed by the quickstart keeps OME in a predictable state:
  - The helper preserves your `.env` while backfilling any new variables introduced upstream so you avoid silent crashes from missing credentials, including the OME managers token.
  - `deploy/ome/Server.generated.xml` is always re-rendered from `deploy/ome/Server.xml` and the refreshed `.env`, eliminating drift between the template and the live config mounted into the container.
  - Database migrations run automatically before the stack restarts, giving you a clean, rerun-safe deploy loop whenever templates or env keys change. Add `--build` when you also need fresh local images.
- Codex CLI users: follow the [Codex CLI guide](codex-cli.md) for installation, authentication, and edit workflows tailored to this repository. Rerun `docker compose up -d` after applying Codex patches so containers reload configuration and binaries.
- Need a safer, step-by-step upgrade flow? Use the upgrade runbook in [`docs/upgrades.md`](upgrades.md#upgrade-essentials-migrations-env-updates-and-ome-re-render) for Compose stop/start sequencing, migration timing, and `.env`/OME template handling.

## Troubleshooting

- **`Error: doctor checks failed` with `Docker: not found` in the doctor output** – The quickstart starts with the BitRiver Live doctor check and exits early when the `docker` binary is not on your `PATH`. Install Docker Engine or Docker Desktop, reopen your shell so `docker` is discoverable, and rerun `go run ./cmd/bitriver quickstart`, `./scripts/quickstart.sh`, or `./scripts/quickstart.ps1`.
- **`Error: Docker is required`** – Install Docker Engine from [docs.docker.com/engine/install](https://docs.docker.com/engine/install/)
  and re-run `go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml` (or the shim for your shell).
- **`Error: Docker Compose V2 is required`** – Install the compose plugin or upgrade Docker Desktop/Engine so the `docker compose`
  sub-command is available for the Go quickstart across all platforms.
- **`permission denied while trying to connect to the Docker daemon socket`** – Add your account to the `docker` group with `sudo usermod -aG docker $USER` followed by `newgrp docker` (or log out and back in), then rerun the quickstart without `sudo`. You can run `sudo go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml` or the shell/PowerShell shims in a pinch, but expect root-owned files like `.env` until you fix the group membership.
- **`postgres repository unavailable: pgx driver stubbed in this build`** – The running binary/artifact was built without Postgres support (stub-only pgx path). Use one remediation path: rebuild and redeploy from release artifacts that are verified to include the `postgres` build tag and the non-stub pgx driver, then restart the stack with those artifacts. Use the provenance checks in [`docs/production-release.md`](production-release.md) to confirm the image/tag/digest set before redeploying.

### Windows troubleshooting (Docker Desktop + WSL 2)

If Docker Desktop fails to accept Compose traffic from WSL, you may see `http2: server: error reading preface from client //./pipe/dockerDesktopLinuxEngine` when running the quickstart or `docker compose up`. When it happens:

1. Restart Docker Desktop to reset the engine and pipe.
2. Confirm the WSL 2 backend is healthy (`wsl --status`, and verify the `docker-desktop` distro is running).
3. Re-run `docker compose up -d` from the repository root (or re-run `bitriver-live` / `go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml`).

- **Port already in use** – Stop or reconfigure any services that currently bind to ports 5432, 6379, 8080, 8081, 8083, 8443, 9000, 9001, or 1935 (plus 1985 when the `srs-api` profile is enabled). Alternatively edit the corresponding `*_PORT` values in `.env` (for example, `BITRIVER_LIVE_PORT=9090` or `BITRIVER_OME_LLHLS_HOST_PORT=8083`) and rerun `docker compose up -d`.
- **`Empty <AccessToken> is not allowed`** – The OvenMediaEngine template detected a missing `BITRIVER_OME_API_TOKEN` in `.env`. Set a non-empty value in `.env`, mirror it into `BITRIVER_OME_ACCESS_TOKEN` if you want the health probe to use a distinct header, rerun `go run ./cmd/bitriver ome render --force --env-file ./.env` (or `./scripts/render-ome-config.sh --force`), and restart the stack with `docker compose up -d` so `deploy/ome/Server.generated.xml` is regenerated with the token.
- **Still seeing OME API auth errors after rendering?** – Verify the stamp and contents of the generated config before restarting OME:
  1. Ensure `.env` sets `BITRIVER_OME_IMAGE_TAG` to the version you actually run and includes a non-empty `BITRIVER_OME_API_TOKEN`.
  2. Force regeneration to align the schema with that tag:
     ```bash
     go run ./cmd/bitriver ome render --force --env-file ./.env
     ```
  3. Confirm the rendered file contains the managers auth block and matches the tag marker:
     ```bash
     grep -n "BITRIVER_OME_IMAGE_TAG" deploy/ome/Server.generated.xml
     rg --heading "AccessToken" deploy/ome/Server.generated.xml
     ```
     Only restart `docker compose up -d ome` after both checks pass (Compose service name is `ome`; `bitriver-ome` is the container name).
- **Reading OME startup logs after template fixes** – Quickstart reruns should show the standard OME banner, FFmpeg and version
  lines, STUN public IP resolution, a burst of ICE candidate logs, and the `All modules are initialized successfully` marker
  followed by `Create HostMetrics...` and `Create ApplicationMetrics(#default#live...)`. You should no longer see
  `Application.type is not optional` or `Empty <AccessToken> is not allowed`. Messages such as `WebRTC publisher is disabled in
  #default#live application, so it was not created` simply reflect the `<Publishers>`/`<Providers>` switches in your
  `<Application>` block being set to `Off`; toggle them to match whether BitRiver Live should push or pull streams via WebRTC,
  LLHLS, and related outputs.
- **OME health check fails** – The compose service pins the hostname to `ome` so the default `BITRIVER_OME_API=http://ome:8081` resolves correctly; keep that alias if you customize the container name. Compose now runs two pre-start checks before OME launches: `ome-config` regenerates `deploy/ome/Server.generated.xml`, then `ome-health-token-check` runs `ome verify-health-token` to confirm `<Managers><API><AccessToken>` matches `${BITRIVER_OME_HEALTHCHECK_TOKEN:-${BITRIVER_OME_ACCESS_TOKEN:-$BITRIVER_OME_API_TOKEN}}` from the same `.env` file. If `bitriver-ome` falls into healthcheck restarts, inspect the helper first:

  ```bash
  docker compose logs ome-health-token-check
  ```

  The helper fails fast with the verification error so you can resolve token/config drift before chasing the OME container loop.

  Healthcheck behavior for OME 0.16 (exact):

  1. Probe `http://localhost:${BITRIVER_OME_HTTP_PORT:-8081}/` without auth headers.
  2. Treat any non-`000` status below `500` as healthy.
  3. Fail container health only on transport failures or `5xx` responses.

  If OME still restarts, re-render config and redeploy so startup/runtime settings stay aligned:

  ```bash
  ./scripts/render-ome-config.sh --force
  docker compose up -d ome
  ```
  
  Run the same verification manually when troubleshooting auth drift:
  ```bash
  ./scripts/verify-ome-health-token.sh --env-file ./.env --config ./deploy/ome/Server.generated.xml
  ```

  Copy/paste manual probe block that reproduces the in-container healthcheck:

  ```bash
  docker compose exec ome sh -lc '
    set -eu
    health_url="http://localhost:${BITRIVER_OME_HTTP_PORT:-8081}/"
    http_status="$(curl -sS -o /dev/null -w "%{http_code}" --connect-timeout 2 --max-time 4 "$health_url")"
    [ -n "$http_status" ] && [ "$http_status" != "000" ] && [ "$http_status" -lt 500 ]
  '
  ```

  If you deploy OME outside of Docker, update `BITRIVER_OME_API` to the reachable host/IP and ensure the configured API access token in `.env` matches the copied `Server.xml` before bringing the stack back up.
- **OME container fails to start or keeps restarting** – Verify the `deploy/ome/Server.generated.xml` mount exists (the compose service binds it into both `origin_conf` and `edge_conf` paths), and re-run `./scripts/render-ome-config.sh --force` if the file is missing or the `ome-config` step failed. Port collisions on `BITRIVER_OME_HTTP_PORT` (`8081`), `BITRIVER_OME_SERVER_PORT`/`BITRIVER_OME_SIGNALLING_PORT` (`9000`), `BITRIVER_OME_SERVER_TLS_PORT` (`9443`), the relay port (`3478`), or the ICE range (`10000-10009/udp`) will also keep the service in a restart loop—adjust the matching `.env` values and restart the stack if those ports are already bound on the host. Invalid `BITRIVER_OME_API_TOKEN` values still break authenticated API calls, so update `.env`, rerender `Server.generated.xml`, and restart OME after fixing them.
- **Quickstart re-run pulled the wrong OME version** – When reusing an existing installation, keep `BITRIVER_OME_IMAGE_TAG`
  aligned with the version that matches your `Server.xml` schema before re-running the Go quickstart (`go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml`) or `docker compose up -d`. The quickstart and `scripts/render-ome-config.sh --check` both compare the tag in `.env` with the marker stamped inside
  `deploy/ome/Server.generated.xml` and force a regeneration before Compose starts if they diverge. The default `0.16.0` tag
  always renders `<Managers><API><AccessToken>` and requires a non-empty `BITRIVER_OME_API_TOKEN`; non-semver image tags are
  rejected by the renderer to avoid generating configs with unsupported authentication fields.
- **Environment tweaks** – Edit `.env` and rerun `docker compose up -d` to apply changes. The compose stack automatically loads
  the file so you never need to touch `deploy/docker-compose.yml` directly.

For more advanced tuning (TLS, Redis-backed rate limiting, scaling) continue with [`docs/advanced-deployments.md`](advanced-deployments.md). If you are deploying behind Cloudflare + Nginx Proxy Manager, follow the dedicated guide at [`docs/reverse-proxy-npm-cloudflare.md`](reverse-proxy-npm-cloudflare.md).
