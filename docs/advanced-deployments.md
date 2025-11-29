# Advanced deployments

Power users who want managed databases, object storage, or automated ingest can dip into the sections below. Postgres is now the standard datastore for every environment; the JSON file backend remains available for quick prototypes when invoked with `--storage-driver json`.

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

## Account management

| Flag | Purpose |
| --- | --- |
| `--allow-self-signup` | Enables or disables unauthenticated account creation. |

| Variable | Description |
| --- | --- |
| `BITRIVER_LIVE_ALLOW_SELF_SIGNUP` | Defaults to `false`; set to `true` to permit viewer self-registration. |

Self-service registration ships disabled so operators can control how new accounts are provisioned. Toggle the feature back on
with `--allow-self-signup` or `BITRIVER_LIVE_ALLOW_SELF_SIGNUP=true` when you are ready to open signups. Administrators can
continue to create accounts manually regardless of this setting.

## Viewer origins and session cookies

| Flag | Purpose |
| --- | --- |
| `--session-cookie-cross-site` | Issues the `bitriver_session` cookie with `SameSite=None; Secure` for cross-site viewer deployments. |
| `--admin-cors-origins` / `--viewer-cors-origins` | Comma-separated list of origins allowed to access the admin and viewer APIs over CORS. |

| Variable | Description |
| --- | --- |
| `BITRIVER_LIVE_SESSION_COOKIE_CROSS_SITE` | Set to `true` to opt into cross-site session cookies; defaults to `false` (Strict). |
| `BITRIVER_LIVE_ADMIN_CORS_ORIGINS` / `BITRIVER_LIVE_VIEWER_CORS_ORIGINS` | Origins (including scheme and host) whitelisted for cross-site requests; defaults deny cross-site origins. |

The default configuration keeps the session cookie in `SameSite=Strict` mode and only marks it as `Secure` when the incoming request arrived over HTTPS, which works for the bundled same-origin viewer. When proxying the viewer from a different domain, enable the cross-site option so the session can flow to the viewer via `SameSite=None`; doing so requires HTTPS end-to-end because browsers reject `SameSite=None` cookies without `Secure`.

When the admin panel or viewer are hosted on different origins, set the corresponding CORS allowlists so browsers can reach the API. Origins must include the scheme and host (for example, `https://admin.example.com,https://watch.example.com`); any origin not listed receives a `403` by default. The quickstart path stays unchanged because same-origin requests remain allowed when the allowlists are empty.

## Postgres backend

BitRiver Live now boots directly against Postgres once the schema is migrated. The Docker Compose bundle ships with a short-lived `postgres-migrations` service that waits for the database, applies every SQL file in `deploy/migrations/`, and exits; `bitriver-live` depends on that helper and will not start until migrations succeed. For bespoke deployments, apply the SQL files with your preferred migration tool or straight through `psql`:

```bash
psql "postgres://bitriver:bitriver@localhost:5432/bitriver?sslmode=disable" \
  --file deploy/migrations/0001_initial.sql
```

With the migrations applied and a Postgres driver such as `pgxpool` available, start the API and point it at the relational database. When compiling from source, always pass the `postgres` build tag so the real driver is linked instead of the lightweight stubs used for JSON-only development:

```bash
go run -tags postgres ./cmd/server \
  --postgres-dsn "postgres://bitriver:bitriver@localhost:5432/bitriver?sslmode=disable" \
  --postgres-max-conns 20 \
  --postgres-min-conns 5 \
  --postgres-acquire-timeout 5s
```

`--postgres-acquire-timeout` bounds how long the API waits to borrow a connection when the pool is exhausted and caps the runtime of the initial transaction or query executed with that connection. It does not affect the TCP/TLS handshake with Postgres.

The same configuration can be supplied via environment variables:

| Variable | Description |
| --- | --- |
| `BITRIVER_LIVE_POSTGRES_DSN` | Connection string passed to the Postgres driver. |
| `BITRIVER_LIVE_POSTGRES_MAX_CONNS` / `BITRIVER_LIVE_POSTGRES_MIN_CONNS` | Pool limits for concurrent and idle connections. |
| `BITRIVER_LIVE_POSTGRES_ACQUIRE_TIMEOUT` | How long to wait when borrowing a connection from the pool and executing the associated statement. |
| `BITRIVER_LIVE_POSTGRES_MAX_CONN_LIFETIME` | Maximum lifetime before a pooled connection is recycled. |
| `BITRIVER_LIVE_POSTGRES_MAX_CONN_IDLE` | Maximum idle time before a connection is closed. |
| `BITRIVER_LIVE_POSTGRES_HEALTH_INTERVAL` | Frequency of pool health probes. |
| `BITRIVER_LIVE_POSTGRES_APP_NAME` | Optional `application_name` reported to Postgres. |

