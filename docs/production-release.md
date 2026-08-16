# Production release runbook

This checklist keeps production releases consistent across the API, viewer, and
supporting services. Follow each section in order before publishing a new tag or
rolling out the artefacts to your infrastructure.

For the promotion ladder that explains which checks are blocking or advisory at
each stage, read [`docs/release-gates.md`](release-gates.md) before changing CI,
release workflows, or operator-facing deployment behavior.

Current reference: [`v1.2.3-rc.13`](releases/v1.2.3-rc.13.md) completed the
signed publication and pull-only product gates. Its clean target-host promotion
checks remain open and must not be inferred from CI or hosted VM qualification.

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

## Release credential threat model

Release credentials are exposed if a workflow uploads a populated environment file, prints a secret-bearing validator error, packages a deployment-time generated config, or passes credentials to a downstream build job. GitHub Actions masking is a secondary safeguard, not permission to retain secret material.

The tag workflow enforces these boundaries:

- Synthetic validation values are materialized only inside `verify-env` and
  `pull-only-product-gate` under `$RUNNER_TEMP`; each job creates its own input
  and deletes it with an `always()` cleanup step.
- Missing, malformed, placeholder, and digest inputs remain validation failures, while validation stays compatible with supported `_FILE` secret sources.
- Validator logs and fixed-schema contract/product evidence are scanned against
  exact injected values. `release-contract-evidence` is retained for 3 days;
  the credential-free image-manifest and golden-path JSON in
  `release-product-evidence` are retained for 14 days.
- OME freshness renders from `deploy/.env.example` into a temporary output. The tracked generated XML and release packages contain placeholders, never deployment credentials.
- Every downloaded build artifact and the final `dist/` payload are scanned before publication. `release-publication-evidence` retains the SHA-256 artifact inventory and scan status for 3 days.
- Intermediate build artifacts have an explicit 7-day retention and are not a credential transport.

The evidence scanner rejects forbidden secret files, private-key material, credential-bearing URLs, secret-shaped assignments, and exact sentinels. A scan failure blocks publication and reports only a rule identifier and file path, not the matched value.

Before promoting a release, review the artifact inventory and both redacted evidence artifacts. If any credential was previously committed or uploaded, rotate it before rollout; deleting an artifact or replacing the tracked value does not revoke it.

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

Migration files are immutable after release. Every schema change must be a new forward migration. A destructive, data-transforming, or rollback-incompatible migration must have explicit release notes covering supported source versions, expected duration, compatibility while old/new binaries overlap, pre-upgrade backup and restore evidence, validation queries, and whether binary rollback requires database restore. Missing guidance blocks promotion.

