## Scoped change: add production canary and rollback gate (#1268)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Establish canary-gate scope
  - Acceptance criteria:
    - `PLAN.md` captures #1268 scope, assumptions, risks, and test plan.
    - `TASKS.md` lists ordered tasks before source/doc edits for this pass.
    - Existing release command, smoke command, operations docs, and production release docs are reviewed.

- [x] Task 2 - Add release canary command
  - Acceptance criteria:
    - `bitriver release canary` accepts `--base-url`, `--artifact-dir`, optional `--logs-file`, optional `--rollback-notes`, and `--require-rollback-notes`.
    - The command writes a JSON canary report and endpoint response artifacts.
    - High-confidence endpoint failures return non-zero; warnings are recorded without implying success where checks were skipped.

- [x] Task 3 - Add log scan and rollback readiness checks
  - Acceptance criteria:
    - Supplied logs are scanned for conservative fatal/error patterns with matching lines recorded.
    - Rollback notes can be required and checked for previous-version, migration/data, env/config, and artifact rollback coverage.
    - Unit tests cover clean logs, bad logs, missing rollback notes, and complete rollback notes.

- [x] Task 4 - Add wrapper and docs
  - Acceptance criteria:
    - `scripts/release-canary.sh` delegates to `bitriver release canary`.
    - `docs/release-gates.md`, `docs/production-release.md`, and `docs/operations.md` explain when/how to run the canary gate.
    - Docs keep the gate scoped to the supported single-host path.

- [x] Task 5 - Verify and record results
  - Acceptance criteria:
    - Focused Go tests pass.
    - Wrapper syntax passes.
    - A local command run against a controlled test endpoint/log fixture passes.
    - `git diff --check` passes.

### Execution log
- Task 1 read-only pass:
  - Confirmed #1267 is closed after PR #1287; selected open issue #1268 as the next release-gate ticket under #1264.
  - Reviewed `cmd/bitriver/release_contract.go`, `cmd/bitriver/release_smoke_gate.go`, `cmd/bitriver/smoke.go`, `docs/release-gates.md`, `docs/operations.md`, and `docs/production-release.md`.
  - Chose a bounded first pass: a non-mutating `release canary` command for already-running stacks, optional log-file scanning, enforceable rollback-note checks, and docs/wrapper updates without deployment contract edits.
- Task 2 implementation:
  - Added `bitriver release canary` with `--base-url`, `--artifact-dir`, `--logs-file`, `--rollback-notes`, `--require-rollback-notes`, and `--timeout`.
  - The command writes `canary-report.json`, CLI version evidence, and redacted endpoint response artifacts for `/readyz`, `/healthz`, `/api/status`, and `/viewer`.
  - Endpoint failures and explicit unhealthy JSON status fields fail the gate; skipped optional evidence is recorded as warnings.
- Task 3 implementation:
  - Added conservative log scanning for fatal/panic, migration failure, connection refused, missing required config, auth/session failure, and transcoder/ffmpeg failure patterns.
  - Added rollback-note coverage checks for previous version/tag, data or migration handling, env/config rollback, and artifact/image rollback path.
  - Added focused tests for healthy canary evidence, degraded endpoint failure, log pattern detection, and rollback-note coverage.
- Task 4 implementation:
  - Added `scripts/release-canary.sh` as the operator wrapper for `go run ./cmd/bitriver release canary`.
  - Updated `docs/release-gates.md`, `docs/production-release.md`, and `docs/operations.md` with Gate 6 command usage, artifacts, failure behavior, and single-host scope.
- Task 5 verification:
  - Passed: `gofmt -w cmd/bitriver/release_contract.go cmd/bitriver/release_canary.go cmd/bitriver/release_canary_test.go`.
  - Passed: `bash -n scripts/release-canary.sh`.
  - Passed: `git diff --check`.
  - Blocked locally: `go test ./cmd/bitriver -run "TestRunRelease|TestReleaseCanary" -count=1 -timeout=120s` with workspace-local `GOCACHE=.gocache-canary-gate` failed at link time because this host reported `There is not enough space on the disk`.
  - Not run locally: `./scripts/verify.sh`; this host still lacks `python3`, `python`, or the Windows `py` launcher required by the env placeholder hygiene phase.
