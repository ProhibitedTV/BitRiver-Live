## Scoped change: viewer chat moderation controls and slash commands (#1275)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Establish moderation UX scope
  - Acceptance criteria:
    - `PLAN.md` captures #1275 scope, assumptions, risks, and test plan.
    - `TASKS.md` lists ordered tasks before source edits for this pass.
    - Existing gateway moderation commands, viewer auth roles, chat row UI, report UI, and protocol docs are reviewed.
  - Check:
    - Read-only analysis only.

- [x] Task 2 - Tighten backend moderation command handling
  - Acceptance criteria:
    - WebSocket moderation commands propagate `reason` into `ModerationEvent`.
    - Users with `moderator` role can moderate through the same backend path as owners/admins.
    - Unauthorized viewers still receive structured errors and cannot moderate by direct WebSocket command.
  - Check:
    - `go test ./internal/chat -run "TestGateway(ModerationFlow|RejectsUnauthorizedModeration|AllowsModeratorRole)" -count=1` passed with repo-local `GOCACHE`/`GOTMPDIR` after the default Go cache path failed to initialize.

- [x] Task 3 - Add viewer slash command handling
  - Acceptance criteria:
    - `/timeout`, `/ban`, `/unban`, `/remove_timeout`, and `/clear` are parsed locally.
    - Valid moderation commands send the existing WebSocket payloads.
    - Invalid commands, missing targets, invalid durations, disconnected sockets, and unauthorized users show useful local feedback.
    - `/me` reports that action messages are not supported yet.
  - Check:
    - `npm.cmd --prefix web/viewer run test -- chatPanel channelPage` passed.

- [x] Task 4 - Add permission-gated message row controls
  - Acceptance criteria:
    - Normal viewers only see existing report controls.
    - Channel owners, admins, and moderators see compact timeout/ban/remove-timeout/unban actions for other users' messages.
    - Destructive actions are explicit and errors surface in the chat panel.
  - Check:
    - `npm.cmd --prefix web/viewer run test -- chatPanel channelPage` passed.

- [x] Task 5 - Update protocol and viewer docs
  - Acceptance criteria:
    - `internal/chat/PROTOCOL.md` documents moderation permissions, command `reason`, and viewer slash command expectations.
    - `web/viewer/README.md` documents local `/clear` behavior and unsupported message deletion/`/me` follow-ups.
  - Check:
    - `git diff --check` passed.
    - Follow-up issue #1291 opened for unsupported message deletion and `/me` action events.

- [ ] Task 6 - Verify, publish, and merge
  - Acceptance criteria:
    - Focused backend and viewer tests pass or host blockers are recorded.
    - `git diff --check` passes.
    - `./scripts/verify.sh` runs or host blockers are recorded.
    - PR is opened, CI is monitored, and #1275 is closed by merge.
  - Check:
    - `git diff --check` passed.
    - `bash ./scripts/verify.sh` blocked locally because Windows `bash.exe` requires a WSL distro and none is installed.

### Execution log
- Task 1 read-only pass:
  - Confirmed #1275 is open and scoped to viewer moderation controls, slash commands, backend permission enforcement, and docs.
  - Reviewed `internal/chat/gateway.go`, `internal/chat/event.go`, `internal/chat/gateway_test.go`, `internal/chat/PROTOCOL.md`, `web/viewer/components/ChatPanel.tsx`, `web/viewer/hooks/useAuth.tsx`, `web/viewer/lib/viewer-api-chat.ts`, `web/viewer/lib/viewer-api-types.ts`, `web/viewer/__tests__/chatPanel.test.tsx`, and `web/viewer/README.md`.
  - Found backend WebSocket commands already exist for `timeout`, `remove_timeout`, `ban`, `unban`, and `report`, but `reason` is not copied into moderation events and `moderator` role is not currently accepted by backend authorization.
- Task 2 implementation:
  - WebSocket moderation commands now copy trimmed `reason` into `ModerationEvent`.
  - Backend moderation authorization now allows channel owners, admins, and moderators.
  - Added regression coverage for unauthorized viewer rejection, moderator role success, and reason propagation.
- Task 3 implementation:
  - Added slash command parsing for `/timeout`, `/ban`, `/unban`, `/remove_timeout`, `/untimeout`, and local `/clear`.
  - Added local errors for unknown commands, invalid durations, unsupported `/me`, disconnected moderation sockets, and unauthorized moderation attempts.
- Task 4 implementation:
  - Passed `channelOwnerId` into `ChatPanel` and gated row controls to owners, admins, and moderators.
  - Added compact timeout, remove-timeout, ban, and unban actions for other users' messages.
  - Kept normal viewers on report-only controls.
- Task 5 documentation:
  - Updated `internal/chat/PROTOCOL.md` with moderation permissions, optional reason, viewer slash commands, local `/clear`, and unsupported `/me`/message delete follow-ups.
  - Updated `web/viewer/README.md` chat control contract with moderator actions and slash command behavior.
  - Created follow-up #1291 for the missing message deletion and `/me` backend event contract.
- Verification so far:
  - `go test ./internal/chat -run "TestGateway(ModerationFlow|RejectsUnauthorizedModeration|AllowsModeratorRole)" -count=1` passed with repo-local `GOCACHE`/`GOTMPDIR`.
  - `npm.cmd --prefix web/viewer run test -- chatPanel channelPage` passed with 44 focused Jest tests.
  - `npm.cmd --prefix web/viewer run test:playwright -- tests/channel-chat-playback.spec.ts` passed with 4 Playwright tests after a production build.
  - `git diff --check` passed.
  - `bash ./scripts/verify.sh` could not run locally: WSL reports `WSL_E_DEFAULT_DISTRO_NOT_FOUND`.
