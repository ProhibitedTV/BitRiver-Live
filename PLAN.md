## Scope (current change)
- Address GitHub issue #1275 by exposing existing chat moderation commands in the viewer without adding a standalone moderation dashboard.
- Add viewer slash command parsing for `/timeout <user> <duration> [reason]`, `/ban <user> [reason]`, `/unban <user>`, `/remove_timeout <user>`, and `/clear`.
- Treat `/clear` as local transcript clearing only; do not mutate room history.
- Surface compact moderator actions from message rows for channel owners, admins, and moderators while keeping normal viewers on report-only controls.
- Tighten backend command behavior where needed: moderator role authorization and WebSocket reason propagation.
- Document unsupported follow-ups for backend message deletion and `/me` action messages.
- Keep the deployment contract untouched.

## Assumptions
- There is no channel-level moderator membership model yet. For this pass, moderation UI is shown to channel owners, users with `admin`, and users with `moderator`.
- Backend authorization remains authoritative. UI hiding is convenience, not security.
- Slash command targets resolve from visible chat users by ID or display name when possible, then fall back to the typed token so the backend can reject invalid users consistently.
- Message deletion/removal and `/me` action messages are not currently supported by the chat gateway and should be documented as follow-up work instead of approximated in the client.

## Risks
- Slash command display-name matching can be ambiguous; prefer exact user IDs and visible unique display names.
- Message-row controls can clutter the transcript; keep labels compact and only render them for users who can moderate the current room.
- WebSocket-only moderation means the viewer must show clear feedback when live chat is disconnected.
- Existing local Windows verification can be constrained by shell/tooling availability; use focused tests and GitHub Actions proof before merge.

## Test plan
- `go test ./internal/chat -run "TestGateway(ModerationFlow|RejectsUnauthorizedModeration|AllowsModeratorRole)" -count=1`
- `npm --prefix web/viewer run test -- chatPanel channelPage`
- `npm --prefix web/viewer run test:playwright -- tests/channel-chat-playback.spec.ts`
- `git diff --check`
- `./scripts/verify.sh`
