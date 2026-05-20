package storage

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"bitriver-live/internal/ingest"
)

// bootResponse stores canned ingest boot outcomes for tests.
type bootResponse struct {
	result ingest.BootResult
	err    error
}

type shutdownCall struct {
	channelID string
	sessionID string
	jobIDs    []string
}

// fakeIngestController supplies deterministic ingest responses for storage tests.
type fakeIngestController struct {
	bootResponses   []bootResponse
	bootDefault     ingest.BootResult
	bootErr         error
	bootCalls       int
	onBoot          func(int)
	shutdownErr     error
	shutdownCalls   []shutdownCall
	healthResponses [][]ingest.HealthStatus
	healthCalls     int
}

func (f *fakeIngestController) BootStream(ctx context.Context, params ingest.BootParams) (ingest.BootResult, error) {
	idx := f.bootCalls
	f.bootCalls++
	if f.onBoot != nil {
		f.onBoot(idx)
	}
	if idx < len(f.bootResponses) {
		resp := f.bootResponses[idx]
		if resp.err != nil {
			return ingest.BootResult{}, resp.err
		}
		return resp.result, nil
	}
	if f.bootErr != nil {
		return ingest.BootResult{}, f.bootErr
	}
	return f.bootDefault, nil
}

func (f *fakeIngestController) ShutdownStream(ctx context.Context, channelID, sessionID string, jobIDs []string) error {
	call := shutdownCall{channelID: channelID, sessionID: sessionID, jobIDs: append([]string{}, jobIDs...)}
	f.shutdownCalls = append(f.shutdownCalls, call)
	if f.shutdownErr != nil {
		return f.shutdownErr
	}
	return nil
}

func (f *fakeIngestController) HealthChecks(ctx context.Context) []ingest.HealthStatus {
	if len(f.healthResponses) == 0 {
		return []ingest.HealthStatus{{Component: "fake", Status: "ok"}}
	}
	idx := f.healthCalls
	if idx >= len(f.healthResponses) {
		idx = len(f.healthResponses) - 1
	}
	f.healthCalls++
	snapshot := append([]ingest.HealthStatus(nil), f.healthResponses[idx]...)
	return snapshot
}

func (f *fakeIngestController) TranscodeUpload(ctx context.Context, params ingest.UploadTranscodeParams) (ingest.UploadTranscodeResult, error) {
	return ingest.UploadTranscodeResult{PlaybackURL: params.SourceURL}, nil
}

func TestRunIngestBootWithRetryStopsOnContextTerminalErrors(t *testing.T) {
	controller := &fakeIngestController{bootResponses: []bootResponse{{err: context.DeadlineExceeded}}}
	retryInterval := 100 * time.Millisecond

	start := time.Now()
	_, err := runIngestBootWithRetry(context.Background(), controller, ingest.BootParams{}, 50*time.Millisecond, 3, retryInterval)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded error, got %v", err)
	}
	if controller.bootCalls != 1 {
		t.Fatalf("expected exactly 1 boot attempt, got %d", controller.bootCalls)
	}
	if elapsed >= retryInterval/2 {
		t.Fatalf("expected no retry sleep on terminal context error; elapsed=%v retryInterval=%v", elapsed, retryInterval)
	}
}

func TestRunIngestBootWithRetryRetriesOnTransientErrors(t *testing.T) {
	transientErr := errors.New("transient ingest failure")
	expected := ingest.BootResult{PlaybackURL: "https://playback.example"}
	controller := &fakeIngestController{bootResponses: []bootResponse{{err: transientErr}, {result: expected}}}
	retryInterval := 25 * time.Millisecond

	start := time.Now()
	boot, err := runIngestBootWithRetry(context.Background(), controller, ingest.BootParams{}, 50*time.Millisecond, 3, retryInterval)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected retry path to succeed, got error %v", err)
	}
	if controller.bootCalls != 2 {
		t.Fatalf("expected 2 boot attempts, got %d", controller.bootCalls)
	}
	if !reflect.DeepEqual(boot, expected) {
		t.Fatalf("unexpected boot result: got %#v want %#v", boot, expected)
	}
	if elapsed < retryInterval/2 {
		t.Fatalf("expected retry delay for transient errors; elapsed=%v retryInterval=%v", elapsed, retryInterval)
	}
}

