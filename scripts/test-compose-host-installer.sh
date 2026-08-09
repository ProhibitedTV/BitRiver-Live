#!/usr/bin/env bash
set -euo pipefail

repo_root=$(CDPATH=; cd -- "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
tmp_root=$(mktemp -d "${TMPDIR:-/tmp}/bitriver-host-installer.XXXXXX")
cleanup() {
  rm -rf -- "$tmp_root"
}
trap cleanup EXIT

source_root="$tmp_root/source bundle/share/bitriver-live"
binary_dir="$tmp_root/source bundle/bin"
host_root="$tmp_root/host root"

mkdir -p "$source_root" "$binary_dir"
bash "$repo_root/scripts/stage-release-assets.sh" --output "$source_root"

cat >"$binary_dir/bitriver" <<'FAKE_BITRIVER'
#!/usr/bin/env bash
set -euo pipefail
if [[ ${1:-} == env && ${2:-} == init ]]; then
  shift 2
  env_file=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --env-file) env_file=$2; shift 2 ;;
      --example) shift 2 ;;
      *) shift ;;
    esac
  done
  [[ -n $env_file ]]
  test "${BITRIVER_CONFIG_ROOT:?}" = "$(dirname "$env_file")"
  cat >"$env_file" <<'SAFE_ENV'
BITRIVER_DEPLOY_IMAGE_SOURCE=pull
BITRIVER_CONFIG_ROOT=..
BITRIVER_LIVE_IMAGE_TAG=test
BITRIVER_VIEWER_IMAGE_TAG=test
BITRIVER_SRS_CONTROLLER_IMAGE_TAG=test
BITRIVER_TRANSCODER_IMAGE_TAG=test
BITRIVER_OME_CONFIG_IMAGE_TAG=test
BITRIVER_POSTGRES_PASSWORD=generated-postgres-secret
BITRIVER_REDIS_PASSWORD=generated-redis-secret
BITRIVER_OME_PASSWORD=generated-ome-secret
BITRIVER_OME_API_TOKEN=generated-ome-token
SAFE_ENV
  chmod 0600 "$env_file"
  exit 0
fi
if [[ ${1:-} == doctor ]]; then
  exit 0
fi
if [[ ${1:-} == quickstart ]]; then
  test -f "${BITRIVER_ENV_FILE:?}"
  test "${BITRIVER_CONFIG_ROOT:?}" = "$(dirname "$BITRIVER_ENV_FILE")"
  exit 0
fi
echo "unexpected fake bitriver invocation: $*" >&2
exit 1
FAKE_BITRIVER
cp "$repo_root/scripts/bitriver-live-wrapper.sh" "$binary_dir/bitriver-live"
chmod +x \
  "$binary_dir/bitriver" \
  "$binary_dir/bitriver-live" \
  "$source_root/scripts/stage-release-assets.sh" \
  "$source_root/deploy/install/compose-host.sh"

installer="$source_root/deploy/install/compose-host.sh"
operator_user=$(id -un)
common_args=(
  --source-root "$source_root"
  --binary-dir "$binary_dir"
  --root-prefix "$host_root"
  --operator-user "$operator_user"
)

"$installer" install "${common_args[@]}"

install_root="$host_root/opt/bitriver-live"
config_root="$host_root/etc/bitriver-live"
data_root="$host_root/var/lib/bitriver-live"
unit_file="$host_root/etc/systemd/system/bitriver-live-compose.service"

test -x "$install_root/bin/bitriver"
test -x "$install_root/bin/bitriver-live"
test -L "$install_root/.env"
test -L "$install_root/deploy/data"
test -L "$install_root/deploy/transcoder-data"
test -L "$install_root/deploy/ome/Server.generated.xml"
test -L "$install_root/deploy/srs/conf/srs.generated.conf"
test -f "$config_root/bitriver.env"
test -f "$unit_file"

if grep -q '@BITRIVER_' "$unit_file"; then
  echo "systemd unit still contains template placeholders" >&2
  exit 1
fi
if ! grep -Fxq "WorkingDirectory=$install_root" "$unit_file"; then
  echo "systemd unit does not contain an absolute unquoted working directory" >&2
  exit 1
