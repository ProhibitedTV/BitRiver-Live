#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROM_CONFIG="$ROOT_DIR/deploy/monitoring/prometheus.yml"
PROM_RULES="$ROOT_DIR/deploy/monitoring/prometheus-alerts.yml"
ALERT_TEMPLATE="$ROOT_DIR/deploy/monitoring/alertmanager.yml.tmpl"
RENDER_SCRIPT="$ROOT_DIR/scripts/render-alertmanager-config.sh"
COMPOSE_BASE="$ROOT_DIR/deploy/docker-compose.yml"
COMPOSE_MONITORING="$ROOT_DIR/deploy/docker-compose.monitoring.yml"
GRAFANA_DS="$ROOT_DIR/deploy/monitoring/grafana/provisioning/datasources/prometheus.yml"
GRAFANA_DASH_PROVISIONING="$ROOT_DIR/deploy/monitoring/grafana/provisioning/dashboards/bitriver-live.yml"
GRAFANA_DASH_JSON="$ROOT_DIR/deploy/monitoring/bitriver-live-dashboard.json"
TMP_DIR="$(mktemp -d)"
ALERT_CONFIG="$TMP_DIR/alertmanager.yml"
trap 'rm -rf "$TMP_DIR"' EXIT

required_files=(
  "$PROM_CONFIG"
  "$PROM_RULES"
  "$ALERT_TEMPLATE"
  "$COMPOSE_MONITORING"
  "$GRAFANA_DS"
  "$GRAFANA_DASH_PROVISIONING"
  "$GRAFANA_DASH_JSON"
)

for f in "${required_files[@]}"; do
  if [[ ! -f "$f" ]]; then
    echo "error: required monitoring file missing: $f" >&2
    exit 1
  fi
done

validate_with_ruby_yaml() {
  ruby - "$@" <<'RUBY'
require 'yaml'
ARGV.each do |path|
  begin
    YAML.safe_load(File.read(path), aliases: true)
  rescue StandardError => e
    warn("yaml parse error in #{path}: #{e}")
    exit(1)
  end
end
RUBY
}

validate_with_ruby_yaml "$GRAFANA_DS" "$GRAFANA_DASH_PROVISIONING" "$COMPOSE_MONITORING"

if command -v promtool >/dev/null 2>&1; then
  promtool check config "$PROM_CONFIG"
  promtool check rules "$PROM_RULES"
elif command -v docker >/dev/null 2>&1; then
  docker run --rm -v "$ROOT_DIR/deploy/monitoring:/etc/prometheus:ro" \
    prom/prometheus:v2.51.2 promtool check config /etc/prometheus/prometheus.yml
  docker run --rm -v "$ROOT_DIR/deploy/monitoring:/etc/prometheus:ro" \
    prom/prometheus:v2.51.2 promtool check rules /etc/prometheus/prometheus-alerts.yml
else
  echo "warning: promtool/docker unavailable, falling back to YAML syntax validation" >&2
  validate_with_ruby_yaml "$PROM_CONFIG" "$PROM_RULES"
fi

"$RENDER_SCRIPT" --output "$ALERT_CONFIG"

if command -v amtool >/dev/null 2>&1; then
  amtool check-config "$ALERT_CONFIG"
elif command -v docker >/dev/null 2>&1; then
  docker run --rm -v "$TMP_DIR:/etc/alertmanager:ro" \
    prom/alertmanager:v0.27.0 amtool check-config /etc/alertmanager/alertmanager.yml
else
  echo "warning: amtool/docker unavailable, falling back to YAML syntax validation" >&2
  validate_with_ruby_yaml "$ALERT_CONFIG"
fi

if command -v docker >/dev/null 2>&1; then
  docker compose -f "$COMPOSE_BASE" -f "$COMPOSE_MONITORING" config >/dev/null
else
  echo "warning: docker unavailable, skipping compose overlay validation" >&2
fi

echo "monitoring config checks passed"
