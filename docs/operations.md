# Operations runbook

This guide focuses on day-two operations for the default Docker Compose stack in `deploy/docker-compose.yml`. It references the
Compose service names (`postgres`, `redis`, `transcoder`) and the host-mounted volumes so you can lift and restore state safely
without breaking the one-command deployment flow.

## Observability (metrics, alerts, logs)

Use `/api/status` as the operator-friendly view (aggregates readiness, dependency checks, ingest probes, and remediation
links), and `/readyz` + `/healthz` for automation. `/readyz` should gate load balancers and orchestration because it only
fails when core dependencies (API, Postgres, Redis) are unhealthy. `/healthz` includes ingest checks (SRS/OME/transcoder)
without flipping non-core failures into HTTP errors, so you can monitor ingest drift without tearing down the API. Use
`/api/status` when you need richer JSON and remediation hints. Also see the health appendix in `docs/quickstart.md`.

### Health endpoints and monitoring usage

Use the endpoints below as tiered signals: `/readyz` for gating deploys and load balancers, `/healthz` for ingest visibility,
and `/api/status` for operator-facing summaries with remediation tips and log hints.

- **`GET /readyz` (readiness):** Returns HTTP `200` when core dependencies (API, Postgres, Redis, rate limiting, chat queue)
  are healthy. Returns HTTP `503` when those dependencies fail, which is the signal to drain traffic or fail a rollout.
- **`GET /healthz` (dependency visibility):** Mirrors `/readyz` for core dependencies and adds ingest component status for
  SRS/OME/transcoder. It only flips to HTTP `503` when core dependencies fail, so ingest-only failures still produce `200`
  with degraded JSON payloads. Use this for dashboards and on-call triage.
- **`GET /api/status` (operator summary):** Aggregates readiness plus ingest probes, remediation hints, and log suggestions
  used in the control centre Overview. Use this endpoint when you want a human-readable payload for alerts or ChatOps.

**Typical monitoring flows:**

- **Load balancer / orchestration probe:** Poll `/readyz` every 10-30s; alert on `503` for 2-3 consecutive checks and remove
  the API instance from rotation until it returns `200`.
- **Ingest service watch:** Poll `/healthz` every 30-60s; alert when any ingest component reports `error` for 3 checks while
  `/readyz` remains healthy. This keeps the API online while surfacing ingest degradation.
- **Operator dashboards:** Poll `/api/status` for an at-a-glance summary, and link to the referenced logs when a component
  is degraded.

### Key metrics to track

- **API latency** (p50/p95/p99 by route, especially `/api/status`, auth, channel CRUD).
- **API error rate** (5xx, 4xx spikes from rate limiting or auth).
- **HTTP volume:** `bitriver_http_requests_total{method,path,status}` with latency via
  `bitriver_http_request_duration_seconds_sum`/`count` (paths are normalised to `:id`).
- **Ingest health & orchestration:** `bitriver_ingest_health{service,status}` gauges alongside
  `bitriver_ingest_attempts_total{operation}` and `bitriver_ingest_failures_total{operation}`.
- **Transcoder workload:** `bitriver_transcoder_jobs_total{kind,status}` counters and
  `bitriver_transcoder_active_jobs` gauge.
- **Viewer QoE events:** `bitriver_viewer_qoe_events_total{event,player,protocol,rendition,latency_mode}` to catch buffering,
  error, and rendition-change spikes.
- **Chat backlog** (Redis stream length and consumer lag).
- **Dependency saturation** (Postgres connection usage, Redis memory usage, disk usage for `./transcoder-data`).

### Tracing (OpenTelemetry)

The API and transcoder emit OpenTelemetry-style spans for HTTP requests, ingest/transcode orchestration, and viewer QoE
events. Configure an OTLP collector and sampling rate via flags or environment variables.

**API flags (cmd/server):**

- `--otel-endpoint` (env: `BITRIVER_LIVE_OTEL_EXPORTER_OTLP_ENDPOINT`, or `OTEL_EXPORTER_OTLP_ENDPOINT`): OTLP HTTP endpoint.
- `--otel-sample-ratio` (env: `BITRIVER_LIVE_OTEL_SAMPLE_RATIO`, or `OTEL_TRACES_SAMPLER_ARG`): sampling ratio (0.0–1.0).

