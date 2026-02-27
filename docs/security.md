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

```bash
go run ./cmd/bitriver env validate --env-file ./.env
```

4. Run full verification before merge/release:

```bash
./scripts/verify.sh
```

## Related docs

- [`docs/secrets-hardening.md`](secrets-hardening.md)
- [`docs/operations.md`](operations.md)
- [`docs/production-release.md`](production-release.md)
