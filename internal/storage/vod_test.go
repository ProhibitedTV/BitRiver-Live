package storage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
)

func TestStopStreamUploadsRecordingArtifacts(t *testing.T) {
	controller := &fakeIngestController{bootResponses: []bootResponse{{result: ingest.BootResult{
		PlaybackURL: "https://playback.example.com/stream.m3u8",
		Renditions: []ingest.Rendition{
			{Name: "1080p", ManifestURL: "https://origin/1080p.m3u8", Bitrate: 6000},
			{Name: "720p", ManifestURL: "https://origin/720p.m3u8", Bitrate: 3500},
		},
	}}}}
	objectCfg := WithObjectStorage(ObjectStorageConfig{
		Bucket:         "vod",
		Prefix:         "vod/assets",
		PublicEndpoint: "https://cdn.example.com/content",
	})
	store := newTestStoreWithController(t, controller, objectCfg)
	fakeStorage := &fakeObjectStorage{prefix: store.objectStorage.Prefix, baseURL: store.objectStorage.PublicEndpoint}
	store.objectClient = fakeStorage

	owner, err := store.CreateUser(CreateUserParams{DisplayName: "Owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Live", "gaming", []string{"speedrun"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if _, err := store.StartStream(channel.ID, []string{"1080p", "720p"}); err != nil {
		t.Fatalf("StartStream: %v", err)
	}
	if _, err := store.StopStream(channel.ID, 42); err != nil {
		t.Fatalf("StopStream: %v", err)
	}

	if len(fakeStorage.uploads) != 3 {
		t.Fatalf("expected 3 uploads (2 manifests + thumbnail), got %d", len(fakeStorage.uploads))
	}

	store.mu.RLock()
	defer store.mu.RUnlock()
	if len(store.data.Recordings) != 1 {
		t.Fatalf("expected a single recording, got %d", len(store.data.Recordings))
	}
	var recording models.Recording
	for _, rec := range store.data.Recordings {
		recording = rec
	}

	if len(recording.Renditions) != 2 {
		t.Fatalf("expected 2 renditions, got %d", len(recording.Renditions))
	}
	if recording.Renditions[0].ManifestURL != fakeStorage.uploads[0].URL {
		t.Fatalf("expected first rendition manifest to reference uploaded object")
	}
	if recording.Renditions[1].ManifestURL != fakeStorage.uploads[1].URL {
		t.Fatalf("expected second rendition manifest to reference uploaded object")
	}
	manifestKey := manifestMetadataKey("1080p")
	if recording.Metadata[manifestKey] != fakeStorage.uploads[0].Key {
		t.Fatalf("expected metadata %s to store manifest key", manifestKey)
	}
	if len(recording.Thumbnails) != 1 {
		t.Fatalf("expected thumbnail metadata to be recorded")
	}
	thumbMetaKey := thumbnailMetadataKey(recording.Thumbnails[0].ID)
	if recording.Metadata[thumbMetaKey] != fakeStorage.uploads[2].Key {
		t.Fatalf("expected thumbnail metadata to store storage key")
	}
	if recording.Thumbnails[0].URL != fakeStorage.uploads[2].URL {
		t.Fatalf("expected thumbnail URL to reference uploaded object")
	}
}

func TestPopulateRecordingArtifactsTimeout(t *testing.T) {
	timeout := 50 * time.Millisecond
	requests := make(chan struct{}, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requests <- struct{}{}:
		default:
		}
		time.Sleep(2 * timeout)
	}))
	t.Cleanup(func() {
		server.CloseClientConnections()
		server.Close()
	})

	store := newTestStoreWithController(t, ingest.NoopController{}, WithObjectStorage(ObjectStorageConfig{
		Endpoint:       server.URL,
		Bucket:         "vod",
		RequestTimeout: timeout,
	}))

	recording := models.Recording{
		ID:        "rec-timeout",
		SessionID: "session-timeout",
		CreatedAt: time.Now(),
		Renditions: []models.RecordingRendition{{
			Name: "source",
		}},
	}
	session := models.StreamSession{
		ID: "session-timeout",
		RenditionManifests: []models.RenditionManifest{{
			Name:        "source",
			ManifestURL: "https://example.com/source.m3u8",
		}},
	}

	err := store.populateRecordingArtifactsLocked(&recording, session)
	if err == nil {
		t.Fatalf("expected populateRecordingArtifactsLocked to fail when request blocks")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded error, got %v", err)
	}

	select {
	case <-requests:
	case <-time.After(time.Second):
		t.Fatal("expected upload request to be issued")
	}
	server.CloseClientConnections()
}