For the upgrade mechanics around schema migrations, safe Compose sequencing, `.env` changes, and OvenMediaEngine config regeneration, follow the upgrade essentials in [`docs/upgrades.md`](upgrades.md#upgrade-essentials-migrations-env-updates-and-ome-re-render).
For secret management hardening patterns that keep the same `.env` + Compose contract, see [`docs/secrets-hardening.md`](secrets-hardening.md).

## 1. Pre-release verification

Run every test suite locally (or on a staging CI run) so the GitHub release
workflow does not discover failures after the tag is pushed.

For the default local quality gate, run `./scripts/verify.sh`; it covers Go and contract checks plus the real-Postgres migration lifecycle, Compose validation, and quickstart smoke (`./scripts/test-quickstart.sh`) in deterministic order when Docker is available.

Before tagging, run the content-level product acceptance gate from a clean
source checkout on a Docker-capable host:

```bash
./scripts/test-production-golden-path.sh \
  --stack quickstart \
  --client docker \
  --artifact-dir .artifacts/production-golden-path
```

Passing requires authenticated account/channel APIs, deterministic 1080p RTMP,
advancing and decodable OME and transcoder HLS, offline transition,
chat/moderation, VOD upload/transcode/publication/playback, and final aggregate
health. The wrapper scans its versioned JSON report against per-run secrets and
tears down the disposable stack. Attach only
`production-golden-path.json`; raw Compose logs and generated media-service
configuration are not release evidence.

Before tagging or promoting a release candidate, run the named golden-path release gate and attach its artifact directory to the release ticket/change request:

```bash
./scripts/release-gate-smoke.sh --tier full --target vX.Y.Z
```

The full tier writes `.artifacts/release-gate/release-gate-report.json`, redacted env evidence, a contract snapshot, Compose config output, quickstart/smoke logs, and Compose diagnostics. If Docker Compose is not available on a local review machine, the fast tier can still produce non-mutating evidence, but the full tier must pass on a Docker-capable release-candidate host before tagging:

```bash
./scripts/release-gate-smoke.sh --tier fast --target vX.Y.Z
```

These source-build gates prove the checked-out code and canonical Compose
contract. They do not prove that a published tag, digest-pinned image set, or
Ubuntu package is installable. Stable promotion still requires the same product
assertions in `--stack running` mode after pull-only installation on a clean
Ubuntu 24.04 amd64 candidate host, followed by the reboot/recovery evidence
described below.

After deploying the release candidate to staging or the production canary host, run the canary/rollback gate and attach its artifact directory to the change request:

```bash
docker compose -f deploy/docker-compose.yml --env-file ./.env logs --tail=200 > .tmp/recent-compose-logs.txt
./scripts/release-canary.sh \
  --base-url https://api.example.com \
  --logs-file .tmp/recent-compose-logs.txt \
  --rollback-notes .tmp/rollback-notes.md \
  --require-rollback-notes \
  --artifact-dir .artifacts/release-canary
```

The canary gate is non-mutating. It checks `/readyz`, `/healthz`, `/api/status`, and `/viewer`, saves redacted response artifacts, scans supplied logs for high-confidence rollout blockers, and verifies rollback notes cover previous version/tag, data or migration handling, env/config rollback, and artifact/image rollback path. Include the `postgres-migrations` log: successful jobs print the non-sensitive ledger, while `DRIFT`, `BLOCKED`, `failed`, or `applying` migration output is a rollout blocker.

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

Before producing release binaries or container images, generate the isolated upstream module graph. For artifacts that expect `BITRIVER_LIVE_STORAGE_DRIVER=postgres`, run both the pre-build pgx guard and post-build metadata inspection:

```bash
go run ./cmd/tools/production-module --output go.production.mod
go mod download -modfile=go.production.mod all
GOFLAGS="-modfile=$PWD/go.production.mod" ./scripts/check-postgres-pgx.sh postgres
go build -modfile=go.production.mod -tags postgres -o bitriver-live ./cmd/server
go run ./cmd/tools/verify-production-binary --require-module github.com/jackc/pgx/v5 bitriver-live
```

The generator fails closed on local replacements outside `third_party` and removes all checked-in offline mirrors from the production graph. The artifact inspector rejects any linked local replacement and proves pgx is present when required. Release workflows and Dockerfiles perform the same checks automatically; do not publish an artifact built directly from the offline root module graph.

### Legal publication checks

Before cutting a release candidate:

- Confirm policy documents are published and versioned: `docs/legal/terms.md`, `docs/legal/privacy.md`, `docs/legal/dmca.md`, and `docs/legal/age-policy.md`.
- Verify the current policy version/date is exposed in your public release notes or site footer.
- Verify DMCA contact metadata (designated agent name, email, and mailing address) is present in published operator docs and matches the values used in support workflows.
- Run a dry-run DMCA intake (`POST /api/legal/dmca`) and admin triage flow in staging.

### Backup and restore release gates

Before tagging production releases, prove backup/restore readiness:

- Confirm at least one **successful backup in the last 24 hours** (from scheduler logs, object storage object timestamp, or job history).
- Confirm the latest backup is a complete three-file set: archive, `bitriver.postgres-backup/v1` manifest, and `.sha256` file covering both exactly. The manifest must identify the exact candidate release and full commit; `unknown` is not release evidence.
- Provide **restore rehearsal evidence within the last 30 days** using `./scripts/restore-postgres.sh` (or equivalent staged workflow). Require the expected release and migration fingerprint, then retain the passing `bitriver.postgres-restore-report/v1` report with matched migration/row-count invariants and observed RPO/RTO.
- Confirm a complete encrypted `bitriver.host-backup/v1` set protects the verified Postgres trio, packaged configuration/secrets/generated config, and local API/transcoder/media data. When external object storage is enabled, require a matching aggregate inventory plus provider restore/versioning evidence.
- Retain a passing `bitriver.disaster-recovery/v1` report from the exact published package. It must prove source-host destruction, fresh host/package installation, fresh-database restore, object invariants, the production golden path, and observed RPO/RTO within the targets in `docs/operations.md`.
- If the release includes schema-heavy changes, run an extra restore rehearsal after migrations are validated in staging.

Keep this evidence attached to the release ticket/change request before maintenance begins. Run the local source-free
foundation with `BITRIVER_DISASTER_RECOVERY_ARTIFACT_DIR=.artifacts/disaster-recovery ./scripts/test-disaster-recovery.sh`,
but do not substitute that staged-bundle result for the required exact published-package and recovered-stack product proof.
The full encrypted workflow and immutable-input boundary are described in
[`docs/operations.md`](operations.md#encrypted-packaged-host-recovery-set).

### Stateful upgrade and rollback release gates

Run the automated data-plane foundation before an exact-image staging upgrade:

```bash
BITRIVER_UPGRADE_REPORT_PATH=.artifacts/stateful-upgrade-report.json \
  ./scripts/test-stateful-upgrade.sh
```

The current `bitriver.stateful-upgrade-report/v1` proof is bound to immutable
RC19 and RC20 release-set hashes and classifies that exact hop as in-place
compatible only for Postgres and the migration layer because their migration
tree and runner are byte-identical. It also proves a manifest-bound pre-upgrade
backup, actual-schema representative state, ambiguous-ledger refusal, exact
post-upgrade/in-place-rollback invariants, and verified fresh-database restore.

Then run the exact-image Compose rehearsal on a Docker host with network access:

```bash
BITRIVER_COMPOSE_UPGRADE_ARTIFACT_DIR=.artifacts/stateful-compose-upgrade \
  ./scripts/test-stateful-compose-upgrade.sh
```

The `bitriver.stateful-compose-upgrade-report/v1` evidence binds clean RC19 and
RC20 trees to their public release sets, asserts all five first-party image
references plus the changed Postgres digest, verifies the backup and preflight,
tests the migration/config-before-application interruption cut, runs the RC20
production golden path, and restores exact source images/generated configuration
without losing source or candidate-created state.

These reports remain necessary but not sufficient for stable promotion. RC19 is
a rejected source release, so its observed source/rollback health does not make
it an approved rollback root. Prove the same healthy rollback with an approved
prior release on the clean-host package path, including reboot evidence.
Reclassify rollback for every migration-changing candidate; never reuse
RC19-to-RC20's in-place decision for a different schema pair.

## 2. Tag one immutable candidate and trigger the build workflow

1. Ensure `CHANGELOG.md` (when present) and version references are up to date.
2. Create one annotated tag that follows
   `vMAJOR.MINOR.PATCH-PRERELEASE`. Increment the numbered candidate for each
   immutable RC attempt:
   ```bash
   release_tag=v1.2.3-rc.N
   git tag -a "$release_tag" -m "Release $release_tag"
   git push origin "$release_tag"
   ```
   Tags are immutable publication inputs. If a candidate fails, fix the source
   and increment the suffix; never force-move or overwrite an existing tag.
3. The push triggers [`.github/workflows/release.yml`](../.github/workflows/release.yml),
   which builds the Go binaries for every platform, packages the viewer
   bundle, publishes version-matched first-party container images (including
   the OME config helper), builds amd64/arm64 launcher archives plus `.deb`/`.rpm`
   packages, signs each launcher binary into a Cosign `.sigstore.json` bundle,
   signs all five exact image digests, and publishes the artifacts to a GitHub
   prerelease. Stable tags do not match this workflow and candidates never move
   `latest`. Monitor the workflow until every job completes successfully.

The release job is blocked on package acceptance that installs and removes the
amd64 package in Ubuntu 24.04, Debian 12, and Rocky Linux 9 containers. This is
package-structure evidence, not a substitute for the clean Ubuntu VM and reboot
rehearsal described below.

The release workflow's `Build binaries` step must compile Postgres-aware
targets with `-tags postgres` (`cmd/server`,
`cmd/tools/bootstrap-admin`, and `cmd/tools/migrate-json-to-postgres`) so the
published binaries include the real pgx-backed repository implementation. The
same step also runs a Linux `amd64` smoke check that starts `bitriver-live`
with Postgres storage flags and fails if output contains `pgx driver stubbed in
this build`, preventing future workflow edits from regressing to stubbed
storage builds.

### Job-local release validation credentials

The release workflow does not require repository `BITRIVER_*` secrets. Its
`verify-env` job derives strict version/package metadata from the tag, copies
[`deploy/.env.example`](../deploy/.env.example) into the runner temporary
directory, and replaces every sample credential with a strong job-local value.
It resolves current third-party image digests once into the sanitized
`release-dependencies.json` member of `release-contract-evidence`, validates the
production contract with `deploy/check-env.sh`, and enforces digest pins with
`scripts/require-image-digests.sh`. The downstream product gate validates and
reuses that exact reference/digest set instead of resolving the same mutable
tags again.

Prepublication first-party digest placeholders prove only contract formatting;
they are labeled that way in the redacted evidence. After the five tagged
images publish, `pull-only-product-gate` downloads the verified third-party
dependency evidence, logs out of GHCR, resolves the first-party images' real
anonymous manifests with bounded retries, pins both evidence sets into a new
temporary input, and runs the canonical production/pull stack plus the full
media/API golden path. GitHub Release creation cannot start until that job
passes.

Generated env and sentinel files remain under `$RUNNER_TEMP`, use restricted
permissions, are never uploaded or printed, and are deleted in `always()`
cleanup steps. Repository Actions configuration needs package write permission
through `GITHUB_TOKEN`, but it does not need real operator credentials.

The workflow retains only redacted status evidence from this validation. OME
freshness is checked separately by rendering `deploy/.env.example` to a
temporary output and comparing it with `deploy/ome/Server.generated.xml`.
Production credentials are rendered only on the deployment host; they are not
needed by build or test jobs and never enter release packages.

### Signed candidate release set

After the pulled-image product gate, the release job flattens the approved
payload, rejects duplicate or missing evidence, scans it, and generates
`release-set.json`. The manifest binds the candidate tag and commit, workflow
identity, every public artifact hash/size, five image digests with SBOM and
signature references, pinned third-party digests, and gate evidence. The job
keylessly signs that root, writes `CHECKSUMS.txt` over every other immutable
asset, re-verifies the complete set, and only then creates the prerelease.

The current public RC13 is the first candidate carrying this root. Its
`release-set.json` SHA-256 is
`795fffee84662aec91624eb4352b9c1a9ef5c34b17838939adaf567418797fa0`.
Verify that root, its exact tag workflow identity, and selected artifact/image
hashes using [`docs/release-promotion.md`](release-promotion.md); a tag or
`CHECKSUMS.txt` alone is not the complete current provenance contract.

### Record image digests for production

After the release images are published, resolve their digests and record them in
the release notes (and/or update `deploy/.env.example`) so production deployments
can pin to immutable references:

```bash
docker buildx imagetools inspect ghcr.io/prohibitedtv/bitriver-live:vX.Y.Z --format '{{.Manifest.Digest}}'
docker buildx imagetools inspect ghcr.io/prohibitedtv/bitriver-viewer:vX.Y.Z --format '{{.Manifest.Digest}}'
docker buildx imagetools inspect ghcr.io/prohibitedtv/bitriver-srs-controller:vX.Y.Z --format '{{.Manifest.Digest}}'
docker buildx imagetools inspect ghcr.io/prohibitedtv/bitriver-transcoder:vX.Y.Z --format '{{.Manifest.Digest}}'
docker buildx imagetools inspect ghcr.io/prohibitedtv/bitriver-ome-config:vX.Y.Z --format '{{.Manifest.Digest}}'
```

Capture any third-party image digests (`redis`, `postgres`, `ossrs/srs`,
`airensoft/ovenmediaengine`, `nginx`, and helper base images) you intend to pin
alongside the release so operators can mirror the same verified set in their
`.env` files.

For production Compose rollouts, keep `BITRIVER_DEPLOY_IMAGE_SOURCE=pull`.
Official release images are public, so anonymous manifest inspection and pulls
must work without `docker login ghcr.io`. A private fork or mirror may require
registry credentials. Pull-only mode enables preflight manifest checks and
avoids accidental source builds on production nodes.

### Clean-host artifact gate (required before announcing the release)

Start with the manual no-checkout
[`clean-host-qualification.yml`](../.github/workflows/clean-host-qualification.yml)
on protected `main`. Supply the exact prerelease tag and independently verified
`release-set.json` SHA-256. Retain its secret-scanned JSON report as the hosted
Ubuntu package/systemd/Docker/OME lifecycle portion of this gate; a passed
hosted runner is not the XOA/NPM/reboot result.

Use a newly provisioned Ubuntu Server 24.04 amd64 VM with no source checkout.
Download the `.deb` or Linux launcher archive and `CHECKSUMS.txt` from the
published tag, then follow [`docs/installing-on-ubuntu.md`](installing-on-ubuntu.md).
Record all of the following against the exact release tag and checksums:

1. Artifact-only install from a path containing spaces and a non-root Docker
   operator using explicit `sudo` lifecycle commands. Before installation,
   extract the downloaded package/archive and confirm all five first-party
   `BITRIVER_*_IMAGE_TAG` values equal the exact release tag.
2. Configuration/doctor/activation success with the systemd unit enabled.
3. OME config render and token consistency, healthy OME/config helper services,
   and an authenticated manager/control request. The unauthenticated root
   health probe is not sufficient evidence by itself.
4. Real RTMP ingest, public manifest/segment retrieval, and playback from a
   separate viewer through the intended Nginx Proxy Manager/NAT topology.
5. VM reboot followed by systemd, critical-service, authenticated OME, ingest,
   and playback recovery checks.
6. Same-tag rerun and upgrade staging preserve `/etc/bitriver-live` and
   `/var/lib/bitriver-live`; ordinary uninstall preserves them and explicit
   purge deletes only the documented paths.

Attach the OS/architecture, XOA VM shape, Docker and Compose versions, public
topology, timestamps, and redacted command results. Do not retain `.env`, OME
tokens, generated `Server.generated.xml`, private keys, or raw secret-bearing
logs. Ubuntu 24.04 amd64 is the only production installation claim until an
equivalent tagged-host evidence set passes for another platform.

### Promote the accepted candidate without rebuilding

Do not create or push a stable tag manually. Commit a reviewed promotion record
under `docs/releases/promotions/` that binds every required evidence URL and
SHA-256 to the candidate `release-set.json` SHA-256. All required gate issues
must also be closed. Then dispatch `.github/workflows/stable-promotion.yml` from
protected `main` with `operation=promote`, the candidate tag, matching stable
tag, and record path.

`Stable promotion gate` is read-only and runs before the `stable-promotion`
environment. It validates public GitHub asset digests, checksum coverage,
candidate/tag/commit identity, Cosign roots and five image signatures,
revocation state, tracked evidence, live issue state, rollback root, and
existing stable state. The environment-approved job revalidates state, creates
or resumes a draft release, retags exact digests, copies candidate files with
their RC filenames/package metadata intact, signs deterministic stable and
rollback metadata, verifies stored asset hashes, and publishes the draft.

Set `publish_latest=true` only as an explicit convenience-alias decision.
Production and rollback records always use a stable/candidate tag plus digest.
The first stable release correctly records no previous stable rollback set.
See [`docs/release-promotion.md`](release-promotion.md) for retry, revocation,
environment, and negative-enforcement details.

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
   The release workflow captures and scans this output before displaying a
   failure, and blocks when any OvenMediaEngine URLs, bind addresses, or ports
   point at loopback addresses, placeholders, or are missing.
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
4. For Ubuntu artifact installs, use `/etc/bitriver-live/bitriver.env` as the
   configuration source and run `sudo bitriver-host upgrade` from the new
   archive/package only after backup and validation. The generated OME and SRS
   files remain under `/etc/bitriver-live`; do not replace them with files from
   a source checkout.

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
