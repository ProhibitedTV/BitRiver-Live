# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [ ] Task 1 — Confirm scope and update `PLAN.md`
  - Acceptance criteria:
    - `PLAN.md` reflects current scope, risks, and test approach.
    - Any scope changes are captured before code edits.
  - Relevant checks:
    - Documentation sanity check (links/paths/commands are valid and present in repo).

- [ ] Task 2 — Implement top-to-bottom work items from plan
  - Acceptance criteria:
    - Each change maps to a concrete plan item.
    - No out-of-order task execution.
  - Relevant checks:
    - Run task-specific tests/checks and record results after each task.

- [ ] Task 3 — Final verification and handoff notes
  - Acceptance criteria:
    - `TASKS.md` statuses are up to date.
    - Final summary references completed tasks and checks.
  - Relevant checks:
    - Any final aggregate check required by scope (for example `./scripts/verify.sh` when applicable).
