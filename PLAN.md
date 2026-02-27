# PLAN

## Scope (current change)
- Add a static placeholder-hygiene lint script for `deploy/.env.example` that prevents empty required credential fields and rejects secret-like production values.
- Reuse the existing `x-required-credentials` source in `deploy/docker-compose.yml` (same parsing contract used by `scripts/test-quickstart-env.py`) to determine which keys must be validated.
- Wire the new lint into `scripts/verify.sh` so local + CI verification gates enforce placeholder safety continuously.
- Document placeholder conventions and examples in `docs/secrets-hardening.md` and a new `docs/security.md` entry.

## Assumptions
- `x-required-credentials` in `deploy/docker-compose.yml` remains the source of truth for required credential keys.
- `scripts/verify.sh` is the canonical verification entrypoint used by CI workflows (`./scripts/test-all.sh` and `.github/workflows/ci.yml`).
- `deploy/.env.example` intentionally contains fake sample values and should never include production-derived secrets.

## Risks
- Overly strict pattern checks could reject legitimate placeholders and create noisy false positives.
- Missing clear sample marker rules could confuse contributors updating `deploy/.env.example`.
- New documentation path (`docs/security.md`) could drift if not linked clearly to existing security guidance.

## Test plan
- `./scripts/check-env-example-placeholders.sh`
- `./scripts/verify.sh`
