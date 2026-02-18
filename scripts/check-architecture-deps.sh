#!/usr/bin/env bash
set -Eeuo pipefail

MODULE_PATH="$(GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go list -m -f '{{.Path}}')"

map_layer() {
  local rel_path="$1"

  if [[ "$rel_path" == cmd/* ]]; then
    echo "cmd"
    return
  fi
  if [[ "$rel_path" == internal/app* ]]; then
    echo "internal/app"
    return
  fi
  if [[ "$rel_path" == internal/api* ]]; then
    echo "internal/api"
    return
  fi
  if [[ "$rel_path" == internal/service* ]]; then
    echo "internal/service"
    return
  fi
  if [[ "$rel_path" == internal/domain* ]]; then
    echo "internal/domain"
    return
  fi
  if [[ "$rel_path" == internal/storage* ]]; then
    echo "internal/storage"
    return
  fi
  if [[ "$rel_path" == internal/ingest* ]]; then
    echo "internal/ingest"
    return
  fi
  if [[ "$rel_path" == internal/chat* ]]; then
    echo "internal/chat"
    return
  fi
  if [[ "$rel_path" == internal/auth* ]]; then
    echo "internal/auth"
    return
  fi
  if [[ "$rel_path" == internal/observability* ]]; then
    echo "internal/observability"
    return
  fi

  echo ""
}

is_forbidden() {
  local from_layer="$1"
  local to_layer="$2"

  case "$from_layer" in
    internal/service|internal/domain)
      case "$to_layer" in
        internal/api|internal/app)
          return 0
          ;;
      esac
      ;;
    internal/storage|internal/ingest|internal/chat|internal/auth|internal/observability)
      case "$to_layer" in
        cmd|internal/app|internal/api|internal/service)
          return 0
          ;;
      esac
      ;;
  esac

  return 1
}

violations=()

while IFS='|' read -r pkg imports; do
  [[ -z "$pkg" ]] && continue
  [[ "$pkg" != "$MODULE_PATH"* ]] && continue

  from_rel="${pkg#${MODULE_PATH}/}"
  from_layer="$(map_layer "$from_rel")"
  [[ -z "$from_layer" ]] && continue

  IFS=',' read -ra import_array <<<"$imports"
  for imp in "${import_array[@]}"; do
    [[ -z "$imp" ]] && continue
    [[ "$imp" != "$MODULE_PATH"* ]] && continue

    to_rel="${imp#${MODULE_PATH}/}"
    to_layer="$(map_layer "$to_rel")"
    [[ -z "$to_layer" ]] && continue

    if is_forbidden "$from_layer" "$to_layer"; then
      violations+=("$pkg -> $imp (layer violation: $from_layer must not import $to_layer)")
    fi
  done
done < <(GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go list -deps -f '{{if not .Standard}}{{.ImportPath}}|{{join .Imports ","}}{{end}}' ./...)

if ((${#violations[@]} > 0)); then
  {
    echo "Architecture import direction check failed."
    echo "The following package imports violate docs/architecture.md:" 
    printf '  - %s\n' "${violations[@]}"
  } >&2
  exit 1
fi

echo "Architecture import direction check passed."
