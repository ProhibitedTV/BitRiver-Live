# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add production status messaging to `README.md`
  - Acceptance criteria:
    - Adds a short “Production status” section.
    - Explicitly states production-capable scope today: operator-managed single-host deployments.
    - Explicitly states recommended/optional overlays: monitoring and runtime limits.
    - Explicitly states roadmap scope: HA/multi-host is not current GA behavior.
    - Includes links to `docs/production-single-host.md`, `docs/upgrades.md`, `docs/security.md`, `docs/monitoring.md`.

- [x] Task 2 — Add lifecycle guarantees doc `docs/production-status.md`
  - Acceptance criteria:
    - Defines `dev`, `beta`, `rc`, and `v1.0` levels clearly.
    - States what is guaranteed at each level in operator-facing language.
    - Includes links to `docs/production-single-host.md`, `docs/upgrades.md`, `docs/security.md`, `docs/monitoring.md`.

- [x] Task 3 — Update GitHub release template for operator-facing releases
  - Acceptance criteria:
    - Release template contains explicit sections for Upgrade notes, Breaking changes, and Operator checklist.
    - Operator checklist aligns with current docs/runbooks and avoids overpromising.

## Execution log
- ✅ Task 1 complete: added a concise `README.md` Production status section covering current production-capable scope, optional/recommended overlays, roadmap boundary, and required doc links.
- ✅ Task 1 check:
  - `rg -n "## Production status|operator-managed|compose.monitoring|compose.limits|docs/production-single-host.md|docs/upgrades.md|docs/security.md|docs/monitoring.md|docs/production-status.md" README.md`

- ✅ Task 2 complete: added `docs/production-status.md` with lifecycle definitions (`dev`, `beta`, `rc`, `v1.0`), per-level guarantees, and operator-facing boundaries.
- ✅ Task 2 check:
  - `rg -n "^# Production Status Policy|^### dev|^### beta|^### rc|^### v1.0|guaranteed|production-capable|production-single-host|upgrades|security|monitoring" docs/production-status.md`

- ✅ Task 3 complete: updated `.github/RELEASE_NOTES_TEMPLATE.md` with explicit Upgrade notes, Breaking changes, and Operator checklist sections linked to key operator docs.
- ✅ Task 3 check:
  - `rg -n "## Upgrade notes|## Breaking changes|## Operator checklist|docs/production-single-host.md|docs/upgrades.md|docs/security.md|docs/monitoring.md|HA/multi-host" .github/RELEASE_NOTES_TEMPLATE.md`


- ⚠️ Repo gate check (post-tasks):
  - `./scripts/verify.sh`
  - Result: failed in existing env placeholder hygiene check (`BITRIVER_LIVE_ADMIN_PASSWORD` sample marker requirement in `deploy/.env.example`), unrelated to this docs-only change.
