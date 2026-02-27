# PLAN

## Scope (current change)
- Add an optional production resource-limits Compose overlay (`deploy/docker-compose.limits.yml`) that sets CPU and memory limits per service using Docker Compose-compatible fields (`cpus`, `mem_limit`, and reservation companions where useful).
- Introduce `.env` resource-tuning knobs with safe defaults in `deploy/.env.example` so operators can size limits without editing Compose YAML.
- Extend `cmd/bitriver` wrappers to support a `--limits` flag that automatically includes the limits overlay for `compose up` and `quickstart`.
- Add `env validate` sanity checks for resource knobs (CPU and memory format/value) so malformed values fail early.
- Update production docs and contract docs to describe recommended production usage, host-size tiers, and limit tuning workflow.

## Assumptions
- Base behavior must stay unchanged when `--limits` is not passed and when only `deploy/docker-compose.yml` is used.
- Overlay should remain compatible with `docker compose` non-Swarm mode, so `cpus`/`mem_limit` are preferred over Swarm-only deploy blocks.
- Resource defaults should be conservative enough for production while still workable for development if the overlay is explicitly enabled.

## Risks
- Overly aggressive defaults could starve transcoding/streaming paths on smaller hosts.
- Compose resource keys can be engine-version sensitive; config validation is required to confirm compatibility.
- CLI flag wiring changes could alter compose command argument order expected by tests.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./cmd/bitriver -count=1`
- `docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.limits.yml config`
- `./scripts/verify.sh`
