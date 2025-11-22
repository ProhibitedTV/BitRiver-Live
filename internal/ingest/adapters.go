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

// Default values used when callers do not provide explicit settings.
const (
	defaultHTTPTimeout  = 10 * time.Second
	defaultMaxAttempts  = 3
	defaultRetryBackoff = 500 * time.Millisecond
)

// channelAdapter defines the behavior required to provision and tear down
// ingest channels on an upstream streaming server (e.g. SRS).
//
// Implementations are responsible for contacting the appropriate control
// plane and returning primary/backup ingest URLs for a given channel ID and
// stream key.
type channelAdapter interface {
	// CreateChannel provisions a new ingest channel identified by channelID
	// and secured by streamKey. It returns primary and backup ingest URLs.
	CreateChannel(ctx context.Context, channelID, streamKey string) (primary string, backup string, err error)

	// DeleteChannel tears down the ingest channel associated with channelID.
	DeleteChannel(ctx context.Context, channelID string) error
}

// applicationAdapter defines the behavior required to manage streaming
// applications on an origin server (e.g. OvenMediaEngine).
//
// Implementations typically create an application per channel and return
// both origin (pull) URLs for the transcoder and playback URLs for viewers.
type applicationAdapter interface {
	// CreateApplication provisions a new application for the given channelID
	// and renditions. It returns the origin URL (used by the transcoder) and
	// the playback URL (used by viewers).
	CreateApplication(ctx context.Context, channelID string, renditions []string) (originURL, playbackURL string, err error)

	// DeleteApplication removes the application associated with channelID.
	DeleteApplication(ctx context.Context, channelID string) error
}

// transcoderAdapter defines the behavior required to manage transcoding
// jobs for both live streams and uploaded VOD assets.
type transcoderAdapter interface {
	// StartJobs starts one or more live transcoding jobs for the given
	// channelID and sessionID, pulling from originURL using the provided
	// rendition ladder. It returns job IDs and the effective renditions used.
	StartJobs(ctx context.Context, channelID, sessionID, originURL string, ladder []Rendition) ([]string, []Rendition, error)

	// StopJob stops a specific transcoding job by its jobID.
	StopJob(ctx context.Context, jobID string) error

	// StartUpload starts a VOD transcoding/upload job for a previously
	// uploaded source, identified by UploadID. It returns a job result that
	// includes the playback URL and effective renditions.
	StartUpload(ctx context.Context, req uploadJobRequest) (uploadJobResult, error)
}

// httpChannelAdapter is an HTTP implementation of channelAdapter that
// communicates with an SRS controller (or similar) using a bearer token.
type httpChannelAdapter struct {
	baseURL       string
	token         string
	client        *http.Client
	logger        *slog.Logger
	maxAttempts   int
	retryInterval time.Duration
}

// httpApplicationAdapter is an HTTP implementation of applicationAdapter
// that communicates with an OvenMediaEngine (OME) API using basic auth.
type httpApplicationAdapter struct {
	baseURL       string
	username      string
	password      string
	client        *http.Client
	logger        *slog.Logger
	maxAttempts   int
	retryInterval time.Duration
}

// httpTranscoderAdapter is an HTTP implementation of transcoderAdapter that
// communicates with an FFmpeg-based transcoding service using a bearer token.
type httpTranscoderAdapter struct {
	baseURL       string
	token         string
	client        *http.Client
	logger        *slog.Logger
	maxAttempts   int
	retryInterval time.Duration
}

// srsChannelRequest is the JSON payload sent to the SRS controller when
// creating a new ingest channel.
type srsChannelRequest struct {
	ChannelID string `json:"channelId"`
	StreamKey string `json:"streamKey"`
}

// srsChannelResponse is the JSON response from the SRS controller when a
// channel is created.
type srsChannelResponse struct {
	PrimaryIngest string `json:"primaryIngest"`
	BackupIngest  string `json:"backupIngest"`
}

// omeApplicationRequest is the JSON payload sent to the OME API when
// creating a new application.
type omeApplicationRequest struct {
	ChannelID  string   `json:"channelId"`
	Renditions []string `json:"renditions"`
}

