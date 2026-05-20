## Scope (current change)
- Address GitHub issue #1242 by stabilizing transcoder test fixtures and health recovery polling in `cmd/transcoder/main_test.go`.
- Keep runtime transcoder behavior unchanged; this change should stay in tests and cleanup tracking docs.
- Cover the observed flaky upload publish path by completing the stubbed transcode deterministically instead of waiting on a subprocess race.
- Confirm the existing FFmpeg stub lookup remains platform-aware for Windows extension behavior and avoid string-only path comparisons.

## Assumptions
- The production launch path in `cmd/transcoder/main.go` is correct and should not need changes.
- `useStubFFmpeg` already uses `exec.LookPath("ffmpeg")` plus `os.SameFile`, which is the right shape for Windows `.cmd`/`.exe` resolution, but tests can avoid relying on it where process timing is not under test.
- Health recovery assertions should wait for explicit test-controlled process completion, then poll only for server-visible state.
- `docs/cleanup-plan.md` is the only source doc that needs a status update because runtime behavior and deployment contracts are unchanged.

## Risks
- Calling a test exit handler before the server records the process can leave stale process entries and hide the real health transition.
- Over-sharing one helper across live and upload tests could obscure whether a test is verifying process launch, publish behavior, or metadata persistence.
- Full repo verification may still exercise Docker and local environment gates; if a host dependency is unavailable, record the blocker rather than changing behavior.

## Test plan
- `go test ./cmd/transcoder -run "TestUploadPublishesHTTPPlayback|TestHealthTracksFFmpegFailuresAndRecovery|TestHealthDegradedWhenPublishFailsAndRecovers|TestHealthDegradedWhenUploadPublishFails" -count=1 -timeout=120s`
- `go test ./cmd/transcoder -count=1 -timeout=120s`
- `go test ./... -count=1 -timeout=120s`
- `git diff --check`
- `./scripts/verify.sh`
