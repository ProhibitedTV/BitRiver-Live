# PLAN

## Scope (current change)
- Execute release verification gates requested by the user from repository root.
- Capture full command logs/artifacts for each automated gate.
- Produce a release checklist report with pass/fail outcomes and remediation notes.

## Assumptions
- Existing repo scripts are the source of truth for verification/release checks.
- `docs/production-release.md` defines additional release-specific gates beyond `./scripts/verify.sh`.
- Viewer checks are in scope when explicitly requested by the user.

## Risks
- Some release checks may require local services/dependencies (Docker daemon, Node modules, Postgres test container).
- Long-running suites could time out; logs must still be captured fully for diagnostics.
- Manual runbook items (legal/release-ticket evidence) cannot be fully automated in this environment.

## Test plan
- `./scripts/verify.sh`
- `./scripts/test-quickstart.sh`
- `npm --prefix web/viewer run lint`
- `npm --prefix web/viewer run test`
- `./scripts/test-postgres.sh`
- `./scripts/check-postgres-pgx.sh postgres`
