# PLAN

## Scope (current change)
- Add a concurrency-focused auth hook test in `web/viewer/__tests__/useAuth.test.tsx`.
- Validate rapid double `signOut` invocation behavior while the first refresh (`/api/viewer/me`) is still pending.
- Assert user-visible final state (`auth-user`, `auth-loading`, `auth-error`) plus fetch call order/count to guard against duplicate refresh churn.

## Assumptions
- Existing `AuthProvider` behavior allows overlapping `signOut` calls when invoked in quick succession.
- Deterministic sequencing can be achieved with deferred promises for refresh responses.

## Risks
- Async race timing can make test flaky if deferred resolution ordering is not explicit.
- Overly strict network assertions could fail if unrelated request metadata differs.

## Test plan
- `cd web/viewer && npm run test -- useAuth.test.tsx`
