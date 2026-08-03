# BitRiver Live vX.Y.Z release notes

## Summary
- Short overview of the release.

## Supported upgrade paths
- Supported from: `v?.?.?` (N-1 minor / no major skipping policy applies)
- Target: `vX.Y.Z`

## Upgrade notes
1. Run `go run ./cmd/bitriver upgrade-plan --compose-file deploy/docker-compose.yml --env-file .env --target vX.Y.Z`.
2. Complete required pre-upgrade backups (DB dump + volumes + `.env`).
3. Review [`docs/upgrades.md`](../docs/upgrades.md) and call out any release-specific steps/operators actions.

## Breaking changes
- None.

_or_

- [ ] Describe each breaking change and who is impacted (operators, API clients, viewer users).
- [ ] Provide required mitigation or migration steps.
- [ ] State whether downtime is expected.

## Migration notes
- DB/schema changes:
- Data backfills:
- Estimated runtime impact:
- Reversibility (reversible / irreversible):

## Rollback notes
- Safe rollback conditions:
- Unsafe rollback conditions:
- Restore prerequisites:

## Operator checklist
- [ ] Confirm single-host production baseline assumptions still hold for this release ([`docs/production-single-host.md`](../docs/production-single-host.md)).
- [ ] Confirm security-impacting changes and required operator actions are documented ([`docs/security.md`](../docs/security.md)).
- [ ] Confirm monitoring/alert updates are documented, including overlay changes if applicable ([`docs/monitoring.md`](../docs/monitoring.md)).
- [ ] Attach/confirm upgrade execution notes for operators ([`docs/upgrades.md`](../docs/upgrades.md)).
- [ ] Call out any features that remain roadmap-only (for example, HA/multi-host) instead of implying current GA support.

## Verification checklist
- [ ] `deploy/check-env.sh`
- [ ] `./scripts/render-ome-config.sh --check`
- [ ] `go run ./cmd/bitriver verify --compose-file deploy/docker-compose.yml --env-file .env`
- [ ] Published `CHECKSUMS.txt` covers every other release asset exactly once.
- [ ] Downloaded package/archive defaults reference the exact release tag for all five first-party images.
- [ ] Pull-only tagged product evidence covers real ingest, decoded playback, offline transition, chat/moderation, and VOD.
