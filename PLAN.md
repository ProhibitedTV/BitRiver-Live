## Scope (current change)
- Address GitHub issue #1265 with a modest first contract/schema drift gate.
- Add a repeatable `cmd/bitriver` release contract snapshot command that emits stable JSON for the operator-facing contract.
- Add a diff command that compares two snapshots and classifies additive, removal/breaking, and security-sensitive default drift.
- Document how to generate and review snapshots without changing the deployment contract itself.

## Assumptions
- The first version should be useful and small: env template keys/defaults/comments, Compose service shape, migration file list, generated artifact presence, and health endpoint names are enough to catch practical drift.
- Snapshot generation should not require a running stack or Docker daemon; it can parse files in the checkout directly.
- The checked-in deployment contract files remain unchanged in this pass.
- Future CI workflow wiring and PR comments can build on the command added here.

## Risks
- A drift gate that depends on unstable formatting will create noisy diffs.
- Parsing Compose YAML without adding dependencies is limited; the first pass should capture stable service names and obvious contract fields, not pretend to fully understand every YAML feature.
- Secret-like env values must be treated as defaults from `deploy/.env.example`, not real secrets from root `.env`.
- Over-classifying additive drift as breaking would slow harmless changes.

## Test plan
- `gofmt -w cmd/bitriver/release_contract.go cmd/bitriver/release_contract_test.go cmd/bitriver/main.go`
- `go test ./cmd/bitriver -run "TestRunRelease|TestBuildContractSnapshot|TestDiffContractSnapshots" -count=1 -timeout=120s`
- `go run ./cmd/bitriver release contract-snapshot --env-file deploy/.env.example --compose-file deploy/docker-compose.yml --output .tmp/contract-snapshot.json`
- `go run ./cmd/bitriver release contract-diff --base .tmp/contract-snapshot.json --head .tmp/contract-snapshot.json`
- `git diff --check`
