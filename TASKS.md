## Scoped change: add contract/schema drift gate (#1265)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Establish the contract drift scope
  - Acceptance criteria:
    - `PLAN.md` captures #1265 scope, assumptions, risks, and test plan.
    - `TASKS.md` lists ordered tasks before source/doc edits for this pass.
    - Existing CLI, contract docs, verifier scripts, and migration layout are reviewed.

- [x] Task 2 - Add release contract snapshot command
  - Acceptance criteria:
    - `go run ./cmd/bitriver release contract-snapshot` emits stable JSON.
    - Snapshot covers env template keys/defaults/comments, Compose services, migration files, generated artifact status, health endpoints, and release artifact inputs.
    - Snapshot generation does not require Docker or a running stack.

- [x] Task 3 - Add contract diff classification
  - Acceptance criteria:
    - `go run ./cmd/bitriver release contract-diff --base <file> --head <file>` compares two snapshots.
    - Diff output classifies additive, breaking/removal, and security-sensitive changes.
    - Identical snapshots exit cleanly and report no drift.

- [x] Task 4 - Document the new drift gate
  - Acceptance criteria:
    - Release-gate or contract docs explain how to generate and compare snapshots.
    - Documentation states this is a first modest gate and future CI wiring can consume the JSON.
    - No deployment contract files are changed.

- [x] Task 5 - Verify and record results
  - Acceptance criteria:
    - Focused Go tests pass.
    - Snapshot and self-diff commands pass locally.
    - `git diff --check` passes.

- [-] Task 6 - Unblock PR #1285 macOS Go CI
  - Acceptance criteria:
    - CI Go test matrices use a stable macOS runner label instead of the currently failing floating `macos-latest` image.
    - Go workflow convention checks pass locally.
    - PR #1285 checks are rechecked after pushing the workflow fix.

### Execution log
- Task 1 read-only pass:
  - PR #1284 was marked ready and squash-merged into `main`; issue #1269 is closed.
  - Selected issue #1265 as the next concrete implementation gate under release-gates epic #1264.
  - Reviewed `cmd/bitriver` command dispatch and existing command styles, `docs/contract.md`, `docs/contract-change-recipe.md`, `docs/release-gates.md`, `scripts/check-contract-invariants.sh`, and migration file layout.
  - Confirmed this pass should avoid deployment contract edits and add a file-parsing release gate that can run without Docker.
- Task 2 complete: added `bitriver release contract-snapshot` with stable JSON covering env template variables, Compose services, migrations, generated artifacts, health endpoints, and release artifact inputs.
  - Check: `go test ./cmd/bitriver -run "TestRunRelease|TestBuildContractSnapshot|TestDiffContractSnapshots" -count=1 -timeout=120s` passed with workspace-local `GOCACHE`.
  - Check: `go run ./cmd/bitriver release contract-snapshot --env-file deploy/.env.example --compose-file deploy/docker-compose.yml --output .tmp/contract-snapshot.json` passed.
- Task 3 complete: added `bitriver release contract-diff` with additive, breaking, security, and undocumented drift classification plus non-zero exit for review-required drift unless `--allow-breaking` is set.
  - Check: self-diff of `.tmp/contract-snapshot.json` passed with zero drift and `changes: []`.
- Task 4 complete: updated `docs/release-gates.md` and `docs/contract-change-recipe.md` with the new snapshot and diff commands, current coverage, and first-version limitations.
  - Check: `rg -n "contract-snapshot|contract-diff|release contract" docs/release-gates.md docs/contract-change-recipe.md README.md cmd/bitriver` confirmed the expected references.
- Task 5 complete:
  - `gofmt -w cmd/bitriver/release_contract.go cmd/bitriver/release_contract_test.go cmd/bitriver/main.go` - passed.
  - `go test ./cmd/bitriver -run "TestRunRelease|TestBuildContractSnapshot|TestDiffContractSnapshots" -count=1 -timeout=120s` - passed with workspace-local `GOCACHE`.
  - `go run ./cmd/bitriver release contract-snapshot --env-file deploy/.env.example --compose-file deploy/docker-compose.yml --output .tmp/contract-snapshot.json` - passed.
  - `go run ./cmd/bitriver release contract-diff --base .tmp/contract-snapshot.json --head .tmp/contract-snapshot.json` - passed with zero drift.
  - `git diff --check` - passed with line-ending warnings only.
  - `./scripts/verify.sh` via Git Bash - attempted; failed during the Go test phase because the Windows host reported paging-file/out-of-memory errors while compiling unrelated packages (`fatal error: out of memory allocating heap arena map`, `The paging file is too small for this operation to complete`).
  - Note: the host disk was full during focused Go tests. Cleared generated cache directories to continue, then restored tracked `.gocache-*` files so they are not part of this change.
- Task 6 read-only pass:
  - Confirmed PR #1285 is blocked only by `Go tests (macos-latest)` while Ubuntu, Windows, docs, image scan, and quickstart checks pass or skip as expected.
  - Failing macOS logs abort generated Go test binaries with `missing LC_UUID load command` on the floating macOS runner image.
- Task 6 local fix:
  - Updated Go test matrices in `.github/workflows/ci.yml` and `.github/workflows/go-unit-tests.yml` from `macos-latest` to `macos-15`.
  - Check: `git diff --check` passed with line-ending warnings only.
  - Check: `& 'C:\Program Files\Git\bin\bash.exe' -lc 'export PATH=/usr/bin:/bin:$PATH; command -v grep; scripts/check-go-workflow-config.sh'` passed.
