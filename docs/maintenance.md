# Maintenance Tracking

This project uses two kinds of planning artifacts:

- **GitHub issues** are durable backlog items. Use them for cleanup work, bugs, follow-ups, and anything that should survive beyond the current branch.
- **`SPEC.md`, `PLAN.md`, and `TASKS.md` are active-scope scratchpads.** Use them while implementing one scoped change, keep them concise, and do not treat them as a permanent changelog.

## Planning Lifecycle

1. Start a scoped change by recording current goals in `SPEC.md` when needed, then update `PLAN.md` with assumptions, risks, and validation.
2. Break the work into small ordered steps in `TASKS.md`.
3. Execute `TASKS.md` top to bottom, updating task status and check results as work completes.
4. Before the branch is finished, keep root `PLAN.md` and `TASKS.md` focused on the active scope only.
5. If prior planning notes still matter, move them under `docs/history/` with a short archive banner.

## Cleanup Plan

`docs/cleanup-plan.md` is a curated maintenance index, not the main backlog. Each unchecked cleanup item should either:

- link to a GitHub issue that owns the work, or
- say it is deferred and explain why.

When a cleanup issue is completed, update `docs/cleanup-plan.md` so the index stays useful without hiding work from the issue tracker.

## Contributor Rule Of Thumb

- Use GitHub issues for work that is not being implemented right now.
- Use root planning files for work that is actively in progress on the current branch.
- Archive old root planning content instead of letting `PLAN.md` and `TASKS.md` grow without bounds.
