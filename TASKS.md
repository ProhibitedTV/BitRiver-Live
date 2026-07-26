# TASKS

## Scoped change: branch hygiene and release CI consolidation

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Audit remote branches and the Actions graph
  - Acceptance criteria:
    - `PLAN.md` records the branch classes, workflow ownership, proven drift,
      release failure cause, risks, and test plan before implementation.
    - Open PRs/protected branches and recent CI/release runs are inspected
      read-only.
    - Untracked operator files, private configuration, and unproved deployment
      claims remain explicit boundaries.
  - Check:
    - Fetched/pruned remote refs contain 1,000 branches: 943 non-default tips are
      merged by ancestry into `origin/main`, and 57 are not.
    - GitHub reports zero open PRs and zero protected branches; the first cleanup
      pass excludes `main` and all 57 non-ancestor branches.
    - Thirteen repository workflow files plus Dependabot are active. `ci.yml` is
      the only automatic PR/main CI orchestrator; the remaining test workflows
      are manual/reusable and release is tag-only.
    - Confirmed duplicate/drift seams include inline CI/manual image scans on
      different Trivy versions, release/reusable Postgres services, and setup
      composites that repeat checkout with different action pins.
    - Release run `30212555952` passed environment and Postgres gates, then
      failed before any build/publication because `GOPROXY=off` leaked from the
      Go job into clean Compose builds.

- [x] Task 2 - Prepare the ancestry-safe remote branch cleanup
  - Acceptance criteria:
    - Classify the deletion set strictly by ancestry to `origin/main`.
    - Preserve `main`, tags, and all non-ancestor branches in the proposed
      operation.
    - Do not execute the broad remote mutation without the separate explicit
      confirmation required by the execution safety gate.
  - Check:
    - The proposed command rechecks all 943 tips immediately before bounded
      deletion batches and aborts on classification change.
    - The execution safety gate rejected mass deletion without a separate
      explicit user confirmation. No remote branch was deleted; the 1,000/943/57
      inventory remains current for the next approved cleanup pass.

- [x] Task 3 - Consolidate reusable CI and setup ownership
  - Acceptance criteria:
    - CI calls reusable single-source workflows for duplicated checks while
      retaining path filters and permissions.
    - Setup composites no longer perform hidden second checkouts.
    - Intentionally distinct full-stack/release gates remain separate and
      documented.
  - Check:
    - `ci.yml` now delegates viewer, image scan, shell, docs, monitoring,
      Go-workflow policy, wizard, and quickstart entrypoint checks to their
      reusable/manual workflow definitions instead of embedding copies.
    - Manual quickstart dispatch retains the full Compose smoke by default; CI
      passes `run_compose_smoke: false` because the unified Ubuntu gate owns the
      same changed-path Docker lifecycle.
    - The single image-scan definition now pins Trivy 0.70.0 and uses bounded,
      fail-closed download validation; the stale 0.50.1 implementation is gone.
    - Setup Go/Node composite actions no longer perform a second checkout or
      accept checkout-depth inputs. The Go action now shares the release's
      pinned setup-go v7.0.0 revision; the full-history manual Go checkout moved
      to its explicit workflow checkout step.
    - Added CI ownership regressions and updated viewer/image workflow tests plus
      testing docs for the reusable model. `go test ./scripts`, CI contract,
      Go-workflow convention, 15-file YAML parsing, and `git diff --check`
      passed.
    - PR run `30213569005` proved GitHub accepts the reusable-call syntax and
      started called docs/policy/image jobs, but reusable viewer/shell/
      quickstart/monitoring/wizard jobs skipped because their own workflow files
      were absent from the corresponding path filters. The follow-up adds every
      reusable workflow/setup action to its owner filter and a regression that
      requires both the filter and call reference.
    - The focused path-filter regression, CI contract, YAML parsing, and diff
      checks passed after the correction; a replacement PR run must exercise the
      complete reusable set before merge.
    - Replacement run `30213641760` selected the full reusable set and exposed a
      dormant monitoring failure: the pinned images launched `prometheus
      promtool`/`alertmanager amtool` instead of the requested CLI. The focused
      fix selects `/bin/promtool` and `/bin/amtool` explicitly and preserves Git
      Bash volume-path conversion.
    - Direct pinned-image proof then showed Prometheus config validation also
      lacked the runtime-mounted metrics token file. Validation now creates a
      mode-0600, non-secret temp token and mounts it read-only; repository and
      log outputs retain no credential.
    - The same proof caught a nested bind-mount conflict and missing runtime
      rules path. Config, rules, and token are now mounted as separate
      read-only files at the exact Compose paths; pinned Prometheus 2.51.2
      accepted the config, discovered one rule file, and validated all nine
      rules.
    - An exact Git Bash plus Docker Desktop run also caught broad MSYS path
      exclusions silently selecting the image-default config. Explicit
      `cygpath` source normalization plus `--mount` now validates the repository
      config and all nine rules through the Windows audience's real shell path.
    - Monitoring Compose validation now supplies `deploy/.env.example`
      explicitly so clean runners do not depend on an operator-owned root
      `.env`. The real base-plus-monitoring overlay rendered successfully.
    - The complete `./scripts` Go suite, Bash syntax, CI contract, Go-workflow
      policy, all workflow/action YAML parsing, and `git diff --check` passed.
      The replacement PR run remains responsible for the exact pinned
      Alertmanager container validation after the local mount safety gate
      declined that read-only fixture mount.
    - Replacement run `30214144580` passed the corrected Prometheus config and
      nine-rule validation, then showed `amtool` could not read the intentionally
      mode-0600 render as the image's default UID. The container now runs as the
      invoking host UID/GID so validation retains the private file mode.
    - Run `30214233612` proved the file is readable, then exposed empty fallback
      webhook values: the renderer assigned defaults without exporting them to
      `envsubst` or Python. All six defaults are now exported, and a functional
      regression renders from an absent env file and requires every non-empty
      example URL/token.

