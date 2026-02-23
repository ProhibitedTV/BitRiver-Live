# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Propagate context through upload delete helpers
  - Acceptance criteria:
    - `deleteUploadMedia` accepts a `context.Context` and all handler call sites pass request context where available.
    - `deleteStoredUploadSource` accepts a `context.Context` and no longer uses `context.Background()` for delete operations.
    - Any async/best-effort cleanup path derives a short timeout context and includes an explanatory code comment.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run TestDeleteUploadRemovesDurableSourceObject -count=1 -timeout=120s`

- [x] Task 2 — Add cancellation/context propagation tests for delete path
  - Acceptance criteria:
    - Tests verify delete is attempted with propagated context.
    - Tests verify canceled contexts are respected by storage delete implementation.
  - Relevant checks:
    - ❌ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run 'TestDeleteUploadMediaPropagatesContextToStoreDelete|TestDeleteUploadMediaRespectsCanceledContext' -count=1 -timeout=120s` (initial failure: test fixture did not initialize uploadSourceOnce)
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run 'TestDeleteUploadMediaPropagatesContextToStoreDelete|TestDeleteUploadMediaRespectsCanceledContext' -count=1 -timeout=120s`

- [x] Task 3 — Run verification and record outcomes
  - Acceptance criteria:
    - Updated package tests pass.
    - Repo verify script is executed and results captured.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1 -timeout=120s`
    - ✅ `./scripts/verify.sh` (passed; docker-dependent checks skipped due missing docker binary)

## Execution log
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run TestDeleteUploadRemovesDurableSourceObject -count=1 -timeout=120s`.
- ❌ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run 'TestDeleteUploadMediaPropagatesContextToStoreDelete|TestDeleteUploadMediaRespectsCanceledContext' -count=1 -timeout=120s` (initial failure: test fixture did not initialize uploadSourceOnce).
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -run 'TestDeleteUploadMediaPropagatesContextToStoreDelete|TestDeleteUploadMediaRespectsCanceledContext' -count=1 -timeout=120s`.
- ✅ `gofmt -w internal/api/uploads_handlers.go internal/api/uploads_handlers_test.go`.
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/api -count=1 -timeout=120s`.
- ✅ `./scripts/verify.sh` (passed; docker-dependent checks skipped due missing docker binary).
