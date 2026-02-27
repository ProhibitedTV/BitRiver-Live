# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add `_FILE` secret resolver + validation integration
  - Acceptance criteria:
    - `cmd/bitriver/env_validation_helpers.go` includes helper logic that resolves `<KEY>`/`<KEY>_FILE` with deterministic precedence.
    - `validateEnv` applies resolver to all required secret-like keys and surfaces:
      - missing when neither direct nor file value is usable,
      - error when `_FILE` path is missing/unreadable/unusable.
    - Precedence behavior is documented in warning output/messages.

- [x] Task 2 — Add tests for `_FILE` behavior
  - Acceptance criteria:
    - `cmd/bitriver/env_validation_test.go` covers direct value only, `_FILE` only, both-set precedence, unreadable/missing path, and empty file content.
    - Tests assert missing/error/warning behavior for each case.

- [x] Task 3 — Update env/docs guidance
  - Acceptance criteria:
    - `deploy/.env.example` includes commented `_FILE` examples for sensitive variables.
    - `docs/secrets-hardening.md` and new `docs/security.md` describe `_FILE` workflow and precedence.

## Execution log
- ✅ Task 1 complete: added `_FILE` resolver support in `validateEnv` with direct-value precedence warning and error reporting for unreadable secret files.
- ✅ Task 1 check:
  - `go test ./cmd/bitriver -count=1`

- ✅ Task 2 complete: added env validation tests for direct-only, file-only, both-set precedence, unreadable file, and empty file content.
- ✅ Task 2 check:
  - `go test ./cmd/bitriver -count=1`

- ✅ Task 3 complete: added `_FILE` examples in `deploy/.env.example`, documented workflow/precedence in `docs/secrets-hardening.md`, and created `docs/security.md`.
- ✅ Task 3 checks:
  - `./scripts/verify.sh`
