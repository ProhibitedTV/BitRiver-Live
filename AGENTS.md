# BitRiver Live – Agent Guide

## Start here
BitRiver Live is a self-hosted live-streaming stack that runs a Go control-plane API, a Next.js viewer, ingest/transcoding services, and stateful data services together. The canonical deployment shape in this repo is Docker Compose driven from `deploy/docker-compose.yml` and a root `.env`.

## Canonical contract
Treat these files as the deployment contract:
- `deploy/docker-compose.yml`
- `./.env` (repo root)
- `deploy/ome/Server.generated.xml` (generated file currently present; keep in sync when OME/env settings change)

If a change affects runtime behavior, confirm the contract still renders and boots.

## Golden path (single happy path)
From repo root:
```bash
./scripts/quickstart.sh
```

Equivalent source command:
```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml
```

## Required checks before merging
Run from repo root unless noted.

Default local gate (recommended):
```bash
./scripts/verify.sh
```

Equivalent manual sequence:

1. Go tests:
```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s
```

2. Viewer checks:
```bash
cd web/viewer && npm run lint
cd web/viewer && npm run test
```
- Local default: run when viewer changes are in scope (or force with `./scripts/verify.sh --viewer`).
- CI default (`CI=1` or `CI=true`): run only when `web/viewer` exists, `node` + `npm` are available, and the workflow is viewer-related; force with `./scripts/verify.sh --ci-viewer` for non-viewer workflows.

3. Docker Compose config validation:
```bash
docker compose -f deploy/docker-compose.yml config
```

4. Smoke test:
```bash
./scripts/test-quickstart.sh
```

## Repo zones
### Product
- Runtime code and user-facing behavior: `cmd/`, `internal/`, `web/viewer/`, `deploy/`.
- Source-of-truth docs for shipped behavior: `README.md`, `docs/quickstart.md`, `docs/contract.md`, `docs/architecture.md`, `docs/code-placement.md`.

### Ops / runbooks
- Operational and release guidance: `docs/operations.md`, `docs/advanced-deployments.md`, `docs/production-release.md`, `docs/testing.md`.
- Operational scripts: `scripts/` (for deploy/test/restore/backup helpers).

### Labs / planning (non-binding)
- Experimental/planning docs only: `docs/labs/`.
- Do not treat Labs docs as release contract.

## How to do changes safely
- Prefer small diffs that isolate one behavior change at a time.
- Update docs whenever behavior, commands, or operator steps change.
- Never change the deployment contract (`deploy/docker-compose.yml`, root `.env`, generated OME config expectations) without updating `docs/contract.md` in the same PR.
- For env/compose/generated-config contract edits, follow `docs/contract-change-recipe.md`.
- When uncertain, write: `TODO: verify in code`.

## Notes for agents
- Check for nested `AGENTS.md` files before editing subdirectories; deeper scope wins.
- Do not invent commands, flags, or files; only use what exists in this repository.

## Canonical policy
- This root `AGENTS.md` is the single canonical source of truth for agent instructions.
- Nested `AGENTS.md` files only point back here and must not add conflicting policy.

## High-signal local reminders (migrated)
- Keep CLI flag/env parity in `cmd/*` entrypoints and preserve graceful shutdown behavior.
- Keep API contracts stable (`internal/api`, `cmd/transcoder`, `cmd/srs-controller`) and update docs when payloads/routes change.
- For schema/storage changes, update `deploy/migrations`, `internal/storage`, and relevant `cmd/tools` import/verification logic together.
- Reuse shared test helpers in `internal/testsupport`; avoid duplicating mocks.
- Preserve middleware ordering and dependency injection patterns in `internal/server` and `internal/api`.
- Keep scripts CI-safe and rerunnable; document workflow-impacting script changes.
- Keep web/static embed flows and viewer (`web/viewer`) API clients/tests in sync with backend changes.
