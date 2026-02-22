# PLAN

## Scope (current change)
- Extend `scripts/verify.sh` with an opt-in flag that allows running Go tests for a targeted package pattern.
- Keep default behavior unchanged: existing full `go test ./...` run must remain the default when no new flag is provided.
- Document the new flag in `usage()` help text.
- Validate behavior with local invocation examples (no dedicated script test harness currently covers `scripts/verify.sh`).

## Assumptions
- The new option will be additive and optional, with no change to existing flags (`--viewer`, `--ci-viewer`).
- Targeted Go tests should still include existing go test arguments (`-count=1 -timeout=120s`) and env vars used by verify.
- No deployment contract files are impacted.

## Risks
- Argument parsing regressions could break existing `verify.sh` flags.
- Incorrect quoting around package patterns could cause `go test` failures.
- Accidentally reordering steps would violate required verify flow.

## Test plan
- `bash -n scripts/verify.sh`
- `./scripts/verify.sh --help`
- `./scripts/verify.sh --go-packages ./internal/chat` (local invocation example for targeted Go tests)
