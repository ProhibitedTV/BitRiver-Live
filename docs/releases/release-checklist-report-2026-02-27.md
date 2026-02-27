# Release Checklist Report — 2026-02-27

## Scope
Consolidated current production release readiness from existing repository artifacts and prior gate logs.

Evidence sources:
- `artifacts/release-checks-20260227-154237/`
- `artifacts/release-checks-20260227-154611/`
- `docs/releases/release-checklist-report-2026-02-25.md`

## Gate summary (current)

| Gate | Command | Latest result | Evidence |
|---|---|---|---|
| Baseline verify | `./scripts/verify.sh` | PASS (with Docker-dependent skips) | `docs/releases/release-checklist-report-2026-02-25.md` |
| Quickstart smoke | `./scripts/test-quickstart.sh` | FAIL (Docker unavailable) | `artifacts/release-checks-20260227-154237/01-test-quickstart.log` |
| Viewer lint | `npm --prefix web/viewer run lint` | PASS | `docs/releases/release-checklist-report-2026-02-25.md` |
| Viewer tests | `npm --prefix web/viewer run test` | FAIL (snapshot mismatch) | `docs/releases/release-checklist-report-2026-02-25.md` |
| Postgres suite (DSN path) | `BITRIVER_TEST_POSTGRES_DSN=... BITRIVER_TEST_POSTGRES_RUN_MIGRATIONS=1 ./scripts/test-postgres.sh` | FAIL (`pgx driver stubbed in this build`) | `artifacts/release-checks-20260227-154611/07-test-postgres-dsn.log` |
| Postgres migration verification | `psql` schema checks | PASS | `artifacts/release-checks-20260227-154611/08-migration-verification.log` |
| pgx guard | `./scripts/check-postgres-pgx.sh postgres` | PASS | `artifacts/release-checks-20260227-154611/09-check-postgres-pgx.log` |

## Release decision

Production release is **NO-GO / NOT READY**.

Blocking items:
1. Quickstart smoke gate cannot run in this environment because Docker CLI/daemon are unavailable.
2. Viewer tests fail due to snapshot mismatch.
3. Postgres storage suite fails under a pgx-stubbed build path.

## Unblock plan before tagging `vX.Y.Z`

1. Run release gates on a Docker-capable runner:
   - `./scripts/test-quickstart.sh`
   - `docker compose -f deploy/docker-compose.yml config`
2. Fix viewer snapshot/test expectations, then rerun:
   - `npm --prefix web/viewer run lint`
   - `npm --prefix web/viewer run test`
3. Ensure release build links the real pgx module (not stub), then rerun:
   - `./scripts/test-postgres.sh`
   - `./scripts/check-postgres-pgx.sh postgres`
4. Re-run full recommended gate:
   - `./scripts/verify.sh`
5. After all gates pass, follow `docs/production-release.md` section "2. Tag the release and trigger the workflow" to create and push annotated tag.

## Contract impact
This update is documentation-only and does not change deployment contract files:
- `deploy/docker-compose.yml`
- `./.env`
- `deploy/ome/Server.generated.xml`
