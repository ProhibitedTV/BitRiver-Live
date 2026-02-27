# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [ ] Task 1 — Environment readiness verification
  - Acceptance criteria:
    - Required local tooling/runtime prerequisites are checked and recorded (Go, Node/npm when viewer scope applies, Docker/Compose, GitHub CLI optional).
    - Availability of release secrets/deployment credentials is explicitly confirmed by owner checklist reference (without exposing values).
  - Check commands:
    - `go version`
    - `node --version`
    - `npm --version`
    - `docker --version`
    - `docker compose version`
    - `gh --version`

- [ ] Task 2 — Digest enforcement/static supply-chain scan
  - Acceptance criteria:
    - Workflow/action references are confirmed SHA-pinned for external actions.
    - Release/deploy manifests reference immutable image digests where contract requires them.
  - Check commands:
    - `./scripts/check-ci-contract.sh`
    - `./scripts/require-image-digests.sh`
    - `rg -n "uses:\\s+[^#\\n]+@(v|main|master|[0-9]+(\\.[0-9]+)*)" .github/workflows`

- [ ] Task 3 — Release workflow parity (scripts/docs/workflows)
  - Acceptance criteria:
    - Release workflow docs map to existing scripts/commands with no invented steps.
    - Any workflow-impacting script references in docs resolve to real files/commands.
  - Check commands:
    - `rg -n "quickstart|verify|production-release|release" docs/ README.md scripts/ .github/workflows`
    - `rg -n "scripts/[A-Za-z0-9._/-]+" docs/ README.md`

- [ ] Task 4 — Runbook parity validation
  - Acceptance criteria:
    - Operator runbooks (`docs/operations.md`, `docs/production-release.md`, and related deploy docs) are mutually consistent on release order, rollback hooks, and verification points.
    - Any identified gaps are documented as follow-up items before execution.
  - Check commands:
    - `rg -n "rollback|release|verify|quickstart|smoke" docs/operations.md docs/production-release.md docs/advanced-deployments.md docs/testing.md`
    - `rg -n "deploy/docker-compose.yml|\.env|Server.generated.xml" docs/contract.md docs/production-release.md docs/operations.md`

- [ ] Task 5 — Final release gate checks and handoff packet
  - Acceptance criteria:
    - Full verification command set is executed (or deferred with explicit blocker + owner).
    - Handoff note records outcomes for environment, digest checks, workflow parity, runbook parity, and outstanding blockers.
  - Check commands:
    - `./scripts/verify.sh`
    - `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s`
    - `docker compose -f deploy/docker-compose.yml config`
    - `./scripts/test-quickstart.sh`

## Execution log
- Pending execution. Update each task status and append command outputs/results immediately after each task is performed.
