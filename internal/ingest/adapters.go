package ingest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bitriver-live/internal/observability/tracing"
)

// Default values used when callers do not provide explicit settings.
const (
	defaultHTTPTimeout  = 10 * time.Second
	defaultMaxAttempts  = 3
	defaultRetryBackoff = 500 * time.Millisecond
)

var defaultHTTPClient = &http.Client{Timeout: defaultHTTPTimeout}

type httpStatusError struct {
	statusCode int
	status     string
	body       string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("%s: %s", e.status, e.body)
}

type adapterConfig struct {
	logger   *slog.Logger
	attempts int
	interval time.Duration
}

func normalizeAdapterConfig(logger *slog.Logger, attempts int, interval time.Duration) adapterConfig {
	if logger == nil {
		logger = slog.Default()
	}
	if attempts <= 0 {
		attempts = defaultMaxAttempts
	}
	if interval == 0 {
		interval = defaultRetryBackoff
	}

	return adapterConfig{
		logger:   logger,
		attempts: attempts,
		interval: interval,
	}
}

// channelAdapter defines the behavior required to provision and tear down
// ingest channels on an upstream streaming server (e.g. SRS).
//
// Implementations are responsible for contacting the appropriate control
// plane and returning primary/backup ingest URLs for a given channel ID and
// stream key. The origin URL is an internal address that the transcoder can
// reach; primary/backup URLs are safe to show to the creator.
type channelAdapter interface {
	// CreateChannel provisions a new ingest channel identified by channelID
	// and secured by streamKey.
	CreateChannel(ctx context.Context, channelID, streamKey string) (primary string, backup string, origin string, err error)

	// DeleteChannel tears down the ingest channel associated with channelID.
	DeleteChannel(ctx context.Context, channelID string) error
}

