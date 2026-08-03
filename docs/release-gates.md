# Release gate ladder

BitRiver Live promotes changes through a small set of gates that protect the supported deployment boundary: an operator-managed, single-host Docker Compose stack using the repository-root `.env`, `deploy/docker-compose.yml`, and the shared source quickstart or packaged launcher paths.

These gates do not promise a Kubernetes-first platform, managed hosting, multi-host high availability, or SaaS operation. They answer one practical question: can a real operator still install, upgrade, verify, and roll back BitRiver Live through the documented single-host path?

## Gate summary

| Gate | Risk caught | When it runs | Status | Command or workflow | Evidence |
| --- | --- | --- | --- | --- | --- |
| 1. Static repo hygiene | Formatting drift, broken unit tests, stale generated contract checks, obvious viewer regressions | Every PR when relevant files change; locally before merge | Path-selective children; blocking aggregate | `./scripts/verify.sh`; GitHub Actions `Ubuntu test-all gate`, cross-platform Go, viewer jobs when paths match; required `Merge gate` | Test logs, verifier phase output, viewer output, aggregate job summary/artifact |
| 2. Contract and schema drift | Accidental changes to Compose, env, API health shape, migrations, generated OME config, or release artifact inputs | PRs that touch deployment, API, migrations, env templates, release packaging, or health surfaces | Blocking for breaking/security-sensitive drift | Current: `go run ./cmd/bitriver release contract-snapshot`, `go run ./cmd/bitriver release contract-diff`, `./scripts/verify.sh`, `docker compose --env-file .env -f deploy/docker-compose.yml config`, `./scripts/render-ome-config.sh --check` | Contract snapshot JSON, drift report, Compose config output, contract invariant output, generated OME check |
| 3. Golden-path quickstart and smoke | A checkout or release candidate compiles but cannot start or pass operator smoke checks | Release candidates; PRs that change quickstart, deploy, smoke, Docker, or runtime startup paths | Blocking for release candidates; path-gated in PR CI | `./scripts/release-gate-smoke.sh --tier fast`; `./scripts/release-gate-smoke.sh --tier full`; `./scripts/test-quickstart.sh`; `go run ./cmd/bitriver smoke --env-file ./.env` | Release-gate report JSON, contract snapshot, redacted env summary, Compose config output, quickstart/smoke logs, Compose state/log diagnostics |
| 4. Production media and workflow acceptance | Healthy containers hide broken RTMP mapping, OME/transcoder output, chat/moderation, VOD processing, or aggregate status | Runtime/media changes; every release candidate; repeated after installing tagged artifacts | Blocking | `./scripts/test-production-golden-path.sh --stack quickstart --client docker`; `.github/workflows/ingest-e2e.yml`; release workflow `pull-only-product-gate`; `--stack running` on a prepared candidate host | Scanner-approved `production-golden-path.json` with stage timing, advancing playlist evidence, 1080p probes, workflow results, image-manifest evidence, and final status |
| 5. AI-authored PR risk scorecard | Large or automated changes landing without clear risk classification, evidence, docs impact, or rollback notes | Every PR; strict when declared or automatically classified as risky | Advisory for docs/planning-only; blocking for risky paths | PR template plus `./scripts/check-pr-release-scorecard.sh --strict-if-risky`; required `Merge gate`; see `docs/pr-release-scorecard.md` | PR summary, changed-area classification, verification commands, skipped-check disclosure, scorecard report artifact |
| 6. Release readiness | Tags published with stale changelog, missing release notes, unpinned or secret-bearing artifacts, or unverifiable Postgres/storage support | Before tagging and while release workflow runs | Blocking | `docs/production-release.md`; `.github/workflows/release.yml`; `.github/RELEASE_NOTES_TEMPLATE.md`; `./scripts/check-postgres-pgx.sh postgres`; `./scripts/require-image-digests.sh`; `./scripts/scan-release-evidence.sh` | Redacted contract status, release artifact inventory and scan status, release notes, image digest status, pgx guard output |
| 7. Canary, observability, and rollback | A production rollout succeeds mechanically but cannot be observed, canaried, or rolled back safely | Staging and production rollout windows | Blocking for production change approval; non-mutating command plus operator evidence | `./scripts/release-canary.sh`; `go run ./cmd/bitriver release canary`; `docs/operations.md`; `/readyz`, `/healthz`, `/api/status` | Canary report JSON, redacted health snapshots, log scan summary, version metadata, rollback readiness notes |

## Gate details

### Pull-request aggregate enforcement

`Merge gate` is the single stable required check for `main`. It runs with
`if: always()` after every CI child and compares each result with the
changed-file outputs. Required jobs must succeed. Correctly unrelated jobs may
be skipped; unexpected skips, failures, and cancellations block the aggregate.
The job publishes a Markdown result table and the PR scorecard report as a
14-day Actions artifact.

Branch protection requires a pull request, a current successful `Merge gate`,
resolved conversations, and admin compliance; force pushes and deletion are
disabled. Emergency changes follow the audited break-glass procedure in
[`SECURITY.md`](../SECURITY.md). This protects merges only. Stable promotion of
an existing immutable candidate is separately enforced by the read-only
`Stable promotion gate`, the `stable-promotion` environment, and
[`docs/release-promotion.md`](release-promotion.md); it must not be inferred
from a green merge gate.

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