**Transcoder env vars (cmd/transcoder):**

- `BITRIVER_LIVE_OTEL_EXPORTER_OTLP_ENDPOINT` (or `OTEL_EXPORTER_OTLP_ENDPOINT`)
- `BITRIVER_LIVE_OTEL_SAMPLE_RATIO` (or `OTEL_TRACES_SAMPLER_ARG`)

**Viewer QoE collection:**

The viewer posts playback events to `POST /api/metrics/qoe` and increments the QoE counters above. Use the trace context
header (`traceparent`) to correlate client-side playback with API spans when your proxy forwards headers end-to-end.

### Recommended alerts + thresholds

Tune to your traffic profile, but the defaults below are a good starting point for the Compose stack:

- **API latency:** p95 > 500ms for 5 minutes (warning), p95 > 1s for 10 minutes (critical).
- **API error rate:** 5xx > 1% for 5 minutes (warning), > 5% for 5 minutes (critical).
- **Ingest health:** SRS/OME/transcoder probe failures for 3 consecutive checks (warning); > 5 minutes (critical).
- **Transcoder failure rate:** > 2% failed jobs in 10 minutes (warning), > 5% in 10 minutes (critical).
- **Chat backlog:** Redis stream length grows continuously for 10 minutes or consumer lag > 1000 messages (warning).
- **Storage:** `./transcoder-data` > 80% full (warning), > 90% (critical).

### Monitoring stack (Prometheus + Alertmanager + Grafana)

The monitoring profile adds Prometheus, Alertmanager, node-exporter, redis-exporter, and Grafana alongside the default Compose stack.

1. Set the API metrics token in `.env` (required in production mode):
   ```bash
   BITRIVER_LIVE_METRICS_TOKEN=replace-with-strong-token
   ```
2. Create a Prometheus token file that matches the API token:
   ```bash
   cp deploy/monitoring/metrics.token.example deploy/monitoring/metrics.token
   ```
   Edit `deploy/monitoring/metrics.token` so it contains the same value as `BITRIVER_LIVE_METRICS_TOKEN` (single line).
3. Create Alertmanager receiver secrets and render the runtime config:
   ```bash
   cp deploy/monitoring/alertmanager.env.example deploy/monitoring/alertmanager.env
   ./scripts/render-alertmanager-config.sh
   ```
4. Validate monitoring configs before deployment:
   ```bash
   ./scripts/check-monitoring-config.sh
   ```
5. Start the monitoring profile:
   ```bash
   docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.monitoring.yml --profile monitoring up -d
   ```

Prometheus listens on `http://localhost:9090`, Alertmanager on `http://localhost:9093`, and Grafana on
`http://localhost:3001` (default credentials are `admin` / `admin`, override with `BITRIVER_GRAFANA_ADMIN_USER`
and `BITRIVER_GRAFANA_ADMIN_PASSWORD` in `.env`).

**Import the dashboard:**

1. Sign into Grafana and add a Prometheus data source pointing at `http://prometheus:9090`.
2. From **Dashboards → New → Import**, upload `deploy/monitoring/bitriver-live-dashboard.json` and select the Prometheus data
   source when prompted.

### Alert runbook map

Each alert in `deploy/monitoring/prometheus-alerts.yml` should page to the remediation below.

