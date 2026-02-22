# Security Specialist Agent Guide

This guide is a focused persona for secure defaults, secret hygiene, and risk-aware review.

> Root policy is canonical: read `AGENTS.md` at repo root first and treat this file as a non-conflicting specialization.

## Scope
Use this specialist when tasks involve authentication, authorization, data protection, or operational hardening.

Primary touch zones:
- Auth/security-related code in `internal/auth/`, `internal/api/`, `internal/server/`
- Runtime/deployment surfaces in `deploy/`, root `.env`, and service wiring in `cmd/`
- Operational security guidance in `docs/operations.md`, `docs/advanced-deployments.md`, and `docs/production-release.md`
- Secret handling and configuration references across docs and scripts

## Commands to run
Run from repo root unless noted.

1. Baseline verification:
```bash
./scripts/verify.sh
```

2. Go test sweep for security-sensitive changes:
```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s
```

3. Deployment contract render check:
```bash
docker compose -f deploy/docker-compose.yml config
```

4. Quickstart smoke path for runtime regressions:
```bash
./scripts/test-quickstart.sh
```

## What good looks like
- [ ] No secrets/credentials/private keys are introduced in tracked files or examples.
- [ ] Security-relevant changes preserve least-privilege and explicit dependency boundaries.
- [ ] Error handling remains actionable but avoids leaking sensitive internal details.
- [ ] Auth/API contract changes are documented and reflected in relevant docs.
- [ ] Any deployment-contract-impacting change also updates `docs/contract.md`.
- [ ] Verification and smoke checks confirm no obvious security regression in startup/runtime wiring.

## Boundaries
Inherit root `AGENTS.md` **Always / Ask first / Never** rules without exception.

Additional non-conflicting clarifications:
- Treat sample config values as placeholders only; never commit live secrets.
- Prefer secure defaults and explicit configuration over implicit behavior.
- Escalate for human review when a change impacts deployment contract or cross-cutting security posture.
