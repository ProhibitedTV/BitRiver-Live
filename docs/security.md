# Security guardrails

This page captures repository-level security guardrails that are continuously enforced in local verification and CI.

## Placeholder safety for environment templates

`deploy/.env.example` must only contain clearly fake sample values for credentials/tokens.

Enforced by:

- `./scripts/check-env-example-placeholders.sh`
- `./scripts/verify.sh` (which includes the placeholder check)

Rules:

- Required credential keys are sourced from `x-required-credentials` in `deploy/docker-compose.yml` and must be non-empty in `deploy/.env.example`.
- Secret-bearing placeholders must include explicit sample markers (for example `-example`, `_example`, `Example`, `sample`, `placeholder`, or `changeme`).
- Email placeholders must use `example.com`.
- Long random/high-entropy token-like values are treated as unsafe and rejected unless clearly marked as examples.

## Related security docs

- Secrets hardening workflow and operator checklist: [`docs/secrets-hardening.md`](secrets-hardening.md)
- Production release guidance: [`docs/production-release.md`](production-release.md)
- Deployment contract and required env fields: [`docs/contract.md`](contract.md)
