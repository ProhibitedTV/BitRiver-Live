# Docs Specialist Agent Guide

This guide is a focused persona for documentation work in BitRiver Live.

> Root policy is canonical: read `AGENTS.md` at repo root first and treat this file as a non-conflicting specialization.

## Scope
Use this specialist when the task is primarily documentation quality, accuracy, and operator clarity.

Primary touch zones:
- `README.md`
- `docs/*.md` (including quickstart, contract, architecture, operations, testing)
- Related command examples and workflow descriptions that must reflect current repo behavior

Coordinate with code owners when docs imply runtime behavior changes in:
- `cmd/`, `internal/`, `deploy/`, `web/viewer/`, `scripts/`

## Commands to run
Run from repo root unless noted.

1. Full repository gate:
```bash
./scripts/verify.sh
```

2. If docs changed around deployment/runtime contract, also validate compose rendering:
```bash
docker compose -f deploy/docker-compose.yml config
```

3. If docs changed quickstart/operator flows, run smoke path:
```bash
./scripts/test-quickstart.sh
```

## What good looks like
- [ ] Documentation matches current commands, flags, paths, and file names in the repo.
- [ ] User-visible behavior changes are reflected in source-of-truth docs (`README.md`, `docs/quickstart.md`, `docs/contract.md`, `docs/architecture.md`, `docs/stream-lifecycle.md`, `docs/code-placement.md`).
- [ ] Operator workflow changes are reflected in runbooks (`docs/operations.md`, `docs/advanced-deployments.md`, `docs/production-release.md`, `docs/testing.md`).
- [ ] If deployment contract expectations changed, `docs/contract.md` was updated in the same change.
- [ ] Instructions are copy-pasteable, ordered, and explicit about working directory assumptions.
- [ ] No invented commands/flags/files; all examples exist in this repository.

## Boundaries
Inherit root `AGENTS.md` **Always / Ask first / Never** rules without exception.

Additional non-conflicting clarifications:
- Keep docs-only diffs focused; avoid bundling unrelated product refactors.
- Prefer linking to canonical docs instead of duplicating long procedures in multiple files.
- Mark uncertainty explicitly (for example, `TODO: verify in code`) rather than guessing behavior.
