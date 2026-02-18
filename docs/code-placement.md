# Code placement guide

Use this guide when deciding where new Go code belongs.

## 1) New package vs existing package (`internal/*`)

Prefer adding to an existing package when:
- The new behavior extends the same bounded context and data ownership.
- The existing package already owns the interface or implementation you need.
- You can keep imports aligned with `docs/architecture.md` dependency direction.

Create a new `internal/<name>` package when all are true:
- The behavior introduces a distinct domain/integration concern.
- Existing packages would become mixed-responsibility if you add it there.
- The package boundary is stable enough to justify separate tests and APIs.

Quick checks:
- **Business rules/invariants?** Put in `internal/domain` or `internal/service`.
- **HTTP transport only?** Put in `internal/api`.
- **Infra adapter for external system?** Put in an adapter package such as `internal/storage`, `internal/chat`, `internal/auth`, `internal/ingest`, or `internal/observability`.

## 2) What belongs in util packages

Utility packages are foundation helpers shared by wiring and adapters. Keep them small and generic.

- `internal/envutil`: environment variable parsing/defaulting/validation helpers.
  - Example: DSN assembly lives under `internal/envutil/pgdsn`.
- `internal/executil`: process execution wrappers and command invocation helpers.
- `internal/serverutil`: generic server lifecycle/run-loop helpers.
- `internal/platformutil`: OS/runtime capability helpers (for example Python/tooling detection).
- `internal/stringsutil`: string helpers with no business semantics.

A util package should usually answer one of these:
- "How do we read config from env safely?"
- "How do we run/stop a process consistently?"
- "How do we do a generic string/platform operation?"

## 3) Do not mix business logic into util packages

Never place business decisions in util code.

Do **not** put in util packages:
- Channel/account/payment/auth policy decisions.
- Domain validation tied to BitRiver entities.
- Storage/query behavior specific to recordings, users, or streams.
- API request/response shaping.

Instead:
- Put domain rules in `internal/domain`.
- Put use-case orchestration in `internal/service` (for example `internal/service/uploads`).
- Put transport mapping in `internal/api`.
- Put concrete integrations in adapter packages (`internal/storage`, `internal/auth`, etc.).

## 4) Repo-specific examples

- If you need a helper to read `BITRIVER_*` env vars for compose/runtime config, add to `internal/envutil` (or `internal/envutil/pgdsn` for PostgreSQL DSN concerns), not `internal/service`.
- If you are adding upload business flow (validation, orchestration, policy), extend `internal/service/uploads`, not `internal/executil`.
- If you need to call ffmpeg/CLI tools, process management helpers belong in `internal/executil`; stream business decisions still belong in `internal/service`.
- If you add HTTP endpoints, handler/request mapping belongs in `internal/api`, while the underlying behavior belongs in `internal/service` or `internal/domain`.
- If you add persistence for a new entity, implementation belongs in `internal/storage`; shared domain contracts should be defined in `internal/domain` or `internal/service`.

## 5) Placement checklist before opening PR

- Confirm package responsibility is single-purpose.
- Confirm imports follow `docs/architecture.md` layer rules.
- Confirm util packages stay dependency-light and business-free.
- If you introduce a new top-level `internal/*` package, document why in PR summary.
