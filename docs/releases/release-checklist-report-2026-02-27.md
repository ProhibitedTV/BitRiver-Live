# Release Checklist Report — 2026-02-27

## Scope
Consolidated current production release readiness from fresh repository gate runs in this environment.

Evidence sources:
- `artifacts/release-checks-20260227-163011/`
- `docs/production-release.md`

## Gate summary (current)

| Gate | Command | Latest result | Evidence |
|---|---|---|---|
| Baseline verify | `./scripts/verify.sh` | PASS (with Docker-dependent skips; quickstart check still blocked) | `artifacts/release-checks-20260227-163011/03-verify.log` |
| Quickstart smoke | `./scripts/test-quickstart.sh` | FAIL (Docker unavailable) | `artifacts/release-checks-20260227-163011/02-test-quickstart.log` |
| Viewer lint | `npm --prefix web/viewer run lint` | PASS (covered in verify log) | `artifacts/release-checks-20260227-163011/03-verify.log` |
| Viewer tests | `npm --prefix web/viewer run test` | PASS (act warnings emitted; covered in verify log) | `artifacts/release-checks-20260227-163011/03-verify.log` |
| Postgres suite | `./scripts/test-postgres.sh` | FAIL (requires Docker or `BITRIVER_TEST_POSTGRES_DSN`; covered in verify log) | `artifacts/release-checks-20260227-163011/03-verify.log` |
| pgx guard | `./scripts/check-postgres-pgx.sh postgres` | PASS (`postgres` mode expected with stub indicator output; covered in verify log) | `artifacts/release-checks-20260227-163011/03-verify.log` |
| Docker Compose config validation | `docker compose -f deploy/docker-compose.yml config` | FAIL (`docker` command not installed) | `artifacts/release-checks-20260227-163011/01-docker-compose-config.log` |

## Release decision

Production release is **NO-GO / NOT READY**.

Remaining blockers:
1. Docker-required release gates cannot run in this environment (`docker compose config` and quickstart smoke).
2. Postgres integration suite cannot run without Docker or a prepared DSN.

## Unblock plan before tagging `vX.Y.Z`

1. Run release gates on a Docker-capable runner:
   - `docker compose -f deploy/docker-compose.yml config`
   - `./scripts/test-quickstart.sh`
2. Provide integration database access (either local Docker or prepared DSN) and rerun:
   - `BITRIVER_TEST_POSTGRES_DSN=... BITRIVER_TEST_POSTGRES_RUN_MIGRATIONS=1 ./scripts/test-postgres.sh`
   - `./scripts/check-postgres-pgx.sh postgres`
3. Re-run full gate:
   - `./scripts/verify.sh`
4. If all gates pass, follow `docs/production-release.md` section "2. Tag the release and trigger the workflow".

## Contract impact
This report-only update is a non-contract change. No deployment contract files were changed:
- `deploy/docker-compose.yml`
- `./.env`
- `deploy/ome/Server.generated.xml`
