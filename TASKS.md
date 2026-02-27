# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Prepare execution artifacts and DSN-backed Postgres path
  - Acceptance criteria:
    - Artifact directory for this run is created and selected.
    - Chosen execution path is documented (`BITRIVER_TEST_POSTGRES_DSN` with local transient Postgres).

- [x] Task 2 — Run Postgres storage suite with migrations and capture logs
  - Acceptance criteria:
    - `./scripts/test-postgres.sh` executed with DSN path.
    - Run output captured into the artifact directory.

- [x] Task 3 — Confirm release-required migrations are present/applied
  - Acceptance criteria:
    - Verified migration files referenced in `docs/production-release.md` exist in `deploy/migrations`.
    - Verified database objects introduced by those migrations are present after migration run.

- [x] Task 4 — Run pgx guard and publish final reporting updates
  - Acceptance criteria:
    - `./scripts/check-postgres-pgx.sh postgres` executed and logged.
    - `TASKS.md` and `docs/releases/release-checklist-report-2026-02-25.md` updated with final outcomes.

## Execution log
- ✅ Task 1 complete: artifact directory `artifacts/release-checks-20260227-154611/` created; selected DSN execution path using local transient Postgres (`BITRIVER_TEST_POSTGRES_DSN`) because Docker CLI is unavailable.
- ⚠️ Task 2 complete with failing result: `./scripts/test-postgres.sh` executed via DSN path with `BITRIVER_TEST_POSTGRES_RUN_MIGRATIONS=1`; migrations applied, but Go postgres-tagged storage tests failed with `postgres repository unavailable: pgx driver stubbed in this build`. Full output: `artifacts/release-checks-20260227-154611/07-test-postgres-dsn.log`.
- ✅ Task 3 complete: verified release-required migration files (`0002_chat_filters.sql`, `0006_profile_social_links.sql`, `0007_auth_mfa.sql`) exist and corresponding schema objects (`chat_filters`, `chat_automod_actions`, `auth_mfa`, `auth_mfa_challenges`, `profiles.social_links`) are present in the migrated validation DB. Output: `artifacts/release-checks-20260227-154611/08-migration-verification.log`.
- ✅ Task 4 complete: `./scripts/check-postgres-pgx.sh postgres` passed (`expected_storage_driver=postgres`). Output: `artifacts/release-checks-20260227-154611/09-check-postgres-pgx.log`.
