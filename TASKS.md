# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Audit channel page test async interactions
  - Acceptance criteria:
    - Potential act/warning-prone interaction points are identified in `web/viewer/__tests__/channelPage.test.tsx`.

- [x] Task 2 — Update interaction/assertion flow to awaited Testing Library patterns
  - Acceptance criteria:
    - Retry/load and related state-updating interactions use `userEvent.setup()` + awaited calls and/or `waitFor`/`act` appropriately.
    - No functional code changes are made in `web/viewer/app/channels/[id]/page.tsx`.

- [x] Task 3 — Validate viewer checks and record results
  - Acceptance criteria:
    - `npm --prefix web/viewer run test` executed; channel page suite is warning-free, while pre-existing warnings remain in other suites.
    - `npm --prefix web/viewer run lint` passes.

## Execution log
- ✅ Task 1 complete: identified warning-prone interactions in retry and signed-out flows (`userEvent` singleton usage, manual `act` + DOM `click`, and `fireEvent.click`).
- ✅ Task 2 complete: converted channel page state-changing interactions to awaited `userEvent.setup()` flows with focused `act`/`waitFor` coverage around retry and auth-prompt paths.
- ✅ Task 3 complete: reran `npm --prefix web/viewer run test -- channelPage.test.tsx`, `npm --prefix web/viewer run test`, and `npm --prefix web/viewer run lint`; channel page warnings are cleared and lint is clean.
