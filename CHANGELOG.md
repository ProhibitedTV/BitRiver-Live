# Changelog

All notable changes to BitRiver Live should be documented in this file.

The format is inspired by Keep a Changelog and the project follows the SemVer policy described in [`docs/versioning.md`](docs/versioning.md).

## [Unreleased]

### Changed

- Cleaned up tracked temp files and release-evidence logs so the repository reads like a public project instead of an internal working directory.
- Added public collaboration and governance surfaces: `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, issue templates, and a PR template.
- Reworked the root README around an honest first-success path, explicit support boundaries, and a simpler architecture overview.
- Added `SUPPORT.md`, issue-template contact links, and clearer maintainer-priority guidance so newcomers can tell where to ask for help and what the project actively supports.
- Tightened the top of `docs/quickstart.md` so install paths, expected first success, and supported scope are clear before the advanced operator details begin.
- Corrected public GitHub repository URLs in packaging/docs metadata and clarified release-stage wording so the public release surface reads consistently.
- Fixed a stale quickstart link in `web/viewer/README.md`.
- Replaced dated internal release-check reports with a cleaner release-notes layout under `docs/releases/`.

## [v1.2.3] - Planned public release

### Highlights

- Operator-managed single-host live-streaming stack with a Go control plane, Next.js viewer, SRS ingest, OvenMediaEngine playback, FFmpeg-based transcoding, Postgres, and Redis.
- Canonical deployment contract built around the repository-root `.env` and `deploy/docker-compose.yml`.
- Source-based quickstart, packaged launcher workflows, multi-platform release artifacts, and CI/release automation.
- Operator docs for quickstart, upgrades, security hardening, monitoring, and release execution.

### Notes

- This changelog entry is the current public-release draft and should be finalized when the tag is published.
- Release-specific operator steps should be captured in the matching note under `docs/releases/`.
