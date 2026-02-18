# BitRiver Live Deployment Contract

## 1) Contract definition

BitRiver Live has one canonical deployment path: the root `.env` rendered/validated from `deploy/.env.example`, then executed with `deploy/docker-compose.yml` through the `cmd/bitriver` flows (`doctor`, `env init`, `env validate`, `quickstart`, and `compose`). Any wrapper (`scripts/quickstart.sh`, platform launchers, or direct `go run ./cmd/bitriver ...`) is expected to converge on this same contract instead of introducing alternate runtime wiring.

## 2) Required files and roles

- `deploy/docker-compose.yml`
  - Canonical service graph and startup order.
  - Defines health checks, inter-service dependencies, migration execution (`postgres-migrations`), and required env interpolation (`:?set via .env`).
- `deploy/.env.example`
  - Canonical schema/template for environment variables.
  - Source for placeholder detection in `cmd/bitriver env validate` and seed values in `cmd/bitriver env init`.
- Root `.env`
  - Runtime source of truth consumed by Docker Compose (`env_file: ../.env`) and CLI validators.
  - Must pass `cmd/bitriver env validate` (also wrapped by `deploy/check-env.sh`).
- Generated files (verified in repository code paths)
  - `deploy/ome/Server.generated.xml`
    - Generated/validated by `cmd/bitriver ome render` and used by the `ome` container mount.
    - Also checked by `ome verify-health-token` during quickstart preflight.
  - `deploy/srs/conf/srs.generated.conf`
    - Referenced by `deploy/docker-compose.yml` and rendered by `scripts/render-srs-config.sh` via the `srs-config` service.
    - TODO (needs verification): confirm whether this file is intentionally always runtime-generated (not committed) for every supported packaging path.

## 3) Environment variable schema

Status meanings:
- **Required**: empty/missing can fail `env validate`, quickstart preflight, compose interpolation, bootstrap, or runtime health.
- **Optional**: has defaults and/or feature-gated behavior.

### A) Ports

| Variable | Default in template/compose | Required? | What breaks if wrong |
| --- | --- | --- | --- |
| `BITRIVER_LIVE_PORT` | `8080` | Optional | API/readyz reachability from host; quickstart readiness probe can target wrong port if mismatched with published mapping. |
| `BITRIVER_LIVE_ADDR` | `:8080` | Optional | API bind mismatch can break container startup/listeners. |
| `BITRIVER_SRS_CONTROLLER_PORT` | `1986` | Optional | Host access to SRS controller endpoint becomes unavailable/unexpected. |
| `BITRIVER_SRS_RTMP_PORT` | `1935` (compose default) | Optional | RTMP ingest clients cannot publish to expected host port. |
| `BITRIVER_POSTGRES_HOST_PORT` | `5432` | Optional (profile-gated) | Only affects `postgres-host` profile publishing. |
| `BITRIVER_OME_HTTP_PORT` | `8081` | Required (validate + compose use) | OME API/health endpoint mapping mismatch; API health + control path may fail. |
| `BITRIVER_OME_HTTP_TLS_PORT` | `8082` | Optional | TLS API mapping mismatch for OME manager endpoint. |
| `BITRIVER_OME_LLHLS_PORT` | `8080` | Optional | LL-HLS playback publication path may be unreachable. |
| `BITRIVER_OME_LLHLS_HOST_PORT` | `8083` (example) / falls back to LLHLS port in compose | Optional | Host LL-HLS exposure may collide or be unreachable. |
| `BITRIVER_OME_LLHLS_TLS_PORT` | `8443` | Optional | TLS LL-HLS playback path unavailable. |
| `BITRIVER_OME_SIGNALLING_PORT` | `9000` | Optional | Host mapping for signalling can drift from expected external port. |
| `BITRIVER_OME_SERVER_PORT` | `9000` | Required (validate) | OME signalling bind/config invalid; validation fails or signalling breaks. |
| `BITRIVER_OME_SERVER_TLS_PORT` | `9443` | Required (validate) | TLS signalling bind/config invalid. |
| `BITRIVER_OME_RELAY_PORT` | `3478` | Optional | TURN/relay path for WebRTC can fail. |
| `BITRIVER_OME_ICE_PORT_RANGE` | `10000-10009` | Optional | ICE candidate connectivity can fail if range mismatched/blocked. |
| `BITRIVER_TRANSCODER_HOST_PORT` | `9001` | Optional | Host access to transcoder controller API changes/breaks. |
| `BITRIVER_TRANSCODER_PUBLIC_PORT` | `9080` | Optional | Host publication of transcoder output via nginx sidecar changes/breaks. |

### B) Viewer URLs and viewer/API routing

| Variable | Default in template/compose | Required? | What breaks if wrong |
| --- | --- | --- | --- |
| `BITRIVER_VIEWER_ORIGIN` | `http://viewer:3000` | Optional | API-to-viewer origin assumptions fail (proxy/callback links). |
| `NEXT_PUBLIC_API_BASE_URL` | empty | Optional | Viewer may call wrong API origin; if loopback/example URL in production, `env validate` fails. |
| `NEXT_VIEWER_BASE_PATH` | `/viewer` | Optional | Viewer routing/base path mismatches reverse-proxy expectations. |
| `NEXT_PUBLIC_VIEWER_URL` | `https://stream.example.com/viewer` in template; `http://localhost:8080/viewer` generated for local init when placeholder | Required (`env validate`) | Viewer links/UI origins become incorrect; placeholder/example/loopback can fail validation in production posture. |

