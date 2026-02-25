# PLAN

## Scope (current change)
- Update `scripts/verify.sh` Docker Compose validation to pass an explicit env file.
- Add `scripts/deploy-smoke.sh` for operator-focused compose boot + `/readyz` smoke validation with guaranteed teardown.
- Keep deployment contract/runtime behavior unchanged (validation-only workflow improvements).

## Assumptions
- Root `.env` is the preferred source for local validation when present.
- `deploy/.env.example` contains enough defaults for compose config rendering when root `.env` is absent.
- API readiness endpoint is reachable at `http://localhost:${BITRIVER_LIVE_PORT:-8080}/readyz` once stack is ready.

## Risks
- Environments without Docker/Compose cannot run smoke validation (script must fail with clear prerequisite messaging).
- Using `deploy/.env.example` fallback must not mask missing user-specific required vars in root `.env` scenarios.

## Test plan
- `bash -n scripts/verify.sh scripts/deploy-smoke.sh`
- `./scripts/verify.sh --go-packages ./cmd/bitriver` (or equivalent targeted run) to confirm compose validation wiring.
- `./scripts/deploy-smoke.sh` on Docker-enabled host to validate boot/readiness/teardown flow.
