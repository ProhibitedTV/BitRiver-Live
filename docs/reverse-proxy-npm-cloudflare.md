# Cloudflare + Nginx Proxy Manager + BitRiver Live

Use this topology when BitRiver Live runs on an Ubuntu VM and Nginx Proxy
Manager (NPM) owns the public HTTPS edge. The supported HTTP shape is one
viewer origin; creator RTMP is a separate TCP path.

```text
viewer -> Cloudflare HTTPS -> NPM proxy host -> BitRiver :8080
                                             -> /hls/ -> transcoder-public :9080
creator -> DNS-only ingest host or NPM Stream -> SRS :1935/TCP
```

Keep Postgres, Redis, the OME manager API, the SRS raw/controller APIs, and the
transcoder controller private.

## 1. Public environment values

Set these values in the root `.env` before rendering Compose. Replace both
example hostnames with records you control.

```dotenv
NEXT_PUBLIC_VIEWER_URL=https://stream.example.com/viewer
BITRIVER_VIEWER_ORIGIN=https://stream.example.com
BITRIVER_LIVE_ADMIN_CORS_ORIGINS=https://stream.example.com
BITRIVER_LIVE_VIEWER_CORS_ORIGINS=https://stream.example.com

# Same-origin OME LL-HLS: BitRiver proxies /live/ to the private OME origin.
BITRIVER_OME_LLHLS_ORIGIN=http://ome:8080
BITRIVER_OME_PUBLIC_LLHLS_BASE_URL=https://stream.example.com/live

# Public HLS renditions served by the transcoder-public sidecar.
BITRIVER_TRANSCODER_PUBLIC_BASE_URL=https://stream.example.com/hls

# Creator-facing TCP endpoint. This is not an HTTP URL.
BITRIVER_SRS_PUBLIC_RTMP_BASE_URL=rtmp://ingest.example.com:1935/live
```

Do not put the Compose-only names `ome`, `srs`, or `transcoder-public` in a
viewer-facing URL. Re-apply the stack after editing `.env`:

```bash
docker compose --env-file .env -f deploy/docker-compose.yml config
docker compose --env-file .env -f deploy/docker-compose.yml up -d
```

## 2. DNS and TLS

Create two records:

- `stream.example.com`: point it at NPM and enable Cloudflare proxying when you
  want Cloudflare in the HTTP path.
- `ingest.example.com`: use a DNS-only record for direct RTMP, or point it at a
  TCP proxy you operate. A normal Cloudflare orange-cloud record does not proxy
  arbitrary RTMP traffic.

For the HTTP hostname, use Cloudflare **Full (strict)** and install a valid
origin certificate in NPM. Enable HTTPS redirect and HTTP/2. Websocket support
must remain enabled for live chat and realtime control-plane routes.

## 3. NPM proxy host

Create a Proxy Host for `stream.example.com`:

- Scheme: `http`
- Forward host/IP: the Ubuntu VM address reachable from NPM
- Forward port: `8080`
- Websockets Support: enabled
- Block Common Exploits: enabled
- SSL: the origin certificate selected, Force SSL enabled

The default location carries `/`, `/viewer`, `/admin`, `/api`, and `/live` to
BitRiver on port `8080`. Do not create a separate `/live` location; the Go edge
preserves that path and streams OME's private response.

Add one custom location for transcoder renditions:

- Location: `/hls/`
- Scheme: `http`
- Forward host/IP: the same Ubuntu VM
- Forward port: `9080`

Preserve the `/hls/` prefix. The shipped nginx sidecar maps it to the read-only
transcoder public directory.

If NPM reaches the VM over an untrusted network, firewall ports `8080` and
`9080` so only the NPM host can connect. Do not expose OME port `8081`, SRS API
ports, Postgres, or Redis publicly.

## 4. RTMP ingest

Expose TCP `1935` using one of these patterns:

1. Forward `ingest.example.com:1935` directly to the Ubuntu VM's SRS port and
   restrict source networks where practical.
2. Create an NPM **Stream** entry for TCP `1935` that forwards to the VM's SRS
   port.
3. Keep ingest private over a LAN or VPN and set
   `BITRIVER_SRS_PUBLIC_RTMP_BASE_URL` to that reachable address.

The NPM Proxy Hosts screen is HTTP-only and cannot carry RTMP. Never publish
the SRS controller/raw API alongside the ingest port.

## 5. Forwarded-client trust

Only enable forwarded-IP trust when every proxy hop is known and pinned:

```dotenv
BITRIVER_LIVE_RATE_TRUST_FORWARDED_HEADERS=true
BITRIVER_LIVE_RATE_TRUSTED_PROXIES=10.0.10.5/32
```

Add the actual NPM address/CIDR and, when Cloudflare connects directly to NPM,
the current Cloudflare proxy ranges according to your network policy. Leaving
trust disabled is safer than accepting spoofable forwarding headers.

## 6. Acceptance checklist

Validate from outside the home network, not only from the VM:

1. `https://stream.example.com/healthz`, `/readyz`, `/viewer`, and `/admin`
   return successfully.
2. Admin login survives a hard refresh and no browser console shows mixed
   content or CORS failures.
3. A test encoder publishes to `rtmp://ingest.example.com:1935/live` with its
   stream key in the separate key field.
4. The channel transitions live and
   `https://stream.example.com/live/<channel-id>/llhls.m3u8` returns through the
   main NPM proxy host.
5. Several seconds of video and audio decode from that public LL-HLS URL.
6. Every advertised rendition under `/hls/` returns successfully.
7. Chat upgrades to websocket (`101`) and messages arrive in a second session.
8. Stopping the encoder returns the channel offline without stale jobs.

OME process health by itself is not playback proof. If the `/live/` check
fails, inspect BitRiver and OME logs together and verify that the authenticated
OME manager route can read `default/live` before changing proxy rules.

## Related docs

- [Ubuntu source-checkout deployment](installing-on-ubuntu.md)
- [Single-host production baseline](production-single-host.md)
- [Deployment contract](contract.md)
- [Security](security.md)
- [Operations](operations.md)
