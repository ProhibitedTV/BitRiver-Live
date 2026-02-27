# Security checklist

Use this quick checklist before merging security-sensitive changes:

- [ ] No credentials, private keys, or local secret dumps are committed.
- [ ] CI `Committed secret file guard` passes (`./scripts/check-no-committed-secrets.sh`).
  - This guard blocks tracked root `.env`, private key/cert bundle artifacts (`*.pem`, `*.key`, `*.p12`, `*.pfx`, `id_rsa`, `id_ed25519`), and common local secret dump files (`*.secret`, `*.secrets`, `*.env.local`).
  - Intended examples remain allowed (for example `deploy/.env.example`).
