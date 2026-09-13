# PLAN

## Current scope - chat message deletion and `/me` action contract (#1291) (2026-09-13)

- Extend the existing `/api/chat/ws` protocol instead of creating a second chat path. Add a persisted action-message kind for `/me` and a persisted message-deletion event that removes the targeted transcript row and tells connected viewers to remove the same row.
- Reuse the current channel owner/admin/moderator authorization boundary for deletion. Ordinary viewers may create action messages under the same ban, timeout, automod, length, and room-membership rules as ordinary messages, but may not delete messages.
- Make action semantics durable rather than encoding them as magic display text. Add a backwards-compatible `kind` field to chat messages with `message` as the default and `action` as the only new value, carry it through in-memory and PostgreSQL storage, REST history, gateway events, and viewer state.
- Keep deletion explicit on the wire with a dedicated event payload containing channel ID, message ID, actor ID, and deletion time. Persistence must validate that the message belongs to the requested channel; a failed persistence action must not be represented as a successful deletion acknowledgement.
- Render action rows as plain React text using the server-authored display identity; never inject HTML. A live delete event removes the matching row idempotently. Restored history preserves the same action/message distinction.
- Update the lightweight protocol documentation and viewer slash-command behavior together. `/me <text>` sends an action command; authorized deletion uses an explicit gateway command/event rather than overloading `/clear`, which remains local-only.

### Risks and boundaries

- This is a chat wire-format and persistence-schema change, so compatibility must be additive: existing rows default to `message`, existing clients can ignore the new delete event and unknown message `kind`, and no existing command changes meaning.
- Async queue persistence currently broadcasts ordinary messages before storage. Deletion must not announce success before the persistent delete is accepted; use the repository's existing delete primitive at the authorization boundary, then broadcast the durable result rather than publishing a best-effort delete that could fail later.
- PostgreSQL and JSON/in-memory repositories must remain behaviorally aligned. Migration and generated Helm migration assets must stay synchronized through the existing asset contract.
- Do not change Compose, root `.env`, release workflows, CI workflows, auth/session semantics, moderation audience policy, or stream lifecycle behavior. Do not claim #1382-#1384 completion from this work.

### Test and rollout plan

- Add focused gateway tests for `/me`/action creation, ordinary message compatibility, authorized deletion, ordinary-viewer refusal, wrong-channel/missing-message refusal, and idempotent client removal semantics.
- Add storage tests for action-kind persistence/history and deletion-event behavior in both in-memory and PostgreSQL paths; extend API history contract coverage for the additive `kind` field.
- Add viewer Jest coverage for `/me` payloads, safe action-row rendering, live deletion, history-restored action rows, and ignored duplicate deletion events. Keep raw HTML out of the rendering path.
- Run focused Go chat/storage/API tests, viewer lint/Jest/build, migration/Helm drift checks, then literal `./scripts/verify.sh` and protected PR CI. Merge only after the exact-head `Merge gate` is green and all review conversations are resolved.
