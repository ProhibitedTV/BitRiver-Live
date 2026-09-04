# Dependency checksum policy

BitRiver Live keeps the default verification path offline:

- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off`
- `./scripts/verify.sh`

Those defaults are intentional and must remain unchanged for local and CI verification.

Production binaries and images use a separate, generated module graph:

```bash
go run ./cmd/tools/production-module --output go.production.mod
go mod download -modfile=go.production.mod all
```

The helper removes every local `third_party` replacement without mutating `go.mod`. Production builds must compile with `-modfile=go.production.mod` and pass `cmd/tools/verify-production-binary`; hand-maintained `-dropreplace` lists are prohibited because they drift as dependencies change.

## When maintainers may refresh `go.sum`

Refresh `go.sum` only when dependency metadata changes, including:

- updates to `go.mod` `require` entries,
- replacement strategy updates for `third_party/` mirrors,
- intentional checksum resync after dependency mirror refreshes.

Do **not** refresh `go.sum` as part of unrelated changes.

## Controlled checksum refresh procedure (network-enabled)

Run this from repository root in a network-enabled shell:

```bash
./scripts/refresh-go-sum.sh
```

The script derives the upstream graph through `cmd/tools/production-module`, downloads it in an isolated temporary module file, and writes a non-empty root `go.sum`.

## Required review before commit

Before committing a refreshed `go.sum`, maintainers must review the dependency diff:

```bash
git diff -- go.sum
```

Only commit expected checksum changes.

## Read-only guard

Use this read-only check locally or in CI to ensure `go.sum` was not cleared:

```bash
./scripts/check-go-sum-not-empty.sh
```

## Supported production lines

- Go: minimum 1.26.0; CI and builder images pin 1.26.5 through `.go-version`. Patch releases are adopted promptly, and a new Go major is evaluated before the current line leaves the official two-release support window.
- Node.js: 24 LTS only for the viewer. `.nvmrc`, CI, release jobs, and the viewer image must agree on major 24.
- Next.js: 16.x Active LTS with React 19.2.x. Security and patch updates are reviewed within seven days; major upgrades require the full viewer build and Playwright gates.
- Lockfiles and checksums are release inputs. Dependency PRs must include the resolved diff, audit result, and applicable runtime tests.

Dependabot checks GitHub Actions, Go modules, and viewer npm dependencies weekly. Minor and patch updates are grouped by runtime area; major updates remain isolated for explicit migration review.

## Vulnerability gates and exceptions

`govulncheck` reachable findings and npm high/critical findings block CI and release. Critical findings cannot receive a release exception. A temporary high-severity exception requires all of the following in a dedicated security PR:

- advisory ID and affected package/module,
- reachability and deployment impact analysis,
- named owner and tracking issue,
- mitigation or compensating control,
- ISO expiry date no more than 30 days out.

Go exceptions live in `scripts/govulncheck-baseline.json`; the scanner rejects incomplete or expired entries. npm remains fail-closed and has no exception file. If an npm high finding must be accepted, maintainers must first add an equivalently validated, package-and-advisory-specific policy mechanism in the same reviewed security PR. Never use `continue-on-error`, lower the audit threshold, or run `npm audit fix --force` to bypass the gate.

## Current below-threshold disposition

As of 2026-09-04, the viewer uses Next.js 16.3.3. Browserslist 4.28.9,
PostCSS 8.5.28, and Sharp 0.35.3 are explicit fixed overrides. A clean npm audit
reports zero findings, including at the high/critical blocking threshold. They
remain release inputs and must be reevaluated whenever Next.js or the viewer
toolchain updates its transitive pins; do not remove them without a clean install,
audit, test, and production build. Maintainers own this temporary override set
and will review it by 2026-09-18 even if no aligned upstream release is available.
