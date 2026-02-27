# Production Status Policy

This document defines how BitRiver Live uses release-stage labels and what operators can expect at each stage.

BitRiver Live can be promoted as **production-capable for operator-managed, single-host deployments** when run on the documented Docker Compose contract.

## Status levels

### dev

Use for active development snapshots and feature prototyping.

What is guaranteed:
- Basic build/test workflows are expected to work for contributors.
- Core docs and interfaces may change without backward-compatibility guarantees.

What is not guaranteed:
- Upgrade compatibility between dev snapshots.
- Stable operational runbooks for production maintenance windows.
- Complete hardening/performance guidance.

### beta

Use when a release is feature-complete enough for broad operator evaluation.

What is guaranteed:
- Documented core workflows should run end-to-end on supported single-host setups.
- Release notes call out known risks and operator-impacting caveats.
- Breaking changes are communicated before stable release.

What is not guaranteed:
- Zero-breaking-change upgrades across beta builds.
- Full operational SLO maturity for every topology.

### rc (release candidate)

Use for final validation before a stable tag.

What is guaranteed:
- Intended stable feature set is frozen except for release-blocking fixes.
- Upgrade and rollback steps are documented and validated for the candidate.
- Operator runbooks are expected to reflect release behavior.

What is not guaranteed:
- No last-minute changes; critical fixes can still land.
- HA/multi-host general availability.

### v1.0

Use for the first stable release with production positioning.

What is guaranteed:
- Production-capable scope is clearly defined as **operator-managed single-host deployments**.
- Upgrade guidance and release notes are maintained for stable versions.
- Security and monitoring runbooks exist for operators.
- Breaking changes are explicitly documented in release messaging.

What remains outside the v1.0 guarantee:
- HA/multi-host deployment guarantees.
- Fully managed operations by the project team.

## Operator-facing interpretation

For production use today, treat the supported baseline as:
- One host running the canonical compose contract.
- Operators responsible for host hardening, backups/restores, and maintenance execution.
- Optional overlays (monitoring and runtime limits) adopted based on your risk profile and capacity planning.

## Related operator docs

- Single-host production baseline: [`docs/production-single-host.md`](production-single-host.md)
- Upgrade planning and execution: [`docs/upgrades.md`](upgrades.md)
- Security hardening guidance: [`docs/security.md`](security.md)
- Monitoring and alerting overlays: [`docs/monitoring.md`](monitoring.md)
