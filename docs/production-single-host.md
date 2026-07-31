# Production profile: single-host deployment

This guide provides an opinionated, conservative starting profile for running BitRiver Live on one physical or virtual host.

For the packaged Ubuntu 24.04 amd64 path, install the current public candidate
through [`docs/installing-on-ubuntu.md`](installing-on-ubuntu.md), then return
here for sizing and optional resource overlays.

> [!IMPORTANT]
> Capacity depends on codec mix, bitrate ladder, viewer behavior, storage backend latency, and network quality. Validate these numbers in your own environment before committing to an SLA.

## Scope and assumptions

This profile assumes the canonical deployment contract in this repository:

- `deploy/docker-compose.yml`
- root `.env`
- `deploy/ome/Server.generated.xml`

Assumptions for sizing below:

- Typical live profile around 1080p input with moderate bitrate ladder/transcoding.
- Mostly sustained live workloads (not short burst traffic).
- Single-host Docker Compose deployment.
- Local SSD-backed runtime storage.

## Baseline host sizes

Use these tiers as a practical starting point for production planning:

| Tier | Suggested host | Recommended use |
| --- | --- | --- |
| Small | 8 vCPU, 32 GB RAM, NVMe SSD | Staging, pilot launches, low stream counts |
| Medium (default production start) | 16 vCPU, 64 GB RAM, NVMe SSD | Initial production rollout |
| Large | 32+ vCPU, 128+ GB RAM, high IOPS NVMe | Higher sustained stream/transcode density |

For the medium baseline, start with **up to 20 concurrent live streams** and scale only after measured load testing.

## Resource limits and why they matter

Compose services without limits can over-consume host CPU/RAM and starve latency-sensitive paths (transcoding, ingest, DB). The optional limits overlay gives you:

- predictable cgroup ceilings per service,
- safer noisy-neighbor behavior on single-host deployments,
- easier tuning through `.env` knobs (without editing Compose YAML).

BitRiver Live ships a production-friendly limits overlay at:

- `deploy/docker-compose.limits.yml`

It uses Docker Compose-compatible fields (`cpus`, `mem_limit`, `mem_reservation`) and is optional so developer workflows stay unchanged.

If you also need higher file-descriptor ceilings for ingest workloads, optionally layer
`deploy/docker-compose.resources.yml` (ulimits-only overlay) on top of the limits overlay.


## Preflight gate

Before bringing up production services, run the canonical environment check wrapper first:

```bash
bash deploy/check-env.sh
```

This executes doctor preflight with the canonical Compose file and then validates your env file.

If you need to inspect doctor output directly, run:

```bash
go run ./cmd/bitriver doctor
```

Treat `FAIL` as a release blocker, and clear `WARN` items (or document risk acceptance) before go-live.

## Recommended production startup path

Use the base stack plus the limits overlay:

```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.limits.yml up -d
```

Or via CLI wrapper:

```bash
go run ./cmd/bitriver quickstart --limits
```

## Tune resource knobs in `.env`

`deploy/.env.example` now includes defaults for service limits, for example:

- `BITRIVER_API_CPUS`, `BITRIVER_API_MEM`
- `BITRIVER_VIEWER_CPUS`, `BITRIVER_VIEWER_MEM`
- `BITRIVER_POSTGRES_CPUS`, `BITRIVER_POSTGRES_MEM`
- `BITRIVER_OME_CPUS`, `BITRIVER_OME_MEM`
- `BITRIVER_TRANSCODER_CPUS`, `BITRIVER_TRANSCODER_MEM`

Tuning guidance:

1. Keep initial defaults for first production cut.
2. Raise `BITRIVER_TRANSCODER_*` and `BITRIVER_OME_*` first for media bottlenecks.
3. Keep at least ~30% host CPU/RAM headroom at peak.
4. Validate changes with `go run ./cmd/bitriver env validate --env-file ./.env` before rollout.

## Known single-host scaling limits

1. CPU saturation from transcoding at higher resolutions/denser ABR ladders.
2. Egress bandwidth ceilings as concurrent viewer count rises.
3. Memory pressure across media + data-plane services.
4. I/O contention across Postgres/Redis/media write paths.
5. Operational blast radius (one host failure affects all workloads).