Current checks are spread across the release contract snapshot command, the verifier, Compose config validation, OME render checks, and contract invariant scripts. Generate a stable contract snapshot with:

```bash
go run ./cmd/bitriver release contract-snapshot \
  --env-file deploy/.env.example \
  --compose-file deploy/docker-compose.yml \
  --output .tmp/contract-snapshot.json
```

Compare two snapshots with:

```bash
go run ./cmd/bitriver release contract-diff \
  --base .tmp/base-contract.json \
  --head .tmp/head-contract.json
```

The first snapshot version is intentionally modest. It captures env template keys/defaults/comments, Compose service shape, migration file hashes, generated artifact presence, health endpoint names, and release artifact inputs without requiring Docker or a running stack. The diff report separates additive changes from breaking removals, security-sensitive default drift, and undocumented env additions.

Intentional drift needs matching documentation in the same PR. Use `docs/contract-change-recipe.md` for deployment contract edits and include release notes or upgrade notes when the change affects existing operators.

### 3. Golden-path quickstart and smoke

The golden path protects the source checkout and packaged launcher flows that converge on the same Compose contract:

1. Prepare `.env` from `deploy/.env.example`.
2. Initialize/validate env values.
3. Render OME config when needed.
4. Start the Compose stack.
5. Run smoke checks against API, viewer, and service health.

Run the fast PR tier with:

```bash
./scripts/release-gate-smoke.sh --tier fast --target vX.Y.Z
```

The fast tier writes `.artifacts/release-gate/` evidence by default:

- `release-gate-report.json` with phase status, artifact paths, and staged follow-up notes
- `version.txt`
- `env-redaction-summary.json` with keys and redaction coverage only, never env values
- `contract-snapshot.json`
- `compose-config.yml` when Docker Compose is available, or an explicit skipped artifact when it is not
- `upgrade-plan.txt` when `--target` is supplied

PR CI runs this fast tier automatically through `scripts/test-all.sh` whenever the existing quickstart path filter enables `BITRIVER_TEST_QUICKSTART=1`. This keeps the workflow wiring centralized while making Gate 3 visible by name.

Before tagging or promoting a release candidate, run the full source-checkout tier on a host with Docker Compose available:

```bash
./scripts/release-gate-smoke.sh --tier full --target vX.Y.Z
```

The full tier runs the fast evidence phases, then executes source quickstart, runs the smoke command, and captures `compose-ps.json` plus selected `compose-logs.txt` diagnostics. A failed phase returns a non-zero exit and names the artifact to inspect.

Before attaching `.artifacts/release-gate/`, canary output, or collected diagnostics outside the operator host, run `./scripts/scan-release-evidence.sh --root <artifact-directory>`. Raw Compose logs and deployment-time generated configs are not automatically safe merely because the surrounding report is redacted.

For source validation, use:

```bash
go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml --image-source build
go run ./cmd/bitriver smoke --env-file ./.env
```

`./scripts/test-quickstart.sh` packages this into the release smoke path used by verification. When this gate fails, collect the failed phase, `docker compose ps`, relevant service logs, and smoke output before changing code.

Packaged launcher and upgrade execution evidence remain staged release-candidate follow-ups for now:

- Packaged launcher smoke: install or unpack the release artifact, run the launcher against a clean env, run smoke, and attach launcher logs plus version output.
- Upgrade smoke: start the previous release or baseline, preserve stateful data, apply the target release, run migrations if applicable, run smoke, and attach upgrade notes.

The `--target` flag produces an upgrade-plan artifact in both tiers so release reviewers can see the migration and operator checklist even before a slower baseline-to-target upgrade rehearsal is automated.

### 4. Production media and workflow acceptance

Quickstart and health probes cannot establish that real media advances or that
product workflows work across service boundaries. Run the destructive
source-checkout gate on a clean Docker-capable host:

```bash
./scripts/test-production-golden-path.sh \
  --stack quickstart \
  --client docker
```

The gate creates creator and viewer sessions through public APIs, bootstraps a
channel, sends deterministic 1920x1080 video plus audio over RTMP, requires the
live/offline transitions, and probes advancing decodable HLS from both
OvenMediaEngine and the FFmpeg transcoder. It also requires chat/history,
moderation, multipart VOD processing/publication/playback, viewer/health/metrics
surfaces, and a final ready aggregate status.

The lifecycle-owning mode uses isolated state and removes the stack afterward.
It retains only
`.artifacts/production-golden-path/production-golden-path.json`, which is
scanned against per-run account, password, token, and stream-key sentinels.
Never attach raw media-service logs or generated configs merely because the
JSON scan passed.

Source-build success is not tagged-artifact proof. For a tag, the release
workflow publishes five images to `ghcr.io/prohibitedtv`, logs out of GHCR,
waits boundedly for every anonymous manifest, pins the actual first- and
third-party digests, and reruns this same gate in production/pull mode. GitHub
Release creation depends on that result.

