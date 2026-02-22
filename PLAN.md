# PLAN

## Scope (current change)
- Identify cohesive helper/function groups in `internal/storage/storage.go` and `internal/storage/postgres_channels.go`.
- Move private helpers/methods into new `internal/storage/*.go` files grouped by concern, without changing package name, exported API surface, or SQL text/behavior.
- Keep all call sites unchanged aside from file placement.
- Run storage-focused tests, including integration-tagged tests that exercise moved PostgreSQL storage code.

## Assumptions
- This is a pure refactor: no runtime behavior changes are intended.
- Grouping by concern can be done via new files while remaining in package `storage`.
- Existing tests provide sufficient regression coverage for moved helpers/methods.

## Risks
- Moving methods can accidentally alter imports or introduce subtle compile issues.
- Integration-tagged tests may require local services; if unavailable, record environment limitation.
- Large file splits can accidentally modify SQL literals if not copied exactly.

## Test plan
- `go test ./internal/storage -count=1`
- `go test ./internal/storage -count=1 -tags=integration`
