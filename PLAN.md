## Scope (current change)
- Address GitHub issue #1246 by extracting small pure upload helpers from the backend upload handler and viewer upload manager.
- Keep upload request/response behavior, visible UI text, and upload state transitions unchanged.
- Add focused helper tests near existing upload coverage so the refactor is locked before broader cleanup.
- Update cleanup tracking after targeted API and viewer tests pass.

## Assumptions
- The backend multipart upload flow should keep accepting the same fields, metadata naming, size parsing behavior, and file-derived defaults.
- The viewer upload manager should keep building the same payloads from form values and metadata rows.
- Helper exports from `UploadManager.tsx` are acceptable for focused unit tests if they remain small and UI-independent.
- This change should not alter deployment contract files or upload storage behavior.

## Risks
- Moving multipart field/default logic can subtly change trimming, ignored malformed sizes, or metadata filtering.
- Viewer payload helper extraction can change `undefined` versus empty-object metadata semantics, which API callers may observe.
- Touching viewer code means both focused Jest coverage and the normal repo verification gate should run.

## Test plan
- `go test ./internal/api -count=1 -timeout=120s`
- `npm.cmd --prefix web/viewer run test -- uploadManager.test.tsx`
- `go test ./... -count=1 -timeout=120s`
- `git diff --check`
- `./scripts/verify.sh --viewer`
