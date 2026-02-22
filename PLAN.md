# PLAN

## Scope (current change)
- Refactor `doWithRetry` in `internal/ingest/adapters.go` to remove the `attempt = attempts` loop-control hack.
- Introduce explicit branching for retryable versus non-retryable failures while preserving existing error text and retry counts.
- Add/adjust tests in `internal/ingest/adapters_test.go` to prove non-429 4xx responses are not retried.

## Assumptions
- Existing status classification remains the same: retry on 5xx and 429; do not retry other 4xx.
- Error message formatting (including HTTP status/body composition) must stay unchanged.

## Risks
- Off-by-one retry regressions if control flow changes around loop exit.
- Accidentally changing returned error content for permanent HTTP failures.

## Test plan
- Run targeted ingest tests: `go test ./internal/ingest -count=1`.
