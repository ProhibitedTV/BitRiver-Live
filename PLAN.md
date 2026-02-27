# PLAN

## Scope (production release preparation)
- Prepare release execution artifacts for a production-release pass focused on five gates:
  1. Environment verification.
  2. Digest enforcement for release manifests/workflows.
  3. Release workflow parity (docs/scripts/workflows aligned).
  4. Runbook parity across operator docs.
  5. Final release checks and handoff evidence.
- Keep this pass planning-only (no release execution in this change).

## Assumptions
- Release owners will execute commands from repo root unless a task states otherwise.
- Required secrets/tokens (registry creds, signing keys, deployment secrets, GitHub release permissions) are provided out-of-band and are not committed to this repo.
- Docker engine/Compose and GitHub Actions tooling availability may differ by environment; static checks can run locally even when runtime infra is unavailable.
- `./scripts/verify.sh` remains the default merge/release gate, but may be deferred until runtime dependencies are present.

## Risks
- Missing or invalid secrets can block image publication, signing, or deployment despite passing static repo checks.
- Docker-unavailable environments can prevent compose render/smoke checks and produce false confidence if not explicitly tracked.
- GitHub Actions permission or runner differences can cause release workflow drift that is not detectable by local-only checks.
- Doc/runbook drift from scripts/workflows can lead to incomplete release execution steps.

## Test/check strategy
- Use explicit per-task commands in `TASKS.md` so each gate has verifiable evidence.
- Prefer static/parity checks first (CI/workflow scans + docs parity checks) when full runtime verify is deferred.
- Reserve full runtime validation (`./scripts/verify.sh`) for the final gate once Docker and required credentials are available.
