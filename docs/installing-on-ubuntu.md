# Install BitRiver Live on Ubuntu 24.04

This is the current source-checkout deployment path for one Ubuntu VM. It is
written for the intended home-hosting shape: XCP-ng/XOA provides the VM,
Docker Compose runs the application, and Nginx Proxy Manager (NPM) provides the
public HTTPS edge.

> **Artifact status:** this repository does not currently publish a GitHub
> Release, `.deb`, `.rpm`, or downloadable installer. Do not follow a command
> that references a release archive until the project has actually published
> and verified one. The steps below build the reviewed source checkout on your
> host.

## Deployment shape

```text
Internet
  |-- HTTPS 443 --> Nginx Proxy Manager --> Ubuntu VM :8080
  |                                      --> Ubuntu VM :9080 (/hls/)
  `-- RTMP 1935 ------------------------> Ubuntu VM :1935 (SRS)

Ubuntu VM / Docker Compose
  BitRiver API + viewer | SRS | OME | transcoder | Postgres | Redis
```

Postgres, Redis, OME's manager API, and both media-control APIs stay private.
The public browser path for OME playback is BitRiver's same-origin `/live/`
route.

## 1. Provision the VM

Use Ubuntu Server 24.04 LTS on x86-64. A practical starting point for a small
home deployment is 8 vCPUs, 16 GB RAM, and at least 100 GB of SSD-backed
storage. Transcoding cost grows with source resolution, frame rate, concurrent
channels, and the rendition ladder; monitor the first real workload before
promising capacity.

Recommended VM settings:

- A static DHCP lease or static LAN address.
- Time synchronization enabled.
- A non-root sudo user with SSH keys.
- Separate or expandable storage for Docker/media data.
- XOA backup jobs configured; snapshots alone are not database backups.

Update and reboot before installing the application:

```bash
sudo apt update
sudo apt full-upgrade -y
sudo reboot
```

## 2. Install Docker Engine and tools

Install Docker Engine from Docker's official Ubuntu repository, including the
Buildx and Compose plugins. Follow the current
[Docker Engine for Ubuntu instructions](https://docs.docker.com/engine/install/ubuntu/),
then verify:

```bash
sudo systemctl enable --now docker
sudo docker version
sudo docker compose version
```

Docker's `docker` group is root-equivalent. If you intentionally allow the
operator account to use Docker without `sudo`, add only that trusted account
and start a new login session:

```bash
sudo usermod -aG docker "$USER"
```

Install the remaining source-checkout tools:

```bash
sudo apt install -y ca-certificates curl git
```

Install the Go version named in the repository's `.go-version` using the
[official Go installation guide](https://go.dev/doc/install), then confirm
`go version` reports Go 1.26 or newer. The container builds still pin their own
toolchain; host Go is used by the deployment CLI.

## 3. Check out a reviewed commit

```bash
sudo git clone https://github.com/ProhibitedTV/BitRiver-Live.git /opt/bitriver-live
sudo chown -R "$USER":"$USER" /opt/bitriver-live
cd /opt/bitriver-live
git status --short --branch
git rev-parse HEAD
```

Record the commit ID in your change log. For a production host, deploy a commit
you have reviewed and tested rather than following `main` blindly.

## 4. Initialize the deployment contract

The root `.env`, `deploy/docker-compose.yml`, and generated OME/SRS configs are
one contract.

```bash
cd /opt/bitriver-live
cp deploy/.env.example .env
go run ./cmd/bitriver env init --env-file ./.env
chmod 600 .env
```

The initializer rotates sample credentials. Save the generated administrator
credential in your password manager and never commit `.env`.

Edit `.env` for your public topology. This is the minimum public-address set
for an NPM deployment:

```dotenv
BITRIVER_LIVE_MODE=production
NEXT_PUBLIC_VIEWER_URL=https://stream.example.com/viewer
BITRIVER_VIEWER_ORIGIN=https://stream.example.com
BITRIVER_LIVE_ADMIN_CORS_ORIGINS=https://stream.example.com
BITRIVER_LIVE_VIEWER_CORS_ORIGINS=https://stream.example.com

BITRIVER_SRS_PUBLIC_RTMP_BASE_URL=rtmp://ingest.example.com:1935/live
BITRIVER_OME_PUBLIC_LLHLS_BASE_URL=https://stream.example.com/live
BITRIVER_TRANSCODER_PUBLIC_BASE_URL=https://stream.example.com/hls
```

Keep these OME listener/origin values on their Compose defaults:

```dotenv
BITRIVER_OME_BIND=0.0.0.0
BITRIVER_OME_IP=0.0.0.0
BITRIVER_OME_LLHLS_ORIGIN=http://ome:8080
```

OME's top-level server IP is a local bind interface. Do not put the VM's public
IP there: that address does not exist inside the container and can prevent OME
from starting. Public WebRTC candidates, when enabled, belong in
`BITRIVER_OME_TCP_RELAY` and `BITRIVER_OME_ICE_CANDIDATE`.

Validate without printing secrets:

```bash
go run ./cmd/bitriver env validate --env-file ./.env
docker compose --env-file .env -f deploy/docker-compose.yml config --quiet
```

## 5. Build and start

Use the canonical quickstart so migrations, generated media configs, health
waits, and administrator bootstrap run in the intended order:

```bash
./scripts/quickstart.sh --image-source build
```

Equivalent source command:

```bash
go run ./cmd/bitriver quickstart \
  --compose-file deploy/docker-compose.yml \
  --env-file ./.env \
  --image-source build
