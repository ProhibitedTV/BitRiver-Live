# Installing BitRiver Live on Ubuntu

This guide walks operators through bringing up BitRiver Live on an Ubuntu Server virtual machine. It covers VM preparation, package installation, data services, ingest components, application builds, and final verification. For reference architectures and service manifests, see [`deploy/docker-compose.yml`](../deploy/docker-compose.yml), [`docs/scaling-topologies.md`](scaling-topologies.md), and the systemd units documented in [`deploy/systemd/README.md`](../deploy/systemd/README.md). The sample SRS configuration in [`deploy/srs/conf/srs.conf`](../deploy/srs/conf/srs.conf) is referenced when enabling ingest.

> **Supported releases:** Ubuntu 22.04 LTS or later. Earlier releases may require updated package repositories for Node.js and Redis.

## 1. Prepare the virtual machine

1. Provision an Ubuntu VM with at least 4 vCPUs, 8 GB RAM, and 100 GB of SSD-backed storage. Place it on a subnet that can reach your ingest and viewer networks.
2. Attach a static public IP or configure DNS for the hostname viewers will use (e.g., `stream.example.com`).
3. Harden the base OS:
   - Create a non-root sudo user and disable password SSH logins.
   - Update packages and reboot into the latest kernel.

```bash
sudo apt update
sudo apt full-upgrade -y
sudo reboot
```

4. Enable uncomplicated firewall (UFW) to expose only the required ports. Adjust for your topology if traffic terminates at a load balancer.

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 1935/tcp   # RTMP ingest to SRS
sudo ufw allow 8080/tcp   # API (adjust if reverse proxy terminates TLS)
sudo ufw allow 9080/tcp   # HLS playback mirror (transcoder-public)
sudo ufw allow 8088/tcp   # SRS WebRTC/HTTP-FLV (optional)
sudo ufw enable
```

> **TLS reminder:** Plan for HTTPS. Either terminate TLS at a reverse proxy (e.g., Nginx, Caddy) or use Certbot to issue certificates for the Go API directly. Schedule renewals before going live.

## 2. Install base packages

Install OS dependencies for the API, database clients, Node.js-based viewer, and ingest tooling.

```bash
sudo apt install -y build-essential git curl ufw pkg-config libcap2-bin \
  golang-go nodejs npm postgresql-client redis-tools docker.io docker-compose-plugin
```

`libcap2-bin` provides the `setcap` utility the Ubuntu installer uses to keep privileged ports working across manual restarts. If you plan to bind to :80 or :443 directly from the Go process, keep it installed; otherwise pass `--addr :8080` (or another high port) when terminating traffic at a reverse proxy.

If you prefer managed runtimes, replace the distribution Go and Node.js packages with upstream toolchains (e.g., via `snap`, `asdf`, or tarballs). Ensure `go version` reports 1.21+ and `node --version` reports 18+.

## 3. Configure PostgreSQL and Redis

Install server packages and enable services.

```bash
sudo apt install -y postgresql postgresql-contrib redis-server
sudo systemctl enable --now postgresql
sudo systemctl enable --now redis-server
```

### PostgreSQL

1. Switch to the `postgres` user to create the application database and credentials.

```bash
sudo -u postgres psql <<'SQL'
CREATE ROLE bitriver WITH LOGIN PASSWORD 'changeme';
CREATE DATABASE bitriver OWNER bitriver;
GRANT ALL PRIVILEGES ON DATABASE bitriver TO bitriver;
SQL
```

2. Enforce TLS between the API and Postgres in production. Update `/etc/postgresql/14/main/postgresql.conf` and `pg_hba.conf` to require `hostssl` entries, and deploy certificates managed by your secrets store (HashiCorp Vault, AWS Secrets Manager, etc.). Restart PostgreSQL after editing:

```bash
sudo systemctl restart postgresql
```

3. Verify connectivity from the application host.

```bash
psql "postgres://bitriver:changeme@localhost:5432/bitriver?sslmode=disable" -c '\l'
```

Replace `sslmode=disable` with `require` when TLS is enabled.

If you are upgrading from the JSON datastore, run:

```bash
go run ./cmd/tools/migrate-json-to-postgres \
  --json /var/lib/bitriver-live/store.json \
  --postgres-dsn "postgres://bitriver:changeme@localhost:5432/bitriver?sslmode=disable"
