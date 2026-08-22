# Operations runbook

This guide focuses on day-two operations for the default Docker Compose stack in `deploy/docker-compose.yml`. It references the
Compose service names (`postgres`, `redis`, `transcoder`) and the host-mounted volumes so you can lift and restore state safely
without breaking the one-command deployment flow.

For baseline hardening posture, exposure boundaries, and rotation checklists, see [`docs/security.md`](security.md).


## Preflight before production changes

For an installed Ubuntu package, run the host-manager preflight before first
activation and before upgrades:

```bash
sudo bitriver-host doctor
sudo bitriver-host status
```

From a source checkout, run the built-in preflight directly:

```bash
go run ./cmd/bitriver doctor --compose-file deploy/docker-compose.yml --env-file ./.env
```

Use `--json` for automation/inventory systems:

```bash
go run ./cmd/bitriver doctor --json
```

Interpretation:

- `PASS`: requirement is satisfied.
- `WARN`: soft risk; rollout can continue but remediation is recommended.
- `FAIL`: hard blocker; `doctor` exits non-zero and rollout should stop.

Doctor minimum preflight thresholds are intentionally conservative for single-host production starts:

- 4 logical CPUs
- 8 GiB RAM
- 20 GiB free disk (repo root or Docker data root when detectable)
- Docker `>= 24.0.0` and Docker Compose v2 `>= 2.20.0`

Adjust thresholds with `--min-cpu`, `--min-ram-gb`, and `--min-free-disk-gb` for larger or smaller environments.

## Media pipeline failure ownership

For the explicit ownership and recovery boundaries of SRS/OME/transcoder/control-plane failures, see [`docs/media-failure-model.md`](media-failure-model.md).

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
  with degraded JSON payloads. When no ingest snapshot exists yet, the first request performs one bounded downstream probe
  and records it; later `/healthz` requests reuse that cached snapshot so routine container liveness probes do not fan out
  on every request. Use this for dashboards and on-call triage.
- **`GET /api/status` (operator summary):** Aggregates readiness plus ingest probes, remediation hints, and log suggestions
  used in the control centre Overview. This endpoint performs the on-demand ingest refresh path, so use it when you want a
  human-readable payload for alerts or ChatOps.

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

Use the dedicated guide in [`docs/monitoring.md`](monitoring.md) for the full quickstart, provisioning behavior,
alert routing setup, and troubleshooting.

In short:

1. Configure `BITRIVER_LIVE_METRICS_TOKEN` in `.env`.
2. Copy `deploy/monitoring/metrics.token.example` to `deploy/monitoring/metrics.token` and keep values in sync.
3. Copy `deploy/monitoring/alertmanager.env.example` to `deploy/monitoring/alertmanager.env`, then run `./scripts/render-alertmanager-config.sh`.
4. Validate configs with `./scripts/check-monitoring-config.sh`.
5. Start the optional overlay:
   ```bash
   docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.monitoring.yml up -d
   ```

Grafana provisioning is automatic: the Prometheus datasource and bundled BitRiver dashboard are loaded on first start.

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

### Release canary gate

After a staging or production-candidate deploy, run the canary gate from a host that can reach the public API endpoint:

```bash
docker compose -f deploy/docker-compose.yml --env-file ./.env logs --tail=200 > .tmp/recent-compose-logs.txt
./scripts/release-canary.sh \
  --base-url https://api.example.com \
  --logs-file .tmp/recent-compose-logs.txt \
  --rollback-notes .tmp/rollback-notes.md \
  --require-rollback-notes \
  --artifact-dir .artifacts/release-canary
```

The command does not start or stop services. It records CLI version metadata, redacted responses from `/readyz`, `/healthz`, `/api/status`, and `/viewer`, a conservative log scan, and rollback readiness evidence. Treat a failed canary as a stop-promotion signal. Treat warnings as missing evidence that must be resolved or explicitly accepted in the release ticket before expanding traffic.

## Capacity qualification harness

Do not turn the sizing estimates below into a support claim without measuring
the exact release candidate on the intended host. The opt-in capacity harness
provides a bounded, secret-safe instrument for that work; it is not part of
ordinary quickstart or `./scripts/verify.sh`.

First validate and bind the small RC scenario without generating load:

```bash
./scripts/test-capacity-qualification.sh --dry-run \
  --release v1.2.3-rc.N \
  --release-set-sha256 <64-hex-release-set-sha256> \
  --source-commit <40-hex-source-commit>
```

