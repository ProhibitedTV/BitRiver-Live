# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add shared context utility
  - Acceptance criteria:
    - New `internal/ctxutil` package provides a function equivalent to existing `normalizeContext` behavior.
    - Minimal unit test verifies nil input returns `context.Background()` behavior and non-nil passthrough.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/ctxutil -count=1 -timeout=120s`

- [x] Task 2 — Migrate server/storage/chat to shared utility
  - Acceptance criteria:
    - `internal/server`, `internal/storage`, and `internal/chat` use shared helper instead of local `normalizeContext`.
    - Package-local duplicate context helper files are removed.
    - Affected files are gofmt-formatted.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server ./internal/storage ./internal/chat -count=1 -timeout=120s`

- [x] Task 3 — Run full verification
  - Acceptance criteria:
    - Repository verification script is executed and results captured.
  - Relevant checks:
    - ✅ `./scripts/verify.sh` (passed; docker-dependent checks skipped because docker is not installed)

## Execution log
- ✅ `gofmt -w internal/ctxutil/context.go internal/ctxutil/context_test.go internal/server/redis_store.go internal/storage/postgres_repository.go internal/chat/redis_queue.go internal/chat/websocket.go`.
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/ctxutil -count=1 -timeout=120s`.
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server ./internal/storage ./internal/chat -count=1 -timeout=120s`.
- ✅ `./scripts/verify.sh` (passed; docker compose validation steps were skipped because docker is not installed in this environment).
