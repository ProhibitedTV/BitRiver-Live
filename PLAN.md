# PLAN

## Current scope - qualify viewer runtime updates (#1406) (2026-09-04)

- Qualify the focused Dependabot runtime group on the clean viewer-verification
  baseline: `hls.js 1.7.1`, `next 16.3.3`, and `ovenplayer 0.10.54`. Treat the
  Next.js patch as security-relevant because its release fixes unauthenticated
  RCE advisories; do not split it away from the already-generated exact lockfile.
- Preserve React, viewer tooling, temporary fixed overrides, shipped routes,
  player selection/fallback behavior, APIs, and the deployment contract. Align
  the checked runtime invariant and dependency-policy disposition with the new
  direct versions; do not absorb the separate tooling or ESLint major PRs.
- Cover the hls.js minor-version type narrowing and OvenPlayer caption/subtitle
  behavior through strict lint, the existing typed unit suite, production build,
  and Playwright integration. Require a clean install, exact resolved versions,
  zero-vulnerability audit, and focused runtime/policy checks before the full gate.
- Require literal `./scripts/verify.sh`, protected cross-platform Go/quickstart,
  viewer integration/build/audit, native arm64, production image scan, review,
  and squash merge. Update the preceding viewer task only with verified merged
  evidence, and keep the broader browser/device roadmap issue #1307 open.

## Current scope - clear viewer verification blockers (2026-09-04)

- Preserve the intended Jest unit boundary while making discovery portable in
  nested Windows worktrees: replace the absolute `<rootDir>` `testMatch` with
  the equivalent root-independent `**/__tests__` glob. Do not use Jest's broad
  defaults, which also collect the separate Playwright `tests/*.spec.ts` files.
- Remove newly published npm audit findings without suppressions. Pin current
  fixed `browserslist 4.28.9` and `postcss 8.5.28`, refresh the exact lockfile,
  align the PostCSS invariant, and update the dependency-policy disposition so
  maintainers know which temporary overrides remain release inputs.
- Keep shipped UI behavior, APIs, deployment contracts, CI workflows, and
  operator configuration unchanged. Record the Windows discovery result as a
  bounded #1307 finding, not closure of its browser/device compatibility scope.
- Require clean install, exact resolved graph, zero-high npm audit, strict lint,
  all 26 Jest suites without Playwright-spec collection, production build,
  focused baseline test, literal `./scripts/verify.sh`, protected cross-platform
  CI, viewer integration, native arm64, production image scan, and review before
  squash merge. Refresh #1406 only after this prerequisite is on `main`.

## Current scope - clear critical x/crypto release finding (#1306) (2026-09-03)

- Treat the blocking image-scan finding as a release-critical dependency update:
  production binaries currently resolve `golang.org/x/crypto v0.54.0`, and
  Trivy reports fixed critical `CVE-2026-56854` with `v0.55.0` as the patched
  version. Do not suppress or accept the finding for stable promotion.
- Update the canonical module requirement and checksums to `v0.55.0`. Preserve
  the checked-in offline replacement, which contains only the locally required
  `pbkdf2` implementation and does not vendor the affected SSH package; verify
  production-module generation removes the replacement and resolves the exact
  patched upstream graph.
- Keep this slice dependency-only: do not change application behavior,
  deployment contracts, CI workflows, scanner thresholds, or operator secrets.
  Record the security-relevant dependency change in the v1.2.3 draft notes.
- Verify checksum/module consistency, offline Go tests, production-module
  download/build identity, repository secret/diff hygiene, and literal
  `./scripts/verify.sh`. Require protected PR CI including the blocking image
  scan before squash merge, then refresh the blocked dependency PRs against the
  patched `main` rather than rerunning their stale vulnerable commits.

## Current scope - recovered immutable stack production golden path (#1299) (2026-08-22)

- Extend the merged published-package lost-host rehearsal with an explicit
  recovered-runtime phase. Start from the exact verified RC21 launcher and its
  encrypted restored host state, then boot the installed
  `/opt/bitriver-live` Compose bundle with pull-only release-set-pinned images;
  do not substitute a checkout deployment tree or rebuild product images.
- Reuse the existing `bitriver.production-golden-path/v1` client unchanged for
  the public product assertions: readiness and authenticated aggregate status,
  account/channel creation, deterministic 1080p RTMP, advancing and decodable
  OME/transcoder playback, offline transition, chat/moderation, VOD upload and
  playback, and final aggregate health.
- Preserve and bind the recovery result rather than running an unrelated clean
  stack. Restore the exact manifest/checksum-bound Postgres trio into a fresh
  database on the recovered Compose volume, retain the recovered encrypted
  configuration and local/external object fixtures, prove representative rows
  exist before the golden path, and prove those rows plus golden-path-created
  state remain afterward.
- Publish one secret-scanned completion report that binds the existing
  `bitriver.disaster-recovery/v1` report, exact release-set/package identity,
  observed runtime image references, restored-state invariants, and golden-path
  report SHA-256. Remove only the recovered-golden-path item from
  `remainingAcceptance`; keep scheduled/off-host freshness and RPO evidence
  explicit for the final #1299 slice.

### Execution and isolation

- Add a dedicated wrapper instead of changing the default local/staged recovery
  command. It must require the exact prerelease tag, full commit, and a fresh
  evidence directory, download and verify the public launcher first, and use
  only generated temporary host/config/database roots.
- Prepare a production-shaped private environment from the package's
  `deploy/.env.example` plus the exact release-set image/dependency digests
  before the source snapshot. The encrypted restore may change only installer-
  managed host path/UID/GID values; relocate those to the predicted final root
  and require every other byte to match. Private values stay in temporary
  sentinel files and may not enter retained reports or command output.
- Refuse canonical-container collisions before Docker mutation. Use a unique
  Compose project and project-owned Postgres/Redis/transcoder volumes, bounded
  service/HTTP waits, `--no-build --pull never` after exact pulls, and
  unconditional container/volume/temp-root cleanup on success, failure, or
  interruption.
- The repository-side harness may prepare fixtures, inspect Docker runtime
  identity, and drive the existing golden-path client. Recovery and runtime
  evidence must still identify the verified public package and its installed
  source-free bundle; repository code is not part of the recovered runtime.

### Risks and evidence boundaries

- The canonical Compose file has fixed container names, so parallel execution
  is unsafe even with a unique project. Fail closed when any canonical name is
  present and never stop or adopt an operator container.
- A fresh Compose Postgres container initially creates its configured database.
  Restore the verified trio into a different fresh database before application
  startup, switch the recovered environment to that database, recreate the
  Postgres container without deleting its project volume, then run canonical
  migrations and applications. Refuse an existing restore target or a database
  identity/hash mismatch before API startup.
- Automated review hardening must keep that recovered Postgres container
  unmodified. Do not install packages or copy backup/restore tooling into
  either exercised Postgres container, including the original fresh-database
  restore inside `test-disaster-recovery.sh`. Make the packaged backup and
  restore scripts compatible with the exact release-set Postgres image's
  `/bin/sh`, run both restore phases in disposable read-only helpers using the
  database containers' network namespaces and read-only script/input mounts,
  and bind both original and runtime restore-helper image references to the
  release-set identity in final evidence.
- Windows filesystem compatibility shims may emulate package-install symlinks,
  but they do not replace RC21's already-passed Ubuntu package/symlink/systemd
  evidence. This slice proves the recovered exact-image Compose product path;
  physical XOA/Nginx Proxy Manager/browser/reboot evidence remains #1297.
- Exact-head review hardening must also keep Linux path preparation valid before
  recovery creates the future `/etc/bitriver-live` subtree. Canonicalize the
  already-existing disposable recovered root, then append the managed relative
  config path; do not call `realpath` on nonexistent parent components.
- Do not change `deploy/docker-compose.yml`, root `.env`, generated OME
  expectations, CI workflows, the release workflow, or the default verification
  gate. Do not claim scheduled backup freshness, off-host durability, alert
  delivery, or a stable release from this result.

### Test and rollout plan

- Add focused tests for release-set-to-environment/runtime binding, original-
  disaster-to-runtime Postgres archive binding, completion-report schema/
  status/hash/invariant checks, exact original/runtime restore-helper image
  refusal, secret refusal, unrelated golden report refusal, and honest
  remaining acceptance.
- Run both Bash and the exact Postgres Alpine image's `/bin/sh` syntax/runtime
  paths for the backup/restore scripts, and assert the recovered-stack wrapper
  and its nested lost-host drill contain no package installation or
  in-container script injection. Add a source regression for Linux-safe future
  config-root construction.
- Run the existing staged and published lost-host rehearsals to guard backward
  compatibility. RC21's first recovered-stack exercise reached the real
  Postgres restore and Dockerized production golden path but is not promotable
  evidence because review found that it modified the runtime Postgres
  container. Merge and publish the portable-helper fix, then rerun the wrapper
  against the exact next public candidate; scan retained evidence and verify
  complete teardown.
- Run Python compilation, Bash syntax, pinned ShellCheck, Markdown links,
  `git diff --check`, and literal `./scripts/verify.sh`. Publish a focused PR,
  resolve automated review, require protected exact-head CI, squash merge, and
  post a bounded #1299 handoff before starting scheduled/off-host RPO work.

### Immutable candidate execution result

- PR #1401 merged the reviewed portable-helper implementation to protected
  `main` as `efca191c`. Annotated tag `v1.2.3-rc.22` points to that exact commit,
  and its complete release workflow passed before qualification.
- Independently downloaded `release-set.json` has SHA-256 `938b22cc...` and
  names the exact tag, commit, repository, five first-party images, and eight
  dependency digests. The no-checkout Ubuntu clean-host workflow passed against
  the same tag/hash.
- `test-recovered-stack-golden-path.sh` passed against the exact public RC22
  release set/package. Secret-scanned completion evidence binds both Postgres
  helper identities and all 13 observed services to the release set; cleanup
  left no rehearsal containers or volumes.
- #1299 now records the bounded release, clean-host, and recovered-stack result.
  Only production-like scheduled/off-host backup freshness and RPO evidence
  remains open; no stable-promotion or off-host-durability claim is inferred.

## Current scope - published-package lost-host recovery qualification (#1299) (2026-08-21)

- Extend the existing disposable lost-host rehearsal so it can consume an
  exact public Linux launcher archive instead of always staging release assets
  from the checkout. Bind the retained `bitriver.disaster-recovery/v1` report
  to the supplied `bitriver.release-set/v1` bytes, candidate tag, full source
  commit, package filename, byte length, and SHA-256.
- Execute the backup, encrypted host-state restore, Postgres fresh-database
  restore, external-object inventory, and recovered packaged-host install with
  the scripts extracted from that verified package. Keep the repository test
  driver and a fixture launcher binary outside the evidence claim so the drill
  remains portable on Windows while proving the shipped recovery-script bytes.
- Add a public-download wrapper for the Linux amd64 launcher package and
  release-set manifest. It must validate identity and package checksum before
  extraction, reject unsafe archive member paths, use a fresh temporary root,
  retain only sanitized reports plus the public manifest when requested, and
  clean all disposable hosts/containers on success, failure, or interruption.
- Preserve the current build-mode rehearsal as the default. Published mode is
  explicit and may claim only release-set/package binding plus recovery-script
  execution; Sigstore verification, Linux binary/package acceptance, the
  recovered production golden path, and scheduled off-host freshness remain
  separately owned evidence.

### Roadmap ordering and evidence boundary

- RC20 predates the packaged encrypted recovery payload. RC21 is the first
  immutable candidate from current `main` that can satisfy #1299's exact
  published-package recovery boundary; its release workflow must finish before
  the live published-mode drill can pass.
- This slice closes only the first of #1299's three recorded remaining items.
  Keep #1299 open for the production golden path on the recovered immutable
  stack and production-like scheduled/off-host RPO evidence. Feed the exact
  candidate identity forward into #1298 after those results exist.
- Do not change the canonical Compose/root-env/generated-OME contract, CI
  workflows, runtime services, release workflow, or default verification gate.

### Risks and boundaries

- A public manifest alone is not signature verification. Record the manifest
  SHA-256 and exact asset match while explicitly stating that Sigstore was not
  verified by this drill; reuse the clean-host/release verification evidence
  for that separate claim.
- Archive extraction is a pre-mutation trust boundary. Verify tag, commit,
  schema, unique asset entry, size, and SHA-256 before extraction, then refuse
  absolute or parent-traversing members before invoking `tar`.
- Review hardening must also bind the exercised `--bundle-root` bytes to the
  verified launcher archive subtree. The low-level published-mode driver
  accepts those paths independently; completeness alone cannot prevent a
  caller from pairing a valid archive with a modified extracted bundle.
  Compare a deterministic content inventory for every regular file and
  directory beneath `share/bitriver-live` before publishing the passing stage.
- The rehearsal intentionally destroys only a generated temporary source-host
  root. It must never accept the repository root, operator paths, or an
  existing target root, and it must scan retained evidence for test secrets.

### Test and rollout plan

- Add focused Python tests for valid public-package binding and schema,
  tag/commit/name/size/hash mismatch refusal. Exercise the report in both local
  and published modes and keep remaining-acceptance fields honest.
- Add shell regression coverage for exact script-root selection, identity
  overrides, pre-extraction verification, unsafe archive-member refusal,
  cleanup, and evidence retention. Run the existing local disaster drill.
- Once RC21 publishes, download its exact Linux amd64 launcher and release set,
  run the published disaster drill, scan retained evidence, and record the
  candidate/report hashes. Then run syntax/ShellCheck, focused tests, literal
  `./scripts/verify.sh`, protected PR CI, squash merge, and a bounded #1299
  handoff.

## Current scope - reusable single-host capacity qualification harness (#1303) (2026-08-21)

- Implement the target-host harness foundation requested by the refreshed
  roadmap: a versioned synthetic 1080p scenario, concurrent RTMP publishers,
  HLS virtual viewers, authenticated chat/API activity, protected metrics
  sampling, optional co-located Docker/host resource sampling, and a
  machine-readable `bitriver.capacity-report/v1` evidence artifact.
- Reuse the public-interface clients and synthetic media helpers from the
  production golden path. The harness must not read the application database,
  container filesystems, generated service configuration, or private operator
  state. Private accounts, sessions, and stream keys are per-run sentinels and
  may never appear in retained evidence.
- Separate scenario definition from execution. A checked-in RC qualification
  scenario defines warm-up, steady-state, spike, and bounded soak phases plus
  explicit host-protection stop conditions. A dry-run validates and hashes the
  exact scenario without contacting a stack; live execution requires the exact
  independently verified `release-set.json` bytes, validates their declared
  hash/tag/commit/five-image structure, and records every claimed collector
  boundary. It does not replace the release gate's Sigstore verification or
  infer that target containers match the release set.
- Sample API health, Prometheus text metrics, request latency/error rates,
  publisher state, viewer playlist/segment traffic, and load-client results at
  a bounded interval. When the runner is co-located with the Compose host,
  additionally collect host CPU/memory/disk and `docker stats`; otherwise mark
  host/container resource evidence unavailable rather than inferring it.
- Fail closed on invalid identities, unsafe workload sizes, unbounded phases,
  mismatched release-set bytes, missing protected metrics authorization,
  exceeded CPU/memory/disk/error thresholds, repeated health failures, early
  publisher exit, incomplete samples, undrained application work, or secret-
  bearing evidence. Always stop child publishers and load workers on success,
  failure, or interruption.
- Keep the wrapper portable across tracked-file mode differences: invoke the
  non-executable shared Python selector through Bash on host clients. Docker
  clients default loopback playback rewriting to `host.docker.internal` but
  must preserve an operator-supplied media-host override for remote targets.

### Roadmap ordering and evidence boundary

- #1297 remains the first production-acceptance issue, but the repository and
  issue history expose no physical XOA/Nginx Proxy Manager endpoint or access
  authority. Its remaining browser/media/firewall/reboot proof cannot be
  manufactured locally. #1303 is explicitly unblocked for reusable harness
  implementation against signed RC20 while the physical target is arranged.
- This slice establishes an executable qualification instrument and may run a
  bounded build-mode rehearsal. It does not publish a safe concurrency limit,
  correct host-sizing claims, close #1303, or stand in for target-host RC20
  measurements. Those require the exact supported physical host and its real
  topology, followed by reviewed raw results and conservative headroom.
- Do not change the canonical Compose/root-env/generated-OME contract, product
  runtime, monitoring deployment, CI workflows, or default verification gate.
  The harness is opt-in and targets an already-running canonical stack.

### Risks and boundaries

- Load generation can exhaust a developer or operator host. Enforce hard caps
  in the scenario parser, require stop conditions for every live scenario,
  ramp workloads by phase, and abort immediately when a configured protection
  threshold persists. The checked-in RC scenario stays intentionally small.
- Synthetic HLS fetchers measure HTTP/media delivery from the load client; they
  are not full browser decoders and do not prove WebRTC, playback UX, CDN, or
  browser compatibility. Those claims remain with #1307 and target-host media
  acceptance.
- API-level metrics do not expose host CPU, disk I/O, network, Postgres pool,
  Redis, or encoder internals by themselves. The report must distinguish
  direct host/Docker samples from application metrics and list uncollected
  requirements explicitly.
- Capacity fixtures persist normal test accounts/channels in the target stack.
  Require a dedicated qualification environment and state that cleanup is a
  disposable-environment responsibility; never target a live production data
  set by default.

### Test and rollout plan

- Add focused Python tests for scenario schema/identity validation, phase and
  stop-condition caps, Prometheus parsing, percent/resource parsing, latency
  summaries, stop-trigger behavior, deterministic scenario hashes, atomic
  sanitized reports, early-child cleanup, and secret refusal.
- Run the checked-in scenario in dry-run mode through both the host wrapper and
  Docker client image, then run a small disposable-stack rehearsal if local
  Docker resources permit. Scan every retained artifact for sentinels.
- Document the command, evidence schema, collector boundary, stop semantics,
  honest non-claims, and target-host follow-up in testing/operations guidance.
  Run Bash syntax/ShellCheck where available, Markdown links, focused tests,
  literal `./scripts/verify.sh`, protected PR CI, and squash merge.

## Current scope - viewer tooling dependency refresh (#1398) (2026-08-21)

- Reconcile the seven Dependabot development-tool updates with the repository's
  explicit viewer runtime baseline. The dependency graph moves
  `@types/node` from 26.1.2 to 26.2.0, so the invariant test must describe the
  reviewed package state instead of rejecting the intentional lockfile change.
- Keep the patch limited to development tooling, the baseline assertion, and
  concise workflow ledgers. Do not change viewer runtime dependencies,
  application behavior, deployment contracts, CI workflows, or release inputs.

### Risks and boundaries

- Type and lint package updates can expose compile, lint, test, accessibility,
  or production-build incompatibilities even when they do not ship in the
  runtime image. Validate the clean lockfile rather than accepting package
  metadata alone.