- [x] Task 4 - Repair and deduplicate release preflight
  - Acceptance criteria:
    - Release calls the reusable migrated Postgres gate.
    - Release verification restores dependency network settings for Compose
      builds while host Go tests remain offline.
    - Focused regressions reject either drift.
  - Check:
    - `release.yml` now calls `./.github/workflows/postgres-tests.yml`; the
      second Postgres 15 service/wait/test implementation was removed while all
      downstream release jobs retain their `postgres-tests` dependency.
    - The reusable service remains permission-minimal and explicitly sets
      `BITRIVER_TEST_POSTGRES_RUN_MIGRATIONS=1` for its fresh supplied DSN.
    - The release `Run verification gate` step restores
      `GOPROXY=https://proxy.golang.org,direct` and `GOSUMDB=sum.golang.org` for
      clean Compose builds. Its job keeps offline defaults, and `verify.sh`
      continues to override host Go tests with `GOPROXY=off GOSUMDB=off`.
    - All six release Go setup steps now call the shared pinned local setup
      action instead of maintaining a second action revision/configuration.
    - Focused release/Postgres/network/setup regressions, CI policy, Go workflow
      policy, and all workflow/action YAML parsing passed.

- [-] Task 5 - Verify locally and through GitHub
  - Acceptance criteria:
    - Workflow/action parsing, policy checks, focused tests, and full
      `./scripts/verify.sh --viewer` pass.
    - A reviewable PR contains only intended files and full remote CI passes.
    - Post-merge targeted workflow proof passes before tagging.
  - Check:
    - Literal `./scripts/verify.sh --viewer` passed with pinned Go 1.26.5 and
      bundled Python: release bundle, all first-party Go/script tests,
      architecture/dependency/contract checks, real Postgres migration
      lifecycle, Compose rendering, and Docker quickstart all passed.
    - The quickstart rebuilt the release-sensitive production module graph,
      brought Postgres/Redis/SRS/controller/transcoder/OME/API/viewer healthy,
      completed migrations, and passed API/viewer probes before clean teardown.
    - Viewer lint plus 26 Jest suites/217 tests/four snapshots passed.
    - The private root `.env` was parked only for verification, restored in
      `finally`, and matched its original SHA-256 hash.
    - PR and post-merge targeted workflow evidence remain pending.

- [ ] Task 6 - Publish and inspect `v1.2.3-rc.3`
  - Acceptance criteria:
    - The immutable tag points at the verified merged commit.
    - Release jobs, packages, checksums, anonymous image pulls, and pull-only
      Docker Desktop product evidence pass.
    - Remaining clean Ubuntu/XOA/NPM/browser/reboot/OME recovery evidence is
      stated without overclaiming.
  - Check:
    - Pending.

## Scoped change: first public release-candidate publication gate (#1297)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Audit the actual tag-to-publication path
  - Acceptance criteria:
    - `PLAN.md` records the publication scope, assumptions, evidence boundary, risks, and test plan before implementation.
    - Remote release/tag/package inventory, Actions secret-name readiness, GHCR ownership, tag metadata, image publication, product-gate ordering, and installer inputs are inspected read-only.
    - The planned candidate does not overclaim clean-host Ubuntu/XOA, browser, reboot, or stable-release evidence.
  - Check:
    - GitHub currently has no Releases or remote tags, and the repository has zero configured Actions secret names; the existing `verify-env` job therefore cannot succeed.
    - The workflow labels credentials `job-local-ephemeral` while reading every value from repository secrets, never runs the production golden path after images publish, treats every `v*` tag as a normal release, and moves `latest` for prereleases.
    - Compose and release automation target `ghcr.io/bitriver-live`, but no such GitHub account is available and this repository is owned by the `ProhibitedTV` user. The repository token can publish only to its owner namespace.
    - The Windows MSI passes the `v...` tag directly to WiX and stages Compose/env below `share/deploy` while WiX reads different `share/*` paths; the tag workflow would fail even after credentials were supplied.
    - PR #1328 merged as `bbd9a1df`, #1300 closed, and full local/remote source-build product proof exists. Pull-only tagged artifacts and the clean Ubuntu/XOA/reboot/browser gates remain unproved.

- [x] Task 2 - Generate secret-safe release metadata and validation input
  - Acceptance criteria:
    - A reusable helper validates SemVer tags, derives stable/prerelease/package/MSI metadata, rotates every sample credential, applies the candidate tag/namespace, and resolves required third-party image digests.
    - Env and sentinel files are mode-restricted, never printed, and deleted on every workflow exit path.
    - Focused tests reject malformed tags, missing keys, sample-value reuse, invalid digests, and evidence containing generated credentials.
  - Check:
    - Added `scripts/prepare_release_candidate.py`, a standard-library helper with strict SemVer parsing, separate stable/prerelease/latest/MSI/nFPM metadata, atomic mode-0600 output, strong job-local credentials, sample-value rejection, sentinel separation, release tag/namespace application, and bounded Docker registry digest resolution.
    - The helper never prints credentials, keeps the Redis/chat credential consistent, applies production/pull and loopback media acceptance values, and writes only non-secret release metadata when requested.
    - `python -B scripts/prepare_release_candidate_test.py` passed eleven tests in Python 3.13, covering stable/RC metadata, malformed tags, rotated samples, digest rendering/failure, missing template keys, lowercase namespace enforcement, private file output, and no secret leakage to stdout/stderr.
    - The real registry command `docker buildx imagetools inspect alpine:3 --format '{{.Manifest.Digest}}'` returned a valid SHA-256 manifest digest; complete dependency resolution remains part of the pull-mode runtime gate.

- [x] Task 3 - Make the canonical stack publishable and pull-testable
  - Acceptance criteria:
    - Compose, env, CLI preflight, Helm, tests, and operator docs share an overridable image namespace whose official default is `ghcr.io/prohibitedtv`.
    - Quickstart keeps build/development defaults but supports an external env plus pull/production mode with no first-party build.
    - Contract render, OME helper render, digest enforcement, source-build quickstart, and focused pull-mode controls pass.
  - Check:
    - Added `BITRIVER_IMAGE_NAMESPACE` with official default `ghcr.io/prohibitedtv`; Compose, env, Helm, CLI manifest preflight, image scans, release-bundle assertions, and focused tests now use the owned namespace while accepting a lowercase mirror override.
    - `test-quickstart.sh` retains build/development defaults but accepts an external `BITRIVER_SMOKE_ENV_FILE` plus explicit build/pull and development/production controls. Pull mode requires the env, preserves its image digests, enforces production third-party pins, pulls the tagged OME helper, and skips all Compose builds.
    - Pinned Go 1.26.5 `go test ./cmd/bitriver ./scripts -count=1 -timeout=120s` passed; shell syntax, generated contract freshness, and both default/overridden namespace assertions passed.
    - Canonical Compose rendered all five first-party images under `ghcr.io/prohibitedtv`. The default Docker Desktop source-build quickstart passed in 109.4 seconds through OME render, dependencies, migrations, API, and viewer, then left no BitRiver container or smoke media volume.