// omeApplicationResponse is the JSON response from the OME API when an
// application is created.
type omeApplicationResponse struct {
	OriginURL   string `json:"originUrl"`
	PlaybackURL string `json:"playbackUrl"`
}

// ffmpegJobRequest is the JSON payload sent to the transcoder service when
// starting live jobs.
type ffmpegJobRequest struct {
	ChannelID  string      `json:"channelId"`
	SessionID  string      `json:"sessionId"`
	OriginURL  string      `json:"originUrl"`
	Renditions []Rendition `json:"renditions"`
}

// ffmpegJobResponse is the JSON response from the transcoder service when
// live jobs are started.
type ffmpegJobResponse struct {
	// JobID is kept for backward-compatibility with backends that only return
	// a single ID.
	JobID string `json:"jobId"`
	// JobIDs contains one or more job identifiers when the backend supports it.
	JobIDs     []string    `json:"jobIds"`
	Renditions []Rendition `json:"renditions"`
}

// uploadJobRequest represents a high-level request to start a VOD upload job.
// This type is internal to the ingest package and is converted to a JSON
// request for the transcoder service.
type uploadJobRequest struct {
	ChannelID  string
	UploadID   string
	SourceURL  string
	Filename   string
	Renditions []Rendition
}

// ffmpegUploadRequest is the JSON payload sent to the transcoder service
// when starting a VOD upload/transcode job.
type ffmpegUploadRequest struct {
	ChannelID  string      `json:"channelId"`
	UploadID   string      `json:"uploadId"`
	SourceURL  string      `json:"sourceUrl"`
	Filename   string      `json:"filename,omitempty"`
	Renditions []Rendition `json:"renditions,omitempty"`
}

// ffmpegUploadResponse is the JSON response from the transcoder service
// when a VOD upload/transcode job is started.
type ffmpegUploadResponse struct {
	JobID       string      `json:"jobId"`
	PlaybackURL string      `json:"playbackUrl"`
	Renditions  []Rendition `json:"renditions"`
}

// uploadJobResult is a high-level result of starting a VOD upload job, used
// internally by the ingest package.
type uploadJobResult struct {
	JobID       string
	PlaybackURL string
	Renditions  []Rendition
}