- Preserve the Node 24 and npm 11 engine contract. `@types/node` 26.2.0 remains
  a compile-time declaration update and must not be described as a Node 26
  runtime upgrade.

### Test and rollout plan

- Run the focused Go runtime-baseline test, clean `npm ci`, viewer lint/Jest,
  the production build, and the blocking production dependency audit.
- Run literal `./scripts/verify.sh --viewer`, staged diff/secret guards, and the
  protected PR matrix after updating the branch onto current `main`; squash
  merge only after every required check passes.

## Current scope - deterministic single-host service restart rehearsal (#1304) (2026-08-15)

- Add an opt-in, disposable rehearsal for the canonical single-host Compose
  stack that injects bounded stop/start failures into the API, Postgres, Redis,
  SRS/controller, OvenMediaEngine, transcoder, and viewer service paths. Record
  the expected public degradation signal for each dependency and the measured
  time until the stack returns to its documented ready state.
- Establish one authenticated account and channel before injection, then prove
  the session and durable channel state survive every recovery. Treat Redis
  queues and transient ingest work as explicitly ephemeral; do not overclaim
  in-flight delivery or media continuity from a control-plane restart check.
- Use `/readyz` for core dependency outages, authenticated `/api/status` for
  live ingest-component probes, and `/viewer` for the viewer boundary. Require
  bounded polling, no autonomous restart-count growth after recovery, and one
  machine-readable `bitriver.service-resilience/v1` report whose retained
  fields contain only release/source identity, component names, status classes,
  booleans, counts, and durations.
- Run from a clean tracked archive with a private copied environment, staged
  bind storage, and project-scoped database/cache volumes so the operator's
  working tree, local data, media, generated OME file, and root `.env` remain
  untouched. Fail closed when any
  canonical fixed-name container already exists; always tear down the isolated
  stack and evidence work directory unless retention is explicitly requested.
- Review hardening must pin `BITRIVER_CONFIG_ROOT` to the staged checkout root.
  A copied packaged/custom environment may otherwise retain an absolute
  operator config path and let the disposable SRS/OME renderers rewrite live
  generated configuration outside the rehearsal boundary.

### Risks and boundaries

- Do not change the canonical Compose/root-env/generated-OME contract,
  `scripts/verify.sh`, packages, CI, or runtime topology. This is a new opt-in
  qualification harness over the shipped contract, not a product behavior or
  deployment-workflow change.
- A local build proves restart and dependency-recovery mechanics only. Keep
  #1304 open for exact-candidate clean-host execution, physical daemon/host
  reboot, network partitions, disk/CPU/memory pressure, credentials/control-
  plane failures, active-stream continuity, and alert/delivery acceptance.
- Use randomly generated private test credentials only in the disposable work
  directory. Never print or retain them; scan the evidence against an exact
  sentinel set before declaring success.
- Bound every startup, degradation, and recovery wait. On interruption or
  failure, capture only sanitized diagnostics, return the failed stage, and
  remove containers, networks, volumes, copied environment, and credentials.

### Test and rollout plan

- Add focused helper tests for cookie persistence, readiness and ingest-status
  classification, durable-state checks, restart-count stabilization, report
  validation, timeout/failure reporting, secret rejection, and an absolute
  operator config-root override that resolves only to the staged tree.
- Run Bash syntax and pinned ShellCheck over the orchestrator, then execute the
  complete build-mode Compose rehearsal with real Postgres/Redis/media services
  and verify no canonical containers or test volumes remain afterward.
- Align resilience, production-release, testing, and draft release guidance
  with the report schema, command, proven boundaries, and remaining #1304
  acceptance; run Markdown links, diff/secret guards, and literal
  `./scripts/verify.sh` before a focused PR.

## Current scope - packaged-host disaster-recovery foundation (#1299) (2026-08-15)

- Extend the manifest-bound Postgres backup with one encrypted packaged-host
  recovery set for the default single-host profile. The set will contain the
  verified three-file Postgres backup, `/etc/bitriver-live`, and
  `/var/lib/bitriver-live` (API state plus transcoder/media data), while its
  public manifest retains only exact release identity, hashes, aggregate tree
  counts/fingerprints, encryption metadata, and timestamps.
- Use a passphrase file with OpenSSL AES-256-CBC plus PBKDF2 rather than placing
  secret material on argv, in environment variables, or in reports. Publish the
  encrypted archive, JSON manifest, and checksum atomically. Restore must verify
  the complete set and expected release before decrypting, refuse a non-fresh
  destination, validate every archive path, stream extraction without a
  plaintext archive on disk, and compare the recovered tree fingerprints.
- Ship the canonical Postgres and host recovery tools in every source-free
  launcher/package payload. Add an opt-in isolated rehearsal that creates
  representative database/config/media/object state, backs it up, removes the
  source deployment, restores into a fresh packaged-host-shaped root, restores
  Postgres into a fresh database, and emits one secret-scanned measured report.
- Reuse the production golden path after recovery rather than adding a parallel
  product smoke implementation. Bind the rehearsal to explicit release/commit
  identity and immutable image inputs; distinguish source-free bundle proof in
  this PR from the exact published-package run required after the next candidate
  is cut.

### Risks and boundaries

- Never read or modify the repository root `.env`, generated OME configuration,
  operator media, or unrelated deployment helpers. Test roots, encryption keys,
  Postgres databases, containers, volumes, reports, and decrypted state remain
  isolated and are removed by default.
- Do not change the canonical Compose/root-env/generated-OME contract, packages,
  CI workflows, or runtime topology. Adding recovery files to the existing
  source-free asset inventory changes package contents only; the installed
  runtime paths and service behavior remain unchanged.
- Require canonical packaged-host paths inside the encrypted archive and
  dereference compatibility symlinks during backup so a fresh installer can
  safely normalize them. Refuse traversal, absolute, device, or unexpected
  archive entries before extraction.
- Snapshot config, local data, the verified Postgres set, and object inventory
  into the private work directory before encryption; derive manifest
  fingerprints from that immutable snapshot so live writes cannot make a
  checksum-valid set whose restored tree disagrees with its manifest.
- Bind disaster evidence to the exact Postgres archive/manifest hashes and
  source identity carried by the host recovery report. Treat reported disaster
  RPO as the maximum of host-snapshot and database-backup age rather than
  allowing a recent host wrapper to hide stale system-of-record data.
- The default single-host proof covers local API/transcoder/media objects.
  External object-store durability remains an operator-managed replicated input;
  its non-secret inventory contract and consistency requirements must be carried
  in the recovery evidence without embedding credentials or object contents.
- This branch cannot prove its own future published package. Keep #1299 open
  until the merged recovery assets are published in an immutable candidate and
  the artifact-only clean-host recovery rehearsal plus production golden path
  pass against that exact release set.

### Test and rollout plan

- Add focused Python validation tests and shell integration cases for success,
  wrong release, checksum corruption, unsafe archive members, non-fresh target,
  wrong passphrase, atomic collision refusal, unrelated Postgres evidence,
  combined RPO, post-archive live mutation, root-importable unit execution, and
  secret-free reports.
- Verify the source-free asset inventory and packaged-host lifecycle expose the
  canonical recovery commands without a source checkout.
- Run the disposable Postgres/host-state rehearsal, production golden path when
  Docker resources permit, Bash/ShellCheck, docs/link/diff/secret checks, and
  literal `./scripts/verify.sh`.
- Publish through a focused high-risk PR and protected CI, post bounded evidence
  to #1299, then use the next immutable candidate for the final clean-host drill.

## Current scope - exact-image Compose upgrade and rollback rehearsal (#1298) (2026-08-15)

- Extend the merged database-layer foundation with a separate opt-in full-stack
  rehearsal for the same immutable `v1.2.3-rc.19` to `v1.2.3-rc.20` pair. Fetch
  the public `release-set.json` assets, require their recorded SHA-256 values,
  source commits, first-party image inventories, and exact third-party
  dependency inventories, then run only the recorded `tag@sha256` references.
  The pair retains the same eight dependency tags but upgrades the pinned
  Postgres 15 manifest digest; treat that as part of the stateful transition.
- Materialize clean source and candidate deployment trees from their recorded
  Git commits rather than the caller's working tree. Generate one rotated,
  private source environment from the published digest evidence, carry the
  same credentials/non-default settings forward, and change only release/image
  identity for the candidate. Use one isolated Compose project with persistent
  named database/Redis/transcoder volumes across source, candidate, and
  rollback trees.
- Start RC19 dependencies and API, record its observed aggregate-health state
  without treating it as source approval, and load the shared real-schema
  representative fixture into that deployment. Capture fixed-record and count
  invariants plus generated OME/SRS config hashes before changing services.
- Execute a documented stop/preflight/backup/upgrade sequence. At the
  post-migration/config but pre-application cut point, require the public health
  endpoint to remain unavailable so an interrupted deployment cannot look
  healthy. Start RC20, verify every first-party container uses the manifest
  digest, require exact source-fixture invariants, and run the existing
  production golden path against the upgraded stack.
- Roll back to the exact RC19 image/config set without removing persistent
  volumes. Require source image references, byte-identical generated config,
  preserved source fixtures and candidate-created row growth, successful
  readiness, and a reachable recorded aggregate-health response. Emit one
  secret-scanned JSON report and operator-readable Markdown summary.

### Risks and boundaries

- Never use or modify the operator's root `.env`, generated OME file, local
  media directory, or unrelated deployment helpers. All checkouts, secrets,
  release manifests, reports, volumes, networks, and containers are isolated
  and removed by default; retained evidence contains only hashes, counts,
  booleans, release identities, image digests, and durations.
- Do not edit the canonical Compose/root-env/generated-OME contract, packages,
  or CI workflows. The rehearsal consumes the exact tagged contract in clean
  temporary trees and remains opt-in because it pulls public release assets and
  images and runs the expensive media golden path.
- RC19 is a rejected source release. Its rollback phase may prove image/config/
  state mechanics and readiness, and the report records the observed aggregate
  status without requiring it to reproduce the earlier startup failure. It
  cannot be described as an approved production rollback target. #1298 remains
  open until a healthy approved previous-version rollback and clean-host
  package/reboot evidence exist.
- Use the merged manifest-bound Postgres backup before candidate mutation. This
  slice exercises the in-place rollback class; destructive-schema restore
  classification remains covered by the database rehearsal and must be rerun
  for any future migration-changing pair.

### Test and rollout plan

- Extract reusable fixture/invariant SQL from the data-plane rehearsal and keep
  its exact positive, restore, and interruption behavior green.
- Add the exact-release Compose orchestrator with release-set refusal cases,
  source/candidate/rollback image and config assertions, the interrupted cut
  point, upgraded golden-path evidence, sanitized reports, and guaranteed
  teardown.
- Run Bash/ShellCheck, helper/unit checks, both stateful rehearsals, docs/link/
  secret/diff checks, and literal `./scripts/verify.sh`. Publish through a
  focused high-risk PR and protected CI, then post bounded evidence to #1298.

## Current scope - stateful RC19 to RC20 upgrade/rollback data-plane rehearsal foundation (#1298) (2026-08-15)

- Use the immediate immutable published pair `v1.2.3-rc.19` at commit
  `1e14e3cf7d5f1d949b396d4f7897660575ea468e` and `v1.2.3-rc.20` at
  commit `9a8516a60c584c96a46b630b55c46df33f46fbdc`. Bind evidence to
  their public release-set SHA-256 identities
  `374a4084d1880abab1fa980d528a47bb5e324ed85541248438015fb13f2cc204`
  and `dd8eabcea7cf920a6f520e3e472cf44d3e1c7b0b7ad74945904f67ea74a47873`.
  RC19 is an upgrade-source fixture, not an approved release claim.
- The two tags have no migration-runner or migration-file diff. Classify this
  exact hop as in-place compatible at the database/migration layer only: apply
  the RC19 migration set, load representative state in the real schema, take a
  manifest-bound backup, execute the RC20 migration plan/apply path, and prove
  exact ledger plus row/value invariants before and after an in-place rollback
  decision.
- Exercise the restore-required decision separately by changing the upgraded
  state, restoring the verified RC19 backup into a fresh isolated database, and
  proving the original invariants. Demonstrate that an ambiguous `applying`
  ledger state blocks the candidate migration path rather than allowing a
  falsely successful upgrade.
- Emit one secret-safe JSON report with exact release identities, schema
  fingerprints, representative invariant results, rollback classification,
  blocked-interruption evidence, and observed backup/upgrade/rollback durations.

### Risks and boundaries

- Never log or retain DSNs, passwords, auth/MFA secrets, session tokens, stream
  keys, or row contents. The report may contain release/schema hashes, aggregate
  counts, boolean/value-hash matches, timings, and rollback decisions only.
- Use actual canonical migrations and table shapes rather than the simplified
  backup-script fixture. Keep all databases/backup artifacts disposable and
  isolated from the operator's root `.env`, generated OME, and runtime media.
- Do not change Compose, root env, generated OME, packages, or CI workflows.
  This foundation does not close #1298: exact-image Compose upgrade, config and
  image rollback, meaningful interrupted deployment cut points, ingest/playback,
  chat/admin/VOD APIs, and the full production golden path remain required.
- RC19 and RC20 have an identical schema, so no destructive-migration rollback
  claim is possible from this pair. Future schema-changing releases must rerun
  the rehearsal and may be classified restore-required or unsupported.

### Test and rollout plan

- Add a Docker-backed Postgres 15 rehearsal with actual migrations plus
  representative admin/creator/moderator/viewer, auth/MFA/session, profile,
  channel/follow/schedule, moderation/legal/chat, upload/recording/object, and
  non-default setting fixtures.
- Require the manifest-bound backup/restore scripts, migration planner/apply
  path, deterministic invariant snapshots, ambiguous-ledger refusal, JSON
  report schema/content, secret scan, and clean database/container teardown.
- Run shell/static checks, the focused rehearsal, docs/link/diff/secret checks,
  and literal `./scripts/verify.sh`. Publish as a focused PR through protected
  CI, then keep #1298 open for the exact-image full-stack phase.

## Current scope - manifest-bound Postgres backup/restore rehearsal foundation (#1299) (2026-08-15)

- Harden the existing `backup-postgres.sh` / `restore-postgres.sh` path instead
  of creating a parallel recovery implementation. Current backups are compressed
  and checksummed, but restore treats the checksum as optional, captures no
  release/schema/database/tool identity, checks only table presence, emits no
  machine-readable result, and has no automated corruption or compatibility
  regression.
- Produce an atomic gzip archive, mandatory checksum, and adjacent JSON manifest
  for each successful backup. Bind the manifest to operator-supplied release and
  commit identity, the database/server and pg tool versions, a deterministic
  applied-migration fingerprint, and exact row counts for every public base
  table. Refuse backup while the migration ledger is absent or non-applied.
- Before creating a rehearsal database, restore must require and verify the
  checksum/manifest, archive identity, manifest schema, and optional expected
  release/schema fingerprint. Restore only into a fresh isolated database,
  then compare the restored migration set and exact public-table row counts with
  the signed-by-checksum backup manifest.
- Emit a secret-safe JSON report with backup age (observed RPO), restore duration
  (observed RTO), compatibility/invariant results, and cleanup/retention state.
  This evidence is a release input, not proof of off-host storage or a complete
  lost-host recovery by itself.

### Risks and boundaries

- Never include passwords, DSNs, object credentials, host addresses, row data,
  or operator configuration contents in manifests/reports. Database/table names,
  release identity, schema checksums, counts, tool versions, timings, and result
  booleans are the only evidence fields in scope.
- Missing checksum or manifest becomes a hard restore refusal. Document that
  legacy dumps must be revalidated/repackaged outside production rather than
  silently restored; checksum or compatibility failure must occur before any
  rehearsal database is created or dropped.
- Keep database identifiers strictly validated before interpolation. Preserve
  cleanup traps, allow explicit retained rehearsal databases, and never restore
  over the source database.
- This slice covers Postgres only. Operator configuration/secrets/certificates,
  `/var/lib/bitriver-live`, local transcoder/media state, and object-storage
  versioning/replication remain required durable inputs; Redis stays explicitly
  ephemeral in the default contract. Do not claim #1299 complete until an
  artifact-only lost-host rebuild plus media/object invariants and the production
  golden path pass.
- Do not change Compose, root env, generated OME, package layout, CI workflows,
  or deployment topology in this slice.
- Treat a timestamped backup set as an owned transaction. A concurrent or
  same-second producer must refuse the collision without deleting an existing
  archive, manifest, or checksum; cleanup may remove final paths only after the
  current process has exclusively claimed that set.
- Preserve the advertised Helm scheduled-backup capability by sourcing its
  producer from the canonical `scripts/backup-postgres.sh`. Keep the generated
  chart copy under the existing Helm asset-sync check so the scheduler cannot
  emit the legacy two-file format that hardened restore intentionally refuses.
  This does not change the canonical Compose/root-env/generated-OME contract.

### Test and rollout plan

- Add a Docker-backed real-Postgres integration test with non-empty role/auth,
  channel, schedule, moderation/legal, chat-filter, upload/recording, and
  migration fixtures. Require backup, isolated restore, exact schema/row-count
  invariants, a measured JSON report, and cleanup.
- Add negative cases proving archive corruption, missing checksum/manifest,
  mismatched expected release, and mismatched schema fingerprint fail before
  the named rehearsal database exists.
- Add a deterministic same-second collision regression proving the first
  complete three-file set remains byte-identical after the second producer is
  refused. Render/check the Helm producer from the synchronized canonical
  script and require the chart to publish the same three-file restore contract.
- Run shell/static checks, focused integration, Markdown/link, diff/secret
  hygiene, and literal `./scripts/verify.sh`. Publish through a focused PR and
  protected CI; keep #1299 open for the remaining lost-host/media/object/golden
  path acceptance after this foundation lands.

## Current scope - prime aggregate ingest health after RC19 rejection (#1297, #1304) (2026-08-09)

- Treat immutable `v1.2.3-rc.19` as rejected clean-host evidence. Release run
  `31351022453` published 46 public assets from exact green main commit
  `1e14e3cf7d5f1d949b396d4f7897660575ea468e`; all 45 checksum entries match,
  the signed release-set SHA-256 is
  `374a4084d1880abab1fa980d528a47bb5e324ed85541248438015fb13f2cc204`,
  and its five first-party images are digest-bound. Do not patch RC19 or move
  its tag.
- Hosted no-checkout run `31351694175` verified the signed release set and
  package, installed on Ubuntu 24.04.4 x86_64, passed production preflight,
  activated the pull-only systemd stack, passed all seven CLI smoke checks,
  and reached authenticated OME control. It then failed the aggregate OME
  assertion because `/healthz` returned the startup snapshot
  `ingest: disabled` instead of probing the configured controller.
