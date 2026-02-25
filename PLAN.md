# PLAN

## Scope (current change)
- Create a root `.env` from `deploy/.env.example` for production-oriented local validation.
- Fill required secrets/tokens and public URL/domain values so env validation passes.
- Run env validation and third-party digest enforcement, then fix any failures.
- Keep `.env` untracked in git.

## Assumptions
- Placeholder/example secrets are rejected by `deploy/check-env.sh` / `cmd/bitriver env validate`.
- Synthetic (non-real) but strong unique values are acceptable for this local setup as long as required fields are populated.
- Digest enforcement accepts any value in `@sha256:<64 hex>` format and does not verify registry existence.

## Risks
- Missing a required env key may cause repeated validation failures.
- Incorrect digest formatting will fail `scripts/require-image-digests.sh`.
- Accidentally staging `.env` would violate repository secret handling policy.

## Test plan
- `deploy/check-env.sh .env`
- `./scripts/require-image-digests.sh --env-file .env`
- `git status --short` (confirm `.env` is not tracked)
