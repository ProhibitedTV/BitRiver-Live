package storage

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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

func TestCreateAndListUser(t *testing.T) {
	store := newTestStore(t)

	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Alice",
		Email:       "alice@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if user.ID == "" {
		t.Fatal("expected user ID to be set")
	}

	users := store.ListUsers()
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Email != "alice@example.com" {
		t.Fatalf("expected email alice@example.com, got %s", users[0].Email)
	}
}

func TestAuthenticateUser(t *testing.T) {
	store := newTestStore(t)
	password := "hunter42!"
	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Viewer",
		Email:       "viewer@example.com",
		Password:    password,
		SelfSignup:  true,
	})
	if err != nil {
		t.Fatalf("CreateUser self signup: %v", err)
	}
	if !user.SelfSignup {
		t.Fatalf("expected self signup flag to be set")
	}
	if user.PasswordHash == "" {
		t.Fatal("expected password hash to be stored")
	}
	if user.PasswordHash == password {
		t.Fatal("expected password hash to differ from password")
	}
	parts := strings.Split(user.PasswordHash, "$")
	if len(parts) != 5 {
		t.Fatalf("unexpected hash format: %s", user.PasswordHash)
	}
	if parts[0] != "pbkdf2" || parts[1] != "sha256" {
		t.Fatalf("unexpected hash identifiers: %v", parts[:2])
	}
	if parts[2] != strconv.Itoa(passwordHashIterations) {
		t.Fatalf("expected iteration count %d, got %s", passwordHashIterations, parts[2])
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		t.Fatalf("decode salt: %v", err)
	}
	if len(salt) != passwordHashSaltLength {
		t.Fatalf("expected salt length %d, got %d", passwordHashSaltLength, len(salt))
	}
	derived, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		t.Fatalf("decode derived key: %v", err)
	}
	if len(derived) != passwordHashKeyLength {
		t.Fatalf("expected key length %d, got %d", passwordHashKeyLength, len(derived))
	}
	if verifyErr := verifyPassword(user.PasswordHash, password); verifyErr != nil {
		t.Fatalf("verifyPassword failed: %v", verifyErr)
	}

	authenticated, err := store.AuthenticateUser("viewer@example.com", password)
	if err != nil {
		t.Fatalf("AuthenticateUser returned error: %v", err)
	}
	if authenticated.ID != user.ID {
		t.Fatalf("expected authenticated user %s, got %s", user.ID, authenticated.ID)
	}

	if _, err := store.AuthenticateUser("viewer@example.com", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid password error, got %v", err)
	}
	if _, err := store.AuthenticateUser("unknown@example.com", password); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected unknown email to return ErrInvalidCredentials, got %v", err)
	}

	reloaded, err := NewStorage(store.filePath)
	if err != nil {
		t.Fatalf("reload storage: %v", err)
	}
	persisted, ok := reloaded.FindUserByEmail("viewer@example.com")
	if !ok {
		t.Fatal("expected persisted user to be found after reload")
	}
	if persisted.PasswordHash != user.PasswordHash {
		t.Fatalf("expected password hash to persist across reloads")
	}
	if _, err := reloaded.AuthenticateUser("viewer@example.com", password); err != nil {
		t.Fatalf("AuthenticateUser on reloaded store returned error: %v", err)
	}
}

func TestUpdateAndDeleteUser(t *testing.T) {
	store := newTestStore(t)

	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Alice",
		Email:       "alice@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	newDisplay := "Alice Cooper"
	newEmail := "alice.cooper@example.com"
	newRoles := []string{"Admin", "moderator", "admin"}
	updated, err := store.UpdateUser(user.ID, UserUpdate{DisplayName: &newDisplay, Email: &newEmail, Roles: &newRoles})
	if err != nil {
		t.Fatalf("UpdateUser returned error: %v", err)
	}
	if updated.DisplayName != newDisplay {
		t.Fatalf("expected display name %q, got %q", newDisplay, updated.DisplayName)
	}
	if updated.Email != "alice.cooper@example.com" {
		t.Fatalf("expected email normalized, got %s", updated.Email)
	}
	if len(updated.Roles) != 2 {
		t.Fatalf("expected deduplicated roles, got %v", updated.Roles)
	}

	if err := store.DeleteUser(user.ID); err != nil {
		t.Fatalf("DeleteUser returned error: %v", err)
	}
	if _, ok := store.GetUser(user.ID); ok {
		t.Fatalf("expected user to be removed")
	}
}

