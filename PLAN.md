## Scope (current change)
- Address GitHub issue #1274 by extending the existing chat WebSocket protocol with live-room state primitives.
- Add gateway-owned presence state for room roster snapshots, join/leave deltas, viewer counts, and chatter counts.
- Add viewer-facing user metadata to live message payloads: display name, normalized role, and badge slots that can evolve without another wire-shape change.
- Add system event support for non-transcript room notices without persisting ephemeral presence as normal chat history.
- Preserve existing client commands and server envelopes: `join`, `leave`, `message`, `timeout`, `remove_timeout`, `ban`, `unban`, `report`, plus `ack`, `event`, and `error`.
- Keep the deployment contract untouched.

## Assumptions
- Presence is live transport state only; it should not flow through the persistence queue or transcript storage.
- The current `domain.User` and `domain.Channel` records are enough for initial display metadata: ID, display name, owner/admin/creator/moderator/viewer role, and first-party badges.
- Multiple WebSocket connections for one authenticated user should count as one visible chatter in a room while still allowing each connection to receive room events.
- System events can be emitted by backend services through a gateway method and should use the same `event` server envelope as chat and moderation events.

## Risks
- Locking can become fragile if presence mutation and broadcast fan-out happen in one critical section; keep snapshots small and copy recipients before sending.
- Existing clients may ignore unknown event types, but they must not lose message/moderation behavior or receive malformed `ack` payloads.
- Adding metadata to message events must remain backward-compatible for queue consumers and storage code.
- Local Windows Go test runs may be constrained by host memory; use focused package tests first and rely on GitHub Actions for full matrix proof if the host cannot compile.

## Test plan
- `go test ./internal/chat -run "TestGateway(Presence|MessageMetadata|SystemEvent|MessageFlow|ModerationFlow)" -count=1`
- `go test ./internal/chat -count=1`
- `go test ./internal/storage -run TestApplyChatEvent -count=1`
- `git diff --check`
- `./scripts/verify.sh`
