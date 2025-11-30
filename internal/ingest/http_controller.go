package ingest

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"bitriver-live/internal/observability/metrics"
)

// HTTPController orchestrates live ingest and VOD processing using
// HTTP-based adapters for channels (SRS), applications (OME), and
// transcoding jobs.
//
// It is intended to be the high-level coordination layer used by API
// handlers or other services that need to:
//   - Boot a live ingest pipeline for a given channel.
//   - Tear down that pipeline when the stream ends.
//   - Submit VOD uploads for transcoding.
//   - Run health checks against the underlying services.
//
// HTTPController is typically configured once at process startup and then
// used concurrently; configuration methods (such as SetLogger) should be
// called before concurrent use.
type HTTPController struct {
	config        Config
	channels      channelAdapter
	applications  applicationAdapter
	transcoder    transcoderAdapter
	logger        *slog.Logger
	retryAttempts int
	retryInterval time.Duration
}

// ensureAdapters ensures HTTP clients, logger, retry settings, and the
// three HTTP adapters are initialized before use.
//
// This method is idempotent and may be called multiple times.
func (c *HTTPController) ensureAdapters() {
	if c.config.HTTPClient == nil {
		// Use the same default timeout as the adapters when we create
		// a client here.
		c.config.HTTPClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	if c.config.HealthTimeout <= 0 {
		// Health checks should be fast; if no timeout is configured,
		// use a small default specific to health probes.
		c.config.HealthTimeout = 2 * time.Second
	}
	c.ensureLogger()
	if c.channels == nil {
		c.channels = newHTTPChannelAdapter(
			c.config.SRSBaseURL,
			c.config.SRSToken,
			c.config.HTTPClient,
			c.logger,
			c.retryAttempts,
			c.retryInterval,
		)
	}
	if c.applications == nil {
		c.applications = newHTTPApplicationAdapter(
			c.config.OMEBaseURL,
			c.config.OMEUsername,
			c.config.OMEPassword,
			c.config.HTTPClient,
			c.logger,
			c.retryAttempts,
			c.retryInterval,
		)
	}
	if c.transcoder == nil {
		c.transcoder = newHTTPTranscoderAdapter(
			c.config.JobBaseURL,
			c.config.JobToken,
			c.config.HTTPClient,
			c.logger,
			c.retryAttempts,
			c.retryInterval,
		)
	}
}

// ensureLogger ensures that the controller has a logger and that retry
// settings are initialized from configuration or sensible defaults.
func (c *HTTPController) ensureLogger() {
	if c.logger == nil {
		c.logger = slog.Default()
	}
	if c.retryAttempts <= 0 {
		c.retryAttempts = c.config.HTTPMaxAttempts
		if c.retryAttempts <= 0 {
			c.retryAttempts = defaultMaxAttempts
		}
	}
	if c.retryInterval == 0 {
		if c.config.HTTPRetryInterval > 0 {
			c.retryInterval = c.config.HTTPRetryInterval
		} else {
			c.retryInterval = defaultRetryBackoff
		}
	}
}

// SetLogger installs a structured logger for ingest orchestration.
//
// If logger is nil, the current logger is left unchanged. This method
// should be called before the controller is used concurrently.
func (c *HTTPController) SetLogger(logger *slog.Logger) {
	if logger == nil {
		return
	}
	c.logger = logger
}

// BootStream initializes a complete ingest pipeline for a live stream.
//
// The operation:
//  1. Provisions a channel in SRS (primary/backup ingest endpoints).
//  2. Creates an OME application (origin + playback URLs).
//  3. Starts transcoding jobs using the configured rendition ladder.
//
// On failure, BootStream attempts to roll back previously created
// resources (e.g., deleting the OME application if transcoder startup
// fails).
//
// Callers should provide a context with an appropriate deadline to bound
// the overall latency of the operation.
func (c *HTTPController) BootStream(ctx context.Context, params BootParams) (BootResult, error) {
	metrics.ObserveIngestAttempt("boot_stream")
	if strings.TrimSpace(params.ChannelID) == "" || strings.TrimSpace(params.StreamKey) == "" {
		metrics.ObserveIngestFailure("boot_stream")
		return BootResult{}, fmt.Errorf("channelID and streamKey are required")
	}

	c.ensureAdapters()

	c.logger.Info("booting ingest pipeline",
		"channel_id", params.ChannelID,
		"session_id", params.SessionID,
	)

	primary, backup, err := c.channels.CreateChannel(ctx, params.ChannelID, params.StreamKey)
	if err != nil {
		c.logger.Error("failed to create SRS channel",
			"channel_id", params.ChannelID,
			"error", err,
		)
		metrics.ObserveIngestFailure("boot_stream")
		return BootResult{}, err
	}

	origin, playback, err := c.applications.CreateApplication(ctx, params.ChannelID, params.Renditions)
	if err != nil {
		c.logger.Error("failed to create OME application",
			"channel_id", params.ChannelID,
			"error", err,
		)
		_ = c.channels.DeleteChannel(ctx, params.ChannelID)
		metrics.ObserveIngestFailure("boot_stream")
		return BootResult{}, err
	}

	jobIDs, renditions, err := c.transcoder.StartJobs(ctx, params.ChannelID, params.SessionID, origin, c.config.LadderProfiles)
	if err != nil {
		c.logger.Error("failed to start transcoder jobs",
			"channel_id", params.ChannelID,
			"session_id", params.SessionID,
			"error", err,
		)
		_ = c.applications.DeleteApplication(ctx, params.ChannelID)
		_ = c.channels.DeleteChannel(ctx, params.ChannelID)
		metrics.ObserveIngestFailure("boot_stream")
		return BootResult{}, err
	}

	c.logger.Info("ingest pipeline ready",
		"channel_id", params.ChannelID,
		"session_id", params.SessionID,
		"jobs", len(jobIDs),
	)

	return BootResult{
		PrimaryIngest: primary,
		BackupIngest:  backup,
		OriginURL:     origin,
		PlaybackURL:   playback,
		Renditions:    renditions,
		JobIDs:        jobIDs,
	}, nil
}

// ShutdownStream tears down an ingest pipeline that was previously
// initialized with BootStream.
//
// It best-effort stops each transcoder job, removes the OME application,
// and deletes the SRS channel. All errors are aggregated and returned
// as a single error if any step fails.
func (c *HTTPController) ShutdownStream(ctx context.Context, channelID, sessionID string, jobIDs []string) error {
	metrics.ObserveIngestAttempt("shutdown_stream")
	c.ensureAdapters()

	c.logger.Info("tearing down ingest pipeline",
		"channel_id", channelID,
		"session_id", sessionID,
		"jobs", len(jobIDs),
	)

	var errs []string

	for _, jobID := range jobIDs {
		if err := c.transcoder.StopJob(ctx, jobID); err != nil {
			c.logger.Error("failed to stop transcoder job",
				"job_id", jobID,
				"error", err,
			)
			errs = append(errs, fmt.Sprintf("stop job %s: %v", jobID, err))
		}
	}

	if err := c.applications.DeleteApplication(ctx, channelID); err != nil {
		c.logger.Error("failed to delete OME application",
			"channel_id", channelID,
			"error", err,
		)
		errs = append(errs, fmt.Sprintf("delete OME app: %v", err))
	}

	if err := c.channels.DeleteChannel(ctx, channelID); err != nil {
		c.logger.Error("failed to delete SRS channel",
			"channel_id", channelID,
			"error", err,
		)
		errs = append(errs, fmt.Sprintf("delete SRS channel: %v", err))
	}

	if len(errs) > 0 {
		metrics.ObserveIngestFailure("shutdown_stream")
		return fmt.Errorf(strings.Join(errs, "; "))
	}

	c.logger.Info("ingest pipeline removed",
		"channel_id", channelID,
		"session_id", sessionID,
	)
	return nil
}

// TranscodeUpload submits a stored media file for HLS (or similar) VOD
// transcoding via the configured transcoder adapter.
//
// The operation does not perform any upload itself; it assumes the source
// media is already accessible at SourceURL. On success, it returns a
// playback URL and the effective renditions used by the transcoder.
func (c *HTTPController) TranscodeUpload(ctx context.Context, params UploadTranscodeParams) (UploadTranscodeResult, error) {
	metrics.ObserveIngestAttempt("upload_transcode")
	if strings.TrimSpace(params.ChannelID) == "" || strings.TrimSpace(params.UploadID) == "" {
		metrics.ObserveIngestFailure("upload_transcode")
		return UploadTranscodeResult{}, fmt.Errorf("channelID and uploadID are required")
	}
	source := strings.TrimSpace(params.SourceURL)
	if source == "" {
		metrics.ObserveIngestFailure("upload_transcode")
		return UploadTranscodeResult{}, fmt.Errorf("sourceURL is required")
	}

	c.ensureAdapters()

	c.logger.Info("starting upload transcode",
		"channel_id", params.ChannelID,
		"upload_id", params.UploadID,
	)

	result, err := c.transcoder.StartUpload(ctx, uploadJobRequest{
		ChannelID:  params.ChannelID,
		UploadID:   params.UploadID,
		SourceURL:  source,
		Filename:   strings.TrimSpace(params.Filename),
		Renditions: cloneRenditions(params.Renditions),
	})
	if err != nil {
		c.logger.Error("failed to start upload transcode",
			"channel_id", params.ChannelID,
			"upload_id", params.UploadID,
			"error", err,
		)
		metrics.ObserveIngestFailure("upload_transcode")
		return UploadTranscodeResult{}, err
	}

	c.logger.Info("upload transcode submitted",
		"channel_id", params.ChannelID,
		"upload_id", params.UploadID,
		"job_id", result.JobID,
	)

	return UploadTranscodeResult{
		PlaybackURL: result.PlaybackURL,
		Renditions:  cloneRenditions(result.Renditions),
		JobID:       result.JobID,
	}, nil
}

// HealthChecks performs health probes against each of the underlying HTTP
// services used by the ingest subsystem:
//
//   - SRS channel controller.
//   - OvenMediaEngine application API.
//   - Transcoder job service.
//
// Each service is probed at:
//
//	<baseURL><HealthEndpoint>
//
// The configured HTTPClient and HealthTimeout are used for each request.
// If a base URL is not configured, the corresponding health status is
// reported as "unknown".
func (c *HTTPController) HealthChecks(ctx context.Context) []HealthStatus {
	c.ensureAdapters()

	type service struct {
		name string
		base string
		auth func(*http.Request)
	}

	services := []service{
		{
			name: "srs",
			base: c.config.SRSBaseURL,
			auth: bearerAuth(c.config.SRSToken),
		},
		{
			name: "ovenmediaengine",
			base: c.config.OMEBaseURL,
			auth: basicAuth(c.config.OMEUsername, c.config.OMEPassword),
		},
		{
			name: "transcoder",
			base: c.config.JobBaseURL,
			auth: bearerAuth(c.config.JobToken),
		},
	}

	statuses := make([]HealthStatus, 0, len(services))

	for _, svc := range services {
		status := HealthStatus{Component: svc.name}

		if strings.TrimSpace(svc.base) == "" {
			status.Status = "unknown"
			status.Detail = "base URL not configured"
			statuses = append(statuses, status)
			continue
		}

		url := fmt.Sprintf("%s%s", strings.TrimRight(svc.base, "/"), c.config.HealthEndpoint)

		reqCtx, cancel := context.WithTimeout(ctx, c.config.HealthTimeout)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
		if err != nil {
			status.Status = "error"
			status.Detail = err.Error()
			statuses = append(statuses, status)
			cancel()
			continue
		}

		if svc.auth != nil {
			svc.auth(req)
		}

		resp, err := c.config.HTTPClient.Do(req)
		if err != nil {
			status.Status = "error"
			status.Detail = err.Error()
			statuses = append(statuses, status)
			cancel()
			continue
		}

		// Fully drain and close the body to allow connection reuse.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			status.Status = "ok"
		} else {
			status.Status = "error"
			status.Detail = resp.Status
		}

		cancel()
		statuses = append(statuses, status)
	}

	return statuses
}

// bearerAuth returns a request mutator that sets a Bearer token
// Authorization header on outgoing HTTP requests. If the token is
// empty or whitespace, nil is returned and no auth is applied.
func bearerAuth(token string) func(*http.Request) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	return func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// basicAuth returns a request mutator that sets HTTP Basic Auth on
// outgoing requests. If both username and password are empty,
// nil is returned and no auth is applied.
func basicAuth(username, password string) func(*http.Request) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" && password == "" {
		return nil
	}
	return func(req *http.Request) {
		req.SetBasicAuth(username, password)
	}
}
