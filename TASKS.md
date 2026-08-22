# TASKS

## Scoped change: viewer tooling dependency refresh (#1398)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Diagnose the protected CI failure and bound the refresh
  - Acceptance criteria:
    - The failing check is traced to a specific dependency/baseline mismatch.
    - `PLAN.md` records package scope, runtime boundaries, risks, and tests
      before implementation.
  - Check:
    - CI run 32054411899 failed in `TestViewerRuntimeBaselineIsAligned`: the
      reviewed package/lock files contain `@types/node` 26.2.0 while the test
      still requires 26.1.2. Image scanning and native arm64 viewer proof
      passed; no product runtime failure was reported.
    - The other six updates are development-only accessibility, testing, type,
      and lint packages. Node 24, npm 11, Next/React runtime dependencies, and
      the deployment contract remain unchanged by this PR.

- [-] Task 2 - Align the baseline, verify the clean graph, and merge
  - Acceptance criteria:
    - The runtime-baseline assertion matches the intentional type package pin.
    - Focused viewer checks, literal full verification, protected CI, and
      squash merge pass without expanding runtime or deployment scope.
  - Check:
    - The invariant now requires `@types/node` 26.2.0. Its focused Go test
      passes against the updated package and lock files.
    - Clean `npm ci`, ESLint, all 26 Jest suites / 218 tests / 4 snapshots,
      and the Next 16.3.0 production build passed. The production-only audit
      exits zero at the blocking high threshold; it reports one non-blocking
      moderate PostCSS advisory already present in the main runtime override.
    - Literal `./scripts/verify.sh --viewer` passed with Go 1.26.0 and Docker
      29.6.2, including repository/release/docs/contract guards, all Go
      packages, migrations, exact image builds, healthy quickstart smoke,
      viewer lint, and the complete Jest suite. Deleted ignored shims supplied
      the two absent loopback media URLs and a path-neutral Jest pattern for
      this nested Windows worktree only.
    - Protected CI and squash merge remain.

## Scoped change: deterministic single-host service restart rehearsal (#1304)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Inventory failure surfaces and bound the local proof
  - Acceptance criteria:
    - Canonical service dependencies, public health semantics, durable state,
      existing restart evidence, and unresolved roadmap acceptance are mapped.
    - `PLAN.md` defines isolation, security, timing, evidence, testing, and the
      exact-candidate/physical-host boundary before implementation.
  - Check:
    - `/readyz` directly probes Postgres and Redis, authenticated `/api/status`
      forces live SRS/OvenMediaEngine/transcoder probes, and `/viewer` isolates
      the viewer boundary. `/healthz` is intentionally excluded as the sole
      ingest signal because its ingest snapshot may be cached.
    - Existing RC20 evidence covers OME, Docker-daemon, and systemd restart
      cases but not deterministic API/database/cache/media/viewer failure
      injection or durable-state checks across those recoveries.
    - The canonical fixed container names prevent a parallel project from
      safely sharing the host. The rehearsal will preflight collisions, stage
      a clean tracked tree with private bind storage and project-scoped named
      volumes, use bounded waits, and always clean its isolated state.

- [x] Task 2 - Implement the probe/evidence helper and focused tests
  - Acceptance criteria:
    - The helper creates a private authenticated fixture, persists its session,
      classifies expected degradation/recovery, and verifies durable state.
    - Report validation and tests reject secrets, unbounded or missing timing,
      incomplete scenarios, and unstable restart counts.
  - Check:
    - `cmd/tools/service-resilience` now owns a standard-library HTTP client
      with an in-memory cookie jar, randomly generated private signup fixture,
      durable session/channel identity checks, `/readyz`, `/api/status`, and
      viewer signal classifiers, plus context-bounded polling.
    - The `bitriver.service-resilience/v1` validator requires all seven named
      scenarios, non-negative degradation/recovery timings, passing durable-
      state and restart-stability results, complete isolation/teardown flags,
      explicit remaining acceptance, and absence of every supplied private
      sentinel before its atomic JSON report is published.
    - Nine focused tests passed with Go 1.26.0: cookie/fixture persistence,
      readiness and ingest-status classification, timeout refusal, report
      completeness/timing/secret refusal, stable restart-count comparison,
      staged private-environment handling, redacted log tails, and host build-
      policy isolation, and in-repository report-path protection.

- [x] Task 3 - Orchestrate deterministic service failures and real recovery
  - Acceptance criteria:
    - The isolated canonical stack experiences bounded stop/start failures for
      API, Postgres, Redis, SRS/controller, OME, transcoder, and viewer paths.
    - Every scenario proves its expected degradation, recovery, durable-state
      invariants, and absence of a restart loop; retained evidence is secret-
      scanned and teardown leaves no test containers or volumes.
  - Check:
    - `scripts/test-service-resilience.sh` now runs the Go orchestrator against
      a clean `git archive` tree with a copied private environment, dedicated
      host ports, staged API/transcoder binds, project-scoped Postgres/Redis
      volumes, canonical-container collision refusal, bounded commands/probes,
      private redacted logs, and guaranteed Compose teardown.
    - The first dry runs exposed and fixed three harness defects before fault
      injection: Windows stdout-pipe Git archive deadlock, metadata-only PAX
      header handling, and leaked host `GOPROXY=off` build policy. The first
      live baseline then correctly caught a staged OME listener/API endpoint
      mismatch; both are now aligned without changing the deployment contract.
    - The complete real-stack rehearsal passed all seven scenarios. Measured
      recovery was API 3.039s, Postgres 1.459s, Redis 1.440s, SRS/controller
      2.176s, OvenMediaEngine 2.853s, transcoder 2.347s, and viewer 4.121s.
      Every outage produced its expected degradation, preserved the same
      authenticated session/channel, and held all service restart counts stable.
    - The secret-scanned `bitriver.service-resilience/v1` report contains seven
      passing scenarios and explicit remaining acceptance. Teardown left zero
      project containers, volumes, or private work directories. Root `.env`
      and generated OME hashes remain exactly unchanged at
      `9D57F716...AAC2` and `01C44166...B1D`.

- [x] Task 4 - Align resilience and release guidance
  - Acceptance criteria:
    - Operator, release, testing, and draft release docs expose the opt-in
      command/report and distinguish this local proof from remaining #1304 work.
    - Documentation and scoped static checks pass.
  - Check:
    - Operations guidance now documents the isolated command, host/tool
      prerequisites, collision refusal, private staging/storage model, seven
      signal paths, durable-state and restart-loop assertions, Redis ephemeral
      boundary, report schema, and exact remaining acceptance.
    - Production-release, testing, and v1.2.3 draft guidance now require the
      `bitriver.service-resilience/v1` local-build foundation without treating
      it as exact-candidate, physical reboot, resource/partition, credential,
      active-stream, or alert-delivery proof.
    - Three Markdown-link unit tests and the complete 89-file tracked public
      Markdown scan passed. Go helper tests, Git Bash syntax, scoped diff check,
      and pinned ShellCheck v0.11.0 also passed.

- [x] Task 5 - Run full gates and publish the focused resilience slice
  - Acceptance criteria:
    - Literal `./scripts/verify.sh`, protected CI, review, and squash merge pass
      without touching operator-owned state or overclaiming #1304 completion.
    - A staged environment always replaces an absolute operator
      `BITRIVER_CONFIG_ROOT` with the disposable checkout root before Compose
      can run writable SRS/OME config renderers.
    - #1304 receives bounded evidence plus the exact remaining follow-up.
  - Check:
    - Literal `./scripts/verify.sh` passed with Go 1.26.0: repository hygiene,
      release bundle, every first-party Go package, architecture/dependency
      guards, release-set and Markdown checks, deployment invariants, Postgres
      migration lifecycle, Compose rendering, exact local image builds, and the
      complete quickstart smoke were green. Viewer checks were correctly
      skipped because this slice does not touch `web/viewer`.
    - The private root `.env` still lacks the two optional SRS/OME public route
      values, so the local run supplied loopback defaults only to Docker through
      an ignored temporary shim; test processes saw the real environment, and
      the shim was deleted immediately after the gate.
    - Pinned ShellCheck v0.11.0, nine focused Go helper tests, the complete
      seven-outage real-stack rehearsal, secret-safe report validation, and
      post-run container/volume/network/hash checks passed. Root `.env` and
      generated OME hashes remain unchanged; no test runtime residue remains.
    - Protected CI passed on exact head `7e651718`; review then identified one
      valid config-root isolation defect. Its focused regression, refreshed
      full gate, review resolution, squash merge, and bounded #1304 handoff
      remain.
    - The review repair now derives an absolute staged config root from the
      private destination and appends it as the effective
      `BITRIVER_CONFIG_ROOT`. The focused helper suite and `go vet` passed with
      a disposable repo-local Go cache.
    - Refreshed literal `./scripts/verify.sh` passed with pinned Go 1.26.0 and
      Docker 29.6.2: all Go packages, architecture/dependency guards, release
      and documentation checks, deployment invariants, Postgres migrations,
      Compose render, image builds, service health, and quickstart smoke were
      green. A deleted ignored Docker-only shim supplied the two documented
      loopback media URLs absent from the private root `.env`; tests continued
      to receive the unmodified operator environment.
    - The review thread was resolved, the branch was updated onto current
      `main`, protected CI run 32549119817 passed, and PR #1397 squash-merged as
      `5d2271b5`. Issue #1304 received the bounded evidence and remains open for
      exact-candidate/physical-host and advanced failure acceptance.

## Scoped change: packaged-host disaster-recovery foundation (#1299)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Inventory durable inputs and bound the packaged recovery proof
  - Acceptance criteria:
    - Existing package layout, backup/restore contract, persistent paths,
      product gate, and published-artifact limitation are evidenced.
    - `PLAN.md` defines encryption, isolation, restore safety, reporting,
      testing, and the remaining exact-publication boundary before code changes.
  - Check:
    - Packaged hosts separate source-free program assets under
      `/opt/bitriver-live`, operator secrets/generated config under
      `/etc/bitriver-live`, and API/transcoder/media data under
      `/var/lib/bitriver-live`; Postgres remains a named volume and Redis is
      explicitly ephemeral.
    - The merged Postgres scripts already publish and verify an atomic
      archive/manifest/checksum set with release/schema/row-count identity, but
      the source-free asset manifest does not ship those scripts and there is no
      encrypted config/media recovery set or destructive lost-host orchestrator.
    - The existing production golden path covers auth, admin, channel, ingest,
      playback, chat/moderation, and VOD. The new drill can reuse it after
      recovery instead of creating a weaker parallel smoke test.
    - Public RC20 release-set SHA-256
      `dd8eabcea7cf920a6f520e3e472cf44d3e1c7b0b7ad74945904f67ea74a47873`
      binds the Linux amd64 launcher archive at SHA-256
      `ef4b1d1095fceab6ff9b6f6b55828b041d891ad36174cbdc2f28feba636e90f8`;
      it predates this recovery payload, so final immutable artifact-only proof
      must use the next published candidate after this slice merges.

- [x] Task 2 - Implement encrypted host recovery sets and refusal tests
  - Acceptance criteria:
    - Backup atomically publishes an encrypted host/Postgres archive, manifest,
      and checksum without exposing passphrases or retained plaintext.
    - Restore verifies identity/checksums/paths before mutation, refuses unsafe
      or non-fresh targets, and proves exact aggregate config/data invariants.
    - Focused unit/integration tests cover success and all planned failures.
  - Check:
    - `scripts/backup-host-state.sh` now publishes an OpenSSL AES-256-CBC,
      PBKDF2-HMAC-SHA256 encrypted archive plus secret-safe manifest and exact
      checksum set. It requires a restricted passphrase file, embeds the
      already-verified Postgres trio, fingerprints packaged config/data/media,
      optionally carries an aggregate external-object inventory, and never
      writes a plaintext host archive.
    - `scripts/restore-host-state.sh` verifies checksum/release/encryption
      identity, refuses non-fresh targets, decrypts once for fail-closed member
      validation and again for streaming extraction, accepts only canonical
      regular files/directories, and emits measured RPO/RTO plus matched
      config/data/Postgres/object invariants without secrets.
    - Five Linux unit tests passed, including release/checksum refusal,
      symlink escape, traversal/special archive members, and restored invariant
      reporting. The full OpenSSL integration passed success, wrong release,
      wrong passphrase, corruption-before-mutation, non-fresh target, object
      inventory, and same-timestamp collision cases.
    - The first integration run exposed a cleanup ownership defect where a
      refused same-timestamp producer removed the existing set. Explicit final
      asset ownership now preserves all three existing files byte-for-byte;
      the repaired integration passed.

- [x] Task 3 - Ship recovery tools and automate a source-free lost-host drill
  - Acceptance criteria:
    - Source-free launcher/package assets contain the canonical recovery tools.
    - A disposable rehearsal destroys source runtime state, rebuilds a fresh
      packaged-host layout, restores Postgres/config/media/object fixtures, and
      emits secret-safe RPO/RTO evidence.
  - Check:
    - The source-free asset manifest now ships canonical Postgres backup,
      restore, pruning, encrypted host backup/restore, Python runtime wrapper,
      and recovery metadata helper files. Release-bundle and packaged-host
      installer lifecycle tests passed with the commands present/executable and
      generated credentials still excluded.
    - `scripts/test-disaster-recovery.sh` stages a source-free bundle, obtains a
      real non-empty manifest-bound Postgres backup from the existing complete
      refusal suite, builds the encrypted config/API/media/Postgres/object set,
      deletes the source host, and restores into a fresh packaged-host root.
    - The fresh installer preserved recovered secrets, normalized generated
      config compatibility links, reconnected durable paths, and installed the
      same recovery commands. A new exact-digest Postgres 15 container restored
      the recovered backup into a fresh database with four roles plus exact
      object metadata; off-host object bytes matched their aggregate inventory.
    - Secret-safe `bitriver.disaster-recovery/v1` evidence records measured
      RPO/RTO, source-free bundle fingerprint, and five passing stages while
      retaining the exact published-package, production golden-path, and
      scheduled off-host proof as explicit remaining acceptance. Six Linux unit
      tests and the complete destructive disposable drill passed.

- [x] Task 4 - Align operations, installation, release, and testing guidance
  - Acceptance criteria:
    - Operator docs define inputs, encrypted backup/restore commands,
      scheduling/off-host ownership, RPO/RTO, object consistency, and evidence.
    - Documentation distinguishes merged source-free proof from the next
      candidate's required immutable clean-host qualification.
  - Check:
    - Operations guidance now inventories the installed recovery commands,
      required GNU tar/Python/OpenSSL/Postgres client tools, restricted
      passphrase handling, atomic encrypted set, pre-install fresh-host restore,
      recovered database handoff, external-object ownership, credential
      rotation, and evidence/report schemas.
    - Ubuntu installation, deployment-contract, deploy README, production
      release gate, testing, and v1.2.3 draft guidance now agree on packaged
      paths, commands, RPO/RTO targets, source-free proof, and the next
      immutable candidate plus recovered-stack golden-path boundary. The guide
      explicitly warns that RC20 predates this package payload.
    - Three doc-link unit tests and the complete 89-file tracked Markdown link
      scan passed. Pinned ShellCheck v0.11.0 passed all seven changed shell
      scripts after replacing two ambiguous numeric validation expressions.
    - The final `/var/backups/bitriver-live/recovery` layout passed six Linux
      helper tests, the encrypted host integration, and the complete destructive
      source-free disaster-recovery drill. Root `.env` and generated OME hashes
      remain exactly unchanged.

- [x] Task 5 - Run full gates and publish the focused recovery slice
  - Acceptance criteria:
    - Literal `./scripts/verify.sh`, protected CI, review, and squash merge pass
      without touching operator-owned files or overclaiming #1299 completion.
    - #1299 receives bounded evidence and the exact next-candidate follow-up.
  - Check:
    - Literal `./scripts/verify.sh` passed with Go 1.26.0: repository hygiene,
      release-bundle and installer contracts, every first-party Go package,
      architecture/dependency guards, Markdown links, deployment invariants,
      Postgres migration lifecycle, Compose rendering, and the complete
      quickstart smoke were green. Viewer checks were correctly skipped because
      this slice does not touch `web/viewer`.
    - The private root `.env` lacked the two optional public media-route values,
      so the local run supplied loopback defaults only to `docker compose` via an
      untracked temporary shim; Go environment-isolation tests remained clean,
      and the shim was deleted immediately after the gate.
    - Automated review found four valid evidence gaps. Backup now inventories a
      private immutable snapshot rather than later mutable live paths; disaster
      evidence binds the exact embedded Postgres archive/manifest identity,
      reports the maximum host/database RPO, and the documented root-level unit
      invocation works. Six Linux unit tests, the deterministic post-snapshot
      mutation integration, pinned ShellCheck v0.11.0, source-free bundle check,
      and the complete real-Postgres lost-host drill pass after the fixes.
    - Literal `./scripts/verify.sh` passed again on the amended tree through the
      full Go, contract, migration, Compose, and quickstart gates; the private
      `.env` and generated OME hashes remain unchanged and no smoke containers
      remain.
    - Protected run `31923999469` passed Ubuntu aggregate verification,
      secrets/docs/ShellCheck, all three quickstart entrypoints on Ubuntu,
      macOS, and Windows, and the enforced high-risk merge gate. All four review
      threads were resolved; PR #1396 squash-merged as
      `730cc0e8403ae1a8c7c1cb1e29a316b99a43bc5a`, and #1299 received bounded
      evidence while remaining open for next-candidate artifact qualification,
      recovered-stack production golden path, and scheduled off-host RPO proof.

## Scoped change: exact-image Compose upgrade and rollback rehearsal (#1298)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Inventory immutable full-stack inputs and bound the rehearsal
  - Acceptance criteria:
    - Source/candidate release-set assets, commits, first-party images, shared
      dependencies, existing golden path, and source limitations are evidenced.
    - `PLAN.md` defines isolation, security, interruption, rollback, reporting,
      and remaining acceptance before implementation.
  - Check:
    - Public RC19 and RC20 release sets revalidated at SHA-256
      `374a4084d1880abab1fa980d528a47bb5e324ed85541248438015fb13f2cc204`
      and `dd8eabcea7cf920a6f520e3e472cf44d3e1c7b0b7ad74945904f67ea74a47873`;
      candidate identity binds commits `1e14e3cf7d5f1d949b396d4f7897660575ea468e`
      and `9a8516a60c584c96a46b630b55c46df33f46fbdc`.
    - Both manifests contain all five first-party immutable image references and
      the same eight digest-pinned third-party references. RC20 changes the
      pinned `postgres:15-alpine` manifest from
      `sha256:3d0f7584ed7d04e27fa050d6683a74746608faf21f202be78460d679cc56461f`
      to `sha256:4006528dcbdd9be8c1aaa50389caea4e93c46d6f54c3533bcd3253725e526e23`;
      the rehearsal therefore includes that stateful dependency-image upgrade.
      The current production golden path already supports a running exact-image
      stack and emits secret-scanned evidence.
    - The canonical Compose contract uses named Postgres/Redis volumes and a
      bind-backed transcoder workspace. A test-only named-volume override plus
      fixed isolated project can carry all three across clean tagged source,
      candidate, and rollback trees without touching operator state.
    - RC19 is a rejected source release and cannot qualify as the approved
      rollback target. The rehearsal records its source/rollback aggregate
      health observation without assuming a particular status and keeps #1298
      open until an approved prior release is proved on a clean host.

- [x] Task 2 - Share the representative fixture and invariant contract
  - Acceptance criteria:
    - Data-plane and Compose rehearsals load the same actual-schema fixture and
      compute the same deterministic fixed-record/count invariant document.
    - The existing backup/migration/restore/interruption rehearsal remains green.
  - Check:
    - Moved the actual-schema account/auth/MFA/session, profile/channel/schedule,
      stream/recording/upload, moderation/chat/legal, and payment fixture into
      `scripts/fixtures/stateful-upgrade.sql` and its deterministic fixed-record
      fingerprint plus aggregate counts into the adjacent invariant query.
    - The data-plane rehearsal now copies and executes those shared files rather
      than carrying private inline duplicates. Fixed user selection keeps the
      value fingerprint stable when the upgraded golden path adds new accounts.
    - Bash syntax, pinned ShellCheck v0.11.0, scoped diff check, and the complete
      manifest-bound Postgres backup/migration/interruption/in-place rollback/
      fresh-restore rehearsal passed after extraction with clean teardown.