// applicationAdapter defines the behavior required to manage streaming
// applications on an origin server (e.g. OvenMediaEngine).
//
// The canonical OME application is declared in Server.xml and must not be
// mutated through the REST API. Implementations validate that application and
// derive a per-channel playback URL.
type applicationAdapter interface {
	// CreateApplication validates the configured application for channelID and
	// returns the supplied origin URL plus the viewer playback URL.
	CreateApplication(ctx context.Context, channelID, originURL string, renditions []string) (validatedOriginURL, playbackURL string, err error)

	// DeleteApplication releases per-stream application state. The canonical
	// static OME application implementation intentionally performs no mutation.
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
// that communicates with an OvenMediaEngine (OME) API using the rendered
// AccessToken as an HTTP Basic credential.
type httpApplicationAdapter struct {
	baseURL         string
	playbackBaseURL string
	accessToken     string
	username        string
	password        string
	client          *http.Client
	logger          *slog.Logger
	maxAttempts     int
	retryInterval   time.Duration
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
	OriginIngest  string `json:"originIngest"`
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

type ffmpegUploadStatusResponse struct {
	JobID  string `json:"jobId"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
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
// If client is nil, a shared default http.Client with a default timeout is used.
func newHTTPChannelAdapter(baseURL, token string, client *http.Client, logger *slog.Logger, attempts int, interval time.Duration) *httpChannelAdapter {
	cfg := normalizeAdapterConfig(logger, attempts, interval)
	return &httpChannelAdapter{
		baseURL:       strings.TrimRight(baseURL, "/"),
		token:         token,
		client:        client,
		logger:        cfg.logger,
		maxAttempts:   cfg.attempts,
		retryInterval: cfg.interval,
	}
}

// newHTTPApplicationAdapter constructs an HTTP-based applicationAdapter.
// See newHTTPChannelAdapter for behavior of the logger, attempts, interval,
// and client parameters.
func newHTTPApplicationAdapter(baseURL, playbackBaseURL, accessToken, username, password string, client *http.Client, logger *slog.Logger, attempts int, interval time.Duration) *httpApplicationAdapter {
	cfg := normalizeAdapterConfig(logger, attempts, interval)
	return &httpApplicationAdapter{
		baseURL:         strings.TrimRight(baseURL, "/"),
		playbackBaseURL: strings.TrimRight(playbackBaseURL, "/"),
		accessToken:     accessToken,
		username:        username,
		password:        password,
		client:          client,
		logger:          cfg.logger,
		maxAttempts:     cfg.attempts,
		retryInterval:   cfg.interval,
	}
}

// newHTTPTranscoderAdapter constructs an HTTP-based transcoderAdapter.
// See newHTTPChannelAdapter for behavior of the logger, attempts, interval,
// and client parameters.
func newHTTPTranscoderAdapter(baseURL, token string, client *http.Client, logger *slog.Logger, attempts int, interval time.Duration) *httpTranscoderAdapter {
	cfg := normalizeAdapterConfig(logger, attempts, interval)
	return &httpTranscoderAdapter{
		baseURL:       strings.TrimRight(baseURL, "/"),
		token:         token,
		client:        client,
		logger:        cfg.logger,
		maxAttempts:   cfg.attempts,
		retryInterval: cfg.interval,
	}
}

// CreateChannel provisions a new channel by calling the configured SRS
// controller.
//
// The method will retry transient failures (network errors and 5xx/429
// responses) up to maxAttempts. Callers are encouraged to pass a context
// with a deadline to bound the overall operation duration.
func (a *httpChannelAdapter) CreateChannel(ctx context.Context, channelID, streamKey string) (string, string, string, error) {
	ctx, span := tracing.Default().StartSpan(ctx, "ingest.srs.create_channel",
		tracing.StringAttr("channel.id", channelID),
	)
	if span != nil {
		defer span.End()
	}
	payload := srsChannelRequest{ChannelID: channelID, StreamKey: streamKey}
	var response srsChannelResponse
	if err := postJSON(ctx, a.client, fmt.Sprintf("%s/v1/channels", a.baseURL), payload, &response, func(req *http.Request) {
		setBearer(req, a.token)
	}, a.logger, a.maxAttempts, a.retryInterval); err != nil {
		if span != nil {
			span.RecordError(err)
		}
		return "", "", "", err
	}
	if strings.TrimSpace(response.PrimaryIngest) == "" || strings.TrimSpace(response.OriginIngest) == "" {
		return "", "", "", fmt.Errorf("SRS controller returned incomplete ingest endpoints")
	}
	return response.PrimaryIngest, response.BackupIngest, response.OriginIngest, nil
}

// DeleteChannel tears down the channel identified by channelID by calling
// the configured SRS controller.
func (a *httpChannelAdapter) DeleteChannel(ctx context.Context, channelID string) error {
	ctx, span := tracing.Default().StartSpan(ctx, "ingest.srs.delete_channel",
		tracing.StringAttr("channel.id", channelID),
	)
	if span != nil {
		defer span.End()
	}
	err := deleteRequest(ctx, a.client, fmt.Sprintf("%s/v1/channels/%s", a.baseURL, channelID), func(req *http.Request) {
		setBearer(req, a.token)
	}, a.logger, a.maxAttempts, a.retryInterval)
	if err != nil && span != nil {
		span.RecordError(err)
	}
	return err
}

// CreateApplication validates the immutable default/live application declared
// in Server.xml and derives the public LL-HLS URL for the forwarded stream.
func (a *httpApplicationAdapter) CreateApplication(ctx context.Context, channelID, originURL string, renditions []string) (string, string, error) {
	ctx, span := tracing.Default().StartSpan(ctx, "ingest.ome.validate_application",
		tracing.StringAttr("channel.id", channelID),
		tracing.StringAttr("renditions", strings.Join(renditions, ",")),
	)
	if span != nil {
		defer span.End()
	}
	channelID = strings.TrimSpace(channelID)
	originURL = strings.TrimSpace(originURL)
	if channelID == "" || originURL == "" {
		return "", "", fmt.Errorf("channel ID and origin URL are required")
	}
	if strings.TrimSpace(a.playbackBaseURL) == "" {
		return "", "", fmt.Errorf("OME public LL-HLS base URL is required")
	}
	if err := getRequest(ctx, a.client, fmt.Sprintf("%s/v1/vhosts/default/apps/live", a.baseURL), func(req *http.Request) {
		setOMEAuth(req, a.accessToken, a.username, a.password)
	}, a.logger, a.maxAttempts, a.retryInterval); err != nil {
		if span != nil {
			span.RecordError(err)
		}
		return "", "", err
	}
	playbackURL := fmt.Sprintf("%s/%s/llhls.m3u8", a.playbackBaseURL, url.PathEscape(channelID))
	return originURL, playbackURL, nil
}

// DeleteApplication intentionally leaves default/live intact because OME does
// not allow API mutation of applications declared in Server.xml. Streams are
// removed automatically when their SRS forward closes.
func (a *httpApplicationAdapter) DeleteApplication(ctx context.Context, channelID string) error {
	_, span := tracing.Default().StartSpan(ctx, "ingest.ome.release_stream",
		tracing.StringAttr("channel.id", channelID),
	)
	if span != nil {
		defer span.End()
	}
	return ctx.Err()
}

// StartJobs starts one or more live transcoding jobs for the given channel,
// session, and origin URL using the provided rendition ladder.
//
// The returned jobIDs slice may contain IDs from both JobID and JobIDs
// response fields to maintain backward compatibility with older backends.
func (a *httpTranscoderAdapter) StartJobs(ctx context.Context, channelID, sessionID, originURL string, ladder []Rendition) ([]string, []Rendition, error) {
	ctx, span := tracing.Default().StartSpan(ctx, "ingest.transcoder.start_jobs",
		tracing.StringAttr("channel.id", channelID),
		tracing.StringAttr("session.id", sessionID),
		tracing.StringAttr("origin.url", originURL),
		tracing.StringAttr("renditions", renditionsLabel(ladder)),
	)
	if span != nil {
		defer span.End()
	}
	payload := ffmpegJobRequest{
		ChannelID:  channelID,
		SessionID:  sessionID,
		OriginURL:  originURL,
		Renditions: CloneRenditions(ladder),
	}
	var response ffmpegJobResponse
	if err := postJSON(ctx, a.client, fmt.Sprintf("%s/v1/jobs", a.baseURL), payload, &response, func(req *http.Request) {
		setBearer(req, a.token)
	}, a.logger, a.maxAttempts, a.retryInterval); err != nil {
		if span != nil {
			span.RecordError(err)
		}
		return nil, nil, err
	}

	jobIDs := make([]string, 0, len(response.JobIDs)+1)
	seenJobIDs := make(map[string]struct{}, len(response.JobIDs)+1)
	for _, jobID := range append(append([]string{}, response.JobIDs...), response.JobID) {
		jobID = strings.TrimSpace(jobID)
		if jobID == "" {
			continue
		}
		if _, seen := seenJobIDs[jobID]; seen {
			continue
		}
		seenJobIDs[jobID] = struct{}{}
		jobIDs = append(jobIDs, jobID)
	}
	renditions := CloneRenditions(response.Renditions)
	return jobIDs, renditions, nil
}

// StopJob stops a live transcoding job with the specified jobID.
func (a *httpTranscoderAdapter) StopJob(ctx context.Context, jobID string) error {
	ctx, span := tracing.Default().StartSpan(ctx, "ingest.transcoder.stop_job",
		tracing.StringAttr("job.id", jobID),
	)
	if span != nil {
		defer span.End()
	}
	err := deleteRequest(ctx, a.client, fmt.Sprintf("%s/v1/jobs/%s", a.baseURL, jobID), func(req *http.Request) {
		setBearer(req, a.token)
	}, a.logger, a.maxAttempts, a.retryInterval)
	var statusErr *httpStatusError
	if errors.As(err, &statusErr) && statusErr.statusCode == http.StatusNotFound {
		return nil
	}
	if err != nil && span != nil {
		span.RecordError(err)
	}
	return err
}

// StartUpload starts a VOD transcoding/upload job for the given upload
// request. It returns a result that includes the job ID, playback URL and
// effective renditions.
//
// Renditions are defensively copied to avoid aliasing.
func (a *httpTranscoderAdapter) StartUpload(ctx context.Context, req uploadJobRequest) (uploadJobResult, error) {
	ctx, span := tracing.Default().StartSpan(ctx, "ingest.transcoder.start_upload",
		tracing.StringAttr("channel.id", req.ChannelID),
		tracing.StringAttr("upload.id", req.UploadID),
		tracing.StringAttr("source.url", req.SourceURL),
		tracing.StringAttr("renditions", renditionsLabel(req.Renditions)),
	)
	if span != nil {
		defer span.End()
	}
	payload := ffmpegUploadRequest{
		ChannelID:  req.ChannelID,
		UploadID:   req.UploadID,
		SourceURL:  req.SourceURL,
		Filename:   req.Filename,
		Renditions: CloneRenditions(req.Renditions),
	}
	var response ffmpegUploadResponse
	if err := postJSON(ctx, a.client, fmt.Sprintf("%s/v1/uploads", a.baseURL), payload, &response, func(httpReq *http.Request) {
		setBearer(httpReq, a.token)
	}, a.logger, a.maxAttempts, a.retryInterval); err != nil {
		if span != nil {
			span.RecordError(err)
		}
		return uploadJobResult{}, err
	}
	if strings.TrimSpace(response.JobID) == "" {
		return uploadJobResult{}, fmt.Errorf("transcoder upload response omitted jobId")
	}
	if err := a.waitForUpload(ctx, response.JobID); err != nil {
		if span != nil {
			span.RecordError(err)
		}
		return uploadJobResult{}, err
	}
	return uploadJobResult{
		JobID:       response.JobID,
		PlaybackURL: response.PlaybackURL,
		Renditions:  CloneRenditions(response.Renditions),
	}, nil
}

func (a *httpTranscoderAdapter) waitForUpload(ctx context.Context, jobID string) error {
	interval := a.retryInterval
	if interval <= 0 {
		interval = defaultRetryBackoff
	}
	statusURL := fmt.Sprintf("%s/v1/uploads/%s", a.baseURL, url.PathEscape(jobID))
	for {
		var response ffmpegUploadStatusResponse
		if err := getJSON(ctx, a.client, statusURL, &response, func(req *http.Request) {
			setBearer(req, a.token)
		}, a.logger, a.maxAttempts, a.retryInterval); err != nil {
			return fmt.Errorf("read upload job status: %w", err)
		}
		switch strings.ToLower(strings.TrimSpace(response.Status)) {
		case "completed":
			return nil
		case "failed":
			message := strings.TrimSpace(response.Error)
			if message == "" {
				message = "unknown transcoder failure"
			}
			return fmt.Errorf("upload job failed: %s", message)
		case "running", "accepted", "processing":
		default:
			return fmt.Errorf("upload job returned unsupported status %q", response.Status)
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func renditionsLabel(renditions []Rendition) string {
	if len(renditions) == 0 {
		return ""
	}
	names := make([]string, 0, len(renditions))
	for _, rendition := range renditions {
		if rendition.Name != "" {
			names = append(names, rendition.Name)
		}
	}
	return strings.Join(names, ",")
}

// postJSON issues an HTTP POST with a JSON payload and decodes the JSON
// response into dest (if non-nil). It uses retry semantics defined by
// doWithRetry. If client is nil, a shared default client with a default
// timeout is used.
func postJSON(ctx context.Context, client *http.Client, url string, payload interface{}, dest interface{}, mutate func(*http.Request), logger *slog.Logger, attempts int, interval time.Duration) error {
	if client == nil {
		client = defaultHTTPClient
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	return doWithRetry(ctx, client, http.MethodPost, url, body, mutate, dest, logger, attempts, interval)
}

func getJSON(ctx context.Context, client *http.Client, url string, dest interface{}, mutate func(*http.Request), logger *slog.Logger, attempts int, interval time.Duration) error {
	if client == nil {
		client = defaultHTTPClient
	}
	return doWithRetry(ctx, client, http.MethodGet, url, nil, mutate, dest, logger, attempts, interval)
}

// deleteRequest issues an HTTP DELETE request and discards any successful
// response body. It uses retry semantics defined by doWithRetry. If client
// is nil, a shared default client with a default timeout is used.
func deleteRequest(ctx context.Context, client *http.Client, url string, mutate func(*http.Request), logger *slog.Logger, attempts int, interval time.Duration) error {
	if client == nil {
		client = defaultHTTPClient
	}
	return doWithRetry(ctx, client, http.MethodDelete, url, nil, mutate, nil, logger, attempts, interval)
}

func getRequest(ctx context.Context, client *http.Client, url string, mutate func(*http.Request), logger *slog.Logger, attempts int, interval time.Duration) error {
	if client == nil {
		client = defaultHTTPClient
	}
	return doWithRetry(ctx, client, http.MethodGet, url, nil, mutate, nil, logger, attempts, interval)
}

// doWithRetry executes an HTTP request with basic retry semantics.
//
// Behavior:
//
//   - Retries on:
//
//   - Network errors (client.Do returns an error).
//
//   - HTTP 5xx responses.
//
//   - HTTP 429 (Too Many Requests).
//
//   - Does NOT retry on:
//
//   - HTTP 4xx responses other than 429 (treated as permanent errors).
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
	var shouldRetry bool

	for attempt := 1; attempt <= attempts; attempt++ {
		shouldRetry = true
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
			shouldRetry = true
		} else {
			func() {
				defer func() {
					_ = resp.Body.Close()
				}()

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
				errMsg := &httpStatusError{statusCode: statusCode, status: resp.Status, body: strings.TrimSpace(string(data))}

				// Determine if this status code is retryable.
				if isRetryableStatus(statusCode) {
					lastErr = errMsg
					shouldRetry = true
					return
				}

				// Non-retryable HTTP status (e.g., 4xx other than 429).
				lastErr = errMsg
				shouldRetry = false
			}()
		}

		if lastErr == nil {
			return nil
		}

		if !shouldRetry {
			return lastErr
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

// setOMEAuth sets OME API Basic auth using the raw rendered AccessToken as
// the full Basic credential string. Basic user/password auth is retained only
// for legacy/custom OME-compatible endpoints without an access token.
func setOMEAuth(req *http.Request, accessToken, username, password string) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(accessToken)))
		return
	}
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" && password == "" {
		return
	}
	req.SetBasicAuth(username, password)
}

// CloneRenditions returns a shallow copy of the provided renditions slice.
// If input is empty, nil is returned to avoid unnecessary allocations.
//
// The Rendition type is defined elsewhere in the ingest package and typically
// contains bitrate, resolution, and other encoding parameters.
func CloneRenditions(input []Rendition) []Rendition {
	if len(input) == 0 {
		return nil
	}
	out := make([]Rendition, len(input))
	copy(out, input)
	return out
}
