# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Refactor `useAuth` sign-out state flow
  - Acceptance criteria:
    - `signOut` routes post-request auth state updates through one consistent path.
    - `loadViewer` is the single source of truth for loading/user transitions after sign-out.
    - Success/failure UX semantics remain unchanged.
  - Relevant checks:
    - `cd web/viewer && npm run test -- --runInBand __tests__/navbar.test.tsx`

- [x] Task 2 — Add auth hook tests for sign-out semantics
  - Acceptance criteria:
    - Tests cover signed-out result after sign-out.
    - Tests cover sign-out failure error handling semantics.
    - Tests cover loading indicator behavior during sign-out-triggered refresh.
  - Relevant checks:
    - `cd web/viewer && npm run test -- --runInBand __tests__/useAuth.test.tsx`

- [x] Task 3 — Run related regression test(s) and finalize task log
  - Acceptance criteria:
    - Existing auth-consuming UI test(s) still pass.
    - Execution log includes all commands run and outcomes.
  - Relevant checks:
    - `cd web/viewer && npm run test -- --runInBand __tests__/navbar.test.tsx`

## Execution log

- ✅ `cd web/viewer && npm run test -- --runInBand __tests__/navbar.test.tsx`
- ✅ `cd web/viewer && npm run test -- --runInBand __tests__/useAuth.test.tsx`
- ✅ `cd web/viewer && npm run test -- --runInBand __tests__/navbar.test.tsx`
- ❌ `./scripts/verify.sh` (fails on pre-existing viewer snapshot mismatch in `__tests__/channelDisplayPrimitives.test.tsx`)
