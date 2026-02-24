# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add reusable bounded polling helper in `scripts/`
  - Acceptance criteria:
    - New helper script exposes a reusable function for bounded polling.
    - Helper supports configurable interval and timeout values.
    - Helper can distinguish success/retry/fail outcomes for callers.
  - Relevant checks:
    - ✅ `bash -n scripts/polling.sh`
    - ⚠️ `shellcheck scripts/polling.sh` (shellcheck not installed in environment)

- [x] Task 2 — Apply helper to quickstart service health waits
  - Acceptance criteria:
    - `scripts/test-quickstart.sh` uses the shared helper for service health polling.
    - Existing timeout behavior (`WAIT_TIMEOUT`, 5s polling) and error meaning are preserved.
  - Relevant checks:
    - ✅ `bash -n scripts/test-quickstart.sh`
    - ⚠️ `shellcheck scripts/test-quickstart.sh` (shellcheck not installed in environment)

- [x] Task 3 — Apply helper to postgres container readiness waits
  - Acceptance criteria:
    - `scripts/test-postgres.sh` uses the shared helper for container health polling.
    - Existing failure exits/messages remain equivalent.
  - Relevant checks:
    - ✅ `bash -n scripts/test-postgres.sh`
    - ⚠️ `shellcheck scripts/test-postgres.sh` (shellcheck not installed in environment)

- [x] Task 4 — Validate script flows
  - Acceptance criteria:
    - Run existing script test flow commands and record outcomes.
  - Relevant checks:
    - ⚠️ `./scripts/test-postgres.sh ./internal/storage/...` (docker unavailable and no `BITRIVER_TEST_POSTGRES_DSN`)
    - ⚠️ `./scripts/test-quickstart.sh` (docker unavailable)

## Execution log
- ✅ `bash -n scripts/polling.sh`
- ✅ `bash -n scripts/test-quickstart.sh`
- ✅ `bash -n scripts/test-postgres.sh`
- ⚠️ `shellcheck scripts/polling.sh` (`shellcheck` command not found)
- ⚠️ `shellcheck scripts/test-quickstart.sh` (`shellcheck` command not found)
- ⚠️ `shellcheck scripts/test-postgres.sh` (`shellcheck` command not found)
- ⚠️ `./scripts/test-postgres.sh ./internal/storage/...` (fails fast by design without docker/DSN in this environment)
- ⚠️ `./scripts/test-quickstart.sh` (fails fast by design without docker in this environment)
