package ingest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"bitriver-live/internal/observability/metrics"
)

// flakyRoundTripper simulates a transient network failure on the first request
// and succeeds on subsequent requests. It is used to test health-check behavior
// when only one of the probes encounters a transient error.
type flakyRoundTripper struct {
	failureReturned atomic.Bool
	transport       http.RoundTripper
}

func (f *flakyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if !f.failureReturned.Load() {
		f.failureReturned.Store(true)
		return nil, errors.New("temporary DNS failure")
	}
	return f.transport.RoundTrip(req)
}

// TestHTTPControllerHealthChecksFailFastOnTransientError verifies that a
// transient error on one health check does not poison the other checks, and
// that the error detail is correctly surfaced for the failing component.
func TestHTTPControllerHealthChecksFailFastOnTransientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := &http.Client{
		Transport: &flakyRoundTripper{transport: http.DefaultTransport},
		Timeout:   time.Second,
	}

	controller := HTTPController{
		config: Config{
			SRSBaseURL:      server.URL,
			SRSToken:        "token",
			OMEBaseURL:      server.URL,
			OMEUsername:     "user",
			OMEPassword:     "pass",
			JobBaseURL:      server.URL,
			JobToken:        "token",
			HealthEndpoint:  "/healthz",
			HealthTimeout:   500 * time.Millisecond,
			HTTPClient:      client,
			LadderProfiles:  []Rendition{{Name: "720p", Bitrate: 1000}},
			HTTPMaxAttempts: 2,
		},
	}

	statuses := controller.HealthChecks(context.Background())

	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(statuses))
	}

	statusMap := make(map[string]HealthStatus)
	for _, status := range statuses {
		statusMap[status.Component] = status
	}

	srsStatus, ok := statusMap["srs"]
	if !ok {
		t.Fatalf("missing SRS status")
	}
	if srsStatus.Status != "error" {
		t.Fatalf("expected SRS status error, got %s", srsStatus.Status)
	}
	if !strings.Contains(srsStatus.Detail, "temporary DNS failure") {
		t.Fatalf("expected transient failure detail, got %s", srsStatus.Detail)
	}

	for _, component := range []string{"ovenmediaengine", "transcoder"} {
		status, ok := statusMap[component]
		if !ok {
			t.Fatalf("missing status for %s", component)
		}
		if status.Status != "ok" {
			t.Fatalf("expected %s status ok, got %s", component, status.Status)
		}
	}
}

// ---- Fake adapters for controller tests ----

type fakeChannelAdapter struct {
	createPrimary string
	createBackup  string
	createErr     error

	deleteErr error

	lastCreateChannelID string
	lastCreateStreamKey string
	lastDeleteChannelID string
}

func (f *fakeChannelAdapter) CreateChannel(ctx context.Context, channelID, streamKey string) (string, string, error) {
	f.lastCreateChannelID = channelID
	f.lastCreateStreamKey = streamKey
	if f.createErr != nil {
		return "", "", f.createErr
	}
	return f.createPrimary, f.createBackup, nil
}

func (f *fakeChannelAdapter) DeleteChannel(ctx context.Context, channelID string) error {
	f.lastDeleteChannelID = channelID
	return f.deleteErr
}

type fakeApplicationAdapter struct {
	origin    string
	playback  string
	createErr error
	deleteErr error

	lastCreateChannelID  string
	lastCreateRenditions []string
	lastDeleteChannelID  string
}

func (f *fakeApplicationAdapter) CreateApplication(ctx context.Context, channelID string, renditions []string) (string, string, error) {
	f.lastCreateChannelID = channelID
	f.lastCreateRenditions = append([]string{}, renditions...)
	if f.createErr != nil {
		return "", "", f.createErr
	}
	return f.origin, f.playback, nil
}

func (f *fakeApplicationAdapter) DeleteApplication(ctx context.Context, channelID string) error {
	f.lastDeleteChannelID = channelID
	return f.deleteErr
}

type fakeTranscoderAdapter struct {
	startJobErr    error
	stopJobErr     error
	startUploadErr error

	startJobIDs        []string
	startJobRenditions []Rendition
	lastStartChannelID string
	lastStartSessionID string
	lastStartOriginURL string

	stopJobIDs []string

	lastUploadReq uploadJobRequest
	uploadResult  uploadJobResult
}

