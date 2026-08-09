# BitRiver Live Deployment Contract

## 1) Contract definition

BitRiver Live has one canonical deployment path: an operator `.env` rendered/validated from `deploy/.env.example`, then executed with `deploy/docker-compose.yml` through the `cmd/bitriver` flows (`doctor`, `env init`, `env validate`, `quickstart`, and `compose`). Any wrapper (`scripts/quickstart.sh`, platform launchers, the Ubuntu `bitriver-host` systemd manager, or direct `go run ./cmd/bitriver ...`) must converge on this same contract instead of introducing alternate runtime wiring.

## 2) Required files and roles

- `deploy/docker-compose.yml`
  - Canonical service graph and startup order.
  - Defines health checks, inter-service dependencies, migration execution (`postgres-migrations`), required env interpolation (`:?set via .env`), and baseline container hardening (public/internal network segmentation, no-new-privileges, and capability dropping with documented exceptions).
  - Current hardening exceptions are explicit and narrow: `postgres` and `redis` retain only the startup capabilities needed to hand off mounted state to their service users, and `transcoder-public` retains `CHOWN`, `SETGID`, and `SETUID` so nginx can initialize its cache directory and then drop privileges to the bundled `nginx` user under the read-only root filesystem layout.
- `deploy/docker-compose.limits.yml` (optional overlay, production-recommended)
  - Adds Docker Compose-compatible CPU/memory limits (`cpus`, `mem_limit`, `mem_reservation`) per service.
  - Activated explicitly via a second `-f` compose file or `cmd/bitriver` `--limits` flag.
- `deploy/docker-compose.monitoring.yml` (optional overlay)
  - Adds Prometheus, Alertmanager, and Grafana for observability quickstart.
  - Activated explicitly via a second `-f` compose file.
- `deploy/.env.example`
  - Canonical schema/template for environment variables.
  - Source for placeholder detection in `cmd/bitriver env validate` and seed values in `cmd/bitriver env init`.
- Root `.env`
  - Runtime source of truth consumed by Docker Compose (`env_file: ../.env`) and CLI validators.
  - Must pass `cmd/bitriver env validate` (also wrapped by `deploy/check-env.sh`).
- `deploy/install/release-assets.txt` and `scripts/stage-release-assets.sh`
  - Canonical allowlist and staging path for source-free release archives and Linux packages.
  - Include every Compose bind-mount/render dependency while excluding root `.env` and deployment-time generated OME/SRS output.
- `deploy/install/compose-host.sh` and `deploy/systemd/bitriver-live-compose.service`
  - Ubuntu archive/package lifecycle around the canonical Compose stack.
  - Separate program assets (`/opt/bitriver-live`), secret configuration (`/etc/bitriver-live`), and durable state (`/var/lib/bitriver-live`). Installation leaves the service disabled until `configure` and `activate` pass.
  - The installer persists `BITRIVER_CONFIG_ROOT=/etc/bitriver-live` plus the numeric non-root Docker operator in `BITRIVER_HOST_UID`/`BITRIVER_HOST_GID`; the packaged unit supplies the same values. Direct and systemd Compose paths therefore resolve the installed workspace's absolute `.env`, OME, and SRS symlinks and run only bind-writing services as the owner of private config/data. Older env files gain the managed keys on upgrade; duplicate active entries are rejected before any operator configuration is rewritten.
  - `srs-config`, `ome-config`, and `ome-health-token-check` mount program assets read-only. The two renderers can write only the separate config-root bind, and the SRS helper executes its CRLF-sanitized copy from container tmpfs. No extra Linux capability is granted to bypass host permissions.
- Generated files (verified in repository code paths)
  - `deploy/ome/Server.generated.xml`
    - Generated/validated by `cmd/bitriver ome render` and used by the `ome` container mount.
    - Also checked by `ome verify-health-token` during quickstart preflight.
    - The tracked repository file is rendered from `deploy/.env.example` and contains placeholders only. Release archives/packages exclude it and seed a deployment-local file from `Server.xml`; deployment-time output is sensitive and must never be uploaded as CI or release evidence.
  - `deploy/srs/conf/srs.generated.conf`
    - Referenced by `deploy/docker-compose.yml` and rendered by `scripts/render-srs-config.sh` via the `srs-config` service.
    - This file is not committed in this repository and is generated from `deploy/srs/conf/srs.conf` + `.env` by `scripts/render-srs-config.sh`. In Compose-first flows, `srs-config` always runs with `--force` before `srs` starts; in packaged systemd flow, `srs.service` also runs the same render script with `--force` in `ExecStartPre`.

