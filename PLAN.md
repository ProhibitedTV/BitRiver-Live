# PLAN

## Scope (current change)
- Add a deterministic shell guard at `scripts/check-no-committed-secrets.sh` that fails CI when tracked files include high-risk secret artifacts (root `.env`, private key/certificate bundles, and common local secret dump patterns).
- Wire the guard into `.github/workflows/ci.yml` so it runs on every push and pull request.
- Document the guard in `docs/security.md` with a brief checklist note about what file classes it blocks.

## Assumptions
- The repository intentionally tracks `deploy/.env.example`, which must remain allowed.
- The guard should only inspect tracked files (`git ls-files`) and avoid scanning untracked workspace state.
- CI runners provide POSIX shell + git only; no extra dependencies should be required.

## Risks
- Over-broad filename matching could block legitimate fixtures/examples if exemptions are not explicit.
- Under-inclusive matching could miss a sensitive filename variant.
- CI workflow edits must stay compatible with existing CI contract checks.

## Test plan
- `bash -n scripts/check-no-committed-secrets.sh`
- `./scripts/check-no-committed-secrets.sh`
- `./scripts/check-ci-contract.sh`