### C) Admin bootstrap and security bootstrap

| Variable | Default in template/compose | Required? | What breaks if wrong |
| --- | --- | --- | --- |
| `BITRIVER_LIVE_ADMIN_EMAIL` | `admin@stream.example.com` placeholder | Required | Quickstart bootstrap-admin step cannot seed admin account; placeholder is blocked. |
| `BITRIVER_LIVE_ADMIN_PASSWORD` | placeholder | Required | Admin bootstrap fails or creates insecure known credential if not rotated. |
| `BITRIVER_LIVE_ALLOW_SELF_SIGNUP` | `false` | Required (`env validate`) | Invalid boolean fails validation; wrong value changes signup behavior. |
| `BITRIVER_LIVE_METRICS_TOKEN` | `metrics-collector-token` placeholder | Conditionally required (or allowlist) | Unprotected `/metrics` rejected by validation in production contract. |
| `BITRIVER_LIVE_METRICS_ALLOW_NETWORKS` | unset | Optional alternative to token | If both token + allowlist empty in production, validation fails. |
| `BITRIVER_LIVE_RATE_LOGIN_LIMIT` | `10` | Required in production contract | Validation fails when unset/zero/non-integer in production mode. |
| `BITRIVER_LIVE_RATE_LOGIN_WINDOW` | `1m` | Optional in schema (used when throttling enabled) | Bad value can weaken/disable intended rate limit behavior. |
| `BITRIVER_LIVE_MODE` | `production` | Required | `development` or empty is rejected by validator for persisted `.env`. |

### D) Postgres

| Variable | Default in template/compose | Required? | What breaks if wrong |
| --- | --- | --- | --- |
| `BITRIVER_POSTGRES_DB` | `brlive_app` (template), compose fallback `bitriver` where referenced directly | Optional (but effectively required for consistent DSN) | DB selection mismatch can break migrations/bootstrap connection assumptions. |
| `BITRIVER_POSTGRES_USER` | `brlive_app` | Required | Compose interpolation/migrations/bootstrapping fail. |
| `BITRIVER_POSTGRES_PASSWORD` | placeholder | Required | Postgres service, migrations, and API DB connections fail. |
| `BITRIVER_LIVE_POSTGRES_DSN` | Derived by compose if unset | Optional | Invalid DSN causes API startup/storage failure. |
| `BITRIVER_LIVE_POSTGRES_MAX_CONNS` | `15` | Optional | Connection pool sizing issues (exhaustion/underutilization). |
| `BITRIVER_LIVE_POSTGRES_MIN_CONNS` | `5` | Optional | Pool warmup/throughput behavior degrades. |
| `BITRIVER_LIVE_POSTGRES_ACQUIRE_TIMEOUT` | `5s` | Optional | DB acquisition failures/timeouts if too aggressive. |
| `BITRIVER_LIVE_POSTGRES_MAX_CONN_LIFETIME` | `30m` | Optional | Connection churn/staleness behavior changes. |
| `BITRIVER_LIVE_SESSION_STORE` | `postgres` | Required by validation list | Session persistence wiring invalid if unsupported/empty. |
| `BITRIVER_LIVE_SESSION_TTL` | `168h` | Required by validation list | Session expiration semantics break or validation fails when empty. |
| `BITRIVER_LIVE_SESSION_IDLE_TIMEOUT` | `0` | Optional | Idle expiry policy changes. |

### E) Redis and chat queue

| Variable | Default in template/compose | Required? | What breaks if wrong |
| --- | --- | --- | --- |
| `BITRIVER_REDIS_PASSWORD` | placeholder | Required | Redis container auth and queue connectivity fail. |
| `BITRIVER_LIVE_CHAT_QUEUE_DRIVER` | `redis` | Optional (current contract expects redis path) | Chat queue backend mismatch may break chat pipeline. |
| `BITRIVER_LIVE_CHAT_QUEUE_REDIS_ADDR` | `redis:6379` | Optional | API cannot connect to Redis queue. |
| `BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD` | placeholder (expected to match Redis password unless intentional override) | Required | API chat queue auth fails; compose also marks required. |
| `BITRIVER_LIVE_CHAT_QUEUE_REDIS_STREAM` | `bitriver-live-chat` | Optional | Producers/consumers diverge from expected stream. |
| `BITRIVER_LIVE_CHAT_QUEUE_REDIS_GROUP` | `bitriver-live-api` | Optional | Consumer-group behavior diverges; message handling issues. |

### F) Ingest and playback/control plane