Live mode is destructive to the qualification environment in the ordinary
product sense: it persists test accounts/channels and media output. Run it only
against a dedicated disposable candidate stack, preferably from a separate
load-generator host:

```bash
./scripts/test-capacity-qualification.sh --live --client docker \
  --confirm-dedicated-environment \
  --release v1.2.3-rc.N \
  --release-set-sha256 <64-hex-release-set-sha256> \
  --release-set-file /verified/release-set.json \
  --source-commit <40-hex-source-commit> \
  --base-url https://candidate.example.com \
  --rtmp-base-url rtmp://candidate.example.com:1935/live \
  --metrics-bearer-file /run/secrets/bitriver-metrics-token \
  --artifact-dir .artifacts/capacity-qualification
```

The checked-in `bitriver.capacity-scenario/v1` scenario ramps through warm-up,
steady-state, spike, and soak phases. Hard parser caps prevent more than 16
publishers, 512 virtual viewers, 100 API requests/s, 50 chat messages/s, a
one-hour phase, or a four-hour total even if a scenario file is edited. The
small RC scenario is intentionally lower: two publishers and 12 HLS viewers at
its spike. Every live run requires protected `/metrics` access and stops when
health repeatedly fails or configured workload/host/container thresholds
persist. A phase also fails if active stream/transcoder gauges do not match its
publisher count, less than 80% of configured viewer/API/chat attempts are
delivered, or its aggregate error rate breaches the limit. A failure or
interrupt stops all FFmpeg publishers and load workers.
Live mode hashes the supplied `release-set.json` bytes, requires that hash to
match `--release-set-sha256`, and validates the declared tag, source commit,
five first-party candidate references/digests, and Sigstore asset reference.
It does not verify the Sigstore signature itself, so independently verify the
release set before the run. The report keeps target runtime-image matching
explicitly unproven until a target-side collector supplies that evidence.

The resulting `bitriver.capacity-report/v1` records the exact candidate,
canonical scenario hash, phase timings, HTTP/media bytes and latency
percentiles, error rates, selected application metrics, collector provenance,
raw bounded samples, stop reasons, and explicit unproven claims. Per-run
account passwords, sessions, stream keys, and the metrics token are sentinel-
scanned and cannot be retained.

Remote mode reports host and Docker resources as unavailable instead of
guessing. For a bounded development rehearsal on the Compose host, use a host
client with `--collector-mode co-located`, `--compose-project NAME`, and
`--data-path DIR`; Linux `/proc`, direct disk usage, and project-scoped
`docker stats` are then sampled. That mode includes the synthetic
publisher/viewer generator's own host cost, so it must not be used to publish a
supported capacity envelope.

Formal #1303 results still need the supported physical target, an external load
generator plus target-side resource telemetry, VOD activity, direct
Postgres/Redis/encoder/dropped-frame measurements, and conservative headroom.

## Resource sizing + kernel tuning

Use `deploy/docker-compose.limits.yml` for enforceable CPU/memory limits under non-Swarm Docker Compose:

```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.limits.yml up -d
```

If ingest services need elevated descriptor ceilings, add the ulimits-only overlay as well:

```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.limits.yml -f deploy/docker-compose.resources.yml up -d
```

`deploy/docker-compose.resources.yml` sets `nofile=262144` for `srs`, `ome`, and `transcoder`.

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

### Recovery targets and drill cadence

Use these defaults unless compliance or customer SLAs demand stricter controls:

- **RPO target:** 24 hours (nightly logical backup).
- **RTO target:** 2 hours (restore latest backup, run smoke checks, re-point API).
- **Restore drill cadence:** at least once every 30 days and after any schema-heavy release.

The restore report records observed RPO (backup age) and RTO (script runtime). Attach the report and operator sign-off to the
release or recovery ticket; the targets above remain the pass/fail thresholds even when the script itself succeeds.

### Durable recovery inventory

A Postgres dump is necessary, but it is not a complete host backup. Inventory and protect each recovery input independently:

