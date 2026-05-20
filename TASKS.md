## Scoped change: issue #1243 auth/server timing tests

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the auth/server timing plan
  - Acceptance criteria:
    - `PLAN.md` captures issue #1243 scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists ordered implementation and validation tasks before source edits continue.
    - The read-only pass identifies the current auth session clock coverage and the remaining stale-bucket sleep in server tests.

- [x] Task 2 - Remove the stale-bucket cleanup sleep
  - Acceptance criteria:
    - `TestRateLimiterCleanupStaleBucketsEventually` advances deterministic test time instead of calling `time.Sleep`.
    - Runtime rate-limiter behavior continues to default to wall-clock time.
    - Cleanup and last-seen calculations use a consistent time source.

- [x] Task 3 - Update cleanup tracking
  - Acceptance criteria:
    - `docs/cleanup-plan.md` task 4 is marked complete after targeted auth/server tests pass.
    - Notes mention that auth session timing tests already used deterministic clocks and server cleanup now does too.

- [x] Task 4 - Validate and publish
  - Acceptance criteria:
    - Targeted auth/server tests, full Go tests, diff hygiene, and the repo verification gate pass or blockers are recorded.
    - Changes are committed, pushed, opened as a draft PR, CI is checked, and the PR is merged when green.

### Execution log (issue #1243 auth/server timing tests)
- Task 1 complete: after merging PR #1249 and syncing `main` to `7c543e8d`, selected issue #1243, created branch `codex/issue-1243-auth-server-time-tests`, fetched the issue, read nested agent notes, and audited `internal/auth/session_test.go`, `internal/auth/session.go`, `internal/server/server_test.go`, `internal/server/ratelimit.go`, and `docs/cleanup-plan.md` before source edits.
- Task 1 checks:
  - GitHub connector: fetched issue #1243.
  - `git checkout main`
  - `git pull --ff-only origin main`
  - `git checkout -b codex/issue-1243-auth-server-time-tests`
  - `Get-Content internal/auth/AGENTS.md`
  - `Get-Content internal/server/AGENTS.md`
  - `rg -n "time\\.Sleep|SessionExpiration|ValidateRefreshesIdleTimeout|ValidateHonorsAbsoluteTTL|stale|bucket|cleanup|rate" internal/auth internal/server`
  - `Get-Content internal/auth/session_test.go`
  - `Get-Content internal/server/server_test.go`
  - `Get-Content internal/server/ratelimit.go`
- Task 2 complete: added a private `rateLimiter` clock hook that defaults to `time.Now`, used that same time source for cleanup and login bucket `lastSeen` calculations, and changed the stale-bucket test to advance deterministic time instead of sleeping.
- Task 2 checks:
  - `gofmt -w internal/server/ratelimit.go internal/server/server_test.go`
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./internal/server -run TestRateLimiterCleanupStaleBucketsEventually -count=1 -timeout=120s` - passed.
- Task 3 complete: marked `docs/cleanup-plan.md` task 4 complete with notes that auth session timing tests already use deterministic clock control and the server stale-bucket test now does too.
- Task 3 checks:
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./internal/auth -count=1 -timeout=120s` - passed.
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./internal/server -count=1 -timeout=120s` - passed.
  - `rg -n "time\\.Sleep|Task 4|rateLimiterTestClock|currentTime|cleanupLocked" internal/auth/session_test.go internal/server/server_test.go internal/server/ratelimit.go docs/cleanup-plan.md` - passed; remaining `time.Sleep` hits are cleanup-plan backlog text for other tasks.
- Task 4 validation progress:
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./... -count=1 -timeout=120s` - passed.
  - `git diff --check` - passed with line-ending warnings only.
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh` - passed full repo verification; viewer checks were skipped because no viewer files changed.
- Task 4 complete: committed and pushed the branch, opened draft PR #1250, and confirmed GitHub CI passed on the PR head before merge.
- Task 4 publishing:
  - `git add PLAN.md TASKS.md docs/cleanup-plan.md internal/server/ratelimit.go internal/server/server_test.go`
  - `git commit -m "server: stabilize rate limiter cleanup test"`
  - `git push -u origin codex/issue-1243-auth-server-time-tests`
  - GitHub connector: opened draft PR #1250.
  - GitHub connector: CI completed successfully for the PR head.
