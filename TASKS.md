# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Identify and map repeated nil-context guards in scoped packages
  - Acceptance criteria:
    - Target files/functions with repeated `ctx == nil` checks are identified for `chat/storage/auth/server/observability`.
    - Planned helper placement avoids duplicate definitions per package.
  - Identified targets:
    - `internal/chat`: `redis_queue.go` (3), `websocket.go` (1) → add one `normalizeContext` helper in package `chat`.
    - `internal/storage`: `postgres_repository.go` (2) → add one `normalizeContext` helper in package `storage`.
    - `internal/auth`: `postgres_store.go` (1), `session.go` (1), `mfa_challenge.go` (1), `postgres_mfa_challenge_store.go` (1) → add one `normalizeContext` helper in package `auth`.
    - `internal/server`: `redis_store.go` (1) → add one `normalizeContext` helper in package `server`.
    - `internal/observability`: `logging/logging.go` (3), `tracing/tracing.go` (3) are separate packages (`logging`, `tracing`) so helper placement is package-specific.
  - Relevant checks:
    - ✅ `rg -n "if ctx == nil" internal/chat internal/storage internal/auth internal/server internal/observability`
  - Result:
    - Completed; helper placement confirmed.

- [x] Task 2 — Add private `normalizeContext` helper(s) and replace inline guards
  - Acceptance criteria:
    - `normalizeContext` exists as an unexported helper in each affected package.
    - Targeted inline `ctx == nil` blocks are replaced with helper calls.
    - Fallback behavior remains `context.Background()`.
  - Relevant checks:
    - ✅ `go test ./internal/chat ./internal/storage ./internal/auth ./internal/server ./internal/observability -count=1`
  - Result:
    - Passed. Added helpers for `chat`, `storage`, `auth`, `server`, and `tracing`; replaced targeted nil-guard blocks in scoped files with `normalizeContext(ctx)`.

- [x] Task 3 — Validate regressions in affected packages
  - Acceptance criteria:
    - Affected package tests pass.
    - Task log records command and result.
  - Relevant checks:
    - ✅ `./scripts/verify.sh`
  - Result:
    - Passed. Full repository verification succeeded; Docker-specific checks were skipped by script because Docker is unavailable in this environment.

## Execution log
- ✅ `rg -n "if ctx == nil" internal/chat internal/storage internal/auth internal/server internal/observability` (pass).
- ✅ `go test ./internal/chat ./internal/storage ./internal/auth ./internal/server ./internal/observability -count=1` (pass).
- ✅ `./scripts/verify.sh` (pass with expected Docker-unavailable skips).
