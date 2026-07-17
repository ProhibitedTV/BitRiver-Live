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
- PostgreSQL schema changes are recorded in a durable, queryable ledger with stable filenames, checksums, status, timestamps, and release provenance.
- Startup applies only pending migrations and refuses edited history, failed migrations, or ambiguous interrupted state.
- Operators can inspect a read-only migration plan/history and recover only through explicit checksum-confirmed commands and documented validation steps.
- Docker Compose and Helm execute the same canonical migration bytes in the same deterministic order.
- Fresh installs, upgrades from a representative previous schema, no-op reruns, drift refusal, and failure/interruption recovery have automated evidence.
- Upgrade and rollback docs define the forward-only policy and require backups plus release notes for destructive or rollback-incompatible changes.
- This work preserves the roadmap toward an Ubuntu 24.04 XOA VM installer behind Nginx Proxy Manager without claiming installer or OvenMediaEngine readiness is complete.
