# Production release runbook

This checklist keeps production releases consistent across the API, viewer, and
supporting services. Follow each section in order before publishing a new tag or
rolling out the artefacts to your infrastructure.

For the promotion ladder that explains which checks are blocking or advisory at
each stage, read [`docs/release-gates.md`](release-gates.md) before changing CI,
release workflows, or operator-facing deployment behavior.

**Canonical deployment path:** Production rollouts must flow through the
repository-root `.env`, `deploy/docker-compose.yml`, and their guardrails
(`deploy/check-env.sh`, `scripts/render-ome-config.sh`). Go CLI shims and CI
workflows wrap these artefacts rather than inventing alternate deployment
paths. Set `BITRIVER_LIVE_MODE=production` in the release `.env` before
starting the API; the binary now fails fast when the mode is missing so
production-only protections (including `/metrics` guardrails) stay enabled.
Production also refuses to start without a non-zero login throttle
(`BITRIVER_LIVE_RATE_LOGIN_LIMIT`/`BITRIVER_LIVE_RATE_LOGIN_WINDOW`) so
password spray protection is always enabled in release builds.

For production runtime safety, prefer enabling the resource limits overlay
(`deploy/docker-compose.limits.yml`) with the canonical stack:

```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.limits.yml up -d
```

The same behavior is available through the CLI wrapper:

```bash
go run ./cmd/bitriver quickstart --limits
```

Tune service limits through the `BITRIVER_*_CPUS`, `BITRIVER_*_MEM`, and
`BITRIVER_*_MEM_RESERVATION` variables in `.env`; `cmd/bitriver env validate`
now sanity-checks these values before deployment.

Recent schema changes to account for:

- `0002_chat_filters.sql` adds `chat_filters` and `chat_automod_actions` to
  support automated chat moderation filters and action logs.
- `0006_profile_social_links.sql` adds a `social_links` JSONB column to
  `profiles` so broadcasters can surface their external accounts. Ensure this
  migration is applied during rollout.
- `0007_auth_mfa.sql` adds `auth_mfa` and `auth_mfa_challenges` for TOTP-based
  MFA. Apply the migration before you promote new admin/creator logins so
  privileged accounts can complete MFA verification.

