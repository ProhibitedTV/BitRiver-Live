# PLAN

## Scope (current change)
- Update `scripts/verify.sh` `viewer_changes_present()` to preserve CI SHA-based diff behavior.
- Improve local/dev fallback order: detect `web/viewer` changes from a merge-base diff first (prefer `origin/main` when available, then `HEAD~1`) before final `git status` fallback.
- Keep existing override behavior unchanged (`--viewer`, `--ci-viewer`).
- Extend script-level verification to deterministically prove committed `web/viewer` changes trigger viewer checks in local mode.

## Assumptions
- `origin/main` may not exist in all local clones; fallback logic must not error when missing.
- `HEAD~1` exists for most active branches but may not in single-commit repos; logic should degrade safely.
- Existing CI detection via base/head SHAs is already correct and should remain first-priority when metadata is provided.

## Risks
- Incorrect merge-base computation could over-trigger viewer checks.
- Diffing against unavailable refs may cause false negatives if fallback sequence is brittle.
- Tests that depend on branch names/history can be flaky unless isolated in temporary repos.

## Test plan
- `bash scripts/test-verify-viewer-detection.sh`
- `./scripts/verify.sh --go-packages ./cmd/bitriver`