```

The first build can take time. It must still finish within the bounded health
wait; do not report success merely because containers are running.

Inspect state and bounded logs:

```bash
docker compose --env-file .env -f deploy/docker-compose.yml ps
docker compose --env-file .env -f deploy/docker-compose.yml \
  logs --tail=200 bitriver-live srs-controller ome transcoder
```

OME must be healthy and its authenticated manager API must be able to read the
declared `default/live` application. The control plane validates that
application; it does not create or delete it for each stream.

## 6. Firewall and NPM

Docker-published ports can interact with host firewall rules in surprising
ways. Enforce the boundary at the router/hypervisor firewall and, when needed,
the Docker `DOCKER-USER` chain; do not assume UFW alone blocks every published
container port.

Expected exposure:

| Port | Source | Purpose |
| --- | --- | --- |
| `22/tcp` | trusted admin network only | SSH |
| `8080/tcp` | NPM only | viewer, API, admin, chat, `/live/` |
| `9080/tcp` | NPM only | `/hls/` transcoder renditions |
| `1935/tcp` | creators or an RTMP TCP proxy | SRS ingest |
| `80/443` | public, on the NPM host | HTTP redirect and HTTPS |

Do not publicly expose `5432`, `6379`, `1985/1986`, `8081`, or `9001`.

Configure the HTTP and RTMP edges with
[the NPM/Cloudflare runbook](reverse-proxy-npm-cloudflare.md). A Cloudflare
orange-cloud record can front HTTPS but does not proxy arbitrary RTMP; use a
DNS-only ingest record, NPM Stream entry, direct route, or VPN.

## 7. Acceptance before real users

Run the control-plane smoke test:

```bash
go run ./cmd/bitriver smoke \
  --compose-file deploy/docker-compose.yml \
  --env-file ./.env
```

Then test from outside the VM and, for public deployments, outside the home
network:

1. `/healthz`, `/readyz`, `/viewer`, and `/admin` return successfully through
   the public HTTPS hostname.
2. OME becomes healthy within the configured timeout and `default/live` is
   readable through its authenticated manager API.
3. A non-sensitive test channel accepts RTMP through the public ingest address
   and transitions to live.
4. `/live/<channel-id>/llhls.m3u8` returns through the public HTTPS origin and
   several seconds of video and audio decode.
5. Every advertised `1080p`, `720p`, and `480p` manifest returns successfully
   when that ladder is enabled.
6. Chat connects over websocket and messages reach a second session.
7. Stopping the publisher returns the channel offline without stale jobs.

Container health is necessary but not sufficient. OME process health without a
publish, public manifest, and decode is not playback proof.

## 8. Reboots, backups, and updates

Docker starts at boot and the canonical services use restart policies. After a
VM reboot, rerun the smoke test and inspect OME/media logs before declaring the
site recovered.

Configure Postgres backups and rehearse restore using the shipped scripts:

```bash
./scripts/backup-postgres.sh
./scripts/prune-backups.sh
```

See [operations](operations.md) for backup variables, isolated restore drills,
Redis/media policy, monitoring, and incident handling. Store a copy outside
the VM/XCP-ng storage domain.

Until immutable releases exist, treat source updates as planned changes:

```bash
cd /opt/bitriver-live
git fetch origin --prune
git log --oneline --decorate HEAD..origin/main
```

Review the changes and migration notes, take a verified backup, then move to a
specific reviewed commit and rerun the canonical quickstart with
`--image-source build`. Record the previous commit and image IDs, but do not
assume application rollback is safe after an irreversible database migration;
follow [the upgrade runbook](upgrades.md).

## Troubleshooting OME startup

If OME never becomes healthy:

1. Confirm `BITRIVER_OME_BIND` and `BITRIVER_OME_IP` remain wildcard listener
   values for Docker Compose.
2. Render again with `go run ./cmd/bitriver ome render --force --env-file ./.env`.
3. Run `go run ./cmd/bitriver ome verify-health-token --env-file ./.env`.
4. Inspect `docker compose ... logs --tail=200 ome ome-config`.
5. Confirm the generated config contains only the intended listener ports and
   that the manager credential matches `.env`; never paste the token into an
   issue or chat.
6. Restart OME through Compose and repeat the bounded application/manifest
   acceptance checks.

Relevant deeper references:

- [Deployment contract](contract.md)
- [Single-host production baseline](production-single-host.md)
- [NPM and Cloudflare](reverse-proxy-npm-cloudflare.md)
- [Operations](operations.md)
- [Security](security.md)
- [Testing](testing.md)
