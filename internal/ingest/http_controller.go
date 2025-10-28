package ingest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPController orchestrates ingest operations via REST endpoints.
type HTTPController struct {
	config       Config
	channels     channelAdapter
	applications applicationAdapter
	transcoder   transcoderAdapter
}

func (c *HTTPController) ensureAdapters() {
	if c.config.HTTPClient == nil {
		c.config.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if c.channels == nil {
		c.channels = newHTTPChannelAdapter(c.config.SRSBaseURL, c.config.SRSToken, c.config.HTTPClient)
	}
	if c.applications == nil {
		c.applications = newHTTPApplicationAdapter(c.config.OMEBaseURL, c.config.OMEUsername, c.config.OMEPassword, c.config.HTTPClient)
	}
	if c.transcoder == nil {
		c.transcoder = newHTTPTranscoderAdapter(c.config.JobBaseURL, c.config.JobToken, c.config.HTTPClient)
	}
}

func (c *HTTPController) BootStream(ctx context.Context, params BootParams) (BootResult, error) {
	if params.ChannelID == "" || params.StreamKey == "" {
		return BootResult{}, fmt.Errorf("channelID and streamKey are required")
	}

	c.ensureAdapters()

	primary, backup, err := c.channels.CreateChannel(ctx, params.ChannelID, params.StreamKey)
	if err != nil {
		return BootResult{}, err
	}

	origin, playback, err := c.applications.CreateApplication(ctx, params.ChannelID, params.Renditions)
	if err != nil {
		_ = c.channels.DeleteChannel(ctx, params.ChannelID)
		return BootResult{}, err
	}

	jobIDs, renditions, err := c.transcoder.StartJobs(ctx, params.ChannelID, params.SessionID, origin, c.config.LadderProfiles)
	if err != nil {
		_ = c.applications.DeleteApplication(ctx, params.ChannelID)
		_ = c.channels.DeleteChannel(ctx, params.ChannelID)
		return BootResult{}, err
	}

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

	var errs []string
	for _, jobID := range jobIDs {
		if err := c.transcoder.StopJob(ctx, jobID); err != nil {
			errs = append(errs, fmt.Sprintf("stop job %s: %v", jobID, err))
		}
	}
	if err := c.applications.DeleteApplication(ctx, channelID); err != nil {
		errs = append(errs, fmt.Sprintf("delete OME app: %v", err))
	}
	if err := c.channels.DeleteChannel(ctx, channelID); err != nil {
		errs = append(errs, fmt.Sprintf("delete SRS channel: %v", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
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
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			status.Status = "error"
			status.Detail = err.Error()
			statuses = append(statuses, status)
			continue
		}
		if svc.auth != nil {
			svc.auth(req)
		}
		resp, err := c.config.HTTPClient.Do(req)
		if err != nil {
			status.Status = "error"
			status.Detail = err.Error()
		} else {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				status.Status = "ok"
			} else {
				status.Status = "error"
				status.Detail = resp.Status
			}
		}
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
