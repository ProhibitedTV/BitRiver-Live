#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT_DIR"

: "${GOTOOLCHAIN:=local}"
: "${GOPROXY:=off}"
: "${GOSUMDB:=off}"

export GOTOOLCHAIN GOPROXY GOSUMDB

GOVULNCHECK_VERSION="v1.1.3"

if ! command -v govulncheck >/dev/null 2>&1; then
  echo "govulncheck not found in PATH." >&2
  echo "Install pinned version with: GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go install golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}" >&2
  echo "Do not use @latest; keep version pinned to match CI workflow (.github/workflows/go-unit-tests.yml)." >&2
  exit 1
fi

echo "Running govulncheck for root module (vendor mode)."

go_version=$(go list -m -f '{{.GoVersion}}')
ignore_stdlib_on_121=0
if [[ "$go_version" == 1.21* ]]; then
  ignore_stdlib_on_121=1
  echo "Go module targets ${go_version}; stdlib-only reachable advisories are informational until the toolchain is raised beyond 1.21."
fi

check_scan_result() {
  local scan_label="$1"
  local scan_json="$2"

  if [[ "$ignore_stdlib_on_121" -eq 0 ]]; then
    # On newer toolchains, fail on any reachable vulnerability.
    return 0
  fi

  local parser
  parser=$(mktemp "${TMPDIR:-/tmp}/govulncheck-parser.XXXXXX.go")
  cat >"$parser" <<'EOF'
package main

import (
  "bufio"
  "encoding/json"
  "fmt"
  "os"
  "sort"
  "strings"
)

type osvEvent struct {
  ID       string `json:"id"`
  Affected []struct {
    Module struct {
      Path string `json:"path"`
    } `json:"module"`
  } `json:"affected"`
}

type finding struct {
  OSV   string `json:"osv"`
  Trace []struct {
    Module string `json:"module"`
  } `json:"trace"`
}

type event struct {
  OSV     *osvEvent `json:"osv"`
  Finding *finding  `json:"finding"`
}

func main() {
  if len(os.Args) != 3 {
    fmt.Fprintln(os.Stderr, "usage: parser <scan-label> <govulncheck-json>")
    os.Exit(2)
  }

  label := os.Args[1]
  path := os.Args[2]

  f, err := os.Open(path)
  if err != nil {
    fmt.Fprintf(os.Stderr, "open %s: %v\n", path, err)
    os.Exit(2)
  }
  defer f.Close()

  osvModules := map[string]map[string]struct{}{}
  nonStdlib := map[string]struct{}{}
  stdlibOnly := map[string]struct{}{}

  scanner := bufio.NewScanner(f)
  for scanner.Scan() {
    line := strings.TrimSpace(scanner.Text())
    if line == "" {
      continue
    }

    var ev event
    if err := json.Unmarshal([]byte(line), &ev); err != nil {
      continue
    }

    if ev.OSV != nil && ev.OSV.ID != "" {
      modules := map[string]struct{}{}
      for _, affected := range ev.OSV.Affected {
        if affected.Module.Path != "" {
          modules[affected.Module.Path] = struct{}{}
        }
      }
      if len(modules) > 0 {
        osvModules[ev.OSV.ID] = modules
      }
      continue
    }

    if ev.Finding == nil || ev.Finding.OSV == "" {
      continue
    }

    id := ev.Finding.OSV
    modules := osvModules[id]
    if len(modules) == 0 && len(ev.Finding.Trace) > 0 && ev.Finding.Trace[0].Module != "" {
      modules = map[string]struct{}{ev.Finding.Trace[0].Module: {}}
    }

    if len(modules) == 0 {
      nonStdlib[id] = struct{}{}
      continue
    }

    onlyStdlib := true
    for module := range modules {
      if module != "stdlib" {
        onlyStdlib = false
        break
      }
    }

    if onlyStdlib {
      stdlibOnly[id] = struct{}{}
    } else {
      nonStdlib[id] = struct{}{}
    }
  }

  if err := scanner.Err(); err != nil {
    fmt.Fprintf(os.Stderr, "scan %s: %v\n", path, err)
    os.Exit(2)
  }

  if len(stdlibOnly) > 0 {
    ids := make([]string, 0, len(stdlibOnly))
    for id := range stdlibOnly {
      ids = append(ids, id)
    }
    sort.Strings(ids)
    fmt.Printf("[govulncheck] %s: ignoring stdlib-only findings on Go 1.21: %s\n", label, strings.Join(ids, ", "))
  }

  if len(nonStdlib) > 0 {
    ids := make([]string, 0, len(nonStdlib))
    for id := range nonStdlib {
      ids = append(ids, id)
    }
    sort.Strings(ids)
    fmt.Printf("[govulncheck] %s: non-stdlib reachable findings detected: %s\n", label, strings.Join(ids, ", "))
    os.Exit(1)
  }
}
EOF

  if ! GOFLAGS="" go run "$parser" "$scan_label" "$scan_json"; then
    rm -f "$parser"
    return 1
  fi
  rm -f "$parser"
  return 0
}

run_govulncheck_scan() {
  local scan_label="$1"
  local goflags="$2"
  shift 2

  local output_file
  output_file=$(mktemp)

  set +e
  GOFLAGS="$goflags" govulncheck -json "$@" >"$output_file"
  local govuln_status=$?
  set -e

  cat "$output_file"

  if [[ "$govuln_status" -eq 0 ]]; then
    rm -f "$output_file"
    return 0
  fi

  if [[ "$govuln_status" -ne 3 ]]; then
    echo "govulncheck failed unexpectedly for ${scan_label} (exit ${govuln_status})." >&2
    rm -f "$output_file"
    return "$govuln_status"
  fi

  if ! check_scan_result "$scan_label" "$output_file"; then
    rm -f "$output_file"
    return 1
  fi

  rm -f "$output_file"
  return 0
}

run_govulncheck_scan "root module" "-mod=vendor" ./...

replace_dirs=$(awk '
  /^replace \(/ {inblock=1; next}
  /^\)/ {inblock=0; next}
  /^replace / || inblock {
    for (i = 1; i <= NF; i++) {
      if ($i == "=>") {
        target = $(i + 1)
        if (target ~ /^\.\/third_party\//) {
          print target
        }
      }
    }
  }
' go.mod | sort -u)

if [ -n "$replace_dirs" ]; then
  echo "Running govulncheck for replaced third_party modules."
  for dir in $replace_dirs; do
    if [ -f "$dir/go.mod" ]; then
      echo "- $dir"
      (
        cd "$dir"
        run_govulncheck_scan "$dir" "" ./...
      )
    else
      echo "- Skipping $dir (no go.mod found)." >&2
    fi
  done
fi
