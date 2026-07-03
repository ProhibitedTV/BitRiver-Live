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

- [-] Task 4 - Commit, merge/sync, and push
  - Acceptance criteria:
    - `origin/main` relationship is checked before pushing.
    - Commit includes only intended scoped files.
    - Push to the requested remote branch succeeds.

- [ ] Task 5 - Deploy and prove health
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
