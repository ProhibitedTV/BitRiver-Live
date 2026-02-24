# PLAN

## Scope (current change)
- Introduce a shared context utility package under `internal/ctxutil` that exposes behavior equivalent to existing package-local `normalizeContext` helpers.
- Replace `normalizeContext` usage in:
  - `internal/server`
  - `internal/storage`
  - `internal/chat`
- Remove duplicate package-local context helper files after all references are migrated.
- Add a minimal unit test for nil-context passthrough behavior in the new shared utility.

## Assumptions
- Existing behavior is: nil context should become `context.Background()`, non-nil context is returned unchanged.
- No call-site semantics should change beyond import/path updates.
- A focused unit test in `internal/ctxutil` is sufficient coverage for shared behavior.

## Risks
- Missed call sites may leave duplicate helpers or compile failures.
- Package import cycles could occur if utility placement is wrong (keep helper dependency-free in `internal/ctxutil`).
- Removing files too early may break packages not included in initial search.

## Test plan
- `gofmt -w internal/ctxutil/context.go internal/ctxutil/context_test.go internal/server/redis_store.go internal/storage/postgres_repository.go internal/chat/websocket.go internal/chat/redis_queue.go`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/ctxutil -count=1 -timeout=120s`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/server ./internal/storage ./internal/chat -count=1 -timeout=120s`
- `./scripts/verify.sh`
