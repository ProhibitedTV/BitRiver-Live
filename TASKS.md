# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Confirm scope and update `PLAN.md` with read-only analysis
  - Acceptance criteria:
    - `PLAN.md` documents scope, assumptions, risks, and test plan for this viewer state-mapping change.
    - Authoritative known persisted `live_state` values are identified from backend code paths.
  - Relevant checks:
    - ✅ `test -s PLAN.md`
  - Result:
    - Passed.

- [x] Task 2 — Implement explicit `deriveControlCentreStatus()` mapping for known states
  - Acceptance criteria:
    - `starting`, `live`, `offline`, and `ended` all have explicit branches with stable `label`, `badgeClassName`, and `reason` behavior.
    - Default branch remains for unknown values with wording indicating unexpected server state.
    - TODO comment is removed.
  - Relevant checks:
    - ✅ `cd web/viewer && npm run test -- creatorLiveStreamStatus`
  - Result:
    - Passed.

- [x] Task 3 — Add/extend viewer tests for all mapped states + unknown fallback
  - Acceptance criteria:
    - Tests cover each known mapped state and one unknown fallback case.
    - Assertions verify stable mapped outputs.
  - Relevant checks:
    - ✅ `cd web/viewer && npm run test -- creatorLiveStreamStatus`
  - Result:
    - Passed.

- [x] Task 4 — Run required validation and finalize task log
  - Acceptance criteria:
    - Required repo check(s) run and outcomes recorded.
    - `TASKS.md` statuses/results are up to date.
  - Relevant checks:
    - ❌ `./scripts/verify.sh`
  - Result:
    - Failed due existing viewer snapshot mismatch in `__tests__/channelDisplayPrimitives.test.tsx` unrelated to this change.

## Execution log
- Task 1 check: `test -s PLAN.md` (pass).
- Task 2 check: `cd web/viewer && npm run test -- creatorLiveStreamStatus` (pass).
- Task 3 check: `cd web/viewer && npm run test -- creatorLiveStreamStatus` (pass).
- Task 4 check: `./scripts/verify.sh` (fail: snapshot mismatch in `channelDisplayPrimitives.test.tsx`; docker compose validation skipped because docker is not installed).