- [x] Task 3 - Automate exact-image Compose upgrade, interruption, and rollback
  - Acceptance criteria:
    - Exact release-set checks, clean tagged trees, immutable image assertions,
      source state, verified backup, candidate upgrade, and interrupted cut
      point are automated in an isolated rerunnable harness.
    - RC20 passes the production golden path after upgrade; rollback restores
      exact source image/config identity without losing source or candidate data.
    - Secret-safe JSON and Markdown evidence record identities, invariants,
      durations, outcomes, and honest RC19 limitations.
  - Check:
    - `scripts/stateful_compose_upgrade.py` validates both release-set SHA-256
      values and identities, all five first-party and eight dependency image
      references, then creates a credential-stable source/candidate env pair.
      Four focused unit tests cover the happy path plus release-set tamper,
      missing-image, and changed dependency-digest behavior.
    - `scripts/test-stateful-compose-upgrade.sh` extracted clean RC19/RC20 Git
      trees, refused canonical container-name collisions, pulled and asserted
      immutable images, loaded the shared representative fixture, created and
      verified the manifest-bound backup, and proved the candidate migration/
      config cut point exposed no public health endpoint.
    - The first complete rehearsal upgraded the persisted stack, passed the
      Dockerized RC20 production golden path, preserved fixed-state invariants
      while retaining candidate-created accounts, then restored exact RC19
      first-party images plus byte-identical generated OME/SRS configuration.
      Source/rollback aggregate health was observed as HTTP 200/200; RC19 stays
      explicitly unapproved. Secret scans and isolated cleanup passed.

- [x] Task 4 - Align operator/release/testing guidance and focused checks
  - Acceptance criteria:
    - Upgrade, production release, testing, and draft release guidance match the
      exact Compose proof and remaining limitation.
    - Bash/ShellCheck, both stateful rehearsals, docs/link/secret/diff checks pass.
  - Check:
    - Upgrade, production-release, testing, and v1.2.3 draft guidance now expose
      both focused commands, report schemas, exact dependency transition,
      verified full-stack behavior, and the approved clean-host rollback/reboot
      acceptance that remains.
    - Seven helper/link unit tests, the 89-file Markdown link scan, committed
      secret guard, installer wording guard, Bash syntax, pinned ShellCheck
      v0.11.0, and scoped diff check passed.
    - The real-Postgres data-plane rehearsal passed again with verified backup,
      candidate preflight, interruption refusal, rollback, fresh restore, secret
      scan, and clean teardown. The exact-image Compose rehearsal passed in Task
      3 and was not repeated after documentation-only changes.

- [x] Task 5 - Run full gates and publish the focused full-stack slice
  - Acceptance criteria:
    - Literal `./scripts/verify.sh`, protected CI, review, and merge pass without
      touching operator-owned files or overclaiming #1298 completion.
    - #1298 receives bounded evidence and remains open for any genuinely unmet
      healthy rollback or clean-host acceptance.
  - Check:
    - Literal full verification passed after final evidence-bundle changes; PR
      #1395 opened from commit `29b51d71` with protected CI in progress.
    - Review found one P1 portability defect: the POSIX rehearsal invoked the
      non-executable `scripts/python.sh` wrapper directly. The call now goes
      through Bash; a real Linux container probe returned Python 3.12.13, Bash
      syntax, ShellCheck v0.11.0, and all four helper tests passed.
    - The complete exact-image rehearsal passed after the repair, including the
      production golden path, evidence scans, exact rollback, and clean teardown.
      Final full verification passed again on commit `9f9ad5c4`.
    - Protected run `31921209601` passed Ubuntu in 4m16s, secrets, docs,
      ShellCheck, and all three quickstart entrypoints. Its merge gate alone
      rejected the initial PR body because it lacked the enforced release
      scorecard headings. The live PR now contains the complete high-risk
      build/CI, data/migrations, and operator-workflow scorecard; rerunning the
      old workflow retained its stale event payload, so this ledger update will
      trigger the fresh metadata-aware run required before merge.
    - Fresh protected run `31921577370` passed Ubuntu in 4m22s, secrets, docs,
      ShellCheck, Ubuntu/macOS/Windows quickstart entrypoints, and the enforced
      high-risk release-scorecard merge gate. The sole P1 thread was resolved;
      PR #1395 squash-merged as
      `645f5b28b0062d8f8488e2fbb32ae34ec7d789f8`, and #1298 received bounded
      evidence while remaining open for an approved prior-release rollback.

## Scoped change: stateful RC19 to RC20 upgrade/rollback data-plane rehearsal (#1298)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Select the exact upgrade pair and bound the first rehearsal
  - Acceptance criteria:
    - Source/candidate tags, commits, release-set hashes, migration delta, and
      existing upgrade/rollback gaps are evidenced before implementation.
    - `PLAN.md` defines the data, security, rollback, interruption, report, and
      remaining full-stack boundaries.
  - Check:
    - Selected immutable public `v1.2.3-rc.19` at
      `1e14e3cf7d5f1d949b396d4f7897660575ea468e` as the immediate source and
      `v1.2.3-rc.20` at `9a8516a60c584c96a46b630b55c46df33f46fbdc`
      as the candidate. Their public `release-set.json` asset SHA-256 values are
      `374a4084d1880abab1fa980d528a47bb5e324ed85541248438015fb13f2cc204`
      and `dd8eabcea7cf920a6f520e3e472cf44d3e1c7b0b7ad74945904f67ea74a47873`.
    - `git diff v1.2.3-rc.19..HEAD` found no migration runner/file change;
      RC19 to RC20 is therefore eligible for database-layer in-place rollback
      proof, not a destructive-schema rollback claim.
    - Existing guidance is manual and the release smoke gate only records an
      upgrade plan. No automated non-empty stateful upgrade/rollback report or
      exact-pair interruption rehearsal exists.

- [x] Task 2 - Automate real-schema state, upgrade, rollback, and interruption proof
  - Acceptance criteria:
    - Actual canonical migrations and representative non-empty state are used.
    - A verified pre-upgrade backup exists; candidate migration preserves exact
      invariants; in-place rollback and restore-required recovery are classified
      and proved for the exact pair.
    - Ambiguous migration state blocks candidate progress, and a secret-safe
      machine-readable report records identities, results, and durations.
  - Check:
    - `scripts/test-stateful-upgrade.sh` asserts the current migration tree and
      runner object identities match the exact RC19/RC20 pair, applies all
      canonical migrations to disposable Postgres 15, and loads actual-schema
      fixtures across accounts/roles, profile/OAuth/auth/MFA/session, channel/
      follow/schedule, stream/recording/upload/object references, moderation,
      chat/filter/report/appeal, legal/audit, and payment state.
    - The first complete run published and verified a manifest-bound RC19
      backup, proved the RC20 plan had no pending migrations, applied the no-op
      candidate path, matched exact ledger and value/count fingerprints, then
      restored the RC19 configuration fixture and classified the database layer
      as in-place compatible.
    - A separate candidate database with a synthetic exact-checksum `applying`
      cut point was blocked by migration preflight. After an observable
      candidate-only value mutation, verified restore to a fresh retained
      database recovered the exact pre-upgrade ledger and value/count
      invariants; all disposable databases/container/evidence were removed.
    - The generated `bitriver.stateful-upgrade-report/v1` binds both tags,
      commits, release-set hashes, migration objects/fingerprints, invariant
      matches, rollback class, interruption refusal, config-fixture hashes,
      durations, and remaining acceptance. The release-evidence secret scan
      passed.

- [x] Task 3 - Align upgrade/release/testing guidance with proven behavior
  - Acceptance criteria:
    - Docs identify the exact proven hop, report, commands, rollback class, and
      explicit remaining full-stack boundary.
    - Shell/report/docs/diff/secret checks pass and task evidence is current.
  - Check:
    - `docs/upgrades.md`, production release guidance, testing guidance, and the
      v1.2.3 draft notes now identify the exact RC19/RC20 release identities,
      focused command, report schema, database-layer in-place classification,
      future schema reclassification rule, and remaining full-stack gate.
    - Upgrade backup guidance now requires the complete manifest-bound
      archive/manifest/checksum set with exact release provenance and points to
      the durable recovery inventory rather than treating an ad hoc dump as
      sufficient release evidence.
    - Focused `bash -n`, cached ShellCheck v0.11.0, Markdown link checker tests,
      Markdown link validation, committed-secret guard, release-evidence scan,
      and scoped `git diff --check` all pass.

- [x] Task 4 - Run full gates and publish the focused foundation
  - Acceptance criteria:
    - Literal `./scripts/verify.sh`, protected CI, review, and merge pass without
      touching operator-owned files or overclaiming #1298 completion.
    - #1298 receives bounded merged evidence and remains open for exact-image
      Compose/config/media/golden-path rehearsal.
  - Check:
    - Literal `./scripts/verify.sh` passed with cached Go 1.26.0 and bundled
      Python, including all first-party Go/script packages, repository/CI/
      hygiene/release checks, documentation and contract checks, real
      PostgreSQL migration lifecycle, Compose rendering, and a rebuilt
      quickstart stack through service health plus API/viewer reachability.
      Viewer lint/tests were correctly skipped because viewer code is outside
      this change.
    - Post-gate cleanup left zero Compose and stateful-rehearsal containers.
      The private root `.env` and generated OME configuration retain their
      pre-run SHA-256 values, the generated file has no Git diff, and the
      temporary verification wrapper was removed.
    - Automated review found that committed migration object IDs could be
      paired with drifted working-tree files. The rehearsal now rejects both
      tracked and untracked drift under the migration tree/runner before Docker
      starts. Focused tracked/untracked refusal probes, Bash syntax, pinned
      ShellCheck v0.11.0, the full Postgres rehearsal, and literal repository
      verification all passed after the repair; both probes were removed and
      the runner returned to an exact clean state.
    - Protected run `31919021597` passed the Ubuntu `test-all` gate in 4m33s,
      secrets, docs, ShellCheck, all three quickstart entrypoints, and the
      aggregate merge gate. The sole P1 thread was answered and resolved.
    - PR #1394 squash-merged as
      `9582ea28dc755f16c4524741ef0347cf842e8881`; bounded evidence was posted to
      #1298, which remains open for the exact-image/config/media/golden-path phase.

## Scoped change: manifest-bound Postgres recovery rehearsal (#1299)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Inventory the existing recovery path and bound the first slice
  - Acceptance criteria:
    - Current backup/restore behavior, durable-state boundaries, and release
      acceptance gaps are evidenced from code/docs without changing runtime.
    - `PLAN.md` defines security, compatibility, isolation, evidence, and test
      boundaries before implementation.
  - Check:
    - Existing backup creates a compressed SQL dump plus checksum and supports
      pruning/upload, but records no release, commit, database, schema, tool,
      or row-invariant identity. Restore warns instead of failing when the
      checksum is absent, creates/drops its target before compatibility checks,
      and runs only database/table-presence smoke queries.
    - No backup/restore regression or recurring rehearsal workflow exists.
      Operations docs state 24-hour RPO, 2-hour RTO, and 30-day drill targets,
      while issue #1299 still requires non-empty isolated restore, corruption
      and incompatibility refusal, invariant/golden-path proof, measured output,
      and lost-host recovery.
    - `PLAN.md` now scopes a Postgres-only manifest/report foundation without
      changing the deployment contract or overclaiming media/object/config
      recovery.

- [x] Task 2 - Make successful backups atomic and manifest-bound
  - Acceptance criteria:
    - A successful backup publishes one gzip archive, mandatory checksum, and
      JSON manifest atomically; failed dumps leave no apparently valid set.
    - The manifest binds release/commit, database/server and pg tool versions,
      applied migration identity/fingerprint, and exact public-table row counts.
    - Missing or non-applied migration ledger state blocks the backup, and no
      sensitive connection or row data appears in output/evidence.
  - Check:
    - Backup now holds one exported repeatable-read Postgres snapshot across
      `pg_dump`, applied-migration capture, and exact public-table row counts.
      Missing/empty/non-applied migration ledger state aborts before publication.
    - The archive and secret-safe JSON manifest publish through partial paths;
      the checksum is moved last and covers both. Upload/prune now treat archive,
      manifest, and checksum as one set, and cleanup removes incomplete output.
    - A disposable real `postgres:15-alpine` database with applied migration,
      two users, and one channel produced exactly one valid set. Both checksum
      entries passed, JSON provenance/fingerprint/count assertions passed, no
      partial file remained, and the container/evidence copy were removed.

- [x] Task 3 - Validate before mutation and emit measured restore evidence
  - Acceptance criteria:
    - Restore requires checksum and manifest, validates archive/tag/schema
      identity before creating or dropping a rehearsal database, and rejects
      unsafe database identifiers.
    - Isolated restore compares exact migration and table-count invariants and
      emits a secret-safe JSON report with observed RPO/RTO and cleanup state.
    - The source database is never a valid restore target; explicit keep mode
      retains only the isolated rehearsal database.
  - Check:
    - Restore now requires an exact two-member checksum set and valid JSON
      manifest, validates archive/source/schema identity and safe database names,
      refuses an existing/protected/source database, and only then creates a
      fresh isolated target. Missing evidence is a hard failure.
    - The restored applied-migration fingerprint and every exact public-table
      row count must equal the backup snapshot. Default cleanup drops the target
      before an atomic report records matched compatibility/invariants, backup
      age, restore duration, and cleanup state; explicit keep mode is bounded to
      the validated isolated database.
    - A disposable real Postgres backup restored successfully with expected
      release/schema identity, matched invariants, emitted a parseable passed
      report, and left no rehearsal database. Wrong-release and source-database
      target attempts failed before mutation; the source retained both rows.

- [x] Task 4 - Add real-Postgres positive/negative proof and operator docs
  - Acceptance criteria:
    - Non-empty representative data survives backup/restore exactly.
    - Corruption, missing evidence, release mismatch, and schema mismatch fail
      before the requested rehearsal database exists.
    - Operations/release docs inventory every durable input, state RPO/RTO and
      evidence handling, and preserve the bounded Postgres-only claim.
  - Check:
    - `./scripts/test-backup-restore.sh` passed twice against disposable
      `postgres:15-alpine`. The final run created a complete backup set,
      verified both checksum entries, restored once with default cleanup and
      once in explicit keep mode, and proved the exact role, channel, object,
      and non-default operator-setting fixtures survived before removing the
      retained database and container.
    - Wrong release, wrong schema fingerprint, corrupt archive, missing
      checksum, missing manifest, and source-database target cases all failed;
      each requested rehearsal database was absent afterward. A non-applied
      migration ledger also blocked a second backup without leaving a valid or
      partial set.
    - Operations now defines the three-file manifest contract, compatibility
      inputs, measured report, RPO/RTO handling, legacy refusal, and the full
      durable recovery inventory. Release/testing guidance requires the bounded
      evidence and names the focused regression without changing CI behavior.
    - `python -m unittest scripts/check_doc_links_test.py`,
      `python scripts/check_doc_links.py`, shell parse checks, the committed
      secret guard, and scoped `git diff --check` passed.

- [x] Task 5 - Run full gates and publish the focused foundation
  - Acceptance criteria:
    - Focused shell/integration/docs/diff/secret checks and literal
      `./scripts/verify.sh` pass without changing operator-owned files.
    - Protected PR CI is green before merge; #1299 receives bounded evidence
      and remains open for artifact-only lost-host/media/object/golden-path work.
  - Check:
    - Focused `./scripts/test-backup-restore.sh`, shell parse, Markdown link,
      committed-secret, and scoped diff checks pass.
    - Literal `./scripts/verify.sh` passed with cached Go 1.26, bundled Python,
      and Docker Desktop after scoping the stale private `.env`'s missing public
      defaults to Compose subprocesses. All Go, release/contract, migration,
      Compose render, rebuilt quickstart, health, and cleanup checks passed;
      viewer checks correctly skipped because this slice does not touch it.
    - The private root `.env` retained SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`,
      the generated OME file retained SHA-256
      `01C441663CF1A44991A7EAD1A37D930509D50C4E1D4C0329A89FD697A11C7B1D`,
      no BitRiver test containers remain, and operator-owned paths are
      unchanged/untracked.
    - Commit, protected PR CI, merge, and bounded #1299 issue evidence remain.
    - PR #1393's first shellcheck run identified the intentional container-side
      `$1` expansion as SC2016. The exact boundary is now documented with a
      scoped suppression; shell parsing and the full Docker-backed rehearsal
      pass. Run `31916021784` then passed ShellCheck, docs, Ubuntu `test-all`,
      and Linux/macOS/Windows quickstart entrypoints. Its aggregate gate found
      the PR scorecard also needed `build/CI` classification for changed shell
      scripts; the PR body is corrected, and a fresh protected run remains
      before merge because failed-job retries retain the original event body.
    - Fresh run `31916342522` passed the aggregate merge gate, Ubuntu
      `test-all`, ShellCheck, docs, secret guard, and all three quickstart
      entrypoint jobs. Automated review then found two valid publication
      blockers: collision cleanup could remove a previously published
      same-second set, and the Helm scheduler still emitted the retired
      archive-plus-checksum format. Fix both with explicit set ownership, a
      collision regression, and a synchronized canonical Helm producer before
      rerunning the gates or merging.
    - The corrected rehearsal passed against disposable Postgres 15. Its
      deterministic same-second retry was refused, the first archive/manifest/
      checksum set remained byte-identical and valid, no partial/lock remained,
      and the complete positive/negative restore matrix still passed.
    - `bash -n` passed for the backup/restore/prune/rehearsal/sync scripts,
      `./scripts/sync-helm-deploy-assets.sh --check` passed, and focused
      `go test ./scripts -run TestHelmBackupUsesManifestBoundCanonicalProducer
      -count=1 -timeout=120s` passed. The contract test requires the chart's
      byte-identical canonical producer, ConfigMap mount, provenance/upload
      inputs, durable object-storage guard, and absence of the legacy inline
      two-file producer. Full verification and fresh protected CI remain.
    - Literal `./scripts/verify.sh` passed again after both review repairs,
      including all Go/script packages, release and contract checks, real
      Postgres migration lifecycle, Compose rendering, rebuilt quickstart
      service health, API/viewer reachability, and teardown. Viewer lint/tests
      correctly skipped because the viewer remains outside this slice.
    - The run left no `deploy` Compose containers or temporary wrapper. The
      private `.env` and generated OME file retained their recorded SHA-256
      hashes, and generated OME has no byte diff. Fresh protected CI, review
      thread resolution, merge, and bounded #1299 issue evidence remain.
    - Review repairs landed in `c8004c47`; both P1 threads were answered and
      resolved. Protected run `31917178911` passed Ubuntu `test-all` in 4m21s,
      ShellCheck, docs, secret guard, Windows/macOS Go, all three quickstart
      entrypoints, and the aggregate merge gate.
    - PR #1393 squash-merged as
      `d680d75032c3e556b5cc5363bf218107f77b4806`. Bounded evidence was posted to
      #1299, which remains open for artifact-only lost-host, encrypted config,
      media/object, production RPO/RTO, and golden-path acceptance.

## Scoped change: initial aggregate ingest health after RC19 rejection (#1297, #1304)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Diagnose and bound the immutable RC19 qualification failure
  - Acceptance criteria:
    - Exact public candidate identity and checksums are verified independently.
    - Sanitized no-checkout evidence and an exact-image reproduction identify
      the failed runtime invariant without exposing credentials.
    - `PLAN.md` records the correction and rollout plan before implementation.
  - Check:
    - Release run `31351022453` published 46 assets from exact green main
      `1e14e3cf7d5f1d949b396d4f7897660575ea468e`; `CHECKSUMS.txt` has exactly
      45 unique entries with zero missing, extra, or mismatched files. The
      signed release-set SHA-256 is
      `374a4084d1880abab1fa980d528a47bb5e324ed85541248438015fb13f2cc204`.
    - Clean-host run `31351694175` used no checkout, passed signed provenance,
      package install, production preflight, systemd activation, CLI smoke
      `7/7`, and direct authenticated OME control. Sanitized evidence retained
      no secrets and records failure at the aggregate OME assertion only.
    - The exact RC19 image digests reproduce `/healthz` with overall `ok` but a
      stale `ingest: disabled` service entry while all ingest endpoint settings
      and credentials are present. Both repository constructors pre-populate
      that cache, preventing the handler's missing-cache live fallback.
    - `PLAN.md` now bounds the empty-initial-cache correction, regression tests,
      documentation, verification, and immutable RC20 rollout.

- [x] Task 2 - Prime configured ingest health on the first request
  - Acceptance criteria:
    - JSON and Postgres repositories expose no cached ingest snapshot before
      their first health probe.
    - First `IngestHealth` calls query the configured controller and record a
      timestamped snapshot; later calls continue updating it.
    - Overlapping calls while that first probe is in flight share one probe and
      cached result for both JSON and Postgres repositories.
    - A no-op/disabled controller still records `ingest: disabled` after its
      first probe, and `/healthz` retains its existing cache semantics.
  - Check:
    - JSON and Postgres constructors no longer seed the health cache with a
      timestamped `ingest: disabled` value. The existing `/healthz` fallback can
      now call `IngestHealth` exactly when no snapshot has yet been recorded.
    - The shared repository scenario requires an empty initial snapshot, proves
      the configured controller supplies and updates the first cached values,
      and separately proves a no-op controller records `ingest: disabled` on
      its first real probe. The scenario runs directly for JSON and is reused by
      the Postgres-tagged integration suite.
    - `go test ./internal/storage ./internal/api ./internal/app -count=1
      -timeout=120s` passed with Go 1.26.5 and an isolated cache.
    - PR #1391's automated review identified that concurrent startup callers
      could still race through the empty-cache fallback. JSON and Postgres now
      publish one in-flight completion signal while the cache is empty, so
      overlapping callers share the first result and later calls keep the
      existing refresh behavior.
    - `go test ./internal/storage -run TestStorageIngestHealthSnapshots
      -count=50 -timeout=120s` and the focused storage/API/app suites passed
      with an isolated Go cache. Protected CI will exercise the shared scenario
      against real migrated Postgres before merge.

- [x] Task 3 - Align operations documentation and run local gates
  - Acceptance criteria:
    - Operations documentation explains first-request cache priming and later
      cached behavior without overclaiming continuous live fan-out.
    - Focused Go/Postgres, Markdown/link, diff, secret, and literal verification
      gates pass without changing operator-owned files.
    - An isolated rebuilt Docker stack reports exactly one healthy OME aggregate
      entry and is completely removed afterward.
  - Check:
    - `docs/operations.md` now states that the first `/healthz` request performs
      one bounded ingest probe when no snapshot exists and that later liveness
      requests reuse the cache. Markdown link tests/check (89 tracked public
      files), committed-secret guard, and `git diff --check` passed.
    - Focused JSON/API/app tests passed, and the exact Postgres regression passed
      against a newly migrated disposable `postgres:15-alpine` database. The
      canonical Bash wrapper also exposed a Windows Git-Bash `/migrations` path
      conversion defect; the equivalent PowerShell-provisioned test completed
      successfully without weakening the Postgres assertion.
    - A rebuilt production API image running against the exact RC19 dependency
      digests returned overall `ok` with exactly `srs: ok`,
      `ovenmediaengine: ok`, and `transcoder: ok`. The isolated containers,
      networks, volumes, test database, API image, and Go caches were removed.
    - Literal `./scripts/verify.sh` passed with Go 1.26.5, including all
      first-party packages, release bundle, Postgres migration lifecycle,
      contract/Compose checks, rebuilt quickstart, OME/API/viewer health, and
      cleanup; unchanged viewer lint/tests were explicitly skipped. The private
      `.env` was restored byte-for-byte at SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`,
      and generated OME bytes again equal `HEAD`.

