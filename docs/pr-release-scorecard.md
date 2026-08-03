# PR release scorecard

Every PR should make its release risk reviewable without reconstructing the change from commit history. The PR template contains a release scorecard for that purpose.

The scorecard is part of Gate 5 in `docs/release-gates.md`. `Merge gate` runs the
validator for every pull request. Warnings are advisory for docs/planning-only
paths and blocking when medium/high risk is selected or the diff touches code,
CI, dependencies, deployment, release packaging, or operator workflow paths.

## What to fill in

- **Change classification:** select every area touched by the PR. Use the highest-impact area when a change spans code and docs.
- **Risk level:** pick one risk level. Use medium for runtime behavior, operator workflow, config defaults, or release packaging. Use high for auth/security, migrations, ports, volumes, credentials, data-loss risk, or rollback-sensitive changes.
- **Evidence map:** list the exact commands or artifacts that prove the change. If a check could not run, select blocked/skipped checks and explain why.
- **Operator/release impact:** state whether docs, release notes, upgrade notes, rollback notes, or canary notes are needed.
- **Medium/high-risk prompts:** answer the prompts in the PR body or reviewer notes when they apply.

Codex-authored PRs must be explicit about skipped checks and environment blockers. Do not present an unrun command as evidence; list it under blocked/skipped checks with the host limitation or follow-up needed.

## Local validation

Run the advisory validator against a PR body draft:

```bash
./scripts/check-pr-release-scorecard.sh --body pr-body.md
```

Pass a newline-delimited changed-file list to catch obvious mismatches:

```bash
git diff --name-only main...HEAD > .tmp/changed-files.txt
./scripts/check-pr-release-scorecard.sh \
  --body pr-body.md \
  --changed-files .tmp/changed-files.txt
```

Use strict mode when a workflow or release manager should fail on warnings:

```bash
./scripts/check-pr-release-scorecard.sh \
  --body pr-body.md \
  --changed-files .tmp/changed-files.txt \
  --strict
```

Use the same risk-aware mode as pull-request CI with:

```bash
./scripts/check-pr-release-scorecard.sh \
  --body pr-body.md \
  --changed-files .tmp/changed-files.txt \
  --strict-if-risky
```

Do not paste pull-request body text into shell source. CI reads the body from
`GITHUB_EVENT_PATH`, writes it to a temporary file, and passes only that path to
the validator.

The validator checks for required scorecard sections, selected classification/risk/impact fields, evidence for medium/high-risk changes, and obvious changed-file mismatches such as migration changes without `data/migrations` selected.
