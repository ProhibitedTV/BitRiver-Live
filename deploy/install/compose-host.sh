#!/usr/bin/env bash
set -euo pipefail

program=bitriver-host
unit_name=bitriver-live-compose.service

usage() {
  cat <<'USAGE'
Usage: bitriver-host <command> [flags]

Commands:
  install      Stage a release bundle and disabled systemd unit (default)
  upgrade      Stage assets from a newer bundle and restart when already active
  configure    Run the guided production environment wizard
  activate     Validate, enable, and start the canonical Compose stack
  doctor       Run the host/resource/port prerequisite report
  status       Show systemd and Compose service status
  logs         Follow the systemd unit log
  uninstall    Remove program/service integration but retain config and data

Flags:
  --source-root DIR       Release asset root containing deploy/ and scripts/
  --binary-dir DIR        Directory containing bitriver and bitriver-live
  --operator-user USER    Non-root account that can access Docker
  --install-dir DIR       Program/runtime workspace (default /opt/bitriver-live)
  --config-dir DIR        Operator configuration (default /etc/bitriver-live)
  --data-dir DIR          Durable application/media data (default /var/lib/bitriver-live)
  --unit-dir DIR          Systemd unit directory (default /etc/systemd/system)
  --manager-path PATH     Installed lifecycle command (default /usr/local/sbin/bitriver-host)
  --root-prefix DIR       Prefix default system paths for package/lifecycle tests
  --purge-data            With uninstall, also select config/data for deletion
  --yes-really-purge      Required confirmation when --purge-data is selected
  -h, --help

Install never starts with sample credentials. After installation run:
  sudo bitriver-host configure
  sudo bitriver-host activate
USAGE
}

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

note() {
  printf '%s\n' "$*"
}

