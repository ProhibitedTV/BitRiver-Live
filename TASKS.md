# TASKS

## Scoped change: chat message deletion and `/me` action contract (#1291)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Reconcile the existing chat, storage, and viewer contract
  - Acceptance criteria:
    - The current WebSocket command/event shapes, persistence path, deletion primitive, authorization boundary, viewer rendering path, and issue acceptance criteria are inspected before implementation.
    - The change does not invent a parallel chat stack or conflate local `/clear` with persistent deletion.
  - Check:
    - Gateway messages publish through the chat queue; in-memory and PostgreSQL repositories persist them. Existing storage already has channel-bound `DeleteChatMessage`, while the wire protocol explicitly lacks `/me` and deletion events.
    - The current domain/database message shape has no durable kind, so action semantics need an additive persisted `kind` field rather than a magic-text convention.
    - Existing moderation authorization is owner/admin/moderator; ordinary viewers remain unauthorized to delete transcript messages.

- [-] Task 2 - Implement durable backend action and deletion contracts
  - Acceptance criteria:
    - `message.kind` is additive/backwards compatible and defaults to `message`; action creation uses the same content/access/automod rules as ordinary chat.
    - Authorized deletion validates channel/message identity, persists the delete before success is broadcast, and produces an explicit realtime delete event.
    - JSON/in-memory and PostgreSQL storage remain aligned, with a forward migration for message kind.
  - Check:
    - Pending implementation and focused Go/storage tests.

- [ ] Task 3 - Wire safe viewer behavior and protocol documentation
  - Acceptance criteria:
    - `/me <text>` produces an action command and action rows render as plain text without raw HTML.
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
