# Secrets hardening

BitRiver Live’s canonical deployment path is a repository-root `.env` consumed by `deploy/docker-compose.yml` (plus generated OME config). That baseline is supported and expected by quickstart, release runbooks, and helper scripts.

This guide explains how to harden secret handling **without breaking that contract**.

## 1) Supported baseline and risk profile

### Baseline the repo supports today

- One root `.env` file (usually copied from `deploy/.env.example`) drives runtime credentials and tokens.
- `deploy/check-env.sh` and `go run ./cmd/bitriver env validate --env-file ./.env` validate required production fields.
- Runtime launchers (`quickstart`, `compose up`, release workflows) expect environment values, not direct secret-manager APIs.

### Risks to manage when using `.env`

- **At-rest exposure:** plaintext secrets in a filesystem file can leak via backups, snapshots, or overly broad permissions.
- **Process/shell exposure:** ad-hoc export patterns can leak values into shell history or process inspection tooling.
- **Reuse drift:** copying one `.env` across environments can accidentally share credentials between dev/staging/prod.
- **Rotation lag:** static secrets tend to persist unless operators enforce a rotation cadence.

The rest of this document keeps the `.env` contract intact while reducing those risks.

## 2) Hardening patterns compatible with this repo

### A. CI-injected job-local environment files

CI may materialize a deployment-specific environment file for validation, but that file is a job-local secret, not an artifact.

Required approach:

1. Keep only the non-secret template in Git (`deploy/.env.example`).
2. Materialize production values under the runner's temporary directory with mode `0600`.
3. Run `deploy/check-env.sh` and image-digest validation in that same job.
4. Scan captured output against the exact injected values and secret-shaped content before displaying or retaining anything.
5. Retain only fixed-schema status evidence with no values, DSNs, rendered credential config, or raw validator logs.
6. Delete the environment input, sentinel list, and raw logs in an always-running cleanup step.

Never upload or download a populated `.env`, secret mount, private key, credential-bearing DSN, or generated OME config. A deployment job must acquire its own credentials from the approved CI or host secret source instead of consuming credentials from a build artifact.

This preserves the repository's `.env` runtime contract while preventing CI artifact storage from becoming a secret distribution system. Keep environment-specific CI contexts isolated and grant production secret access only to jobs that validate or deploy production.

### B. Mounted env files with strict filesystem permissions

When deploying on hosts (Compose or systemd wrappers), keep `.env` files outside shared home/work directories and lock permissions.

Recommended host controls:

- Store deployment `.env` in an operator-owned directory (for example under `/opt/bitriver-*` for systemd installs).
- Apply restrictive ownership and mode before starting services:
  - owner: deployment service user (or root-managed with least privilege)
  - mode: `0600` (or stricter where supported)
- Prevent accidental world/group reads from backup tooling, sync jobs, and support bundles.
- Treat deployment-time rendered config containing credentials (such as OME output) as sensitive data in backup and access policies. The tracked OME generated file and release packages must contain example placeholders only.

This pattern remains compatible because services still read plain environment variables; only file placement and host ACLs change.

### C. Docker/host secret distribution approaches

BitRiver Live does not require a specific external secret manager integration in-repo. You can still use one of these compatible operational approaches:

- **Host secret manager → rendered `.env` file:** fetch secrets on the host at deploy time, write `.env` with strict permissions, then run existing commands.
- **Orchestrator secret store → env injection:** where your platform can project secrets as environment variables, materialize a runtime `.env` (or equivalent env map) that preserves expected variable names.
- **Image/source separation:** keep `BITRIVER_DEPLOY_IMAGE_SOURCE=pull` in production and separate registry credentials from app credentials.

Guardrail: regardless of source, keep exported variable names aligned with `deploy/.env.example` and validate with the existing scripts before rollout.

## 3) Placeholder hygiene for `deploy/.env.example`

`deploy/.env.example` is documentation and a bootstrap template, so secret-bearing values must stay obviously fake.

Conventions enforced by `./scripts/check-env-example-placeholders.sh`:

- Required credential keys from `x-required-credentials` in `deploy/docker-compose.yml` must be present and non-empty.
- Secret-bearing placeholders (`*PASSWORD*`, `*TOKEN*`, `*SECRET*`, `*KEY*`) must include a sample marker such as `-example`, `_example`, `Example`, `sample`, `placeholder`, or `changeme`.
- Email placeholders must use the `example.com` domain.
- Long high-entropy token-like values are rejected unless clearly marked as examples.

