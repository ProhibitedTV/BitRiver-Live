# PLAN

## Scope (current change)
- Add a reusable shell helper under `scripts/` for bounded polling that accepts interval and timeout values.
- Refactor `scripts/test-quickstart.sh` service health waiting to use the shared helper.
- Refactor `scripts/test-postgres.sh` postgres container readiness waiting to use the shared helper while preserving existing failure semantics.

## Assumptions
- Shared helper can be sourced from both scripts without changing their current shell options (`set -euo pipefail`).
- Existing readiness logic can be expressed as callback commands that return pass/retry/fail semantics.
- Current user-facing error messages should remain unchanged unless made more specific with the same meaning.

## Risks
- Refactoring loops into a helper may accidentally change timeout boundaries or retry cadence.
- `set -e` behavior in callback commands can cause early exits if not handled carefully.
- Polling helper API might be too rigid for existing scripts unless designed with explicit hooks.

## Test plan
- `bash -n scripts/polling.sh scripts/test-quickstart.sh scripts/test-postgres.sh`
- `shellcheck scripts/polling.sh scripts/test-quickstart.sh scripts/test-postgres.sh`
- `./scripts/test-postgres.sh ./internal/storage/...` (may fail if Docker unavailable; still validates flow attempt)
- `./scripts/test-quickstart.sh` (may fail if Docker unavailable; still validates flow attempt)
