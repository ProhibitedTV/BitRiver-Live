# PLAN

## Scope (current change)
- Investigate the `channelDisplayPrimitives` snapshot/test mismatch in the viewer test suite.
- Confirm intended UX copy for status badges, meta labels, and CTA text across shared channel display components.
- Apply the minimal fix: either restore regressed component output or update brittle test/snapshot expectations if UX copy was intentionally changed.
- Re-run viewer checks and capture outcomes in `TASKS.md`.

## Assumptions
- `channel-presenters.ts` and current component render output represent the canonical UX copy unless contradicted by product docs/tests.
- The snapshot mismatch is localized to viewer rendering primitives and can be validated via viewer lint/tests.

## Risks
- Snapshot updates can mask real regressions if component output diverged unintentionally.
- Text assertions may be too specific and break on harmless copy refinements.
- Viewer tests may take longer when running the full suite; targeted runs are needed for fast iteration.

## Test plan
- `npm --prefix web/viewer run test -- channelDisplayPrimitives.test.tsx`
- `npm --prefix web/viewer run lint`
- `npm --prefix web/viewer run test`
