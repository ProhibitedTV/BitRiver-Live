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
- A Windows 11 operator can evaluate the source checkout with Docker Desktop's Linux-container backend and native PowerShell using copyable, tested commands.
- Docker/Compose prerequisites, canonical config rendering, startup, and health failures produce enough context to distinguish host setup problems from application failures.
- The README shows the real public viewer and operator surfaces captured from a running BitRiver Live stack, with no generated concept art or private data.
- README workflows accurately explain local evaluation, creator ingest, viewer playback, administration, and the single-host deployment boundary.
- GitHub Releases and downloadable packages are described as unavailable until a tag is actually published; the current source-checkout path is never hidden behind dead release links.
- Ubuntu 24.04 on an XOA VM behind Nginx Proxy Manager remains the production target without claiming clean-host, real ingest/playback, or OME recovery acceptance before those gates pass.
