# Security checklist

Use this quick checklist before merging security-sensitive changes:

- [ ] No credentials, private keys, or local secret dumps are committed.
- [ ] CI `Committed secret file guard` passes (`./scripts/check-no-committed-secrets.sh`).
  - This guard blocks tracked root `.env`, private key/cert bundle artifacts (`*.pem`, `*.key`, `*.p12`, `*.pfx`, `id_rsa`, `id_ed25519`), and common local secret dump files (`*.secret`, `*.secrets`, `*.env.local`).
  - Intended examples remain allowed (for example `deploy/.env.example`).
# Security

This document summarizes security-sensitive operator workflows for BitRiver Live.

## Secret handling with `_FILE`

`bitriver env validate` supports `<KEY>_FILE` for required secret-like keys so operators can mount secrets as files instead of inlining plaintext values in `.env`.

Supported pattern:

- Direct value: `BITRIVER_POSTGRES_PASSWORD=...`
- File value: `BITRIVER_POSTGRES_PASSWORD_FILE=/run/secrets/bitriver_postgres_password`

Resolution rules:

1. If `BITRIVER_POSTGRES_PASSWORD` is non-empty, it wins.
2. If both direct and `_FILE` are set, validation warns and keeps the direct value.
3. If direct is empty and `_FILE` is set, validation reads and trims file content.
4. Missing/unreadable `_FILE` path is a validation error.
5. Empty/whitespace-only file content is treated as missing.

Use this consistently for sensitive values such as admin password, database/redis passwords, OME/SRS/transcoder/API tokens, and chat queue redis password.

## Operator workflow

1. Mount secrets from your secret store to files (for example under `/run/secrets`).
2. Set matching `*_FILE` variables in `.env` (see `deploy/.env.example` comments).
3. Validate before rollout:
# Security hardening guide

This is the operator-facing security entrypoint for BitRiver Live deployments using the canonical contract:

- `deploy/docker-compose.yml`
- `deploy/.env.example` (copied to your root `.env`)
- `go run ./cmd/bitriver env validate --env-file ./.env`

Use this guide before exposing your stack publicly, and revisit it during each release/rotation window.

## 1) Threat model summary

### Public attack surface

Assume anything bound to host ports in `deploy/docker-compose.yml` is internet-reachable unless a firewall/reverse proxy denies it.

Primary externally exposed paths in a default deployment:

- **`bitriver-live` HTTP API/control center** (`BITRIVER_LIVE_PORT`, default `8080`)
- **Viewer route** (typically proxied through `bitriver-live` at `/viewer`, backed by the `viewer` service)
- **Ingest/transcoder exposure** (`srs` RTMP, `transcoder`, `transcoder-public`)
- **Optional host-published internals** (`postgres-host` and `srs-api` profiles)
- **OME ports** (HTTP, TLS, LL-HLS, signaling, relay, ICE)

### Trusted boundary

Treat the Compose-internal network as the trusted boundary for service-to-service links:

- `bitriver-live -> postgres`, `redis`, `srs-controller`, `ome`, `transcoder`
- `viewer -> bitriver-live`
- Internal-only defaults (for example, `viewer` uses `expose`, not host `ports`)

Security posture: keep internal service links private and only publish the minimum required public entrypoints.

### Sensitive assets

Protect at-rest and in-transit handling for:

- Admin/session credentials (`BITRIVER_LIVE_ADMIN_*`, session store values)
- OME/SRS/transcoder auth material (`BITRIVER_OME_*`, `BITRIVER_SRS_TOKEN`, `BITRIVER_TRANSCODER_TOKEN`)
- Postgres data (users, channels, metadata, audit data)
- Redis-backed chat queue data (`BITRIVER_LIVE_CHAT_QUEUE_*`)

## 2) Network exposure guidance

Use this as the default exposure policy.

| Service | Port(s) in compose | Exposure policy |
|---|---:|---|
| `bitriver-live` | `8080` (`BITRIVER_LIVE_PORT`) | **Public by default** behind reverse proxy/TLS. |
| `viewer` | internal `3000` via `expose` | **Must stay internal** (reachable through API/proxy route). |
| `srs` | `1935` (`BITRIVER_SRS_RTMP_PORT`) | Public only if you accept external RTMP publishers; otherwise restrict by firewall/IP allowlist. |
| `srs-controller` | host `1986 -> 1985` (`BITRIVER_SRS_CONTROLLER_PORT`) | Prefer internal/private access; avoid internet exposure. |
| `srs-api` profile | host `1985` (`BITRIVER_SRS_API_PORT`) | **Debug-only** profile; do not keep enabled in production. |
| `ome` | `8081`, `8082`, `8080/8083`, `8443`, `9000`, `9443`, `3478`, `10000-10009` | Publish only ports required by your playback protocol; avoid broad open exposure. |
| `transcoder` | host `9001 -> 9000` (`BITRIVER_TRANSCODER_HOST_PORT`) | Keep internal/private where possible; if exposed, protect with token + firewall. |
| `transcoder-public` | `9080` (`BITRIVER_TRANSCODER_PUBLIC_PORT`) | Public only when serving HLS directly; otherwise front with CDN/proxy. |
| `postgres` | no direct host port by default | **Must stay internal** unless temporary maintenance requires host publish. |
| `postgres-host` profile | host `5432` (`BITRIVER_POSTGRES_HOST_PORT`) | **Debug-only** profile; keep disabled for normal production runtime. |
| `redis` | no host ports | **Must stay internal**. |

Profile guidance:

- `COMPOSE_PROFILES=postgres-host` and `COMPOSE_PROFILES=srs-api` should be treated as **temporary troubleshooting modes** only.
- If you enable either profile, constrain source IPs at the host firewall and disable immediately after use.

