# Trivy exception policy

This directory stores repository-managed Trivy ignore files used by
`.github/workflows/image-scan.yml`.

## Rules

- Scope exceptions to a single image whenever possible (`<image>.txt`).
- Match by explicit CVE ID only; never suppress by severity.
- Every entry must include:
  - why the vulnerability is currently unavoidable (for example, upstream marks it as `will_not_fix`),
  - who reviewed it/date reviewed,
  - an expiry/review date.
- Remove entries as soon as the pinned image is updated with a fix.

## Current files

- `default.txt`: intentionally empty broad policy file.
- `postgres-15-alpine.txt`: exception(s) only for `postgres:15-alpine`.