```

The helper copies records into Postgres and verifies the row counts before exiting.

### Redis

1. Harden Redis for networked deployments:
   - Bind Redis to `127.0.0.1` or your private subnet.
   - Set a strong password in `/etc/redis/redis.conf` (`requirepass`).
   - Enable TLS if clients connect over untrusted networks (stunnel or Redis 6+ native TLS).

2. Restart and validate:

```bash
sudo systemctl restart redis-server
redis-cli -a 'changeme' ping
```

## 4. Download BitRiver Live release assets

Always install from a tagged release so the binaries, installer scripts, and Docker Compose manifests stay in sync. Download the archive that matches your architecture from the [GitHub Releases](https://github.com/BitRiver-Live/BitRiver-Live/releases) page.

```bash
# Replace v1.2.3 with the release tag you plan to deploy
export BITRIVER_LIVE_VERSION="v1.2.3"
export BITRIVER_LIVE_PACKAGE="bitriver-live-linux-amd64.tar.gz"  # Use linux-arm64 on Ampere/Graviton hosts

curl -LO "https://github.com/BitRiver-Live/BitRiver-Live/releases/download/${BITRIVER_LIVE_VERSION}/${BITRIVER_LIVE_PACKAGE}"
sudo mkdir -p /opt/bitriver-live
sudo tar -C /opt/bitriver-live -xzf "${BITRIVER_LIVE_PACKAGE}"
sudo chown -R $USER:$USER /opt/bitriver-live
rm "${BITRIVER_LIVE_PACKAGE}"
```

The archive expands into `/opt/bitriver-live` with the compiled binaries plus the `deploy/` directory (`docker-compose.yml`, `install/`, `srs/`, `ome/`, and `systemd/`) referenced throughout this guide.

## 5. Deploy ingest services

BitRiver Live relies on SRS for ingest and OvenMediaEngine (OME) plus a transcoder for playback. Choose the approach that matches your operations model.

### Option A: Docker Compose

The compose bundle under [`deploy/docker-compose.yml`](../deploy/docker-compose.yml) wires SRS, OME, the transcoder, PostgreSQL, Redis, and the API/viewer. Adapt it for production by overriding secrets and persistent volumes.

Before starting the services, create an environment file and replace the placeholder credentials. Docker Compose refuses to launch until these values are present.

```bash
cd /opt/bitriver-live
cp deploy/.env.example .env
${EDITOR:-nano} .env
./deploy/check-env.sh
```

Update the entries for:

- `BITRIVER_POSTGRES_USER` and `BITRIVER_POSTGRES_PASSWORD`
- `BITRIVER_LIVE_ADMIN_EMAIL` and `BITRIVER_LIVE_ADMIN_PASSWORD`
- `BITRIVER_SRS_TOKEN`
- `BITRIVER_OME_USERNAME` and `BITRIVER_OME_PASSWORD`
- `BITRIVER_TRANSCODER_TOKEN`

Ensure `BITRIVER_LIVE_POSTGRES_DSN` references the same Postgres user and password you configure above before bringing the stack online.

Rerun `./deploy/check-env.sh` until it reports the environment file is ready. The compose manifest also uses required-variable expansion, so `docker compose` fails with an explanatory error when any of the credentials are missing or unchanged from the defaults.

```bash
cd /opt/bitriver-live
sudo docker compose -f deploy/docker-compose.yml pull
sudo docker compose -f deploy/docker-compose.yml up -d srs ome transcoder transcoder-public
```

The compose bundle now binds `./transcoder-data` on the host to `/work` inside the FFmpeg controller so HLS manifests survive container restarts. Create the directory structure once before starting production traffic:

```bash
mkdir -p /opt/bitriver-live/transcoder-data/public
```

By default the stack serves `/work/public` through the `transcoder-public` Nginx sidecar. It forwards container port `8080` to host port `9080`, which keeps playback URLs stable for both local and remote viewers. The transcoder advertises the mirror via `BITRIVER_TRANSCODER_PUBLIC_DIR=/work/public` and `BITRIVER_TRANSCODER_PUBLIC_BASE_URL=http://transcoder-public:8080`. When you operate behind a CDN or existing HTTP origin, override `BITRIVER_TRANSCODER_PUBLIC_BASE_URL` in `.env` so the API publishes the correct URLs while keeping the durable volume mounted at `/opt/bitriver-live/transcoder-data`. The FFmpeg controller refuses to start when `BITRIVER_TRANSCODER_PUBLIC_BASE_URL` is empty, so point it at a reachable playback origin before launching the stack.

