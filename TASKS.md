# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Run baseline verification gate (`./scripts/verify.sh`)
  - Acceptance criteria:
    - Command executed from repo root.
    - Full log captured to an artifact file.

- [x] Task 2 — Run quickstart smoke gate (`./scripts/test-quickstart.sh`)
  - Acceptance criteria:
    - Command executed from repo root.
    - Full log captured to an artifact file.

- [x] Task 3 — Run viewer lint/test commands
  - Acceptance criteria:
    - `npm --prefix web/viewer run lint` executed.
    - `npm --prefix web/viewer run test` executed.
    - Full logs captured to artifact files.

- [x] Task 4 — Run production-release checks (automated subset)
  - Acceptance criteria:
    - `./scripts/test-postgres.sh` executed.
    - `./scripts/check-postgres-pgx.sh postgres` executed.
    - Full logs captured to artifact files.

- [x] Task 5 — Produce release checklist report
  - Acceptance criteria:
    - Gate-by-gate pass/fail status documented.
    - Remediation items included for any non-pass/manual gates.

## Execution log
- ✅ `./scripts/verify.sh 2>&1 | tee artifacts/release-checks-20260225-013109/01-verify.log` (pass; docker-dependent subchecks skipped by script due to missing Docker binary)

- ⚠️ `./scripts/test-quickstart.sh 2>&1 | tee artifacts/release-checks-20260225-013109/02-test-quickstart.log` (blocked: docker is required for smoke checks)
- ✅ `npm --prefix web/viewer run lint 2>&1 | tee artifacts/release-checks-20260225-013109/03-viewer-lint.log` (pass)
- ❌ `npm --prefix web/viewer run test 2>&1 | tee artifacts/release-checks-20260225-013109/04-viewer-test.log` (failed: snapshot mismatch in `__tests__/channelDisplayPrimitives.test.tsx`)
- ⚠️ `./scripts/test-postgres.sh 2>&1 | tee artifacts/release-checks-20260225-013109/05-test-postgres.log` (blocked: Docker or BITRIVER_TEST_POSTGRES_DSN required)
- ✅ `./scripts/check-postgres-pgx.sh postgres 2>&1 | tee artifacts/release-checks-20260225-013109/06-check-postgres-pgx.log` (pass; script exited 0)
- ✅ Authored release checklist report: `docs/releases/release-checklist-report-2026-02-25.md`
