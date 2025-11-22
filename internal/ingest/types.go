package ingest

import "context"

// BootParams captures the information required to start an ingest and
// transcoding pipeline for a channel.
//
// BootStream uses these fields to:
//   - Provision ingest endpoints keyed by ChannelID and StreamKey.
//   - Attach transcoder jobs to a logical SessionID.
//   - Optionally parameterize OME applications via Renditions (e.g. naming
//     or whitelisting particular renditions).
type BootParams struct {
	// ChannelID is the logical identifier for a stream/channel. It is used
	// as the stable key across SRS, OME, and transcoder resources.
	ChannelID string

	// SessionID uniquely identifies a streaming session within a channel.
	// This can be used to distinguish between multiple consecutive streams
	// on the same channel.
	SessionID string

	// StreamKey is the secret used by the encoder to authenticate and publish
	// to the ingest endpoint (e.g., RTMP).
	StreamKey string

	// Renditions is an optional list of rendition names passed to the
	// application adapter. It may be used by the origin (OME) to configure
	// which renditions are exposed for a particular channel.
	Renditions []string
}

// Rendition describes an output profile in the encoding ladder.
//
// A rendition typically corresponds to a particular resolution and bitrate.
// The ManifestURL field is set by downstream services (e.g., the transcoder)
// to indicate the playback URL for that rendition.
type Rendition struct {
	Name        string `json:"name"`
	ManifestURL string `json:"manifestUrl"`
	Bitrate     int    `json:"bitrate,omitempty"`
}

// BootResult summarizes the resources created by a successful BootStream call.
//
// It includes ingress endpoints, origin and playback URLs, and the set of
// transcoder jobs and renditions associated with the current session.
type BootResult struct {
	PrimaryIngest string      `json:"primaryIngest"`
	BackupIngest  string      `json:"backupIngest,omitempty"`
	OriginURL     string      `json:"originUrl"`
	PlaybackURL   string      `json:"playbackUrl"`
	Renditions    []Rendition `json:"renditions"`
	JobIDs        []string    `json:"jobIds"`
}

// UploadTranscodeParams describes the work required to convert a pre-uploaded
// asset into a VOD ladder (e.g., HLS).
//
// The ingest system assumes the asset is already accessible via SourceURL;
// it does not perform the upload itself.
type UploadTranscodeParams struct {
	// ChannelID associates the VOD asset with a logical channel.
	ChannelID string

	// UploadID is a unique identifier for the uploaded asset (e.g., a UUID).
	UploadID string

	// SourceURL is the location of the uploaded media that the transcoder
	// can read from (e.g., object storage URL).
	SourceURL string

	// Filename is an optional human-friendly name for the asset; it may be
	// used in logs or downstream storage.
	Filename string

	// Renditions describes the desired output ladder for the VOD asset.
	Renditions []Rendition
}

// UploadTranscodeResult summarizes the transcoding output for an upload.
//
// The playback URL points to the root manifest for the generated ladder,
// and Renditions reflects the effective outputs created by the transcoder.
type UploadTranscodeResult struct {
	PlaybackURL string      `json:"playbackUrl"`
	Renditions  []Rendition `json:"renditions"`
	JobID       string      `json:"jobId"`
}

// HealthStatus captures the availability/health of an external dependency
// involved in ingest orchestration (e.g. SRS, OME, transcoder).
type HealthStatus struct {
	// Component is the logical name of the system being reported on,
	// such as "srs", "ovenmediaengine", or "transcoder".
	Component string `json:"component"`

	// Status is a coarse-grained state indicator, such as "ok", "error",
	// "disabled", or "unknown".
	Status string `json:"status"`

	// Detail contains optional human-readable information about the status,
	// such as an error message or HTTP status code.
	Detail string `json:"detail,omitempty"`
}

// Controller provisions ingest resources, manages their lifecycle, and
// reports health for external dependencies involved in ingest.
//
// Implementations should be safe for concurrent use.
type Controller interface {
	// BootStream initializes a live ingest pipeline for the given params and
	// returns a summary of the created resources.
	BootStream(ctx context.Context, params BootParams) (BootResult, error)

	// ShutdownStream tears down the ingest pipeline and associated resources
	// for the given channel and session, best-effort stopping jobs and
	// cleaning up.
	ShutdownStream(ctx context.Context, channelID, sessionID string, jobIDs []string) error

	// HealthChecks returns a snapshot of the health of each ingest-related
	// dependency (e.g. SRS, OME, transcoder).
	HealthChecks(ctx context.Context) []HealthStatus

	// TranscodeUpload submits a pre-uploaded asset for VOD transcoding and
	// returns the resulting playback location and renditions.
	TranscodeUpload(ctx context.Context, params UploadTranscodeParams) (UploadTranscodeResult, error)
}

// NoopController is a Controller implementation used in tests and in
// deployments where ingest is not configured or intentionally disabled.
//
// It performs no external calls and returns benign defaults so that callers
// can safely invoke ingest operations without needing conditional logic.
type NoopController struct{}

// BootStream implements Controller by returning an empty BootResult.
//
// It does not provision any external resources and always returns a nil error.
func (NoopController) BootStream(ctx context.Context, params BootParams) (BootResult, error) {
	return BootResult{}, nil
}

// ShutdownStream implements Controller by performing no work and always
// returning nil, regardless of the provided identifiers or job IDs.
func (NoopController) ShutdownStream(ctx context.Context, channelID, sessionID string, jobIDs []string) error {
	return nil
}

// TranscodeUpload implements Controller by returning the SourceURL as the
// playback location to preserve caller expectations during tests and when
// ingest is disabled.
//
// No actual transcoding is performed.
func (NoopController) TranscodeUpload(ctx context.Context, params UploadTranscodeParams) (UploadTranscodeResult, error) {
	return UploadTranscodeResult{PlaybackURL: params.SourceURL}, nil
}

// HealthChecks reports that ingest orchestration is disabled by returning a
// single HealthStatus entry with component "ingest" and status "disabled".
func (NoopController) HealthChecks(ctx context.Context) []HealthStatus {
	return []HealthStatus{{Component: "ingest", Status: "disabled"}}
}
