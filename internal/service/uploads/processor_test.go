package uploads

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/ingest"
)

func TestUploadProcessorStartShutdown(t *testing.T) {
	store := newFakeUploadStore()
	store.uploads = map[string]domain.Upload{
		"upload-1": {
			ID:        "upload-1",
			ChannelID: "channel-1",
			Status:    "pending",
			Metadata:  map[string]string{"sourceUrl": "https://example.com/a.mp4"},
		},
		"upload-2": {
			ID:        "upload-2",
			ChannelID: "channel-1",
			Status:    "processing",
			Metadata:  map[string]string{"sourceUrl": "https://example.com/b.mp4"},
		},
		"upload-3": {
			ID:        "upload-3",
			ChannelID: "channel-1",
			Status:    "pending",
			Metadata:  map[string]string{"sourceUrl": "https://example.com/c.mp4"},
		},
		"upload-4": {
			ID:        "upload-4",
			ChannelID: "channel-1",
			Status:    "ready",
			Metadata:  map[string]string{"sourceUrl": "https://example.com/d.mp4"},
		},
	}

	ingestFake := newFakeIngest()
	ingestFake.setResult("upload-1", ingest.UploadTranscodeResult{PlaybackURL: "https://vod.example.com/a.m3u8"}, nil)
	ingestFake.setResult("upload-2", ingest.UploadTranscodeResult{PlaybackURL: "https://vod.example.com/b.m3u8"}, nil)
	ingestFake.setResult("upload-3", ingest.UploadTranscodeResult{PlaybackURL: "https://vod.example.com/c.m3u8"}, nil)

	upload1Updates := store.updatesFor("upload-1")
	upload2Updates := store.updatesFor("upload-2")
	upload3Updates := store.updatesFor("upload-3")
	ingest1Done := ingestFake.completion("upload-1")
	ingest2Done := ingestFake.completion("upload-2")
	ingest3Done := ingestFake.completion("upload-3")

	processor := NewUploadProcessor(UploadProcessorConfig{
		Store:     store,
		Ingest:    ingestFake,
		Workers:   3,
		QueueSize: 8,
		Timeout:   time.Second,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	processor.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := processor.Shutdown(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("shutdown error: %v", err)
		}
	})

	processor.Enqueue("upload-3")
	processor.Enqueue("upload-3")

	waitForCompletion(t, ingest1Done, "upload-1", 2*time.Second)
	waitForCompletion(t, ingest2Done, "upload-2", 2*time.Second)
	waitForCompletion(t, ingest3Done, "upload-3", 2*time.Second)

	waitForUploadUpdate(t, upload1Updates, time.Second, func(upload domain.Upload) bool {
		return upload.Status == "ready" && upload.PlaybackURL == "https://vod.example.com/a.m3u8" && upload.Progress == 100 && upload.RecordingID != nil && *upload.RecordingID != ""
	})
	waitForUploadUpdate(t, upload2Updates, time.Second, func(upload domain.Upload) bool {
		return upload.Status == "ready" && upload.PlaybackURL == "https://vod.example.com/b.m3u8" && upload.Progress == 100
	})
	waitForUploadUpdate(t, upload3Updates, time.Second, func(upload domain.Upload) bool {
		return upload.Status == "ready" && upload.PlaybackURL == "https://vod.example.com/c.m3u8" && upload.Progress == 100 && upload.RecordingID != nil && *upload.RecordingID != ""
	})
	if len(store.recordings) != 3 {
		t.Fatalf("expected 3 recordings linked to uploads, got %d", len(store.recordings))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := processor.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}
}

func TestUploadProcessorFailUpload(t *testing.T) {
	store := newFakeUploadStore()
	store.uploads = map[string]domain.Upload{
		"upload-err": {
			ID:        "upload-err",
			ChannelID: "channel-1",
			Status:    "pending",
			Metadata:  map[string]string{"sourceUrl": "https://example.com/error.mp4"},
		},
	}

	ingestFake := newFakeIngest()
	ingestFake.setResult("upload-err", ingest.UploadTranscodeResult{}, errors.New("transcode failed"))
	errUpdates := store.updatesFor("upload-err")
	ingestDone := ingestFake.completion("upload-err")

	processor := NewUploadProcessor(UploadProcessorConfig{
		Store:   store,
		Ingest:  ingestFake,
		Workers: 1,
		Timeout: time.Second,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	processor.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := processor.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown error: %v", err)
		}
	}()

	processor.Enqueue("upload-err")

	waitForCompletion(t, ingestDone, "upload-err", time.Second)
	waitForUploadUpdate(t, errUpdates, time.Second, func(upload domain.Upload) bool {
		return upload.Status == "failed" && upload.Progress == 0 && upload.Metadata["sourceUrl"] == "https://example.com/error.mp4" && upload.Error == "transcode failed"
	})
}

