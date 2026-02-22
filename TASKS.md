# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Convert random generators to error-returning helpers
  - Acceptance criteria:
    - `randomSecret` and `randomSuffix` return `(string, error)`.
    - A test seam exists to force entropy-read failure in tests.
  - Relevant checks:
    - ✅ `go test ./cmd/bitriver -count=1 -run 'TestGenerateEnvValuesReturnsErrorWhen'`
  - Result:
    - Passed.

- [x] Task 2 — Thread and wrap errors through env generation + callers
  - Acceptance criteria:
    - `generateEnvValues` surfaces generator failures as errors.
    - Call sites handle and wrap errors with `%w` while preserving successful CLI behavior.
  - Relevant checks:
    - ✅ `go test ./cmd/bitriver -count=1 -run 'TestEnvInitWritesGeneratedValues'`
  - Result:
    - Passed.

- [x] Task 3 — Add/adjust tests for failure and happy paths
  - Acceptance criteria:
    - Unit tests cover entropy-read failures for secret and suffix generation.
    - Existing happy-path expectations remain unchanged and pass.
  - Relevant checks:
    - ✅ `go test ./cmd/bitriver -count=1 -run 'TestGenerateEnvValues|TestValidateEnvAcceptsFreshInitDefaults|TestEnvInitWritesGeneratedValues'`
  - Result:
    - Passed.

## Execution log
- `go test ./cmd/bitriver -count=1` (fails non-deterministically in existing `TestGenerateStrongPasswordHasRequiredClasses`; unrelated to this change).
- `go test ./cmd/bitriver -count=1 -run 'TestGenerateEnvValuesReturnsErrorWhen'` (pass).
- `go test ./cmd/bitriver -count=1 -run 'TestEnvInitWritesGeneratedValues'` (pass).
- `go test ./cmd/bitriver -count=1 -run 'TestGenerateEnvValues|TestValidateEnvAcceptsFreshInitDefaults|TestEnvInitWritesGeneratedValues'` (pass).
