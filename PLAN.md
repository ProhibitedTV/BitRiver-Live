# PLAN

## Scope (current change)
- Fix OME XML validation in `scripts/test-quickstart.sh` so required-tag checks no longer contradict the legacy-bind guard.
- Keep rejection of legacy root `<Bind><IP>` / `<Bind><Address>` tags.
- Validate canonical root `<IP>` value against `BITRIVER_OME_BIND` when present.

## Assumptions
- Root-level `<IP>` is the canonical bind node in current generated OME config.
- Legacy `<Bind><IP>` / `<Bind><Address>` tags should remain forbidden.

## Risks
- If canonical bind location differs from current contract, validation may fail in environments with custom templates.
- Tightened bind-value checks may surface pre-existing env/config drift.

## Test plan
- `./scripts/test-quickstart.sh`
