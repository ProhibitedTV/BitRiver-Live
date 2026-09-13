# TASKS

## Scoped change: chat message deletion and `/me` action contract (#1291)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Reconcile the existing chat, storage, and viewer contract
  - Acceptance criteria:
    - The current WebSocket command/event shapes, persistence path, deletion primitive, authorization boundary, viewer rendering path, and issue acceptance criteria are inspected before implementation.
    - The change does not invent a parallel chat stack or conflate local `/clear` with persistent deletion.
  - Check:
    - Gateway messages publish through the chat queue; in-memory and PostgreSQL repositories persist them. Existing storage already has channel-bound `DeleteChatMessage`, while the wire protocol explicitly lacks `/me` and deletion events.
    - Action semantics can remain durable without a schema migration by persisting the canonical `/me <text>` source form and classifying that source at the realtime/API boundary.
    - Existing persistent-delete authorization is channel owner/admin only; moderator-role users without owner/admin authority remain unauthorized for transcript deletion.

- [-] Task 2 - Implement durable backend action and deletion contracts
  - Acceptance criteria:
    - `message.kind` is additive/backwards compatible and defaults to `message`; action creation uses the same content/access/automod rules as ordinary chat while canonical `/me <text>` remains the stored source form.
    - Owner/admin deletion validates channel/message identity, persists the delete before success is broadcast, and produces an explicit realtime delete event. Ordinary viewers and moderator-only users are refused.
    - JSON/in-memory and PostgreSQL storage remain aligned without a schema migration.
  - Check:
    - Pending implementation and focused Go/storage tests.

- [ ] Task 3 - Wire safe viewer behavior and protocol documentation
  - Acceptance criteria:
    - `/me <text>` produces an action message and action rows render as plain text without raw HTML.
    - Live delete events remove the target idempotently; REST-restored action rows retain their semantic presentation.
    - Protocol docs define command/event/persistence/authorization behavior and preserve `/clear` as local-only.
  - Check:
    - Pending viewer tests, lint, build, and docs checks.

- [ ] Task 4 - Run full qualification and review
  - Acceptance criteria:
    - Focused backend/viewer checks and literal `./scripts/verify.sh` pass.
    - Protected PR CI passes on the exact head, including the aggregate `Merge gate`.
    - Review conversations are resolved without weakening tests or authorization.
  - Check:
    - Pending.

- [ ] Task 5 - Squash merge and close #1291
  - Acceptance criteria:
    - The side branch is squash-merged into current protected `main` without bypassing checks.
    - #1291 is closed only after the merged code satisfies the documented acceptance criteria.
    - Final ledger state records the merged PR/head evidence and no unrelated issue is claimed complete.
  - Check:
    - Pending.