| Recovery input | What it protects | Recovery rule |
| --- | --- | --- |
| Postgres backup set | Accounts, channels, schedules, auth state, moderation/legal records, settings, and media metadata | Keep the archive, manifest, and checksum together; rehearse the exact set before promotion. |
| `/etc/bitriver-live` on packaged hosts | Operator environment, secrets, and generated SRS/OME configuration | Store encrypted with restricted access. Restore to the same path; never attach its contents to public evidence. |
| `/var/lib/bitriver-live` on packaged hosts | Durable application and local media data | Snapshot with filesystem ownership and permissions preserved. |
| `deploy/transcoder-data` on checkout-based Compose hosts | HLS, upload, and recording objects stored locally | Snapshot in the same recovery window as Postgres so metadata and objects agree. |
| External object storage | Upload and recording objects when object storage is enabled | Enable provider versioning/replication and record bucket, prefix, and retention policy without recording credentials. |
| Immutable release set | The launcher/package, Compose bundle, images, signatures, and release-set digests used by the recovered host | Retain or reference the exact verified release set; do not rebuild a historical release during recovery. |
| Redis | Ephemeral chat fan-out, rate-limit, and cache state in the default deployment | No backup is required in the default cache-only mode; expect this transient state to reset. |

Document where every applicable item lives, its retention, its encryption/access policy, and the person responsible for a
restore. A lost-host drill is incomplete until configuration, Postgres, media/object data, and the immutable release inputs
have all been exercised together.

### Packaged recovery commands

Source-free launcher archives and Linux packages install the canonical recovery
commands under `/opt/bitriver-live/scripts/`; a source checkout exposes the same
paths under `./scripts/`:

- `./scripts/backup-postgres.sh`: atomically publishes a gzip-compressed logical dump, JSON manifest, and SHA-256 checksum set, and can upload all three to S3-compatible object storage.
- `./scripts/prune-backups.sh`: prunes complete local backup sets older than the configured retention window while preserving a minimum backup count.
- `./scripts/restore-postgres.sh`: verifies the backup set before mutation, restores into a fresh isolated database, compares migration and exact public-table row-count invariants, and writes a non-secret JSON report.
- `./scripts/backup-host-state.sh`: encrypts packaged configuration, local API/media data, the verified Postgres set, and an optional aggregate object inventory into one atomic host recovery set.
- `./scripts/restore-host-state.sh`: validates release/checksum/archive-member identity before streaming a host recovery set into fresh canonical paths and writing a secret-safe invariant/RPO/RTO report.
- `./scripts/test-backup-restore.sh`: runs the positive rehearsal and pre-mutation refusal cases against a disposable Postgres 15 container.
- `./scripts/test-disaster-recovery.sh`: deletes only its disposable source host, rebuilds a source-free packaged-host layout, restores a real non-empty Postgres set plus local/external object fixtures, and writes `bitriver.disaster-recovery/v1` evidence.
- `./scripts/test-published-disaster-recovery.sh`: downloads one exact public Linux amd64 launcher, validates its release-set identity/checksum before extraction, executes that package's recovery scripts through the lost-host drill, and retains secret-scanned manifest-bound evidence.

The backup manifest has schema `bitriver.postgres-backup/v1`. It binds the archive name/hash/size, source release and commit,
database/server/tool versions, applied migration ledger and fingerprint, and exact public-table row counts. The dump and all
manifest invariants use one exported repeatable-read snapshot. The checksum file covers the archive and manifest exactly;
copy, upload, retain, and prune the three files as one set.

Common environment variables:

- `BITRIVER_BACKUP_DIR` (default `./data/backups/postgres`)
- `BITRIVER_BACKUP_RETENTION_DAYS` (default `14`)
- `BITRIVER_BACKUP_KEEP_MIN` (default `3`)
- `BITRIVER_BACKUP_SOURCE_RELEASE` (exact `vMAJOR.MINOR.PATCH[-PRERELEASE]`; release evidence must not use `unknown`)
- `BITRIVER_BACKUP_SOURCE_COMMIT` (full lowercase 40-character commit SHA; release evidence must not use `unknown`)
- `BITRIVER_BACKUP_UPLOAD_ENABLED` (`1` to enable upload)
- `BITRIVER_BACKUP_UPLOAD_BUCKET`, `BITRIVER_BACKUP_UPLOAD_PREFIX`, `BITRIVER_BACKUP_UPLOAD_REGION`, `BITRIVER_BACKUP_UPLOAD_ENDPOINT`

### Run backup and prune manually

```bash
BITRIVER_BACKUP_POSTGRES_HOST=postgres \
BITRIVER_BACKUP_POSTGRES_USER="$BITRIVER_POSTGRES_USER" \
BITRIVER_BACKUP_POSTGRES_PASSWORD="$BITRIVER_POSTGRES_PASSWORD" \
BITRIVER_BACKUP_POSTGRES_DB="$BITRIVER_POSTGRES_DB" \
BITRIVER_BACKUP_SOURCE_RELEASE="$release_version" \
BITRIVER_BACKUP_SOURCE_COMMIT="$release_commit" \
./scripts/backup-postgres.sh

./scripts/prune-backups.sh
```

