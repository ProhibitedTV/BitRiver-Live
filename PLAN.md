# PLAN

## Technical plan
1. Read `SPEC.md` and relevant repo docs/code in read-only mode.
2. Update this file with scope, assumptions, and implementation approach.
3. Break implementation into ordered tasks in `TASKS.md`.
4. Execute tasks top-to-bottom; do not skip ahead.

## Risks
- Tasks may be too large to review quickly.
- Test mapping may be unclear for doc-only or infra-only changes.
- Plan drift if `PLAN.md` is not refreshed when scope changes.

## Test plan
- For each task, choose the smallest relevant check(s) (example: targeted unit test, lint, or script).
- Run checks immediately after task implementation.
- Record command(s) and pass/fail in `TASKS.md` before moving to next task.
