# PLAN

## Scope (current change)
- Execute the postgres release checks via a prepared integration database path.
- Capture command output in a new timestamped release evidence directory under `artifacts/`.
- Update `docs/releases/release-checklist-report-2026-02-27.md` with final status, evidence paths, and residual risks.

## Assumptions
- Docker may still be unavailable; if so, a locally prepared postgres instance can satisfy `BITRIVER_TEST_POSTGRES_DSN`.
- Running migrations with `BITRIVER_TEST_POSTGRES_RUN_MIGRATIONS=1` is acceptable for the prepared DB path.
- Scope is limited to release evidence/docs unless triage identifies a real failure in storage or migrations.

## Risks
- Local postgres bootstrap could fail due to missing binaries or permissions, delaying evidence capture.
- Migration/test failures may require touching `internal/storage`, `deploy/migrations`, or `cmd/tools/pgx-mode-check` under time pressure.

## Test plan
- Run `BITRIVER_TEST_POSTGRES_DSN=... BITRIVER_TEST_POSTGRES_RUN_MIGRATIONS=1 ./scripts/test-postgres.sh` and capture output.
- Run `./scripts/check-postgres-pgx.sh postgres` and capture output.
- If failures occur, patch relevant files and rerun both checks until both pass.
