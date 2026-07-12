#!/usr/bin/env bash
set -Eeuo pipefail

# Direct Git Bash invocations on Windows may omit the bundled Unix tools from PATH.
if [[ -d /usr/bin ]]; then
  PATH="/usr/bin:/bin:$PATH"
fi

usage() {
  cat <<'USAGE'
Usage: ./scripts/scan-release-evidence.sh --root DIR [--sentinel-file FILE] [--inventory FILE]

Scans release artifacts and retained evidence for secret-bearing files and content.
Findings report only a rule identifier and relative path; matched values are never printed.
USAGE
}

root=""
sentinel_file=""
inventory_file=""

while (($# > 0)); do
  case "$1" in
    --root)
      shift
      root="${1:-}"
      ;;
    --sentinel-file)
      shift
      sentinel_file="${1:-}"
      ;;
    --inventory)
      shift
      inventory_file="${1:-}"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

if [[ -z "$root" || ! -d "$root" ]]; then
  echo "error: --root must name an existing directory" >&2
  exit 2
fi
if [[ -n "$sentinel_file" && ! -f "$sentinel_file" ]]; then
  echo "error: --sentinel-file must name an existing file" >&2
  exit 2
fi

root="$(cd "$root" && pwd)"
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT
sentinel_patterns="$work_dir/sentinel-patterns.txt"
: >"$sentinel_patterns"

if [[ -n "$sentinel_file" ]]; then
  while IFS= read -r value || [[ -n "$value" ]]; do
    value="${value%$'\r'}"
    [[ -n "$value" ]] || continue
    printf '%s\n' "$value" >>"$sentinel_patterns"
  done <"$sentinel_file"
fi

violations=0

report_violation() {
  local rule="$1"
  local path="$2"
  printf 'secret scan violation [%s]: %s\n' "$rule" "$path" >&2
  violations=$((violations + 1))
}

to_lower() {
  printf '%s' "$1" | LC_ALL=C tr '[:upper:]' '[:lower:]'
}

display_path() {
  local file="$1"
  local prefix="$2"
  local relative="${file#"$root"/}"
  if [[ "$file" == "$root" ]]; then
    relative="."
  fi
  if [[ -n "$prefix" ]]; then
    printf '%s!%s' "$prefix" "$relative"
  else
    printf '%s' "$relative"
  fi
}

is_allowed_example() {
  local name
  name="$(to_lower "$1")"
  [[ "$name" == ".env.example" || "$name" == *.example.env ]]
}

scan_file() {
  local file="$1"
  local label="$2"
  local base="${file##*/}"
  local lower_base
  lower_base="$(to_lower "$base")"

  if ! is_allowed_example "$base"; then
    case "$lower_base" in
      .env|*.env|.env.*|*.env.local|*.secret|*.secrets|*.pem|*.key|*.p12|*.pfx|id_rsa|id_ed25519)
        report_violation "forbidden-file" "$label"
        ;;
    esac
  fi

  if [[ -s "$sentinel_patterns" ]] && LC_ALL=C grep -aqF -f "$sentinel_patterns" -- "$file"; then
    report_violation "known-secret-value" "$label"
  fi

  if LC_ALL=C grep -aEq -- '-----BEGIN ([A-Z0-9 ]+ )?PRIVATE KEY-----' "$file"; then
    report_violation "private-key" "$label"
  fi

  # Shape checks are limited to text. Exact sentinel checks above still inspect binaries.
  if ! LC_ALL=C grep -Iq . "$file"; then
    return
  fi

  local line lower value
  while IFS= read -r line || [[ -n "$line" ]]; do
    lower="$(to_lower "$line")"
    if [[ "$lower" =~ ://[^[:space:]/:@]+:[^[:space:]@/]+@ ]]; then
      value="${BASH_REMATCH[0]}"
      if [[ "$value" != *"\${"* && "$value" != *"example"* && "$value" != *"sample"* && "$value" != *"placeholder"* && "$value" != *"redacted"* ]]; then
        report_violation "credential-url" "$label"
        break
      fi
    fi
  done <"$file"

  if is_allowed_example "$base"; then
    return
  fi

  while IFS= read -r line || [[ -n "$line" ]]; do
    lower="$(to_lower "$line")"
    if [[ "$lower" =~ [\"\']?[a-z0-9_]*(password|token|secret|private_key|dsn)[a-z0-9_]*[\"\']?[[:space:]]*[:=][[:space:]]*(.*)$ ]]; then
      value="${BASH_REMATCH[2]}"
      value="${value%%,*}"
      value="${value%%\}*}"
      value="${value//[[:space:]]/}"
      [[ -n "$value" ]] || continue
      if [[ "$value" == *"redacted"* || "$value" == *"example"* || "$value" == *"sample"* || "$value" == *"placeholder"* || "$value" == *"changeme"* || "$value" == *"\${"* || "$value" == \<*\> || "$value" == "***" || "$value" == "null" ]]; then
        continue
      fi
      report_violation "secret-assignment" "$label"
      break
    fi
  done <"$file"

  local xml_credential_re='<(accesstoken|password|privatekey|secret|token)>([^<]+)</(accesstoken|password|privatekey|secret|token)>'
  while IFS= read -r line || [[ -n "$line" ]]; do
    lower="$(to_lower "$line")"
    if [[ "$lower" =~ $xml_credential_re ]]; then
      value="${BASH_REMATCH[2]}"
      value="${value//[[:space:]]/}"
      if [[ "$value" != *"redacted"* && "$value" != *"example"* && "$value" != *"sample"* && "$value" != *"placeholder"* && "$value" != *"changeme"* && "$value" != *"\${"* && "$value" != \<*\> && "$value" != "***" ]]; then
        report_violation "xml-credential" "$label"
        break
      fi
    fi
  done <"$file"
}

scan_tree() {
  local tree="$1"
  local prefix="${2:-}"
  local file label
  while IFS= read -r -d '' file; do
    if [[ -n "$inventory_file" && "$file" == "$inventory_file" ]]; then
      continue
    fi
    if [[ -n "$prefix" ]]; then
      label="$prefix!${file#"$tree"/}"
    else
      label="$(display_path "$file" "")"
    fi
    scan_file "$file" "$label"
  done < <(find "$tree" -type f -print0)
}

archive_kind() {
  local lower
  lower="$(to_lower "$1")"
  case "$lower" in
    *.tar.gz|*.tgz) printf 'targz' ;;
    *.tar.xz|*.txz) printf 'tarxz' ;;
    *.tar) printf 'tar' ;;
    *.zip) printf 'zip' ;;
    *.deb) printf 'deb' ;;
    *.rpm) printf 'rpm' ;;
    *) return 1 ;;
  esac
}

