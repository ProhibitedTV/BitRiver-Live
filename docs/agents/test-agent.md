# Test Specialist Agent Guide

This guide is a focused persona for validation, regression checks, and test coverage alignment.

> Root policy is canonical: read `AGENTS.md` at repo root first and treat this file as a non-conflicting specialization.

## Scope
Use this specialist when tasks involve test execution, test updates, or verification workflow reliability.

Primary touch zones:
- Go tests across `cmd/` and `internal/`
- Viewer checks in `web/viewer/` (`npm run lint`, `npm run test`)
- Verification and smoke workflows in `scripts/verify.sh` and `scripts/test-quickstart.sh`
- Test-related docs (`docs/testing.md` and workflow notes)

## Commands to run
Run from repo root unless noted.

1. Default validation gate:
```bash
./scripts/verify.sh
```

2. Manual Go test pass (when isolating Go failures):
```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s
```

3. Viewer checks (when viewer changes are in scope):
```bash
cd web/viewer && npm run lint
cd web/viewer && npm run test
```

4. Compose contract rendering:
```bash
docker compose -f deploy/docker-compose.yml config
```

5. Quickstart smoke test:
```bash
./scripts/test-quickstart.sh
```

## What good looks like
- [ ] `./scripts/verify.sh` passes (or failures are triaged with clear root cause and scope).
- [ ] New/changed behavior has corresponding automated test coverage where practical.
- [ ] Existing tests are updated with minimal blast radius and stable assertions.
- [ ] Viewer checks are run when viewer code changes.
- [ ] Smoke and compose validation are run for changes that can affect startup/runtime wiring.
- [ ] Test changes preserve readability, determinism, and reuse of shared helpers (for example `internal/testsupport`).

## Boundaries
Inherit root `AGENTS.md` **Always / Ask first / Never** rules without exception.

Additional non-conflicting clarifications:
- Do not weaken required checks to make failures disappear; fix causes instead.
- Prefer small, behavior-focused test updates over broad rewrites.
- Keep CI-safe, rerunnable behavior in scripts and test workflows.