Set `release_version` and `release_commit` from the approved immutable release metadata, not from an unverified working tree.
A backup is not valid release evidence until the archive, manifest, and checksum are all durably stored.

### Restore rehearsal (isolated DB + invariant report)

Keep the matching `.manifest.json` and `.sha256` files beside the selected archive. Read the migration fingerprint from the
manifest approved for the release, choose a fresh rehearsal database name, and require both release and schema identity:

```bash
backup=./data/backups/postgres/bitriver-postgres-20260815T020000Z.sql.gz
expected_fingerprint=<64-lowercase-hex-from-approved-manifest>

BITRIVER_BACKUP_POSTGRES_HOST=postgres \
BITRIVER_BACKUP_POSTGRES_USER="$BITRIVER_POSTGRES_USER" \
BITRIVER_BACKUP_POSTGRES_PASSWORD="$BITRIVER_POSTGRES_PASSWORD" \
BITRIVER_RESTORE_REHEARSAL_DB=bitr_restore_20260815 \
BITRIVER_RESTORE_EXPECT_RELEASE="$release_version" \
BITRIVER_RESTORE_EXPECT_SCHEMA_FINGERPRINT="$expected_fingerprint" \
BITRIVER_RESTORE_REPORT_PATH=./data/backups/postgres/restore-20260815.json \
./scripts/restore-postgres.sh "$backup"
```

The script validates the checksum and manifest before creating the rehearsal database, refuses an existing/protected/source
database, restores into the isolated database, and compares every public-table row count plus the migration fingerprint. It
drops the rehearsal database by default and atomically writes a `bitriver.postgres-restore-report/v1` report containing
release/schema compatibility, hashes, invariant results, table count, and observed RPO/RTO. Set
`BITRIVER_RESTORE_KEEP_DB=1` only when an operator needs the isolated database for further inspection.

Backups created by the old archive-only workflow are intentionally refused because their source identity and consistency
cannot be proved. Create a fresh manifest-bound backup and rehearse it outside production. Never point the script at the
source or production database.

Run the repository-owned rehearsal test after changing the scripts:

```bash
./scripts/test-backup-restore.sh
```

### Encrypted packaged-host recovery set

The host-state wrapper requires GNU tar, Python 3, OpenSSL, and a restricted
passphrase file containing at least 20 bytes. Provision the passphrase through
your secret manager and store a separately protected recovery copy; losing it
makes the encrypted archive unrecoverable. Never place the passphrase itself in
an environment variable, command argument, ticket, or release evidence.

Create the Postgres set first, then bind it to the exact installed release and
full commit while encrypting the packaged-host state:

```bash
postgres_backup=/var/backups/bitriver-live/postgres/bitriver-postgres-20260815T020000Z.sql.gz
recovery_target=/mnt/off-host/bitriver-live

sudo /opt/bitriver-live/scripts/backup-host-state.sh \
  --postgres-backup "$postgres_backup" \
  --source-release "$release_version" \
  --source-commit "$release_commit" \
  --passphrase-file /root/bitriver-recovery.pass \
  --output-dir "$recovery_target"
```

When external object storage is enabled, create a non-secret
`bitriver.object-inventory/v1` aggregate from a restored/exported object mirror
and pass it with `--object-inventory`. The inventory proves counts, bytes, and a
deterministic fingerprint; it does not copy provider objects. Versioning,
replication, retention, credentials, and the actual object restore remain owned
by the storage provider/operator.

The three host files are an encrypted `.tar.gz.enc` archive, adjacent
`bitriver.host-backup/v1` manifest, and `.sha256` file. Copy and retain them as
one transaction. The public manifest contains hashes, counts, release identity,
encryption parameters, and timestamps only. It never contains configuration,
paths below the protected roots, object keys, passphrases, or row contents.

For a lost host, first verify and extract the exact release launcher on the
fresh machine. Before installing or activating the package, restore only into
absent `/etc/bitriver-live`, `/var/lib/bitriver-live`, and
`/var/backups/bitriver-live/recovery` paths:

