# PLAN

## Scope (current change)
- Execute the quickstart smoke gate from repo root using `./scripts/test-quickstart.sh`.
- Capture complete command output in a dated artifact directory that matches existing `artifacts/release-checks-*` naming.
- If failures occur, map each failure to the runtime deployment contract files (`deploy/docker-compose.yml`, `.env`, `deploy/ome/Server.generated.xml`).
- Update `TASKS.md` and the dated release checklist report with pass/fail status and remediation notes.

## Assumptions
- Docker daemon access is available on this host as requested.
- Existing release report format in `docs/releases/release-checklist-report-*.md` is the expected checklist report structure.

## Risks
- Quickstart smoke can fail from host-level Docker constraints, image pulls, or contract mismatch.
- Long-running integration output could be truncated unless fully redirected to a log artifact.

## Test plan
- `./scripts/test-quickstart.sh` (captured with `tee` into the new artifact log)