If you prefer a different publication path, mount an object storage bucket or a directory served by another reverse proxy and point the transcoder at it with `BITRIVER_TRANSCODER_PUBLIC_DIR` (local staging directory) plus `BITRIVER_TRANSCODER_PUBLIC_BASE_URL` (public HTTP origin). S3-compatible storage works well with `s3fs`, `rclone mount`, or a periodic sync (`aws s3 sync /opt/bitriver-live/transcoder-data/public s3://cdn-bucket/uploads/`).

Review `deploy/srs/conf/srs.conf` for the default SRS ports and authentication settings. Mount a customised version into the container when you need stricter access control or TLS certificates for RTMP/RTMPS.

### Option B: systemd services

If you run SRS, OME, and the transcoder as native services, use [`deploy/systemd/README.md`](../deploy/systemd/README.md) for installation guidance. Copy the tracked unit files into `/etc/systemd/system/`, create the matching `/opt/bitriver-*/.env` files, and enable each service:

```bash
sudo install -d -m 0755 /opt/bitriver-srs /opt/bitriver-ome /opt/bitriver-transcoder
sudo install -m 0644 deploy/systemd/srs.service /etc/systemd/system/srs.service
sudo install -m 0644 deploy/systemd/ome.service /etc/systemd/system/ome.service
sudo install -m 0644 deploy/systemd/bitriver-transcoder.service /etc/systemd/system/bitriver-transcoder.service
sudo systemctl daemon-reload
sudo systemctl enable --now srs.service
sudo systemctl enable --now ome.service
sudo systemctl enable --now bitriver-transcoder.service
```

Populate the `.env` files with the ports, tokens, and image tags described in [`deploy/systemd/README.md`](../deploy/systemd/README.md) before starting traffic.

Check status and logs to confirm ingest readiness.

```bash
sudo systemctl status srs.service
journalctl -u srs.service -f
```

## 6. Deploy the API service

### Guided setup

For a prompt-driven experience, run the wizard at [`deploy/install/wizard.sh`](../deploy/install/wizard.sh) from the release directory you extracted earlier:

```bash
cd /opt/bitriver-live
./deploy/install/wizard.sh
```

The wizard walks through the common inputs—install directory (default `/opt/bitriver-live`), data directory (default `/var/lib/bitriver-live`), service user (default `bitriver`), listen address, storage driver, optional hostname hint, TLS certificate/key paths, rate-limiting values, whether to allow public self-signup, and whether to redirect systemd logs. It now defaults to the Postgres storage backend; be ready with a DSN and a database that has been migrated with the SQL files in [`deploy/migrations/`](../deploy/migrations). When you choose the Postgres storage backend it prompts for the DSN (required) and optionally a Postgres session-store DSN, letting you reuse the primary connection string or point to a dedicated database. When the wizard detects a source checkout it validates that Go 1.21+ is available; when invoked from a release tarball it skips the Go check because the binaries are already present. It still warns if a `bitriver-live.service` unit already exists before invoking the Ubuntu installer. Because the underlying helper uses `sudo` to create users, directories, and systemd units, the wizard highlights those privileged steps and asks for confirmation first.

