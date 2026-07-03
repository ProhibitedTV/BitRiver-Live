## Scope (current change)
- Release the viewer UI/UX simplification work already implemented and verified.
- Improve first-run guidance and verification ergonomics only where it removes ambiguity found during local verification, especially Windows shell guidance, secret-safe verifier output, source-build context size, and Docker Compose 5 smoke parsing.
- Preserve the canonical deployment contract: no changes to `deploy/docker-compose.yml`, root `.env`, generated OME expectations, backend API shape, auth behavior, or data model.
- Exclude unsafe untracked deployment reports/scripts unless they are sanitized and intentionally folded into canonical docs.

## Assumptions
- A first-time source-checkout user should prefer `go run ./cmd/bitriver quickstart` or `scripts/quickstart.ps1` on Windows PowerShell, not a Bash wrapper that might resolve to a broken WSL install.
- The existing Go quickstart, env validation, smoke command, and `./scripts/verify.sh --viewer` remain the source of truth for deployment readiness.
- The untracked deployment assurance/guide files are local artifacts because they contain environment credentials and stale parallel workflow claims.
- Docker Compose may print `ps --format json` either as one JSON array or as newline-delimited JSON objects depending on Compose version.

## Risks
- Staging untracked deployment files could leak secrets or document a non-canonical startup path.
- Process tweaks can become stale if they introduce commands or Docker-ignore rules that do not match the repository build.
- Pushing directly from `main` requires confirming `origin/main` has not moved unexpectedly.
- Deployment verification depends on Docker Desktop/engine availability and local ports.
- Smoke parser changes are shared with quickstart diagnostics and need focused CLI coverage.

## Test plan
- `git diff --check`
- `./scripts/verify.sh --viewer`
- `BITRIVER_VERIFY_SOURCE_ONLY=1 ./scripts/verify.sh`
- `go test ./cmd/bitriver -run "TestRunSmoke|TestSmokeCheckComposeState|TestParseComposeServiceStatesSupportsNDJSON|TestPortOrDefault" -count=1 -timeout=120s`
- `git fetch origin --prune`
- `git rev-list --left-right --count main...origin/main`
- `git status --short --branch`
- Deploy with the canonical quickstart or equivalent Compose path.
- Smoke/health checks after deployment: `go run ./cmd/bitriver smoke --env-file ./.env`, Compose status, and viewer/API endpoint probes.
