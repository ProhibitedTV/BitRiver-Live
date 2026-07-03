## Scoped change: release, push, and first-run deployment readiness

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Reframe the release/deploy scope
  - Acceptance criteria:
    - `PLAN.md` captures release, first-run guidance, deployment, and verification scope.
    - `TASKS.md` lists ordered tasks before any source/doc edits for this pass.
    - Unsafe local deployment artifacts are identified before staging.

- [x] Task 2 - Tighten first-run process guidance
  - Acceptance criteria:
    - Any doc/process change uses commands that already exist in the repository.
    - Windows/PowerShell guidance avoids the WSL Bash failure mode seen during verification.
    - Verification no longer prints local secrets during a successful Compose config check.
    - Docker source-build context excludes local caches and generated viewer build output.
    - No deployment contract files are changed.

- [x] Task 3 - Re-run release verification
  - Acceptance criteria:
    - Diff hygiene passes.
    - `./scripts/verify.sh --viewer` passes or blockers are recorded.
    - The release diff excludes secrets and unsafe local artifacts.

- [x] Task 4 - Commit, merge/sync, and push
  - Acceptance criteria:
    - `origin/main` relationship is checked before pushing.
    - Commit includes only intended scoped files.
    - Push to the requested remote branch succeeds.

- [x] Task 5 - Deploy and prove health
  - Acceptance criteria:
    - Canonical quickstart/Compose deployment is run.
    - API and viewer endpoints respond.
    - Smoke/health checks pass or blockers are recorded.

### Execution log
- Task 1 read-only pass:
  - `git status --short --branch` showed the viewer simplification diff on `main` plus untracked deployment docs/scripts.
  - Read root/deploy/scripts/docs agent notes, `PLAN.md`, `TASKS.md`, quickstart docs, canonical scripts, and deployment docs before source edits.
  - Inspected untracked deployment files and found local credential values plus non-canonical/stale deployment workflow claims; they will not be staged as-is.
- Task 2 complete: updated `README.md`, `docs/quickstart.md`, and `deploy/README.md` with native PowerShell quickstart/preflight guidance and Git Bash/WSL clarification without changing deployment contract files.
- Task 2 follow-up: updated `scripts/verify.sh` so successful Compose config validation prints only a success line and failure output is redacted; updated `.dockerignore` to keep local caches and generated viewer build output out of Docker build contexts.
- Task 3 checks:
  - `git diff --check` - passed with line-ending normalization warnings only.
  - `& 'C:\Program Files\Git\bin\bash.exe' -n ./scripts/verify.sh` - passed.
  - `& 'C:\Program Files\Git\bin\bash.exe' ./scripts/verify.sh --viewer` - passed after verifier redaction and Docker-ignore updates; included Go tests, contract checks, Docker Compose config validation, quickstart smoke, viewer lint, and full viewer Jest suite.
  - `docker build --progress=plain --target builder --build-arg GOPROXY=direct --build-arg GOSUMDB=off -f Dockerfile .` - passed and confirmed the root Docker build context is down to roughly 30 KB for the API builder path.
  - Notes: viewer lint still reports the existing `UploadManager` exhaustive-deps warning; Jest still emits existing React `act(...)` warnings while all tests pass.
- Task 4 complete:
  - `git fetch origin --prune` - passed.
  - `git rev-list --left-right --count main...origin/main` - returned `0 0` before release staging.
  - `git add -u` staged tracked files only; unsafe untracked deployment artifacts were left untracked.
  - `git commit -m "viewer: simplify UI and first-run flow"` created `68ba0bbb`.
  - `git push origin main` pushed `main` to `origin/main`.
- Task 5 deployment proof:
  - `./scripts/quickstart.sh --compose-file deploy/docker-compose.yml --image-source build` correctly rejected the local persisted production/build `.env` combination.
  - Local ignored `.env` was restored to development mode for a source build without staging secrets or contract files.
  - `go run ./cmd/bitriver env render-ome --env-file ./.env --output deploy/ome/Server.generated.xml` rendered the local OME config.
  - `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build --pull never` built and started the stack.
  - `docker compose --env-file .env -f deploy/docker-compose.yml ps` showed core services healthy and viewer/transcoder public services running.
  - `Invoke-WebRequest` probes returned HTTP 200 for `/healthz`, `/readyz`, and `/viewer`.
  - First smoke run exposed Docker Compose 5 newline-delimited JSON output from `ps --format json`; updated the smoke parser to accept both arrays and JSON object streams.
  - Focused Go smoke parser tests passed.
  - `go run ./cmd/bitriver smoke --compose-file deploy/docker-compose.yml --env-file ./.env` passed all 7 smoke checks against the running deployment.
  - Final `./scripts/verify.sh --viewer` rerun passed after the smoke parser fix; it rebuilt/restarted the source stack, completed quickstart smoke, and ran viewer lint plus all 205 Jest tests.
