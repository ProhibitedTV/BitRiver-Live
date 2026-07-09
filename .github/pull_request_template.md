## Summary

- What changed?
- Why was it needed?

## Testing

- [ ] `./scripts/verify.sh`
- [ ] Focused commands, if any:

## Release scorecard

### Change classification

- [ ] docs-only
- [ ] test-only
- [ ] build/CI
- [ ] viewer/UI
- [ ] API/control plane
- [ ] deployment/Compose/env
- [ ] auth/security
- [ ] data/migrations
- [ ] release packaging
- [ ] operator workflow

### Risk level

- [ ] low - docs, tests, or additive behavior with narrow blast radius
- [ ] medium - runtime behavior, operator workflow, config defaults, or release packaging
- [ ] high - auth/security, migrations, ports/volumes, credentials, data loss, or rollback-sensitive change

### Evidence map

- [ ] Unit/focused tests:
- [ ] Viewer lint/tests:
- [ ] `./scripts/verify.sh`:
- [ ] Compose/contract/release gate:
- [ ] Manual operator-path check:
- [ ] Docs/release notes:
- [ ] Blocked/skipped checks explained:

### Operator/release impact

- [ ] No operator-facing impact
- [ ] Docs updated
- [ ] Release notes/changelog follow-up needed
- [ ] Upgrade notes required
- [ ] Rollback/canary notes included

### Medium/high-risk review prompts

- Does this preserve the supported single-host Compose boundary?
- Does it change API shape, env vars, ports, volumes, credentials, defaults, or generated config?
- Does it require upgrade, rollback, canary, or operator runbook updates?
- Does it alter auth/security posture or release artifact provenance?

## Docs and contract impact

- [ ] No user-facing docs changes needed
- [ ] README or docs updated
- [ ] Deployment contract changed (`deploy/docker-compose.yml`, root `.env`, or generated OME expectations)
- [ ] Release notes / changelog follow-up needed

## Notes for reviewers

- Screenshots, migration notes, rollout caveats, or follow-ups.
