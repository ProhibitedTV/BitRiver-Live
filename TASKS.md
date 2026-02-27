# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add operator security entrypoint doc (`docs/security.md`)
  - Acceptance criteria:
    - Includes all required sections: threat model summary, network exposure guidance, reverse proxy + TLS recommendation, auth/session/cookie settings guidance, admin bootstrap practices, secret rotation approach, logging guidance, and production checklist.
    - Guidance is stack-specific and references `deploy/docker-compose.yml`, `deploy/.env.example`, and `cmd/bitriver env validate` where relevant.
    - Includes service/port exposure table and explicit callouts that `postgres-host` and `srs-api` profiles are debug-only.

- [x] Task 2 — Add discoverability links to security doc
  - Acceptance criteria:
    - Add link(s) from `README.md` and/or `docs/operations.md` so operators can quickly find `docs/security.md`.
    - Link placement is near other operator-facing runbook references.

## Execution log
- ✅ Task 1 complete: created `docs/security.md` as the operator-facing security entrypoint with all required sections and stack-specific guidance.
- ✅ Task 1 check:
  - `rg -n "^## 1\)|^## 2\)|^## 3\)|^## 4\)|^## 5\)|^## 6\)|^## 7\)|^## 8\)|postgres-host|srs-api|env validate" docs/security.md`

- ✅ Task 2 complete: added discoverability links to `docs/security.md` from `README.md` and `docs/operations.md`.
- ✅ Task 2 checks:
  - `./scripts/verify.sh`
