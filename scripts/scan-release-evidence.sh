#!/usr/bin/env bash
set -Eeuo pipefail

# Direct Git Bash invocations on Windows may omit the bundled Unix tools from PATH.
if [[ -d /usr/bin ]]; then
  PATH="/usr/bin:/bin:$PATH"
fi

# macOS still ships Bash 3.2, so use case-insensitive matching instead of the
# Bash 4-only lowercase parameter expansion for filenames and extensions.
shopt -s nocasematch

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
scan_counter=0

report_violation() {
  local rule="$1"
  local path="$2"
  printf 'secret scan violation [%s]: %s\n' "$rule" "$path" >&2
  violations=$((violations + 1))
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
  local name="$1"
  [[ "$name" == ".env.example" || "$name" == *.example.env ]]
}

# This is literal awk source, not a shell expression.
# shellcheck disable=SC2016
readonly TEXT_SHAPE_AWK='
    function base_name(path, parts, count) {
      gsub(/\\/, "/", path)
      count = split(path, parts, "/")
      return tolower(parts[count])
    }
    function allowed_example(path, name) {
      name = base_name(path)
      return name == ".env.example" || name ~ /\.example\.env$/
    }
    function code_path(path) {
      path = tolower(path)
      return path ~ /\.(js|cjs|mjs|ts|cts|mts|map|css|html|rsc|sh|bash|ps1|go|sql|py|rb|yml|yaml|toml|service|md)$/
    }
    function json_path(path) {
      path = tolower(path)
      return path ~ /\.json$/
    }
    function assignment_key(prefix, key) {
      key = prefix
      sub(/[[:space:]]*[:=][[:space:]]*$/, "", key)
      gsub(/["\047[:space:]]/, "", key)
      sub(/^[^a-z_]+/, "", key)
      return tolower(key)
    }
    function sensitive_key(key, is_code) {
      if (!is_code) {
        return key ~ /(^|_)(password|token|secret|private_key|dsn)(_|$)/ ||
          key ~ /^(password|token|secret|privatekey|dsn)$/ ||
          key ~ /^(api|access|auth|admin|client|refresh|bearer|signing|webhook|session|database|db)(password|token|secret|privatekey|dsn)$/
      }
      return key ~ /(^|_)(password|secret|private_key|dsn)$/ ||
        key ~ /^(password|secret|privatekey|dsn)$/ ||
        key ~ /(^|_)(api|access|auth|admin|client|refresh|bearer|signing|webhook|session)(_)?token$/ ||
        key ~ /^(api|access|auth|admin|client|refresh|bearer|signing|webhook|session)(password|token|secret|privatekey|dsn)$/
    }
    function reset_findings() {
      credential_url = 0
      secret_assignment = 0
      xml_credential = 0
    }
    function emit_finding(rule) {
      if (include_filename) {
        printf "%s\t%s%c", rule, current_file, 0
      } else {
        print rule
      }
    }
    function flush_findings() {
      if (current_file == "") return
      if (credential_url) emit_finding("credential-url")
      if (secret_assignment) emit_finding("secret-assignment")
      if (xml_credential) emit_finding("xml-credential")
    }
    FNR == 1 {
      flush_findings()
      current_file = FILENAME
      scan_sensitive = !allowed_example(FILENAME)
      scan_code_references = code_path(FILENAME)
      reset_findings()
    }
    {
      lower = tolower($0)

      if (!credential_url && match(lower, /:\/\/[^[:space:]\/:@]+:[^[:space:]@\/]+@/)) {
        value = substr(lower, RSTART, RLENGTH)
        if (value !~ /(\$\{|example|sample|placeholder|redacted)/) {
          credential_url = 1
        }
      }

      if (scan_sensitive && !secret_assignment &&
          match(lower, /(^|[^a-z0-9_-])["\047]?[a-z_][a-z0-9_]*(password|token|secret|private_key|dsn)[a-z0-9_]*["\047]?[[:space:]]*[:=][[:space:]]*/)) {
        assignment = substr(lower, RSTART, RLENGTH)
        key = assignment_key(assignment)
        quoted_key = assignment ~ /["\047][a-z_][a-z0-9_]*(password|token|secret|private_key|dsn)[a-z0-9_]*["\047][[:space:]]*[:=]/
        value = substr(lower, RSTART + RLENGTH)
        sub(/^[[:space:]]+/, "", value)
        quote = substr(value, 1, 1)
        quoted = quote == "\"" || quote == "\047"
        if (quoted) {
          value = substr(value, 2)
          quote_end = index(value, quote)
          if (quote_end > 0) value = substr(value, 1, quote_end - 1)
        } else {
          sub(/[[:space:],};].*$/, "", value)
        }
        compact = value
        gsub(/[[:space:]]/, "", compact)
        code_reference = value ~ /^[a-z_$][a-z0-9_$]*(\.[a-z_$][a-z0-9_$]*)*$/ ||
          value ~ /^\$[a-z_][a-z0-9_]*$/ || value ~ /^\$env:[a-z_][a-z0-9_]*$/ ||
          value ~ /^\$\(/
        if (sensitive_key(key, scan_code_references) && length(compact) > 0 &&
            compact !~ /(redacted|example|sample|placeholder|changeme|\$\{)/ &&
            compact !~ /^<.*>$/ && compact != "***" && compact != "null" &&
            !(scan_code_references && (!quoted || length(compact) < 8 || code_reference)) &&
            !(json_path(FILENAME) && !quoted_key)) {
          secret_assignment = 1
        }
      }

      if (scan_sensitive && !xml_credential &&
          match(lower, /<(accesstoken|password|privatekey|secret|token)>[^<]+<\/(accesstoken|password|privatekey|secret|token)>/)) {
        value = substr(lower, RSTART, RLENGTH)
        sub(/^<[^>]+>/, "", value)
        sub(/<.*/, "", value)
        gsub(/[[:space:]]/, "", value)
        if (value !~ /(redacted|example|sample|placeholder|changeme|\$\{)/ &&
            value !~ /^<.*>$/ && value != "***") {
          xml_credential = 1
        }
      }
    }
    END { flush_findings() }
'

readonly RG_CREDENTIAL_PATTERN='://[^[:space:]/:@]+:[^[:space:]@/]+@'
readonly RG_ASSIGNMENT_PATTERN="(^|[^a-z0-9_-])[\"']?[a-z_][a-z0-9_]*(password|token|secret|private_key|dsn)[a-z0-9_]*[\"']?[[:space:]]*[:=][[:space:]]*(\"[^\"\\r\\n]*\"|'[^'\\r\\n]*'|[^[:space:],};]+)"
readonly RG_XML_PATTERN='<(accesstoken|password|privatekey|secret|token)>[^<]+</(accesstoken|password|privatekey|secret|token)>'

# This is literal awk source, not a shell expression.
# shellcheck disable=SC2016
readonly RG_MATCH_AWK='
    function code_path(path) {
      path = tolower(path)
      return path ~ /\.(js|cjs|mjs|ts|cts|mts|map|css|html|rsc|sh|bash|ps1|go|sql|py|rb|yml|yaml|toml|service|md)$/
    }
    function json_path(path) {
      path = tolower(path)
      return path ~ /\.json$/
    }
    function base_name(path, parts, count) {
      gsub(/\\/, "/", path)
      count = split(path, parts, "/")
      return tolower(parts[count])
    }
    function allowed_example(path, name) {
      name = base_name(path)
      return name == ".env.example" || name ~ /\.example\.env$/
    }
    function assignment_key(prefix, key) {
      key = prefix
      sub(/[[:space:]]*[:=][[:space:]]*$/, "", key)
      gsub(/["\047[:space:]]/, "", key)
      sub(/^[^a-z_]+/, "", key)
      return tolower(key)
    }
    function sensitive_key(key, is_code) {
      if (!is_code) {
        return key ~ /(^|_)(password|token|secret|private_key|dsn)(_|$)/ ||
          key ~ /^(password|token|secret|privatekey|dsn)$/ ||
          key ~ /^(api|access|auth|admin|client|refresh|bearer|signing|webhook|session|database|db)(password|token|secret|privatekey|dsn)$/
      }
      return key ~ /(^|_)(password|secret|private_key|dsn)$/ ||
        key ~ /^(password|secret|privatekey|dsn)$/ ||
        key ~ /(^|_)(api|access|auth|admin|client|refresh|bearer|signing|webhook|session)(_)?token$/ ||
        key ~ /^(api|access|auth|admin|client|refresh|bearer|signing|webhook|session)(password|token|secret|privatekey|dsn)$/
    }
    function emit(rule, path, key) {
      key = rule SUBSEP path
      if (seen[key]) return
      seen[key] = 1
      printf "%s\t%s%c", rule, path, 0
    }
    {
      separator = index($0, "\t")
      if (separator == 0) next
      path = substr($0, 1, separator - 1)
      lower = tolower(substr($0, separator + 1))

      if (mode == "credential") {
        if (lower !~ /(\$\{|example|sample|placeholder|redacted)/) {
          emit("credential-url", path)
        }
        next
      }

      if (mode == "assignment") {
        if (allowed_example(path)) next
        if (!match(lower, /(^|[^a-z0-9_-])["\047]?[a-z_][a-z0-9_]*(password|token|secret|private_key|dsn)[a-z0-9_]*["\047]?[[:space:]]*[:=][[:space:]]*/)) next
        assignment = substr(lower, RSTART, RLENGTH)
        key = assignment_key(assignment)
        quoted_key = assignment ~ /["\047][a-z_][a-z0-9_]*(password|token|secret|private_key|dsn)[a-z0-9_]*["\047][[:space:]]*[:=]/
        value = substr(lower, RSTART + RLENGTH)
        sub(/^[[:space:]]+/, "", value)
        quote = substr(value, 1, 1)
        quoted = quote == "\"" || quote == "\047"
        if (quoted) {
          value = substr(value, 2)
          quote_end = index(value, quote)
          if (quote_end > 0) value = substr(value, 1, quote_end - 1)
        } else {
          sub(/[[:space:],};].*$/, "", value)
        }
        compact = value
        gsub(/[[:space:]]/, "", compact)
        code_reference = value ~ /^[a-z_$][a-z0-9_$]*(\.[a-z_$][a-z0-9_$]*)*$/ ||
          value ~ /^\$[a-z_][a-z0-9_]*$/ || value ~ /^\$env:[a-z_][a-z0-9_]*$/ ||
          value ~ /^\$\(/
        if (sensitive_key(key, code_path(path)) && length(compact) > 0 &&
            compact !~ /(redacted|example|sample|placeholder|changeme|\$\{)/ &&
            compact !~ /^<.*>$/ && compact != "***" && compact != "null" &&
            !(code_path(path) && (!quoted || length(compact) < 8 || code_reference)) &&
            !(json_path(path) && !quoted_key)) {
          emit("secret-assignment", path)
        }
        next
      }

      if (mode == "xml") {
        if (allowed_example(path)) next
        value = lower
        sub(/^<[^>]+>/, "", value)
        sub(/<.*/, "", value)
        gsub(/[[:space:]]/, "", value)
        if (value !~ /(redacted|example|sample|placeholder|changeme|\$\{)/ &&
            value !~ /^<.*>$/ && value != "***") {
          emit("xml-credential", path)
        }
      }
    }
'

has_fast_rg() {
  [[ "${BITRIVER_SCAN_DISABLE_RG:-0}" != "1" ]] && command -v rg >/dev/null 2>&1
}

tree_rg_shape_findings() {
  local tree="$1"
  local pattern mode raw status
  local index=0
  for mode in credential assignment xml; do
    index=$((index + 1))
    raw="$work_dir/rg-shape-$scan_counter-$index.txt"
    case "$mode" in
      credential) pattern="$RG_CREDENTIAL_PATTERN" ;;
      assignment) pattern="$RG_ASSIGNMENT_PATTERN" ;;
      xml) pattern="$RG_XML_PATTERN" ;;
    esac

    set +e
    (
      cd "$tree"
      LC_ALL=C rg --hidden --no-ignore --ignore-case \
        --with-filename --only-matching --field-match-separator $'\t' \
        --regexp "$pattern" -- .
    ) >"$raw"
    status=$?
    set -e
    if ((status > 1)); then
      return "$status"
    fi
    [[ -s "$raw" ]] || continue
    LC_ALL=C awk -v mode="$mode" "$RG_MATCH_AWK" "$raw" || return 2
  done
}

scan_text_shapes() {
  local file="$1"
  local label="$2"
  local findings rule
  if ! findings="$(LC_ALL=C awk -v include_filename=0 "$TEXT_SHAPE_AWK" "$file")"; then
    report_violation "scanner-error" "$label"
    return
  fi

  while IFS= read -r rule; do
    [[ -n "$rule" ]] || continue
    report_violation "$rule" "$label"
  done <<<"$findings"
}

scan_file() {
  local file="$1"
  local label="$2"
  local base="${file##*/}"

  if ! is_allowed_example "$base"; then
    case "$base" in
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

  scan_text_shapes "$file" "$label"
}

tree_known_secret_matches() {
  local tree="$1"
  if has_fast_rg; then
    (cd "$tree" && LC_ALL=C rg --hidden --no-ignore --null \
      --files-with-matches --text --fixed-strings \
      --file "$sentinel_patterns" -- .)
  else
    (cd "$tree" && LC_ALL=C grep -rlaFZ -f "$sentinel_patterns" -- .)
  fi
}

tree_private_key_matches() {
  local tree="$1"
  if has_fast_rg; then
    (cd "$tree" && LC_ALL=C rg --hidden --no-ignore --null \
      --files-with-matches --text \
      --regexp '-----BEGIN ([A-Z0-9 ]+ )?PRIVATE KEY-----' -- .)
  else
    (cd "$tree" && LC_ALL=C grep -rlaEZ -- \
      '-----BEGIN ([A-Z0-9 ]+ )?PRIVATE KEY-----' .)
  fi
}

tree_text_files() {
  local tree="$1"
  (cd "$tree" && LC_ALL=C grep -rIlZ . -- .)
}

scan_tree() {
  local tree="$1"
  local prefix="${2:-}"
  local file label base finding rule relative status
  local sentinel_matches private_matches shape_matches text_files absolute_text_files
  scan_counter=$((scan_counter + 1))
  sentinel_matches="$work_dir/sentinel-matches-$scan_counter.bin"
  private_matches="$work_dir/private-matches-$scan_counter.bin"
  shape_matches="$work_dir/shape-matches-$scan_counter.bin"
  text_files="$work_dir/text-files-$scan_counter.bin"
  absolute_text_files="$work_dir/absolute-text-files-$scan_counter.bin"

  while IFS= read -r -d '' file; do
    if [[ -n "$inventory_file" && "$file" == "$inventory_file" ]]; then
      continue
    fi
    base="${file##*/}"
    if is_allowed_example "$base"; then
      continue
    fi
    case "$base" in
      .env|*.env|.env.*|*.env.local|*.secret|*.secrets|*.pem|*.key|*.p12|*.pfx|id_rsa|id_ed25519)
        if [[ -n "$prefix" ]]; then
          label="$prefix!${file#"$tree"/}"
        else
          label="${file#"$root"/}"
        fi
        report_violation "forbidden-file" "$label"
        ;;
    esac
  done < <(find "$tree" -type f -print0)

  if [[ -s "$sentinel_patterns" ]]; then
    set +e
    tree_known_secret_matches "$tree" >"$sentinel_matches"
    status=$?
    set -e
    if ((status > 1)); then
      report_violation "scanner-error" "${prefix:-.}"
      return
    fi
    while IFS= read -r -d '' relative; do
      relative="${relative//\\//}"
      relative="${relative#./}"
      if [[ -n "$prefix" ]]; then
        label="$prefix!$relative"
      else
        label="$relative"
      fi
      report_violation "known-secret-value" "$label"
    done <"$sentinel_matches"
  fi

  set +e
  tree_private_key_matches "$tree" >"$private_matches"
  status=$?
  set -e
  if ((status > 1)); then
    report_violation "scanner-error" "${prefix:-.}"
    return
  fi
  while IFS= read -r -d '' relative; do
    relative="${relative//\\//}"
    relative="${relative#./}"
    if [[ -n "$prefix" ]]; then
      label="$prefix!$relative"
    else
      label="$relative"
    fi
    report_violation "private-key" "$label"
  done <"$private_matches"

  if has_fast_rg; then
    set +e
    tree_rg_shape_findings "$tree" >"$shape_matches"
    status=$?
    set -e
    if ((status != 0)); then
      report_violation "scanner-error" "${prefix:-.}"
      return
    fi
  else
    set +e
    tree_text_files "$tree" >"$text_files"
    status=$?
    set -e
    if ((status > 1)); then
      report_violation "scanner-error" "${prefix:-.}"
      return
    fi
    : >"$absolute_text_files"
    while IFS= read -r -d '' relative; do
      relative="${relative//\\//}"
      relative="${relative#./}"
      printf '%s/%s\0' "$tree" "$relative" >>"$absolute_text_files"
    done <"$text_files"
    xargs -0 -r awk -v include_filename=1 "$TEXT_SHAPE_AWK" \
      <"$absolute_text_files" >"$shape_matches"
  fi

  while IFS= read -r -d '' finding; do
    rule="${finding%%$'\t'*}"
    file="${finding#*$'\t'}"
    file="${file//\\//}"
    file="${file#./}"
    if [[ -n "$prefix" ]]; then
      label="$prefix!$file"
    else
      label="$file"
    fi
    report_violation "$rule" "$label"
  done <"$shape_matches"
}

archive_kind() {
  case "$1" in
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
  local payload
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
      payload="${destination}.cpio"
      # Debian's rpm2cpio can return 1 for an nFPM RPM after emitting a complete,
      # valid cpio stream. Treat the cpio parser as the payload-integrity gate.
      rpm2cpio "$archive" >"$payload" || [[ -s "$payload" ]] || return 1
      (cd "$destination" && cpio -idm --quiet --no-absolute-filenames <"$payload") || return 1
      rm -f "$payload"
      [[ -n "$(find "$destination" -mindepth 1 -print -quit)" ]] || return 1
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
