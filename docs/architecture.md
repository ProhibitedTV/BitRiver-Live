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
    -> internal/foundation
    -> internal/storage, internal/ingest, internal/chat, internal/auth, internal/observability
    -> internal/security
```

Rules:

1. `cmd/*` contains process entrypoints only (flags, config loading, startup/shutdown wiring). `cmd/server` must delegate dependency composition to `internal/app.NewServerRuntime`.
2. `internal/app` orchestrates composition and lifecycle management; it can wire concrete adapters to interfaces.
3. `internal/api` contains transport handlers and request/response translation only. Handlers must depend on use-case interfaces (owned by `internal/service` or `internal/domain`) and must not call `storage.Repository` directly for business operations.
4. `internal/service` contains use-case/application logic.
5. `internal/domain` contains core domain models, invariants, and domain-level interfaces.
6. `internal/domain` is the canonical home for core business entities and value objects. `internal/models` is a legacy compatibility package that aliases `internal/domain` for incremental migration only; new business logic must import `internal/domain` directly.
7. `internal/config`, `internal/envutil`, `internal/executil`, `internal/platformutil`, `internal/serverutil`, and `internal/stringsutil` form a shared foundation/utilities layer used by top-level wiring and adapters. Keep these packages dependency-light: they must not import higher-level application, domain, or adapter layers.
8. `internal/storage`, `internal/ingest`, `internal/chat`, `internal/auth`, `internal/security`, and `internal/observability` are infrastructure and integration adapters.
9. `internal/service/uploads` currently defines the upload-processing contract used by storage-backed implementations; `internal/storage` may import this package until that contract is relocated to a neutral domain-owned package.
10. Infrastructure packages must not import `internal/api` or `cmd/*` (except for the `internal/storage -> internal/service/uploads` migration compatibility edge above).
11. `internal/service` and `internal/domain` must not depend on concrete transport types (HTTP request/response structs), CLI flags, or Docker-specific runtime types.

## Deployment contract (canonical)

BitRiver Live has one deployment pipeline regardless of launcher: `deploy/docker-compose.yml` orchestrated with the repository-root `.env` lifecycle (generate/validate/render/bootstrap). Platform-specific launchers only change command syntax; they must not introduce a second operational runbook. See [`docs/quickstart.md`](quickstart.md#shared-backend-pipeline-all-launchers) for the shared stage sequence and [`docs/cross-platform-plan.md`](labs/cross-platform-plan.md#canonical-production-deployment-path) for cross-platform rollout constraints.


## Domain migration status

- `internal/domain` now exists as the canonical home for core business entities.
- Core business entities and value objects now live in `internal/domain` and should be imported directly.
- `internal/models` remains as a legacy compatibility layer that aliases domain types while older call sites are migrated.
- The historical type-by-type migration map remains in `internal/domain/migration_map.md` for reference.

## Frontend boundary (Next.js viewer)

- `web/viewer` is an independent UI delivery layer.
- Viewer code should consume stable API contracts and avoid importing backend Go concepts directly.
- Backend behaviour changes that alter payload shape or semantics must update viewer-facing docs and migration notes in the same PR.

## Data/control flow expectations

- Incoming HTTP/API requests are validated and translated in `internal/api`, then handed to service interfaces.
- Service/domain logic calls infrastructure through interfaces (repositories, gateways, publishers).
- Adapter implementations live in infrastructure packages and are injected by `internal/app`.

## Repository boundary contract

- Repository interfaces consumed by use cases should be owned by `internal/domain` or `internal/service` and scoped by bounded context (for example: auth/users, channels, recordings, payments, legal).
- Service method signatures should exchange domain-owned DTOs; avoid exposing adapter package DTOs (such as `internal/storage` param structs) in service contracts.
- `internal/storage` may provide temporary type aliases/adapters for migration compatibility, but new logic should target domain-owned interfaces first.

## Enforcement checklist for contributors

Before opening a PR that touches runtime behaviour:

- Confirm imports respect the dependency direction above.
- Confirm new business logic is in `internal/service`/`internal/domain`, not in handlers or entrypoints.
- Confirm integration-specific code is isolated in adapter packages.
- Add/update tests at the layer where behaviour is defined.
- If this contract changes, update this document and the root `AGENTS.md` in the same PR, and explain the rationale in the PR body.