- [x] Task 4 - Block candidate publication on pulled-image product evidence
  - Acceptance criteria:
    - The release workflow publishes tagged images, waits boundedly for registry availability, runs the canonical production/pull stack plus full golden path, and uploads only scanner-approved JSON.
    - GitHub Release creation depends on that job; prereleases do not move `latest` and are marked prerelease.
    - Linux package and MSI versions are normalized, and Windows staging uses the canonical release asset manifest.
  - Check:
    - `release.yml` now derives strict SemVer metadata once, prepares scanner-separated ephemeral credentials without repository secrets, validates non-loopback production contract input, and labels prepublication first-party digest values honestly as format-only.
    - Five multi-architecture images publish under the lowercase repository-owner namespace with OCI source/revision/version labels. RC tags publish only their immutable tag; `latest` is conditional on a stable tag.
    - The new 30-minute `pull-only-product-gate` proves anonymous access to all tagged manifests with bounded retries, pins their real digests plus resolved third-party digests, runs production/pull Compose and the full 1080p golden path, scans against candidate credentials, and uploads only the two JSON evidence files. GitHub Release creation depends on this job.
    - GitHub Release metadata marks hyphenated tags as prereleases. Linux packages use normalized version/prerelease fields, and the Windows job normalizes WiX ProductVersion, stages `release-assets.txt`, harvests the full `share/bitriver-live` tree, and no longer points WiX at nonexistent two-file paths.
    - The real production env and digest validators passed against a disposable prepublication profile; the source-free release bundle passed from a path containing spaces.
    - Pinned nFPM v2.47.0 built amd64/arm64 `.deb`/`.rpm` plus `v1.2.3-rc.1` prerelease packages and retained `rc.1` in Debian metadata. Ten/then eleven helper tests, focused Go workflow/CLI suites, release/nFPM YAML parsing, WiX XML parsing, shell syntax, generated contract freshness, and diff checks passed.
    - Actual Windows WiX compilation, anonymous GHCR visibility, registry propagation, and pulled-image runtime execution necessarily remain Task 6 tag-workflow evidence.

- [x] Task 5 - Update public release and installation guidance
  - Acceptance criteria:
    - README and release/install docs explain the real official namespace, RC semantics, anonymous package/image checks, and exact candidate versus stable boundary.
    - The no-release notice remains until an actual candidate publishes, then changes only in a post-publication evidence update.
    - Contract and release notes remain aligned with Compose/env/workflow behavior.
  - Check:
    - README keeps the no-release notice while explaining the first immutable RC, official `ghcr.io/prohibitedtv` namespace, pull-only gate, and exact clean-host/stable boundary.
    - Ubuntu, release, gate, testing, viewer, systemd, and draft release-note guidance now agree on RC semantics, stable-only `latest`, anonymous manifest checks, job-local workflow credentials, actual image digests, and clean Ubuntu/XOA/NPM/OME/browser/reboot follow-up evidence.
    - All current operator-doc and executable references to the nonexistent `ghcr.io/bitriver-live` namespace were removed; the literal remains only in historical planning/evidence text and an intentional forbidden-string workflow regression assertion.
    - `./scripts/check-doc-installer-language.sh`, `./scripts/generate-contract-doc.sh --check`, and `git diff --check` passed.

