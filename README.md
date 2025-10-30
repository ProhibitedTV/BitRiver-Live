# BitRiver-Live

BitRiver Live is a modern, full-stack solution for building your own live streaming platform—similar to Twitch or DLive—that you can operate on your own infrastructure. The project is designed for creators, communities, and developers who want freedom, scalability, and transparency in how live media is delivered.

---

## Set up BitRiver Live at home

BitRiver Live ships with a self-contained Go API and control center so you can explore the product plan—users, channels, stream sessions, and chat messages—without provisioning databases or external services.

### Prerequisites

- Install [Go 1.21+](https://go.dev/doc/install) on your workstation.
- Clone this repository and switch into it:
  ```bash
  git clone https://github.com/BitRiver-Live/BitRiver-Live.git
  cd BitRiver-Live
  ```

### Run the development server

Start the API in development mode from the repository root:

```bash
go run ./cmd/server --mode development
```

Leave the terminal open while the server is running. Browse to [http://localhost:8080](http://localhost:8080) to open the **BitRiver Live Control Center** and sign up your first account.

### Promote your first admin

Roles control which buttons light up inside the control center. The first account you create starts as a regular user, so promote it to `admin` before trying to manage channels or other accounts:

1. Stop the server with `Ctrl+C`.
2. Open `data/store.json` in a text editor.
3. Locate your user entry and add `"admin"` to the `roles` array (for example, `"roles": ["admin"]`).
4. Save the file and restart the server with `go run ./cmd/server --mode development`.
5. Sign back in—channel and user management buttons now work because the account has administrator access.

This one-time edit is required before the control center can issue admin-only API calls or generate access tokens.

### Explore the control center

With an administrator signed in, the web interface lets you:

- Create users, channels, and streamer profiles without touching the command line
- Edit or retire accounts, rotate channel metadata, and keep stream keys handy with one-click copy actions
- Start or stop live sessions, export a JSON snapshot of the state, and review live analytics cards powered by `/api/analytics/overview`
- Seed chat conversations, moderate or remove messages across every channel in one view
- Capture recorded broadcasts automatically when a stream ends, manage retention windows, and surface VOD manifests to viewers
- Curate streamer profiles with featured channels, top friends, and crypto donation links through a guided form
- Generate a turn-key installer script that provisions BitRiver Live as a systemd service on a home server, complete with optional log directories
- Offer a self-service `/signup` experience so viewers can create password-protected accounts on their own

#### Analytics overview

The Control Center surfaces the `/api/analytics/overview` endpoint through a dashboard that blends platform-wide metrics with channel-level detail. The response shape is:

```json
{
  "summary": {
    "liveViewers": 0,
    "streamsLive": 0,
    "watchTimeMinutes": 0,
    "chatMessages": 0
  },
  "perChannel": [
    {
      "channelId": "string",
      "title": "string",
      "liveViewers": 0,
      "followers": 0,
      "avgWatchMinutes": 0,
      "chatMessages": 0
    }
  ]
}
```

`summary.watchTimeMinutes` tracks total viewer minutes over the last 24 hours, `summary.chatMessages` counts messages posted today, and `summary.streamsLive` reports active broadcasts. Each channel entry mirrors those aggregates with the current live audience, average watch time across recorded sessions, follower totals, and messages sent since midnight UTC.

The UI talks directly to the same REST API documented below, so you can always fall back to curl or an API client when you need to automate advanced workflows.

The server also respects the `BITRIVER_LIVE_ADDR`, `BITRIVER_LIVE_MODE`, and `BITRIVER_LIVE_DATA` environment variables if you prefer configuring runtime options without flags. Switch to production-ready defaults by running:

```bash
go run ./cmd/server --mode production --data /var/lib/bitriver-live/store.json
```

In production mode BitRiver Live binds to port 80 by default, letting viewers access the control center without appending a port number to your domain.

To serve HTTPS directly from the Go process provide a certificate/key pair generated via [Let's Encrypt](https://letsencrypt.org/), your reverse proxy, or another certificate authority:

```bash
go run ./cmd/server \
  --mode production \
  --addr :443 \
  --data /var/lib/bitriver-live/store.json \
  --tls-cert /etc/letsencrypt/live/stream.example.com/fullchain.pem \
  --tls-key /etc/letsencrypt/live/stream.example.com/privkey.pem
```

The same values can be supplied through environment variables (`BITRIVER_LIVE_TLS_CERT` and `BITRIVER_LIVE_TLS_KEY`). Pair this with a lightweight cron job or Certbot renewal hook to keep certificates fresh, or terminate TLS at a reverse proxy if you prefer automatic ACME handling upstream.

Prefer containers? Check out `deploy/docker-compose.yml` for a pre-wired stack that mounts persistent storage, exposes metrics, and optionally links Redis for shared rate-limiting state. Chat queue behaviour is configured entirely through the `--chat-queue-*` flags defined in `cmd/server/main.go`; set `--chat-queue-driver redis` to enable Redis Streams support and provide the related connection details via the accompanying flags. The queue constructor will automatically create the configured stream and consumer group when it connects. Operators planning for growth can review [`docs/scaling-topologies.md`](docs/scaling-topologies.md) for single-node, origin/edge, and CDN-assisted layouts that pair with the compose and systemd manifests.

## Appendix: Advanced deployments

Power users who want managed databases, object storage, or automated ingest can dip into the sections below. They are optional for home experimentation—the JSON datastore and development server are all you need to try BitRiver Live locally.

| Flag | Purpose |
| --- | --- |
| `--chat-queue-driver` | Selects the chat queue implementation (`memory` for the in-process queue, `redis` for Redis Streams). |
| `--chat-queue-redis-addr` / `--chat-queue-redis-addrs` | Redis endpoint(s) used by the chat queue. |
| `--chat-queue-redis-username` / `--chat-queue-redis-password` | Credentials for authenticating to Redis. |
| `--chat-queue-redis-stream` / `--chat-queue-redis-group` | Names the Redis Stream and consumer group used for chat events. |
| `--chat-queue-redis-sentinel-master` | Sentinel master name when connecting through Redis Sentinel. |
| `--chat-queue-redis-pool-size` | Maximum number of Redis connections maintained for chat operations. |
| `--chat-queue-redis-tls-ca` / `--chat-queue-redis-tls-cert` / `--chat-queue-redis-tls-key` | TLS certificate material for securing Redis connections. |
| `--chat-queue-redis-tls-server-name` | Overrides the expected Redis TLS server name. |
| `--chat-queue-redis-tls-skip-verify` | Skips Redis TLS certificate verification (use with caution). |

### Postgres backend

BitRiver Live now boots directly against Postgres once the schema is migrated. Apply the SQL files in `deploy/migrations/` with your preferred migration tool or straight through `psql`:

```bash
psql "postgres://bitriver:bitriver@localhost:5432/bitriver?sslmode=disable" \
  --file deploy/migrations/0001_initial.sql
```

With the migrations applied and a Postgres driver such as `pgxpool` available, start the API with the Postgres storage driver:

```bash
go run ./cmd/server \
  --storage-driver postgres \
  --postgres-dsn "postgres://bitriver:bitriver@localhost:5432/bitriver?sslmode=disable" \
  --postgres-max-conns 20 \
  --postgres-min-conns 5 \
  --postgres-acquire-timeout 5s
```

`--postgres-acquire-timeout` bounds how long the API waits to borrow a connection when the pool is exhausted; it does not affect the TCP/TLS handshake with Postgres.

The same configuration can be supplied via environment variables:

| Variable | Description |
| --- | --- |
| `BITRIVER_LIVE_STORAGE_DRIVER` | Set to `postgres` to enable the relational repository. |
| `BITRIVER_LIVE_POSTGRES_DSN` | Connection string passed to the Postgres driver. |
| `BITRIVER_LIVE_POSTGRES_MAX_CONNS` / `BITRIVER_LIVE_POSTGRES_MIN_CONNS` | Pool limits for concurrent and idle connections. |
| `BITRIVER_LIVE_POSTGRES_ACQUIRE_TIMEOUT` | How long to wait when borrowing a connection from the pool. |
| `BITRIVER_LIVE_POSTGRES_MAX_CONN_LIFETIME` | Maximum lifetime before a pooled connection is recycled. |
| `BITRIVER_LIVE_POSTGRES_MAX_CONN_IDLE` | Maximum idle time before a connection is closed. |
| `BITRIVER_LIVE_POSTGRES_HEALTH_INTERVAL` | Frequency of pool health probes. |
| `BITRIVER_LIVE_POSTGRES_APP_NAME` | Optional `application_name` reported to Postgres. |

`deploy/docker-compose.yml` provisions a local Postgres container and wires these environment variables automatically. The Postgres repository implementation lives in `internal/storage/postgres_repository.go`; ensure the migrations in `deploy/migrations/` stay in sync with the Go structs as development progresses.

### Public viewer

The BitRiver Live viewer is what your audience sees: a polished channel directory, live chat beside the stream, and quick links into each creator’s back catalogue. It is optional if you only need the control center, but it makes the platform feel complete for non-technical viewers.

#### Before you start

- [Node.js 18+](https://nodejs.org/en/download/package-manager)
- npm (ships with Node.js, or install separately if your distribution omits it)
- A running BitRiver Live API ([quickstart](#set-up-bitriver-live-at-home))

#### Quick preview

1. `cd web/viewer`
2. `npm install`
3. Set the API endpoint the browser should call:
   ```bash
   NEXT_PUBLIC_API_BASE_URL="http://localhost:8080" npm run dev
   ```
   Omit the variable if the viewer and API share the same origin.

This launches the viewer at [http://localhost:3000](http://localhost:3000) with hot reload so you can browse the public directory, open a channel page, and watch chat update in real time.

#### Production build (optional)

1. `cd web/viewer`
2. `npm ci`
3. Provide the public API URL and compile the standalone output:
   ```bash
   NEXT_PUBLIC_API_BASE_URL="https://api.example.com" npm run build
   ```
4. Start the production server:
   ```bash
   node server.js
   ```

Point the Go API at the viewer by setting `BITRIVER_VIEWER_ORIGIN` (for example, `http://127.0.0.1:3000`). Once that variable is in place, `/viewer` requests proxy straight to the Next.js server so the control center and public site work together.

The viewer now bundles real-time chat, searchable channel discovery, subscriber tooling, and VOD rails. Every channel page exposes a responsive player, a live moderation-aware chat panel, and a replay gallery that pulls straight from the API. The header ships with a theme toggle that mirrors the control-center palette so dark rooms and bright studios both look great.

Docker users can `docker compose up` from `deploy/` to launch both the API and viewer; the compose file wires environment variables and networking automatically. Systemd operators can use the manifests in `deploy/systemd/` to run `bitriver-viewer.service` alongside the API service.

The server exposes a REST API under the `/api` prefix:

| Endpoint | Method | Description |
| --- | --- | --- |
| `/api/auth/signup` | `POST` | Self-service viewer registration with password hashing |
| `/api/auth/login` | `POST` | Issue a session token for password-based sign-in |
| `/api/auth/session` | `GET`, `DELETE` | Inspect or revoke active sessions |
| `/api/users` | `POST`, `GET` | Create new accounts and list all users |
| `/api/users/{id}` | `GET`, `PATCH`, `DELETE` | Inspect, update, or remove a control-center account |
| `/api/channels` | `POST`, `GET` | Provision channels for creators and filter them by owner |
| `/api/channels/{id}` | `GET`, `PATCH`, `DELETE` | Fetch, update, or delete channel metadata |
| `/api/channels/{id}/stream/start` | `POST` | Mark a channel live and begin a stream session |
| `/api/channels/{id}/stream/stop` | `POST` | End the active session and capture peak concurrents |
| `/api/channels/{id}/sessions` | `GET` | Retrieve the session history for a channel |
| `/api/channels/{id}/chat` | `POST`, `GET` | Persist chat messages and fetch recent history |
| `/api/channels/{id}/chat/{messageId}` | `DELETE` | Remove a single chat message for moderation |
| `/api/recordings?channelId={id}` | `GET` | List published recordings (creators can view drafts when authenticated) |
| `/api/recordings/{id}` | `GET`, `DELETE` | Fetch a recording manifest or remove it from storage |
| `/api/recordings/{id}/publish` | `POST` | Mark a recording as publicly accessible and extend its retention window |
| `/api/recordings/{id}/clips` | `GET`, `POST` | List exported highlights or queue a new clip for processing |
| `/api/profiles/{userId}` | `PUT`, `GET` | Configure streamer bios, top friends, and crypto-only donation links |

### Recording retention and object storage

Stopping a stream now generates a recording entry that captures the session metadata, playback manifests, and retention window. Creators can publish the VOD when it is ready, delete it entirely, or export smaller highlight clips via the REST API or the control centre. Configure how long recordings should be kept—both before and after publication—and where the underlying artefacts live using the flags and environment variables below:

| Variable | Description |
| --- | --- |
| `BITRIVER_LIVE_OBJECT_ENDPOINT` | URL for the MinIO/S3-compatible endpoint that stores VOD manifests and thumbnails. |
| `BITRIVER_LIVE_OBJECT_REGION` | Optional region hint for the object storage provider. |
| `BITRIVER_LIVE_OBJECT_ACCESS_KEY` / `BITRIVER_LIVE_OBJECT_SECRET_KEY` | Credentials used when uploading manifests or thumbnails. |
| `BITRIVER_LIVE_OBJECT_BUCKET` | Bucket where recordings, manifests, and thumbnails should be written. |
| `BITRIVER_LIVE_OBJECT_PREFIX` | Prefix applied to each uploaded object (useful for multitenancy). |
| `BITRIVER_LIVE_OBJECT_PUBLIC_ENDPOINT` | Base URL exposed to clients when referencing manifests or thumbnails. |
| `BITRIVER_LIVE_OBJECT_USE_SSL` | Set to `true` when the object storage endpoint expects HTTPS. |
| `BITRIVER_LIVE_OBJECT_LIFECYCLE_DAYS` | Optional lifecycle policy for the bucket; the API shares this with workers that prune stale artefacts. |
| `BITRIVER_LIVE_RECORDING_RETENTION_PUBLISHED` | Duration (e.g. `720h`) that published VODs should be retained before being purged. Use `0` to keep them indefinitely. |
| `BITRIVER_LIVE_RECORDING_RETENTION_UNPUBLISHED` | Duration that drafts stay on disk; `0` disables automatic removal before publication. |

Flags with the same names (see `--object-endpoint`, `--object-bucket`, `--recording-retention-published`, etc.) override the environment variables when provided. The server keeps recordings in the JSON datastore until the retention window elapses and mirrors the policy into object storage lifecycle configuration.

Example: create a user, launch a channel, and start a stream session. These requests require an administrator session token—after promoting your account, log in at `/api/auth/login` and copy the `token` value from the JSON response.

```bash
# Sign in and capture the session token (replace with your email/password)
SESSION_TOKEN=$(curl -s --request POST http://localhost:8080/api/auth/login \
  --header 'Content-Type: application/json' \
  --data '{"email":"you@example.com","password":"secret"}' | jq -r '.token')

# Create a user
curl -s --request POST http://localhost:8080/api/users \
  --header 'Content-Type: application/json' \
  --header "Authorization: Bearer ${SESSION_TOKEN}" \
  --data '{"displayName":"River","email":"river@example.com","roles":["creator"]}'

# Create a channel for that user (replace OWNER_ID with the user ID from above)
curl -s --request POST http://localhost:8080/api/channels \
  --header 'Content-Type: application/json' \
  --header "Authorization: Bearer ${SESSION_TOKEN}" \
  --data '{"ownerId":"OWNER_ID","title":"River Rafting","tags":["outdoors","travel"]}'

# Start streaming (replace CHANNEL_ID)
curl -s --request POST http://localhost:8080/api/channels/CHANNEL_ID/stream/start \
  --header 'Content-Type: application/json' \
  --header "Authorization: Bearer ${SESSION_TOKEN}" \
  --data '{"renditions":["1080p","720p"]}'
```

If you do not have [`jq`](https://stedolan.github.io/jq/) installed, run the login request separately and paste the `token` value into the `SESSION_TOKEN` environment variable manually.

Troubleshooting: a `403 Forbidden` response means the token is missing admin privileges or the `Authorization` header was omitted. Double-check that your user has the `admin` role in `data/store.json`, sign back in to mint a new token, and retry the request.

### Authentication tokens

All authenticated endpoints expect the BitRiver Live session token to be supplied via the `Authorization` header using the
standard Bearer format:

```bash
curl --request GET http://localhost:8080/api/auth/session \
  --header "Authorization: Bearer SESSION_TOKEN"
```

Future releases will also support the same token stored in a secure `bitriver_session` cookie for browser clients. Query-string
tokens are no longer accepted.

To stop the session, POST to `/api/channels/CHANNEL_ID/stream/stop` with an optional `peakConcurrent` value. Chat messages can be posted to `/api/channels/CHANNEL_ID/chat` and retrieved with pagination (`?limit=25`).

Once a creator has at least one friend on the platform, they can publish a profile that highlights their live channels, a MySpace-style “top eight”, and crypto donation addresses that the platform never touches:

```bash
curl -s --request PUT http://localhost:8080/api/profiles/STREAMER_ID \
  --header 'Content-Type: application/json' \
  --data '{
    "bio":"Streaming straight from the river",
    "avatarUrl":"https://cdn.example.com/avatar.png",
    "bannerUrl":"https://cdn.example.com/banner.png",
    "featuredChannelId":"CHANNEL_ID",
    "topFriends":["FRIEND_ID_1","FRIEND_ID_2"],
    "donationAddresses":[
      {"currency":"eth","address":"0x123","note":"main wallet"},
      {"currency":"btc","address":"bc1xyz"}
    ]
  }'
```

The API normalizes currency symbols (e.g., `eth` → `ETH`) and enforces a maximum of eight top friends to preserve the throwback feel. Donation links are peer-to-peer: viewers send crypto directly to streamers with zero custody by the BitRiver Live backend.

For non-technical viewers, the bundled `/signup` page provides a friendly registration and sign-in flow that talks to the authentication endpoints above and persists session tokens to the browser.

### Run automated checks

```bash
go test ./...

cd web/viewer
npm install
npm run lint
npm run test:integration
```

The Postgres-backed storage tests run behind the build tag `postgres`. Provide a
clean database that has been migrated with the contents of
`deploy/migrations/` and point `BITRIVER_TEST_POSTGRES_DSN` at it before invoking
`go test`:

```bash
BITRIVER_TEST_POSTGRES_DSN="postgres://bitriver:bitriver@127.0.0.1:5432/bitriver_test?sslmode=disable" \
  go test -count=1 -tags postgres ./internal/storage/...
```

The DSN must reference an otherwise-empty database so tests can freely create
and tear down rows. The repository ships with `scripts/test-postgres.sh`, which
spins up an ephemeral Postgres container, applies the migrations, and runs the
tagged suite in a single command (Docker required):

```bash
./scripts/test-postgres.sh
```

See [docs/testing.md](docs/testing.md) for the consolidated checklist used in CI.

The Go suite exercises the JSON storage layer, REST handlers, and stream/chat flows end-to-end without requiring any external services or libraries beyond the Go standard library. The viewer integration suite combines Jest component coverage with Playwright accessibility checks—installing dependencies once with `npm install` prepares both harnesses. Playwright downloads its browsers on first run; if you need a CI-friendly install, run `npx playwright install --with-deps` ahead of the test command.

### Configure ingest orchestration

BitRiver Live can orchestrate end-to-end ingest and transcode jobs by talking to an SRS edge, an OvenMediaEngine application, and an FFmpeg job controller. Provide connection details via environment variables when starting the server:

| Variable | Description |
| --- | --- |
| `BITRIVER_SRS_API` | Base URL (including port, e.g. `http://srs:1985`) for the SRS management API. |
| `BITRIVER_SRS_TOKEN` | Bearer token used when creating/deleting SRS channels. |
| `BITRIVER_OME_API` | Base URL for the OvenMediaEngine REST API (defaults to port `8081`). |
| `BITRIVER_OME_USERNAME` / `BITRIVER_OME_PASSWORD` | Basic-auth credentials for OvenMediaEngine. |
| `BITRIVER_TRANSCODER_API` | Base URL for the FFmpeg job runner (e.g. a lightweight controller on port `9000`). |
| `BITRIVER_TRANSCODER_TOKEN` | Bearer token for FFmpeg job APIs. |
| `BITRIVER_TRANSCODE_LADDER` | Optional ladder definition (`1080p:6000,720p:4000,480p:2500`). |
| `BITRIVER_INGEST_MAX_BOOT_ATTEMPTS` | Number of times to retry encoder boot before giving up. |
| `BITRIVER_INGEST_RETRY_INTERVAL` | Delay between retry attempts (e.g. `500ms`). |
| `BITRIVER_INGEST_HTTP_MAX_ATTEMPTS` | Retries for individual HTTP calls to SRS/OME/transcoder (default `3`). |
| `BITRIVER_INGEST_HTTP_RETRY_INTERVAL` | Backoff between HTTP retries (default `500ms`). |
| `BITRIVER_INGEST_HEALTH` | Path that exposes dependency health (default `/healthz`). |

To keep bootstrapping predictable the server now fails fast if any of the required endpoints or credentials above are missing. A complete setup requires:

- An **SRS** management API reachable on port `1985` (or your custom management port) and a bearer token configured via `BITRIVER_SRS_TOKEN`.
- An **OvenMediaEngine** API listener (default `8081`) with an account that has permission to create and delete applications. Provide the username/password through `BITRIVER_OME_USERNAME` and `BITRIVER_OME_PASSWORD`.
- A **transcoder job controller** (such as an FFmpeg fleet manager) exposed over HTTP—commonly on port `9000`—secured with a bearer token supplied in `BITRIVER_TRANSCODER_TOKEN`.

Open the management ports to the BitRiver Live API host and ensure the credentials map to accounts that can create/delete the corresponding resources. Set the optional `BITRIVER_INGEST_HEALTH` path if your services expose health checks somewhere other than `/healthz`.

When these variables are set the API will:

1. POST to `SRS /v1/channels` to allocate RTMP/SRT ingest keys for the channel.
2. POST to `OvenMediaEngine /v1/applications` to configure the playback application.
3. POST to the FFmpeg controller `/v1/jobs` endpoint to launch the adaptive bitrate ladder.

Stopping a stream reverses the process with DELETE calls to `/v1/jobs/{id}`, `/v1/applications/{channelId}`, and `/v1/channels/{channelId}`.

The `/healthz` endpoint now returns JSON that includes the status of these external services so dashboards and probes can surface degraded dependencies early.

### Operations runbook

Operators can use the manifests under `deploy/` as a reference architecture for production or staging clusters. For a step-by-step Ubuntu installation, follow the [Installing BitRiver Live on Ubuntu guide](docs/installing-on-ubuntu.md).

1. **Provision ingest dependencies first.** Bring up SRS, OvenMediaEngine (OME), and the FFmpeg job controller before starting the BitRiver Live API. The compose file at `deploy/docker-compose.yml` defines the services as `srs`, `ome`, and `transcoder` respectively. Each service exposes an HTTP health probe on `/healthz` (with fallbacks to vendor-specific paths) so you can validate readiness with `docker compose ps` or an external probe before the API starts.
2. **Configure secrets securely.**
   - Generate an SRS management token and set it via `BITRIVER_SRS_TOKEN`.
   - Create an administrator account in OME (matching the credentials in `deploy/ome/Server.xml` or your customized configuration) and surface the username/password as `BITRIVER_OME_USERNAME` and `BITRIVER_OME_PASSWORD`.
   - Issue a bearer token for the FFmpeg job controller and inject it with `BITRIVER_TRANSCODER_TOKEN`.
   Store these values in a secrets manager or `.env` file rather than committing them to version control. The sample compose file ships with placeholder values for local development—override them in production.
3. **Boot the API last.** Once the ingest dependencies report healthy you can start the `bitriver-live` service. The server persists the ingest endpoints, playback URLs, and job IDs returned during boot so the current session can be recovered after a restart or audited later via `/api/channels/{id}/sessions`.
4. **Monitor health continuously.** Poll `/healthz` on the API to capture the aggregated ingest status, or query the upstream services directly using the health endpoints listed above. A failing dependency will surface as an `error` status with human-readable detail to aid in incident response.

For Kubernetes deployments replicate the boot order and secret wiring with native primitives (e.g. StatefulSets for ingest services, Secrets for credentials, and readiness probes targeting `/healthz`).

### Rate limiting and audit logging

The HTTP server now enforces an optional global rate limit along with per-IP throttling for login attempts. Configure the guards to taste (and optionally back them with Redis for multi-node deployments):

| Variable | Description |
| --- | --- |
| `BITRIVER_LIVE_RATE_GLOBAL_RPS` | Maximum requests-per-second allowed across the process. |
| `BITRIVER_LIVE_RATE_GLOBAL_BURST` | Optional burst size for the global limiter. |
| `BITRIVER_LIVE_RATE_LOGIN_LIMIT` | Maximum login attempts per IP within the configured window. |
| `BITRIVER_LIVE_RATE_LOGIN_WINDOW` | Rolling window (e.g. `2m`) for counting login attempts. |
| `BITRIVER_LIVE_RATE_REDIS_ADDR` | Redis address used to coordinate login throttling across replicas. |
| `BITRIVER_LIVE_RATE_REDIS_PASSWORD` | Password for the Redis instance if required. |
| `BITRIVER_LIVE_RATE_REDIS_TIMEOUT` | Timeout for Redis operations (`2s` by default). |

All state-changing API calls emit structured audit logs containing the authenticated user (when available), path, status code, and remote IP so you can feed them into `journalctl` or your preferred log pipeline.

### Observability endpoints

BitRiver Live exports Prometheus-compatible metrics and improved health reporting out-of-the-box:

- `GET /healthz` summarises dependency health and ingest orchestration.
- `GET /metrics` emits request counters/latency, stream lifecycle events, ingest gauges, and the current number of active streams.

Point Prometheus, Grafana Agent, or another scraper at `/metrics` to track latency and ingest health. The installer script and deployment assets configure the same endpoints automatically so home operators can wire them into dashboards with minimal effort.

---

## Project roadmap

The high-level product planning notes now live in [docs/product-roadmap.md](docs/product-roadmap.md).
