# PLAN

## Scope
- Resolve production blocker #1295 by moving Go, Node.js, Next.js, React, and viewer tooling onto supported production lines.
- Preserve the repository's offline local verification path while making every production build use upstream Go modules instead of local `third_party` replacements.
- Migrate the viewer from removed Next.js 13 APIs and configuration without redesigning unrelated viewer or admin workflows.
- Establish blocking high/critical dependency gates, grouped automated updates, and a documented exception policy.
- Prove release binaries, containers, `/viewer` base-path behavior, and standalone viewer packaging use the intended runtimes and dependencies.

## Baseline Decision
- Go minimum: 1.26.0; pinned CI and container toolchain: 1.26.5.
- Node.js: supported 24 LTS major across CI, containers, and contributor guidance.
- Viewer: Next.js 16.2.10, React/React DOM 19.2.7, TypeScript 6.0.3, and ESLint 9.39.5 with flat configuration.
- Upgrade Jest, Playwright, Testing Library, and React/Node types to compatible maintained releases recorded in `package-lock.json`.
- Keep local replacements only for the explicit offline test path. Production module preparation must remove every replacement whose target is under `third_party/`, not a hand-maintained subset.

## Assumptions
- The canonical Compose shape and root `.env` do not need to change.
- Next.js 16 request APIs may require small route-component changes, but route ownership and information architecture are outside this issue.
- Production builds may access the public Go module proxy; local verification remains network-independent.
- A high-severity dependency finding may be accepted only through a documented, owner-assigned, time-bounded exception. Critical findings are never releasable.

## Risks
- A partial replacement cleanup can silently publish stubbed implementations; generate one production modfile and inspect built binaries for all replacement metadata.
- A major Next/React upgrade can pass type checks while breaking hydration, base paths, or standalone output; cover route request APIs, build output, Playwright navigation, and container smoke.
- Running the latest major of every auxiliary tool adds unnecessary migration risk; update only to a mutually compatible maintained set and review the lockfile and audit result.
- Toolchain strings duplicated across Dockerfiles, CI, and docs can drift; add static contract tests that compare every production declaration to the baseline.
- Blocking audits can become routinely bypassed if exceptions are vague; require advisory, impact, owner, mitigation, and expiry fields in tracked policy.
- Real pgx and go-redis behavior can diverge from offline mirrors; keep real-driver-only assertions guarded on the offline graph, run them with the production modfile upstream, and fix ownership, protocol, row-lifecycle, and scan-contract defects at the narrowest boundary.
- The staged-release wizard fixture can drift from packaged binary names; keep it aligned with the required `bitriver-live` and `bootstrap-admin` pair and exercise that exact layout in CI.
- Quickstart entrypoint checks that execute the Go CLI must install the pinned `.go-version` toolchain first; otherwise Go's automatic toolchain probe can pass before the wrapper's intentional `GOTOOLCHAIN=local` execution falls back to an older runner binary.
- Runtime-baseline assertions must tolerate Git's CRLF checkout policy on Windows while still matching an exact Go directive; normalize line endings inside the test rather than rewriting module files.
- Next.js streaming may briefly overlap the directory Suspense fallback with settled content; Playwright assertions must identify the settled relay state before requiring a single hero instead of using a strict shared-class locator during the reveal.

## Test Plan
- Focused tests for production modfile generation, replacement rejection, runtime-version alignment, installer minimum-version behavior, and workflow audit policy.
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s` with Go 1.26.5.
- Postgres-tagged tests, race/static checks where practical, `govulncheck`, and release-binary metadata inspection proving real pgx and no local replacements.
- Run the real-pgx storage suite against an isolated migrated Postgres database; verify cleanup order, nested-query row lifetimes, subscription column parity, and pending-status visibility.
- `npm run lint`, `npm test`, `npm audit --audit-level=high`, `npm run build`, and Playwright integration tests under Node 24.
- Docker Compose config validation, production image builds, `/viewer` base-path/standalone smoke, and `./scripts/test-quickstart.sh`.
- `./scripts/verify.sh`, `git diff --check`, pull-request CI, and release-workflow static checks before squash merge.

## Boundaries
- Do not modify `deploy/docker-compose.yml`, root `.env`, or generated OME values.
- Do not combine the runtime migration with viewer/admin route consolidation or a visual redesign.
- Do not remove the offline test mirrors until a replacement local-development contract is separately approved.
- Do not stage or modify unrelated local deployment-guide and diagnostics files.
