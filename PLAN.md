# PLAN

## Scope (current change)
- Create a new operator-facing security guide at `docs/security.md` as the primary security entrypoint for this stack.
- Tailor guidance to the current deployment contract and validation flow (`deploy/docker-compose.yml`, `deploy/.env.example`, `cmd/bitriver env validate`).
- Add prominent navigation links so operators discover the security guide from high-traffic docs (`README.md` and/or `docs/operations.md`).

## Assumptions
- Security guidance should match the current single-host Docker Compose deployment model and documented optional profiles.
- This change is documentation-only and does not alter runtime behavior or deployment contract values.
- `_FILE`-based secrets are not yet implemented in the current stack, so guidance should mention future compatibility without claiming support exists today.

## Risks
- Overstating default exposure levels for ports/services could mislead operators if wording is not anchored to current compose defaults.
- Security guidance can drift from env validation behavior if we cite controls not enforced by `cmd/bitriver env validate`.
- If the new doc is not linked clearly, operators may continue to miss security hardening steps.

## Test plan
- Markdown/documentation lint via static review for section completeness and command correctness.
- `./scripts/verify.sh`
