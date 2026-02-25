# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Create root `.env` from deployment template
  - Acceptance criteria:
    - Root `.env` exists and is based on `deploy/.env.example`.
    - `.env` remains ignored/untracked by git.
  - Relevant checks:
    - `test -f .env`
    - `git status --short`

- [x] Task 2 — Populate required production secrets/URLs/tokens
  - Acceptance criteria:
    - Required admin credentials, DB/Redis/SRS/OME/transcoder tokens, and public URL values are non-placeholder values.
  - Relevant checks:
    - `deploy/check-env.sh .env`

- [x] Task 3 — Pin required third-party image digests
  - Acceptance criteria:
    - All required third-party digest env vars are set and format-valid.
  - Relevant checks:
    - `./scripts/require-image-digests.sh --env-file .env`

## Execution log
- ✅ `cp deploy/.env.example .env && test -f .env && git status --short`
- ❌ `deploy/check-env.sh .env` (initial failures: placeholder secrets/URLs and missing production digest pins)
- ✅ `deploy/check-env.sh .env`
- ✅ `./scripts/require-image-digests.sh --env-file .env`
- ✅ `git status --short` (confirmed `.env` is still untracked/ignored)
