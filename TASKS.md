# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Harden monitoring overlay compose contract
  - Acceptance criteria:
    - `deploy/docker-compose.monitoring.yml` defines Prometheus, Grafana, and Alertmanager as optional overlay services.
    - Overlay mounts monitoring configs/rules/dashboard/provisioning files needed for zero-touch startup.
    - Port exposure defaults are documented with reverse-proxy guidance.

- [x] Task 2 — Add Grafana provisioning + Prometheus scrape alignment
  - Acceptance criteria:
    - Grafana auto-provisions a Prometheus datasource and loads `deploy/monitoring/bitriver-live-dashboard.json`.
    - `deploy/monitoring/prometheus.yml` scrapes the BitRiver API metrics endpoint and overlay monitoring targets.

- [x] Task 3 — Add first-class monitoring docs
  - Acceptance criteria:
    - New `docs/monitoring.md` covers quickstart commands, required env/tokens, expected healthy signals, alert routing setup, and troubleshooting.

- [x] Task 4 — Extend CI sanity checks for monitoring overlay/provisioning
  - Acceptance criteria:
    - Monitoring config check script validates overlay/provisioning files in addition to Prometheus/Alertmanager syntax.

## Execution log
- ✅ Task 1 complete: rebuilt `deploy/docker-compose.monitoring.yml` into an explicit overlay for Prometheus, Alertmanager, and Grafana with loopback-bound default ports and provisioning/dashboard mounts.
- ⚠️ Task 1 check: `docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.monitoring.yml config` (failed in this environment: `docker` CLI not installed).

- ✅ Task 2 complete: added Grafana provisioning files under `deploy/monitoring/grafana/provisioning/**`; updated `deploy/monitoring/prometheus.yml` jobs for API/transcoder/srs-controller + alertmanager/prometheus.
- ✅ Task 2 check: `./scripts/check-monitoring-config.sh`.

- ✅ Task 3 complete: added `docs/monitoring.md` and aligned `docs/operations.md` quickstart text; updated `docs/contract.md` optional overlay contract + monitoring env knobs.
- ✅ Task 3 check: static docs inspection in diff review.

- ✅ Task 4 complete: extended `scripts/check-monitoring-config.sh` to assert monitoring overlay/provisioning files and to run compose overlay validation when Docker is available.
- ✅ Task 4 check: `./scripts/check-monitoring-config.sh`.

- ✅ Final gate check: `./scripts/verify.sh` (passed; Docker-dependent checks skipped because Docker CLI is unavailable in this environment).
