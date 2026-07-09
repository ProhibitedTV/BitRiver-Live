## Scope (current change)
- Address GitHub issue #1267 with an advisory PR release scorecard and risk gate.
- Update `.github/pull_request_template.md` with a concise release scorecard that Codex/humans can fill in.
- Add `./scripts/check-pr-release-scorecard.sh` to validate obvious scorecard omissions from a PR body and optional changed-file list.
- Document how Codex-authored PRs should disclose risk, evidence, blocked checks, and operator/release impact.
- Map the scorecard to Gate 4 in `docs/release-gates.md`.

## Assumptions
- The first version should be advisory by default to avoid noisy false positives; `--strict` can make warnings fail.
- The validator should work from plain files so it can be used locally, in PR tooling, or by future workflow wiring.
- Medium/high-risk PRs should list concrete evidence or explicitly disclose blocked checks.
- Changed-file heuristics should catch obvious mismatches only: deploy/env, migrations, CI/build, security/auth, release artifacts, docs.
- No workflow YAML changes are required in this pass because the issue accepts a script-based validator.

## Risks
- Overly broad parsing could produce review noise; keep checks explicit and explain remediation.
- A scorecard can become wall-of-text; keep the PR template short and checkbox-driven.
- The script must not require GitHub API access or hidden CI context to be useful.
- Advisory output must still be clear enough that reviewers can decide when to block manually.

## Test plan
- `bash -n scripts/check-pr-release-scorecard.sh`
- `./scripts/check-pr-release-scorecard.sh --body .github/pull_request_template.md --advisory`
- `./scripts/check-pr-release-scorecard.sh --body <temp complete body> --changed-files <temp changed files> --strict`
- `./scripts/check-pr-release-scorecard.sh --body <temp incomplete body> --changed-files <temp changed files> --strict` should fail
- `git diff --check`
