# Advanced deployments

Power users who want managed databases, object storage, or automated ingest can dip into the sections below. Postgres is the only supported runtime datastore for every environment.

## Ingest → transcode → playback lifecycle

The live pipeline wires together three control-plane components. Use the paths below to trace behaviour and diagnose failures:

- **SRS hook handling:** `internal/api/streams_srs_handlers.go` consumes the `on_publish/on_unpublish/on_play/on_stop` callbacks configured in `deploy/srs/conf/srs.conf`. The handler validates the shared token (`BITRIVER_SRS_TOKEN`), maps stream keys back to channels, and starts/stops sessions in storage. Invalid tokens or stream keys are logged with context and returned as `401/404` responses so operators can see why a publish failed.
- **Transcoder jobs:** `cmd/transcoder` exposes `/v1/jobs` and `/v1/uploads` for the ingest controller. Jobs are persisted under the configured output root, restarted on process restarts, and tracked through a component-aware health endpoint at `/healthz` so FFmpeg crashes or publish failures surface immediately. Job mirrors under `public/live` are refreshed on restart so operators do not need to clean up stale symlinks manually.
- **OvenMediaEngine output:** `deploy/ome/Server.xml` keeps LL-HLS enabled for the `live` application by default. The Go quickstart (`go run ./cmd/bitriver quickstart` plus thin shell/PowerShell wrappers) rewrites bind addresses/ports from `BITRIVER_OME_*` (including `BITRIVER_OME_LLHLS_PORT` / `BITRIVER_OME_LLHLS_TLS_PORT`) and mounts the generated `Server.generated.xml` into the OME container. HLS/DASH clients should read from the LL-HLS publisher on port `8080` (or `BITRIVER_OME_LLHLS_PORT` after templating) to reach the symlinked `public/live/<job>/index.m3u8` manifests produced by the transcoder. For Docker Compose, publish the LL-HLS listener via `BITRIVER_OME_LLHLS_HOST_PORT` to avoid colliding with `BITRIVER_LIVE_PORT` on the host.

| Flag | Purpose |
| --- | --- |
| `--chat-queue-driver` / `BITRIVER_LIVE_CHAT_QUEUE_DRIVER` | Selects the chat queue implementation (`memory` for the in-process queue, `redis` for Redis Streams). |
| `--chat-queue-redis-addr` / `--chat-queue-redis-addrs` | Redis endpoint(s) used by the chat queue. |
| `--chat-queue-redis-username` / `--chat-queue-redis-password` | Credentials for authenticating to Redis. |
| `--chat-queue-redis-stream` / `--chat-queue-redis-group` | Names the Redis Stream and consumer group used for chat events. |
| `--chat-queue-redis-sentinel-master` | Sentinel master name when connecting through Redis Sentinel. |
| `--chat-queue-redis-pool-size` | Maximum number of Redis connections maintained for chat operations. |
| `--chat-queue-redis-tls-ca` / `--chat-queue-redis-tls-cert` / `--chat-queue-redis-tls-key` | TLS certificate material for securing Redis connections. |
| `--chat-queue-redis-tls-server-name` | Overrides the expected Redis TLS server name. |
| `--chat-queue-redis-tls-skip-verify` | Skips Redis TLS certificate verification (use with caution). |

The server honours the driver from either the flag or `BITRIVER_LIVE_CHAT_QUEUE_DRIVER`, defaulting to the in-process `memory` queue when both are unset.

## Kubernetes (Helm) deployments

The Helm chart under `deploy/helm/bitriver-live` mirrors the Compose stack (API, viewer, SRS controller, SRS, OME, transcoder, Redis, Postgres, and the transcoder public proxy). Use it when you want Kubernetes-managed services while still honoring the same `.env` validation that the Compose path expects.

1. Copy the baseline environment template and fill in production-grade credentials:
   ```bash
   cp deploy/.env.example .env
   # edit .env with your production values
   ```
2. Run the same env validation used by Compose to catch placeholder credentials and unsafe URLs before templating values:
   ```bash
   go run ./cmd/bitriver env validate --env-file ./.env
   ```
   (You can also use the `deploy/check-env.sh` wrapper, which delegates to the same Go validator.)
3. Translate `.env` values into a Helm values file. Use `values.env` for non-secret settings (ports, URLs, retention) and `values.secrets` for credentials and tokens. For example:
   ```yaml
   # values.prod.yaml
   env:
     BITRIVER_TRANSCODER_PUBLIC_BASE_URL: https://cdn.example.com/hls
     NEXT_PUBLIC_VIEWER_URL: https://stream.example.com/viewer
   secrets:
     BITRIVER_LIVE_ADMIN_EMAIL: admin@stream.example.com
     BITRIVER_LIVE_ADMIN_PASSWORD: "your-admin-password"
     BITRIVER_SRS_TOKEN: "your-srs-token"
     BITRIVER_OME_API_TOKEN: "your-ome-api-token"
     BITRIVER_TRANSCODER_TOKEN: "your-transcoder-token"
     BITRIVER_REDIS_PASSWORD: "your-redis-password"
     BITRIVER_POSTGRES_PASSWORD: "your-postgres-password"
   ```
4. Install or upgrade the release:
   ```bash
   helm upgrade --install bitriver-live deploy/helm/bitriver-live -f values.prod.yaml
   ```
5. Watch rollout status and confirm health endpoints (`/healthz`, `/readyz`) once the deployments settle:
   ```bash
   kubectl rollout status deployment/bitriver-live-api
   kubectl get pods
   ```

The chart renders `Server.xml` for OME from `values.secrets`/`values.ome`, and it runs Postgres migrations as a pre-install/upgrade hook using generated SQL copies under `deploy/helm/bitriver-live/migrations`. Canonical migration sources remain in `deploy/migrations`, and canonical SRS config remains in `deploy/srs/conf/srs.conf`.

After changing either canonical source set, run:

```bash
./scripts/sync-helm-deploy-assets.sh
```

Then validate there is no drift before shipping:

```bash
./scripts/check-helm-deploy-assets-drift.sh
```

Do not edit generated Helm files directly (`deploy/helm/bitriver-live/migrations/*.sql` and `deploy/helm/bitriver-live/files/srs.conf`); the sync step rewrites them and adds provenance headers automatically.

Persistent volumes mirror Compose mounts: the API stores state under `/var/lib/bitriver-live`, Redis under `/data`, Postgres under `/var/lib/postgresql/data`, and the transcoder workspace under `/work`. Adjust `values.persistence.*` sizes and storage classes for your cluster.

## Fault handling and recovery

