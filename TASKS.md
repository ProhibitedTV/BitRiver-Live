## Scoped change: extend chat room protocol events (#1274)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Establish protocol scope
  - Acceptance criteria:
    - `PLAN.md` captures #1274 scope, assumptions, risks, and test plan.
    - `TASKS.md` lists ordered tasks before source edits for this pass.
    - Existing chat gateway, event types, queue persistence, protocol docs, and viewer parsing are reviewed.
  - Check:
    - Read-only analysis only.

- [x] Task 2 - Add protocol event shapes
  - Acceptance criteria:
    - Chat events can carry optional display metadata without breaking message persistence.
    - Presence and system events have typed payloads with channel IDs, counts, and timestamps.
    - Event channel routing includes message, moderation, report, automod, presence, and system payloads.
  - Check:
    - `gofmt -w internal/chat/event.go internal/chat/gateway.go internal/chat/gateway_test.go` passed.
    - `go test ./internal/chat -run "TestGateway(Presence|MessageMetadata|SystemEvent|MessageFlow|ModerationFlow)" -count=1` blocked by the default Go build cache error, then by zero free disk space in the Windows temp directory.

- [x] Task 3 - Track room presence in the gateway
  - Acceptance criteria:
    - `join` sends `ack` and a `presence_snapshot` to the joining connection.
    - First connection for a user broadcasts `presence_join`; final disconnect/leave broadcasts `presence_leave`.
    - Multiple tabs for the same user do not inflate `chatterCount`.
    - Existing `join`, `leave`, `message`, and moderation commands still behave.
  - Check:
    - Focused gateway test command attempted; blocked by local disk exhaustion before package compilation completed.

- [x] Task 4 - Emit role/badge metadata and system events
  - Acceptance criteria:
    - Message events include safe author metadata for compact viewer rows.
    - Owner/admin/creator/moderator/viewer role resolution is deterministic.
    - Backend code can broadcast a room `system` event without writing it to chat history.
  - Check:
    - Focused gateway test command attempted; blocked by local disk exhaustion before package compilation completed.

- [x] Task 5 - Update protocol docs
  - Acceptance criteria:
    - `internal/chat/PROTOCOL.md` documents presence snapshot/join/leave, user metadata, system events, and compatibility notes.
    - Docs state that presence/system room notices are live events and not transcript persistence.
  - Check:
    - `git diff --check` passed.

- [-] Task 6 - Verify, publish, and merge
  - Acceptance criteria:
    - Focused chat tests pass.
    - Relevant storage compatibility test passes.
    - `git diff --check` passes.
    - `./scripts/verify.sh` runs or host blockers are recorded.
    - PR is opened, CI is monitored, and #1274 is closed by merge.
  - Check:
    - Pending.

### Execution log
- Task 1 read-only pass:
  - Confirmed #1274 is open and scoped to backend/protocol support for room roster, presence events, role/badge metadata, and system events.
  - Reviewed `internal/chat/event.go`, `internal/chat/gateway.go`, `internal/chat/gateway_test.go`, `internal/chat/websocket.go`, `internal/chat/queue.go`, `internal/chat/redis_queue.go`, `internal/storage/chat_events.go`, `internal/chat/PROTOCOL.md`, `web/viewer/components/ChatPanel.tsx`, and `web/viewer/lib/viewer-api-types.ts`.
  - Chose ephemeral gateway presence: live `event` envelopes to room subscribers, no queue persistence for presence or system notices by default.
- Task 2 implementation:
  - Added `presence_snapshot`, `presence_join`, `presence_leave`, and `system` event types.
  - Added `UserMetadata`, `UserBadge`, `PresenceEvent`, and `SystemEvent` payloads.
  - Added optional `message.user` metadata while leaving `message.userId` unchanged for existing clients and storage consumers.
- Task 3 implementation:
  - Added gateway presence tracking keyed by channel and user with per-user connection counts.
  - `join` now sends `ack` plus a `presence_snapshot` to the joining connection.
  - First user connection broadcasts `presence_join`; final user leave/disconnect broadcasts `presence_leave`.
- Task 4 implementation:
  - Added deterministic role/badge metadata for owner, admin, moderator, broadcaster, and viewer users.
  - Added `BroadcastSystemEvent` for live room notices that do not enter chat transcript persistence.
  - Updated the viewer chat WebSocket parser to prefer live author display metadata when present.
- Task 5 documentation:
  - Updated `internal/chat/PROTOCOL.md` with message metadata, presence event, system event, and compatibility examples.
- Local verification:
  - `git diff --check` passed.
  - `go test ./internal/chat -run "TestGateway(Presence|MessageMetadata|SystemEvent|MessageFlow|ModerationFlow)" -count=1` failed before compilation because the default Go build cache could not create `AppData\Local\go-build\00`.
  - Rerunning with repo-local `GOCACHE` reached compilation, then failed because `C:\Users\RHYTHM~1\AppData\Local\Temp` reported `There is not enough space on the disk`.
  - `Get-PSDrive -Name C` reported zero free space, so local Go verification and full `./scripts/verify.sh` are blocked until disk space is recovered or CI runs.
