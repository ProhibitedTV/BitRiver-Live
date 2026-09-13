# PLAN

## Current scope - chat message deletion and `/me` action contract (#1291) (2026-09-13)

- Extend the existing `/api/chat/ws` protocol instead of creating a second chat path. Add an explicit action-message wire kind for `/me` and an explicit message-deletion event that removes the targeted transcript row and tells connected viewers to remove the same row.
- Preserve the existing persistent transcript schema by storing an action's canonical source form as `/me <text>`. Classify that form at the gateway/API boundary so history reloads recover `kind: "action"` and the display body without requiring a database migration. Ordinary messages remain `kind: "message"`; older clients that ignore `kind` continue to see meaningful `/me` source text in stored history.
- Reuse the current persistent-delete authorization boundary exactly: channel owners and admins may delete; ordinary viewers and moderator-role users without owner/admin authority may not. Ordinary viewers may create action messages under the same ban, timeout, automod, length, and room-membership rules as ordinary messages.
- Keep deletion explicit on the wire with a dedicated event payload containing channel ID, message ID, actor ID, and deletion time. Persistence must validate that the message belongs to the requested channel; a failed persistence action must not be represented as a successful deletion acknowledgement.
- Render action rows as plain React text using the server-authored display identity; never inject HTML. A live delete event removes the matching row idempotently. Restored history preserves the same action/message distinction by classifying canonical `/me ` source text.
- Update the lightweight protocol documentation and viewer slash-command behavior together. `/me <text>` sends an action command; authorized deletion uses an explicit gateway command/event rather than overloading `/clear`, which remains local-only.

### Risks and boundaries

- This is an additive chat wire-format change, not a persistence-schema migration: existing stored rows remain valid, existing clients can ignore the new delete event and message `kind`, and no existing command changes meaning.
- Canonical `/me ` source text is part of the persistence contract. Gateway-created ordinary messages that begin with `/me ` must therefore be classified as actions too, preventing older clients from producing live/history semantic drift.
- Async queue persistence currently broadcasts ordinary messages before storage. Deletion must not announce success before the persistent delete is accepted; use the repository's existing delete primitive at the authorization boundary, then broadcast the durable result rather than publishing a best-effort delete that could fail later.
- PostgreSQL and JSON/in-memory repositories must remain behaviorally aligned. No Compose, migration, Helm, root `.env`, release workflow, CI workflow, auth/session, moderation audience-policy, or stream-lifecycle change belongs in this slice.
- Do not claim #1382-#1384 completion from this work.

### Test and rollout plan

- Add focused gateway tests for `/me`/action creation, ordinary message compatibility, canonical-source classification, authorized deletion, ordinary-viewer/moderator refusal, and wrong-channel/missing-message refusal.
- Add storage/API tests proving canonical action source persists unchanged, history returns action kind/body, and deletion remains channel-bound in both repository implementations.
- Add viewer Jest coverage for `/me` payloads, safe action-row rendering, live deletion, history-restored action rows, and ignored duplicate deletion events. Keep raw HTML out of the rendering path.
- Run focused Go chat/storage/API tests, viewer lint/Jest/build, docs checks, then literal `./scripts/verify.sh` and protected PR CI. Merge only after the exact-head `Merge gate` is green and all review conversations are resolved.