## 3) Reverse proxy + TLS recommendation

For public deployments:

1. Front all public HTTP endpoints with a reverse proxy/load balancer that terminates TLS.
2. Forward only sanitized forwarding headers (`X-Forwarded-For`, `X-Forwarded-Proto`, `Host`).
3. Keep direct container host ports firewalled from the public internet whenever possible.

Hardening alignment with this stack:

- Keep `BITRIVER_LIVE_RATE_TRUST_FORWARDED_HEADERS=false` unless you explicitly pin `BITRIVER_LIVE_RATE_TRUSTED_PROXIES`.
- Keep `BITRIVER_LIVE_UPLOADS_TRUST_FORWARDED_HEADERS=false` unless your trusted proxy chain is pinned and header sanitation is guaranteed.
- Ensure proxy trust/rate-limit settings match your real network path before enabling forwarded-header trust.

## 4) Auth/session/cookie settings guidance

Baseline production settings (from `deploy/.env.example` + env validation expectations):

- Keep `BITRIVER_LIVE_MODE=production` in the saved `.env`.
- Keep `BITRIVER_LIVE_ALLOW_SELF_SIGNUP=false` unless your policy explicitly supports open registration.
- Enforce login throttling (`BITRIVER_LIVE_RATE_LOGIN_LIMIT`, `BITRIVER_LIVE_RATE_LOGIN_WINDOW`) with non-zero production values.
- Set `BITRIVER_LIVE_SESSION_TTL` to a finite value and enable `BITRIVER_LIVE_SESSION_IDLE_TIMEOUT` for shorter inactivity expiry in high-risk environments.

Cookie/session expectations:

- Use secure transport (HTTPS) so session cookies are always `Secure` end-to-end.
- Keep cookies `HttpOnly`; do not rely on browser-accessible session tokens.
- Only enable cross-site cookie mode when your deployment topology requires it, and pair that with explicit CORS origin allowlists.

Origin and CORS alignment:

- `BITRIVER_LIVE_ADMIN_CORS_ORIGINS` and `BITRIVER_LIVE_VIEWER_CORS_ORIGINS` must exactly match public operator/viewer origins.
- `NEXT_PUBLIC_API_BASE_URL`, `NEXT_PUBLIC_VIEWER_URL`, and proxy routes must align with those same public origins.

Validation command (run before restart/release):

```bash
go run ./cmd/bitriver env validate --env-file ./.env
```

## 5) Admin bootstrap practices

- Bootstrap exactly one initial admin account.
- On first login, rotate the bootstrap password immediately.
- Limit admin account count; issue named operator accounts instead of shared credentials.
- Remove temporary/bootstrap credentials from terminals, shell history snippets, and ticket comments.

Recommended flow:

1. Run quickstart/install.
2. Login with bootstrap admin.
3. Force password reset/rotation.
4. Create least-privilege operational accounts.
5. Disable/delete bootstrap artifacts.

## 6) Secret rotation approach

Use an inventory + cadence model so rotation is scheduled, not ad hoc.

### Rotatable inventory by class

- **Platform auth:** `BITRIVER_LIVE_ADMIN_PASSWORD`, `BITRIVER_LIVE_METRICS_TOKEN`
- **Media/control tokens:** `BITRIVER_SRS_TOKEN`, `BITRIVER_OME_API_TOKEN`, `BITRIVER_OME_HEALTHCHECK_TOKEN`, `BITRIVER_TRANSCODER_TOKEN`
- **Data-plane credentials:** `BITRIVER_POSTGRES_PASSWORD`, `BITRIVER_REDIS_PASSWORD`, `BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD`

### Suggested cadence

- High-impact credentials (admin, control tokens): every 30–90 days.
- Database/cache credentials: every 90 days or after staff/vendor boundary changes.
- Immediate rotation after suspected leak or operator offboarding.

### Staged order to reduce downtime

1. Introduce new secret in source-of-truth secret store.
2. Update dependent services/config.
3. Restart/redeploy affected containers.
4. Confirm health (`/readyz`, `/healthz`, `/api/status`).
5. Revoke old secret.

### `_FILE`-based secret mounts

When `_FILE` secret inputs are implemented in this stack, prefer mounted secret files over plaintext values in `.env`.
Until then, keep `.env` access tightly controlled and never commit real secrets.

## 7) Logging guidance

### Never log

- Passwords, tokens, API keys
- Full DSNs with embedded credentials
- Raw Authorization/session headers

### Redaction patterns

- Emit `value=<redacted>` for startup/auth checks.
- Log secret source names (for example, env key name), not secret values.
- For connection strings, strip credential/userinfo components before logging.

### Startup/health log examples

Good:

- `ome healthcheck auth_mode=accesstoken source=BITRIVER_OME_API_TOKEN value=<redacted>`
- `postgres connection established using configured DSN (credentials redacted)`

Avoid:

- `BITRIVER_OME_API_TOKEN=...`
- `postgres://user:password@host/db`

## 8) Production security checklist

Before exposing or updating production, confirm all items:

- [ ] TLS is enabled at the public edge and HTTP-to-HTTPS redirects are enforced.
- [ ] Only required service ports are publicly reachable; internal services remain private.
- [ ] `postgres-host` and `srs-api` optional profiles are disabled outside troubleshooting windows.
- [ ] All sample/default credentials from `deploy/.env.example` are replaced.
- [ ] `go run ./cmd/bitriver env validate --env-file ./.env` passes.
- [ ] Secrets come from an approved source with restricted access (and plaintext `.env` exposure is minimized).
- [ ] Last secret rotation date is recorded and within policy.
- [ ] `/metrics` access is protected (token and/or network allowlist).
- [ ] Backups/restores are tested and recovery logs are current.
- [ ] Audit/security log forwarding and retention controls are enabled.