extract_archive() {
  local archive="$1"
  local kind="$2"
  local destination="$3"
  case "$kind" in
    targz) tar -xzf "$archive" -C "$destination" ;;
    tarxz) tar -xJf "$archive" -C "$destination" ;;
    tar) tar -xf "$archive" -C "$destination" ;;
    zip)
      command -v unzip >/dev/null 2>&1 || return 3
      unzip -qq "$archive" -d "$destination"
      ;;
    deb)
      command -v dpkg-deb >/dev/null 2>&1 || return 3
      dpkg-deb -x "$archive" "$destination"
      ;;
    rpm)
      command -v rpm2cpio >/dev/null 2>&1 || return 3
      command -v cpio >/dev/null 2>&1 || return 3
      (cd "$destination" && rpm2cpio "$archive" | cpio -idm --quiet)
      ;;
  esac
}

scan_archives() {
  local source_tree="$1"
  local depth="$2"
  local prefix="${3:-}"
  ((depth < 3)) || return

  local archive kind destination status label counter=0
  while IFS= read -r -d '' archive; do
    kind="$(archive_kind "$archive")" || continue
    counter=$((counter + 1))
    destination="$work_dir/archive-${depth}-${counter}"
    mkdir -p "$destination"
    if [[ -n "$prefix" ]]; then
      label="$prefix!${archive#"$source_tree"/}"
    else
      label="$(display_path "$archive" "")"
    fi

    set +e
    extract_archive "$archive" "$kind" "$destination"
    status=$?
    set -e
    if [[ $status -eq 3 ]]; then
      # Raw sentinel scanning still covered this package; deep inspection needs a runner tool.
      continue
    fi
    if [[ $status -ne 0 ]]; then
      report_violation "unreadable-archive" "$label"
      continue
    fi
    scan_tree "$destination" "$label"
    scan_archives "$destination" $((depth + 1)) "$label"
  done < <(find "$source_tree" -type f -print0)
}

write_inventory() {
  local output="$1"
  local output_dir
  output_dir="$(dirname "$output")"
  [[ -d "$output_dir" ]] || mkdir -p "$output_dir"
  : >"$output"
  printf 'sha256\tbytes\tpath\n' >>"$output"

  local file relative bytes digest
  while IFS= read -r -d '' file; do
    [[ "$file" == "$output" ]] && continue
    relative="${file#"$root"/}"
    bytes="$(wc -c <"$file" | tr -d '[:space:]')"
    digest="$(sha256sum "$file" | awk '{print $1}')"
    printf '%s\t%s\t%s\n' "$digest" "$bytes" "$relative" >>"$output"
  done < <(find "$root" -type f -print0 | sort -z)
}

scan_tree "$root"
scan_archives "$root" 0

if [[ -n "$inventory_file" ]]; then
  if [[ "$inventory_file" != /* ]]; then
    inventory_file="$root/$inventory_file"
  fi
  write_inventory "$inventory_file"
  scan_file "$inventory_file" "${inventory_file#"$root"/}"
fi

if ((violations > 0)); then
  printf 'Release evidence secret scan failed with %d violation(s).\n' "$violations" >&2
  exit 1
fi

echo "Release evidence secret scan passed."