- Exact-image Docker Desktop reproduction confirms every ingest URL and
  credential is present. Both storage constructors seed a non-empty disabled
  snapshot with a current timestamp; `/healthz` intentionally trusts any
  recorded snapshot, so its documented live fallback is unreachable on first
  boot and the configured SRS/OME/transcoder controller is hidden indefinitely
  until another route explicitly refreshes it.
- Start repository health caches empty. The first `/healthz` request must then
  perform one bounded live probe and record the result; later requests retain
  the existing cache behavior. A genuinely disabled deployment still reports
  `ingest: disabled` through the no-op controller after that first probe.

### Risks and boundaries

- Do not make every `/healthz` request fan out to ingest services. Only the
  missing initial snapshot should trigger the existing live fallback; once
  recorded, the cache and `/api/status` refresh behavior remain unchanged.
- Coalesce callers that arrive while the initial probe is in flight so startup
  liveness and rollout smoke cannot duplicate downstream SRS/OME/transcoder
  requests or race to overwrite the first snapshot. Calls that begin after the
  initial snapshot is recorded must retain the existing explicit refresh path.
- Keep JSON and Postgres repository behavior aligned so development/test and
  the production installer cannot diverge. Do not change Compose, `.env`, OME
  configuration, authentication, public ports, or the release contract.
- Preserve the private root `.env`, tracked generated OME placeholder bytes,
  and the six operator-owned untracked paths. Exact-image diagnostic resources
  must be removed after evidence is captured.

### Test and rollout plan

- Extend the shared repository health scenario to require an empty/zero initial
  snapshot before the first controller probe, then require the first and later
  probes to populate/update the cache for both JSON and real Postgres backends.
  Add a blocked-controller concurrency regression proving overlapping initial
  calls share one probe and one recorded result for both repositories.
- Retain API handler coverage proving `/healthz` uses a valid cache and falls
  back to a live probe only when no snapshot exists. Update the operations
  contract to state that the first request primes the cache.
- Run focused storage/API tests, real Postgres-tagged tests, documentation/link,
  diff/secret hygiene, and literal `./scripts/verify.sh`. Rebuild the API in an
  isolated Docker Desktop project and require `/healthz` to report exactly one
  `ovenmediaengine: ok` entry before cleanup.
- Publish through a focused PR and protected CI, merge only when the aggregate
  gate is green, rerun exact-main CI, and cut only the next unused immutable
  candidate (`v1.2.3-rc.20`). Independently verify its public release set and
  rerun the complete no-checkout qualification through upgrade, restart
  recovery, retained uninstall, sanitized evidence, and cleanup.
- Even a hosted pass remains bounded evidence. Keep #1297/#1304 and stable epic
  #1293 open for the real XOA/NPM/firewall/host-reboot/media path and the other
  state, capacity, resilience, SLO, security, and browser gates.

### Qualification result and roadmap handoff (2026-08-15)

- PR #1391 merged without bypass as exact main commit
  `9a8516a60c584c96a46b630b55c46df33f46fbdc`; protected PR run
  `31911599915` and exact-main run `31911889892` passed every required gate.
- Immutable `v1.2.3-rc.20` release run `31912204601` passed and published 46
  public assets. Independent verification found 45 unique checksum entries,
  exact candidate commit/tag identity, complete payload coverage, and signed
  release-set SHA-256
  `dd8eabcea7cf920a6f520e3e472cf44d3e1c7b0b7ad74945904f67ea74a47873`.
- No-checkout Ubuntu 24.04 qualification run `31912782711` passed package
  installation, immutable preflight, pull-only activation, smoke and
  authenticated OME control, same-tag upgrade staging, OME/Docker/systemd
  restart recovery, retained-data uninstall, sanitized evidence, and cleanup.
- The ledger closeout reran literal `./scripts/verify.sh` successfully with the
  current public contract values in a disposable env copy. Real Postgres,
  Compose, rebuilt quickstart, endpoint health, and teardown passed; the private
  root `.env` was restored at SHA-256
  `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`,
  and generated OME bytes remained unchanged.
- This completes the hosted RC20 forward-fix slice. It does not close #1297,
  #1304, or #1293: physical XOA/Nginx Proxy Manager/firewall/host-reboot/media
  evidence and the remaining production gates are still required. The next
  primary recoverability slice is #1299; #1298 explicitly depends on its proven
  backup/restore path.

## Current scope - separate post-start smoke from pre-start port readiness after RC18 rejection (#1297, #1304) (2026-08-09)

- Treat immutable `v1.2.3-rc.18` as rejected clean-host evidence. Release run
  `31348429963` published 46 public assets from exact green main commit
  `4e496e389d2a606976a55a89cd08ef79762eed2f`; all 45 checksum entries match,
  the signed release-set SHA-256 is
  `2de0cb71af2af8d41d7a535d28478003987d6b180568a84bc5dc3eae680ef014`,
  and its five first-party images are digest-bound. Do not patch RC18 or move
  its tag.
- Hosted no-checkout run `31348986535` verified the public signature and
  package checksum, installed on Ubuntu 24.04.4 x86_64, staged immutable
  inputs, passed production preflight, and activated the pull-only systemd
  stack. It then failed `smoke-and-ome-control` because `bitriver smoke` calls
  the complete pre-start `doctor` with default paths after the stack is
  running; doctor correctly sees BitRiver's own 23 TCP/UDP listeners as port
  conflicts, so the post-start command can never advance to its Compose and
  endpoint checks on a normal installation.
- Preserve `bitriver doctor` and quickstart host-port refusal as pre-start
  safety gates. Narrow only the smoke prerequisite stage to the Docker CLI,
  daemon, and Compose availability/version checks it claims to perform; smoke
  must then use its caller-selected Compose/env files for running-stack state
  and endpoint checks. Do not add a public bypass flag or weaken preflight.

### Risks and boundaries

- Skipping all doctor logic would hide a stopped Docker daemon or unsupported
  Compose version. Reuse the existing binary/version check implementations and
  minimums while excluding pre-start-only port availability, host sizing, and
  bind-mount writability from the post-start phase.
- Keep `verify` ordering intact: doctor remains first, before Compose config and
  smoke. A standalone post-start smoke must not require ports to be free.
- Update the smoke command documentation to state the phase boundary. This is
  a CLI behavior correction, not a deployment-contract, Compose, env, or OME
  configuration change.
- Preserve the private root `.env`, tracked generated OME placeholder bytes,
  and the six operator-owned untracked paths.

### Test and rollout plan

- Add focused Go coverage proving the smoke prerequisite checks reject missing
  Docker/Compose but do not invoke host-port availability checks, and retain
  existing running-stack/endpoint failure coverage.
- Run focused `./cmd/bitriver` tests, documentation/link checks, diff/secret
  hygiene, and literal `./scripts/verify.sh`. Exercise the running-stack smoke
  against Docker Desktop so occupied BitRiver ports are accepted only after
  Compose state and health endpoints pass.
- Publish through a focused PR and protected CI, merge only when the aggregate
  gate is green, rerun exact-main CI, and cut only the next unused immutable
  candidate (`v1.2.3-rc.19`). Independently verify its public release set and
  rerun the complete no-checkout clean-host qualification through smoke,
  authenticated OME control, upgrade, restart recovery, retained uninstall,
  sanitized evidence, and cleanup.
- A hosted pass remains bounded evidence. Keep #1297/#1304 and stable epic
  #1293 open for the real XOA/NPM/firewall/host-reboot/media path and the other
  state, capacity, resilience, SLO, security, and browser gates.

## Current scope - align installed generated config after RC17 rejection (#1297, #1304) (2026-08-09)

- Treat immutable `v1.2.3-rc.17` as rejected clean-host evidence. Hosted run
  `31344257847` passed the no-checkout boundary, signed provenance, public
  package install, disabled-service staging, immutable production inputs,
  doctor, and preflight, then failed systemd activation because `ome-config`
  exited 1. Do not patch RC17, move its tag, or weaken activation.
- Align the managed Ubuntu configuration layout with the source-shaped path
  already used by the read-only Compose helpers: persist OME output at
  `/etc/bitriver-live/deploy/ome/Server.generated.xml` and SRS output at
  `/etc/bitriver-live/deploy/srs/conf/srs.generated.conf`, while the installed
  `/opt/bitriver-live/deploy/...` paths remain symlinks to those operator-owned
  files.
- Migrate the former flat `/etc/bitriver-live/Server.generated.xml` and
  `/etc/bitriver-live/srs.generated.conf` files without replacing operator
  content. Move a sole legacy file into the canonical tree, retain bounded
  compatibility symlinks, accept an idempotent already-migrated layout, and
  reject divergent dual copies instead of guessing which one to discard.
- Keep the private environment at `/etc/bitriver-live/bitriver.env`, data under
  `/var/lib/bitriver-live`, and the existing `BITRIVER_CONFIG_ROOT` mount. This
  is an installed generated-config path correction, not a new deployment
  topology or another OME/SRS render path.

### Root cause, risks, and boundaries

- PR #1381 correctly moved helper writes off the read-only `/workspace` mount
  and into `/etc/bitriver-live/deploy/...`. Source checkouts passed because
  those directories already exist. The package installer still stored the two
  generated files flat under `/etc/bitriver-live`; SRS happened to create its
  missing parent directories, while the OME Go renderer uses `os.WriteFile`
  and failed on the absent parent. Creating the OME directory alone would be
  unsafe because the OME consumer would still read the old flat file.
- Protected PR run `31346223822` exposed a second contract edge after the path
  fix: SRS cannot read an operator-owned mode-`0600` generated config while it
  runs with all Linux capabilities dropped. The tracked OME placeholder masked
  the equivalent fresh-install risk because an existing file keeps its prior
  mode when `os.WriteFile` truncates it. Preserve `bitriver.env` as owner-only,
  but make the two generated runtime configs mode `0640`, owned by the selected
  Docker operator UID/GID, and add only that GID as a supplementary group for
  the read-only OME/SRS consumers. Do not add capabilities or make either
  runtime mount writable. Both renderers must enforce the final mode on every
  render, including existing files.
- Corrected run `31346863473` proved the runtime group-read design in the base
  Compose file but exposed a Linux smoke-fixture mismatch: its override assigns
  the runner UID/GID to the config renderers without persisting or exporting
  `BITRIVER_HOST_GID`, so the base SRS/OME supplementary group resolves to `0`.
  Keep the production/package contract unchanged and add the fixture's own
  `host_gid` to only its SRS/OME services. Do not export host identity globally,
  because this smoke intentionally retains the transcoder image UID on its
  isolated named volume.
- Third run `31347182741` passed the complete product matrix after that fixture
  alignment, including Ubuntu test-all and cross-platform Go/entrypoint checks.
  Its merge gate failed only because PR #1386's prose risk section did not use
  the repository's machine-checked scorecard headings/checkboxes. Preserve the
  high-risk classification and evidence, update the PR body to the exact
  template shape, and trigger one fresh synchronize run before merging.
- Upgrade migration must be fail-safe under interruption and must not erase a
  modified operator config. Use same-filesystem moves where possible, create
  parents before migration, apply the established `0600` file and operator
  ownership boundary, and make reruns deterministic.
- Keep `/etc/bitriver-live` mode-restricted; do not loosen credentials, grant
  capabilities, make the helper workspace writable, or retain raw renderer
  logs in qualification evidence.
- Update `docs/contract.md` and operator/install guidance in the same change.
  Preserve the private root `.env` and the six operator-owned untracked paths.

### Test and rollout plan

- Extend the Linux installer lifecycle for a fresh structured layout, migration
  from legacy flat files, compatibility symlinks, byte preservation, exact
  ownership/modes, idempotent reinstall, and divergent-copy refusal.
- Add static release/Compose contracts that the helper outputs, installed
  symlinks, and documented paths all name the same source-shaped files. Run an
  exact package-style Docker fixture using arbitrary non-root UID/GID and prove
  SRS render, OME render, token verification, and consumer-visible file bytes.
- Exercise actual SRS/OME consumer startup under dropped capabilities so the
  renderer mode, operator group, and runtime supplementary-group contract is
  proven rather than inferred from a renderer-only fixture.
- Run focused installer/Go/shell/docs checks, literal `./scripts/verify.sh`,
  protected PR CI, and exact-main CI. Publish only the next unused immutable
  candidate, `v1.2.3-rc.18`, from that green main commit; independently verify
  its public signed release set and rerun the no-checkout qualification.
- A hosted pass remains bounded evidence. Keep #1297/#1304 open until the real
  XOA/NPM/firewall/reboot/media requirements assigned to those issues pass.

## Current scope - define and reconcile the live-room chat target (#1272) (2026-08-09)

- Add one product-source-of-truth specification under `docs/` that turns the
  ivlog-inspired direction into an adapted BitRiver Live target state. Cover
  desktop and mobile layout, dense message anatomy, the room roster, roles and
  badges, system and moderation rows, accessibility, busy-room performance,
  and an MVP-versus-later boundary without copying ivlog.tv assets, branding,
  CSS, or exact visual treatment.
- Audit each visible feature against the current `/api/chat/ws` protocol, Go
  gateway, viewer chat panel, and responsive channel page. Mark each item as
  shipped, protocol-only/partially wired, or requiring a backend/viewer change;
  do not treat closed implementation issues as proof when the production call
  path is absent.
- Link the completed foundation issues #1273, #1274, and #1275, then create
  focused follow-up issues for concrete gaps discovered by the audit. Keep one
  canonical chat stack: existing business and transport behavior stays in the
  Go service/chat/API boundaries, and the Next.js viewer consumes that protocol.
- Link the specification from the contributor-facing architecture/UI docs so a
  future implementation PR can find the target state without chat history.

### Findings, risks, and boundaries

- The viewer already ships the video-first desktop dock and mobile-below-video
  layout, dense bounded transcript, follow-versus-read scroll behavior,
  loading/retry/empty/auth states, a scoped accessible live log, reports,
  moderator row actions, and supported slash-command parsing.
- The Go gateway already emits de-duplicated `presence_snapshot`,
  `presence_join`, and `presence_leave` events and carries normalized role and
  badge metadata. The viewer does not currently consume these presence events;
  its visible roster is inferred from recent messages, and badge payloads are
  parsed but not rendered as real badge elements.
- `BroadcastSystemEvent` and the `system` envelope exist and the viewer can
  render live system rows, but no production service currently calls the
  broadcaster for stream lifecycle or pinned-room notices. `/me`, persistent
  message removal/room clearing, slow mode, and title/announcement management
  remain unsupported protocol/product decisions and must not be implied as MVP.
- This slice is documentation and issue decomposition only. It does not change
  the deployment contract, chat wire format, authorization rules, persistence,
  or user-supplied content rendering. The private root `.env` and the six
  operator-owned untracked paths remain untouched.

### Test and rollout plan

- Validate every shipped/partial/missing claim against named source files and
  existing tests, then run the repository Markdown-link checker and
  `git diff --check`.
- Run the focused Go chat tests and viewer chat-panel tests because the spec is
  a product contract derived from those behaviors; run literal
  `./scripts/verify.sh` before publication as required by repository policy.
- Publish through a focused protected PR. Merge only after required checks are
  green, verify exact-main CI, and close #1272 only when the checked-in spec and
  linked follow-up issue set satisfy every epic acceptance criterion.

### Completion evidence

- Added `docs/live-room-chat.md` with the adapted-not-copied boundary, all
  eleven requested product areas, a source-backed feature/protocol matrix,
  MVP/later scope, and the single-stack architecture contract. Filed remaining
  MVP #1382, #1383, and #1384 after auditing closed foundation #1273-#1275.
- Focused chat/viewer/docs checks and literal repository verification passed.
  PR #1385 passed protected run `31344956999`, squash-merged as exact main
  `8e9f46f146e7e065cb097a53103a5317fd77230e`, and exact-main run
  `31345003256` passed. #1272 closed as completed and the repository returned
  to zero open pull requests.

## Current scope - artifact-only Ubuntu host qualification (#1297, #1304) (2026-08-03)

- Add a manual, least-privilege Ubuntu 24.04 amd64 qualification workflow that
  performs no repository checkout. It must download one public prerelease's
  signed release set, exact `.deb`, checksums, and five image signature bundles;
  verify their exact tag/workflow identity; and use only those published bytes
  for installation and runtime acceptance.
- Install the package on the clean hosted runner, stage the disabled systemd
  service for a non-root Docker operator, derive immutable image pins from the
  signed manifest, validate the production environment, and activate the
  canonical pull-only Compose stack. Exercise `doctor`, `smoke`, authenticated
  OME manager access, same-tag upgrade, OME restart, Docker daemon restart,
  full systemd restart, status, ordinary uninstall, package removal, and
  config/data retention.
- Emit a sanitized `bitriver.clean-host-qualification/v1` JSON report even on
  failure. Bind it to candidate tag, source commit, release-set SHA-256,
  package checksum, exact image digests, OS/architecture, tool versions,
  workflow run, and per-stage outcomes without retaining credentials, raw env,
  generated media credentials, OME XML, or broad logs.
- Refresh first-install documentation from superseded RC12 wording to the
  public signed RC13 release set. Keep Ubuntu 24.04 amd64 as the sole production
  installation target and keep Debian/RPM/arm64, real XOA reboot, Nginx Proxy
  Manager browser/media routing, firewall/NAT, and target-host ingest/playback
  explicitly provisional.

### Risks and boundaries

- A GitHub-hosted Ubuntu VM can prove a clean package/systemd/Docker lifecycle,
  but it cannot prove the user's XCP-ng/XOA reboot path, router/firewall rules,
  or Nginx Proxy Manager TLS/WebSocket/media topology. #1297 and #1304 remain
  open until those external checks are attached to the same RC13 manifest hash.
- Never trust the candidate tag alone. Require the operator-supplied release-
  set hash, verify the root with checksum-verified Cosign against the exact
  release workflow/tag identity, verify all five exact image bundles, and
  populate Compose digests from that signed root before Docker pulls.
- The qualification job runs with generated ephemeral credentials. It must not
  print or upload `/etc/bitriver-live/bitriver.env`, generated OME/SRS config,
  container logs, stream keys, or credentials. Diagnostics are restricted to
  safe versions, systemd state, Compose status, and explicit stage outcomes.
- Preserve the canonical Compose/env/generated OME contract, the private root
  `.env`, and the six operator-owned untracked paths. This slice adds a consumer
  of the release contract; it does not create a second deployment topology.

### Test and rollout plan

- Add focused workflow contract tests for manual-only triggering, no checkout,
  least privilege, exact release-set/image signature verification, release-
  only package install, non-root systemd lifecycle, bounded OME/Docker/systemd
  recovery, retention-safe uninstall, always-produced sanitized evidence, and
  explicit external-gate boundaries.
- Parse workflow YAML, run the focused Go contract test, installer-language and
  Markdown-link checks, committed-secret guard, `git diff --check`, and literal
  `./scripts/verify.sh` with the private environment restored byte-for-byte.
- Merge only through the protected aggregate gate and require exact-main CI.
  Then dispatch the workflow for `v1.2.3-rc.13` and release-set SHA-256
  `795fffee84662aec91624eb4352b9c1a9ef5c34b17838939adaf567418797fa0`.
