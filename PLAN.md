# PLAN

## Scope (current change)
- Update `deploy/check-env.sh` so doctor preflight runs by default with both `--env-file` and canonical `--compose-file deploy/docker-compose.yml` before env validation.
- Preserve CI/operator compatibility by keeping default invocation argument-free and adding an explicit escape hatch `--skip-doctor` with documented usage.
- Improve script UX with explicit phase headings and actionable failure guidance for doctor failures.
- Update `docs/quickstart.md` and `docs/production-single-host.md` so `deploy/check-env.sh` is called out as the first environment preflight step.

## Assumptions
- `bitriver doctor` already encodes WARN vs FAIL semantics (WARN should return success; FAIL should return non-zero).
- Existing CI usage calls `bash deploy/check-env.sh` (or equivalent) without positional changes.

## Risks
- Parsing logic for optional `--skip-doctor` could regress if argument handling becomes order-sensitive.
- Doc updates may drift if they duplicate command examples inconsistently between quickstart and production docs.

## Test plan
- `bash deploy/check-env.sh --help`
- `bash deploy/check-env.sh --skip-doctor`
- `bash deploy/check-env.sh`

## Scope (previous change)
- Upgrade `bitriver doctor` into a production preflight with actionable PASS/WARN/FAIL checks while preserving `func runDoctor(args []string) bool` compatibility used by `verify` and `main`.
- Add flags `--env-file`, `--compose-file`, and `--json` to support environment-aware checks and machine-readable output.
- Expand checks to include host sizing, required/optional binaries, Docker/Compose minimum versions, port conflicts, and compose bind-mount readability/writability.
- Document the preflight workflow and minimum host guidance in operations/production docs.

## Assumptions
- Existing quickstart port requirement helpers remain authoritative for env-driven service ports.
- Compose file parsing should be best-effort using stdlib only (no heavy YAML dependency).
- `verify` must continue to call `runDoctor(nil)` unchanged.

## Risks
- Compose parsing false positives/negatives if lines use uncommon YAML shapes.
- OS-specific host resource probes may be incomplete outside Linux and should degrade to WARN.
- Version parsing may fail on unusual Docker output; should warn with manual remediation.

## Test plan
- `go test ./... -count=1`
- `go run ./cmd/bitriver doctor --compose-file deploy/docker-compose.yml`
- `go run ./cmd/bitriver doctor --json --compose-file deploy/docker-compose.yml`
- `go run ./cmd/bitriver doctor --compose-file deploy/does-not-exist.yml` (expect FAIL/non-zero)
- `go run ./cmd/bitriver verify`
