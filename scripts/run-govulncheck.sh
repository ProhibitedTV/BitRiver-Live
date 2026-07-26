#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT_DIR"

: "${GOTOOLCHAIN:=local}"
: "${GOPROXY:=off}"
: "${GOSUMDB:=off}"

export GOTOOLCHAIN GOPROXY GOSUMDB

GOVULNCHECK_VERSION="v1.6.0"
BASELINE_FILE="${GOVULNCHECK_BASELINE:-${ROOT_DIR}/scripts/govulncheck-baseline.json}"
OUTPUT_ROOT="${GOVULNCHECK_OUT_DIR:-${ROOT_DIR}/.artifacts/govulncheck}"
RUN_ID="$(date +%Y%m%d-%H%M%S)"
OUTPUT_DIR="${OUTPUT_ROOT}/${RUN_ID}"
RAW_DIR="${OUTPUT_DIR}/raw"
mkdir -p "$RAW_DIR"

if ! command -v govulncheck >/dev/null 2>&1; then
  echo "govulncheck not found in PATH." >&2
  echo "Install pinned version with: GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go install golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}" >&2
  echo "Do not use @latest; keep version pinned to match CI workflow (.github/workflows/go-unit-tests.yml)." >&2
  exit 1
fi

if [ ! -f "$BASELINE_FILE" ]; then
  echo "Missing govulncheck baseline: $BASELINE_FILE" >&2
  exit 1
fi

go_version=$(go list -m -f '{{.GoVersion}}')
goos=$(go env GOOS)
goarch=$(go env GOARCH)

scan_index_file="${OUTPUT_DIR}/scans.tsv"
: >"$scan_index_file"

sanitize_label() {
  printf '%s' "$1" | tr ' /' '__' | tr -cd '[:alnum:]_.-'
}

python_path() {
  local path="$1"
  local platform
  platform="$(uname -s)"
  if [[ "$platform" =~ ^(MINGW|MSYS|CYGWIN) ]] && command -v cygpath >/dev/null 2>&1; then
    cygpath -w "$path"
    return
  fi
  printf '%s\n' "$path"
}

run_govulncheck_scan() {
  local scan_label="$1"
  local goflags="$2"
  shift 2

  local safe_label
  safe_label="$(sanitize_label "$scan_label")"

  local output_file="${RAW_DIR}/${safe_label}.jsonl"
  local metadata_file="${RAW_DIR}/${safe_label}.meta"

  printf 'scan_label=%s\n' "$scan_label" >"$metadata_file"
  printf 'goos=%s\n' "$goos" >>"$metadata_file"
  printf 'goarch=%s\n' "$goarch" >>"$metadata_file"

  set +e
  GOFLAGS="$goflags" govulncheck -json "$@" >"$output_file"
  local govuln_status=$?
  set -e

  if [ "$govuln_status" -ne 0 ] && [ "$govuln_status" -ne 3 ]; then
    echo "govulncheck failed unexpectedly for ${scan_label} (exit ${govuln_status})." >&2
    return "$govuln_status"
  fi

  printf '%s\t%s\n' "$scan_label" "$(python_path "$output_file")" >>"$scan_index_file"
  return 0
}

run_govulncheck_scan "root module" "" ./...

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
  for dir in $replace_dirs; do
    if [ -f "$dir/go.mod" ]; then
      (
        cd "$dir"
        run_govulncheck_scan "$dir" "" ./...
      )
    fi
  done
fi

summary_file="${OUTPUT_DIR}/summary.json"
new_file="${OUTPUT_DIR}/new-findings.json"
all_findings_file="${OUTPUT_DIR}/findings.json"

baseline_file_python="$(python_path "$BASELINE_FILE")"
scan_index_file_python="$(python_path "$scan_index_file")"
summary_file_python="$(python_path "$summary_file")"
new_file_python="$(python_path "$new_file")"
all_findings_file_python="$(python_path "$all_findings_file")"

python3 - "$baseline_file_python" "$scan_index_file_python" "$summary_file_python" "$new_file_python" "$all_findings_file_python" "$go_version" "$goos" "$goarch" <<'PY'
import json
import pathlib
import sys
from datetime import datetime, timezone

baseline_path = pathlib.Path(sys.argv[1])
scan_index_path = pathlib.Path(sys.argv[2])
summary_path = pathlib.Path(sys.argv[3])
new_path = pathlib.Path(sys.argv[4])
findings_path = pathlib.Path(sys.argv[5])
go_version = sys.argv[6]
goos = sys.argv[7]
goarch = sys.argv[8]

def load_json(path: pathlib.Path):
    with path.open("r", encoding="utf-8") as f:
        return json.load(f)

def load_json_stream(path: pathlib.Path):
    data = path.read_text(encoding="utf-8")
    decoder = json.JSONDecoder()
    offset = 0
    while offset < len(data):
        while offset < len(data) and data[offset].isspace():
            offset += 1
        if offset >= len(data):
            return
        event, offset = decoder.raw_decode(data, offset)
        if isinstance(event, dict):
            yield event

baseline = load_json(baseline_path)
entries = baseline.get("entries", [])

if not isinstance(entries, list):
    raise SystemExit("govulncheck baseline entries must be a list")

