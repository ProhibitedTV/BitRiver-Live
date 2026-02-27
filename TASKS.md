# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Compose network segmentation + hardening defaults
  - Acceptance criteria:
    - `deploy/docker-compose.yml` defines `public` and `internal` networks and attaches services per exposure role.
    - Long-running services include `security_opt: ["no-new-privileges:true"]`.
    - Safe services include `cap_drop: ["ALL"]` and `read_only: true` with explicit writable mounts as needed.
    - Non-root users are set where known-safe; inline exceptions explain root/mutable FS cases.

- [x] Task 2 — Hardening documentation updates
  - Acceptance criteria:
    - Add/update `docs/security.md` with a concise “container hardening defaults + exceptions” section mapping exact services to rationale.
    - Update `docs/contract.md` where needed so deployment contract docs reflect compose networking/hardening behavior.

## Execution log
- ✅ Task 1 complete: updated `deploy/docker-compose.yml` with explicit `public`/`internal` networks, conservative dual-attachments, `no-new-privileges`, `cap_drop`, read-only rootfs where feasible, explicit tmpfs writable paths, non-root users on supported images, and inline exception comments for compatibility-sensitive services.
- ✅ Task 1 checks:
  - `./scripts/verify.sh` (pass; docker-dependent checks skipped because docker is unavailable in environment)

- ✅ Task 2 complete: added `docs/security.md` with hardening defaults/exceptions and updated `docs/contract.md` contract definition to mention compose hardening/network segmentation baseline.
- ✅ Task 2 checks:
  - `./scripts/verify.sh` (pass; docker-dependent checks skipped because docker is unavailable in environment)
