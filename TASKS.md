## Scoped change: add golden-path release gate (#1266)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Establish release-gate scope
  - Acceptance criteria:
    - `PLAN.md` captures #1266 scope, assumptions, risks, and test plan.
    - `TASKS.md` lists ordered tasks before source/doc edits for this pass.
    - Existing quickstart, smoke, verify, upgrade-plan, release-gate docs, and CI entrypoints are reviewed.

- [x] Task 2 - Add named smoke-gate command
  - Acceptance criteria:
    - `go run ./cmd/bitriver release smoke-gate` runs a fast tier locally and writes artifacts.
    - Full tier can run source quickstart plus smoke and collect Compose diagnostics.
    - Failure output names the failed phase and artifact path.
    - Env artifact redacts values and records redaction coverage only.

- [x] Task 3 - Add wrapper and CI fast-tier wiring
  - Acceptance criteria:
    - `./scripts/release-gate-smoke.sh` delegates to the Go release command.
    - `scripts/test-all.sh` runs the fast tier when quickstart smoke is enabled.
    - Script syntax checks pass.

- [x] Task 4 - Document release-gate operation
  - Acceptance criteria:
    - `docs/release-gates.md` names the new gate command and artifacts.
    - Release docs state which gate must pass before tagging a release.
    - Packaged launcher and upgrade-path execution are documented as staged follow-up evidence where not fully automated.

- [x] Task 5 - Verify and record results
  - Acceptance criteria:
    - Focused Go tests pass.
    - Fast tier command and wrapper pass locally.
    - Source-only script load/syntax checks pass.
    - `git diff --check` passes.

### Execution log
- Task 1 read-only pass:
  - Confirmed #1265 is closed after PR #1285; selected open issue #1266 as the next release-gate ticket under #1264.
  - Reviewed existing `quickstart`, `smoke`, `verify`, `upgrade-plan`, and `release contract-*` command surfaces.
  - Reviewed `scripts/verify.sh`, `scripts/test-all.sh`, `scripts/test-quickstart.sh`, `.github/workflows/ci.yml`, `docs/release-gates.md`, `docs/quickstart.md`, and `docs/production-release.md`.
  - Confirmed nested `AGENTS.md` files under `scripts/` and `docs/` only point back to the root guide.
  - Chose a bounded first pass: compose existing checks into a named fast/full gate, save artifacts, and wire fast tier through the existing quickstart-enabled test-all path without deployment contract edits.
- Task 2 complete:
  - Added `bitriver release smoke-gate` with `fast` and `full` tiers, phase-level artifacts, redacted env summaries, upgrade-plan evidence, source quickstart/smoke orchestration, and Compose diagnostics.
  - Added focused tests for fast artifacts, full-tier quickstart/smoke/diagnostics orchestration, and actionable phase failure output.
  - Check: `gofmt -w cmd\bitriver\release_contract.go cmd\bitriver\release_smoke_gate.go cmd\bitriver\release_smoke_gate_test.go` passed.
  - Check: `go test ./cmd/bitriver -run "TestRunRelease|TestReleaseSmokeGate" -count=1 -timeout=120s` failed before compile because the default Windows Go build cache path could not create `go-build\00`.
  - Check: same focused Go test with workspace-local `GOCACHE=.gocache-release-gate` passed.
- Task 3 complete:
  - Added `./scripts/release-gate-smoke.sh` wrapper for `go run ./cmd/bitriver release smoke-gate`.
  - Updated `scripts/test-all.sh` so quickstart-enabled CI runs the release smoke gate fast tier before the slower quickstart smoke.
  - Adjusted the fast tier to record a skipped Compose config artifact when Docker Compose is unavailable locally; full tier still requires Compose.
  - Check: `bash -n scripts/release-gate-smoke.sh scripts/test-all.sh` passed under Git Bash.
  - Check: `./scripts/release-gate-smoke.sh --tier fast --artifact-dir .tmp/release-gate-wrapper --target v0.0.0-test` passed with workspace-local `GOCACHE`; local Compose config phase was skipped because `docker compose version` is unavailable on this host.
- Task 4 complete:
  - Updated `docs/release-gates.md` to name `./scripts/release-gate-smoke.sh --tier fast` and `--tier full`, list artifacts, and explain PR CI fast-tier wiring through `scripts/test-all.sh`.
  - Updated `docs/production-release.md` to require the full release smoke gate before tagging on a Docker-capable release-candidate host and allow fast-tier evidence for non-mutating local review.
  - Check: `rg -n "release-gate-smoke|smoke-gate|release-gate-report|Gate 3|full tier|fast tier" docs/release-gates.md docs/production-release.md scripts/test-all.sh scripts/release-gate-smoke.sh cmd/bitriver` confirmed expected references.
- Task 5 complete:
  - Check: `go test ./cmd/bitriver -run "TestRunRelease|TestReleaseSmokeGate" -count=1 -timeout=120s` passed with workspace-local `GOCACHE=.gocache-release-gate`.
  - Check: `go run ./cmd/bitriver release smoke-gate --tier fast --artifact-dir .tmp/release-gate-smoke --target v0.0.0-test` passed with workspace-local `GOCACHE`; local Compose config phase was skipped because `docker compose version` is unavailable on this host.
  - Check: `./scripts/release-gate-smoke.sh --tier fast --artifact-dir .tmp/release-gate-wrapper --target v0.0.0-test` passed with workspace-local `GOCACHE`; local Compose config phase was skipped for the same host reason.
  - Check: `bash -n scripts/release-gate-smoke.sh scripts/test-all.sh` passed under Git Bash.
  - Check: `BITRIVER_VERIFY_SOURCE_ONLY=1 ./scripts/verify.sh` passed under Git Bash.
  - Check: `git diff --check` passed with line-ending warnings only.
  - Check: `./scripts/verify.sh --go-packages ./cmd/bitriver` was attempted under Git Bash and stopped at `Env example placeholder hygiene` because no Python runtime is installed on this host (`python`, `python3`, and `py -3` are unavailable). Docker Compose is also unavailable locally (`docker compose version` fails), so full local Docker-dependent release smoke remains unproven on this machine.
  - Cleanup: removed generated `.tmp/release-gate-smoke`, `.tmp/release-gate-wrapper`, and `.gocache-release-gate` directories.
- PR #1286 CI follow-up:
  - Ubuntu test-all initially failed in the new `Release smoke gate fast tier` step after `verify.sh`/quickstart cleanup removed root `.env`; `deploy/.env.example` as `--env-file` did not satisfy Compose service-level `env_file: ../.env`.
  - Updated the smoke gate to create a temporary root `.env` from the fallback env only when the requested default root `.env` is absent, remove it after the gate, and include command-output tails in failed phase errors.
  - Check: focused Go test passed again with workspace-local `GOCACHE=.gocache-release-gate`.
  - Check: wrapper fast tier passed again with workspace-local `GOCACHE=.gocache-release-gate`; local Compose config remained skipped because Docker Compose is unavailable on this host.
  - Check: `git diff --check` passed with line-ending warnings only.
