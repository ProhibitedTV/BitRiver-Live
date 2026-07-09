## Scoped change: add PR release scorecard (#1267)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Establish scorecard scope
  - Acceptance criteria:
    - `PLAN.md` captures #1267 scope, assumptions, risks, and test plan.
    - `TASKS.md` lists ordered tasks before source/doc edits for this pass.
    - Existing PR template, release-gate docs, and CI/doc scripts are reviewed.

- [x] Task 2 - Update PR template scorecard
  - Acceptance criteria:
    - PR template includes changed-area classification.
    - PR template includes risk level, evidence map, release/operator impact, and blocked-check disclosure.
    - Medium/high-risk prompts are concise and reviewable.

- [x] Task 3 - Add advisory scorecard validator
  - Acceptance criteria:
    - `./scripts/check-pr-release-scorecard.sh` validates a PR body file.
    - Optional changed-file input warns on obvious mismatches.
    - `--strict` exits non-zero for warnings; advisory mode exits zero with clear warnings.

- [x] Task 4 - Document Codex/release-gate expectations
  - Acceptance criteria:
    - Docs explain how Codex-authored PRs disclose risk, evidence, blocked checks, and operator impact.
    - `docs/release-gates.md` maps the scorecard to Gate 4.
    - Docs stay short and practical.

- [x] Task 5 - Verify and record results
  - Acceptance criteria:
    - Script syntax and sample validator runs pass.
    - Expected strict failure case fails.
    - `git diff --check` passes.

### Execution log
- Task 1 read-only pass:
  - Confirmed #1266 is closed after PR #1286; selected open issue #1267 as the next release-gate ticket under #1264.
  - Reviewed `.github/pull_request_template.md`, `docs/release-gates.md`, `scripts/check-ci-contract.sh`, docs consistency workflow, and Codex/test agent docs.
  - Chose an advisory-first implementation: PR template scorecard, local validation script, and documentation without workflow YAML changes.
- Task 2 implementation:
  - Expanded `.github/pull_request_template.md` with changed-area classification, risk level, evidence map, operator/release impact, blocked-check disclosure, and medium/high-risk review prompts.
- Task 3 implementation:
  - Added `scripts/check-pr-release-scorecard.sh` with advisory default mode, strict failure mode, required scorecard section checks, medium/high-risk evidence checks, and changed-file mismatch heuristics.
- Task 4 implementation:
  - Added `docs/pr-release-scorecard.md` with scorecard field guidance, Codex skipped-check expectations, and advisory/strict validator examples.
  - Updated `docs/release-gates.md` Gate 4 to reference the PR template, scorecard validator, and scorecard guide.
  - Updated `docs/codex-cli.md` with Codex-authored PR scorecard expectations.
- Task 5 verification:
  - Passed: `bash -n scripts/check-pr-release-scorecard.sh`.
  - Passed with expected advisory warnings: `./scripts/check-pr-release-scorecard.sh --body .github/pull_request_template.md --advisory`.
  - Passed: strict validation of a complete temporary scorecard with changed-file input.
  - Passed as expected failure: strict validation of an incomplete temporary deployment scorecard returned non-zero with warnings.
  - Passed: `git diff --check`.
  - Skipped: `shellcheck scripts/check-pr-release-scorecard.sh` because `shellcheck` is not installed on this host.
  - Blocked locally: `./scripts/verify.sh` stopped at `Env example placeholder hygiene` because this host has no `python3`, `python`, or Windows `py` launcher available.
