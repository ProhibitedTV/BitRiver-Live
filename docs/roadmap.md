# BitRiver Live Roadmap

_Last reviewed: 2026-07-17_

This is the living delivery roadmap for BitRiver Live. It translates the supported product boundary into an execution order. It is not a promise that every future idea will ship on a fixed date.

For release-control details, use the active GitHub release epic and the release-gate documentation. When this roadmap, an issue, and the code disagree, the code and the active release epic are the immediate sources of truth and the roadmap should be corrected.

## Product boundary

BitRiver Live is currently aimed at operators who want to run a self-hosted live-streaming service on one managed host.

The supported baseline is:

- Go API and admin control plane
- Next.js public viewer
- SRS ingest
- FFmpeg-based transcoding
- OvenMediaEngine playback
- PostgreSQL and Redis
- the repository-root `.env` plus `deploy/docker-compose.yml` deployment contract
- a 1080p-oriented single-host operating model

The roadmap does not treat Kubernetes, multi-region delivery, managed hosting, hands-off high availability, unlimited scale, or a formal 4K guarantee as part of the first stable release.

## Roadmap principles

1. **Prove the operator path before expanding the feature surface.** Install, stream, upgrade, roll back, restore, monitor, and verify releases before claiming production readiness.
2. **Support one honest baseline.** New deployment topologies remain experimental until they have repeatable evidence and an explicit support policy.
3. **Build once and promote verified artifacts.** Stable releases must be tied to immutable digests, checksums, provenance, and the exact source commit that passed the gates.
4. **Keep release work and feature work separate.** Product improvements do not block a stable release unless they repair a regression in the supported baseline.
5. **Prefer evidence over checklists.** Closing a roadmap item requires reproducible test output, runbooks, or release artifacts—not only an implementation claim.
6. **Avoid duplicate work.** New issues should link to an active parent epic and either extend, replace, or explicitly defer existing tickets.

## Now: v1.2.3 — first stable operator release

The active source of truth is GitHub issue **#1293**.

### Completed foundation

- [x] #1294 — remove plaintext production secrets from release artifacts
- [x] #1295 — move Go and Next.js onto supported production baselines
- [x] establish the canonical single-host deployment contract and source-based quickstart
- [x] document the release-gate ladder and add initial contract, smoke, scorecard, and canary gates
- [x] ship the current BitRiver network-console viewer identity and live-room chat foundation

### Release blockers

These must close before v1.2.3 can be promoted as the first stable release:

- [ ] #1296 — migration ledger, checksum drift detection, and rollback policy
- [ ] #1297 — clean-host installation from published artifacts
- [ ] #1298 — stateful upgrade and rollback rehearsal
- [ ] #1299 — automated backup, restore, and disaster-recovery proof
- [ ] #1300 — full-stack production golden-path end-to-end test
- [ ] #1301 — build-once promotion of immutable release digests
- [ ] #1302 — one deterministic required merge gate and stable-promotion gate

The release-gate work in #1264, #1270, and #1271 should converge into those blockers rather than create a second release process.

### Production qualification

A release candidate must also pass or explicitly reduce scope around:

- [ ] #1303 — measured single-host capacity envelope
- [ ] #1304 — restart, dependency-failure, and resource-pressure resilience
- [ ] #1305 — service objectives, alert rules, and alert delivery
- [ ] #1306 — release-focused security and abuse review
- [ ] #1307 — browser, device, and playback compatibility matrix

### v1.2.3 exit criteria

v1.2.3 is ready only when an operator can use published artifacts to:

1. install on every claimed supported host;
2. bootstrap an administrator and validate authentication/session behavior;
3. publish a deterministic RTMP stream and watch it through the public viewer;
4. exercise chat and a short VOD/upload path;
5. upgrade a populated deployment and execute the documented rollback path;
6. restore required state from a verified backup;
7. operate within a published capacity envelope with actionable alerts; and
8. verify every released artifact against immutable digests, checksums, SBOMs, provenance, and the release source commit.

