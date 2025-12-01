# Scaling topologies for BitRiver Live

BitRiver Live scales from a single host to a multi-region footprint. This guide summarises three proven layouts and explains how to size each component, where to terminate TLS, and how to introduce load balancers or CDNs while keeping ingest and playback responsive.

## Service reference

The tables below consolidate the default roles and network bindings defined in [`deploy/docker-compose.yml`](../deploy/docker-compose.yml) and the systemd units in [`deploy/systemd/`](../deploy/systemd/). Use them to map firewalls, load balancers, and health probes.

| Service | Role | Default ports | Notes |
| --- | --- | --- | --- |
| bitriver-live | Go control centre and API | 8080/TCP (HTTP), optional 443/TCP via TLS flags | Depends on PostgreSQL, Redis, SRS, OME, and the transcoder for ingest/processing. [`deploy/docker-compose.yml`](../deploy/docker-compose.yml) maps `8080:8080` and the systemd unit runs the same binary. |
| bitriver-viewer | Next.js viewer runtime | 3000/TCP (HTTP) | Proxied behind the API when `BITRIVER_VIEWER_ORIGIN` is set. The systemd unit launches `node .next/standalone/server.js`. |
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
- **Load balancing:** Terminate TLS at the edge load balancer. Forward `/api` to the API pool on port 8080 (or 443 if the API handles TLS) and `/viewer` to the viewer nodes. Health check `/readyz` on the API (falling back to `/healthz` for ingest visibility) and `GET /` on the viewer runtime.
- **Sizing:** Scale viewer nodes horizontally—start with 2 vCPUs/4 GB RAM per node. Keep a separate 4–8 vCPU instance for the Go API. Reserve CPU headroom for SRS and the transcoder (dedicated 4 vCPUs each) when pushing multiple renditions.
- **Ingest protection:** Use security groups or firewall rules to only allow RTMP (1935/TCP) from trusted encoders. Expose the SRS management API (1985/TCP) to operators only.

This layout isolates public-facing workloads while keeping persistent state centralised. Add read replicas or managed PostgreSQL when the control plane becomes CPU bound.

## Topology 3: CDN-assisted delivery

For global audiences, offload segment delivery and static assets to a CDN while keeping ingest and control traffic on origin hosts.

- **Segment flow:** Configure OME to publish HLS/DASH manifests to object storage, then front the bucket with a CDN (CloudFront, Fastly, Cloudflare). The CDN pulls from origin over HTTPS and serves viewers from the nearest PoP. Keep WebRTC paths on regional edge servers for low-latency interactivity.
- **API and auth:** Terminate TLS at a global load balancer (Cloudflare Tunnel, AWS Global Accelerator) and forward REST/WebSocket traffic to the API origin pool. Maintain sticky sessions only if you enable WebSocket chat over the API.
- **Viewer assets:** Build the Next.js viewer as a static export and host it behind the CDN, or continue proxying `/viewer` through the API while caching static assets (`/_next/static/*`).
- **Load balancing:** Use CDN health checks to watch the API `/readyz` and remove unhealthy origins automatically. Pair with an internal L4 balancer for SRS/OME if you run multiple ingest regions, while `/healthz` can continue feeding ingest status dashboards.
- **Sizing:** Keep at least two API instances (4 vCPUs/8 GB RAM each) per region for redundancy. Run SRS/OME/transcoder clusters per region; dedicate GPU-backed nodes for transcoding-heavy channels. PostgreSQL should be managed with high availability and read replicas close to API origins.
- **Security:** Restrict origin IPs to accept only the CDN’s edge ranges on ports 80/443. Require mutual TLS or signed URLs when the CDN requests HLS segments.

## Component-by-component scaling playbook

Start with [`deploy/.env.example`](../deploy/.env.example) as your template (`cp deploy/.env.example .env`), then tune each component as you scale beyond a single host. The health checks below double as load-balancer targets so unhealthy instances drain automatically.

### API (control plane)

1. **Clone the template and point at your database.** Keep the generated `.env` beside your deployment manifests so both `docker compose` and systemd units share the same values. Populate the Postgres credentials and pool limits (`BITRIVER_LIVE_POSTGRES_MAX_CONNS`, `BITRIVER_LIVE_POSTGRES_MIN_CONNS`, `BITRIVER_LIVE_POSTGRES_ACQUIRE_TIMEOUT`) to match expected concurrency; raise pool sizes when adding API replicas so each instance has headroom.【F:deploy/.env.example†L20-L40】
2. **Expose a stable origin for viewers.** Set `BITRIVER_VIEWER_ORIGIN` to the URL in front of your viewer nodes so the API can proxy `/viewer` and issue absolute links; update `NEXT_PUBLIC_API_BASE_URL`/`NEXT_PUBLIC_VIEWER_URL` to match the load-balanced hostname.【F:deploy/.env.example†L41-L64】
3. **Wire up object storage early.** Configure the MinIO/S3 endpoint, credentials, bucket, and public endpoint (`BITRIVER_LIVE_OBJECT_*` or `--object-*`) before opening traffic so VOD exports and thumbnails publish to the correct origin; align lifecycle (`BITRIVER_LIVE_OBJECT_LIFECYCLE_DAYS`) with your retention policy to avoid early expiry.【F:docs/advanced-deployments.md†L225-L248】
4. **Load balancer + health checks.** Balance `/api` across API instances on port 8080 (or 443 when TLS-terminating on the API) and point readiness probes at `/readyz` with `/healthz` as a fallback when you need ingest component detail.【F:docs/advanced-deployments.md†L27-L73】