- [x] Task 4 - Publish RC20 and rerun complete clean-host qualification
  - Acceptance criteria:
    - Focused PR and exact-main CI are fully green before tagging.
    - Immutable RC20 public assets, checksums, signed release set, and five
      image identities verify independently.
    - No-checkout qualification passes smoke/authenticated OME, same-tag
      upgrade, OME/Docker/systemd recovery, retained uninstall, sanitized
      evidence, and cleanup, or yields a new bounded forward-fix failure.
  - Check:
    - PR #1391 merged without bypass as exact main
      `9a8516a60c584c96a46b630b55c46df33f46fbdc`; protected run
      `31911599915` and exact-main run `31911889892` passed every required gate.
    - Release run `31912204601` passed and published 46 immutable RC20 assets.
      The repository verifier independently accepted all 45 checksum entries,
      exact tag/commit identity, and complete asset coverage at signed
      release-set SHA-256
      `dd8eabcea7cf920a6f520e3e472cf44d3e1c7b0b7ad74945904f67ea74a47873`.
    - No-checkout Ubuntu 24.04 run `31912782711` passed public provenance and
      package verification, install/preflight, pull-only systemd activation,
      smoke/authenticated OME, same-tag upgrade, OME/Docker/systemd recovery,
      retained-data uninstall, sanitized evidence, and cleanup in 5m10s.
      #1297/#1304 remain open for the physical-host acceptance boundaries.
    - Literal `./scripts/verify.sh` passed for this closeout, including all Go
      packages, release-set/docs checks, real Postgres migration lifecycle,
      Compose validation, rebuilt quickstart health, and cleanup. The private
      root `.env` was hash-restored and generated OME bytes were unchanged;
      unchanged viewer checks were explicitly skipped.

## Scoped change: post-start smoke after RC18 rejection (#1297, #1304)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Diagnose and bound the immutable RC18 qualification failure
  - Acceptance criteria:
    - Exact public candidate identity and checksums are verified independently.
    - Sanitized no-checkout evidence identifies the failed phase without
      exposing credentials or altering the published candidate.
    - `PLAN.md` records the correction and rollout plan before implementation.
  - Check:
    - Release run `31348429963` published 46 assets from exact green main
      `4e496e389d2a606976a55a89cd08ef79762eed2f`; `CHECKSUMS.txt` has exactly
      45 unique entries with zero missing, extra, or mismatched files. The
      signed release-set SHA-256 is
      `2de0cb71af2af8d41d7a535d28478003987d6b180568a84bc5dc3eae680ef014`.
    - Clean-host run `31348986535` used no checkout, verified the public
      signature/package, installed Ubuntu 24.04.4 x86_64, passed preflight, and
      activated the pull-only systemd stack. Sanitized evidence retained no
      secrets and records failure only at `smoke-and-ome-control`.
    - Failed logs show `bitriver smoke` invokes full default `doctor` after
      activation. Doctor reports BitRiver's own 23 occupied TCP/UDP ports, so
      smoke retries the same impossible pre-start condition and never reaches
      Compose/endpoint verification. `PLAN.md` was updated before code.

- [x] Task 2 - Make smoke prerequisites phase-correct and add regressions
  - Acceptance criteria:
    - Standalone `doctor` and quickstart still fail on real pre-start host-port
      conflicts.
    - Post-start smoke checks Docker/Compose availability and supported
      versions without requiring the running stack's ports to be free.
    - Smoke still fails for missing/stopped Docker, unreachable Compose state,
      unreadable env files, and unhealthy required endpoints.
  - Check:
    - `bitriver smoke` now calls a dedicated prerequisite runner that reuses
      doctor's required-binary and Docker/Compose version checks only. Doctor,
      quickstart host-port refusal, host sizing, and mount checks are unchanged.
    - Failure output now names Docker prerequisite failure rather than claiming
      an arbitrary full-doctor failure.
    - A regression replaces the host-port checker with a fatal test callback;
      smoke prerequisites pass with supported synthetic Docker 28.0.4 and
      Compose 2.38.2 without invoking it. The complete `./cmd/bitriver` suite
      passed on Go 1.26.5 (`-count=1 -timeout=120s`).

- [x] Task 3 - Align smoke documentation and run local gates
  - Acceptance criteria:
    - Operator docs distinguish doctor/preflight from post-start smoke.
    - Focused Go, Markdown/link, diff, secret, and literal verification gates
      pass without changing operator-owned files.
    - A running Docker Desktop stack proves smoke accepts its own occupied
      ports and reaches Compose plus endpoint checks.
  - Check:
    - `docs/smoke-test.md` and `docs/contract.md` now distinguish full
      pre-start doctor from post-start smoke and state why a running stack's
      own ports are not conflicts. Markdown links, committed-secret guard,
      generated contract docs, and `git diff --check` pass.
    - Literal `./scripts/verify.sh` passed with Go 1.26.5 and Python 3.12.13
      after safely sidelining and hash-restoring the private legacy `.env` so
      the verifier could use its tracked fixture. All first-party Go packages,
      release bundle, Postgres migration lifecycle, contract, Compose render,
      rebuilt Docker quickstart, OME/API health, and viewer reachability passed;
      the unchanged viewer lint/test phase was explicitly skipped.
    - An isolated Docker Desktop project then started nine services from a
      disposable env copy. The corrected CLI reached running Compose state and
      returned `PASS (7/7)` for Docker/Compose, API readiness/health, SRS,
      transcoder, and OME HTTP while the stack owned all configured ports.
    - The exact project containers, networks, volumes, credential-bearing temp
      env, and OME backup were removed/restored. The private `.env` is restored
      byte-for-byte at SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`;
      the OME worktree object equals `HEAD` and the six operator-owned untracked
      paths remain excluded.

- [x] Task 4 - Publish RC19 and rerun complete clean-host qualification
  - Acceptance criteria:
    - Focused PR and exact-main CI are fully green before tagging.
    - Immutable RC19 public assets, checksums, signed release set, and five
      image identities verify independently.
    - No-checkout qualification passes smoke/authenticated OME, same-tag
      upgrade, OME/Docker/systemd recovery, retained uninstall, sanitized
      evidence, and cleanup, or yields a new bounded forward-fix failure.
  - Check:
    - PR #1389 merged as exact main
      `1e14e3cf7d5f1d949b396d4f7897660575ea468e`; its protected run
      `31350396806` and exact-main run `31350698194` passed every required gate.
    - Release run `31351022453` passed all 33 jobs and published 46 RC19 assets.
      All 45 checksum entries match, the signed release set is exact, and its
      five first-party images are digest-bound.
    - No-checkout run `31351694175` passed artifact verification, package
      installation, preflight, activation, CLI smoke `7/7`, and authenticated
      OME control. It then exposed the bounded initial aggregate-health cache
      defect tracked by the new top scope; RC19 is rejected and unchanged.

## Scoped change: installed generated-config layout after RC17 rejection (#1297, #1304)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Diagnose the immutable RC17 clean-host failure
  - Acceptance criteria:
    - Public candidate bytes and the caller-supplied release root verify before
      qualification evidence is trusted.
    - The failed step and sanitized artifact identify a source-backed root cause
      without exposing credentials or patching the installed candidate.
    - `PLAN.md` records the correction, migration risks, and test plan first.
  - Check:
    - All 45 entries in RC17 `CHECKSUMS.txt` match the 45 downloaded public
      assets; there are no missing, extra, duplicate, or mismatched entries.
      The signed release-set SHA-256 is
      `38c4937f86b0c77cbe46044e224cd1569e5d88ddc1ef3ae5539b9f43e10be3e5`.
    - Run `31344257847` used no checkout and passed signed provenance, public
      `.deb` install, host staging, immutable input validation, and doctor. It
      failed activation when only `ome-config` exited 1; sanitized evidence and
      cleanup passed.
    - The helper now writes `/etc/bitriver-live/deploy/ome/...`, while the
      installer still creates and the OME consumer still follows the flat
      `/etc/bitriver-live/Server.generated.xml`. Source checkouts hide the
      mismatch because their source-shaped parents already exist. Updated
      `PLAN.md` before installer, contract, test, or documentation changes.

- [x] Task 2 - Migrate the installer to one canonical generated-config tree
  - Acceptance criteria:
    - Fresh installs create OME/SRS targets in the source-shaped config tree and
      installed `/opt` links resolve to those exact files.
    - A sole legacy flat file migrates byte-for-byte; compatibility paths remain
      bounded and reruns are idempotent.
    - Divergent legacy and canonical files are rejected without data loss.
    - Modes and ownership remain private and assigned to the Docker operator;
      read-only media consumers receive only the operator group needed to read
      generated runtime config while all capabilities remain dropped.
  - Check:
    - The installer now owns source-shaped OME/SRS targets under
      `$config_dir/deploy/...`; the installed `/opt` paths and bounded former
      flat paths resolve to those same regular files.
    - A migration helper moves a sole flat file, collapses byte-identical dual
      files, accepts only the expected compatibility link, and rejects an
      unexpected link, non-file, canonical symlink, or divergent copies before
      staging new program assets.
    - Fresh and migrated files retain mode `0640`, operator UID/GID ownership,
      and mode `0750` parents. The environment remains mode `0600`. Linux
      lifecycle tests preserve legacy hashes,
      prove fresh/idempotent layouts, reject conflicts without changing either
      config or env bytes, and pass end-to-end in the pinned Go 1.26.5 container.
      Git Bash syntax checks pass.

- [x] Task 3 - Update tests and the deployment/operator contract
  - Acceptance criteria:
    - Installer lifecycle tests cover fresh, legacy, idempotent, conflict, path,
      mode, and ownership behavior.
    - A package-style non-root fixture proves both renders, token verification,
      and consumer-visible config use through the canonical paths.
    - Contract and operator docs name one generated-config layout and include
      the upgrade compatibility behavior.
  - Check:
    - Extended the Linux lifecycle to seed RC17-style flat files, require
      byte-identical migration, verify exact `/opt` and compatibility link
      targets, private parents/files, operator ownership, fresh installs,
      idempotent reruns, equal-copy collapse, and conflict refusal without env
      or config mutation.
    - OME now creates or refreshes secret-bearing output with mode `0640`; the
      SRS renderer explicitly applies the same mode. Focused Go tests cover the OME
      mode and static installer/Compose/SRS invariants; the complete Linux
      lifecycle passes after the hardening.
    - An isolated package-shaped volume with UID/GID 1001, mode-`0640` seeded
      files, and the exact signed RC17 OME helper digest rendered SRS and OME,
      verified the OME health token, and produced non-empty 5913-byte/1859-byte
      consumer paths. All exact fixture volumes and its synthetic env were
      removed.
    - Updated the deployment contract, Ubuntu path table/migration guidance,
      upgrade runbook, and deploy map. Generated contract docs, a disposable
      40,841-byte contract snapshot, Markdown links, shell syntax, and diff
      hygiene pass.

- [x] Task 4 - Verify, merge, publish RC18, and rerun qualification
  - Acceptance criteria:
    - Focused checks, literal verifier, secret guard, protected PR CI, and
      exact-main CI pass without touching operator-owned files.
    - Immutable RC18 is published only from exact green main; all public assets,
      signed release root, and five image signatures verify independently.
    - No-checkout clean-host qualification advances through activation, OME,
      smoke, recovery, upgrade, and retained uninstall or produces a new bounded
      forward-fix failure. External XOA/NPM/reboot/media gates remain open.
  - Check:
    - Literal `./scripts/verify.sh` passed with pinned Go 1.26.5 and the bundled
      Python interpreter: release-bundle, all Go packages, architecture,
      dependency, release-set, Markdown, contract, and hygiene gates are green.
      Docker checks were deliberately separated from that invocation; the
      unchanged viewer was not forced.
    - Docker Desktop 4.85.0 (Linux amd64 engine 29.6.2, Compose 5.3.1) built and
      started the canonical source stack from an ephemeral environment copy.
      Migrations completed, all critical services including OME became healthy,
      and `/healthz`, `/readyz`, `/viewer`, and `/admin` returned HTTP 200. The
      exact test stack and its newly-created volumes were removed afterward.
    - The final pinned Linux installer lifecycle, focused Go tests, package-style
      UID/GID 1001 render fixture, ShellCheck 0.11.0, production/pull third-party
      digest guard using RC17's public dependency manifest, generated-contract
      check, contract snapshot, Markdown links, and `git diff --check` pass.
    - The root `.env` remains byte-identical at SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`;
      the regenerated tracked OME placeholder has the exact `HEAD` object hash,
      and all six operator-owned untracked paths remain excluded.
    - Protected run `31347682245` passed every required job after the PR
      scorecard was corrected. PR #1386 was squash-merged as exact main commit
      `4e496e389d2a606976a55a89cd08ef79762eed2f`; exact-main CI run
      `31348092520` also passed every required job.
    - Protected run `31346223822` passed secrets, docs, ShellCheck, image scan,
      and native arm64 viewer checks, then failed the Ubuntu smoke because SRS
      correctly lacked permission to read an operator-owned mode-`0600` config
      with all capabilities dropped. The merge gate failed closed. Forward fix:
      enforce mode `0640` on every OME/SRS render and give only the selected
      operator GID to the read-only OME/SRS consumers; rerun the complete gate.
    - The forward fix passes focused Linux Go tests, the complete installer
      lifecycle, ShellCheck 0.11.0, the regenerated 40,877-byte contract
      snapshot, Docker Desktop Compose rendering, the production digest guard,
      and a capability-dropped runtime fixture. In that fixture actual SRS and
      OME processes remained running with read-only mode-`0640` configs owned by
      UID/GID 1001, `cap_drop: ALL`, and only supplementary GID 1001. Its exact
      containers and volumes were removed. Literal `./scripts/verify.sh` also
      passes again; protected CI rerun remains pending.
    - Corrected run `31346863473` again passed every independent check but the
      Ubuntu smoke still gave SRS group `0`: its Linux-only override assigns the
      runner GID to renderers without supplying that synthetic GID to the media
      consumers. The installed-host env contract is unaffected. Align only the
      smoke SRS/OME `group_add` values with its local `host_gid`, retain the
      transcoder image UID, and rerun the protected matrix.
    - The aligned smoke override passes focused Linux Go coverage, ShellCheck
      0.11.0, syntax/diff hygiene, and another literal `./scripts/verify.sh` run.
      Third run `31347182741` passed Ubuntu test-all in 5m23s, Windows/macOS Go,
      all three quickstart entrypoint checks, scans, ShellCheck, docs, and secret
      checks. Its merge gate failed only because the PR body did not use the
      machine-checked release-scorecard template. PR #1386 now contains the
      exact headings and explicit high-risk/evidence checkboxes; the fresh
      synchronize run passed before merge.
    - RC18 release run `31348429963` passed all 33 jobs and published 46 public
      assets. Independent download verification matched all 45 checksum-covered
      files; release-set SHA-256 is
      `2de0cb71af2af8d41d7a535d28478003987d6b180568a84bc5dc3eae680ef014`.
    - No-checkout run `31348986535` verified/install/activated RC18, proving the
      generated-config path and runtime group-read correction. It then exposed
      a new bounded post-start smoke defect: full doctor rejects the stack's own
      occupied ports. Sanitized evidence and cleanup passed; RC18 is rejected
      and the forward fix is the next scoped change above.

## Scoped change: live-room chat target and gap reconciliation (#1272)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Audit the epic against the shipped chat stack
  - Acceptance criteria:
    - #1272 and its existing follow-ups are inspected live.
    - Viewer, protocol, gateway, responsive layout, and tests are inspected
      read-only before planning or implementation.
    - `PLAN.md` records scope, evidence boundaries, risks, and tests first.
  - Check:
    - #1272 is open; foundation issues #1273, #1274, and #1275 are closed as
      completed. No checked-in target-state spec currently satisfies the epic.
    - The viewer ships the compact dock/mobile layout, bounded transcript,
      accessible live log, scroll preservation, system/moderation rows, reports,
      row actions, and supported slash commands over the existing gateway.
    - The backend ships presence, role/badge, and system event shapes, but the
      viewer roster still derives from recent messages, badges are not visibly
      rendered, and `BroadcastSystemEvent` has no production caller.
    - Updated `PLAN.md` before changing product documentation or issue state.

- [x] Task 2 - Check in the human-readable target-state specification
  - Acceptance criteria:
    - The spec explicitly defines an adapted BitRiver Live experience, not an
      ivlog.tv clone, and covers all eleven scope areas in #1272.
    - A feature-to-protocol matrix distinguishes shipped, partial, missing, and
      later behavior without overstating current implementation.
    - Architecture ownership and the prohibition on a second chat stack are
      explicit; discoverability links are added to current product docs.
  - Check:
    - Added `docs/live-room-chat.md` with an explicit adapted-not-copied
      boundary, responsive layout, message/roster/role/system/moderation
      anatomy, accessibility, busy-room performance, MVP/later scope, and
      architecture ownership.
    - The feature matrix maps each visible target to named current protocol,
      gateway, viewer, and layout sources and labels it shipped, partial,
      required, or later. It records the recent-chatter roster, unrendered badge
      data, history metadata loss, missing production system-event caller, and
      moderation-event audience policy as gaps.
    - Linked the product contract from `docs/ui-ux-model.md` and the canonical
      frontend architecture boundary. The local-link check for tracked docs and
      `git diff --check` pass; the new spec is validated again after staging so
      the repository checker includes it.

