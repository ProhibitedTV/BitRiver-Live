# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Prepare release evidence directory and postgres execution path
  - Acceptance criteria:
    - A new `artifacts/release-checks-<timestamp>/` directory exists for this run.
    - Selected execution path is documented in the task log (prepared integration DB via DSN).

- [x] Task 2 — Run postgres release checks and capture logs
  - Acceptance criteria:
    - `BITRIVER_TEST_POSTGRES_DSN=... BITRIVER_TEST_POSTGRES_RUN_MIGRATIONS=1 ./scripts/test-postgres.sh` passes and log is captured.
    - `./scripts/check-postgres-pgx.sh postgres` passes and log is captured.

- [x] Task 3 — Update release checklist report with status and residual risks
  - Acceptance criteria:
    - `docs/releases/release-checklist-report-2026-02-27.md` references new evidence logs.
    - Report reflects final status and residual risks from this execution.

## Execution log
- ✅ Task 1 complete: created `artifacts/release-checks-20260227-163026` and selected prepared integration DB path via `BITRIVER_TEST_POSTGRES_DSN`.

- ✅ Task 2 complete: after triage in `internal/storage/postgres_acquire_timeout_integration_test.go`, reran postgres checks and captured logs in `artifacts/release-checks-20260227-163026/`.
- ✅ Task 3 complete: updated `docs/releases/release-checklist-report-2026-02-27.md` with final status and residual risks from the new evidence set.
- ✅ Post-format verification: reran postgres checks and captured final passing logs (`05-test-postgres-post-gofmt.log`, `06-check-postgres-pgx-post-gofmt.log`).
- ✅ Additional gate: `./scripts/verify.sh` passed (Docker-dependent checks skipped by script) and evidence captured at `07-verify.log`.
