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
- Next.js: 16.x Active LTS with React 19.3.x. Security and patch updates are reviewed within seven days; major upgrades require the full viewer build and Playwright gates.
- Lockfiles and checksums are release inputs. Dependency PRs must include the resolved diff, audit result, and applicable runtime tests.

Dependabot checks GitHub Actions, Go modules, and viewer npm dependencies weekly. Minor and patch updates are grouped by runtime area; major updates remain isolated for explicit migration review.

Viewer lock updates must retain optional WebAssembly dependency entries used by
Sharp and the native resolver fallback, even when they are not installed on the
maintainer's host. Windows npm can accept a graph that Alpine's newer npm rejects
as incomplete. If that happens, regenerate the lock with the existing Node 24
Alpine builder and an empty dependency directory, then prove clean installs on
both Windows and Alpine. The structural WASM lock test catches missing entries;
it does not replace `npm ci`, version resolution, audit, or image qualification.

The October 2026 tooling refresh pins Node types 26.6.4 and ts-jest 29.4.14.
Node type package versions do not change the supported Node 24 runtime.
The ts-jest patch reverts compiler utility changes, so qualification must run
the transformed Jest suite as well as lint, the production build and browser
matrix. See the [upstream changelog](https://github.com/kulshekhar/ts-jest/blob/v29.4.14/CHANGELOG.md).

## Vulnerability gates and exceptions

`govulncheck` reachable findings and npm high/critical findings block CI and release. Critical findings cannot receive a release exception. A temporary high-severity exception requires all of the following in a dedicated security PR:

- advisory ID and affected package/module,
- reachability and deployment impact analysis,
- named owner and tracking issue,
- mitigation or compensating control,
- ISO expiry date no more than 30 days out.

Go exceptions live in `scripts/govulncheck-baseline.json`; the scanner rejects incomplete or expired entries. npm remains fail-closed and has no exception file. If an npm high finding must be accepted, maintainers must first add an equivalently validated, package-and-advisory-specific policy mechanism in the same reviewed security PR. Never use `continue-on-error`, lower the audit threshold, or run `npm audit fix --force` to bypass the gate.

## Current security mitigations

The October 2026 viewer qualification adopts Next.js/ESLint config 16.3.8 and
brace-expansion 5.0.12 to address newly reported critical/high findings. The
existing CommonJS adapter remains necessary for legacy minimatch consumers.
Browserslist 4.28.9, PostCSS 8.5.28, and Sharp 0.35.5 overrides remain release
inputs; reevaluate their resolved pins whenever the viewer toolchain changes.

The October 7 refresh adopts Sharp 0.35.5 for the upstream librsvg fix
([GHSA-wq5f-xc86-pv6w](https://github.com/lovell/sharp/security/advisories/GHSA-wq5f-xc86-pv6w))
and pins source-map-js 1.2.2 for indexed source-map offset denial of service
([GHSA-68fv-2mgg-jv7q](https://github.com/advisories/GHSA-68fv-2mgg-jv7q)).
The high/critical audit gate remains unchanged. Remaining moderate findings in
the Jest tooling chain must be reported, not described as a clean audit.
These source patches do not change already published RC23 bytes; that candidate
cannot be approved for stable promotion using the refreshed source audit.

The unpatched `braces@3.0.3` nesting advisory GHSA-vfj7-8cjw-p6xm affects the
ESLint-only `fast-glob -> micromatch` dependency chain. A private MIT-licensed
fork in `web/viewer/vendor/braces` bounds syntax nesting and validates external
ASTs before recursive walkers. This is a real code mitigation, not an audit
exception. Package-name substitution means a clean audit is not independent
proof of safety: review the source and attack/compatibility tests too.

BitRiver Live maintainers own the fork and will review removal by 2026-10-16.
See its `UPSTREAM.md` for source provenance, limits, and removal procedure.
Do not remove dependency overrides without a clean install, audit, tests,
production build, and Docker qualification. No vulnerability gate is weakened.