fi
if ! grep -Fxq "Environment=\"BITRIVER_CONFIG_ROOT=$config_root\"" "$unit_file"; then
  echo "systemd unit does not expose the absolute operator config root to Compose" >&2
  exit 1
fi
if grep -Eq 'P0stgres-Example!|R3dis-Example!|OME-Example-(Pass|Access-Token)' "$config_root/bitriver.env"; then
  echo "installed environment retains release sample credentials" >&2
  exit 1
fi
if [[ $(grep -c '^BITRIVER_CONFIG_ROOT=' "$config_root/bitriver.env") -ne 1 ]] ||
  ! grep -Fxq "BITRIVER_CONFIG_ROOT=$config_root" "$config_root/bitriver.env"; then
  echo "installed environment does not persist exactly one absolute config root" >&2
  exit 1
fi

printf '\nBITRIVER_TEST_PRESERVE=yes\n' >>"$config_root/bitriver.env"
"$installer" install "${common_args[@]}"
grep -q '^BITRIVER_TEST_PRESERVE=yes$' "$config_root/bitriver.env"
[[ $(grep -c '^BITRIVER_CONFIG_ROOT=' "$config_root/bitriver.env") -eq 1 ]]
grep -Fxq "BITRIVER_CONFIG_ROOT=$config_root" "$config_root/bitriver.env"

# Older package environments predate BITRIVER_CONFIG_ROOT. Upgrade must append
# the installer-owned value without replacing unrelated operator settings.
grep -v '^BITRIVER_CONFIG_ROOT=' "$config_root/bitriver.env" >"$tmp_root/older-bitriver.env"
mv "$tmp_root/older-bitriver.env" "$config_root/bitriver.env"
"$installer" install "${common_args[@]}"
grep -q '^BITRIVER_TEST_PRESERVE=yes$' "$config_root/bitriver.env"
[[ $(grep -c '^BITRIVER_CONFIG_ROOT=' "$config_root/bitriver.env") -eq 1 ]]
grep -Fxq "BITRIVER_CONFIG_ROOT=$config_root" "$config_root/bitriver.env"

# Ambiguous duplicate path ownership must fail before mutating the env file.
cp "$config_root/bitriver.env" "$tmp_root/bitriver.env.before-duplicate"
printf 'BITRIVER_CONFIG_ROOT=/ambiguous-duplicate\n' >>"$config_root/bitriver.env"
if "$installer" install "${common_args[@]}"; then
  echo "installer unexpectedly accepted duplicate BITRIVER_CONFIG_ROOT entries" >&2
  exit 1
fi
mv "$tmp_root/bitriver.env.before-duplicate" "$config_root/bitriver.env"
if find "$config_root" -maxdepth 1 -name '.bitriver.env.*' -print -quit | grep -q .; then
  echo "installer left a temporary environment file behind" >&2
  exit 1
fi

fake_path="$tmp_root/fake path"
custom_env="$tmp_root/custom config/bitriver.env"
mkdir -p "$fake_path"
cat >"$fake_path/docker" <<'FAKE_DOCKER'
#!/usr/bin/env bash
set -euo pipefail
case "$*" in
  version|"compose version") exit 0 ;;
  *) echo "unexpected fake docker invocation: $*" >&2; exit 1 ;;
esac
FAKE_DOCKER
chmod +x "$fake_path/docker"
PATH="$fake_path:$PATH" \
  BITRIVER_LAUNCHER_ROOT="$source_root" \
  BITRIVER_ENV_FILE="$custom_env" \
  BITRIVER_BINARY="$binary_dir/bitriver" \
  "$binary_dir/bitriver-live" start
test -f "$custom_env"

touch "$config_root/keep-config" "$data_root/keep-data"
"$installer" uninstall --root-prefix "$host_root" --operator-user "$operator_user"
test ! -e "$install_root"
test -f "$config_root/keep-config"
test -f "$data_root/keep-data"

if "$installer" uninstall \
  --root-prefix "$host_root" \
  --operator-user "$operator_user" \
  --purge-data; then
  echo "purge unexpectedly succeeded without explicit confirmation" >&2
  exit 1
fi

"$installer" uninstall \
  --root-prefix "$host_root" \
  --operator-user "$operator_user" \
  --purge-data \
  --yes-really-purge
test ! -e "$config_root"
test ! -e "$data_root"

echo "PASS: compose host installer lifecycle"