func (f *fakeTranscoderAdapter) StartJobs(ctx context.Context, channelID, sessionID, originURL string, ladder []Rendition) ([]string, []Rendition, error) {
	f.lastStartChannelID = channelID
	f.lastStartSessionID = sessionID
	f.lastStartOriginURL = originURL
	f.startJobRenditions = cloneRenditions(ladder)
	if f.startJobErr != nil {
		return nil, nil, f.startJobErr
	}
	return append([]string{}, f.startJobIDs...), cloneRenditions(f.startJobRenditions), nil
}

func (f *fakeTranscoderAdapter) StopJob(ctx context.Context, jobID string) error {
	f.stopJobIDs = append(f.stopJobIDs, jobID)
	return f.stopJobErr
}

func (f *fakeTranscoderAdapter) StartUpload(ctx context.Context, req uploadJobRequest) (uploadJobResult, error) {
	f.lastUploadReq = req
	if f.startUploadErr != nil {
		return uploadJobResult{}, f.startUploadErr
	}
	return f.uploadResult, nil
}

// ---- BootStream tests ----

// TestHTTPControllerBootStreamSuccess verifies the happy path for BootStream:
// channel created, application created, jobs started, and a complete BootResult
// is returned.
func TestHTTPControllerBootStreamSuccess(t *testing.T) {
	ch := &fakeChannelAdapter{
		createPrimary: "rtmp://primary",
		createBackup:  "rtmp://backup",
	}
	app := &fakeApplicationAdapter{
		origin:   "http://origin",
		playback: "https://playback",
	}
	tr := &fakeTranscoderAdapter{
		startJobIDs: []string{"job-1", "job-2"},
		startJobRenditions: []Rendition{
			{Name: "720p", Bitrate: 2500},
		},
	}

	controller := HTTPController{
		config: Config{
			LadderProfiles: []Rendition{{Name: "720p", Bitrate: 2500}},
		},
		channels:     ch,
		applications: app,
		transcoder:   tr,
	}

	params := BootParams{
		ChannelID:  "channel-123",
		StreamKey:  "stream-key",
		SessionID:  "session-abc",
		Renditions: []string{"1080p"},
	}

	result, err := controller.BootStream(context.Background(), params)
	if err != nil {
		t.Fatalf("BootStream: %v", err)
	}

	if result.PrimaryIngest != "rtmp://primary" || result.BackupIngest != "rtmp://backup" {
		t.Fatalf("unexpected ingest endpoints: %+v", result)
	}
	if result.OriginURL != "http://origin" || result.PlaybackURL != "https://playback" {
		t.Fatalf("unexpected origin/playback: %+v", result)
	}
	if len(result.JobIDs) != 2 {
		t.Fatalf("expected 2 jobIDs, got %d", len(result.JobIDs))
	}
	if ch.lastCreateChannelID != "channel-123" || ch.lastCreateStreamKey != "stream-key" {
		t.Fatalf("unexpected channel create args: %+v", ch)
	}
	if app.lastCreateChannelID != "channel-123" {
		t.Fatalf("unexpected app create channelID: %s", app.lastCreateChannelID)
	}
	// Ensure StartJobs saw the LadderProfiles from config.
	if tr.lastStartChannelID != "channel-123" || tr.lastStartSessionID != "session-abc" || tr.lastStartOriginURL != "http://origin" {
		t.Fatalf("unexpected transcoder args: channel=%s session=%s origin=%s",
			tr.lastStartChannelID, tr.lastStartSessionID, tr.lastStartOriginURL)
	}
	if len(tr.startJobRenditions) != 1 || tr.startJobRenditions[0].Name != "720p" {
		t.Fatalf("unexpected rendition ladder: %+v", tr.startJobRenditions)
	}
}

