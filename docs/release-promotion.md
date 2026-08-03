# Immutable candidate and stable promotion

BitRiver Live builds a release once. Candidate tags create binaries, installers,
viewer assets, SBOMs, five first-party images, scanner-approved evidence, and a
signed `release-set.json`. A stable release copies those exact candidate bytes
and retags those exact image digests. It never rebuilds a stable tag.

## Trust boundary

Use the candidate tag plus the digest in `release-set.json` as production
identity. Stable image tags are verified aliases for those digests. `latest` is
an optional convenience alias and is never an integrity or rollback source.

Candidate releases created by the current workflow contain:

- `release-set.json` and its readable `release-set.md` summary;
- `release-set.sigstore.json`, signed by the exact candidate tag invocation of
  `.github/workflows/release.yml`;
- `CHECKSUMS.txt`, covering every other immutable candidate asset exactly once;
- five image SBOMs and five image Sigstore bundles;
- sanitized contract, dependency, image, product-gate, and scan evidence; and
- the cross-platform archives, packages, viewer bundle, MSI, and formula.

Verify a downloaded candidate set before installation:

```bash
release_tag=v1.2.3-rc.N
identity="https://github.com/ProhibitedTV/BitRiver-Live/.github/workflows/release.yml@refs/tags/${release_tag}"

cosign verify-blob \
  --bundle release-set.sigstore.json \
  --certificate-identity "$identity" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  release-set.json
artifact=bitriver-live_v1.2.3-rc.N_amd64.deb
expected="$(jq -r --arg name "$artifact" '.artifacts[] | select(.name == $name) | .sha256' release-set.json)"
printf '%s  %s\n' "$expected" "$artifact" | sha256sum --check -
python3 scripts/release_set.py verify-candidate \
  --assets-dir . \
  --manifest release-set.json \
  --expected-tag "$release_tag"
```

The last command needs every release asset plus `scripts/release_set.py` from
the same tag and checks the complete `CHECKSUMS.txt` coverage. For an
artifact-only host, the minimum path is the signed manifest plus a direct
comparison of the selected artifact against its hash in that manifest;
`CHECKSUMS.txt` by itself is not a signed root.

## Promotion record

Stable promotion requires a reviewed JSON record under
`docs/releases/promotions/`. The record binds all required gate evidence to the
SHA-256 of one candidate `release-set.json`; a green check from another RC is
invalid. Use [`promotion-record.example.json`](releases/promotion-record.example.json)
as the shape, replace every example value, and commit it through the protected
`main` pull-request path.

The required gates are clean-host installation (#1297), backup/restore (#1299),
upgrade/rollback (#1298), capacity (#1303), resilience and OME recovery (#1304),
SLO/alerts (#1305), security review (#1306), and viewer/browser compatibility
(#1307). Each gate must be closed in GitHub and retain a durable HTTPS evidence
URL plus SHA-256 for the exact candidate.

## Guarded promotion

Run `.github/workflows/stable-promotion.yml` from protected `main` with:

- `operation=promote`;
- the immutable candidate tag;
- its matching stable base tag;
- the tracked promotion-record path; and
- `publish_latest=false` unless moving convenience aliases is intentional.

`Stable promotion gate` has read-only permissions and runs before environment
approval. It verifies GitHub asset digests, complete checksum coverage, the
candidate tag and commit, root and image signatures, the tracked record, live
gate issue states, revocation markers, prior rollback metadata, and current
stable state. A failure cannot reach the `stable-promotion` environment.

After required review, the write job rechecks live state, creates or resumes a
draft release, retags each image by digest, copies candidate assets without
renaming or rebuilding them, signs deterministic stable/rollback metadata,
verifies GitHub's stored hashes, and publishes the draft. A retry is a no-op or
safe resume when existing bytes match; any conflicting tag, asset, or stable
alias fails closed.

The first stable release records that no previous stable rollback set exists.
Later releases include the previously signed stable root and exact image
digests. Database/config backups are still required; image rollback metadata is
not a stateful-data rollback.

## Candidate revocation

Run the same workflow with `operation=revoke` and an actionable reason. After
environment review, it appends a uniquely named JSON marker and Sigstore bundle
to the candidate release. It never overwrites an existing marker and has no
package/image write permission. Promotion rejects a candidate as soon as any
revocation marker is present; revoking a candidate does not move or rewrite an
already published stable release.

Revocation markers are an append-only overlay, so they are deliberately not
part of the candidate's original immutable `CHECKSUMS.txt`. Verify each marker
with the exact `stable-promotion.yml@refs/heads/main` identity.

## Repository environment

The `stable-promotion` GitHub environment must require a named reviewer and
protected-branch deployments. This project permits self-review because it is a
single-owner public repository; the manual approval and audit event remain
required. The workflow file alone does not prove the live environment rule is
configured, so release evidence includes an API readback and a negative
dispatch that fails in the read-only gate before approval or mutation.
