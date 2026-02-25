# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Review placeholder-like keys in `deploy/.env.example`
  - Acceptance criteria:
    - Placeholder/sample domain, email, password, and token values are identified for replacement in runtime `.env`.
  - Relevant checks:
    - `rg -n "example\\.com|example|Example|secure-token-example|admin@" deploy/.env.example`

- [x] Task 2 — Create/update deployment `.env` with non-placeholder production values
  - Acceptance criteria:
    - Root `.env` exists.
    - Placeholder credentials/domains are replaced with non-sample values.
    - `BITRIVER_LIVE_MODE=production` remains set.
    - `BITRIVER_LIVE_RATE_LOGIN_LIMIT` and `BITRIVER_LIVE_RATE_LOGIN_WINDOW` are non-zero/non-empty.
    - Viewer URL vars point to the selected public endpoints.
  - Relevant checks:
    - `test -f .env`
    - `rg -n "^(BITRIVER_LIVE_MODE|BITRIVER_LIVE_RATE_LOGIN_LIMIT|BITRIVER_LIVE_RATE_LOGIN_WINDOW|NEXT_PUBLIC_VIEWER_URL|NEXT_PUBLIC_API_BASE_URL|BITRIVER_PUBLIC_DOMAIN)=" .env`
    - `rg -n "(example\\.com|admin@stream|Example|secure-token-example|Sup3rSecureAdmin)" .env`

- [x] Task 3 — Validate deployment env consistency
  - Acceptance criteria:
    - `deploy/check-env.sh .env` passes.
  - Relevant checks:
    - `deploy/check-env.sh .env`

## Execution log
- ✅ `rg -n "example\.com|example|Example|secure-token-example|admin@" deploy/.env.example` (identified placeholder-like defaults for domains, emails, passwords, and tokens)
- ✅ `cp deploy/.env.example .env`
- ✅ `test -f .env`
- ✅ `rg -n "^(BITRIVER_LIVE_MODE|BITRIVER_LIVE_RATE_LOGIN_LIMIT|BITRIVER_LIVE_RATE_LOGIN_WINDOW|NEXT_PUBLIC_VIEWER_URL|NEXT_PUBLIC_API_BASE_URL|BITRIVER_PUBLIC_DOMAIN)=" .env`
- ✅ `rg -n "(example\.com|admin@stream|Example|secure-token-example|Sup3rSecureAdmin)" .env || true` (only comment text contains example markers; active values replaced)
- ❌ `deploy/check-env.sh .env` (initially failed due HTTPS viewer/API without TLS cert/key and placeholder OME bind/IP, then missing required production image digests)
- ✅ `deploy/check-env.sh .env`
- ✅ Updated `.env` deployment endpoints and security-critical keys: `BITRIVER_LIVE_MODE=production`, login throttling (`10`/`1m`), viewer/API URLs (`http://live.bitriver.net/viewer`, `http://live.bitriver.net`), OME bind/IP (`live.bitriver.net`), and required production image digests (`BITRIVER_*_IMAGE_DIGEST`).
