# Project Roadmap

## Name Ideas

Top pick: **Cascade Live** (fast, fluid, memorable)

Other options: **BitRiver Live**, **Rivestream**, **FluxCast**, **Tributary**

---

## 1. Core Product Scope

### Creators
- Start/stop live streams
- Stream key management
- Title, category, and thumbnail controls
- Customize profile bios, avatars, banners, and featured channels
- Curate a MySpace-style top eight of fellow streamers
- VOD recording
- Basic analytics (concurrency, watch time, retention)

### Viewers
- Channel directory and discovery
- Low-latency player (LL-HLS/WebRTC)
- Live chat
- Follow and subscribe actions
- Rich streamer profiles that surface live channels, top friends, and crypto donation links
- Dark mode and mobile web support

### Moderation
- Role-based access (owner, moderator)
- Chat tooling (delete, timeout, ban, mute)
- User reports and appeals
- Word filters and automod

### Monetization (Phase 2)
- Direct, peer-to-peer crypto tips (platform never takes custody)
- Subscriptions
- Ad slots and sponsor cards

### Compliance
- Terms of Service, Privacy Policy, DMCA agent
- COPPA/CCPA/GDPR baseline support
- Copyright takedown workflow

---

## 2. Reference Architecture (Self-Hosted, Scalable)

### Ingest & Real-Time
- Protocols: **RTMP**, **SRT**, **WHIP/WebRTC**
- Streaming servers:
  - **SRS** (RTMP/SRT/WebRTC, HLS/DASH, built-in stats)
  - **OvenMediaEngine** (WebRTC LL, CMAF)
  - **Nginx-RTMP** (simple MVP option)

### Transcode & Packaging
- Distributed **FFmpeg** workers (NVENC/AMF/VAAPI)
- Adaptive bitrate ladder: 1080p/6 Mbps, 720p/3.5, 480p/2, 360p/1.2
- CMAF HLS/DASH segments (LL-HLS for near-real-time)
- Optional WebRTC SFU for sub-second latency use cases

### Origin & Edge Cache
- **Origin:** Nginx/Caddy serving manifests and segments from object storage
- **Edge:** Nginx or **Varnish/ATS** nodes with future PoPs
- **Object storage:** **MinIO** for VODs, thumbnails, and short-lived segments

### Backend Services
- Language: Go, Elixir (Phoenix), or Node.js/TypeScript (Fastify/Nest)
- Datastores: **PostgreSQL** (primary), **Redis** (cache, rate limiting), **Kafka/NATS** (events)
- Search: **Meilisearch** or **OpenSearch**
- Authentication: **Keycloak** or custom auth service

### Player & Frontend
- Player: **HLS.js** / **Shaka Player** for LL-HLS/DASH, **OvenPlayer** for WebRTC
- Frontend: **Next.js** or **SvelteKit** with Tailwind CSS and SSR

### Chat & Real-Time UX
- WebSocket service (Elixir Phoenix Channels or Go + NATS)
- Alternative: Federated **Matrix (Synapse)** with custom UI skin

### Observability & Operations
- Metrics: **Prometheus** + **Grafana**
- Logs: **Loki** or ELK stack
- Tracing: **OpenTelemetry** + **Jaeger**
- Video QoE: startup time, rebuffer ratio, dropped frames, bitrate per viewer
- CI/CD: Gitea + Woodpecker or GitHub Actions
- IaC: Terraform, configuration via Ansible
- Runtime: Docker Compose (dev), Kubernetes + Helm (prod)
- Edge security: Fail2ban, Cloudflare Tunnels/anycast, WAF and rate limiting at edge

---

## 3. Data Model (Minimum)

- **User:** id, auth profile, roles, wallet/payout info
- **Channel:** id, owner_id, stream_key, title/category, live_state, tags
- **StreamSession:** start/stop timestamps, renditions, peak concurrent viewers
- **ChatMessage/ModerationAction:** channel_id, user_id, content, flags
- **VOD/Recording:** storage key, duration, thumbnails, visibility
- **Profile:** user_id, bio, avatar/banner URLs, featured channel, top eight friend IDs, crypto donation addresses (currency, address, note)
- **Tip/Sub:** user_id, channel_id, amount, provider, status

