package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPController orchestrates ingest operations via REST endpoints.
type HTTPController struct {
	config Config
}

type srsChannelRequest struct {
	ChannelID string `json:"channelId"`
	StreamKey string `json:"streamKey"`
}

type srsChannelResponse struct {
	PrimaryIngest string `json:"primaryIngest"`
	BackupIngest  string `json:"backupIngest"`
}

type omeApplicationRequest struct {
	ChannelID  string   `json:"channelId"`
	Renditions []string `json:"renditions"`
}

type omeApplicationResponse struct {
	OriginURL   string `json:"originUrl"`
	PlaybackURL string `json:"playbackUrl"`
}

type ffmpegJobRequest struct {
	ChannelID  string      `json:"channelId"`
	SessionID  string      `json:"sessionId"`
	OriginURL  string      `json:"originUrl"`
	Renditions []Rendition `json:"renditions"`
}

type ffmpegJobResponse struct {
	JobID      string      `json:"jobId"`
	JobIDs     []string    `json:"jobIds"`
	Renditions []Rendition `json:"renditions"`
}

func (c *HTTPController) BootStream(ctx context.Context, params BootParams) (BootResult, error) {
	if params.ChannelID == "" || params.StreamKey == "" {
		return BootResult{}, fmt.Errorf("channelID and streamKey are required")
	}

	httpClient := c.config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	primary, backup, err := c.provisionSRS(ctx, httpClient, params)
	if err != nil {
		return BootResult{}, err
	}

	origin, playback, err := c.provisionOME(ctx, httpClient, params)
	if err != nil {
		c.rollbackSRS(ctx, httpClient, params.ChannelID)
		return BootResult{}, err
	}

	jobIDs, renditions, err := c.startFFmpeg(ctx, httpClient, params, origin)
	if err != nil {
		c.rollbackOME(ctx, httpClient, params.ChannelID)
		c.rollbackSRS(ctx, httpClient, params.ChannelID)
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
	httpClient := c.config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	var errs []string
	for _, jobID := range jobIDs {
		if err := c.delete(ctx, httpClient, fmt.Sprintf("%s/v1/jobs/%s", strings.TrimRight(c.config.JobBaseURL, "/"), jobID), c.config.JobToken); err != nil {
			errs = append(errs, fmt.Sprintf("stop job %s: %v", jobID, err))
		}
	}
	if err := c.delete(ctx, httpClient, fmt.Sprintf("%s/v1/applications/%s", strings.TrimRight(c.config.OMEBaseURL, "/"), channelID), basicAuth(c.config.OMEUsername, c.config.OMEPassword)); err != nil {
		errs = append(errs, fmt.Sprintf("delete OME app: %v", err))
	}
	if err := c.delete(ctx, httpClient, fmt.Sprintf("%s/v1/channels/%s", strings.TrimRight(c.config.SRSBaseURL, "/"), channelID), bearer(c.config.SRSToken)); err != nil {
		errs = append(errs, fmt.Sprintf("delete SRS channel: %v", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
}

func (c *HTTPController) HealthChecks(ctx context.Context) []HealthStatus {
	httpClient := c.config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	services := []struct {
		name   string
		base   string
		header string
	}{
		{name: "srs", base: c.config.SRSBaseURL, header: bearer(c.config.SRSToken)},
		{name: "ovenmediaengine", base: c.config.OMEBaseURL, header: basicAuth(c.config.OMEUsername, c.config.OMEPassword)},
		{name: "transcoder", base: c.config.JobBaseURL, header: bearer(c.config.JobToken)},
	}

	statuses := make([]HealthStatus, 0, len(services))
	for _, service := range services {
		status := HealthStatus{Component: service.name}
		if service.base == "" {
			status.Status = "unknown"
			status.Detail = "base URL not configured"
			statuses = append(statuses, status)
			continue
		}
		url := fmt.Sprintf("%s%s", strings.TrimRight(service.base, "/"), c.config.HealthEndpoint)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			status.Status = "error"
			status.Detail = err.Error()
			statuses = append(statuses, status)
			continue
		}
		if service.header != "" {
			parts := strings.SplitN(service.header, " ", 2)
			if len(parts) == 2 && strings.ToLower(parts[0]) == "basic" {
				req.SetBasicAuth(c.config.OMEUsername, c.config.OMEPassword)
			} else {
				req.Header.Set("Authorization", service.header)
			}
		}
		resp, err := httpClient.Do(req)
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

func (c *HTTPController) provisionSRS(ctx context.Context, client *http.Client, params BootParams) (string, string, error) {
	payload := srsChannelRequest{ChannelID: params.ChannelID, StreamKey: params.StreamKey}
	var response srsChannelResponse
	if err := c.post(ctx, client, fmt.Sprintf("%s/v1/channels", strings.TrimRight(c.config.SRSBaseURL, "/")), payload, &response, bearer(c.config.SRSToken)); err != nil {
		return "", "", err
	}
	return response.PrimaryIngest, response.BackupIngest, nil
}

func (c *HTTPController) provisionOME(ctx context.Context, client *http.Client, params BootParams) (string, string, error) {
	payload := omeApplicationRequest{ChannelID: params.ChannelID, Renditions: params.Renditions}
	var response omeApplicationResponse
	if err := c.post(ctx, client, fmt.Sprintf("%s/v1/applications", strings.TrimRight(c.config.OMEBaseURL, "/")), payload, &response, basicAuth(c.config.OMEUsername, c.config.OMEPassword)); err != nil {
		return "", "", err
	}
	return response.OriginURL, response.PlaybackURL, nil
}

func (c *HTTPController) startFFmpeg(ctx context.Context, client *http.Client, params BootParams, origin string) ([]string, []Rendition, error) {
	payload := ffmpegJobRequest{ChannelID: params.ChannelID, SessionID: params.SessionID, OriginURL: origin, Renditions: c.config.LadderProfiles}
	var response ffmpegJobResponse
	if err := c.post(ctx, client, fmt.Sprintf("%s/v1/jobs", strings.TrimRight(c.config.JobBaseURL, "/")), payload, &response, bearer(c.config.JobToken)); err != nil {
		return nil, nil, err
	}
	jobIDs := append([]string{}, response.JobIDs...)
	if response.JobID != "" {
		jobIDs = append(jobIDs, response.JobID)
	}
	return jobIDs, response.Renditions, nil
}

func (c *HTTPController) rollbackSRS(ctx context.Context, client *http.Client, channelID string) {
	_ = c.delete(ctx, client, fmt.Sprintf("%s/v1/channels/%s", strings.TrimRight(c.config.SRSBaseURL, "/"), channelID), bearer(c.config.SRSToken))
}

func (c *HTTPController) rollbackOME(ctx context.Context, client *http.Client, channelID string) {
	_ = c.delete(ctx, client, fmt.Sprintf("%s/v1/applications/%s", strings.TrimRight(c.config.OMEBaseURL, "/"), channelID), basicAuth(c.config.OMEUsername, c.config.OMEPassword))
}

func (c *HTTPController) post(ctx context.Context, client *http.Client, url string, payload interface{}, dest interface{}, authHeader string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		if strings.HasPrefix(strings.ToLower(authHeader), "basic ") {
			req.SetBasicAuth(c.config.OMEUsername, c.config.OMEPassword)
		} else {
			req.Header.Set("Authorization", authHeader)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	if dest == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func (c *HTTPController) delete(ctx context.Context, client *http.Client, url string, authHeader string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	if authHeader != "" {
		if strings.HasPrefix(strings.ToLower(authHeader), "basic ") {
			req.SetBasicAuth(c.config.OMEUsername, c.config.OMEPassword)
		} else {
			req.Header.Set("Authorization", authHeader)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	data, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(data)))
}

func bearer(token string) string {
	if token == "" {
		return ""
	}
	return "Bearer " + token
}

func basicAuth(username, password string) string {
	if username == "" && password == "" {
		return ""
	}
	return "Basic "
}
