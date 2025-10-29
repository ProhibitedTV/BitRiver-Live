# Scaling topologies for BitRiver Live

BitRiver Live scales from a single host to a multi-region footprint. This guide summarises three proven layouts and explains how to size each component, where to terminate TLS, and how to introduce load balancers or CDNs while keeping ingest and playback responsive.

## Service reference

The tables below consolidate the default roles and network bindings defined in [`deploy/docker-compose.yml`](../deploy/docker-compose.yml) and the systemd units in [`deploy/systemd/`](../deploy/systemd/). Use them to map firewalls, load balancers, and health probes.

| Service | Role | Default ports | Notes |
| --- | --- | --- | --- |
| bitriver-live | Go control centre and API | 8080/TCP (HTTP), optional 443/TCP via TLS flags | Depends on PostgreSQL, Redis, SRS, OME, and the transcoder for ingest/processing. [`deploy/docker-compose.yml`](../deploy/docker-compose.yml) maps `8080:8080` and the systemd unit runs the same binary. |
| bitriver-viewer | Next.js viewer runtime | 3000/TCP (HTTP) | Proxied behind the API when `BITRIVER_VIEWER_ORIGIN` is set. The systemd unit launches `node server.js`. |
| postgres | Relational store | 5432/TCP | Required for channel metadata, recordings, and accounts. |
| redis | Rate limiting and chat queues | 6379/TCP | Optional unless Redis-backed queues or rate limits are enabled. |
| srs | RTMP/WebRTC ingest | 1935/TCP (RTMP), 1985/TCP (HTTP API) | Health checks query `/healthz` or `/api/v1/versions`. |
| ome | OvenMediaEngine origin | 8081/TCP (API/WebRTC), 9000/TCP (ICE/transfer) | Serves WebRTC/HLS/DASH segments to edges or CDNs. |
| transcoder | FFmpeg job controller | 9000/TCP (control plane) exposed as 9001/TCP on the host | Schedules ladder transcodes and VOD packaging. |

> **Tip:** The docker compose bundle also exposes persistent volumes for PostgreSQL, Redis, and application data so you can relocate stateful services to managed offerings without rewriting manifests.

## Topology 1: Single-node appliance

Deploy everything on one bare-metal server or VM when you need a contained appliance. This is the layout created by `deploy/docker-compose.yml` or the generated systemd installer.

- **Sizing:** 8 vCPUs and 16 GB RAM sustain ~1 Gbps of aggregate viewing with a single 1080p ladder (source + 1 transcode). Increase RAM if you cache segments locally.
- **Networking:** Expose ports 80/443 for viewers and 1935 for RTMP ingest. Map 8080 internally; terminate TLS on the Go API (`BITRIVER_LIVE_TLS_CERT`/`BITRIVER_LIVE_TLS_KEY`) or at a local reverse proxy (Caddy, Nginx).
- **Load balancing:** Optional. Rely on the OS firewall (UFW) and health checks baked into the compose file to restart failed containers.
- **Operations:** Keep PostgreSQL and Redis local. Back up `/var/lib/bitriver-live` and database volumes nightly.

Use this mode for creator labs, QA stacks, or on-premise demos where latency between services is negligible.

## Topology 2: Origin/edge split

When live audiences grow, dedicate hosts per function and introduce a load balancer in front of viewer traffic.

```
[RTMP Encoders] -> [Edge Firewall] -> [SRS origin]
                                  \-> [OME origin + Transcoder]
Viewers -> [HTTP(S) LB] -> [Viewer nodes] -> [API origin]
                                   \-> [PostgreSQL]
                                   \-> [Redis]
```

- **Origins:** Run the Go API, PostgreSQL, Redis, SRS, OME, and the transcoder on a trusted subnet. Keep the API close to the database to avoid cross-AZ latency spikes.
- **Edges:** Place one or more viewer nodes (Next.js runtime) behind an HTTP load balancer such as HAProxy, Envoy, or an L7 service (ALB, Cloud Run). Configure `BITRIVER_VIEWER_ORIGIN` to point to the load-balanced viewer URL so the API proxies `/viewer` correctly.
- **Load balancing:** Terminate TLS at the edge load balancer. Forward `/api` to the API pool on port 8080 (or 443 if the API handles TLS) and `/viewer` to the viewer nodes. Health check `/healthz` on the API and `GET /` on the viewer runtime.
- **Sizing:** Scale viewer nodes horizontally—start with 2 vCPUs/4 GB RAM per node. Keep a separate 4–8 vCPU instance for the Go API. Reserve CPU headroom for SRS and the transcoder (dedicated 4 vCPUs each) when pushing multiple renditions.
- **Ingest protection:** Use security groups or firewall rules to only allow RTMP (1935/TCP) from trusted encoders. Expose the SRS management API (1985/TCP) to operators only.

This layout isolates public-facing workloads while keeping persistent state centralised. Add read replicas or managed PostgreSQL when the control plane becomes CPU bound.

## Topology 3: CDN-assisted delivery

For global audiences, offload segment delivery and static assets to a CDN while keeping ingest and control traffic on origin hosts.

- **Segment flow:** Configure OME to publish HLS/DASH manifests to object storage, then front the bucket with a CDN (CloudFront, Fastly, Cloudflare). The CDN pulls from origin over HTTPS and serves viewers from the nearest PoP. Keep WebRTC paths on regional edge servers for low-latency interactivity.
- **API and auth:** Terminate TLS at a global load balancer (Cloudflare Tunnel, AWS Global Accelerator) and forward REST/WebSocket traffic to the API origin pool. Maintain sticky sessions only if you enable WebSocket chat over the API.
- **Viewer assets:** Build the Next.js viewer as a static export and host it behind the CDN, or continue proxying `/viewer` through the API while caching static assets (`/_next/static/*`).
- **Load balancing:** Use CDN health checks to watch the API `/healthz` and remove unhealthy origins automatically. Pair with an internal L4 balancer for SRS/OME if you run multiple ingest regions.
- **Sizing:** Keep at least two API instances (4 vCPUs/8 GB RAM each) per region for redundancy. Run SRS/OME/transcoder clusters per region; dedicate GPU-backed nodes for transcoding-heavy channels. PostgreSQL should be managed with high availability and read replicas close to API origins.
- **Security:** Restrict origin IPs to accept only the CDN’s edge ranges on ports 80/443. Require mutual TLS or signed URLs when the CDN requests HLS segments.

## Additional recommendations

- **Configuration management:** Template `.env` files for the systemd units and use secrets managers to rotate API keys. The units in [`deploy/systemd/`](../deploy/systemd/) read environment files so you can update settings without editing service definitions.
- **Observability:** Expose API health checks at `http://<api-host>:8080/healthz` and monitor container health probes from the compose file. Mirror these endpoints into your load balancer configuration.
- **Disaster recovery:** Snapshots for PostgreSQL and object storage plus replicated Redis caches ensure quick failover. Regularly test restores on staging clusters that mirror your production topology.
