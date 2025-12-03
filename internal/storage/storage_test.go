package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"reflect"

	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
)

func newTestStore(t *testing.T) *Storage {
	return newTestStoreWithController(t, ingest.NoopController{})
}

func newTestStoreWithController(t *testing.T, controller ingest.Controller, extra ...Option) *Storage {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	if controller == nil {
		controller = ingest.NoopController{}
	}
	opts := []Option{WithIngestController(controller), WithIngestRetries(1, 0)}
	opts = append(opts, extra...)
	store, err := NewStorage(path, opts...)
	if err != nil {
		t.Fatalf("NewStorage error: %v", err)
	}
	return store
}

func jsonRepositoryFactory(t *testing.T, opts ...Option) (Repository, func(), error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	defaults := []Option{WithIngestController(ingest.NoopController{}), WithIngestRetries(1, 0)}
	opts = append(defaults, opts...)
	store, err := NewStorage(path, opts...)
	if err != nil {
		return nil, nil, err
	}
	return store, func() {}, nil
}

type bootResponse struct {
	result ingest.BootResult
	err    error
}

type shutdownCall struct {
	channelID string
	sessionID string
	jobIDs    []string
}

type fakeIngestController struct {
	bootResponses   []bootResponse
	bootDefault     ingest.BootResult
	bootErr         error
	bootCalls       int
	shutdownErr     error
	shutdownCalls   []shutdownCall
	healthResponses [][]ingest.HealthStatus
	healthCalls     int
}

func (f *fakeIngestController) BootStream(ctx context.Context, params ingest.BootParams) (ingest.BootResult, error) {
	idx := f.bootCalls
	f.bootCalls++
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

func firstRecordingID(store *Storage) string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	for id := range store.data.Recordings {
		return id
	}
	return ""
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

func TestStorageIngestHealthSnapshots(t *testing.T) {
	RunRepositoryIngestHealthSnapshots(t, jsonRepositoryFactory)
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

func TestDeleteUserPersistFailureLeavesDataUntouched(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}

	target, err := store.CreateUser(CreateUserParams{
		DisplayName: "Target",
		Email:       "target@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser target: %v", err)
	}

	channel, err := store.CreateChannel(owner.ID, "Main", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if _, err := store.CreateChatMessage(channel.ID, target.ID, "hello"); err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}

	bio := "Hi"
	if _, err := store.UpsertProfile(target.ID, ProfileUpdate{Bio: &bio}); err != nil {
		t.Fatalf("UpsertProfile: %v", err)
	}

	friendBio := "Friend"
	topFriends := []string{target.ID}
	if _, err := store.UpsertProfile(owner.ID, ProfileUpdate{Bio: &friendBio, TopFriends: &topFriends}); err != nil {
		t.Fatalf("UpsertProfile friend: %v", err)
	}

	store.persistOverride = func(dataset) error {
		return errors.New("persist failed")
	}

	if err := store.DeleteUser(target.ID); err == nil {
		t.Fatalf("expected DeleteUser error when persist fails")
	}

	store.persistOverride = nil

	if _, ok := store.GetUser(target.ID); !ok {
		t.Fatalf("expected user %s to remain", target.ID)
	}

	profile, ok := store.GetProfile(target.ID)
	if !ok {
		t.Fatalf("expected profile for %s to remain", target.ID)
	}
	if profile.Bio != bio {
		t.Fatalf("expected profile bio %q, got %q", bio, profile.Bio)
	}

	friendProfile, ok := store.GetProfile(owner.ID)
	if !ok {
		t.Fatalf("expected owner profile to exist")
	}
	if len(friendProfile.TopFriends) != 1 || friendProfile.TopFriends[0] != target.ID {
		t.Fatalf("expected top friends to remain unchanged, got %v", friendProfile.TopFriends)
	}

	if messages, err := store.ListChatMessages(channel.ID, 0); err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	} else if len(messages) != 1 {
		t.Fatalf("expected chat messages to remain, got %d", len(messages))
	}
}

