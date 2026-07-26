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
PROM_TOKEN="$TMP_DIR/metrics.token"
PROM_CONFIG_VALIDATION="$TMP_DIR/prometheus.yml"
PROM_RULES_DIR="$TMP_DIR/rules"
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

docker_host_path() {
  if command -v cygpath >/dev/null 2>&1; then
    cygpath -w "$1"
    return
  fi
  printf '%s\n' "$1"
}

validate_with_ruby_yaml "$GRAFANA_DS" "$GRAFANA_DASH_PROVISIONING" "$COMPOSE_MONITORING"

umask 077
printf 'monitoring-config-validation-only\n' >"$PROM_TOKEN"
mkdir -p "$PROM_RULES_DIR"
cp "$PROM_RULES" "$PROM_RULES_DIR/prometheus-alerts.yml"
sed \
  -e "s|/etc/prometheus/metrics.token|$PROM_TOKEN|g" \
  -e "s|/etc/prometheus/rules/\\*.yml|$PROM_RULES_DIR/*.yml|g" \
  "$PROM_CONFIG" >"$PROM_CONFIG_VALIDATION"

if command -v promtool >/dev/null 2>&1; then
  promtool check config "$PROM_CONFIG_VALIDATION"
  promtool check rules "$PROM_RULES"
elif command -v docker >/dev/null 2>&1; then
  PROM_CONFIG_MOUNT="$(docker_host_path "$PROM_CONFIG")"
  PROM_RULES_MOUNT="$(docker_host_path "$PROM_RULES")"
  PROM_TOKEN_MOUNT="$(docker_host_path "$PROM_TOKEN")"
  MSYS_NO_PATHCONV=1 docker run --rm \
    --entrypoint /bin/promtool \
    --mount "type=bind,source=$PROM_CONFIG_MOUNT,target=/etc/prometheus/prometheus.yml,readonly" \
    --mount "type=bind,source=$PROM_RULES_MOUNT,target=/etc/prometheus/rules/prometheus-alerts.yml,readonly" \
    --mount "type=bind,source=$PROM_TOKEN_MOUNT,target=/etc/prometheus/metrics.token,readonly" \
    prom/prometheus:v2.51.2 check config /etc/prometheus/prometheus.yml
  MSYS_NO_PATHCONV=1 docker run --rm \
    --entrypoint /bin/promtool \
    --mount "type=bind,source=$PROM_RULES_MOUNT,target=/etc/prometheus/prometheus-alerts.yml,readonly" \
    prom/prometheus:v2.51.2 check rules /etc/prometheus/prometheus-alerts.yml
else
  echo "warning: promtool/docker unavailable, falling back to YAML syntax validation" >&2
  validate_with_ruby_yaml "$PROM_CONFIG" "$PROM_RULES"
fi

"$RENDER_SCRIPT" --output "$ALERT_CONFIG"

if command -v amtool >/dev/null 2>&1; then
  amtool check-config "$ALERT_CONFIG"
elif command -v docker >/dev/null 2>&1; then
  ALERT_CONFIG_MOUNT="$(docker_host_path "$ALERT_CONFIG")"
  MSYS_NO_PATHCONV=1 docker run --rm \
    --entrypoint /bin/amtool \
    --mount "type=bind,source=$ALERT_CONFIG_MOUNT,target=/etc/alertmanager/alertmanager.yml,readonly" \
    prom/alertmanager:v0.27.0 check-config /etc/alertmanager/alertmanager.yml
else
  echo "warning: amtool/docker unavailable, falling back to YAML syntax validation" >&2
  validate_with_ruby_yaml "$ALERT_CONFIG"
fi

if command -v docker >/dev/null 2>&1; then
  docker compose --env-file "$ROOT_DIR/deploy/.env.example" \
    -f "$COMPOSE_BASE" \
    -f "$COMPOSE_MONITORING" \
    config >/dev/null
else
  echo "warning: docker unavailable, skipping compose overlay validation" >&2
fi

echo "monitoring config checks passed"
