#!/usr/bin/env bash
# BitRiver Live guided installer wizard.
#
# Collects interactive input and runs deploy/install/ubuntu.sh with the
# gathered configuration. Run this script from the repository root.

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
INSTALLER="$SCRIPT_DIR/ubuntu.sh"

if [[ ! -f go.mod ]]; then
        echo "This wizard must be run from the BitRiver Live repository root (go.mod not found)." >&2
        exit 1
fi

if [[ ! -x $INSTALLER ]]; then
        INSTALLER=(bash "$INSTALLER")
else
        INSTALLER=("$INSTALLER")
fi

if ! command -v go >/dev/null 2>&1; then
        echo "Go 1.21 or newer is required (go command not found)." >&2
        exit 1
fi

GO_VERSION_OUTPUT=$(go version)
if [[ $GO_VERSION_OUTPUT =~ go([0-9]+)\.([0-9]+)\.([0-9]+) ]]; then
        GO_MAJOR=${BASH_REMATCH[1]}
        GO_MINOR=${BASH_REMATCH[2]}
elif [[ $GO_VERSION_OUTPUT =~ go([0-9]+)\.([0-9]+) ]]; then
        GO_MAJOR=${BASH_REMATCH[1]}
        GO_MINOR=${BASH_REMATCH[2]}
else
        echo "Unable to determine Go version from: $GO_VERSION_OUTPUT" >&2
        exit 1
fi

if (( GO_MAJOR < 1 )) || { (( GO_MAJOR == 1 )) && (( GO_MINOR < 21 )); }; then
        echo "Go 1.21 or newer is required (found ${GO_VERSION_OUTPUT#*go})." >&2
        exit 1
fi

if command -v systemctl >/dev/null 2>&1; then
        if systemctl status bitriver-live.service >/dev/null 2>&1; then
                echo "Warning: bitriver-live.service already exists. The installer will overwrite the unit if you continue." >&2
        fi
fi

prompt_default() {
        local prompt=$1
        local default=$2
        local value
        read -r -p "$prompt [$default]: " value
        if [[ -z $value ]]; then
                printf '%s' "$default"
        else
                printf '%s' "$value"
        fi
}

prompt_optional() {
        local prompt=$1
        local value
        read -r -p "$prompt: " value
        printf '%s' "$value"
}

prompt_secret() {
        local prompt=$1
        local value
        read -r -s -p "$prompt: " value
        echo
        printf '%s' "$value"
}

prompt_yes_no() {
        local prompt=$1
        local default=$2
        local default_hint
        if [[ $default == "y" ]]; then
                default_hint="Y/n"
        else
                default_hint="y/N"
        fi
        while true; do
                read -r -p "$prompt [$default_hint]: " reply
                if [[ -z $reply ]]; then
                        reply=$default
                fi
                case ${reply,,} in
                y|yes)
                        return 0
                        ;;
                n|no)
                        return 1
                        ;;
                esac
                echo "Please answer yes or no."
        done
}

INSTALL_DIR=$(prompt_default "Install directory" "/opt/bitriver-live")
DATA_DIR=$(prompt_default "Data directory" "/var/lib/bitriver-live")
SERVICE_USER=$(prompt_default "Service user" "bitriver")
MODE=$(prompt_default "Mode (production/development)" "production")
ADDR_DEFAULT=":80"
if [[ ${MODE,,} == "development" ]]; then
        ADDR_DEFAULT=":8080"
fi
ADDR=$(prompt_default "HTTP listen address" "$ADDR_DEFAULT")
HOSTNAME_HINT=$(prompt_optional "Hostname viewers will use (optional)")

STORAGE_DRIVER=""
while [[ -z $STORAGE_DRIVER ]]; do
        candidate=$(prompt_default "Storage driver (json/postgres)" "json")
        candidate=${candidate,,}
        case $candidate in
        json|postgres)
                STORAGE_DRIVER=$candidate
                ;;
        *)
                echo "Please enter either 'json' or 'postgres'." >&2
                ;;
        esac
done

POSTGRES_DSN=""
SESSION_STORE_DSN=""
USE_POSTGRES_SESSION=false
SESSION_STORE_DRIVER=""
if [[ $STORAGE_DRIVER == "postgres" ]]; then
        while [[ -z $POSTGRES_DSN ]]; do
                POSTGRES_DSN=$(prompt_default "Postgres DSN" "postgres://bitriver:changeme@localhost:5432/bitriver?sslmode=require")
                if [[ -z $POSTGRES_DSN ]]; then
                        echo "Postgres DSN is required when selecting the postgres storage driver." >&2
                fi
        done
        if prompt_yes_no "Use Postgres for session storage" "y"; then
                USE_POSTGRES_SESSION=true
                SESSION_STORE_DRIVER="postgres"
                SESSION_STORE_DSN=$(prompt_optional "  Session store Postgres DSN (leave blank to reuse primary DSN)")
        else
                SESSION_STORE_DRIVER="memory"
        fi
fi

TLS_CERT=""
TLS_KEY=""
if prompt_yes_no "Configure TLS certificate paths for the API" "n"; then
        TLS_CERT=$(prompt_optional "  Path to TLS certificate")
        TLS_KEY=$(prompt_optional "  Path to TLS private key")
fi

RATE_GLOBAL_RPS=""
RATE_LOGIN_LIMIT=""
RATE_LOGIN_WINDOW=""
REDIS_ADDR=""
REDIS_PASSWORD=""
if prompt_yes_no "Configure API rate limiting now" "n"; then
        RATE_GLOBAL_RPS=$(prompt_optional "  Global requests-per-second limit (leave blank to skip)")
        RATE_LOGIN_LIMIT=$(prompt_optional "  Login attempts limit (leave blank to skip)")
        RATE_LOGIN_WINDOW=$(prompt_optional "  Login window duration, e.g. 1m (leave blank to skip)")
        REDIS_ADDR=$(prompt_optional "  Redis address for rate limiter (leave blank to skip)")
        REDIS_PASSWORD=$(prompt_optional "  Redis password (leave blank to skip)")
