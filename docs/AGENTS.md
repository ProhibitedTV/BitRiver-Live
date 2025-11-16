# `docs/` Guidance

Product/deployment documentation lives here. Follow the root `AGENTS.md` for repo-wide requirements.

## Expectations
- Keep guides aligned with the promises in `README.md` (quickstart, one-command deployment, tooling). When commands, flags, or workflows change, update the relevant doc in the same PR.
- Link new documents from `README.md` or existing docs so they stay discoverable.
- Testing instructions (`docs/testing.md`) define the canonical commands; update them whenever CI expectations shift.

## Before opening a PR
- Proofread for copy/paste-ready commands (prefix with `$` only when necessary, keep shell blocks executable).
- Verify that referenced files/paths exist after your change.
- Run markdown linting if you have it configured locally.