When any dependency falters, start with the API health surfaces. `/readyz` returns `503` once core dependencies such as Postgres or the chat queue fail their pings, while `/healthz` keeps the same status code but downgrades the JSON payload when ingest services are unreachable so dashboards still capture SRS/OME/transcoder detail.【F:internal/api/handlers.go†L84-L123】【F:internal/api/health_helpers.go†L14-L44】 The ingest controller probes each service at `<baseURL><BITRIVER_INGEST_HEALTH>` (default `/healthz`) using the credentials seeded in `.env`, and records an `error` status when any probe fails.【F:internal/ingest/http_controller.go†L308-L401】【F:deploy/.env.example†L57-L74】

### Postgres offline

- **What degrades:** Repository pings fail, pushing `/readyz` to `503` and marking the datastore component `degraded`, which blocks automation from routing traffic back until the pool can talk to Postgres again.【F:internal/api/health_helpers.go†L14-L44】 Requests that need persistence (signups, chat history, ingest hooks) will return errors because the store cannot be reached.
- **API/reactive response:** `/healthz` mirrors the degraded status so operators can spot the outage even when other ingest dependencies are healthy.【F:internal/api/handlers.go†L84-L123】 Session cookies and auth remain initialised, but without a backing store mutations fail fast.
- **Recovery checklist:**
  1. Bring Postgres up first and validate the DSN/connection pool knobs the API will use (`BITRIVER_LIVE_POSTGRES_DSN`, `BITRIVER_POSTGRES_*`, `--postgres-*`).【F:deploy/.env.example†L20-L35】【F:cmd/server/main.go†L124-L135】 Re-run migrations if needed.
  2. Restart the API once `psql` or the managed console confirms the database answers connections; `/readyz` should flip back to `200`.
  3. Smoke-test mutations (create a channel, send chat) and check `/healthz` for a clean datastore status before reopening ingest traffic.

### Redis or chat queue offline

- **What degrades:** When `chat_queue` pings fail the API marks readiness `degraded`, and websocket publishers/consumers stop enqueueing messages because the configured queue driver cannot reach Redis.【F:internal/api/health_helpers.go†L14-L44】 Other API routes continue to serve.
- **API/reactive response:** `/readyz` returns `503` until the queue responds, while `/healthz` continues to report ingest status separately so you can distinguish chat-only failures.【F:internal/api/handlers.go†L84-L123】 You can switch to the in-memory queue in emergencies with `--chat-queue-driver memory` or its env twin.
- **Recovery checklist:**
  1. Restore Redis (or swap drivers) and confirm the address/credentials (`BITRIVER_LIVE_CHAT_QUEUE_DRIVER`, `BITRIVER_LIVE_CHAT_QUEUE_REDIS_*`, `--chat-queue-*`) match the running instance.【F:deploy/.env.example†L43-L46】【F:cmd/server/main.go†L167-L180】
  2. Restart the API so new websocket joins rebind to the queue backend; monitor `/readyz` until it returns `200`.
  3. Verify chat delivery by posting in a live room and ensuring consumers drain the Redis stream before reopening to viewers.

### SRS offline

- **What degrades:** RTMP/WebRTC ingest endpoints go dark, so SRS webhook calls never reach the API and new sessions cannot be provisioned. `/readyz` stays `200` because core dependencies remain healthy, but `/healthz` will flag the `srs` component as `error` and overall ingest status as `degraded`.【F:internal/ingest/http_controller.go†L308-L401】
- **API/reactive response:** Attempts to boot ingest pipelines during the outage fail at channel creation, and the controller tears down any partially-created resources before returning the error.【F:internal/ingest/http_controller.go†L120-L195】 Published VODs continue to play from object storage/CDN because playback URLs are served independently of live ingest.【F:docs/advanced-deployments.md†L166-L205】
- **Recovery checklist:**
  1. Restart SRS and verify its `/healthz` (or the path from `BITRIVER_INGEST_HEALTH`) responds using the token in `BITRIVER_SRS_TOKEN`/`--srs-token`.【F:deploy/.env.example†L72-L74】【F:deploy/.env.example†L84-L84】
  2. Once `/healthz` shows `srs` back to `ok`, retry ingest from the broadcaster; the API will accept hooks and start new sessions again.
  3. Reconcile sessions by checking `/api/channels/{id}/sessions` for any aborted attempts before resuming traffic.

### OvenMediaEngine offline

- **What degrades:** Live playback drops because the origin is unreachable; ingest bootstraps fail when the controller cannot create an OME application and roll back SRS provisioning. `/healthz` reports the `ovenmediaengine` component as `error` while `/readyz` remains healthy if Postgres and chat are online.【F:internal/ingest/http_controller.go†L308-L401】【F:internal/ingest/http_controller.go†L120-L195】
- **API/reactive response:** Existing VOD assets remain playable from object storage/CDN, but new live sessions cannot start until OME accepts control-plane requests.【F:docs/advanced-deployments.md†L166-L205】
  - **Recovery checklist:**
    1. Confirm the control credentials and bind config (`BITRIVER_OME_API`, `BITRIVER_OME_API_TOKEN`, `BITRIVER_OME_BIND`, `BITRIVER_OME_SERVER_PORT`, `BITRIVER_OME_SERVER_TLS_PORT`) match the running OME instance. Use `BITRIVER_OME_API_TOKEN` as the authoritative token. Set `BITRIVER_OME_HEALTHCHECK_TOKEN` only when a temporary healthcheck override is required, and keep it aligned during that window.【F:deploy/.env.example†L53-L57】【F:deploy/.env.example†L85-L90】
  2. Restart OME, watch its health endpoint, then re-run `/healthz` on the API to ensure the component returns to `ok`.
  3. Retry ingest start; the controller will reprovision the application and transcoder jobs once OME responds.

### Transcoder offline

- **What degrades:** Live ingest and VOD uploads cannot start jobs; the controller rolls back OME/SRS provisioning when job startup fails, leaving channels without active sessions. `/healthz` shows the `transcoder` component as `error`, while `/readyz` remains healthy for the rest of the API.【F:internal/ingest/http_controller.go†L120-L195】【F:internal/ingest/http_controller.go†L308-L401】
- **API/reactive response:** Already-published VOD manifests stay available because they live on disk/object storage, but no new renditions are produced until the job service returns.【F:docs/advanced-deployments.md†L166-L205】
- **Recovery checklist:**
  1. Bring the transcoder back up, validating the token and endpoints (`BITRIVER_TRANSCODER_API`, `BITRIVER_TRANSCODER_TOKEN`, `BITRIVER_TRANSCODER_PUBLIC_BASE_URL`).【F:deploy/.env.example†L63-L71】【F:deploy/.env.example†L85-L90】
  2. Check the transcoder’s `/healthz` (or the path from `BITRIVER_INGEST_HEALTH`) until it returns `ok` and the API `/healthz` clears the error.
  3. Restart failed ingests or VOD uploads; the controller resubmits jobs and refreshes playback manifests once the backend responds.

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
| `--session-ttl` | Sets the absolute session lifetime (defaults to 7d when unset). |
| `--session-idle-timeout` | Enables idle expiry with refresh-on-activity semantics up to the absolute TTL. |
| `--admin-cors-origins` / `--viewer-cors-origins` | Comma-separated list of origins allowed to access the admin and viewer APIs over CORS. |