- [x] Task 3 - Create and link focused follow-up issues
  - Acceptance criteria:
    - Existing foundation issues remain linked with their completed scope.
    - Each verified current gap has one bounded, testable follow-up rather than
      an unowned aspirational bullet.
    - The spec links the resulting issue numbers and gives an implementation
      sequence for the current MVP and later extensions.
  - Check:
    - Filed #1382 for authoritative viewer presence, counts, badges, and safe
      history metadata parity; #1383 for server-enforced moderation/automod
      audience policy; and #1384 for de-duplicated production live/offline room
      notices.
    - Each issue links #1272 and the product contract, names the verified
      current gap, preserves the single chat stack, and has testable backend,
      viewer, compatibility, performance, or failure-path acceptance criteria.
    - The spec links completed foundation #1273-#1275 and sequences remaining
      MVP #1382, #1383, then #1384. Later commands, message management, pinned
      announcements, and room modes stay outside MVP until separately approved.

- [x] Task 4 - Verify, publish, merge, and reconcile the epic
  - Acceptance criteria:
    - Markdown links, focused chat tests, literal verifier, secret guard, diff
      review, protected PR checks, and exact-main CI pass.
    - The change excludes credentials and the six operator-owned untracked
      paths and does not mutate the deployment or chat protocol contract.
    - #1272 is closed only after its checked-in spec, mapping, architecture
      boundary, and linked follow-ups meet every acceptance criterion.
  - Check:
    - Focused viewer `chatPanel` tests pass (24/24), pinned Go 1.26.5
      `./internal/chat` tests pass, and the staged Markdown checker includes all
      89 tracked public files with no broken local link.
    - Literal `./scripts/verify.sh` passes with pinned Go 1.26.5 and bundled
      Python: release bundle, all first-party Go packages, architecture,
      dependency, CI, hygiene, secret/example, release-set, docs, and generated
      contract gates are green. Docker is deliberately absent from that
      docs-only tool path and the viewer suite is proven separately above.
    - `git diff --check` passes; root `.env` remains byte-identical at SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`;
      all six operator-owned untracked paths remain excluded.
    - PR #1385 passed protected run `31344956999`, squash-merged as exact main
      `8e9f46f146e7e065cb097a53103a5317fd77230e`, and exact-main run
      `31345003256` passed. #1272 closed as completed, its local/remote topic
      branches were deleted, and the repository returned to zero open PRs.

## Scoped change: artifact-only Ubuntu host qualification (#1297, #1304)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Audit the live clean-host gate and public candidate
  - Acceptance criteria:
    - #1297/#1304 dependencies and remaining evidence are inspected live.
    - The current installer, package, systemd, release-set, image-signature,
      OME, smoke, documentation, and public RC assets are inspected read-only.
    - `PLAN.md` records scope, trust boundary, external limitations, risks, and
      tests before workflow or documentation changes.
  - Check:
    - #1296 and #1300 are closed; #1297 is the next P0 dependency while #1298
      remains sequenced after clean-host and backup/restore proof. #1304 is open
      and assigns initial host/Docker/OME recovery evidence to the same target.
    - Public `v1.2.3-rc.13` contains the signed release-set root, exact amd64
      `.deb`, all five exact image signature bundles, checksums, SBOMs, and the
      passed pull-only product report. Its release-set SHA-256 is
      `795fffee84662aec91624eb4352b9c1a9ef5c34b17838939adaf567418797fa0`.
    - The existing installer already provides safe two-phase systemd lifecycle,
      non-root Docker operation, bounded startup, OME diagnostics, and retained
      config/data uninstall. Existing release CI proves package payloads in
      containers but not a clean VM systemd/Docker lifecycle.
    - README, quickstart, Ubuntu install, production status, and draft notes
      still call RC12 current, even though RC13 is the first candidate eligible
      for signed-root qualification and stable-promotion evidence.

- [x] Task 2 - Add the no-checkout Ubuntu qualification workflow
  - Acceptance criteria:
    - Manual-only Ubuntu 24.04 amd64 workflow uses no repository checkout and
      grants only read access to public release metadata.
    - Exact root/package/image signatures and checksums bind all installed and
      pulled bytes to one candidate tag, commit, and release-set SHA-256.
    - Package, non-root installer, doctor/env, systemd activation, smoke,
      authenticated OME, OME/Docker/systemd restart, status, and retained-data
      uninstall paths are bounded and asserted.
    - A sanitized versioned report is uploaded on pass or failure and states
      the XOA/NPM/reboot/media evidence that the hosted VM cannot prove.
  - Check:
    - Added manual-only `clean-host-qualification.yml` on a pinned Ubuntu 24.04
      amd64 runner with `contents: read`, no checkout action, an explicit empty-
      workspace assertion, bounded job/recovery timeouts, and no registry login.
    - The workflow verifies the caller-supplied release-set SHA-256, exact tag/
      repository/commit/workflow identity, checksum-matched Cosign version,
      signed root, five exact image bundles, package hash, and dependency pins
      before installing the public `.deb`.
    - It stages the package's systemd manager for the non-root runner, writes
      only public URLs and signed digest pins into the generated private env,
      then proves validation, doctor, pull-only activation, smoke, authenticated
      OME control, aggregate OME health, viewer content, same-tag upgrade, OME
      restart, Docker daemon restart, full systemd restart, and config/data-
      preserving manager plus package removal.
    - An always-run evidence step emits a versioned JSON/Markdown report, safe
      systemd/Compose state, explicit XOA/NPM/reboot/media limitations, and
      scans every retained file against generated secret values before a pinned
      30-day artifact upload. Workflow YAML parsing, repository CI-contract
      checks, and `git diff --check` pass.

- [x] Task 3 - Add workflow contract tests and focused verification
  - Acceptance criteria:
    - Tests fail on checkout/source use, weak permissions, missing signature or
      lifecycle gates, secret-bearing evidence, unbounded recovery, unsafe
      uninstall, absent retention assertions, or overclaimed external proof.
    - Workflow YAML and relevant focused checks pass.
  - Check:
    - Added focused Go workflow contracts covering manual-only/no-checkout/
      read-only execution, root and five-image provenance, checksum-matched
      Cosign, anonymous registry use, verify-before-install ordering, package/
      systemd/doctor/smoke/OME lifecycle, bounded restart timeouts, ordinary
      uninstall retention, versioned reports, pinned uploads, secret scanning,
      and explicit XOA/NPM/media limitations.
    - The tests reject repository scripts/source use, write/package/OIDC
      permissions, checkout/login/build seams, journal/container-log retention,
      environment or generated-config upload, missing cleanup assertions, and
      missing artifact retention policy.
    - Isolated Go 1.26.5 `go test ./scripts -run TestCleanHostQualification`,
      js-yaml parsing, `check-ci-contract.sh`, and `git diff --check` pass. The
      first Go attempt did not run because sandbox policy denied a new
      `C:\\tmp` cache; the successful rerun uses uniquely scoped workspace
      caches without touching retained operator/evidence paths.

- [x] Task 4 - Refresh first-install and release-candidate documentation
  - Acceptance criteria:
    - README, quickstart, Ubuntu guide, production status, release notes, gates,
      and testing guidance identify RC13 and its signed root accurately.
    - Hosted qualification versus real XOA/NPM/reboot/firewall/media acceptance
      is explicit, and provisional platform support is not promoted.
    - Stale no-installer/RC12-current claims are removed from operator docs.
  - Check:
    - README, quickstart, Ubuntu install, production status/release, testing,
      release gates, viewer deployment, advanced deployment, changelog, release
      index, and stable draft now identify signed RC13 rather than presenting
      RC12 or a future candidate as current.
    - Added the RC13 note with exact commit/run/signed-root hash, 46-asset/45-
      checksum coverage, five image digests, install choices, product evidence,
      and all still-open external gates. Historical RC12 notes remain intact.
    - Ubuntu and release guidance explains the no-checkout hosted workflow as
      partial package/systemd/Docker/OME evidence, then requires the same bytes
      on the real XOA/NPM/firewall/reboot/media topology before promotion.
    - Removed the stale advanced-deployment claim that no package exists and
      updated the installer-language guard itself to require RC13 plus its
      release note. Installer-language checks, focused workflow tests, YAML
      parsing, and `git diff --check` pass.

- [-] Task 5 - Verify, merge, run live qualification, and retain evidence
  - Acceptance criteria:
    - Focused checks, literal verifier, committed-secret guard, PR CI, protected
      aggregate gate, and exact-main CI pass without operator-owned files.
    - The merged manual workflow runs against exact RC13 and its signed-root
      hash; its public artifact is independently inspected and retained durably.
    - #1297/#1304 receive bounded evidence without closing external gates or
      enabling stable promotion prematurely.
    - Config helpers write and verify generated files through the dedicated
      config-root mount while the installed/source workspace remains read-only.
  - Check:
    - Local focused checks, the literal verifier, PR #1369 CI, its protected
      merge gate, and exact-main run `30819170684` passed. PR #1369 merged as
      `2c8dc599541f1879b9415ff008d992a3487da71c` without touching the private
      root `.env` or the six retained operator-owned paths.
    - The first RC13 dispatch was rejected before GitHub created a run because
      job-level `env` cannot evaluate the `runner.temp` expression context.
      This is parser evidence only: no package, container, release, or evidence
      mutation occurred. Runtime `$RUNNER_TEMP` initialization, a regression
      contract, full revalidation, protected merge, and redispatch are in
      progress.
    - The correction removes all `${{ runner.* }}` expressions, derives the
      three isolated paths from `$RUNNER_TEMP` in the first Bash step, and
      exports them through `$GITHUB_ENV` only for later steps. Focused Go 1.26.5
      workflow contracts, js-yaml parsing, the repository CI-contract check,
      and `git diff --check` pass.
    - The literal `./scripts/verify.sh` gate passes against a temporary RC13
      production env with the signed five first-party and eight dependency
      digests: Go/tests/contracts/docs/migrations, Compose render, image builds,
      healthy Postgres/Redis/SRS/OME/transcoder/API/viewer, and quickstart all
      pass. Its isolated Compose project leaves zero containers/volumes; the
      private `.env` is restored to SHA-256 `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`,
      and all six operator-owned paths remain present.
    - Parser correction PR #1370 passed refreshed protected CI run `30821426755`
      after the required PR release scorecard was supplied, merged as
      `2789b8b614791bacd65006e1544bba23ed9d49f1`, and passed exact-main run
      `30822039774`; its remote branch was deleted.
    - Live run `30822618916` established the no-checkout boundary and verified
      the signed release-set blob, then failed before package installation:
      Cosign 3.1.2 container `verify` rejects its unsupported `--bundle` flag.
      The sanitized failure/cleanup steps passed. Exact-binary diagnosis proves
      the five downloaded image bundles match their signed image entries,
      signed artifact entries, and `CHECKSUMS.txt`, while supported anonymous
      registry-backed verification succeeds for all five immutable digests.
      Bundle-byte assertions, the supported container command, revalidation,
      protected merge, and redispatch are in progress.
    - The correction now requires each downloaded image bundle SHA-256 to match
      its signed image record, signed artifact record, and checksum entry before
      supported registry-backed verification of the exact digest. Its contract
      rejects `--bundle` specifically from the Cosign container-verify block.
      Focused Go 1.26.5 contracts, js-yaml parsing, CI policy, committed-secret,
      and `git diff --check` gates pass.
    - The literal verifier passes again with the RC13 five-image/eight-dependency
      production pins, including all repository checks, Compose render/build,
      healthy OME and dependent services, API/viewer smoke, and cleanup. The
      isolated project leaves zero containers/volumes, the six operator paths
      remain present, and the private `.env` returns to its exact recorded hash.
    - Cosign correction PR #1371 passed protected run `30823439715`, merged as
      `e24ffe7cc07a90169825f8ded8c021281fc15bb3`, passed exact-main run
      `30824059167`, and had its remote branch deleted.
    - Live run `30824712866` passed signed provenance, public `.deb` install,
      disabled systemd staging, env validation, and host doctor. It stopped
      before activation because `config --images` excluded optional profiles,
      so the audit could not see the signed Alpine images used only by
      `postgres-host` and `srs-api`. Sanitized evidence and cleanup passed.
      All-profile digest rendering, regression coverage, revalidation,
      protected merge, and exact-input redispatch are in progress.
    - The audit now renders `docker compose --profile '*' ... config --images`
      without starting optional services. Exact RC13 reproduction shows the
      default render misses only the two optional-profile digests, while the
      all-profile render contains all 15 service references and all 13 unique
      signed digests. Focused Go 1.26.5 contracts, js-yaml, CI policy,
      committed-secret, and diff checks pass.
    - The literal verifier passes again with the signed production pins and
      healthy OME/dependency/API/viewer smoke. Cleanup leaves zero isolated
      containers/volumes, preserves all six operator paths, and restores the
      private `.env` to its exact recorded hash.
    - Optional-profile audit PR #1372 passed protected run `30825405672`, merged
      as `d0c2825b760be346065e126522ce24ecf21a6bb0`, passed exact-main run
      `30826129909`, and had its remote branch deleted.
    - Live run `30827018880` passed signed release/package verification, public
      `.deb` installation, disabled systemd staging, immutable environment
      validation, doctor, and the complete signed-digest audit. Activation then
      failed because the packaged unit rendered
      `WorkingDirectory="/opt/bitriver-live"`; systemd treats the quotes as part
      of the path and rejects it as non-absolute. Sanitized evidence upload and
      cleanup passed. RC13 is rejected for clean-host qualification; the unit
      template fix, rendered-unit regression, full gates, next immutable RC14,
      and RC14 qualification are in progress without patching installed RC13.
    - The canonical package template now renders an unquoted absolute
      `WorkingDirectory`, and the installer lifecycle test requires the exact
      fully substituted directive. Git Bash syntax, `git diff --check`, and the
      complete installer lifecycle in the existing `golang:1.26.5-bookworm`
      Linux container pass, including rerunnable install, retained uninstall,
      guarded purge, and confirmed purge.
    - Focused Go `./scripts` tests pass offline with the repository-required
      Go 1.26.5 after the host-default Go 1.25.6 correctly refused the 1.26
      module. CI contract, committed-secret, installer-language, and diff gates
      pass. Literal `./scripts/verify.sh` passes Go, release bundle, contracts,
      docs, migrations, and repository policy with the bundled Python runtime.
    - A Docker-visible verifier rerun reached Compose validation but the private
      operator `.env` predates `BITRIVER_OME_PUBLIC_LLHLS_BASE_URL`; no service
      was started. A process-only retry supplying just missing non-secret keys
      from the tracked example was blocked by the local execution usage limit.
      The private `.env` was never edited or printed and retains SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`.
      Fresh Ubuntu PR CI remains mandatory before merge or RC14 publication.
    - PR #1378 opened from the focused branch. After rebasing onto current main,
      the Windows worktree rematerialized the systemd template as CRLF and the
      strict Linux fixture correctly rejected the trailing carriage return.
      Added an exact `.gitattributes` LF rule for the packaged Compose unit and
      normalized the worktree; `git ls-files --eol` now reports `i/lf w/lf`,
      and the complete Linux installer lifecycle passes again. Focused Go,
      CI-contract, committed-secret, installer-language, and diff checks pass
      on the rebased head.
    - A first Docker-visible full-verifier attempt supplied missing Compose keys
      as global process variables and correctly caused environment-isolation
      unit tests to fail; it was discarded as harness contamination. The clean
      literal `./scripts/verify.sh` rerun passes all applicable Go, policy,
      bundle, docs, migration, and contract checks without overrides. A separate
      `verify-windows-docker.ps1` run with an isolated credential-rotated temp
      env passes Docker Desktop 29.6.2 / Compose 5.3.1 canonical config proof.
      The changed Linux installer boundary passes in Docker, while fresh Ubuntu
      PR CI remains the authoritative integrated gate.
    - Rebased PR #1378 head `e11f5767` passed protected run `31333507523`:
      Ubuntu test-all, macOS/Windows Go tests, Ubuntu/macOS/Windows quickstart
      entrypoints, ShellCheck, committed-secret guard, and the aggregate Merge
      gate are green. A final ledger-only commit and fresh protected run precede
      squash merge; RC14 remains untagged until exact-main CI also passes.
    - The ledger update passed protected run `31333807382`; PR #1378 then
      squash-merged as exact main
      `9f97c1533613fd9a1cb40353c2df1d159c51a2aa`, exact-main run `31334103956`
      passed, and the remote branch was deleted. Obsolete PR #1377 was closed
      without merge after its ESLint-only workflow deletion was audited as an
      incomplete and unsafe CI replacement. The repository now has zero open
      pull requests.
    - Immutable `v1.2.3-rc.14` published from exact green main in passed release
      run `31334396479`. Independent verification accepted all 46 public assets;
      the signed-root SHA-256 is
      `ae27e14f4e3883e216c57c145e73d8571838e8b80098afd7145cc6ed9f5923f3`
      and the amd64 `.deb` SHA-256 is
      `ea29e57eac226a56a3de8dc718cc4305fe05b55f94690131f90b4eb0674ccd00`.
    - No-checkout run `31334880405` passed signed provenance, public package
      install, disabled service staging, production env validation, doctor,
      and immutable-image audit. The corrected unit parsed and launched, then
      activation failed because `srs-config` and `ome-config` both exited 1;
      sanitized evidence and cleanup passed.
    - Read-only diagnosis traced both exits to the package's intentional
      absolute symlinks from `/opt/bitriver-live` into `/etc/bitriver-live`.
      The renderer namespace currently mounts only the install root, leaving
      its `/workspace/.env` and generated-config symlinks broken. The focused
      config-root mount correction, contract tests, protected PR, next
      immutable candidate, and qualification rerun are in progress; RC14 is
      not patched or qualified.
    - Compose now mounts topology-specific `BITRIVER_CONFIG_ROOT` at
      `/etc/bitriver-live` writable in `srs-config`/`ome-config` and read-only in
      `ome-health-token-check`. Source defaults remain the repository root;
      the packaged systemd unit supplies its absolute config directory, and
      both installed launchers derive the same boundary from their selected
      env file when invoked directly.
    - Installer and Go regressions require the fully substituted package unit,
      exactly two writable renderer mounts, one read-only verifier mount, and
      the source default. The generated contract index, deployment contract,
      Ubuntu guide, and deploy map describe the new value and secret boundary.
    - An isolated named-volume fixture reproduced the package's absolute
      `/opt`-to-`/etc` symlinks with the exact signed RC14 OME helper digest:
      SRS render, OME render, and read-only OME token verification all passed,
      wrote durable config outputs, and removed both temporary volumes. The
      first fixture attempt copied the Windows worktree's CRLF example env and
      was discarded after Bash rejected its carriage returns; normalized
      release-shaped bytes passed without a product change.
    - Pinned Go 1.26.5 focused and full `./scripts` tests, Linux installer
      lifecycle, shell and PowerShell parsing, CI/secret/docs/contract guards,
      generated-doc check, host `git diff --check`, and Docker Desktop 29.6.2 /
      Compose 5.3.1 rendering pass. Literal `./scripts/verify.sh` passes all Go,
      policy, release-bundle, installer, docs, and contract checks in the
      pinned Linux container; Docker/viewer phases are explicitly unavailable
      there and Docker Compose rendering is proven separately. The private
      `.env` remains byte-identical at its recorded SHA-256.
    - PR #1379 opened at `cf3cf297` with a strict high-risk release scorecard.
      Run `31335824708` passed Ubuntu test-all, blocking image scans, viewer
      lint/tests/build/audit, macOS/Windows Go, all entrypoint checks,
      ShellCheck, docs, secret guard, and the aggregate Merge gate.
    - Automated review then identified a valid first-run edge case: the Unix
      wrapper resolved a custom env parent before `ensure_assets` could create
      it. Config-root derivation now occurs after env seeding, the static test
      requires that ordering, and the Linux lifecycle executes the launcher
      with a missing custom directory plus fake Docker/CLI boundaries. Focused
      Go, shell syntax, and the complete installer lifecycle pass.
    - Refreshed protected run `31336273494` passed, PR #1379 squash-merged as
      `91525a32f43d202561a9cc12d9d5452f48813f16`, exact-main run
      `31336615980` passed, and the remote branch was deleted. The repository
      again had zero open pull requests.
    - Immutable `v1.2.3-rc.15` published in passed run `31336980513` from that
      exact main commit. Independent public verification accepted all 46
      assets, the signed release root, and all five exact image signatures; the
      release-set SHA-256 is
      `69acc96dae732fe9e6eea6f0a8953f57bd3796a5ddc373fec81872a81c9abb2e`
      and the amd64 `.deb` SHA-256 is
      `d54347660312ded8266387efa65a6abe543ac9ab3cd52532b6fdcf64942e4202`.
    - No-checkout run `31337618217` passed provenance, package install, host
      staging, immutable input configuration, environment validation, and
      doctor, then failed activation when both config renderers exited 1.
      Sanitized evidence and cleanup passed; RC15 remains immutable and
      rejected. Issue #1297 was reopened because its XOA/NPM/reboot/media
      acceptance criteria remain unmet.
    - The exact public package renders SRS and OME successfully against its
      installed absolute symlinks when `/etc/bitriver-live` is mounted. The
      installed env still persists source default `BITRIVER_CONFIG_ROOT=..`,
      leaving package/manual Compose dependent on a transient process override.
      Persisting exactly one absolute installer-owned value, retaining all
      unrelated operator values, and rerunning package/CI/release/qualification
      evidence are in progress.
    - The host installer now supplies `BITRIVER_CONFIG_ROOT` to every
      operator-scoped command and atomically persists its absolute config
      directory in `bitriver.env`. It replaces one managed value, appends the
      key for older package env files, rejects duplicates before rewriting,
      uses a mode-0600 temporary file, and never prints env contents.
    - The Linux lifecycle regression now starts from source default `..`,
      requires the absolute persisted value on first install and idempotent
      reinstall, proves older-env append plus unrelated-value retention, and
      proves duplicate refusal leaves no temporary secret file. Ubuntu,
      deployment-contract, and deploy-map docs describe the persisted boundary.
    - Pinned Linux shell syntax/lifecycle and full `go test ./scripts` pass.
      Pinned nFPM built/inspected stable and prerelease amd64/arm64 `.deb`/`.rpm`
      payloads. Applying the corrected installer to the exact public RC15
      installed-layout volumes persisted exactly one absolute value; concurrent
      SRS/OME render and read-only OME token verification then passed with the
      exact signed RC15 helper digest. The three isolated volumes were removed.
    - Literal `./scripts/verify.sh` passed with pinned Go 1.26.5, covering the
      full Go suite, release bundle, installer lifecycle, release-set/docs/
      architecture/contracts, and policy guards; Docker/viewer were explicitly
      unavailable in that container. The first read-only worktree invocation
      reached the Go suite but could not create the secret-scanner's designated
      `.tmp` fixtures, so it was discarded and rerun with the verifier's normal
      writable fixture boundary. Windows Docker Desktop 29.6.2 / Compose 5.3.1
      rendered the canonical contract. Installer wording, generated docs,
      committed-secret guard, host `git diff --check`, and changed-script
      ShellCheck (apart from pre-existing SC2034) pass. The private `.env`
      remains byte-identical at its recorded SHA-256.
    - PR #1380 opened at `c42f8210`. Protected run `31338627256` passed the
      Ubuntu test-all gate, macOS/Windows Go, all three quickstart entrypoints,
      remote ShellCheck, docs consistency, and committed-secret guard. The
      aggregate Merge gate correctly failed because the initial PR body omitted
      the mandatory high-risk release scorecard. No check was bypassed: the PR
      body now records deployment/package/operator classification, high risk,
      test/manual evidence, secret and rollback boundaries, and remaining
      external gates; a fresh pull-request event is required before merge.
    - Refreshed run `31338969678` passed every protected check. PR #1380
      squash-merged as `0a6be1600b08bcf2a4caeca6c580698f7e2f8ce4`, exact-main
      run `31339266505` passed, and its remote branch was deleted. The
      repository again had zero open pull requests.
    - Immutable RC16 published from that exact main commit in passed release
      run `31339591099`. Independent inspection verified all 46 public assets,
      signed-root SHA-256
      `c4d34bd82264995723a679b88e497c1a02aa192a99ac8bf3458de771b7b34b79`,
      amd64 package SHA-256
      `a8e02419b1fbc51e2477030e32cca69145aa25d3be259dca513c3f77cdd5363a`,
      and all five exact image signatures.
    - No-checkout run `31340245262` passed provenance, public package install,
      disabled host staging, persisted config-root selection, immutable input
      configuration, validation, doctor, and complete digest rendering. Both
      renderer jobs still exited 1 during activation; sanitized evidence and
      cleanup passed, so RC16 is immutable and rejected.
    - Read-only diagnosis identifies the remaining Linux-only ownership defect:
      private config/data binds are owned by the non-root operator, while
      renderer, API, and transcoder services use unrelated container UIDs with
      every capability dropped. Persisting the operator UID/GID, applying it
      only to bind-writing services, and narrowing renderer mounts are now the
      next top-to-bottom correction before RC17.
    - The focused correction now persists `BITRIVER_HOST_UID`/
      `BITRIVER_HOST_GID` atomically with the config root, supplies all three
      values to operator commands and systemd, and uses image-specific UID/GID
      fallbacks outside managed Linux starts. API/transcoder and all three
      config helpers use the selected owner; renderer workspaces are read-only,
      the SRS sanitized copy runs from tmpfs, and no capability or secret mode
      was loosened. Bash syntax and the complete pinned-Linux installer
      lifecycle pass, including first install, idempotent reinstall, legacy-env
      append, unrelated-setting retention, multi-key duplicate refusal before
      mutation, wrapper derivation, retained uninstall, and guarded purge.
    - A permission-faithful named-volume fixture extracted the exact public
      RC16 amd64 package layout, reproduced mode-0750/0600 UID/GID-1001 config
      and data ownership, and ran the exact signed RC16 Debian dependency, OME
      helper, API, and transcoder images as `1001:1001` with all capabilities
      dropped and read-only roots/assets. SRS render, OME render, read-only token
      verification, API durable write, and transcoder durable write passed;
      every output remained owned by 1001 and all four exact fixture volumes
      were removed. The first tmpfs attempt correctly exposed `noexec`; the
      product now invokes the sanitized SRS file through Bash instead of
      weakening the tmpfs mount.
    - The required contract-doc generator and contract snapshot include both
      host-identity keys. Full pinned `go test ./scripts`, focused environment
      validation, Docker Desktop Compose rendering with explicit `1234:2345`,
      production third-party digest enforcement against the signed RC16 set,
      changed-script ShellCheck 0.11.0, installer wording, and committed-secret
      guards pass. Literal `./scripts/verify.sh` passes the full Go suite,
      release bundle, installer, release-set, Markdown, architecture, contract,
      migration, and repository policy gates; Docker/viewer are explicitly
      unavailable in that pinned Linux verifier and are covered separately.
      Managed-key duplicate detection also rejects whitespace-obfuscated active
      entries before mutation. The private root `.env` remains byte-identical.
    - PR #1381 protected run `31341715983` passed committed-secret, docs,
      ShellCheck, native arm64 viewer, and blocking image-scan jobs. Its Ubuntu
      smoke failed when `ome-config` named
      `/workspace/deploy/ome/Server.generated.xml` after the workspace became
      read-only; the aggregate gate failed as designed. Both renderers must now
      target the existing writable `/etc/bitriver-live/deploy` alias and the
      verifier must consume that same path before refreshed CI can merge.
    - Compose now directs OME and SRS output through that writable alias and
      verifies the OME token from the same path; no helper writes through
      `/workspace`. The focused pinned-Linux Compose regression passed. An
      isolated Docker Desktop config root then ran both real renderers and the
      read-only token verifier successfully, produced 5,916-byte OME and
      1,919-byte SRS outputs, left the tracked generated config unchanged, and
      removed its exact temporary directory, containers, and network.
    - Full pinned `go test ./scripts`, ShellCheck 0.11.0, Docker Compose render,
      `git diff --check`, and the literal `./scripts/verify.sh` pass on the
      corrected diff. The literal verifier covered all Go packages, installer,
      release bundle/set, docs, architecture, contract, migration, and policy
      checks; Docker/viewer were explicitly unavailable in that verifier and
      the changed Docker renderer path was exercised separately above.
    - Automated review's P1 renderer-path finding is fixed by the config-root
      output change. Its P2 owner-precedence finding is also fixed: the Unix
      quickstart wrapper inspects the selected env path without sourcing or
      printing it, preserves a declared UID/GID pair for Compose, leaves a
      partial declaration to validation, and derives both current IDs only
      when process and file values are absent. Pinned Linux execution tests pass
      for split/equal env-file flags and empty-value fallback; Bash syntax and
      ShellCheck 0.11.0 pass for both modified shell entrypoints.
    - The literal verifier was rerun after both review corrections and passed
      the same complete applicable gate set on the final diff. No generated
      config, private env, runtime media, or retained operator path changed.
    - Corrected head `c5dc3165` passed protected PR run `31342402729`: Ubuntu
      test-all and its previously failing real source renderer path, viewer
      lint/tests/build/audit, blocking image scans, native arm64 viewer, macOS/
      Windows Go, all three quickstart entrypoints, docs, ShellCheck, secret
      guard, and the aggregate Merge gate are green. Both automated review
      threads have evidence replies and are resolved. A final ledger-only run
      precedes squash merge; RC17 remains untagged until exact-main CI passes.

