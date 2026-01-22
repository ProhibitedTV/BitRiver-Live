# BitRiver Live

BitRiver Live is a self-hosted streaming stack built with Go on the backend and Next.js on the frontend. One Docker command
starts the API, control centre, public viewer, RTMP ingest, transcoder, chat, analytics, Postgres, and Redis so you can run a
Twitch-style experience on hardware you control.

## Supported versions

- **Viewer:** Next.js 13.5.x with Node.js LTS (validated on Node 18/20; Next 13.5 requires Node 16.14+). Aligns with the pins
  in `web/viewer/package.json`.
- **Postgres:** pgx v5.7.x (via `github.com/jackc/pgx/v5`) supports PostgreSQL 10–16; the bundled Compose stack runs Postgres
  15 by default.
- **Redis:** go-redis v9.5.x targets Redis 6+; Docker Compose ships Redis 7-alpine.

## What you get out of the box

- **Control centre + API** – `cmd/server` serves the admin UI, chat, analytics, webhooks, and REST endpoints under one binary.
- **Public viewer** – `web/viewer` is a Next.js app (proxied through the Go API by default) so fans can browse channels and
  watch streams.
- **Streaming pipeline** – Docker Compose wires SRS (RTMP ingest), OvenMediaEngine (HLS/DASH playback), and the FFmpeg-based
  transcoder in `cmd/transcoder` for adaptive bitrates.
- **Stateful storage** – Postgres stores users and channels, Redis handles chat fan-out and rate limiting, and local volumes keep
  recordings and transcoder data.
- **Ready-to-run tooling** – `cmd/bitriver` builds images, seeds the admin account, and keeps all configuration in a
  single `.env` file (wrappers live under `scripts/`).

## Quickstart: installer-first (recommended)

Use the packaged launcher (`bitriver-live`) if you just want to bring the stack up with Docker Compose. It bundles the compose file, copies `deploy/.env.example` into place on the first run, checks Docker/Compose, pulls release images, and starts the stack without requiring Go or Node.

- **macOS (Homebrew):**
  ```bash
  brew install --formula https://github.com/bitriver-live/bitriver-live/releases/latest/download/bitriver-live.rb
  bitriver-live
  ```