func TestDeleteRecordingArtifactsTimeout(t *testing.T) {
	methods := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case methods <- r.Method:
		default:
		}
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	timeout := 50 * time.Millisecond
	store := newTestStoreWithController(t, ingest.NoopController{}, WithObjectStorage(ObjectStorageConfig{
		Endpoint:       server.URL,
		Bucket:         "vod",
		RequestTimeout: timeout,
	}))

	recording := models.Recording{
		Metadata: map[string]string{
			manifestMetadataKey("source"): "recordings/rec-timeout/manifests/source.json",
		},
	}

	err := store.deleteRecordingArtifactsLocked(recording)
	if err == nil {
		t.Fatalf("expected deleteRecordingArtifactsLocked to fail when request blocks")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded error, got %v", err)
	}

	select {
	case method := <-methods:
		if method != http.MethodDelete {
			t.Fatalf("expected DELETE request, got %s", method)
		}
	case <-time.After(time.Second):
		t.Fatal("expected delete request to be issued")
	}
}

func TestRecordingLifecycle(t *testing.T) {
	store := newTestStore(t)
	owner, err := store.CreateUser(CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Show", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	session, err := store.StartStream(channel.ID, []string{"1080p"})
	if err != nil {
		t.Fatalf("StartStream: %v", err)
	}
	if _, err := store.StopStream(channel.ID, 42); err != nil {
		t.Fatalf("StopStream: %v", err)
	}

	recordings, err := store.ListRecordings(channel.ID, true)
	if err != nil {
		t.Fatalf("ListRecordings: %v", err)
	}
	if len(recordings) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(recordings))
	}
	recording := recordings[0]
	if recording.SessionID != session.ID {
		t.Fatalf("expected recording to reference session %s", session.ID)
	}
	if recording.PublishedAt != nil {
		t.Fatalf("expected recording to start unpublished")
	}

	published, err := store.PublishRecording(recording.ID)
	if err != nil {
		t.Fatalf("PublishRecording: %v", err)
	}
	if published.PublishedAt == nil {
		t.Fatalf("expected publish to set timestamp")
	}

	clip, err := store.CreateClipExport(recording.ID, ClipExportParams{Title: "Highlight", StartSeconds: 0, EndSeconds: 5})
	if err != nil {
		t.Fatalf("CreateClipExport: %v", err)
	}
	if clip.RecordingID != recording.ID {
		t.Fatalf("expected clip to reference recording")
	}

	clips, err := store.ListClipExports(recording.ID)
	if err != nil {
		t.Fatalf("ListClipExports: %v", err)
	}
	if len(clips) != 1 || clips[0].ID != clip.ID {
		t.Fatalf("expected to list created clip")
	}

	fetched, ok := store.GetRecording(recording.ID)
	if !ok {
		t.Fatalf("GetRecording should succeed")
	}
	if len(fetched.Clips) != 1 || fetched.Clips[0].ID != clip.ID {
		t.Fatalf("expected recording to include clip summary")
	}

	if err := store.DeleteRecording(recording.ID); err != nil {
		t.Fatalf("DeleteRecording: %v", err)
	}
	recordings, err = store.ListRecordings(channel.ID, true)
	if err != nil {
		t.Fatalf("ListRecordings after delete: %v", err)
	}
	if len(recordings) != 0 {
		t.Fatalf("expected no recordings after delete")
	}
}

