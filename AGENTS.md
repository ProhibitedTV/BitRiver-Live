# BitRiver Live – Agent Guide

## Start here
BitRiver Live is a self-hosted live-streaming stack that runs a Go control-plane API, a Next.js viewer, ingest/transcoding services, and stateful data services together. The canonical deployment shape in this repo is Docker Compose driven from `deploy/docker-compose.yml` and a root `.env`.

## Spec-driven workflow (required)
For every scoped change, follow this sequence:
1. Start with **read-only analysis** and update `PLAN.md` first (scope, assumptions, risks, tests).
2. Only then implement tasks listed in `TASKS.md`, strictly top-to-bottom.
3. After each task, run the relevant test/check command(s) and update task status/results in `TASKS.md` before moving on.

Working artifacts:
- `SPEC.md`: user goals and success criteria.
- `PLAN.md`: current technical plan, risks, and test plan.
- `TASKS.md`: small reviewable tasks with acceptance criteria and status tracking.

Keep these docs concise and current so a new contributor can execute `SPEC → PLAN → TASKS → Implement` without chat history.

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
- In `./scripts/verify.sh`, Docker-dependent checks run in deterministic order: Compose config validation first, then quickstart smoke.
- When Docker is unavailable, both Docker-dependent steps are skipped with explicit messages.

## Repo zones
### Product
- Runtime code and user-facing behavior: `cmd/`, `internal/`, `web/viewer/`, `deploy/`.
- Source-of-truth docs for shipped behavior: `README.md`, `docs/quickstart.md`, `docs/contract.md`, `docs/architecture.md`, `docs/stream-lifecycle.md`, `docs/code-placement.md`.

### Ops / runbooks
- Operational and release guidance: `docs/operations.md`, `docs/advanced-deployments.md`, `docs/production-release.md`, `docs/testing.md`.
- Operational scripts: `scripts/` (for deploy/test/restore/backup helpers).

### Labs / planning (non-binding)
- Experimental/planning docs only: `docs/labs/`.
- Do not treat Labs docs as release contract.

## Code style
- Keep handlers/services context-aware and explicit about dependencies.
- Return wrapped errors (`%w`) with actionable context.
- Keep logging structured and close to failure/success boundaries.
- Preserve DI boundaries: wire concrete dependencies in `cmd/*`; pass interfaces into `internal/*` services.

Go (preferred pattern):
```go
func (s *StreamService) Start(ctx context.Context, id string) error {
	if err := s.repo.MarkStarting(ctx, id); err != nil {
		s.log.Error("mark starting failed", "stream_id", id, "err", err)
		return fmt.Errorf("mark stream starting: %w", err)
	}
	return nil
}
```

- Keep React components small, typed, and testable.
- Isolate fetch/API logic in client helpers/hooks, not JSX trees.
- Add stable `data-testid` attributes for critical interactive paths.

TS/React (preferred pattern):
```tsx
export function StreamStatus({ streamId }: { streamId: string }) {
  const { data, isLoading } = useStream(streamId)
  if (isLoading) return <p data-testid="stream-status-loading">Loading…</p>
  return <p data-testid="stream-status-value">{data?.status ?? "unknown"}</p>
}
```

## Git workflow
- Branch naming: `feat/<scope>-<topic>`, `fix/<scope>-<topic>`, `chore/<scope>-<topic>`.
- Commit messages: short imperative with scope, e.g. `api: validate stream payload`.
- PRs: keep diffs small; run required checks (`./scripts/verify.sh`); update docs when behavior/contracts/workflows change.
- Merge strategy: **squash merge** to keep history focused per change.

## Boundaries
### ✅ Always do
- Run `./scripts/verify.sh` before opening/merging a PR.
- Keep `deploy/docker-compose.yml`, root `.env`, and `deploy/ome/Server.generated.xml` aligned with `docs/contract.md`.
- Update docs in `docs/` when user-visible behavior or operator workflow changes.

### ⚠️ Ask first
- Any deployment contract change (compose, root `.env`, generated OME expectations).
- CI/workflow changes (including `scripts/verify.sh` behavior or required checks).
- Cross-cutting refactors touching multiple product zones in one PR.

### 🚫 Never do
- Commit secrets, credentials, or private keys (including real `.env` values).
- Invent commands/flags/files not present in this repository.
- Bypass required checks or merge with failing validation.

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
## Required end-of-run self-check (must include in final response)
Before finishing, add this short checklist-style audit:
- [ ] Did I run the right commands? (List commands; default: `./scripts/verify.sh`)
- [ ] Did I update docs if contract/runtime behavior changed?
- [ ] Did I add/adjust tests?
- [ ] Any boundaries violated?
- [ ] What remains incomplete?