## 3) Environment variable schema

Status meanings:
- **Required**: empty/missing can fail `env validate`, quickstart preflight, compose interpolation, bootstrap, or runtime health.
- **Optional**: has defaults and/or feature-gated behavior.

### Deployment paths

| Variable | Default in template/compose | Required? | What breaks if wrong |
| --- | --- | --- | --- |
| `BITRIVER_CONFIG_ROOT` | `..` (source checkout); `/etc/bitriver-live` persisted by the packaged installer and supplied by its systemd unit | Optional with topology-specific default | The packaged renderer containers cannot dereference `/workspace/.env` or generated OME/SRS symlinks, so activation stops before media services start. |
| `BITRIVER_HOST_UID` | Empty (service-specific image UID fallback); numeric Docker operator UID persisted by the packaged installer and derived by Unix launch wrappers | Optional with topology-specific default | Config helpers cannot read/write mode-restricted operator config, and the API/transcoder cannot write packaged durable bind paths. |
| `BITRIVER_HOST_GID` | Empty (service-specific image GID fallback); numeric Docker operator GID persisted by the packaged installer and derived by Unix launch wrappers | Optional with topology-specific default | Same ownership mismatch as `BITRIVER_HOST_UID`; arbitrary manual values can also create host files that the operator cannot manage. |

When either host identity value is present, `env validate` requires both values
to be unsigned 32-bit numeric IDs. Managed-install operators should rerun the
installer with the intended account instead of editing these values by hand.

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
| `BITRIVER_OME_LLHLS_HOST_PORT` | `8083` (example) / falls back to LLHLS port in compose | Optional diagnostics | Direct host OME exposure may collide or be unreachable; the supported viewer path is the BitRiver `/live/` proxy. |
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
| `BITRIVER_RELEASE_COMMIT` | `unknown` | Optional | Migration history remains usable, but newly applied rows lack immutable source-commit provenance. |
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

### G) Optional resource-limit knobs (limits overlay)

These variables are consumed by `deploy/docker-compose.limits.yml` and validated by `cmd/bitriver env validate` when set.

| Variable pattern | Default style | Required? | What breaks if wrong |
| --- | --- | --- | --- |
| `BITRIVER_*_CPUS` | decimal cores (for example `0.50`, `1.00`, `4.00`) | Optional | Invalid/non-positive values fail env validation before deploy. |
| `BITRIVER_*_MEM` | Docker Compose memory size (for example `256m`, `1g`) | Optional | Invalid memory format fails env validation; compose limits may not apply as expected. |
| `BITRIVER_*_MEM_RESERVATION` | Docker Compose memory size (for example `128m`, `512m`) | Optional | Invalid memory format fails env validation; reservation hints are ignored or rejected. |

Primary service knobs include `BITRIVER_API_*`, `BITRIVER_VIEWER_*`, `BITRIVER_POSTGRES_*`, `BITRIVER_SRS_*`, `BITRIVER_OME_*`, and `BITRIVER_TRANSCODER_*` variants declared in `deploy/.env.example`.


### I) Optional monitoring overlay knobs

These variables are consumed by `deploy/docker-compose.monitoring.yml` when set.

| Variable | Default in overlay | Required? | What breaks if wrong |
| --- | --- | --- | --- |
| `BITRIVER_PROMETHEUS_BIND` | `127.0.0.1` | Optional | Prometheus host binding/publish may fail or expose unexpectedly. |
| `BITRIVER_PROMETHEUS_HOST_PORT` | `9090` | Optional | Prometheus host port collisions or unexpected access path. |
| `BITRIVER_ALERTMANAGER_BIND` | `127.0.0.1` | Optional | Alertmanager host binding/publish may fail or expose unexpectedly. |
| `BITRIVER_ALERTMANAGER_HOST_PORT` | `9093` | Optional | Alertmanager host port collisions or unexpected access path. |
| `BITRIVER_GRAFANA_BIND` | `127.0.0.1` | Optional | Grafana host binding/publish may fail or expose unexpectedly. |
| `BITRIVER_GRAFANA_HOST_PORT` | `3001` | Optional | Grafana host port collisions or unexpected access path. |
| `BITRIVER_GRAFANA_ADMIN_USER` | `admin` | Optional | Dashboard login credentials drift from operator expectation. |
| `BITRIVER_GRAFANA_ADMIN_PASSWORD` | `admin` | Optional | Weak credentials in production if not overridden. |
| `BITRIVER_GRAFANA_DOMAIN` | `localhost` | Optional | Generated Grafana links/redirects may be incorrect. |
| `BITRIVER_GRAFANA_ROOT_URL` | `http://localhost:3001` | Optional | Reverse-proxy link generation may be incorrect. |
| `BITRIVER_PROMETHEUS_RETENTION` | `15d` | Optional | Retention too low/high for host storage capacity. |

