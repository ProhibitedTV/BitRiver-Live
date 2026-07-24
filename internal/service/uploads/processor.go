package uploads

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/ingest"
)

// UploadStore exposes only the upload-related persistence operations required
// by UploadProcessor. It intentionally omits unrelated repository methods so
// that upload processing stays decoupled from broader storage concerns.
type Store interface {
	ListPendingUploads(ctx context.Context, limit int) ([]domain.Upload, error)
	GetUpload(ctx context.Context, id string) (domain.Upload, bool)
	EnsureUploadRecording(ctx context.Context, uploadID string, playbackURL string, completedAt time.Time) (string, error)
	UpdateUpload(ctx context.Context, id string, update domain.UploadUpdate) (domain.Upload, error)
}

// UploadIngestClient captures the ingest functionality needed to process
// uploads.
type IngestClient interface {
	TranscodeUpload(ctx context.Context, params ingest.UploadTranscodeParams) (ingest.UploadTranscodeResult, error)
}

// UploadProcessorConfig describes the collaborators and tunable settings used
// to process archived uploads, including storage, ingest coordination, worker
// concurrency, and back pressure limits.
type UploadProcessorConfig struct {
	Store                 Store
	Ingest                IngestClient
	Cleaner               SourceArtifactCleaner
	Renditions            []ingest.Rendition
	Workers               int
	QueueSize             int
	Timeout               time.Duration
	ReadySourceRetention  time.Duration
	FailedSourceRetention time.Duration
	Logger                *slog.Logger
}

// SourceArtifactCleaner removes persisted upload source artifacts once the
// processor reaches a status that permits cleanup.
type SourceArtifactCleaner interface {
	Delete(ctx context.Context, upload domain.Upload, sourceKey string) error
}

// UploadProcessor runs background workers that resolve pending uploads by
// coordinating persistence, ingest, and rendition generation while honoring
// queue limits and cancellation.
type UploadProcessor struct {
	store                 Store
	ingest                IngestClient
	renditions            []ingest.Rendition
	workers               int
	timeout               time.Duration
	logger                *slog.Logger
	cleaner               SourceArtifactCleaner
	readySourceRetention  time.Duration
	failedSourceRetention time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	queue chan string
	wg    sync.WaitGroup

	mu       sync.Mutex
	inFlight map[string]struct{}
	started  bool
}

// Enqueuer captures the queue interaction the API layer needs to trigger upload processing.
type Enqueuer interface {
	Enqueue(id string)
}

const (
	defaultUploadWorkers         = 2
	defaultUploadQueueSize       = 64
	defaultUploadTimeout         = 30 * time.Minute
	uploadRetryBaseDelay         = 50 * time.Millisecond
	uploadMaxRetryAttempts       = 3
	defaultFailedSourceRetention = 24 * time.Hour

	metadataRetryAttemptKey = "retryAttempt"
	metadataNextRetryAtKey  = "nextRetryAt"
)

var errorStatusCodePattern = regexp.MustCompile(`\b([1-5][0-9]{2})\b`)

// NewUploadProcessor configures a worker pool for upload processing, applying
// sensible defaults for worker count, queue size, timeout, and logging when
// the configuration omits them.
func NewUploadProcessor(cfg UploadProcessorConfig) *UploadProcessor {
	workers := cfg.Workers
	if workers <= 0 {
		workers = defaultUploadWorkers
	}
	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = defaultUploadQueueSize
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultUploadTimeout
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	processor := &UploadProcessor{
		store:                 cfg.Store,
		ingest:                cfg.Ingest,
		cleaner:               cfg.Cleaner,
		renditions:            ingest.CloneRenditions(cfg.Renditions),
		workers:               workers,
		timeout:               timeout,
		logger:                logger,
		readySourceRetention:  maxDuration(cfg.ReadySourceRetention, 0),
		failedSourceRetention: maxDuration(cfg.FailedSourceRetention, defaultFailedSourceRetention),
		ctx:                   ctx,
		cancel:                cancel,
		queue:                 make(chan string, queueSize),
		inFlight:              make(map[string]struct{}),
	}
	return processor
}

// Start performs start and returns an error when dependent systems reject the operation.
func (p *UploadProcessor) Start() {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return
	}
	p.started = true
	p.mu.Unlock()

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}

	p.wg.Add(1)
	go p.recoverPending()
}