## Scoped change: immutable release sets and stable promotion (#1301, #1271, #1302)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Audit current publication, provenance, and live stable state
  - Acceptance criteria:
    - #1293/#1301/#1271 dependencies, release jobs/artifacts/tests/docs, current
      candidate evidence schemas, GitHub environments, stable tags/releases,
      and all public `latest` aliases are inspected read-only.
    - `PLAN.md` records the immutable boundary, transaction/idempotence risks,
      evidence contract, permissions, environment rollout, and test plan before
      release workflow implementation.
  - Check:
    - `release.yml` currently builds on every `v*` tag, publishes stable aliases
      and `latest` inside image build jobs, and has no release-set manifest or
      separate promotion workflow. Stable tags therefore rebuild instead of
      promoting candidate bytes.
    - Public `v1.2.3-rc.12` has 33 checksum-covered assets and five anonymous
      image digests, but its sanitized contract/product/publication evidence is
      retained only as Actions artifacts and it has no signed provenance root.
    - The repository has no GitHub environments, no `v1.2.3` tag/release, and
      no public `latest` alias for any of the five first-party images. #1271,
      #1301, and the promotion half of #1302 are open; #1294 and #1300 are
      closed while required external production gates remain open.

- [x] Task 2 - Add deterministic release-set and promotion-record fixtures
  - Acceptance criteria:
    - A standard-library generator emits deterministic JSON/Markdown release-
      set manifests and verifies exact artifact/checksum/image/evidence content.
    - Promotion records bind every required gate and durable evidence digest to
      one candidate manifest hash and the matching base stable version.
    - Fixtures reject tampering, missing/duplicate/path-traversal assets,
      incomplete or cross-candidate gates, revoked candidates, tag/version
      mismatch, and unsafe idempotence state.
  - Check:
    - Added standard-library `release_set.py` with deterministic candidate JSON/
      Markdown generation, sorted complete checksums, full payload verification,
      strict promotion records, stable/rollback metadata, revocation markers,
      and existing-state classification.
    - The contract requires the complete cross-platform archive/installer set,
      five canonical SBOMs and image signature bundles, sanitized contract/
      dependency/image/product/scan evidence, exact tag/commit/reference/digest
      relationships, one signed provenance root, and eight external gates bound
      to the same release-set SHA-256.
    - Eight Python fixtures pass for deterministic output, complete verification,
      tampering/uncovered assets, missing signatures, path traversal, incomplete
      and cross-candidate promotion records, revocation, first-stable rollback,
      safe resume/complete state, mismatched existing state, and tag contracts.
    - Python compilation, Bash verification-entrypoint syntax, isolated-cache Go
      1.26.5 `go test ./scripts`, and `git diff --check` pass.

- [x] Task 3 - Publish signed candidate release sets without stable aliases
  - Acceptance criteria:
    - Candidate tags alone build; stable tags cannot enter the build workflow
      and no candidate job creates `latest`.
    - Exact image digests and the release-set root are keylessly signed and
      verified against the exact release workflow identity.
    - Candidate releases attach manifest JSON/Markdown, signature bundles,
      evidence, SBOMs, and checksum coverage only after scanner-approved gates.
  - Check:
    - `release.yml` now accepts prerelease-shaped tags only, contains no
      `:latest` or stable-tag publication path, and always marks the resulting
      GitHub Release as a prerelease.
    - Four generic images and the native multi-architecture viewer manifest are
      keylessly signed at exact digests; the pull-only product gate verifies all
      five against the exact tag-ref workflow identity before Compose starts.
    - The release job fail-closes on duplicate/missing evidence, scans the
      downloaded and flattened payload, attaches sanitized evidence and five
      image bundles, signs `release-set.json`, creates sorted checksum coverage,
      re-verifies the complete candidate set, and publishes only `dist/*`.
    - Workflow YAML parsing, Git Bash syntax, isolated-cache Go 1.26.5
      `go test ./scripts`, and `git diff --check` pass. The eight release-set
      Python fixtures passed in Task 2; this host currently has no Python
      interpreter on PATH, so the literal verifier/CI rerun remains in Task 6.

- [x] Task 4 - Add guarded stable promotion and candidate revocation
  - Acceptance criteria:
    - A read-only stable gate validates tracked evidence, release assets,
      signatures, digests, revocation, issue state, and idempotence before writes.
    - The write job uses least privilege plus the `stable-promotion` environment,
      performs no build, retags by digest, publishes through a draft release,
      emits stable/rollback manifests, and rejects mismatched existing state.
    - Revocation is signed, append-only, and cannot modify stable state.
  - Check:
    - Added manual `stable-promotion.yml` with an unprivileged `Stable promotion
      gate` that downloads a public candidate, checks GitHub asset digests,
      checksum/manifest coverage, exact candidate tag/commit, root and five
      image signatures, tracked promotion-record binding, all eight live issue
      states, revocation overlays, prior rollback root, and existing stable
      state before an environment can be reached.
    - The environment-approved promotion job revalidates live state, refuses
      mismatched tags/assets/aliases, creates deterministic signed stable and
      first/previous rollback metadata, creates a draft release, retags exact
      image digests without build steps, uploads only missing byte-identical
      assets, verifies server hashes, and publishes only after final checks.
    - The separate least-privilege revocation job signs unique run-scoped
      markers, refuses overwrite, appends them to the candidate, and has no
      package permission or stable/image mutation path. Promotion rejects any
      existing revocation marker before approval.
    - YAML parsing, eight Python release-set fixtures (including deterministic
      stable retry metadata), and focused Go 1.26.5 stable-workflow contracts
      pass.

- [x] Task 5 - Update operator, release, security, and verification guidance
  - Acceptance criteria:
    - Docs explain candidate versus stable identity, manifest/signature checking,
      tracked promotion records, environment approval, revocation, rollback,
      optional `latest`, and first-stable no-previous-release behavior.
    - Public install examples use candidate/stable manifest digests as truth and
      do not claim unresolved external gates have passed.
  - Check:
    - Added `docs/release-promotion.md`, a complete example record, and the
      tracked promotion-record directory contract. They document signed-root
      and selected-artifact verification, exact digest identity, environment
      review, live gates, draft/resume behavior, optional non-authoritative
      `latest`, append-only revocation, and first-stable rollback limits.
    - Refreshed README, SPEC, production release/status/gates, Ubuntu install,
      testing/versioning/security guidance, draft v1.2.3 notes, and the release
      notes checklist. RC12 is accurately identified as predating the new root;
      no unresolved target-host gate is presented as complete.
    - The Ubuntu/XOA/NPM workflow keeps authenticated OME control, real ingest/
      playback, reboot, and repeated OME recovery explicit. Stable packages
      retaining RC filenames are explained as byte-identical promotion, not
      stale rebuild output.
    - Installer wording, Markdown-link unit fixtures, focused links for every
      changed/new document, JSON parsing, and `git diff --check` pass.

- [x] Task 6 - Verify, merge, configure, and negatively prove enforcement
  - Acceptance criteria:
    - Focused checks, literal verifier, complete PR CI, and exact-main CI pass
      with private/operator state excluded.
    - Live `stable-promotion` environment readback proves required review and
      protected-branch deployment.
    - An unverified promotion dispatch fails in the read-only stable gate before
      approval/mutation; issue evidence distinguishes implementation from the
      later candidate publication and external stable qualification.
  - Check:
    - Literal `./scripts/verify.sh` passed with Go 1.26.5, containerized Python
      3.13, sanitized immutable RC12 dependency/image evidence, and a disposable
      production-shaped environment. It covered all Go/Python/docs/contracts,
      Postgres migration lifecycle, digest enforcement, Compose rendering,
      first-party/SRS builds, and healthy Postgres/Redis/SRS/OME/transcoder/API/
      viewer startup; the private `.env` was restored to its original SHA-256
      and the `deploy` Compose project retained zero containers/volumes.
    - The full gate first exposed and then proved a repair for build-mode smoke
      digest suffixes: SRS could not build under an `@sha256` tag, and sourcing
      the env later reintroduced remote digests for locally built services.
      One tested override function now clears only buildable digests in build
      mode and reapplies after sourcing; pull/production pins remain enforced.
    - PR #1366 squash-merged as `38cfeb13` after all PR checks and protected
      `Merge gate` passed in run `30793307725`. Exact-main run `30793780301`
      also passed the unified Ubuntu verifier, cross-platform jobs, scans, and
      aggregate gate.
    - Environment `stable-promotion` readback shows required reviewer
      `ProhibitedTV`, self-review allowed for the single-owner project, and
      protected-branch-only deployment.
    - Deliberate negative run `30794248391` failed in the unprivileged `Stable
      promotion gate` because `docs/releases/promotions/v1.2.3.json` is not a
      tracked approval record. Both mutation jobs were skipped and direct
      release/tag inspection confirmed that `v1.2.3` was not created.
    - Evidence comments were posted to #1271, #1301, and already-closed #1302.
      #1271/#1301 and external gates remain open for actual candidate/stable
      outcomes; implementation enforcement is not presented as qualification.

## Scoped change: first signed release-set candidate (#1271, #1301)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Audit the next candidate identity and publication boundary
  - Acceptance criteria:
    - Existing public candidate tags/releases and RC12 assets are inspected.
    - `PLAN.md` identifies the next unused tag, immutable tag boundary, public
      verification requirements, and external stable blockers before tagging.
  - Check:
    - Remote annotated tags exist through `v1.2.3-rc.12`; no RC13 tag/release
      exists, so `v1.2.3-rc.13` is the next unused candidate identity.
    - RC12 has 33 checksum-covered assets but no `release-set.json` or signing
      bundle, matching the documented pre-contract boundary.
    - The plan requires exact green main, candidate-only publication, signed
      public-root inspection, no stable/latest mutation, and fix-forward tags.

