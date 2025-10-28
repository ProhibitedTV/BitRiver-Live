package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type channelAdapter interface {
	CreateChannel(ctx context.Context, channelID, streamKey string) (primary string, backup string, err error)
	DeleteChannel(ctx context.Context, channelID string) error
}

type applicationAdapter interface {
	CreateApplication(ctx context.Context, channelID string, renditions []string) (originURL, playbackURL string, err error)
	DeleteApplication(ctx context.Context, channelID string) error
}

type transcoderAdapter interface {
	StartJobs(ctx context.Context, channelID, sessionID, originURL string, ladder []Rendition) ([]string, []Rendition, error)
	StopJob(ctx context.Context, jobID string) error
}

type httpChannelAdapter struct {
	baseURL       string
	token         string
	client        *http.Client
	logger        *slog.Logger
	maxAttempts   int
	retryInterval time.Duration
}

type httpApplicationAdapter struct {
	baseURL       string
	username      string
	password      string
	client        *http.Client
	logger        *slog.Logger
	maxAttempts   int
	retryInterval time.Duration
}

type httpTranscoderAdapter struct {
	baseURL       string
	token         string
	client        *http.Client
	logger        *slog.Logger
	maxAttempts   int
	retryInterval time.Duration
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

func newHTTPChannelAdapter(baseURL, token string, client *http.Client, logger *slog.Logger, attempts int, interval time.Duration) *httpChannelAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	if attempts <= 0 {
		attempts = 1
	}
	return &httpChannelAdapter{
		baseURL:       strings.TrimRight(baseURL, "/"),
		token:         token,
		client:        client,
		logger:        logger,
		maxAttempts:   attempts,
		retryInterval: interval,
	}
}

func newHTTPApplicationAdapter(baseURL, username, password string, client *http.Client, logger *slog.Logger, attempts int, interval time.Duration) *httpApplicationAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	if attempts <= 0 {
		attempts = 1
	}
	return &httpApplicationAdapter{
		baseURL:       strings.TrimRight(baseURL, "/"),
		username:      username,
		password:      password,
		client:        client,
		logger:        logger,
		maxAttempts:   attempts,
		retryInterval: interval,
	}
}

func newHTTPTranscoderAdapter(baseURL, token string, client *http.Client, logger *slog.Logger, attempts int, interval time.Duration) *httpTranscoderAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	if attempts <= 0 {
		attempts = 1
	}
	return &httpTranscoderAdapter{
		baseURL:       strings.TrimRight(baseURL, "/"),
		token:         token,
		client:        client,
		logger:        logger,
		maxAttempts:   attempts,
		retryInterval: interval,
	}
}

func (a *httpChannelAdapter) CreateChannel(ctx context.Context, channelID, streamKey string) (string, string, error) {
	payload := srsChannelRequest{ChannelID: channelID, StreamKey: streamKey}
	var response srsChannelResponse
	if err := postJSON(ctx, a.client, fmt.Sprintf("%s/v1/channels", a.baseURL), payload, &response, func(req *http.Request) {
		setBearer(req, a.token)
	}, a.logger, a.maxAttempts, a.retryInterval); err != nil {
		return "", "", err
	}
	return response.PrimaryIngest, response.BackupIngest, nil
}

func (a *httpChannelAdapter) DeleteChannel(ctx context.Context, channelID string) error {
	return deleteRequest(ctx, a.client, fmt.Sprintf("%s/v1/channels/%s", a.baseURL, channelID), func(req *http.Request) {
		setBearer(req, a.token)
	}, a.logger, a.maxAttempts, a.retryInterval)
}

func (a *httpApplicationAdapter) CreateApplication(ctx context.Context, channelID string, renditions []string) (string, string, error) {
	payload := omeApplicationRequest{ChannelID: channelID, Renditions: append([]string{}, renditions...)}
	var response omeApplicationResponse
	if err := postJSON(ctx, a.client, fmt.Sprintf("%s/v1/applications", a.baseURL), payload, &response, func(req *http.Request) {
		req.SetBasicAuth(a.username, a.password)
	}, a.logger, a.maxAttempts, a.retryInterval); err != nil {
		return "", "", err
	}
	return response.OriginURL, response.PlaybackURL, nil
}