// TestHTTPControllerBootStreamRollsBackOnAppFailure verifies that if the
// OME application creation fails, the previously created SRS channel is
// deleted as part of rollback.
func TestHTTPControllerBootStreamRollsBackOnAppFailure(t *testing.T) {
	ch := &fakeChannelAdapter{
		createPrimary: "rtmp://primary",
		createBackup:  "rtmp://backup",
	}
	app := &fakeApplicationAdapter{
		createErr: errors.New("OME down"),
	}
	tr := &fakeTranscoderAdapter{}

	controller := HTTPController{
		config:       Config{},
		channels:     ch,
		applications: app,
		transcoder:   tr,
	}

	params := BootParams{
		ChannelID: "channel-123",
		StreamKey: "stream-key",
	}

	_, err := controller.BootStream(context.Background(), params)
	if err == nil {
		t.Fatal("expected BootStream error, got nil")
	}
	if ch.lastDeleteChannelID != "channel-123" {
		t.Fatalf("expected channel delete for rollback, got %q", ch.lastDeleteChannelID)
	}
	if app.lastDeleteChannelID != "" {
		t.Fatalf("did not expect app delete for app failure rollback, got %q", app.lastDeleteChannelID)
	}
}

// TestHTTPControllerBootStreamRollsBackOnTranscoderFailure verifies that if
// transcoder job startup fails, both the OME application and SRS channel
// are deleted as part of rollback.
func TestHTTPControllerBootStreamRollsBackOnTranscoderFailure(t *testing.T) {
	ch := &fakeChannelAdapter{
		createPrimary: "rtmp://primary",
		createBackup:  "rtmp://backup",
	}
	app := &fakeApplicationAdapter{
		origin:   "http://origin",
		playback: "https://playback",
	}
	tr := &fakeTranscoderAdapter{
		startJobErr: errors.New("transcoder unreachable"),
	}

	controller := HTTPController{
		config:       Config{},
		channels:     ch,
		applications: app,
		transcoder:   tr,
	}

	params := BootParams{
		ChannelID: "channel-123",
		StreamKey: "stream-key",
		SessionID: "session-abc",
	}

	_, err := controller.BootStream(context.Background(), params)
	if err == nil {
		t.Fatal("expected BootStream error, got nil")
	}
	if app.lastDeleteChannelID != "channel-123" {
		t.Fatalf("expected app delete for rollback, got %q", app.lastDeleteChannelID)
	}
	if ch.lastDeleteChannelID != "channel-123" {
		t.Fatalf("expected channel delete for rollback, got %q", ch.lastDeleteChannelID)
	}
}

func TestHTTPControllerBootStreamMetricsRecorded(t *testing.T) {
	metrics.Default().Reset()
	t.Cleanup(metrics.Default().Reset)

	controller := HTTPController{
		channels:     &fakeChannelAdapter{createPrimary: "rtmp://primary", createBackup: "rtmp://backup"},
		applications: &fakeApplicationAdapter{origin: "http://origin", playback: "https://play"},
		transcoder:   &fakeTranscoderAdapter{startJobIDs: []string{"job-1"}},
	}

	params := BootParams{ChannelID: "channel-1", StreamKey: "key", SessionID: "session-1"}
	if _, err := controller.BootStream(context.Background(), params); err != nil {
		t.Fatalf("BootStream: %v", err)
	}

	attempts, failures := metrics.Default().IngestCounts()
	if attempts["boot_stream"] != 1 {
		t.Fatalf("expected one boot attempt, got %d", attempts["boot_stream"])
	}
	if failures["boot_stream"] != 0 {
		t.Fatalf("expected zero boot failures, got %d", failures["boot_stream"])
	}
}

func TestHTTPControllerBootStreamMetricsFailure(t *testing.T) {
	metrics.Default().Reset()
	t.Cleanup(metrics.Default().Reset)

	controller := HTTPController{
		channels: &fakeChannelAdapter{createErr: errors.New("boom")},
	}

	params := BootParams{ChannelID: "channel-1", StreamKey: "key", SessionID: "session-1"}
	if _, err := controller.BootStream(context.Background(), params); err == nil {
		t.Fatal("expected BootStream error")
	}

	attempts, failures := metrics.Default().IngestCounts()
	if attempts["boot_stream"] != 1 {
		t.Fatalf("expected one boot attempt, got %d", attempts["boot_stream"])
	}
	if failures["boot_stream"] != 1 {
		t.Fatalf("expected one boot failure, got %d", failures["boot_stream"])
	}
}

