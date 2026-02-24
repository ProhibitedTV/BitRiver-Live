# PLAN

## Scope (current change)
- Update `scripts/verify.sh` so `viewer_changes_present()` detects viewer changes from CI commit metadata (base/head SHA range) when available.
- Preserve existing CLI override behavior for `--viewer` and `--ci-viewer`.
- Add script-level contract tests under `scripts/` that validate viewer change detection in representative git states.
- Verify viewer checks still only skip when viewer scope is truly absent.

## Assumptions
- CI metadata will be provided through common environment variables (e.g. GitHub base/head refs or SHAs) and may be absent in local runs.
- Local developer workflows should continue to rely on `git status --porcelain -- web/viewer` semantics.
- Contract checks can run without executing the full `./scripts/verify.sh` suite by unit-testing helper behavior in isolation.

## Risks
- Incorrect SHA/ref resolution in CI could overrun viewer checks (false positives) or skip needed checks (false negatives).
- Diff command failures on shallow clones or missing refs could break verification if fallback logic is not robust.
- New tests may become brittle if they depend on repository history rather than isolated temp repos.

## Test plan
- `bash scripts/test-verify-viewer-detection.sh`
- `./scripts/verify.sh --go-packages ./cmd/bitriver`
