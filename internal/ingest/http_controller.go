package ingest

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// HTTPController orchestrates ingest operations via REST endpoints.
type HTTPController struct {
	config        Config
	channels      channelAdapter
	applications  applicationAdapter
	transcoder    transcoderAdapter
	logger        *slog.Logger
	retryAttempts int
	retryInterval time.Duration
}

func (c *HTTPController) ensureAdapters() {
	if c.config.HTTPClient == nil {
		c.config.HTTPClient = &http.Client{Timeout: 2 * time.Second}
	}
	if c.config.HealthTimeout <= 0 {
		c.config.HealthTimeout = 2 * time.Second
	}
	c.ensureLogger()
	if c.channels == nil {
		c.channels = newHTTPChannelAdapter(c.config.SRSBaseURL, c.config.SRSToken, c.config.HTTPClient, c.logger, c.retryAttempts, c.retryInterval)
	}
	if c.applications == nil {
		c.applications = newHTTPApplicationAdapter(c.config.OMEBaseURL, c.config.OMEUsername, c.config.OMEPassword, c.config.HTTPClient, c.logger, c.retryAttempts, c.retryInterval)
	}
	if c.transcoder == nil {
		c.transcoder = newHTTPTranscoderAdapter(c.config.JobBaseURL, c.config.JobToken, c.config.HTTPClient, c.logger, c.retryAttempts, c.retryInterval)
	}
}

func (c *HTTPController) ensureLogger() {
	if c.logger == nil {
		c.logger = slog.Default()
	}
	if c.retryAttempts <= 0 {
		c.retryAttempts = c.config.HTTPMaxAttempts
		if c.retryAttempts <= 0 {
			c.retryAttempts = 1
		}
	}
	if c.retryInterval == 0 {
		c.retryInterval = c.config.HTTPRetryInterval
	}
}

// SetLogger installs a structured logger for ingest orchestration.
func (c *HTTPController) SetLogger(logger *slog.Logger) {
	if logger == nil {
		return
	}
	c.logger = logger
}

func (c *HTTPController) BootStream(ctx context.Context, params BootParams) (BootResult, error) {
	if params.ChannelID == "" || params.StreamKey == "" {
		return BootResult{}, fmt.Errorf("channelID and streamKey are required")
	}

	c.ensureAdapters()

	c.logger.Info("booting ingest pipeline", "channel_id", params.ChannelID, "session_id", params.SessionID)

	primary, backup, err := c.channels.CreateChannel(ctx, params.ChannelID, params.StreamKey)
	if err != nil {
		c.logger.Error("failed to create SRS channel", "channel_id", params.ChannelID, "error", err)
		return BootResult{}, err
	}

	origin, playback, err := c.applications.CreateApplication(ctx, params.ChannelID, params.Renditions)
	if err != nil {
		c.logger.Error("failed to create OME application", "channel_id", params.ChannelID, "error", err)
		_ = c.channels.DeleteChannel(ctx, params.ChannelID)
		return BootResult{}, err
	}

	jobIDs, renditions, err := c.transcoder.StartJobs(ctx, params.ChannelID, params.SessionID, origin, c.config.LadderProfiles)
	if err != nil {
		c.logger.Error("failed to start transcoder jobs", "channel_id", params.ChannelID, "session_id", params.SessionID, "error", err)
		_ = c.applications.DeleteApplication(ctx, params.ChannelID)
		_ = c.channels.DeleteChannel(ctx, params.ChannelID)
		return BootResult{}, err
	}

	c.logger.Info("ingest pipeline ready", "channel_id", params.ChannelID, "session_id", params.SessionID, "jobs", len(jobIDs))

	return BootResult{
		PrimaryIngest: primary,
		BackupIngest:  backup,
		OriginURL:     origin,
		PlaybackURL:   playback,
		Renditions:    renditions,
		JobIDs:        jobIDs,
	}, nil
}

func (c *HTTPController) ShutdownStream(ctx context.Context, channelID, sessionID string, jobIDs []string) error {
	c.ensureAdapters()

	c.logger.Info("tearing down ingest pipeline", "channel_id", channelID, "session_id", sessionID, "jobs", len(jobIDs))
	var errs []string
	for _, jobID := range jobIDs {
		if err := c.transcoder.StopJob(ctx, jobID); err != nil {
			c.logger.Error("failed to stop transcoder job", "job_id", jobID, "error", err)
			errs = append(errs, fmt.Sprintf("stop job %s: %v", jobID, err))
		}
	}
	if err := c.applications.DeleteApplication(ctx, channelID); err != nil {
		c.logger.Error("failed to delete OME application", "channel_id", channelID, "error", err)
		errs = append(errs, fmt.Sprintf("delete OME app: %v", err))
	}
	if err := c.channels.DeleteChannel(ctx, channelID); err != nil {
		c.logger.Error("failed to delete SRS channel", "channel_id", channelID, "error", err)
		errs = append(errs, fmt.Sprintf("delete SRS channel: %v", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	c.logger.Info("ingest pipeline removed", "channel_id", channelID, "session_id", sessionID)
	return nil
}

// TranscodeUpload submits a stored media file for HLS transcoding via the
// configured transcoder adapter.
func (c *HTTPController) TranscodeUpload(ctx context.Context, params UploadTranscodeParams) (UploadTranscodeResult, error) {
	if strings.TrimSpace(params.ChannelID) == "" || strings.TrimSpace(params.UploadID) == "" {
		return UploadTranscodeResult{}, fmt.Errorf("channelID and uploadID are required")
	}
	source := strings.TrimSpace(params.SourceURL)
	if source == "" {
		return UploadTranscodeResult{}, fmt.Errorf("sourceURL is required")
	}

	c.ensureAdapters()

	c.logger.Info("starting upload transcode", "channel_id", params.ChannelID, "upload_id", params.UploadID)
	result, err := c.transcoder.StartUpload(ctx, uploadJobRequest{
		ChannelID:  params.ChannelID,
		UploadID:   params.UploadID,
		SourceURL:  source,
		Filename:   strings.TrimSpace(params.Filename),
		Renditions: cloneRenditions(params.Renditions),
	})
	if err != nil {
		c.logger.Error("failed to start upload transcode", "channel_id", params.ChannelID, "upload_id", params.UploadID, "error", err)
		return UploadTranscodeResult{}, err
	}
	c.logger.Info("upload transcode submitted", "channel_id", params.ChannelID, "upload_id", params.UploadID, "job_id", result.JobID)

	return UploadTranscodeResult{
		PlaybackURL: result.PlaybackURL,
		Renditions:  cloneRenditions(result.Renditions),
		JobID:       result.JobID,
	}, nil
}

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
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
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

func bearerAuth(token string) func(*http.Request) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	return func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

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
