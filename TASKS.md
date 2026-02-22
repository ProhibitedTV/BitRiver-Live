# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Update planning artifacts for quickstart preflight scope
  - Acceptance criteria:
    - `PLAN.md` documents scope, assumptions, risks, and test approach for this change.
    - `TASKS.md` lists ordered implementation tasks.
  - Relevant checks:
    - ✅ `test -s PLAN.md && test -s TASKS.md`
  - Result:
    - Passed.

- [x] Task 2 — Implement deployment preflight checks in quickstart
  - Acceptance criteria:
    - Quickstart validates Docker daemon availability, Docker Compose v2, and env-derived host-port availability before deployment starts.
    - Failures include clear one-line next actions.
    - No deployment pipeline behavior changes beyond preflight/output.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./cmd/bitriver -run Quickstart -count=1`
  - Result:
    - Passed.

- [x] Task 3 — Final verification and handoff
  - Acceptance criteria:
    - Required checks executed and results recorded.
    - Task statuses updated to complete.
  - Relevant checks:
    - ✅ `./scripts/verify.sh`
  - Result:
    - Passed (with expected docker-dependent steps skipped by script in this environment).
