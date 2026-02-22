# PLAN

## Scope (current change)
- Refactor `waitFor` in `cmd/transcoder/main_test.go` to use `context.WithTimeout` and `time.Ticker` instead of deadline + sleep polling.
- Preserve existing transcoder test behavior assertions while reducing busy waiting in helper loop.
- Update all `waitFor` call sites in `cmd/transcoder/main_test.go` to the new helper signature and include explicit failure context.

## Assumptions
- Change is test-only and scoped to `cmd/transcoder/main_test.go`; no runtime code or deployment contract files are affected.
- Existing assertion expectations in transcoder tests should remain unchanged after helper refactor.

## Risks
- Helper signature update touches many call sites; a missed update could break compilation.
- Polling timing changes could make flaky tests more visible if timeout messages are not clear.

## Test plan
- Run targeted transcoder tests: `go test ./cmd/transcoder -count=1`.
