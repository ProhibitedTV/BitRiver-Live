## Scope (current change)
- Address GitHub issue #1245 by routing Postgres-backed legal repository methods through the repository acquire/timeout helpers.
- Keep legal repository public APIs and observable behavior unchanged.
- Extract small normalization helpers only where they reduce repeated trim/status handling without widening the change.
- Update cleanup tracking after targeted storage validation passes.

## Assumptions
- Repository methods remain non-contextual today, so `withConn` and `acquireContext` are the available cancellation/timeout boundary.
- Best-effort legal state-history inserts should remain best-effort unless the existing method already treats history persistence as part of the main operation.
- `Get*` methods should continue returning `(zero, false)` on query/acquire failures because the public contract does not expose an error.
- There are no focused storage legal tests today, so package-level storage tests plus static scans are the main validation path.

## Risks
- Moving multiple statements onto one acquired connection can change pool pressure slightly, especially around best-effort history inserts.
- Adding helper wrappers around bool-returning getters must not accidentally convert infrastructure errors into user-visible errors.
- Legal status and ID normalization should remain byte-for-byte equivalent where possible to avoid changing stored values.

## Test plan
- `go test ./internal/storage -count=1 -timeout=120s`
- `go test ./... -count=1 -timeout=120s`
- `git diff --check`
- `./scripts/verify.sh`