| Variable | Default in template/compose | Required? | What breaks if wrong |
| --- | --- | --- | --- |
| `BITRIVER_SRS_API` | `http://srs-controller:1985` | Optional | API-to-ingest-control calls fail. |
| `BITRIVER_SRS_TOKEN` | placeholder | Required | SRS controller auth fails; compose interpolation fails. |
| `SRS_CONTROLLER_UPSTREAM` | `http://srs:1985/api/` | Optional | Controller cannot proxy to SRS upstream API. |
| `BITRIVER_INGEST_HEALTH` | `/healthz` | Optional | API ingest health probe path mismatches service endpoint. |
| `BITRIVER_OME_API` | `http://ome:8081` | Required by validator | API cannot control/query OME manager API if wrong/unreachable. |
| `BITRIVER_OME_BIND` | `0.0.0.0` | Required by validator | OME binding contract invalid; validator rejects loopback placeholder in production. |
| `BITRIVER_OME_IP` | `0.0.0.0` | Required by validator | OME advertised/public endpoint invalid; validator rejects placeholder/loopback for production. |
| `BITRIVER_OME_USERNAME` | `ome-operator` placeholder pattern | Required (especially when auth mode `basic`) | Basic-auth OME control flow fails if empty. |
| `BITRIVER_OME_PASSWORD` | placeholder | Required (especially when auth mode `basic`) | Basic-auth OME control flow fails if empty. |
| `BITRIVER_OME_API_TOKEN` | placeholder | Required | Quickstart preflight fails; OME render/health-token checks fail. |
| `BITRIVER_OME_HEALTHCHECK_AUTH_MODE` | `accesstoken` | Optional (defaults) | Unsupported values fail preflight/startup. |
| `BITRIVER_OME_HEALTHCHECK_TOKEN` | unset | Optional override | Token drift against rendered config can fail `ome verify-health-token`. |
| `BITRIVER_OME_RELAY_PROTOCOL` | `tcp` | Optional | Relay transport mismatch for clients/network policy. |
| `BITRIVER_OME_TCP_RELAY` | `*:3478` | Optional | TURN relay candidate construction may break. |
| `BITRIVER_OME_ICE_CANDIDATE` | `*:10000-10009/udp` | Optional | WebRTC ICE connectivity/candidate advertisement can fail. |
| `BITRIVER_TRANSCODER_API` | `http://transcoder:9000` | Optional | API transcoder job control fails. |
| `BITRIVER_TRANSCODER_TOKEN` | placeholder | Required | Transcoder job-controller auth fails. |
| `BITRIVER_TRANSCODER_PUBLIC_BASE_URL` | `https://cdn.example.com/hls` placeholder (template), auto-localized by `env init` when placeholder | Required | Validator blocks placeholder/loopback in production; published playback URLs become unreachable if wrong. |
| `BITRIVER_TRANSCODER_PUBLIC_DIR` | `/work/public` (compose) | Optional | Artifact path mismatch between transcoder and nginx sidecar. |
| `BITRIVER_TRANSCODER_RETENTION_LIVE` | unset | Optional | Retention cleanup behavior changes. |
| `BITRIVER_TRANSCODER_RETENTION_UPLOADS` | unset | Optional | Retention cleanup behavior changes. |

## 4) Golden path: definition of successful deployment

A deployment is considered successful when all of the following are true in the canonical quickstart flow (`cmd/bitriver quickstart`):

1. `doctor` passes (`docker` and `docker compose version` checks).
2. `.env` exists and passes `env validate` (including production contract checks and placeholder rejection).
3. OME auth preflight succeeds:
   - `ome render --force` regenerates `deploy/ome/Server.generated.xml`.
   - `ome verify-health-token` confirms rendered token and runtime token precedence agree.
4. `postgres-migrations` completes successfully (compose service exits success).
5. `docker compose up` succeeds for canonical services.
6. API readiness probe succeeds at `http://127.0.0.1:<BITRIVER_LIVE_PORT>/readyz`.
7. Critical compose services report healthy: `bitriver-live`, `ome`, `srs`, `srs-controller`, `transcoder`, `postgres`, `redis`.
8. Admin bootstrap completes via `/app/bootstrap-admin` using env credentials.

TODO (needs verification): document a single post-deploy smoke command set for operators who do not run `cmd/bitriver quickstart` directly.

## 5) Out of scope / not guaranteed by this contract

The canonical contract does **not** guarantee equivalence for:

- Advanced/alternative topologies (compose override stacks, custom reverse-proxy layering, split-host or multi-node topologies).
- Kubernetes/Helm behavior parity with compose-first quickstart semantics.
- Bare-metal/systemd installs as a canonical release path (they exist, but are separate operational guidance).
- Roadmap/proposed deployment approaches not implemented in `deploy/docker-compose.yml`, `deploy/.env.example`, and `cmd/bitriver` quickstart/doctor/env flows.

## 6) Pointers to relevant existing docs

- `docs/quickstart.md` — operator entrypoints and canonical quickstart stage narrative.
- `docs/advanced-deployments.md` — non-default deployment patterns and caveats.
- `docs/production-release.md` — release and production operational expectations.
- `docs/testing.md` — test commands expected before release.
- `deploy/README.md` — deployment asset map and environment notes.
- `deploy/check-env.sh` — wrapper invoking `cmd/bitriver env validate`.
