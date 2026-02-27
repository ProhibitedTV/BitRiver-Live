# PLAN

## Scope (current change)
- Update `scripts/check-go-workflow-config.sh` so workflow validation no longer requires literal `actions/setup-go@v5`.
- Accept either SHA-pinned direct `actions/setup-go@<40-hex>` usage in each target workflow or approved local composite action usage (`./.github/actions/setup-go`) that pins `actions/setup-go` by SHA.
- Keep existing enforcement for `go-version-file: go.mod` and offline Go environment guards (`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`).
- Update the related note in `docs/testing.md` to describe SHA-based setup-go enforcement.

## Assumptions
- The approved local action remains `./.github/actions/setup-go` and its implementation is in `.github/actions/setup-go/action.yml`.
- Existing target workflows remain the same list currently hardcoded in `scripts/check-go-workflow-config.sh`.
- Text-based checks are acceptable for this script (no YAML parser dependency required).

## Risks
- Regex checks may accidentally miss valid YAML formatting variants if patterns are too strict.
- Local action validation could pass incorrectly if the script does not verify SHA pinning inside the composite action file.
- Overly broad matching could allow non-approved local actions.

## Test plan
- Run `scripts/check-go-workflow-config.sh` and confirm it passes against current workflows.
- Validate that the script still checks `go-version-file: go.mod` and offline guards by reviewing logic and pass output.
- Verify updated wording in `docs/testing.md` mentions SHA-pinned direct setup-go usage or approved local composite action pinning.
