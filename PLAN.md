## Scope (current change)
- Address GitHub issue #1241 by verifying and, if needed, hardening the quickstart wrapper tests against missing or unusable shell environments.
- Keep the change limited to `scripts/quickstart_test.go`, cleanup tracking docs, and root planning files unless the read-only pass uncovers a real wrapper behavior gap.
- Preserve wrapper behavior coverage for delegating to `go run ./cmd/bitriver quickstart` and for failing clearly when CLI sources are missing.

## Assumptions
- `scripts/quickstart_test.go` may already satisfy the issue because it now discovers `BITRIVER_TEST_BASH`, Git Bash install paths, and `exec.LookPath("bash")` before skipping with a precise reason.
- The test should require a usable Bash only for the Unix shell wrapper behavior; no supported host should fail before the wrapper logic is exercised when Bash is available.
- If targeted tests pass and the helper behavior is already present, the appropriate change is to update cleanup tracking rather than invent more test harness complexity.

## Risks
- Tightening shell discovery unnecessarily could make the tests depend on local Windows installation details again.
- Marking the cleanup task complete without running targeted tests would hide a real host-specific regression.
- The full repo gate is still useful because `scripts` changes sit near CI and quickstart validation paths.

## Test plan
- `go test ./scripts -count=1 -timeout=120s`
- `go test ./... -count=1 -timeout=120s`
- `git diff --check`
- `./scripts/verify.sh`
