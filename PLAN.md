# PLAN

## Scope (current change)
- Promote monitoring assets into an official optional Compose overlay at `deploy/docker-compose.monitoring.yml` for Prometheus, Grafana, and Alertmanager.
- Add Grafana provisioning files so Prometheus datasource + bundled BitRiver dashboard auto-load on first start.
- Ensure `deploy/monitoring/prometheus.yml` scrapes BitRiver API metrics and monitoring dependencies included in the overlay.
- Add dedicated operator guide `docs/monitoring.md` covering quickstart, env/token setup, alert routing, expected healthy state, and troubleshooting.
- Extend monitoring CI sanity checks to validate the overlay/provisioning contract in addition to Prometheus/Alertmanager syntax.

## Assumptions
- Monitoring must remain optional and only enabled when the overlay file is supplied.
- Existing API `/metrics` endpoint already exists and should be scraped with bearer token from `deploy/monitoring/metrics.token`.
- Docker may be unavailable in some local/CI contexts, so checks should keep graceful fallbacks.

## Risks
- Missing Grafana provisioning paths/permissions can cause dashboard auto-import to fail silently.
- Prometheus target names and compose service names can drift, causing false-down targets.
- Alertmanager config rendering requirements can be confusing without explicit docs.

## Test plan
- `./scripts/check-monitoring-config.sh`
- `docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.monitoring.yml config`
- `./scripts/verify.sh`
