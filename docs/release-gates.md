# Release gate ladder

BitRiver Live promotes changes through a small set of gates that protect the supported deployment boundary: an operator-managed, single-host Docker Compose stack using the repository-root `.env`, `deploy/docker-compose.yml`, and the shared source quickstart or packaged launcher paths.

These gates do not promise a Kubernetes-first platform, managed hosting, multi-host high availability, or SaaS operation. They answer one practical question: can a real operator still install, upgrade, verify, and roll back BitRiver Live through the documented single-host path?

## Gate summary

| Gate | Risk caught | When it runs | Status | Command or workflow | Evidence |
| --- | --- | --- | --- | --- | --- |
| 1. Static repo hygiene | Formatting drift, broken unit tests, stale generated contract checks, obvious viewer regressions | Every PR when relevant files change; locally before merge | Blocking | `./scripts/verify.sh`; GitHub Actions `Ubuntu test-all gate`, `Go unit tests`, viewer jobs when paths match | Test logs, verifier phase output, viewer lint/test output |
| 2. Contract and schema drift | Accidental changes to Compose, env, API health shape, migrations, generated OME config, or release artifact inputs | PRs that touch deployment, API, migrations, env templates, release packaging, or health surfaces | Blocking when implemented; currently partially covered by verifier contract checks | Current: `./scripts/verify.sh`, `docker compose --env-file .env -f deploy/docker-compose.yml config`, `./scripts/render-ome-config.sh --check`; planned: contract snapshot/diff gate | Compose config output, contract invariant output, generated OME check, future drift report |
| 3. Golden-path quickstart and smoke | A checkout or release candidate compiles but cannot start or pass operator smoke checks | Release candidates; PRs that change quickstart, deploy, smoke, Docker, or runtime startup paths | Blocking for release candidates; path-gated in PR CI | `./scripts/test-quickstart.sh`; `go run ./cmd/bitriver quickstart`; `go run ./cmd/bitriver smoke --env-file ./.env` | Quickstart smoke log, Compose service state, smoke check results, selected service logs on failure |
| 4. AI-authored PR risk scorecard | Large or automated changes landing without clear risk classification, evidence, docs impact, or rollback notes | PR review for Codex/AI-authored or high-risk changes | Advisory until automated; reviewer-blocking by policy when risk is unresolved | PR template plus reviewer checklist; planned scorecard automation | PR summary, changed-area classification, verification commands, docs/release note decisions |
| 5. Release readiness | Tags published with stale changelog, missing release notes, unpinned artifacts, or unverifiable Postgres/storage support | Before tagging and while release workflow runs | Blocking | `docs/production-release.md`; `.github/workflows/release.yml`; `.github/RELEASE_NOTES_TEMPLATE.md`; `./scripts/check-postgres-pgx.sh postgres`; `./scripts/require-image-digests.sh` | Release workflow artifacts, release notes, image digest list, env validation logs, pgx guard output |
| 6. Canary, observability, and rollback | A production rollout succeeds mechanically but cannot be observed, canaried, or rolled back safely | Staging and production rollout windows | Blocking for production change approval; mostly manual today | `docs/operations.md`, `docs/upgrades.md`, `/readyz`, `/healthz`, `/api/status`, monitoring dashboards and backup/restore runbooks | Canary notes, health snapshots, alert dashboard links, backup/restore evidence, rollback decision record |

## Gate details

### 1. Static repo hygiene

Run the local verifier before merging code or workflow changes:

```bash
./scripts/verify.sh
```

This is the default fast quality gate. It runs Go checks and contract checks, then runs Docker-dependent Compose validation and quickstart smoke when Docker is available. Viewer checks run when requested locally with `--viewer` or when CI path filters classify the change as viewer-related.

Investigate failures by reading the first failed verifier phase, then rerun the narrow command printed by that phase. Do not skip this gate for runtime changes; record any host-only blocker in `TASKS.md` and the PR.

### 2. Contract and schema drift

