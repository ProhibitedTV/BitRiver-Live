# PLAN

## Scope (current change)
- Pin `.github/workflows/ci.yml` `dorny/paths-filter` usage from floating `@v3` to a full commit SHA with a release comment.
- Audit all workflow files under `.github/workflows/` and pin any remaining third-party `uses:` entries that are not already full SHAs.

## Assumptions
- Existing workflow pinning convention is `uses: owner/repo@<40-char-sha> # vX.Y.Z` (or equivalent release label).
- Local composite actions (`uses: ./.github/actions/...`) are intentionally unpinned and should remain unchanged.

## Risks
- Pinning to an incorrect SHA could break CI behavior; use the upstream tag commit for the intended release.
- Missing another non-SHA `uses:` line would leave policy drift in workflows.

## Test plan
- Run a repository scan to list workflow `uses:` entries and confirm no non-SHA external actions remain.
- Optionally run `./scripts/check-ci-contract.sh` to ensure CI workflow contract checks still pass.
