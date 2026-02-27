# BitRiver Live release versioning rules

This document defines the release contract for version numbers and upgrade expectations.

## SemVer policy

BitRiver Live uses Semantic Versioning (`MAJOR.MINOR.PATCH`, typically tagged as `vMAJOR.MINOR.PATCH`).

- **PATCH**: bug fixes, security fixes, dependency updates, and operational hardening that do not require operator workflow changes.
- **MINOR**: backward-compatible features, new optional config, new endpoints, additive schema changes, and non-breaking behavior improvements.
- **MAJOR**: breaking changes requiring operator action, compatibility resets, or removals.

## What counts as breaking

A change is breaking and requires a **major** bump when it does any of the following:

- Removes or renames API routes, fields, auth flows, or payload semantics used by existing clients.
- Requires non-backward-compatible schema/data migration.
- Removes or changes meaning of existing required env vars, compose wiring, or deployment contract behavior.
- Changes default runtime behavior in a way that can break existing production assumptions (for example protocol/auth/port compatibility).

## Upgrade compatibility promise

- Supported upgrades are defined in `docs/upgrades.md` and validated by `go run ./cmd/bitriver upgrade-plan --to <tag>`.
- N-1 minor hops only; no skipped majors.
- Major upgrades must include explicit migration and rollback guidance.

## Required release notes content

Every release must include:

1. **Upgrade notes**: supported source versions, required prechecks, and ordered commands.
2. **Breaking changes** section (or explicit `None`).
3. **Migration notes**: irreversible operations, data backfills, and estimated impact.
4. **Rollback notes**: safe/unsafe criteria and required backups.

Use `.github/RELEASE_NOTES_TEMPLATE.md` as the baseline format.