`deploy/docker-compose.yml` provisions a local Postgres container and wires these environment variables automatically. The Postgres repository implementation lives in `internal/storage/postgres_repository.go`; ensure the migrations in `deploy/migrations/` stay in sync with the Go structs as development progresses. Existing JSON installs can be upgraded with `cmd/tools/migrate-json-to-postgres`, which copies `store.json` records into Postgres and verifies the row counts before finishing.

The Postgres-backed storage tests run behind the build tag `postgres`. Provide a clean database that has been migrated with the contents of `deploy/migrations/` and point `BITRIVER_TEST_POSTGRES_DSN` at it before invoking `go test`:

```bash
BITRIVER_TEST_POSTGRES_DSN="postgres://bitriver:bitriver@127.0.0.1:5432/bitriver_test?sslmode=disable" \
  go test -count=1 -tags postgres ./internal/storage/...
```

The DSN must reference an otherwise-empty database so tests can freely create and tear down rows. The repository ships with `scripts/test-postgres.sh`, which spins up an ephemeral Postgres container, applies the migrations, and runs the tagged suite in a single command (Docker required):

```bash
./scripts/test-postgres.sh
```

See [docs/testing.md](testing.md) for the consolidated checklist used in CI.

### Monetization amounts

Tips and subscriptions now store their amounts as fixed-precision minor units (1e-8 of the major currency) to avoid floating point drift in both the JSON store and Postgres. Operators should continue to send human-readable decimal numbers such as `4.99` or `0.00000025` in API requests—values with more than eight fractional digits are rejected. When seeding data or editing snapshots manually, preserve the decimal string form to keep the minor-unit representation consistent. The API keeps the decimal format on the wire; for example, a tip can be recorded with:

```json
{
  "amount": 4.99,
  "currency": "USD",
  "provider": "stripe",
  "reference": "campaign-42"
}
```

Subscriptions follow the same rule and accept decimal amounts up to eight fractional digits:

```json
{
  "tier": "supporter",
  "provider": "stripe",
  "amount": 12.34000001,
  "currency": "USD",
  "durationDays": 30
}
```

## Recording retention and object storage

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

## Configure ingest orchestration

BitRiver Live can orchestrate end-to-end ingest and transcode jobs by talking to an SRS edge, an OvenMediaEngine application, and an FFmpeg job controller. Provide connection details via environment variables when starting the server:

| Variable | Description |
| --- | --- |
| `BITRIVER_SRS_API` | Base URL (including port, e.g. `http://srs-controller:1985`) for the SRS management API proxy. |
| `BITRIVER_SRS_TOKEN` | Bearer token used when creating/deleting SRS channels. |
| `BITRIVER_OME_API` | Base URL for the OvenMediaEngine REST API (defaults to port `8081`). |
| `BITRIVER_OME_BIND` | Address written to the control listener `<Bind>`/`<IP>` fields in `Server.xml` (defaults to `0.0.0.0`). |
| `BITRIVER_OME_IP` | Public IP rendered into the `<Server><IP>` block for signalling (defaults to `BITRIVER_OME_BIND`). |
| `BITRIVER_OME_SERVER_PORT` | Port rendered into the top-level `<Bind><Port>` entry for WebRTC signalling (defaults to `9000`). |
| `BITRIVER_OME_SERVER_TLS_PORT` | Port rendered into `<Bind><TLSPort>` for TLS signalling (defaults to `9443`). |
| `BITRIVER_OME_USERNAME` / `BITRIVER_OME_PASSWORD` | Basic-auth credentials for OvenMediaEngine. |
| `BITRIVER_TRANSCODER_API` | Base URL for the FFmpeg job runner (e.g. a lightweight controller on port `9000`). |
| `BITRIVER_TRANSCODER_TOKEN` | Bearer token for FFmpeg job APIs. |
| `BITRIVER_TRANSCODE_LADDER` | Optional ladder definition (`1080p:6000,720p:4000,480p:2500`). |
| `BITRIVER_INGEST_MAX_BOOT_ATTEMPTS` | Number of times to retry encoder boot before giving up. |
| `BITRIVER_INGEST_RETRY_INTERVAL` | Delay between retry attempts (e.g. `500ms`). |
| `BITRIVER_INGEST_HTTP_MAX_ATTEMPTS` | Retries for individual HTTP calls to SRS/OME/transcoder (default `3`). |
| `BITRIVER_INGEST_HTTP_RETRY_INTERVAL` | Backoff between HTTP retries (default `500ms`). |
| `BITRIVER_INGEST_HEALTH` | Path that exposes dependency health (default `/healthz`). |

