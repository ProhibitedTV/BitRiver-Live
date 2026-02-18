# Architecture contract

This document defines the **rigid architecture** for BitRiver Live. Treat it as the canonical reference when adding features, refactoring, or reviewing pull requests.

## Goals

- Keep dependencies one-way and predictable.
- Keep business logic independent from HTTP, CLI, and infrastructure concerns.
- Make behaviour testable without Docker, network services, or framework bootstrapping.

## Backend layering (Go)

Allowed dependency direction:

```text
cmd/*
  -> internal/app
    -> internal/api
    -> internal/service
    -> internal/domain
    -> internal/storage, internal/ingest, internal/chat, internal/auth, internal/observability
```

Rules:

1. `cmd/*` contains process entrypoints only (flags, config loading, startup/shutdown wiring).
2. `internal/app` orchestrates composition and lifecycle management; it can wire concrete adapters to interfaces.
3. `internal/api` contains transport handlers and request/response translation only.
4. `internal/service` contains use-case/application logic.
5. `internal/domain` contains core domain models, invariants, and domain-level interfaces.
6. `internal/storage`, `internal/ingest`, `internal/chat`, `internal/auth`, and `internal/observability` are infrastructure and integration adapters.
7. Infrastructure packages must not import `internal/api` or `cmd/*`.
8. `internal/service` and `internal/domain` must not depend on concrete transport types (HTTP request/response structs), CLI flags, or Docker-specific runtime types.

## Deployment contract (canonical)

BitRiver Live has one deployment pipeline regardless of launcher: `deploy/docker-compose.yml` orchestrated with the repository-root `.env` lifecycle (generate/validate/render/bootstrap). Platform-specific launchers only change command syntax; they must not introduce a second operational runbook. See [`docs/quickstart.md`](quickstart.md#shared-backend-pipeline-all-launchers) for the shared stage sequence and [`docs/cross-platform-plan.md`](cross-platform-plan.md#canonical-production-deployment-path) for cross-platform rollout constraints.


## Domain migration status

- `internal/domain` now exists as the canonical home for core business entities.
- During migration, `internal/domain` re-exports symbols from `internal/models` to keep incremental import changes safe.
- The type-by-type mapping is tracked in `internal/domain/migration_map.md`.
- New service and API code should import `internal/domain`; direct `internal/models` imports should be considered legacy and migrated opportunistically.

## Frontend boundary (Next.js viewer)

- `web/viewer` is an independent UI delivery layer.
- Viewer code should consume stable API contracts and avoid importing backend Go concepts directly.
- Backend behaviour changes that alter payload shape or semantics must update viewer-facing docs and migration notes in the same PR.

## Data/control flow expectations

- Incoming HTTP/API requests are validated and translated in `internal/api`, then handed to service interfaces.
- Service/domain logic calls infrastructure through interfaces (repositories, gateways, publishers).
- Adapter implementations live in infrastructure packages and are injected by `internal/app`.

## Enforcement checklist for contributors

Before opening a PR that touches runtime behaviour:

- Confirm imports respect the dependency direction above.
- Confirm new business logic is in `internal/service`/`internal/domain`, not in handlers or entrypoints.
- Confirm integration-specific code is isolated in adapter packages.
- Add/update tests at the layer where behaviour is defined.
- If this contract changes, update this document and the root `AGENTS.md` in the same PR, and explain the rationale in the PR body.
