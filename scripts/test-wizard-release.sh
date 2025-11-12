#!/usr/bin/env bash
# Regression test ensuring the install wizard works when run from a staged release directory.

set -euo pipefail

tmpdir=$(mktemp -d)
cleanup() {
        rm -rf "$tmpdir"
}
trap cleanup EXIT

release_dir="$tmpdir/release"
installer_dir="$release_dir/deploy/install"
invocation_file="$tmpdir/installer-invocation"

mkdir -p "$installer_dir"

cp deploy/install/wizard.sh "$installer_dir/wizard.sh"

cat >"$installer_dir/ubuntu.sh" <<EOF
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" >"$invocation_file"
EOF
chmod +x "$installer_dir/ubuntu.sh"

cat >"$release_dir/server" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$release_dir/server"

cat >"$release_dir/bootstrap-admin" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$release_dir/bootstrap-admin"

responses=$(cat <<'EOF'










n

EOF
)

set +e
output=$(cd "$release_dir" && PATH="/usr/bin:/bin" bash deploy/install/wizard.sh <<<"$responses" 2>&1)
status=$?
set -e

if [[ $status -ne 0 ]]; then
        echo "wizard.sh failed when run from a staged release directory" >&2
        echo "$output" >&2
        exit 1
fi

if [[ ! -f $invocation_file ]]; then
        echo "Expected ubuntu.sh stub to be invoked" >&2
        exit 1
fi

if grep -q "Go 1.21" <<<"$output"; then
        echo "Go toolchain checks should not run when using release binaries" >&2
        echo "$output" >&2
        exit 1
fi
