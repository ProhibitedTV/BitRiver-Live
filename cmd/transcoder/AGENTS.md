# `cmd/transcoder` Guidance

The transcoder binary runs the FFmpeg job controller used by Docker Compose and advanced deployments. Review the root `AGENTS.md` first.

## Responsibilities
- Orchestrates FFmpeg processes per job, tracks PIDs, and mirrors job status via the public API. Each job runs in its own goroutine; make sure new code joins/shuts these routines to avoid leaked processes.
- Persists job metadata via `metadataStore`. Any handler change must continue to write/read job state, especially across crashes.
- Exposes `/healthz` and `/v1/jobs` (GET/POST) for controllers. Preserve routing, JSON formats, and auth checks.

## Configuration
- Mandatory env vars include `JOB_CONTROLLER_TOKEN` (bearer auth) and `BITRIVER_TRANSCODER_PUBLIC_BASE_URL`. Document new vars in the README and Compose files.
- Handlers must validate tokens using constant-time comparison before accepting writes.
- Process tracking should capture stderr/stdout piping, FFmpeg exit codes, and retry/backoff policies; keep logging structured.

## Before opening a PR
- Confirm `/healthz` stays cheap and `/v1/jobs` remains backward compatible.
- Test per-job goroutine lifecycle by starting and cancelling jobs locally.
- Update docs/Compose/scripts when env vars or ports change.
- Run `go test ./cmd/transcoder -count=1` plus repository-wide suites.