today = datetime.now(timezone.utc).date()
for index, entry in enumerate(entries):
    if not isinstance(entry, dict):
        raise SystemExit(f"govulncheck baseline entry {index} must be an object")
    for field in ("id", "owner", "reason", "tracking_issue", "expires"):
        if not str(entry.get(field, "")).strip():
            raise SystemExit(f"govulncheck baseline entry {index} missing {field}")
    try:
        expires = datetime.strptime(str(entry["expires"]), "%Y-%m-%d").date()
    except ValueError as exc:
        raise SystemExit(f"govulncheck baseline entry {index} has invalid expires date") from exc
    if expires < today:
        raise SystemExit(f"govulncheck baseline entry {index} expired on {expires.isoformat()}")

def matches(entry, finding):
    def field(name):
        return str(entry.get(name, "*") or "*")
    checks = {
        "id": field("id"),
        "module": field("module"),
        "scan": field("scan"),
        "goos": field("goos"),
        "goarch": field("goarch"),
    }
    for key, expected in checks.items():
        if expected != "*" and finding.get(key) != expected:
            return False
    return True

all_findings = {}
scan_files = []

with scan_index_path.open("r", encoding="utf-8") as f:
    for raw in f:
        raw = raw.strip()
        if not raw:
            continue
        label, file_path = raw.split("\t", 1)
        scan_files.append((label, pathlib.Path(file_path)))

for label, scan_file in scan_files:
    osv_modules = {}
    for event in load_json_stream(scan_file):
        osv = event.get("osv")
        if isinstance(osv, dict) and osv.get("id"):
            modules = set()
            for affected in osv.get("affected", []) or []:
                if not isinstance(affected, dict):
                    continue
                module = affected.get("module")
                package = affected.get("package")
                if isinstance(module, dict) and module.get("path"):
                    modules.add(module["path"])
                elif isinstance(package, dict) and package.get("name"):
                    modules.add(package["name"])
            if modules:
                osv_modules[osv["id"]] = sorted(modules)
            continue

        finding = event.get("finding")
        if not isinstance(finding, dict):
            continue

        vuln_id = finding.get("osv")
        if not vuln_id:
            continue

        modules = list(osv_modules.get(vuln_id, []))
        if not modules:
            trace = finding.get("trace") or []
            first_frame = trace[0] if trace and isinstance(trace[0], dict) else {}
            module = first_frame.get("module")
            if isinstance(module, dict):
                module = module.get("path")
            if module:
                modules = [module]
        if not modules:
            modules = ["unknown"]

        for module in modules:
            key = (vuln_id, module, label, goos, goarch)
            all_findings[key] = {
                "id": vuln_id,
                "module": module,
                "scan": label,
                "goos": goos,
                "goarch": goarch,
            }

processed = []
new_disallowed = []
baselined_disallowed = []
informational = []

for finding in sorted(all_findings.values(), key=lambda item: (item["id"], item["scan"], item["module"])):
    finding["policy"] = "disallowed-reachable"
    is_baselined = any(matches(entry, finding) for entry in entries)
    if is_baselined:
        finding["status"] = "baselined"
        baselined_disallowed.append(finding)
    else:
        finding["status"] = "new"
        new_disallowed.append(finding)
    processed.append(finding)

summary = {
    "generated_at": datetime.now(timezone.utc).isoformat(),
    "go_version": go_version,
    "goos": goos,
    "goarch": goarch,
    "scan_files": [{"scan": label, "file": str(path)} for label, path in scan_files],
    "counts": {
        "total_findings": len(processed),
        "new_disallowed": len(new_disallowed),
        "baselined_disallowed": len(baselined_disallowed),
        "informational": len(informational),
    },
    "new_disallowed": new_disallowed,
    "baselined_disallowed": baselined_disallowed,
    "informational": informational,
}

summary_path.write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
new_path.write_text(json.dumps(new_disallowed, indent=2) + "\n", encoding="utf-8")
findings_path.write_text(json.dumps(processed, indent=2) + "\n", encoding="utf-8")
PY

echo "=== New vulnerabilities introduced ==="
if [ "$(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))))' "$new_file_python")" -eq 0 ]; then
  echo "None"
else
  python3 - "$new_file_python" <<'PY'
import json
import sys
items = json.load(open(sys.argv[1], "r", encoding="utf-8"))
for item in items:
    print(f"- {item['id']} module={item['module']} scan={item['scan']} platform={item['goos']}/{item['goarch']}")
    print(f"::error title=New govulncheck finding::{item['id']} in {item['module']} ({item['scan']}, {item['goos']}/{item['goarch']})")
PY
fi

echo
echo "=== Govulncheck summary ==="
python3 - "$summary_file_python" <<'PY'
import json
import sys
summary = json.load(open(sys.argv[1], "r", encoding="utf-8"))
counts = summary.get("counts", {})
print(f"Go version: {summary.get('go_version')}")
print(f"Platform: {summary.get('goos')}/{summary.get('goarch')}")
print(f"Total findings: {counts.get('total_findings', 0)}")
print(f"New disallowed: {counts.get('new_disallowed', 0)}")
print(f"Baselined disallowed: {counts.get('baselined_disallowed', 0)}")
print(f"Informational: {counts.get('informational', 0)}")
PY

echo "Artifacts written to: ${OUTPUT_DIR}"

test "$(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))))' "$new_file_python")" -eq 0