| Variable | Description |
| --- | --- |
| `BITRIVER_LIVE_SESSION_COOKIE_CROSS_SITE` | Set to `true` to opt into cross-site session cookies; defaults to `false` (Strict). |
| `BITRIVER_LIVE_SESSION_TTL` | Absolute session lifetime (for example `168h` for 7 days). |
| `BITRIVER_LIVE_SESSION_IDLE_TIMEOUT` | Idle timeout that refreshes expiry on activity when set. |
| `BITRIVER_LIVE_ADMIN_CORS_ORIGINS` / `BITRIVER_LIVE_VIEWER_CORS_ORIGINS` | Origins (including scheme and host) whitelisted for cross-site requests; defaults deny cross-site origins. |

For Docker Compose deployments, set `BITRIVER_LIVE_SESSION_TTL` (and optionally `BITRIVER_LIVE_SESSION_IDLE_TIMEOUT`) in `.env` before running `docker compose up -d` so the API container receives the desired session lifetime values.

The default configuration keeps the session cookie in `SameSite=Strict` mode and only marks it as `Secure` when the incoming request arrived over HTTPS, which works for the bundled same-origin viewer. Sessions expire after 7 days by default; set an idle timeout to refresh the expiry on activity while still enforcing the absolute TTL. When proxying the viewer from a different domain, enable the cross-site option so the session can flow to the viewer via `SameSite=None`; doing so requires HTTPS end-to-end because browsers reject `SameSite=None` cookies without `Secure`.

When the admin panel or viewer are hosted on different origins, set the corresponding CORS allowlists so browsers can reach the API. Origins must include the scheme and host (for example, `https://admin.example.com,https://watch.example.com`); any origin not listed receives a `403` by default. The quickstart path stays unchanged because same-origin requests remain allowed when the allowlists are empty.

## TLS reverse proxy (Caddy)

The optional TLS compose override (`deploy/docker-compose.tls.yml`) runs a Caddy reverse proxy with automatic HTTPS. It routes `/viewer` to the Next.js viewer, forwards `/api` and websocket traffic to the API container, and keeps `/` served by the API. To enable it:

1. Set the public hostname and ACME email in `.env`:
   ```bash
   BITRIVER_PUBLIC_DOMAIN=stream.example.com
   BITRIVER_TLS_EMAIL=admin@stream.example.com
   ```
2. Update the viewer URL to the HTTPS origin you expect users to hit:
   ```bash
   NEXT_PUBLIC_VIEWER_URL=https://stream.example.com/viewer
   ```
3. Start Compose with the override:
   ```bash
   docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.tls.yml up -d
   ```

### Disable HTTP-only host ports

Once HTTPS is in place, remove or firewall the host ports that should never be exposed publicly (for example the API service on `8080`, the OME control plane on `8081`, and the transcoder control port on `9001`). The recommended approach is a local override that clears the `ports:` list for those services, while keeping ingest-facing ports (RTMP/WebRTC) published:

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

Then run:

```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.tls.yml -f deploy/docker-compose.no-http.yml up -d
```

If you prefer not to manage another override file, enforce the same restrictions with host firewall rules or security groups so only ports 80/443 (plus your ingest ports) are reachable.

## Cloudflare + Nginx Proxy Manager routing

If you are fronting BitRiver Live with Cloudflare and Nginx Proxy Manager (NPM), use the dedicated runbook at [`docs/reverse-proxy-npm-cloudflare.md`](reverse-proxy-npm-cloudflare.md). It documents the required `.env` keys (`NEXT_PUBLIC_VIEWER_URL`, `BITRIVER_VIEWER_ORIGIN`, CORS allowlists, and `BITRIVER_TRANSCODER_PUBLIC_BASE_URL`), recommended Cloudflare TLS posture (`orange-cloud` + `Full (strict)`), NPM websocket/path forwarding for `/`, `/api`, and `/viewer`, plus an end-to-end validation checklist.

## API websocket proxy requirements

Even when most API calls are regular HTTP, production proxies must be websocket-safe for the API surface so chat/status realtime behaviour keeps working during upgrades and future endpoint additions.

### Required proxy behaviours

- Use HTTP/1.1 between proxy and upstream for websocket-capable routes.
- Preserve upgrade headers on proxied requests:
  - `Connection: upgrade`
  - `Upgrade: websocket`
  - `Sec-WebSocket-*` headers
- Keep read/send timeouts long-lived (for example `>= 3600s`) so idle realtime connections are not closed early.
- Forward origin/scheme headers so auth/cookie/CORS behaviour remains coherent:
  - `Host`
  - `X-Forwarded-Host`
  - `X-Forwarded-Proto`
  - `X-Forwarded-For`

### Routes that must be websocket-safe

Apply websocket-safe proxying to **all** `/api/*` routes. At minimum, ensure the realtime-facing endpoints below are covered:

- `/api/channels/*/chat` (viewer/admin chat delivery)
- `/api/status` (control-centre status refreshes and related realtime health checks)

If you split routing by path blocks, keep `/api` on the websocket-capable upstream config instead of creating separate non-upgrade API locations.

### Minimal Nginx config block

```nginx
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

location /api/ {
    proxy_http_version 1.1;
    proxy_pass http://bitriver_api;

    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $connection_upgrade;

    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}
```

### NPM UI mapping checklist

For Nginx Proxy Manager, verify all of the following in the Proxy Host UI:

1. **Proxy Host:** domain points to the BitRiver API upstream (`http://<bitriver-host>:8080` by default).
2. **Path mappings:** include `/` and `/api` (and `/viewer` when serving viewer through the same host).
3. **Websocket Support:** toggled **on**.
4. **Forwarded headers:** keep/pass `Host` and `X-Forwarded-Proto` (NPM defaults usually do this; do not override them away in advanced snippets).

### Troubleshooting websocket proxy issues

| Symptom | Likely proxy misconfiguration | What to check/fix |
| --- | --- | --- |
| Chat repeatedly disconnects every 30-60 seconds | Proxy read timeout too low | Raise `proxy_read_timeout`/upstream idle timeout to long-lived values (for example `3600s`). |
| Browser console shows websocket handshake `400` or `426` | Upgrade headers stripped or `proxy_http_version` not `1.1` | Ensure `proxy_http_version 1.1`, `Upgrade`, and `Connection` headers are explicitly forwarded. |
| Chat/status works directly on API port but fails through proxy | `/api` mapped to a non-websocket path block | Route `/api/*` through the websocket-capable location or enable websocket support on that path in NPM. |
| Session/cookie behaviour breaks only behind TLS proxy | Forwarded scheme/host headers missing | Pass `Host` + `X-Forwarded-Proto` so secure-cookie and origin handling match the public URL. |