| Alert name | Trigger | Triage steps | Expected remediation |
| --- | --- | --- | --- |
| `BitRiverReadyzUnavailable` | `/readyz` returns non-2xx for 2 minutes. | Check `docker compose ps`; inspect API/Postgres/Redis logs with `docker compose logs -f bitriver-live postgres redis`; confirm `.env` DSN and Redis credentials. | Restore DB/Redis connectivity, restart failed dependency containers, then verify `/readyz` returns `200`. |
| `BitRiverHealthzCoreUnavailable` | `/healthz` returns HTTP 503 for 3 minutes. | Compare `/readyz` and `/healthz`; if both fail, treat as core outage; if only `/healthz` fails, inspect ingest services (`srs`, `ome`, `transcoder`, `srs-controller`). | Recover failing core or ingest dependency, then confirm `/healthz` JSON reports healthy core checks. |
| `BitRiverHealthComponentDegraded` | Any `bitriver_ingest_health{status="degraded"}` stays degraded for 3 minutes. | Use `/api/status` to identify degraded service and remediation hints; inspect targeted logs for SRS/OME/transcoder and health endpoint auth tokens. | Fix endpoint reachability, rotate bad ingest credentials, or restart degraded ingest service. |
| `BitRiverAuthFailureSpike` | 401/403/429 ratio on `/api/auth/*` exceeds 20% for 10 minutes. | Check whether failures map to one source IP/user agent; inspect audit/auth logs for brute-force patterns; verify MFA/session expiry settings. | Block abusive IPs at proxy/WAF, rotate exposed credentials, tune rate limits, and communicate user-impacting auth incidents. |
| `BitRiverChatQueueLagHigh` | `redis_stream_consumer_lag` on chat stream exceeds 1000 for 10 minutes. | Query Redis stream/group lag (`XINFO STREAM` / `XINFO GROUPS`); inspect API latency and chat consumer logs; confirm Redis CPU/memory headroom. | Scale chat consumers/API replicas, reduce downstream bottlenecks, or recover stalled consumers and replay pending entries. |
| `BitRiverDiskUsageHigh` | `transcoder-data` filesystem usage exceeds 85% for 15 minutes. | Validate mount utilization on host and inspect `./transcoder-data` growth (recordings, orphaned HLS artifacts). | Prune expired recordings, archive media, or expand storage before transcoder writes fail. |
| `BitRiverDiskUsageCritical` | `transcoder-data` usage exceeds 92% for 10 minutes. | Perform urgent capacity check and identify largest directories/files under `./transcoder-data`. | Emergency cleanup/archive, then add durable capacity and retention policies. |
| `BitRiverIngestFailureRateHigh` | Ingest failure ratio exceeds 5% for 10 minutes. | Compare `bitriver_ingest_failures_total` by operation; inspect recent ingest API calls and dependency health in `/api/status`. | Repair failing operation path (credentials/network), retry failed ingest workflows, and confirm failure ratio drops. |
| `BitRiverTranscoderFailuresHigh` | Transcoder fail ratio exceeds 5% for 10 minutes. | Inspect transcoder logs for ffmpeg exits, missing inputs, or permission issues; verify CPU/memory and `./transcoder-data` free space. | Restart/recover transcoder workers, fix input or codec config issues, and rebalance workload/capacity. |

## Resource sizing + kernel tuning

Use the optional Compose override `deploy/docker-compose.resources.yml` to set higher `nofile` limits and baseline CPU/memory
reservations for SRS, OME, and the transcoder:

```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.resources.yml up -d
```

It sets `nofile=262144` and these baseline reservations/limits:

| Service | CPU reservation | CPU limit | Memory reservation | Memory limit |
| --- | --- | --- | --- | --- |
| `srs` | 1.0 | 2.0 | 1G | 2G |
| `ome` | 1.5 | 4.0 | 2G | 4G |
| `transcoder` | 2.0 | 6.0 | 4G | 12G |

**Host kernel recommendations:** ensure the Docker daemon can raise file descriptor limits and that the host allows them:

```bash
sudo tee /etc/sysctl.d/99-bitriver-live.conf <<'EOF'
fs.file-max = 1048576
fs.nr_open = 1048576
net.core.somaxconn = 4096
net.ipv4.ip_local_port_range = 10240 65535
EOF

sudo sysctl --system
```

If you are using systemd, set `LimitNOFILE=262144` in a Docker service override so containers can inherit the higher limit.

**Tuning by stream count:**

- **Transcoder:** add ~1 vCPU and ~1–2 GB RAM per additional 1080p stream (double for 4K or higher ladders).
- **OME + SRS:** add ~0.25 vCPU and ~256–512 MB RAM per additional 1,000 concurrent viewers or ~50 publishers, plus headroom.

Scale transcoder first when encode latency rises, then increase OME/SRS headroom when ingest connections churn or viewer drops
increase.

### Log locations and triage flows

**Log locations (default Compose stack):**

- **API, viewer, transcoder, ingest services:** stdout/stderr via Docker; tail with:
  ```bash
  docker compose logs -f
  ```
- **Targeted logs:** filter by service name when triaging a specific component:
  ```bash
  docker compose logs -f bitriver-live
  docker compose logs -f transcoder
  docker compose logs -f srs srs-controller ome
  ```