fi

ENABLE_LOGS=false
LOG_DIR=""
if prompt_yes_no "Redirect systemd logs to a file" "n"; then
        ENABLE_LOGS=true
        LOG_DIR=$(prompt_default "  Log directory" "$DATA_DIR/logs")
fi

ADMIN_EMAIL=""
ADMIN_PASSWORD=""
if prompt_yes_no "Seed an administrator account now" "y"; then
        ADMIN_EMAIL=$(prompt_default "  Admin email" "admin@example.com")
        while [[ -z $ADMIN_PASSWORD ]]; do
                ADMIN_PASSWORD=$(prompt_secret "  Temporary password (8+ characters)")
                if [[ ${#ADMIN_PASSWORD} -lt 8 ]]; then
                        echo "  Password must be at least 8 characters." >&2
                        ADMIN_PASSWORD=""
                fi
        done
fi

args=()
args+=("${INSTALLER[@]}")
args+=("--install-dir" "$INSTALL_DIR")
args+=("--data-dir" "$DATA_DIR")
args+=("--service-user" "$SERVICE_USER")
args+=("--mode" "$MODE")
args+=("--addr" "$ADDR")
args+=("--storage-driver" "$STORAGE_DRIVER")

if [[ -n $HOSTNAME_HINT ]]; then
        args+=("--hostname" "$HOSTNAME_HINT")
fi
if [[ -n $TLS_CERT ]]; then
        args+=("--tls-cert" "$TLS_CERT")
fi
if [[ -n $TLS_KEY ]]; then
        args+=("--tls-key" "$TLS_KEY")
fi
if [[ -n $RATE_GLOBAL_RPS ]]; then
        args+=("--rate-global-rps" "$RATE_GLOBAL_RPS")
fi
if [[ -n $RATE_LOGIN_LIMIT ]]; then
        args+=("--rate-login-limit" "$RATE_LOGIN_LIMIT")
fi
if [[ -n $RATE_LOGIN_WINDOW ]]; then
        args+=("--rate-login-window" "$RATE_LOGIN_WINDOW")
fi
if [[ -n $REDIS_ADDR ]]; then
        args+=("--redis-addr" "$REDIS_ADDR")
fi
if [[ -n $REDIS_PASSWORD ]]; then
        args+=("--redis-password" "$REDIS_PASSWORD")
fi
if [[ $ENABLE_LOGS == true ]]; then
        args+=("--enable-logs")
        if [[ -n $LOG_DIR ]]; then
                args+=("--log-dir" "$LOG_DIR")
        fi
fi
if [[ $STORAGE_DRIVER == "postgres" ]]; then
        args+=("--postgres-dsn" "$POSTGRES_DSN")
        if [[ $USE_POSTGRES_SESSION == true && -n $SESSION_STORE_DSN ]]; then
                args+=("--session-store-dsn" "$SESSION_STORE_DSN")
        fi
fi
if [[ -n $SESSION_STORE_DRIVER ]]; then
        args+=("--session-store" "$SESSION_STORE_DRIVER")
fi
if [[ -n $ADMIN_EMAIL && -n $ADMIN_PASSWORD ]]; then
        args+=("--bootstrap-admin-email" "$ADMIN_EMAIL")
        args+=("--bootstrap-admin-password" "$ADMIN_PASSWORD")
fi

cat <<EOF

The installer will run with the following options:
  Install directory: $INSTALL_DIR
  Data directory:    $DATA_DIR
  Service user:      $SERVICE_USER
  Mode:              $MODE
  Listen address:    $ADDR
  Storage driver:    $STORAGE_DRIVER
EOF

if [[ -n $HOSTNAME_HINT ]]; then
        echo "  Hostname hint:   $HOSTNAME_HINT"
fi
if [[ $STORAGE_DRIVER == "postgres" ]]; then
        echo "  Postgres DSN:    $POSTGRES_DSN"
fi
if [[ -n $SESSION_STORE_DRIVER ]]; then
        if [[ $SESSION_STORE_DRIVER == "postgres" ]]; then
                if [[ -n $SESSION_STORE_DSN ]]; then
                        echo "  Session store DSN: $SESSION_STORE_DSN"
                else
                        echo "  Session store:    postgres (reuse primary DSN)"
                fi
        else
                echo "  Session store:    $SESSION_STORE_DRIVER"
        fi
fi
if [[ -n $TLS_CERT || -n $TLS_KEY ]]; then
        echo "  TLS certificate: $TLS_CERT"
        echo "  TLS key:         $TLS_KEY"
fi
if [[ -n $RATE_GLOBAL_RPS || -n $RATE_LOGIN_LIMIT || -n $RATE_LOGIN_WINDOW || -n $REDIS_ADDR ]]; then
        echo "  Rate limiting:   enabled"
fi
if [[ $ENABLE_LOGS == true ]]; then
        echo "  Log directory:   $LOG_DIR"
fi
if [[ -n $ADMIN_EMAIL ]]; then
        echo "  Bootstrap admin: $ADMIN_EMAIL"
fi

cat <<'NOTE'

Note: deploy/install/ubuntu.sh uses sudo to create system users, directories, and
systemd units. You may be prompted for your password.
NOTE

echo "About to execute:"
printf '  %q' "${args[@]}"
echo -e "\n"

if prompt_yes_no "Proceed with installation" "y"; then
        "${args[@]}"
else
        echo "Installation aborted."
fi
