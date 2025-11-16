# `internal/chat` Guidance

Chat gateway logic, moderation, and persistence live here. Review the root `AGENTS.md` first.

## Protocol + dependencies
- The WebSocket protocol is documented in `PROTOCOL.md`. Update the doc whenever message schemas or event names change.
- `Gateway` depends on `Queue` and `Store`. Keep the abstractions clean—inject dependencies instead of importing concrete types directly.
- Moderation/reporting flows run through `Gateway.CreateMessage`, `ApplyModeration`, and `SubmitReport`. Ensure each step enforces rate limits and auditing hooks.
- The lightweight JS client at `web/static/chat-client.js` must be updated in lock-step with protocol changes.

## Testing
- Cover behaviour in `internal/chat/*_test.go` (unit) and reuse Redis stubs from `internal/testsupport` for queue behaviour.
- When adding new message types, add fixtures/tests for encoding/decoding and client compatibility.

## Before opening a PR
- Sync documentation (`PROTOCOL.md`, README) and the web client.
- Run `go test ./internal/chat -count=1` plus repo suites.
- Exercise viewer/chat flows manually (see `web/manual-qa.md`) when changing UX-visible behaviour.
