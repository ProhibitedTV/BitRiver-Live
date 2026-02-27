# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add committed-secrets guard script
  - Acceptance criteria:
    - `scripts/check-no-committed-secrets.sh` checks tracked files only and fails on root `.env`, private key/cert bundle filenames, and common local secret dump filenames.
    - Script allows intended tracked templates/examples (notably `deploy/.env.example`).
    - Script is deterministic and requires only shell + git.

- [x] Task 2 — Wire guard into CI
  - Acceptance criteria:
    - `.github/workflows/ci.yml` runs the new guard on every PR/push execution.
    - Workflow ordering/dependencies stay CI-contract compliant.

- [x] Task 3 — Security docs note
  - Acceptance criteria:
    - `docs/security.md` includes a brief checklist note that this guard exists and lists blocked artifact classes.

## Execution log
- ✅ Task 1 complete: added `scripts/check-no-committed-secrets.sh` with tracked-file checks, explicit exemptions, and deterministic shell+git behavior.
- ✅ Task 1 checks:
  - `bash -n scripts/check-no-committed-secrets.sh`
  - `./scripts/check-no-committed-secrets.sh`

- ✅ Task 2 complete: updated `.github/workflows/ci.yml` with a `secret-guard` job that runs on every CI invocation and executes `./scripts/check-no-committed-secrets.sh`.
- ✅ Task 2 check:
  - `./scripts/check-ci-contract.sh`

- ✅ Task 3 complete: added `docs/security.md` checklist note for committed-secret guard coverage and allowed template exception.
- ✅ Task 3 check:
  - `./scripts/check-no-committed-secrets.sh`

- ✅ Final required gate:
  - `./scripts/verify.sh`
