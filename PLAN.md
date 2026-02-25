# PLAN

## Scope (current change)
- Audit `deploy/.env.example` for placeholder/sample defaults (domains, emails, passwords, tokens).
- Create/update deployment `.env` with non-placeholder production-style values and routable public domains.
- Ensure production mode and login throttling values are explicitly set to safe non-zero defaults.
- Align viewer URL variables with the chosen public deployment endpoints.

## Assumptions
- The deployment-specific runtime env for this repo is the root `.env` copied from `deploy/.env.example`.
- Synthetic strong values are acceptable for this task as long as they are not sample placeholders.
- Validation should be done with `deploy/check-env.sh .env`.

## Risks
- Placeholder-like strings can remain if only partially replaced.
- URL/domain mismatches can break viewer/API routing.
- Root `.env` is intentionally untracked; verification must not rely on git diff for env content.

## Test plan
- `test -f .env`
- `deploy/check-env.sh .env`
- `rg -n "(example\\.com|admin@|Example|secure-token-example|Sup3rSecureAdmin)" .env`
- `git status --short`