## Resource limits + ulimits override

For production-ish deployments, use `deploy/docker-compose.limits.yml` for enforceable CPU/memory limits under standard Docker Compose:

```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.limits.yml up -d
```

If you also need higher `nofile` limits for ingest services (SRS, OME, transcoder), layer `deploy/docker-compose.resources.yml` as an additional ulimits-only override:

```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.limits.yml -f deploy/docker-compose.resources.yml up -d
```

The ulimits overlay applies `nofile=262144` to ingest services and does not define CPU/memory limits.

### Host kernel settings (recommended)

Make sure the host kernel and Docker daemon allow the higher file descriptor limits. On Linux, a typical baseline is:

```bash
sudo tee /etc/sysctl.d/99-bitriver-live.conf <<'EOF'
fs.file-max = 1048576
fs.nr_open = 1048576
net.core.somaxconn = 4096
net.ipv4.ip_local_port_range = 10240 65535
EOF

sudo sysctl --system
```

Ensure the Docker daemon inherits a matching limit (e.g., `LimitNOFILE=262144` in a systemd override) and validate with `docker run --rm alpine sh -c "ulimit -n"`.

### Tuning based on stream count

Use the defaults as a starting point for ~2-3 concurrent 1080p streams with a 3-rendition ladder. Then scale roughly as follows:

- **Transcoder:** add ~1 vCPU and ~1–2 GB RAM per additional 1080p stream (double for 4K or larger ladders). GPU-backed encoding can trade CPU for GPU capacity but still needs RAM for segmenting and muxing.
- **OME + SRS:** add ~0.25 vCPU and ~256–512 MB RAM per additional 1,000 concurrent viewers or per ~50 publishers, plus headroom for peaks. If you push WebRTC or low-latency HLS aggressively, favor CPU headroom over tight limits.

If queues back up (late segments, rising encode latency, or dropped viewer connections), increase transcoder CPU/memory first, then expand OME/SRS headroom.

## Security headers

The API emits hardening headers by default so the control centre and embedded viewer ship with internet-safe defaults:

- `Content-Security-Policy`: `default-src 'self'; connect-src 'self'; img-src 'self' data:; script-src 'self'; style-src 'self'; font-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'`
- `X-Frame-Options`: `DENY`
- `Referrer-Policy`: `no-referrer`
- `Permissions-Policy`: `camera=(), microphone=(), geolocation=()`
- `X-Content-Type-Options`: `nosniff`

When you proxy the Next.js viewer through `/viewer`, BitRiver automatically relaxes only that route family's `script-src` directive to `script-src 'self' 'unsafe-inline'` so the shipped Next bootstrap scripts can hydrate the page. The rest of the default directives stay the same, and `--security-csp` / `BITRIVER_LIVE_SECURITY_CSP` still replace the entire policy when you need a custom header.

Override the policy when you need to embed the admin panel or viewer inside a trusted host or allow external resources. Flags and environment variables let you tune the response headers without recompiling:

| Flag | Purpose |
| --- | --- |
| `--security-csp` | Replaces the default `Content-Security-Policy` value. |
| `--security-frame-ancestors` | Updates the `frame-ancestors` directive used in the default CSP. |
| `--security-frame-options` | Overrides `X-Frame-Options` (default `DENY`). |
| `--security-referrer-policy` | Overrides `Referrer-Policy` (default `no-referrer`). |
| `--security-permissions-policy` | Overrides `Permissions-Policy` (default `camera=(), microphone=(), geolocation=()`). |
| `--security-content-type-options` | Overrides `X-Content-Type-Options` (default `nosniff`). |

| Variable | Description |
| --- | --- |
| `BITRIVER_LIVE_SECURITY_CSP` | Replaces the default `Content-Security-Policy` value. |
| `BITRIVER_LIVE_SECURITY_FRAME_ANCESTORS` | Updates the `frame-ancestors` directive used in the default CSP. |
| `BITRIVER_LIVE_SECURITY_FRAME_OPTIONS` | Overrides `X-Frame-Options` (default `DENY`). |
| `BITRIVER_LIVE_SECURITY_REFERRER_POLICY` | Overrides `Referrer-Policy` (default `no-referrer`). |
| `BITRIVER_LIVE_SECURITY_PERMISSIONS_POLICY` | Overrides `Permissions-Policy` (default `camera=(), microphone=(), geolocation=()`). |
| `BITRIVER_LIVE_SECURITY_CONTENT_TYPE_OPTIONS` | Overrides `X-Content-Type-Options` (default `nosniff`). |

## Postgres backend

BitRiver Live now boots directly against Postgres once the schema is migrated. The Docker Compose bundle ships with a short-lived `postgres-migrations` service that waits for the database, applies every SQL file in `deploy/migrations/`, and exits; `bitriver-live` depends on that helper and will not start until migrations succeed. For bespoke deployments, apply the SQL files with your preferred migration tool or straight through `psql`:

```bash
psql "postgres://bitriver:bitriver@localhost:5432/bitriver?sslmode=disable" \
  --file deploy/migrations/0001_initial.sql
```

The local Compose Postgres container does not ship with TLS certificates, so `sslmode=disable` is acceptable for `localhost` or the Compose service. When you target an external or managed database, require TLS by switching to `sslmode=require` or `sslmode=verify-full` and supply `sslrootcert=/certs/postgres-ca.pem` (or another mounted CA path) in the DSN.

With the migrations applied and a Postgres driver such as `pgxpool` available, start the API and point it at the relational database. The repository has an explicit pgx switch point: local/offline builds default to the checked-in stub module, while release builds that expect `BITRIVER_LIVE_STORAGE_DRIVER=postgres` must switch to a non-stub pgx source before compiling. Validate the switch with `./scripts/check-postgres-pgx.sh postgres`, then compile with the `postgres` build tag so the Postgres-capable driver wiring is linked:

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

`deploy/docker-compose.yml` provisions a local Postgres container and wires these environment variables automatically, defaulting to `sslmode=disable` for the local `postgres` service. The environment validator rejects `sslmode=disable` when you point the DSN at anything other than the Compose service, so remember to enable TLS for managed databases. The Postgres repository implementation lives in `internal/storage/postgres_repository.go`; ensure the migrations in `deploy/migrations/` stay in sync with the Go structs as development progresses. Existing JSON installs can be upgraded with `cmd/tools/migrate-json-to-postgres`, which copies `store.json` records into Postgres and verifies the row counts before finishing.

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

### Postgres backups and recovery

For a step-by-step runbook (including Compose service names and restore commands), see the operations guide in
[`docs/operations.md`](operations.md).