- If the live run passes, download and independently inspect its report, commit
  the durable redacted evidence in a follow-up, and attach it to #1297/#1304.
  Do not close either issue or enable stable promotion until the XOA/NPM/reboot
  and remaining production gates genuinely pass.

### Live rollout correction

- The first post-merge dispatch was rejected by GitHub before a run existed:
  `runner.temp` is not an available expression context in job-level `env`.
  Move qualification path construction into the first Bash step using the
  runner-provided `$RUNNER_TEMP`, append only those non-secret paths to
  `$GITHUB_ENV`, and keep every later step on the same isolated directories.
- Add a contract assertion that the workflow never uses `${{ runner.* }}` in
  job-level configuration and requires `$RUNNER_TEMP` initialization before
  any path is consumed. Rerun focused/YAML/policy checks and the literal
  verifier, then land the correction through PR and exact-main gates before
  redispatching RC13. The rejected dispatch is parser evidence only and must
  not be reported as a clean-host execution attempt.
- Run `30822618916` proved the parser correction and no-checkout boundary, then
  stopped before package installation because exact Cosign 3.1.2 supports
  `--bundle` for `verify-blob` but not container-image `verify`. Keep bundled
  root verification unchanged; checksum each downloaded image bundle against
  its signed image entry, signed artifact entry, and `CHECKSUMS.txt`, then use
  Cosign's supported anonymous registry-backed `verify` command for each exact
  immutable image digest and release-workflow identity. The five public RC13
  bundle hashes match all three signed/checksum sources, and the exact Cosign
  binary verifies all five remote digests anonymously with that command.
- Extend the workflow contract so image bundle byte verification is mandatory
  and container verification rejects the unsupported `--bundle` flag. Run the
  focused/YAML/policy and literal verifier gates, land through a new protected
  PR and exact-main run, and redispatch the same immutable RC13 inputs. Treat
  `30822618916` as a provenance-stage compatibility failure only; its sanitized
  artifact may be inspected, but it is not clean-host lifecycle evidence.
- Run `30824712866` then proved signed provenance, public package installation,
  and disabled systemd staging. Env validation and `bitriver-host doctor`
  passed, but the digest-render assertion omitted disabled Compose profiles and
  therefore could not see the signed `alpine:3`/`alpine:3.19` references used
  only by the optional `postgres-host` and `srs-api` services. Render the audit
  with `docker compose --profile '*' ... config --images` so all canonical
  services are checked without enabling those profiles during activation.
- Add a contract assertion for all-profile digest rendering, repeat focused and
  literal gates, protected PR/main validation, and redispatch exact RC13. Treat
  `30824712866` as package/preflight evidence only, not activation or lifecycle
  evidence; its failure artifact and successful cleanup remain diagnostic.
- Run `30827018880` passed the no-checkout/provenance boundary, public `.deb`
  installation, disabled-service staging, immutable environment validation,
  doctor, and the complete all-profile digest audit. Activation then exposed a
  real immutable RC13 package defect: the rendered systemd unit retained quotes
  around `WorkingDirectory=/opt/bitriver-live`, which systemd rejects as a
  non-absolute path. Do not patch the installed candidate in qualification or
  weaken the activation gate; RC13 is rejected for clean-host qualification.
- Fix the canonical packaged Compose unit and add rendered-unit regression
  assertions at the installer boundary. Run focused installer/unit checks plus
  the literal repository verifier, then land the correction through a protected
  PR and exact-main CI. Publish the next unused immutable candidate, RC14, only
  from that exact green main commit and rerun the artifact-only qualification
  against RC14's signed release-set hash. RC13 and run `30827018880` remain
  bounded failure evidence; stable promotion and #1297/#1304 remain open.
- The post-rebase Linux lifecycle exposed that the canonical `.service`
  template is stored with CRLF because the repository attributes cover shell
  scripts but not systemd units. Enforce LF for this exact packaged unit and
  normalize its tracked bytes; keep the strict rendered-directive regression
  so Windows checkouts cannot mask release-byte drift. Repeat the Linux
  lifecycle, focused policy checks, and fresh PR CI before merge.
- PR #1378 passed protected run `31333807382`, squash-merged as exact main
  `9f97c1533613fd9a1cb40353c2df1d159c51a2aa`, and exact-main run
  `31334103956` passed. Immutable `v1.2.3-rc.14` was published from that commit;
  release run `31334396479` passed all 46 public assets and the independently
  verified signed-root SHA-256 is
  `ae27e14f4e3883e216c57c145e73d8571838e8b80098afd7145cc6ed9f5923f3`.
- Clean-host run `31334880405` proved RC14 provenance, public `.deb` install,
  disabled systemd staging, production environment validation, doctor, and
  the complete immutable-image audit. The corrected systemd unit parsed and
  invoked the package launcher, but activation stopped because both
  `srs-config` and `ome-config` exited 1. RC14 remains immutable and rejected
  for clean-host qualification; do not patch its installed bytes or move its
  tag.
- The package intentionally keeps `.env`, OME output, and SRS output under
  `/etc/bitriver-live` and exposes them in `/opt/bitriver-live` as absolute
  symlinks. The renderer containers mount `/opt/bitriver-live` at `/workspace`,
  so `/workspace/.env` and both generated paths resolve to container-local
  `/etc/bitriver-live`, which is currently absent. This explains the paired
  renderer failure before SRS/OME consumers can start.
- Add one Compose contract value, `BITRIVER_CONFIG_ROOT`, defaulting to the
  source checkout root. Mount it at `/etc/bitriver-live` only in `srs-config`,
  `ome-config`, and `ome-health-token-check`; the two renderers receive a
  writable bind while the verifier receives it read-only. The packaged
  installer must idempotently set the value to its absolute operator config
  directory, while source checkouts retain the relative default and existing
  generated-file consumer mounts.
- Extend installer and Compose contract tests to require the managed package
  value, exact renderer/verifier mounts, read-only verification, source-default
  rendering, and no credential output. Re-run the Linux package lifecycle,
  focused Go/shell/contract checks, isolated Docker Compose rendering, and the
  literal verifier while preserving the private root `.env` and six unrelated
  operator paths.
- Automated PR review identified that the Unix launcher initially derived the
  config root before `ensure_assets` created a custom env file's missing parent
  directory. Move derivation immediately after env seeding and lock that order
  in the wrapper regression so first-run custom paths remain supported.
- PR #1379 passed refreshed protected run `31336273494`, squash-merged as exact
  main `91525a32f43d202561a9cc12d9d5452f48813f16`, and exact-main run
  `31336615980` passed. Immutable `v1.2.3-rc.15` published from that commit in
  passed release run `31336980513`. Independent public verification accepted
  all 46 assets, the release-set signature, and all five exact image signatures;
  the signed-root SHA-256 is
  `69acc96dae732fe9e6eea6f0a8953f57bd3796a5ddc373fec81872a81c9abb2e`
  and the amd64 `.deb` SHA-256 is
  `d54347660312ded8266387efa65a6abe543ac9ab3cd52532b6fdcf64942e4202`.
- Clean-host run `31337618217` advanced RC15 through signed provenance,
  package installation, disabled service staging, immutable production input
  configuration, environment validation, and doctor. Systemd activation still
  stopped because `srs-config` and `ome-config` both exited 1; sanitized
  evidence upload and cleanup passed. RC15 remains immutable and rejected.
- An exact downloaded-package reproduction proves the installed symlinks,
  renderer commands, and `/etc/bitriver-live` mount work when the correct source
  is selected. The remaining durable defect is that `bitriver env init` seeds
  the source-checkout value `BITRIVER_CONFIG_ROOT=..` into the installed
  `/etc/bitriver-live/bitriver.env`; only the systemd process and launcher
  attempted to override it. Persist the absolute installer config directory in
  that managed env file on install/upgrade, without printing or replacing other
  operator values. Append the key for older package env files that predate it,
  replace the single source default, and reject ambiguous duplicates.
- Extend the installer lifecycle regression to require exactly one persisted
  absolute config-root entry after first install and idempotent reinstall, while
  retaining unrelated operator values. Repeat the exact installed-package
  renderer proof, focused tests, literal verifier, protected PR/exact-main CI,
  next unused immutable candidate, and no-checkout qualification.
- Land the correction through a focused protected PR and exact-main CI. Publish
  only the next unused immutable candidate from that green commit, independently
  verify its signed public release set, and rerun the no-checkout qualification.
  A hosted pass still does not close #1297/#1304 or prove XOA reboot,
  Nginx Proxy Manager, firewall/NAT, browser playback, or target-host media.
- PR #1380 passed refreshed protected run `31338969678`, squash-merged as exact
  main `0a6be1600b08bcf2a4caeca6c580698f7e2f8ce4`, and passed exact-main run
  `31339266505`. Immutable `v1.2.3-rc.16` published from that commit in passed
  release run `31339591099`; all 46 public assets, the signed release root, and
  all five exact image signatures independently verify. The release-set
  SHA-256 is
  `c4d34bd82264995723a679b88e497c1a02aa192a99ac8bf3458de771b7b34b79`
  and the amd64 `.deb` SHA-256 is
  `a8e02419b1fbc51e2477030e32cca69145aa25d3be259dca513c3f77cdd5363a`.
- Clean-host run `31340245262` proved RC16 signed provenance, public package
  installation, disabled systemd staging, persisted config-root resolution,
  production environment validation, doctor, and the all-profile immutable
  image audit. Activation still stopped when both config renderers exited 1;
  sanitized evidence upload and cleanup passed. RC16 remains immutable and
  rejected for clean-host qualification.
- The remaining paired failure is a Unix ownership boundary. The installer
  correctly keeps `/etc/bitriver-live` and durable bind data mode-restricted and
  owned by the non-root Docker operator, but Compose forces the renderer jobs,
  API, and transcoder to unrelated image UIDs. With all capabilities dropped,
  the renderer jobs cannot traverse the private config directory; the API and
  transcoder would likewise be unable to write their durable package paths.
- Add optional `BITRIVER_HOST_UID`/`BITRIVER_HOST_GID` Compose inputs. The
  packaged installer must atomically persist the resolved operator IDs, while
  Unix launch wrappers derive them for source/archive starts when absent.
  Apply them only to services that write operator-owned bind mounts:
  `srs-config`, `ome-config`, `ome-health-token-check`, `bitriver-live`, and
  `transcoder`. Preserve each image's existing UID fallback when the values are
  absent outside the managed installer.
- Reduce the renderer namespace while correcting ownership: mount the installed
  workspace read-only, move the sanitized SRS helper copy to tmpfs, and keep
  only `/etc/bitriver-live` writable for the two render jobs. Do not grant new
  Linux capabilities, loosen mode `0600` secrets, chown operator config to a
  container-only account, or upload renderer logs containing credentials.
- Extend Compose, installer, wrapper, and release-contract regressions for exact
  UID/GID persistence, idempotent upgrade, duplicate refusal, image-specific
  fallbacks, read-only workspaces, tmpfs execution, and arbitrary non-root
  bind writes. Run Linux lifecycle/package fixtures, permission-faithful exact
  image rendering, Docker Compose validation, focused Go/shell tests, literal
  `./scripts/verify.sh`, protected PR/exact-main CI, and then publish only the
  next unused immutable candidate (RC17) for another no-checkout qualification.
- Protected PR run `31341715983` proved the ownership, docs, ShellCheck, native
  arm64 viewer, and blocking image-scan gates, but the Ubuntu smoke exposed a
  path-alias regression: `ome-config` still writes the generated file through
  the now-read-only `/workspace` mount. The writable config-root mount already
  aliases the same source-checkout or installed target, so make both renderers
  write directly under `/etc/bitriver-live/deploy`, make the verifier read from
  that same canonical mount, and keep templates/assets read-only. Add a static
  Compose assertion plus a real source-checkout render that proves the host
  generated path changes without granting workspace write access. Re-run the
  focused renderer tests, literal verifier, and fresh protected CI; do not
  weaken mounts or bypass the failed gate.
- Automated review also identified that `scripts/quickstart.sh` derives process
  UID/GID values before Compose reads the selected env file. Because process
  values win over `--env-file`, an explicit operator owner in `.env` can be
  silently replaced by the invoking account. Detect both supported env-file
  flag forms without sourcing or printing the file; if either managed identity
  key has a non-empty env-file value, export neither fallback and let Compose
  plus `env validate` retain or reject the pair. Derive both current IDs only
  when neither the process nor selected file declares an identity. Exercise
  the wrapper with stubbed Go for default derivation and both explicit flag
  forms, then rerun shell/static/full verification.

## Current scope - publish and inspect the first signed release-set candidate (#1271, #1301) (2026-08-03)

- Use `v1.2.3-rc.13`, the next unused candidate tag after the public RC12, and
  point it at the exact protected-main commit that passed CI. Do not move or
  recreate any prior tag.
- Let the candidate-only release workflow build once, sign all five exact image
  digests, exercise the anonymous pull-only product gate, generate/sign the
  deterministic release-set root, and publish the complete prerelease assets.
- Inspect the public GitHub release, checksum coverage, release-set signature,
  exact tag/commit/workflow binding, five image signature bundles, evidence,
  SBOMs, and anonymous GHCR visibility before calling the candidate valid.
- Close #1271 only if the public candidate proves its complete provenance
  contract. Keep #1301 and every external stable gate open: publishing a signed
  candidate is not byte-identical stable promotion or Ubuntu/XOA qualification.

### Risks and boundaries

- Tag publication is externally visible and immutable in normal operation. Tag
  only the exact green main commit and never repair a bad candidate by moving
  the tag; fix forward with another prerelease.
- Registry propagation, keyless signing, scanners, multi-architecture builds,
  and the pull-only media stack are network-dependent. Treat a failed run as
  evidence to diagnose, not permission to weaken or bypass a gate.
- Preserve the private root `.env` and the six operator-owned untracked paths.
  A candidate does not authorize stable aliases, `latest`, or production
  exposure through Nginx Proxy Manager.

### Test and rollout plan

- Merge the completed promotion rollout ledger through the protected aggregate
  gate and require exact-main CI to remain green.
- Create an annotated `v1.2.3-rc.13` tag at that exact main commit and push only
  the tag. Monitor the release workflow to a terminal result.
- Verify public release assets and digest/checksum/signature relationships with
  the repository verifier plus direct GitHub/GHCR inspection. Confirm the
  release is a prerelease and that `v1.2.3` and `latest` remain untouched.
- Record the workflow/release evidence in `TASKS.md` and the relevant issues.
  If publication fails, retain the failed tag/run for audit and fix forward.

### Outcome

- Ledger PR #1367 squash-merged as `d416968e`; exact-main CI run
  `30795396504` passed before the annotated `v1.2.3-rc.13` tag was created at
  that exact commit and pushed without moving an earlier tag.
- Release run `30795492882` passed every environment, Postgres, Go, package,
  multi-platform build, exact-image signing, anonymous pull-only product,
  payload scan, release-set signing, and publication job. The public release is
  a prerelease with 46 assets and no stable or `latest` alias.
- The complete public download passed `release_set.py verify-candidate`; every
  downloaded SHA-256 matched GitHub's server digest. `release-set.json` has
  SHA-256 `795fffee84662aec91624eb4352b9c1a9ef5c34b17838939adaf567418797fa0`
  and binds RC13, commit `d416968e`, run `30795492882`, 42 payload artifacts,
  five evidence files, eight passed workflow gates, and eight pending external
  gates.
- Checksum-verified Cosign v3.1.2 independently verified the signed release-set
  root and all five public image signatures against the exact
  `release.yml@refs/tags/v1.2.3-rc.13` identity and GitHub OIDC issuer. Empty-
  credential GHCR inspection resolved all five RC13 tags to their manifest
  digests and confirmed every `v1.2.3`/`latest` alias remained absent.
- The public golden-path evidence passed real RTMP ingest, live/offline state,
  OME LL-HLS playlist advancement and decoded 1080p H.264 media, transcoder HLS,
  chat/moderation, VOD upload/playback, and final aggregate readiness. #1271 is
  closed; #1301 and #1297/#1298/#1299/#1303-#1307 remain open for actual stable
  promotion and target Ubuntu/XOA/NPM qualification.

## Current scope - immutable candidate release sets and stable promotion (#1301, #1271, #1302) (2026-08-03)

- Make prerelease candidate tags the only build inputs. A stable tag must never
  invoke the build workflow or move `latest`; stable publication is a separate
  manual promotion from one already-published candidate tag and digest set.
- Add a deterministic `bitriver.release-set/v1` generator/verifier. The signed
  root manifest records the candidate tag and commit, workflow run, every
  public artifact checksum/size, five first-party image digests and SBOM/
  signature references, pinned third-party digests, toolchain policy, gate
  results, evidence hashes, and explicit skipped/remaining external gates.
- Sign all five image manifest digests with keyless Cosign, verify those exact
  digests during the anonymous pull-only product gate, then sign the release-
  set manifest. Publish the scanner-approved manifest, bundle, checksums, and
  fixed-schema evidence as candidate release assets.
- Add a `workflow_dispatch` stable-promotion workflow on protected `main`.
  Its read-only `Stable promotion gate` downloads the candidate assets, checks
  GitHub asset digests, checksum coverage, root/image signatures, revocation
  state, the candidate commit, and a tracked promotion record whose required
  gate issues/evidence all target the same release-set hash.
- Only after validation may an environment-approved job retag exact image
  digests, generate stable and previous-stable rollback manifests, copy the
  unchanged candidate assets into a draft stable release, verify all aliases,
  optionally update convenience-only `latest`, and publish the draft. Reruns
  must be idempotent and must reject any existing tag, asset, or alias that
  points at different bytes.
- Add an environment-approved candidate-revocation operation that publishes a
  signed, append-only revocation marker. Promotion refuses any candidate with
  that marker; no prior stable asset or alias is overwritten by revocation.
- After the workflow merges and exact-main CI passes, configure the public
  repository's `stable-promotion` environment with a required reviewer,
  self-review allowed for the single-owner project, and protected-branch-only
  deployment. Do not invent completion evidence for the still-open Ubuntu/XOA,
  recoverability, resilience, capacity, security, or browser gates.

### Risks and boundaries

- Cross-repository image aliases and GitHub release publication are not one
  atomic transaction. Validate every immutable input before the first write,
  publish through a draft release, retain no-op/retry semantics, and keep
  production truth at candidate tag plus digest rather than `latest`.
- Candidate assets intentionally retain their candidate filenames/package
  metadata when promoted; renaming or rebuilding them would violate identical-
  byte promotion. Stable metadata points back to the candidate release set.
- Keyless signature verification must bind the exact repository workflow
  identity, tag ref, and GitHub OIDC issuer. Do not accept wildcard identities,
  disable claim checking, or treat an unsigned checksum file as provenance.
