#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
env_file="$repo_root/deploy/.env.example"
doc_file="$repo_root/docs/contract.md"

begin_marker='<!-- BEGIN GENERATED ENV -->'
end_marker='<!-- END GENERATED ENV -->'
check_mode=0

usage() {
  cat <<'USAGE'
Usage: scripts/generate-contract-doc.sh [--check]

Regenerates the environment-variable section in docs/contract.md from deploy/.env.example.

Options:
  --check   Verify docs/contract.md is up to date without writing changes.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check)
      check_mode=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ ! -f "$env_file" ]]; then
  echo "missing env template: $env_file" >&2
  exit 1
fi

if [[ ! -f "$doc_file" ]]; then
  echo "missing contract doc: $doc_file" >&2
  exit 1
fi

generated_block_file="$(mktemp)"
updated_doc_file="$(mktemp)"
trap 'rm -f "$generated_block_file" "$updated_doc_file"' EXIT

awk '
function group_for(name, parts, n) {
  if (name ~ /^BITRIVER_/) {
    return "BITRIVER_*"
  }
  if (name ~ /^NEXT_PUBLIC_/) {
    return "NEXT_PUBLIC_*"
  }
  if (name ~ /^NEXT_/) {
    return "NEXT_*"
  }

  n = split(name, parts, "_")
  if (n > 1) {
    return parts[1] "_*"
  }

  return "OTHER"
}

BEGIN {
  print "<!-- BEGIN GENERATED ENV -->"
  print ""
  print "_This section is generated from `deploy/.env.example` by `scripts/generate-contract-doc.sh`. Do not edit by hand._"
  print ""
}

/^[[:space:]]*#/ || /^[[:space:]]*$/ {
  next
}

/^[A-Za-z_][A-Za-z0-9_]*=/ {
  split($0, pair, "=")
  name = pair[1]
  value = substr($0, length(name) + 2)
  group = group_for(name)

  if (!(group in seen_group)) {
    seen_group[group] = 1
    group_order[++group_count] = group
  }

  item_count[group]++
  items[group, item_count[group], "name"] = name
  items[group, item_count[group], "value"] = value
}

END {
  for (gidx = 1; gidx <= group_count; gidx++) {
    group = group_order[gidx]
    print "### `" group "`"
    print ""
    print "| Variable | Default |"
    print "| --- | --- |"

    for (i = 1; i <= item_count[group]; i++) {
      name = items[group, i, "name"]
      value = items[group, i, "value"]

      if (value == "") {
        value = "_(empty)_"
      } else {
        gsub(/`/, "\\`", value)
        value = "`" value "`"
      }

      print "| `" name "` | " value " |"
    }

    print ""
  }

  print "<!-- END GENERATED ENV -->"
}
' "$env_file" > "$generated_block_file"

if ! grep -Fq "$begin_marker" "$doc_file"; then
  cat >> "$doc_file" <<DOCBLOCK

## Generated environment variable index

$begin_marker
$end_marker
DOCBLOCK
fi

awk -v begin="$begin_marker" -v end="$end_marker" -v replacement="$generated_block_file" '
BEGIN {
  in_block = 0
  replaced = 0
}

index($0, begin) {
  while ((getline line < replacement) > 0) {
    print line
  }
  close(replacement)
  in_block = 1
  replaced = 1
  next
}

index($0, end) {
  in_block = 0
  next
}

{
  if (!in_block) {
    print
  }
}

END {
  if (!replaced) {
    exit 2
  }
}
' "$doc_file" > "$updated_doc_file"

if [[ "$check_mode" -eq 1 ]]; then
  if ! cmp -s "$doc_file" "$updated_doc_file"; then
    echo "docs/contract.md generated env section is out of date." >&2
    echo "Run: ./scripts/generate-contract-doc.sh" >&2
    exit 1
  fi

  echo "docs/contract.md generated env section is up to date"
  exit 0
fi

mv "$updated_doc_file" "$doc_file"
echo "Updated docs/contract.md generated env section"
