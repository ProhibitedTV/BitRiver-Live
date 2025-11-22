// Package ingest provides the orchestration layer for creating, managing,
// and tearing down live streaming ingest pipelines.
//
// Overview
//
// The ingest package coordinates three major subsystems involved in turning
// a user’s stream key into a functional live or VOD workflow:
//
//   1. SRS (or similar) channel controller
//      - Provisions ingest endpoints (RTMP primary/backup).
//      - Secured via bearer-token authentication.
//      - Exposed through the channelAdapter interface.
//
//   2. OvenMediaEngine (OME) application API
//      - Creates per-channel applications containing playback endpoints and
//        origin pull URLs consumed by the transcoding layer.
//      - Authenticated with HTTP Basic Auth.
//      - Exposed through the applicationAdapter interface.
//
//   3. FFmpeg-based transcoding service
//      - Starts and stops transcoding jobs for live workflows.
//      - Starts VOD processing jobs for uploaded source files.
//      - Authenticated via bearer token.
//      - Exposed through the transcoderAdapter interface.
//
// High-Level Workflow
//
// A typical live ingest operation (triggered when a user begins streaming)
// proceeds through the following steps:
//
//   - CreateChannel:
//       The channelAdapter talks to the SRS controller to provision one or
//       two RTMP ingest URLs (primary and backup). These are returned to the
//       caller as the endpoints the client encoder should publish to.
//
//   - CreateApplication:
//       The applicationAdapter contacts OvenMediaEngine (OME) and creates a
//       per-channel application. This returns:
//         • OriginURL: Pull URL for the transcoder.
//         • PlaybackURL: HLS or CMAF URL for viewers.
//
//   - StartJobs:
//       The transcoderAdapter starts transcoding jobs using the OriginURL,
//       applying the requested rendition ladder. It returns job IDs and the
//       effective renditions used.
//
// When a user ends their stream, the reverse happens:
//
//   - StopJob
//   - DeleteApplication
//   - DeleteChannel
//
// VOD uploads follow a similar pattern via StartUpload on the
// transcoderAdapter.
//
// Retry Semantics
//
// All HTTP adapters share common retry behavior implemented in doWithRetry:
//
//   - Retries:
//       * Transient network errors (client.Do failures).
//       * HTTP 5xx responses.
//       * HTTP 429 (Too Many Requests).
//
//   - No retries:
//       * Any 4xx response except 429 (e.g. 400/401/403/404).
//         These are treated as permanent failures.
//
// Each adapter constructor allows callers to configure maxAttempts and
// retryInterval. When no custom http.Client is supplied, a client with a
// default timeout is created to avoid indefinite hangs.
//
// Defensive Copying
//
// Input renditions and payload slices are defensively copied by adapters to
// avoid aliasing and mutation by callers after an operation begins.
//
// Testing
//
// The ingest package includes comprehensive test coverage for:
//
//   - Channel creation/deletion.
//   - Application creation/deletion.
//   - Job creation/stop.
//   - Upload/VOD job creation.
//   - Retry behavior for 5xx, 429, and 4xx status codes.
//
// This ensures that the adapters conform to expected behavior under real
// production-like conditions.
//
// Usage
//
// External services should not use this package directly. It is intended to
// be consumed internally by the BitRiver ingest coordinator, which performs
// higher-level orchestration and lifecycle management across the entire
// ingestion pipeline.
package ingest
