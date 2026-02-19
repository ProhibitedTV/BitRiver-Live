# Dependency checksum policy

BitRiver Live keeps the default verification path offline:

- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off`
- `./scripts/verify.sh`

Those defaults are intentional and must remain unchanged for local and CI verification.

## When maintainers may refresh `go.sum`

Refresh `go.sum` only when dependency metadata changes, including:

- updates to `go.mod` `require` entries,
- replacement strategy updates for `third_party/` mirrors,
- intentional checksum resync after dependency mirror refreshes.

Do **not** refresh `go.sum` as part of unrelated changes.

## Controlled checksum refresh procedure (network-enabled)

Run this from repository root in a network-enabled shell:

```bash
./scripts/refresh-go-sum.sh
```

The script generates checksums from upstream module releases in an isolated temp module and writes a non-empty root `go.sum`.

## Required review before commit

Before committing a refreshed `go.sum`, maintainers must review the dependency diff:

```bash
git diff -- go.sum
```

Only commit expected checksum changes.

## Read-only guard

Use this read-only check locally or in CI to ensure `go.sum` was not cleared:

```bash
./scripts/check-go-sum-not-empty.sh
```
