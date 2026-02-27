# PLAN

## Scope (current change)
- Execute the Postgres validation path using a provided DSN (`BITRIVER_TEST_POSTGRES_DSN`) instead of Docker, because Docker is unavailable in this environment.
- Ensure release-required migrations from `docs/production-release.md` are present in `deploy/migrations` and applied on the validation database used by `./scripts/test-postgres.sh`.
- Re-run `./scripts/check-postgres-pgx.sh postgres` and keep artifacts together with Postgres test logs.
- Update `TASKS.md` and `docs/releases/release-checklist-report-2026-02-25.md` with final outcomes and artifact references.

## Assumptions
- Package installation is available so we can provision local Postgres binaries (`psql`, `postgres`, `initdb`, `pg_ctl`).
- A local transient Postgres data directory/instance is acceptable for release-check execution and evidence capture.

## Risks
- Host package installation or Postgres startup could fail due to environment limitations.
- Migrations may fail if run more than once against a non-empty schema; we will target a fresh temporary database.
- Test logs can be incomplete unless stdout/stderr are fully captured with `tee`.

## Test plan
- `./scripts/test-postgres.sh` with `BITRIVER_TEST_POSTGRES_DSN` set and `BITRIVER_TEST_POSTGRES_RUN_MIGRATIONS=1`.
- `./scripts/check-postgres-pgx.sh postgres`.
- Migration verification query against `information_schema.columns` and `information_schema.tables` for release-required objects.
