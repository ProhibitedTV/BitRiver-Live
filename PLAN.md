# PLAN

## Scope (current change)
- Refactor `cmd/bitriver/commands_env_compose.go` to extract shared polling/deadline/sleep control flow into an internal helper (e.g. `pollUntil`).
- Apply the helper in both `waitForAPIReadiness` and `waitForComposeServiceHealth` without changing existing user-facing output/error strings.
- Add focused tests for polling helper behavior (success, timeout, cancellation) in `cmd/bitriver/main_test.go` or a dedicated `_test.go`.

## Assumptions
- Change is limited to CLI control-flow internals in `cmd/bitriver`; no deployment contract files are touched.
- Existing tests around readiness/compose health should remain valid if messages remain unchanged.

## Risks
- Subtle behavior drift in loop termination timing could alter timeout/cancellation surfaces.
- Refactor may accidentally alter error wrapping or emitted text.

## Test plan
- Run targeted bitriver command tests including new polling tests: `go test ./cmd/bitriver -count=1`.
- Run required repo gate: `./scripts/verify.sh`.