func (a *httpApplicationAdapter) DeleteApplication(ctx context.Context, channelID string) error {
	return deleteRequest(ctx, a.client, fmt.Sprintf("%s/v1/applications/%s", a.baseURL, channelID), func(req *http.Request) {
		req.SetBasicAuth(a.username, a.password)
	}, a.logger, a.maxAttempts, a.retryInterval)
}

func (a *httpTranscoderAdapter) StartJobs(ctx context.Context, channelID, sessionID, originURL string, ladder []Rendition) ([]string, []Rendition, error) {
	payload := ffmpegJobRequest{ChannelID: channelID, SessionID: sessionID, OriginURL: originURL, Renditions: cloneRenditions(ladder)}
	var response ffmpegJobResponse
	if err := postJSON(ctx, a.client, fmt.Sprintf("%s/v1/jobs", a.baseURL), payload, &response, func(req *http.Request) {
		setBearer(req, a.token)
	}, a.logger, a.maxAttempts, a.retryInterval); err != nil {
		return nil, nil, err
	}
	jobIDs := append([]string{}, response.JobIDs...)
	if response.JobID != "" {
		jobIDs = append(jobIDs, response.JobID)
	}
	renditions := append([]Rendition(nil), response.Renditions...)
	return jobIDs, renditions, nil
}

func (a *httpTranscoderAdapter) StopJob(ctx context.Context, jobID string) error {
	return deleteRequest(ctx, a.client, fmt.Sprintf("%s/v1/jobs/%s", a.baseURL, jobID), func(req *http.Request) {
		setBearer(req, a.token)
	}, a.logger, a.maxAttempts, a.retryInterval)
}

func postJSON(ctx context.Context, client *http.Client, url string, payload interface{}, dest interface{}, mutate func(*http.Request), logger *slog.Logger, attempts int, interval time.Duration) error {
	if client == nil {
		client = &http.Client{}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	return doWithRetry(ctx, client, http.MethodPost, url, body, mutate, dest, logger, attempts, interval)
}

func deleteRequest(ctx context.Context, client *http.Client, url string, mutate func(*http.Request), logger *slog.Logger, attempts int, interval time.Duration) error {
	if client == nil {
		client = &http.Client{}
	}
	return doWithRetry(ctx, client, http.MethodDelete, url, nil, mutate, nil, logger, attempts, interval)
}

func doWithRetry(ctx context.Context, client *http.Client, method, url string, payload []byte, mutate func(*http.Request), dest interface{}, logger *slog.Logger, attempts int, interval time.Duration) error {
	if attempts <= 0 {
		attempts = 1
	}
	if interval < 0 {
		interval = 0
	}
	if logger == nil {
		logger = slog.Default()
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		reqBody := io.Reader(nil)
		if payload != nil {
			reqBody = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return err
		}
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if mutate != nil {
			mutate(req)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			func() {
				defer resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					if dest == nil {
						lastErr = nil
						return
					}
					decoderErr := json.NewDecoder(resp.Body).Decode(dest)
					if decoderErr != nil {
						lastErr = decoderErr
					} else {
						lastErr = nil
					}
					return
				}
				data, _ := io.ReadAll(resp.Body)
				lastErr = fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(data)))
			}()
		}
		if lastErr == nil {
			return nil
		}
		if attempt < attempts {
			logger.Warn("ingest HTTP request failed", "method", method, "url", url, "attempt", attempt, "error", lastErr)
			if interval > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(interval):
				}
			} else {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
			}
			continue
		}
	}
	return lastErr
}

func setBearer(req *http.Request, token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
}

func cloneRenditions(input []Rendition) []Rendition {
	if len(input) == 0 {
		return nil
	}
	out := make([]Rendition, len(input))
	copy(out, input)
	return out
}