- Promotion evidence may arrive after the candidate build. It must be a tracked
  main-branch record with durable URL plus SHA-256 references and the exact
  signed release-set hash; expiring Actions logs alone are insufficient.
- Do not change the deployment contract, runtime code, private root `.env`, or
  the six user-owned untracked deployment/runtime paths in this slice.

### Test and rollout plan

- Unit-test deterministic manifest output, checksum coverage, exact image/
  evidence schemas, signature references, promotion-record gate completeness,
  revoked candidates, stable/candidate tag matching, rollback metadata, path
  traversal, duplicate assets, tampering, and idempotence classifications.
- Extend workflow contracts for candidate-only tag triggers, no build-time
  `latest`, pinned Cosign installation, exact-digest signing/verification,
  signed release-set publication, least-privilege promotion permissions,
  environment approval, no build steps, validation-before-mutation, draft
  publication, rollback metadata, and refusal to overwrite mismatched state.
- Run Python fixtures, YAML parsing, action pinning/policy checks, focused Go
  workflow tests, ShellCheck where applicable, documentation checks, secret
  scanning, `git diff --check`, and literal `./scripts/verify.sh`.
- Require full PR CI, exact merged-main CI, live environment API readback, and
  a deliberately unverified manual promotion run that fails in `Stable
  promotion gate` before any environment or registry mutation. A new immutable
  RC is the later end-to-end manifest proof; stable promotion remains blocked
  until its external gate record is genuinely complete.
- Literal verification with a digest-complete synthetic environment exposed a
  source-build smoke seam: SRS is the only third-party image that also has a
  local build target, and Compose cannot use its pull-only `@sha256` suffix as
  the resulting build tag. In build-mode smoke only, clear
  `BITRIVER_SRS_IMAGE_DIGEST` alongside the five first-party build digests and
  reapply those overrides after sourcing the env file before `compose up`;
  preserve every pin in pull/production mode and add a shell contract fixture
  so release digest enforcement is never weakened.

### Outcome

- PR #1366 squash-merged as `38cfeb13` after PR CI run `30793307725` and its
  protected `Merge gate` passed. Exact-main CI run `30793780301` also passed,
  including the unified Ubuntu gate, cross-platform Go/quickstart jobs, image
  scans, and aggregate gate.
- GitHub environment `stable-promotion` was created and read back with required
  reviewer `ProhibitedTV`, self-review allowed for the single-owner project,
  and protected-branch-only deployment.
- Deliberate negative run `30794248391` rejected the absent tracked promotion
  record in the unprivileged `Stable promotion gate`; both mutation jobs were
  skipped, and no `v1.2.3` tag or release was created. Issue evidence was added
  to #1271, #1301, and already-closed #1302 without claiming external gates.

## Current scope - enforce one aggregate merge gate (#1302, #1270) (2026-08-03)

- Preserve the consolidated, path-filtered reusable workflow graph. Add one
  always-run `Merge gate` that evaluates the result of every existing child
  job against the changed-file outputs and fails on failed, cancelled, or
  unexpectedly skipped required work. Do not duplicate child test commands.
- Reuse `scripts/check-pr-release-scorecard.sh`. Keep low-risk/docs-only
  scorecards advisory, but make the scorecard blocking when changed paths
  automatically indicate workflow, deployment, migration, auth/security, or
  release/operator risk. Publish the scorecard and aggregate table as a
  human-readable Actions artifact and job summary.
- Live GitHub audit found no rulesets and `main` is not protected. After the
  implementation PR and exact merged-main CI pass, require the stable `Merge
  gate` context with strict up-to-date status, pull requests, conversation
  resolution, admin enforcement, and no force pushes or deletion.
- Demonstrate the aggregate failure locally and in the implementation PR before
  correcting its scorecard. After protection is active, use a disposable
  intentionally failing canary PR to prove the protected branch blocks a bad
  aggregate result, then close it and delete its branch.
- This slice completes #1270's gate clarity/artifact boundary and the merge half
  of #1302. Stable-promotion enforcement remains explicitly blocked on #1301's
  immutable release-set manifest and is not reimplemented here.

### Risks and boundaries

- GitHub reports conditional jobs as `skipped` both when they are correctly
  irrelevant and when an upstream required job fails. Expected-job decisions
  must therefore come from the changed-file outputs, while every unexpected
  failure/cancellation blocks regardless of path selection.
- The aggregate job must run with `if: always()` and depend on every child job;
  otherwise GitHub can skip the aggregate precisely when it is most needed.
- PR body content is untrusted input. Read it from `GITHUB_EVENT_PATH`, never
  interpolate it into shell source, and keep workflow permissions read-only.
- Do not change the deployment contract, release artifacts, runtime behavior,
  private root `.env`, or the six user-owned untracked deployment/runtime paths.

### Test and rollout plan

- Add fixture coverage for docs-only success, selected-job success, required
  skip, child failure/cancellation, unexpected extra failure, and scorecard
  failure; add workflow contracts for complete `needs`, `if: always()`, stable
  job naming, full-history checkout, summary, and pinned artifact upload.
- Extend scorecard tests for advisory low-risk and blocking risky-path modes.
  Run shell syntax/ShellCheck, YAML parsing, pinned Go script tests, CI contract,
  committed-secret guard, `git diff --check`, and literal `./scripts/verify.sh`.
- Require an intentional red aggregate run followed by corrected full PR CI,
  exact merged-main CI, live branch-protection API verification, and a blocked
  disposable canary PR before calling the merge enforcement active.

### Outcome

- PR #1363 merged as `200bf414`; exact-main run `30787937537` passed every
  selected child and the stable `Merge gate`.
- Live `main` protection now requires the strict, app-bound GitHub Actions
  `Merge gate`, pull requests, current-head checks, resolved conversations, and
  admin enforcement; force pushes and branch deletion are disabled.
- Ready canary PR #1364 failed the required gate in run `30788386196`, GitHub
  reported it `BLOCKED`, and it was closed unmerged with both branch refs
  removed. Issue #1270 is closed; #1302 stays open only for #1301's immutable
  stable-promotion work.

## Current scope - remove tracked generated Go caches (2026-08-02)

- Remove the four root `.gocache-*` directories that were accidentally added
  by commit `b4617f75`. They contain 6,775 generated Go build-cache objects and
  416,199,766 bytes in the current tree; no source, workflow, documentation, or
  release path references them.
- Preserve repository history, tags, releases, and every non-cache path. This
  is a HEAD cleanup, not a history rewrite: future checkouts stop materializing
  the 416 MB cache payload, while old commits remain reproducible.
- Keep the existing `/.gocache-*/` and `.gocache/` ignore rules. Add a focused
  repository-hygiene regression to reject any tracked root Go cache even when
  it is force-added, and run that guard through the canonical verification
  path rather than creating another overlapping workflow.
- PR #1361 exposed a release-blocking large-diff seam in the existing CI
  orchestrator: `dorny/paths-filter` used GitHub's pull-request files API,
  received only the first 3,000 of 6,780 changed paths, and therefore skipped
  every substantive job even though three `scripts/**` files changed. Keep the
  pinned action, but use its documented empty-token mode so the complete
  checkout and unbounded `git diff` drive pull-request routing.
- Close the preceding documentation task with its exact merged-main evidence:
  PR #1354 merged as `f3952624`, `main` CI run `30779028786` passed, and the
  remote returned to 58 branches after the PR head was deleted.

### Risks and boundaries

- A deletion-only cache diff is mechanically large. Build the removal set from
  `git ls-files` under the four exact audited roots, verify that all deleted
  entries match those roots, and never use a broad recursive cleanup against
  the repository.
- Do not touch the private root `.env`, generated OME configuration, or the
  user's six untracked deployment/runtime paths.
- Do not claim that deleting tracked files removes their objects from existing
  Git history or immediately shrinks every established clone; repository
  history rewriting is deliberately out of scope.
- Do not weaken path selectivity or force every expensive job for every PR.
  The correction must preserve the existing filters while changing only their
  complete changed-file source.

### Test plan

- Prove the new hygiene guard accepts the cleaned tree and rejects a temporary
  repository with a force-added root `.gocache-*` artifact.
- Require the tracked deletion inventory to contain exactly 6,775 files under
  the four audited roots and zero other paths; run `git diff --check` and the
  committed-secret guard.
- Add a CI workflow regression that requires the changed-file detector's
  supported Git fallback plus full-history checkout, and prove PR #1361 now
  selects at least the Ubuntu, ShellCheck, and quickstart jobs for the changed
  canonical verification scripts.
- Run literal `./scripts/verify.sh`, PR CI, and exact merged-main CI before
  considering the cleanup complete.

### Completion outcome

- PR #1361 squash-merged as `5e736f2b`; exact merged-main CI run
  `30785253246` passed the complete selected matrix.
- The PR head was deleted and stale local tracking refs were pruned. The remote
  now has 64 branches rather than the earlier 58 because Dependabot created six
  separate viewer dependency heads during the documentation/cleanup window.
  Those bot-authored, non-audited heads are preserved as concurrent work; they
  are not residue from the cleanup branch or justification for blind deletion.

## Current scope - first-time installer documentation refresh (2026-07-31)

- Treat published prerelease `v1.2.3-rc.12` as the current consumer artifact
  baseline. Its 33-asset release is checksum-complete; downloaded Linux amd64
  launcher and `.deb` payloads contain the exact RC12 tag for all five
  first-party images; retained scans and the eight-stage pull-only product gate
  passed at commit `3a9572f0`.
- Rewrite the README for two clear audiences: a Windows user evaluating the
  full stack with Docker Desktop from a checkout, and an Ubuntu 24.04 amd64
  operator installing the published package/archive for an XOA VM and Nginx
  Proxy Manager deployment. Lead with outcomes, prerequisites, exact commands,
  first-stream flow, and links to deeper operator runbooks.
- Keep the two committed screenshots because they are provenance-known captures
  of the shipped viewer; remove stale release-availability language, malformed
  punctuation, unsupported GitHub Pages guidance, and repeated source-only or
  planned-`rc.1` copy. Prefer concise workflow diagrams/text over promotional
  imagery that is not runtime evidence.
- Align quickstart, Ubuntu install, viewer deployment, release index/notes,
  changelog, production status/release, operations, and support navigation with
  the shipped RC12 commands and assets. Strengthen documentation consistency
  checks so the most damaging pre-release-only wording cannot return.

### Documentation evidence boundary

- Say exactly what RC12 proves: Docker Desktop source verification on this
  Windows host, package install/remove acceptance on Ubuntu 24.04/Debian/Rocky,
  anonymous five-image pulls, OME-backed ingest/live playback, offline
  transition, chat/moderation, and VOD playback in the tagged product gate.
- Do not claim a clean XOA VM installation, Nginx Proxy Manager browser path,
  host reboot recovery, or repeated OME restart/media recovery has been
  exercised. Present the existing runbooks and acceptance commands for those
  remaining operator checks.
- Run focused wording/link/Markdown/YAML checks, the documentation consistency
  gate, `git diff --check`, and literal `./scripts/verify.sh`; then require PR
  CI and exact merged-main CI before updating the public repository front door.

## Previous scope - stamp exact image tags into release artifacts (2026-07-31)

- Preserve immutable published prerelease `v1.2.3-rc.11` at commit
  `96e99fd6`. Its workflow, checksums, images, pull-only product gate, and
  publication evidence passed, but a direct download/extraction audit found a
  release-blocking installer defect: the Linux `.deb` and launcher archive
  both seed all five first-party image tags as stable `v1.2.3`, while RC11
  publishes only `v1.2.3-rc.11` images.
- Keep the canonical root/development `deploy/.env.example` defaults at stable
  `v1.2.3`. Add an explicit, validated release-tag input to the canonical
  staging helper and rewrite only the staged copy's five
  `BITRIVER_*_IMAGE_TAG` values. Require every release bundle, launcher,
  Linux-package, and Windows-installer staging call to pass the immutable Git
  tag.
- Add focused regressions that prove prerelease-tagged staged payloads use the
  exact tag for all five first-party images, the source env remains unchanged,
  and release workflow packaging cannot omit the tag argument.
- Do not present RC11 as a working first-time Ubuntu installation path. After
  focused checks and literal `./scripts/verify.sh`, require complete PR CI and
  exact merged-main CI before creating immutable `v1.2.3-rc.12`.

### `rc.12` artifact-acceptance plan

- Accept RC12 only after its complete release workflow and pull-only tagged
  product gate pass, publication evidence is retained, and the GitHub
  prerelease has a checksum-complete asset inventory.
- Download the published RC12 Linux `.deb` and launcher archive, extract both
  independently, and confirm that each installed/staged `.env.example` names
  `v1.2.3-rc.12` for API, viewer, SRS controller, transcoder, and OME config.
- Keep clean Ubuntu/XOA installation, Nginx Proxy Manager browser access,
  reboot recovery, and repeated OME restart/media recovery as separate,
  unproved promotion evidence until exercised on the target VM.
- Resume the consumer README/docs refresh only after the published package
  audit passes, so the documented first-install path is based on an artifact
  users can actually run.

## Previous scope - ancestry-safe remote branch cleanup (2026-07-31)

- Preserve published prerelease `v1.2.3-rc.11`, every tag, `main`, and every
  branch whose tip is not an ancestor of current `origin/main`.
- The immediate post-release fetch/prune and GitHub API inventory agreed on
  1,002 real remote branches: `main`, 942 ancestry-merged non-default tips, and
  59 non-ancestor tips. GitHub reported no protected branches.
- Delete only those 942 ancestry-merged refs. Every 50-ref atomic batch must
  follow a fresh fetch and confirm exact object IDs, ancestry, and the current
  open-PR exclusions; stop on drift or a failed remote update.
- During the final reconciliation, Dependabot PRs #1346 and #1344 merged and
  GitHub deleted their two previously preserved heads while advancing `main`
  from `96e99fd6` to `d3740828`. Those heads were not members of the deletion
  set. The resulting authoritative inventory is `main` plus 57 non-ancestor
  branches, with zero merged non-default branches remaining.

### Branch-cleanup verification outcome

- All 942 candidate names and object IDs were classified before mutation and
  revalidated across 19 atomic batches (18 of 50 refs and one of 42).
- The final local remote-ref inventory and GitHub API both report 58 branches;
  ancestry classification reports zero merged non-default tips and 57 retained
  non-ancestor tips.
- The two-branch difference from the expected 60 was classified as concurrent,
  successful PR merges with normal head deletion, not accidental cleanup.
- Keep the six user-owned untracked deployment paths and the private root
  `.env` untouched.

## Previous scope - publish file-backed release notes (2026-07-31)

- Preserve immutable `v1.2.3-rc.10` at verified main commit `48e4b878` and
  failed release run `30665278361`. The failed-job rerun passed the pull-only
  tagged product gate, scanned the complete publication payload in 31 seconds,
  retained publication evidence, and generated release notes. No GitHub
  prerelease exists. Do not rerun, move the tag, or publish it manually.
- Repair only the final publication input boundary. Because this is the
  repository's first GitHub Release, generated notes contain the full prior
  history; passing that body inline to the Node action exceeded the runner's
  process argument limit before the action started.
- Write the generated body as UTF-8 under `RUNNER_TEMP`, expose only its short
  path as a step output, and use the release action's supported `body_path`
  input. Do not truncate the changelog, weaken the atomic finalizer, or bypass
  its complete dependency/evidence gates.
- Add a workflow contract regression that requires file-backed notes and
  rejects the oversized inline-body form.

### `rc.11` test and publication plan

- Run the focused release-workflow regression, complete `go test ./scripts`,
  workflow/action YAML parsing, both CI policy checks, `git diff --check`, and
  literal `./scripts/verify.sh` while preserving the private root `.env`
  byte-for-byte.
- Require complete PR CI and exact merged-main CI before creating annotated,
  immutable `v1.2.3-rc.11`.
- Accept RC11 only after every release job passes, the pull-only OME-backed
  product gate succeeds, publication evidence is retained, the GitHub
  prerelease exists, and its asset/checksum inventory is complete.
- Clean Ubuntu/XOA installation, Nginx Proxy Manager browser access, reboot,
  and repeated OME restart/media recovery remain separate promotion evidence.

## Previous scope - bound release payload scanning (2026-07-31)

- Preserve immutable `v1.2.3-rc.9` and failed release workflow `30655699977`.
  All application, package, image/SBOM, and pull-only tagged product gates
  passed; the final scanner spent 59 minutes finding 224 false-positive
  assignments plus RPM extraction errors, then failed before publication. No
  GitHub prerelease exists. Do not rerun, move the tag, or publish manually.
- Treat the finalizer duration as a release-engineering defect independent of
  RC9's eventual terminal result. The scanner has processed a roughly 250 MB
  mixed artifact set since 18:45 UTC without completing; a local 8.3 MB subset
  containing the viewer bundle exceeded five minutes.
- Preserve every covered secret class, archive-depth limit, path-only finding,
  and no-secret-output property. Use ripgrep to prefilter credential-shaped
  matches and classify only bounded matches in one awk process per rule; retain
  a tested grep/awk fallback when ripgrep is absent. Distinguish real literal
  credential keys from package hashes, framework cache tokens, parser tokens,
  and ordinary code references without allowing literal `api_token` values.
- Exclude Buildx `.dockerbuild` records from the final release artifact
  download. They are workflow diagnostics, not publication payloads or SBOMs,
  and GitHub CLI cannot reliably extract at least one RC9 record as a ZIP.
- Give the payload-scan step an explicit timeout so a future pathological
  artifact blocks publication promptly instead of consuming the default job
  maximum. Do not weaken scan failures or publish on timeout.
- Extract RPM payloads with absolute filenames disabled so deep inspection
  remains inside the scanner scratch directory and valid packages do not fail
  as unreadable archives.

### Payload-scanner test plan

- Retain all focused positive/negative secret-scanner tests, including nested
  archives, sentinel values, forbidden filenames, assignments, credential
  URLs, XML credentials, the no-ripgrep fallback, and non-disclosure of matched
  values.
- Add workflow contracts for excluding `.dockerbuild` records and bounding the
  scan step. Add a high-line-count benign fixture with a practical elapsed-time
  assertion so the external-process regression is reproducible without a large
  binary artifact corpus.
- Re-run the complete scanner/release-workflow Go suites, shell syntax,
  workflow/action YAML parsing, CI policy checks, `git diff --check`, and the
  repository verification gate before publication. Re-scan the real RC9 viewer
  artifact subset and an actual RC9 RPM when available.
- After PR and exact-main CI pass, publish the next immutable candidate
  `v1.2.3-rc.10`; accept it only if the final scanner, retained evidence,
  complete release asset set, and GitHub prerelease creation all pass.

## Previous scope - repair the `rc.8` image publisher and publish `rc.9` (2026-07-31)