// Shutdown performs shutdown and returns an error when dependent systems reject the operation.
func (p *UploadProcessor) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.cancel()
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Enqueue performs enqueue and returns an error when dependent systems reject the operation.
func (p *UploadProcessor) Enqueue(id string) {
	if p == nil || strings.TrimSpace(id) == "" {
		return
	}
	select {
	case <-p.ctx.Done():
		return
	default:
	}
	select {
	case p.queue <- id:
	case <-p.ctx.Done():
	}
}

// worker performs worker and propagates validation or dependency failures to the caller.
func (p *UploadProcessor) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case id := <-p.queue:
			if strings.TrimSpace(id) == "" {
				continue
			}
			if !p.beginWork(id) {
				continue
			}
			p.processUpload(id)
			p.finishWork(id)
		}
	}
}

// beginWork performs begin work and propagates validation or dependency failures to the caller.
func (p *UploadProcessor) beginWork(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.inFlight[id]; exists {
		return false
	}
	p.inFlight[id] = struct{}{}
	return true
}

// finishWork performs finish work and propagates validation or dependency failures to the caller.
func (p *UploadProcessor) finishWork(id string) {
	p.mu.Lock()
	delete(p.inFlight, id)
	p.mu.Unlock()
}

// recoverPending performs recover pending and propagates validation or dependency failures to the caller.
func (p *UploadProcessor) recoverPending() {
	defer p.wg.Done()

	if p.store == nil {
		return
	}
	uploads, err := p.store.ListPendingUploads(p.ctx, 0)
	if err != nil {
		p.logger.Error("failed to list pending uploads", "error", err)
	}
	for _, upload := range uploads {
		select {
		case <-p.ctx.Done():
			return
		default:
		}
		p.Enqueue(upload.ID)
	}
}

