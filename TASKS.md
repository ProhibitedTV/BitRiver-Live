# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Analyze failing viewer primitive tests and related components (read-only)
  - Acceptance criteria:
    - Reviewed `channelDisplayPrimitives` test + snapshot.
    - Reviewed `DirectoryGrid`, `LiveNowGrid`, `ChannelRail`, `FeaturedChannel`, and `ChannelStatusBadge`.
    - Decision recorded: intentional UX change vs regression.

- [x] Task 2 — Apply minimal fix aligned with product intent
  - Acceptance criteria:
    - If intentional behavior: update snapshot and brittle assertions.
    - If regression: fix component output and align snapshot expectations.

- [x] Task 3 — Run viewer checks and record outcomes
  - Acceptance criteria:
    - `npm --prefix web/viewer run test -- channelDisplayPrimitives.test.tsx` executed.
    - `npm --prefix web/viewer run lint` executed.
    - `npm --prefix web/viewer run test` executed.
    - Results logged in execution log.

## Execution log
- ✅ Read-only analysis completed across test/snapshot and related components; mismatch isolated to `FeaturedChannel` CTA copy/structure (now single primary action: “View stream” with aria-label “View featured channel”), indicating intentional UX change to simplified action set.

- ✅ `npm --prefix web/viewer run test -- channelDisplayPrimitives.test.tsx -u` (pass; updated featured CTA snapshot to reflect intentional single-action UX copy).
- ✅ `npm --prefix web/viewer run lint` (pass).
- ✅ `npm --prefix web/viewer run test` (pass; suite emits existing React act() warnings in unrelated tests).