func TestUpdateUserPersistFailureLeavesDataUntouched(t *testing.T) {
	store := newTestStore(t)

	original, err := store.CreateUser(CreateUserParams{
		DisplayName: "Alice",
		Email:       "alice@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	newEmail := "changed@example.com"
	store.persistOverride = func(dataset) error {
		return errors.New("persist failed")
	}

	if _, err := store.UpdateUser(original.ID, UserUpdate{Email: &newEmail}); err == nil {
		t.Fatalf("expected UpdateUser error when persist fails")
	}

	store.persistOverride = nil

	current, ok := store.GetUser(original.ID)
	if !ok {
		t.Fatalf("expected user %s to remain", original.ID)
	}
	if current.Email != original.Email {
		t.Fatalf("expected email %s, got %s", original.Email, current.Email)
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
	responses := [][]ingest.HealthStatus{
		{{Component: "srs", Status: "ok"}},
		{{Component: "transcoder", Status: "error", Detail: "timeout"}},
	}
	fake := &fakeIngestController{healthResponses: responses}
	store := newTestStoreWithController(t, fake)

	first := store.IngestHealth(context.Background())
	if fake.healthCalls != 1 {
		t.Fatalf("expected health to be queried once, got %d", fake.healthCalls)
	}
	if !reflect.DeepEqual(first, responses[0]) {
		t.Fatalf("unexpected health payload: %+v", first)
	}
	recorded, ts1 := store.LastIngestHealth()
	if !reflect.DeepEqual(recorded, first) {
		t.Fatalf("expected last health to match recorded snapshot")
	}
	if ts1.IsZero() {
		t.Fatal("expected health timestamp to be set")
	}

	second := store.IngestHealth(context.Background())
	if fake.healthCalls != 2 {
		t.Fatalf("expected second health call to increment counter, got %d", fake.healthCalls)
	}
	if !reflect.DeepEqual(second, responses[1]) {
		t.Fatalf("unexpected second health payload: %+v", second)
	}
	recorded, ts2 := store.LastIngestHealth()
	if !reflect.DeepEqual(recorded, second) {
		t.Fatalf("expected snapshot to update on subsequent call")
	}
	if ts2.Before(ts1) {
		t.Fatal("expected subsequent health timestamp to be >= initial timestamp")
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

func TestListChatMessagesOrdering(t *testing.T) {
	store := newTestStore(t)
	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Alice",
		Email:       "alice@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(user.ID, "My Channel", "", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	msg1, err := store.CreateChatMessage(channel.ID, user.ID, "first")
	if err != nil {
		t.Fatalf("CreateChatMessage #1: %v", err)
	}
	msg2, err := store.CreateChatMessage(channel.ID, user.ID, "second")
	if err != nil {
		t.Fatalf("CreateChatMessage #2: %v", err)
	}

	msgs, err := store.ListChatMessages(channel.ID, 0)
	if err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].ID != msg2.ID {
		t.Fatalf("expected newest message first, got %s", msgs[0].ID)
	}
	if msgs[1].ID != msg1.ID {
		t.Fatalf("expected oldest message last, got %s", msgs[1].ID)
	}
}

func TestDeleteChatMessage(t *testing.T) {
	store := newTestStore(t)
	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Alice",
		Email:       "alice@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(user.ID, "My Channel", "", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	msg, err := store.CreateChatMessage(channel.ID, user.ID, "hello")
	if err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}

	if err := store.DeleteChatMessage(channel.ID, msg.ID); err != nil {
		t.Fatalf("DeleteChatMessage: %v", err)
	}
	if err := store.DeleteChatMessage(channel.ID, msg.ID); err == nil {
		t.Fatalf("expected error when deleting already removed message")
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

func TestChatReportsLifecycle(t *testing.T) {
        store := newTestStore(t)
        owner, err := store.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
        if err != nil {
                t.Fatalf("create owner: %v", err)
        }
        reporter, err := store.CreateUser(CreateUserParams{DisplayName: "reporter", Email: "reporter@example.com"})
        if err != nil {
                t.Fatalf("create reporter: %v", err)
        }
        target, err := store.CreateUser(CreateUserParams{DisplayName: "target", Email: "target@example.com"})
        if err != nil {
                t.Fatalf("create target: %v", err)
        }
        channel, err := store.CreateChannel(owner.ID, "Lobby", "gaming", nil)
        if err != nil {
                t.Fatalf("create channel: %v", err)
        }

        report, err := store.CreateChatReport(channel.ID, reporter.ID, target.ID, "spam", "msg-1", "")
        if err != nil {
                t.Fatalf("CreateChatReport: %v", err)
        }
        if report.Status != "open" {
                t.Fatalf("expected new report to be open")
        }

        pending, err := store.ListChatReports(channel.ID, false)
        if err != nil {
                t.Fatalf("ListChatReports: %v", err)
        }
        if len(pending) != 1 {
                t.Fatalf("expected 1 pending report, got %d", len(pending))
        }

        resolved, err := store.ResolveChatReport(report.ID, owner.ID, "handled")
        if err != nil {
                t.Fatalf("ResolveChatReport: %v", err)
        }
        if resolved.Status != "resolved" || resolved.Resolution != "handled" {
                t.Fatalf("expected resolved report, got %+v", resolved)
        }

        pending, err = store.ListChatReports(channel.ID, false)
        if err != nil {
                t.Fatalf("ListChatReports pending: %v", err)
        }
        if len(pending) != 0 {
                t.Fatalf("expected no pending reports, got %d", len(pending))
        }

        all, err := store.ListChatReports(channel.ID, true)
        if err != nil {
                t.Fatalf("ListChatReports(includeResolved): %v", err)
        }
        if len(all) != 1 {
                t.Fatalf("expected resolved report to be listed, got %d", len(all))
        }
}

func TestCreateTipAndList(t *testing.T) {
        store := newTestStore(t)
        owner, err := store.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
        if err != nil {
                t.Fatalf("create owner: %v", err)
        }
        supporter, err := store.CreateUser(CreateUserParams{DisplayName: "fan", Email: "fan@example.com"})
        if err != nil {
                t.Fatalf("create supporter: %v", err)
        }
        channel, err := store.CreateChannel(owner.ID, "Lobby", "gaming", nil)
        if err != nil {
                t.Fatalf("create channel: %v", err)
        }

        tip, err := store.CreateTip(CreateTipParams{
                ChannelID:  channel.ID,
                FromUserID: supporter.ID,
                Amount:     5.5,
                Currency:   "usd",
                Provider:   "stripe",
                Reference:  "ref-1",
                Message:    "keep it up",
        })
        if err != nil {
                t.Fatalf("CreateTip: %v", err)
        }
        if tip.ID == "" {
                t.Fatalf("expected tip id to be set")
        }

        tips, err := store.ListTips(channel.ID, 10)
        if err != nil {
                t.Fatalf("ListTips: %v", err)
        }
        if len(tips) != 1 || tips[0].ID != tip.ID {
                t.Fatalf("expected persisted tip, got %+v", tips)
        }
}

func TestCreateSubscriptionAndCancel(t *testing.T) {
        store := newTestStore(t)
        owner, err := store.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
        if err != nil {
                t.Fatalf("create owner: %v", err)
        }
        viewer, err := store.CreateUser(CreateUserParams{DisplayName: "viewer", Email: "viewer@example.com"})
        if err != nil {
                t.Fatalf("create viewer: %v", err)
        }
        channel, err := store.CreateChannel(owner.ID, "Lobby", "gaming", nil)
        if err != nil {
                t.Fatalf("create channel: %v", err)
        }

        subscription, err := store.CreateSubscription(CreateSubscriptionParams{
                ChannelID: channel.ID,
                UserID:    viewer.ID,
                Tier:      "gold",
                Provider:  "stripe",
                Amount:    9.99,
                Currency:  "usd",
                Duration:  30 * 24 * time.Hour,
                AutoRenew: true,
        })
        if err != nil {
                t.Fatalf("CreateSubscription: %v", err)
        }
        if subscription.Status != "active" {
                t.Fatalf("expected active subscription")
        }

        subs, err := store.ListSubscriptions(channel.ID, false)
        if err != nil {
                t.Fatalf("ListSubscriptions: %v", err)
        }
        if len(subs) != 1 {
                t.Fatalf("expected one active subscription, got %d", len(subs))
        }

        cancelled, err := store.CancelSubscription(subscription.ID, viewer.ID, "")
        if err != nil {
                t.Fatalf("CancelSubscription: %v", err)
        }
        if cancelled.Status != "cancelled" || cancelled.CancelledBy != viewer.ID {
                t.Fatalf("expected subscription to be cancelled by viewer, got %+v", cancelled)
        }

        subs, err = store.ListSubscriptions(channel.ID, false)
        if err != nil {
                t.Fatalf("ListSubscriptions after cancel: %v", err)
        }
        if len(subs) != 0 {
                t.Fatalf("expected no active subscriptions after cancellation")
        }
        subs, err = store.ListSubscriptions(channel.ID, true)
        if err != nil {
                t.Fatalf("ListSubscriptions include inactive: %v", err)
        }
        if len(subs) != 1 {
                t.Fatalf("expected cancelled subscription to be listed, got %d", len(subs))
        }
}

func TestMain(m *testing.M) {
        // ensure tests do not leave temp files behind by relying on testing package cleanup
        code := m.Run()
        os.Exit(code)
}
