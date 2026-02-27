# PLAN

## Scope (current change)
- Harden `deploy/docker-compose.yml` conservatively while preserving default behavior.
- Add explicit `public`/`internal` network segmentation and service attachments.
- Apply container hardening defaults (`security_opt`, `cap_drop`, `read_only`, and non-root `user`) where feasible with inline exceptions.
- Document hardening defaults and service exceptions in `docs/security.md`.
- Keep deployment contract docs aligned for compose behavior in `docs/contract.md`.

## Assumptions
- Public-facing services are those with host-published ports or viewer-facing endpoints.
- Stateful services (`postgres`, `redis`) and control/config jobs should remain isolated on `internal` unless a public service must reach them.
- Some vendor images may require root/mutable filesystems; those will be documented inline as exceptions.

## Risks
- Over-restricting filesystems/capabilities can break entrypoints, healthchecks, or runtime writes.
- Network over-segmentation could block required API calls between control-plane and ingest/transcoding services.
- Non-root execution may fail for images expecting privileged startup paths.

## Test plan
- `docker compose -f deploy/docker-compose.yml config`
- `./scripts/verify.sh`
