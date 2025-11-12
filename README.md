# BitRiver Live

BitRiver Live is a self-hostable streaming platform built with Go on the backend and Next.js on the frontend. It bundles the API, admin control centre, public viewer, RTMP ingest, HLS packaging, chat, analytics, and FFmpeg-based transcoding so you can run a Twitch-like experience on hardware you control.

## Why BitRiver Live?

- **Go API + Control Centre** – `cmd/server` serves a REST API, real-time chat, analytics, and the bundled admin single-page app under `/`.
- **Public viewer** – `web/viewer` is a Next.js site that proxies through the Go API (or runs independently) so fans can browse channels and watch streams.
- **Streaming pipeline** – Docker compose wires SRS for RTMP ingest, OvenMediaEngine for HLS/DASH playback, and the FFmpeg job controller in `cmd/transcoder` for adaptive bitrates.
- **Stateful storage** – Postgres stores users, channels, recordings, and chat history; Redis powers chat fan-out and login throttling. A JSON datastore is available for quick demos.
- **Batteries included tooling** – Helper binaries in `cmd/tools`, migrations under `deploy/migrations`, and docs in `docs/` keep day-two operations predictable.

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

The Go server also proxies `/viewer` to the Next.js runtime when you run the viewer separately.

## Quick start (Docker, one command)

1. Install [Docker Engine](https://docs.docker.com/engine/install/) and ensure the Compose v2 plugin is available (`docker compose version`).
2. Clone the repository and switch into it:
   ```bash
   git clone https://github.com/BitRiver-Live/BitRiver-Live.git
   cd BitRiver-Live
   ```
3. Launch the stack with the helper script:
   ```bash
   ./scripts/quickstart.sh
   ```

The script builds the Go API and transcoder images, brings up Postgres, Redis, SRS, OvenMediaEngine, the viewer, and the API itself, then applies migrations and seeds an admin account. It prints the admin credentials at the end—sign in immediately and change the password from **Settings → Security**.

Update the generated `.env` file to match your domain, change the seeded credentials, and rerun `docker compose up -d` whenever you tweak environment variables. For troubleshooting tips and common follow-up commands, see [`docs/quickstart.md`](docs/quickstart.md).

### Configure Docker Compose credentials

When deploying from `deploy/docker-compose.yml`, populate a dedicated environment file so the stack starts with unique secrets:

1. Copy `deploy/.env.example` to `.env` at the repository root.
2. Replace the placeholder values for:
   - `BITRIVER_LIVE_ADMIN_EMAIL` and `BITRIVER_LIVE_ADMIN_PASSWORD`
   - `BITRIVER_SRS_TOKEN`
   - `BITRIVER_OME_USERNAME` and `BITRIVER_OME_PASSWORD`
   - `BITRIVER_TRANSCODER_TOKEN`
3. Run `deploy/check-env.sh` to confirm the file no longer contains defaults. The script exits non-zero until each credential is customised. Docker Compose also fails fast thanks to the required-variable checks baked into the manifest.

Keep the filled `.env` file private—it is `.gitignore`'d and should never be committed to version control.

## Manual development workflow

Want to hack on the Go API without Docker? Use the JSON datastore for a fast start, then graduate to Postgres when you need persistence guarantees.

### Option A – JSON datastore (no external services)

```bash
# From the repository root
mkdir -p data
BITRIVER_LIVE_MODE=development \
  go run ./cmd/server \
    --storage-driver json \
    --data data/store.json
```

1. Keep the server running and open [http://localhost:8080](http://localhost:8080).
2. Sign up your first user, then promote it to admin:
   ```bash
   go run ./cmd/tools/bootstrap-admin \
     --json data/store.json \
     --email you@example.com \
     --name "Your Name" \
     --password "temporary-password"
   ```
3. Restart the server (if needed) and sign in with the seeded credentials.

### Option B – Postgres + Redis

1. Start Postgres and Redis however you like. The quickest option is Docker:
   ```bash
   docker run --rm --name bitriver-postgres \
     -e POSTGRES_PASSWORD=bitriver \
     -e POSTGRES_USER=bitriver \
     -e POSTGRES_DB=bitriver_live \
     -p 5432:5432 postgres:15-alpine

   docker run --rm --name bitriver-redis -p 6379:6379 redis:7-alpine
   ```
2. Apply the migrations in `deploy/migrations/`:
   ```bash
   for file in deploy/migrations/*.sql; do
     psql "postgres://bitriver:bitriver@127.0.0.1:5432/bitriver_live?sslmode=disable" -f "$file"
   done
   ```
3. Start the Go server with Postgres storage:
   ```bash
   BITRIVER_LIVE_MODE=development \
   BITRIVER_LIVE_POSTGRES_DSN="postgres://bitriver:bitriver@127.0.0.1:5432/bitriver_live?sslmode=disable" \
   BITRIVER_LIVE_SESSION_STORE=postgres \
   BITRIVER_LIVE_SESSION_POSTGRES_DSN="postgres://bitriver:bitriver@127.0.0.1:5432/bitriver_live?sslmode=disable" \
   go run ./cmd/server \
     --mode development \
     --chat-queue-driver redis \
     --chat-queue-redis-addr 127.0.0.1:6379
   ```
4. Seed an admin with the Postgres helper:
   ```bash
   go run ./cmd/tools/bootstrap-admin \
     --postgres-dsn "postgres://bitriver:bitriver@127.0.0.1:5432/bitriver_live?sslmode=disable" \
     --email you@example.com \
     --name "Your Name" \
     --password "temporary-password"
   ```

### Viewer development

The Go server proxies `/viewer` when `BITRIVER_VIEWER_ORIGIN` is set, or you can run the Next.js app yourself:

```bash
cd web/viewer
npm install
NEXT_PUBLIC_API_BASE_URL="http://localhost:8080" npm run dev
```

Browse to [http://localhost:3000](http://localhost:3000) for hot reload while the Go API keeps serving `/`.

## Running tests

```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=10s
```

For storage-specific tests backed by Postgres, use the helper:

```bash
./scripts/test-postgres.sh ./internal/storage/...
```

## Repository layout

| Path | Purpose |
| --- | --- |
| `cmd/server` | Go HTTP API and control centre binary |
| `cmd/transcoder` | FFmpeg job controller used by Docker and advanced deployments |
| `cmd/tools` | Helper CLIs (for example, `bootstrap-admin`) |
| `deploy/` | Docker Compose stack, systemd units, and SQL migrations |
| `docs/` | Additional guides covering installs, testing, scaling, and roadmap |
| `internal/` | Application packages (API handlers, chat, ingest orchestration, storage, auth) |
| `web/static` | Embedded admin UI assets served by the Go binary |
| `web/viewer` | Public Next.js viewer |

## Next steps

- Read the [quickstart guide](docs/quickstart.md) for Docker usage tips.
- Head to [`docs/advanced-deployments.md`](docs/advanced-deployments.md) for TLS, object storage, and scaling recipes.
- Explore [`docs/testing.md`](docs/testing.md) when you need integration tests or linting details.
- Learn about the long-term roadmap in [`docs/product-roadmap.md`](docs/product-roadmap.md).

Questions or improvements? Open an issue or dive into `internal/api/handlers.go` to start extending the platform.