Postgres is the source of truth for channels, recordings, chat metadata, and audit history. The Compose bundle derives the DSN from `BITRIVER_POSTGRES_DB`, `BITRIVER_POSTGRES_USER`, and `BITRIVER_POSTGRES_PASSWORD` in `deploy/.env.example`; override it directly with `BITRIVER_LIVE_POSTGRES_DSN` or `--postgres-dsn` when pointing the API at managed instances.【F:deploy/.env.example†L20-L36】【F:cmd/server/main.go†L126-L138】 Keep connection pool limits aligned with your backup strategy so `pg_dump` sessions are not throttled (`BITRIVER_LIVE_POSTGRES_MAX_CONNS`, `BITRIVER_LIVE_POSTGRES_MIN_CONNS`, `--postgres-max-conns`, and `--postgres-min-conns`).【F:deploy/.env.example†L28-L31】【F:cmd/server/main.go†L128-L133】

Routine logical backups rely on `pg_dump` against the DSN the server uses:

```bash
pg_dump "$BITRIVER_LIVE_POSTGRES_DSN" \
  --format=custom \
  --file=bitriver-live.backup
```

Restore into a fresh database with `pg_restore --clean --if-exists --create` or by piping the archive back through `psql`, then point `BITRIVER_LIVE_POSTGRES_DSN` at the restored endpoint before restarting `cmd/server` so it becomes the active source of truth. Enable WAL archiving and base backups if you need point-in-time recovery; the application does not manage archiving for you, but it tolerates replaying to any timestamp as long as migrations from `deploy/migrations/` have been applied. When converting from JSON snapshots, import them before your first backup using `cmd/tools/migrate-json-to-postgres` so subsequent dumps capture a single authoritative store.

`pg_dump`/`pg_restore` run outside the container stack; use the `postgres-host` Compose profile to expose the port only during maintenance, or connect through your cloud provider’s managed endpoint to keep traffic off the application network.【F:deploy/.env.example†L37-L40】 After a restore, smoke-test with `scripts/test-postgres.sh` to verify migrations and connectivity mirror production before reopening traffic.

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

Flags with the same names (see `--object-endpoint`, `--object-bucket`, `--recording-retention-published`, etc.) override the environment variables when provided. The server keeps recording metadata in Postgres until the retention window elapses and mirrors the policy into object storage lifecycle configuration.

### Object storage lifecycle for VODs and thumbnails

Buckets should enforce the same retention you configure on the server so thumbnails and manifests expire in lockstep. The `--object-lifecycle-days` flag (or `BITRIVER_LIVE_OBJECT_LIFECYCLE_DAYS`) allows the API to communicate the desired lifecycle to workers that prune old artefacts; align it with `--recording-retention-published`/`--recording-retention-unpublished` (or their env counterparts) so object expiration never precedes the database record expiry.【F:cmd/server/main.go†L182-L193】【F:cmd/server/main.go†L318-L330】 When storing regulatory copies or enabling creator rollbacks, turn on bucket versioning and set lifecycle rules to retain previous versions longer than the published retention window so deletes stay reversible.

Endpoints and credentials for uploads come from the object storage flags (`--object-endpoint`, `--object-region`, `--object-access-key`, `--object-secret-key`, `--object-bucket`, `--object-prefix`, `--object-public-endpoint`, `--object-use-ssl`) or their `BITRIVER_LIVE_OBJECT_*` equivalents, letting you target MinIO/S3 in different regions without recompiling.【F:cmd/server/main.go†L182-L190】【F:cmd/server/main.go†L318-L328】 Operators running on bespoke S3 tiers should keep bucket versioning and lifecycle policies consistent with `BITRIVER_LIVE_OBJECT_LIFECYCLE_DAYS` even when CDN cache TTLs differ; the API will continue to serve presigned URLs until the backing object is deleted.

### Redis persistence expectations

Redis backs the chat queue (`--chat-queue-driver redis`) and optional distributed login throttling; it is treated as a cache and transport layer rather than a system of record. The Compose template wires the chat queue to `BITRIVER_LIVE_CHAT_QUEUE_REDIS_ADDR`/`BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD`, and the server exposes matching flags for addresses, credentials, streams, and TLS material so you can point at managed clusters or Sentinel.【F:deploy/.env.example†L43-L46】【F:cmd/server/main.go†L167-L180】 Chat messages are delivered through Redis Streams; if the node is lost without persistence (RDB/AOF), in-flight chat and rate-limit counters are discarded, but published recordings and account state remain intact in Postgres. Enable RDB snapshots or AOF on the Redis side when you want stream history to survive restarts, and monitor reconnections—the API will recreate consumer groups and continue processing once the Redis endpoint is reachable again. The operations runbook calls out the default cache-only Compose settings, Redis restore steps, and media directory backups.【F:docs/operations.md†L105-L189】

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

Troubleshooting: a `403 Forbidden` response means the token is missing admin privileges or the `Authorization` header was omitted. Double-check that your user has the `admin` role in Postgres, sign back in to mint a new token, and retry the request.

## Configure ingest orchestration

BitRiver Live can orchestrate end-to-end ingest and transcode jobs by talking to an SRS edge, an OvenMediaEngine application, and an FFmpeg job controller. Provide connection details via environment variables when starting the server:

| Variable | Description |
| --- | --- |
| `BITRIVER_SRS_API` | Base URL (including port, e.g. `http://srs-controller:1985`) for the SRS management API proxy. |
| `BITRIVER_SRS_TOKEN` | Bearer token used when creating/deleting SRS channels. |
| `BITRIVER_OME_API` | Base URL for the OvenMediaEngine REST API (defaults to port `8081`). |
| `BITRIVER_OME_BIND` | Address written to control-listener bind fields in `Server.xml` (defaults to `0.0.0.0`). The top-level `<Server><IP>` remains the canonical server bind host field, and root `<Bind><IP>`/`<Bind><Address>` host tags are not rendered. |
| `BITRIVER_OME_IP` | Public IP rendered into the `<Server><IP>` block for signalling (defaults to `BITRIVER_OME_BIND`). |
| `BITRIVER_OME_SERVER_PORT` | Port rendered into the top-level `<Bind><Port>` entry for WebRTC signalling (defaults to `9000`). |
| `BITRIVER_OME_SERVER_TLS_PORT` | Port rendered into `<Bind><TLSPort>` for TLS signalling (defaults to `9443`). |
| `BITRIVER_OME_RELAY_PORT` / `BITRIVER_OME_RELAY_PROTOCOL` | Host port and protocol published for the TURN relay (Compose exposes `3478/udp` plus `3478/<protocol>`; set the protocol to `tcp` when your network blocks UDP). |
| `BITRIVER_OME_TCP_RELAY` | Address advertised in `<TcpRelay>` for TURN/TCP candidates (defaults to `*:3478`; set to the externally reachable host/port when NATting). |
| `BITRIVER_OME_ICE_PORT_RANGE` | Host-facing UDP range published for media relays (defaults to `10000-10009`). |
| `BITRIVER_OME_ICE_CANDIDATE` | ICE candidate advertised to browsers (defaults to `*:10000-10009/udp`; replace with public IP/port range when NATting or port-forwarding). |
| `BITRIVER_OME_API_TOKEN` | Control-plane API token for OvenMediaEngine. Canonical OME token contract uses `BITRIVER_OME_API_TOKEN` by default, with optional temporary override `BITRIVER_OME_HEALTHCHECK_TOKEN`. Migration note: operators still setting `BITRIVER_OME_ACCESS_TOKEN` should copy that value into `BITRIVER_OME_API_TOKEN` (or temporary `BITRIVER_OME_HEALTHCHECK_TOKEN`) before upgrading to the release that drops legacy fallback support. |
| `BITRIVER_TRANSCODER_API` | Base URL for the FFmpeg job runner (e.g. a lightweight controller on port `9000`). |
| `BITRIVER_TRANSCODER_TOKEN` | Bearer token for FFmpeg job APIs. |
| `BITRIVER_TRANSCODE_LADDER` | Optional ladder definition (`1080p:6000,720p:4000,480p:2500`). |
| `BITRIVER_INGEST_MAX_BOOT_ATTEMPTS` | Number of times to retry encoder boot before giving up. |
| `BITRIVER_INGEST_RETRY_INTERVAL` | Delay between retry attempts (e.g. `500ms`). |
| `BITRIVER_INGEST_HTTP_MAX_ATTEMPTS` | Retries for individual HTTP calls to SRS/OME/transcoder (default `3`). |
| `BITRIVER_INGEST_HTTP_RETRY_INTERVAL` | Backoff between HTTP retries (default `500ms`). |
| `BITRIVER_INGEST_HEALTH` | Path that exposes dependency health (default `/healthz`). |