func TestUpsertProfilePersistFailureLeavesDataUntouched(t *testing.T) {
	store := newTestStore(t)

	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "User",
		Email:       "user@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	initialBio := "initial"
	if _, err := store.UpsertProfile(user.ID, ProfileUpdate{Bio: &initialBio}); err != nil {
		t.Fatalf("UpsertProfile initial: %v", err)
	}

	updatedBio := "updated"
	store.persistOverride = func(dataset) error {
		return errors.New("persist failed")
	}

	if _, err := store.UpsertProfile(user.ID, ProfileUpdate{Bio: &updatedBio}); err == nil {
		t.Fatalf("expected UpsertProfile error when persist fails")
	}

	store.persistOverride = nil

	profile, _ := store.GetProfile(user.ID)
	if profile.Bio != initialBio {
		t.Fatalf("expected bio %q, got %q", initialBio, profile.Bio)
	}
}

func TestUpdateChannelPersistFailureLeavesDataUntouched(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	channel, err := store.CreateChannel(owner.ID, "Title", "gaming", []string{"fun"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	newTitle := "Updated"
	store.persistOverride = func(dataset) error {
		return errors.New("persist failed")
	}

	if _, err := store.UpdateChannel(channel.ID, ChannelUpdate{Title: &newTitle}); err == nil {
		t.Fatalf("expected UpdateChannel error when persist fails")
	}

	store.persistOverride = nil

	current, ok := store.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("expected channel to remain")
	}
	if current.Title != channel.Title {
		t.Fatalf("expected title %q, got %q", channel.Title, current.Title)
	}
}

func TestDeleteChannelPersistFailureLeavesDataUntouched(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
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

	store.persistOverride = func(dataset) error {
		return errors.New("persist failed")
	}

	if err := store.DeleteChannel(channel.ID); err == nil {
		t.Fatalf("expected DeleteChannel error when persist fails")
	}

	store.persistOverride = nil

	if _, ok := store.GetChannel(channel.ID); !ok {
		t.Fatalf("expected channel to remain")
	}
	if _, ok := store.data.StreamSessions[session.ID]; !ok {
		t.Fatalf("expected stream session to remain")
	}
	if messages, err := store.ListChatMessages(channel.ID, 0); err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	} else if len(messages) != 1 {
		t.Fatalf("expected chat message to remain, got %d", len(messages))
	}
}

func TestStorageLoadsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	store, err := NewStorage(path)
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	if users := store.ListUsers(); len(users) != 0 {
		t.Fatalf("expected no users, got %d", len(users))
	}

	if _, err := store.CreateUser(CreateUserParams{DisplayName: "Alice", Email: "alice@example.com"}); err != nil {
		t.Fatalf("CreateUser on recovered store: %v", err)
	}
}

func TestPersistUsesAtomicReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission semantics differ on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	store, err := NewStorage(path)
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}

	if _, err := store.CreateUser(CreateUserParams{DisplayName: "Alice", Email: "alice@example.com"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected permissions 0600, got %o", perm)
	}
}

func TestStoragePersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")

	store, err := NewStorage(path)
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Alice",
		Email:       "alice@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := store.persist(); err != nil {
		t.Fatalf("persist: %v", err)
	}

	// reopen store and ensure data is present
	reopened, err := NewStorage(path)
	if err != nil {
		t.Fatalf("NewStorage reopen: %v", err)
	}
	if got, ok := reopened.GetUser(user.ID); !ok {
		t.Fatalf("expected to find persisted user %s", user.ID)
	} else if got.Email != user.Email {
		t.Fatalf("expected email %s, got %s", user.Email, got.Email)
	}
}

