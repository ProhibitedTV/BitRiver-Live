## Scope (current change)
- Address GitHub issue #1268 with a pragmatic post-deploy canary, observability, and rollback gate.
- Add `go run ./cmd/bitriver release canary` for release-candidate/staging stacks that are already running.
- Check deployed HTTP health surfaces from a base URL, save redacted response artifacts, and fail on high-confidence unhealthy responses.
- Scan supplied logs for conservative fatal/error patterns and write canary evidence artifacts.
- Validate rollback-note readiness when requested, without changing deployment contract files.
- Document how Gate 6 should be run after a release-candidate deploy.

## Assumptions
- The first version should run against an already-started stack; it should not start, stop, or mutate Compose services.
- The command should be useful without Docker by checking HTTP endpoints from `--base-url`.
- Log scanning should work from a supplied log file first; Compose log collection can remain a documented operator step.
- Rollback notes should be optional by default for local canaries but enforceable with `--require-rollback-notes`.
- No deployment contract changes are required for this pass.

## Risks
- Endpoint response shapes may vary; only fail on transport/HTTP failures and clear unhealthy/degraded status fields.
- Log scanning can become noisy; keep patterns conservative and attach matching lines to the report.
- A canary command can imply production automation that does not exist; docs must keep it scoped to the single-host operator path.
- Local verification may not hit a live stack on this host, so tests should cover the command behavior with controlled HTTP/log fixtures.

## Test plan
- `gofmt -w cmd/bitriver/release_contract.go cmd/bitriver/release_canary.go cmd/bitriver/release_canary_test.go`
- `go test ./cmd/bitriver -run "TestRunRelease|TestReleaseCanary" -count=1 -timeout=120s`
- `go run ./cmd/bitriver release canary --base-url <test server> --logs-file <temp clean logs> --rollback-notes <temp rollback notes> --require-rollback-notes --artifact-dir <temp artifacts>`
- `bash -n scripts/release-canary.sh`
- `git diff --check`
