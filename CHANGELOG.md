# Changelog

All notable changes to BitRiver Live should be documented in this file.

The format is inspired by Keep a Changelog and the project follows the SemVer policy described in [`docs/versioning.md`](docs/versioning.md).

## [Unreleased]

### Changed

- Added a manual no-checkout Ubuntu 24.04 host-qualification workflow for
  signed public candidates, including package/systemd/Docker lifecycle,
  authenticated OME and restart recovery, retained-data uninstall, and
  sanitized evidence with explicit XOA/NPM limitations.
- Replaced pre-release-only onboarding with current RC13 download, Docker
  Desktop, Ubuntu package, first-stream, Nginx Proxy Manager, and recovery
  guidance backed by the published artifacts.
- Removed the unimplemented GitHub Pages viewer path and aligned public docs
  with the Node 24 / Next.js 16.2.11 release baseline.
- Cleaned up tracked temp files and release-evidence logs so the repository reads like a public project instead of an internal working directory.
- Added public collaboration and governance surfaces: `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, issue templates, and a PR template.
- Reworked the root README around an honest first-success path, explicit support boundaries, and a simpler architecture overview.
- Added `SUPPORT.md`, issue-template contact links, and clearer maintainer-priority guidance so newcomers can tell where to ask for help and what the project actively supports.
- Tightened the top of `docs/quickstart.md` so install paths, expected first success, and supported scope are clear before the advanced operator details begin.
- Corrected public GitHub repository URLs in packaging/docs metadata and clarified release-stage wording so the public release surface reads consistently.
- Fixed a stale quickstart link in `web/viewer/README.md`.
- Replaced dated internal release-check reports with a cleaner release-notes layout under `docs/releases/`.

## [v1.2.3-rc.13] - 2026-08-03

### Added

- A deterministic signed `bitriver.release-set/v1` root covering every public
  payload, five exact image digests and signature bundles, SBOMs, dependency
  pins, sanitized workflow evidence, and remaining external gates.
- A protected stable-promotion transaction that can copy one accepted
  candidate byte-for-byte without rebuilding it, plus signed rollback metadata
  and append-only candidate revocation.

### Changed

- Candidate tags are now the only build inputs. Stable tags and `latest` cannot
  be produced by the candidate workflow.
- Release consumers can verify one exact root before selecting packages or
  image digests; stable promotion refuses incomplete, cross-candidate, revoked,
  or mismatched evidence.

### Verified

- All 46 public assets were downloaded and matched their server-reported and
  published SHA-256 values; `CHECKSUMS.txt` covers the other 45 assets.
- Checksum-matched Cosign v3.1.2 verified the release-set root and five exact
  GHCR image signatures against the RC13 tag workflow identity.
- Anonymous pull-only deployment passed real 1080p RTMP, decoded OME and
  transcoder media, offline transition, chat/moderation, VOD, and aggregate
  health in release run `30795492882`.

See [`docs/releases/v1.2.3-rc.13.md`](docs/releases/v1.2.3-rc.13.md) for the
signed-root identity, install entry points, and remaining target-host gates.

## [v1.2.3-rc.12] - 2026-07-31

### Fixed

- Release packages and launcher archives now stamp the exact immutable
  candidate tag into all five first-party image defaults. RC11 packages named
  unpublished stable `v1.2.3` images and should not be used for a first-time
  package installation.

### Added

- Public checksum-covered binaries, launcher archives, Linux `.deb`/`.rpm`
  packages for amd64/arm64, a Windows MSI, a Homebrew formula, signatures, and
  software bills of materials.
- Anonymous multi-architecture images for the API, viewer, SRS controller,
  transcoder, and OME configuration helper under `ghcr.io/prohibitedtv`.

### Verified

- Package install/inspect/remove on Ubuntu 24.04, Debian 12, and Rocky Linux 9.
- Pull-only tagged deployment with bounded OME startup and an eight-stage gate
  covering RTMP, decoded live media, offline transition, chat/moderation, VOD,
  and aggregate status.
- A 33-asset public prerelease with 32 checksum entries covering every other
  asset and retained passed publication scans bound to commit `3a9572f0`.

See [`docs/releases/v1.2.3-rc.12.md`](docs/releases/v1.2.3-rc.12.md) for install
entry points, evidence boundaries, and known limits.

## [v1.2.3] - Planned public release

### Highlights

- Operator-managed single-host live-streaming stack with a Go control plane, Next.js viewer, SRS ingest, OvenMediaEngine playback, FFmpeg-based transcoding, Postgres, and Redis.
- Canonical deployment contract built around the repository-root `.env` and `deploy/docker-compose.yml`.
- Source-based quickstart, packaged launcher workflows, multi-platform release artifacts, and CI/release automation.
- Operator docs for quickstart, upgrades, security hardening, monitoring, and release execution.

### Notes

- This changelog entry is the current public-release draft and should be finalized when the tag is published.
- Release-specific operator steps should be captured in the matching note under `docs/releases/`.