func TestUploadProcessorTimeout(t *testing.T) {
	store := newFakeUploadStore()
	store.uploads = map[string]domain.Upload{
		"upload-slow": {
			ID:        "upload-slow",
			ChannelID: "channel-1",
			Status:    "pending",
			Metadata:  map[string]string{"sourceUrl": "https://example.com/slow.mp4"},
		},
	}

	ingestFake := newFakeIngest()
	ingestFake.setDelay("upload-slow", 200*time.Millisecond)
	slowUpdates := store.updatesFor("upload-slow")

	processor := NewUploadProcessor(UploadProcessorConfig{
		Store:   store,
		Ingest:  ingestFake,
		Workers: 1,
		Timeout: 50 * time.Millisecond,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	processor.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := processor.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown error: %v", err)
		}
	}()

	processor.Enqueue("upload-slow")

	waitForIngestCallCount(t, ingestFake, "upload-slow", 3, 3*time.Second)
	waitForUploadUpdate(t, slowUpdates, 3*time.Second, func(upload domain.Upload) bool {
		if upload.Status != "failed" || upload.Progress != 0 {
			return false
		}
		if upload.Metadata["sourceUrl"] != "https://example.com/slow.mp4" {
			return false
		}
		return strings.Contains(upload.Error, "retry budget exhausted") && upload.Metadata["retryAttempt"] == "3"
	})
}

func TestUploadProcessorRetryUpdateFailures(t *testing.T) {
	store := newFakeUploadStore()
	store.uploads = map[string]domain.Upload{
		"upload-retry": {
			ID:        "upload-retry",
			ChannelID: "channel-1",
			Status:    "pending",
			Metadata:  map[string]string{"sourceUrl": "https://example.com/retry.mp4"},
		},
	}
	store.failFirstUpdateFor("upload-retry", errors.New("transient store failure"))

	ingestFake := newFakeIngest()
	ingestFake.setResult("upload-retry", ingest.UploadTranscodeResult{PlaybackURL: "https://vod.example.com/retry.m3u8"}, nil)
	retryUpdates := store.updatesFor("upload-retry")
	retryDone := ingestFake.completion("upload-retry")

	processor := NewUploadProcessor(UploadProcessorConfig{
		Store:   store,
		Ingest:  ingestFake,
		Workers: 1,
		Timeout: time.Second,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	processor.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := processor.Shutdown(ctx); err != nil {
			t.Fatalf("shutdown error: %v", err)
		}
	})

	processor.Enqueue("upload-retry")

	waitForUploadUpdate(t, retryUpdates, time.Second, func(domain.Upload) bool {
		return store.updateAttemptCount("upload-retry") >= 1
	})

	waitForCompletion(t, retryDone, "upload-retry", 3*time.Second)
	waitForUploadUpdate(t, retryUpdates, 3*time.Second, func(upload domain.Upload) bool {
		return upload.Status == "ready" && upload.PlaybackURL == "https://vod.example.com/retry.m3u8" && upload.Progress == 100
	})

	if count := ingestFake.callCount("upload-retry"); count != 1 {
		t.Fatalf("expected single ingest call, got %d", count)
	}

	if attempts := store.updateAttemptCount("upload-retry"); attempts < 3 {
		t.Fatalf("expected at least 3 update attempts, got %d", attempts)
	}
}

func TestUploadProcessorTransientRetryProgression(t *testing.T) {
	store := newFakeUploadStore()
	store.uploads = map[string]domain.Upload{
		"upload-transient": {
			ID:        "upload-transient",
			ChannelID: "channel-1",
			Status:    "pending",
			Metadata:  map[string]string{"sourceUrl": "https://example.com/transient.mp4"},
		},
	}

	ingestFake := newFakeIngest()
	ingestFake.setErrorSequence("upload-transient", []error{
		errors.New("503 service unavailable"),
		errors.New("429 too many requests"),
		nil,
	})
	ingestFake.setResult("upload-transient", ingest.UploadTranscodeResult{PlaybackURL: "https://vod.example.com/transient.m3u8"}, nil)
	updates := store.updatesFor("upload-transient")

	processor := NewUploadProcessor(UploadProcessorConfig{
		Store:   store,
		Ingest:  ingestFake,
		Workers: 1,
		Timeout: 100 * time.Millisecond,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	processor.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = processor.Shutdown(ctx)
	})

	processor.Enqueue("upload-transient")
	waitForIngestCallCount(t, ingestFake, "upload-transient", 3, 3*time.Second)
	waitForUploadUpdate(t, updates, 3*time.Second, func(upload domain.Upload) bool {
		return upload.Status == "ready" && upload.Progress == 100 && upload.Metadata["retryAttempt"] == "" && upload.Metadata["nextRetryAt"] == ""
	})
}

