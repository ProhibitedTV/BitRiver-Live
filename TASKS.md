# TASKS

## Scoped change: secret-safe production release evidence (#1294)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Establish the release secret boundary
  - Acceptance criteria:
    - `PLAN.md` records scope, assumptions, risks, tests, and contract boundaries.
    - `SPEC.md` identifies secret-safe release evidence as the current success target.
    - The current release, quickstart, canary, diagnostics, and packaging artifact flows are reviewed before implementation.
  - Check:
    - Read-only workflow, validation-script, release-gate, canary, artifact, and documentation review completed.
    - `git diff --check -- PLAN.md TASKS.md SPEC.md` passed.

- [x] Task 2 - Add a release evidence scanner
  - Acceptance criteria:
    - A reusable scanner rejects forbidden secret files, known sentinel values, private keys, credential-bearing URLs, and secret-shaped assignments.
    - Supported archives are inspected recursively.
    - Findings identify the file and rule without printing the secret value.
    - Automated tests cover safe and unsafe fixtures.
  - Check:
    - `bash -n scripts/scan-release-evidence.sh` passed.
    - `go test ./scripts -run ReleaseEvidence -count=1 -timeout=120s` passed (workspace-local Go cache used because the host default cache is damaged).

- [x] Task 3 - Keep production secret validation job-local
  - Acceptance criteria:
    - The release workflow no longer uploads or downloads a populated `.env`.
    - Missing, malformed, digest, and `_FILE`-compatible secret inputs still fail in the owning job.
    - Validation output is scanned and only redacted status evidence is retained.
    - Temporary secret files are removed in an always-running cleanup step.
  - Check:
    - No `release-env`, `.env` artifact path, or verified-environment download remains in `.github/workflows/release.yml`.
    - `go test ./cmd/bitriver -run 'TestValidateEnv.*File|TestResolveEnv.*File' -count=1 -timeout=120s` passed, covering direct and `_FILE` secret resolution behavior.
    - `git diff --check -- .github/workflows/release.yml scripts/scan-release-evidence.sh scripts/release_evidence_test.go PLAN.md SPEC.md TASKS.md` passed.

- [x] Task 4 - Gate release artifacts and retained evidence
  - Acceptance criteria:
    - OME freshness uses non-secret fixture inputs and a temporary output.
    - Release artifacts are inventoried and scanned before publication.
    - Temporary build and evidence artifacts have explicit short retention periods.
    - Static tests prevent reintroducing `release-env` transfer or unscanned release evidence.
  - Check:
    - `go test ./scripts -run 'Release(Evidence|Workflow)' -count=1 -timeout=120s` passed.
    - `js-yaml` parsed `.github/workflows/release.yml` with all 11 jobs present.
    - All 10 `upload-artifact` steps have explicit retention; temporary build artifacts retain 7 days and evidence retains 3 days.
    - A temporary OME render from `deploy/.env.example` byte-matches the refreshed tracked generated XML.
    - `git diff --check` passed.

- [x] Task 5 - Document the release threat model
  - Acceptance criteria:
    - Security and production release docs prohibit secret-bearing CI artifacts and logs.
    - Docs define allowed redacted evidence, cleanup expectations, sentinel scanning, and `_FILE` compatibility.
    - Release-gate documentation names the retained artifact inventory and scan result.
  - Check:
    - Stale guidance to persist or reuse a verified release `.env` was removed from source-of-truth docs.
    - `git diff --check -- docs/contract.md docs/production-release.md docs/release-gates.md docs/secrets-hardening.md docs/security.md PLAN.md TASKS.md` passed.
    - Contract docs now require tracked and packaged OME XML to remain placeholder-only and deployment-time renders to remain local.

- [-] Task 6 - Verify, publish, and close #1294
  - Acceptance criteria:
    - Focused tests, repository verification, and diff hygiene pass or blockers are recorded.
    - Pull request checks pass before squash merge.
    - The issue is closed by the merged pull request and local `main` is synchronized.
  - Check:
    - `go test ./... -count=1 -timeout=120s` passed with a workspace-local Go cache.
    - Focused scanner/workflow and `_FILE` validation suites passed.
    - `js-yaml` parsed the 11-job release workflow and `git diff --check` passed.
    - `docker compose --env-file .env -f deploy/docker-compose.yml config --quiet` passed.
    - `./scripts/test-quickstart.sh` passed after release-mode images built, migrations completed, all services became healthy, and API/viewer probes succeeded.
    - `./scripts/verify.sh` passed the go.sum and CI contract phases, then stopped at env placeholder hygiene because this host has no Python 3 interpreter; equivalent available gates were run separately and CI remains required before merge.

### Execution log
- Task 1 analysis:
  - Confirmed `.github/workflows/release.yml` creates a fully populated production `.env`, uploads it as `release-env`, and downloads it in `go-tests` for OME freshness validation.
  - Confirmed downstream release jobs only require the validation gate; they do not need production credential values.
  - Confirmed existing security guidance permits a generated CI `.env` artifact and must be corrected.
  - Kept the canonical Compose, root environment, and generated OME deployment contract out of scope.
- Task 2 implementation:
  - Added a reusable release evidence scanner with exact known-value detection, forbidden filename checks, private-key and secret-shape rules, and recursive archive inspection.
  - Added a deterministic SHA-256 inventory mode and diagnostics that report only rule identifiers and relative paths.
  - Added Go fixtures for safe redacted evidence, direct leak classes, output non-disclosure, and an archived sentinel leak.
- Task 3 implementation:
  - Replaced the cross-job `.env` artifact with a runner-temporary validation input and exact-value sentinel list.
  - Captured validation output privately, scanned it before display, retained only fixed-schema status evidence, and added an always-running cleanup step.
  - Removed the downstream environment artifact download while preserving production validation, digest enforcement, and existing `_FILE` resolution behavior.
- Task 4 analysis:
  - Placeholder OME rendering exposed a credential-shaped access token in the tracked generated XML, which is copied into release packages.
  - Expanded the scoped contract work only enough to refresh that generated file from `deploy/.env.example`; Compose and root `.env` remain untouched.
- Task 4 implementation:
  - Added non-mutating OME freshness validation against `deploy/.env.example` and sanitized the tracked generated XML to that canonical placeholder render.
  - Added final downloaded-artifact and publication-payload scans plus a SHA-256 artifact inventory before GitHub Release publication.
  - Added explicit short retention to every workflow artifact and static tests that prevent plaintext env transfer, missing scans, or retention drift.
- Task 5 documentation:
  - Replaced unsafe guidance to upload a generated CI environment file with a job-local validation and cleanup model.
  - Added release CI threat boundaries, sentinel and artifact scanning, evidence retention, rotation response, and `_FILE` compatibility guidance.
  - Updated release-gate and deployment contract docs for redacted evidence, inventory review, and placeholder-only packaged OME config.
- Task 6 verification:
  - Hardened scanner review added mixed-line bypass fixtures, XML credential detection, and a credential-only sentinel allowlist to avoid false positives from ordinary release values.
  - The first quickstart attempt inherited local offline Go settings and failed while downloading real pgx; rerunning the canonical command without those overrides passed end to end.
  - Smoke cleanup restored the tracked generated XML from `HEAD`, so the canonical placeholder render was reapplied and rescanned afterward.
  - PR #1309 Shell lint found three `SC2016` info findings for intentionally literal `${` placeholder checks; replaced single-quoted literals with escaped double-quoted literals without changing scanner behavior.
