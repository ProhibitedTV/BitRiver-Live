# PLAN

## Scope (current change)
- Inspect `web/viewer/__tests__/channelPage.test.tsx` retry/load interactions around the failing warning location.
- Update async interaction/assertion flow to use awaited Testing Library patterns and remove React state update warnings.
- Re-run viewer test and lint commands to capture clean release evidence for this scoped fix.

## Assumptions
- The warning originates from un-awaited or non-`userEvent.setup()` interactions in channel page tests, not from runtime component logic.
- No functional behavior changes are required in `web/viewer/app/channels/[id]/page.tsx`.

## Risks
- Over-waiting or changing assertion timing could accidentally weaken regression coverage.
- Converting interaction helpers might change exact sequencing and require minor assertion timing updates.

## Test plan
- Run `npm --prefix web/viewer run test` after async test flow updates.
- Run `npm --prefix web/viewer run lint` after tests pass.
