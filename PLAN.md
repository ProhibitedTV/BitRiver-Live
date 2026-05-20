## Scope (current change)
- Address GitHub issue #1229 by refreshing cleanup tracking and retiring stale `PLAN.md`/`TASKS.md` scratchpad history.
- Convert the remaining unchecked `docs/cleanup-plan.md` tasks into explicit GitHub issue links, unless a task is intentionally deferred with a reason.
- Archive completed historical planning logs under `docs/history/` so the repo root planning files show only the active scoped change.
- Add concise maintenance guidance that explains when contributors should use planning docs versus GitHub issues.

## Assumptions
- The existing long `PLAN.md` and `TASKS.md` content is useful history, so archiving is safer than deletion.
- `docs/cleanup-plan.md` remains useful as a summarized cleanup index once each open task links to durable issue tracking.
- GitHub issues are the canonical place for follow-up cleanup work that is not actively being implemented in the current branch.
- This change is documentation and process hygiene only; no runtime, deployment contract, or CI workflow behavior should change.

## Risks
- Moving large planning files can obscure recent context if the archive path is not obvious.
- Creating duplicate GitHub issues would make cleanup tracking noisier, so issue search should precede new issue creation.
- `PLAN.md`/`TASKS.md` still need to satisfy the agent workflow while no longer carrying merged historical logs.
- Documentation should be explicit enough to prevent these root scratchpads from becoming a permanent changelog again.

## Test plan
- GitHub connector issue search for existing cleanup-tracking issues before creating new ones.
- `rg -n "Task [2-7].*github.com|tracking issue|deferred" docs/cleanup-plan.md`
- `rg -n "PLAN.md|TASKS.md|cleanup-plan|GitHub issues|temporary planning" docs/maintenance.md CONTRIBUTING.md`
- `git diff --check`
- `./scripts/verify.sh`