The SRS controller proxy accepts two optional environment variables of its own: `SRS_CONTROLLER_BIND` to override the listen address (default `:1985`) and `SRS_CONTROLLER_UPSTREAM` to point at the actual SRS raw API endpoint (default `http://srs:1985/api/`).

Leaving all of the ingest variables empty disables the controller and produces a startup warning so deployments without ingest can proceed. Supplying only part of the configuration is treated as an error: the API process will exit and log the specific `BITRIVER_*` variables still required. A complete setup requires:

- An **SRS API proxy** (the `srs-controller` service) reachable on port `1985` (or your custom management port). The proxy validates `BITRIVER_SRS_TOKEN` on every request and forwards authenticated calls to the upstream SRS raw API.
- An **SRS** instance the proxy can reach on port `1985` (or your custom management port) with `raw_api` enabled.
- An **OvenMediaEngine** API listener (default `8081`) with an account that has permission to create and delete applications. Provide the control-plane API token through `BITRIVER_OME_API_TOKEN`. If you set `BITRIVER_OME_HEALTHCHECK_TOKEN`, keep it equal to `BITRIVER_OME_API_TOKEN` except during short migration/rotation windows, then remove the override.
- A **transcoder job controller** (such as an FFmpeg fleet manager) exposed over HTTP—commonly on port `9000`—secured with a bearer token supplied in `BITRIVER_TRANSCODER_TOKEN`.

Open the management ports to the BitRiver Live API host and ensure the credentials map to accounts that can create/delete the corresponding resources. Set the optional `BITRIVER_INGEST_HEALTH` path if your services expose health checks somewhere other than `/healthz`.

Compose liveness/readiness for the `ome` service is intentionally unauthenticated and targets `http://localhost:${BITRIVER_OME_HTTP_PORT:-8081}/`. The probe is healthy for any HTTP status that is not `000` and is `<500`; it is unhealthy only on transport failures (curl `000`) or `5xx` responses. Keep this distinct from control-plane API auth. For API calls (for example `/v1/...` routes), operators should send HTTP Basic auth where the raw rendered access token is the full pre-base64 credential string (not `AccessToken: <token>` and not `Authorization: Bearer <token>`), and keep `BITRIVER_OME_HEALTHCHECK_TOKEN` consistent with `BITRIVER_OME_API_TOKEN` whenever the override is configured. The template writes API auth to top-level `<Managers><API><AccessToken>`, rewrites control-listener bind values from `BITRIVER_OME_BIND`, keeps root `<Bind>` focused on protocol sections (`<Providers>`, `<Publishers>`), omits unsupported root bind host child tags, and uses top-level `<Server><IP>` (derived from `BITRIVER_OME_IP`) as the canonical server bind host field. Application output profiles must remain direct children of `<Application><OutputProfiles>` to stay schema-compatible; avoid deprecated `<Application><Outputs>` wrappers. Keep LL-HLS in `<Bind><Publishers><LLHLS>` only and do not add `<Application><LLHLS>`.

Source of truth: when this guidance and behavior diverge, trust the `ome` service `healthcheck:` block in `deploy/docker-compose.yml`.

When enabling WebRTC playback, forward the relay paths end users will hit:

- Publish the TURN relay port on `3478` (TCP or UDP depending on your firewall). Compose binds `3478/udp` plus `3478/<protocol>` from `BITRIVER_OME_RELAY_PROTOCOL`; mirror the externally reachable host/port in `BITRIVER_OME_TCP_RELAY` so `<TcpRelay>` advertises the correct endpoint.
- Publish the UDP relay range from `10000-10009` (or your chosen slice in `BITRIVER_OME_ICE_PORT_RANGE`) and update `BITRIVER_OME_ICE_CANDIDATE` to point at the public IP/port range when NAT or port-forwarding rewrites the container addresses.

Open matching firewall or security group rules before testing playback:

- Allow UDP traffic to `3478` (and TCP `3478` when `BITRIVER_OME_RELAY_PROTOCOL=tcp`) so TURN allocation succeeds.
- Allow UDP traffic across the `BITRIVER_OME_ICE_PORT_RANGE` (default `10000-10009`) to pass the media relay packets.

After changing these values, run `go run ./cmd/bitriver ome render --force --env-file ./.env` (or `./scripts/render-ome-config.sh --force`) so `deploy/ome/Server.generated.xml` advertises the updated ICE candidates, then open matching firewall/NAT rules on your load balancer or host security group. Browsers will refuse WebRTC playback when they cannot reach the configured relay ports.

When refreshing an existing OME node, replace any custom `origin_conf/Server.xml` with the template from this repository before restarting the container. Keep the bind/IP entries scoped to `<Modules><Control><Server><Listeners><TCP>` and re-render the credentials with the provided helper:

