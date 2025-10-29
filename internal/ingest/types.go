package ingest

import "context"

// BootParams captures the information required to start an ingest and transcode
// pipeline for a channel.
type BootParams struct {
	ChannelID  string
	SessionID  string
	StreamKey  string
	Renditions []string
}

// Rendition describes a ladder output that will be produced by the encoder.
type Rendition struct {
	Name        string `json:"name"`
	ManifestURL string `json:"manifestUrl"`
	Bitrate     int    `json:"bitrate,omitempty"`
}

// BootResult summarises the resources created by a successful ingest boot.
type BootResult struct {
	PrimaryIngest string      `json:"primaryIngest"`
	BackupIngest  string      `json:"backupIngest,omitempty"`
	OriginURL     string      `json:"originUrl"`
	PlaybackURL   string      `json:"playbackUrl"`
	Renditions    []Rendition `json:"renditions"`
	JobIDs        []string    `json:"jobIds"`
}

// UploadTranscodeParams describes the work required to convert an uploaded
// asset into an HLS ladder.
type UploadTranscodeParams struct {
	ChannelID  string
	UploadID   string
	SourceURL  string
	Filename   string
	Renditions []Rendition
}

// UploadTranscodeResult summarises the transcoding output for an upload.
type UploadTranscodeResult struct {
	PlaybackURL string
	Renditions  []Rendition
	JobID       string
}

// HealthStatus captures the availability of an external dependency.
type HealthStatus struct {
	Component string `json:"component"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
}

// Controller provisions ingest resources and reports health.
type Controller interface {
	BootStream(ctx context.Context, params BootParams) (BootResult, error)
	ShutdownStream(ctx context.Context, channelID, sessionID string, jobIDs []string) error
	HealthChecks(ctx context.Context) []HealthStatus
	TranscodeUpload(ctx context.Context, params UploadTranscodeParams) (UploadTranscodeResult, error)
}

// NoopController is used in tests and when ingest is not configured.
type NoopController struct{}

// BootStream implements Controller by returning an empty BootResult.
func (NoopController) BootStream(ctx context.Context, params BootParams) (BootResult, error) {
	return BootResult{}, nil
}

// ShutdownStream implements Controller by performing no work.
func (NoopController) ShutdownStream(ctx context.Context, channelID, sessionID string, jobIDs []string) error {
	return nil
}

// TranscodeUpload implements Controller by returning the source URL as the
// playback location to preserve caller expectations during tests.
func (NoopController) TranscodeUpload(ctx context.Context, params UploadTranscodeParams) (UploadTranscodeResult, error) {
	return UploadTranscodeResult{PlaybackURL: params.SourceURL}, nil
}

// HealthChecks reports that ingest orchestration is disabled.
func (NoopController) HealthChecks(ctx context.Context) []HealthStatus {
	return []HealthStatus{{Component: "ingest", Status: "disabled"}}
}
