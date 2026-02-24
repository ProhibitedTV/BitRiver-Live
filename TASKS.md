# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Create internal viewer API core + first domain slices
  - Acceptance criteria:
    - Add shared request/types internals used by domain modules.
    - Move a small subset of domain implementation from `viewer-api.ts` into new modules without changing behavior.
    - `viewer-api.ts` remains compiling as public facade.
  - Relevant checks:
    - ✅ `cd web/viewer && npm run test -- --runInBand __tests__/viewer-api.test.ts`

- [x] Task 2 — Complete domain split and keep facade exports stable
  - Acceptance criteria:
    - `viewer-api.ts` re-exports all previously exported types/functions unchanged.
    - Remaining implementation is moved into domain modules under `web/viewer/lib/`.
  - Relevant checks:
    - ✅ `cd web/viewer && npm run test -- --runInBand --testPathPattern=viewer-api`

- [x] Task 3 — Final verification and task log update
  - Acceptance criteria:
    - Run requested viewer API tests and record outcomes.
    - Task statuses and execution log are current.
  - Relevant checks:
    - ✅ `cd web/viewer && npm run test -- --runInBand __tests__/viewer-api.test.ts`
    - ✅ `cd web/viewer && npm run test -- --runInBand --testPathPattern=viewer-api`

## Execution log
- ✅ `cd web/viewer && npm run test -- --runInBand __tests__/viewer-api.test.ts`
- ✅ `cd web/viewer && npm run test -- --runInBand --testPathPattern=viewer-api`
