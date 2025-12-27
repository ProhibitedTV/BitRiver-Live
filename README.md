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
- **Ready-to-run tooling** – `scripts/quickstart.sh` builds images, seeds the admin account, and keeps all configuration in a
  single `.env` file.

## Quickstart: Go CLI first

Use the Go-based quickstart command to build images, seed credentials, and launch the Compose stack. The legacy shell (`scripts/quickstart.sh`) and PowerShell (`scripts/quickstart.ps1`) helpers remain as thin shims for environments that cannot run the Go CLI directly.

### Prerequisites at a glance

| Platform | Docker runtime | Notes |
| --- | --- | --- |
| macOS 12+ | Docker Desktop with Compose V2 enabled | Start Docker Desktop first and keep at least 15GB free on Docker's data root. |
| Ubuntu 22.04+ / other Linux | Docker Engine + Compose plugin | Add your user to the `docker` group (or prefix commands with `sudo`) and confirm `docker compose` works without root. |
| Windows 11 (WSL 2 backend) | Docker Desktop | Run the quickstart inside WSL with the WSL 2 backend enabled; verify the `docker-desktop` data disk has 15GB free. |
| Windows 11 (native PowerShell) | Docker Desktop (WSL 2 backend) | Same Docker Desktop install as above; PowerShell shells forward to the same CLI. |

### Run the Go quickstart

```bash
# macOS, Linux, or Windows via WSL/bash
go run ./cmd/quickstart --compose-file deploy/docker-compose.yml

# Windows PowerShell (same CLI, different shell)
pwsh -c "go run ./cmd/quickstart --compose-file deploy/docker-compose.yml"
```

The command checks Docker/Compose availability, validates disk space, generates `.env` with strong credentials, renders `deploy/ome/Server.generated.xml`, builds the API/viewer/SRS controller/transcoder images locally, runs migrations, and prints the seeded admin login. Re-run it any time you update `.env` or pull new code to rebuild images and refresh configs.

> Need a wrapper for a managed shell? Call `./scripts/quickstart.sh` from POSIX shells or `./scripts/quickstart.ps1` from PowerShell—they simply forward to the Go quickstart while handling shell-specific permission quirks.

If the command exits with an error, fix the reported problem and rerun it; runs are idempotent and safe to repeat when credentials or templates change.

### Step 3 – Use the running stack

1. Check that the API is ready:
   ```bash
   curl http://localhost:8080/readyz
   ```
   The endpoint returns HTTP 200 when core dependencies (database, sessions, rate limiting) are available.
   To inspect ingest dependencies, call `/healthz` as well:
   ```bash
   curl http://localhost:8080/healthz
   ```
   The JSON payload reports `status: "degraded"` when SRS/OME/transcoder probes fail, but the HTTP status will stay 200 unless
   core services are unavailable.
