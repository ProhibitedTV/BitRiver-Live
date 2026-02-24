# PLAN

## Scope (current change)
- Refactor `web/viewer/hooks/useAuth.tsx` `signOut` flow so auth state transitions are centralized via `loadViewer` instead of split between `finally` and `loadViewer`.
- Preserve current user-visible semantics for sign-out success/failure (error messaging and signed-out state behavior).
- Add/adjust viewer auth tests to lock sign-out UX semantics: signed-out user state, error handling, and loading indicator behavior during sign-out refresh.

## Assumptions
- `loadViewer` remains the source of truth for `user`, `loading`, and `error` transitions after sign-out requests.
- Existing components consuming `useAuth` should not require call-site changes.
- Jest/jsdom test environment can validate hook behavior through an `AuthProvider` test harness with mocked `fetch`.

## Risks
- Changing `signOut` sequencing could unintentionally hide errors or alter loading timing.
- Race conditions between initial `loadViewer` and sign-out-triggered `loadViewer` can make tests flaky if assertions are not synchronized.
- Refactor might regress unauthorized handling (401/403) if error normalization path changes.

## Test plan
- `cd web/viewer && npm run test -- --runInBand __tests__/useAuth.test.tsx`
- `cd web/viewer && npm run test -- --runInBand __tests__/navbar.test.tsx`
