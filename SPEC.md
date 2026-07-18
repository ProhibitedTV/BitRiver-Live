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
- A source-free Ubuntu 24.04 LTS x86_64 host can install the canonical pull-only stack from a tagged launcher archive or Linux package.
- Release bundles contain every Compose bind mount, migration, renderer/template, proxy asset, wrapper, service definition, and operator document required at runtime.
- Program assets, configuration, and durable data have separate ownership/lifecycle boundaries; uninstall retains operator data unless an explicit destructive flag is supplied.
- First activation performs actionable prerequisite/resource/port/DNS/permission checks, generates production secrets, validates the environment, applies migrations, and waits for bounded service health.
- The OME renderer is a published multi-architecture release image; a clean host never depends on a locally built `ome-config:local` image or a Go/source checkout.
- The stack is enabled for reboot recovery through systemd and exposes status, logs, upgrade, and safe removal commands.
- XOA/XCP-ng and Nginx Proxy Manager documentation distinguishes HTTP(S)/WebSocket proxying from direct media/firewall ports and internal-only control services.
- Automated evidence covers release-shaped paths containing spaces, rerunnable installation, package contents, safe uninstall, and failure diagnostics without overstating Debian, ARM64, reboot, or real playback evidence.
