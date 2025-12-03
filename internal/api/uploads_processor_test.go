package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
	"bitriver-live/internal/storage"
)

func TestUploadProcessorStartShutdown(t *testing.T) {
	store := newFakeUploadStore()
	store.channels = []models.Channel{{ID: "channel-1"}}
	store.uploads = map[string]models.Upload{
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

	waitForUploadUpdate(t, upload1Updates, time.Second, func(upload models.Upload) bool {
		return upload.Status == "ready" && upload.PlaybackURL == "https://vod.example.com/a.m3u8" && upload.Progress == 100
	})
	waitForUploadUpdate(t, upload2Updates, time.Second, func(upload models.Upload) bool {
		return upload.Status == "ready" && upload.PlaybackURL == "https://vod.example.com/b.m3u8" && upload.Progress == 100
	})
	waitForUploadUpdate(t, upload3Updates, time.Second, func(upload models.Upload) bool {
		return upload.Status == "ready" && upload.PlaybackURL == "https://vod.example.com/c.m3u8" && upload.Progress == 100
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := processor.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}
}

func TestUploadProcessorFailUpload(t *testing.T) {
	store := newFakeUploadStore()
	store.channels = []models.Channel{{ID: "channel-1"}}
	store.uploads = map[string]models.Upload{
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
	waitForUploadUpdate(t, errUpdates, time.Second, func(upload models.Upload) bool {
		return upload.Status == "failed" && upload.Progress == 0 && upload.Metadata["sourceUrl"] == "https://example.com/error.mp4" && upload.Error == "transcode failed"
	})
}

func TestUploadProcessorTimeout(t *testing.T) {
	store := newFakeUploadStore()
	store.channels = []models.Channel{{ID: "channel-1"}}
	store.uploads = map[string]models.Upload{
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
	slowDone := ingestFake.completion("upload-slow")

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

	waitForCompletion(t, slowDone, "upload-slow", time.Second)
	waitForUploadUpdate(t, slowUpdates, time.Second, func(upload models.Upload) bool {
		if upload.Status != "failed" || upload.Progress != 0 {
			return false
		}
		if upload.Metadata["sourceUrl"] != "https://example.com/slow.mp4" {
			return false
		}
		return upload.Error == context.DeadlineExceeded.Error() || upload.Error == context.Canceled.Error()
	})
}

func TestUploadProcessorRetryUpdateFailures(t *testing.T) {
	store := newFakeUploadStore()
	store.channels = []models.Channel{{ID: "channel-1"}}
	store.uploads = map[string]models.Upload{
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

	waitForUploadUpdate(t, retryUpdates, time.Second, func(models.Upload) bool {
		return store.updateAttemptCount("upload-retry") >= 1
	})

	waitForCompletion(t, retryDone, "upload-retry", 3*time.Second)
	waitForUploadUpdate(t, retryUpdates, 3*time.Second, func(upload models.Upload) bool {
		return upload.Status == "ready" && upload.PlaybackURL == "https://vod.example.com/retry.m3u8" && upload.Progress == 100
	})

	if count := ingestFake.callCount("upload-retry"); count != 1 {
		t.Fatalf("expected single ingest call, got %d", count)
	}

	if attempts := store.updateAttemptCount("upload-retry"); attempts < 3 {
		t.Fatalf("expected at least 3 update attempts, got %d", attempts)
	}
}

type fakeUploadStore struct {
	mu              sync.Mutex
	channels        []models.Channel
	uploads         map[string]models.Upload
	failFirstUpdate map[string]error
	updateAttempts  map[string]int
	updateCh        map[string]chan models.Upload
}

func newFakeUploadStore() *fakeUploadStore {
	return &fakeUploadStore{
		uploads:         make(map[string]models.Upload),
		failFirstUpdate: make(map[string]error),
		updateAttempts:  make(map[string]int),
		updateCh:        make(map[string]chan models.Upload),
	}
}

func (f *fakeUploadStore) Ping(context.Context) error { return nil }

func (f *fakeUploadStore) ListChannels(ownerID, query string) []models.Channel {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]models.Channel, len(f.channels))
	copy(out, f.channels)
	return out
}

func (f *fakeUploadStore) ListUploads(channelID string) ([]models.Upload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var uploads []models.Upload
	for _, upload := range f.uploads {
		if upload.ChannelID == channelID {
			uploads = append(uploads, cloneUpload(upload))
		}
	}
	return uploads, nil
}

func (f *fakeUploadStore) GetUpload(id string) (models.Upload, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	upload, ok := f.uploads[id]
	if !ok {
		return models.Upload{}, false
	}
	return cloneUpload(upload), true
}

func (f *fakeUploadStore) updatesFor(id string) <-chan models.Upload {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch, ok := f.updateCh[id]
	if !ok {
		ch = make(chan models.Upload, 8)
		f.updateCh[id] = ch
	}
	return ch
}

func (f *fakeUploadStore) UpdateUpload(id string, update storage.UploadUpdate) (models.Upload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	upload, ok := f.uploads[id]
	if !ok {
		return models.Upload{}, errors.New("upload not found")
	}
	attempt := f.updateAttempts[id]
	f.updateAttempts[id] = attempt + 1
	if err, shouldFail := f.failFirstUpdate[id]; shouldFail && attempt == 0 {
		delete(f.failFirstUpdate, id)
		return models.Upload{}, err
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
	if update.Metadata != nil {
		upload.Metadata = cloneMetadata(update.Metadata)
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

func cloneMetadata(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneUpload(upload models.Upload) models.Upload {
	upload.Metadata = cloneMetadata(upload.Metadata)
	return upload
}

// Unimplemented methods of storage.Repository.
func (f *fakeUploadStore) ListUsers() []models.User { return []models.User{} }

func (f *fakeUploadStore) GetUser(id string) (models.User, bool) { return models.User{}, false }

func (f *fakeUploadStore) UpdateUser(id string, update storage.UserUpdate) (models.User, error) {
	return models.User{}, nil
}

func (f *fakeUploadStore) SetUserPassword(id, password string) (models.User, error) {
	return models.User{}, nil
}

func (f *fakeUploadStore) DeleteUser(id string) error { return nil }

func (f *fakeUploadStore) UpsertProfile(userID string, update storage.ProfileUpdate) (models.Profile, error) {
	return models.Profile{}, nil
}

func (f *fakeUploadStore) GetProfile(userID string) (models.Profile, bool) {
	return models.Profile{}, false
}

func (f *fakeUploadStore) ListProfiles() []models.Profile { return []models.Profile{} }

func (f *fakeUploadStore) CreateChannel(ownerID, title, category string, tags []string) (models.Channel, error) {
	return models.Channel{}, nil
}

func (f *fakeUploadStore) UpdateChannel(id string, update storage.ChannelUpdate) (models.Channel, error) {
	return models.Channel{}, nil
}

func (f *fakeUploadStore) RotateChannelStreamKey(id string) (models.Channel, error) {
	return models.Channel{}, nil
}

func (f *fakeUploadStore) DeleteChannel(id string) error { return nil }

func (f *fakeUploadStore) GetChannel(id string) (models.Channel, bool) {
	return models.Channel{}, false
}

func (f *fakeUploadStore) GetChannelByStreamKey(streamKey string) (models.Channel, bool) {
	return models.Channel{}, false
}

func (f *fakeUploadStore) FollowChannel(userID, channelID string) error { return nil }

func (f *fakeUploadStore) UnfollowChannel(userID, channelID string) error { return nil }

func (f *fakeUploadStore) IsFollowingChannel(userID, channelID string) bool { return false }

func (f *fakeUploadStore) CountFollowers(channelID string) int { return 0 }

func (f *fakeUploadStore) ListFollowedChannelIDs(userID string) []string { return []string{} }

func (f *fakeUploadStore) StartStream(channelID string, renditions []string) (models.StreamSession, error) {
	return models.StreamSession{}, nil
}

func (f *fakeUploadStore) StopStream(channelID string, peakConcurrent int) (models.StreamSession, error) {
	return models.StreamSession{}, nil
}

func (f *fakeUploadStore) CurrentStreamSession(channelID string) (models.StreamSession, bool) {
	return models.StreamSession{}, false
}

func (f *fakeUploadStore) ListStreamSessions(channelID string) ([]models.StreamSession, error) {
	return []models.StreamSession{}, nil
}

func (f *fakeUploadStore) ListRecordings(channelID string, includeUnpublished bool) ([]models.Recording, error) {
	return []models.Recording{}, nil
}

func (f *fakeUploadStore) GetRecording(id string) (models.Recording, bool) {
	return models.Recording{}, false
}

func (f *fakeUploadStore) PublishRecording(id string) (models.Recording, error) {
	return models.Recording{}, nil
}

func (f *fakeUploadStore) DeleteRecording(id string) error { return nil }

func (f *fakeUploadStore) CreateUpload(params storage.CreateUploadParams) (models.Upload, error) {
	return models.Upload{}, nil
}

func (f *fakeUploadStore) DeleteUpload(id string) error { return nil }

func (f *fakeUploadStore) CreateClipExport(recordingID string, params storage.ClipExportParams) (models.ClipExport, error) {
	return models.ClipExport{}, nil
}

func (f *fakeUploadStore) ListClipExports(recordingID string) ([]models.ClipExport, error) {
	return []models.ClipExport{}, nil
}

func (f *fakeUploadStore) CreateChatMessage(channelID, userID, content string) (models.ChatMessage, error) {
	return models.ChatMessage{}, nil
}

func (f *fakeUploadStore) DeleteChatMessage(channelID, messageID string) error {
	return nil
}

func (f *fakeUploadStore) ListChatMessages(channelID string, limit int) ([]models.ChatMessage, error) {
	return []models.ChatMessage{}, nil
}

func (f *fakeUploadStore) ChatRestrictions() chat.RestrictionsSnapshot {
	return chat.RestrictionsSnapshot{}
}

func (f *fakeUploadStore) IsChatBanned(channelID, userID string) bool { return false }

func (f *fakeUploadStore) ChatTimeout(channelID, userID string) (time.Time, bool) {
	return time.Time{}, false
}

func (f *fakeUploadStore) ApplyChatEvent(evt chat.Event) error { return nil }

func (f *fakeUploadStore) ListChatRestrictions(channelID string) []models.ChatRestriction {
	return []models.ChatRestriction{}
}

func (f *fakeUploadStore) CreateChatReport(channelID, reporterID, targetID, reason, messageID, evidenceURL string) (models.ChatReport, error) {
	return models.ChatReport{}, nil
}

func (f *fakeUploadStore) ListChatReports(channelID string, includeResolved bool) ([]models.ChatReport, error) {
	return []models.ChatReport{}, nil
}

func (f *fakeUploadStore) ResolveChatReport(reportID, resolverID, resolution string) (models.ChatReport, error) {
	return models.ChatReport{}, nil
}

func (f *fakeUploadStore) CreateTip(params storage.CreateTipParams) (models.Tip, error) {
	return models.Tip{}, nil
}

func (f *fakeUploadStore) ListTips(channelID string, limit int) ([]models.Tip, error) {
	return []models.Tip{}, nil
}

func (f *fakeUploadStore) CreateSubscription(params storage.CreateSubscriptionParams) (models.Subscription, error) {
	return models.Subscription{}, nil
}

func (f *fakeUploadStore) ListSubscriptions(channelID string, includeInactive bool) ([]models.Subscription, error) {
	return []models.Subscription{}, nil
}

func (f *fakeUploadStore) GetSubscription(id string) (models.Subscription, bool) {
	return models.Subscription{}, false
}

func (f *fakeUploadStore) CancelSubscription(id, cancelledBy, reason string) (models.Subscription, error) {
	return models.Subscription{}, nil
}

var _ storage.Repository = (*fakeUploadStore)(nil)

type fakeIngest struct {
	mu        sync.Mutex
	results   map[string]ingest.UploadTranscodeResult
	errors    map[string]error
	delays    map[string]time.Duration
	callTotal map[string]int
	done      map[string]chan struct{}
}

func newFakeIngest() *fakeIngest {
	return &fakeIngest{
		results:   make(map[string]ingest.UploadTranscodeResult),
		errors:    make(map[string]error),
		delays:    make(map[string]time.Duration),
		callTotal: make(map[string]int),
		done:      make(map[string]chan struct{}),
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

func (f *fakeIngest) BootStream(ctx context.Context, params ingest.BootParams) (ingest.BootResult, error) {
	return ingest.BootResult{}, nil
}

func (f *fakeIngest) ShutdownStream(ctx context.Context, channelID, sessionID string, jobIDs []string) error {
	return nil
}

func (f *fakeIngest) HealthChecks(ctx context.Context) []ingest.HealthStatus {
	return []ingest.HealthStatus{}
}

var _ ingest.Controller = (*fakeIngest)(nil)

func waitForCompletion(t *testing.T, done <-chan struct{}, id string, timeout time.Duration) {
	t.Helper()
	select {
	case <-done:
		return
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for ingest completion of %s", id)
	}
}

func waitForUploadUpdate(t *testing.T, updates <-chan models.Upload, timeout time.Duration, predicate func(models.Upload) bool) {
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
