# SPEC

## Product Goal
BitRiver Live is a self-hostable live streaming website for communities that need the core Twitch-style loop without depending on a hosted platform: account signup, creator channels, RTMP ingest, live playback, chat, moderation, and published past broadcasts.

## User Goals
- Operators can deploy the stack with the repository's Docker Compose contract and verify it without private chat context.
- Viewers can sign up, browse live channels, watch a stream, chat while signed in, follow creators, and report abusive chat.
- Creators can create a channel, schedule upcoming streams, find ingest credentials, go live from a common encoder such as OBS, monitor stream health, and publish recordings as VODs.
- Moderators and channel owners can review reports and take action on disruptive chat behavior.
- Contributors can execute scoped work through `SPEC.md` -> `PLAN.md` -> `TASKS.md` in order.

## Product Success Criteria
- A clean checkout can render the Compose contract and boot the canonical stack from the documented quickstart.
- Public self-signup works when enabled, including sign-in, sign-out, session expiry, and creator onboarding.
- A creator can create or manage a channel and retrieve RTMP ingest settings without editing deployment files.
- A creator can publish an upcoming stream schedule that appears on the public channel page.
- A real ingest session transitions the channel live, exposes playback to viewers, and returns offline after publish stops.
- Live chat loads, sends messages, updates in near real time, handles auth-required states, and supports viewer reports.
- Completed recordings stay private until a creator publishes them; published VODs appear on channel and Videos pages.
- Operational docs cover backups, restore, deploy smoke checks, and manual acceptance of the live streaming happy path.

## Contributor Success Criteria
- `PLAN.md` is updated during initial read-only analysis before implementation.
- `TASKS.md` lists small, reviewable tasks in execution order with status, acceptance criteria, and check results.
- After each completed task, the assignee runs relevant tests/checks and records status in `TASKS.md`.
- Runtime behavior, operator workflows, or public contracts are reflected in source-of-truth docs.
- Workflow docs remain short working artifacts.

## Current Change Success Criteria
- Only a valid `vMAJOR.MINOR.PATCH-PRERELEASE` tag can enter the build workflow; validation credentials remain generated, consumed, and destroyed inside one job.
- The candidate publishes public first-party images by exact tag and digest, then blocks GitHub prerelease creation until the anonymous pull-only Compose stack passes the scanner-approved full product path.
- Every public candidate artifact, SBOM, image signature, dependency/image manifest, and fixed-schema gate result is rooted in complete checksums plus a keylessly signed deterministic release-set manifest.
- Stable publication is a manual, environment-reviewed promotion of one existing candidate manifest. It performs no build, copies candidate artifacts byte-for-byte, and retags the five exact image digests.
- A tracked promotion record binds clean-host, backup/restore, upgrade/rollback, capacity, OME resilience, SLO, security, and viewer evidence to one candidate manifest hash; missing, open, revoked, or cross-candidate evidence fails before write approval.
- Stable promotion is retry-safe, publishes through a draft release, refuses conflicting tag/asset/alias state, emits signed stable and rollback roots, and treats `latest` as optional and non-authoritative.
- Candidate revocation is signed and append-only, cannot change stable state, and prevents later promotion.
- A release candidate is not described as stable or clean-host approved until Ubuntu/XOA install, Nginx Proxy Manager/browser playback, reboot/recovery, and the other tracked production gates genuinely pass.

## Previous Change Success Criteria - production golden path
- One release-blocking harness exercises the real canonical Compose services rather than mocked adapters or only the storage package.
- The harness creates real accounts and a creator channel, publishes deterministic 1080p RTMP with audio, observes the live/offline lifecycle, and proves OME plus transcoder playback is decodable and advancing.
- Authenticated chat send/history and an owner moderation action pass through the real API and Redis-backed chat path.
- A generated short VOD is uploaded, transcoded, published, listed for viewers, and media-probed through the supported playback surface.
- Health, readiness, status, viewer metadata, and media content assertions use bounded retries that identify the failed stage instead of hiding hangs.
- Evidence is machine-readable, includes timing and media probe results, excludes secrets, and passes a sentinel-based release-evidence scan on both success and failure.
- The same running-stack assertions can be reused by source CI, tagged pull-only release promotion, and clean Ubuntu/XOA acceptance.
- Build-mode Docker Desktop/CI proof is not described as tagged pull-only, browser recovery/quality, reboot, or repeated-run stability evidence until those direct gates pass.