- [-] Task 6 - Verify, merge, publish, and inspect the first candidate
  - Acceptance criteria:
    - Required local verification and full remote PR CI pass with unrelated user files and private `.env` excluded.
    - The merged commit receives one immutable RC tag; the release workflow, packages, checksums, public image access, and pulled-image golden path pass.
    - Failure never force-moves a tag; the next attempt increments the RC. Remaining clean Ubuntu/XOA/NPM/browser/reboot work is stated exactly.
    - Every workflow-owned fresh Postgres service explicitly applies repository
      migrations before tagged storage tests, with a contract regression covering
      both the reusable and release workflows.
  - Check:
    - Local `./scripts/verify.sh --viewer` passed in 196.1 seconds with pinned Go 1.26.5: all Go packages, architecture/contract/schema checks, release-bundle validation, Postgres migration lifecycle, Compose config, Docker Desktop source quickstart, viewer lint, and 25 suites/215 tests passed.
    - The private root `.env` was moved outside the checkout only for that run, restored in `finally`, and retained the exact SHA-256 hash. The canonical smoke left no BitRiver containers.
    - Eleven Python release-helper tests, focused Go CLI/workflow tests, YAML/WiX parsing, shell syntax, generated contract checks, doc consistency checks, and `git diff --check` passed.
    - First PR run `30132326373` exposed ShellCheck SC1007 on the intentionally empty stable nFPM prerelease assignment. Changed it to the explicit `NFPM_PRERELEASE=''` form; no package behavior changed.
    - Replacement run `30132400350` then exposed two escaped OME-helper selectors in the CI/standalone image-scan workflows that still matched the retired namespace even though the images built under the new namespace. Updated both selectors and their shared regression test; the failure occurred before Trivy evaluated any CVE, so it was workflow drift rather than a vulnerability finding.
    - Current-head run `30132686142` passed Ubuntu full-stack, image CVE,
      cross-platform Go/entrypoint, lint, unit, browser, and viewer build work,
      then `npm audit` blocked on newly published
      `GHSA-mh99-v99m-4gvg` (`brace-expansion<=5.0.7`). The advisory names
      5.0.8 as the only patched release; ordinary non-breaking `npm audit fix`
      could update only the existing v5 copy and still reported 27 vulnerable
      transitive paths. A tested 5.0.8 override is therefore the next release
      gate; force-upgrading ESLint or suppressing the advisory is not accepted.
    - A direct all-majors override cleared audit but broke legacy minimatch's
      callable CommonJS contract, so it was rejected. The final install hook
      keeps the exact upstream `brace-expansion@5.0.8` package and implementation
      and changes only its CommonJS export shape so legacy callable and current
      named-export consumers both work; no vulnerable expansion implementation
      is copied or retained. Focused unit coverage protects both export shapes
      and the patched maximum-length behavior.
    - The viewer image dependency stage now copies `vendor/` before `npm ci`;
      a Go workflow regression test enforces that ordering so the local
      compatibility hook cannot pass host CI while failing the published
      container build.
    - A folder-based first draft installed and executed correctly but left npm
      reporting nested local links as invalid. Keep the override registry-backed
      and apply the reviewed legacy CommonJS export hook during `postinstall`;
      require clean `npm ci`, `npm ls`, audit, tests, and the real container
      build before publication.
    - Final proof passed: clean `npm ci`; a valid, fully deduplicated
      `npm ls brace-expansion --all` graph containing only 5.0.8; live
      `npm audit --audit-level=high` with zero vulnerabilities; viewer lint;
      26 suites/217 tests; all 36 Playwright tests; Next.js production builds;
      and a clean viewer Docker build (`sha256:5c8e3407...`). The complete
      `./scripts/verify.sh --viewer` gate then passed in 194 seconds with Go
      1.26.5, all Go/contract/Postgres checks, Docker Compose full-stack
      quickstart, OME health, API/viewer probes, and matching private `.env`
      restoration hash.
    - PR #1329 merged as `fbf0df9c`; verified Dependabot PR #1327 then
      fast-forwarded `main` to `6d78d75e`. The immutable
      `v1.2.3-rc.1` tag was created and pushed on that exact commit.
    - Release run `30211792842` passed secret-safe production-environment
      validation, then stopped before builds/publication in `Postgres storage
      tests`: the workflow supplied a fresh service DSN without
      `BITRIVER_TEST_POSTGRES_RUN_MIGRATIONS=1`, so the script correctly
      rejected the missing schema. Keep `rc.1` immutable, fix both CI-owned
      Postgres service workflows with regression coverage, and use `rc.2`.
    - Both workflow steps now opt into migrations explicitly. The focused Go
      workflow-contract suite passed, and a disposable Linux proof using
      Postgres 15 plus Go 1.26.5 applied migrations 0001-0011 through the
      supplied-DSN branch and passed `go test -tags postgres
      ./internal/storage/...`.
    - The complete `./scripts/verify.sh` gate passed on the fix branch:
      all Go/architecture/contract/Postgres checks, Docker Compose render,
      source-build quickstart, OME health, API/viewer probes, and private
      `.env` hash restoration succeeded.
    - Published assets/images, anonymous pull, and tag-workflow product
      evidence remain pending.

## Scoped change: full-stack production golden-path E2E (#1300)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Audit the existing E2E boundary and define the real product contract
  - Acceptance criteria:
    - `PLAN.md` records the real-stack stages, reusable execution shape, secret-safe evidence boundary, risks, and validation plan before implementation.
    - Existing ingest, quickstart, release-gate, evidence-scan, Compose, auth/channel/chat/VOD, and workflow paths are inspected.
    - `SPEC.md` distinguishes direct media/API proof from future pull-only, browser recovery, reboot, and stability evidence.
  - Check:
    - `scripts/test-ingest-e2e.sh` currently runs only `TestIngestPipelineEndToEnd` in `internal/storage`; it does not start or contact any canonical Compose service.
    - The existing quickstart smoke already owns deterministic Compose startup, OME render/token/process health, migrations, API/viewer reachability, diagnostics, and clean teardown, making it the correct lifecycle host for a reusable running-stack product harness.
    - Real self-signup/session cookies, first-channel creator bootstrap, RTMP callbacks, OME/transcoder adapters, Redis chat/moderation, multipart upload processing, recording publication, and public viewer/VOD APIs already exist; the missing release gate is orchestration and content-level evidence.
    - `scripts/scan-release-evidence.sh` already supports external sentinel files and path-only findings, so new reports can reuse the established no-secret publication boundary.

- [x] Task 2 - Build the running-stack media/API harness and evidence report
  - Acceptance criteria:
    - A standard-library harness creates creator/viewer sessions and a channel without direct database access.
    - Runtime-generated 1080p/audio media publishes by RTMP and produces content-level OME/transcoder playback evidence plus an offline transition.
    - Chat/history/moderation and VOD upload/transcode/publish/list/playback use authenticated real API paths.
    - A versioned report records stage status/duration and sanitized evidence without credentials.
  - Check:
    - Added `scripts/production_golden_path.py`, a Python standard-library black-box client that uses only public HTTP/RTMP surfaces plus host `ffmpeg`/`ffprobe`.
    - The harness creates separate creator/viewer sessions, bootstraps a channel, publishes deterministic 1920x1080 video plus audio, requires advancing and decodable OME/transcoder HLS, observes offline transition, exercises chat/history/timeout moderation, and uploads/transcodes/publishes/probes a public VOD.
    - `bitriver.production-golden-path/v1` evidence is written atomically with per-stage timing, first-failure ownership, sanitized URLs/errors, and runtime sentinel redaction; credentials and stream keys are excluded.
    - `python -B scripts/production_golden_path_test.py -v` passed 8 focused sanitization, polling, playlist, rendition-selection, and failed-evidence tests; AST parsing, CLI help, and `git diff --check` passed.
    - Real Compose/media execution remains Task 6 acceptance; implementation completion is not being treated as runtime proof.

