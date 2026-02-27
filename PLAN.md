# PLAN

## Scope (current change)
- Expand `cmd/bitriver doctor` into a production-safe preflight that reports PASS/WARN/FAIL for host resources, Docker/Compose availability + minimum versions, host port conflicts, writable runtime paths, and optional GPU expectations.
- Add machine-readable output via `bitriver doctor --json` while keeping human-friendly actionable output by default.
- Enforce doctor in canonical env-check wrapper by updating `deploy/check-env.sh` to run doctor first, with an escape hatch `--skip-doctor`.
- Update operator docs with minimum host requirements and doctor usage/interpreting WARN vs FAIL.

## Assumptions
- Existing `doctor` command name remains canonical (no alias needed unless trivial).
- Conservative defaults should avoid false negatives across OSes; when host introspection is unreliable, checks degrade to WARN.
- Required host ports should match the same `.env`-driven ports used by quickstart preflight.

## Risks
- Overly strict version/resource thresholds could block valid development setups.
- Cross-platform host metrics collection can be inconsistent (Linux/macOS/Windows).
- `check-env.sh` argument parsing changes could break scripted callers if not backward-compatible.

## Test plan
- `go test ./cmd/bitriver -count=1` for command behavior and injected failure paths.
- `go test ./... -count=1 -timeout=120s` for repo-wide validation.
- `go run ./cmd/bitriver doctor --help` to verify CLI wiring/help text.
- `go run ./cmd/bitriver doctor --min-cpu 999` (or equivalent) to confirm intentional FAIL returns non-zero.
- `bash deploy/check-env.sh --skip-doctor` smoke check for wrapper flag parsing.