- [x] Task 2 - Land the rollout ledger through protected main
  - Acceptance criteria:
    - PLAN/TASKS accurately record the completed promotion rollout and bounded
      RC13 plan without touching runtime/deployment contracts.
    - Focused docs/diff checks, PR CI, protected `Merge gate`, and exact-main CI
      pass with operator-owned paths excluded.
  - Check:
    - PLAN was updated first, followed by this task ledger.
    - The first literal `./scripts/verify.sh` attempt stopped at the Go gate
      because Git Bash resolved host Go 1.25.6 while `go.mod` requires 1.26.0+;
      earlier repository/CI/hygiene/bundle checks passed. This is a local PATH
      mismatch, so the unchanged verifier must be rerun with Go 1.26.5 first.
    - The Go 1.26.5 rerun passed all Go, architecture, release-set Python, link,
      hygiene, and bundle checks, then stopped at Compose interpolation because
      the private root `.env` lacks required `BITRIVER_OME_PUBLIC_LLHLS_BASE_URL`.
      Verification will use environment-only synthetic contract values; the
      private file remains untouched and its hash must be rechecked afterward.
    - Exporting those missing values at process scope was rejected as an
      approach: Go environment-precedence fixtures correctly failed because the
      overrides leaked into isolated test cases. The next run must use a
      supported alternate env file or a safely restored disposable root file,
      never global process overrides or weakened tests.
    - Literal `./scripts/verify.sh` then passed with Go 1.26.5 and a generated,
      application-validated, non-secret production-shaped environment bridged
      under `try/finally`. It covered all Go/Python/docs/contracts, digest
      enforcement, Postgres migrations, Compose rendering, local image builds,
      and healthy Postgres/Redis/SRS/OME/transcoder/API/viewer startup with API
      and viewer endpoints reachable; viewer checks correctly skipped because
      no viewer paths changed.
    - The private `.env` was restored to SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`.
      Only this run's generated verifier env, RC12 evidence download, and
      isolated external Go cache were removed; older `.tmp` evidence and all
      six operator-owned untracked paths were preserved.
    - Only PLAN/TASKS were committed. Committed-secret guard and
      `git diff --check` passed; PR #1367's docs-only protected `Merge gate`
      passed, it squash-merged as `d416968e`, and exact-main run `30795396504`
      passed before any tag was created.

- [x] Task 3 - Publish `v1.2.3-rc.13` from exact green main
  - Acceptance criteria:
    - One annotated tag points to the exact protected-main commit and is pushed
      without moving any prior tag or creating a stable tag.
    - The candidate release workflow completes without bypassing any gate.
  - Check:
    - Annotated tag `v1.2.3-rc.13` points to exact protected-main commit
      `d416968e0cadb900820ecf1b4307b101b82ffbbc`; no prior or stable tag moved.
    - Release run `30795492882` passed production env and Postgres validation,
      the unified verifier, cross-platform binaries/installers/packages,
      multi-architecture image builds, exact-digest signatures, package
      acceptance, anonymous pull-only product proof, payload scanning,
      release-set signing, and publication without a bypass.

- [x] Task 4 - Verify the public signed candidate set
  - Acceptance criteria:
    - Public assets, checksums, signed release-set root, five exact image
      signatures, SBOM/evidence coverage, tag/commit/run binding, and anonymous
      GHCR pulls are verified.
    - The release is a prerelease; stable and `latest` remain unchanged.
  - Check:
    - Public prerelease `v1.2.3-rc.13` has 46 assets. All downloaded bytes
      matched GitHub's server-reported SHA-256 digests, and the complete payload
      passed `release_set.py verify-candidate` for RC13 and commit `d416968e`.
    - `release-set.json` SHA-256 is
      `795fffee84662aec91624eb4352b9c1a9ef5c34b17838939adaf567418797fa0`.
      It binds 42 payload artifacts, five evidence assets, five image digests/
      SBOM/signature bundles, eight passed workflow gates, and eight explicitly
      pending external gates to release run `30795492882`.
    - Official Cosign v3.1.2 matched its published checksum. It independently
      verified the release-set blob and all five registry image signatures
      against the exact `release.yml@refs/tags/v1.2.3-rc.13` identity and GitHub
      OIDC issuer, including transparency-log and code-signing certificate
      checks.
    - Empty-credential GHCR inspection resolved all five public RC13 tags to
      the exact signed digests. Every first-party `v1.2.3` and `latest` alias,
      plus the stable GitHub tag/release, remained absent.
    - Public golden-path evidence passed real RTMP publish/live state, OME
      LL-HLS playlist advancement and 3-second decoded 1080p H.264 playback,
      transcoder HLS, offline transition, chat/moderation, VOD upload/playback,
      and final aggregate readiness. Publication and payload scans passed.
    - The first attempted local image verification used blob-only `--bundle`
      syntax with `cosign verify` and failed before verification because that
      image subcommand does not accept the flag. Correct registry verification
      then passed for every exact digest; bundle file hashes remain covered by
      the independently verified release-set root.

- [x] Task 5 - Record release evidence and update issue state truthfully
  - Acceptance criteria:
    - TASKS/PLAN and #1271/#1301 receive durable public evidence.
    - #1271 closes only after provenance publication is proven; #1301 and all
      external stable gates remain open until byte-identical promotion is real.
  - Check:
    - PLAN/TASKS record the public release, workflow, signed root, exact image
      digests/signatures, anonymous access, OME/media evidence, and boundaries.
    - #1271 closed with complete public provenance evidence. #1301 received the
      candidate-side proof and remains open for no-build stable promotion;
      #1297/#1304 received the exact candidate and OME baseline while remaining
      open for Ubuntu/XOA/NPM/reboot/recovery evidence.
    - #1298/#1299/#1303/#1305/#1306/#1307 remain open and are still recorded as
      pending in the signed root. No stable release readiness is claimed.

## Scoped change: aggregate merge gate and main protection (#1302, #1270)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Audit current CI enforcement and protection state
  - Acceptance criteria:
    - The reusable job graph, scorecard behavior, gate documentation, live
      rulesets/protection, and related issue contracts are inspected read-only.
    - `PLAN.md` records scope, risks, exclusions, test plan, and rollout before
      workflow implementation.
  - Check:
    - `ci.yml` has thirteen conditional/always-on child results but no aggregate
      job. Run `30784385158` demonstrated that GitHub could report success when
      the first 3,000 changed files hid every substantive selector.
    - The scorecard validator exists but is not wired into CI; it is advisory by
      default and strict for every warning only when manually requested.
    - GitHub reports zero repository rulesets and `main` branch protection
      returns `404 Branch not protected`. Issue #1270 assigns remaining gate
      clarity/artifact work to #1302; #1302's stable-promotion half still
      depends on #1301's immutable release-set manifest.

- [x] Task 2 - Add deterministic aggregate-result and scorecard fixtures
  - Acceptance criteria:
    - One shell guard evaluates always-required and path-required child results,
      rejects failed/cancelled/unexpectedly skipped work, and emits a concise
      Markdown table without running child tests itself.
    - Fixtures cover docs-only success, selected success, required skip,
      child failure/cancellation, unexpected failure, and scorecard failure.
    - Scorecard validation gains a tested risky-path mode that remains advisory
      for low-risk/docs-only paths and blocks sensitive workflow/operator paths.
  - Check:
    - Added `check-ci-merge-gate.sh`, which derives the cross-platform Go
      expectation from Go/deploy selectors, classifies every child result, and
      writes the same secret-free Markdown table to stdout, the job summary,
      and an optional artifact path.
    - The guard rejects required skips, failures, cancellations, missing/invalid
      booleans, and failures from unexpectedly executed jobs while allowing
      correctly skipped or extra-successful non-required work.
    - Added `--strict-if-risky` to the existing scorecard validator. Missing
      scorecards remain advisory for docs-only paths but block when medium/high
      risk is selected or code, CI, dependencies, deployment, or operator paths
      change; low-risk declarations on those paths now warn.
    - Linux shell syntax, six aggregate result scenarios, advisory/strict risky
      scorecard fixtures, and pinned ShellCheck 0.11.0 all pass.

- [x] Task 3 - Wire one always-run merge gate and evidence artifact
  - Acceptance criteria:
    - `Merge gate` needs every child, uses `if: always()`, and passes every
      changed-file expectation/result into the focused guard.
    - PR body input is read safely from the event file; the scorecard and gate
      table reach the job summary and a pinned artifact upload.
    - Static workflow regressions prevent missing needs, unstable naming,
      shallow checkout, unpinned upload, or aggregate bypass.
  - Check:
    - Added one `Merge gate` with `if: always()` and explicit `needs` for all
      twelve child jobs. It consumes all eleven relevant selector outputs and
      every child result, including the macOS/Windows matrix aggregate.
    - The PR scorecard step checks out full history, obtains body text from
      `GITHUB_EVENT_PATH`, diffs the exact PR SHAs, and records its outcome
      without interpolating untrusted text into shell source. The aggregate
      makes that outcome blocking on PRs.
    - The gate writes a job summary and uploads the merge/scorecard reports for
      14 days with pinned `actions/upload-artifact` v7.0.1. On non-PR main pushes
      the scorecard is correctly unrequired/skipped.
    - Added a static Go contract covering stable name, unconditional execution,
      complete needs/results/selectors, full checkout, safe scorecard input,
      aggregate invocation, and pinned fail-closed artifact upload.
    - CI workflow YAML parsing, pinned Go 1.26.5 `go test ./scripts`, CI policy,
      aggregate fixtures, shell syntax, pinned ShellCheck 0.11.0, and
      `git diff --check` pass.

- [x] Task 4 - Document blocking, conditional, advisory, and manual gates
  - Acceptance criteria:
    - Release-gate, testing, scorecard, contributing, and security guidance name
      the stable required check and explain expected skips and break-glass use.
    - Documentation does not claim stable-promotion enforcement before #1301.
  - Check:
    - Contributor, testing, scorecard, and release-gate docs now identify
      `Merge gate` as the single stable required context, explain path-selective
      expected skips, and document risk-triggered scorecard enforcement plus
      the retained report artifact.
    - Security policy records PR/current-check/conversation/admin enforcement,
      no force/delete, and a narrow audited break-glass procedure with immediate
      restoration and follow-up evidence.
    - Documentation explicitly limits this control to merges; it does not claim
      stable promotion before #1301 supplies the immutable release-set input.
    - Markdown link tests/check, installer wording, generated contract freshness,
      scorecard fixtures, and `git diff --check` pass.

- [x] Task 5 - Prove, merge, protect, and negatively demonstrate enforcement
  - Acceptance criteria:
    - Focused checks, literal `./scripts/verify.sh`, intentional failing PR
      evidence, corrected PR CI, and exact merged-main CI pass.
    - Live `main` protection requires a current successful `Merge gate`, PRs,
      conversation resolution, admin enforcement, and rejects force/delete.
    - A disposable failing canary PR is blocked, closed, and its branch deleted;
      #1270 is closed with evidence while #1302 remains open for #1301.
  - Check:
    - Literal `./scripts/verify.sh` passed in 126.4 seconds with the existing
      Windows Go 1.26.5 toolchain and a temporary env copy upgraded through the
      supported CLI. It covered the new fixtures, all Go packages, Postgres
      migrations, Compose rendering, healthy SRS/OME/transcoder/API/viewer
      smoke, and clean teardown.
    - The private `.env` returned to SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`,
      both temporary env files were removed, and zero Compose-labeled containers
      or volumes remain.
    - Draft PR #1363 intentionally omitted its risky-path scorecard. Run
      `30786811135` passed Ubuntu `test-all`, docs consistency, ShellCheck,
      macOS/Windows Go, and all quickstart entrypoint checks; `Merge gate` then
      failed only the required scorecard row and retained 956-byte artifact
      `merge-gate-30786811135-1` (ID `8845612070`). Log review found a literal
      newline escape in the failure list; the formatter and fixture now prevent
      that cosmetic regression before the corrected PR run.
    - Corrected PR run `30787193752` passed every selected child: Ubuntu
      `test-all`, docs consistency, ShellCheck, macOS/Windows Go, and all three
      quickstart entrypoint checks. Unrelated viewer, monitoring, image,
      workflow-consistency, and wizard jobs skipped as selected; `Merge gate`
      classified each result and passed the required scorecard.
    - The successful aggregate retained two reports for 14 days as 626-byte
      artifact `merge-gate-30787193752-1` (ID `8845751471`, SHA-256
      `f24e03d4c599679bc504431797ee35a8b1173385df56c2bbfe99b7948f7882f7`).
      Its Markdown table ends in `Result: PASS` and contains no literal newline
      escape regression.
    - Final PR-head run `30787600492` passed on `2c520855`; PR #1363 was marked
      ready and squash-merged as `200bf414`. Push run `30787937537` then passed
      on that exact `main` commit, including the stable aggregate.
    - Live `main` protection requires the strict `Merge gate` check bound to
      GitHub Actions app ID `15368`, pull requests with stale-review dismissal,
      resolved conversations, and admin enforcement. Force pushes and branch
      deletion are disabled; API readback confirmed every setting.
    - Ready canary PR #1364 changed one disposable `.github` marker and omitted
      the risky-path scorecard. Run `30788386196` failed `Merge gate`; GitHub
      reported `mergeStateStatus: BLOCKED` and REST `mergeable_state: blocked`.
      The PR was closed unmerged, both branch refs were deleted, and the remote
      branch count returned from 65 to its 64-branch baseline.
    - Issue #1270 was closed with pass/fail artifact and canary evidence. #1302
      records the completed merge half and remains open because its stable-
      promotion half and negative promotion proof still depend on open #1301.
    - The evidence-ledger follow-up passed literal `./scripts/verify.sh` in
      129.7 seconds: aggregate fixtures, all Go packages, real Postgres
      migrations, Compose rendering, and healthy SRS/OME/transcoder/API/viewer
      smoke all passed. The temporary env files were removed, the private env
      retained the SHA-256 above, and the BitRiver `deploy` Compose project left
      zero containers and volumes; unrelated `novel-generator` Docker objects
      were identified and preserved.

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
    - Run `30214370984` passed Prometheus, all nine rules, and `amtool`, then
      exposed Compose's separate service-level `env_file: ../.env` resolution.
      Validation now creates a private example-derived root `.env` only when
      absent and removes only its own file through the exit trap; an existing
      operator `.env` is neither rewritten nor deleted.
    - Run `30214468760` passed monitoring and the blocking image scan, then
      exposed that the reusable Windows quickstart entrypoint check inherited
      runner Go 1.24.13 even though `-ValidateOnly` builds the Go 1.26 CLI. The
      cross-platform entrypoint matrix now invokes the shared pinned Go setup
      after checkout, and the optional full smoke job does the same before its
      source wrapper, with an ordering/count regression.

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

- [x] Task 5 - Verify locally and through GitHub
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
    - PR run `30214732829` passed on implementation head `b9837e38`: unified
      Ubuntu verification, blocking first-party and informational dependency
      image scans, complete monitoring validation, viewer integration/build/
      audit, Windows/macOS Go, Ubuntu/macOS/Windows quickstart entrypoints,
      ShellCheck, docs/workflow policy, wizard, and committed-secret guard.
    - PR #1331 is mergeable and its implementation matrix is green.
      Post-merge targeted workflow evidence remains pending.
    - PR #1331 merged as `14ed412b`; post-merge monitoring run `30215316794`
      and migrated Postgres run `30215315759` passed on that exact commit.
    - Standalone Go run `30215317804` exposed two post-merge blockers: Ubuntu
      leaked `GOPROXY=off` into production-module Docker downloads, while
      macOS passed Go tests but found the generated Helm SRS copy stale.
      The focused follow-up must restore network only around Ubuntu
      `verify.sh`, regenerate Helm assets canonically, and repeat affected
      release gates before tagging.
    - The focused workflow regression, CI contract/policy checks, YAML parse,
      generated Helm asset check, and `git diff --check` passed locally.
    - Literal `./scripts/verify.sh` passed after selecting the repository's
      installed Go 1.26.5 toolchain: all Go/script checks, Compose rendering,
      migrations, production-module builds, OME/API/viewer health, and the
      complete Docker quickstart smoke passed. The private root `.env` was
      restored with its original SHA-256.
    - PR #1332 passed its selected CI matrix and merged as `3556c049`.
      Merged-main standalone run `30215961551` then passed the repaired Ubuntu
      full verification gate and macOS, but Windows failed the byte-for-byte
      Helm SRS drift check because the canonical/generated `.conf` paths have
      unspecified checkout line endings.
    - Follow-up: pin both SRS paths to LF in `.gitattributes`, cover the
      invariant with a focused test, rerun local policy/sync/full verification,
      and require the standalone three-OS workflow to pass on merged `main`
      before tagging.
    - The LF attribute regression, generated-asset byte comparison, CI
      contract check, and `git diff --check` passed locally. Literal
      `./scripts/verify.sh` also passed with Go 1.26.5, including migrations,
      production-module builds, OME/API/viewer health, and full Docker
      quickstart smoke; the private `.env` retained its original SHA-256.
    - Explicit PR-head standalone run `30216357352` passed the formerly failing
      Windows Helm drift step. Windows then failed later because
      `run-govulncheck.sh` passes Git Bash `/d/...` artifact paths to native
      Python, which resolves them as missing `\d\...` paths.
    - Follow-up: translate Bash paths only at the native Python boundary,
      retain the blocking vulnerability policy, add focused Windows-path
      coverage, and repeat the full local and three-OS remote gates.
    - Added bounded Git Bash-to-native-Python path conversion for policy
      inputs, outputs, and scan-index entries. Focused dependency-policy tests,
      shell syntax, generated-asset comparison, CI contract checks, and another
      literal `./scripts/verify.sh` passed; the Docker smoke again reached
      healthy OME/API/viewer and the private `.env` hash was unchanged.
    - PR-head standalone run `30216694278` passed on Ubuntu, macOS, and Windows;
      Windows specifically passed Helm drift, function coverage, build,
      blocking govulncheck policy, and artifact upload. Automatic PR run
      `30216675113` passed unified Ubuntu verification, ShellCheck,
      Windows/macOS Go tests, all three quickstart entrypoint paths, and the
      secret guard. Exact merged-main proof remains required before tagging.
    - PR #1333 merged as `7b3d95d6`; exact merged-main standalone run
      `30217192656` passed Ubuntu full verification plus macOS/Windows unit,
      generated-asset, function-coverage, build, blocking govulncheck, and
      artifact-upload gates. Task 5 is complete and `v1.2.3-rc.3` may be tagged
      from the evidence-only mainline successor.

- [x] Task 6 - Tag and inspect `v1.2.3-rc.3`
  - Acceptance criteria:
    - The immutable tag points at the verified merged commit.
    - Every failed release job is classified before a successor tag is created.
    - No GitHub Release is created and no success claim is made when artifact
      or pull-only product gates fail.
  - Check:
    - Annotated tag `v1.2.3-rc.3` points at verified merged commit `c9d5a9f3`;
      release run `30217498138` preserved the immutable failure evidence.
    - Release environment validation, migrated Postgres, Go verification, all
      five first-party image publications/SBOMs, and Linux amd64 CLI/release
      artifacts passed.
    - Cross-target CLI/artifact jobs failed because target `GOOS`/`GOARCH`
      leaked into `go run` for the host-side binary verifier.
    - Windows MSI failed because PowerShell passed the literal
      `$env:PRODUCTION_MODFILE` as the Go `-modfile`; four cross-target
      launchers hit the foreign verifier, while Linux amd64 reached Cosign 3
      and failed because the deprecated detached-signature flag was ignored.
    - Viewer tests/build passed and packaging alone failed on an absent optional
      `public/` directory. The pull-only product gate reached anonymous image
      resolution, then failed because Compose's service-level `env_file`
      expected a clean-runner root `.env`.
    - GitHub Release creation and downstream package/Homebrew jobs were skipped.
      `rc.3` is not a published release candidate; corrections move to
      immutable `v1.2.3-rc.4`.

