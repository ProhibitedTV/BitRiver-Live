# `docs/` Guidance

Product/deployment documentation lives here. Follow the root `AGENTS.md` for repo-wide requirements.

## Expectations
- Keep guides aligned with the promises in `README.md` (quickstart, one-command deployment, tooling). When commands, flags, or workflows change, update the relevant doc in the same PR.
- Keep every production/deployment guide tied to the single path of `deploy/docker-compose.yml` plus the repo-root `.env` (validated by `deploy/check-env.sh` and rendered via `scripts/render-ome-config.sh`). If you mention Go CLI shims, make it clear they delegate to the same Compose flow so readers do not pick up divergent runbooks.
- Link new documents from `README.md` or existing docs so they stay discoverable.
- Testing instructions (`docs/testing.md`) define the canonical commands; update them whenever CI expectations shift.
- `docs/architecture.md` defines the rigid architecture contract and dependency direction; keep it in sync with `AGENTS.md` whenever package boundaries or layering rules change.

## Before opening a PR
- Proofread for copy/paste-ready commands (prefix with `$` only when necessary, keep shell blocks executable).
- Verify that referenced files/paths exist after your change.
- Run markdown linting if you have it configured locally.
