## Scoped change: issue #1241 quickstart shell test hardening

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the quickstart shell-test plan
  - Acceptance criteria:
    - `PLAN.md` captures issue #1241 scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists ordered implementation and validation tasks before source edits continue.
    - The read-only pass identifies current quickstart wrapper tests, script behavior, scripts-scoped agent guidance, and any existing shell-discovery helper.

- [x] Task 2 - Verify whether the cleanup item is already resolved
  - Acceptance criteria:
    - `scripts/quickstart_test.go` is checked for hard-coded shell behavior versus platform-aware discovery/skip handling.
    - Targeted scripts tests are run before deciding whether code changes are required.
    - If the issue is stale, cleanup tracking is updated with evidence instead of changing behavior.

- [x] Task 3 - Apply any required quickstart test harness fix
  - Acceptance criteria:
    - Quickstart wrapper tests do not fail before exercising wrapper logic on Windows when a supported shell is available.
    - Unsupported hosts skip with a precise reason.
    - Wrapper delegation and missing-source assertions remain covered.

- [-] Task 4 - Validate and publish
  - Acceptance criteria:
    - Targeted scripts tests, full Go tests, diff hygiene, and the repo verification gate pass or blockers are recorded.
    - Changes are committed, pushed, opened as a draft PR, CI is checked, and the PR is merged when green.

### Execution log (issue #1241 quickstart shell test hardening)
- Task 1 complete: after merging PR #1247 and syncing `main` to `0ba11a57`, selected issue #1241, created branch `codex/issue-1241-quickstart-shell-tests`, read the scripts-scoped agent note, fetched the issue, and audited `scripts/quickstart_test.go`, `scripts/quickstart.sh`, and quickstart-related references before source edits.
- Task 1 checks:
  - GitHub connector: fetched issue #1241.
  - `git checkout main`
  - `git pull --ff-only origin main`
  - `git checkout -b codex/issue-1241-quickstart-shell-tests`
  - `Get-Content scripts/AGENTS.md`
  - `Get-Content scripts/quickstart_test.go`
  - `Get-Content scripts/quickstart.sh`
  - `rg -n "quickstart|bash|script|TestQuickstart" scripts cmd internal`
- Task 2 complete: `scripts/quickstart_test.go` already uses a platform-aware `testBash` helper that checks `BITRIVER_TEST_BASH`, common Git Bash install paths on Windows, and `exec.LookPath("bash")`, then skips with a precise reason only when no candidate can run `printf ok`.
- Task 2 checks:
  - `go test ./scripts -count=1 -timeout=120s` - first attempt did not run tests because the host default Go build cache failed to initialize.
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./scripts -count=1 -timeout=120s` - passed.
- Task 3 complete: no harness code change was required because the current test helper already satisfies issue #1241 acceptance criteria; updated `docs/cleanup-plan.md` task 2 to complete with verification notes.
- Task 4 in progress: ran targeted scripts tests, full Go tests, reran the unrelated transcoder timeout, and ran the full verification gate.
- Task 4 checks:
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./scripts -count=1 -timeout=120s` - passed.
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./... -count=1 -timeout=120s` - failed in `cmd/transcoder` on `TestUploadPublishesHTTPPlayback` timing out while waiting for completed upload metadata; this is outside the scripts scope and aligns with tracking issue #1242.
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./cmd/transcoder -run TestUploadPublishesHTTPPlayback -count=1 -timeout=120s` - passed on rerun.
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh` - passed full repo verification, including Go tests, contract checks, Docker Compose config validation, quickstart smoke, and skipped viewer checks because no viewer files changed.