- [x] Task 7 - Repair release packaging and inspect `v1.2.3-rc.4`
  - Acceptance criteria:
    - Cross-platform binary verification runs as a host tool without weakening
      production-module inspection.
    - MSI production-module setup, Cosign bundles, viewer bundle packaging, and
      clean-runner pull-only Compose env resolution are covered by regressions.
    - Release candidate output remains private and atomic across transient
      Windows file-sharing failures.
    - Pull-only SRS readiness does not depend on tools added only by the source
      wrapper image, and Compose/Helm probe contracts remain aligned.
    - The Docker-internal product harness keeps production Secure cookies and
      authenticates only same-origin API calls without forwarding the session
      credential cross-origin.
    - Third-party image tags are resolved once into sanitized immutable evidence
      and reused by the product gate without a second registry-resolution pass.
    - Focused checks, literal full verification, complete PR CI, and exact
      merged-main evidence pass before tagging.
    - The immutable `rc.4` workflow is accepted only if complete; otherwise all
      failures are classified before a successor tag is created.
  - Check:
    - Workflow regressions now keep host-side binary verification on the runner
      platform, pass the MSI modfile as one PowerShell argument, use Cosign v3
      bundles, and package the viewer without requiring an optional `public/`
      directory.
    - The quickstart smoke creates and owns a mode-restricted root env bridge
      only when an explicit external release env is supplied and no operator
      root `.env` exists. Windows atomic output now closes descriptors safely,
      conditionally applies POSIX mode changes, retries only transient replace
      failures, and cleans up terminal failures.
    - The raw pinned `ossrs/srs:v5.0.185` image proved it has Bash but neither
      curl nor wget. Compose and Helm now use the same `/dev/tcp` HTTP 200 probe;
      focused Go contracts and a rendered Compose validation passed.
    - The first repaired pull-only rehearsal made SRS healthy and reached the
      media harness, then exposed production Secure cookies not returning over
      the trusted Docker-internal HTTP hop. The client now applies the supported
      Bearer fallback only to the exact API origin, records the session as a
      secret sentinel, and a two-server regression proves no cross-origin
      credential forwarding.
    - A Docker Hub 429 on the immediate retry exposed duplicate third-party tag
      resolution in preflight and the product gate. Preflight now resolves one
      strict, sanitized `release-dependencies.json`; the downstream job
      downloads, validates, and reuses it. Fifteen release-helper tests and the
      complete Go scripts suite pass, including schema/completeness/reference
      binding and single-resolution workflow regressions.
    - The complete local pull-only Docker Desktop rehearsal then passed with
      immutable `v1.2.3-rc.3` first-party images plus the corrected local
      orchestration: all eight product stages passed (surface, accounts/channel,
      RTMP live state, decodable OME/transcoder media, offline transition,
      chat/moderation, VOD publish/playback, and final aggregate status). Both
      evidence scans passed, teardown left no BitRiver containers, temporary
      secret inputs were removed, and the operator `.env` retained SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`.
    - Full repository verification, PR #1335 CI, merge commit `e67a9304`,
      merged-main run `30220230319`, and annotated `v1.2.3-rc.4` tagging passed.
    - Release run `30220542359` passed environment validation, migrated
      Postgres, the unified Go/Compose gate, all cross-platform CLI and release
      binary builds, four launcher/signature jobs, Linux amd64 packages, all
      five image publications/SBOMs, and the complete pull-only media/API gate.
    - The run then preserved three packaging failures: current
      `windows-latest` has no hardcoded WiX v3.11 installation; target
      `GOARCH=arm64` leaked into the host nFPM installation; and the viewer
      archive producer used `web/viewer/dist` while its uploader consumed root
      `dist`. Package acceptance, Homebrew generation, and GitHub Release
      creation were skipped. `rc.4` is not a published GitHub prerelease.

- [x] Task 8 - Repair `rc.4` packaging failures and inspect `v1.2.3-rc.5`
  - Acceptance criteria:
    - The MSI job provisions and resolves a pinned compatible WiX toolchain
      without depending on hosted-runner preinstallation.
    - nFPM is built for the host OS/architecture while Linux packages retain the
      matrix target architecture.
    - The viewer bundle producer and uploader use the same workspace-root path.
    - Focused workflow regressions, YAML parsing, repository verification, PR
      CI, and exact merged-main CI pass before the successor tag.
    - The immutable `rc.5` workflow is inspected completely; any failure keeps
      GitHub Release creation blocked and is classified before a successor tag.
  - Check:
    - `PLAN.md` records the three exact `rc.4` failure causes, risks, evidence
      boundary, and `rc.5` test/publication plan before implementation.
    - The MSI job now downloads official WiX 3.14.1 binaries, enforces SHA-256
      `6ac824e1642d6f7277d0ed7ea09411a508f6116ba6fae0aa5f2c7daa2ff43d31`,
      validates required tools, and resolves the executables/extension from its
      job-local directory.
    - The launcher matrix now installs nFPM with explicit host
      `GOOS`/`GOARCH` and `GOBIN`, then invokes that host helper by absolute path
      while retaining the matrix architecture in package metadata.
    - Viewer packaging now writes and uploads the same absolute
      `${{ github.workspace }}/dist` archive path.
    - Focused MSI/tool/viewer workflow regressions, the complete Go scripts
      suite, CI/workflow policy checks, docs checks, 19-file workflow/action
      YAML parsing, and `git diff --check` passed.
    - Official WiX 3.14.1 tools were checksum-verified locally; the canonical
      staged assets and production-module binary compiled and linked into a
      7.7 MB MSI. Local ICE validation could not access this sandbox's Windows
      Installer service and remains a required unsuppressed GitHub Windows
      check.
    - Literal `./scripts/verify.sh --viewer` passed: all Go/script checks,
      migrations, production-module image builds, Compose render/quickstart,
      SRS/OME/API/viewer health, viewer lint, and 26 Jest suites/217 tests/four
      snapshots. The operator `.env` was restored with unchanged SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`.
      PR run `30221501875`, merge commit `72283baf`, and exact merged-main run
      `30221776092` passed before annotated `v1.2.3-rc.5` was pushed.
    - Release run `30222035324` passed environment validation, migrated
      Postgres, repository verification, the viewer bundle, all five image
      publications/SBOMs, every CLI/release/launcher matrix entry, Linux amd64
      and arm64 packages, Ubuntu 24.04/Debian 12/Rocky 9 package acceptance,
      Homebrew generation, and the pull-only tagged media/API product gate.
    - Hosted Windows downloaded and checksum-verified pinned WiX 3.14.1, then
      `candle.exe` rejected the literal value `$env:MSI_VERSION` received from
      the inline `-dProductVersion` argument. The MSI artifact failed and the
      GitHub Release job skipped. `rc.5` remains immutable and is not a
      published prerelease.

- [x] Task 9 - Restore the release baseline and inspect `v1.2.3-rc.6`
  - Acceptance criteria:
    - Current `main` returns to green by restoring the six viewer versions
      guarded by the proven runtime baseline without reverting compatible
      unrelated dependency updates.
    - WiX definition arguments are precomposed as PowerShell strings; the
      hosted MSI build keeps unsuppressed validation and uploads the result.
    - Focused workflow/runtime tests, clean viewer install/audit/tests/build,
      viewer image build, YAML parsing, literal full verification, PR CI, and
      exact merged-main CI pass before tagging.
    - The immutable `rc.6` workflow is inspected completely; any release or
      image-publication failure is classified before a successor tag.
  - Check:
    - Read-only live inspection found current-main CI run `30404788813` red:
      `TestViewerRuntimeBaselineIsAligned` rejected six drifted versions and
      clean Docker `npm ci` rejected TypeScript 7.0.2 because
      `ts-jest@29.4.11` requires TypeScript `<7`.
    - `PLAN.md` records the exact current-main and `rc.5` failure evidence,
      bounded repair, risks, tests, and immutable publication plan before code
      changes.
    - Local repair proof passes with Go 1.26.5: the focused runtime/MSI
      regressions and full `go test ./scripts` suite; clean `npm ci`; the exact
      dependency tree; a production audit with zero vulnerabilities; lint; 26
      Jest suites/217 tests/four snapshots; the Next production build; all 36
      Playwright tests; and a clean viewer Docker image build.
    - All 15 workflow/action YAML files parse, both CI contract scripts pass,
      `git diff --check` passes, and pinned WiX 3.14.1 compiles the product and
      harvested assets with the precomposed `1.2.3`, source, and asset
      definition arguments.
    - Literal `./scripts/verify.sh --viewer` passes all Go packages, migrated
      Postgres, Compose rendering, canonical image builds, and healthy
      Postgres/Redis/SRS/SRS controller/OME/transcoder/API/viewer smoke checks.
      The operator `.env` was restored with unchanged SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`.
    - PR #1345 run `30642592147`, squash merge `d94ac432`, and exact
      merged-main run `30643297236` passed before annotated
      `v1.2.3-rc.6` was pushed.
    - Release run `30643868431` passed the restored viewer checks, Go/Postgres,
      every CLI/release/launcher entry, Linux packages, Ubuntu 24.04/Debian
      12/Rocky 9 package acceptance, Homebrew, and four first-party image
      publishers. The hosted MSI compiled both WiX sources with concrete
      arguments, then `light.exe` rejected the doubled-backslash shortcut
      registry keys with `ICE03: Invalid registry path`.
    - The viewer amd64 image built, while emulated arm64 `npm ci` crashed with
      `qemu: uncaught target signal 4 (Illegal instruction)` and left Buildx in
      the same step. The already-failed run was cancelled after the job remained
      nonterminal; viewer SBOM, pull-only product gate, release aggregation, and
      GitHub prerelease publication did not run. `rc.6` remains immutable and
      is not a published prerelease.

- [-] Task 9a - Repair hosted MSI and viewer arm64 publication, then publish `v1.2.3-rc.7`
  - Acceptance criteria:
    - WiX registry and shortcut paths use canonical single backslashes; pinned
      WiX compiles and links with unsuppressed ICE validation.
    - Viewer amd64 and arm64 images build on matching native GitHub runners,
      publish architecture tags, and assemble the release tag into one
      multi-architecture manifest with a release SBOM.
    - Image jobs have bounded timeouts; the viewer path does not use QEMU; the
      pull-only product gate and final release depend on the assembled viewer
      manifest.
    - Focused/full workflow regressions, YAML parsing, local architecture/MSI
      proof, literal verification, PR CI, and exact merged-main CI pass before
      tagging.
    - The immutable `rc.7` workflow publishes the MSI, packages, complete
      assets/checksums, public multi-architecture images/SBOMs, the OME-backed
      pull-only product gate, and a GitHub prerelease.
  - Check:
    - `PLAN.md` records both exact `rc.6` hosted failures, bounded fixes, risks,
      tests, immutable-tag rule, and the remaining clean-host evidence boundary
      before implementation.
    - Local RC7 proof passes: focused and full workflow/CI regression suites;
      all 15 workflow/action YAML files; both CI contract scripts; pinned WiX
      3.14.1 compile plus unsuppressed ICE link to a 7.7 MB MSI; and native
      amd64 viewer image build/runtime (`x86_64`, Node 24). PR CI is the
      authoritative native arm64 build/runtime gate before tagging.
    - Literal `./scripts/verify.sh --viewer` passes all Go packages, migrated
      Postgres, Compose rendering, canonical image builds, healthy
      Postgres/Redis/SRS/SRS controller/OME/transcoder/API/viewer smoke, and 26
      Jest suites/217 tests/four snapshots. The operator `.env` was restored
      with unchanged SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`.
      PR #1347 and exact merged-main CI passed. Hosted release run
      `30648508975` proved the MSI, native amd64/arm64 viewer images, assembled
      viewer manifest, all other images/SBOMs, packages and package acceptance,
      Homebrew, and the pull-only tagged product gate. The viewer bundle alone
      failed: its configured-login Playwright test clicked before initial auth
      discovery completed and observed the local auth URL. GitHub prerelease
      creation therefore skipped; `rc.7` remains immutable and unpublished.

- [-] Task 9b - Repair the viewer auth-discovery race and publish `v1.2.3-rc.8`
  - Acceptance criteria:
    - Navbar authentication actions cannot be invoked until initial viewer-auth
      discovery completes.
    - The configured-login Playwright test waits for the mocked auth response
      and deterministically redirects to `/login?redirect=%2F`.
    - Docker Compose image builds use the public Go module proxy with direct
      fallback and checksum verification, while host Go tests remain offline.
    - Focused viewer tests, full viewer integration/build checks, workflow
      contracts/YAML, literal verification, PR CI, and exact merged-main CI pass
      before tagging.
    - The immutable `rc.8` workflow publishes every artifact, checksum, image,
      SBOM, package, and the GitHub prerelease after the pull-only product gate.
  - Check:
    - `PLAN.md` records the exact `rc.7` failure, bounded product/test fix,
      evidence plan, immutable-tag rule, and remaining clean-host boundary
      before implementation.
    - Focused Navbar Jest regression passes: one suite, 30 tests, including
      disabled sign-in/create-account actions during initial auth discovery.
    - Focused configured-login Playwright regression passes against a fresh
      production build and standalone server (one test, 14 seconds).
    - Full `npm run test:integration` passes: zero-warning lint, 26 Jest
      suites/218 tests/four snapshots, a production Next.js build, and all 36
      Playwright workflows.
    - Go 1.26.5 passes the full `./scripts` workflow-contract package; Node
      parses all 19 workflow/action YAML files; `git diff --check` passes.
    - Literal `./scripts/verify.sh --viewer` passes in a clean-checkout env:
      release bundle, all Go packages, contract invariants, migrated Postgres,
      Compose render/build, healthy Postgres/Redis/SRS/SRS controller/OME/
      transcoder/API, viewer smoke, lint, and 26 Jest suites/218 tests/four
      snapshots. The operator `.env` was restored with unchanged SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`.
    - PR CI run `30650568202` passed secret/path selection and native arm64
      viewer build/runtime, but both the initial image scan/Ubuntu gate and a
      failed-job rerun hit repeatable HTTP 502 responses from direct
      `gopkg.in` module fetches. This justifies a build-only proxy repair rather
      than another blind rerun.
    - Focused build-network regressions, Bash syntax, the full Go 1.26.5
      `./scripts` package, all 19 workflow/action YAML files, and
      `git diff --check` pass after the repair.
    - A second literal `./scripts/verify.sh --viewer` passes with the hardened
      build path; clean container logs show
      `GOPROXY=https://proxy.golang.org,direct` and `GOSUMDB=sum.golang.org`,
      then migrated Postgres and all SRS/OME/transcoder/API/viewer health gates
      pass. The private `.env` hash remains unchanged.
    - `docs/testing.md` documents the offline-host/build-network separation and
      the two optional Docker-build mirror overrides.
    - Installer wording and generated contract documentation consistency checks
      pass after the testing-guide update.
    - PR #1348 CI run `30651676770` passes completely: Ubuntu test-all, image
      build/scan, native arm64 viewer build/runtime, shellcheck, docs
      consistency, macOS/Windows Go, viewer integration/build/audit, and all
      quickstart entrypoint matrices.
    - Final PR run `30652301783`, squash merge `5295c1e4`, and exact
      merged-main run `30652845968` passed before annotated immutable
      `v1.2.3-rc.8` was pushed.
    - Release run `30653362368` passed production validation, migrated
      Postgres, repository verification, every binary/launcher/package build,
      the hosted MSI, Linux package acceptance, Homebrew, the viewer bundle,
      native viewer images, and the viewer multi-architecture manifest.
      The srs-controller, ome-config, and transcoder multi-architecture image
      jobs all entered their Dockerfiles with `GOPROXY=direct` and
      `GOSUMDB=off`; `gopkg.in` returned HTTP 502 during dependency download.
      The pull-only product gate and GitHub prerelease therefore skipped, and
      `rc.8` remains immutable and unpublished.

- [x] Task 9c - Repair tag-only image dependency resolution and evaluate `v1.2.3-rc.9`
  - Acceptance criteria:
    - The release multi-architecture image publisher passes the public Go
      module proxy with direct fallback and checksum verification to every
      first-party Go Dockerfile.
    - Host Go verification remains offline; Dockerfile defaults, runtime
      networking, image names, and the deployment contract remain unchanged.
    - A workflow contract regression covers the tag-only publisher; focused
      tests, full scripts tests, YAML/policy checks, literal verification, PR
      CI, and exact merged-main CI pass before tagging.
    - The immutable `rc.9` workflow either publishes the complete release or
      fails closed without moving/reusing the tag or bypassing publication.
  - Check:
    - `PLAN.md` records the exact three-job `rc.8` failure, bounded fix,
      immutable-tag rule, evidence plan, and remaining clean-host boundary
      before implementation.
    - The release Buildx matrix now passes
      `GOPROXY=https://proxy.golang.org,direct` and
      `GOSUMDB=sum.golang.org` to all four first-party Go image Dockerfiles;
      Dockerfile defaults and runtime configuration are unchanged.
    - The focused publisher regression and complete pinned Go 1.26.5
      `./scripts` suite pass. Both CI policy scripts pass, all 19 GitHub
      workflow/action/template YAML files parse, and `git diff --check` passes.
    - Literal `./scripts/verify.sh --viewer` passes with pinned Go 1.26.5:
      release bundle, all Go packages, migrated Postgres, production-module
      Compose builds, healthy Postgres/Redis/SRS/SRS controller/OME/
      transcoder/API/viewer, lint, and 26 Jest suites/218 tests/four snapshots.
      Cleanup left no `deploy` project containers, and the private `.env`
      retains SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`.
    - PR #1349 run `30654775807`, squash merge `e92240bd`, and exact
      merged-main run `30655235823` passed before annotated immutable
      `v1.2.3-rc.9` was pushed.
    - Release run `30655699977` passed every application, package,
      multi-architecture image/SBOM, and pull-only tagged production gate. Its
      final payload scan ran for 59 minutes, reported 224 false-positive
      assignments plus unsafe absolute-path RPM extraction failures, and
      exited before notes/publication. No GitHub prerelease exists; RC9 remains
      immutable and was not rerun or bypassed.

- [x] Task 9d - Bound release scanning and evaluate `v1.2.3-rc.10`
  - Acceptance criteria:
    - Exclude Buildx `.dockerbuild` diagnostics from the release download while
      retaining all release assets, SBOMs, checksums, and product evidence.
    - Scan credential-shaped matches in batches, preserve a tested fallback,
      reject real secret classes without printing values, and allow known
      package/framework/code false positives.
    - Keep RPM extraction inside scratch storage and fail closed on scanner or
      archive errors. Bound the workflow scan step to ten minutes.
    - Focused/full tests, literal verification, PR CI, and exact merged-main CI
      pass before creating immutable `v1.2.3-rc.10`.
    - RC10 either publishes the complete GitHub prerelease or fails closed
      without moving/reusing the tag or bypassing publication.
  - Check:
    - The real 8.3 MB RC9 viewer subset, including its 34.6 MB expanded bundle,
      passes deep scanning in about 35 seconds instead of exceeding five
      minutes. The complete non-Buildx RC9 payload (32 artifact groups, 36
      files, 274,928,685 bytes) passes inside Debian 12 in 30 seconds, including
      nested archives and both real Linux package formats.
    - Literal JavaScript credentials, JSON credentials, sentinels, private
      keys, credential URLs, XML credentials, and forbidden filenames remain
      covered without printing values. Package hashes, framework/parser token
      constants, source references, `.env.example`, and compiled binaries are
      regression-covered; the no-ripgrep fallback is also tested.
    - RC9 terminal logs confirm the release failed closed before notes or
      publication. They also exposed absolute-path RPM entries, now extracted
      with `cpio --no-absolute-filenames`; the actual RC9 amd64 `.deb` and `.rpm`
      both pass full extraction and scanning in a disposable Debian container.
    - The focused scanner/workflow suite and complete pinned Go 1.26.5
      `./scripts` suite pass. Shell syntax, both CI policy scripts, all 19
      GitHub YAML files, and `git diff --check` pass.
    - Literal `./scripts/verify.sh` passes: release bundle, all Go packages,
      migrated Postgres, Compose rendering/builds, and healthy Postgres/Redis/
      SRS/controller/OME/transcoder/API/viewer smoke all pass. Cleanup leaves no
      BitRiver containers; the private root `.env` is restored byte-for-byte at
      SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`.
    - PR #1350 run `30663196812` passed the complete Ubuntu gate, both image
      gates, shellcheck, and all quickstart entrypoints. Its macOS Go job caught
      Bash 4-only lowercase expansion in the scanner on the platform's Bash
      3.2; filename matching now uses portable `nocasematch`, and a static
      regression rejects reintroduction of the unsupported expansion.
    - Replacement run `30663742226` passed those gates and all quickstart
      entrypoints, then proved macOS lacks the GNU grep NUL-output behavior used
      by the no-ripgrep fallback. The fallback now uses portable `find -print0`
      plus per-file grep and avoids GNU-only `xargs -r`; an actual Bash 3.2
      container passed allowed-example and known-sentinel scans, and a
      disposable Debian ShellCheck run passed.
    - PR #1350 merged as `48e4b878`; exact-main CI run `30664862234` passed.
      Immutable RC10 release run `30665278361` first hit a transient BuildKit
      pull timeout, then its failed-job rerun passed the pull-only OME-backed
      product gate, downloaded the non-Buildx artifacts, scanned the complete
      publication payload in 31 seconds, uploaded retained evidence, and
      generated release notes.
    - Final publication failed closed before the release action started:
      GitHub's generated first-release history was passed as an inline action
      input and exceeded the hosted runner process argument limit. No RC10
      GitHub Release exists; the tag remains immutable and unpublished.

