# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Expand release verify-env `.env` emission inputs
  - Acceptance criteria:
    - `.github/workflows/release.yml` `Create production env file` `env:` map includes:
      - `BITRIVER_LIVE_MODE` (`production`)
      - `BITRIVER_DEPLOY_IMAGE_SOURCE` (`pull`)
      - Third-party digest variables required by `scripts/require-image-digests.sh`
      - Production security guardrail vars: `BITRIVER_LIVE_RATE_LOGIN_LIMIT`, `BITRIVER_LIVE_RATE_LOGIN_WINDOW`, and one metrics protection input used by env validation.
    - The same variables are present in the `vars=(...)` list so missing/empty values fail fast.

- [x] Task 2 — Keep digest enforcement active in release verify-env
  - Acceptance criteria:
    - `./scripts/require-image-digests.sh --env-file .env` remains in `verify-env` and runs with production conditions active from emitted `.env`.
    - A local script check demonstrates missing/invalid digest values fail under production settings.

- [x] Task 3 — Sync production release docs with workflow requirements
  - Acceptance criteria:
    - `docs/production-release.md` secret requirements match variable names/requirements enforced by `release.yml`.

## Execution log
- ✅ Task 1 complete: expanded `verify-env` `Create production env file` to emit production mode/image-source constants, digest vars, and production security vars in both `env:` and `vars=(...)`.
- ✅ Task 1 check: `rg -n "BITRIVER_LIVE_MODE|BITRIVER_DEPLOY_IMAGE_SOURCE|BITRIVER_LIVE_METRICS_TOKEN|BITRIVER_LIVE_RATE_LOGIN_LIMIT|BITRIVER_LIVE_RATE_LOGIN_WINDOW|BITRIVER_(REDIS|POSTGRES|SRS|OME|NGINX|ALPINE_3|ALPINE_3_19|DEBIAN)_IMAGE_DIGEST" .github/workflows/release.yml`.

- ✅ Task 2 complete: `verify-env` now emits `BITRIVER_LIVE_MODE=production` and `BITRIVER_DEPLOY_IMAGE_SOURCE=pull`, activating digest enforcement in the release job.
- ✅ Task 2 check: `./scripts/require-image-digests.sh --env-file /tmp/digests-pass.env` (passes with valid production digests).
- ✅ Task 2 check: `./scripts/require-image-digests.sh --env-file /tmp/digests-fail.env` (expected failure on missing/invalid digest values).
- ✅ Task 3 complete: updated release runbook secret requirements to match enforced workflow inputs and documented that production mode/image-source are job constants.
- ✅ Task 3 check: `rg -n "BITRIVER_LIVE_MODE=production|BITRIVER_DEPLOY_IMAGE_SOURCE=pull|BITRIVER_LIVE_METRICS_TOKEN|BITRIVER_LIVE_RATE_LOGIN_WINDOW" docs/production-release.md`.
