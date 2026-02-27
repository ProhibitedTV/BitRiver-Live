# Monitoring quickstart (Prometheus + Grafana + Alertmanager)

This guide turns BitRiver Live's bundled monitoring assets into an optional production-friendly Compose overlay.

> Monitoring is **optional**. The base stack still runs with only `deploy/docker-compose.yml`.

## What the overlay adds

`deploy/docker-compose.monitoring.yml` adds:

- **Prometheus** (`localhost:9090` by default)
- **Grafana** (`localhost:3001` by default)
- **Alertmanager** (`localhost:9093` by default)

All ports bind to `127.0.0.1` by default. For production, put Grafana/Prometheus/Alertmanager behind a reverse proxy with TLS + authentication instead of exposing them publicly.

## Prerequisites

1. Ensure the API metrics token exists in root `.env`:

```bash
BITRIVER_LIVE_METRICS_TOKEN=replace-with-strong-token
```

2. Copy the Prometheus token file and keep it in sync with `.env`:

```bash
cp deploy/monitoring/metrics.token.example deploy/monitoring/metrics.token
# then edit deploy/monitoring/metrics.token to match BITRIVER_LIVE_METRICS_TOKEN
```

3. Render Alertmanager config from template:

```bash
cp deploy/monitoring/alertmanager.env.example deploy/monitoring/alertmanager.env
./scripts/render-alertmanager-config.sh
```

## Start the stack with monitoring

```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.monitoring.yml up -d
```

## Validate everything is healthy

1. **Prometheus targets** (`http://localhost:9090/targets`) should show UP for:
   - `bitriver-api` (`bitriver-live:8080`)
   - `bitriver-transcoder` (`transcoder:9000`)
   - `bitriver-srs-controller` (`srs-controller:1985`)
   - `alertmanager`
2. **Grafana** (`http://localhost:3001`) should already include folder **BitRiver Live** with dashboard **BitRiver Live** preloaded from `deploy/monitoring/bitriver-live-dashboard.json`.
3. **Alertmanager** (`http://localhost:9093`) should show active routes/receivers from `deploy/monitoring/alertmanager.yml`.

## Default Grafana + Prometheus provisioning

The overlay mounts provisioning files from `deploy/monitoring/grafana/provisioning/`:

- `datasources/prometheus.yml` provisions default datasource **BitRiver Prometheus** (`http://prometheus:9090`).
- `dashboards/bitriver-live.yml` provisions dashboards from `/var/lib/grafana/dashboards`.
- `deploy/monitoring/bitriver-live-dashboard.json` is mounted into that dashboards path and auto-imported on first start.

## Alerting setup notes

Prometheus rules are loaded from `deploy/monitoring/prometheus-alerts.yml`.

Alertmanager routing is template-driven:

- edit `deploy/monitoring/alertmanager.env` with your endpoints/tokens
- rerun `./scripts/render-alertmanager-config.sh`
- restart Alertmanager container

Placeholders you should set:

- default webhook URL/token
- critical webhook URL/token
- auth webhook URL/token

You can map these to Slack/email/webhook adapters behind your preferred notification gateway.

## Port and reverse-proxy recommendations

You can tune host bindings in `.env`:

- `BITRIVER_PROMETHEUS_BIND`, `BITRIVER_PROMETHEUS_HOST_PORT`
- `BITRIVER_GRAFANA_BIND`, `BITRIVER_GRAFANA_HOST_PORT`
- `BITRIVER_ALERTMANAGER_BIND`, `BITRIVER_ALERTMANAGER_HOST_PORT`

Recommended production pattern:

1. keep Compose bindings on loopback
2. publish only via reverse proxy
3. enforce auth + TLS at the proxy
4. restrict upstream access to monitoring hosts/operators only

## Troubleshooting

- **Prometheus 403 scraping `bitriver-api`:** token mismatch between `.env` and `deploy/monitoring/metrics.token`.
- **Grafana dashboard missing:** verify provisioning mounts exist and check `docker compose logs grafana` for provisioning errors.
- **Alertmanager fails to start:** rerun `./scripts/render-alertmanager-config.sh` and validate with `./scripts/check-monitoring-config.sh`.
- **Monitoring config regressions:** run

```bash
./scripts/check-monitoring-config.sh
```

- **Compose merge issues:** run

```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.monitoring.yml config
```
