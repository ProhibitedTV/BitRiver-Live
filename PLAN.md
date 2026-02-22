# PLAN

## Scope (current change)
- Complete `deriveControlCentreStatus()` in `web/viewer/app/creator/live/[channelId]/page.tsx` so every known persisted backend `live_state` value is explicitly mapped.
- Remove the existing TODO about verifying additional persisted values.
- Add/extend viewer tests to cover each mapped known state plus an unknown-state fallback.

## Read-only analysis summary
- Backend storage validation currently allows persisted `live_state` values: `offline`, `live`, `starting`, and `ended`.
- Control-plane handlers and directory responses frequently use `offline`/`starting`/`live`; `ended` is also accepted by storage update paths and should be treated as known.
- The creator live page currently maps `starting`, `live`, and `offline`, while unknown values (including `ended`) fall through to a generic error branch and TODO.

## Assumptions
- No deployment contract/runtime orchestration files are affected (`deploy/docker-compose.yml`, root `.env`, generated OME config unchanged).
- This change is viewer-only behavior and test coverage; no backend API contract changes are required.

## Implementation approach
1. Update `deriveControlCentreStatus()` with explicit handling for `ended` and refine fallback wording for unexpected server values.
2. Remove TODO once mapping is complete.
3. Add focused unit tests around `deriveControlCentreStatus()` covering `offline`, `starting`, `live`, `ended`, and unknown input.
4. Run viewer-relevant checks and record outcomes in `TASKS.md` after each task.

## Risks
- Status messaging regressions if `offline` conditional branches are unintentionally altered.
- Test brittleness if assertions over-couple to non-essential text.

## Test plan
- Viewer unit tests for stream-status derivation mapping.
- Repo-required check: run `./scripts/verify.sh` before handoff.