func TestUpsertProfileCreatesProfile(t *testing.T) {
	store := newTestStore(t)
	owner, err := store.CreateUser(CreateUserParams{
		DisplayName: "Streamer",
		Email:       "streamer@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	friend, err := store.CreateUser(CreateUserParams{
		DisplayName: "Friend",
		Email:       "friend@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser friend: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Main Stage", "music", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	bio := "Welcome to my river stage"
	avatar := "https://cdn.example.com/avatar.png"
	banner := "https://cdn.example.com/banner.png"
	featured := channel.ID
	topFriends := []string{friend.ID}
	donation := []models.CryptoAddress{{Currency: "eth", Address: "0xabc", Note: "Primary"}}

	profile, err := store.UpsertProfile(owner.ID, ProfileUpdate{
		Bio:               &bio,
		AvatarURL:         &avatar,
		BannerURL:         &banner,
		FeaturedChannelID: &featured,
		TopFriends:        &topFriends,
		DonationAddresses: &donation,
	})
	if err != nil {
		t.Fatalf("UpsertProfile: %v", err)
	}

	if profile.Bio != bio {
		t.Fatalf("expected bio %q, got %q", bio, profile.Bio)
	}
	if profile.FeaturedChannelID == nil || *profile.FeaturedChannelID != channel.ID {
		t.Fatalf("expected featured channel %s", channel.ID)
	}
	if len(profile.TopFriends) != 1 || profile.TopFriends[0] != friend.ID {
		t.Fatalf("expected top friends to include %s", friend.ID)
	}
	if len(profile.DonationAddresses) != 1 {
		t.Fatalf("expected 1 donation address, got %d", len(profile.DonationAddresses))
	}
	if profile.DonationAddresses[0].Currency != "ETH" {
		t.Fatalf("expected currency to be normalized to ETH, got %s", profile.DonationAddresses[0].Currency)
	}
	if profile.CreatedAt.IsZero() || profile.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps to be populated")
	}

	loaded, ok := store.GetProfile(owner.ID)
	if !ok {
		t.Fatalf("expected persisted profile")
	}
	if loaded.UpdatedAt.Before(profile.UpdatedAt) {
		t.Fatalf("expected loaded profile updated at >= stored profile")
	}

	// second update clears top friends and replaces donation details
	topFriends = []string{}
	donation = []models.CryptoAddress{{Currency: "btc", Address: "bc1xyz"}}
	updated, err := store.UpsertProfile(owner.ID, ProfileUpdate{
		TopFriends:        &topFriends,
		DonationAddresses: &donation,
	})
	if err != nil {
		t.Fatalf("UpsertProfile second update: %v", err)
	}
	if len(updated.TopFriends) != 0 {
		t.Fatalf("expected top friends cleared")
	}
	if len(updated.DonationAddresses) != 1 || updated.DonationAddresses[0].Currency != "BTC" {
		t.Fatalf("expected BTC donation address")
	}

	_, existing := store.GetProfile(friend.ID)
	if existing {
		t.Fatalf("expected friend to have no explicit profile yet")
	}
}

func TestUpsertProfileDonationValidation(t *testing.T) {
	store := newTestStore(t)
	owner, err := store.CreateUser(CreateUserParams{
		DisplayName: "Creator",
		Email:       "creator@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}

	valid := []models.CryptoAddress{{Currency: "eth", Address: "0xabc123"}}
	if _, err := store.UpsertProfile(owner.ID, ProfileUpdate{DonationAddresses: &valid}); err != nil {
		t.Fatalf("expected valid donation addresses to succeed: %v", err)
	}

	testCases := []struct {
		name     string
		donation []models.CryptoAddress
	}{
		{
			name:     "invalid currency",
			donation: []models.CryptoAddress{{Currency: "et1", Address: "0xabc123"}},
		},
		{
			name:     "too short",
			donation: []models.CryptoAddress{{Currency: "ETH", Address: "abc"}},
		},
		{
			name:     "invalid characters",
			donation: []models.CryptoAddress{{Currency: "ETH", Address: "bad address"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := store.UpsertProfile(owner.ID, ProfileUpdate{DonationAddresses: &tc.donation})
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestUpsertProfileTopFriendsLimit(t *testing.T) {
	store := newTestStore(t)
	owner, err := store.CreateUser(CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}

	friendIDs := make([]string, 0, 9)
	for i := 0; i < 9; i++ {
		friend, err := store.CreateUser(CreateUserParams{
			DisplayName: "Friend",
			Email:       fmt.Sprintf("friend%d@example.com", i),
		})
		if err != nil {
			t.Fatalf("CreateUser friend %d: %v", i, err)
		}
		friendIDs = append(friendIDs, friend.ID)
	}

	if _, err := store.UpsertProfile(owner.ID, ProfileUpdate{TopFriends: &friendIDs}); err == nil {
		t.Fatalf("expected error for more than eight top friends")
	}
}

func TestFollowChannelLifecycle(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{DisplayName: "Creator", Email: "creator@example.com"})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	viewer, err := store.CreateUser(CreateUserParams{DisplayName: "Viewer", Email: "viewer@example.com"})
	if err != nil {
		t.Fatalf("CreateUser viewer: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Workshop", "maker", []string{"cnc"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	if count := store.CountFollowers(channel.ID); count != 0 {
		t.Fatalf("expected zero followers, got %d", count)
	}
	if store.IsFollowingChannel(viewer.ID, channel.ID) {
		t.Fatal("expected viewer to not follow channel")
	}
	if followed := store.ListFollowedChannelIDs(viewer.ID); followed != nil {
		t.Fatalf("expected no followed channels, got %v", followed)
	}

	if err := store.FollowChannel(viewer.ID, channel.ID); err != nil {
		t.Fatalf("FollowChannel: %v", err)
	}
	if err := store.FollowChannel(viewer.ID, channel.ID); err != nil {
		t.Fatalf("FollowChannel idempotency: %v", err)
	}
	if count := store.CountFollowers(channel.ID); count != 1 {
		t.Fatalf("expected one follower, got %d", count)
	}
	if !store.IsFollowingChannel(viewer.ID, channel.ID) {
		t.Fatal("expected viewer to follow channel")
	}
	followed := store.ListFollowedChannelIDs(viewer.ID)
	if len(followed) != 1 || followed[0] != channel.ID {
		t.Fatalf("unexpected followed list: %v", followed)
	}

	if err := store.UnfollowChannel(viewer.ID, channel.ID); err != nil {
		t.Fatalf("UnfollowChannel: %v", err)
	}
	if err := store.UnfollowChannel(viewer.ID, channel.ID); err != nil {
		t.Fatalf("UnfollowChannel idempotency: %v", err)
	}
	if count := store.CountFollowers(channel.ID); count != 0 {
		t.Fatalf("expected zero followers after unfollow, got %d", count)
	}
	if store.IsFollowingChannel(viewer.ID, channel.ID) {
		t.Fatal("expected viewer to not follow channel after unfollow")
	}
}

func TestListFollowedChannelIDsOrdersByRecency(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{DisplayName: "Creator", Email: "creator@example.com"})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	viewer, err := store.CreateUser(CreateUserParams{DisplayName: "Viewer", Email: "viewer@example.com"})
	if err != nil {
		t.Fatalf("CreateUser viewer: %v", err)
	}
	first, err := store.CreateChannel(owner.ID, "Alpha", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel alpha: %v", err)
	}
	second, err := store.CreateChannel(owner.ID, "Beta", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel beta: %v", err)
	}

	if err := store.FollowChannel(viewer.ID, first.ID); err != nil {
		t.Fatalf("FollowChannel alpha: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := store.FollowChannel(viewer.ID, second.ID); err != nil {
		t.Fatalf("FollowChannel beta: %v", err)
	}

	followed := store.ListFollowedChannelIDs(viewer.ID)
	if len(followed) != 2 || followed[0] != second.ID || followed[1] != first.ID {
		t.Fatalf("expected channels ordered by recency, got %v", followed)
	}
}

func TestRepositoryStreamKeyRotation(t *testing.T) {
	RunRepositoryStreamKeyRotation(t, jsonRepositoryFactory)
}

func TestRepositoryOAuthLinking(t *testing.T) {
	RunRepositoryOAuthLinking(t, jsonRepositoryFactory)
}

func TestRepositoryChannelSearch(t *testing.T) {
	RunRepositoryChannelSearch(t, jsonRepositoryFactory)
}

func TestRepositoryChannelLookupByStreamKey(t *testing.T) {
	RunRepositoryChannelLookupByStreamKey(t, jsonRepositoryFactory)
}

func TestRepositoryStreamLifecycleWithoutIngest(t *testing.T) {
	RunRepositoryStreamLifecycleWithoutIngest(t, jsonRepositoryFactory)
}

func TestRepositoryStreamTimeouts(t *testing.T) {
	RunRepositoryStreamTimeouts(t, jsonRepositoryFactory)
}

func TestMain(m *testing.M) {
	// ensure tests do not leave temp files behind by relying on testing package cleanup
	code := m.Run()
	os.Exit(code)
}
