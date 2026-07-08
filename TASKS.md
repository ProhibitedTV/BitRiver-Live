## Scoped change: document release-gate ladder (#1269)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Establish the documentation scope
  - Acceptance criteria:
    - `PLAN.md` captures #1269 scope, assumptions, risks, and test plan.
    - `TASKS.md` lists ordered tasks before release documentation edits.
    - Existing release docs and workflow references are reviewed before adding new guidance.

- [x] Task 2 - Add release-gate ladder documentation
  - Acceptance criteria:
    - A concise `docs/release-gates.md` exists.
    - The doc names each practical gate and explains risk, timing, blocking status, command/workflow, artifacts, and failure triage.
    - The doc states the supported single-host Compose boundary and does not imply Kubernetes-first or managed-SaaS support.
    - Planned gates are clearly marked where implementation is not yet present.

- [x] Task 3 - Link release-gate docs from release-facing entry points
  - Acceptance criteria:
    - Existing release docs link to `docs/release-gates.md`.
    - README key docs and release readiness pointers include the new guide where useful.
    - No runtime behavior, deployment contract, or workflow behavior changes are introduced.

- [x] Task 4 - Verify and record results
  - Acceptance criteria:
    - `git diff --check` passes.
    - Documentation is checked against #1269 acceptance criteria.
    - Any skipped runtime checks are justified as docs-only.

### Execution log
- Task 1 read-only pass:
  - `git status --short --branch` showed `main` with untracked local deployment artifacts (`DEPLOYMENT_ASSURANCE.md`, `DEPLOYMENT_GUIDE.md`, `deploy/ome-diagnostics.sh`, `deploy/startup.sh`, `deploy/validate-deployment.sh`) that remain out of scope.
  - GitHub issue list showed #1269 as the oldest concrete documentation issue under release-gate epic #1264; #1265 and #1266 depend on this release-gate source of truth.
  - Reviewed `README.md`, `docs/production-release.md`, `docs/operations.md`, release workflow references, and docs search results before planning edits.
  - Check: documentation scope review complete; no runtime command required for read-only planning.
- Task 2 complete: added `docs/release-gates.md` with a six-step release-gate ladder covering static hygiene, contract/schema drift, golden-path smoke, AI-authored PR risk, release readiness, and canary/rollback evidence.
  - Check: issue #1269 acceptance criteria reviewed against the new doc; runtime tests not required for docs-only content.
- Task 3 complete: linked `docs/release-gates.md` from the README key docs list, README release readiness checklist, and the opening of `docs/production-release.md`.
  - Check: `rg -n "release-gates|Release gates|gate ladder" README.md docs/production-release.md docs/release-gates.md` confirmed all intended links.
- Task 4 complete:
  - `git diff --check` - passed with line-ending warnings only.
  - Final read of `docs/release-gates.md` confirmed each #1269 gate field is covered and planned gates are marked.
  - Runtime checks skipped because this change only updates documentation and working artifacts.
