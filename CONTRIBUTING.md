# Contributing to BitRiver Live

Thanks for taking the time to contribute.

## Before you start

- Read the root [README.md](README.md) for the current project scope and quickstart path.
- Read [SUPPORT.md](SUPPORT.md) if you are unsure whether your question or change is in the best-supported scope for this repository.
- Use the documented deployment contract: the repository-root `.env` together with `deploy/docker-compose.yml`.
- For behavior changes, update docs in the same pull request when setup, runtime behavior, or operator workflow changes.

## Good first contributions

- Documentation fixes and onboarding improvements.
- Focused bug fixes with tests.
- Small UX, validation, or error-message improvements.
- CI, release, or packaging fixes that stay within the existing deployment model.

Please open an issue before starting large features, cross-cutting refactors, or deployment-contract changes.

## Maintainer priorities

Maintainers currently prioritize:

- the supported single-host Docker Compose baseline
- onboarding and documentation clarity
- release and packaging reliability
- focused fixes with tests or reproduction steps

Changes aimed at unsupported deployment models or broad speculative refactors are less likely to be reviewed quickly unless they first align with the supported baseline.

## Local setup

1. Copy `deploy/.env.example` to `.env` at the repository root.
2. Review the required values and rotate sample credentials.
3. Start with the documented quickstart:

```bash
go run ./cmd/bitriver quickstart --compose-file deploy/docker-compose.yml
```

Helpful references:

- Quickstart: [`docs/quickstart.md`](docs/quickstart.md)
- Architecture: [`docs/architecture.md`](docs/architecture.md)
- Testing: [`docs/testing.md`](docs/testing.md)
- Deployment contract: [`docs/contract.md`](docs/contract.md)
- Support expectations: [`SUPPORT.md`](SUPPORT.md)

## Development workflow

- Keep changes small and reviewable.
- Prefer one behavior change per pull request.
- If you touch runtime code, keep handlers/services context-aware and preserve the existing dependency-injection boundaries.
- Do not commit secrets, local `.env` files, generated temp files, or local release artifacts.

For larger scoped work, maintainers use a lightweight `SPEC.md -> PLAN.md -> TASKS.md` flow to keep reasoning and execution visible. You do not need to use that for every typo fix, but it is encouraged for multi-file or higher-risk changes.

## Validation

Run the closest relevant checks before opening a pull request.

Recommended full gate:

```bash
./scripts/verify.sh
```

Common focused commands:

```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s
npm --prefix web/viewer run lint
npm --prefix web/viewer run test
docker compose --env-file .env -f deploy/docker-compose.yml config
```

If Docker-dependent checks are blocked on your machine, note that clearly in the pull request.

## Pull requests

- Explain what changed and why.
- Call out any operator-facing or deployment-contract changes.
- Include screenshots or short recordings for UI changes when useful.
- Mention the commands you ran.
- Link the issue being fixed, if there is one.
- Keep claims tight and source-backed; if a limitation remains, document it instead of hand-waving around it.

## Commit and release expectations

- Follow SemVer expectations in [`docs/versioning.md`](docs/versioning.md).
- Keep release-impacting changes aligned with [`docs/production-release.md`](docs/production-release.md).
- If you change release behavior, packaging, or upgrade steps, update the relevant docs in the same pull request.
