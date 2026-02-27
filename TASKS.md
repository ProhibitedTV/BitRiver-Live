# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add optional Compose limits overlay + env knobs
  - Acceptance criteria:
    - `deploy/docker-compose.limits.yml` exists and defines CPU/memory limits per service using Compose-compatible keys.
    - `deploy/.env.example` includes resource knob defaults referenced by the overlay.

- [x] Task 2 — Add CLI and validation support for limits mode
  - Acceptance criteria:
    - `cmd/bitriver compose up` and `cmd/bitriver quickstart` support `--limits` and include the overlay automatically.
    - `bitriver env validate` sanity-checks resource knob formats/values.

- [x] Task 3 — Update docs for production guidance and contract updates
  - Acceptance criteria:
    - `docs/production-single-host.md` documents resource limits rationale, host size tiers, and tuning guidance.
    - `docs/production-release.md` documents production compose command with limits overlay.
    - `docs/contract.md` reflects the limits overlay and resource knob contract.

## Execution log
- ✅ Task 1 complete: added `deploy/docker-compose.limits.yml` with per-service `cpus`, `mem_limit`, and `mem_reservation`; added corresponding `BITRIVER_*_CPUS`, `BITRIVER_*_MEM`, and `BITRIVER_*_MEM_RESERVATION` defaults to `deploy/.env.example`.
- ✅ Task 1 check: `docker compose --env-file deploy/.env.example -f deploy/docker-compose.yml -f deploy/docker-compose.limits.yml config` (failed in this environment: `docker` command not available).

- ✅ Task 2 complete: added `--limits` support in `cmd/bitriver compose up` and `cmd/bitriver quickstart`; added env validation rules for `*_CPUS`, `*_MEM`, and `*_MEM_RESERVATION`; added/updated tests in `cmd/bitriver/main_test.go` and `cmd/bitriver/env_validation_test.go`.
- ✅ Task 2 check: `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./cmd/bitriver -count=1`.

- ✅ Task 3 complete: updated production and contract docs for limits overlay usage, sizing guidance, and env tuning path; regenerated contract generated section.
- ✅ Task 3 check: `./scripts/generate-contract-doc.sh`.
- ✅ Task 3 check: `./scripts/verify.sh`.
