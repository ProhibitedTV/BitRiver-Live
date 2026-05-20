## Scope (current change)
- Address GitHub issue #1244 by making ingest boot retries cancellation-aware and reducing duplicated Postgres timeout plumbing where package boundaries allow.
- Keep public repository/store APIs unchanged.
- Update call sites in in-memory and Postgres stream start paths to pass an explicit parent context to `runIngestBootWithRetry`.
- Share auth Postgres operation/ping context helpers across session and MFA challenge stores.

## Assumptions
- Repository methods remain non-contextual today, so production call sites should pass `context.Background()` while the helper itself becomes cancellable for current and future callers.
- Retrying ingest boot should still stop immediately on terminal context errors from `BootStream`.
- Auth Postgres session and MFA challenge stores can share package-private helpers without changing exported options or behavior.
- Postgres legal flows are intentionally left to issue #1245.

## Risks
- Returning the parent cancellation error during retry delay could change error wrapping expectations if tests rely on the last transient ingest error.
- Retrying with a child timeout must still preserve the existing per-attempt deadline behavior.
- Over-eager Postgres cleanup could cross into storage legal work reserved for the next issue.

## Test plan
- `go test ./internal/storage -run "TestRunIngestBootWithRetry" -count=1 -timeout=120s`
- `go test ./internal/storage -count=1 -timeout=120s`
- `go test ./internal/auth ./internal/storage -count=1 -timeout=120s`
- `go test ./... -count=1 -timeout=120s`
- `git diff --check`
- `./scripts/verify.sh`
