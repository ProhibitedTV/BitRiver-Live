# PLAN

## Scope
- Resolve production blocker #1294 by removing plaintext production secrets from release artifacts and cross-job transfer.
- Keep production secret validation job-local and ephemeral while preserving missing-value, malformed-value, image-digest, and `_FILE` validation behavior.
- Validate the non-secret deployment contract and tracked OME configuration with placeholder inputs only.
- Retain only redacted release evidence and a deterministic artifact inventory, with short retention and automated secret scanning.
- Document the release threat model, evidence policy, and operator expectations.

## Assumptions
- GitHub Actions encrypted secrets remain the current release credential source; selecting an external secret manager is outside this issue.
- The canonical root `.env` and Compose file do not need to change.
- Secret names and validation status are safe evidence; secret values, credential-bearing DSNs, rendered secret files, and private key material are not.
- Release build artifacts can be scanned centrally before publication, with archive contents inspected where standard runner tools support them.

## Analysis Update
- A placeholder render exposed substantive drift in `deploy/ome/Server.generated.xml`: its tracked access token was credential-shaped rather than the sample value in `deploy/.env.example`.
- Because the generated XML is copied into release packages, refresh that file from the canonical example and document rotation expectations. Do not change Compose shape or root environment behavior.

## Risks
- Redacting validation output too aggressively can hide actionable failures; report the variable or rule name without reporting its value.
- Pattern-only scanners can miss encoded or package-specific payloads; combine known-value sentinel scanning, forbidden-file checks, secret-shaped content checks, and archive inspection.
- Cleanup steps can be skipped after a failed validation unless they use `if: always()` and runner-temporary paths.
- OME freshness checks can accidentally rewrite the tracked contract; render to a temporary output and compare without mutating the worktree.
- A credential already committed in generated config remains in history; operators must rotate any environment that reused it even after the tracked file is sanitized.
- Workflow-only failures are expensive to discover; add local tests for scanner behavior and static release workflow invariants.

## Test Plan
- Focused Go tests for release evidence scanning and release workflow invariants.
- Scanner fixtures proving safe redacted evidence passes and `.env`, private keys, credential-bearing DSNs, sentinel values, and leaked archive contents fail without echoing secret values.
- `git diff --check`.
- `./scripts/verify.sh` (or the Windows wrapper when host prerequisites require it).
- GitHub release/CI checks on the pull request before squash merge.

## Boundaries
- Do not modify `deploy/docker-compose.yml` or root `.env`.
- Limit generated OME changes to replacing the discovered credential-shaped value with the canonical example render; update contract/security docs in the same PR.
- Do not introduce a new secret manager or replace the canonical `.env` plus Compose deployment contract.
- Do not stage or modify unrelated local deployment-guide and diagnostics files.
