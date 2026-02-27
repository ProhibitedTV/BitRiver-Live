# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add env-example placeholder lint script
  - Acceptance criteria:
    - New script validates required credentials in `deploy/.env.example` are present and non-empty.
    - Script parses required credential keys from `x-required-credentials` in `deploy/docker-compose.yml`.
    - Script rejects unsafe secret-looking values unless they follow explicit sample marker conventions.

- [x] Task 2 — Wire placeholder lint into verification + CI path
  - Acceptance criteria:
    - `scripts/verify.sh` runs the new check as part of standard verification.
    - CI path that depends on verify (`scripts/test-all.sh`/workflows) enforces the check without duplicate bespoke steps.

- [x] Task 3 — Document placeholder conventions
  - Acceptance criteria:
    - `docs/secrets-hardening.md` documents accepted sample marker conventions for secret-bearing placeholders.
    - `docs/security.md` includes/links placeholder hygiene expectations and references verification command(s).

## Execution log
- ✅ Task 1 complete: added `scripts/check-env-example-placeholders.sh` and aligned `deploy/.env.example` admin password placeholder marker.
- ✅ Task 1 check:
  - `./scripts/check-env-example-placeholders.sh`

- ✅ Task 2 complete: wired placeholder hygiene check into `scripts/verify.sh` (therefore CI paths that invoke verify/test-all also enforce it).
- ✅ Task 2 check:
  - `./scripts/verify.sh` (initial run surfaced stale generated contract docs)

- ✅ Task 3 complete: documented placeholder conventions in `docs/secrets-hardening.md` and created `docs/security.md` guardrails page.
- ✅ Task 3 checks:
  - `./scripts/generate-contract-doc.sh`
  - `./scripts/verify.sh` (final pass)