func TestRunIngestBootWithRetryStopsWhenParentContextCanceled(t *testing.T) {
	transientErr := errors.New("transient ingest failure")
	ctx, cancel := context.WithCancel(context.Background())
	controller := &fakeIngestController{
		bootResponses: []bootResponse{{err: transientErr}, {result: ingest.BootResult{PlaybackURL: "https://playback.example"}}},
		onBoot: func(attempt int) {
			if attempt == 0 {
				cancel()
			}
		},
	}
	retryInterval := time.Hour

	start := time.Now()
	_, err := runIngestBootWithRetry(ctx, controller, ingest.BootParams{}, 50*time.Millisecond, 3, retryInterval)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if controller.bootCalls != 1 {
		t.Fatalf("expected exactly 1 boot attempt, got %d", controller.bootCalls)
	}
	if elapsed >= 100*time.Millisecond {
		t.Fatalf("expected retry wait to stop on cancellation, elapsed=%v", elapsed)
	}
}

func TestRunIngestBootWithRetrySkipsAttemptsWhenParentContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	controller := &fakeIngestController{bootDefault: ingest.BootResult{PlaybackURL: "https://playback.example"}}

	_, err := runIngestBootWithRetry(ctx, controller, ingest.BootParams{}, 50*time.Millisecond, 3, time.Hour)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if controller.bootCalls != 0 {
		t.Fatalf("expected no boot attempts, got %d", controller.bootCalls)
	}
}

func TestCreateChannelAndStartStopStream(t *testing.T) {
	store := newTestStore(t)
	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Alice",
		Email:       "alice@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	channel, err := store.CreateChannel(user.ID, "My Channel", "gaming", []string{"Action", "Indie"})
	if err != nil {
		t.Fatalf("CreateChannel returned error: %v", err)
	}
	if channel.StreamKey == "" {
		t.Fatal("expected stream key to be generated")
	}
	if channel.LiveState != "offline" {
		t.Fatalf("expected liveState offline, got %s", channel.LiveState)
	}

	session, err := store.StartStream(channel.ID, []string{"1080p", "720p"})
	if err != nil {
		t.Fatalf("StartStream returned error: %v", err)
	}
	if session.ChannelID != channel.ID {
		t.Fatalf("session channel mismatch: %s", session.ChannelID)
	}
	updated, ok := store.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("channel %s not found after start", channel.ID)
	}
	if updated.LiveState != "live" {
		t.Fatalf("expected live state live, got %s", updated.LiveState)
	}
	if updated.CurrentSessionID == nil || *updated.CurrentSessionID != session.ID {
		t.Fatal("expected current session ID to be set")
	}

	ended, err := store.StopStream(channel.ID, 42)
	if err != nil {
		t.Fatalf("StopStream returned error: %v", err)
	}
	if ended.EndedAt == nil {
		t.Fatal("expected session to have end time")
	}
	updated, ok = store.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("channel %s not found after stop", channel.ID)
	}
	if updated.LiveState != "offline" {
		t.Fatalf("expected live state offline, got %s", updated.LiveState)
	}
	if updated.CurrentSessionID != nil {
		t.Fatal("expected current session to be cleared")
	}
}

func TestStorageStartStreamTimesOutWhenIngestBlocks(t *testing.T) {
	timeout := 30 * time.Millisecond
	controller := &timeoutIngestController{bootBlock: true}
	store := newTestStoreWithController(t, controller, WithIngestTimeout(timeout))

	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Creator",
		Email:       "creator@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(user.ID, "Timeouts", "gaming", []string{"speedrun"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	start := time.Now()
	if _, err := store.StartStream(channel.ID, []string{"720p"}); err == nil {
		t.Fatal("expected StartStream to fail when ingest blocks")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("StartStream exceeded timeout expectation: %v", time.Since(start))
	}

	updated, ok := store.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("expected to reload channel %s", channel.ID)
	}
	if updated.LiveState != "offline" {
		t.Fatalf("expected channel to remain offline, got %s", updated.LiveState)
	}
	if updated.CurrentSessionID != nil {
		t.Fatalf("expected current session to remain nil, got %v", *updated.CurrentSessionID)
	}
}