// newHTTPChannelAdapter constructs an HTTP-based channelAdapter.
// If logger is nil, slog.Default is used.
// If attempts <= 0, a sane default is applied.
// If interval is zero, a small default backoff is used.
// If client is nil, a new http.Client with a default timeout is created
// for each request.
func newHTTPChannelAdapter(baseURL, token string, client *http.Client, logger *slog.Logger, attempts int, interval time.Duration) *httpChannelAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	if attempts <= 0 {
		attempts = defaultMaxAttempts
	}
	if interval == 0 {
		interval = defaultRetryBackoff
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

// newHTTPApplicationAdapter constructs an HTTP-based applicationAdapter.
// See newHTTPChannelAdapter for behavior of the logger, attempts, interval,
// and client parameters.
func newHTTPApplicationAdapter(baseURL, username, password string, client *http.Client, logger *slog.Logger, attempts int, interval time.Duration) *httpApplicationAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	if attempts <= 0 {
		attempts = defaultMaxAttempts
	}
	if interval == 0 {
		interval = defaultRetryBackoff
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

// newHTTPTranscoderAdapter constructs an HTTP-based transcoderAdapter.
// See newHTTPChannelAdapter for behavior of the logger, attempts, interval,
// and client parameters.
func newHTTPTranscoderAdapter(baseURL, token string, client *http.Client, logger *slog.Logger, attempts int, interval time.Duration) *httpTranscoderAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	if attempts <= 0 {
		attempts = defaultMaxAttempts
	}
	if interval == 0 {
		interval = defaultRetryBackoff
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

// CreateChannel provisions a new channel by calling the configured SRS
// controller.
//
// The method will retry transient failures (network errors and 5xx/429
// responses) up to maxAttempts. Callers are encouraged to pass a context
// with a deadline to bound the overall operation duration.
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

// DeleteChannel tears down the channel identified by channelID by calling
// the configured SRS controller.
func (a *httpChannelAdapter) DeleteChannel(ctx context.Context, channelID string) error {
	return deleteRequest(ctx, a.client, fmt.Sprintf("%s/v1/channels/%s", a.baseURL, channelID), func(req *http.Request) {
		setBearer(req, a.token)
	}, a.logger, a.maxAttempts, a.retryInterval)
}

// CreateApplication provisions a new application on the origin server (OME)
// for the given channel and renditions.
//
// The renditions slice is defensively copied to avoid accidental mutation by
// callers after the request is initiated.
func (a *httpApplicationAdapter) CreateApplication(ctx context.Context, channelID string, renditions []string) (string, string, error) {
	payload := omeApplicationRequest{
		ChannelID:  channelID,
		Renditions: append([]string{}, renditions...),
	}
	var response omeApplicationResponse
	if err := postJSON(ctx, a.client, fmt.Sprintf("%s/v1/applications", a.baseURL), payload, &response, func(req *http.Request) {
		req.SetBasicAuth(a.username, a.password)
	}, a.logger, a.maxAttempts, a.retryInterval); err != nil {
		return "", "", err
	}
	return response.OriginURL, response.PlaybackURL, nil
}

// DeleteApplication removes the application associated with channelID from
// the origin server (OME).
func (a *httpApplicationAdapter) DeleteApplication(ctx context.Context, channelID string) error {
	return deleteRequest(ctx, a.client, fmt.Sprintf("%s/v1/applications/%s", a.baseURL, channelID), func(req *http.Request) {
		req.SetBasicAuth(a.username, a.password)
	}, a.logger, a.maxAttempts, a.retryInterval)
}

// StartJobs starts one or more live transcoding jobs for the given channel,
// session, and origin URL using the provided rendition ladder.
//
// The returned jobIDs slice may contain IDs from both JobID and JobIDs
// response fields to maintain backward compatibility with older backends.
func (a *httpTranscoderAdapter) StartJobs(ctx context.Context, channelID, sessionID, originURL string, ladder []Rendition) ([]string, []Rendition, error) {
	payload := ffmpegJobRequest{
		ChannelID:  channelID,
		SessionID:  sessionID,
		OriginURL:  originURL,
		Renditions: cloneRenditions(ladder),
	}
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
	renditions := cloneRenditions(response.Renditions)
	return jobIDs, renditions, nil
}

// StopJob stops a live transcoding job with the specified jobID.
func (a *httpTranscoderAdapter) StopJob(ctx context.Context, jobID string) error {
	return deleteRequest(ctx, a.client, fmt.Sprintf("%s/v1/jobs/%s", a.baseURL, jobID), func(req *http.Request) {
		setBearer(req, a.token)
	}, a.logger, a.maxAttempts, a.retryInterval)
}

// StartUpload starts a VOD transcoding/upload job for the given upload
// request. It returns a result that includes the job ID, playback URL and
// effective renditions.
//
// Renditions are defensively copied to avoid aliasing.
func (a *httpTranscoderAdapter) StartUpload(ctx context.Context, req uploadJobRequest) (uploadJobResult, error) {
	payload := ffmpegUploadRequest{
		ChannelID:  req.ChannelID,
		UploadID:   req.UploadID,
		SourceURL:  req.SourceURL,
		Filename:   req.Filename,
		Renditions: cloneRenditions(req.Renditions),
	}
	var response ffmpegUploadResponse
	if err := postJSON(ctx, a.client, fmt.Sprintf("%s/v1/uploads", a.baseURL), payload, &response, func(httpReq *http.Request) {
		setBearer(httpReq, a.token)
	}, a.logger, a.maxAttempts, a.retryInterval); err != nil {
		return uploadJobResult{}, err
	}
	return uploadJobResult{
		JobID:       response.JobID,
		PlaybackURL: response.PlaybackURL,
		Renditions:  cloneRenditions(response.Renditions),
	}, nil
}

// postJSON issues an HTTP POST with a JSON payload and decodes the JSON
// response into dest (if non-nil). It uses retry semantics defined by
// doWithRetry. If client is nil, a temporary client with a default timeout
// is created for this call.
func postJSON(ctx context.Context, client *http.Client, url string, payload interface{}, dest interface{}, mutate func(*http.Request), logger *slog.Logger, attempts int, interval time.Duration) error {
	if client == nil {
		client = &http.Client{
			Timeout: defaultHTTPTimeout,
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	return doWithRetry(ctx, client, http.MethodPost, url, body, mutate, dest, logger, attempts, interval)
}

// deleteRequest issues an HTTP DELETE request and discards any successful
// response body. It uses retry semantics defined by doWithRetry. If client
// is nil, a temporary client with a default timeout is created for this call.
func deleteRequest(ctx context.Context, client *http.Client, url string, mutate func(*http.Request), logger *slog.Logger, attempts int, interval time.Duration) error {
	if client == nil {
		client = &http.Client{
			Timeout: defaultHTTPTimeout,
		}
	}
	return doWithRetry(ctx, client, http.MethodDelete, url, nil, mutate, nil, logger, attempts, interval)
}

// doWithRetry executes an HTTP request with basic retry semantics.
//
// Behavior:
//
//   - Retries on:
//     * Network errors (client.Do returns an error).
//     * HTTP 5xx responses.
//     * HTTP 429 (Too Many Requests).
//
//   - Does NOT retry on:
//     * HTTP 4xx responses other than 429 (treated as permanent errors).
//
//   - Honors the provided context for both the HTTP request and the
//     backoff delay between attempts.
//
// Callers are encouraged to pass a context with a deadline to avoid
// unbounded waits if the upstream service is unreachable.
func doWithRetry(
	ctx context.Context,
	client *http.Client,
	method, url string,
	payload []byte,
	mutate func(*http.Request),
	dest interface{},
	logger *slog.Logger,
	attempts int,
	interval time.Duration,
) error {
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
			// NewRequestWithContext failing is typically non-retryable (e.g. bad URL
			// or canceled context), so we return immediately.
			return fmt.Errorf("build request: %w", err)
		}

		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if mutate != nil {
			mutate(req)
		}

		resp, err := client.Do(req)
		if err != nil {
			// Network or transport-level error. Treat as retryable.
			lastErr = err
		} else {
			func() {
				defer resp.Body.Close()

				statusCode := resp.StatusCode

				if statusCode >= 200 && statusCode < 300 {
					// Success.
					if dest == nil {
						lastErr = nil
						return
					}
					decoderErr := json.NewDecoder(resp.Body).Decode(dest)
					if decoderErr != nil {
						lastErr = fmt.Errorf("decode response: %w", decoderErr)
					} else {
						lastErr = nil
					}
					return
				}

				// Read response body for diagnostics.
				data, _ := io.ReadAll(resp.Body)
				errMsg := fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(data)))

				// Determine if this status code is retryable.
				if isRetryableStatus(statusCode) {
					lastErr = errMsg
					return
				}

				// Non-retryable HTTP status (e.g., 4xx other than 429).
				lastErr = errMsg
				// We return early to avoid additional retries.
				attempt = attempts
			}()
		}

		if lastErr == nil {
			return nil
		}

		if attempt < attempts {
			logger.Warn("ingest HTTP request failed",
				"method", method,
				"url", url,
				"attempt", attempt,
				"error", lastErr,
			)

			// Backoff between attempts while honoring context cancellation.
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

// isRetryableStatus reports whether an HTTP status code should be treated
// as transient and therefore retried.
//
// We currently consider 5xx and 429 as retryable. All other 4xx responses
// are treated as permanent failures.
func isRetryableStatus(statusCode int) bool {
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	if statusCode >= 500 && statusCode <= 599 {
		return true
	}
	return false
}

// setBearer sets a Bearer token Authorization header on the provided request.
// If token is empty or whitespace, the header is not set.
func setBearer(req *http.Request, token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
}

// cloneRenditions returns a shallow copy of the provided renditions slice.
// If input is empty, nil is returned to avoid unnecessary allocations.
//
// The Rendition type is defined elsewhere in the ingest package and typically
// contains bitrate, resolution, and other encoding parameters.
func cloneRenditions(input []Rendition) []Rendition {
	if len(input) == 0 {
		return nil
	}
	out := make([]Rendition, len(input))
	copy(out, input)
	return out
}