If a run fails midway, fix the highlighted issue and start the wizard again—it is safe to rerun, and you can accept the previous defaults to regenerate the service.

### Option A: Automated installer (recommended)

The UI-generated installer script now wraps the tracked helper at [`deploy/install/ubuntu.sh`](../deploy/install/ubuntu.sh). Run it from the extracted release so it uses the binaries and migrations from the same tag:

Provide the required inputs (install directory, data directory, and service user) via flags or matching environment variables. The installer now defaults the storage backend to Postgres and refuses to continue until you provide `--postgres-dsn <DSN>` (or `BITRIVER_LIVE_POSTGRES_DSN`). Apply the SQL files in [`deploy/migrations/`](../deploy/migrations) to that database before re-running the helper so the schema is ready for the API. When Postgres is in use the session manager automatically persists to the same DSN; pass `--session-store memory` to keep ephemeral sessions or `--session-store-dsn` to point at a dedicated session database. Use `--storage-driver json` only when you intentionally opt into the legacy JSON store for development.

```bash
cd /opt/bitriver-live
./deploy/install/ubuntu.sh \
  --install-dir /opt/bitriver-live \
  --data-dir /var/lib/bitriver-live \
  --service-user bitriver \
  --mode production \
  --addr :80 \
  --postgres-dsn "postgres://bitriver:changeme@localhost:5432/bitriver_live?sslmode=disable" \
  --enable-logs \
  --hostname stream.example.com
```

Run the helper from the release root—the script reuses the packaged `server`/`bootstrap-admin` binaries or, when a checked-out module is present, rebuilds them from source.

The script builds the API binary, writes `$INSTALL_DIR/.env`, configures optional TLS and rate-limiting variables, and registers a `bitriver-live.service` systemd unit. Review the generated `.env` file to ensure storage selections (JSON or Postgres), database DSNs, session-store driver settings, and Redis credentials are present before starting traffic.

When the listen address resolves to a privileged port (<1024) the installer injects `AmbientCapabilities=CAP_NET_BIND_SERVICE`/`CapabilityBoundingSet=CAP_NET_BIND_SERVICE` into the systemd unit and runs `sudo setcap 'cap_net_bind_service=+ep' "$INSTALL_DIR/bitriver-live"` so manual restarts keep the binding. Operators fronting the service with Nginx, Caddy, or another reverse proxy should set `--addr :8080` (or a similar high port) and forward 80/443 from the proxy to avoid capabilities altogether.

Provide `--bootstrap-admin-email` and `--bootstrap-admin-password` to seed the first control-center account automatically. The installer runs the `bootstrap-admin` helper after copying the binaries so the JSON datastore or Postgres database already contains an administrator when systemd starts the service. Capture the printed credentials and rotate the password immediately after logging in.

Environment variable equivalents:

* `INSTALL_DIR`, `DATA_DIR`, `SERVICE_USER`
* `BITRIVER_LIVE_ADDR`, `BITRIVER_LIVE_MODE`
* `BITRIVER_LIVE_TLS_CERT`, `BITRIVER_LIVE_TLS_KEY`
* `BITRIVER_LIVE_RATE_GLOBAL_RPS`, `BITRIVER_LIVE_RATE_LOGIN_LIMIT`, `BITRIVER_LIVE_RATE_LOGIN_WINDOW`
* `BITRIVER_LIVE_RATE_REDIS_ADDR`, `BITRIVER_LIVE_RATE_REDIS_PASSWORD`
* `BITRIVER_LIVE_ENABLE_LOGS`, `BITRIVER_LIVE_LOG_DIR`
* `BITRIVER_LIVE_HOSTNAME_HINT`
* `BITRIVER_LIVE_ALLOW_SELF_SIGNUP`
* `BITRIVER_LIVE_POSTGRES_DSN`
* `BITRIVER_LIVE_SESSION_STORE`, `BITRIVER_LIVE_SESSION_POSTGRES_DSN` (defaults to Postgres and reuses `BITRIVER_LIVE_POSTGRES_DSN` when left unset)

