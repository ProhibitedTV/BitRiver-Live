# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Wire verify compose config to explicit env-file selection
  - Acceptance criteria:
    - `scripts/verify.sh` selects env file in this order: root `.env` then `deploy/.env.example`.
    - Compose validation command is `docker compose --env-file <selected> -f deploy/docker-compose.yml config`.
    - Clear error emitted if neither env file exists.
  - Relevant checks:
    - `bash -n scripts/verify.sh`

- [x] Task 2 — Add deploy smoke helper script
  - Acceptance criteria:
    - New `scripts/deploy-smoke.sh` starts compose stack using a temporary project name.
    - Script waits for API `/readyz` and prints concise PASS/FAIL summary.
    - Script always tears down compose resources on exit.
  - Relevant checks:
    - `bash -n scripts/deploy-smoke.sh`
    - `./scripts/deploy-smoke.sh`

- [x] Task 3 — Update operator/testing docs for new validation path
  - Acceptance criteria:
    - Docs describe verify env-file fallback behavior.
    - Docs include `scripts/deploy-smoke.sh` as one-command confidence check.
  - Relevant checks:
    - `rg -n "deploy-smoke|--env-file" docs/testing.md`

## Execution log
- ✅ `bash -n scripts/verify.sh scripts/deploy-smoke.sh`
- ❌ `./scripts/deploy-smoke.sh` (docker unavailable in this environment: `FAIL: docker is required`)
- ✅ `rg -n "deploy-smoke|--env-file|readyz|Docker Compose config validation" docs/testing.md scripts/verify.sh scripts/deploy-smoke.sh`
- ❌ `./scripts/verify.sh` (fails on pre-existing viewer snapshot mismatch in `web/viewer/__tests__/channelDisplayPrimitives.test.tsx`)
