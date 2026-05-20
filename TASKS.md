## Scoped change: issue #1246 upload helper extraction

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the upload helper plan
  - Acceptance criteria:
    - `PLAN.md` captures issue #1246 scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists ordered implementation and validation tasks before source edits continue.
    - The read-only pass identifies backend multipart parsing/default logic, viewer payload/state mapping, existing upload tests, and cleanup-plan task 7.

- [x] Task 2 - Extract backend upload parsing helpers
  - Acceptance criteria:
    - `internal/api/uploads_handlers.go` sheds at least one pure parsing/state-mapping helper.
    - Multipart field parsing and file-derived defaults preserve current trimming, metadata, title, filename, and size behavior.
    - Focused Go tests cover the extracted helper behavior.

- [x] Task 3 - Extract viewer upload payload helpers
  - Acceptance criteria:
    - `web/viewer/components/UploadManager.tsx` sheds at least one pure payload/state-mapping helper.
    - Metadata and size parsing keep the same `undefined` and trimming behavior.
    - Focused Jest tests cover the extracted helper behavior.

- [x] Task 4 - Update cleanup tracking
  - Acceptance criteria:
    - `docs/cleanup-plan.md` task 7 is marked complete after targeted API/viewer tests pass.
    - Notes mention backend and frontend helper extraction.

- [ ] Task 5 - Validate and publish
  - Acceptance criteria:
    - Targeted API/viewer tests, full Go tests, diff hygiene, and the viewer-inclusive repo verification gate pass or blockers are recorded.
    - Changes are committed, pushed, opened as a draft PR, CI is checked, and the PR is merged when green.

### Execution log (issue #1246 upload helper extraction)
- Task 1 complete: after merging PR #1252 and syncing `main` to `e457f91`, selected issue #1246, created branch `codex/issue-1246-extract-upload-helpers`, fetched the issue, read nested API/viewer agent notes, and audited `internal/api/uploads_handlers.go`, `web/viewer/components/UploadManager.tsx`, existing upload tests, viewer test config, and `docs/cleanup-plan.md` before source edits.
- Task 1 checks:
  - GitHub connector: fetched issue #1246.
  - `git checkout main`
  - `git pull --ff-only origin main`
  - `git checkout -b codex/issue-1246-extract-upload-helpers`
  - `Get-Content internal/api/AGENTS.md`
  - `Get-Content web/viewer/AGENTS.md`
  - `Get-Content internal/api/uploads_handlers.go`
  - `Get-Content web/viewer/components/UploadManager.tsx`
  - `rg -n "Upload|upload|UploadManager|createUpload|metadata|deriveTitle|formatFileSize" internal/api web/viewer -g "*_test.go" -g "*.test.ts" -g "*.test.tsx"`
  - `Get-Content internal/api/uploads_handlers_test.go`
  - `Get-Content web/viewer/__tests__/uploadManager.test.tsx`
- Task 2 complete: extracted backend multipart field parsing and file-derived default mapping into focused helpers, then switched `createUploadFromMultipart` to use them.
- Task 2 checks:
  - `gofmt -w internal/api/uploads_handlers.go internal/api/uploads_handlers_test.go`
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./internal/api -count=1 -timeout=120s` - passed.
- Task 3 complete: extracted viewer metadata and upload-payload construction helpers from the React submit callback, then added focused Jest coverage for trim, blank metadata value, empty metadata, and size parsing behavior.
- Task 3 checks:
  - `npm.cmd --prefix web/viewer run test -- uploadManager.test.tsx` - passed.
- Task 4 complete: marked `docs/cleanup-plan.md` task 7 complete with notes for backend multipart helpers and viewer payload builders.
- Task 4 checks:
  - `rg -n "Task 7|applyUploadMultipartField|applyUploadMediaDefaults|buildUploadMetadata|buildUploadPayload" docs/cleanup-plan.md internal/api web/viewer TASKS.md PLAN.md` - passed.
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./internal/api -count=1 -timeout=120s` - passed.
  - `npm.cmd --prefix web/viewer run test -- uploadManager.test.tsx` - passed.
- Task 5 validation progress:
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./... -count=1 -timeout=120s` - passed.
  - `git diff --check` - passed with line-ending warnings only.
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh --viewer` - passed full repo verification with viewer lint/test included.
