# Cleanup Plan

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

Evidence collected on 2026-03-21:
- `go test ./... -count=1 -timeout=120s` currently fails in `cmd/transcoder`, `internal/executil`, and `scripts` on this Windows host.
- Read-only scans show several sleep-driven tests, duplicated context helpers, and oversized mixed-responsibility modules in the upload flow.

## Top hotspots

1. `internal/executil/executil_test.go` (`TestRunReturnsCommandErrorWithStderrTail`, `TestRunTrimsStderrTail`)
   - Why brittle/inefficient: the tests shell out to `sh`, `head`, and `/dev/zero`, so the current Windows run reports `ExitCode=-1` and an empty stderr tail instead of exercising `CommandError`.
   - Risk: medium
   - Proposed fix:
     - Replace POSIX-only shell commands with a helper subprocess implemented through the Go test binary.
     - Keep the same assertions on streamed stdout, stderr tail capture, and truncation length.
   - Verify:
     - `go test ./internal/executil -count=1 -timeout=120s`
     - `go test ./... -count=1 -timeout=120s`

2. `scripts/quickstart_test.go` (`TestQuickstartDelegatesToCli`, `TestQuickstartFailsWhenCliSourcesMissing`)
   - Why brittle/inefficient: both tests hard-code `bash`, and the current Windows run fails before the script logic executes with `Bash/Service/CreateInstance/E_ACCESSDENIED`.
   - Risk: high
   - Proposed fix:
     - Introduce a platform-aware harness that uses an available shell or skips with a precise reason when no supported shell exists.
     - Keep coverage focused on wrapper behavior instead of host-specific shell availability.
   - Verify:
     - `go test ./scripts -count=1 -timeout=120s`
     - `go test ./... -count=1 -timeout=120s`

3. `cmd/transcoder/main_test.go` (`useStubFFmpeg`, `TestHealthTracksFFmpegFailuresAndRecovery`, `TestHealthDegradedWhenPublishFailsAndRecovers`)
   - Why brittle/inefficient: the tests assume `exec.LookPath("ffmpeg")` resolves to a path without `.exe`, and several health checks rely on short wall-clock polling windows that currently miss recovery on Windows.
   - Risk: high
   - Proposed fix:
     - Make the FFmpeg stub assertion platform-aware.
     - Reduce timing sensitivity by waiting on deterministic state transitions instead of short fixed windows.
   - Verify:
     - `go test ./cmd/transcoder -count=1 -timeout=120s`
     - `go test ./... -count=1 -timeout=120s`

4. `internal/auth/session_test.go` (`TestSessionExpiration`, `TestValidateRefreshesIdleTimeout`, `TestValidateHonorsAbsoluteTTL`)
   - Why brittle/inefficient: these tests depend on `time.Sleep(10-70ms)`, which makes expiry behavior timing-sensitive and slower than necessary.
   - Risk: medium
   - Proposed fix:
     - Add a controllable clock seam to session-manager tests.
     - Replace sleep-based assertions with deterministic time advancement.
   - Verify:
     - `go test ./internal/auth -count=1 -timeout=120s`

5. `internal/server/server_test.go` (`TestRateLimiterCleanupStaleBucketsEventually`)
   - Why brittle/inefficient: the test sleeps for `1100ms` to cross a cleanup threshold, which slows the suite and makes cleanup behavior timing-dependent.
   - Risk: medium
   - Proposed fix:
     - Inject a clock or cleanup timestamp seam into the rate limiter.
     - Assert stale-bucket cleanup via deterministic time control.
   - Verify:
     - `go test ./internal/server -count=1 -timeout=120s`

6. `internal/storage/ingest_boot_helpers.go` (`runIngestBootWithRetry`)
   - Why brittle/inefficient: retry attempts always use `context.Background()` and `time.Sleep`, so caller cancellation is ignored between attempts and failures wait longer than necessary.
   - Risk: high
   - Proposed fix:
     - Thread caller context through retry attempts.
     - Replace bare sleeps with a cancellable timer/select path and add cancellation coverage.
   - Verify:
     - `go test ./internal/storage -count=1 -timeout=120s`

7. `internal/auth/postgres_store.go`, `internal/auth/postgres_mfa_challenge_store.go`, `internal/storage/postgres_repository.go`
   - Why brittle/inefficient: timeout/acquire-context helpers are duplicated across three Postgres-backed stores, which increases drift risk for cancellation and error-wrapping behavior.
   - Risk: medium
   - Proposed fix:
     - Extract shared timeout/context helpers where package boundaries allow.
     - Align error wrapping and timeout behavior with focused tests.
   - Verify:
     - `go test ./internal/auth ./internal/storage -count=1 -timeout=120s`

8. `internal/auth/context.go`, `internal/ctxutil/context.go`, `internal/observability/tracing/context.go`
   - Why brittle/inefficient: the same nil-context normalization helper exists in three places, creating easy drift for trivial behavior.
   - Risk: low
   - Proposed fix:
     - Reuse `internal/ctxutil.Normalize` where package boundaries permit.
     - Remove duplicate helpers once call sites are aligned.
   - Verify:
     - `go test ./internal/auth ./internal/ctxutil ./internal/observability/... -count=1 -timeout=120s`

9. `internal/storage/postgres_legal.go` (`ListDMCACases`, `GetDMCACase`, `UpdateDMCACase`, `ListDataSubjectRequests`, `ListLegalStateHistory`)
   - Why brittle/inefficient: these methods repeatedly use bare `context.Background()` and duplicate trim/query patterns, so database work ignores repository timeout policy and cancellation.
   - Risk: medium
   - Proposed fix:
     - Route legal-store queries through the repository acquire/timeout helpers.
     - Extract small normalization helpers for repeated trim/status logic.
   - Verify:
     - `go test ./internal/storage -count=1 -timeout=120s`