---

## 4. Scaling Path

### MVP (Single Node)
- SRS/OvenMediaEngine + FFmpeg (CPU/GPU)
- Nginx serving HLS
- PostgreSQL, Redis, and single-node MinIO

### V1 (Multi-Node)
- Dedicated ingest nodes
- Stateless transcode workers (autoscale via queue)
- MinIO high-availability deployment
- Origin/edge split
- PostgreSQL read replicas and Redis clustering

### At Scale
- Multi-region edge PoPs
- Regional ingest sharding
- Kafka for chat, analytics, live-state events
- ClickHouse for analytics warehousing
- Object storage lifecycle policies (hot → warm tiers)

---

## 5. Hardware Guidelines

- **Ingest/packager:** multi-core CPU, 10GbE for many channels
- **Transcode:** 1–2 GPUs with NVENC (RTX 4000/A2000 class) for multiple 1080p ladders
- **Origin/edge:** fast SSDs, ample RAM for caching; scale horizontally
- **Storage:** MinIO on HDD with SSD cache or full NVMe if budget allows

---

## 6. Bandwidth Planning

Aggregate egress ≈ viewers × average rendition bitrate:

- 1,000 viewers @ ~2 Mbps → **~2 Gbps**
- 5,000 viewers @ ~2 Mbps → **~10 Gbps**
- 10,000 viewers @ ~2 Mbps → **~20 Gbps**

Plan uplinks, edges, and peering accordingly.

---

## 7. Security & Abuse Mitigation

- Per-channel RTMP/SRT auth keys with rotation
- Rate limiting for login, chat, APIs, segment fetches
- DDoS resilience via edge caching and scrubbing providers
- Moderation tooling: word lists, URL blocks, image/GIF filters, reporting workflows
- Optional on-prem NLP (e.g., Detoxify) for toxicity hints
- Default VOD privacy until creators publish

---

## 8. Legal & Payments

- DMCA: designate agent, define takedown workflow and logging
- Terms of Service & Privacy Policy covering retention, cookies, analytics
- Payment processing (Stripe/Adyen) for tips and subscriptions
- Age gates for mature content and COPPA notices as required

---

## 9. Developer Ergonomics

- `docker compose up` for local Postgres, Redis, MinIO, SRS, backend, and frontend
- FFmpeg fixtures for fake streams; k6/Locust for chat/viewer load testing
- Feature flags (e.g., Unleash) and schema migrations (Prisma/Goose/Ecto)

---

## 10. Phased Delivery Roadmap

### Phase 0 (2–3 Weeks)
- RTMP ingest → single transcode → HLS delivery → web player
- User authentication
- Channel creation and management
- Stream start/stop controls
- Basic chat functionality
- Single-box deployment

### Phase 1 (4–6 Weeks)
- LL-HLS or WebRTC low-latency path
- Full adaptive bitrate ladder
- Channel directory and search
- VOD recording workflow
- Moderation tools
- Analytics v1

### Phase 2
- Multi-node scaling across ingest, transcode, and edge
- Payments, subscriptions, sponsor cards
- Advanced moderation
- Multi-region deployment

---

## 11. Open Source Stack Recommendation

- **Media:** SRS + FFmpeg + CMAF LL-HLS; OvenPlayer/HLS.js
- **Backend:** Go (Fiber/FastHTTP) + PostgreSQL + Redis + NATS + MinIO
- **Chat:** Elixir Phoenix (Channels) or Go WebSockets + NATS
- **Frontend:** Next.js + Tailwind CSS
- **Operations:** Kubernetes + Prometheus/Grafana + Loki + Jaeger; Terraform + Helm for infra as code

---

## Next Steps

Potential enhancements include a clickable architecture diagram, a Docker Compose starter kit, and a Kubernetes Helm chart skeleton. Contributions and feedback are welcome!
