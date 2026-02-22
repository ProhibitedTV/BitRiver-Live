# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Confirm scope and update `PLAN.md`
  - Acceptance criteria:
    - `PLAN.md` reflects current scope, risks, and test approach.
    - Any scope changes are captured before code edits.
  - Relevant checks:
    - Documentation sanity check (links/paths/commands are valid and present in repo).

- [x] Task 2 — Implement top-to-bottom work items from plan
  - Acceptance criteria:
    - Each change maps to a concrete plan item.
    - No out-of-order task execution.
  - Relevant checks:
    - Run task-specific tests/checks and record results after each task.

- [x] Task 3 — Final verification and handoff notes
  - Acceptance criteria:
    - `TASKS.md` statuses are up to date.
    - Final summary references completed tasks and checks.
  - Relevant checks:
    - Any final aggregate check required by scope (for example `./scripts/verify.sh` when applicable).


## Execution log
- Task 1 check: Reviewed updated `PLAN.md` for scope/risks/test-plan alignment (pass).
- Task 2 check: `rg -n "Required end-of-run self-check|Did I run the right commands\?|scripts/verify\.sh|What remains incomplete\?" AGENTS.md` (pass; required footer bullets and default command present).
- Task 3 check: Verified task statuses and log entries are up to date in `TASKS.md` (pass).