### H) Ingest and playback/control plane

| Variable | Default in template/compose | Required? | What breaks if wrong |
| --- | --- | --- | --- |
| `BITRIVER_SRS_API` | `http://srs-controller:1985` | Optional | API-to-ingest-control calls fail. |
| `BITRIVER_SRS_TOKEN` | placeholder | Required | SRS controller auth fails; compose interpolation fails. |
| `BITRIVER_SRS_PUBLIC_RTMP_BASE_URL` | `rtmp://localhost:1935/live` in the template | Required | Creators receive an unreachable or incorrect RTMP server URL. Use the public host/IP and TCP port that reaches SRS. |
| `SRS_CONTROLLER_INTERNAL_RTMP_BASE_URL` | `rtmp://srs:1935/live` | Optional | The API/transcoder origin URL cannot reach the private SRS stream. |
| `SRS_CONTROLLER_UPSTREAM` | `http://srs:1985/api/` | Optional | The controller's SRS health probe cannot reach the upstream API. |
| `BITRIVER_INGEST_HEALTH` | `/healthz` | Optional | API ingest health probe path mismatches service endpoint. |
| `BITRIVER_OME_API` | `http://ome:8081` | Required by validator | API cannot control/query OME manager API if wrong/unreachable. |
| `BITRIVER_OME_LLHLS_ORIGIN` | `http://ome:8080` | Optional | The same-origin `/live/` edge cannot fetch OME manifests and segments. Keep this private. |
| `BITRIVER_OME_PUBLIC_LLHLS_BASE_URL` | `http://localhost:8080/live` in the template | Required | Persisted viewer playback URLs point at an unreachable origin. Production should use the public BitRiver origin plus `/live`. |
| `BITRIVER_OME_BIND` | `0.0.0.0` | Required by validator | Legacy/local OME listener binding becomes invalid. Wildcard is correct inside the canonical container. |
| `BITRIVER_OME_IP` | `0.0.0.0` | Required by validator | Top-level OME server listener becomes invalid. This is a local bind value, not the public viewer address; wildcard is correct in Compose. |
| `BITRIVER_OME_USERNAME` | `ome-operator` placeholder pattern | Required (especially when auth mode `basic`) | Basic-auth OME control flow fails if empty. |
| `BITRIVER_OME_PASSWORD` | placeholder | Required (especially when auth mode `basic`) | Basic-auth OME control flow fails if empty. |
| `BITRIVER_OME_API_TOKEN` | placeholder | Required | Quickstart preflight fails; OME render/health-token checks fail. |
| `BITRIVER_OME_HEALTHCHECK_AUTH_MODE` | `accesstoken` | Optional (defaults) | Unsupported values fail preflight/startup. |
| `BITRIVER_OME_HEALTHCHECK_TOKEN` | unset | Optional override | Token drift against rendered config can fail `ome verify-health-token`. |
| `BITRIVER_OME_RELAY_PROTOCOL` | `tcp` | Optional | Relay transport mismatch for clients/network policy. |
| `BITRIVER_OME_TCP_RELAY` | `*:3478` | Optional | TURN relay candidate construction may break. |
| `BITRIVER_OME_ICE_CANDIDATE` | `*:10000-10009/udp` | Optional | WebRTC ICE connectivity/candidate advertisement can fail. |
| `BITRIVER_TRANSCODER_API` | `http://transcoder:9000` | Optional | API transcoder job control fails. |
| `BITRIVER_TRANSCODE_LADDER` | unset (built-in defaults) | Optional | Live and VOD rendition policy changes; malformed entries are rejected by ingest configuration parsing. |
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
4. `postgres-migrations` completes successfully (compose service exits success): checksum preflight passes, only pending SQL is applied, no `applying`/`failed` rows remain, and the sanitized final ledger is logged.
5. `docker compose up` succeeds for canonical services.
6. API readiness probe succeeds at `http://127.0.0.1:<BITRIVER_LIVE_PORT>/readyz`.
7. Critical compose services report healthy: `bitriver-live`, `ome`, `srs`, `srs-controller`, `transcoder`, `postgres`, `redis`.
8. Admin bootstrap completes via `/app/bootstrap-admin` using env credentials.

