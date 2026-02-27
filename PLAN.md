# PLAN

## Scope (current change)
- Update `.github/workflows/release.yml` `verify-env` job so the generated production `.env` includes runtime mode/image-source production toggles, required third-party image digest variables, and production security guardrail variables.
- Preserve fast-fail secret checks while ensuring `./scripts/require-image-digests.sh --env-file .env` runs under production conditions.
- Sync `docs/production-release.md` secret requirements with the workflow variable contract.

## Assumptions
- Release tags must always enforce production settings (`BITRIVER_LIVE_MODE=production`, `BITRIVER_DEPLOY_IMAGE_SOURCE=pull`) even if repository defaults differ.
- `deploy/check-env.sh` validation accepts either `BITRIVER_LIVE_METRICS_TOKEN` or `BITRIVER_LIVE_METRICS_ALLOW_NETWORKS` for metrics protection; workflow should include one required path to fail fast.
- Third-party digest values are provided via GitHub secrets and must match `@sha256:<64 lowercase hex>` for production validation.

## Risks
- Adding new required env inputs can break existing release workflows until repository/org secrets are populated.
- Mismatch between docs and workflow can cause operator confusion and failed releases.
- Incorrect env wiring (missing in `env:` or `vars=(...)`) could silently omit values from `.env` and weaken preflight checks.

## Test plan
- Run a targeted syntax/contract check on `.github/workflows/release.yml` by inspecting the updated `Create production env file` block.
- Run `./scripts/require-image-digests.sh --env-file <fixture>` once in production mode with valid digests and once with an invalid/missing digest to confirm enforcement behavior.
- Verify `docs/production-release.md` secret list and release guidance reflect every newly required workflow variable.
