# Media pipeline failure model

This document describes failure ownership for the default Docker Compose deployment and what BitRiver Live does (and does **not**) recover automatically.

Scope:
- `deploy/docker-compose.yml`
- API ingest orchestration (`internal/ingest`)
- Runtime health/status endpoints (`/readyz`, `/healthz`, `/api/status`)

The goal is operator clarity, not theoretical completeness.

## Component ownership boundaries

### SRS (ingest runtime)
SRS owns:
- Accepting encoder ingest traffic (RTMP/WebRTC as configured).
- Maintaining live ingress sessions.
- Serving SRS API responses consumed by `srs-controller`.

BitRiver Live owns around SRS:
- Creating/deleting channel metadata through `srs-controller` (`/v1/channels*`).
- Polling SRS health via the configured ingest health endpoint (`BITRIVER_INGEST_HEALTH`, default `/healthz`).
- Reporting SRS status in API health payloads.

### OME (origin/playback runtime)
OME owns:
- Origin application runtime and media serving.
- OME API behavior and auth enforcement.
- Startup validity of `Server.generated.xml` for the running image.

BitRiver Live owns around OME:
- Rendering and mounting `deploy/ome/Server.generated.xml` before OME startup.
- Failing fast on known health-token/config mismatches via `ome-health-token-check`.
- Creating/deleting per-channel applications via ingest orchestration.
- Polling OME health and surfacing degraded status.

### Transcoder (ffmpeg job service)
Transcoder owns:
- Starting/stopping ffmpeg processes for live and upload jobs.
- Persisting/reloading job metadata under `/work` and attempting resume on restart.
- Its own `/healthz` component status (`degraded` when internal components are unhealthy).

BitRiver Live owns around the transcoder:
- Requesting job start/stop during stream boot/shutdown.
- Treating transcoder errors as ingest orchestration failures.
- Polling transcoder health and surfacing degraded status.

### Control plane (BitRiver Live API + ingest orchestrator)
Control plane owns:
- Sequencing `BootStream` (SRS channel -> OME app -> transcoder jobs).
- Best-effort rollback during boot failures (delete already-created upstream resources).
- Best-effort shutdown (`StopJob`, `DeleteApplication`, `DeleteChannel`) with aggregated errors.
- Exposure of readiness/health/status for operators and automation.

Control plane does **not** own:
- Deep healing of third-party runtime internals (SRS/OME/ffmpeg internals).
- Guaranteed cleanup if upstream APIs are unavailable during rollback.

## What BitRiver Live recovers automatically

Automatic recovery in the current contract is limited to these cases:

1. **Container/process restart by Compose**
   - `restart: unless-stopped` is set for `bitriver-live`, `srs-controller`, `srs`, `ome`, and `transcoder`.
   - If a service process exits and the host is healthy, Docker will attempt restart.

2. **Boot-time rollback when pipeline creation fails**
   - If OME app creation fails after SRS channel creation, control plane attempts `DeleteChannel`.
   - If transcoder start fails after OME/SRS success, control plane attempts `DeleteApplication` and `DeleteChannel`.
   - This is best-effort cleanup, not transactional rollback.

3. **Bounded retry for ingest HTTP adapters**
   - Ingest adapters retry transient HTTP/network failures (5xx, 429, transport errors).
   - Non-429 4xx responses are treated as permanent and are not retried.

4. **Transcoder in-process resume attempt on service restart**
   - On transcoder startup, incomplete tracked jobs/uploads are reloaded and restart is attempted.
   - Failures are logged and reflected in transcoder health.

## What requires operator intervention

You should expect manual action for:

1. **Persistent auth/config mismatches**
   - Examples: bad SRS token, bad OME credentials/token, bad transcoder token.
   - Symptoms: repeated 401/403/4xx in ingest orchestration, degraded health, failed stream boot.

2. **Port collisions / networking / DNS issues**
   - Symptoms: container restart loops, connection refused/timeouts, upstream probe failures.

3. **Failed best-effort cleanup**
   - If shutdown or rollback cannot reach an upstream API, orphaned channels/apps/jobs may remain until manually removed.

4. **Dependency outages outside ingest**
   - Postgres/Redis failures drive `/readyz` to `503`; ingestion APIs may be indirectly impacted.

5. **Resource exhaustion**
   - Disk pressure in `./transcoder-data`, CPU starvation, memory pressure, file-handle limits.

Use these operator-facing signals first:
- API: `GET /readyz`, `GET /healthz`, `GET /api/status`
- Service probes:
  - `http://127.0.0.1:${BITRIVER_SRS_CONTROLLER_PORT:-1986}/healthz`
  - `http://127.0.0.1:${BITRIVER_TRANSCODER_HOST_PORT:-9001}/healthz`
  - `http://127.0.0.1:${BITRIVER_OME_HTTP_PORT:-8081}/` (Compose health probe target)
- Logs:
  - `docker compose logs -f bitriver-live`
  - `docker compose logs -f srs-controller srs ome transcoder`
  - `docker compose logs -f postgres redis`

## Intentionally not handled

The current stack intentionally does **not** promise:

- Exactly-once provisioning/deprovisioning across SRS + OME + transcoder.
- Distributed transactions or compensating actions that are guaranteed to succeed when dependencies are down.
- Automatic reconciliation of orphaned upstream resources after prolonged outages.
- Automatic credential rotation propagation across all services.
- Automatic recovery from invalid operator edits to `.env`, compose overrides, or generated OME config beyond failing health/startup checks.
- Multi-region failover or cross-cluster self-healing semantics.

If these guarantees are needed, treat them as future work and implement explicit reconciliation and control-loop behavior before documenting stronger promises.
