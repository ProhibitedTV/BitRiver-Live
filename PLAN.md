# PLAN

## Scope (current change)
- Update `scripts/verify.sh` startup flow to explicitly call `require_tool` for required dependencies at the point they are needed.
- Require `go` before running Go tests.
- Require `python3` before running `./scripts/check-contract-invariants.sh`.
- Keep docker/node/npm checks intentionally optional, and add an explicit local fallback path when `git` is unavailable for viewer change detection.

## Assumptions
- `git` may be unavailable in some local environments even when the repository checkout exists.
- Existing CI behavior should remain unchanged for viewer checks.
- This change is limited to script behavior/messages; no deployment contract files are touched.

## Risks
- Incorrect handling of missing `git` could accidentally force or skip viewer checks unexpectedly.
- Moving tool checks might alter the failure point/order, so messaging must remain clear.

## Test plan
- `bash -n scripts/verify.sh`
- `./scripts/verify.sh --go-packages ./cmd/bitriver`
