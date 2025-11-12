package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"bitriver-live/internal/ingest"
	"bitriver-live/internal/storage"
)

type UploadProcessorConfig struct {
	Store      storage.Repository
	Ingest     ingest.Controller
	Renditions []ingest.Rendition
	Workers    int
	QueueSize  int
	Timeout    time.Duration
	Logger     *slog.Logger
}

type UploadProcessor struct {
	store      storage.Repository
	ingest     ingest.Controller
	renditions []ingest.Rendition
	workers    int
	timeout    time.Duration
	logger     *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc

	queue chan string
	wg    sync.WaitGroup

	mu       sync.Mutex
	inFlight map[string]struct{}
	started  bool
}

const (
	defaultUploadWorkers   = 2
	defaultUploadQueueSize = 64
	defaultUploadTimeout   = 30 * time.Minute
)

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
		store:      cfg.Store,
		ingest:     cfg.Ingest,
		renditions: cloneUploadRenditions(cfg.Renditions),
		workers:    workers,
		timeout:    timeout,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		queue:      make(chan string, queueSize),
		inFlight:   make(map[string]struct{}),
	}
	return processor
}

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

	go p.recoverPending()
}

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

func (p *UploadProcessor) beginWork(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.inFlight[id]; exists {
		return false
	}
	p.inFlight[id] = struct{}{}
	return true
}

func (p *UploadProcessor) finishWork(id string) {
	p.mu.Lock()
	delete(p.inFlight, id)
	p.mu.Unlock()
}

func (p *UploadProcessor) recoverPending() {
	if p.store == nil {
		return
	}
	channels := p.store.ListChannels("", "")
	for _, channel := range channels {
		select {
		case <-p.ctx.Done():
			return
		default:
		}
		uploads, err := p.store.ListUploads(channel.ID)
		if err != nil {
			p.logger.Error("failed to list uploads", "channel_id", channel.ID, "error", err)
			continue
		}
		for _, upload := range uploads {
			status := strings.ToLower(strings.TrimSpace(upload.Status))
			if status == "pending" || status == "processing" {
				p.Enqueue(upload.ID)
			}
		}
	}
}

func (p *UploadProcessor) processUpload(id string) {
	if p.store == nil {
		return
	}
	upload, ok := p.store.GetUpload(id)
	if !ok {
		return
	}
	status := strings.ToLower(strings.TrimSpace(upload.Status))
	if status == "ready" || status == "completed" || status == "failed" {
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
		p.failUpload(id, "", fmt.Errorf("source URL is required"))
		return
	}

	processing := "processing"
	progress := 10
	metadata := map[string]string{"sourceUrl": source}
	if _, err := p.store.UpdateUpload(id, storage.UploadUpdate{
		Status:   &processing,
		Progress: &progress,
		Metadata: metadata,
		Error:    stringPtr(""),
	}); err != nil {
		p.logger.Error("failed to mark upload processing", "upload_id", id, "error", err)
		return
	}

	if p.ingest == nil {
		p.failUpload(id, source, fmt.Errorf("ingest controller unavailable"))
		return
	}

	ctx, cancel := context.WithTimeout(p.ctx, p.timeout)
	defer cancel()
	result, err := p.ingest.TranscodeUpload(ctx, ingest.UploadTranscodeParams{
		ChannelID:  upload.ChannelID,
		UploadID:   upload.ID,
		SourceURL:  source,
		Filename:   upload.Filename,
		Renditions: cloneUploadRenditions(p.renditions),
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			if ctxErr := ctx.Err(); ctxErr != nil && !errors.Is(err, ctxErr) {
				err = ctxErr
			}
		}
		p.failUpload(id, source, err)
		return
	}

	ready := "ready"
	progress = 100
	playbackURL := strings.TrimSpace(result.PlaybackURL)
	if playbackURL == "" {
		playbackURL = source
	}
	completedAt := time.Now().UTC()
	metadata = map[string]string{"sourceUrl": source}
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
	if _, err := p.store.UpdateUpload(id, storage.UploadUpdate{
		Status:      &ready,
		Progress:    &progress,
		PlaybackURL: &playbackURL,
		Metadata:    metadata,
		CompletedAt: &completedAt,
		Error:       stringPtr(""),
	}); err != nil {
		p.logger.Error("failed to mark upload ready", "upload_id", id, "error", err)
		return
	}
	p.logger.Info("upload transcoded", "upload_id", id, "channel_id", upload.ChannelID, "playback_url", playbackURL)
}

func (p *UploadProcessor) failUpload(id, source string, err error) {
	if p.store == nil {
		return
	}
	failed := "failed"
	progress := 0
	message := strings.TrimSpace(err.Error())
	metadata := map[string]string{}
	if source != "" {
		metadata["sourceUrl"] = source
	}
	if _, updateErr := p.store.UpdateUpload(id, storage.UploadUpdate{
		Status:   &failed,
		Progress: &progress,
		Metadata: metadata,
		Error:    &message,
	}); updateErr != nil {
		p.logger.Error("failed to update failed upload", "upload_id", id, "error", updateErr, "failure", err)
		return
	}
	p.logger.Error("upload transcode failed", "upload_id", id, "error", err)
}

func cloneUploadRenditions(r []ingest.Rendition) []ingest.Rendition {
	if len(r) == 0 {
		return nil
	}
	out := make([]ingest.Rendition, len(r))
	copy(out, r)
	return out
}

func stringPtr(s string) *string {
	return &s
}
