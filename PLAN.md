# PLAN

## Scope (current change)
- Refactor `randomSecret` and `randomSuffix` in `cmd/bitriver/env_validation_helpers.go` to return `(string, error)` instead of panicking.
- Thread entropy generation errors through `generateEnvValues` and all CLI call sites with wrapped `%w` errors while preserving existing success-path console output.
- Add unit tests that force entropy-read failures via a test seam and verify surfaced errors.
- Verify happy-path outputs remain unchanged.

## Assumptions
- Entropy read failures are exceptional and should fail fast with contextual errors.
- Existing success-path formatting and generated output values (shape/prefixes) must remain stable.

## Risks
- Signature changes can ripple through command code and tests.
- Error handling could unintentionally change user-visible output on successful runs.

## Test plan
- Targeted bitriver tests: `go test ./cmd/bitriver -count=1`.
- Focused happy-path and failure-path unit tests around env generation helpers.
