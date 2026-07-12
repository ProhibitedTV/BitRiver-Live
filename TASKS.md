## Scoped change: BitRiver network-console viewer identity

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Establish visual and UX scope
  - Acceptance criteria:
    - `PLAN.md` records the visual direction, boundaries, risks, and test plan.
    - Current viewer behavior, UI contracts, screenshots, responsive surface, and production roadmap are reviewed before source edits.
  - Check:
    - Read-only source review and local desktop screenshot completed.

- [x] Task 2 - Build the shared network-console visual system
  - Acceptance criteria:
    - Dark and light tokens use the BitRiver gold/cyan/red status palette with high contrast.
    - Navigation, buttons, inputs, surfaces, chips, and state panels use restrained square geometry and visible focus treatment.
    - Existing navigation/auth behavior remains unchanged.
  - Check:
    - `npm.cmd --prefix web/viewer run test -- navbar viewerShell` passed (37 tests).

- [x] Task 3 - Recompose homepage discovery and empty-install states
  - Acceptance criteria:
    - Homepage hierarchy reads as a live network surface rather than a marketing hero.
    - Fresh installs show clear network-ready status, useful next actions, and compact discovery sections.
    - Live, loading, empty, search, and error behavior remains represented.
  - Check:
    - `npm.cmd --prefix web/viewer run test -- directoryPage` passed (14 tests; existing search interaction `act()` warnings remain).

- [x] Task 4 - Document the viewer identity contract
  - Acceptance criteria:
    - Viewer docs describe the shared ecosystem visual language and the public-viewer restraint rules.
    - No deployment contract docs change.
  - Check:
    - `git diff --check` passed after removing one trailing blank line from `PLAN.md`.

- [x] Task 5 - Verify responsive behavior and release readiness
  - Acceptance criteria:
    - Focused tests, lint, and production build pass or blockers are recorded.
    - Desktop and mobile browser checks show no horizontal overflow or clipped critical actions.
    - The canonical viewer verify gate is attempted and any host blocker is recorded.
  - Check:
    - `npm.cmd --prefix web/viewer run test -- directoryPage navbar viewerShell` passed (51 tests).
    - `npm.cmd --prefix web/viewer run lint` passed with one pre-existing `UploadManager` hook dependency warning.
    - `npm.cmd --prefix web/viewer run build` passed with the same hook warning and existing client-rendering notices.
    - Desktop 1440x1000, mobile 390x844, and mobile light-theme browser checks passed with no horizontal overflow.
    - `git diff --check` passed.
    - Git Bash reached `./scripts/verify.sh --viewer`, passed go.sum and CI contract checks, then stopped because no Python 3 interpreter is installed on this host.

- [-] Task 6 - Correct light-theme active-chip contrast and merge
  - Acceptance criteria:
    - Active chips use a deterministic background/text pair that meets WCAG AA in dark and light themes.
    - `tests/accessibility.spec.ts` passes locally.
    - PR #1308 Viewer CI and all required merge gates pass.
    - PR #1308 is squash-merged into `main` only after green CI.
  - Check:
    - Viewer CI failure confirmed at 1.18:1 contrast for active browse chips; focused fix approved by the user.
    - First local accessibility rerun cleared the chip violation and exposed light-theme hovered primary actions at 2.69:1; correction remains in Task 6 scope.
    - `npm.cmd --prefix web/viewer run test:playwright -- tests/accessibility.spec.ts` passed (2 tests) after both contrast pairs were corrected.
    - `npm.cmd --prefix web/viewer run test -- directoryPage navbar viewerShell` passed (51 tests; existing search `act()` warnings and build-output haste warning remain).
    - `git diff --check` passed.

### Execution log
- Task 1 review:
  - Compared the supplied BitRiver Radio and Visual Archive screenshots against the running viewer homepage.
  - Confirmed the viewer currently uses large rounded panels, gradient CTAs, and generic SaaS spacing that does not match the ecosystem identity.
  - Reviewed `docs/ui-ux-model.md`, `web/viewer/README.md`, the public shell/navigation components, homepage composition, CSS tokens, and open production roadmap issues.
  - Kept production epic #1293 and compatibility issue #1307 as release context; this user-directed visual pass does not claim to close either issue.
- Task 2 implementation:
  - Added a late-loaded `network-console.css` identity layer so existing component behavior and older route-specific styles remain intact.
  - Replaced generic gradients and rounded chrome with gold structural lines, cyan live/focus states, red error states, compact square controls, and monospace operational metadata.
  - Preserved the light theme with an adapted high-contrast paper-console palette.
- Task 3 implementation:
  - Added a real-data network status rail for node identity, live-signal count, directory count, and relay state.
  - Reworked the homepage hierarchy into a public relay surface with a stronger first-stream state and a compact featured-signal panel.
  - Added a regression test proving an empty installation presents itself as ready and actionable rather than broken.
- Task 4 documentation:
  - Added the network-console visual identity contract and public-viewer restraint rules to `web/viewer/README.md`.
  - Added ecosystem identity and truthful visual status cues to the cross-role UX principles in `docs/ui-ux-model.md`.
- Task 5 verification:
  - Confirmed desktop and mobile layouts keep critical status, setup, search, and featured-relay actions visible without horizontal overflow.
  - Confirmed the retained light theme applies the documented paper-console palette and remains overflow-free.
  - Focused tests, lint, production build, and diff hygiene passed; canonical verification remains host-blocked at its Python prerequisite.
- Task 6 CI diagnosis:
  - Viewer CI ran 36 Playwright integration tests; 35 passed and the directory accessibility test failed.
  - Axe identified active `Live` and `All` browse chips using `#f7ffff` text over a translucent `#daefe5` light-theme composite at 1.18:1 contrast.
  - The first local rerun confirmed the chip fix and then identified `#080601` text on hovered `#6f4f08` primary actions at 2.69:1.
  - Active chips now use an opaque accent/contrast pair, and light-theme primary actions use a light foreground across normal, hover, and focus states.
  - The exact accessibility spec and focused Jest regression set pass locally.