### Viewer runtime

1. **Scale horizontally behind HTTP(S) L7.** Run multiple viewer nodes and terminate TLS on your load balancer, forwarding `/` and static paths to the viewer port (default 3000). Set `BITRIVER_VIEWER_ORIGIN` to the balancer URL so the API can continue to proxy viewer traffic consistently.【F:deploy/.env.example†L41-L64】
2. **Health checks.** Use `GET /` (or a lightweight static path) as the balancer probe; Next.js will surface 200/500 responses that indicate whether the node is booted. Keep `/viewer` routing sticky only when debugging cross-origin cookie flows.

### Redis chat queue (sharding/replication)

1. **Pick a topology.** Start with a single Redis node and move to Sentinel or clustered Redis as chat traffic grows. Supply multiple addresses via `--chat-queue-redis-addrs` (or `BITRIVER_LIVE_CHAT_QUEUE_REDIS_ADDR` with TCP load balancing in front) and name the Sentinel master with `--chat-queue-redis-sentinel-master` when applicable.【F:docs/advanced-deployments.md†L13-L23】
2. **Secure and pool connections.** Set the password/username flags or environment variables and bump `--chat-queue-redis-pool-size` when adding API replicas so each instance keeps enough connections without overrunning the server.【F:docs/advanced-deployments.md†L13-L23】
3. **Health checks.** Point the balancer or monitoring at the API `/readyz`; chat readiness turns `503` when Redis is unavailable, giving you a single probe to gate chat-dependent routes.【F:docs/advanced-deployments.md†L27-L45】

### SRS and OvenMediaEngine (ingest + origin)

1. **Separate ingest from playback.** Keep RTMP/WebRTC (SRS) on a restricted network and front OME with either a CDN or an HTTP load balancer. Populate `BITRIVER_SRS_API`, `BITRIVER_OME_API`, and bind settings in `.env` to point the controller at the correct control-plane addresses before scaling out.【F:deploy/.env.example†L41-L48】
2. **Distribute health probes.** Reuse the ingest health path from `BITRIVER_INGEST_HEALTH` (default `/healthz`) for both SRS and OME, and have the API poll those endpoints; your external balancer can watch the same URLs to pull bad origins out of rotation.【F:deploy/.env.example†L57-L58】【F:docs/advanced-deployments.md†L27-L64】
3. **Edge/caching guidance.** When handing off to a CDN or multi-region edges, ensure OME’s published manifests point at the same HTTP origin you expose publicly (for example via `BITRIVER_TRANSCODER_PUBLIC_BASE_URL`) so segment URLs remain routable.【F:deploy/.env.example†L49-L56】

### Transcoder

1. **Scale per ladder and region.** Run at least one transcoder instance per ingest region and allocate CPU/GPU based on your ladder count. Point the API at each pool via `BITRIVER_TRANSCODER_API` and secure access with `BITRIVER_TRANSCODER_TOKEN`. Expose the transcoder output through the load-balanced/public URL in `BITRIVER_TRANSCODER_PUBLIC_BASE_URL` so viewers always reach the published manifests.【F:deploy/.env.example†L48-L56】【F:deploy/.env.example†L69-L73】
2. **Health checks.** Send liveness probes to `<transcoder>/healthz` (or the `BITRIVER_INGEST_HEALTH` path) and let the API’s `/healthz` mirror failures; remove unhealthy nodes from the job queue until they recover.【F:deploy/.env.example†L57-L58】【F:docs/advanced-deployments.md†L27-L73】

## Additional recommendations

- **Configuration management:** Template `.env` files for the systemd units and use secrets managers to rotate API keys. The units in [`deploy/systemd/`](../deploy/systemd/) read environment files so you can update settings without editing service definitions.
- **Observability:** Expose API health checks at `http://<api-host>:8080/readyz` for readiness and `http://<api-host>:8080/healthz` for ingest visibility, and monitor container health probes from the compose file. Mirror these endpoints into your load balancer configuration.
- **Disaster recovery:** Snapshots for PostgreSQL and object storage plus replicated Redis caches ensure quick failover. Regularly test restores on staging clusters that mirror your production topology.
