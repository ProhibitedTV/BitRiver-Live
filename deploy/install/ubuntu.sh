#!/usr/bin/env bash
# BitRiver Live Ubuntu installation helper.
#
# This script prepares a systemd-managed deployment of the BitRiver Live control center.
# Run it from the repository root on a host with Go 1.21+ and systemd available.
#
# Required inputs can be provided either as flags or environment variables:
#   --install-dir  / INSTALL_DIR        Absolute path for the BitRiver Live binaries.
#   --data-dir     / DATA_DIR           Directory for persistent data (store.json, logs, etc.).
#   --service-user / SERVICE_USER       System user that should own the files and run the service.
#   --addr         / BITRIVER_LIVE_ADDR Listen address for the HTTP server (default depends on mode).
#   --tls-cert     / BITRIVER_LIVE_TLS_CERT
#   --tls-key      / BITRIVER_LIVE_TLS_KEY
# Optional parameters can be supplied via flags or their matching environment variables:
#   --mode                 / BITRIVER_LIVE_MODE              (production|development, default production)
#   --enable-logs                                           (enable systemd log redirection)
#   --log-dir              / BITRIVER_LIVE_LOG_DIR           (defaults to DATA_DIR/logs when enabled)
#   --rate-global-rps      / BITRIVER_LIVE_RATE_GLOBAL_RPS
#   --rate-login-limit     / BITRIVER_LIVE_RATE_LOGIN_LIMIT
#   --rate-login-window    / BITRIVER_LIVE_RATE_LOGIN_WINDOW
#   --redis-addr           / BITRIVER_LIVE_RATE_REDIS_ADDR
#   --redis-password       / BITRIVER_LIVE_RATE_REDIS_PASSWORD
#   --hostname             / BITRIVER_LIVE_HOSTNAME_HINT     (used for the informational hint at the end)
#
# Example:
#   ./deploy/install/ubuntu.sh \
#     --install-dir /opt/bitriver-live \
#     --data-dir /var/lib/bitriver-live \
#     --service-user bitriver \
#     --mode production \
#     --addr :80 \
#     --enable-logs
#
# The script assumes sudo privileges are available for creating users, directories and systemd units.

set -euo pipefail

usage() {
        cat <<'USAGE'
Usage: ubuntu.sh [flags]

Run from the repository root after cloning BitRiver Live.

Required flags (or environment variables):
  --install-dir / INSTALL_DIR
  --data-dir / DATA_DIR
  --service-user / SERVICE_USER

Optional flags:
  --mode (production|development)
  --addr LISTEN_ADDR
  --enable-logs
  --log-dir LOG_DIR
  --tls-cert CERT_PATH
  --tls-key KEY_PATH
  --rate-global-rps VALUE
  --rate-login-limit VALUE
  --rate-login-window VALUE
  --redis-addr ADDRESS
  --redis-password PASSWORD
  --hostname HOSTNAME
  -h, --help
USAGE
}

require_arg() {
        if [[ $# -lt 2 ]]; then
                echo "missing value for $1" >&2
                usage
                exit 1
        fi
}

INSTALL_DIR=${INSTALL_DIR:-}
DATA_DIR=${DATA_DIR:-}
SERVICE_USER=${SERVICE_USER:-}
MODE=${BITRIVER_LIVE_MODE:-production}
ADDR=${BITRIVER_LIVE_ADDR:-}
ENABLE_LOGS=${BITRIVER_LIVE_ENABLE_LOGS:-false}
LOG_DIR=${BITRIVER_LIVE_LOG_DIR:-}
TLS_CERT=${BITRIVER_LIVE_TLS_CERT:-}
TLS_KEY=${BITRIVER_LIVE_TLS_KEY:-}
RATE_GLOBAL_RPS=${BITRIVER_LIVE_RATE_GLOBAL_RPS:-}
RATE_LOGIN_LIMIT=${BITRIVER_LIVE_RATE_LOGIN_LIMIT:-}
RATE_LOGIN_WINDOW=${BITRIVER_LIVE_RATE_LOGIN_WINDOW:-}
REDIS_ADDR=${BITRIVER_LIVE_RATE_REDIS_ADDR:-}
REDIS_PASSWORD=${BITRIVER_LIVE_RATE_REDIS_PASSWORD:-}
HOSTNAME_HINT=${BITRIVER_LIVE_HOSTNAME_HINT:-}

while [[ $# -gt 0 ]]; do
        case "$1" in
        --install-dir)
                require_arg "$@"
                INSTALL_DIR=$2
                shift 2
                ;;
        --data-dir)
                require_arg "$@"
                DATA_DIR=$2
                shift 2
                ;;
        --service-user)
                require_arg "$@"
                SERVICE_USER=$2
                shift 2
                ;;
        --mode)
                require_arg "$@"
                MODE=$2
                shift 2
                ;;
        --addr)
                require_arg "$@"
                ADDR=$2
                shift 2
                ;;
        --enable-logs)
                ENABLE_LOGS=true
                shift
                ;;
        --log-dir)
                require_arg "$@"
                LOG_DIR=$2
                shift 2
                ;;
        --tls-cert)
                require_arg "$@"
                TLS_CERT=$2
                shift 2
                ;;
        --tls-key)
                require_arg "$@"
                TLS_KEY=$2
                shift 2
                ;;
        --rate-global-rps)
                require_arg "$@"
                RATE_GLOBAL_RPS=$2
                shift 2
                ;;
        --rate-login-limit)
                require_arg "$@"
                RATE_LOGIN_LIMIT=$2
                shift 2
                ;;
        --rate-login-window)
                require_arg "$@"
                RATE_LOGIN_WINDOW=$2
                shift 2
                ;;
        --redis-addr)
                require_arg "$@"
                REDIS_ADDR=$2
                shift 2
                ;;
        --redis-password)
                require_arg "$@"
                REDIS_PASSWORD=$2
                shift 2
                ;;
        --hostname)
                require_arg "$@"
                HOSTNAME_HINT=$2
                shift 2
                ;;
        -h|--help)
                usage
                exit 0
                ;;
        *)
                echo "unknown flag: $1" >&2
                usage
                exit 1
                ;;
        esac
