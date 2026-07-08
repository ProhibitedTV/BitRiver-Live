## Scope (current change)
- Address GitHub issue #1269: document the BitRiver Live release-gate ladder.
- Add a concise source-of-truth release-gates document that maps promotion gates to intent, timing, blocking status, commands/workflows, artifacts, and failure triage.
- Link the new guidance from existing release-facing docs so future gate implementation issues can reference it.
- Preserve the supported baseline: single-host, operator-managed, Docker Compose deployment with source quickstart and packaged launcher paths.

## Assumptions
- Documentation should lead the heavier release-gate implementation issues (#1265, #1266, #1267, #1268, #1270, #1271).
- Existing commands and workflows are the only commands that should be named as available; planned gates should be clearly marked as staged/future work.
- This pass is documentation-only and does not change runtime behavior, deployment contract files, CI behavior, schemas, or release artifacts.

## Risks
- Overstating planned gates as implemented could mislead operators and contributors.
- Adding another release document could fragment the release process unless it is linked from `docs/production-release.md` and README key docs.
- Release-gate guidance can become compliance-style noise if it is too long or vague.

## Test plan
- `git diff --check`
- Documentation review against issue #1269 acceptance criteria.
- No runtime tests required unless source or workflow files change.