- Preserve immutable `v1.2.3-rc.8` at verified main commit `5295c1e4`.
  Release run `30653362368` proved production configuration, migrated
  Postgres, repository verification, every CLI/release/launcher build, the
  hosted MSI, Linux packages and package acceptance, Homebrew, the viewer
  bundle, native viewer images, and its assembled multi-architecture manifest.
  It did not create a GitHub prerelease.
- Repair the shared tag-only image publication boundary. The srs-controller,
  ome-config, and transcoder multi-architecture builds invoked their
  Dockerfiles without build arguments, so all three used the offline-oriented
  defaults `GOPROXY=direct` and `GOSUMDB=off`. Both `gopkg.in/check.v1` and
  `gopkg.in/yaml.v3` then returned HTTP 502 during the arm64 dependency fetch;
  the pull-only product gate and GitHub Release correctly skipped.
- Pass `https://proxy.golang.org,direct` and `sum.golang.org` only to the
  release image Buildx step, matching the already-proven quickstart and image
  scan build boundary. Keep host Go verification offline and leave runtime
  network, image names, Dockerfiles, and the deployment contract unchanged.
- Add a release-workflow contract regression so a future refactor cannot
  silently return tag-only builders to direct-only dependency resolution.

### `rc.9` test and publication plan

- Run the focused release workflow regression, complete Go `./scripts` suite,
  workflow/action YAML parsing, CI policy checks, and `git diff --check`.
- Run literal `./scripts/verify.sh --viewer` while preserving the private root
  `.env` byte-for-byte; require full PR CI and exact merged-main CI before
  creating annotated immutable `v1.2.3-rc.9`.
- Accept `rc.9` only after every release job passes, including all image/SBOM
  publishers, the pull-only tagged product gate, complete assets/checksums, and
  GitHub prerelease creation. Never move or reuse the failed `rc.8` tag.
- Clean Ubuntu/XOA installation, Nginx Proxy Manager browser access, reboot,
  and repeated OME restart/media recovery remain separate promotion evidence.

## Previous scope - repair the `rc.7` viewer-auth race and publish `rc.8` (2026-07-31)

- Preserve immutable `v1.2.3-rc.7` at verified main commit `2dfa77d0`.
  Release run `30648508975` proved the hosted MSI, both native viewer image
  architectures, the assembled viewer multi-architecture manifest, all other
  first-party images, Linux packages and package acceptance, Homebrew, and the
  pull-only tagged product gate. It did not create a GitHub prerelease.
- Repair the only failed producer at the product boundary. The viewer bundle
  passed install, audit, lint, Jest, and production build, then Playwright
  clicked the navbar sign-in action before the initial `/api/viewer/me`
  response had populated its configured `/login` URL. The UI opened the local
  auth flow (`/?auth=signin&next=%2F`) instead of redirecting to
  `/login?redirect=%2F`.
- Keep authentication actions unavailable while initial auth discovery is in
  progress, and make the configured-login browser regression wait for the
  mocked auth response before clicking. This prevents the observed user-facing
  misroute and proves the expected external-login contract deterministically.
- Harden the Docker dependency-fetch boundary exposed by PR CI. Both the
  initial run and failed-job rerun reached direct `gopkg.in` fetches and failed
  on HTTP 502 while native arm64 viewer proof passed. Keep host Go tests
  offline, but give quickstart/verification Compose builds and the reusable
  image-scan build an explicit `https://proxy.golang.org,direct` plus checksum
  database configuration. Do not change runtime networking or image tags.

### `rc.8` test and publication plan

- Add/adjust focused Navbar and Playwright regressions for initial auth loading
  and configured-login navigation. Add workflow/script contracts that require
  the build-only Go proxy in quickstart and image scan; keep the release
  workflow and offline host-test policy unchanged.
- Run viewer lint, Jest, Playwright, and production build, the relevant Go
  workflow contracts, workflow/action YAML parsing, `git diff --check`, and
  literal `./scripts/verify.sh --viewer` while preserving the operator `.env`
  hash.
- Require complete PR CI and exact merged-main CI before annotated immutable
  `v1.2.3-rc.8`. Accept the candidate only after every release job passes and
  GitHub publishes the prerelease with complete assets/checksums.
- Clean Ubuntu/XOA installation, Nginx Proxy Manager browser access, reboot,
  and repeated OME restart/media recovery remain separate promotion evidence.

## Previous scope - repair the `rc.6` hosted release failures and publish `rc.7` (2026-07-31)

- Preserve immutable `v1.2.3-rc.6` at verified main commit `d94ac432`.
  Release run `30643868431` proved the restored viewer baseline, Go/Postgres,
  every CLI/release/launcher matrix entry, Linux packages and all three
  package-acceptance hosts, Homebrew, and four first-party multi-architecture
  images. It did not create a GitHub prerelease.
- Repair the hosted MSI at the actual ICE validation boundary. Precomposed WiX
  definition arguments worked and `candle.exe` compiled both sources, but
  `light.exe` rejected both shortcut key-path rows with `ICE03: Invalid
  registry path`. The WiX XML incorrectly uses doubled backslashes in registry
  keys (and shortcut target paths), even though XML does not escape a
  backslash. Use canonical single-backslash Windows paths and retain ICE
  validation.
- Remove emulated Node installation from viewer image publication. The amd64
  image completed, while the arm64 Buildx leg crashed inside `npm ci` with
  `qemu: uncaught target signal 4 (Illegal instruction)` and never unwound.
  Build amd64 on `ubuntu-latest` and arm64 on GitHub's native
  `ubuntu-24.04-arm` runner, then assemble the public multi-architecture
  manifest and SBOM in a bounded finalizer job. Keep the other four image
  publishers unchanged apart from an explicit timeout.

### `rc.7` test and publication plan

- Add regressions requiring canonical WiX registry/shortcut paths, native
  per-architecture viewer jobs, absence of QEMU from the viewer path, bounded
  image jobs, final manifest assembly, and downstream product/release
  dependencies.
- Prove the native amd64 viewer image locally and structurally verify exact
  runner/platform pairing for both architectures; hosted PR/release jobs are
  authoritative for native arm64 build/runtime evidence. Parse all
  workflow/action YAML, compile/link the MSI with pinned WiX, run focused and
  full workflow tests, then finish with literal `./scripts/verify.sh --viewer`
  while preserving the operator `.env` hash.
- Require complete PR CI and exact merged-main CI before annotated immutable
  `v1.2.3-rc.7`. Accept the candidate only after the hosted MSI, both native
  viewer architectures, multi-architecture manifest/SBOM, all packages and
  release assets/checksums, anonymous GHCR pulls, OME-backed product gate, and
  GitHub prerelease publication pass.
- Clean Ubuntu/XOA installation, Nginx Proxy Manager browser access, reboot,
  and repeated OME restart/media recovery remain separate promotion evidence.

## Previous scope - restore the release baseline and publish `rc.6` (2026-07-31)

- Preserve the immutable `v1.2.3-rc.5` tag at commit `72283baf`. Release run
  `30222035324` passed environment validation, migrated Postgres, repository
  verification, the viewer bundle, every CLI/release/launcher artifact except
  the MSI, Linux arm64 and amd64 packages, Ubuntu/Debian/Rocky package
  acceptance, Homebrew generation, all five container publications/SBOMs, and
  the pull-only tagged media/API product gate. GitHub Release creation skipped
  because the MSI job failed, so `rc.5` is not a published prerelease.
- Fix the remaining MSI failure without weakening validation. Pinned WiX 3.14.1
  downloaded and checksum-verified on the hosted Windows runner, but PowerShell
  passed `-dProductVersion=$env:MSI_VERSION` literally to `candle.exe`. Compose
  the WiX definition arguments as explicit PowerShell strings before invoking
  the native tool, and keep ICE validation enabled.
- Restore the viewer's proven release baseline before any successor tag.
  Dependabot merges moved `main` to Next 16.2.12, React/React DOM 19.2.8,
  Node types 26.1.1, ESLint 10.8.0, and TypeScript 7.0.2. Current-main CI run
  `30404788813` failed the pinned runtime-baseline contract and clean viewer
  image construction; `ts-jest@29.4.11` requires TypeScript `<7`. Restore only
  the six guarded versions while retaining unrelated compatible updates, then
  prove the lockfile, clean install, audit, tests, build, and container image.

### `rc.6` test and publication plan

- Extend release-workflow regression coverage to require precomposed
  `ProductVersion`, `SourceDir`, and `ReleaseAssetsDir` arguments and reject the
  inline PowerShell environment-variable form that reached WiX literally.
- Run focused Go workflow/runtime-baseline suites, parse all workflow/action
  YAML, run a clean viewer `npm ci`, dependency-tree validation, production
  audit, lint, Jest, Playwright, Next production build, and a real viewer image
  build. Finish with literal `./scripts/verify.sh --viewer` and confirm the
  operator-owned root `.env` hash is unchanged.
- Require complete PR CI and exact merged-main CI. Only then create the next
  annotated immutable tag, `v1.2.3-rc.6`, and accept it only if the hosted MSI,
  package acceptance, release assets/checksums, anonymous GHCR pulls, and the
  OME-backed pull-only product gate all pass and GitHub publishes a prerelease.
- Clean Ubuntu/XOA installation, Nginx Proxy Manager browser access, reboot,
  and repeated OME restart/media recovery remain separate promotion evidence.

## Next scope - public repository hygiene and first-time install docs (2026-07-31)

- The refreshed remote inventory contains 1,003 branches including `main`:
  943 non-default tips are ancestors of `origin/main`; 59 are not merged and
  include the two open Dependabot PR heads. Delete only the 943 ancestry-merged
  refs after an immediate pre-delete ancestry recheck; preserve `main`, tags,
  all 59 non-ancestor refs, and any newly opened PR head.
- Re-audit the repository from a first-time consumer's point of view after a
  successful public candidate exists. README should lead with what BitRiver
  Live is, supported install choices, prerequisites, a short Docker Desktop
  evaluation path, the Ubuntu 24.04 artifact/package path, post-install stream
  workflow, reverse-proxy/media-port boundaries, and honest release status.
- Replace promotional/generated imagery with actual product and workflow
  screenshots only when their provenance and currentness can be verified.
  Until then, prefer concise diagrams or text over imagery that misrepresents
  the shipped UI.
- Align quickstart, Ubuntu/XOA/Nginx Proxy Manager, production release,
  operations, security, architecture, upgrade/backup, troubleshooting, and
  release-note docs with the same commands and evidence boundaries. Add link
  and wording regressions so stale no-release or source-only guidance cannot
  silently return.

## Current scope - repair the `rc.4` release fan-out (2026-07-26)

- Preserve the immutable `v1.2.3-rc.4` tag at merged commit `e67a9304`.
  Merged-main CI passed, but release run `30220542359` cannot create a GitHub
  Release because three packaging jobs failed. Corrections use the next
  immutable candidate tag, `v1.2.3-rc.5`.
- Provision the Windows MSI toolchain explicitly. `windows-latest` no longer
  contains the hardcoded WiX v3.11 installation, so the release must install a
  pinned compatible WiX release and resolve `heat`, `candle`, and `light`
  through the installed tool path instead of a runner-image assumption.
- Keep packaging tools on the runner platform. The Linux arm64 launcher build
  correctly targets the BitRiver binary to arm64, but that target environment
  leaked into `go install nfpm`, placing an arm64 helper outside the host
  executable path. Build nFPM with explicit `GOHOSTOS`/`GOHOSTARCH`, while
  retaining arm64 only for the shipped binary and package metadata.
- Anchor viewer output to the workspace artifact directory. Viewer lint, 217
  Jest tests, 36 Playwright tests, and the production build passed, but the job
  created its archive under `web/viewer/dist` while `upload-artifact` consumed
  repository-root `dist`.

### `rc.4` failure evidence

- Release environment validation, migrated Postgres, the unified verification
  gate, cross-platform CLI/release binaries, four launcher/signature entries,
  Linux amd64 packages, all five image publications/SBOMs, and the pull-only
  tagged media/API product gate passed.
- Windows MSI job `89842557873` failed before WiX compilation because
  `C:\Program Files (x86)\WiX Toolset v3.11\bin\heat.exe` does not exist on the
  current hosted runner.
- Linux arm64 launcher job `89842558023` signed its target binary, then failed
  with `nfpm: command not found` after target `GOARCH=arm64` affected the host
  tool installation.
- Viewer job `89842557853` passed every check and packaged successfully, then
  failed because producer and upload paths were rooted in different working
  directories.
- Downstream package acceptance, Homebrew generation, and GitHub Release
  creation were skipped; they are not evidence from `rc.4`.

### Test and publication plan

- Add workflow-contract regressions that require pinned MSI tool provisioning,
  host-scoped nFPM installation, and identical workspace-root viewer
  producer/upload paths.
- Parse every workflow/action YAML file, run the focused release and CI contract
  suites, run `git diff --check`, then run literal
  `./scripts/verify.sh --viewer`.
- Require complete pull-request and merged-main CI before tagging
  `v1.2.3-rc.5`. Monitor every release job; accept the candidate only after
  package acceptance, checksums/signatures/assets, anonymous GHCR access, and
  the pull-only media/API product gate all pass.
- Clean Ubuntu/XOA, Nginx Proxy Manager browser access, reboot, and OME
  restart/media recovery remain separate promotion evidence.

## Current scope - repair the `rc.3` release fan-out (2026-07-26)

- Preserve the failed `v1.2.3-rc.3` tag at `c9d5a9f3`. The tag workflow
  published the five first-party candidate images, but no GitHub Release was
  created because artifact and pull-only acceptance jobs failed. Corrections
  use the next immutable tag, `v1.2.3-rc.4`.
- Keep cross-compilation and host-side binary inspection separate. Target
  `GOOS`/`GOARCH` remain scoped to `go build`; the Go verifier must compile for
  the Ubuntu runner so it can inspect Windows, macOS, and arm64 binaries without
  attempting to execute a foreign verifier.
- Quote the Windows production modfile as one PowerShell argument. The MSI job
  currently passes the literal text `$env:PRODUCTION_MODFILE` to Go, which Go
  rejects because it does not end in `.mod`.
- Emit the current Cosign v3 bundle format and publish that bundle beside each
  launcher binary. Cosign 3 ignores the deprecated `--output-signature` flag,
  leaving an empty bundle path and failing every launcher matrix entry.
- Package the viewer's built output without assuming an optional `public/`
  directory exists. Preserve `.next`, remove its cache, and copy `public/` only
  when present.
- Bridge an explicitly supplied smoke env to the canonical root `.env` only
  when the root file is absent, because Compose service-level `env_file:
  ../.env` resolution is independent from `docker compose --env-file`. Track
  ownership and remove only the smoke-created bridge during cleanup; never
  rewrite or remove an operator-owned root `.env`.
- Keep secret/evidence publication atomic on Windows Docker Desktop as well as
  Linux runners. Open the temporary descriptor under its closing context before
  applying POSIX-only `fchmod`, skip that syscall where unavailable, and retry
  only transient sharing/permission failures from the final same-directory
  `os.replace`. Retain mode 0600 output where supported and remove the
  temporary file on terminal failure.
- Make SRS readiness valid for both source-build and pull-only deployments.
  Production pull mode intentionally selects the pinned upstream SRS digest,
  while source mode builds a wrapper that happens to add `curl`. Replace the
  curl-dependent SRS probe with a bounded Bash `/dev/tcp` HTTP status check
  against `/api/v1/versions`, and keep the Compose and Helm health contracts
  aligned.
- Preserve production-secure session cookies while allowing the Docker product
  harness to authenticate over its internal HTTP hop. The loopback candidate
  correctly marks `bitriver_session` as `Secure`, so Python's cookie jar stores
  it but will not return it to `host.docker.internal`. For same-origin requests
  only, use the captured session value as the API's supported Bearer token
  fallback; never attach it to an absolute cross-origin URL.
- Resolve third-party release dependencies once per tag workflow. The current
  environment preflight and pull-only product gate independently resolve the
  same eight mutable tags, wasting registry quota and allowing the two gates to
  select different manifests. Preflight must publish sanitized, immutable
  reference/digest evidence; the product gate validates and consumes that exact
  artifact while keeping first-party post-publication evidence separate.

### `rc.3` failure evidence

- Release run `30217498138` passed release-env validation, migrated Postgres,
  the Go verification gate, all five multi-architecture image publications,
  SBOM generation, and Linux amd64 CLI/release-artifact builds.
- Every non-Linux-amd64 CLI and release-artifact job failed with `exec format
  error` while `go run ./cmd/tools/verify-production-binary` inherited the
  target platform. This is one host-versus-target environment defect.
- The MSI job generated `go.production.mod`, then failed because
  `-modfile=$env:PRODUCTION_MODFILE` reached Go literally.
- Four cross-target launcher jobs failed at the same foreign host-verifier
  execution. Linux amd64 reached signing, then failed when Cosign 3.0.6 ignored
  `--output-signature` and attempted to create a bundle at an empty path.
- Viewer lint, unit/integration tests, and production build passed; packaging
  alone failed at `cp -R public` because the viewer has no `public/` directory.
- The anonymous pull-only product gate resolved the published images and
  enforced third-party digests, then failed before startup because the clean
  runner had no root `.env`. GitHub Release creation was skipped, so `rc.3`
  must not be presented as a published candidate.
- The first local Docker Desktop pull-only rehearsal resolved the public
  `rc.3` image manifests, then exposed that Windows Python has no `os.fchmod`.
  The exception occurred before `os.fdopen` owned the descriptor, so the leaked
  handle also made cleanup appear as a sharing violation. No private root env
  was moved or changed. Close ownership must begin before the POSIX-only mode
  operation, with the mode operation conditional on platform support.
- After the Windows helper repair, the local pull-only rehearsal passed the
  former root-env failure, anonymously pulled every digest-pinned image, and
  started the stack. It then proved the raw upstream SRS image has neither
  `curl` nor `wget`; SRS answered `/api/v1/versions`, but its curl-based
  healthcheck remained unhealthy until the bounded gate failed. The pinned
  image does provide Bash, and an ephemeral real-image `/dev/tcp` HTTP probe
  returned status 200.
- With the curl-free probe applied, the next local pull-only rehearsal made SRS
  healthy, started every service, and reached the real media harness. Account
  signup succeeded, but the first authenticated `/api/status` request returned
  `401 missing session token`: production correctly emitted a Secure session
  cookie while the trusted Docker client reached the API over plain internal
  HTTP. The harness needs a same-origin Bearer fallback, not a weaker production
  cookie policy.
- After the same-origin session repair passed focused tests, the immediate
  rehearsal retry was blocked before startup by Docker Hub `429 Too Many
  Requests` while resolving `ossrs/srs:v5.0.185`. Release preflight already
  resolves all eight third-party tags, but the downstream product job repeats
  that work on a fresh runner. Carrying immutable dependency evidence between
  jobs removes this avoidable quota and time-of-check/time-of-use seam.

### Risks

- A syntactically green workflow does not prove cross-platform binaries,
  signatures, packages, or MSI output. `rc.4` must complete the entire remote
  matrix and downstream package acceptance before publication.
