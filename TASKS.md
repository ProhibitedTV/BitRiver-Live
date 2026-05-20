## Scoped change: issue #1244 ingest/Postgres cancellation helpers

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the ingest/Postgres helper plan
  - Acceptance criteria:
    - `PLAN.md` captures issue #1244 scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists ordered implementation and validation tasks before source edits continue.
    - The read-only pass identifies `runIngestBootWithRetry`, stream start call sites, auth Postgres helper duplication, and cleanup-plan task 5.

- [x] Task 2 - Make ingest boot retries cancellation-aware
  - Acceptance criteria:
    - `runIngestBootWithRetry` accepts a parent context and stops before or between attempts when it is canceled.
    - Retry waits use a cancellable timer/select path rather than `time.Sleep`.
    - Existing terminal context-error and transient-retry behavior remains covered by tests.

- [x] Task 3 - Reduce auth Postgres timeout helper duplication
  - Acceptance criteria:
    - Session and MFA challenge Postgres stores share package-private operation/ping timeout helpers.
    - Exported store constructors, options, and method signatures remain unchanged.
    - Existing auth tests continue to pass.

- [x] Task 4 - Update cleanup tracking
  - Acceptance criteria:
    - `docs/cleanup-plan.md` task 5 is marked complete after targeted auth/storage tests pass.
    - Notes mention cancellable ingest retries and shared auth Postgres timeout helpers.

- [x] Task 5 - Validate and publish
  - Acceptance criteria:
    - Targeted storage/auth tests, full Go tests, diff hygiene, and the repo verification gate pass or blockers are recorded.
    - Changes are committed, pushed, opened as a draft PR, CI is checked, and the PR is merged when green.

### Execution log (issue #1244 ingest/Postgres cancellation helpers)
- Task 1 complete: after merging PR #1250 and syncing `main` to `840377f4`, selected issue #1244, created branch `codex/issue-1244-ingest-postgres-cancel`, fetched the issue, read nested agent notes, and audited `internal/storage/ingest_boot_helpers.go`, stream start call sites, `internal/storage/postgres_repository.go`, auth Postgres stores, and `docs/cleanup-plan.md` before source edits.
- Task 1 checks:
  - GitHub connector: fetched issue #1244.
  - `git checkout main`
  - `git pull --ff-only origin main`
  - `git checkout -b codex/issue-1244-ingest-postgres-cancel`
  - `Get-Content internal/storage/AGENTS.md`
  - `Get-Content internal/auth/AGENTS.md`
  - `rg -n "runIngestBootWithRetry|context\\.Background\\(\\)|time\\.Sleep|WithTimeout|Acquire\\(|acquire|timeout|postgres" internal/storage internal/auth`
  - `Get-Content internal/storage/ingest_boot_helpers.go`
  - `Get-Content internal/storage/stream_test.go`
  - `Get-Content internal/storage/storage.go`
  - `Get-Content internal/storage/postgres_repository.go`
  - `Get-Content internal/auth/postgres_store.go`
  - `Get-Content internal/auth/postgres_mfa_challenge_store.go`
- Task 2 complete: `runIngestBootWithRetry` now accepts a parent context, checks cancellation before attempts, uses per-attempt child timeouts from that parent, and waits between retries with a cancellable timer instead of `time.Sleep`.
- Task 2 checks:
  - `gofmt -w internal/storage/ingest_boot_helpers.go internal/storage/storage.go internal/storage/postgres_channels.go internal/storage/stream_test.go internal/auth/postgres_context.go internal/auth/postgres_store.go internal/auth/postgres_mfa_challenge_store.go`
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./internal/storage -run "TestRunIngestBootWithRetry" -count=1 -timeout=120s` - passed.
- Task 3 complete: added shared auth Postgres operation and ping helpers, then switched session and MFA challenge stores to use them without changing constructors, options, or public method signatures.
- Task 3 checks:
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./internal/auth -count=1 -timeout=120s` - passed.
- Task 4 complete: marked `docs/cleanup-plan.md` task 5 complete with notes for cancellable ingest retry waits and shared auth Postgres timeout/ping helpers.
- Task 4 checks:
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./internal/storage -count=1 -timeout=120s` - passed.
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./internal/auth ./internal/storage -count=1 -timeout=120s` - passed.
  - `rg -n "Task 5|runIngestBootWithRetry|postgresOperationContext|time\\.Sleep\\(retryInterval\\)|operationContext\\(" docs/cleanup-plan.md internal/storage internal/auth TASKS.md PLAN.md` - passed; no bare retry sleep or duplicated auth operation context helper remains.
- Task 5 validation progress:
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./... -count=1 -timeout=120s` - passed.
  - `git diff --check` - passed with line-ending warnings only.
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh` - passed full repo verification; viewer checks were skipped because no viewer files changed.
- Task 5 complete: committed and pushed the branch, opened draft PR #1251, and confirmed GitHub CI passed on the PR head before merge.
- Task 5 publishing:
  - `git add PLAN.md TASKS.md docs/cleanup-plan.md internal/auth/postgres_context.go internal/auth/postgres_mfa_challenge_store.go internal/auth/postgres_store.go internal/storage/ingest_boot_helpers.go internal/storage/postgres_channels.go internal/storage/storage.go internal/storage/stream_test.go`
  - `git commit -m "storage: make ingest retries cancellable"`
  - `git push -u origin codex/issue-1244-ingest-postgres-cancel`
  - GitHub connector: opened draft PR #1251.
  - GitHub connector: CI completed successfully for the PR head.