// processUpload performs process upload and propagates validation or dependency failures to the caller.
func (p *UploadProcessor) processUpload(id string) {
	if p.store == nil {
		return
	}
	upload, ok := p.store.GetUpload(p.ctx, id)
	if !ok {
		return
	}
	status := strings.ToLower(strings.TrimSpace(upload.Status))
	if status == "ready" || status == "completed" || status == "failed" {
		p.scheduleSourceCleanup(upload, status)
		return
	}
	source := strings.TrimSpace(upload.Metadata["sourceUrl"])
	if source == "" {
		source = strings.TrimSpace(upload.Metadata["sourceURL"])
	}
	if source == "" {
		source = strings.TrimSpace(upload.PlaybackURL)
	}
	if source == "" {
		p.failUpload(upload, fmt.Errorf("source URL is required"), 0, time.Time{})
		return
	}

	processing := "processing"
	progress := 10
	metadata := cloneMetadata(upload.Metadata)
	metadata["sourceUrl"] = source
	var processingUpload domain.Upload
	if err := p.retryPersistence("mark upload processing", id, func() error {
		var updateErr error
		processingUpload, updateErr = p.store.UpdateUpload(p.ctx, id, domain.UploadUpdate{
			Status:   &processing,
			Progress: &progress,
			Metadata: metadata,
			Error:    stringPtr(""),
		})
		return updateErr
	}); err != nil {
		p.logger.Error("failed to mark upload processing", "upload_id", id, "error", err)
		return
	}
	upload = processingUpload

	if p.ingest == nil {
		p.failUpload(upload, fmt.Errorf("ingest controller unavailable"), 0, time.Time{})
		return
	}

	ctx, cancel := context.WithTimeout(p.ctx, p.timeout)
	defer cancel()
	result, err := p.ingest.TranscodeUpload(ctx, ingest.UploadTranscodeParams{
		ChannelID:  upload.ChannelID,
		UploadID:   upload.ID,
		SourceURL:  source,
		Filename:   upload.Filename,
		Renditions: ingest.CloneRenditions(p.renditions),
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			if ctxErr := ctx.Err(); ctxErr != nil && !errors.Is(err, ctxErr) {
				err = ctxErr
			}
		}
		p.handleTranscodeError(upload, source, err)
		return
	}

	ready := "ready"
	progress = 100
	playbackURL := strings.TrimSpace(result.PlaybackURL)
	if playbackURL == "" {
		playbackURL = source
	}
	completedAt := time.Now().UTC()
	metadata = cloneMetadata(upload.Metadata)
	metadata["sourceUrl"] = source
	delete(metadata, metadataRetryAttemptKey)
	delete(metadata, metadataNextRetryAtKey)
	if result.JobID != "" {
		metadata["transcodeJobId"] = result.JobID
	}
	if len(result.Renditions) > 0 {
		names := make([]string, 0, len(result.Renditions))
		for _, rendition := range result.Renditions {
			if name := strings.TrimSpace(rendition.Name); name != "" {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			metadata["renditions"] = strings.Join(names, ",")
		}
	}
	metadata["playbackUrl"] = playbackURL
	var recordingID string
	if err := p.retryPersistence("ensure upload recording", id, func() error {
		var ensureErr error
		recordingID, ensureErr = p.store.EnsureUploadRecording(p.ctx, id, playbackURL, completedAt)
		return ensureErr
	}); err != nil {
		p.logger.Error("failed to ensure upload recording", "upload_id", id, "error", err)
		upload.Metadata = metadata
		upload.PlaybackURL = playbackURL
		p.failUpload(upload, err, uploadMaxRetryAttempts, time.Time{})
		return
	}
	if recordingID != "" {
		metadata["recordingId"] = recordingID
	}
	var updatedUpload domain.Upload
	if err := p.retryPersistence("mark upload ready", id, func() error {
		var updateErr error
		updatedUpload, updateErr = p.store.UpdateUpload(p.ctx, id, domain.UploadUpdate{
			Status:      &ready,
			Progress:    &progress,
			RecordingID: &recordingID,
			PlaybackURL: &playbackURL,
			Metadata:    metadata,
			CompletedAt: &completedAt,
			Error:       stringPtr(""),
		})
		return updateErr
	}); err != nil {
		p.logger.Error("failed to mark upload ready", "upload_id", id, "error", err)
		upload.Metadata = metadata
		upload.PlaybackURL = playbackURL
		upload.RecordingID = &recordingID
		p.failUpload(upload, err, uploadMaxRetryAttempts, time.Time{})
		return
	}
	p.scheduleSourceCleanup(updatedUpload, ready)
	p.logger.Info("upload transcoded", "upload_id", id, "channel_id", upload.ChannelID, "playback_url", playbackURL)
}

// scheduleRetry performs schedule retry and propagates validation or dependency failures to the caller.
func (p *UploadProcessor) scheduleRetry(id string, delay time.Duration) {
	if p == nil || strings.TrimSpace(id) == "" {
		return
	}
	select {
	case <-p.ctx.Done():
		return
	default:
	}
	if delay < 0 {
		delay = 0
	}
	timer := time.NewTimer(delay)
	go func() {
		defer timer.Stop()
		select {
		case <-p.ctx.Done():
			return
		case <-timer.C:
		}
		p.Enqueue(id)
	}()
}

func (p *UploadProcessor) retryPersistence(operation string, uploadID string, fn func() error) error {
	if p == nil || fn == nil {
		return fmt.Errorf("%s persistence unavailable", strings.TrimSpace(operation))
	}
	operation = strings.TrimSpace(operation)
	if operation == "" {
		operation = "upload"
	}
	var lastErr error
	for attempt := 1; attempt <= uploadMaxRetryAttempts; attempt++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt >= uploadMaxRetryAttempts {
			break
		}
		delay := uploadRetryBaseDelay * time.Duration(1<<(attempt-1))
		p.logger.Warn("upload persistence transient failure",
			"upload_id", uploadID,
			"operation", operation,
			"attempt", attempt,
			"next_retry_in", delay,
			"error", lastErr,
		)
		timer := time.NewTimer(delay)
		select {
		case <-p.ctx.Done():
			timer.Stop()
			return p.ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("%s persistence retry budget exhausted after %d attempts: %w", operation, uploadMaxRetryAttempts, lastErr)
}

func (p *UploadProcessor) handleTranscodeError(upload domain.Upload, source string, err error) {
	attempt := parseRetryAttempt(upload.Metadata) + 1
	if !isTransientTranscodeError(err) {
		p.failUpload(upload, err, attempt, time.Time{})
		return
	}
	if attempt >= uploadMaxRetryAttempts {
		p.failUpload(upload, fmt.Errorf("retry budget exhausted after %d attempts: %w", attempt, err), attempt, time.Time{})
		return
	}

	delay := uploadRetryBaseDelay * time.Duration(1<<(attempt-1))
	nextRetry := time.Now().UTC().Add(delay)
	message := strings.TrimSpace(err.Error())
	status := "pending"
	progress := 0
	metadata := cloneMetadata(upload.Metadata)
	metadata["sourceUrl"] = source
	metadata[metadataRetryAttemptKey] = strconv.Itoa(attempt)
	metadata[metadataNextRetryAtKey] = nextRetry.Format(time.RFC3339Nano)
	if updateErr := p.retryPersistence("mark upload for retry", upload.ID, func() error {
		_, persistErr := p.store.UpdateUpload(p.ctx, upload.ID, domain.UploadUpdate{
			Status:   &status,
			Progress: &progress,
			Metadata: metadata,
			Error:    &message,
		})
		return persistErr
	}); updateErr != nil {
		p.logger.Error("failed to mark upload for retry", "upload_id", upload.ID, "error", updateErr, "failure", err)
		return
	}
	p.logger.Warn("upload transcode transient failure", "upload_id", upload.ID, "attempt", attempt, "next_retry_at", nextRetry, "error", err)
	p.scheduleRetry(upload.ID, delay)
}

// failUpload performs fail upload and propagates validation or dependency failures to the caller.
func (p *UploadProcessor) failUpload(upload domain.Upload, err error, attempt int, nextRetry time.Time) {
	if p.store == nil {
		return
	}
	id := strings.TrimSpace(upload.ID)
	if id == "" {
		return
	}
	failed := "failed"
	progress := 0
	message := strings.TrimSpace(err.Error())
	metadata := cloneMetadata(upload.Metadata)
	source := strings.TrimSpace(metadata["sourceUrl"])
	if source != "" {
		metadata["sourceUrl"] = source
	}
	if attempt > 0 {
		metadata[metadataRetryAttemptKey] = strconv.Itoa(attempt)
	}
	if !nextRetry.IsZero() {
		metadata[metadataNextRetryAtKey] = nextRetry.UTC().Format(time.RFC3339Nano)
	}
	updatedUpload, updateErr := p.store.UpdateUpload(p.ctx, id, domain.UploadUpdate{
		Status:   &failed,
		Progress: &progress,
		Metadata: metadata,
		Error:    &message,
	})
	if updateErr != nil {
		p.logger.Error("failed to update failed upload", "upload_id", id, "error", updateErr, "failure", err)
		return
	}
	p.scheduleSourceCleanup(updatedUpload, failed)
	p.logger.Error("upload transcode failed", "upload_id", id, "error", err)
}

func (p *UploadProcessor) scheduleSourceCleanup(upload domain.Upload, status string) {
	if p == nil || p.cleaner == nil {
		return
	}
	key := strings.TrimSpace(upload.Metadata["sourceObjectKey"])
	if key == "" {
		key = strings.TrimSpace(upload.Metadata["mediaPath"])
	}
	if key == "" {
		return
	}
	retention, ok := p.retentionForStatus(status)
	if !ok {
		return
	}
	delay := retention
	if delay < 0 {
		delay = 0
	}
	go func() {
		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-p.ctx.Done():
				return
			case <-timer.C:
			}
		}
		if err := p.cleaner.Delete(p.ctx, upload, key); err != nil {
			p.logger.Warn("cleanup upload source artifact failed", "upload_id", upload.ID, "status", status, "source_key", key, "error", err)
			return
		}
		p.logger.Info("cleanup upload source artifact", "upload_id", upload.ID, "status", status, "source_key", key)
	}()
}