That hosted-runner proof is still not clean-host proof. Before stable
promotion, install the immutable pull-only artifacts on the supported Ubuntu
24.04 amd64 candidate and rerun the same assertions with `--stack running`.
Authenticated OME control, Nginx Proxy Manager/browser playback, and VM
reboot/recovery remain separate required evidence.

### 5. AI-authored PR risk scorecard

AI-authored changes should make risk legible to human reviewers. Fill in the release scorecard in the PR template and keep it current when the diff changes.

- changed areas and deployment/runtime impact
- commands run and any skipped checks
- whether docs, release notes, or upgrade notes changed
- whether contract drift is intentional
- rollback or mitigation notes for high-risk runtime changes

Run the advisory validator locally against a PR body draft:

```bash
./scripts/check-pr-release-scorecard.sh --body pr-body.md
```

Pass `--changed-files` to catch obvious mismatches between the diff and selected
scorecard fields. Pull-request CI uses `--strict-if-risky`; `--strict` remains
available when release management wants every warning to fail. See
`docs/pr-release-scorecard.md` for examples and Codex-authored PR expectations.

Treat missing evidence as advisory for small docs-only changes and blocking for runtime, deployment, auth, data, migration, or release workflow changes.

### 6. Release readiness

Before tagging, follow `docs/production-release.md`. A release candidate should prove:

- the verifier and required targeted gates passed
- release notes and changelog entries match the diff
- release workflow artifacts are built from the intended tag
- production credentials remain job-local and no populated environment or generated credential config is uploaded
- `release-contract-evidence` reports successful environment validation and
  includes the scanned immutable third-party reference/digest set reused by the
  product gate, without credentials
- every first-party manifest is anonymously readable under
  `ghcr.io/prohibitedtv/<image>:<exact-tag>` before publication proceeds
- `release-product-evidence` proves the pulled, digest-pinned image set passed
  the production media/API golden path
- `release-publication-evidence` inventories and passes scans for downloaded artifacts and the final publication payload
- a signed `release-set.json` binds every public candidate asset, five image
  digests/signatures, dependency pins, and fixed-schema evidence; complete
  `CHECKSUMS.txt` coverage re-verifies before publication
- Postgres-aware binaries/images include the real pgx-backed implementation when required
- production image digests and third-party digests are recorded
- tracked and packaged OME config matches the placeholder contract; deployment-time credential renders remain local to the deployment host

Failures before tagging mean the tag should wait. A failed tag is immutable:
fix the source, increment the RC number, and rerun rather than force-moving the
existing tag. RC tags are prereleases and do not update `latest`; stable tags
never enter the build workflow. The guarded promotion workflow may move stable
and optional `latest` aliases only after one tracked record binds all required
gate evidence to the same signed candidate root. A revocation marker blocks
promotion before environment approval.

### 7. Canary, observability, and rollback

Production promotion needs operator evidence, not just CI logs. Before reopening or expanding traffic:

- confirm `/readyz`, `/healthz`, and `/api/status` reflect the expected state
- watch the alerts and metrics documented in `docs/operations.md`
- confirm recent backup and restore-rehearsal evidence for stateful releases
- keep rollback notes aligned with `docs/upgrades.md`
- record canary duration, observed errors, and the decision to continue or roll back

After a release-candidate stack is deployed, run the non-mutating canary gate from a host that can reach the API:

```bash
./scripts/release-canary.sh \
  --base-url https://api.example.com \
  --logs-file .tmp/recent-compose-logs.txt \
  --rollback-notes .tmp/rollback-notes.md \
  --require-rollback-notes \
  --artifact-dir .artifacts/release-canary
```

The gate writes `canary-report.json`, CLI version evidence, redacted endpoint response artifacts, a log scan summary, and rollback readiness evidence. It fails on endpoint transport/HTTP failures, explicit unhealthy/degraded JSON status fields, high-confidence fatal/error log patterns, or missing required rollback-note coverage. Optional evidence that is not supplied is recorded as a warning so local dry-runs remain useful without implying production approval.

If canary health is ambiguous, stop promotion and collect logs before applying more changes. A successful release is one that can be explained and reversed, not only one that started.

## Contributor guidance

Classify each change by the highest-risk area it touches:

- **Docs only:** run `git diff --check` and any doc-specific review. No runtime gate is required unless the doc changes an operator workflow.
- **Viewer only:** run viewer lint/tests plus the default verifier when practical.
- **API, auth, storage, ingest, or migrations:** run focused tests and the default verifier; add upgrade notes for schema or behavior changes.
- **Deploy, env, Compose, generated config, release packaging, or CI:** run the default verifier and the relevant contract or smoke command. Follow `docs/contract-change-recipe.md` for deployment contract edits.
- **Security-sensitive defaults or production rollout behavior:** update `docs/security.md`, `docs/operations.md`, or `docs/production-release.md` as appropriate and include rollback notes.

PRs should include the commands run, links to artifacts where available, and a short explanation of any accepted drift. When a future automated gate disagrees with the intended change, prefer updating the documented contract and baseline together instead of bypassing the gate silently.