2. Open [http://localhost:8080/signup](http://localhost:8080/signup) in your browser and sign in with the admin credentials
   printed by the script, then change the password under **Settings → Security**.
3. Visit [http://localhost:8080/viewer](http://localhost:8080/viewer) in another tab to see the public viewer that proxies
   through the API.
4. Point OBS or any RTMP encoder at `rtmp://localhost:1935/live` with the stream key shown in the control centre and watch the
   broadcast arrive in the viewer within seconds.

### Step 4 – Start, stop, and troubleshoot

Run these commands from the repository root (where `.env` lives). Set the compose path once so Docker finds `deploy/docker-comp
ose.yml`, then use the usual subcommands:

```bash
export COMPOSE_FILE=deploy/docker-compose.yml

# Show container status
docker compose ps

# Follow logs for everything
docker compose logs -f

# Stop the stack but keep data
docker compose down

# Restart after editing .env
docker compose up -d

# Refresh the OME config when .env changes
./scripts/render-ome-config.sh
```

If ports are already in use, edit the matching values in `.env` (for example `BITRIVER_LIVE_PORT=9090`), save the file, and rerun
`docker compose up -d`. See [`docs/quickstart.md`](docs/quickstart.md) for extra tips, common errors, and guidance on updating
the generated environment file before inviting real users.

### Key environment variables at a glance

The quickstart script pre-populates `.env` so Docker Compose can bind each service to predictable ports. Edit the values below to match your host and network before rerunning `docker compose up -d`.

| Variable | Default | What it controls |
| --- | --- | --- |
| `BITRIVER_LIVE_PORT` | `8080` | Host port for the Go API and proxied viewer (`deploy/docker-compose.yml` maps host `8080` to container `8080`). |
| `BITRIVER_SRS_RTMP_PORT` | `1935` | Host RTMP ingest port forwarded to the SRS container (`1935`). |
| `BITRIVER_SRS_API_PORT` | `1985` | Host port for the SRS API, used by the controller and health checks. |
| `BITRIVER_SRS_CONTROLLER_PORT` | `1986` | Host port for the SRS controller’s HTTP API (container listens on `1985`). |
| `BITRIVER_OME_HTTP_PORT` | `8081` | Host port for the OvenMediaEngine control plane and health checks. |
| `BITRIVER_OME_SIGNALLING_PORT` | `9000` | Host port for OME WebRTC signalling; defaults to `BITRIVER_OME_SERVER_PORT` when left unset so host/container bindings stay aligned. |
| `BITRIVER_OME_SERVER_PORT` | `9000` | OME WebRTC signalling port rendered into `Server.xml`; Docker maps `BITRIVER_OME_SIGNALLING_PORT` on the host to this container port. |
| `BITRIVER_OME_SERVER_TLS_PORT` | `9443` | OME TLS signalling port rendered into `Server.xml` and exposed on the same host/container port. |
| `BITRIVER_OME_BIND` | `0.0.0.0` | Listener address injected into the generated OME `Server.xml` control listener `<Bind>`/`<IP>` fields and the root `<Bind><Address>` entry. |
| `BITRIVER_OME_IP` | `0.0.0.0` | Public IP advertised in the top-level `<Server><IP>` block (defaults to `BITRIVER_OME_BIND`). |
| `BITRIVER_LIVE_POSTGRES_DSN` | `postgres://bitriver:bitriver@postgres:5432/bitriver?sslmode=disable` | Connection string the API uses for its primary database. Combine with `BITRIVER_POSTGRES_HOST_PORT` (default `5432`, profile `postgres-host`) to publish Postgres to the host. |
| `BITRIVER_LIVE_CHAT_QUEUE_REDIS_ADDR` | `redis:6379` | Redis endpoint for chat fan-out; update alongside `BITRIVER_REDIS_PASSWORD` if you run Redis outside Compose. |

Common tweaks:

- **Change host ports:** Adjust the `*_PORT` values above (for example, move the API to `BITRIVER_LIVE_PORT=9090` or RTMP ingest to `BITRIVER_SRS_RTMP_PORT=1936`) and rerun `docker compose up -d`. Leave `BITRIVER_OME_SIGNALLING_PORT` empty when you want the host binding to track `BITRIVER_OME_SERVER_PORT`; set it explicitly only when the host port must differ from the value baked into `Server.xml`.
- **Enable TLS on the API/viewer:** Mount certs under `./certs` and set `BITRIVER_LIVE_TLS_CERT`/`BITRIVER_LIVE_TLS_KEY`, then restart so the API listens with HTTPS.
- **Lock down viewer origins:** Point `BITRIVER_VIEWER_ORIGIN` and `NEXT_PUBLIC_VIEWER_URL` at your public domain or reverse proxy to align CORS and cookie scope.
- **Control session lifetimes:** Set `BITRIVER_LIVE_SESSION_TTL` in `.env` to cap absolute session length (for example, `168h` for seven days) and optionally add `BITRIVER_LIVE_SESSION_IDLE_TIMEOUT` to expire idle sessions sooner. Rerun `docker compose up -d` so the API picks up the new values.
- **Keep OvenMediaEngine credentials in sync:** Whenever you edit `BITRIVER_OME_USERNAME`, `BITRIVER_OME_PASSWORD`, `BITRIVER_OME_API_TOKEN`, `BITRIVER_OME_BIND`, or `BITRIVER_OME_IP` in `.env`, rerun `./scripts/render-ome-config.sh` to regenerate `deploy/ome/Server.generated.xml`. The renderer overwrites the generated file on every invocation, and the quickstart script calls it automatically so template changes from `git pull` land before Compose starts. The pinned OME tag (default `0.16.0`) expects a non-empty managers API token; `BITRIVER_OME_ACCESS_TOKEN` mirrors it for health probes unless you override it. The compose bundle still runs a lightweight `ome-config` preflight before starting `ome`; it will fail fast if the generated file is missing, so fix and re-render before retrying `docker compose up -d`.

Find deeper explanations and additional variables (rate limiting, transcoder public URLs, external Redis/Postgres) in [`docs/quickstart.md`](docs/quickstart.md).

## Need more control?

- **Tweak settings:** Edit `.env` to change domain names, exposed ports, Redis/Postgres credentials, or viewer origins, then run
  `docker compose up -d` again to apply the changes.
- **Install TLS / go beyond one host:** Follow [`docs/advanced-deployments.md`](docs/advanced-deployments.md).
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
