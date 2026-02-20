# VOD upload current state (repo reality)

This document describes the **current implementation** for creator VOD uploads.

## Entry points

### API endpoints
- `POST /api/uploads` (JSON or multipart): `internal/api/uploads_handlers.go` (`Handler.Uploads`, `createUploadFromJSON`, `createUploadFromMultipart`)
- `GET /api/uploads?channelId=...`: list uploads for channel owner/admin (`Handler.Uploads`)
- `GET /api/uploads/{id}` and `DELETE /api/uploads/{id}`: upload detail/delete (`Handler.UploadByID`)
- `GET /api/uploads/{id}/media?token=...`: serves stored source file (`serveUploadMedia`), now reading object storage by `sourceObjectKey` when durable storage is enabled while preserving local-path fallback compatibility

### Viewer/admin UI that initiates uploads
- Creator page: `web/viewer/app/creator/uploads/[channelId]/page.tsx`
- Upload UI: `web/viewer/components/UploadManager.tsx`
- API client calls:
  - `createUpload(...)` uses multipart + `XMLHttpRequest` if a file is selected: `web/viewer/lib/viewer-api.ts`
  - `fetchChannelUploads(...)` for status list: `web/viewer/lib/viewer-api.ts`

## Actual upload flow

1. **User submits from creator UI**
   - `UploadManager` builds payload and calls `createUpload`.
   - If file exists, frontend sends `multipart/form-data` with `file`, `channelId`, optional fields, and `metadata[...]`.

2. **API receives file**
   - `createUploadFromMultipart` streams parts via `MultipartReader`.
   - `saveMultipartFile` writes the file part to a temp file in `UploadMediaDir` (fallback `os.TempDir()/bitriver-uploads`).

3. **Metadata + upload record creation**
   - Multipart source files are persisted to configured object storage when `BITRIVER_LIVE_OBJECT_ENDPOINT` + bucket settings are present; otherwise the local upload media directory fallback is used. Upload metadata stores `sourceObjectKey` (and `sourceObjectURL` when available).
   - `createUploadEntry` validates channel/ownership and calls `uploadsService().CreateUpload(...)`.
   - Backing persistence:
     - in-memory/file store: `internal/storage/vod.go` (`CreateUpload`)
     - postgres: `internal/storage/postgres_channels.go` (`CreateUpload`), table from `deploy/migrations/0001_initial.sql` (`uploads`)

4. **File storage location**
   - `persistUploadMedia` uploads source bytes to the configured object-storage backend and stores the durable key; when object storage is not configured, it falls back to local `uploadMediaDir()` storage.

5. **Source URL and tokenization**
   - `attachMediaToUpload` stores metadata keys:
     - `mediaPath`
     - `mediaToken`
     - `uploadedFilename`
     - `contentType`
     - `sourceUrl` = `/api/uploads/{id}/media?token=...` absolute URL built from request host/forwarded headers.

6. **Transcoding trigger**
   - `createUploadEntry` enqueues background worker (`UploadProcessor.Enqueue`) if processor configured.
   - `internal/service/uploads/processor.go`:
     - loads pending uploads
     - sets status to `processing`
     - calls `ingest.Controller.TranscodeUpload(...)` with `SourceURL`
     - on success sets status `ready`, `progress=100`, `playbackUrl`
     - on error sets status `failed` and error text

7. **Where transcoding happens**
   - `internal/ingest/http_controller.go` (`TranscodeUpload`) submits to transcoder adapter.
   - `cmd/transcoder/main.go` exposes `/v1/uploads` job API.

8. **How status is exposed to UI**
   - UI reads `GET /api/uploads?channelId=...` and renders `status`, `progress`, `error` (`UploadManager`).
   - Upload transport progress bar is frontend XHR upload progress only.
   - Backend processing progress/status appears after refresh/load.

## Recording/VOD linkage reality

- Public VOD list endpoint (`GET /api/channels/{id}/vods`) reads **published recordings**, not uploads directly: `internal/api/channels_directory_handlers.go`.
- It iterates uploads and only includes entries with `upload.RecordingID != nil`, then loads recording and requires `PublishedAt != nil`.
- Current upload processor updates upload status/playback URL but does **not create recordings** or set `RecordingID`.
- Therefore, uploaded items can be `ready` in uploads list but absent from channel `vods` list unless another path sets recording linkage.

## Concrete failure points / gaps observed

1. **No multipart request/file size limit on API ingest path**
   - `createUploadFromMultipart` + `saveMultipartFile` stream file with `io.Copy` and no max-bytes guard.

2. **No server-side media type/extension validation**
   - Accepted file content is persisted regardless of media type or extension.

3. **Source media stored only on API local disk**
   - Files are not moved to object storage in this flow.
   - If API filesystem is ephemeral/replaced, DB upload rows can outlive file availability.

4. **`sourceUrl` reachability depends on request host/header correctness**
   - `uploadMediaURL` builds absolute URL from incoming host/forwarded headers.
   - Misconfigured proxy/host can produce URLs unreachable by transcoder.

5. **No automatic retry when transcode itself fails**
   - Worker retries on DB update failure (`scheduleRetry`) but transcode errors call `failUpload` and stop.

6. **No built-in recording creation/linking from successful upload**
   - Processor marks upload `ready` but does not assign `RecordingID`.
   - Public `/api/channels/{id}/vods` depends on recording linkage + publish state.

7. **No explicit lifecycle cleanup for successful/failed upload source files**
   - Media file deletion is wired to upload delete, but no automatic post-processing retention/cleanup path is evident in upload flow.
