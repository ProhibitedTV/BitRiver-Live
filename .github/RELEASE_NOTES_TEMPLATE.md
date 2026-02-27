# BitRiver Live vX.Y.Z release notes

## Summary
- Short overview of the release.

## Supported upgrade paths
- Supported from: `v?.?.?` (N-1 minor / no major skipping policy applies)
- Target: `vX.Y.Z`

## Upgrade notes
1. Run `go run ./cmd/bitriver upgrade-plan --env-file .env --to vX.Y.Z --check-schema --current-schema <current_schema_version>`.
2. Required pre-upgrade backup steps completed (DB dump + volumes + `.env`).
3. Any release-specific operator actions.

## Breaking changes
- None

_or_

- [ ] Describe each breaking change with impacted modules/operators.
- [ ] Required mitigation or migration steps.

## Migration notes
- DB/schema changes:
- Data backfills:
- Estimated runtime impact:
- Reversibility (reversible / irreversible):

## Rollback notes
- Safe rollback conditions:
- Unsafe rollback conditions:
- Restore prerequisites:

## Verification checklist
- [ ] `deploy/check-env.sh`
- [ ] `./scripts/render-ome-config.sh --check`
- [ ] `go run ./cmd/bitriver verify --compose-file deploy/docker-compose.yml --env-file .env`