- [x] Task 3 - Integrate the harness with canonical quickstart and safe diagnostics
  - Acceptance criteria:
    - The harness can validate an already-running stack and can run once inside quickstart before teardown.
    - Failures name the stage, preserve bounded redacted diagnostics, scan sentinel secrets, and leave no running Compose project or generated credential diff.
    - The cheap storage integration test remains separately named and callable.
  - Check:
    - `scripts/test-production-golden-path.sh` supports `running` and `quickstart` lifecycle modes plus host or Docker clients; both success and failure run the existing release-evidence sentinel scan.
    - The quickstart smoke uses an isolated named transcoder volume, preserves operator-owned `.env` and runtime media, emits only safe Compose state on failure, and tears down services, networks, data, configs, and the media volume.
    - Real Docker Desktop proof passed in 115.5 seconds. The retained report passed all eight stages in 27.998 seconds: accounts/channel, 1080p RTMP live state, advancing and decodable OME/transcoder HLS, offline transition, chat/timeout moderation, 1080p VOD upload/transcode/publication/playback, and final aggregate status (`ready`, seven checks, zero down).
    - Both live paths and VOD decoded H.264 at 1920x1080 for three seconds; the evidence scanner passed and `docker ps -a --filter name=bitriver` plus the smoke-volume query returned empty after teardown.
    - An isolated Alpine check reproduced Git Bash rewriting `/healthz` to `C:/Program Files/Git/healthz`; `MSYS2_ENV_CONV_EXCL=*` preserved the container path. The quickstart now applies only that environment guard and retains argument conversion for native Docker temp paths, with a focused Go contract test.

- [x] Task 4 - Wire release-blocking workflows and regression tests
  - Acceptance criteria:
    - The ingest workflow runs the real canonical stack and uploads sanitized machine-readable evidence.
    - CI path filters and `test-all` avoid duplicate expensive stack startups.
    - Static/focused tests fail on legacy storage-only E2E wiring, missing scanner invocation, or unbounded stage configuration.
  - Check:
    - `scripts/test-ingest-e2e.sh` is now a compatibility entrypoint for the real canonical-Compose product gate; the prior storage/controller integration test remains available as the honestly named `scripts/test-ingest-storage.sh`.
    - `test-all.sh` and `test-integration.sh` expose `--production-golden-path` plus a matching environment control, retain the old ingest aliases, and skip their ordinary quickstart when the product gate owns that lifecycle.
    - `.github/workflows/ingest-e2e.yml` is now named `Production golden path`, has a 30-minute job bound, executes the real gate, and always attempts to upload only the scanner-approved versioned JSON report for 14 days.
    - CI ingest path filters include the harness, client image, wrapper, and separated storage guard; the unified Ubuntu gate enables the accurately named product control.
    - Pinned Go 1.26.5 `go test ./scripts -count=1 -timeout=120s` passed, including a regression that rejects storage-only E2E wiring, missing bounded/upload workflow controls, or duplicate quickstart ownership. Shell syntax, eight Python harness tests, and `git diff --check` passed.

- [x] Task 5 - Document the canonical gate and honest release boundary
  - Acceptance criteria:
    - `docs/release-gates.md`, testing docs, production release guidance, and release notes identify the new gate and its evidence.
    - Build-mode, pull-only/tagged, browser, reboot, and repeated-run claims remain explicit and separate.
  - Check:
    - README operator acceptance now includes the lifecycle warning, exact product-gate command, and sanitized report location.
    - Testing/status docs distinguish the cheap storage/controller guard from real product acceptance and document opt-in umbrella controls, stages, evidence, and running-stack semantics.
    - `docs/release-gates.md` adds a blocking production media/workflow gate and renumbers scorecard/readiness/canary references consistently; the production runbook requires source proof before tag and pull-only clean-host repetition before stable promotion.
    - Ubuntu installation guidance and v1.2.3 draft notes state that source-built Compose proof exists while tagged images/packages, clean-host Ubuntu, reboot recovery, and browser recovery/quality remain unproved.
    - `check-doc-installer-language.sh`, generated contract freshness, targeted wording/reference searches, and `git diff --check` passed.

- [x] Task 6 - Run local Docker proof, full verification, and publication lifecycle
  - Acceptance criteria:
    - The real-stack gate passes locally with media/API evidence and clean teardown.
    - Required repository checks and remote CI pass; diff review excludes secrets, generated configs, runtime media, and unrelated user files.
    - PR/issue evidence states exactly what remains for #1297/#1300/#1304.
  - Check:
    - Local real-stack product gate passed all eight stages and evidence scanning on Docker Desktop; teardown left no BitRiver containers or smoke transcoder volume.
    - Pinned Go 1.26.5 focused runtime/workflow packages passed; eight Python harness tests, shell syntax, generated contract freshness, installer wording, and diff checks passed.
    - Full clean-checkout `./scripts/verify.sh --viewer` passed in 208.3 seconds: all Go packages, architecture/dependency/contract checks, release bundle, Postgres migrations, Compose validation, quickstart smoke, viewer lint, and 25 viewer suites (215 tests, four snapshots).
    - The operator-local root `.env` was moved outside the checkout only for the clean-checkout verification and restored in a guaranteed `finally` block; no backup/leftover file or runtime container/volume remained.
    - PR #1328 first remote run passed secret, docs, workflow, shell, and image-scan jobs. Ubuntu then reproduced `mkdir /work/live: permission denied`: the Linux smoke override forced host UID 1001 onto the named volume initialized for the transcoder image UID 10001. The fix retains the image user for that volume and replaces raw `docker inspect` failure output with a state-only diagnostic.
    - The second remote run confirmed Linux quickstart passes and progressed to the tagged Postgres tier, which found `postgres_ingest_e2e_test.go` still using removed `ingeststub.Options.PlaybackURL` and legacy OME application lifecycle expectations.
    - The tagged scenario now derives playback through `OMEPlaybackBaseURL`, expects current authenticated application validation, and passed `go test -count=1 -timeout=120s -tags postgres ./internal/storage/...` against a migrated disposable Postgres 15 container.
    - Final CI run 30128551732 passed on head `712f9275`, including the complete cross-platform matrix. Its Ubuntu test-all job executed the canonical production golden path, produced `/evidence/production-golden-path.json`, passed the release-evidence scan, and avoided a duplicate quickstart lifecycle.
    - Draft PR #1328 records local and remote evidence plus the remaining tagged Ubuntu/XOA, reboot, browser recovery/quality, and pull-only release boundaries owned by #1297/#1304; squash merge remains the publication step after this task record lands.

## Scoped change: clean-host Ubuntu Compose installer foundation (#1297)

