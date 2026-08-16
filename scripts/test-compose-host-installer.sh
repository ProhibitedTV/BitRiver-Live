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
  test "${BITRIVER_HOST_UID:?}" = "$(id -u)"
  test "${BITRIVER_HOST_GID:?}" = "$(id -g)"
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
  test "${BITRIVER_HOST_UID:?}" = "$(id -u)"
  test "${BITRIVER_HOST_GID:?}" = "$(id -g)"
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
operator_uid=$(id -u)
operator_gid=$(id -g)
common_args=(
  --source-root "$source_root"
  --binary-dir "$binary_dir"
  --root-prefix "$host_root"
  --operator-user "$operator_user"
)

# Reproduce the flat generated-config layout created by RC17 and older packages.
# Installation must preserve these bytes while moving them into the canonical
# source-shaped tree.
legacy_config_root="$host_root/etc/bitriver-live"
mkdir -p "$legacy_config_root"
printf 'legacy-ome-config\n' >"$legacy_config_root/Server.generated.xml"
printf 'legacy-srs-config\n' >"$legacy_config_root/srs.generated.conf"
legacy_ome_sha=$(sha256sum "$legacy_config_root/Server.generated.xml" | awk '{print $1}')
legacy_srs_sha=$(sha256sum "$legacy_config_root/srs.generated.conf" | awk '{print $1}')

"$installer" install "${common_args[@]}"

install_root="$host_root/opt/bitriver-live"
config_root="$host_root/etc/bitriver-live"
data_root="$host_root/var/lib/bitriver-live"
unit_file="$host_root/etc/systemd/system/bitriver-live-compose.service"
ome_config="$config_root/deploy/ome/Server.generated.xml"
srs_config="$config_root/deploy/srs/conf/srs.generated.conf"
legacy_ome_config="$config_root/Server.generated.xml"
legacy_srs_config="$config_root/srs.generated.conf"

test -x "$install_root/bin/bitriver"
test -x "$install_root/bin/bitriver-live"
test -x "$install_root/scripts/backup-postgres.sh"
test -x "$install_root/scripts/restore-postgres.sh"
test -x "$install_root/scripts/prune-backups.sh"
test -x "$install_root/scripts/backup-host-state.sh"
test -x "$install_root/scripts/restore-host-state.sh"
test -f "$install_root/scripts/host_recovery.py"
test -f "$install_root/scripts/python.sh"
test -L "$install_root/.env"
test -L "$install_root/deploy/data"
test -L "$install_root/deploy/transcoder-data"
test -L "$install_root/deploy/ome/Server.generated.xml"
test -L "$install_root/deploy/srs/conf/srs.generated.conf"
test "$(readlink "$install_root/deploy/ome/Server.generated.xml")" = "$ome_config"
test "$(readlink "$install_root/deploy/srs/conf/srs.generated.conf")" = "$srs_config"
test -f "$ome_config"
test -f "$srs_config"
test "$(sha256sum "$ome_config" | awk '{print $1}')" = "$legacy_ome_sha"
test "$(sha256sum "$srs_config" | awk '{print $1}')" = "$legacy_srs_sha"
test -L "$legacy_ome_config"
test -L "$legacy_srs_config"
test "$(readlink "$legacy_ome_config")" = "$ome_config"
test "$(readlink "$legacy_srs_config")" = "$srs_config"
test "$(stat -c '%a' "$ome_config")" = 640
test "$(stat -c '%a' "$srs_config")" = 640
test "$(stat -c '%u:%g' "$ome_config")" = "$operator_uid:$operator_gid"
test "$(stat -c '%u:%g' "$srs_config")" = "$operator_uid:$operator_gid"
test "$(stat -c '%a' "$config_root/deploy/ome")" = 750
test "$(stat -c '%a' "$config_root/deploy/srs/conf")" = 750
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
grep -Fxq "Environment=\"BITRIVER_HOST_UID=$operator_uid\"" "$unit_file"
grep -Fxq "Environment=\"BITRIVER_HOST_GID=$operator_gid\"" "$unit_file"
if grep -Eq 'P0stgres-Example!|R3dis-Example!|OME-Example-(Pass|Access-Token)' "$config_root/bitriver.env"; then
  echo "installed environment retains release sample credentials" >&2
  exit 1
fi
if [[ $(grep -c '^BITRIVER_CONFIG_ROOT=' "$config_root/bitriver.env") -ne 1 ]] ||
  ! grep -Fxq "BITRIVER_CONFIG_ROOT=$config_root" "$config_root/bitriver.env"; then
  echo "installed environment does not persist exactly one absolute config root" >&2
  exit 1
fi
for entry in "BITRIVER_HOST_UID=$operator_uid" "BITRIVER_HOST_GID=$operator_gid"; do
  key=${entry%%=*}
  [[ $(grep -c "^${key}=" "$config_root/bitriver.env") -eq 1 ]]
  grep -Fxq "$entry" "$config_root/bitriver.env"
done

