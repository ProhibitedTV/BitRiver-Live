# PLAN

## Scope (current change)
- Strengthen the upgrade contract to v1.0-grade guidance in `docs/upgrades.md` with explicit supported version hops, backup/restore checklist, and rollback safety boundaries.
- Add a new CLI planner command (`bitriver upgrade-plan`) that reads deployed version hints from `.env` image tags, compares with a target tag, and prints actionable upgrade steps + breaking-change warnings.
- Add dedicated release versioning rules in `docs/versioning.md` and align release process docs/templates to require upgrade notes + breaking-change callouts.

## Assumptions
- Deployments use `deploy/docker-compose.yml` plus a repository `.env` where `BITRIVER_LIVE_IMAGE_TAG` is the canonical application version hint.
- DB schema version cannot always be auto-discovered, so schema checks should be opt-in and non-breaking when metadata is unavailable.
- Existing upgrade defaults must stay non-disruptive: no forced behavior changes at runtime.

## Risks
- Ambiguous semver parsing (with/without `v` prefixes) could misclassify supported hops.
- Overpromising rollback safety when migrations are irreversible could create operator risk.
- Release template/process changes may drift if not linked from existing release runbook.

## Test plan
- `go test ./cmd/bitriver -count=1`
- `./scripts/verify.sh`