10. `internal/api/uploads_handlers.go` plus `web/viewer/components/UploadManager.tsx`
   - Why brittle/inefficient: the backend upload handler (`Uploads`, `createUploadFromMultipart`, `createUploadEntry`, `serveUploadMedia`, `deleteUploadMedia`) and the viewer upload UI (`UploadManager`) are both large mixed-responsibility units, which makes error paths and state transitions hard to reason about.
   - Risk: medium
   - Proposed fix:
     - Extract pure helpers for upload parsing/state mapping before touching behavior.
     - Add focused unit tests around those helpers so future refactors stop depending on broad end-to-end coverage.
   - Verify:
     - `go test ./internal/api -count=1 -timeout=120s`
     - `npm.cmd --prefix web/viewer run test -- uploadManager.test.tsx`

## Ordered tasks

- [x] Task 1 — Make `internal/executil` tests platform-neutral
  - Acceptance criteria:
    - `internal/executil/executil_test.go` no longer depends on `sh`, `head`, or `/dev/zero`.
    - `go test ./internal/executil -count=1 -timeout=120s` passes on this Windows host.
    - `go test ./... -count=1 -timeout=120s` no longer fails in `internal/executil`.
  - Notes:
    - Replaced the POSIX-only shell commands with a helper subprocess launched through the Go test binary (`TestExecutilHelperProcess`), so the package still asserts streamed stdout, captured stderr, and stderr-tail trimming without depending on Unix tools.
  - Follow-ups:
    - The full Go suite still fails in `cmd/transcoder` and `scripts`, which lines up with tasks 2 and 3 below.

- [x] Task 2 — Harden `scripts` quickstart tests against shell availability
  - Tracking issue: [#1241](https://github.com/ProhibitedTV/BitRiver-Live/issues/1241)
  - Acceptance criteria:
    - `scripts/quickstart_test.go` no longer fails before exercising wrapper logic on Windows.
    - Any skip path is explicit and limited to genuinely unsupported hosts.
  - Notes:
    - Verified on 2026-05-20 that `scripts/quickstart_test.go` already discovers `BITRIVER_TEST_BASH`, Git Bash install paths, and `exec.LookPath("bash")`, then skips with a precise reason only when no usable Bash is available.

- [x] Task 3 — Stabilize `cmd/transcoder` test fixtures and health recovery polling
  - Tracking issue: [#1242](https://github.com/ProhibitedTV/BitRiver-Live/issues/1242)
  - Acceptance criteria:
    - The FFmpeg stub setup works cross-platform.
    - Health recovery tests stop depending on narrow wall-clock windows.
  - Notes:
    - Completed on 2026-05-20 by using a Windows `ffmpeg.exe` stub path where extension resolution matters and by replacing sleep-driven health exits with test-controlled process completion.
    - `TestUploadPublishesHTTPPlayback` now completes the stubbed upload transcode deterministically while still generating HLS fixtures through the shared FFmpeg argument parser.

- [x] Task 4 — Replace sleep-driven auth/server timing tests with deterministic clock control
  - Tracking issue: [#1243](https://github.com/ProhibitedTV/BitRiver-Live/issues/1243)
  - Acceptance criteria:
    - `internal/auth/session_test.go` and the stale-bucket rate-limiter test no longer use wall-clock sleeps.
    - Runtime behavior remains unchanged.
  - Notes:
    - Completed on 2026-05-20 after verifying the auth session timing tests already use a deterministic test clock.
    - The in-memory rate limiter now has a private clock hook for deterministic tests while defaulting to wall-clock time in runtime code.

- [x] Task 5 — Make ingest/Postgres helpers cancellation-aware and reduce duplicated timeout plumbing
  - Tracking issue: [#1244](https://github.com/ProhibitedTV/BitRiver-Live/issues/1244)
  - Acceptance criteria:
    - `runIngestBootWithRetry` respects caller cancellation between attempts.
    - Shared timeout/context logic is reduced across Postgres-backed stores without changing public APIs.
  - Notes:
    - Completed on 2026-05-20 by threading a parent context through ingest boot retries and replacing retry sleeps with a cancellable timer.
    - Auth Postgres session and MFA challenge stores now share package-private timeout/ping helpers.

- [x] Task 6 — Route Postgres legal flows through repository timeout helpers
  - Tracking issue: [#1245](https://github.com/ProhibitedTV/BitRiver-Live/issues/1245)
  - Acceptance criteria:
    - Legal repository queries stop using bare `context.Background()` for DB operations.
    - Existing legal behavior stays unchanged under targeted storage tests.
  - Notes:
    - Completed on 2026-05-20 by moving Postgres legal create/list/get/update/audit/history queries onto `withConn`, so they now share repository acquire timeout and connection-release behavior.
    - Focused trim/status and scan helpers reduce repeated normalization while keeping public legal APIs unchanged.

- [ ] Task 7 — Extract pure upload helpers from backend/frontend upload flows
  - Tracking issue: [#1246](https://github.com/ProhibitedTV/BitRiver-Live/issues/1246)
  - Acceptance criteria:
    - `internal/api/uploads_handlers.go` and `web/viewer/components/UploadManager.tsx` each shed at least one pure helper/state-mapping slice.
    - New focused tests lock in current upload behavior before broader refactors.