command_name=install
if [[ $# -gt 0 && $1 != -* ]]; then
  command_name=$1
  shift
fi

source_root=""
binary_dir=""
operator_user=${BITRIVER_OPERATOR_USER:-${SUDO_USER:-}}
install_dir=""
config_dir=""
data_dir=""
unit_dir=""
manager_path=""
root_prefix=""
purge_data=false
purge_confirmed=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source-root) [[ $# -ge 2 ]] || die "--source-root requires a value"; source_root=$2; shift 2 ;;
    --binary-dir) [[ $# -ge 2 ]] || die "--binary-dir requires a value"; binary_dir=$2; shift 2 ;;
    --operator-user) [[ $# -ge 2 ]] || die "--operator-user requires a value"; operator_user=$2; shift 2 ;;
    --install-dir) [[ $# -ge 2 ]] || die "--install-dir requires a value"; install_dir=$2; shift 2 ;;
    --config-dir) [[ $# -ge 2 ]] || die "--config-dir requires a value"; config_dir=$2; shift 2 ;;
    --data-dir) [[ $# -ge 2 ]] || die "--data-dir requires a value"; data_dir=$2; shift 2 ;;
    --unit-dir) [[ $# -ge 2 ]] || die "--unit-dir requires a value"; unit_dir=$2; shift 2 ;;
    --manager-path) [[ $# -ge 2 ]] || die "--manager-path requires a value"; manager_path=$2; shift 2 ;;
    --root-prefix) [[ $# -ge 2 ]] || die "--root-prefix requires a value"; root_prefix=$2; shift 2 ;;
    --purge-data) purge_data=true; shift ;;
    --yes-really-purge) purge_confirmed=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

prefix_path() {
  local absolute=$1
  if [[ -n $root_prefix ]]; then
    printf '%s%s' "${root_prefix%/}" "$absolute"
  else
    printf '%s' "$absolute"
  fi
}

install_dir=${install_dir:-$(prefix_path /opt/bitriver-live)}
config_dir=${config_dir:-$(prefix_path /etc/bitriver-live)}
data_dir=${data_dir:-$(prefix_path /var/lib/bitriver-live)}
unit_dir=${unit_dir:-$(prefix_path /etc/systemd/system)}
manager_path=${manager_path:-$(prefix_path /usr/local/sbin/bitriver-host)}
env_file="$config_dir/bitriver.env"
unit_path="$unit_dir/$unit_name"

validate_path() {
  local label=$1 value=$2
  [[ $value == /* ]] || die "$label must be an absolute path: $value"
  case "$value" in
    *$'\n'*|*$'\r'*|*\"*) die "$label contains unsupported control/quote characters" ;;
  esac
}

for pair in \
  "install-dir:$install_dir" \
  "config-dir:$config_dir" \
  "data-dir:$data_dir" \
  "unit-dir:$unit_dir" \
  "manager-path:$manager_path"; do
  validate_path "${pair%%:*}" "${pair#*:}"
done

validate_removal_dir() {
  local label=$1 value=$2
  case "${value%/}" in
    ""|/|/bin|/boot|/dev|/etc|/home|/opt|/proc|/root|/run|/sbin|/srv|/sys|/tmp|/usr|/usr/local|/var)
      die "$label is too broad for safe lifecycle removal: $value"
      ;;
  esac
}

validate_removal_dir install-dir "$install_dir"
validate_removal_dir config-dir "$config_dir"
validate_removal_dir data-dir "$data_dir"

script_dir=$(CDPATH=; cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)

find_source_root() {
  local candidate
  for candidate in \
    "$script_dir/../.." \
    "$script_dir/share/bitriver-live" \
    "$script_dir/../share/bitriver-live" \
    "/usr/local/share/bitriver-live"; do
    if [[ -f $candidate/deploy/docker-compose.yml && -f $candidate/deploy/install/release-assets.txt ]]; then
      (CDPATH=; cd -- "$candidate" && pwd -P)
      return 0
    fi
  done
  return 1
}

find_binary_dir() {
  local bundle_root candidate
  bundle_root=$(CDPATH=; cd -- "$source_root/../.." 2>/dev/null && pwd -P || true)
  for candidate in \
    "$bundle_root/bin" \
    "$source_root/bin" \
    "$source_root" \
    "/usr/local/bin"; do
    if [[ -x $candidate/bitriver && -x $candidate/bitriver-live ]]; then
      (CDPATH=; cd -- "$candidate" && pwd -P)
      return 0
    fi
  done
  return 1
}

resolve_sources() {
  if [[ -z $source_root ]]; then
    source_root=$(find_source_root) || die "cannot locate release assets; pass --source-root"
  fi
  source_root=$(CDPATH=; cd -- "$source_root" && pwd -P) || die "source root does not exist: $source_root"
  [[ -f $source_root/deploy/docker-compose.yml ]] || die "source root lacks deploy/docker-compose.yml: $source_root"
  [[ -x $source_root/scripts/stage-release-assets.sh ]] || die "source root lacks executable scripts/stage-release-assets.sh"

  if [[ -z $binary_dir ]]; then
    binary_dir=$(find_binary_dir) || die "cannot locate bitriver and bitriver-live; pass --binary-dir"
  fi
  binary_dir=$(CDPATH=; cd -- "$binary_dir" && pwd -P) || die "binary dir does not exist: $binary_dir"
  [[ -x $binary_dir/bitriver ]] || die "bitriver is not executable in $binary_dir"
  [[ -x $binary_dir/bitriver-live ]] || die "bitriver-live is not executable in $binary_dir"
}

require_root_for_system_paths() {
  if [[ -z $root_prefix && ${EUID:-$(id -u)} -ne 0 ]]; then
    die "system installation requires root; rerun with sudo (tests may use --root-prefix)"
  fi
}

resolve_operator() {
  if [[ -z $operator_user ]]; then
    if [[ -n $root_prefix ]]; then
      operator_user=$(id -un)
    else
      die "--operator-user is required when SUDO_USER is unavailable"
    fi
  fi
  id "$operator_user" >/dev/null 2>&1 || die "operator user does not exist: $operator_user"
  operator_group=$(id -gn "$operator_user")
  [[ $operator_user =~ ^[A-Za-z0-9_.-]+$ ]] || die "unsupported operator user name: $operator_user"
  [[ $operator_group =~ ^[A-Za-z0-9_.-]+$ ]] || die "unsupported operator group name: $operator_group"
  if [[ $operator_user == root && -z $root_prefix ]]; then
    note "WARNING: root is selected as the Docker operator; prefer a dedicated non-root account with Docker access."
  fi
}

run_as_operator() {
  if [[ ${EUID:-$(id -u)} -eq 0 && $operator_user != root ]]; then
    command -v sudo >/dev/null 2>&1 || die "sudo is required to run configuration as $operator_user"
    sudo -u "$operator_user" -- env BITRIVER_ROOT="$install_dir" BITRIVER_LAUNCHER_ROOT="$install_dir" BITRIVER_ENV_FILE="$env_file" BITRIVER_BINARY="$install_dir/bin/bitriver" "$@"
  else
    env BITRIVER_ROOT="$install_dir" BITRIVER_LAUNCHER_ROOT="$install_dir" BITRIVER_ENV_FILE="$env_file" BITRIVER_BINARY="$install_dir/bin/bitriver" "$@"
  fi
}

replace_empty_path_with_symlink() {
  local link_path=$1 target=$2
  if [[ -L $link_path ]]; then
    rm -f -- "$link_path"
  elif [[ -d $link_path ]]; then
    if find "$link_path" -mindepth 1 -print -quit | grep -q .; then
      die "refusing to replace non-empty runtime path: $link_path"
    fi
    rmdir -- "$link_path"
  elif [[ -e $link_path ]]; then
    die "refusing to replace unexpected runtime path: $link_path"
  fi
  ln -s "$target" "$link_path"
}

replace_file_with_symlink() {
  local link_path=$1 target=$2 seed=$3 mode=$4
  if [[ ! -e $target ]]; then
    install -D -m "$mode" "$seed" "$target"
  fi
  if [[ -L $link_path || -f $link_path ]]; then
    rm -f -- "$link_path"
  elif [[ -e $link_path ]]; then
    die "refusing to replace unexpected generated-config path: $link_path"
  fi
  ln -s "$target" "$link_path"
}

render_unit() {
  local template="$install_dir/deploy/systemd/bitriver-live-compose.service"
  local temporary
  temporary=$(mktemp)
  sed \
    -e "s|@BITRIVER_OPERATOR_USER@|$operator_user|g" \
    -e "s|@BITRIVER_OPERATOR_GROUP@|$operator_group|g" \
    -e "s|@BITRIVER_INSTALL_DIR@|$(printf '%s' "$install_dir" | sed 's/[&|\\]/\\&/g')|g" \
    -e "s|@BITRIVER_ENV_FILE@|$(printf '%s' "$env_file" | sed 's/[&|\\]/\\&/g')|g" \
    -e "s|@BITRIVER_CONFIG_DIR@|$(printf '%s' "$config_dir" | sed 's/[&|\\]/\\&/g')|g" \
    "$template" >"$temporary"
  install -D -m 0644 "$temporary" "$unit_path"
  rm -f -- "$temporary"
}

stage_install() {
  require_root_for_system_paths
  resolve_sources
  resolve_operator

  install -d -m 0755 "$install_dir" "$install_dir/bin" "$unit_dir" "$(dirname "$manager_path")"
  bash "$source_root/scripts/stage-release-assets.sh" --output "$install_dir"
  install -m 0755 "$binary_dir/bitriver" "$install_dir/bin/bitriver"
  install -m 0755 "$binary_dir/bitriver-live" "$install_dir/bin/bitriver-live"
  install -m 0755 "$source_root/deploy/install/compose-host.sh" "$manager_path"

  if [[ ${EUID:-$(id -u)} -eq 0 ]]; then
    install -d -m 0750 -o "$operator_user" -g "$operator_group" \
      "$config_dir" "$data_dir" "$data_dir/api" "$data_dir/transcoder"
  else
    [[ $operator_user == "$(id -un)" ]] || die "non-root --root-prefix tests must use the current operator user"
    install -d -m 0750 "$config_dir" "$data_dir" "$data_dir/api" "$data_dir/transcoder"
  fi

  replace_empty_path_with_symlink "$install_dir/deploy/data" "$data_dir/api"
  replace_empty_path_with_symlink "$install_dir/deploy/transcoder-data" "$data_dir/transcoder"
  replace_file_with_symlink \
    "$install_dir/deploy/ome/Server.generated.xml" \
    "$config_dir/Server.generated.xml" \
    "$source_root/deploy/ome/Server.xml" 0600
  replace_file_with_symlink \
    "$install_dir/deploy/srs/conf/srs.generated.conf" \
    "$config_dir/srs.generated.conf" \
    "$source_root/deploy/srs/conf/srs.conf" 0600

  if [[ ! -f $env_file ]]; then
    run_as_operator "$install_dir/bin/bitriver" env init \
      --env-file "$env_file" \
      --example "$install_dir/deploy/.env.example" </dev/null
  fi
  chmod 0600 "$env_file" "$config_dir/Server.generated.xml" "$config_dir/srs.generated.conf"
  if [[ ${EUID:-$(id -u)} -eq 0 ]]; then
    chown "$operator_user:$operator_group" "$env_file" "$config_dir/Server.generated.xml" "$config_dir/srs.generated.conf"
  fi

  if [[ -L $install_dir/.env || -f $install_dir/.env ]]; then
    rm -f -- "$install_dir/.env"
  elif [[ -e $install_dir/.env ]]; then
    die "refusing to replace unexpected env path: $install_dir/.env"
  fi
  ln -s "$env_file" "$install_dir/.env"

  render_unit
  if [[ -z $root_prefix ]] && command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload
  fi

  note "Installed BitRiver Live program assets in $install_dir"
  note "Preserved operator configuration in $config_dir"
  note "Preserved durable data in $data_dir"
  note "The service remains disabled until production configuration passes."
  note "Next: sudo $manager_path configure"
  note "Then: sudo $manager_path activate"
}

check_docker_operator() {
  command -v docker >/dev/null 2>&1 || die "docker is missing; install Docker Engine and the Compose v2 plugin"
  docker compose version >/dev/null 2>&1 || die "docker compose v2 is missing or unavailable"
  if [[ $operator_user != root ]] && ! id -nG "$operator_user" | tr ' ' '\n' | grep -qx docker; then
    die "$operator_user is not in the docker group; run 'sudo usermod -aG docker $operator_user', sign out/in, then retry"
  fi
}

configure_host() {
  require_root_for_system_paths
  resolve_operator
  [[ -x $install_dir/bin/bitriver ]] || die "BitRiver Live is not installed in $install_dir"
  run_as_operator "$install_dir/bin/bitriver" env init --wizard \
    --env-file "$env_file" \
    --example "$install_dir/deploy/.env.example"
  chmod 0600 "$env_file"
  note "Configuration updated. Run: sudo $manager_path activate"
}

doctor_host() {
  resolve_operator
  [[ -x $install_dir/bin/bitriver ]] || die "BitRiver Live is not installed in $install_dir"
  run_as_operator "$install_dir/bin/bitriver" doctor \
    --env-file "$env_file" \
    --compose-file "$install_dir/deploy/docker-compose.yml"
}

activation_diagnostics() {
  note "Activation failed. Safe status follows; credentials are not printed."
  systemctl status "$unit_name" --no-pager || true
  run_as_operator docker compose \
    --file "$install_dir/deploy/docker-compose.yml" \
    --env-file "$env_file" ps || true
  note "Inspect targeted logs with: sudo journalctl -u $unit_name -n 200 --no-pager"
  note "OME-specific Compose logs: sudo -u $operator_user docker compose --file '$install_dir/deploy/docker-compose.yml' --env-file '$env_file' logs --tail=120 ome ome-config ome-health-token-check bitriver-live"
  note "After correction retry: sudo $manager_path activate"
}

activate_host() {
  require_root_for_system_paths
  [[ -z $root_prefix ]] || die "activate is unavailable with --root-prefix"
  resolve_operator
  check_docker_operator
  run_as_operator "$install_dir/bin/bitriver" doctor \
    --env-file "$env_file" \
    --compose-file "$install_dir/deploy/docker-compose.yml"
  run_as_operator "$install_dir/bin/bitriver" env validate --env-file "$env_file"
  if ! systemctl enable --now "$unit_name"; then
    activation_diagnostics
    return 1
  fi
  systemctl status "$unit_name" --no-pager
  note "Activation completed only after bounded quickstart and critical-service health checks passed."
  note "Real ingest/playback acceptance remains required before production release approval."
}

status_host() {
  resolve_operator
  if [[ -z $root_prefix ]] && command -v systemctl >/dev/null 2>&1; then
    systemctl status "$unit_name" --no-pager || true
  fi
  run_as_operator docker compose \
    --file "$install_dir/deploy/docker-compose.yml" \
    --env-file "$env_file" ps
}

logs_host() {
  [[ -z $root_prefix ]] || die "logs is unavailable with --root-prefix"
  command -v journalctl >/dev/null 2>&1 || die "journalctl is unavailable"
  journalctl -u "$unit_name" -f
}

uninstall_host() {
  require_root_for_system_paths
  if [[ $purge_data == true && $purge_confirmed != true ]]; then
    die "--purge-data permanently deletes $config_dir and $data_dir; repeat with --yes-really-purge"
  fi

  if [[ -z $root_prefix ]] && command -v systemctl >/dev/null 2>&1; then
    systemctl disable --now "$unit_name" >/dev/null 2>&1 || true
  fi
  rm -f -- "$unit_path" "$manager_path"
  if [[ -d $install_dir ]]; then
    rm -rf -- "$install_dir"
  fi
  if [[ -z $root_prefix ]] && command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload
  fi

  if [[ $purge_data == true ]]; then
    note "WARNING: permanently deleting BitRiver Live configuration and data."
    rm -rf -- "$config_dir" "$data_dir"
    note "Program, configuration, and data removed."
  else
    note "Program/service integration removed."
    note "Configuration retained at $config_dir"
    note "Data retained at $data_dir"
  fi
}

case "$command_name" in
  install) stage_install ;;
  upgrade)
    was_active=false
    if [[ -z $root_prefix ]] && systemctl is-active --quiet "$unit_name"; then was_active=true; fi
    stage_install
    if [[ $was_active == true ]]; then
      systemctl restart "$unit_name" || { activation_diagnostics; exit 1; }
    fi
    ;;
  configure) configure_host ;;
  activate) activate_host ;;
  doctor) doctor_host ;;
  status) status_host ;;
  logs) logs_host ;;
  uninstall) uninstall_host ;;
  *) usage >&2; die "unknown command: $command_name" ;;
esac
