# BitRiver-Live

BitRiver Live is a modern, full-stack solution for building your own live streaming platform—similar to Twitch or DLive—that you can operate on your own infrastructure. The project is designed for creators, communities, and developers who want freedom, scalability, and transparency in how live media is delivered.

---

## Set up BitRiver Live at home

### One-command Docker quickstart

To spin up the entire BitRiver Live stack (API, viewer, database, ingest pipeline, and transcoder) with sane defaults, run the quickstart helper from the repository root:

> **Linux tip:** Add your account to the `docker` group so you can talk to the daemon without `sudo`:
> 
> ```bash
> sudo usermod -aG docker $USER
> newgrp docker  # or log out and back in
> ```
> 
> You can also run the quickstart with `sudo ./scripts/quickstart.sh`, but that will create root-owned files like `.env`, so fixing the group membership first is strongly recommended.

```bash
./scripts/quickstart.sh
```

The script checks for Docker/Docker Compose, writes a `.env` that matches `deploy/docker-compose.yml`, and boots the containers with `docker compose up -d`. The first run also builds local images for the Go API and the bundled FFmpeg job controller, so you never need to authenticate against a private registry. After the API is reachable it runs the `bootstrap-admin` helper against the compose database, prints the seeded credentials, and reminds you to rotate the password on first login. Review and edit `.env` before inviting real viewers, then consult [`docs/quickstart.md`](docs/quickstart.md) for common follow-up commands and troubleshooting tips.

### Manual Go workflow

BitRiver Live ships with a self-contained Go API and control center so you can explore the product plan—users, channels, stream sessions, and chat messages—without provisioning databases or external services.

#### Prerequisites

