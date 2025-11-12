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
#   --storage-driver       / BITRIVER_LIVE_STORAGE_DRIVER (default postgres)
#   --postgres-dsn         / BITRIVER_LIVE_POSTGRES_DSN
#   --session-store        / BITRIVER_LIVE_SESSION_STORE (default postgres when using Postgres storage)
#   --session-store-dsn    / BITRIVER_LIVE_SESSION_POSTGRES_DSN
#   --bootstrap-admin-email
#   --bootstrap-admin-password
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
  --addr LISTEN_ADDR (defaults to :80 in production / :8080 in development; privileged ports grant CAP_NET_BIND_SERVICE)
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
  --storage-driver DRIVER (default postgres)
  --postgres-dsn DSN
  --session-store DRIVER (defaults to postgres when the primary storage uses Postgres)
  --session-store-dsn DSN
  --bootstrap-admin-email EMAIL
  --bootstrap-admin-password PASSWORD
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

extract_listen_port() {
	local addr=$1
	addr=${addr#*://}
	if [[ $addr =~ ^\[[^]]*\]:(.+)$ ]]; then
		echo "${BASH_REMATCH[1]}"
		return 0
	fi
	if [[ $addr =~ ^:([0-9]+)$ ]]; then
		echo "${BASH_REMATCH[1]}"
		return 0
	fi
	if [[ $addr =~ ^.+:([0-9]+)$ ]]; then
		echo "${BASH_REMATCH[1]}"
		return 0
	fi
	echo ""
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
STORAGE_DRIVER=${BITRIVER_LIVE_STORAGE_DRIVER:-}
POSTGRES_DSN=${BITRIVER_LIVE_POSTGRES_DSN:-}
SESSION_STORE_DRIVER=${BITRIVER_LIVE_SESSION_STORE:-}
SESSION_STORE_DSN=${BITRIVER_LIVE_SESSION_POSTGRES_DSN:-}
BOOTSTRAP_ADMIN_EMAIL=${BOOTSTRAP_ADMIN_EMAIL:-}
BOOTSTRAP_ADMIN_PASSWORD=${BOOTSTRAP_ADMIN_PASSWORD:-}

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
        --storage-driver)
                require_arg "$@"
                STORAGE_DRIVER=$2
                shift 2
                ;;
        --postgres-dsn)
                require_arg "$@"
                POSTGRES_DSN=$2
                shift 2
                ;;
        --session-store)
                require_arg "$@"
                SESSION_STORE_DRIVER=$2
                shift 2
                ;;
        --session-store-dsn)
                require_arg "$@"
                SESSION_STORE_DSN=$2
                shift 2
                ;;
        --bootstrap-admin-email)
                require_arg "$@"
                BOOTSTRAP_ADMIN_EMAIL=$2
                shift 2
                ;;
        --bootstrap-admin-password)
                require_arg "$@"
                BOOTSTRAP_ADMIN_PASSWORD=$2
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

if [[ -n $BOOTSTRAP_ADMIN_EMAIL || -n $BOOTSTRAP_ADMIN_PASSWORD ]]; then
        if [[ -z $BOOTSTRAP_ADMIN_EMAIL || -z $BOOTSTRAP_ADMIN_PASSWORD ]]; then
                echo "--bootstrap-admin-email and --bootstrap-admin-password must be provided together" >&2
                exit 1
        fi
fi

STORAGE_DRIVER=${STORAGE_DRIVER,,}
if [[ -z $STORAGE_DRIVER ]]; then
        STORAGE_DRIVER="postgres"
fi

POSTGRES_DSN=${POSTGRES_DSN:-}
SESSION_STORE_DRIVER=${SESSION_STORE_DRIVER,,}
SESSION_STORE_DSN=${SESSION_STORE_DSN:-}

if [[ $STORAGE_DRIVER == "postgres" && -z $POSTGRES_DSN ]]; then
        echo "Postgres storage requires --postgres-dsn (BITRIVER_LIVE_POSTGRES_DSN) and a database migrated with deploy/migrations" >&2
        exit 1
fi

if [[ -z $SESSION_STORE_DRIVER ]]; then
        if [[ $STORAGE_DRIVER == "postgres" || -n $SESSION_STORE_DSN ]]; then
                SESSION_STORE_DRIVER="postgres"
        fi
fi

if [[ $SESSION_STORE_DRIVER == "postgres" ]]; then
        if [[ -z $SESSION_STORE_DSN ]]; then
                if [[ -n $POSTGRES_DSN ]]; then
                        SESSION_STORE_DSN=$POSTGRES_DSN
                else
                        echo "Postgres sessions require --session-store-dsn (BITRIVER_LIVE_SESSION_POSTGRES_DSN) or --postgres-dsn" >&2
                        exit 1
                fi
        fi
fi

if [[ -z $ADDR ]]; then
	if [[ $MODE == "production" ]]; then
		ADDR=":80"
	else
		ADDR=":8080"
	fi
fi

