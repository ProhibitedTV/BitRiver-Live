# Cross-platform plan

This document inventories the platform-specific assumptions in BitRiver Live today and outlines how we will converge on a single cross-platform control plane without changing behaviour yet.

## Platform-specific assumptions in the current repo

- **Bash-first automation and Linux paths**
  - `scripts/quickstart.sh` requires Bash, Linux/macOS tooling, and defaults to `/var/lib/docker` when `docker info` cannot report a root directory on Linux hosts.
  - `deploy/check-env.sh` loads `.env` with Bash, enforces production secrets, and assumes the file lives at the repo root or a path passed as the first argument.
  - `deploy/install/ubuntu.sh` and the interactive `deploy/install/wizard.sh` assume sudo, systemd, and Linux file ownership semantics while staging binaries and data under paths such as `/opt/bitriver-live` and `/var/lib/bitriver-live`.
  - `scripts/render-ome-config.sh` is a Bash renderer for the OvenMediaEngine template and reuses `.env` defaults for Linux Compose deployments.
- **Windows-specific helper**
  - `scripts/quickstart.ps1` targets Windows PowerShell and Docker Desktop, with guidance to rerun from an elevated shell when the daemon requires it.
- **Systemd-managed services**
  - The unit files under `deploy/systemd/*.service` and their README assume Ubuntu/Debian hosts with systemd, sudo, and directories rooted at `/opt/bitriver-live`, `/opt/bitriver-viewer`, and related ingest dirs. They remain Linux-only helpers; the recommended default for all platforms is to run `go run ./cmd/bitriver compose up` so Docker Compose orchestrates the stack.
- **Ubuntu-focused installation docs**
  - `docs/installing-on-ubuntu.md` documents only Ubuntu Server 22.04+ with `apt`, `ufw`, `systemctl`, and hard-coded ports for RTMP/HLS/API endpoints.
- **Compose and repo layout expectations**
  - The root `README.md` and quickstart scripts expect `.env` at the repo root and `deploy/docker-compose.yml` as the default Compose file, reinforcing a Unix-style path layout even on Windows/WSL.

## Support tiers

- **Tier 1 (official):** Windows 10/11 with Docker Desktop, macOS with Docker Desktop, and Ubuntu/Debian with Docker Engine + Compose plugin.
- **Tier 2 (best effort):** Other Linux distributions (Fedora, Arch, Alpine, Amazon Linux, etc.) where Docker/Podman differences and init systems may diverge.

## Proposed control plane approach

Adopt a Go-based control plane CLI (`cmd/bitriver`) that mirrors the current behaviours while smoothing over platform differences:

- **Compose orchestration:** Wrap `deploy/docker-compose.yml` bring-up/teardown, Docker root discovery, and disk space checks without relying on Bash or PowerShell.
- **Config rendering and validation:** Port `scripts/render-ome-config.sh` and `deploy/check-env.sh` logic into Go so `.env` validation, template rendering, and secret generation work uniformly on Windows, macOS, and Linux.
- **Installer flows:** Replace `deploy/install/ubuntu.sh` and `deploy/install/wizard.sh` with CLI subcommands that stage binaries/configs under user-selected paths and emit OS-specific service definitions (systemd on Linux; launchd/Windows Service templates later).
- **Gradual deprecation of shell wrappers:** Keep `scripts/quickstart.sh` and `scripts/quickstart.ps1` as thin shims that call the Go CLI, preserving current entrypoints while steering users to the cross-platform binary.

You can exercise the initial CLI scaffold today with:

```
go run ./cmd/bitriver doctor
```

## Canonical production deployment path

- Production deployments continue to flow through `deploy/docker-compose.yml` with configuration sourced from the repo-root
  `.env` (generated from `deploy/.env.example`, validated by `deploy/check-env.sh`, and rendered into
  `deploy/ome/Server.generated.xml` via `scripts/render-ome-config.sh`).
- The Go control plane will wrap these same steps instead of introducing a second runbook. Any new subcommands must treat the
  Compose file and `.env` guardrails as source-of-truth inputs so the production pipeline stays identical on Windows, macOS,
  and Linux.

## Migration checklist and milestones

1. **Codify support tiers in docs:** Publish this plan and reference it from the root README to set expectations for Tier 1 vs. Tier 2 platforms.
2. **Scaffold the Go control plane:** Create `cmd/bitriver` with subcommands that replicate quickstart (Compose up/down, env generation) without changing defaults; reuse existing `cmd` packages where possible.
3. **Port env validation and template rendering:** Move `deploy/check-env.sh` and `scripts/render-ome-config.sh` logic into Go modules, keeping the shell scripts as delegates while the CLI stabilises.
4. **Add cross-platform installers:** Implement CLI subcommands to stage binaries, viewer assets, and ingest configs to configurable paths, then emit service definitions for systemd and placeholders for launchd/Windows Service.
5. **Document and deprecate:** Update `README.md`, `docs/quickstart.md`, and platform guides to recommend the Go CLI first, marking Bash/systemd helpers as legacy once feature parity is confirmed.
6. **Release artifacts:** Ship signed CLI binaries for Windows/macOS/Linux alongside Docker images so users can adopt the control plane without building from source.