printf '\nBITRIVER_TEST_PRESERVE=yes\n' >>"$config_root/bitriver.env"
"$installer" install "${common_args[@]}"
grep -q '^BITRIVER_TEST_PRESERVE=yes$' "$config_root/bitriver.env"
[[ $(grep -c '^BITRIVER_CONFIG_ROOT=' "$config_root/bitriver.env") -eq 1 ]]
grep -Fxq "BITRIVER_CONFIG_ROOT=$config_root" "$config_root/bitriver.env"
grep -Fxq "BITRIVER_HOST_UID=$operator_uid" "$config_root/bitriver.env"
grep -Fxq "BITRIVER_HOST_GID=$operator_gid" "$config_root/bitriver.env"
test "$(sha256sum "$ome_config" | awk '{print $1}')" = "$legacy_ome_sha"
test "$(sha256sum "$srs_config" | awk '{print $1}')" = "$legacy_srs_sha"

# A fresh host without legacy files creates the same canonical tree and bounded
# compatibility links.
fresh_host_root="$tmp_root/fresh host root"
fresh_args=(
  --source-root "$source_root"
  --binary-dir "$binary_dir"
  --root-prefix "$fresh_host_root"
  --operator-user "$operator_user"
)
"$installer" install "${fresh_args[@]}"
fresh_config_root="$fresh_host_root/etc/bitriver-live"
test -f "$fresh_config_root/deploy/ome/Server.generated.xml"
test -f "$fresh_config_root/deploy/srs/conf/srs.generated.conf"
test "$(readlink "$fresh_config_root/Server.generated.xml")" = \
  "$fresh_config_root/deploy/ome/Server.generated.xml"
test "$(readlink "$fresh_config_root/srs.generated.conf")" = \
  "$fresh_config_root/deploy/srs/conf/srs.generated.conf"

# Older package environments predate the managed config/ownership keys. Upgrade
# must append them without replacing unrelated operator settings.
grep -Ev '^BITRIVER_(CONFIG_ROOT|HOST_UID|HOST_GID)=' "$config_root/bitriver.env" >"$tmp_root/older-bitriver.env"
mv "$tmp_root/older-bitriver.env" "$config_root/bitriver.env"
"$installer" install "${common_args[@]}"
grep -q '^BITRIVER_TEST_PRESERVE=yes$' "$config_root/bitriver.env"
[[ $(grep -c '^BITRIVER_CONFIG_ROOT=' "$config_root/bitriver.env") -eq 1 ]]
grep -Fxq "BITRIVER_CONFIG_ROOT=$config_root" "$config_root/bitriver.env"
grep -Fxq "BITRIVER_HOST_UID=$operator_uid" "$config_root/bitriver.env"
grep -Fxq "BITRIVER_HOST_GID=$operator_gid" "$config_root/bitriver.env"

# Any ambiguous managed ownership key must fail before mutating the env file.
printf '  BITRIVER_HOST_GID = 99999\n' >>"$config_root/bitriver.env"
cp "$config_root/bitriver.env" "$tmp_root/bitriver.env.with-duplicate"
if "$installer" install "${common_args[@]}"; then
  echo "installer unexpectedly accepted duplicate BITRIVER_HOST_GID entries" >&2
  exit 1
fi
cmp "$tmp_root/bitriver.env.with-duplicate" "$config_root/bitriver.env"
grep -v '^[[:space:]]*BITRIVER_HOST_GID[[:space:]]*=[[:space:]]*99999$' "$config_root/bitriver.env" >"$tmp_root/bitriver.env.repaired"
mv "$tmp_root/bitriver.env.repaired" "$config_root/bitriver.env"
if find "$config_root" -maxdepth 1 -name '.bitriver.env.*' -print -quit | grep -q .; then
  echo "installer left a temporary environment file behind" >&2
  exit 1
fi

# Equal dual copies collapse back to the compatibility link. Divergent copies
# fail before config bytes or the managed environment can be changed.
rm -f -- "$legacy_srs_config"
cp "$srs_config" "$legacy_srs_config"
"$installer" install "${common_args[@]}"
test -L "$legacy_srs_config"

rm -f -- "$legacy_ome_config"
printf 'divergent-legacy-ome\n' >"$legacy_ome_config"
cp "$legacy_ome_config" "$tmp_root/divergent-legacy-ome"
cp "$ome_config" "$tmp_root/canonical-ome-before-conflict"
cp "$config_root/bitriver.env" "$tmp_root/env-before-config-conflict"
if "$installer" install "${common_args[@]}"; then
  echo "installer unexpectedly accepted divergent OME generated configs" >&2
  exit 1
fi
cmp "$tmp_root/divergent-legacy-ome" "$legacy_ome_config"
cmp "$tmp_root/canonical-ome-before-conflict" "$ome_config"
cmp "$tmp_root/env-before-config-conflict" "$config_root/bitriver.env"
rm -f -- "$legacy_ome_config"
ln -s "$ome_config" "$legacy_ome_config"

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