LISTEN_PORT=$(extract_listen_port "$ADDR")
REQUIRES_CAP_NET_BIND_SERVICE=false
if [[ -n $LISTEN_PORT && $LISTEN_PORT =~ ^[0-9]+$ && $LISTEN_PORT -lt 1024 ]]; then
	REQUIRES_CAP_NET_BIND_SERVICE=true
	if ! command -v setcap >/dev/null 2>&1; then
		echo "Binding to privileged port $LISTEN_PORT requires setcap (install libcap2-bin) or choose --addr :8080." >&2
		exit 1
	fi
fi

BUILD_FROM_SOURCE=false
if [[ -f go.mod ]]; then
        BUILD_FROM_SOURCE=true
elif [[ -x server && -x bootstrap-admin ]]; then
        BUILD_FROM_SOURCE=false
else
        echo "Provide either the BitRiver Live source tree (go.mod) or the prebuilt server/bootstrap-admin binaries in the current directory." >&2
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

if [[ $BUILD_FROM_SOURCE == true ]]; then
        if ! command -v go >/dev/null 2>&1; then
                echo "Go 1.21+ is required to build BitRiver Live" >&2
                exit 1
        fi

        GOFLAGS="-trimpath" go build -o bitriver-live ./cmd/server
        GOFLAGS="-trimpath" go build -o bootstrap-admin ./cmd/tools/bootstrap-admin
        sudo install -m 0755 bitriver-live "$INSTALL_DIR/bitriver-live"
        sudo install -m 0755 bootstrap-admin "$INSTALL_DIR/bootstrap-admin"
        rm -f bitriver-live bootstrap-admin
else
        sudo install -m 0755 server "$INSTALL_DIR/bitriver-live"
        sudo install -m 0755 bootstrap-admin "$INSTALL_DIR/bootstrap-admin"
fi
if [[ $REQUIRES_CAP_NET_BIND_SERVICE == true ]]; then
        sudo setcap 'cap_net_bind_service=+ep' "$INSTALL_DIR/bitriver-live"
fi

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
        echo "BITRIVER_LIVE_STORAGE_DRIVER=$STORAGE_DRIVER"
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
        if [[ -n $POSTGRES_DSN ]]; then
                echo "BITRIVER_LIVE_POSTGRES_DSN=$POSTGRES_DSN"
        fi
        if [[ -n $SESSION_STORE_DRIVER ]]; then
                echo "BITRIVER_LIVE_SESSION_STORE=$SESSION_STORE_DRIVER"
        fi
        if [[ -n $SESSION_STORE_DSN ]]; then
                echo "BITRIVER_LIVE_SESSION_POSTGRES_DSN=$SESSION_STORE_DSN"
        fi
} >"$env_file"

sudo install -m 0644 "$env_file" "$INSTALL_DIR/.env"

if [[ -n $BOOTSTRAP_ADMIN_EMAIL ]]; then
        echo "Bootstrapping administrator account..."
        if [[ $STORAGE_DRIVER == "postgres" ]]; then
                sudo -u "$SERVICE_USER" "$INSTALL_DIR/bootstrap-admin" \
                        --postgres-dsn "$POSTGRES_DSN" \
                        --email "$BOOTSTRAP_ADMIN_EMAIL" \
                        --password "$BOOTSTRAP_ADMIN_PASSWORD" \
                        --name "Administrator"
        else
                sudo -u "$SERVICE_USER" "$INSTALL_DIR/bootstrap-admin" \
                        --json "$DATA_FILE" \
                        --email "$BOOTSTRAP_ADMIN_EMAIL" \
                        --password "$BOOTSTRAP_ADMIN_PASSWORD" \
                        --name "Administrator"
        fi
fi

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
        if [[ $REQUIRES_CAP_NET_BIND_SERVICE == true ]]; then
                echo "AmbientCapabilities=CAP_NET_BIND_SERVICE"
                echo "CapabilityBoundingSet=CAP_NET_BIND_SERVICE"
                echo "NoNewPrivileges=yes"
        fi
        echo ""
        echo "[Install]"
        echo "WantedBy=multi-user.target"
} >"$service_file"

sudo install -m 0644 "$service_file" /etc/systemd/system/bitriver-live.service

sudo systemctl daemon-reload
sudo systemctl enable --now bitriver-live.service

if [[ $REQUIRES_CAP_NET_BIND_SERVICE == true ]]; then
        echo "Granted CAP_NET_BIND_SERVICE to bitriver-live.service and $INSTALL_DIR/bitriver-live for privileged port $LISTEN_PORT."
        echo "Use --addr :8080 (or another high port) when terminating traffic at a reverse proxy to avoid capability requirements."
fi

if [[ -n $HOSTNAME_HINT ]]; then
        echo "Reverse proxy hint: point $HOSTNAME_HINT to this service and expose TLS traffic on 443."
else
        if [[ $MODE == "production" ]]; then
                echo "Configure your reverse proxy or tailnet to expose the service. The API listens on $ADDR."
        else
                echo "Configure your reverse proxy or tailnet to expose the service. Development mode keeps the control center on :8080."
        fi
fi

echo "Service is running on $ADDR ($MODE mode). TLS settings and metrics are configured via $INSTALL_DIR/.env"