func TestStorageStopStreamTimesOutWhenIngestBlocks(t *testing.T) {
	timeout := 30 * time.Millisecond
	controller := &timeoutIngestController{bootResult: ingest.BootResult{PlaybackURL: "https://playback.example"}}
	store := newTestStoreWithController(t, controller, WithIngestTimeout(timeout))

	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Creator",
		Email:       "creator@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(user.ID, "Timeouts", "gaming", []string{"speedrun"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	session, err := store.StartStream(channel.ID, []string{"720p"})
	if err != nil {
		t.Fatalf("StartStream: %v", err)
	}

	controller.shutdownBlock = true

	start := time.Now()
	if _, err := store.StopStream(channel.ID, 25); err == nil {
		t.Fatal("expected StopStream to fail when ingest shutdown blocks")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("StopStream exceeded timeout expectation: %v", time.Since(start))
	}

	updated, ok := store.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("expected to reload channel %s", channel.ID)
	}
	if updated.LiveState != "live" {
		t.Fatalf("expected channel to remain live, got %s", updated.LiveState)
	}
	if updated.CurrentSessionID == nil || *updated.CurrentSessionID != session.ID {
		t.Fatalf("expected current session to remain %s, got %v", session.ID, updated.CurrentSessionID)
	}

	current, ok := store.CurrentStreamSession(channel.ID)
	if !ok {
		t.Fatalf("expected current stream session %s to persist", session.ID)
	}
	if current.EndedAt != nil {
		t.Fatal("expected session to remain active after shutdown timeout")
	}
}