- Copying a release env into the checkout can expose generated credentials if
  it survives cleanup or becomes an uploaded artifact. The bridge is
  mode-restricted, ownership-tracked, removed on every exit path, and excluded
  from evidence.
- A session fallback must not turn into credential forwarding. Restrict it to
  the exact configured API origin, preserve explicit Authorization headers, and
  cover both same-origin and cross-origin requests in a local HTTP regression.
- Dependency evidence must remain non-secret and candidate-specific. Validate
  its schema, require every expected env key/reference, reject duplicate or
  unexpected entries and malformed digests, scan it with the existing sentinel
  guard, and never allow it to substitute for first-party anonymous GHCR proof.
- Cosign bundles change the released filename/verification contract from the
  historical `.sig` assumption. Workflow tests and operator release
  documentation must agree on the `.sigstore.json` artifact.
- The `rc.3` images exist even though the release failed. They remain immutable
  failure evidence and are not retagged as `rc.4` or `latest`.

### Test and publication plan

- Add focused workflow-contract regressions for host-scoped verifier execution,
  PowerShell modfile quoting, Cosign bundle output, optional viewer assets, and
  root-env bridge ownership/cleanup.
- Add a unit regression that injects transient atomic-replace failures and
  proves eventual success, plus a terminal-failure check that leaves neither
  final nor temporary secret material behind.
- Add Compose/Helm contract coverage that rejects curl-dependent SRS probes and
  requires the same built-in Bash HTTP status check in both deployment shapes.
- Add a functional Python HTTP test proving a Secure signup cookie authenticates
  the next same-origin request through Bearer fallback while an absolute
  cross-origin request receives no session credential.
- Add release-helper and workflow regressions proving preflight resolves one
  complete third-party dependency set, uploads sanitized evidence, and the
  pull-only job downloads and validates it instead of resolving mutable tags
  again.
- Run focused Go script tests, release-candidate Python tests, Bash syntax,
  workflow YAML parsing, CI/workflow policy checks, and `git diff --check`.
- Run literal `./scripts/verify.sh --viewer`, preserving and hashing the
  operator's private root `.env`, then require the complete pull-request CI
  matrix.
- Merge only green evidence, tag `v1.2.3-rc.4`, and monitor every release job.
  Accept the candidate only after GitHub Release assets/checksums, anonymous
  GHCR access, package acceptance, and the pull-only product gate pass. Clean
  Ubuntu/XOA, Nginx Proxy Manager browser access, reboot, and OME restart/media
  recovery remain separate promotion evidence.

## Current scope - branch hygiene and release CI consolidation (2026-07-26)

- Reduce the remote branch inventory without risking active or unincorporated
  work. Delete only branches whose tips are ancestors of `origin/main`; retain
  every non-ancestor branch until its pull-request history and remaining diff
  are classified separately.
- Keep `ci.yml` as the only automatic pull-request/main orchestrator. Make the
  targeted manual workflows reusable, then have CI call those definitions
  instead of maintaining inline copies for viewer, image scan, shell, docs,
  monitoring, workflow-policy, wizard, and quickstart-entrypoint checks.
- Keep intentionally distinct heavyweight paths separate: the unified Ubuntu
  `test-all` gate owns changed-path integration/Compose work, the production
  golden-path workflow owns full media acceptance, and the standalone Go gate
  owns manual full-matrix/govulncheck coverage.
- Make the tag workflow call the reusable Postgres workflow rather than owning a
  second service definition. Keep host Go verification offline, but restore the
  public Go proxy/checksum database only for the release verification step so
  clean Compose image builds can resolve `go.production.mod`.
- Centralize runtime setup in the local setup actions without a hidden second
  checkout. Every workflow remains responsible for one explicit, SHA-pinned
  checkout before invoking a setup action.
- The reusable quickstart entrypoint matrix must invoke the shared Go setup
  after checkout. `quickstart.ps1 -ValidateOnly` compiles and runs the real CLI,
  so accepting whatever Go version happens to be preinstalled on each runner
  makes the Windows/macOS results nondeterministic. The optional full Compose
  smoke job must use the same setup before invoking its source-driven wrapper.
- Do not create `v1.2.3-rc.3` until focused workflow regressions, the complete
  local gate, the pull-request matrix, and a post-merge targeted workflow run
  are green. Failed `rc.1` and `rc.2` tags remain immutable.

### Audit evidence and assumptions

- The fetched remote contains exactly 1,000 branches: 943 non-default branch
  tips are ancestors of `origin/main`, while 57 are not. GitHub reports no open
  pull requests and no protected branches. `main` and every non-ancestor branch
  are excluded from the first cleanup pass.
- The repository registers 13 workflow files plus GitHub's Dependabot workflow.
  Only `ci.yml` automatically runs for pull requests/main; the other CI
  workflows are manual and/or reusable, and `release.yml` is tag-only.
- Proven drift exists today: `ci.yml` embeds Trivy 0.70.0 with bounded download
  retries while the standalone image workflow embeds 0.50.1 without them.
  Release also duplicates the reusable Postgres service, which already caused
  `rc.1` to diverge, and both setup composite actions perform a second checkout
  using a different action pin than their callers.
- The first consolidation PR run proved GitHub accepts the reusable-call graph
  and executes called docs/policy/image jobs, but showed that a reusable
  workflow edit did not select its own path-gated CI job. Each reusable workflow
  and setup action must therefore be an explicit input to the checks it owns.
- Once selected, the dormant monitoring job exposed a stale container command:
  the pinned Prometheus/Alertmanager images use server binaries as their
  entrypoints, so passing `promtool`/`amtool` after the image makes the server
  parse them as invalid arguments. Select `/bin/promtool` and `/bin/amtool`
  explicitly and prove both real pinned images before accepting the job.
- Prometheus config validation must also represent the runtime file contract
  without using a real metrics credential. Create a private validation-only
  token in the existing temporary directory, mount it read-only only for
  container `promtool check config`, rewrite only the two runtime file paths in
  a temporary config for native `promtool`, and remove all fixtures through the
  existing exit trap. Mount the config, rules, and token as separate read-only
  files at their real runtime paths; a read-only parent-directory mount cannot
  accept a nested token mount and does not reproduce the Compose rules path.
  On Docker Desktop through Git Bash, normalize only bind sources with
  `cygpath` and disable automatic argument conversion for each `docker run`;
  broad `/etc/...` exclusions also suppress conversion of the containing mount
  argument and can silently validate the image-default config instead.
- Keep the rendered Alertmanager config mode 0600. Its container validator must
  run as the invoking host UID/GID so the bind remains readable without making
  a potentially credential-bearing render group- or world-readable.
- Export the renderer's six fallback webhook variables after assigning them.
  Both supported substitution engines consume the process environment, so
  unexported shell defaults otherwise become empty URLs/tokens and produce an
  invalid clean-runner config.
- Monitoring's final Compose overlay render must use
  `deploy/.env.example` explicitly. Clean GitHub runners do not have the
  operator-owned root `.env`, so relying on Compose's implicit env discovery
  makes the reusable validation nondeterministic and cannot prove the
  repository-owned contract.
- Because service-level `env_file: ../.env` is resolved independently from
  Compose's interpolation `--env-file`, create a mode-0600 root validation copy
  from `deploy/.env.example` only when `.env` is absent. Track ownership and
  remove only that validator-created file on every exit; never rewrite or
  delete an existing operator `.env`.
- The `rc.2` release failure is bounded to the `go-tests` verification step:
  job-level `GOPROXY=off` is inherited by Compose build arguments, while
  `verify.sh` already applies offline variables directly to host Go tests.
- Post-merge standalone Go run `30215317804` exposed the same network-scope
  leak in `go-unit-tests.yml`: its Ubuntu `verify.sh` step inherits job-level
  offline settings into clean Docker builds. Restore the public proxy/checksum
  database only for that verification step, matching the repaired release
  workflow while leaving direct host Go steps offline.
- The same run passed macOS Go tests, then failed the generated Helm asset
  drift check because `deploy/helm/bitriver-live/files/srs.conf` predates the
  canonical forwarding-boundary comments in `deploy/srs/conf/srs.conf`.
  Regenerate through `scripts/sync-helm-deploy-assets.sh`; do not hand-edit the
  generated Helm copy or change canonical SRS behavior.
- Merged-main standalone run `30215961551` proved the network repair on Ubuntu
  and the regenerated asset on macOS, but Windows still reports SRS drift.
  Both SRS files currently have unspecified Git EOL attributes; on Windows the
  checked-out generated copy becomes CRLF throughout, while the sync script
  builds its generated header with LF before appending the checked-out
  canonical content. Pin both the canonical and generated SRS paths to LF in
  `.gitattributes` and add a regression test for that repository invariant.
  Do not weaken byte-for-byte generated-asset validation.
- Explicit PR-head run `30216357352` proves that LF fix on Windows, then exposes
  a later independent failure: Git Bash writes POSIX-style `/d/...` artifact
  paths into the govulncheck scan index and passes the same form to native
  Windows Python. `pathlib` interprets those as `\d\...` on the current drive.
  Convert Bash paths at the Python boundary with `cygpath -w` when available,
  including scan-index entries and every Python input/output argument; preserve
  unchanged POSIX paths elsewhere and keep the vulnerability policy blocking.
- User-owned untracked deployment notes/helpers/data and the private root
  `.env` remain outside this change and must not be staged, rewritten, or
  included in branch-cleanup evidence.

### Risks

- Squash merges do not make a historical head an ancestor of `main`. This is why
  the 57 non-ancestor branches are retained during the ancestry-safe cleanup
  even when their work may already be represented on `main`.
- Remote deletion of all 943 ancestry-merged branches is intentionally pending
  a separate explicit confirmation after the execution safety gate rejected the
  broad mutation. No branch was deleted; CI/release work can proceed
  independently from the preserved cleanup classification.
- Reusable-workflow calls change the displayed check hierarchy. There are no
  protected-branch required-check rules today, but the complete PR run must
  prove all path-gated jobs still execute or skip as intended before merge.
- A called workflow cannot elevate caller permissions. The image scan caller
  must explicitly retain `contents: read` and `packages: read`.
- Consolidation must not make CI repeat the same Docker lifecycle. The CI call
  to quickstart smoke will run entrypoint checks only because the unified Ubuntu
  gate already owns Compose smoke for relevant changes.
- A green source/CI candidate still does not prove clean Ubuntu/XOA install,
  Nginx Proxy Manager browser access, reboot recovery, or OME recovery. Those
  remain stable-promotion evidence after tagged artifacts exist.

### Test and publication plan

- Add workflow-contract tests requiring CI reusable calls, one checkout per
  workflow path, release reuse of Postgres, explicit migrations in the reusable
  service, and release verification proxy/checksum restoration.
- Update existing image/viewer/release tests to inspect the single source of
  truth instead of requiring duplicated commands in `ci.yml`.
- Parse every workflow/action YAML file, run `check-ci-contract.sh`,
  `check-go-workflow-config.sh`, focused Go script tests, shell syntax, and
  `git diff --check`.
- Run literal `./scripts/verify.sh --viewer` while preserving/restoring the
  operator's private root `.env`, then push a small PR and require the complete
  remote CI result.
- After merge, manually dispatch the reusable Postgres and other release-relevant
  targeted gates as needed. Only then tag `v1.2.3-rc.3`, monitor every release
  job, inspect checksums/packages, prove anonymous GHCR pulls, and run the
  Docker Desktop pull-only product gate before clean Ubuntu/XOA acceptance.

## Current scope - first public release-candidate publication gate (#1297, 2026-07-24)

- Make the tag-triggered release workflow runnable from this public repository without preloading deployment credentials. Generate strong job-local validation credentials, retain only sentinel-scanned status evidence, and keep real operator secrets entirely outside GitHub Actions.
- Publish first-party images to the repository owner's real GHCR namespace, exposed through one deployment-contract variable so official installs default to `ghcr.io/prohibitedtv` while forks and mirrors can override it.
- Validate release tags as SemVer, normalize package/MSI versions separately from the human tag, mark hyphenated tags as GitHub prereleases, and prevent prerelease tags from moving `latest`.
- After all five tagged images publish, run the canonical Compose stack in production/pull mode and execute the same 1080p media/API golden path. The GitHub Release job must depend on this scanner-approved pull-only evidence.
- Repair the Windows MSI staging/version seam so the release matrix cannot be blocked by paths that disagree with the canonical release asset manifest.
- Only after the workflow change is merged and its PR gates pass, create the first immutable `v1.2.3-rc.1` tag. Treat it as a public candidate for clean Ubuntu/XOA acceptance, not as the stable v1.2.3 announcement.

### Release-candidate design

- Add a deterministic release-env preparation helper that copies `deploy/.env.example`, replaces sample credentials with cryptographically random job-local values, applies the exact release tag/official namespace, resolves current third-party registry digests, and writes a separate temporary sentinel file. It must never print values.
- Extend the quickstart smoke through explicit `BITRIVER_SMOKE_*` controls. Existing local/CI callers keep build/development defaults; the release job supplies an external env file and selects pull/production without rewriting a checkout-owned `.env`.
- In pull mode, skip every Compose build, pull all rendered image references, enforce production dependency digests, render OME with the tagged helper image, run the existing service checks, then run `test-production-golden-path.sh --stack running`.
- Upload only `production-golden-path.json` after the existing evidence scan. Raw Compose logs, generated OME/SRS files, env files, cookies, stream keys, and registry credentials remain runner-local and are removed on every exit path.
- Derive `release_version` and `prerelease` once from the tag. Use the normalized numeric core for MSI, SemVer components for Linux packaging, the original tag for filenames/image tags, and the prerelease flag for GitHub Release metadata.
- Stage the Windows launcher from `deploy/install/release-assets.txt`, and make WiX source paths match the staged `share/bitriver-live` layout rather than maintaining a second incomplete asset list.

### Release-candidate risks

- `ghcr.io/bitriver-live` is not an owned GitHub account namespace, while the repository owner is `ProhibitedTV`; the current workflow cannot publish the references Compose names. Changing the official default is a deployment-contract change and must update Compose, env, CLI preflight, Helm/docs, and generated contract evidence together.
- GHCR packages may require a one-time public-visibility action after their first push. The workflow must prove anonymous manifest access before creating a GitHub Release; if visibility cannot be changed with the repository token, stop with the exact external action rather than publishing unusable assets.
- Tag workflows publish immutable external state. A failed `v1.2.3-rc.1` is never force-moved or overwritten; corrections use `rc.2`.
- The immutable `v1.2.3-rc.1` run reached release validation but stopped
  before artifact/image publication because its fresh Postgres service database
  was not migrated. `test-postgres.sh` intentionally requires
  `BITRIVER_TEST_POSTGRES_RUN_MIGRATIONS=1` for an externally supplied DSN;
  both the tagged and reusable Postgres workflows must opt in explicitly.
- The immutable `v1.2.3-rc.2` run crossed the Postgres gate, then stopped
  before builds because the release Go job's host-only `GOPROXY=off` policy
  leaked through Compose into clean Docker builds. Keep host Go tests offline,
  but give the release verification step the real upstream proxy/checksum
  settings already used by artifact builders.
- Multi-architecture image publication is slower than registry index propagation. Use bounded manifest retries before the pull-only gate, not unbounded sleeps.
- Production mode currently requires third-party digest pins but not first-party pins. Resolve and record first-party manifest digests in candidate evidence/release notes; do not claim digest-pinned clean-host proof from tag-only pulls.
- The existing Windows workflow passes `v...` directly to WiX and stages files under paths WiX does not read. Static checks are insufficient; the remote Windows MSI job remains required before candidate publication.
- The Jul 24 GitHub Advisory Database update for
  `GHSA-mh99-v99m-4gvg` marks every `brace-expansion` release through 5.0.7
  vulnerable to attacker-controlled memory exhaustion and names 5.0.8 as the
  patched release. Viewer CI installs older transitive majors through
  ESLint/Jest tooling, and ordinary `npm audit fix` cannot update those parent
  ranges without a breaking ESLint major. Use one explicit npm override to
  5.0.8 only if clean `npm ci`, lint, unit, browser, build, and audit all pass;
  do not accept `--force` or suppress the advisory.
- GitHub-hosted pull-only proof still is not a clean Ubuntu/XOA install, Nginx Proxy Manager/browser test, or host reboot. Those remain explicit #1297/#1304 promotion gates.

### Release-candidate test plan

- Unit-test tag parsing, prerelease/latest behavior, env replacement, secret uniqueness, sentinel separation, digest formatting, no-value output, and failure on malformed tags or unresolved images.
- Add workflow-contract tests requiring job-local credentials, the official/overridable GHCR namespace, the post-publish pull-only product job, scanner-approved artifact upload, stable-only `latest`, prerelease metadata, and release-job dependency ordering.
- Add a workflow-contract regression requiring every CI-owned fresh Postgres
  service DSN to opt into repository migrations, then run the real
  Postgres-tagged suite against a disposable service database before `rc.2`.
- Add a release-workflow regression requiring the verification step to restore
  production dependency network settings before Docker builds while
  `verify.sh` continues to force host Go tests offline.
- Add quickstart regression tests proving build/development remains the default and pull/production performs no build while enforcing the supplied external env/digest contract.
- Run shell syntax/ShellCheck, generated contract checks, focused Go/Python tests, Compose rendering in both build and pull shapes, release-bundle/package tests, and `git diff --check`.
- Run `./scripts/verify.sh --viewer` and the full PR matrix. After merge, tag the RC, monitor every release job, inspect/download the published assets/checksums, verify anonymous GHCR access and image digests, and run a Docker Desktop pull-only golden path before handing the candidate to the clean Ubuntu/XOA gate.
- For `GHSA-mh99-v99m-4gvg`, require the lock graph to contain only
  `brace-expansion@5.0.8`, `npm audit --audit-level=high` to report zero
  vulnerabilities, and the complete viewer lint/unit/Playwright/build sequence
  to pass after a clean install. Build the real viewer container too, because
  its dependency stage must copy the local compatibility hook before `npm ci`.
  Keep the override registry-backed so nested consumers receive ordinary
  package copies and `npm ls` reports a valid dependency graph.

## Current scope - production golden-path E2E (#1300, 2026-07-24)

- Replace the misleading ingest "E2E" boundary, which currently runs only a storage package test, with one reusable acceptance harness against the real canonical Compose stack: Postgres, Redis, SRS, controller, transcoder, OvenMediaEngine, API, and viewer.
- Generate deterministic 1080p video plus audio at runtime; publish it over the creator-facing RTMP path; require the channel to transition live and back offline; and prove both OME LL-HLS and transcoder HLS are decodable and advancing rather than checking only HTTP status.
- Exercise real self-signup/session cookies, first-channel creator bootstrap, chat send/history, an owner moderation action, multipart VOD upload/transcode/publication, viewer metadata, and health/readiness/status surfaces.
- Emit a versioned machine-readable stage report, media probe evidence, endpoint summaries, timing, and failure context without retaining credentials. Scan retained evidence against per-run sentinel values before accepting it.
- Make the live-stack tier reusable from source quickstart, release workflows, and an already-running clean-host installation. Keep the cheap storage integration test separate so local unit/integration commands do not accidentally claim production coverage.

