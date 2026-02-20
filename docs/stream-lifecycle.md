# Stream lifecycle (canonical)

This document defines the canonical stream lifecycle for BitRiver Live based on the behavior currently implemented in:

- `internal/api/streams_srs_handlers.go`
- `internal/storage/storage.go`
- `internal/ingest/http_controller.go`
- `internal/ingest/adapters.go`
- `cmd/transcoder/main.go`
- `cmd/srs-controller/main.go`

It is intentionally conservative: transitions are listed only when they are already observable in the running system.

## State model

| State | Meaning in current system | Enforced in code? |
| --- | --- | --- |
| `CREATED` | Channel exists and can receive ingest events, but no active stream session exists yet (`channel.live_state` is effectively `offline`). | **Observed-only** (represented by `offline`, not a dedicated enum/state value). |
| `INGESTING` | Encoder publish has begun (`on_publish` path), and ingest boot is in progress (SRS/OME/transcoder provisioning). | **Observed-only** (transient phase; channel is set to `starting` internally). |
| `LIVE` | Boot completed: session is persisted, ingest endpoints/origin/playback metadata are stored, and channel live state is set to `live`. | Enforced (`internal/storage.StartStream`). |
| `DEGRADED` | One or more ingest components report non-OK health (`/healthz`, `/api/status`, transcoder component snapshots), while stream/session may still be active. | **Observed-only** (health/reporting state, not a persisted stream state). |
| `ENDED` | Stream has been stopped/unpublished cleanly; ingest shutdown ran; session has `endedAt`; channel returns to `offline`. | Enforced (`internal/storage.StopStream` and unpublish handling). |
| `ERROR` | A lifecycle operation failed (boot failure, shutdown failure, dependency unavailable, FFmpeg/process error, upstream control-plane errors). | **Observed-only** (returned/logged errors and degraded health; no persisted `error` stream state). |

## Transition map

### 1) `CREATED -> INGESTING`
- **Trigger:** SRS `on_publish` hook (or explicit `/stream/start`) reaches control plane.
- **Owner:** Control plane API (`internal/api/streams_srs_handlers.go`) initiates; SRS is the event source.
- **What happens:** `StartStream` is called, channel enters `starting`, and boot orchestration begins.

### 2) `INGESTING -> LIVE`
- **Trigger:** `BootStream` succeeds end-to-end.
- **Owner by sub-step:**
  - SRS adapter provisions channel (`CreateChannel`).
  - OME adapter provisions application (`CreateApplication`).
  - Transcoder starts live jobs (`StartJobs`).
  - Control plane persists session and flips channel to `live`.
- **What happens:** Session metadata (job IDs, ingest endpoints, playback/origin URLs) is stored and surfaced by API.

### 3) `INGESTING -> ERROR`
- **Trigger:** Any boot stage fails or ingest controller is unavailable.
- **Owner:** Failing subsystem (SRS/OME/transcoder/control plane), with control plane handling rollback/reporting.
- **What happens:**
  - Control plane retries boot according to ingest config (`ingestMaxAttempts`, `ingestRetryInterval`).
  - HTTP adapters retry transient calls (network errors, HTTP `5xx`, HTTP `429`).
  - On stage failure, previously created upstream resources are best-effort cleaned up.
  - Channel/session is reset back to `offline`/no active session.

### 4) `LIVE -> DEGRADED`
- **Trigger:** Health checks show ingest component errors (SRS controller probe failure, OME/transcoder probe failure, transcoder component error).
- **Owner by signal source:**
  - SRS controller: `/healthz` proxy/probe status.
  - Transcoder: component state (`ffmpeg`, publishing) and `/healthz`.
  - Control plane ingest health aggregation (`HealthChecks`, `/api/status`).
- **What happens:** System reports degraded/down health for operators. Stream is not automatically terminated solely due to degraded health.

### 5) `DEGRADED -> LIVE`
- **Trigger:** Health checks recover to OK.
- **Owner:** Component that recovered (SRS/OME/transcoder), observed by control plane health aggregation.
- **What happens:** Status endpoints return to healthy/ready without creating a new stream session.

### 6) `LIVE -> ENDED`
- **Trigger:** SRS `on_unpublish` hook or explicit `/stream/stop`.
- **Owner:** Control plane coordinates shutdown; transcoder/OME/SRS execute teardown.
- **What happens:**
  - Transcoder jobs are stopped.
  - OME app is deleted.
  - SRS channel is deleted.
  - Session gets `endedAt`, optional peak concurrency update, and channel returns to `offline`.

### 7) `LIVE -> ERROR`
- **Trigger:** Shutdown fails, or runtime transcoder process exits with error.
- **Owner:** Transcoder and/or control plane teardown path.
- **What happens:**
  - Stop/unpublish API call returns error when shutdown fails.
  - Transcoder marks component error and records failed job metrics when FFmpeg exits abnormally.
  - Recovery is currently operator-driven (retry stop/start or restart failing components).

## Failure policy (current behavior)

### Retry
- **Control plane boot retry:** `StartStream` retries `BootStream` up to configured attempts with configured interval.
- **HTTP adapter retry:** SRS/OME/transcoder adapter calls retry on transient/network errors, HTTP `5xx`, and HTTP `429`.

### Downgrade
- **Health downgrade exists:** dependency failures surface as degraded/down in health/status endpoints.
- **Lifecycle downgrade does not exist as a persisted stream state:** `DEGRADED` is operationally observed, not written as channel/session lifecycle state.

### Terminate
- **Terminate on explicit stop/unpublish:** normal path tears down transcoder -> OME -> SRS and closes session.
- **Terminate on boot failure:** control plane rolls back partial resources and reverts to offline.
- **No automatic terminate on health degradation alone:** currently requires operator or upstream event intervention.

## Enforcement notes

- `CREATED`, `INGESTING`, `DEGRADED`, and `ERROR` are part of the canonical lifecycle vocabulary but are **observed-only** in current code.
- `LIVE` and `ENDED` are directly represented by persisted session/channel mutations.
- When implementing future lifecycle enforcement, keep this document aligned with actual state transitions and ownership boundaries.

## Upload-to-VOD publish policy

- Successful upload transcode processing now ensures a `recordings` row exists and links `uploads.recording_id` to that recording.
- Upload-created recordings are **unpublished by default** (`published_at` remains `NULL`).
- `/api/channels/{id}/vods` only returns recordings with `published_at` set, so upload VODs remain hidden until an explicit publish action occurs through recording APIs.
