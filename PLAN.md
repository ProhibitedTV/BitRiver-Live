# PLAN

## Scope (current change)
- Introduce a private shared default `http.Client` in `internal/ingest/adapters.go` for nil-client call paths.
- Update `postJSON` and `deleteRequest` to reuse the shared client while preserving existing timeout and retry semantics.
- Add tests in `internal/ingest/adapters_test.go` covering nil-client behavior for helper paths.

## Assumptions
- This change is internal to ingest adapter HTTP helper behavior and does not alter API contracts.
- Timeout value (`defaultHTTPTimeout`) and retry logic (`doWithRetry`) must remain unchanged.

## Risks
- Accidentally changing effective client timeout or retry behavior when refactoring nil-client fallback.
- Missing regression coverage for both POST and DELETE nil-client helper paths.

## Test plan
- Run targeted ingest tests: `go test ./internal/ingest -count=1`.
