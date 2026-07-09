## Scope (current change)
- Address GitHub issue #1266 with a named golden-path release gate.
- Add `go run ./cmd/bitriver release smoke-gate` as the evidence-producing gate command.
- Add `./scripts/release-gate-smoke.sh` as the operator/CI wrapper for the command.
- Wire the fast tier into the existing `scripts/test-all.sh` quickstart path so PR CI runs it when quickstart/deploy/runtime paths change.
- Document how fast, full, packaged, and upgrade-path evidence map to Gate 3 without changing deployment contract files.

## Assumptions
- The first pass should compose existing checks instead of duplicating quickstart, smoke, contract snapshot, or upgrade-plan logic.
- Fast PR tier should stay bounded: version evidence, env redaction summary, contract snapshot, and Compose config output.
- Full release-candidate tier can run the source quickstart plus smoke and collect Compose state/log artifacts.
- Packaged launcher and real baseline-to-target upgrade execution are slower release/nightly concerns; this pass should document staged follow-up evidence and provide an upgrade-plan artifact hook.
- Existing CI path filters already enable quickstart-focused validation through `BITRIVER_TEST_QUICKSTART=1`; `scripts/test-all.sh` is the right centralized entrypoint for the new fast tier.

## Risks
- Running the full Compose quickstart on every PR would be too slow; keep full mode opt-in.
- Artifact files must avoid leaking secrets; env evidence should report redaction coverage, not values.
- Failure output must name the failed phase and artifact path so operators can debug without guessing.
- CI script wiring changes required checks for quickstart-path PRs, so local verification must include the new wrapper and relevant Go tests.

## Test plan
- `gofmt -w cmd/bitriver/release_contract.go cmd/bitriver/release_contract_test.go`
- `go test ./cmd/bitriver -run "TestRunRelease|TestReleaseSmokeGate" -count=1 -timeout=120s`
- `go run ./cmd/bitriver release smoke-gate --tier fast --artifact-dir .tmp/release-gate-smoke --target v0.0.0-test`
- `./scripts/release-gate-smoke.sh --tier fast --artifact-dir .tmp/release-gate-wrapper --target v0.0.0-test`
- `BITRIVER_VERIFY_SOURCE_ONLY=1 ./scripts/verify.sh`
- `bash -n scripts/release-gate-smoke.sh scripts/test-all.sh`
- `git diff --check`
