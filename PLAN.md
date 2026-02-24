# PLAN

## Scope (current change)
- Refactor `web/viewer/lib/viewer-api.ts` into internal domain-focused submodules under `web/viewer/lib/` (auth/channel/chat/directory/profile/uploads/metrics style split).
- Keep `web/viewer/lib/viewer-api.ts` as the stable public facade, re-exporting the same types/functions without caller changes.
- Move implementation in small slices to reduce risk and preserve blame.

## Assumptions
- Existing callers should continue importing only from `viewer-api.ts`.
- Internal-only helpers can be relocated to private modules and re-exported from the facade.
- Jest coverage around viewer API request helpers is sufficient to catch regressions for this refactor.

## Risks
- Accidentally changing exported symbol names/order or introducing circular dependencies.
- Moving shared helpers (`viewerRequest`, `multipartRequest`, `ViewerApiError`) could alter runtime behavior if types/imports drift.
- Domain split may miss a consumer if a type is no longer re-exported.

## Test plan
- `cd web/viewer && npm run test -- --runInBand __tests__/viewer-api.test.ts`
- `cd web/viewer && npm run test -- --runInBand --testPathPattern=viewer-api`
