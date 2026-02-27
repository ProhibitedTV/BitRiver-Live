# PLAN

## Scope (current change)
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