- [x] Task 1 - Inventory release packages and define the clean-host contract
  - Acceptance criteria:
    - `PLAN.md` records supported-host claims, asset/config/data layout, systemd lifecycle, OME readiness boundary, Nginx Proxy Manager topology, risks, and evidence plan.
    - Existing launcher/archive/package contents, Compose bind mounts, pull-only image behavior, installers, and docs are inspected before implementation.
    - Unrelated working-tree files and unproved Debian/ARM64/reboot/playback claims are explicit boundaries.
  - Check:
    - The Linux launcher package currently ships only `deploy/docker-compose.yml` and `deploy/.env.example`; canonical Compose also binds migrations, the migration runner, SRS/OME generated configs, Nginx config, and transcoder data.
    - Pull-only Compose still names `bitriver-live/ome-config:local`, but release automation publishes only API, viewer, SRS controller, and transcoder images.
    - Packaged CLI root discovery falls back to the current directory when no `go.mod` exists, so OME rendering cannot reliably locate installed templates.
    - The historical Ubuntu/systemd installer deploys native API binaries rather than the canonical full Compose stack.

- [x] Task 2 - Make release bundles self-contained and pull-only
  - Acceptance criteria:
    - One canonical asset manifest/staging helper builds the launcher layout used by archives and Linux packages.
    - Installed-root discovery is explicit and works outside a source checkout and from paths containing spaces.
    - Compose uses a published, version-matched OME config image in pull mode; release automation publishes/scans multi-architecture output.
    - Every Compose bind mount and render dependency required by the release-shaped stack is present without source files.
  - Check:
    - Added `deploy/install/release-assets.txt` plus `scripts/stage-release-assets.sh`; both release binary archives and launcher/package jobs now consume the same source-free asset set.
    - Staging passed in a Linux container with an output path containing spaces and included Compose/env, all migrations, the canonical migration runner, SRS/OME render inputs, Nginx config, installer/systemd assets, scripts, and operator docs.
    - Packaged root discovery now honors `BITRIVER_ROOT` and launcher/package layouts outside a Go checkout; focused CLI and wrapper tests passed with Go 1.26.5.
    - Compose now pulls `ghcr.io/bitriver-live/bitriver-ome-config` by release tag/digest; GHCR preflight and required production digest validation include it.
    - Release automation now publishes the OME helper for amd64/arm64 and emits Linux amd64/arm64 CLI, launcher, `.deb`, and `.rpm` artifacts. Compose config, release workflow tests, shell syntax, and `git diff --check` passed.

- [x] Task 3 - Add the Ubuntu host installer and safe lifecycle commands
  - Acceptance criteria:
    - Installer supports archive and package layouts, separates assets/config/data, creates a bounded systemd unit, and never starts with sample credentials.
    - Install is rerunnable; status/log/upgrade commands are actionable; non-root operation uses explicit sudo boundaries.
    - Uninstall disables/removes program integration while retaining config/data by default; destructive purge requires an explicit flag and warning.
    - OME failure leaves the unit failed with redacted service diagnostics and retry commands.
  - Check:
    - Added `deploy/install/compose-host.sh` with install/upgrade/configure/activate/doctor/status/logs/uninstall commands and a 15-minute bounded systemd unit.
    - Program assets stage under `/opt/bitriver-live`; root-owned source assets are separated from `/etc/bitriver-live` configuration and `/var/lib/bitriver-live` application/transcoder data through explicit symlinks.
    - First install runs non-interactive env initialization to rotate sample credentials but leaves the service disabled until the guided wizard, doctor, production env validation, Docker access, and bounded quickstart pass.
    - Activation failure reports systemd plus Compose status and exact targeted OME/retry commands without automatically dumping credential-bearing environment or raw logs.
    - The isolated lifecycle test passed twice from a source path and target path containing spaces; configuration survived the rerun, ordinary uninstall retained config/data, unconfirmed purge failed, and confirmed purge removed them.

- [x] Task 4 - Add artifact-only and package lifecycle evidence
  - Acceptance criteria:
    - Tests assemble and execute the bundle outside the checkout in a path containing spaces.
    - Tests cover complete contents, rerunnable install, configuration gate, systemd/service shape, restart behavior, upgrade staging, safe uninstall, and explicit purge.
    - `.deb`/`.rpm` payload generation and canonical asset parity are checked for amd64/arm64 inputs.
    - Relevant focused checks pass before documentation work proceeds.
  - Check:
    - `scripts/test-release-bundle.sh` staged the canonical payload outside the checkout in a path containing spaces, verified manifest parity, and rejected generated credential-bearing OME/SRS files.
    - `scripts/test-compose-host-installer.sh` covered rerunnable install, rotated configuration, rendered systemd shape, upgrade-safe state retention, ordinary uninstall, rejected purge, and confirmed purge without touching the host.
    - `scripts/test-linux-packages.sh` used nFPM v2.47.0 to build and inspect amd64/arm64 `.deb` and `.rpm` payloads from the staged release bundle.
    - Real package generation exposed and fixed unsupported nFPM template syntax and an extra asset-directory nesting level; package paths now resolve to `/usr/local/share/bitriver-live/deploy/...`.

- [x] Task 5 - Document Ubuntu/XOA/Nginx Proxy Manager installation and support boundaries
  - Acceptance criteria:
    - README, quickstart, Ubuntu install guide, deployment contract, release notes, and production release guide describe the artifact-only path.
    - VM sizing, Docker/Compose setup, non-root/sudo workflow, boot recovery, firewall/NAT ports, WebSockets, trusted proxies, TLS, backup/upgrade/uninstall, and diagnostics are explicit.
    - Ubuntu 24.04 amd64 is the only production claim unless additional direct evidence passes; real ingest/playback and OME restart remain assigned to #1300/#1304.
  - Check:
    - Replaced the stale native Ubuntu service guide with the artifact-only Compose host path for Ubuntu 24.04 amd64, including XOA VM sizing, Docker/Compose prerequisites, archive/package checksums, two-phase activation, paths, backup, upgrade, uninstall, and reboot evidence.
    - Documented NPM app/media proxy hosts, WebSockets, exact trusted-proxy CIDR, TLS/public URL values, and the direct RTMP/WebRTC firewall/NAT boundary that an HTTP reverse proxy cannot satisfy.
    - README, quickstart, deployment contract, release guide/notes, deploy map, testing, upgrades, and the NPM/Cloudflare guide now agree on the release asset and systemd lifecycle.
    - OME language explicitly requires authenticated control plus real ingest/playback/recovery against the tagged VM; an unauthenticated root health probe is not release approval.
    - Generated contract environment index and installer-language consistency checks passed.

