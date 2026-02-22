# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add opt-in targeted Go package flag to `scripts/verify.sh`
  - Acceptance criteria:
    - New CLI flag parses a package pattern/value for Go tests.
    - Default path still runs `go test ./... -count=1 -timeout=120s` when flag is omitted.
    - Verify step order remains unchanged.
  - Relevant checks:
    - ✅ `bash -n scripts/verify.sh`
  - Result:
    - Added `--go-packages` parsing with `./...` default, preserving default full test target and verify step ordering.

- [x] Task 2 — Document new flag in `scripts/verify.sh` help text
  - Acceptance criteria:
    - `usage()` includes the new flag syntax and behavior.
    - Existing help entries remain accurate.
  - Relevant checks:
    - ✅ `./scripts/verify.sh --help`
  - Result:
    - Help usage/options now document `--go-packages <pattern>` while keeping existing options intact.

- [x] Task 3 — Validate via local invocation examples
  - Acceptance criteria:
    - Script syntax check passes.
    - `--help` reflects the new flag.
    - Example invocation demonstrates targeted Go package test mode.
  - Relevant checks:
    - ✅ `bash -n scripts/verify.sh`
    - ✅ `./scripts/verify.sh --help`
    - ✅ `./scripts/verify.sh --go-packages ./internal/chat`
  - Result:
    - Targeted Go package mode works as expected; verify completed successfully with `internal/chat` Go tests plus existing checks in original order.

## Execution log
- ✅ `bash -n scripts/verify.sh` (pass).
- ✅ `./scripts/verify.sh --help` (pass).
- ✅ `./scripts/verify.sh --go-packages ./internal/chat` (pass; Docker compose validation skipped due to unavailable docker).

- ✅ `./scripts/verify.sh` (pass; Docker compose validation skipped due to unavailable docker).
