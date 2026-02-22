# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Identify cohesive helper sections for extraction
  - Acceptance criteria:
    - Function groups in `storage.go` and `postgres_channels.go` are identified by concern.
    - Planned destination files are listed.
  - Identified groups:
    - `storage.go`: dataset init/clone helpers, object-key metadata helpers, OAuth identity normalization helpers, channel query/tag helpers.
    - `postgres_channels.go`: stream-session/recording construction helpers, object artifact helpers, recording retention/purge helpers, row-loading helpers.
  - Planned destination files:
    - `internal/storage/storage_dataset_helpers.go`
    - `internal/storage/storage_object_helpers.go`
    - `internal/storage/storage_oauth_helpers.go`
    - `internal/storage/storage_channel_helpers.go`
    - `internal/storage/postgres_recording_helpers.go`
  - Relevant checks:
    - ✅ `go test ./internal/storage -count=1`
  - Result:
    - Passed.

- [x] Task 2 — Move `storage.go` private helpers into concern-grouped files
  - Acceptance criteria:
    - Private helper functions/methods from `storage.go` are moved into new files under `internal/storage/`.
    - Package name and exported symbols remain unchanged.
    - No SQL/runtime behavior changes.
  - Relevant checks:
    - ✅ `go test ./internal/storage -count=1`
  - Result:
    - Passed.

- [x] Task 3 — Move `postgres_channels.go` private helpers into concern-grouped files
  - Acceptance criteria:
    - Private PostgreSQL helper methods are moved into new files under `internal/storage/` grouped by concern.
    - SQL text and behavior remain unchanged.
  - Relevant checks:
    - ✅ `go test ./internal/storage -count=1`
    - ✅ `go test ./internal/storage -count=1 -tags=integration`
  - Result:
    - Passed.

## Execution log
- `go test ./internal/storage -count=1` (pass).
- `go test ./internal/storage -count=1` (pass, post-refactor).
- `go test ./internal/storage -count=1 -tags=integration` (pass).
