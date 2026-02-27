# PLAN

## Scope (current change)
- Use current release readiness evidence to address blockers that are actionable in-repo.
- Stabilize the flaky upload cleanup tests causing `./scripts/verify.sh` failures.
- Refresh `docs/releases/release-checklist-report-2026-02-27.md` with latest gate outcomes and an updated go/no-go summary.

## Assumptions
- Docker is unavailable in this environment, so Docker-gated checks remain blocked and must be documented.
- Viewer lint/tests are runnable locally and can be used as fresh release evidence.
- Upload processor behavior should remain unchanged; only flaky test orchestration should be adjusted.

## Risks
- Adjusting tests could mask a real duplicate-cleanup regression if assertions become too weak.
- Release report can drift quickly if subsequent runs are not captured with explicit evidence paths.

## Test plan
- Run `go test ./internal/service/uploads -count=1` to validate the flaky suite after test updates.
- Run `./scripts/verify.sh` to exercise the default release gate.
- Run viewer checks explicitly:
  - `npm --prefix web/viewer run lint`
  - `npm --prefix web/viewer run test`
- Re-run environment-limited release checks and record outcomes:
  - `./scripts/check-postgres-pgx.sh postgres`
  - `./scripts/test-postgres.sh`
  - `docker compose -f deploy/docker-compose.yml config`
  - `./scripts/test-quickstart.sh`