Use these patterns when editing templates:

- `BITRIVER_SRS_TOKEN=srs-secure-token-example`
- `BITRIVER_OME_API_TOKEN=OME-Example-Access-Token`
- `BITRIVER_LIVE_ADMIN_EMAIL=admin@stream.example.com`

## 4) Operator checklist

Use this short checklist for every environment:

- [ ] **Unique per-environment credentials:** never reuse production secrets in staging/dev.
- [ ] **Rotation cadence:** define and follow a schedule for admin, database, cache, ingest, and API tokens.
- [ ] **No sample/default secrets:** confirm `deploy/check-env.sh` passes and defaults from `deploy/.env.example` are replaced.
- [ ] **Backup/restore handling:** include secret material handling in backup policy (encryption, restricted restore access, and post-restore rotation where required).
- [ ] **Access control and audit:** limit secret read/write permissions to release operators and record secret change events in your change-management/audit trail.

## 5) Practical rollout workflow

1. Start from `deploy/.env.example` and fill environment-specific values.
2. Generate/store the resulting `.env` through CI or host secret tooling (not Git).
3. Lock filesystem permissions on `.env` (and any rendered secret-bearing config).
4. Run:
   - `deploy/check-env.sh`
   - `go run ./cmd/bitriver env validate --env-file ./.env`
   - `docker compose -f deploy/docker-compose.yml config`
5. Deploy via the standard quickstart/compose path.
6. Document rotation owner + next rotation date in release/change records.

This keeps operations aligned with repository behavior while materially improving secret hygiene.

## 6) Using *_FILE secrets with Docker Compose mounts

`go run ./cmd/bitriver env validate --env-file ./.env` supports file-backed secret values for required secret keys using the `<KEY>_FILE` convention.

Behavior:

- Set `BITRIVER_SOME_SECRET` directly for inline env values.
- Or set `BITRIVER_SOME_SECRET_FILE` to a readable file path containing the secret.
- If **both** are set, direct `BITRIVER_SOME_SECRET` takes precedence and validation emits a warning so precedence is explicit and deterministic.
- If `_FILE` points to a missing/unreadable path, validation reports an error.
- If `_FILE` is readable but resolves to an empty value after trailing newline trimming, validation treats the effective secret as missing.

Example pattern:

```env
BITRIVER_POSTGRES_PASSWORD_FILE=/run/secrets/bitriver_postgres_password
BITRIVER_REDIS_PASSWORD_FILE=/run/secrets/bitriver_redis_password
BITRIVER_LIVE_ADMIN_PASSWORD_FILE=/run/secrets/bitriver_live_admin_password
```

Recommended operator workflow:

1. Mount secrets to files on the host/orchestrator (for example, `/run/secrets/...`).
2. Reference those paths with `*_FILE` variables in your environment file.
3. Run `go run ./cmd/bitriver env validate --env-file ./.env` before deploy.
4. Keep file permissions restricted (`0600` where applicable) and rotate secrets per environment policy.


Concrete Docker Compose mount example:

```yaml
# deploy/docker-compose.override.yml (operator-managed)
services:
  bitriver-live:
    volumes:
      - /srv/bitriver/secrets:/run/secrets:ro
  srs-controller:
    volumes:
      - /srv/bitriver/secrets:/run/secrets:ro
  transcoder:
    volumes:
      - /srv/bitriver/secrets:/run/secrets:ro
```

```env
# .env
BITRIVER_POSTGRES_PASSWORD_FILE=/run/secrets/bitriver_postgres_password
BITRIVER_REDIS_PASSWORD_FILE=/run/secrets/bitriver_redis_password
BITRIVER_LIVE_ADMIN_PASSWORD_FILE=/run/secrets/bitriver_live_admin_password
BITRIVER_SRS_TOKEN_FILE=/run/secrets/bitriver_srs_token
BITRIVER_OME_PASSWORD_FILE=/run/secrets/bitriver_ome_password
BITRIVER_OME_API_TOKEN_FILE=/run/secrets/bitriver_ome_api_token
BITRIVER_TRANSCODER_TOKEN_FILE=/run/secrets/bitriver_transcoder_token
BITRIVER_LIVE_METRICS_TOKEN_FILE=/run/secrets/bitriver_live_metrics_token
```