- **Linux (Deb/RPM):** Download the `.deb` or `.rpm` from the latest [releases](https://github.com/bitriver-live/bitriver-live/releases), install it with `sudo dpkg -i` or `sudo rpm -i`, then run `bitriver-live` from your shell or the desktop shortcut named **Start BitRiver Live**.
- **Windows:** Install the MSI from the latest release. The installer drops `bitriver.exe` and `bitriver-live.ps1` into `Program Files\BitRiver Live` and adds **Start BitRiver Live** shortcuts to the Start menu and desktop.

The launcher keeps using the Compose bundle under `/usr/local/share/bitriver-live` (macOS/Linux) or `Program Files\BitRiver Live` (Windows). Edit the generated `.env` there before restarting the service.

### Desktop control panel (default for operators)

The installer now ships a lightweight desktop/tray control panel so non-developers can manage Docker Compose without memorising commands. Launch it from the Start menu/Applications folder or from the shell:

- **macOS/Linux:** `bitriver-live ui` (or `./scripts/bitriver-live-wrapper.sh ui` if you are running from a cloned repo)
- **Windows:** `bitriver-live.ps1 -Command ui`

The panel polls `docker compose ps` to show service state + health, tails recent logs, and exposes Start/Stop/Restart/Refresh logs buttons that shell out to Compose. Use the **Open health dashboard** link to jump back into the control centre overview when troubleshooting. Contributors can still call the CLI directly (`bitriver-live start`, `bitriver-live stop`, or `go run ./cmd/bitriver desktop --compose-file deploy/docker-compose.yml`) when they prefer the terminal.

## Advanced quickstart: Go CLI from source

Prefer the source-based workflow when developing features or rebuilding images locally. The Go CLI in `cmd/bitriver` handles environment generation, Docker Compose orchestration, and health checks. Use it directly or fall back to `scripts/quickstart.sh` / `scripts/quickstart.ps1` if your shell cannot run Go.

### Prerequisites at a glance

| Platform | Docker runtime | Notes |
| --- | --- | --- |
| macOS 12+ | Docker Desktop with Compose V2 enabled | Start Docker Desktop first and keep at least 15GB free on Docker's data root. |
| Ubuntu 22.04+ / other Linux | Docker Engine + Compose plugin | Add your user to the `docker` group (or prefix commands with `sudo`) and confirm `docker compose` works without root. |
| Windows 10/11 | Docker Desktop (WSL 2 backend) | Enable the WSL 2 backend, start Docker Desktop, and ensure the `docker-desktop` data disk has 15GB free. |

Install Go 1.21+ if you plan to run the source-based quickstart (`go run ./cmd/bitriver quickstart`) or any of the CLI helper
commands. The installer-backed launcher bundles everything it needs and does not require Go.

Both `go run ./cmd/bitriver quickstart` and `./scripts/quickstart.sh` start by running the built-in doctor checks. If Docker is missing from your `PATH`, the command prints a `BitRiver Live doctor` block that includes `Docker: not found` and exits with `Error: doctor checks failed` so you can fix the prerequisite before it touches Compose.

### macOS (Docker Desktop, zsh/bash)

```bash
cd BitRiver-Live
go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml
```

### Ubuntu 22.04+ (Docker Engine + Compose plugin)

```bash
cd BitRiver-Live
go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml
```

Add `sudo` if your user is not in the `docker` group. The CLI will confirm prerequisites before touching Docker.

### Windows 10/11 (Docker Desktop + PowerShell)

Run from a PowerShell prompt with Docker Desktop running and the WSL 2 backend enabled:

```powershell
Set-Location BitRiver-Live
go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml
```

The CLI checks Docker/Compose, generates `.env` from `deploy/.env.example` with strong credentials when the file is missing, renders `deploy/ome/Server.generated.xml` via the Go binary (no Python dependency), builds API/viewer/SRS controller/transcoder images, runs migrations, and prints the seeded admin login. The `.env` file is gitignored; `go run ./cmd/bitriver env init` and the quickstart scripts will create it and mint fresh credentials the first time you run them so new clones never share secrets. Re-run the commands any time `.env` or templates change; they are idempotent.

### Step 3 – Use the running stack

1. Open [http://localhost:8080/signup](http://localhost:8080/signup) in your browser and sign in with the admin credentials
   printed by the CLI, then change the password under **Settings → Security**.
2. Check the **System status** card on the control centre overview at [http://localhost:8080](http://localhost:8080). It pulls
   from `/api/status` to merge readiness, database/Redis checks, and ingest probes (SRS/OME/transcoder) with remediation tips
   and copy-to-clipboard log commands—no `curl` required.
3. Visit [http://localhost:8080/viewer](http://localhost:8080/viewer) in another tab to see the public viewer that proxies
   through the API.
4. Point OBS or any RTMP encoder at `rtmp://localhost:1935/live` with the stream key shown in the control centre and watch the
   broadcast arrive in the viewer within seconds.

### Step 4 – Start, stop, and troubleshoot

Run these commands from the repository root (where `.env` lives). The Go CLI wraps Docker Compose so you do not need to export `COMPOSE_FILE`:

```bash
# Show container status
go run ./cmd/bitriver compose up --file deploy/docker-compose.yml
docker compose ps

# Follow logs for everything
docker compose logs -f

# Stop the stack but keep data
go run ./cmd/bitriver compose down --file deploy/docker-compose.yml

# Restart after editing .env
go run ./cmd/bitriver compose up --file deploy/docker-compose.yml

# Refresh the OME config when .env changes
go run ./cmd/bitriver ome render --force --env-file ./.env
```

If ports are already in use, edit the matching values in `.env` (for example `BITRIVER_LIVE_PORT=9090`), save the file, and rerun
`docker compose up -d`. See [`docs/quickstart.md`](docs/quickstart.md) for extra tips, common errors, and guidance on updating
the generated environment file before inviting real users.

### Key environment variables at a glance

Set `BITRIVER_LIVE_MODE=production` in `.env` (the default). `deploy/check-env.sh` now fails fast when the mode is empty or still set to `development` so the server always boots with production guardrails. For local HTTP-only demos, leave `.env` at production and override the variable inline (for example, `BITRIVER_LIVE_MODE=development docker compose --env-file ./.env -f deploy/docker-compose.yml up -d`) instead of committing the change to your deployment file.

The CLI pre-populates `.env` so Docker Compose can bind each service to predictable ports. Edit the values below to match your host and network before rerunning `docker compose up -d`.

| Variable | Default | What it controls |
| --- | --- | --- |
| `BITRIVER_LIVE_MODE` | `production` | Required runtime mode (`production` for deployments; override inline for one-off local demos). `deploy/check-env.sh` fails when this is unset or set to `development` so `/metrics` protection and production hardening are never skipped by accident. |
| `BITRIVER_LIVE_PORT` | `8080` | Host port for the Go API and proxied viewer (`deploy/docker-compose.yml` maps host `8080` to container `8080`). |
| `BITRIVER_SRS_RTMP_PORT` | `1935` | Host RTMP ingest port forwarded to the SRS container (`1935`). |
| `BITRIVER_SRS_API_PORT` | `1985` | Host port for the SRS HTTP API when the optional `srs-api` Compose profile is enabled (not exposed by default). |
| `BITRIVER_SRS_CONTROLLER_PORT` | `1986` | Host port for the SRS controller’s HTTP API (container listens on `1985`). |
| `BITRIVER_OME_HTTP_PORT` | `8081` | Host port for the OvenMediaEngine control plane and health checks. |
| `BITRIVER_OME_SIGNALLING_PORT` | `9000` | Host port for OME WebRTC signalling; defaults to `BITRIVER_OME_SERVER_PORT` when left unset so host/container bindings stay aligned. |
| `BITRIVER_OME_SERVER_PORT` | `9000` | OME WebRTC signalling port rendered into `Server.xml`; Docker maps `BITRIVER_OME_SIGNALLING_PORT` on the host to this container port. |
| `BITRIVER_OME_SERVER_TLS_PORT` | `9443` | OME TLS signalling port rendered into `Server.xml` and exposed on the same host/container port. |
| `BITRIVER_OME_BIND` | `0.0.0.0` | Listener address injected into the generated OME `Server.xml` control listener `<Bind>`/`<IP>` fields and the root `<Bind><Address>` entry. |
| `BITRIVER_OME_IP` | `0.0.0.0` | Public IP advertised in the top-level `<Server><IP>` block (defaults to `BITRIVER_OME_BIND`). |
| `BITRIVER_LIVE_POSTGRES_DSN` | `postgres://bitriver:bitriver@postgres:5432/bitriver?sslmode=disable` | Connection string the API uses for its primary database. Combine with `BITRIVER_POSTGRES_HOST_PORT` (default `5432`, profile `postgres-host`) to publish Postgres to the host. |
| `BITRIVER_LIVE_METRICS_TOKEN` | `metrics-collector-token` | Bearer token required to scrape `/metrics`; production mode refuses to start without this or `BITRIVER_LIVE_METRICS_ALLOW_NETWORKS`. Pair with `BITRIVER_LIVE_METRICS_ALLOW_NETWORKS` (comma-separated CIDRs/IPs) to restrict scrapes to trusted networks. |
| `BITRIVER_LIVE_RATE_LOGIN_LIMIT` | `10` | Required in production. Caps login attempts per IP within the configured window; raise cautiously. Override `BITRIVER_LIVE_RATE_LOGIN_WINDOW` (default `1m`) to stretch the window if you need fewer attempts over more time. |
| `BITRIVER_LIVE_CHAT_QUEUE_REDIS_ADDR` | `redis:6379` | Redis endpoint for chat fan-out; update alongside `BITRIVER_REDIS_PASSWORD` if you run Redis outside Compose. |

The bundled Compose Postgres defaults to `sslmode=disable` because the container does not ship with TLS certificates. For external or managed databases, require TLS by setting `sslmode=require` or `sslmode=verify-full` on `BITRIVER_LIVE_POSTGRES_DSN` and `BITRIVER_LIVE_SESSION_POSTGRES_DSN`. When Postgres uses a private CA, mount the certificate into the Compose bundle (for example, `./certs/postgres-ca.pem`) and append `sslrootcert=/certs/postgres-ca.pem` to the DSNs; the validators refuse `sslmode=disable` unless the DSN points at the local Compose `postgres` service.

In production mode you must set `BITRIVER_LIVE_METRICS_TOKEN` or
`BITRIVER_LIVE_METRICS_ALLOW_NETWORKS` to keep the `/metrics` endpoint
protected; the server will fail fast when both are empty. Production also
requires non-zero login throttling via `BITRIVER_LIVE_RATE_LOGIN_LIMIT`
so password spray protection is never skipped.

Common tweaks:

- **Change host ports:** Adjust the `*_PORT` values above (for example, move the API to `BITRIVER_LIVE_PORT=9090` or RTMP ingest to `BITRIVER_SRS_RTMP_PORT=1936`) and rerun `docker compose up -d`. Leave `BITRIVER_OME_SIGNALLING_PORT` empty when you want the host binding to track `BITRIVER_OME_SERVER_PORT`; set it explicitly only when the host port must differ from the value baked into `Server.xml`.
- **Expose the SRS HTTP API for debugging:** Run `docker compose --profile srs-api up -d` (or `BITRIVER_COMPOSE_PROFILES=srs-api docker compose up -d`) to publish `BITRIVER_SRS_API_PORT` on the host. Keep the profile disabled in production and never expose the port to the public internet.
- **Enable TLS on the API/viewer:** The setup wizard and installers now stage certificates automatically. Provide your certificate and key paths and they will be copied into `deploy/certs/` (Compose) or `<install-dir>/certs` (systemd), update `BITRIVER_LIVE_TLS_CERT`/`BITRIVER_LIVE_TLS_KEY` in `.env`, and restart the service so the API listens with HTTPS.
- **Lock down viewer origins:** Point `BITRIVER_VIEWER_ORIGIN` and `NEXT_PUBLIC_VIEWER_URL` at your public domain or reverse proxy to align CORS and cookie scope.
- **Control session lifetimes:** Set `BITRIVER_LIVE_SESSION_TTL` in `.env` to cap absolute session length (for example, `168h` for seven days) and optionally add `BITRIVER_LIVE_SESSION_IDLE_TIMEOUT` to expire idle sessions sooner. Rerun `docker compose up -d` so the API picks up the new values.
- **Keep OvenMediaEngine credentials in sync:** Whenever you edit `BITRIVER_OME_USERNAME`, `BITRIVER_OME_PASSWORD`, `BITRIVER_OME_API_TOKEN`, `BITRIVER_OME_BIND`, or `BITRIVER_OME_IP` in `.env`, rerun `go run ./cmd/bitriver ome render --force --env-file ./.env` (or the `./scripts/render-ome-config.sh` wrapper) to regenerate `deploy/ome/Server.generated.xml`. The Go renderer overwrites the generated file on every invocation, and the CLI calls it automatically so template changes from `git pull` land before Compose starts. The pinned OME tag (default `0.16.0`) expects a non-empty managers API token; `BITRIVER_OME_ACCESS_TOKEN` mirrors it for health probes unless you override it. The compose bundle still runs a lightweight `ome-config` preflight before starting `ome`; it will fail fast if the generated file is missing, so fix and re-render before retrying `docker compose up -d`.

Find deeper explanations and additional variables (rate limiting, transcoder public URLs, external Redis/Postgres) in [`docs/quickstart.md`](docs/quickstart.md).

### Appendix: Health endpoints (advanced)

The dashboard’s **System status** card is the primary health check for operators. For automation or low-level debugging:

- `/api/status` returns the aggregated health payload used by the dashboard, including remediation hints and log suggestions.
- `/readyz` reports core dependency readiness (database, session store, rate limiting) for load balancers.
- `/healthz` adds ingest details to `/readyz` but keeps HTTP 200 unless core dependencies are down.

## Need more control?

- **Tweak settings:** Edit `.env` to change domain names, exposed ports, Redis/Postgres credentials, or viewer origins, then run
  `docker compose up -d` again to apply the changes.
- **Install TLS / go beyond one host:** Follow [`docs/advanced-deployments.md`](docs/advanced-deployments.md).
- **Run backups + restores:** Use the operations runbook in [`docs/operations.md`](docs/operations.md) for Postgres, Redis,
  and transcoder/recordings data.
- **Plan upgrades:** Follow [`docs/upgrades.md`](docs/upgrades.md#upgrade-essentials-migrations-env-updates-and-ome-re-render) for the safe Compose upgrade flow, migration timing, and `.env`/OME template handling.
- **Understand every service:** Read [`docs/production-release.md`](docs/production-release.md) and the release notes under
  `docs/releases/` when preparing a launch.
- **Plan for portability:** Review [`docs/cross-platform-plan.md`](docs/cross-platform-plan.md) for the current platform
  assumptions and how we will converge on a Go-based control plane.
- **AI-assisted edits:** Use the [Codex CLI guide](docs/codex-cli.md) to install the CLI, authenticate, and point it at this repository.

## Manual development workflow (optional)

You only need the steps below if you want to hack on the Go code without Docker Compose.

### Option A – JSON datastore (fastest, single process)

```bash
mkdir -p data
BITRIVER_LIVE_MODE=development \
  go run -tags postgres ./cmd/server \
    --storage-driver json \
    --data data/store.json
```

Keep the server running, open [http://localhost:8080](http://localhost:8080), and seed an admin with:

```bash
go run -tags postgres ./cmd/tools/bootstrap-admin \
  --json data/store.json \
  --email you@example.com \
  --name "Your Name" \
  --password "temporary-password"
```

### Option B – Local Postgres + Redis

```bash
# Start databases
docker run --rm --name bitriver-postgres \
  -e POSTGRES_PASSWORD=bitriver \
  -e POSTGRES_USER=bitriver \
  -e POSTGRES_DB=bitriver_live \
  -p 5432:5432 postgres:15-alpine &

docker run --rm --name bitriver-redis -p 6379:6379 redis:7-alpine &

# Apply migrations
for file in deploy/migrations/*.sql; do \
  psql "postgres://bitriver:bitriver@127.0.0.1:5432/bitriver_live?sslmode=disable" -f "$file"; \
done

# Run the API
BITRIVER_LIVE_MODE=development \
BITRIVER_LIVE_POSTGRES_DSN="postgres://bitriver:bitriver@127.0.0.1:5432/bitriver_live?sslmode=disable" \
BITRIVER_LIVE_SESSION_STORE=postgres \
BITRIVER_LIVE_SESSION_POSTGRES_DSN="postgres://bitriver:bitriver@127.0.0.1:5432/bitriver_live?sslmode=disable" \
go run -tags postgres ./cmd/server \
  --mode development \
  --chat-queue-driver redis \
  --chat-queue-redis-addr 127.0.0.1:6379
```

Seed an admin via Postgres:

```bash
go run -tags postgres ./cmd/tools/bootstrap-admin \
  --postgres-dsn "postgres://bitriver:bitriver@127.0.0.1:5432/bitriver_live?sslmode=disable" \
  --email you@example.com \
  --name "Your Name" \
  --password "temporary-password"
```

Session tokens stored via the Postgres session backend are hashed (SHA-256) on
write so bearer tokens are not exposed to database operators. Apply the latest
`deploy/migrations` SQL files (including
`0004_auth_session_hashes.sql`) before booting the API to ensure the hashed
column exists.

## Architecture at a glance

```mermaid
flowchart LR
    Creator((OBS / Encoder)) -->|RTMP| SRS
    SRS -->|live media| Transcoder
    Transcoder -->|HLS segments| OME[OvenMediaEngine]
    Viewer[Next.js Viewer] -->|HTTPS /viewer| API
    Admin[Control Centre SPA] -->|HTTPS| API
    API[(Go API)] -->|REST| Postgres
    API -->|chat + sessions| Redis
    API -->|stream control| SRS
    API -->|packaging control| OME
    API -->|jobs| Transcoder
```

## Repository layout

| Path | Purpose |
| --- | --- |
| `cmd/server` | Go HTTP API and control centre binary |
| `cmd/transcoder` | FFmpeg job controller used by Docker and advanced deployments |
| `cmd/tools` | Helper CLIs (for example, `bootstrap-admin`) |
| [`deploy/`](deploy/README.md) | Docker Compose stack, systemd units, and SQL migrations |
| `docs/` | Guides for installs, scaling, releases, and troubleshooting |
| `internal/` | API handlers, chat, ingest orchestration, storage, auth |
| `web/static` | Embedded admin UI assets served by the Go binary |
| `web/viewer` | Public Next.js viewer |

## Tests

```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s
```

See [`docs/testing.md`](docs/testing.md) for suite-specific instructions and
[`docs/testing-status.md`](docs/testing-status.md) for the latest reliability
notes.

Need Postgres-backed tests? Use the helper:

```bash
./scripts/test-postgres.sh ./internal/storage/...
```

The script keeps module access offline (`GOPROXY=off GOSUMDB=off` with
`-mod=vendor`) to preserve vendored replacements and avoid touching
`go.mod`/`go.sum`.

Questions or improvements? Open an issue or explore `internal/api/handlers.go` to start extending the platform.
