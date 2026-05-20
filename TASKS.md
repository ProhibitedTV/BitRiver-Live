## Scoped change: issue #1229 cleanup tracking hygiene

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the repo hygiene plan
  - Acceptance criteria:
    - `PLAN.md` captures issue #1229 scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists ordered implementation and validation tasks before source edits continue.
    - The read-only pass identifies cleanup-plan gaps, historical root scratchpad content, and existing contributor guidance.

- [x] Task 2 - Create durable tracking for remaining cleanup-plan work
  - Acceptance criteria:
    - Existing open issues are searched before new cleanup issues are created.
    - Remaining `docs/cleanup-plan.md` tasks 2-7 link to GitHub issues or are explicitly deferred with a reason.
    - `docs/cleanup-plan.md` remains readable as a summary index.

- [x] Task 3 - Archive historical planning scratchpads
  - Acceptance criteria:
    - Previous `PLAN.md` history is preserved under `docs/history/`.
    - Previous `TASKS.md` history is preserved under `docs/history/`.
    - Root `PLAN.md` and `TASKS.md` contain only the current active scope.

- [x] Task 4 - Document planning and issue-tracking lifecycle
  - Acceptance criteria:
    - Contributor-facing docs explain when to use `SPEC.md`, `PLAN.md`, `TASKS.md`, `docs/cleanup-plan.md`, and GitHub issues.
    - The guidance makes temporary planning files active-scope artifacts, not permanent historical logs.
    - The new or updated doc is discoverable from existing contributor docs.

- [-] Task 5 - Validate and publish
  - Acceptance criteria:
    - Documentation/link checks and diff hygiene pass.
    - The repo validation gate is run or any host blocker is recorded.
    - Changes are committed, pushed, opened as a draft PR, CI is checked, and the PR is merged when green.

### Execution log (issue #1229 cleanup tracking hygiene)
- Task 1 complete: after merging PR #1240 and syncing `main` to `d14f7274`, selected issue #1229 as the next open ticket, created branch `codex/issue-1229-cleanup-tracking`, read the docs-scoped agent note, audited `docs/cleanup-plan.md`, existing root `PLAN.md`/`TASKS.md` history, and contributor docs that mention the planning workflow.
- Task 1 checks:
  - GitHub connector: searched open issues sorted by creation date and selected issue #1229.
  - `git checkout main`
  - `git pull --ff-only origin main`
  - `git checkout -b codex/issue-1229-cleanup-tracking`
  - `rg --files -g AGENTS.md`
  - `Get-Content docs/AGENTS.md`
  - `Get-Content docs/cleanup-plan.md`
  - `Get-Content PLAN.md | Select-Object -First 120`
  - `Get-Content TASKS.md | Select-Object -First 140`
  - `rg -n "maintenance|cleanup-plan|PLAN\\.md|TASKS\\.md|planning docs|GitHub issues|temporary planning|archive" README.md CONTRIBUTING.md docs`
  - GitHub connector searches for existing cleanup tracking issues by quickstart, transcoder, deterministic clock, Postgres legal, and upload-helper keywords found no matches.
- Task 2 complete: created GitHub issues [#1241](https://github.com/ProhibitedTV/BitRiver-Live/issues/1241), [#1242](https://github.com/ProhibitedTV/BitRiver-Live/issues/1242), [#1243](https://github.com/ProhibitedTV/BitRiver-Live/issues/1243), [#1244](https://github.com/ProhibitedTV/BitRiver-Live/issues/1244), [#1245](https://github.com/ProhibitedTV/BitRiver-Live/issues/1245), and [#1246](https://github.com/ProhibitedTV/BitRiver-Live/issues/1246) for the remaining cleanup-plan tasks, then linked them from `docs/cleanup-plan.md`.
- Task 2 checks:
  - GitHub connector: created issues #1241-#1246.
  - `docs/cleanup-plan.md` tasks 2-7 now each include a tracking issue link.
- Task 3 complete: moved prior root `PLAN.md` and `TASKS.md` content into `docs/history/PLAN-archive-2026-05-20.md` and `docs/history/TASKS-archive-2026-05-20.md`, added archive banners, and replaced the root files with the active #1229 plan/task list.
- Task 3 checks:
  - Root `PLAN.md` and `TASKS.md` now contain only issue #1229 active scope.
- Task 4 complete: added `docs/maintenance.md` to explain the planning-doc versus GitHub-issue lifecycle and linked it from `CONTRIBUTING.md`.
- Task 4 checks:
  - `CONTRIBUTING.md` now lists `docs/maintenance.md` in helpful references and points larger scoped work guidance to it.
- Task 5 in progress: ran documentation tracking checks, diff hygiene, and the full repo verification gate before staging.
- Task 5 checks:
  - `rg -n "Task [2-7]|Tracking issue: \\[#124[1-6]\\]" docs/cleanup-plan.md` - passed.
  - `rg -n "PLAN\\.md|TASKS\\.md|cleanup-plan|GitHub issues|temporary planning|active-scope|docs/history" docs/maintenance.md CONTRIBUTING.md` - passed.
  - `git diff --check` - passed with expected working-tree line-ending warnings.
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh` - passed full repo verification, including Go tests, contract checks, Docker Compose config validation, quickstart smoke, and skipped viewer checks because no viewer files changed.