### Option B: Manual install

If you prefer hand-crafted units, follow the manual process below.

1. Install the API binary from the release archive.

```bash
cd /opt/bitriver-live
install -d -m 755 bin
install -m 755 server bin/bitriver-live
```

2. Install a dedicated system user and directories for configuration and data.

```bash
sudo useradd --system --home /var/lib/bitriver-live --shell /usr/sbin/nologin bitriver
sudo mkdir -p /etc/bitriver-live /var/lib/bitriver-live
sudo chown -R bitriver:bitriver /var/lib/bitriver-live
```

3. Create `/etc/bitriver-live/bitriver-live.env` with secrets and connection details. Store passwords, tokens, and API keys in a secrets manager (Vault, SOPS, AWS SSM). Distribute them at boot time via encrypted disks or templating tools (e.g., `ansible-vault`, `systemd-creds`).

```
BITRIVER_LIVE_ADDR=:8080
BITRIVER_LIVE_MODE=production
BITRIVER_LIVE_TLS_CERT=/etc/letsencrypt/live/stream.example.com/fullchain.pem
BITRIVER_LIVE_TLS_KEY=/etc/letsencrypt/live/stream.example.com/privkey.pem
BITRIVER_LIVE_STORAGE_DRIVER=postgres
BITRIVER_LIVE_POSTGRES_DSN=postgres://bitriver:changeme@localhost:5432/bitriver?sslmode=require
BITRIVER_LIVE_RATE_REDIS_ADDR=127.0.0.1:6379
BITRIVER_LIVE_RATE_REDIS_PASSWORD=changeme
BITRIVER_LIVE_SESSION_STORE=postgres
# Optional: override if you want a dedicated session database.
# BITRIVER_LIVE_SESSION_POSTGRES_DSN=postgres://bitriver:changeme@localhost:5432/bitriver_sessions?sslmode=require
BITRIVER_SRS_TOKEN=REPLACE_ME
BITRIVER_OME_USERNAME=REPLACE_ME
BITRIVER_OME_PASSWORD=REPLACE_ME
BITRIVER_TRANSCODER_TOKEN=REPLACE_ME
BITRIVER_TRANSCODER_PUBLIC_BASE_URL=https://cdn.example.com/hls
BITRIVER_TRANSCODER_PUBLIC_DIR=/var/lib/bitriver-transcoder/public
```

4. Install the systemd unit from `deploy/systemd/bitriver-live.service` or author a minimal unit:

```ini
[Unit]
Description=BitRiver Live API
After=network-online.target postgresql.service redis-server.service

[Service]
User=bitriver
Group=bitriver
EnvironmentFile=/etc/bitriver-live/bitriver-live.env
ExecStart=/opt/bitriver-live/bin/bitriver-live --data /var/lib/bitriver-live/store.json
Restart=on-failure
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

5. Reload systemd and start the service.

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now bitriver-live.service
sudo systemctl status bitriver-live.service
```

## 7. Deploy the viewer

The GitHub Release for each BitRiver Live tag publishes a production-ready viewer bundle (`bitriver-viewer-<tag>.tar.gz`) and a container image (`ghcr.io/bitriver-live/bitriver-viewer:<tag>`). Use one of the options below so the API and viewer stay on the same version. Only clone the repository and build `web/viewer` from source when you intentionally need to test local changes.

### Option A: Run the standalone bundle (recommended)