The SRS controller proxy accepts two optional environment variables of its own: `SRS_CONTROLLER_BIND` to override the listen address (default `:1985`) and `SRS_CONTROLLER_UPSTREAM` to point at the actual SRS raw API endpoint (default `http://srs:1985/api/`).

To keep bootstrapping predictable the server now fails fast if any of the required endpoints or credentials above are missing. A complete setup requires:

- An **SRS API proxy** (the `srs-controller` service) reachable on port `1985` (or your custom management port). The proxy validates `BITRIVER_SRS_TOKEN` on every request and forwards authenticated calls to the upstream SRS raw API.
- An **SRS** instance the proxy can reach on port `1985` (or your custom management port) with `raw_api` enabled.
- An **OvenMediaEngine** API listener (default `8081`) with an account that has permission to create and delete applications. Provide the username/password through `BITRIVER_OME_USERNAME` and `BITRIVER_OME_PASSWORD`.
- A **transcoder job controller** (such as an FFmpeg fleet manager) exposed over HTTP—commonly on port `9000`—secured with a bearer token supplied in `BITRIVER_TRANSCODER_TOKEN`.

Open the management ports to the BitRiver Live API host and ensure the credentials map to accounts that can create/delete the corresponding resources. Set the optional `BITRIVER_INGEST_HEALTH` path if your services expose health checks somewhere other than `/healthz`.

OvenMediaEngine's control server enforces basic authentication on `/healthz`; the compose bundle mounts `deploy/ome/Server.generated.xml` (rendered from `deploy/ome/Server.xml`) and forwards the same `BITRIVER_OME_USERNAME`/`BITRIVER_OME_PASSWORD` pair to the probe so a 401 will mark the container unhealthy. Keep `.env` aligned with that rendered configuration if you edit the template. The template rewrites the control listener `<Bind>`/`<IP>` values from `BITRIVER_OME_BIND` and stamps the root `<Bind>` block with `<IP>`, `<Port>`, and `<TLSPort>` derived from `BITRIVER_OME_BIND`, `BITRIVER_OME_SERVER_PORT`, and `BITRIVER_OME_SERVER_TLS_PORT` so the bind configuration stays consistent across restarts.

When refreshing an existing OME node, replace any custom `origin_conf/Server.xml` with the template from this repository before restarting the container. Keep the bind/IP entries scoped to `<Modules><Control><Server><Listeners><TCP>` and re-render the credentials with the provided helper:

```bash
cd /opt/bitriver-live
./scripts/render_ome_config.py \
  --template deploy/ome/Server.xml \
  --output deploy/ome/Server.generated.xml \
  --bind "$BITRIVER_OME_BIND" \
  --port "${BITRIVER_OME_SERVER_PORT:-9000}" \
  --tls-port "${BITRIVER_OME_SERVER_TLS_PORT:-9443}" \
  --username "$BITRIVER_OME_USERNAME" \
  --password "$BITRIVER_OME_PASSWORD"
```

Mount the generated file into the container at `/opt/ovenmediaengine/bin/origin_conf/Server.xml` (Compose already wires this path for you) and restart OME so the control listener bind/IP and credentials stay in sync with `.env`.

When these variables are set the API will:

1. POST to `SRS /v1/channels` to allocate RTMP/SRT ingest keys for the channel.
2. POST to `OvenMediaEngine /v1/applications` to configure the playback application.
3. POST to the FFmpeg controller `/v1/jobs` endpoint to launch the adaptive bitrate ladder.

Stopping a stream reverses the process with DELETE calls to `/v1/jobs/{id}`, `/v1/applications/{channelId}`, and `/v1/channels/{channelId}`.

The `/healthz` endpoint returns JSON that includes the status of these external services so dashboards and probes can surface degraded dependencies early, while HTTP 200/503 status codes are reserved for core API dependencies.