- **Postgres / Redis:** stdout/stderr via Docker (same command). Postgres data lives under the `postgres-data` volume.
- **Host-mounted assets:** `./transcoder-data` contains HLS output and recordings (also used for disk usage checks).
- **Audit trail:** the API emits structured audit logs for state-changing requests (user, path, status, IP). Forward these
  logs to your SIEM for forensics and alert correlation.

**Example triage flows:**

1. **Overall outage**
   - Check `/readyz` to confirm core dependencies are failing vs. ingest-only issues.
   - If `/readyz` is failing, check Postgres/Redis container health and logs (`docker compose ps`, `docker compose logs -f`).
   - If `/readyz` is healthy but `/healthz` flags ingest, inspect the SRS/OME/transcoder logs.
2. **API latency spike**
   - Use `/api/status` for dependency hints.
   - Check Postgres connection usage or slow queries (logs) and Redis latency.
   - Verify CPU/memory saturation on the API container and host.
3. **Ingest errors**
   - `/healthz` + `/api/status` should call out which ingest dependency is failing.
   - Inspect SRS/OME logs; confirm the API credentials match `.env` and the health URLs resolve from the API container.
4. **Transcoder failures**
   - Check transcoder logs for failed jobs and missing media inputs.
   - Verify `./transcoder-data` capacity and permissions.
5. **Chat backlog**
   - Confirm Redis health; check stream length and consumer lag, for example:
     ```bash
     docker compose exec -T redis redis-cli --no-auth-warning -a "$BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD" \
       XINFO STREAM "$BITRIVER_LIVE_CHAT_QUEUE_REDIS_STREAM"
     ```
   - If lag grows, scale chat consumers or investigate downstream API latency.

## Postgres logical backups and restores

Postgres is the system of record for channels, users, recordings metadata, and audit history. The Compose service name is
`postgres`, and the bundled `.env` defines `BITRIVER_POSTGRES_DB`, `BITRIVER_POSTGRES_USER`, and
`BITRIVER_POSTGRES_PASSWORD`.

### Backup with `pg_dump`

1. Ensure the stack is running so `postgres` is healthy.
2. Run `pg_dump` inside the container and copy the archive to the host:

```bash
mkdir -p ./backups

docker compose exec -T postgres \
  pg_dump -U "$BITRIVER_POSTGRES_USER" \
  -d "$BITRIVER_POSTGRES_DB" \
  -F c \
  -f /tmp/bitriver-live.backup

docker compose cp postgres:/tmp/bitriver-live.backup \
  ./backups/bitriver-live-$(date +%F).backup
```

If you prefer to run `pg_dump` from a workstation or a job runner, point it at the same DSN the API uses (`postgres:5432`
inside Compose, or the host port if you enable the `postgres-host` profile with
`docker compose --profile postgres-host up -d`).

### Restore with `pg_restore`

1. Stop the API so it does not write to Postgres during the restore:

```bash
docker compose stop bitriver-live
```

2. Copy the backup archive into the `postgres` container and restore it:

```bash
docker compose cp ./backups/bitriver-live-2024-01-01.backup postgres:/tmp/bitriver-live.backup

docker compose exec -T postgres \
  pg_restore --clean --if-exists --create \
  -U "$BITRIVER_POSTGRES_USER" \
  -d postgres \
  /tmp/bitriver-live.backup
```

3. Start the API again and confirm it reconnects:

```bash
docker compose start bitriver-live
```

If you restore into a new database name or host, update `BITRIVER_LIVE_POSTGRES_DSN` in `.env` before restarting the stack.
Re-run the `postgres-migrations` helper if you restore into an empty database without schema (the helper is wired into
Compose and will apply `deploy/migrations/*.sql` on the next `docker compose up -d`).

## Redis persistence and recovery

The Compose `redis` service is configured as cache-only by default (`--save ""` and `--appendonly no`), so chat queue history
and rate-limit counters are not persisted across restarts.

If you want Redis persistence:

1. Update the `redis` command in `deploy/docker-compose.yml` to enable either RDB snapshots or AOF.
2. Restart the service so Redis writes data under the `redis-data` volume.
3. Trigger a snapshot when needed:

```bash
docker compose exec -T redis redis-cli --no-auth-warning -a "$BITRIVER_REDIS_PASSWORD" save
```

4. Back up the `redis-data` volume alongside Postgres:

```bash
mkdir -p ./backups

docker run --rm \
  -v bitriver-live_redis-data:/data \
  -v "$PWD/backups":/backup \
  alpine \
  tar czf /backup/redis-data-$(date +%F).tgz -C /data .
```

To restore, stop the stack, replace the contents of the `redis-data` volume with the archive, then start `redis` again:

```bash
docker compose stop redis

docker run --rm \
  -v bitriver-live_redis-data:/data \
  -v "$PWD/backups":/backup \
  alpine \
  sh -c "rm -rf /data/* && tar xzf /backup/redis-data-2024-01-01.tgz -C /data"

docker compose start redis
```

When running cache-only (default), you can skip Redis backups entirely.

## Media assets: transcoder output + recordings

The transcoder writes HLS manifests and segments under `/work/public` inside the `transcoder` container. The Compose stack
mounts that directory to `./transcoder-data` on the host so media survives container restarts. Back up the host directory with
standard filesystem tooling:

```bash
rsync -av --delete ./transcoder-data/ /mnt/backup/bitriver-transcoder-data/
```

Recordings metadata lives in Postgres, but the media files live wherever you configured the recording storage:

- **Default Compose stack:** recordings and HLS assets live under `./transcoder-data/public` (from `/work/public`). Back up the
  directory alongside the other transcoder data. If you keep long-term recordings on disk, add an explicit retention policy
  (see below) so storage does not grow without bound.
- **Object storage configured:** follow your provider’s snapshot/replication runbook, and make sure bucket lifecycle rules
  align with the retention settings configured in the API.
- **Mounted storage (NFS/S3FS/etc.):** treat the mount point as the source of truth and back it up using your normal
  filesystem snapshots.

Keep backups of Postgres and media assets in the same recovery window so channel metadata and playback artefacts stay in
sync after a restore.

### Retention expectations

Use the settings below to match your compliance or storage policies. Unless stated otherwise, leaving a value empty or `0`
keeps data indefinitely.

#### Transcoder artifacts (`/work/public`, `./transcoder-data`)

- **Live sessions:** FFmpeg output is written under `/work/live/<jobId>` and mirrored to `/work/public/live/<jobId>`. The
  mirror symlink is removed when a stream stops, but the output directory stays on disk by default.
- **Uploads/VOD jobs:** Outputs land in `/work/uploads/<jobId>` and are copied into `/work/public/uploads/<jobId>` for
  playback. These folders are retained until you delete them manually or configure cleanup.
- **Cleanup configuration:** Set `BITRIVER_TRANSCODER_RETENTION_LIVE` and/or `BITRIVER_TRANSCODER_RETENTION_UPLOADS` (for
  example, `168h`) to let the transcoder delete stopped live outputs or completed upload outputs after the specified
  duration. The cleanup sweep runs every 30 minutes and removes both the output directory and the public mirror.

#### Recordings and VOD metadata

- **API retention windows:** Configure `BITRIVER_LIVE_RECORDING_RETENTION_PUBLISHED` and
  `BITRIVER_LIVE_RECORDING_RETENTION_UNPUBLISHED` to control how long recordings are kept after a stream stops. Defaults
  are 90 days for published VODs and 14 days for unpublished drafts. Use `0` to disable expiry.
- **Purge behavior:** When the retention window elapses, the API deletes the recording metadata, clip exports, and any
  object-storage manifests/thumbnails it created. Local transcoder artifacts on disk are not deleted by the API, so pair
  this with the transcoder retention settings (above) or a storage lifecycle policy for on-disk HLS outputs.

#### Chat messages and moderation logs

- **Stored history:** Chat transcripts, moderation reports, and automated filter actions are stored in Postgres/JSON.
  Redis only handles live fan-out and does not persist history by default. Automated actions are only visible in the
  control-centre moderation views and are not broadcast to viewer chat transcripts.
- **Retention configuration:** Use `BITRIVER_LIVE_CHAT_RETENTION_MESSAGES` for chat history and
  `BITRIVER_LIVE_CHAT_RETENTION_MODERATION_LOGS` for moderation reports and automod actions (example: `720h`). These apply
  to report creation timestamps or resolution timestamps when present.
- **Purge behavior:** Retention is enforced when the API loads chat history or moderation queues, so expired messages,
  reports, and automod actions are deleted on access.