SRS readiness deliberately uses the pinned upstream image's built-in Bash
`/dev/tcp` support to request `/api/v1/versions` and require HTTP 200. It must
not depend on `curl`: source mode's local wrapper installs that tool, but
production pull mode runs the immutable upstream digest directly. The Compose
and Helm probes share this curl-free contract.

For the Ubuntu artifact install, `bitriver-host activate` is the systemd entrypoint for the same sequence. The unit is bounded to 15 minutes and remains failed if quickstart or critical health does not pass. Installation paths are different, but the Compose graph and validation contract are not.

These startup checks prove the control plane is ready; they do not prove media delivery. Production acceptance additionally requires a bounded RTMP publish, authenticated validation of OME's declared `default/live` application, a successful public `/live/<channel-id>/llhls.m3u8` fetch and audio/video decode, advertised rendition-manifest checks, and a clean offline transition after the publisher stops.

Tagged Ubuntu acceptance additionally requires host reboot/recovery evidence. A green unauthenticated OME root probe alone is not sufficient for release approval.

Migration safety is part of deployment health, not a best-effort startup step. The canonical runner at `deploy/postgres-migrate.sh` owns `public.schema_migrations`, identifies migrations by complete filename, records raw SHA-256 plus release provenance, and refuses edited/removed history or ambiguous state. Compose mounts this runner directly; Helm packages a generated byte-identical runner and byte-identical SQL copies. Use `go run ./cmd/bitriver migrations --mode plan|status` for read-only inspection and follow `docs/upgrades.md` for checksum-confirmed recovery. A failed migration job prevents the API dependency from starting and the deployment must not be reported healthy.

## 4.1) Post-deploy verification

Run from repo root (or use `bitriver smoke` in packaged installs):

```bash
docker compose --env-file ./.env -f deploy/docker-compose.yml ps
curl -fsS "http://127.0.0.1:${BITRIVER_LIVE_PORT:-8080}/readyz"
curl -fsS "http://127.0.0.1:${BITRIVER_LIVE_PORT:-8080}/healthz"
curl -fsS "http://127.0.0.1:${BITRIVER_SRS_CONTROLLER_PORT:-1986}/healthz"
curl -fsS "http://127.0.0.1:${BITRIVER_TRANSCODER_HOST_PORT:-9001}/healthz"
curl -fsS -o /dev/null -w "%{http_code}\n" "http://127.0.0.1:${BITRIVER_OME_HTTP_PORT:-8081}/"
go run ./cmd/bitriver smoke --compose-file deploy/docker-compose.yml --env-file ./.env
```

`bitriver smoke` is the canonical single-command equivalent and checks compose reachability plus the same host health endpoints.

For a media acceptance run, publish a non-sensitive test source with a freshly
created channel key, then verify the viewer-facing URLs returned by the API. At
minimum, require the same-origin OME manifest and every advertised transcoder
manifest to return successfully, decode several seconds of audio and video, and
confirm stop/unpublish clears the live state. Never place the stream key in
logs, screenshots, issue bodies, or CI artifacts.

## 5) Out of scope / not guaranteed by this contract

The canonical contract does **not** guarantee equivalence for:

- Advanced/alternative topologies (compose override stacks, custom reverse-proxy layering, split-host or multi-node topologies).
- Kubernetes/Helm behavior parity with compose-first quickstart semantics.
- Native per-binary systemd layouts as an equivalent to the canonical Compose graph. The supported Ubuntu unit only wraps Compose.
- Roadmap/proposed deployment approaches not implemented in `deploy/docker-compose.yml`, `deploy/.env.example`, and `cmd/bitriver` quickstart/doctor/env flows.

## 6) Pointers to relevant existing docs

- `docs/installing-on-ubuntu.md` - artifact-only Ubuntu/XOA host lifecycle and reverse-proxy boundaries.
- `docs/quickstart.md` — operator entrypoints and canonical quickstart stage narrative.
- `docs/advanced-deployments.md` — non-default deployment patterns and caveats.
- `docs/production-release.md` — release and production operational expectations.
- `docs/testing.md` — test commands expected before release.
- `deploy/README.md` — deployment asset map and environment notes.
- `deploy/check-env.sh` — wrapper invoking `cmd/bitriver env validate`.

