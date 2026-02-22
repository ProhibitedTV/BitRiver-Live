# PLAN

## Scope (current change)
- Add a reusable async wait helper in `internal/testsupport` for polling test conditions with timeout handling.
- Replace the local `waitUntil` helper in `internal/chat/gateway_test.go` with the shared helper.
- Preserve existing assertion semantics in chat gateway tests while improving timeout failure diagnostics.

## Assumptions
- This is a test-only refactor; runtime behavior and deployment contract files are unaffected.
- Existing chat test expectations remain the same (success still depends on condition becoming true before timeout).

## Risks
- Moving to a shared helper could subtly change polling cadence or fatal message formatting.
- If helper API is unclear, future tests may pass unhelpful context; include explicit diagnostic message support.

## Test plan
- Run targeted chat tests: `go test ./internal/chat -count=1`.