For the upgrade mechanics around schema migrations, safe Compose sequencing, `.env` changes, and OvenMediaEngine config regeneration, follow the upgrade essentials in [`docs/upgrades.md`](upgrades.md#upgrade-essentials-migrations-env-updates-and-ome-re-render).
For secret management hardening patterns that keep the same `.env` + Compose contract, see [`docs/secrets-hardening.md`](secrets-hardening.md).

## 1. Pre-release verification

Run every test suite locally (or on a staging CI run) so the GitHub release
workflow does not discover failures after the tag is pushed.

For the default local quality gate, run `./scripts/verify.sh`; it covers Go and contract checks plus Docker-gated Compose validation and quickstart smoke (`./scripts/test-quickstart.sh`) in deterministic order when Docker is available.

Before tagging or promoting a release candidate, run the named golden-path release gate and attach its artifact directory to the release ticket/change request:

```bash
./scripts/release-gate-smoke.sh --tier full --target vX.Y.Z
```

The full tier writes `.artifacts/release-gate/release-gate-report.json`, redacted env evidence, a contract snapshot, Compose config output, quickstart/smoke logs, and Compose diagnostics. If Docker Compose is not available on a local review machine, the fast tier can still produce non-mutating evidence, but the full tier must pass on a Docker-capable release-candidate host before tagging:

```bash
./scripts/release-gate-smoke.sh --tier fast --target vX.Y.Z
```

### GitHub Actions supply-chain pinning

All workflow `uses:` references must pin to immutable commit SHAs rather than
floating major tags. Keep a trailing comment beside each `uses:` entry that
records the human-readable upstream release tag (`# v4`, `# v5`, etc.) so
reviewers can still audit intent quickly.

Dependabot (`.github/dependabot.yml`) is the approved mechanism for bumping
these SHAs. Review and merge its GitHub Actions update PRs routinely so
security patches from upstream actions are not delayed.

### Go unit tests

```bash
GOTOOLCHAIN=local \
  GOPROXY=off \
  GOSUMDB=off \
  go test ./... -count=1 -timeout=120s
```

### Postgres storage tests

Point `BITRIVER_TEST_POSTGRES_DSN` at an empty, migrated database and execute
the tagged suite. The helper spins up a disposable Postgres container, applies
the tracked migrations, and runs the integration tests:

```bash
./scripts/test-postgres.sh
```

### Viewer lint and integration tests

Run the viewer quality gates from the repository root.

Use `npm ci` for lockfile-faithful release validation, and run it from
`web/viewer`.

```bash
cd web/viewer
npm ci
npm run lint
npm run test:integration
```

### Postgres artifact pgx guard

Before producing release binaries or container images that expect `BITRIVER_LIVE_STORAGE_DRIVER=postgres`, run the pgx guard to verify the build is not linking the stubbed `third_party` pgx module:

```bash
./scripts/check-postgres-pgx.sh postgres
```

If this fails with `pgx.IsStub=true`, switch the build job to the approved non-stub pgx source path (vendored real module mirror or controlled replace strategy) before publishing artifacts. In Docker release mode (`BITRIVER_PGX_MODE=real`), also drop stubbed transitive replacements (for example `golang.org/x/text`) before `go mod download` so pgx dependencies like `secure/precis` resolve from full upstream modules.

### Legal publication checks

Before cutting a release candidate:

- Confirm policy documents are published and versioned: `docs/legal/terms.md`, `docs/legal/privacy.md`, `docs/legal/dmca.md`, and `docs/legal/age-policy.md`.
- Verify the current policy version/date is exposed in your public release notes or site footer.
- Verify DMCA contact metadata (designated agent name, email, and mailing address) is present in published operator docs and matches the values used in support workflows.
- Run a dry-run DMCA intake (`POST /api/legal/dmca`) and admin triage flow in staging.

### Backup and restore release gates

Before tagging production releases, prove backup/restore readiness:

- Confirm at least one **successful backup in the last 24 hours** (from scheduler logs, object storage object timestamp, or job history).
- Confirm the latest backup has both archive and checksum (`.sha256`) artefacts.
- Provide **restore rehearsal evidence within the last 30 days** using `./scripts/restore-postgres.sh` (or equivalent staged workflow) including smoke query output.
- If the release includes schema-heavy changes, run an extra restore rehearsal after migrations are validated in staging.

Keep this evidence attached to the release ticket/change request before maintenance begins.

## 2. Tag the release and trigger the workflow

1. Ensure `CHANGELOG.md` (when present) and version references are up to date.
2. Create an annotated tag that follows the `vMAJOR.MINOR.PATCH` pattern:
   ```bash
   git tag -a vX.Y.Z -m "Release vX.Y.Z"
   git push origin vX.Y.Z
   ```
3. The push triggers [`.github/workflows/release.yml`](../.github/workflows/release.yml),
   which rebuilds the Go binaries for every platform, packages the viewer
   bundle, and publishes the artefacts to the GitHub Release. Monitor the
   workflow until every job completes successfully.

The release workflow's `Build binaries` step must compile Postgres-aware
targets with `-tags postgres` (`cmd/server`,
`cmd/tools/bootstrap-admin`, and `cmd/tools/migrate-json-to-postgres`) so the
published binaries include the real pgx-backed repository implementation. The
same step also runs a Linux `amd64` smoke check that starts `bitriver-live`
with Postgres storage flags and fails if output contains `pgx driver stubbed in
this build`, preventing future workflow edits from regressing to stubbed
storage builds.

### Repository secrets for the release workflow

The `verify-env` job in the release workflow renders a production-ready `.env`
file, validates it with `deploy/check-env.sh`, and then enforces third-party
image digest pins via `scripts/require-image-digests.sh`. Configure the
following
repository secrets (mirroring [`deploy/.env.example`](../deploy/.env.example))
so the job can populate every required variable and image tag:

The workflow sets `BITRIVER_LIVE_MODE=production` and
`BITRIVER_DEPLOY_IMAGE_SOURCE=pull` directly in the job (not from secrets) so
digest enforcement always runs under production conditions for release tags.

- `BITRIVER_POSTGRES_USER`
- `BITRIVER_POSTGRES_PASSWORD`
- `BITRIVER_REDIS_PASSWORD`
- `BITRIVER_OME_API`
- `BITRIVER_LIVE_ADMIN_EMAIL`
- `BITRIVER_LIVE_ADMIN_PASSWORD`
- `BITRIVER_LIVE_SESSION_TTL` (use the same duration as `deploy/.env.example`,
  currently `168h`, unless your session policy requires a shorter window)
- `BITRIVER_LIVE_ALLOW_SELF_SIGNUP` (set to `false` in most production deploys)
- `BITRIVER_LIVE_METRICS_TOKEN` (required by the release workflow) so
  `/metrics` remains protected
- `BITRIVER_LIVE_RATE_LOGIN_LIMIT` (set to a non-zero value, such as `10`)
- `BITRIVER_LIVE_RATE_LOGIN_WINDOW` (for example `1m`, paired with
  `BITRIVER_LIVE_RATE_LOGIN_LIMIT` to cap password spray attempts)
- `BITRIVER_SRS_TOKEN`
- `BITRIVER_OME_USERNAME`
- `BITRIVER_OME_PASSWORD`
- `BITRIVER_OME_API_TOKEN` (override `BITRIVER_OME_ACCESS_TOKEN` only if the health probe must use a different token)
- `BITRIVER_OME_BIND`, `BITRIVER_OME_IP`, `BITRIVER_OME_SERVER_PORT`, and `BITRIVER_OME_SERVER_TLS_PORT`
- `BITRIVER_TRANSCODER_TOKEN`
- `BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD`
- `BITRIVER_TRANSCODER_PUBLIC_BASE_URL`
- `BITRIVER_LIVE_IMAGE_TAG`
- `BITRIVER_VIEWER_IMAGE_TAG`
- `BITRIVER_SRS_CONTROLLER_IMAGE_TAG`
- `BITRIVER_TRANSCODER_IMAGE_TAG`
- `BITRIVER_SRS_IMAGE_TAG`
- `BITRIVER_OME_IMAGE_TAG`
- `BITRIVER_REDIS_IMAGE_DIGEST`
- `BITRIVER_POSTGRES_IMAGE_DIGEST`
- `BITRIVER_SRS_IMAGE_DIGEST`
- `BITRIVER_OME_IMAGE_DIGEST`
- `BITRIVER_NGINX_IMAGE_DIGEST`
- `BITRIVER_ALPINE_3_IMAGE_DIGEST`
- `BITRIVER_ALPINE_3_19_IMAGE_DIGEST`
- `BITRIVER_DEBIAN_IMAGE_DIGEST`
- `NEXT_PUBLIC_API_BASE_URL`
- `NEXT_PUBLIC_VIEWER_URL`

The release workflow persists the verified `.env` from this job and reuses it
to render the production OvenMediaEngine config. The `build` matrix now fails
if `deploy/ome/Server.generated.xml` would change when rendered for the
tagged release, preventing stale placeholders from landing in the packaged
artefacts.

### Record image digests for production

After the release images are published, resolve their digests and record them in
the release notes (and/or update `deploy/.env.example`) so production deployments
can pin to immutable references:

```bash
docker buildx imagetools inspect ghcr.io/bitriver-live/bitriver-live:vX.Y.Z --format '{{.Manifest.Digest}}'
docker buildx imagetools inspect ghcr.io/bitriver-live/bitriver-viewer:vX.Y.Z --format '{{.Manifest.Digest}}'
docker buildx imagetools inspect ghcr.io/bitriver-live/bitriver-srs-controller:vX.Y.Z --format '{{.Manifest.Digest}}'
docker buildx imagetools inspect ghcr.io/bitriver-live/bitriver-transcoder:vX.Y.Z --format '{{.Manifest.Digest}}'
```

Capture any third-party image digests (`redis`, `postgres`, `ossrs/srs`,
`airensoft/ovenmediaengine`, `nginx`, and helper base images) you intend to pin
alongside the release so operators can mirror the same verified set in their
`.env` files.

For production Compose rollouts, keep `BITRIVER_DEPLOY_IMAGE_SOURCE=pull` and
preconfigure GHCR credentials (`docker login ghcr.io`) on every host before the
maintenance window. This keeps deploys pull-only, enables preflight manifest
checks, and avoids accidental source builds on production nodes.

## 3. Rotate credentials and validate environment files

Every deployment environment must own unique secrets. Before rolling the new
build out:

1. Copy the updated `deploy/.env.example` into the release directory or target
   host and fill in the values for Postgres, Redis, SRS, OvenMediaEngine, and
   transcoder credentials. Run `cmd/bitriver env init` when you need the tool to
   regenerate fresh secrets so none of the sample credentials from
   `deploy/.env.example` survive in production. Ensure `NEXT_PUBLIC_API_BASE_URL`
   and `NEXT_PUBLIC_VIEWER_URL` point at the public API and viewer endpoints
   users will reach (not localhost or example.com placeholders).
2. Run the guard scripts to confirm defaults are gone, service URLs match the
   target environment, and production image digests are pinned:
   ```bash
   deploy/check-env.sh
   ./scripts/require-image-digests.sh
   ```
   The release workflow surfaces this output in the deploy logs and fails when
   any OvenMediaEngine URLs, bind addresses, or ports point at loopback
   addresses, placeholders, or are missing.
   The same preflight now errors when `/metrics` lacks
   `BITRIVER_LIVE_METRICS_TOKEN`/`BITRIVER_LIVE_METRICS_ALLOW_NETWORKS` or the
   login throttling floor (`BITRIVER_LIVE_RATE_LOGIN_LIMIT`) is missing, so
   release deployments match the protections enforced by the runtime binary.
   Postgres DSNs in production must use TLS; keep `sslmode` at `require` or
   `verify-full` on both `BITRIVER_LIVE_POSTGRES_DSN` and
   `BITRIVER_LIVE_SESSION_POSTGRES_DSN`, and append
   `sslrootcert=/certs/postgres-ca.pem` (or a similar mounted path) when the
   database presents a private CA. The validator rejects `sslmode=disable` for
   any host other than the local Compose `postgres` service so plaintext
   connections cannot slip into a release.
3. Verify the rendered OvenMediaEngine config matches the image tag in `.env`
   before cutting the release tag or starting Compose:
   ```bash
   ./scripts/render-ome-config.sh --check
   ```
   The guard fails when `deploy/ome/Server.generated.xml` was rendered for a
   different `BITRIVER_OME_IMAGE_TAG`, ensuring the preflight stays in sync
   with the tag baked into the container image.
3. For systemd-based installs, refresh the `.env` files under `/opt/bitriver-*`
   and restart the services only after the script reports success. Ensure any
   container image tags (`BITRIVER_LIVE_IMAGE_TAG`, `BITRIVER_VIEWER_IMAGE_TAG`,
   etc.) match the newly published release.

### Helm-based releases

If you deploy via `deploy/helm/bitriver-live`, keep the same `.env` validation flow and then map the validated settings into your Helm values file:

1. Update `.env` with the new image tags and any new variables, then rerun the guard scripts:
   ```bash
   deploy/check-env.sh
   ./scripts/render-ome-config.sh --check
   ```
2. Translate the `.env` changes into your Helm values file (`values.prod.yaml`), updating `values.tags`, `values.env`, and `values.secrets` with the new release credentials and URLs.
3. Apply the upgrade:
   ```bash
   helm upgrade --install bitriver-live deploy/helm/bitriver-live -f values.prod.yaml
   ```
4. Confirm the pre-upgrade Postgres migration job completes and the API reports healthy status before reopening traffic:
   ```bash
   kubectl get jobs
   kubectl rollout status deployment/bitriver-live-api
   ```

## 4. Confirm ingest and object storage configuration

Review [`docs/advanced-deployments.md`](advanced-deployments.md) and verify the
following before rollout:

- SRS, OvenMediaEngine, and transcoder configuration directories point at the
  release you are deploying, and image tags match `vX.Y.Z`.
- Object storage variables (`BITRIVER_LIVE_OBJECT_*`) reference the intended
  endpoint, credentials, bucket, and lifecycle policies.
- Recording retention windows (`BITRIVER_LIVE_RECORDING_RETENTION_*`) align with
  the business requirements for VOD publishing and archival.

## Release notes and artifact consistency

Before publishing a tag/release, ensure notes follow the versioning contract:

- Apply `docs/versioning.md` rules (SemVer + breaking-change classification).
- Fill `.github/RELEASE_NOTES_TEMPLATE.md` for every release.
- Include explicit **Upgrade notes** and **Breaking changes** (or `None`) in release notes.
- For major releases, add a dedicated rollback safety statement that matches `docs/upgrades.md`.

## 5. Post-release smoke checks

Once the artefacts are live:

1. Verify the API reports the new version and serves the admin UI without
   console errors.
2. Load the viewer at `/viewer`, confirm linted assets are present, and stream a
   test channel end-to-end (RTMP ingest → HLS playback).
3. Inspect the database to ensure migrations completed and new tables/columns
   exist.
4. Upload a short VOD to confirm object storage credentials, prefixes, and
   retention windows are honoured.
5. Rotate any temporary credentials created during testing and archive the
   release artefacts in your asset registry for rollback.
