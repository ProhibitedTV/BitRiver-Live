# Release Checklist Report — 2026-02-25

## Scope
Executed the requested automated release gates from repository root and captured full logs under:

- `artifacts/release-checks-20260225-013109/`

## Gate results

| Gate | Command | Result | Log artifact | Remediation / notes |
|---|---|---|---|---|
| Baseline verify | `./scripts/verify.sh` | PASS (with environment skips) | `01-verify.log` | Script passed; docker-dependent checks were skipped because Docker is unavailable in this environment. |
| Quickstart smoke | `./scripts/test-quickstart.sh` | WARNING (not executed fully) | `02-test-quickstart.log` | Install/enable Docker, then rerun smoke gate. |
| Viewer lint | `npm --prefix web/viewer run lint` | PASS | `03-viewer-lint.log` | No ESLint issues. |
| Viewer test | `npm --prefix web/viewer run test` | FAIL | `04-viewer-test.log` | Fix snapshot mismatch in `__tests__/channelDisplayPrimitives.test.tsx` (badge/action text changed), then rerun. |
| Postgres suite | `./scripts/test-postgres.sh` | WARNING (not executed fully) | `05-test-postgres.log` | Provide Docker or set `BITRIVER_TEST_POSTGRES_DSN` to a prepared migrated database, then rerun. |
| pgx guard | `./scripts/check-postgres-pgx.sh postgres` | PASS | `06-check-postgres-pgx.log` | Script exited 0 and printed pgx mode check output. |

## Release-specific runbook checks from `docs/production-release.md`

### Automated checks run here
- Go/unit and contract checks (via `./scripts/verify.sh`) — pass with Docker-dependent skips.
- Viewer lint/test — lint passed, tests failed on snapshot mismatch.
- Postgres suite — blocked by missing Docker/DSN.
- pgx guard — pass.

### Manual/operational checks pending (not automatable in this session)
- Legal publication checklist and DMCA dry-run evidence.
- Backup/restore freshness and rehearsal evidence attachment.
- Tagging/publishing workflow execution and monitoring.
- Environment-specific secrets rotation and production host rollout steps.

## Summary
Release gate status is **NOT READY** due to one failing gate (viewer tests) and environment-blocked gates (quickstart smoke and Postgres suite).
