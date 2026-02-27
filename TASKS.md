# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Update production release viewer install command
  - Acceptance criteria:
    - `docs/production-release.md` replaces `npm install` with `npm ci` in "Viewer lint and integration tests".
    - `npm run lint` and `npm run test:integration` remain unchanged in that block.
    - A short note states `npm ci` is required for lockfile-faithful release validation and should be run from `web/viewer`.

- [x] Task 2 — Align other release/testing docs for reproducible viewer installs
  - Acceptance criteria:
    - Any additional release/testing doc with viewer install command(s) for reproducible validation is updated from `npm install` to `npm ci`.
    - Non-release/non-reproducibility guidance is left unchanged.

- [x] Task 3 — Verify documentation consistency
  - Acceptance criteria:
    - Command-based checks show targeted sections use `npm ci`.
    - A search confirms no remaining `npm install` in release/testing docs where reproducibility is expected.

## Execution log
- ✅ Task 1 complete: updated `docs/production-release.md` viewer quality-gate section to use `npm ci` and added lockfile-faithful note scoped to `web/viewer`.
- ✅ Task 1 check: `rg -n "Viewer lint and integration tests|npm ci|npm run lint|npm run test:integration" docs/production-release.md`.
- ✅ Task 2 complete: aligned `docs/testing.md` Web viewer install command to `npm ci` for reproducible testing setup.
- ✅ Task 2 check: `rg -n "Web viewer|npm ci|npm install|test:integration" docs/testing.md`.
- ✅ Task 3 complete: validated release/testing docs now use `npm ci`; remaining `npm install` mentions in docs are non-install-required statements.
- ✅ Task 3 checks:
  - `rg -n "npm install|npm ci" docs/production-release.md docs/testing.md`
  - `rg -n "npm install" docs -g '*.md'`
