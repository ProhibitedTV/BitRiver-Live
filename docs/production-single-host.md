# Production profile: single-host deployment

This guide provides a **conservative starting profile** for running BitRiver Live on one physical or virtual host.

> [!IMPORTANT]
> Capacity depends on codec mix, bitrate ladder, viewer behavior, storage backend latency, and network quality. The numbers below are **not a guarantee** and should be validated in your environment before committing to an SLA.

## Scope and assumptions

This profile assumes the canonical deployment contract in this repository:

- `deploy/docker-compose.yml`
- root `.env`
- `deploy/ome/Server.generated.xml`

Assumptions for the estimates below:

- Typical live profile around 1080p input with moderate bitrate ladder/transcoding.
- Most streams are active and sustained (not short burst traffic).
- Single-host Docker Compose deployment (no external transcoding or multi-node sharding).
- Local SSD-backed storage for runtime data paths.

## Recommended baseline hardware (single host)

Use this as a conservative baseline for initial production rollout:

- **CPU:** 16 physical cores (or 16 high-performance vCPU minimum)
- **RAM:** 64 GB
- **Disk:**
  - 1 TB NVMe SSD minimum
  - Sustained random I/O suitable for Postgres/Redis plus media-related writes
- **Network interface:**
  - 10 GbE preferred
  - 1 GbE minimum only for low stream counts and controlled egress

## Maximum recommended concurrent streams (starting limit)

For this baseline, start with:

- **Up to 20 concurrent live streams** (conservative initial cap)

This cap is intentionally cautious. Raise only after staged load validation in your own environment.

## Expected latency range

For a healthy single-host deployment with stable network paths:

- **Glass-to-glass latency:** typically **3-8 seconds**

Latency can rise with heavier transcoding ladders, constrained CPU headroom, network jitter, or reverse-proxy misconfiguration.

## Known scaling limits on a single host

Single-host deployments are typically constrained by:

1. **CPU saturation from transcoding**, especially at higher resolutions or dense ABR ladders.
2. **Egress bandwidth ceilings** as concurrent viewers and bitrates increase.
3. **Memory pressure** from combined media pipeline, caches, and API/data services.
4. **I/O contention** across database, Redis persistence, and media-related write paths.
5. **Operational blast radius**: one-host failure impacts all control-plane and media workloads.

## Operator guidance

- Treat this profile as a **first production checkpoint**, not final capacity planning.
- Keep at least **30% CPU and RAM headroom** during peak periods.
- Increase limits gradually (for example, +10-20% stream count per test stage) and monitor saturation signals.
- If sustained utilization exceeds safe headroom, move to a horizontally scaled architecture.