```bash
cd /opt/bitriver-live
go run ./cmd/bitriver ome render --force --env-file /opt/bitriver-live/.env
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
| `BITRIVER_TRANSCODER_PUBLIC_BASE_URL` | HTTP origin advertised to viewers for the mirrored directory. Set this to the CDN, reverse proxy, or other routable hostname you expose; the Go env validator (used by `deploy/check-env.sh` and Compose) fails fast when it is empty or points at loopback. |

Local and single-node installs can rely on the `transcoder-public` Nginx sidecar defined in `deploy/docker-compose.yml`. It serves `/work/public` read-only (following the live-job symlinks via `disable_symlinks off;`) and publishes the content on port `9080` (`docker compose` host). Override `BITRIVER_TRANSCODER_PUBLIC_BASE_URL` when fronting the directory with an existing CDN, S3 static site, or reverse proxy. Advanced operators can also bind additional volumes (e.g. an object storage mount) to `/work` while keeping the base URL aligned with the distribution tier. Whatever origin you select must resolve for end users—playback will fail until viewers can reach the advertised URL.

## Operations runbook

### Troubleshooting

- **`postgres repository unavailable: pgx driver stubbed in this build`** – This indicates a stub-only API binary/image was deployed without Postgres support linked in. Follow one action path only: rebuild and redeploy from artifacts that are verified to include the `postgres` build tag and the non-stub pgx driver, then roll the deployment forward with those artifacts. Validate artifact provenance (tag + digest/workflow outputs) using the release checks in [`docs/production-release.md`](production-release.md).

Operators can use the manifests under `deploy/` as a reference architecture for production or staging clusters. For host-managed installs, the BitRiver CLI ships installers today for systemd (Linux), launchd (macOS), and Windows Service definitions alongside the [Installing BitRiver Live on Ubuntu guide](installing-on-ubuntu.md).

1. **Provision ingest dependencies first.** Bring up SRS, the SRS controller proxy, OvenMediaEngine (OME), and the FFmpeg job controller before starting the BitRiver Live API. The compose file at `deploy/docker-compose.yml` defines the services as `srs`, `srs-controller`, `ome`, and `transcoder` respectively. Each service exposes an HTTP health probe so you can validate readiness with `docker compose ps` or an external probe before the API starts (`/healthz` for BitRiver API, SRS controller, SRS proxy, and transcoder; `http://localhost:${BITRIVER_OME_HTTP_PORT:-8081}/` for OME with no auth header, where any non-`000` and `<500` status is healthy).
2. **Render and verify the OME config before each launch.** Run `go run ./cmd/bitriver ome render --force --env-file /opt/bitriver-live/.env` (or the `./scripts/render-ome-config.sh` wrapper) immediately before starting OME (the `ome.service` unit performs the same preflight). The render step stamps `deploy/ome/Server.generated.xml` with the image tag marker and exits non-zero when the template is missing or stale, preventing Docker from booting with an outdated control listener or credentials. The only supported render path is the Go renderer and its wrapper script.
   - The wrapper accepts `--check` when you want a CI guard that compares `BITRIVER_OME_IMAGE_TAG` in `.env` with the stamped marker in `deploy/ome/Server.generated.xml`, failing if they diverge so Compose reloads a freshly rendered config.
   - Use the version/token matrix below to line up your `.env` before rendering:

| `BITRIVER_OME_IMAGE_TAG` | `BITRIVER_OME_API_TOKEN` expectation | Rendering behaviour |
| --- | --- | --- |
| `0.16.0+` (default) | Must be non-empty. Runtime uses `BITRIVER_OME_API_TOKEN` by default, with optional `BITRIVER_OME_HEALTHCHECK_TOKEN` override, and startup checks fail when configured values disagree. | The renderer injects `<Managers><API><AccessToken>` with the canonical token, stamps the render marker, and preserves the modern output schema. |

Example: set `BITRIVER_OME_API_TOKEN=shared-token` and leave `BITRIVER_OME_HEALTHCHECK_TOKEN` unset; render/validation/healthchecks all resolve `shared-token`.
3. **Configure secrets securely.**
   - Generate an SRS management token and set it via `BITRIVER_SRS_TOKEN`.
   - Issue a bearer token for the FFmpeg job controller and inject it with `BITRIVER_TRANSCODER_TOKEN`.
   Store these values in a secrets manager or `.env` file rather than committing them to version control. The sample compose file ships with placeholder values for local development—override them in production.
4. **Boot the API last.** Once the ingest dependencies report healthy you can start the `bitriver-live` service. The server persists the ingest endpoints, playback URLs, and job IDs returned during boot so the current session can be recovered after a restart or audited later via `/api/channels/{id}/sessions`.
5. **Monitor health continuously.** Poll `/healthz` on the API to capture the aggregated ingest status, or query the upstream services directly using the health endpoints listed above. A failing dependency will surface as an `error` status with human-readable detail to aid in incident response, even though the HTTP status will stay 200 when only ingest services are degraded. Point readiness probes at `/readyz` so deployments only fail over when core API dependencies are unhealthy.

For Kubernetes deployments replicate the boot order and secret wiring with native primitives (e.g. StatefulSets for ingest services, Secrets for credentials, and readiness probes targeting `/readyz`).

## Rate limiting and audit logging

The HTTP server now enforces an optional global rate limit along with per-IP throttling for login attempts. Configure the guards to taste (and optionally back them with Redis for multi-node deployments):

| Variable | Description |
| --- | --- |
| `BITRIVER_LIVE_RATE_GLOBAL_RPS` | Maximum requests-per-second allowed across the process. |
| `BITRIVER_LIVE_RATE_GLOBAL_BURST` | Optional burst size for the global limiter. |
| `BITRIVER_LIVE_RATE_LOGIN_LIMIT` | Maximum login attempts per IP within the configured window. |
| `BITRIVER_LIVE_RATE_LOGIN_WINDOW` | Rolling window (e.g. `2m`) for counting login attempts. |
| `BITRIVER_LIVE_RATE_TRUST_FORWARDED_HEADERS` | Defaults to `false`. When enabled, login throttling can use forwarded client-IP headers (only from `BITRIVER_LIVE_RATE_TRUSTED_PROXIES`). |
| `BITRIVER_LIVE_RATE_TRUSTED_PROXIES` | Comma-separated trusted proxy CIDRs/IPs allowed to supply `X-Forwarded-*`/`Forwarded` data for rate-limit IP resolution. |
| `BITRIVER_LIVE_UPLOADS_TRUST_FORWARDED_HEADERS` | Defaults to `false`. Controls whether upload URL generation trusts forwarded `Host`/`Proto` headers from trusted proxy hops. |
| `BITRIVER_LIVE_UPLOAD_MEDIA_BASE_URL` | Optional canonical externally reachable base URL (for example `https://media.example.com` or `https://media.example.com/prefix`). When set, upload media `sourceUrl` values are built from this base URL instead of forwarded/request host data. |
| `BITRIVER_LIVE_RATE_REDIS_ADDR` | Redis address used to coordinate login throttling across replicas. |
| `BITRIVER_LIVE_RATE_REDIS_PASSWORD` | Password for the Redis instance if required. |
| `BITRIVER_LIVE_RATE_REDIS_TIMEOUT` | Timeout for Redis operations (`2s` by default). |

Safe usage guidance:

- Keep forwarded-header trust disabled unless you have pinned proxy CIDRs/IPs in `BITRIVER_LIVE_RATE_TRUSTED_PROXIES`.
- For Cloudflare + Nginx Proxy Manager deployments, include both Cloudflare source ranges and your NPM private source CIDR/IP so only known hops are trusted.
- Never trust broad ranges such as `0.0.0.0/0` or `::/0`; that makes `X-Forwarded-*` headers spoofable and weakens per-IP controls.

Practical examples:

- **Single reverse proxy (one Nginx/Traefik hop):**
  ```dotenv
  BITRIVER_LIVE_RATE_TRUST_FORWARDED_HEADERS=true
  BITRIVER_LIVE_RATE_TRUSTED_PROXIES=10.0.10.5/32
  BITRIVER_LIVE_UPLOADS_TRUST_FORWARDED_HEADERS=true
  ```

- **Cloudflare + NPM chained proxies:**
  ```dotenv
  BITRIVER_LIVE_RATE_TRUST_FORWARDED_HEADERS=true
  BITRIVER_LIVE_RATE_TRUSTED_PROXIES=173.245.48.0/20,103.21.244.0/22,103.22.200.0/22,10.0.10.5/32
  BITRIVER_LIVE_UPLOADS_TRUST_FORWARDED_HEADERS=true
  ```

All state-changing API calls emit structured audit logs containing the authenticated user (when available), path, status code, and remote IP so you can feed them into `journalctl` or your preferred log pipeline.

## Observability endpoints

BitRiver Live exports Prometheus-compatible metrics and improved health reporting out-of-the-box:

- `GET /healthz` summarises dependency health and ingest orchestration.
- `GET /metrics` exposes the metrics below. Guard this endpoint with `--metrics-token`/`BITRIVER_LIVE_METRICS_TOKEN` (validated against the `Authorization: Bearer` or `X-Metrics-Token` header) or lock it to specific CIDRs/IPs via `--metrics-allow-networks`/`BITRIVER_LIVE_METRICS_ALLOW_NETWORKS`. Health and readiness endpoints stay public.

### Metric families

- **HTTP:** `bitriver_http_requests_total{method,path,status}` counters plus `bitriver_http_request_duration_seconds_sum`/`bitriver_http_request_duration_seconds_count` for cumulative request latency by method/path/status (paths are normalised by replacing identifiers with `:id`).
- **Streams:** `bitriver_stream_events_total{event}` counters for start/stop activity and the `bitriver_active_streams` gauge tracking concurrent live channels.
- **Ingest:** `bitriver_ingest_health{service,status}` gauges (`1=ok`, `0=disabled`, `-1=degraded`) alongside `bitriver_ingest_attempts_total{operation}` and `bitriver_ingest_failures_total{operation}` for boot/shutdown/upload orchestration.
- **Chat:** `bitriver_chat_events_total{event}` counters for viewer chat activity, moderation, and reports.
- **Monetization:** `bitriver_monetization_events_total{event}` counters and `bitriver_monetization_amount_sum{event}` tracking the aggregated decimal amount per tip/subscription type.
- **Transcoder:** `bitriver_transcoder_jobs_total{kind,status}` counters and the `bitriver_transcoder_active_jobs` gauge for live/upload encoding work.

## Moderation API pagination

The moderation API endpoints accept pagination controls so operators can fetch moderation data incrementally:

- `GET /api/moderation/queue`
  - `limit` (default `50`): maximum number of open queue items to return.
  - `cursor`: RFC3339 timestamp cursor returned by `queueMeta.nextCursor`, used to fetch the next (older) page.
  - `actionsLimit` (default `20`): maximum number of resolved actions to return.
  - `actionsCursor`: RFC3339 timestamp cursor returned by `actionsMeta.nextCursor`, used to fetch older resolved actions.
- `GET /api/moderation/automod`
  - `limit` (default `50`): maximum number of auto-moderation actions.
  - `cursor`: RFC3339 timestamp cursor returned by `meta.nextCursor`, used to page older items.

Each response includes pagination metadata to support incremental fetches:

- `/api/moderation/queue` includes `queueMeta` and `actionsMeta` objects with `nextCursor`, `limit`, and `hasMore`.
- `/api/moderation/automod` includes a `meta` object with the same fields.

Example requests:

```bash
curl "$BITRIVER_BASE_URL/api/moderation/queue?limit=50&actionsLimit=20"
curl "$BITRIVER_BASE_URL/api/moderation/queue?cursor=2024-02-01T12:00:00Z"
curl "$BITRIVER_BASE_URL/api/moderation/automod?limit=50"
curl "$BITRIVER_BASE_URL/api/moderation/automod?cursor=2024-02-01T12:00:00Z"
```

### Prometheus scrape example

Point Prometheus, Grafana Agent, or another scraper at `/metrics` to track latency and ingest health. The installer script and deployment assets configure the same endpoints automatically so home operators can wire them into dashboards with minimal effort.

```yaml
scrape_configs:
  - job_name: bitriver-live
    scrape_interval: 15s
    metrics_path: /metrics
    bearer_token: "changeme-metrics-token" # matches --metrics-token or BITRIVER_LIVE_METRICS_TOKEN
    static_configs:
      - targets: ["bitriver-live:8080"]
```

To further tighten access, run the API behind an ingress or firewall that only allows scrapers to hit `/metrics`, combine `--metrics-token` with `--metrics-allow-networks` to require both a token and a trusted IP/CIDR, and avoid publishing the endpoint publicly.

## Payment webhooks, secret rotation, and reconciliation

Payment providers now complete tip/subscription lifecycle transitions asynchronously through signed webhook callbacks. Creation APIs record `pending` payment rows first, and provider events move them to `confirmed`, `failed`, or `refunded`.

### Provider secret rotation

- Configure a per-provider webhook secret and roll it regularly (for example every 90 days).
- During rotation, accept both the current and next secret at your edge (API gateway / ingress), then switch the application secret and remove the old one after provider retry windows expire.
- Keep rotation runbooks in your secret manager and include rollback steps for failed cutovers.

### Webhook ingress hardening

- Only expose `POST /api/payments/webhooks/{provider}` through TLS.
- Enforce source allow-lists for provider CIDR ranges at the ingress layer.
- Require `X-Bitriver-Signature` verification and reject unsigned or mismatched requests.
- Apply body-size limits and rate limits on webhook routes distinct from user APIs.
- Keep `/metrics` and webhook routes behind separate ingress policies so scraper/network exceptions never expand webhook access.

### Reconciliation jobs

- Run a scheduled reconciliation worker (recommended every 5–15 minutes) that queries provider APIs for recently `pending` references and backfills missing webhook outcomes.
- Reconciliation should use provider event IDs and idempotency keys when writing transitions so reruns are safe.
- Alert if `pending` rows exceed your provider SLA window or if duplicate-event counters spike.