```bash
sudo share/bitriver-live/scripts/restore-host-state.sh \
  --archive /mnt/off-host/bitriver-live/bitriver-host-20260815T021000Z.tar.gz.enc \
  --expected-release "$release_version" \
  --expected-commit "$release_commit" \
  --passphrase-file /root/bitriver-recovery.pass \
  --report /var/backups/bitriver-live/host-restore-report.json
```

The command verifies the outer checksum and release identity before decryption,
rejects traversal, links, devices, unexpected members, wrong passphrases, and
non-fresh targets, and never writes a plaintext archive. It restores the
Postgres trio under `/var/backups/bitriver-live/recovery/postgres`.

After host-state recovery, install the exact verified package. The installer
preserves the recovered environment/data, normalizes generated-config
compatibility links, and reconnects durable mounts. Restore Postgres into a
fresh database with `BITRIVER_RESTORE_KEEP_DB=1`, point
`BITRIVER_POSTGRES_DB` at that recovered database, run migration preflight, and
only then activate the stack. Restore provider objects, compare their aggregate
inventory, and run the production golden path. Rotate recovered credentials
when policy or incident scope requires it.

Run the complete disposable foundation after changing any recovery/package
input:

```bash
BITRIVER_DISASTER_RECOVERY_ARTIFACT_DIR=.artifacts/disaster-recovery \
  ./scripts/test-disaster-recovery.sh
```

After the recovery payload is present in an immutable candidate, qualify the
exact public archive in a fresh evidence directory:

```bash
./scripts/test-published-disaster-recovery.sh \
  --release "$release_version" \
  --source-commit "$release_commit" \
  --artifact-dir .artifacts/published-disaster-recovery
```

The wrapper verifies repository/tag/commit identity and the exact launcher
name, byte length, and SHA-256 declared by `release-set.json` before archive
listing or extraction. The retained report binds the lost-host result to the
release-set and package hashes but deliberately does not claim Sigstore
verification; pair it with independent release/clean-host signature evidence.
It also keeps the recovered production golden path and production-like
scheduled/off-host RPO evidence explicit until those separate drills pass.

### Single-host service restart rehearsal

Before qualifying a candidate, run the opt-in resilience rehearsal from an
otherwise idle source checkout with Docker and Go 1.26 available:

```bash
./scripts/test-service-resilience.sh \
  --report .artifacts/service-resilience/report.json
```

The command refuses any existing canonical BitRiver container or occupied test
port. It exports the tracked commit into a private temporary tree, copies the
root environment without printing it, assigns dedicated host ports, and keeps
API/transcoder state in staged binds plus Postgres/Redis in project-scoped
volumes. It never runs Compose against the operator's checkout, root `.env`,
generated OME file, or media directory, and it removes the temporary stack and
private tree before publishing evidence.

Seven bounded stop/start scenarios exercise the API, Postgres, Redis,
SRS/controller path, OvenMediaEngine, transcoder, and viewer. `/readyz` is the
core Postgres/Redis signal, authenticated `/api/status` forces live ingest
probes, and `/viewer` isolates viewer proxy availability. After each recovery,
the runner requires the original authenticated session and channel identity,
then samples all long-running Docker restart counts twice to reject a continuing
restart loop. Redis remains cache/transport state in the default profile; the
proof does not claim that in-flight chat, rate-limit counters, or transient
ingest work survive its restart.

Retain the secret-scanned `bitriver.service-resilience/v1` JSON with the release
ticket. It records the source commit as `local-build`, expected signal classes,
degradation/recovery durations, durable-state booleans, restart stability,
isolation results, and explicit remaining acceptance. Rerun the same proof from
the exact immutable candidate on the clean target host. Physical Docker/host
reboot, network partitions, disk/CPU/memory pressure, bad credentials/control-
plane failures, active-stream continuity, and alert delivery are separate
acceptance and must not be inferred from this stop/start report.

### Scheduling examples (Compose + Kubernetes + Helm)

- Compose cron override example: `deploy/docker-compose.backups.yml`
- Kubernetes CronJob examples: `deploy/kubernetes/postgres-backup-cronjob.yaml`
- Helm scheduling hooks: set `backups.enabled=true`, require durable object upload with `backups.objectStorage.enabled=true` plus its bucket/connection settings, and set `backups.sourceRelease` / the full release `backups.sourceCommit` in `deploy/helm/bitriver-live/values.yaml`. The chart mounts the synchronized canonical backup runner and uploads the same archive, manifest, and checksum contract; it refuses to render an enabled scheduler without object storage because pod-local output would disappear with the Job.

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