1. Download the viewer archive that matches the API version and extract it alongside the API release:

   ```bash
   # Replace v1.2.3 with the release you are deploying
   export BITRIVER_LIVE_VERSION="v1.2.3"
   curl -LO "https://github.com/BitRiver-Live/BitRiver-Live/releases/download/${BITRIVER_LIVE_VERSION}/bitriver-viewer-${BITRIVER_LIVE_VERSION}.tar.gz"

   sudo mkdir -p /opt/bitriver-viewer
   sudo tar -xzvf "bitriver-viewer-${BITRIVER_LIVE_VERSION}.tar.gz" -C /opt/bitriver-viewer --strip-components=1
   rm "bitriver-viewer-${BITRIVER_LIVE_VERSION}.tar.gz"
   ```

   The archive expands into `/opt/bitriver-viewer` with `.next/standalone/`, `.next/static/`, and `public/` directories that mirror the output of `npm run build`.

2. Create `/opt/bitriver-viewer/.env` with runtime settings for the standalone server. At minimum provide the API origin and listening address:

   ```bash
   sudo tee /opt/bitriver-viewer/.env >/dev/null <<'EOF'
NEXT_PUBLIC_API_BASE_URL=https://stream.example.com
NEXT_VIEWER_BASE_PATH=/viewer
PORT=3000
HOSTNAME=0.0.0.0
EOF
   ```

3. Start the bundled server with Node.js 20+ (or wrap it in systemd as documented in [`deploy/systemd/README.md`](../deploy/systemd/README.md)):

   ```bash
   cd /opt/bitriver-viewer
   node .next/standalone/server.js
   ```

   The `standalone` output includes all production dependencies so no additional `npm install` step is required. Reverse proxies such as Nginx or Caddy can front the process; make sure `BITRIVER_VIEWER_ORIGIN` in the API `.env` points at the viewer URL.

For more deployment patterns—including systemd units and CDN hosting—see [`docs/viewer-deployment.md`](viewer-deployment.md).

### Option B: Deploy the container image

If you prefer containers, run the published image for the same release tag. Mount the environment file so configuration stays consistent with the standalone deployment:

```bash
docker run -d --name bitriver-viewer \
  --restart unless-stopped \
  -p 3000:3000 \
  --env-file /opt/bitriver-viewer/.env \
  ghcr.io/bitriver-live/bitriver-viewer:${BITRIVER_LIVE_VERSION}
```

When fronting the viewer with Nginx or another proxy, route `/viewer` requests to the container (or standalone server) and terminate TLS upstream.

> **Building from source?** Clone the repository and follow [`web/viewer/README.md`](../web/viewer/README.md) only when you intentionally want to modify the Next.js app or test development builds. Production installations should stay on the tagged release assets above.

## 8. Post-install checks

1. Validate services are running.

```bash
systemctl --failed
sudo systemctl status bitriver-live.service bitriver-viewer.service srs.service ome.service bitriver-transcoder.service
```

2. Confirm database connectivity and migrations.

```bash
psql "postgres://bitriver:changeme@localhost:5432/bitriver?sslmode=require" \
  --command "SELECT NOW(), current_user;"
```

3. Check Redis health.

```bash
redis-cli -a 'changeme' info server | head
```

4. Hit the API and health endpoints.

```bash
curl -k https://stream.example.com/healthz
curl -k https://stream.example.com/api/channels
```

5. Inspect logs for ingest services.

```bash
journalctl -u srs.service -u ome.service -u bitriver-transcoder.service --since "-5 minutes"
```

6. Ensure TLS certificates renew automatically (`certbot renew --dry-run`) and firewall rules persist across reboots (`sudo ufw status`). Rotate secrets periodically and audit access logs.

With these steps complete the BitRiver Live stack should be ready to accept creators, ingest live streams via SRS, transcode them through the FFmpeg controller, and serve viewers via the Next.js frontend.
