# PLAN

## Scope (current change)
- Replace the hardcoded `Last state change reason: TODO: verify in code` text in `web/static/app.js` stream cards with data-driven messaging.
- Derive the reason from `channel.liveState`, `channel.currentSessionId`, and the computed `latestSession`.
- Align static dashboard messaging with creator UI stream-state semantics where they overlap (ended normally, ingest lost, unexpected state).
- Add/update static-app tests to prove the reason line is no longer hardcoded TODO text.

## Assumptions
- This is a UI logic and test change scoped to `web/static/*`; no deployment contract files are impacted.
- Existing creator UI semantics in `web/viewer/app/creator/live/[channelId]/page.tsx` are the canonical source for reason mapping language.

## Risks
- Semantic drift if static copy differs too much from creator control centre messages.
- Incomplete fallback handling if sessions are missing or inconsistent.
- Test brittleness if assertions depend on full card DOM shape instead of focused reason text.

## Test plan
- Run focused static app unit tests: `node --test web/static/app.test.mjs`.
- Run repository validation gate per policy: `./scripts/verify.sh`.
- Record task-level check results in `TASKS.md` after each task.
