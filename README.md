# BitRiver-Live

BitRiver Live is a modern, full-stack solution for building your own live streaming platform—similar to Twitch or DLive—that you can operate on your own infrastructure. The project is designed for creators, communities, and developers who want freedom, scalability, and transparency in how live media is delivered.

---

## Developer Quickstart

The repository now includes a self-contained Go API that covers the foundational entities outlined in the product plan—users, channels, stream sessions, and chat messages. It persists data to a simple JSON datastore so you can experiment without needing any external infrastructure.

### Requirements

- [Go](https://go.dev/) 1.21 or newer

### Run the API server

```bash
go run ./cmd/server --mode development --addr :8080 --data data/store.json
```

When the server is running, visit [http://localhost:8080](http://localhost:8080) to open the **BitRiver Live Control Center**. The built-in web interface lets you:

- Create users, channels, and streamer profiles without touching the command line
- Edit or retire accounts, rotate channel metadata, and keep stream keys handy with one-click copy actions
- Start or stop live sessions, review rolling analytics, and export a JSON snapshot of the state
- Seed chat conversations, moderate or remove messages across every channel in one view
- Curate streamer profiles with featured channels, top friends, and crypto donation links through a guided form
- Generate a turn-key installer script that provisions BitRiver Live as a systemd service on a home server, complete with optional log directories
- Offer a self-service `/signup` experience so viewers can create password-protected accounts on their own

The UI talks directly to the same REST API documented below, so you can always fall back to curl or an API client when you need to automate advanced workflows.

The server also respects the `BITRIVER_LIVE_ADDR`, `BITRIVER_LIVE_MODE`, and `BITRIVER_LIVE_DATA` environment variables if you prefer configuring runtime options without flags. Switch to production-ready defaults by running:

```bash
go run ./cmd/server --mode production --data /var/lib/bitriver-live/store.json
```

In production mode BitRiver Live binds to port 80 by default, letting viewers access the control center without appending a port number to your domain.

The server exposes a REST API under the `/api` prefix:

| Endpoint | Method | Description |
| --- | --- | --- |
| `/api/auth/signup` | `POST` | Self-service viewer registration with password hashing |
| `/api/auth/login` | `POST` | Issue a session token for password-based sign-in |
| `/api/auth/session` | `GET`, `DELETE` | Inspect or revoke active sessions |
| `/api/users` | `POST`, `GET` | Create new accounts and list all users |
| `/api/users/{id}` | `GET`, `PATCH`, `DELETE` | Inspect, update, or remove a control-center account |
| `/api/channels` | `POST`, `GET` | Provision channels for creators and filter them by owner |
| `/api/channels/{id}` | `GET`, `PATCH`, `DELETE` | Fetch, update, or delete channel metadata |
| `/api/channels/{id}/stream/start` | `POST` | Mark a channel live and begin a stream session |
| `/api/channels/{id}/stream/stop` | `POST` | End the active session and capture peak concurrents |
| `/api/channels/{id}/sessions` | `GET` | Retrieve the session history for a channel |
| `/api/channels/{id}/chat` | `POST`, `GET` | Persist chat messages and fetch recent history |
| `/api/channels/{id}/chat/{messageId}` | `DELETE` | Remove a single chat message for moderation |
| `/api/profiles/{userId}` | `PUT`, `GET` | Configure streamer bios, top friends, and crypto-only donation links |

Example: create a user, launch a channel, and start a stream session.

```bash
# Create a user
curl -s --request POST http://localhost:8080/api/users \
  --header 'Content-Type: application/json' \
  --data '{"displayName":"River","email":"river@example.com","roles":["creator"]}'

# Create a channel for that user (replace OWNER_ID with the user ID from above)
curl -s --request POST http://localhost:8080/api/channels \
  --header 'Content-Type: application/json' \
  --data '{"ownerId":"OWNER_ID","title":"River Rafting","tags":["outdoors","travel"]}'

# Start streaming (replace CHANNEL_ID)
curl -s --request POST http://localhost:8080/api/channels/CHANNEL_ID/stream/start \
  --header 'Content-Type: application/json' \
  --data '{"renditions":["1080p","720p"]}'
```

### Authentication tokens

All authenticated endpoints expect the BitRiver Live session token to be supplied via the `Authorization` header using the
standard Bearer format:

```bash
curl --request GET http://localhost:8080/api/auth/session \
  --header "Authorization: Bearer SESSION_TOKEN"
```

Future releases will also support the same token stored in a secure `bitriver_session` cookie for browser clients. Query-string
tokens are no longer accepted.

To stop the session, POST to `/api/channels/CHANNEL_ID/stream/stop` with an optional `peakConcurrent` value. Chat messages can be posted to `/api/channels/CHANNEL_ID/chat` and retrieved with pagination (`?limit=25`).

Once a creator has at least one friend on the platform, they can publish a profile that highlights their live channels, a MySpace-style “top eight”, and crypto donation addresses that the platform never touches:

```bash
curl -s --request PUT http://localhost:8080/api/profiles/STREAMER_ID \
  --header 'Content-Type: application/json' \
  --data '{
    "bio":"Streaming straight from the river",
    "avatarUrl":"https://cdn.example.com/avatar.png",
    "bannerUrl":"https://cdn.example.com/banner.png",
    "featuredChannelId":"CHANNEL_ID",
    "topFriends":["FRIEND_ID_1","FRIEND_ID_2"],
    "donationAddresses":[
      {"currency":"eth","address":"0x123","note":"main wallet"},
      {"currency":"btc","address":"bc1xyz"}
    ]
  }'
```

The API normalizes currency symbols (e.g., `eth` → `ETH`) and enforces a maximum of eight top friends to preserve the throwback feel. Donation links are peer-to-peer: viewers send crypto directly to streamers with zero custody by the BitRiver Live backend.

For non-technical viewers, the bundled `/signup` page provides a friendly registration and sign-in flow that talks to the authentication endpoints above and persists session tokens to the browser.

### Run automated checks

```bash
go test ./...
```

The suite exercises the JSON storage layer, REST handlers, and stream/chat flows end-to-end without requiring any external services or libraries beyond the Go standard library.

### Configure ingest orchestration

BitRiver Live can orchestrate end-to-end ingest and transcode jobs by talking to an SRS edge, an OvenMediaEngine application, and an FFmpeg job controller. Provide connection details via environment variables when starting the server:

| Variable | Description |
| --- | --- |
| `BITRIVER_SRS_API` | Base URL (including port, e.g. `http://srs:1985`) for the SRS management API. |
| `BITRIVER_SRS_TOKEN` | Bearer token used when creating/deleting SRS channels. |
| `BITRIVER_OME_API` | Base URL for the OvenMediaEngine REST API (defaults to port `8081`). |
| `BITRIVER_OME_USERNAME` / `BITRIVER_OME_PASSWORD` | Basic-auth credentials for OvenMediaEngine. |
| `BITRIVER_TRANSCODER_API` | Base URL for the FFmpeg job runner (e.g. a lightweight controller on port `9000`). |
| `BITRIVER_TRANSCODER_TOKEN` | Bearer token for FFmpeg job APIs. |
| `BITRIVER_TRANSCODE_LADDER` | Optional ladder definition (`1080p:6000,720p:4000,480p:2500`). |
| `BITRIVER_INGEST_MAX_BOOT_ATTEMPTS` | Number of times to retry encoder boot before giving up. |
| `BITRIVER_INGEST_RETRY_INTERVAL` | Delay between retry attempts (e.g. `500ms`). |
| `BITRIVER_INGEST_HEALTH` | Path that exposes dependency health (default `/healthz`). |

When these variables are set the API will:

1. POST to `SRS /v1/channels` to allocate RTMP/SRT ingest keys for the channel.
2. POST to `OvenMediaEngine /v1/applications` to configure the playback application.
3. POST to the FFmpeg controller `/v1/jobs` endpoint to launch the adaptive bitrate ladder.

Stopping a stream reverses the process with DELETE calls to `/v1/jobs/{id}`, `/v1/applications/{channelId}`, and `/v1/channels/{channelId}`.

The `/healthz` endpoint now returns JSON that includes the status of these external services so dashboards and probes can surface degraded dependencies early.

---

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
