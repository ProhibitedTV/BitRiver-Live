# Install BitRiver Live on Ubuntu

This is the artifact-only installation path for an operator-managed Ubuntu VM. It installs the canonical Docker Compose stack; it does not build application images or require a source checkout.

> **Current candidate:** [`v1.2.3-rc.13`](https://github.com/ProhibitedTV/BitRiver-Live/releases/tag/v1.2.3-rc.13) publishes checksum-covered Ubuntu packages, launcher archives, SBOMs, five signed first-party image digests, sanitized product evidence, and a signed `release-set.json`. It is a prerelease, not stable promotion.

A tag containing a hyphen is a GitHub prerelease and does not move the `latest`
image tag. Official images use `ghcr.io/prohibitedtv`; forks and mirrors can
override `BITRIVER_IMAGE_NAMESPACE`. The release workflow logs out of GHCR and
proves that all five exact-tag manifests are anonymously readable before it
publishes the GitHub prerelease.

## Support and evidence boundary

- Production target: Ubuntu Server 24.04 LTS on amd64.
- Package build coverage: amd64 and arm64 `.deb`/`.rpm` artifacts.
- Package install coverage: Ubuntu 24.04, Debian 12, and Rocky Linux 9 containers.
- RC13 release evidence: Ubuntu 24.04, Debian 12, and Rocky Linux 9 package
  install/inspect/remove passed; the pull-only five-image stack passed real
  RTMP, decoded OME/transcoder media, chat/moderation, VOD, and aggregate
  status checks. The release set binds that evidence to source commit
  `d416968e0cadb900820ecf1b4307b101b82ffbbc` and workflow run `30795492882`.
- Provisional until target-host evidence is recorded: Debian/Rocky production
  support, arm64, clean XOA/XCP-ng installation, NPM browser access, reboot
  recovery, and repeated OME failure/media recovery.

The installer proves rendered configuration, migration completion, process
health, and critical Compose health. RC13's tag workflow also proved the real
media/API product path from published images. Stable promotion still requires
the package to be installed and exercised on the target Ubuntu VM through its
real proxy, firewall, authenticated OvenMediaEngine control path, and reboot
cycle.

The repository's manual **Clean host qualification** workflow automates the
release-only Ubuntu portion on an ephemeral Ubuntu 24.04 amd64 VM. It accepts
the candidate tag and expected signed-root SHA-256, performs no repository
checkout, verifies the root/package/five image signatures, installs the `.deb`,
activates the non-root systemd stack, and tests doctor, smoke, authenticated OME
control, same-tag upgrade, OME/Docker/systemd restart, and data-preserving
uninstall. Its
sanitized report remains partial evidence: an ephemeral hosted runner cannot
prove your XCP-ng/XOA reboot, NPM TLS/WebSocket route, router/firewall rules, or
post-reboot ingest and decoded playback.

## XOA/XCP-ng VM baseline

Create a bridged Ubuntu 24.04 VM with:

- 4 vCPU, 8 GiB RAM, and 40 GiB or more storage for a practical starting point.
- A stable DHCP reservation or static LAN address.
- Correct DNS, NTP, and a hostname that survives reboot.
- A separate backup target for Postgres, `/etc/bitriver-live`, and `/var/lib/bitriver-live`.

The built-in doctor floor is 4 logical CPUs, 8 GiB RAM, and 20 GiB free disk.
Transcoding, concurrent streams, and media retention normally need more. Take
an XOA snapshot before upgrades, but do not treat a VM snapshot as the only
database backup.

## 1. Install Docker Engine and Compose

Use Docker's official Ubuntu repository instructions rather than the convenience script for a production VM:

- [Install Docker Engine on Ubuntu](https://docs.docker.com/engine/install/ubuntu/)
- [Install the Docker Compose plugin](https://docs.docker.com/compose/install/linux/)

The required packages from that repository are:

```bash
sudo apt update
sudo apt install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl enable --now docker
sudo usermod -aG docker "$USER"
```

Sign out and back in after changing group membership, then verify the non-root operator can reach Docker:

```bash
docker version
docker compose version
docker run --rm hello-world
```

Membership in the `docker` group is effectively root-level host access. Use a dedicated operator account and do not grant it casually.

## 2. Download and verify a release artifact

Download the Ubuntu amd64 `.deb` or launcher archive plus `CHECKSUMS.txt` from
the same [RC13 release](https://github.com/ProhibitedTV/BitRiver-Live/releases/tag/v1.2.3-rc.13).
Never mix a package, archive, or checksum file from different tags.

```bash
release_tag=v1.2.3-rc.13
base_url="https://github.com/ProhibitedTV/BitRiver-Live/releases/download/${release_tag}"
curl -fLO "${base_url}/bitriver-live_${release_tag}_amd64.deb"
curl -fLO "${base_url}/bitriver-launcher-linux-amd64.tar.gz"
curl -fLO "${base_url}/CHECKSUMS.txt"
curl -fLO "${base_url}/release-set.json"
curl -fLO "${base_url}/release-set.sigstore.json"
grep "  bitriver-live_${release_tag}_amd64.deb$" CHECKSUMS.txt | sha256sum --check -
grep '  bitriver-launcher-linux-amd64.tar.gz$' CHECKSUMS.txt | sha256sum --check -
grep '  release-set.json$' CHECKSUMS.txt | sha256sum --check -
```

Verify the exact tag workflow identity, then compare the package directly with
its hash in the signed manifest:

```bash
identity="https://github.com/ProhibitedTV/BitRiver-Live/.github/workflows/release.yml@refs/tags/${release_tag}"
artifact="bitriver-live_${release_tag}_amd64.deb"
cosign verify-blob \
  --bundle release-set.sigstore.json \
  --certificate-identity "$identity" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  release-set.json
expected="$(jq -r --arg name "$artifact" '.artifacts[] | select(.name == $name) | .sha256' release-set.json)"
printf '%s  %s\n' "$expected" "$artifact" | sha256sum --check -
```

Do not install when the manifest has no exact entry or a signed revocation
marker is attached to the candidate. Stable releases retain the RC package
filename because promotion copies candidate bytes instead of rebuilding them.

For RC13, the expected `release-set.json` SHA-256 is
`795fffee84662aec91624eb4352b9c1a9ef5c34b17838939adaf567418797fa0`.
Confirm that value from the GitHub Release and tracked release note rather than
copying it from an untrusted forum or installation script.

Confirm the matching images are public before changing the host:

```bash
for image in bitriver-live bitriver-viewer bitriver-srs-controller bitriver-transcoder bitriver-ome-config; do
  docker buildx imagetools inspect "ghcr.io/prohibitedtv/${image}:${release_tag}" >/dev/null
done
```

Example archive verification:

```bash
tar -xzf bitriver-launcher-linux-amd64.tar.gz
cd bitriver-launcher-linux-amd64
```

The launcher also contains a keyless Sigstore bundle for its `bitriver`
binary. If Cosign is installed, verify that the binary was signed by the exact
tag workflow before running the installer:

```bash
cosign verify-blob \
  --bundle bin/bitriver.sigstore.json \
  --certificate-identity "https://github.com/ProhibitedTV/BitRiver-Live/.github/workflows/release.yml@refs/tags/${release_tag}" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  bin/bitriver
```

## 3. Stage the disabled service

Archive path:

```bash
sudo ./install.sh install --operator-user "$USER"
```

Package path:

```bash
sudo apt install "./bitriver-live_${release_tag}_amd64.deb"
sudo bitriver-host install --operator-user "$USER"
```

Installation is deliberately two-phase. It rotates sample credentials and installs a disabled systemd unit, but it does not start public services before production network values pass validation.

The filesystem contract is:

| Path | Purpose | Removal behavior |
| --- | --- | --- |
| `/opt/bitriver-live` | Versioned program assets, binaries, and Compose workspace | Replaced on upgrade; removed on uninstall |
| `/etc/bitriver-live/bitriver.env` | Operator environment and secrets, mode `0600` | Preserved by default |
| `/etc/bitriver-live/Server.generated.xml` | Deployment-time OME config | Preserved by default; never publish it |
| `/etc/bitriver-live/srs.generated.conf` | Deployment-time SRS config | Preserved by default; never publish it |
| `/var/lib/bitriver-live` | Postgres-backed application/media state mounted by the stack | Preserved by default |
| `/etc/systemd/system/bitriver-live-compose.service` | Bounded Compose lifecycle unit | Removed on uninstall |

The packaged installer persists `BITRIVER_CONFIG_ROOT=/etc/bitriver-live` and
the selected Docker operator's numeric `BITRIVER_HOST_UID`/
`BITRIVER_HOST_GID` in `bitriver.env`. The packaged unit supplies the same
values. Compose can therefore map the private config directory into its three
short-lived config helpers and run only config/data bind-writing services as
the host owner. Program assets stay read-only in those helpers; only the
separate config bind is writable during rendering. These values are
installer-managed: upgrades preserve every other operator setting and reject
any duplicate managed key before rewriting the file. Keep using
`bitriver-host activate` or the installed unit so durable config/data remains
reachable without loosening mode `0600` secrets or copying them into
`/opt/bitriver-live`.

Do not copy numeric IDs from another machine. If the Docker operator changes,
rerun `sudo bitriver-host install --operator-user NEW_USER` from the installed
release so all three managed values and the unit are regenerated together.

## 4. Configure public values

Run the guided wizard:

```bash
sudo bitriver-host configure
sudo bitriver-host doctor
```

For a VM behind Nginx Proxy Manager (NPM), use values equivalent to:

```dotenv
BITRIVER_LIVE_MODE=production
BITRIVER_DEPLOY_IMAGE_SOURCE=pull
NEXT_PUBLIC_API_BASE_URL=
NEXT_PUBLIC_VIEWER_URL=https://stream.example.com/viewer
BITRIVER_VIEWER_ORIGIN=https://stream.example.com
BITRIVER_LIVE_ADMIN_CORS_ORIGINS=https://stream.example.com
BITRIVER_LIVE_VIEWER_CORS_ORIGINS=https://stream.example.com
BITRIVER_PUBLIC_DOMAIN=stream.example.com
BITRIVER_LIVE_RATE_TRUSTED_PROXIES=10.0.10.5/32
BITRIVER_OME_BIND=0.0.0.0
BITRIVER_OME_IP=0.0.0.0
BITRIVER_OME_LLHLS_ORIGIN=http://ome:8080
BITRIVER_OME_PUBLIC_LLHLS_BASE_URL=https://stream.example.com/live
BITRIVER_TRANSCODER_PUBLIC_BASE_URL=https://stream.example.com/hls
BITRIVER_SRS_PUBLIC_RTMP_BASE_URL=rtmp://ingest.example.com:1935/live
```

Use the NPM host's exact LAN IP/CIDR for `BITRIVER_LIVE_RATE_TRUSTED_PROXIES`; do not trust an entire LAN unless every machine on it may set forwarded client-IP headers. OME's local listener remains wildcard inside Docker. Public LL-HLS is advertised through BitRiver's same-origin `/live` proxy, while the RTMP URL must name the TCP endpoint creators can reach.

Before activation, review the secret file without copying it into tickets or logs:

```bash
sudoedit /etc/bitriver-live/bitriver.env
sudo -u "$USER" /opt/bitriver-live/bin/bitriver env validate \
  --env-file /etc/bitriver-live/bitriver.env
```

Activation in production pull mode requires immutable first-party digests for `bitriver-live`, `bitriver-viewer`, `bitriver-srs-controller`, `bitriver-transcoder`, and `bitriver-ome-config`. Resolve each published tag as shown in [`docs/production-release.md`](production-release.md), then store it in the matching `/etc/bitriver-live/bitriver.env` key with an `@` prefix, for example `BITRIVER_OME_CONFIG_IMAGE_DIGEST=@sha256:...`. Pin the documented third-party image digests as well; never copy a digest from a different release tag or architecture set.

## 5. Configure Nginx Proxy Manager and the firewall

Create an NPM Proxy Host for `stream.example.com`:

- Forward scheme: `http`
- Forward host: the BitRiver VM's stable LAN address
- Forward port: `8080`
- Websockets Support: enabled
- TLS certificate: enabled, with Force SSL

The default API entrypoint serves `/`, `/api`, `/admin`, and reverse-proxies `/viewer` and `/live`, so keep those paths on the same upstream. Add an NPM custom location for `/hls/` that forwards to the same VM on port `9080` and preserves the prefix. The example above therefore advertises `https://stream.example.com/hls`. A separate media hostname is also valid when it forwards to `9080` and the public base URL matches it exactly.

Restrict host-published management ports to trusted networks. The default exposure plan is:

| Port | Protocol | Exposure |
| --- | --- | --- |
| `8080` | TCP/HTTP | NPM to BitRiver app; do not publish directly when NPM fronts it |
| `9080` | TCP/HTTP | NPM to transcoder media host |
| `1935` | TCP/RTMP | Direct/NAT only when remote encoders ingest |
| `8083` | TCP/HTTP | Private OME LL-HLS diagnostic mapping; the supported viewer path is `/live/` through port `8080` |
| `9000` or `9443` | TCP/WebSocket | OME signalling; requires a validated public WSS route |
| `3478` | TCP and UDP | OME relay when WebRTC uses it |
| `10000-10009` | UDP | OME ICE media range when WebRTC uses it |
| `8081`, `8082`, `9001`, `1985`, `1986`, `5432` | TCP | Management/internal; keep private unless a specific runbook requires access |

NPM is an HTTP reverse proxy and does not replace the UDP/NAT rules required by WebRTC. Docker-published ports can also bypass ordinary UFW expectations; enforce public exposure at the hypervisor/router and Docker `DOCKER-USER` chain as appropriate. Keep the XOA management network separate from public media traffic.

If Cloudflare is also in front of NPM, follow [`docs/reverse-proxy-npm-cloudflare.md`](reverse-proxy-npm-cloudflare.md). Do not assume Cloudflare's normal HTTP proxy carries RTMP, TURN, or arbitrary UDP.

## 6. Activate and prove OME is online

Activation validates the host and environment, enables the service, runs bounded quickstart, and fails when critical Compose services do not become healthy:

```bash
sudo bitriver-host activate
sudo bitriver-host status
curl -fsS https://stream.example.com/readyz
curl -fsS https://stream.example.com/healthz
```

For OME-specific diagnosis:

```bash
sudo -u "$USER" docker compose \
  --file /opt/bitriver-live/deploy/docker-compose.yml \
  --env-file /etc/bitriver-live/bitriver.env \
  ps ome ome-config ome-health-token-check bitriver-live

sudo -u "$USER" docker compose \
  --file /opt/bitriver-live/deploy/docker-compose.yml \
  --env-file /etc/bitriver-live/bitriver.env \
  logs --tail=120 ome ome-config ome-health-token-check bitriver-live
```

Do not paste the environment or generated OME XML into public logs. A green container root probe is necessary but not sufficient. Before release approval, also record:

1. Authenticated OME manager/control success using the deployment token.
2. Real RTMP publish from an encoder.
3. `https://stream.example.com/live/<channel-id>/llhls.m3u8` returns through NPM and several seconds of audio/video decode.
4. Every advertised transcoder rendition under `/hls/` returns successfully.
5. Stop/start and failure recovery without stale OME applications or jobs.

## 7. Reboot acceptance on XOA

Run this against the actual installed tag, not a source checkout:

```bash
sudo reboot
```

After the VM returns:

```bash
systemctl is-enabled bitriver-live-compose.service
systemctl is-active bitriver-live-compose.service
sudo bitriver-host status
curl -fsS https://stream.example.com/readyz
```

Repeat the authenticated OME, ingest, and playback checks. Attach the release tag, VM architecture, Ubuntu version, Docker/Compose versions, timestamps, and redacted command results to the release evidence. Until this is recorded, reboot recovery remains unproved.

## Operations

Status and logs:

```bash
sudo bitriver-host status
sudo bitriver-host logs
sudo journalctl -u bitriver-live-compose.service -n 200 --no-pager
```

Archive upgrade:

```bash
tar -xzf bitriver-launcher-linux-amd64.tar.gz
cd bitriver-launcher-linux-amd64
sudo ./install.sh upgrade --operator-user "$USER"
```

Package upgrade:

```bash
sudo apt install ./bitriver-live_vNEXT_amd64.deb
sudo bitriver-host upgrade --operator-user "$USER"
```

Back up and validate restore procedures before upgrading; see [`docs/upgrades.md`](upgrades.md).

Safe uninstall removes program/service integration and retains configuration and data:

```bash
sudo bitriver-host uninstall
sudo apt remove bitriver-live  # package installs only
```

Permanent purge requires both explicit flags:

```bash
sudo bitriver-host uninstall --purge-data --yes-really-purge
sudo apt remove bitriver-live  # package installs only
```

The purge deletes `/etc/bitriver-live` and `/var/lib/bitriver-live`. Verify backups first.
