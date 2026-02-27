# PLAN

## Scope (current change)
- Update `docs/production-release.md` in the "Viewer lint and integration tests" section to use `npm ci` instead of `npm install` while keeping `npm run lint` and `npm run test:integration` unchanged.
- Add a short release note in that section clarifying that `npm ci` is required for lockfile-faithful release validation and must be run from `web/viewer`.
- Audit other release/testing docs for viewer install commands and align release-reproducibility guidance to `npm ci` where appropriate.

## Assumptions
- `npm ci` is valid for the viewer workspace because `web/viewer/package-lock.json` exists and is the lockfile source of truth.
- "Release/testing docs" in scope are Markdown docs under `docs/` that describe release gates or testing workflows.

## Risks
- Changing install commands in general developer docs could unintentionally affect local iterative workflows; only update contexts where reproducibility is explicitly expected.
- Missing a second release/testing doc would leave inconsistent guidance.

## Test plan
- Run targeted searches for viewer install commands in release/testing docs and confirm intended commands now use `npm ci`.
- Review updated sections to ensure `npm run lint` and `npm run test:integration` lines are unchanged.
