# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add rapid double-signOut test coverage
  - Acceptance criteria:
    - Render `AuthProvider` with `AuthHarness` and a test trigger that invokes `signOut` twice rapidly.
    - Use a mock fetch sequence with controlled delayed `/api/viewer/me` refresh responses.
    - Assert final user-visible state is stable and correct (`auth-loading=idle`, `auth-user` matches final refresh, `auth-error` semantics preserved).
  - Relevant checks:
    - `cd web/viewer && npm run test -- useAuth.test.tsx`

- [x] Task 2 — Add network call order/count assertions for churn protection
  - Acceptance criteria:
    - Verify expected fetch call count/order for init + two sign-out flows.
    - Ensure assertions focus on request intent (path/method) and catch unintended extra refresh loops.
  - Relevant checks:
    - `cd web/viewer && npm run test -- useAuth.test.tsx`

## Execution log
- ✅ `cd web/viewer && npm run test -- useAuth.test.tsx`
- ✅ `cd web/viewer && npm run test -- useAuth.test.tsx`
- ❌ `./scripts/verify.sh` (fails on existing snapshot mismatch in `web/viewer/__tests__/channelDisplayPrimitives.test.tsx` unrelated to this change)
