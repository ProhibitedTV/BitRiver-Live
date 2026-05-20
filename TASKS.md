## Scoped change: issue #1245 legal Postgres timeout helpers

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the legal Postgres helper plan
  - Acceptance criteria:
    - `PLAN.md` captures issue #1245 scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists ordered implementation and validation tasks before source edits continue.
    - The read-only pass identifies direct `context.Background()` legal DB calls, existing `withConn`/`acquireContext` patterns, and cleanup-plan task 6.

- [x] Task 2 - Route legal queries through repository timeout helpers
  - Acceptance criteria:
    - Postgres legal methods stop using bare `context.Background()` for repository DB operations.
    - Read/list/update/audit/history paths use `withConn` or `acquireContext` consistently with surrounding storage code.
    - Nil/unavailable repository behavior remains consistent with existing Postgres methods.

- [x] Task 3 - Extract focused legal normalization helpers
  - Acceptance criteria:
    - Repeated trim/status normalization in `postgres_legal.go` is reduced where it clarifies behavior.
    - Existing stored values, status casing, and not-found handling remain unchanged.

- [x] Task 4 - Update cleanup tracking
  - Acceptance criteria:
    - `docs/cleanup-plan.md` task 6 is marked complete after targeted storage validation passes.
    - Notes mention repository timeout helper usage for Postgres legal flows.

- [ ] Task 5 - Validate and publish
  - Acceptance criteria:
    - Storage tests, full Go tests, diff hygiene, and the repo verification gate pass or blockers are recorded.
    - Changes are committed, pushed, opened as a draft PR, CI is checked, and the PR is merged when green.

### Execution log (issue #1245 legal Postgres timeout helpers)
- Task 1 complete: after merging PR #1251 and syncing `main` to `f84717b`, selected issue #1245, created branch `codex/issue-1245-legal-postgres-timeouts`, fetched the issue, read nested storage agent notes, and audited `internal/storage/postgres_legal.go`, existing repository timeout helpers, legal storage behavior, and `docs/cleanup-plan.md` before source edits.
- Task 1 checks:
  - GitHub connector: fetched issue #1245.
  - `git checkout main`
  - `git pull --ff-only origin main`
  - `git checkout -b codex/issue-1245-legal-postgres-timeouts`
  - `rg --files -g AGENTS.md`
  - `Get-Content internal/storage/AGENTS.md`
  - `Get-Content internal/storage/postgres_legal.go`
  - `Get-Content internal/storage/postgres_repository.go`
  - `Get-Content internal/storage/legal.go`
  - `rg -n "DMCACase|DataSubject|LegalState|legal" internal/storage -g "*_test.go"`
  - `rg -n "context\\.Background\\(\\)|pool\\.(Query|Exec|QueryRow)|withConn\\(|acquireContext\\(" internal/storage --glob "postgres_*.go"`
- Task 2 complete: Postgres legal list/get/update/create/audit/history methods now run database work through `withConn`, sharing the repository acquire timeout and connection lifecycle instead of using bare background contexts.
- Task 2 checks:
  - `gofmt -w internal/storage/postgres_legal.go`
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./internal/storage -count=1 -timeout=120s` - passed.
  - `rg -n "context\\.Background\\(\\)|r\\.pool\\.(Query|Exec|QueryRow)" internal/storage/postgres_legal.go` - passed; no matches.
- Task 3 complete: added focused legal trim/status and scan helpers, then reused them across DMCA and data-subject paths while preserving existing status casing and not-found behavior.
- Task 3 checks:
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./internal/storage -count=1 -timeout=120s` - passed.
- Task 4 complete: marked `docs/cleanup-plan.md` task 6 complete with notes for Postgres legal `withConn` usage and focused normalization helpers.
- Task 4 checks:
  - `rg -n "Task 6|withConn|context\\.Background\\(\\)|r\\.pool\\.(Query|Exec|QueryRow)|trimLegalText|normalizeLegalStatus" docs/cleanup-plan.md internal/storage/postgres_legal.go TASKS.md PLAN.md` - passed; source hits show helper usage and no bare background/direct pool calls in `postgres_legal.go`.
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./internal/storage -count=1 -timeout=120s` - passed.
- Task 5 validation progress:
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./... -count=1 -timeout=120s` - passed.
  - `git diff --check` - passed with line-ending warnings only.
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh` - passed full repo verification; viewer checks were skipped because no viewer files changed.