func TestUploadProcessorTerminalFailureAfterRetryBudget(t *testing.T) {
	store := newFakeUploadStore()
	store.uploads = map[string]domain.Upload{
		"upload-budget": {
			ID:        "upload-budget",
			ChannelID: "channel-1",
			Status:    "pending",
			Metadata:  map[string]string{"sourceUrl": "https://example.com/budget.mp4"},
		},
	}

	ingestFake := newFakeIngest()
	ingestFake.setErrorSequence("upload-budget", []error{
		errors.New("503 service unavailable"),
		errors.New("503 service unavailable"),
		errors.New("503 service unavailable"),
	})
	updates := store.updatesFor("upload-budget")

	processor := NewUploadProcessor(UploadProcessorConfig{
		Store:   store,
		Ingest:  ingestFake,
		Workers: 1,
		Timeout: 100 * time.Millisecond,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	processor.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = processor.Shutdown(ctx)
	})

	processor.Enqueue("upload-budget")
	waitForIngestCallCount(t, ingestFake, "upload-budget", 3, 3*time.Second)
	waitForUploadUpdate(t, updates, 3*time.Second, func(upload domain.Upload) bool {
		return upload.Status == "failed" && strings.Contains(upload.Error, "retry budget exhausted") && upload.Metadata["retryAttempt"] == "3"
	})
}

type fakeUploadStore struct {
	mu              sync.Mutex
	uploads         map[string]domain.Upload
	recordings      map[string]domain.Recording
	failFirstUpdate map[string]error
	updateAttempts  map[string]int
	updateCh        map[string]chan domain.Upload
}

func newFakeUploadStore() *fakeUploadStore {
	return &fakeUploadStore{
		uploads:         make(map[string]domain.Upload),
		recordings:      make(map[string]domain.Recording),
		failFirstUpdate: make(map[string]error),
		updateAttempts:  make(map[string]int),
		updateCh:        make(map[string]chan domain.Upload),
	}
}

func (f *fakeUploadStore) ListPendingUploads(ctx context.Context, limit int) ([]domain.Upload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	pending := make([]domain.Upload, 0)
	for _, upload := range f.uploads {
		select {
		case <-ctx.Done():
			return pending, ctx.Err()
		default:
		}
		status := strings.ToLower(strings.TrimSpace(upload.Status))
		if status != "pending" && status != "processing" {
			continue
		}
		pending = append(pending, cloneUpload(upload))
		if limit > 0 && len(pending) >= limit {
			break
		}
	}
	return pending, nil
}

func (f *fakeUploadStore) GetUpload(ctx context.Context, id string) (domain.Upload, bool) {
	select {
	case <-ctx.Done():
		return domain.Upload{}, false
	default:
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	upload, ok := f.uploads[id]
	if !ok {
		return domain.Upload{}, false
	}
	return cloneUpload(upload), true
}

func (f *fakeUploadStore) updatesFor(id string) <-chan domain.Upload {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch, ok := f.updateCh[id]
	if !ok {
		ch = make(chan domain.Upload, 8)
		f.updateCh[id] = ch
	}
	return ch
}

func (f *fakeUploadStore) EnsureUploadRecording(ctx context.Context, id string, playbackURL string, completedAt time.Time) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	upload, ok := f.uploads[id]
	if !ok {
		return "", errors.New("upload not found")
	}
	if upload.RecordingID != nil && strings.TrimSpace(*upload.RecordingID) != "" {
		return strings.TrimSpace(*upload.RecordingID), nil
	}
	recordingID := "recording-" + id
	f.recordings[recordingID] = domain.Recording{
		ID:              recordingID,
		ChannelID:       upload.ChannelID,
		SessionID:       "upload-" + id,
		Title:           upload.Title,
		DurationSeconds: 0,
		PlaybackBaseURL: playbackURL,
		CreatedAt:       completedAt,
	}
	return recordingID, nil
}