func TestRotateChannelStreamKey(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{DisplayName: "Owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}

	channel, err := store.CreateChannel(owner.ID, "Control", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	originalKey := channel.StreamKey
	if originalKey == "" {
		t.Fatal("expected initial stream key")
	}

	persisted := false
	store.persistOverride = func(data dataset) error {
		persisted = true
		updated, ok := data.Channels[channel.ID]
		if !ok {
			t.Fatalf("channel %s missing from persisted dataset", channel.ID)
		}
		if updated.StreamKey == originalKey {
			t.Fatalf("expected persisted stream key to differ from original")
		}
		return nil
	}

	rotated, err := store.RotateChannelStreamKey(channel.ID)
	store.persistOverride = nil
	if err != nil {
		t.Fatalf("RotateChannelStreamKey returned error: %v", err)
	}
	if !persisted {
		t.Fatal("expected rotation to persist dataset")
	}
	if rotated.StreamKey == "" {
		t.Fatal("expected rotated stream key to be populated")
	}
	if rotated.StreamKey == originalKey {
		t.Fatal("expected rotated stream key to differ from original")
	}

	fetched, ok := store.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("channel %s not found after rotation", channel.ID)
	}
	if fetched.StreamKey != rotated.StreamKey {
		t.Fatalf("expected fetched stream key %s, got %s", rotated.StreamKey, fetched.StreamKey)
	}
}

func TestRotateChannelStreamKeyPersistFailure(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{DisplayName: "Owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}

	channel, err := store.CreateChannel(owner.ID, "Control", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	store.persistOverride = func(dataset) error {
		return errors.New("persist failed")
	}

	if _, err := store.RotateChannelStreamKey(channel.ID); err == nil {
		t.Fatal("expected rotation error when persist fails")
	}

	store.persistOverride = nil

	fetched, ok := store.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("channel %s not found after failed rotation", channel.ID)
	}
	if fetched.StreamKey != channel.StreamKey {
		t.Fatalf("expected stream key %s to remain after failure, got %s", channel.StreamKey, fetched.StreamKey)
	}
}

func TestStartStreamPersistsIngestMetadata(t *testing.T) {
	fake := &fakeIngestController{bootResponses: []bootResponse{{result: ingest.BootResult{
		PrimaryIngest: "rtmp://primary/live",
		BackupIngest:  "rtmp://backup/live",
		OriginURL:     "http://origin/hls",
		PlaybackURL:   "https://cdn/master.m3u8",
		JobIDs:        []string{"job-1", "job-2"},
		Renditions: []ingest.Rendition{
			{Name: "1080p", ManifestURL: "https://cdn/1080p.m3u8", Bitrate: 6000},
			{Name: "720p", ManifestURL: "https://cdn/720p.m3u8", Bitrate: 4000},
		},
	}}}}
	store := newTestStoreWithController(t, fake)

	user, err := store.CreateUser(CreateUserParams{DisplayName: "Creator", Email: "creator@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(user.ID, "Tech", "science", []string{"hardware"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	session, err := store.StartStream(channel.ID, []string{"1080p", "720p"})
	if err != nil {
		t.Fatalf("StartStream: %v", err)
	}
	if fake.bootCalls != 1 {
		t.Fatalf("expected BootStream to be called once, got %d", fake.bootCalls)
	}
	expectedEndpoints := []string{"rtmp://primary/live", "rtmp://backup/live"}
	if !reflect.DeepEqual(session.IngestEndpoints, expectedEndpoints) {
		t.Fatalf("unexpected ingest endpoints: %v", session.IngestEndpoints)
	}
	if session.OriginURL != "http://origin/hls" {
		t.Fatalf("expected origin URL to persist, got %q", session.OriginURL)
	}
	if session.PlaybackURL != "https://cdn/master.m3u8" {
		t.Fatalf("expected playback URL to persist, got %q", session.PlaybackURL)
	}
	if !reflect.DeepEqual(session.IngestJobIDs, []string{"job-1", "job-2"}) {
		t.Fatalf("unexpected job IDs: %v", session.IngestJobIDs)
	}
	if len(session.RenditionManifests) != 2 {
		t.Fatalf("expected 2 rendition manifests, got %d", len(session.RenditionManifests))
	}
	stored := store.data.StreamSessions[session.ID]
	if stored.PlaybackURL != session.PlaybackURL {
		t.Fatalf("expected stored session to retain playback URL")
	}
}

func TestStartStreamRetriesBootFailures(t *testing.T) {
	fake := &fakeIngestController{bootResponses: []bootResponse{
		{err: errors.New("transcoder offline")},
		{result: ingest.BootResult{OriginURL: "http://origin/hls"}},
	}}
	store := newTestStoreWithController(t, fake, WithIngestRetries(2, 0))

	user, err := store.CreateUser(CreateUserParams{DisplayName: "Creator", Email: "creator@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(user.ID, "Tech", "science", []string{"hardware"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	if _, err := store.StartStream(channel.ID, []string{"1080p"}); err != nil {
		t.Fatalf("StartStream: %v", err)
	}
	if fake.bootCalls != 2 {
		t.Fatalf("expected two boot attempts, got %d", fake.bootCalls)
	}
}

func TestStartStreamDefaultsAttemptsToOne(t *testing.T) {
	fake := &fakeIngestController{bootErr: errors.New("transcoder offline")}
	store := newTestStoreWithController(t, fake, WithIngestRetries(0, 0))

	user, err := store.CreateUser(CreateUserParams{DisplayName: "Creator", Email: "creator@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(user.ID, "Tech", "science", []string{"hardware"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	if _, err := store.StartStream(channel.ID, []string{"1080p"}); err == nil {
		t.Fatal("expected StartStream to fail")
	} else if got := err.Error(); got != "boot ingest: transcoder offline" {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.bootCalls != 1 {
		t.Fatalf("expected one boot attempt, got %d", fake.bootCalls)
	}
}

func TestStartStreamAppliesRetryInterval(t *testing.T) {
	interval := 20 * time.Millisecond
	fake := &fakeIngestController{bootResponses: []bootResponse{
		{err: errors.New("transcoder offline")},
		{err: errors.New("transcoder still offline")},
		{result: ingest.BootResult{OriginURL: "http://origin/hls"}},
	}}
	store := newTestStoreWithController(t, fake, WithIngestRetries(3, interval))

	user, err := store.CreateUser(CreateUserParams{DisplayName: "Creator", Email: "creator@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(user.ID, "Tech", "science", []string{"hardware"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	start := time.Now()
	if _, err := store.StartStream(channel.ID, []string{"1080p"}); err != nil {
		t.Fatalf("StartStream: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 2*interval {
		t.Fatalf("expected retries to wait at least %v, got %v", 2*interval, elapsed)
	}
	if fake.bootCalls != 3 {
		t.Fatalf("expected three boot attempts, got %d", fake.bootCalls)
	}
}

func TestStartStreamFailureRollsBackState(t *testing.T) {
	fake := &fakeIngestController{bootResponses: []bootResponse{
		{err: errors.New("network error")},
		{err: errors.New("still failing")},
	}}
	store := newTestStoreWithController(t, fake, WithIngestRetries(2, 0))

	user, err := store.CreateUser(CreateUserParams{DisplayName: "Creator", Email: "creator@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(user.ID, "Tech", "science", []string{"hardware"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	if _, err := store.StartStream(channel.ID, []string{"1080p"}); err == nil {
		t.Fatal("expected StartStream to fail after retries")
	}
	updated, ok := store.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("channel %s not found", channel.ID)
	}
	if updated.LiveState != "offline" {
		t.Fatalf("expected channel to remain offline, got %s", updated.LiveState)
	}
	if updated.CurrentSessionID != nil {
		t.Fatalf("expected current session to remain nil")
	}
}

func TestStopStreamInvokesShutdown(t *testing.T) {
	fake := &fakeIngestController{bootResponses: []bootResponse{{result: ingest.BootResult{
		JobIDs: []string{"job-123"},
	}}}}
	store := newTestStoreWithController(t, fake)

	user, err := store.CreateUser(CreateUserParams{DisplayName: "Creator", Email: "creator@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(user.ID, "Tech", "science", []string{"hardware"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	session, err := store.StartStream(channel.ID, []string{"1080p"})
	if err != nil {
		t.Fatalf("StartStream: %v", err)
	}
	stopped, err := store.StopStream(channel.ID, 25)
	if err != nil {
		t.Fatalf("StopStream: %v", err)
	}
	if stopped.EndedAt == nil {
		t.Fatal("expected stop to set endedAt")
	}
	if len(fake.shutdownCalls) != 1 {
		t.Fatalf("expected shutdown to be invoked once, got %d", len(fake.shutdownCalls))
	}
	call := fake.shutdownCalls[0]
	if call.channelID != channel.ID || call.sessionID != session.ID {
		t.Fatalf("unexpected shutdown call payload: %+v", call)
	}
	if !reflect.DeepEqual(call.jobIDs, []string{"job-123"}) {
		t.Fatalf("expected job IDs to propagate, got %v", call.jobIDs)
	}
}

func TestDeleteChannelRemovesArtifacts(t *testing.T) {
	store := newTestStore(t)
	owner, err := store.CreateUser(CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	viewer, err := store.CreateUser(CreateUserParams{DisplayName: "Viewer", Email: "viewer@example.com"})
	if err != nil {
		t.Fatalf("CreateUser viewer: %v", err)
	}

	channel, err := store.CreateChannel(owner.ID, "Main", "gaming", []string{"retro"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	session, err := store.StartStream(channel.ID, []string{"1080p"})
	if err != nil {
		t.Fatalf("StartStream: %v", err)
	}
	if _, err := store.StopStream(channel.ID, 10); err != nil {
		t.Fatalf("StopStream: %v", err)
	}
	if _, err := store.CreateChatMessage(channel.ID, owner.ID, "hello"); err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}
	if err := store.FollowChannel(viewer.ID, channel.ID); err != nil {
		t.Fatalf("FollowChannel: %v", err)
	}

	if err := store.DeleteChannel(channel.ID); err != nil {
		t.Fatalf("DeleteChannel: %v", err)
	}
	if _, ok := store.GetChannel(channel.ID); ok {
		t.Fatalf("expected channel to be removed")
	}
	if _, err := store.ListStreamSessions(channel.ID); err == nil {
		t.Fatalf("expected ListStreamSessions to error for deleted channel")
	}
	if _, ok := store.data.StreamSessions[session.ID]; ok {
		t.Fatalf("expected session %s to be removed", session.ID)
	}
	if followers := store.CountFollowers(channel.ID); followers != 0 {
		t.Fatalf("expected follower count reset, got %d", followers)
	}
	if following := store.ListFollowedChannelIDs(viewer.ID); len(following) != 0 {
		t.Fatalf("expected viewer follow list to be cleared, got %v", following)
	}
}