## Surface transcoder playback artefacts

The FFmpeg job controller drops HLS manifests and segments under `/work/public` by default. The compose bundle binds that path to `./transcoder-data` on the host so artefacts survive container restarts and can be mirrored elsewhere. Live jobs appear as symlinks at `/work/public/live/<jobID>` that point at the active output directory and are removed when the stream ends, preventing stale session directories from piling up. Populate the directory once before bootstrapping production traffic:

```bash
mkdir -p /opt/bitriver-live/transcoder-data/public
```

Two environment variables determine how playback links are minted:

| Variable | Purpose |
| --- | --- |
| `BITRIVER_TRANSCODER_PUBLIC_DIR` | Absolute path inside the transcoder container that should be mirrored to a CDN or web server (defaults to `/work/public`). |
| `BITRIVER_TRANSCODER_PUBLIC_BASE_URL` | HTTP origin advertised to viewers for the mirrored directory. Set this to the CDN, reverse proxy, or other routable hostname you expose; `deploy/check-env.sh` and Compose fail fast when it is empty or points at loopback. |

Local and single-node installs can rely on the `transcoder-public` Nginx sidecar defined in `deploy/docker-compose.yml`. It serves `/work/public` read-only (following the live-job symlinks via `disable_symlinks off;`) and publishes the content on port `9080` (`docker compose` host). Override `BITRIVER_TRANSCODER_PUBLIC_BASE_URL` when fronting the directory with an existing CDN, S3 static site, or reverse proxy. Advanced operators can also bind additional volumes (e.g. an object storage mount) to `/work` while keeping the base URL aligned with the distribution tier. Whatever origin you select must resolve for end users—playback will fail until viewers can reach the advertised URL.

## Operations runbook

Operators can use the manifests under `deploy/` as a reference architecture for production or staging clusters. For a step-by-step Ubuntu installation, follow the [Installing BitRiver Live on Ubuntu guide](installing-on-ubuntu.md).

1. **Provision ingest dependencies first.** Bring up SRS, the SRS controller proxy, OvenMediaEngine (OME), and the FFmpeg job controller before starting the BitRiver Live API. The compose file at `deploy/docker-compose.yml` defines the services as `srs`, `srs-controller`, `ome`, and `transcoder` respectively. Each service exposes an HTTP health probe on `/healthz` (with fallbacks to vendor-specific paths) so you can validate readiness with `docker compose ps` or an external probe before the API starts.
2. **Configure secrets securely.**
   - Generate an SRS management token and set it via `BITRIVER_SRS_TOKEN`.
   - Create an administrator account in OME (matching the credentials rendered from `deploy/ome/Server.xml` into `deploy/ome/Server.generated.xml`) and surface the username/password as `BITRIVER_OME_USERNAME` and `BITRIVER_OME_PASSWORD`.
   - Issue a bearer token for the FFmpeg job controller and inject it with `BITRIVER_TRANSCODER_TOKEN`.
   Store these values in a secrets manager or `.env` file rather than committing them to version control. The sample compose file ships with placeholder values for local development—override them in production.
3. **Boot the API last.** Once the ingest dependencies report healthy you can start the `bitriver-live` service. The server persists the ingest endpoints, playback URLs, and job IDs returned during boot so the current session can be recovered after a restart or audited later via `/api/channels/{id}/sessions`.
4. **Monitor health continuously.** Poll `/healthz` on the API to capture the aggregated ingest status, or query the upstream services directly using the health endpoints listed above. A failing dependency will surface as an `error` status with human-readable detail to aid in incident response, even though the HTTP status will stay 200 when only ingest services are degraded. Point readiness probes at `/readyz` so deployments only fail over when core API dependencies are unhealthy.

For Kubernetes deployments replicate the boot order and secret wiring with native primitives (e.g. StatefulSets for ingest services, Secrets for credentials, and readiness probes targeting `/readyz`).

## Rate limiting and audit logging

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

## Observability endpoints

BitRiver Live exports Prometheus-compatible metrics and improved health reporting out-of-the-box:

- `GET /healthz` summarises dependency health and ingest orchestration.
- `GET /metrics` emits request counters/latency, stream lifecycle events, ingest gauges, and the current number of active streams.

Point Prometheus, Grafana Agent, or another scraper at `/metrics` to track latency and ingest health. The installer script and deployment assets configure the same endpoints automatically so home operators can wire them into dashboards with minimal effort.