func (f *fakeUploadStore) UpdateUpload(ctx context.Context, id string, update domain.UploadUpdate) (domain.Upload, error) {
	select {
	case <-ctx.Done():
		return domain.Upload{}, ctx.Err()
	default:
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	upload, ok := f.uploads[id]
	if !ok {
		return domain.Upload{}, errors.New("upload not found")
	}
	attempt := f.updateAttempts[id]
	f.updateAttempts[id] = attempt + 1
	if err, shouldFail := f.failFirstUpdate[id]; shouldFail && attempt == 0 {
		delete(f.failFirstUpdate, id)
		return domain.Upload{}, err
	}
	if update.Status != nil {
		upload.Status = *update.Status
	}
	if update.Progress != nil {
		upload.Progress = *update.Progress
	}
	if update.PlaybackURL != nil {
		upload.PlaybackURL = *update.PlaybackURL
	}
	if update.RecordingID != nil {
		recordingID := strings.TrimSpace(*update.RecordingID)
		if recordingID == "" {
			upload.RecordingID = nil
		} else {
			upload.RecordingID = &recordingID
		}
	}
	if update.Metadata != nil {
		upload.Metadata = cloneMetadataMap(update.Metadata)
	}
	if update.Error != nil {
		upload.Error = *update.Error
	}
	if update.CompletedAt != nil {
		upload.CompletedAt = update.CompletedAt
	}
	f.uploads[id] = upload
	if ch, ok := f.updateCh[id]; ok {
		select {
		case ch <- cloneUpload(upload):
		default:
		}
	}
	return cloneUpload(upload), nil
}

func (f *fakeUploadStore) failFirstUpdateFor(id string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err == nil {
		err = errors.New("forced update failure")
	}
	f.failFirstUpdate[id] = err
	f.updateAttempts[id] = 0
}

func (f *fakeUploadStore) updateAttemptCount(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.updateAttempts[id]
}

func cloneMetadataMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneUpload(upload domain.Upload) domain.Upload {
	upload.Metadata = cloneMetadataMap(upload.Metadata)
	return upload
}

var _ Store = (*fakeUploadStore)(nil)

type fakeIngest struct {
	mu            sync.Mutex
	results       map[string]ingest.UploadTranscodeResult
	errors        map[string]error
	errorSequence map[string][]error
	delays        map[string]time.Duration
	callTotal     map[string]int
	done          map[string]chan struct{}
}

func newFakeIngest() *fakeIngest {
	return &fakeIngest{
		results:       make(map[string]ingest.UploadTranscodeResult),
		errors:        make(map[string]error),
		errorSequence: make(map[string][]error),
		delays:        make(map[string]time.Duration),
		callTotal:     make(map[string]int),
		done:          make(map[string]chan struct{}),
	}
}

func (f *fakeIngest) setResult(id string, result ingest.UploadTranscodeResult, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results[id] = result
	if err != nil {
		f.errors[id] = err
	} else {
		delete(f.errors, id)
	}
}

func (f *fakeIngest) setErrorSequence(id string, errs []error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	copied := make([]error, len(errs))
	copy(copied, errs)
	f.errorSequence[id] = copied
}

func (f *fakeIngest) setDelay(id string, delay time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.delays[id] = delay
}

func (f *fakeIngest) callCount(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.callTotal[id]
}

func (f *fakeIngest) completion(id string) <-chan struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch, ok := f.done[id]
	if !ok {
		ch = make(chan struct{})
		f.done[id] = ch
	}
	return ch
}

func (f *fakeIngest) signalComplete(id string) {
	f.mu.Lock()
	ch := f.done[id]
	f.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case <-ch:
	default:
		close(ch)
	}
}

func (f *fakeIngest) TranscodeUpload(ctx context.Context, params ingest.UploadTranscodeParams) (ingest.UploadTranscodeResult, error) {
	f.mu.Lock()
	f.callTotal[params.UploadID]++
	delay := f.delays[params.UploadID]
	result, hasResult := f.results[params.UploadID]
	err := f.errors[params.UploadID]
	if seq, ok := f.errorSequence[params.UploadID]; ok && len(seq) > 0 {
		err = seq[0]
		f.errorSequence[params.UploadID] = seq[1:]
	}
	f.mu.Unlock()

	defer f.signalComplete(params.UploadID)

	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ingest.UploadTranscodeResult{}, ctx.Err()
		case <-timer.C:
		}
	}

	if err != nil {
		return ingest.UploadTranscodeResult{}, err
	}
	if hasResult {
		return result, nil
	}
	return ingest.UploadTranscodeResult{PlaybackURL: params.SourceURL}, nil
}

var _ IngestClient = (*fakeIngest)(nil)

func waitForIngestCallCount(t *testing.T, fake *fakeIngest, id string, expected int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fake.callCount(id) >= expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %d ingest calls of %s", expected, id)
}

func waitForCompletion(t *testing.T, done <-chan struct{}, id string, timeout time.Duration) {
	t.Helper()
	select {
	case <-done:
		return
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for ingest completion of %s", id)
	}
}

func waitForUploadUpdate(t *testing.T, updates <-chan domain.Upload, timeout time.Duration, predicate func(domain.Upload) bool) {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case upload := <-updates:
			if predicate(upload) {
				return
			}
		case <-timer.C:
			t.Fatalf("condition not met within %s", timeout)
		}
	}
}
