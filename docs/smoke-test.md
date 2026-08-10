# Smoke test command (`bitriver smoke`)

Use `bitriver smoke` as a fast post-install verification after quickstart or upgrades.

It is designed to be:

- Fast (single-pass checks, no long waits)
- Deterministic
- Safe to run repeatedly
- Actionable on failure

## Command

From the repository root:

```bash
go run ./cmd/bitriver smoke --compose-file deploy/docker-compose.yml --env-file ./.env
```

Packaged launcher equivalent:

```bash
bitriver smoke
```

## What it checks

1. **Docker + Docker Compose availability**
   - Reuses `doctor`'s required-binary and supported Docker/Compose version
     checks.
   - Does not rerun pre-start port availability, host sizing, or bind-mount
     writability checks after the stack is running. BitRiver owns those ports
     during smoke; run `bitriver doctor` before startup for the full preflight.

2. **Compose stack reachability**
   - Runs `docker compose ps --format json` using the selected compose/env files.
   - Verifies that services are listed and `bitriver-live` is present and not dead/exited.

3. **HTTP health endpoints (host ports)**
   - Calls known local endpoints based on `.env` values (or defaults):
     - API `/readyz` and `/healthz`
     - SRS controller `/healthz`
     - Transcoder `/healthz`
     - OME HTTP endpoint (`/`) with success threshold `<500` (to match compose health intent)

4. **PASS/FAIL summary**
   - Prints per-check results and a final summary line.
   - Returns a non-zero exit code if any check fails.

## Failure output

Failures include:

- What check failed
- Why it failed (error/status)
- How to fix it (specific follow-up command)

Common fixes are included inline, such as:

- Start Docker Engine/Desktop
- Run quickstart or compose up
- Inspect service logs with `docker compose logs --tail=80 <service>`

## Related command: `bitriver verify`

Use `bitriver verify` when you want a single backend/deploy health gate that runs:

1. `bitriver doctor`
2. `docker compose -f <compose-file> config` (when Docker is available)
3. `bitriver smoke`

Example:

```bash
go run ./cmd/bitriver verify --compose-file deploy/docker-compose.yml --env-file ./.env
```

`bitriver verify` intentionally skips viewer npm checks.

## Notes

- `bitriver smoke` validates the running stack state; it does not mutate data or restart services.
- Run `bitriver doctor` before first startup or after changing ports, paths, or
  host resources. Run `bitriver smoke` after startup or upgrade.
- If `.env` is missing, initialize it first:

```bash
go run ./cmd/bitriver env init --env-file ./.env
```
