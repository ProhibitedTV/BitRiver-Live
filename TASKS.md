## Scoped change: issue #1242 transcoder test stability

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Record the transcoder stability plan
  - Acceptance criteria:
    - `PLAN.md` captures issue #1242 scope, assumptions, risks, and validation commands.
    - `TASKS.md` lists ordered implementation and validation tasks before source edits continue.
    - The read-only pass identifies the FFmpeg stub setup, upload publish test, health recovery tests, and cleanup-plan task.

- [x] Task 2 - Replace sleep-driven health process exits with test-controlled completion
  - Acceptance criteria:
    - Health recovery tests complete the stub process explicitly after submit calls return.
    - Tests no longer depend on the `10ms` goroutine sleep or an atomic error pointer.
    - Running-process assertions observe cleanup after the exit handler runs.

- [x] Task 3 - Make upload publish completion deterministic
  - Acceptance criteria:
    - `TestUploadPublishesHTTPPlayback` does not rely on a real subprocess finishing within a broad wall-clock window.
    - Stub HLS output is still generated from the same FFmpeg argument parser used by the helper process.
    - Persisted metadata and published files remain asserted.

- [x] Task 4 - Update cleanup tracking
  - Acceptance criteria:
    - `docs/cleanup-plan.md` task 3 is marked complete after targeted tests pass.
    - Notes mention the deterministic process launcher and preserved runtime behavior.

- [x] Task 5 - Validate and publish
  - Acceptance criteria:
    - Targeted transcoder tests, full transcoder tests, full Go tests, diff hygiene, and the repo verification gate pass or blockers are recorded.
    - Changes are committed, pushed, opened as a draft PR, CI is checked, and the PR is merged when green.

### Execution log (issue #1242 transcoder test stability)
- Task 1 complete: after merging PR #1248 and syncing `main` to `9c889485`, selected issue #1242, created branch `codex/issue-1242-transcoder-test-stability`, fetched the issue, and audited `cmd/transcoder/main_test.go`, `cmd/transcoder/main.go`, and `docs/cleanup-plan.md` before source edits.
- Task 1 checks:
  - GitHub connector: fetched issue #1242.
  - `git checkout main`
  - `git pull --ff-only origin main`
  - `git checkout -b codex/issue-1242-transcoder-test-stability`
  - `Get-Content cmd/transcoder/AGENTS.md`
  - `Get-Content cmd/transcoder/main_test.go`
  - `Get-Content cmd/transcoder/main.go`
  - `rg -n "startStubTranscoder|useStubFFmpeg|TestUploadPublishesHTTPPlayback|TestHealthTracksFFmpegFailuresAndRecovery|TestHealthDegradedWhenPublishFailsAndRecovers|TestHealthDegradedWhenUploadPublishFails|atomic\\." cmd/transcoder/main_test.go`
- Task 2 complete: replaced the sleep-driven `startStubTranscoder` exit goroutine with a controlled launcher that records started processes and lets tests call `complete` with the intended exit error after submit handlers return.
- Task 2 checks:
  - `gofmt -w cmd/transcoder/main_test.go`
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./cmd/transcoder -run "TestHealthTracksFFmpegFailuresAndRecovery|TestHealthDegradedWhenPublishFailsAndRecovers|TestHealthDegradedWhenUploadPublishFails" -count=1 -timeout=120s` - passed.
- Task 3 complete: `TestUploadPublishesHTTPPlayback` now uses the controlled launcher to write stub HLS output through `runFFmpegStub(plan.args)` and complete the upload job directly instead of relying on an external FFmpeg stub process to finish within broad wall-clock waits.
- Task 3 checks:
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./cmd/transcoder -run TestUploadPublishesHTTPPlayback -count=1 -timeout=120s` - passed.
- Task 4 complete: marked `docs/cleanup-plan.md` task 3 complete with notes for the Windows `ffmpeg.exe` stub path, deterministic health process completion, and deterministic upload-publish fixture completion.
- Task 4 checks:
  - `rg -n "Task 3|TestUploadPublishesHTTPPlayback|test-controlled" docs/cleanup-plan.md TASKS.md PLAN.md` - passed.
- Task 5 validation progress:
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./cmd/transcoder -run "TestUploadPublishesHTTPPlayback|TestHealthTracksFFmpegFailuresAndRecovery|TestHealthDegradedWhenPublishFailsAndRecovers|TestHealthDegradedWhenUploadPublishFails" -count=1 -timeout=120s` - passed.
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./cmd/transcoder -count=1 -timeout=120s` - passed.
  - `$env:GOCACHE='C:\Users\RhythmicCarnage\Desktop\BitRiver-Live\.codex-tmp\go-build'; go test ./... -count=1 -timeout=120s` - passed.
  - `git diff --check` - passed with line-ending warnings only.
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh` - passed full repo verification; viewer checks were skipped because no viewer files changed.
- Task 5 complete: committed and pushed the branch, opened draft PR #1249, and confirmed GitHub CI passed on the PR head before merge.
- Task 5 publishing:
  - `git add PLAN.md TASKS.md cmd/transcoder/main_test.go docs/cleanup-plan.md`
  - `git commit -m "transcoder: stabilize test fixtures"`
  - `git push -u origin codex/issue-1242-transcoder-test-stability`
  - GitHub connector: opened draft PR #1249.
  - GitHub connector: CI completed successfully for the PR head.