## Next: v1.2.x — stabilize the supported baseline

After the first stable release, the next work should reduce operator risk rather than immediately widen the architecture.

Priorities:

- patch security, compatibility, recovery, and operator-facing defects;
- keep clean-host install, stateful upgrade, rollback, restore, and golden-path tests continuously green;
- publish repeatable canary and release-evidence bundles for every stable tag;
- tighten documentation around supported operating systems, hardware, reverse proxies, TLS, storage, backups, and alert response;
- establish an explicit maintenance cadence for dependencies, vulnerability exceptions, and runtime baselines;
- improve diagnostics for ingest, transcoding, playback, chat, storage, and viewer failures;
- collect real operator feedback before changing the supported deployment contract.

A v1.2.x change must not silently broaden the support promise. New topology or protocol claims require a separately scoped roadmap item.

## Then: v1.3 — operator experience and core product maturity

Once the single-host release process is dependable, focus on making the product easier to operate and more complete for creators and viewers.

Candidate work:

- finish the ivlog-inspired live-room experience under #1272 without introducing a second chat stack;
- define and implement the chat deletion and `/me` event contract in #1291;
- improve stream-health, playback, rendition, and failure diagnostics for operators and creators;
- strengthen channel discovery, profile, scheduling, moderation, VOD, upload, and analytics workflows already present in the product model;
- reduce admin friction around stream keys, channel setup, user roles, moderation actions, and recovery tasks;
- add accessibility, mobile, and browser coverage as part of feature acceptance rather than as after-the-fact cleanup;
- make public API and event contracts explicit before adding more clients or integrations.

The v1.3 release should remain deployable through the same supported single-host contract unless a separate compatibility plan is approved.

## Later: v1.4 — optional high-end media capabilities

4K and broader hardware acceleration remain opt-in work until the 1080p baseline is stable and measured.

Related issues:

- #1276 — 4K ingest, transcoding, and playback profiles
- #1277 — viewer quality selection and playback diagnostics
- #1278 — 4K end-to-end smoke test and operator runbook

Before claiming formal 4K support, the project must publish:

- tested input and rendition profiles;
- CPU/GPU, memory, storage, and bandwidth expectations;
- supported encoder paths;
- viewer compatibility and quality-selection behavior;
- load, thermal, and failure evidence; and
- a repeatable operator runbook.

Parts of #1277 that improve ordinary 1080p diagnostics may ship earlier, but a 2160p support claim must remain gated by the complete evidence set.

## Future: v2.0 — deployment expansion

Multi-host or highly available operation should begin only after the single-host baseline has stable releases and real operator evidence.

Possible v2 work:

- dedicated ingest, transcode, playback, and edge roles;
- external object storage and origin/edge separation;
- worker scheduling and capacity-aware placement;
- database and Redis availability strategies;
- multi-node upgrades, rollback, backup, and disaster recovery;
- deployment automation for a clearly selected production topology;
- federation, integrations, or additional clients built on versioned public contracts.

Kubernetes, Helm, Terraform, multi-region delivery, CDN design, and managed-service ideas belong here only after the project chooses a concrete support target. They are not default requirements merely because they are common at larger platforms.

## Explicitly deferred

The following do not block the current stable roadmap:

- managed BitRiver hosting or SaaS operations;
- hands-off autoscaling and high availability;
- multi-region or global CDN guarantees;
- formal 4K support;
- subscriptions, custodial payments, or a platform-operated financial layer;
- broad protocol expansion without a tested operator use case;
- a Kubernetes-first rewrite of the supported deployment path.

## Roadmap maintenance

- Keep one active release epic for the release currently being promoted.
- Mark work as **Now**, **Next**, **Later**, or **Future** in issue bodies or project views.
- Link every implementation issue to a parent epic or roadmap section.
- Record dependencies and an observable exit criterion in every release-blocking issue.
- Close or supersede duplicate tickets instead of allowing parallel implementations of the same contract.
- Review this document after every stable release, support-boundary change, or major architecture decision.