Contract drift covers operator-facing changes to:

- `deploy/docker-compose.yml`, root `.env`, and `deploy/.env.example`
- generated OME expectations under `deploy/ome/`
- API health/readiness/status response shape
- database migrations and storage import/verification paths
- release launcher inputs and artifact layout
- security-sensitive defaults for auth, sessions, CORS, metrics, public URLs, and image source

Current checks are spread across the verifier, Compose config validation, OME render checks, and contract invariant scripts. Planned contract snapshot and diff automation should produce one human-readable report that separates additive changes from breaking removals and security-sensitive default drift.

Intentional drift needs matching documentation in the same PR. Use `docs/contract-change-recipe.md` for deployment contract edits and include release notes or upgrade notes when the change affects existing operators.

### 3. Golden-path quickstart and smoke

The golden path protects the source checkout and packaged launcher flows that converge on the same Compose contract:

1. Prepare `.env` from `deploy/.env.example`.
2. Initialize/validate env values.
3. Render OME config when needed.
4. Start the Compose stack.
5. Run smoke checks against API, viewer, and service health.

For source validation, use:

```bash
go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml --image-source build
go run ./cmd/bitriver smoke --env-file ./.env
```

`./scripts/test-quickstart.sh` packages this into the release smoke path used by verification. When this gate fails, collect the failed phase, `docker compose ps`, relevant service logs, and smoke output before changing code.

### 4. AI-authored PR risk scorecard

AI-authored changes should make risk legible to human reviewers. Until automation exists, include these points in the PR body or review summary:

- changed areas and deployment/runtime impact
- commands run and any skipped checks
- whether docs, release notes, or upgrade notes changed
- whether contract drift is intentional
- rollback or mitigation notes for high-risk runtime changes

Treat missing evidence as advisory for small docs-only changes and blocking for runtime, deployment, auth, data, migration, or release workflow changes.

### 5. Release readiness

Before tagging, follow `docs/production-release.md`. A release candidate should prove:

- the verifier and required targeted gates passed
- release notes and changelog entries match the diff
- release workflow artifacts are built from the intended tag
- Postgres-aware binaries/images include the real pgx-backed implementation when required
- production image digests and third-party digests are recorded or intentionally deferred
- generated OME config matches the release env and image tag expectations

Failures here usually mean the tag should wait. Fix the source, rerun the relevant gate, and only then publish or promote artifacts.

### 6. Canary, observability, and rollback

Production promotion needs operator evidence, not just CI logs. Before reopening or expanding traffic:

- confirm `/readyz`, `/healthz`, and `/api/status` reflect the expected state
- watch the alerts and metrics documented in `docs/operations.md`
- confirm recent backup and restore-rehearsal evidence for stateful releases
- keep rollback notes aligned with `docs/upgrades.md`
- record canary duration, observed errors, and the decision to continue or roll back

If canary health is ambiguous, stop promotion and collect logs before applying more changes. A successful release is one that can be explained and reversed, not only one that started.

## Contributor guidance

Classify each change by the highest-risk area it touches:

- **Docs only:** run `git diff --check` and any doc-specific review. No runtime gate is required unless the doc changes an operator workflow.
- **Viewer only:** run viewer lint/tests plus the default verifier when practical.
- **API, auth, storage, ingest, or migrations:** run focused tests and the default verifier; add upgrade notes for schema or behavior changes.
- **Deploy, env, Compose, generated config, release packaging, or CI:** run the default verifier and the relevant contract or smoke command. Follow `docs/contract-change-recipe.md` for deployment contract edits.
- **Security-sensitive defaults or production rollout behavior:** update `docs/security.md`, `docs/operations.md`, or `docs/production-release.md` as appropriate and include rollback notes.

PRs should include the commands run, links to artifacts where available, and a short explanation of any accepted drift. When a future automated gate disagrees with the intended change, prefer updating the documented contract and baseline together instead of bypassing the gate silently.