// ---- ShutdownStream tests ----

// TestHTTPControllerShutdownStreamAggregatesErrors verifies that
// ShutdownStream aggregates errors from the underlying adapters and
// returns a combined error string.
func TestHTTPControllerShutdownStreamAggregatesErrors(t *testing.T) {
	ch := &fakeChannelAdapter{deleteErr: errors.New("channel delete failed")}
	app := &fakeApplicationAdapter{deleteErr: errors.New("app delete failed")}
	tr := &fakeTranscoderAdapter{stopJobErr: errors.New("stop failed")}

	controller := HTTPController{
		config:       Config{},
		channels:     ch,
		applications: app,
		transcoder:   tr,
	}

	err := controller.ShutdownStream(context.Background(), "channel-123", "session-abc", []string{"job-1", "job-2"})
	if err == nil {
		t.Fatal("expected ShutdownStream error, got nil")
	}
	msg := err.Error()
	for _, substr := range []string{"stop job job-1", "stop job job-2", "delete OME app", "delete SRS channel"} {
		if !strings.Contains(msg, substr) {
			t.Fatalf("expected error message to contain %q, got %q", substr, msg)
		}
	}
	if len(tr.stopJobIDs) != 2 {
		t.Fatalf("expected 2 jobs stopped, got %d", len(tr.stopJobIDs))
	}
}

func TestHTTPControllerShutdownMetricsRecorded(t *testing.T) {
	metrics.Default().Reset()
	t.Cleanup(metrics.Default().Reset)

	controller := HTTPController{
		channels:     &fakeChannelAdapter{},
		applications: &fakeApplicationAdapter{},
		transcoder:   &fakeTranscoderAdapter{},
	}

	if err := controller.ShutdownStream(context.Background(), "channel-123", "session-abc", []string{"job-1"}); err != nil {
		t.Fatalf("ShutdownStream: %v", err)
	}

	attempts, failures := metrics.Default().IngestCounts()
	if attempts["shutdown_stream"] != 1 {
		t.Fatalf("expected one shutdown attempt, got %d", attempts["shutdown_stream"])
	}
	if failures["shutdown_stream"] != 0 {
		t.Fatalf("expected zero shutdown failures, got %d", failures["shutdown_stream"])
	}
}

func TestHTTPControllerShutdownMetricsFailure(t *testing.T) {
	metrics.Default().Reset()
	t.Cleanup(metrics.Default().Reset)

	controller := HTTPController{
		channels:     &fakeChannelAdapter{},
		applications: &fakeApplicationAdapter{},
		transcoder:   &fakeTranscoderAdapter{stopJobErr: errors.New("stop failed")},
	}

	if err := controller.ShutdownStream(context.Background(), "channel-123", "session-abc", []string{"job-1"}); err == nil {
		t.Fatal("expected ShutdownStream error")
	}

	attempts, failures := metrics.Default().IngestCounts()
	if attempts["shutdown_stream"] != 1 {
		t.Fatalf("expected one shutdown attempt, got %d", attempts["shutdown_stream"])
	}
	if failures["shutdown_stream"] != 1 {
		t.Fatalf("expected one shutdown failure, got %d", failures["shutdown_stream"])
	}
}

// ---- TranscodeUpload tests ----

// TestHTTPControllerTranscodeUploadValidatesInputs verifies that
// TranscodeUpload rejects missing identifiers or source URL.
func TestHTTPControllerTranscodeUploadValidatesInputs(t *testing.T) {
	controller := HTTPController{config: Config{}}

	_, err := controller.TranscodeUpload(context.Background(), UploadTranscodeParams{})
	if err == nil || !strings.Contains(err.Error(), "channelID and uploadID are required") {
		t.Fatalf("expected missing IDs error, got %v", err)
	}

	_, err = controller.TranscodeUpload(context.Background(), UploadTranscodeParams{
		ChannelID: "channel-123",
		UploadID:  "upload-abc",
	})
	if err == nil || !strings.Contains(err.Error(), "sourceURL is required") {
		t.Fatalf("expected missing sourceURL error, got %v", err)
	}
}

