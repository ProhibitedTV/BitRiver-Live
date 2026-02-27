# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Implement structured doctor/preflight checks in CLI
  - Acceptance criteria:
    - `bitriver doctor` evaluates host resources, Docker/Compose availability + minimum versions, host port conflicts, writable dirs/files, and optional GPU hinting.
    - Output includes per-check PASS/WARN/FAIL with actionable mitigation guidance.
    - Exit code is non-zero when any FAIL is present; WARN-only runs exit zero.
    - `--json` emits machine-readable structured results.

- [x] Task 2 — Add doctor test coverage including intentional fail simulation
  - Acceptance criteria:
    - Unit tests cover summary/exit behavior and at least one intentionally failing condition via flags or mockable check functions.
    - Existing command tests continue to pass.

- [x] Task 3 — Enforce doctor in deploy/check-env.sh with skip override
  - Acceptance criteria:
    - `deploy/check-env.sh` runs doctor before `env validate` by default.
    - `--skip-doctor` bypasses the preflight while preserving existing env validation behavior.
    - Failure output explains next steps.

- [x] Task 4 — Update docs for minimum host requirements and doctor usage
  - Acceptance criteria:
    - Docs include baseline host requirements and factors that change them.
    - Docs describe `bitriver doctor` and WARN vs FAIL interpretation.

## Execution log
- ✅ Task 1 complete: moved doctor into a dedicated preflight implementation (`cmd/bitriver/doctor.go`) with structured checks for host resources, Docker/Compose versions, host ports, writable runtime paths, and optional GPU profile detection.
- ✅ Task 1 check: `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver doctor --help`.
- ✅ Task 1 check: `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go run ./cmd/bitriver doctor --min-cpu 999` (intentional FAIL path exits non-zero).

- ✅ Task 2 complete: added `cmd/bitriver/doctor_test.go` for intentional failure threshold, JSON report shape, and dependency-injected pass behavior.
- ✅ Task 2 check: `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./cmd/bitriver -count=1`.

- ✅ Task 3 complete: updated `deploy/check-env.sh` to run doctor preflight by default, added `--skip-doctor`, and added explicit remediation text when doctor fails.
- ✅ Task 3 check: `bash deploy/check-env.sh --skip-doctor` (script path/flag parsing exercised; env validation failed because repo `.env` is absent in this environment).

- ✅ Task 4 complete: updated `docs/quickstart.md` with safe-default minimum host requirements and doctor command guidance (`--json`, WARN vs FAIL semantics).
- ✅ Task 4 check: `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s`.
