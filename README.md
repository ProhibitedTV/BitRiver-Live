# BitRiver Live

BitRiver Live is a self-hosted streaming stack built with Go on the backend and Next.js on the frontend. One Docker command
starts the API, control centre, public viewer, RTMP ingest, transcoder, chat, analytics, Postgres, and Redis so you can run a
Twitch-style experience on hardware you control.

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

## Quickstart: copy, paste, and get a running demo

The steps below assume Ubuntu 22.04+ with an account that can run `sudo`. Other operating systems can use Docker Desktop with
Compose V2 enabled. Every command is ready to paste into a terminal.

### Step 0 – Install Docker Engine and Docker Compose V2

```bash
sudo apt update
sudo apt install -y ca-certificates curl gnupg lsb-release
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo usermod -aG docker $USER
newgrp docker

# Confirm everything works without sudo
docker --version
docker compose version
```

> **Tip:** If you prefer running Docker with `sudo`, skip the `usermod` and `newgrp` lines and prefix the later commands with
> `sudo`. Using the Docker group keeps file ownership simple.

### Step 1 – Download BitRiver Live

```bash
git clone https://github.com/ProhibitedTV/BitRiver-Live.git
cd BitRiver-Live
```

### Step 2 – Let the quickstart script do the heavy lifting

```bash
./scripts/quickstart.sh
```

The helper script will:

1. Check that Docker and `docker compose` are available.
2. Create (or update) a `.env` file with sensible defaults and strong random passwords.
3. Build the Go API, viewer, SRS controller, and transcoder Docker images locally, so no registry login is needed.
4. Launch Postgres, Redis, SRS, OvenMediaEngine, the transcoder, the API, and the viewer using `deploy/docker-compose.yml`.
5. Run the SQL migrations and seed an admin user, then print the admin email and password in your terminal.

 The helper also regenerates `deploy/ome/Server.generated.xml` from the bundled template, applying `BITRIVER_OME_BIND` (default
 `0.0.0.0`) to the `<Bind>` and `<IP>` fields required by current OME images and wiring in the OME credentials from `.env` so rerunning the quickstart keeps the control service healthy.

If the script exits with an error, re-run it after fixing the reported problem. You can always re-run the script to rebuild the
stack or refresh credentials.

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
```

If ports are already in use, edit the matching values in `.env` (for example `BITRIVER_LIVE_PORT=9090`), save the file, and rerun
`docker compose up -d`. See [`docs/quickstart.md`](docs/quickstart.md) for extra tips, common errors, and guidance on updating
the generated environment file before inviting real users.

## Need more control?

- **Tweak settings:** Edit `.env` to change domain names, exposed ports, Redis/Postgres credentials, or viewer origins, then run
  `docker compose up -d` again to apply the changes.
- **Install TLS / go beyond one host:** Follow [`docs/advanced-deployments.md`](docs/advanced-deployments.md).
- **Understand every service:** Read [`docs/production-release.md`](docs/production-release.md) and the release notes under
  `docs/releases/` when preparing a launch.
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
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=10s
```

See [`docs/testing.md`](docs/testing.md) for suite-specific instructions and
[`docs/testing-status.md`](docs/testing-status.md) for the latest reliability
notes.

Need Postgres-backed tests? Use the helper:

```bash
./scripts/test-postgres.sh ./internal/storage/...
```

The script keeps module access offline (`GOPROXY=off GOSUMDB=off` with
`-mod=readonly`) to preserve vendored replacements and avoid touching
`go.mod`/`go.sum`.

Questions or improvements? Open an issue or explore `internal/api/handlers.go` to start extending the platform.