- [x] Task 9e - File-back release notes and publish `v1.2.3-rc.11`
  - Acceptance criteria:
    - Generated release notes are written as UTF-8 under `RUNNER_TEMP`, and the
      publisher receives only the file path through its supported `body_path`
      input.
    - The complete generated history is retained; finalizer dependencies,
      evidence, asset selection, and fail-closed publication stay unchanged.
    - A regression requires file-backed notes and rejects inline note bodies.
    - Focused/full tests, literal verification, PR CI, and exact merged-main CI
      pass before creating immutable `v1.2.3-rc.11`.
    - RC11 publishes only after every producer and pull-only product gate
      succeeds; its release assets and checksums are inventoried afterward.
  - Check:
    - `PLAN.md` records the exact RC10 failure, bounded repair, immutable-tag
      rule, evidence plan, and remaining clean-host boundary before workflow
      implementation.
    - The notes step writes the complete generated body as mode-0600 UTF-8
      under `RUNNER_TEMP`, exposes only that path, and the pinned release action
      consumes `body_path`. A focused regression rejects the RC10 inline-body
      form.
    - The focused regression and complete pinned Go 1.26.5 `./scripts` suite
      pass. Both CI policy scripts pass, all 19 GitHub YAML files parse, and
      `git diff --check` passes.
    - Literal `./scripts/verify.sh` passes in 146 seconds: release bundle, all
      Go packages, contract invariants, migrated Postgres, Compose rendering
      and builds, and healthy SRS/controller/OME/transcoder/API/viewer all pass.
      Cleanup leaves no BitRiver containers; the private root `.env` is
      restored byte-for-byte at SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`.
    - PR #1351 run `30667406420` passed the complete Ubuntu gate, both image
      gates, native arm64 viewer proof, macOS/Windows Go, all three quickstart
      entrypoints, workflow policy, and committed-secret guard. The PR squash
      merged as exact main commit `96e99fd6`.
    - Exact-main CI run `30667801411` passed the same release-sensitive matrix.
      Annotated immutable tag `v1.2.3-rc.11` points at that verified commit.
    - Release run `30668214206` passed every producer, hosted MSI, native
      multi-architecture viewer publication, Ubuntu/Debian/Rocky package
      acceptance, Homebrew, all image/SBOM publishers, the pull-only OME-backed
      product gate, the complete payload scan, retained evidence upload, and
      file-backed GitHub Release publication.
    - The public RC11 prerelease contains 33 uploaded assets. `CHECKSUMS.txt`
      has 32 entries: every entry maps to a release asset and every other asset
      is covered. Retained `release-scan-status.json` binds both passed scans to
      tag `v1.2.3-rc.11` and commit `96e99fd6`.

- [x] Task 10 - Execute ancestry-safe remote branch cleanup
  - Acceptance criteria:
    - Immediately refetch/prune and recompute ancestry against current
      `origin/main` before deletion.
    - Delete only non-default refs whose tips are ancestors of `origin/main`;
      preserve tags, `main`, every non-ancestor ref, and every open PR head.
    - Use bounded batches, stop on any classification change/failure, and
      publish before/after counts plus the retained-branch inventory.
  - Check:
    - The post-RC11 fetch/prune and GitHub API both report 1,002 real remote
      branches including `main`: 942 ancestry-merged non-default refs and 59
      non-ancestor refs. GitHub reports no protected branches.
    - Open PR heads `dependabot/github_actions/docker/login-action-4.5.2` and
      `dependabot/npm_and_yarn/web/viewer/viewer-tooling-4767752826` are in the
      preserved non-ancestor set.
    - All 942 ancestry-merged refs were deleted in 19 atomic batches: 18 batches
      of 50 and one batch of 42. Each batch followed a fresh fetch and exact
      object-ID, ancestry, and open-PR exclusion revalidation.
    - During final reconciliation, PRs #1346 and #1344 merged independently as
      `bf57dd8f` and `d3740828`; GitHub deleted their two heads and advanced
      `main`. Neither head was in the cleanup candidate set.
    - Final local refs and GitHub API agree on 58 branches: `main`, zero merged
      non-default refs, and these 57 retained non-ancestor refs:

      ```text
      chore/close-1295-ledger
      chore/runtime-supported-baselines
      codex/add-hasrole-method-and-refactor-usage
      codex/add-paths-filters-and-concurrency-to-workflows
      codex/add-periodic-session-cleanup-in-server
      codex/add-replace-directive-in-go.mod
      codex/enforce-required-image-tags-in-deploy-script
      codex/extend-navbar-with-routing-and-styles
      codex/fix-extraction-process-failure
      codex/fix-high-priority-bug-in-chat-api-client
      codex/fix-high-priority-bug-in-ome-config-rendering
      codex/fix-high-priority-bug-in-test-postgres.sh
      codex/fix-high-priority-bug-in-upload-flow
      codex/fix-metrics-protection-in-setup-wizard
      codex/fix-missing-page-links-in-navbar
      codex/fix-missing-variable-error-in-release-workflow
      codex/fix-navbar-search-params-test-issues
      codex/fix-path-filters-in-ci-workflows
      codex/fix-pgx-stub-to-maintain-postgres-access
      codex/fix-postgres-dsn-unreachable-host-issue
      codex/fix-purge-worker-shutdown-blocking-issue
      codex/fix-srs-ingest-hook-endpoint-issues
      codex/fix-unauthenticated-profile-read-issue
      codex/implement-setup-wizard-for-configurations
      codex/issue-1221-browse-discovery
      codex/issue-1222-following-focus
      codex/issue-1223-channel-experience
      codex/issue-1224-management-primitives
      codex/issue-1225-mobile-viewer-layout
      codex/issue-1226-split-viewer-css
      codex/issue-1229-cleanup-tracking
      codex/issue-1241-quickstart-shell-tests
      codex/issue-1242-transcoder-test-stability
      codex/issue-1243-auth-server-time-tests
      codex/issue-1244-ingest-postgres-cancel
      codex/issue-1245-legal-postgres-timeouts
      codex/issue-1246-extract-upload-helpers
      codex/mirror-postgres-service-to-release.yml
      codex/move-stub-implementations-behind-build-constraint
      codex/replace-text-inputs-with-drag-and-drop-file-upload
      codex/update-authmiddleware-for-optional-get-requests
      codex/update-chat-helpers-and-tests
      codex/update-cross-platform-documentation-and-consistency
      codex/update-cross-platform-documentation-and-consistency-pbwl9h
      codex/update-healthcheck-auth-mode-normalization
      codex/update-healthcheck-auth-mode-normalization-g5rchq
      codex/update-healthcheck-auth-mode-normalization-iqu0ud
      codex/update-healthcheck-auth-mode-normalization-nllxqe
      codex/update-healthcheck-auth-mode-normalization-uivqm8
      codex/update-health-check-to-probe-upstream-srs
      codex/update-health-check-to-probe-upstream-srs-r6kswc
      codex/update-server.xml-for-ome-bind-setting-pg8dxl
      docs/roadmap-2026-07
      feat/migration-ledger
      feat/ubuntu-clean-host-installer
      feat/windows-docker-readme
      feat/windows-verify-wrapper
      ```

- [x] Task 11 - Audit the repository as a first-time installer
  - Acceptance criteria:
    - Inventory README claims, screenshots, release/download links, install
      commands, supported hosts, reverse-proxy/media ports, first-stream flow,
      upgrades/backups, troubleshooting, and evidence boundaries against code
      and the published candidate.
    - Identify stale, duplicated, source-only, or no-longer-available guidance
      before rewriting public docs.
    - Record a bounded screenshot plan using only current product/runtime
      captures with known provenance.
  - Check:
    - README, quickstart, Ubuntu-install, viewer-deployment, release-note,
      production-status, and production-release guidance were compared with
      the current code, workflow, and public RC11 assets. Stale claims include
      "no GitHub Release", a planned `rc.1`, source-only installation, and an
      unimplemented GitHub Pages workflow.
    - The two committed README screenshots are real captures of the shipped
      viewer, not generated promotional art. They remain usable as honest
      workflow evidence, although stronger post-broadcast captures are a
      future presentation improvement.
    - The reverse-proxy runbook correctly separates same-origin app/API,
      `/hls`, RTMP, private management ports, authenticated OME control, and
      real media-decode acceptance. The first-stream labels were confirmed
      against the current viewer.
    - RC11 published 33 checksum-complete assets and passed its pull-only
      product gate, but its generated release description is an uncurated
      124,999-character history dump and needs a concise operator-facing
      replacement in the next candidate.
    - Directly downloaded RC11 Linux `.deb` and launcher payloads both contain
      stable `v1.2.3` for all five first-party image tags. Because RC11
      publishes only `v1.2.3-rc.11`, its first-install path is blocked before
      activation; documentation refresh is paused until a corrected immutable
      candidate is published and re-downloaded.

- [x] Task 11a - Stamp and prove exact image tags in release artifacts
  - Acceptance criteria:
    - The canonical staging helper accepts and validates an optional release
      tag, then changes only the staged `.env.example` image-tag values.
    - Every GitHub release packaging path supplies the exact immutable tag;
      focused tests cover all five values and preserve the canonical source.
    - Focused checks, literal repository verification, PR CI, and exact-main
      CI pass before `v1.2.3-rc.12` is tagged.
    - The published RC12 `.deb` and launcher archive are downloaded and
      independently inspected before the Ubuntu path is documented as usable.
  - Check:
    - `stage-release-assets.sh --release-tag` now validates the same supported
      SemVer tag shape as release preparation, rejects numeric prerelease
      identifiers with leading zeroes, and rewrites exactly one copy of each
      first-party tag assignment in only the staged env.
    - All three release staging paths pass `${{ github.ref_name }}` explicitly:
      cross-platform release binaries, launcher/Linux packages, and the hosted
      Windows MSI. A Go workflow regression requires exactly three tagged
      calls and their job/step environment boundaries.
    - The release-bundle regression passed with a path containing spaces,
      exact `v1.2.3-rc.12` values for all five images, invalid-tag rejection
      before output creation, and byte-identical source env preservation.
    - The pinned nFPM v2.47.0 acceptance built stable amd64/arm64 and separate
      tag-correct prerelease amd64 `.deb`/`.rpm` payloads. The Linux path now
      extracts the prerelease Debian payload and checks all five installed
      values; that extraction awaits CI because the local host is Windows.
    - Complete pinned Go 1.26.5 `go test ./scripts`, both CI configuration
      checks, shell syntax, all 19 GitHub YAML parses, and `git diff --check`
      pass.
    - Literal `./scripts/verify.sh` passed in 161.3 seconds, including all Go
      packages, release contracts, migrated Postgres, Compose rendering/build,
      and healthy SRS/controller/OME/transcoder/API/viewer. Teardown left no
      BitRiver containers, generated configs have no diff, and the private
      root `.env` was restored byte-for-byte at SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`.
    - PR #1353 run `30671011370` passed the unified Ubuntu product gate,
      image scans, native arm64 viewer proof, Windows/macOS Go tests, and the
      Windows quickstart entrypoint. Its only defect was ShellCheck SC2016 on
      an intentional literal `$RELEASE_TAG` assertion, repeated by the Linux
      and macOS quickstart jobs; the assertion now uses an escaped double-
      quoted pattern so the literal contract remains intact without a lint
      suppression.
    - The corrected PR run `30671401731` passed every required job. PR #1353
      squash-merged as exact main commit `3a9572f0`, and exact-main CI run
      `30671758215` passed the full matrix before the tag was created.
    - Annotated immutable tag `v1.2.3-rc.12` triggered release run
      `30672085853`; every producer, hosted MSI, three Linux package-acceptance
      hosts, five image/SBOM publishers, multi-architecture viewer manifest,
      pull-only OME-backed product gate, artifact scan, and atomic publisher
      passed.
    - The public RC12 prerelease has 33 assets and 32 checksum entries covering
      every other asset. Retained publication evidence binds both passed scans
      to tag `v1.2.3-rc.12` and commit `3a9572f0`.
    - Independently downloaded checksum-verified Linux amd64 launcher and
      `.deb` payloads were extracted. Each contains exactly five first-party
      tag assignments at `v1.2.3-rc.12` with no stale stable value. Retained
      product evidence passed all eight stages, including RTMP live state,
      decoded live media, offline transition, chat/moderation, and VOD.

- [x] Task 12 - Rewrite README around the consumer golden path
  - Acceptance criteria:
    - Lead with the product, honest support boundary, prerequisites, and two
      visible install choices: Docker Desktop evaluation and Ubuntu 24.04
      artifact/package hosting.
    - Show install, configure, verify, first stream, playback, reverse proxy,
      upgrade, backup, and recovery entry points without duplicating runbooks.
    - Replace stale promotional imagery with verified current product captures
      or concise workflow diagrams.
  - Check:
    - README now leads with the product, a linked RC12 prerelease badge/status,
      and provenance-known captures of the shipped viewer instead of generated
      promotional art.
    - A consumer choice table and copy-paste paths cover native PowerShell plus
      Docker Desktop evaluation and checksum-verified Ubuntu 24.04 amd64
      package installation, with the two-phase activation boundary explicit.
    - Creator setup, first OBS broadcast, same-origin playback, the SRS/API/OME
      mapping, NPM routing, RTMP/UDP exposure, operations, recovery, support,
      and contribution entry points are concise and link to deeper runbooks.
    - RC12's actual package/image/product proof is separated from pending clean
      XOA VM, NPM browser, reboot, and repeated OME recovery evidence.
    - Installer wording checks and `git diff --check` pass; all 32 README local
      Markdown link/image targets resolve.

- [x] Task 13 - Align install, operations, release, and support documentation
  - Acceptance criteria:
    - Quickstart, Ubuntu/XOA/Nginx Proxy Manager, production release,
      operations, security, architecture, troubleshooting, upgrade/backup, and
      release-note docs use the same shipped commands and support claims.
    - Release assets and image names match the actual GitHub prerelease; OME
      diagnostics and bounded recovery remain explicit.
    - Stale/no-release wording and contradictory source-only paths are removed
      or clearly labeled for contributors.
  - Check:
    - Quickstart and Ubuntu guides now link the public RC12 assets, use exact
      package/download names, distinguish Docker Desktop/source evaluation
      from boot-managed hosting, and retain the clean-host/NPM/reboot boundary.
    - Production status/release, operations, single-host sizing, reverse proxy,
      upgrades, support, testing, dependency policy, versioning, and the
      release-note template now use shipped commands and current Go 1.26.5,
      Node 24, Next.js 16.2.11, package, and evidence contracts.
    - Added a concise published RC12 note and changelog record; the stable-line
      draft and release index now distinguish the candidate from future stable
      promotion. Removed the nonexistent GitHub Pages deployment path.
    - The documentation guard now rejects old no-release claims, superseded RC
      examples, Pages action claims, missing current-candidate references, and
      missing/empty real viewer captures.
    - Installer wording, generated contract freshness, Compose contract
      invariants, and `git diff --check` pass. All 102 local targets across 22
      changed Markdown files resolve, and the private root `.env` was restored
      byte-for-byte after Compose validation.

- [x] Task 14 - Verify and publish the repository refresh
  - Acceptance criteria:
    - Link/reference/wording regressions, generated docs, Markdown checks,
      required repository verification, PR CI, and exact merged-main CI pass.
    - Final public README rendering and all referenced images/links are checked
      against GitHub before merge.
    - The handoff distinguishes proven Docker Desktop/tag/package behavior from
      still-pending clean XOA/NPM/reboot/repeated OME recovery evidence.
  - Check:
    - The new tracked-Markdown link checker and its three unit tests pass across
      85 public Markdown files; installer wording, generated contract freshness,
      13 workflow YAML parses, CI contract checks, ShellCheck, and
      `go test ./scripts -count=1 -timeout=120s` also pass.
    - The dependency-policy guard caught the removed review deadline during this
      pass; the current zero-high/critical Next.js override disposition now keeps
      its required 2026-08-12 review date.
    - The first full-gate attempt exposed missing Python-runner assignment in
      `scripts/verify.sh`; the launcher is now retained for `python3`, `py -3`,
      and `python`, with a Go workflow-contract regression. Focused Go,
      `bash -n`, CI-contract, and ShellCheck validation pass.
    - Literal `./scripts/verify.sh` passed in 153 seconds, including all Go
      packages, PostgreSQL migrations, Compose rendering, healthy SRS,
      authenticated OME token/health, transcoder, API, and viewer smoke. It left
      no BitRiver containers, and the private root `.env` was restored to SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`.
    - Both README images were visually inspected at original resolution and are
      real shipped-viewer captures.
    - GitHub rendered the branch README successfully with all three badges, both
      real screenshots, the install table and code blocks, the Mermaid media
      flow, and repository-relative navigation intact. The public repository
      About description and topics now match the one-host Compose/Ubuntu support
      boundary.
    - PR #1354 CI runs `30674183171` and `30674523079` passed the Ubuntu
      `test-all` gate, Windows
      and macOS Go tests, macOS/Windows/Ubuntu quickstart entrypoint checks,
      Docs consistency, ShellCheck, changed-file detection, and the committed
      secret guard. The PR squash-merged as `f3952624`; exact merged-main CI run
      `30779028786` passed the same required matrix, and deleting the PR branch
      returned the remote inventory to 58 branches.

## Scoped change: remove tracked generated Go caches

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 15 - Audit root cache provenance and removal boundary
  - Acceptance criteria:
    - The exact tracked roots, file count, current-tree byte size, introduction
      commit, ignore coverage, and references outside those roots are recorded.
    - The plan explicitly excludes history rewriting and every non-cache path.
  - Check:
    - `.gocache-bitriver-fix`, `.gocache-ingest`, `.gocache-metrics-fix`, and
      `.gocache-scripts-fix` contain exactly 6,775 tracked generated objects and
      416,199,766 bytes in `HEAD`; commit `b4617f75` introduced them.
    - No repository path outside those roots references them. `.gitignore`
      already contains both `.gocache/` and `/.gocache-*/` coverage.

- [x] Task 16 - Remove the caches and add a tracked-cache regression
  - Acceptance criteria:
    - Exactly the 6,775 audited paths are removed and no unrelated path changes.
    - A focused guard accepts the cleaned repository and rejects a force-added
      root `.gocache-*` artifact in an isolated fixture repository.
  - Check:
    - `git rm` removed exactly 6,775 tracked paths under the four audited roots;
      an explicit path inventory found zero deletion outside those roots.
    - Added `scripts/check-repository-hygiene.sh` plus an isolated fixture test.
      The cleaned repository passes, while a temporary repository with a
      force-added `.gocache-forced/00/cache-entry-a` is rejected with the exact
      offending path.
    - The guard and its fixture test now run near the start of
      `scripts/verify.sh`. Shell syntax, the focused fixture, cleaned-tree guard,
      committed-secret guard, and staged/unstaged diff checks pass.

- [x] Task 17 - Verify and publish repository hygiene cleanup
  - Acceptance criteria:
    - Focused guard tests, deletion inventory, secret guard, `git diff --check`,
      literal `./scripts/verify.sh`, PR CI, and exact merged-main CI pass.
    - Large pull requests cannot truncate CI path routing before changed
      verification scripts are evaluated; a focused workflow regression locks
      the complete-checkout Git diff boundary.
    - The cleanup head is deleted after merge; any concurrent branch-count
      drift is classified before unrelated remote work is changed.
  - Check:
    - Docker Desktop initially stopped at a failed automatic-update recovery
      dialog because its staging check saw 348 MB free against a 3,394 MB
      requirement. The installed 4.83.0 / Engine 29.6.2 rollback recovered
      without deleting unrelated user files, and Docker became healthy.
    - Docker-backed ShellCheck passed. Literal `./scripts/verify.sh` passed in
      140.3 seconds, including the new fixture/clean-tree guard, all Go packages,
      PostgreSQL migrations, Compose rendering, healthy SRS, authenticated OME
      token/health, transcoder, API, and viewer smoke.
    - Verification left no BitRiver containers, no generated-config drift, and
      restored the private root `.env` to SHA-256
      `9D57F7161B241315158B0654CA51DA997A8BBF9408A1D6E944AE39648D91AAC2`.
      The staged inventory remains exactly 6,775 audited deletions and zero
      unexpected deletion.
    - PR #1361 run `30784385158` exposed the existing GitHub REST changed-files
      limit: `dorny/paths-filter` received 3,000 of 6,780 paths, all substantive
      filters returned false, and the Ubuntu, ShellCheck, and quickstart jobs
      skipped despite three changed `scripts/**` files. The pinned action's
      documented empty-token mode uses the complete checkout plus `git diff`;
      the workflow correction, rerun PR CI, and merged-main CI remain pending.
    - Added the empty-token input without changing any selector, plus a focused
      regression requiring both the complete-history checkout and Git fallback.
      Pinned Go 1.26.5 `go test ./scripts -count=1 -timeout=120s`, the Linux CI
      contract check, workflow YAML parsing, committed-secret guard, shell
      syntax, and `git diff --check` pass. The host's Go 1.25.6 was correctly
      rejected by the module's Go 1.26 requirement.
    - Corrected PR run `30784659494` detected the complete diff in 9 seconds
      and selected the intended jobs. Ubuntu `test-all` passed in 4m16s,
      ShellCheck passed, macOS/Windows Go tests passed, and Ubuntu/macOS/Windows
      quickstart entrypoint checks passed; unrelated viewer, docs, monitoring,
      image-scan, wizard, and Go-workflow jobs remained correctly skipped.
    - Final PR-head run `30784972644` passed the same selected matrix. PR #1361
      squash-merged as `5e736f2b`, its remote head was deleted, and exact
      merged-main run `30785253246` passed Ubuntu `test-all` in 4m17s,
      ShellCheck, macOS/Windows Go, and all three quickstart entrypoint checks.
    - Post-merge pruning removed four stale local tracking refs. GitHub reports
      64 current branches: the previous 58-branch baseline plus six separate
      viewer dependency heads authored by `dependabot[bot]` at 02:36 UTC. They
      were preserved as concurrent, non-audited work rather than blindly
      deleted to force the old numeric count.
    - The completion-ledger branch reran literal `./scripts/verify.sh` with the
      existing pinned Windows Go 1.26.5 toolchain and a temporary copy upgraded
      through the supported `bitriver env init` path. It passed in 150.9 seconds
      through Go, Postgres, Compose, SRS, authenticated OME health, transcoder,
      API, and viewer smoke. The private `.env` was restored to its original
      SHA-256, both temporary env files were removed, and zero Compose-labeled
      containers or volumes remained.

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
