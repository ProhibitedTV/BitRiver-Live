# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add verify smoke phase with deterministic Docker-gated ordering
  - Acceptance criteria:
    - `scripts/verify.sh` runs quickstart smoke immediately after Docker Compose validation when Docker is available.
    - Docker-unavailable path prints explicit skip messaging for both compose validation and quickstart smoke.
    - Failures from `./scripts/test-quickstart.sh` fail `./scripts/verify.sh`.

- [x] Task 2 — Sync command contract docs with verify coverage
  - Acceptance criteria:
    - `AGENTS.md` required checks section reflects verify gate sequence including smoke phase and Docker skip semantics.
    - `docs/testing.md` verify section documents quickstart smoke phase under verify.
    - `docs/production-release.md` mentions default verify coverage consistently where relevant.

- [x] Task 3 — Validate and record results
  - Acceptance criteria:
    - Run syntax/behavior checks (`bash -n scripts/verify.sh`, `./scripts/verify.sh`).
    - Record outcomes in the execution log below.

## Execution log
- ✅ Task 1 complete: `scripts/verify.sh` now runs Docker compose validation then `./scripts/test-quickstart.sh` when Docker is available, and prints explicit skip messages for both steps when Docker is unavailable.
  - Check: `bash -n scripts/verify.sh` (pass).
- ✅ Task 2 complete: updated `AGENTS.md`, `docs/testing.md`, and `docs/production-release.md` to reflect verify smoke-phase coverage and Docker skip semantics.
  - Check: `rg -n "quickstart smoke|Docker-dependent|scripts/verify.sh" AGENTS.md docs/testing.md docs/production-release.md` (pass).
- ✅ Task 3 complete: verification commands passed, including full `./scripts/verify.sh` run with deterministic Docker skip messaging and successful remaining gates.
