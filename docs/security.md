# Security Hardening Defaults

## Container hardening defaults + exceptions

`deploy/docker-compose.yml` applies conservative runtime hardening defaults intended to preserve the existing stack behavior while reducing container privilege.

### Defaults applied

- `security_opt: ["no-new-privileges:true"]` is set on all long-running services (`bitriver-live`, `viewer`, `redis`, `postgres`, `srs-controller`, `srs`, `ome`, `transcoder`, `transcoder-public`) and operational helpers where supported.
- `cap_drop: ["ALL"]` is applied broadly to remove ambient Linux capabilities from services that do not require elevated kernel features.
- `read_only: true` is enabled for stateless services and one-shot validation jobs where writable paths are explicitly provided with bind mounts or `tmpfs` (`bitriver-live`, `viewer`, `postgres-migrations`, `srs-controller`, `ome-health-token-check`, `transcoder`, `transcoder-public`, `srs-api`, `postgres-host-port`).
- Non-root users are explicitly configured where image support is known:
  - `bitriver-live` runs as `65532:65532` (`appuser` from the runtime image).
  - `srs-controller` runs as `10002:10002` (`appuser` from the runtime image).
  - `transcoder` runs as `10001:10001` (`appuser` from the runtime image).
  - `viewer` runs as the image-provided `node` user.

### Documented exceptions (kept for compatibility)

- `redis`: kept writable and image-default user due to upstream entrypoint/runtime filesystem behavior.
- `postgres`: kept writable and image-default user due to upstream initdb/runtime write requirements and UID/GID handling.
- `srs`: kept writable and image-default user because upstream runtime writes process/log artifacts and is not documented for non-root startup.
- `ome`: kept writable and image-default user because vendor startup/runtime expects writable internal paths.
- `srs-config` and `ome-config`: one-shot config generation jobs remain writable/root to modify files in the bind-mounted repository workspace.
- `transcoder-public`: stays image-default user for nginx runtime behavior while still using read-only root plus explicit `tmpfs` runtime paths.

### Network segmentation defaults

- `public` network: services with host-facing ingress/egress paths (`bitriver-live`, `srs-controller`, `srs`, `ome`, `transcoder`, `transcoder-public`).
- `internal` network: state/control plane and coordination services (`postgres`, `redis`, `postgres-migrations`, `srs-config`, `ome-config`, `ome-health-token-check`) plus dual-attached services that must communicate with both layers (`bitriver-live`, `srs-controller`, `srs`, `ome`, `transcoder`, `viewer`).

These defaults aim for least privilege without changing the default deployment topology or published ports.