- Install [Go 1.21+](https://go.dev/doc/install) on your workstation. Ubuntu users can choose one of the following workflows (ensure the resulting version is 1.21 or newer):
  - Install from APT:
    ```bash
    sudo apt update
    sudo apt install golang-go
    ```
  - Download and install the official tarball:
    ```bash
    wget https://go.dev/dl/go1.22.3.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf go1.22.3.linux-amd64.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    ```
- Run `go version` to confirm your toolchain meets the minimum requirement.
- Clone this repository and switch into it:
  ```bash
  git clone https://github.com/BitRiver-Live/BitRiver-Live.git
  cd BitRiver-Live
  ```

#### Run the development server

Point the server at a Postgres instance before starting it. Supply the DSN via `--postgres-dsn`, `BITRIVER_LIVE_POSTGRES_DSN`, or `DATABASE_URL`:

```bash
export BITRIVER_LIVE_POSTGRES_DSN="postgres://bitriver:bitriver@127.0.0.1:5432/bitriver_live?sslmode=disable"
go run ./cmd/server --mode development
```

Leave the terminal open while the server is running. Browse to [http://localhost:8080](http://localhost:8080) to open the **BitRiver Live Control Center** and sign up your first account. If you need the legacy JSON datastore for a quick demo, pass `--storage-driver json --data data/store.json` explicitly—the flag opt-in keeps production environments on Postgres by default.

#### Promote your first admin

Roles control which buttons light up inside the control center. The first account you create starts as a regular user, so seed an administrator before trying to manage channels or other accounts. The `bootstrap-admin` helper assigns the `admin` role and updates the password hash without hand-editing JSON:

1. Stop the server with `Ctrl+C`.
2. Run the helper from the repository root, substituting your preferred credentials (passwords must be at least eight characters). Point it at the same datastore the server will use:
   ```bash
   go run ./cmd/tools/bootstrap-admin \
     --postgres-dsn "$BITRIVER_LIVE_POSTGRES_DSN" \
     --email you@example.com \
     --name "Your Display Name" \
     --password "temporary-password"
   ```
   For a JSON datastore provide `--json data/store.json` instead.
3. Restart the server with `go run ./cmd/server --mode development` and sign in with the seeded email/password.
4. Rotate the password immediately from the control center settings page once you're logged in.

The helper can be rerun at any time to promote additional accounts or replace compromised credentials.

#### Explore the control center

With an administrator signed in, the web interface lets you:

- Create users, channels, and streamer profiles without touching the command line
- Edit or retire accounts, rotate channel metadata, and keep stream keys handy with one-click copy actions
- Start or stop live sessions, export a JSON snapshot of the state, and review live analytics cards powered by `/api/analytics/overview`
- Seed chat conversations, moderate or remove messages across every channel in one view
- Capture recorded broadcasts automatically when a stream ends, manage retention windows, and surface VOD manifests to viewers
- Curate streamer profiles with featured channels, top friends, and crypto donation links through a guided form
- Generate a turn-key installer script that provisions BitRiver Live as a systemd service on a home server, complete with optional log directories
- Offer a self-service `/signup` experience so viewers can create password-protected accounts on their own

##### Analytics overview

The Control Center surfaces the `/api/analytics/overview` endpoint through a dashboard that blends platform-wide metrics with channel-level detail. The response shape is:

```json
{
  "summary": {
    "liveViewers": 0,
    "streamsLive": 0,
    "watchTimeMinutes": 0,
    "chatMessages": 0
  },
  "perChannel": [
    {
      "channelId": "string",
      "title": "string",
      "liveViewers": 0,
      "followers": 0,
      "avgWatchMinutes": 0,
      "chatMessages": 0
    }
  ]
}
```

`summary.watchTimeMinutes` tracks total viewer minutes over the last 24 hours, `summary.chatMessages` counts messages posted today, and `summary.streamsLive` reports active broadcasts. Each channel entry mirrors those aggregates with the current live audience, average watch time across recorded sessions, follower totals, and messages sent since midnight UTC.

The UI talks directly to the same REST API documented below, so you can always fall back to curl or an API client when you need to automate advanced workflows.

The server also respects the `BITRIVER_LIVE_ADDR`, `BITRIVER_LIVE_MODE`, `BITRIVER_LIVE_POSTGRES_DSN`, and `DATABASE_URL` environment variables if you prefer configuring runtime options without flags. Switch to production-ready defaults by running:

```bash
BITRIVER_LIVE_POSTGRES_DSN="postgres://bitriver:bitriver@db:5432/bitriver_live?sslmode=disable" \
go run ./cmd/server --mode production
```

In production mode BitRiver Live binds to port 80 by default, letting viewers access the control center without appending a port number to your domain.

To serve HTTPS directly from the Go process provide a certificate/key pair generated via [Let's Encrypt](https://letsencrypt.org/), your reverse proxy, or another certificate authority:

```bash
go run ./cmd/server \
  --mode production \
  --addr :443 \
  --postgres-dsn "$BITRIVER_LIVE_POSTGRES_DSN" \
  --tls-cert /etc/letsencrypt/live/stream.example.com/fullchain.pem \
  --tls-key /etc/letsencrypt/live/stream.example.com/privkey.pem
```

The same values can be supplied through environment variables (`BITRIVER_LIVE_TLS_CERT` and `BITRIVER_LIVE_TLS_KEY`). Pair this with a lightweight cron job or Certbot renewal hook to keep certificates fresh, or terminate TLS at a reverse proxy if you prefer automatic ACME handling upstream.

Prefer containers? Check out `deploy/docker-compose.yml` for a pre-wired stack that mounts persistent storage, exposes metrics, and optionally links Redis for shared rate-limiting state. Chat queue behaviour is configured entirely through the `--chat-queue-*` flags defined in `cmd/server/main.go`; set `--chat-queue-driver redis` to enable Redis Streams support and provide the related connection details via the accompanying flags. The queue constructor will automatically create the configured stream and consumer group when it connects. Operators planning for growth can review [`docs/scaling-topologies.md`](docs/scaling-topologies.md) for single-node, origin/edge, and CDN-assisted layouts that pair with the compose and systemd manifests.

**Advanced deployment guides:** For Postgres, object storage, ingest orchestration, and other production setups, see [docs/advanced-deployments.md](docs/advanced-deployments.md) alongside [docs/scaling-topologies.md](docs/scaling-topologies.md).

### Public viewer

The BitRiver Live viewer packages the audience-facing Next.js experience with channel discovery, live chat, and VOD playback.
Full installation, development, and deployment steps live in [`web/viewer/README.md`](web/viewer/README.md).

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

cd web/viewer
npm install
npm run lint
npm run test:integration
```

See [docs/testing.md](docs/testing.md) for the consolidated checklist used in CI.

The Go suite exercises the JSON storage layer, REST handlers, and stream/chat flows end-to-end without requiring any external services or libraries beyond the Go standard library. The viewer integration suite combines Jest component coverage with Playwright accessibility checks—installing dependencies once with `npm install` prepares both harnesses. Playwright downloads its browsers on first run; if you need a CI-friendly install, run `npx playwright install --with-deps` ahead of the test command.





## Project roadmap

The high-level product planning notes now live in [docs/product-roadmap.md](docs/product-roadmap.md).
