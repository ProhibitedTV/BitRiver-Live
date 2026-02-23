# PLAN

## Scope (current change)
- Update upload delete cleanup helpers in `internal/api/uploads_handlers.go` so handler call sites pass request `context.Context` into:
  - `deleteUploadMedia`
  - `deleteStoredUploadSource`
- Remove direct use of `context.Background()` for upload source deletion.
- Where cleanup runs outside the request lifecycle (best-effort rollback path), derive a short timeout context and document why.
- Add focused tests in `internal/api/uploads_handlers_test.go` verifying:
  - delete is attempted with the provided context
  - canceled context is propagated and respected by delete path

## Assumptions
- Delete failures remain best-effort/logged and must not change HTTP response semantics.
- Request-scoped delete should use `r.Context()` so cancellation/deadlines flow through object-store operations.
- Async/best-effort rollback cleanup should not block indefinitely, so a bounded timeout context is appropriate.

## Risks
- Changing helper signatures can miss call sites and break compilation.
- Tight timeout for async cleanup might be too short for some environments; keep it short but reasonable and document intent.
- Tests that rely on cancellation need deterministic synchronization to avoid flakiness.

## Test plan
- `gofmt -w internal/api/uploads_handlers.go internal/api/uploads_handlers_test.go`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run 'TestDeleteUploadRemovesDurableSourceObject|TestDeleteUploadMediaPropagatesContextToStoreDelete|TestDeleteUploadMediaRespectsCanceledContext' -count=1 -timeout=120s`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1 -timeout=120s`
- `./scripts/verify.sh`
