# VOD upload current state

This describes the shipped upload path and its remaining limitations. The
[stream lifecycle](stream-lifecycle.md#upload-to-vod-publish-policy) defines
the upload-to-recording publish policy.

## Entry points

- `POST /api/uploads`: JSON registration or multipart file upload.
- `GET /api/uploads?channelId=...`: channel owner/admin upload list.
- `GET /api/uploads/{id}` and `DELETE /api/uploads/{id}`: owner/admin detail
  and deletion, including original-source cleanup on deletion.
- `GET /api/uploads/{id}/media?token=...`: token-authorized original source
  retrieval for transcoding. Treat the URL/token as a capability, not a public
  viewer link; the source may be removed after processing.

The creator route is `web/viewer/app/creator/uploads/[channelId]/page.tsx`.
`UploadManager` handles transport/status/publishing; the client helpers live in
`web/viewer/lib/viewer-api-upload.ts` and are exported by `viewer-api.ts`.
The server handlers are in `internal/api/uploads_handlers.go`.

## File validation and ownership

Multipart requests and file copying are bounded by the API upload limit, which
defaults to 512 MiB. The whole request includes multipart headers and fields,
so a file exactly at the limit can exceed the request budget. Oversized input
returns 413. Allowed extensions are `.mp4`, `.m4v`, `.mov` and `.webm`; declared
content type and inspected media headers must match the supported formats.
These checks are not a complete media-parser, malware or resource-abuse review.

Both JSON and multipart reject client-supplied `mediaPath`, `mediaToken`,
`sourceObjectKey` and `sourceObjectURL` with 400, including case/whitespace
variants. The server creates those references and capabilities for file uploads.
Other custom metadata and external `sourceUrl` registration remain supported.
Channel ownership is checked before persistence/enqueueing. Rejected multipart
requests remove their pending file, including when a later field is invalid.

This restriction prevents new forged references; it does not repair old rows.
Review existing source-reference provenance before public exposure, and do not
blindly delete suspicious rows through the API. Follow the
[upload security boundary](security.md#upload-source-ownership-boundary).

## Processing and publication

1. A selected file uses multipart plus XHR upload progress; registration without
   a file uses JSON. The API parses/validates fields and saves a pending file.
2. After ownership/input validation, the API creates the upload row. File bytes
   go to configured object storage when endpoint and bucket settings are present;
   otherwise they move into `UploadMediaDir` (default
   `os.TempDir()/bitriver-uploads`). See `internal/api/upload_source_storage.go`.
3. The server stores the source key and a generated media token. A configured
   object public endpoint can supply the source URL; otherwise the tokenized API
   media URL uses `BITRIVER_LIVE_UPLOAD_MEDIA_BASE_URL` when set, or the request
   origin with the configured trusted-forwarded-header policy.
4. `internal/service/uploads/processor.go` enqueues pending uploads, marks them
   `processing`, and calls the ingest controller/transcoder adapter. Defaults
   are two workers, a queue of 64 and a 30-minute attempt timeout.
5. Transient failures (network/context errors, HTTP 429 and 5xx) have a bounded
   three-attempt budget with exponential backoff. Permanent errors or exhausted
   retries mark the upload `failed`; persistence operations also have bounded
   retries. Retry state is stored with the upload.
6. Success calls `EnsureUploadRecording`, links `RecordingID`, and marks the
   upload `ready` with progress 100 and its playback URL. The JSON and Postgres
   stores create the recording **unpublished by default**.
7. The creator refreshes the upload list and explicitly publishes the linked
   recording. `/api/channels/{id}/vods` lists only published recordings.
   Upload readiness alone does not publish a VOD.

XHR progress measures transfer to the API, not transcoding completion. Backend
processing status/progress is visible after list reload or the UI's **Refresh**
action; `UploadManager` does not periodically poll processing status.

## Source cleanup and operator limitations

The server wires `UploadSourceCleaner` for both object and local sources.
Successful processing schedules immediate original-source cleanup; failed
processing schedules cleanup after 24 hours. Deleting an upload also cleans up
its original source. Linked recordings/transcoded output have a separate
lifecycle; source deletion is not a promise to remove every VOD artifact.

Cleanup runs on in-process timers tied to the processor context. A restart can
cancel delayed cleanup, and cleanup failures are logged rather than retried by
a durable retention scheduler. Do not treat the 24-hour delay as a guaranteed
storage-retention SLA across restarts. Local fallback sources can also disappear
when the API filesystem is replaced.

The transcoder must reach the chosen source URL. Configure the canonical media
base URL and trusted proxy policy correctly; verify object-storage and media
delivery access policies independently. Publishing controls public listings,
not authorization of every known object/CDN/playback URL. Private media access,
external-source/SSRF behavior and media-parser/resource-abuse acceptance remain
part of the broader [security review](security.md).
