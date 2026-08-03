# Production Status Policy

This document defines how BitRiver Live uses release-stage labels and what operators can expect at each stage.

BitRiver Live can be promoted as **production-capable for operator-managed, single-host deployments** when run on the documented Docker Compose contract.

## Current public candidate

[`v1.2.3-rc.13`](https://github.com/ProhibitedTV/BitRiver-Live/releases/tag/v1.2.3-rc.13)
is the current prerelease. Its 46 public assets, package acceptance, five
signed anonymous image digests, pull-only eight-stage media/API product gate,
and signed release-set root passed at commit `d416968e` in release run
`30795492882`. The signed-root SHA-256 is
`795fffee84662aec91624eb4352b9c1a9ef5c34b17838939adaf567418797fa0`.

That is release-candidate evidence, not stable promotion. Clean installation on
the target Ubuntu/XOA VM, browser/media access through its real Nginx Proxy
Manager and firewall path, reboot recovery, and repeated OME recovery still
need operator evidence.

RC13 is the first candidate eligible for the protected stable-promotion
workflow. Eligibility is not approval: the tracked clean-host, backup/restore,
upgrade/rollback, capacity, resilience, SLO, security, and browser gates must
all bind durable evidence to this exact signed-root hash before stable bytes or
aliases may be created.

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
- Candidates from the current workflow are immutable signed release sets and
  never update stable or `latest` image aliases.

What is not guaranteed:
- No last-minute changes; critical fixes can still land.
- HA/multi-host general availability.

### stable

Use for stable public releases once the supported baseline and release process are in place.

What is guaranteed:
- Production-capable scope is clearly defined as **operator-managed single-host deployments**.
- Upgrade guidance and release notes are maintained for stable versions.
- Security and monitoring runbooks exist for operators.
- Breaking changes are explicitly documented in release messaging.
- The stable release copies one approved candidate byte-for-byte, retags exact
  image digests, and publishes signed stable/rollback metadata through a
  reviewed environment; it is not rebuilt from the stable tag.

What remains outside the stable guarantee:
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
