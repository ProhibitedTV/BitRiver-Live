# Release notes

This directory holds draft and published release-note snapshots for BitRiver Live.

It is intentionally limited to human-facing release notes. Do not add CI logs, checklist dumps, or one-off release evidence here; keep those in the change request, release ticket, or workflow artefacts instead.

Use the release docs in this order:

1. [`../../CHANGELOG.md`](../../CHANGELOG.md) for the human-readable summary of what changed.
2. [`../production-release.md`](../production-release.md) for the operational release checklist.
3. [`.github/RELEASE_NOTES_TEMPLATE.md`](../../.github/RELEASE_NOTES_TEMPLATE.md) when drafting the GitHub Release body for a new tag.

Naming convention:

- `vX.Y.Z-draft.md` while notes are still being prepared
- `vX.Y.Z.md` once the release is published and the note is final

Current draft:

- [`v1.2.3-draft.md`](v1.2.3-draft.md)