func TestDeleteRecordingRemovesStorageArtifacts(t *testing.T) {
	controller := &fakeIngestController{bootResponses: []bootResponse{{result: ingest.BootResult{
		Renditions: []ingest.Rendition{{Name: "1080p", ManifestURL: "https://origin/1080p.m3u8"}},
	}}}}
	objectCfg := WithObjectStorage(ObjectStorageConfig{
		Bucket:         "vod",
		Prefix:         "vod/assets",
		PublicEndpoint: "https://cdn.example.com/content",
	})
	store := newTestStoreWithController(t, controller, objectCfg)
	fakeStorage := &fakeObjectStorage{prefix: store.objectStorage.Prefix, baseURL: store.objectStorage.PublicEndpoint}
	store.objectClient = fakeStorage

	owner, err := store.CreateUser(CreateUserParams{DisplayName: "Owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "VODs", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if _, err := store.StartStream(channel.ID, []string{"1080p"}); err != nil {
		t.Fatalf("StartStream: %v", err)
	}
	if _, err := store.StopStream(channel.ID, 25); err != nil {
		t.Fatalf("StopStream: %v", err)
	}
	recordingID := firstRecordingID(store)
	clip, err := store.CreateClipExport(recordingID, ClipExportParams{Title: "Highlight", StartSeconds: 0, EndSeconds: 5})
	if err != nil {
		t.Fatalf("CreateClipExport: %v", err)
	}

	store.mu.Lock()
	recording := store.data.Recordings[recordingID]
	manifestKeys := make([]string, 0)
	thumbnailKeys := make([]string, 0)
	for metaKey, objectKey := range recording.Metadata {
		switch {
		case strings.HasPrefix(metaKey, metadataManifestPrefix):
			manifestKeys = append(manifestKeys, objectKey)
		case strings.HasPrefix(metaKey, metadataThumbnailPrefix):
			thumbnailKeys = append(thumbnailKeys, objectKey)
		}
	}
	clipStored := store.data.ClipExports[clip.ID]
	clipStored.StorageObject = buildObjectKey("clips", clip.ID+".mp4")
	store.data.ClipExports[clip.ID] = clipStored
	store.mu.Unlock()

	fakeStorage.deletes = nil
	if err := store.DeleteRecording(recordingID); err != nil {
		t.Fatalf("DeleteRecording: %v", err)
	}

	expectedDeletes := make(map[string]struct{})
	for _, key := range manifestKeys {
		if key != "" {
			expectedDeletes[key] = struct{}{}
		}
	}
	for _, key := range thumbnailKeys {
		if key != "" {
			expectedDeletes[key] = struct{}{}
		}
	}
	if clipStored.StorageObject != "" {
		expectedDeletes[clipStored.StorageObject] = struct{}{}
	}
	for _, deleted := range fakeStorage.deletes {
		delete(expectedDeletes, deleted)
	}
	if len(expectedDeletes) != 0 {
		t.Fatalf("expected storage deletes for manifests, thumbnails, and clips; missing %v", expectedDeletes)
	}
}

func TestDeleteClipArtifactsHonorsTimeout(t *testing.T) {
	store := newTestStore(t)
	store.objectStorage.RequestTimeout = 10 * time.Millisecond
	store.objectClient = &hangingDeleteObjectStorage{}

	clip := models.ClipExport{ID: "clip-123", StorageObject: "clips/clip-123.mp4"}

	store.mu.Lock()
	start := time.Now()
	err := store.deleteClipArtifactsLocked(clip)
	store.mu.Unlock()

	if err == nil {
		t.Fatalf("expected deleteClipArtifactsLocked to return error when delete hangs")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded error, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("expected delete to honor timeout, took %v", elapsed)
	}
}

func TestRepositoryStreamKeyRotation(t *testing.T) {
	RunRepositoryStreamKeyRotation(t, jsonRepositoryFactory)
}

func TestRepositoryStreamLifecycleWithoutIngest(t *testing.T) {
	RunRepositoryStreamLifecycleWithoutIngest(t, jsonRepositoryFactory)
}

func TestRepositoryStreamTimeouts(t *testing.T) {
	RunRepositoryStreamTimeouts(t, jsonRepositoryFactory)
}

func TestRecordingRetentionPurgesExpired(t *testing.T) {
	RunRepositoryRecordingRetention(t, jsonRepositoryFactory)
}