### Golden-path design

- Implement the product exercise as a standard-library Python harness plus a small shell entrypoint. The harness talks only through public HTTP/RTMP surfaces and invokes host `ffmpeg`/`ffprobe`; it must not reach directly into Postgres or mutate containers to manufacture success.
- The shell entrypoint supports two modes: `--stack running` validates an already-running deployment, while `--stack quickstart` delegates lifecycle to the canonical quickstart smoke and runs the same product assertions before teardown.
- Test credentials and stream keys exist only in process memory or a temporary sentinel file outside the evidence directory. Reports store stable labels, IDs, status, durations, URLs with query/userinfo removed, and redacted command descriptions.
- A phase fails with a bounded, stage-specific error. The wrapper then captures Compose state and selected recent logs through the existing redaction/scanning boundary; success and failure both leave a report that names the first failed stage.
- Keep the first PR tier build-based for deterministic source validation. Tagged RC/stable promotion must call the same running-stack harness after pull-only immutable images are installed; build-mode success alone is not publication approval.

### Golden-path risks

- LL-HLS manifests can exist before media is decodable. Require a media probe and advancing segment/timestamp evidence, not a single successful manifest GET.
- RTMP publication is asynchronous across SRS callbacks, OME forwarding, and transcoder startup. Use explicit per-stage deadlines with last-observed state; do not use unbounded sleeps or retries that hide hangs.
- The first real VOD run exposed that the runtime passed `storeUseCases` into the upload-processing adapter even though that facade does not preserve the repository's upload-recording method. A successful transcode was therefore followed by `upload recording store unavailable`; the unbudgeted persistence retry re-enqueued the whole upload and submitted thousands of duplicate jobs. Wire the concrete repository through a compile-time-complete narrow adapter, retry persistence operations in place with a fixed budget, and never resubmit an accepted transcode merely because recording/update persistence failed.
- The next VOD run exposed two additional contract gaps hidden by handler-only tests: global API authentication rejected the signed `GET /api/uploads/{id}/media` request before its constant-time media-token check, and Compose did not pass the supported `BITRIVER_TRANSCODE_LADDER` setting into the API container. Exempt only the exact signed-media GET route from session auth, keep all neighboring upload routes protected, and add the ladder variable to Compose/env/documentation together.
- Upload FFmpeg failures were also silent because the shared launcher selected log context only from the live-job map. Resolve live or upload metadata under the server lock before constructing the process logger so a failed VOD job remains diagnosable without placing signed source URLs in retained release evidence.
- The authenticated source then proved that `POST /v1/uploads` is asynchronous while `UploadProcessor` treated acceptance as completion: it marked the upload ready and deleted the source before FFmpeg opened it, producing a 404. Add an authenticated upload-job status resource, persist success/failure state, and make the HTTP ingest adapter wait under the processor's existing bounded context. Source cleanup and public readiness may occur only after FFmpeg plus publication complete.
- The final operator probe identified a Docker Desktop/Git Bash harness distortion rather than unhealthy services: MSYS rewrote the exported `/healthz` value to `C:/Program Files/Git/healthz` before Compose passed it into the Linux API container. Disable only MSYS environment conversion for native Docker invocations in the Windows smoke path; retain argument conversion so temporary Compose paths still resolve, then require the aggregate status to pass against the unmodified container endpoint.
- The first Ubuntu CI run exposed an ownership conflict in the new isolated media volume: the image creates `/work` for UID 10001, but the legacy Linux smoke override forced the host runner UID 1001, so the transcoder failed on `mkdir /work/live: permission denied`. Keep host UID overrides only for services that write bind-mounted checkout paths; let the transcoder use its image user with the named volume. Failure diagnostics must also report only sanitized container state rather than dumping `docker inspect` configuration and environment values.
- After Linux quickstart passed, the tagged Postgres tier exposed stale test-only ingest-stub usage: `postgres_ingest_e2e_test.go` still supplied the removed `PlaybackURL` option and expected the superseded OME application create/delete sequence. Align the Postgres repository scenario with the current `OMEPlaybackBaseURL` and application-validation contract, then run the actual `-tags postgres` suite before relying on the remote rerun.
- Multipart VOD source URLs generated from a host request must also be reachable from the transcoder container. Set the request Host to the canonical internal API origin while connecting through the published host port, then verify the returned playback URL externally.
- Live and VOD transcoding are CPU-heavy on hosted runners and Docker Desktop. Use a deterministic short fixture, a release-grade timeout, and measured phase durations; do not lower the 1080p content assertion.
- Raw Compose logs and generated configuration can contain operator credentials. Never put `.env`, generated OME/SRS config, request cookies, authorization headers, or unredacted command lines in the evidence directory.
- Browser playback proof may need a separate Playwright phase after the media/API harness is stable. Do not claim browser recovery/quality behavior from FFprobe evidence alone.
- The current workflow may invoke quickstart and the legacy ingest test in one job. Rewire it so the expensive real-stack path runs once, while the cheap storage test remains independently callable.

### Golden-path test plan

- Add static/unit coverage for report redaction, URL sanitization, timeout/failure stage reporting, deterministic fixture/probe parsing, and workflow wiring.
- Add a runtime-wiring regression plus processor tests that force recording and ready-state persistence failures; require a bounded terminal state and exactly one ingest submission.
- Run the cheap storage ingest integration test separately and prove its name/docs no longer describe it as the canonical product E2E.
- Run the new harness against Docker Desktop from a clean Compose teardown, require real 1080p RTMP publish, SRS live state, OME and transcoder playback probes, chat/moderation, VOD publication/playback, health surfaces, offline transition, evidence scan, and clean teardown.
- Deliberately break at least one dependency input in a focused test and require a failure at the named stage with no secret echoed.
- Run `./scripts/verify.sh`, viewer checks where browser evidence changes, Compose config validation, the upgraded ingest workflow contract tests, and required remote CI before merge.
- Leave tagged pull-only Ubuntu/XOA repetition, repeated-run flake measurements, and browser player recovery/quality evidence explicitly pending until their direct runs exist.

## Scope
- Advance production blocker #1297 with a clean-host Linux Compose installer that consumes release artifacts only and targets Ubuntu 24.04 LTS x86_64 first.
- Make launcher archives plus `.deb`/`.rpm` packages self-contained for the canonical pull-only stack: CLI/wrapper, Compose/env contract, migrations, renderers/templates, proxy config, systemd integration, and operator docs.
- Install immutable program assets separately from operator-owned configuration and data; provide idempotent install/status/log/upgrade entrypoints plus a safe uninstall that retains data unless destruction is explicitly requested.
- Publish the OME config helper as a multi-architecture release image so a source-free host never needs `ome-config:local` or a Go toolchain.
- Add repeatable artifact-only acceptance that proves package contents, non-root/sudo operation, paths containing spaces, restart semantics, diagnostics, and data-preserving uninstall.
- Document the XOA/XCP-ng VM and Nginx Proxy Manager topology, including public HTTP(S), WebSocket forwarding, trusted proxies, media/firewall ports, and internal-only control services.

## Main reconciliation scope (2026-07-24)
- Reconcile the installer foundation with current `main`, including merged PR #1326 and its proven SRS callback, public RTMP/LL-HLS, same-origin `/live/`, OME application, transcoder, Windows evaluation, and README contracts.
- Resolve workflow and release-asset conflicts by composing both requirements: the Ubuntu artifacts must include every new required media URL, and image scans/quickstart fixtures must continue to render the canonical contract.
- Keep the public documentation truthful before the first tag: installer/package code may be release-ready while GitHub Releases, `.deb`, `.rpm`, and launcher downloads remain unavailable.
- Re-run the installer lifecycle, release bundle/package checks, canonical Compose render, full repository verification, and remote PR gates on the reconciled head before marking #1325 ready.

## Design Decisions
- The production install root is `/opt/bitriver-live`; configuration lives under `/etc/bitriver-live`; durable application/transcoder data lives under `/var/lib/bitriver-live`. Release assets remain replaceable and operator data remains outside package ownership.
- A single systemd unit wraps the canonical Docker Compose stack. Docker retains container restart behavior; systemd provides boot ordering, status, bounded startup, and an operator-visible failure boundary.
- The installer stages but does not silently weaken or bypass production validation. First activation uses the existing `bitriver env init --wizard`, `doctor`, `env validate`, pull preflight, migration runner, quickstart, and health checks.
- Linux packages install the same bundle layout as the launcher archive. Package installation may create directories and the disabled unit, but it must not start with sample credentials.
- `bitriver-ome-config` is a version-matched GHCR image for `linux/amd64` and `linux/arm64`; Compose and image preflight use its release tag/digest just like other first-party services.
- Ubuntu 24.04 amd64 is the declared production target for this change. Debian 12 and Linux arm64 remain provisional until their clean-host evidence passes; release docs must not overclaim them.
- Installer completion means files and service integration are installed. Production readiness additionally requires successful quickstart, OME process health, authenticated OME control-plane access, aggregate API health, and the basic ingest/playback acceptance owned by #1300.

## Assumptions
- Docker Engine and the Compose v2 plugin are installed from Docker's supported repository, or the installer may install them only after explicit operator confirmation.
- The VM has at least 4 vCPU, 8 GiB RAM, 20 GiB free disk, working DNS, and outbound access to GHCR plus pinned third-party registries.
- Nginx Proxy Manager terminates TLS on a separate trusted host or VM and proxies the viewer/API HTTP origin to the BitRiver VM. RTMP, LL-HLS/WebRTC, TURN/relay, and ICE traffic are forwarded directly rather than tunneled through the HTTP proxy host.
- The tagged release publishes all first-party multi-architecture images before clean-host acceptance runs.

## Risks
- Existing launcher packages omit runtime assets and resolve repo-relative paths from the current directory; add an explicit installed asset root and verify the bundle outside the source checkout.
- Compose currently uses local-only OME render images in pull mode; publishing and pinning this helper changes the deployment contract and release workflow together.
- SRS/OME render jobs write generated files into the installed asset tree; use an operator-owned runtime workspace or deliberately writable generated paths without making immutable package files credential-bearing.
- A systemd timeout that is too short can kill a valid first image pull; make it bounded but generous and preserve redacted logs plus exact recovery commands.
- Nginx Proxy Manager handles HTTP/WebSocket traffic but not the full media port surface. Documentation must distinguish proxy routes from XOA/firewall/NAT rules.
- GitHub-hosted CI cannot prove an actual XOA VM reboot. Automate everything repeatable, then leave final tagged-release reboot evidence explicitly pending rather than claiming it.
- Read-only Go dependency and verification checks must stay inside first-party Go roots and must not request VCS build stamping: on Windows-mounted Linux workspaces, an implicit `git status`, `go test ./...`, or a blanket `find .` can livelock on frontend dependencies and media before test timeouts start. Set `GOFLAGS=-buildvcs=false`, scope default Go verification and models-import scans to `cmd`, `internal`, `scripts`, and `web`, and guard these contracts in tests.
- ShellCheck treats single-quoted workflow-contract patterns containing `$out_dir` or `$launcher_root` as suspicious non-expansion. Preserve the intended literal dollar signs with escaped variables in double-quoted fixed-string assertions so CI remains strict without suppressions.
- Image-scan logic is duplicated between `ci.yml` and the manual workflow; a hard-coded legacy `bitriver-live/ome-config:local` reference can diverge from the Compose-built GHCR tag and make Trivy scan a nonexistent image. Resolve the OME helper from the collected Compose image list in both workflows and reject missing or ambiguous matches.
- Compose classifies `ome-health-token-check` as non-buildable even though it shares the locally built `ome-config` image. A blanket `compose pull --ignore-buildable --policy missing` can still make a denied registry request for that sibling service. Enumerate rendered image references, retain any image already inspectable in the local daemon, and pull only genuinely absent runtime images.
- The installer branch and PR #1326 both changed release workflows, deployment variables, quickstart smoke, and operator documentation. A mechanical conflict choice could silently discard either the published OME helper or the verified media-routing contract; reconcile each overlapping file by rendered/runtime behavior, not by branch preference.
- Current `main` correctly says no tagged downloads exist. Do not replace that statement with live download commands until the release workflow has actually published a tag and assets.
- The quickstart smoke may reuse an operator's existing root `.env`, including files created before public RTMP/LL-HLS variables became required. Because the smoke already forces an isolated development/build posture and host ports, it must also supply non-secret smoke defaults for every required public media URL without rewriting the operator file.
- Current `main` merged three incompatible major development-tool bumps: TypeScript 7 is outside the latest `ts-jest` range (`typescript >=4.3 <7`), ESLint 10 is outside the Next.js-bundled lint plugins' declared ranges, and Node 26 types exceed the enforced Node 24 runtime. Restore the proven TypeScript 6.0.3, ESLint 9.39.5, and `@types/node` 24.13.3 pins until the complete toolchain supports the newer majors; do not bypass peer resolution with `--force` or `--legacy-peer-deps`.
- The official npm audit reports three high-severity runtime findings against Next 16.2.10 and its PostCSS/Sharp dependency chain. Upgrade `next` and `eslint-config-next` together to the non-major fixed 16.2.11 release and require the blocking production audit to report no high/critical findings.
- Next 16.2.11 fixes its direct advisories but still hard-pins vulnerable PostCSS 8.4.31 and allows only vulnerable Sharp 0.34.x; no newer stable Next release exists. Temporarily override those transitive packages to fixed PostCSS 8.5.22 and Sharp 0.35.3, lock the exception in the runtime-baseline test, and exercise the production image. The viewer already sets `images.unoptimized: true`, so it does not depend on Next's image optimizer; remove the override once an aligned stable Next release ships.
- The root Docker build context currently includes `deploy/transcoder-data/`; this checkout's local media made the context roughly 200 MB and could leak runtime artifacts to a builder/cache. Exclude that runtime directory explicitly and lock the boundary in a static regression.
- The reconciled CI image scan found CVE-2026-59873 in `tar@7.5.15` under the Node 24 Alpine image's global npm installation, not in the viewer application dependency tree. The production runner needs the Node executable but never invokes npm or npx; remove the package manager payload and entrypoints from the final runtime stage, retain npm in the build stages, and guard both the runtime capability and reduced attack surface in the Dockerfile baseline test.

## Test Plan
- Shell syntax, installer unit tests, and focused Go tests for installed-root discovery, image preflight, Compose invocation, and OME helper image selection.
- Assemble launcher bundles in temporary paths containing spaces and verify every Compose bind mount plus required executable/document exists without reading the checkout.
- Exercise install twice, disabled-before-configuration behavior, status/log commands, upgrade staging, service enablement, and safe uninstall/data purge using isolated filesystem roots.
- Extract `.deb` and `.rpm` payloads and compare them to the canonical bundle manifest; run package install checks in Ubuntu 24.04 and Debian 12 containers where possible.
- Render Compose in pull-only mode, run the existing quickstart smoke with the release-shaped bundle, and require bounded OME/aggregate health diagnostics.
- Run `./scripts/verify.sh`, release workflow contract tests, `git diff --check`, and pull-request CI before publication.
- Re-run default Go verification, architecture, and models-import checks in the pinned Linux toolchain; assert VCS stamping is disabled and package/filesystem traversal is limited to first-party Go roots so mounted-workspace verification remains bounded.
- Run ShellCheck on the corrected release-bundle assertion and rerun its focused test before pushing the CI repair.
- Add a workflow regression guard for Compose-derived OME scan selection, parse both workflow files, and reproduce the image-selection logic against the rendered CI Compose image list before republishing.
- Add a quickstart regression guard against blanket non-buildable pulls, run shell syntax plus the full quickstart smoke locally, and require the unified Ubuntu gate to pass on the same corrected path.
- After merging current `main`, run focused Go workflow/contract tests, `bash -n` on changed shell scripts, the release bundle and Compose-host lifecycle tests, `docker compose ... config --quiet` with a generated production-shaped env, `scripts/test-quickstart.sh`, and `./scripts/verify.sh`.
- Inspect the staged reconciliation for root `.env`, generated OME/SRS output, runtime data, secrets, and the user's unrelated untracked deployment helpers before publication.
- Run the quickstart smoke with a pre-existing env that omits the public RTMP/LL-HLS variables and require Compose interpolation to proceed through the runtime gate.
- Run a clean `npm ci`, viewer lint/test/build, and the viewer Docker build after restoring the supported Node 24 / TypeScript 6 / ESLint 9 development baseline; require the clean install to avoid peer-override warnings.
- Run `npm audit --omit=dev --audit-level=high` after the Next.js patch and record any remaining high/critical production finding as release-blocking.
- Start the clean production viewer image after the security overrides and require its public route to return successfully; do not accept a zero-count audit if the resulting image cannot boot.
- Rebuild the root OME/helper context after updating `.dockerignore` and confirm local transcoder output is neither transferred nor packaged.
- Rebuild the production viewer image, confirm `node` and the Next standalone server remain runnable while npm/npx and `/usr/local/lib/node_modules/npm` are absent, then rerun the blocking image scan and full remote CI.

## Boundaries
- The user explicitly authorized installer, deployment-contract, and roadmap work for the Ubuntu/XOA/Nginx Proxy Manager target, including the necessary release workflow changes.
- Do not edit root `.env`, stage generated OME credentials/config, or include the user's untracked deployment helper files/runtime data.
- Do not expose PostgreSQL, Redis, SRS control, OME Managers API, or transcoder control ports through Nginx Proxy Manager.
- Do not claim Debian, ARM64, host reboot, or real playback acceptance until direct evidence exists.
- Do not automatically delete operator configuration, database volumes, recordings, or transcoder data during package removal.

## Completion
- Local implementation and acceptance are complete: pinned-toolchain repository verification, package generation, PostgreSQL migration acceptance, release-shaped Compose/OME smoke, and clean teardown passed.
- Draft PR #1325 is published. Its ShellCheck failure, stale local-only OME Trivy target, and sibling-service registry pull were repaired with focused Linux bundle/workflow/Go checks plus a full OME/API/viewer host smoke and clean teardown passing. Required CI is green on implementation head `de869492`, including the Ubuntu test-all gate, image vulnerability scan, viewer integration, cross-platform Go, and entrypoint checks.
- The installer candidate is reconciled with current `main` on head `e3f96cb5`. CI run 30102433565 is green across the unified Ubuntu gate, first-party blocking image scan, viewer integration/build/audit, secret/docs/shell/workflow guards, cross-platform Go, and quickstart entrypoint matrix.
- Tagged Ubuntu/XOA reboot, authenticated OME control-plane access, and real ingest/playback/recovery remain required external release evidence for #1297/#1300/#1304; this local candidate does not claim them.