## Generated environment variable index

<!-- BEGIN GENERATED ENV -->

_This section is generated from `deploy/.env.example` by `scripts/generate-contract-doc.sh`. Do not edit by hand._

### `BITRIVER_*`

| Variable | Default |
| --- | --- |
| `BITRIVER_DEPLOY_IMAGE_SOURCE` | `pull` |
| `BITRIVER_CONFIG_ROOT` | `..` |
| `BITRIVER_HOST_UID` | _(empty)_ |
| `BITRIVER_HOST_GID` | _(empty)_ |
| `BITRIVER_IMAGE_NAMESPACE` | `ghcr.io/prohibitedtv` |
| `BITRIVER_LIVE_IMAGE_TAG` | `v1.2.3` |
| `BITRIVER_RELEASE_COMMIT` | `unknown` |
| `BITRIVER_VIEWER_IMAGE_TAG` | `v1.2.3` |
| `BITRIVER_SRS_CONTROLLER_IMAGE_TAG` | `v1.2.3` |
| `BITRIVER_TRANSCODER_IMAGE_TAG` | `v1.2.3` |
| `BITRIVER_OME_CONFIG_IMAGE_TAG` | `v1.2.3` |
| `BITRIVER_SRS_IMAGE_TAG` | `v5.0.185` |
| `BITRIVER_OME_IMAGE_TAG` | `0.16.0` |
| `BITRIVER_LIVE_IMAGE_DIGEST` | _(empty)_ |
| `BITRIVER_VIEWER_IMAGE_DIGEST` | _(empty)_ |
| `BITRIVER_SRS_CONTROLLER_IMAGE_DIGEST` | _(empty)_ |
| `BITRIVER_TRANSCODER_IMAGE_DIGEST` | _(empty)_ |
| `BITRIVER_OME_CONFIG_IMAGE_DIGEST` | _(empty)_ |
| `BITRIVER_SRS_IMAGE_DIGEST` | _(empty)_ |
| `BITRIVER_OME_IMAGE_DIGEST` | _(empty)_ |
| `BITRIVER_REDIS_IMAGE_DIGEST` | _(empty)_ |
| `BITRIVER_POSTGRES_IMAGE_DIGEST` | _(empty)_ |
| `BITRIVER_NGINX_IMAGE_DIGEST` | _(empty)_ |
| `BITRIVER_ALPINE_3_IMAGE_DIGEST` | _(empty)_ |
| `BITRIVER_ALPINE_3_19_IMAGE_DIGEST` | _(empty)_ |
| `BITRIVER_DEBIAN_IMAGE_DIGEST` | _(empty)_ |
| `BITRIVER_LIVE_PORT` | `8080` |
| `BITRIVER_API_CPUS` | `1.50` |
| `BITRIVER_API_MEM` | `1g` |
| `BITRIVER_API_MEM_RESERVATION` | `768m` |
| `BITRIVER_VIEWER_CPUS` | `0.50` |
| `BITRIVER_VIEWER_MEM` | `512m` |
| `BITRIVER_VIEWER_MEM_RESERVATION` | `256m` |
| `BITRIVER_REDIS_CPUS` | `0.50` |
| `BITRIVER_REDIS_MEM` | `256m` |
| `BITRIVER_REDIS_MEM_RESERVATION` | `128m` |
| `BITRIVER_POSTGRES_CPUS` | `1.00` |
| `BITRIVER_POSTGRES_MEM` | `1g` |
| `BITRIVER_POSTGRES_MEM_RESERVATION` | `512m` |
| `BITRIVER_POSTGRES_MIGRATIONS_CPUS` | `0.50` |
| `BITRIVER_POSTGRES_MIGRATIONS_MEM` | `512m` |
| `BITRIVER_POSTGRES_MIGRATIONS_MEM_RESERVATION` | `256m` |
| `BITRIVER_POSTGRES_HOST_PORT_CPUS` | `0.25` |
| `BITRIVER_POSTGRES_HOST_PORT_MEM` | `128m` |
| `BITRIVER_POSTGRES_HOST_PORT_MEM_RESERVATION` | `64m` |
| `BITRIVER_SRS_CONTROLLER_CPUS` | `0.50` |
| `BITRIVER_SRS_CONTROLLER_MEM` | `512m` |
| `BITRIVER_SRS_CONTROLLER_MEM_RESERVATION` | `256m` |
| `BITRIVER_SRS_CPUS` | `2.00` |
| `BITRIVER_SRS_MEM` | `2g` |
| `BITRIVER_SRS_MEM_RESERVATION` | `1g` |
| `BITRIVER_SRS_API_CPUS` | `0.25` |
| `BITRIVER_SRS_API_MEM` | `128m` |
| `BITRIVER_SRS_API_MEM_RESERVATION` | `64m` |
| `BITRIVER_SRS_CONFIG_CPUS` | `0.25` |
| `BITRIVER_SRS_CONFIG_MEM` | `128m` |
| `BITRIVER_SRS_CONFIG_MEM_RESERVATION` | `64m` |
| `BITRIVER_OME_CONFIG_CPUS` | `0.25` |
| `BITRIVER_OME_CONFIG_MEM` | `128m` |
| `BITRIVER_OME_CONFIG_MEM_RESERVATION` | `64m` |
| `BITRIVER_OME_HEALTH_TOKEN_CHECK_CPUS` | `0.25` |
| `BITRIVER_OME_HEALTH_TOKEN_CHECK_MEM` | `128m` |
| `BITRIVER_OME_HEALTH_TOKEN_CHECK_MEM_RESERVATION` | `64m` |
| `BITRIVER_OME_CPUS` | `4.00` |
| `BITRIVER_OME_MEM` | `4g` |
| `BITRIVER_OME_MEM_RESERVATION` | `2g` |
| `BITRIVER_TRANSCODER_CPUS` | `4.00` |
| `BITRIVER_TRANSCODER_MEM` | `4g` |
| `BITRIVER_TRANSCODER_MEM_RESERVATION` | `2g` |
| `BITRIVER_TRANSCODER_PUBLIC_CPUS` | `0.50` |
| `BITRIVER_TRANSCODER_PUBLIC_MEM` | `256m` |
| `BITRIVER_TRANSCODER_PUBLIC_MEM_RESERVATION` | `128m` |
| `BITRIVER_LIVE_STORAGE_DRIVER` | `postgres` |
| `BITRIVER_PGX_MODE` | `real` |
| `BITRIVER_LIVE_MODE` | `production` |
| `BITRIVER_LIVE_ADDR` | `:8080` |
| `BITRIVER_LIVE_ALLOW_SELF_SIGNUP` | `false` |
| `BITRIVER_LIVE_METRICS_TOKEN` | `metrics-collector-token` |
| `BITRIVER_LIVE_RATE_LOGIN_LIMIT` | `10` |
| `BITRIVER_LIVE_RATE_LOGIN_WINDOW` | `1m` |
| `BITRIVER_LIVE_RATE_TRUST_FORWARDED_HEADERS` | `false` |
| `BITRIVER_LIVE_RATE_TRUSTED_PROXIES` | _(empty)_ |
| `BITRIVER_LIVE_UPLOADS_TRUST_FORWARDED_HEADERS` | `false` |
| `BITRIVER_LIVE_UPLOAD_MEDIA_BASE_URL` | _(empty)_ |
| `BITRIVER_POSTGRES_DB` | `brlive_app` |
| `BITRIVER_POSTGRES_USER` | `brlive_app` |
| `BITRIVER_POSTGRES_PASSWORD` | `P0stgres-Example!` |
| `BITRIVER_REDIS_PASSWORD` | `R3dis-Example!` |
| `BITRIVER_LIVE_POSTGRES_MAX_CONNS` | `15` |
| `BITRIVER_LIVE_POSTGRES_MIN_CONNS` | `5` |
| `BITRIVER_LIVE_POSTGRES_ACQUIRE_TIMEOUT` | `5s` |
| `BITRIVER_LIVE_POSTGRES_MAX_CONN_LIFETIME` | `30m` |
| `BITRIVER_LIVE_SESSION_STORE` | `postgres` |
| `BITRIVER_LIVE_SESSION_TTL` | `168h` |
| `BITRIVER_LIVE_CHAT_QUEUE_DRIVER` | `redis` |
| `BITRIVER_LIVE_CHAT_QUEUE_REDIS_ADDR` | `redis:6379` |
| `BITRIVER_LIVE_CHAT_QUEUE_REDIS_STREAM` | `bitriver-live-chat` |
| `BITRIVER_LIVE_CHAT_QUEUE_REDIS_GROUP` | `bitriver-live-api` |
| `BITRIVER_POSTGRES_HOST_PORT` | `5432` |
| `BITRIVER_VIEWER_ORIGIN` | `http://viewer:3000` |
| `BITRIVER_SRS_API` | `http://srs-controller:1985` |
| `BITRIVER_SRS_PUBLIC_RTMP_BASE_URL` | `rtmp://localhost:1935/live` |
| `BITRIVER_OME_API` | `http://ome:8081` |
| `BITRIVER_OME_BIND` | `0.0.0.0` |
| `BITRIVER_OME_HTTP_PORT` | `8081` |
| `BITRIVER_OME_HTTP_TLS_PORT` | `8082` |
| `BITRIVER_OME_IP` | `0.0.0.0` |
| `BITRIVER_OME_SIGNALLING_PORT` | `9000` |
| `BITRIVER_OME_SERVER_PORT` | `9000` |
| `BITRIVER_OME_SERVER_TLS_PORT` | `9443` |
| `BITRIVER_OME_LLHLS_PORT` | `8080` |
| `BITRIVER_OME_LLHLS_TLS_PORT` | `8443` |
| `BITRIVER_OME_LLHLS_HOST_PORT` | `8083` |
| `BITRIVER_OME_LLHLS_ORIGIN` | `http://ome:8080` |
| `BITRIVER_OME_PUBLIC_LLHLS_BASE_URL` | `http://localhost:8080/live` |
| `BITRIVER_OME_RELAY_PORT` | `3478` |
| `BITRIVER_OME_RELAY_PROTOCOL` | `tcp` |
| `BITRIVER_OME_TCP_RELAY` | `*:3478` |
| `BITRIVER_OME_ICE_PORT_RANGE` | `10000-10009` |
| `BITRIVER_OME_ICE_CANDIDATE` | `*:10000-10009/udp` |
| `BITRIVER_TRANSCODER_API` | `http://transcoder:9000` |
| `BITRIVER_TRANSCODE_LADDER` | _(empty)_ |
| `BITRIVER_TRANSCODER_PUBLIC_BASE_URL` | `https://cdn.example.com/hls` |
| `BITRIVER_TRANSCODER_HOST_PORT` | `9001` |
| `BITRIVER_INGEST_HEALTH` | `/healthz` |
| `BITRIVER_SRS_CONTROLLER_PORT` | `1986` |
| `BITRIVER_LIVE_ADMIN_CORS_ORIGINS` | `https://admin.example.com` |
| `BITRIVER_LIVE_VIEWER_CORS_ORIGINS` | `https://watch.example.com` |
| `BITRIVER_PUBLIC_DOMAIN` | `stream.example.com` |
| `BITRIVER_TLS_EMAIL` | `admin@stream.example.com` |
| `BITRIVER_LIVE_ADMIN_EMAIL` | `admin@stream.example.com` |
| `BITRIVER_LIVE_ADMIN_PASSWORD` | `Sup3rSecureAdmin-Example!` |
| `BITRIVER_SRS_TOKEN` | `srs-secure-token-example` |
| `BITRIVER_OME_USERNAME` | `ome-operator` |
| `BITRIVER_OME_PASSWORD` | `OME-Example-Pass!` |
| `BITRIVER_OME_API_TOKEN` | `OME-Example-Access-Token` |
| `BITRIVER_OME_HEALTHCHECK_AUTH_MODE` | `accesstoken` |
| `BITRIVER_TRANSCODER_TOKEN` | `transcoder-secure-token-example` |
| `BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD` | `R3dis-Example!` |

### `SRS_*`

| Variable | Default |
| --- | --- |
| `SRS_CONTROLLER_UPSTREAM` | `http://srs:1985/api/` |
| `SRS_CONTROLLER_INTERNAL_RTMP_BASE_URL` | `rtmp://srs:1935/live` |

### `NEXT_PUBLIC_*`

| Variable | Default |
| --- | --- |
| `NEXT_PUBLIC_API_BASE_URL` | _(empty)_ |
| `NEXT_PUBLIC_VIEWER_URL` | `https://stream.example.com/viewer` |

### `NEXT_*`

| Variable | Default |
| --- | --- |
| `NEXT_VIEWER_BASE_PATH` | `/viewer` |

<!-- END GENERATED ENV -->
