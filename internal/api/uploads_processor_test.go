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

	waitFor(t, time.Second*2, func() bool {
		return ingestFake.callCount("upload-1") == 1 && ingestFake.callCount("upload-2") == 1 && ingestFake.callCount("upload-3") == 1
	})

	waitFor(t, time.Second, func() bool {
		upload, ok := store.GetUpload("upload-1")
		if !ok {
			return false
		}
		return upload.Status == "ready" && upload.PlaybackURL == "https://vod.example.com/a.m3u8" && upload.Progress == 100
	})
	waitFor(t, time.Second, func() bool {
		upload, ok := store.GetUpload("upload-2")
		if !ok {
			return false
		}
		return upload.Status == "ready" && upload.PlaybackURL == "https://vod.example.com/b.m3u8" && upload.Progress == 100
	})
	waitFor(t, time.Second, func() bool {
		upload, ok := store.GetUpload("upload-3")
		if !ok {
			return false
		}
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

	waitFor(t, time.Second, func() bool { return ingestFake.callCount("upload-err") == 1 })
	waitFor(t, time.Second, func() bool {
		upload, ok := store.GetUpload("upload-err")
		if !ok {
			return false
		}
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

	waitFor(t, time.Second, func() bool { return ingestFake.callCount("upload-slow") == 1 })
	waitFor(t, time.Second, func() bool {
		upload, ok := store.GetUpload("upload-slow")
		if !ok {
			return false
		}
		if upload.Status != "failed" || upload.Progress != 0 {
			return false
		}
		if upload.Metadata["sourceUrl"] != "https://example.com/slow.mp4" {
			return false
		}
		return upload.Error == context.DeadlineExceeded.Error() || upload.Error == context.Canceled.Error()
	})
}

type fakeUploadStore struct {
	mu       sync.Mutex
	channels []models.Channel
	uploads  map[string]models.Upload
}

func newFakeUploadStore() *fakeUploadStore {
	return &fakeUploadStore{uploads: make(map[string]models.Upload)}
}

func (f *fakeUploadStore) ListChannels(ownerID string) []models.Channel {
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

func (f *fakeUploadStore) UpdateUpload(id string, update storage.UploadUpdate) (models.Upload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	upload, ok := f.uploads[id]
	if !ok {
		return models.Upload{}, errors.New("upload not found")
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
	return cloneUpload(upload), nil
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

func (f *fakeUploadStore) IngestHealth(ctx context.Context) []ingest.HealthStatus {
	panic("not implemented")
}
func (f *fakeUploadStore) LastIngestHealth() ([]ingest.HealthStatus, time.Time) {
	panic("not implemented")
}
func (f *fakeUploadStore) CreateUser(params storage.CreateUserParams) (models.User, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) AuthenticateUser(email, password string) (models.User, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) AuthenticateOAuth(params storage.OAuthLoginParams) (models.User, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) ListUsers() []models.User              { panic("not implemented") }
func (f *fakeUploadStore) GetUser(id string) (models.User, bool) { panic("not implemented") }
func (f *fakeUploadStore) UpdateUser(id string, update storage.UserUpdate) (models.User, error) {
	panic("not implemented")
}

func (f *fakeUploadStore) SetUserPassword(id, password string) (models.User, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) DeleteUser(id string) error { panic("not implemented") }
func (f *fakeUploadStore) UpsertProfile(userID string, update storage.ProfileUpdate) (models.Profile, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) GetProfile(userID string) (models.Profile, bool) { panic("not implemented") }
func (f *fakeUploadStore) ListProfiles() []models.Profile                  { panic("not implemented") }
func (f *fakeUploadStore) CreateChannel(ownerID, title, category string, tags []string) (models.Channel, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) UpdateChannel(id string, update storage.ChannelUpdate) (models.Channel, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) RotateChannelStreamKey(id string) (models.Channel, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) DeleteChannel(id string) error                    { panic("not implemented") }
func (f *fakeUploadStore) GetChannel(id string) (models.Channel, bool)      { panic("not implemented") }
func (f *fakeUploadStore) FollowChannel(userID, channelID string) error     { panic("not implemented") }
func (f *fakeUploadStore) UnfollowChannel(userID, channelID string) error   { panic("not implemented") }
func (f *fakeUploadStore) IsFollowingChannel(userID, channelID string) bool { panic("not implemented") }
func (f *fakeUploadStore) CountFollowers(channelID string) int              { panic("not implemented") }
func (f *fakeUploadStore) ListFollowedChannelIDs(userID string) []string    { panic("not implemented") }
func (f *fakeUploadStore) StartStream(channelID string, renditions []string) (models.StreamSession, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) StopStream(channelID string, peakConcurrent int) (models.StreamSession, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) CurrentStreamSession(channelID string) (models.StreamSession, bool) {
	panic("not implemented")
}
func (f *fakeUploadStore) ListStreamSessions(channelID string) ([]models.StreamSession, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) ListRecordings(channelID string, includeUnpublished bool) ([]models.Recording, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) GetRecording(id string) (models.Recording, bool) { panic("not implemented") }
func (f *fakeUploadStore) PublishRecording(id string) (models.Recording, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) DeleteRecording(id string) error { panic("not implemented") }
func (f *fakeUploadStore) CreateUpload(params storage.CreateUploadParams) (models.Upload, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) DeleteUpload(id string) error { panic("not implemented") }
func (f *fakeUploadStore) CreateClipExport(recordingID string, params storage.ClipExportParams) (models.ClipExport, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) ListClipExports(recordingID string) ([]models.ClipExport, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) CreateChatMessage(channelID, userID, content string) (models.ChatMessage, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) DeleteChatMessage(channelID, messageID string) error {
	panic("not implemented")
}
func (f *fakeUploadStore) ListChatMessages(channelID string, limit int) ([]models.ChatMessage, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) ChatRestrictions() chat.RestrictionsSnapshot { panic("not implemented") }
func (f *fakeUploadStore) IsChatBanned(channelID, userID string) bool  { panic("not implemented") }
func (f *fakeUploadStore) ChatTimeout(channelID, userID string) (time.Time, bool) {
	panic("not implemented")
}
func (f *fakeUploadStore) ApplyChatEvent(evt chat.Event) error { panic("not implemented") }
func (f *fakeUploadStore) ListChatRestrictions(channelID string) []models.ChatRestriction {
	panic("not implemented")
}
func (f *fakeUploadStore) CreateChatReport(channelID, reporterID, targetID, reason, messageID, evidenceURL string) (models.ChatReport, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) ListChatReports(channelID string, includeResolved bool) ([]models.ChatReport, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) ResolveChatReport(reportID, resolverID, resolution string) (models.ChatReport, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) CreateTip(params storage.CreateTipParams) (models.Tip, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) ListTips(channelID string, limit int) ([]models.Tip, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) CreateSubscription(params storage.CreateSubscriptionParams) (models.Subscription, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) ListSubscriptions(channelID string, includeInactive bool) ([]models.Subscription, error) {
	panic("not implemented")
}
func (f *fakeUploadStore) GetSubscription(id string) (models.Subscription, bool) {
	panic("not implemented")
}
func (f *fakeUploadStore) CancelSubscription(id, cancelledBy, reason string) (models.Subscription, error) {
	panic("not implemented")
}

var _ storage.Repository = (*fakeUploadStore)(nil)

type fakeIngest struct {
	mu        sync.Mutex
	results   map[string]ingest.UploadTranscodeResult
	errors    map[string]error
	delays    map[string]time.Duration
	callTotal map[string]int
}

func newFakeIngest() *fakeIngest {
	return &fakeIngest{
		results:   make(map[string]ingest.UploadTranscodeResult),
		errors:    make(map[string]error),
		delays:    make(map[string]time.Duration),
		callTotal: make(map[string]int),
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

func (f *fakeIngest) TranscodeUpload(ctx context.Context, params ingest.UploadTranscodeParams) (ingest.UploadTranscodeResult, error) {
	f.mu.Lock()
	f.callTotal[params.UploadID]++
	delay := f.delays[params.UploadID]
	result, hasResult := f.results[params.UploadID]
	err := f.errors[params.UploadID]
	f.mu.Unlock()

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
	panic("not implemented")
}

func (f *fakeIngest) ShutdownStream(ctx context.Context, channelID, sessionID string, jobIDs []string) error {
	panic("not implemented")
}

func (f *fakeIngest) HealthChecks(ctx context.Context) []ingest.HealthStatus {
	panic("not implemented")
}

var _ ingest.Controller = (*fakeIngest)(nil)

func waitFor(t *testing.T, timeout time.Duration, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !predicate() {
		t.Fatalf("condition not met within %s", timeout)
	}
}