done

if [[ -z $INSTALL_DIR ]]; then
        echo "--install-dir (or INSTALL_DIR) is required" >&2
        exit 1
fi
if [[ -z $DATA_DIR ]]; then
        echo "--data-dir (or DATA_DIR) is required" >&2
        exit 1
fi
if [[ -z $SERVICE_USER ]]; then
        echo "--service-user (or SERVICE_USER) is required" >&2
        exit 1
fi

if [[ -z $ADDR ]]; then
        if [[ $MODE == "production" ]]; then
                ADDR=":80"
        else
                ADDR=":8080"
        fi
fi

if [[ ! -f go.mod ]]; then
        echo "This script must be run from the BitRiver Live repository root (go.mod not found)." >&2
        exit 1
fi

DATA_FILE="$DATA_DIR/store.json"
if [[ ${ENABLE_LOGS,,} == "true" || $ENABLE_LOGS == "1" ]]; then
        if [[ -z $LOG_DIR ]]; then
                LOG_DIR="$DATA_DIR/logs"
        fi
else
        LOG_DIR=""
fi

if ! id -u "$SERVICE_USER" >/dev/null 2>&1; then
        sudo useradd --system --create-home --shell /usr/sbin/nologin "$SERVICE_USER"
fi

sudo install -d -o "$SERVICE_USER" -g "$SERVICE_USER" "$INSTALL_DIR" "$DATA_DIR"
if [[ -n $LOG_DIR ]]; then
        sudo install -d -o "$SERVICE_USER" -g "$SERVICE_USER" "$LOG_DIR"
fi

if ! command -v go >/dev/null 2>&1; then
        echo "Go 1.21+ is required to build BitRiver Live" >&2
        exit 1
fi

GOFLAGS="-trimpath" go build -o bitriver-live ./cmd/server
sudo install -m 0755 bitriver-live "$INSTALL_DIR/bitriver-live"
rm -f bitriver-live

env_file=$(mktemp)
service_file=$(mktemp)
cleanup() {
        rm -f "$env_file" "$service_file"
}
trap cleanup EXIT
{
        echo "BITRIVER_LIVE_ADDR=$ADDR"
        echo "BITRIVER_LIVE_MODE=$MODE"
        echo "BITRIVER_LIVE_DATA=$DATA_FILE"
        if [[ -n $TLS_CERT ]]; then
                echo "BITRIVER_LIVE_TLS_CERT=$TLS_CERT"
        fi
        if [[ -n $TLS_KEY ]]; then
                echo "BITRIVER_LIVE_TLS_KEY=$TLS_KEY"
        fi
        if [[ -n $RATE_GLOBAL_RPS ]]; then
                echo "BITRIVER_LIVE_RATE_GLOBAL_RPS=$RATE_GLOBAL_RPS"
        fi
        if [[ -n $RATE_LOGIN_LIMIT ]]; then
                echo "BITRIVER_LIVE_RATE_LOGIN_LIMIT=$RATE_LOGIN_LIMIT"
        fi
        if [[ -n $RATE_LOGIN_WINDOW ]]; then
                echo "BITRIVER_LIVE_RATE_LOGIN_WINDOW=$RATE_LOGIN_WINDOW"
        fi
        if [[ -n $REDIS_ADDR ]]; then
                echo "BITRIVER_LIVE_RATE_REDIS_ADDR=$REDIS_ADDR"
        fi
        if [[ -n $REDIS_PASSWORD ]]; then
                echo "BITRIVER_LIVE_RATE_REDIS_PASSWORD=$REDIS_PASSWORD"
        fi
} >"$env_file"

sudo install -m 0644 "$env_file" "$INSTALL_DIR/.env"
sudo chown -R "$SERVICE_USER":"$SERVICE_USER" "$INSTALL_DIR" "$DATA_DIR"
{
        echo "[Unit]"
        echo "Description=BitRiver Live Streaming Control Center"
        echo "After=network.target"
        echo ""
        echo "[Service]"
        echo "Type=simple"
        echo "User=$SERVICE_USER"
        echo "EnvironmentFile=$INSTALL_DIR/.env"
        echo "WorkingDirectory=$INSTALL_DIR"
        echo "ExecStart=$INSTALL_DIR/bitriver-live"
        echo "Restart=on-failure"
        if [[ -n $LOG_DIR ]]; then
                echo "StandardOutput=append:$LOG_DIR/server.log"
                echo "StandardError=append:$LOG_DIR/server.log"
        fi
        echo ""
        echo "[Install]"
        echo "WantedBy=multi-user.target"
} >"$service_file"

sudo install -m 0644 "$service_file" /etc/systemd/system/bitriver-live.service

sudo systemctl daemon-reload
sudo systemctl enable --now bitriver-live.service

if [[ -n $HOSTNAME_HINT ]]; then
        echo "Reverse proxy hint: point $HOSTNAME_HINT to this service and expose TLS traffic on 443."
else
        if [[ $MODE == "production" ]]; then
                echo "Configure your reverse proxy or tailnet to expose the service. Port 80 is used by default."
        else
                echo "Configure your reverse proxy or tailnet to expose the service. Development mode keeps the control center on :8080."
        fi
fi

echo "Service is running on $ADDR ($MODE mode). TLS settings and metrics are configured via $INSTALL_DIR/.env"