- [x] Task 6 - Run full verification and prepare publication evidence
  - Acceptance criteria:
    - Full repository verification, release/package tests, Compose rendering, and quickstart smoke pass or exact environment blockers are recorded.
    - Diff review excludes credentials, generated runtime output, and unrelated deployment helpers/data.
    - Final task evidence distinguishes implementation proof from tagged-release VM reboot and playback evidence.
  - Check:
    - Literal `./scripts/verify.sh` passed in the pinned Go 1.26.5 plus Python container, including release-bundle, installer-lifecycle, all first-party Go package, architecture, models-import, dependency-source, contract-invariant, and generated-contract checks. Docker and viewer phases reported explicit container-tooling skips.
    - The verification entrypoint now disables VCS stamping and bounds default Go/filesystem traversal to first-party Go roots; focused regression tests prevent the Windows-mounted-workspace livelocks exposed by this final run.
    - `scripts/test-postgres-migrations.sh` passed the real PostgreSQL migration lifecycle. `scripts/test-quickstart.sh` then rebuilt the release-shaped stack and passed OME helper rendering/validation, OME health-token preflight, service health, migrations, API health, and retried viewer health.
    - `scripts/test-linux-packages.sh` generated and inspected amd64/arm64 `.deb` and `.rpm` payloads with nFPM v2.47.0. Compose rendering, YAML parsing, PowerShell parsing, shell syntax, installer-language consistency, and `git diff --check` also passed.
    - Post-smoke `docker compose --env-file .env -f deploy/docker-compose.yml ps --all` returned an empty service table; generated OME/SRS config and root `.env` have no diff.
    - GitHub authentication was restored; commit `c3dd9c65` was pushed and draft PR #1325 opened without closing #1297. The first CI pass caught ShellCheck SC2016 in two intentional literal workflow assertions; escaped fixed-string patterns now preserve the contract without suppressions, and Linux `bash -n` plus `scripts/test-release-bundle.sh` pass locally.
    - The repaired CI run then exposed a stale `bitriver-live/ome-config:local` Trivy target duplicated in `ci.yml`, even though Compose built `ghcr.io/bitriver-live/bitriver-ome-config:ci`. Both scan workflows now select exactly one OME helper from the collected Compose image list; YAML parsing, CI contract validation, rendered image selection, and the focused Go 1.26.5 regression test pass.
    - The next image scan passed, but the Ubuntu gate exposed `compose pull --ignore-buildable` contacting GHCR for the non-buildable `ome-health-token-check` sibling after its shared helper image had already been built locally. Quickstart now enumerates rendered image references, retains locally inspectable images, and pulls only genuinely absent images.
    - Linux syntax and the focused Go 1.26.5 quickstart regression passed. A full host smoke then reused the local OME helper, passed OME render/token/process health, migrations, API health, and viewer retry, and cleaned down to an empty Compose project.
    - Final CI run 29628820507 passed on implementation head `de869492`: Ubuntu test-all, image vulnerability scan, ShellCheck, docs/workflow consistency, committed-secret guard, viewer integration, Windows/macOS Go tests, and Ubuntu/macOS/Windows entrypoint checks are green.
    - Draft PR #1325 remains unmerged. No production-release claim is permitted until the external release gates below pass.
    - Unrelated deployment helpers/data remain untracked and are explicitly excluded from the intended change set; the temporary `.gomodcache/` was removed after verification.
    - Tagged Ubuntu/XOA reboot, authenticated OME control-plane, and real ingest/playback acceptance remain external release evidence owned by #1297/#1300/#1304 and are not claimed by this local candidate.

- [x] Task 7 - Reconcile the installer candidate with current main
  - Acceptance criteria:
    - PR #1326's merged SRS/OME/transcoder/public media URL and Windows documentation contracts remain intact.
    - Ubuntu release assets, OME helper publication, package/systemd lifecycle, and pull-only behavior remain intact.
    - README and quickstart distinguish implemented future release paths from downloads that do not exist before the first tag.
    - Focused contract tests, release-bundle/installer checks, Compose rendering, full verification, and remote PR gates pass on the reconciled head.
  - Check:
    - Read-only merge analysis identified overlapping release workflows, Compose/env validation, quickstart smoke, and operator documentation; `PLAN.md` now records the reconciliation rules and validation plan.
    - Current `main` is `0f557e81`; PR #1325 remains draft at `5d2f3d11`, with its historical CI green but its head non-mergeable until this reconciliation is complete.
    - Merged current `main` locally without rewriting the published branch. The runtime/workflow changes combined automatically; eight planning/operator-doc conflicts were resolved to preserve the installer lifecycle, the verified media path, and the pre-tag availability boundary.
    - In `golang:1.26.5-bookworm`, shell syntax, focused `./cmd/bitriver` and `./scripts` tests, the source-free release bundle, and the isolated Compose-host installer lifecycle all passed.
    - The pinned nFPM v2.47.0 acceptance rebuilt and inspected amd64/arm64 `.deb` and `.rpm` payloads from the reconciled canonical release bundle.
    - Literal `./scripts/verify.sh` passed in the pinned Go 1.26.5 plus Python Linux environment: release bundle, installer lifecycle, all first-party Go packages, architecture/dependency, CI contract, env hygiene, and generated-contract checks were green. Docker and viewer phases were explicitly skipped inside that tool-only container and are validated separately below.
    - Viewer lint, all 25 Jest suites / 215 tests / 4 snapshots, and the Next.js 16.2.10 production build passed on the reconciled dependency baseline.
    - The first live quickstart attempt exposed a backward-compatibility defect before container startup: when the smoke reused the operator's older root `.env`, Compose could not interpolate `BITRIVER_OME_PUBLIC_LLHLS_BASE_URL`. The plan now requires smoke-only public media defaults plus a regression for pre-existing env files.
    - The second smoke reached clean image builds and exposed an upstream dependency regression already merged to `main`: `typescript@7.0.2` violates the latest `ts-jest@29.4.12` peer range (`typescript >=4.3 <7`), so Docker's clean `npm ci` fails. The release plan now restores the last proven TypeScript 6.0.3 pin instead of bypassing peer validation.
    - After the TypeScript repair, the clean image build passed but identified peer overrides from ESLint 10 and a Node 26 type package against the declared Node 24 runtime. The reconciled release baseline now restores ESLint 9.39.5 and `@types/node` 24.13.3 as well.
    - The official npm advisory audit classified the remaining three high findings as production dependencies through Next 16.2.10; npm identifies the aligned 16.2.11 patch as the non-major fix for Next, PostCSS, and Sharp.
    - Next 16.2.11 removes the direct Next advisories but still pins vulnerable PostCSS 8.4.31 and Sharp 0.34.x. Because no newer stable Next exists and the viewer disables Next image optimization, the plan now requires explicit fixed-version overrides plus clean audit/build/boot evidence.
    - The successful Compose build also showed a roughly 200 MB root build context traced to the user's untracked `deploy/transcoder-data/`; the release boundary now requires that runtime media directory to be ignored by Docker.
    - The aligned Node 24 / TypeScript 6 / ESLint 9 baseline plus Next 16.2.11 and the fixed PostCSS/Sharp overrides passed an isolated clean `npm ci`, lint, 25 Jest suites / 215 tests / 4 snapshots, production build, and `npm audit --omit=dev --audit-level=high` with zero vulnerabilities.
    - The rebuilt canonical quickstart passed OME helper render/validation, all image builds, migration completion, critical service health including OME, API health, and viewer reachability; cleanup left an empty Compose project and restored generated configuration.
    - Adding `deploy/transcoder-data/` to the root `.dockerignore` reduced the helper rebuild context transfer from roughly 200 MB to 37 kB and kept the user's local media outside the builder.
    - Reconciled CI run 30101348208 passed the unified Ubuntu gate, viewer CI/audit, Windows/macOS Go tests, entrypoint checks, docs, ShellCheck, and secret guard. Its sole failure was the blocking viewer image scan: the Node 24 Alpine base supplied global npm with `tar@7.5.15` (CVE-2026-59873), while the application tree was clean.
    - The production viewer stage now removes unused npm/npx runtime payloads while retaining them in build stages. The focused runtime-baseline test passed; the rebuilt image retained Node 24, omitted npm/npx, and served `/viewer` with HTTP 200.
    - Final reconciled CI run 30102433565 passed on head `e3f96cb5`, including the unified Ubuntu gate, first-party blocking image scan, viewer integration/build/audit, cross-platform Go, quickstart entrypoint matrix, and all secret/docs/shell/workflow guards. PR #1325 is mergeable and Task 7 is complete.