func (p *UploadProcessor) retentionForStatus(status string) (time.Duration, bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "completed":
		return p.readySourceRetention, true
	case "failed":
		return p.failedSourceRetention, true
	default:
		return 0, false
	}
}

func maxDuration(value, fallback time.Duration) time.Duration {
	if value < 0 {
		return 0
	}
	if value == 0 {
		return fallback
	}
	return value
}

func parseRetryAttempt(metadata map[string]string) int {
	if metadata == nil {
		return 0
	}
	raw := strings.TrimSpace(metadata[metadataRetryAttemptKey])
	if raw == "" {
		return 0
	}
	attempt, err := strconv.Atoi(raw)
	if err != nil || attempt < 0 {
		return 0
	}
	return attempt
}

func isTransientTranscodeError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	statusCode := statusCodeFromError(err.Error())
	if statusCode == 0 {
		return false
	}
	if statusCode == 429 || (statusCode >= 500 && statusCode <= 599) {
		return true
	}
	if statusCode >= 400 && statusCode <= 499 {
		return false
	}
	return true
}

func statusCodeFromError(message string) int {
	matches := errorStatusCodePattern.FindStringSubmatch(message)
	if len(matches) < 2 {
		return 0
	}
	statusCode, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return statusCode
}

func cloneMetadata(src map[string]string) map[string]string {
	if src == nil {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

// stringPtr returns a stable string form for flag and log output.
func stringPtr(s string) *string {
	return &s
}
