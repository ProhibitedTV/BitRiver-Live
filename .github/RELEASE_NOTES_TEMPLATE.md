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
- Previous signed stable release-set/tag/digests (or explicitly none for the first stable release):

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
- [ ] `release-set.json` and its Sigstore bundle verify against the exact candidate tag workflow identity.
- [ ] The signed manifest records all five exact image digests, SBOMs/signatures, dependency pins, and sanitized gate evidence.
- [ ] Downloaded package/archive defaults reference the exact release tag for all five first-party images.
- [ ] Pull-only tagged product evidence covers real ingest, decoded playback, offline transition, chat/moderation, and VOD.
- [ ] Stable only: a tracked promotion record binds every required durable evidence hash to this candidate release-set hash.
- [ ] Stable only: `Stable promotion gate` passed before environment review, no revocation marker exists, and candidate assets were copied byte-for-byte without rebuild.
- [ ] Stable only: stable aliases resolve to the recorded digests; any `latest` move is explicit and non-authoritative.