### Execution log
- Task 1 analysis:
  - Confirmed #1297 requires artifact-only install/restart/reboot/status/log/upgrade/uninstall behavior and an explicit Ubuntu/Debian/ARM64 support matrix.
  - Selected Ubuntu 24.04 amd64 as the first production target; Debian 12 and Linux arm64 remain provisional despite current cross-build/package jobs.
  - Selected `/opt/bitriver-live`, `/etc/bitriver-live`, and `/var/lib/bitriver-live` as separate program/config/data boundaries with data-preserving uninstall.
  - Identified local-only OME config image publication and packaged-root discovery as prerequisites to any truthful clean-host success claim.
- Task 2 implementation:
  - Replaced release-workflow copy fragments with a canonical manifest-driven asset staging step shared by binary archives and launcher/package payloads.
  - Added explicit installed asset-root resolution so the Go renderer, Compose defaults, doctor, migrations, and release commands resolve the release bundle rather than the invoking shell's directory.
  - Converted the OME helper from a local-only image name to a tagged/digest-pinnable GHCR contract and added it to multi-architecture publication plus vulnerability scanning.
  - Expanded launcher/package builds to Linux arm64 without declaring support until clean-host evidence passes.
- Task 3 implementation:
  - Added a release-layout-aware host manager and systemd unit rather than extending the historical native API-only installer.
  - Made package/archive installation safe-by-default: sample secrets are rotated, but no service is enabled before production network values and Docker prerequisites validate.
  - Kept immutable source/package payloads separate from the installed runtime workspace and operator-owned configuration/data so upgrades and package removal cannot silently erase state.
  - Added explicit OME failure/retry guidance while reserving real playback and restart acceptance for #1300/#1304.
- Task 4 verification:
  - Added permanent release-bundle, installer-lifecycle, and opt-in real Linux package acceptance scripts.
  - Added container package-install/remove acceptance to the release workflow for Ubuntu 24.04, Debian 12, and Rocky Linux 9 while keeping the production support claim limited to Ubuntu 24.04 amd64.
  - Proved nFPM emits all four Linux package variants and that the package payload is sourced from the same asset manifest as release archives.
- Task 5 documentation:
  - Established Ubuntu 24.04 amd64 as the production installation target while keeping Debian/RPM/arm64 claims provisional pending tagged clean-host evidence.
  - Added a concrete XOA plus Nginx Proxy Manager runbook that separates HTTP reverse proxying from RTMP/WebRTC L4 and UDP exposure.
  - Added clean-host/reboot evidence requirements to the production release gate and v1.2.3 draft notes instead of treating container health as end-to-end media proof.
- Task 6 verification/publication:
  - Passed literal repository verification in the pinned Go 1.26.5 environment, real PostgreSQL migration acceptance, real nFPM package generation, and a rebuilt Compose quickstart smoke through OME/API/viewer health; confirmed clean teardown afterward.
  - Hardened verification against implicit VCS stamping and unbounded mounted-workspace traversal after the complete gate exposed both portability defects.
  - Published the local candidate as draft PR #1325. The initial CI run's sole early failure was ShellCheck SC2016 on intentional literal variables; corrected it with escaped fixed-string assertions and passed the focused Linux syntax/bundle checks before republishing.
  - The next vulnerability job found the main CI workflow still scanned the retired local OME helper tag. Unified both scan workflows around the image rendered by Compose and added a regression guard against missing, ambiguous, or legacy targets.
  - After the image gate passed, the Ubuntu gate found quickstart's blanket non-buildable pull tried to fetch the locally built helper through its health-token sibling. Switched to local-first rendered-image inspection and passed the full host smoke plus clean teardown.
  - Completed the local/publication task with required PR CI green. Kept PR #1325 draft and #1297 open because tagged XOA/reboot/media-path evidence necessarily remains pending.
