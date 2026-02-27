# PLAN

## Scope (current change)
- Update production messaging to describe BitRiver Live as production-capable for operator-managed single-host deployments without overstating guarantees.
- Add a dedicated production status policy document at `docs/production-status.md` with clear lifecycle definitions (`dev`, `beta`, `rc`, `v1.0`) and guarantee boundaries.
- Update release note templating so every release captures upgrade notes, breaking changes, and an explicit operator checklist.
- Ensure docs cross-link required operator references: `docs/production-single-host.md`, `docs/upgrades.md`, `docs/security.md`, and `docs/monitoring.md`.

## Assumptions
- This is a documentation-only change; no runtime or deployment-contract files are modified.
- Existing repo language around single-host Docker Compose remains authoritative and should be reused where possible.
- The current release template location is `.github/RELEASE_NOTES_TEMPLATE.md`.

## Risks
- Messaging could imply stronger guarantees than currently documented if wording is too broad.
- Inconsistent lifecycle definitions between README and new production-status doc could confuse operators.
- Missing required doc links would fail acceptance criteria and reduce operator discoverability.

## Test plan
- Static docs QA by checking modified markdown for required sections/links.
- `./scripts/verify.sh`