// TestHTTPControllerTranscodeUploadSuccess verifies the happy path for
// TranscodeUpload and ensures the input renditions slice is not mutated.
func TestHTTPControllerTranscodeUploadSuccess(t *testing.T) {
	tr := &fakeTranscoderAdapter{
		uploadResult: uploadJobResult{
			JobID:       "job-upload",
			PlaybackURL: "https://cdn/hls/index.m3u8",
			Renditions: []Rendition{
				{Name: "720p", ManifestURL: "https://cdn/hls/720p.m3u8", Bitrate: 3000},
			},
		},
	}

	controller := HTTPController{
		config:     Config{},
		transcoder: tr,
	}

	inputRenditions := []Rendition{
		{Name: "720p", Bitrate: 3000},
	}

	result, err := controller.TranscodeUpload(context.Background(), UploadTranscodeParams{
		ChannelID:  "channel-123",
		UploadID:   "upload-abc",
		SourceURL:  "https://cdn/source.mp4",
		Filename:   "source.mp4",
		Renditions: inputRenditions,
	})
	if err != nil {
		t.Fatalf("TranscodeUpload: %v", err)
	}

	if result.JobID != "job-upload" || result.PlaybackURL != "https://cdn/hls/index.m3u8" {
		t.Fatalf("unexpected upload result: %+v", result)
	}
	if len(result.Renditions) != 1 || result.Renditions[0].ManifestURL != "https://cdn/hls/720p.m3u8" {
		t.Fatalf("unexpected renditions: %+v", result.Renditions)
	}

	// Ensure controller did not mutate the caller's renditions slice.
	if inputRenditions[0].ManifestURL != "" {
		t.Fatalf("expected input renditions to remain unchanged, got manifest %q", inputRenditions[0].ManifestURL)
	}

	// Ensure StartUpload saw the channel/upload/source information.
	if tr.lastUploadReq.ChannelID != "channel-123" ||
		tr.lastUploadReq.UploadID != "upload-abc" ||
		tr.lastUploadReq.SourceURL != "https://cdn/source.mp4" {
		t.Fatalf("unexpected upload request: %+v", tr.lastUploadReq)
	}
}

func TestHTTPControllerTranscodeUploadMetricsRecorded(t *testing.T) {
	metrics.Default().Reset()
	t.Cleanup(metrics.Default().Reset)

	tr := &fakeTranscoderAdapter{
		uploadResult: uploadJobResult{JobID: "job-upload"},
	}
	controller := HTTPController{transcoder: tr}

	if _, err := controller.TranscodeUpload(context.Background(), UploadTranscodeParams{
		ChannelID: "channel-123",
		UploadID:  "upload-abc",
		SourceURL: "https://cdn/source.mp4",
	}); err != nil {
		t.Fatalf("TranscodeUpload: %v", err)
	}

	attempts, failures := metrics.Default().IngestCounts()
	if attempts["upload_transcode"] != 1 {
		t.Fatalf("expected one upload attempt, got %d", attempts["upload_transcode"])
	}
	if failures["upload_transcode"] != 0 {
		t.Fatalf("expected zero upload failures, got %d", failures["upload_transcode"])
	}
}

func TestHTTPControllerTranscodeUploadMetricsFailure(t *testing.T) {
	metrics.Default().Reset()
	t.Cleanup(metrics.Default().Reset)

	tr := &fakeTranscoderAdapter{startUploadErr: errors.New("upload failed")}
	controller := HTTPController{transcoder: tr}

	if _, err := controller.TranscodeUpload(context.Background(), UploadTranscodeParams{
		ChannelID: "channel-123",
		UploadID:  "upload-abc",
		SourceURL: "https://cdn/source.mp4",
	}); err == nil {
		t.Fatal("expected TranscodeUpload error")
	}

	attempts, failures := metrics.Default().IngestCounts()
	if attempts["upload_transcode"] != 1 {
		t.Fatalf("expected one upload attempt, got %d", attempts["upload_transcode"])
	}
	if failures["upload_transcode"] != 1 {
		t.Fatalf("expected one upload failure, got %d", failures["upload_transcode"])
	}
}
