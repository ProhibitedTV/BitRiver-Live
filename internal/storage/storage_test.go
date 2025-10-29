package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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

type fakeObjectStorage struct {
	uploads []fakeUpload
	deletes []string
	prefix  string
	baseURL string
}

type memoryS3Server struct {
	mu       sync.Mutex
	objects  map[string]map[string][]byte
	requests []memoryS3Request
}

type memoryS3Request struct {
	Method        string
	Authorization string
	ContentSHA    string
}

func newMemoryS3Server() *memoryS3Server {
	return &memoryS3Server{objects: make(map[string]map[string][]byte)}
}

func (m *memoryS3Server) addBucket(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.objects[name]; !exists {
		m.objects[name] = make(map[string][]byte)
	}
}

func (m *memoryS3Server) getObject(bucket, key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	objs, ok := m.objects[bucket]
	if !ok {
		return nil, false
	}
	data, ok := objs[key]
	if !ok {
		return nil, false
	}
	copyData := append([]byte(nil), data...)
	return copyData, true
}

func (m *memoryS3Server) lastRequest() memoryS3Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.requests) == 0 {
		return memoryS3Request{}
	}
	return m.requests[len(m.requests)-1]
}

func (m *memoryS3Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	bucket, key, err := parseS3Path(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusInternalServerError)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, memoryS3Request{
		Method:        r.Method,
		Authorization: r.Header.Get("Authorization"),
		ContentSHA:    r.Header.Get("X-Amz-Content-Sha256"),
	})
	bucketObjects, exists := m.objects[bucket]
	if !exists {
		http.Error(w, "bucket not found", http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodPut:
		bucketObjects[key] = append([]byte(nil), body...)
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		delete(bucketObjects, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func parseS3Path(path string) (string, string, error) {
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" {
		return "", "", fmt.Errorf("missing bucket")
	}
	parts := strings.SplitN(trimmed, "/", 2)
	bucket := parts[0]
	key := ""
	if len(parts) == 2 {
		key = parts[1]
	}
	if bucket == "" {
		return "", "", fmt.Errorf("missing bucket")
	}
	return bucket, key, nil
}

func TestS3ObjectStorageClientUploadDelete(t *testing.T) {
	server := newMemoryS3Server()
	server.addBucket("vod")
	ts := httptest.NewServer(server)
	defer ts.Close()

	cfg := ObjectStorageConfig{
		Endpoint:       strings.TrimPrefix(ts.URL, "http://"),
		Region:         "us-east-1",
		AccessKey:      "AKIAEXAMPLE",
		SecretKey:      "secretKeyExample",
		Bucket:         "vod",
		UseSSL:         false,
		Prefix:         "vod/assets",
		PublicEndpoint: "https://cdn.example.com/content",
	}

	client := newObjectStorageClient(cfg)
	s3Client, ok := client.(*s3ObjectStorageClient)
	if !ok {
		t.Fatalf("expected s3ObjectStorageClient, got %T", client)
	}

	ctx := context.Background()
	payload := []byte("stream manifest data")
	ref, err := s3Client.Upload(ctx, "manifests/stream.m3u8", "application/vnd.apple.mpegurl", payload)
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	expectedKey := "vod/assets/manifests/stream.m3u8"
	if ref.Key != expectedKey {
		t.Fatalf("expected key %s, got %s", expectedKey, ref.Key)
	}
	expectedURL := "https://cdn.example.com/content/" + expectedKey
	if ref.URL != expectedURL {
		t.Fatalf("expected url %s, got %s", expectedURL, ref.URL)
	}
	stored, ok := server.getObject("vod", expectedKey)
	if !ok {
		t.Fatalf("expected object %s to be stored", expectedKey)
	}
	if !bytes.Equal(stored, payload) {
		t.Fatalf("stored payload mismatch: got %q", stored)
	}
	uploadReq := server.lastRequest()
	if uploadReq.Method != http.MethodPut {
		t.Fatalf("expected PUT request, got %s", uploadReq.Method)
	}
	if uploadReq.Authorization == "" || !strings.Contains(uploadReq.Authorization, cfg.AccessKey) {
		t.Fatal("expected authorization header to include access key")
	}
	if uploadReq.ContentSHA == "" {
		t.Fatal("expected content hash header to be set")
	}

	if err := s3Client.Delete(ctx, ref.Key); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, ok := server.getObject("vod", expectedKey); ok {
		t.Fatalf("expected object %s to be removed", expectedKey)
	}
	deleteReq := server.lastRequest()
	if deleteReq.Method != http.MethodDelete {
		t.Fatalf("expected DELETE request, got %s", deleteReq.Method)
	}
	if deleteReq.Authorization == "" || !strings.Contains(deleteReq.Authorization, cfg.AccessKey) {
		t.Fatal("expected delete request to include authorization header")
	}
}

type fakeUpload struct {
	Key         string
	ContentType string
	Body        []byte
	URL         string
}

func (f *fakeObjectStorage) Enabled() bool { return true }

func (f *fakeObjectStorage) Upload(ctx context.Context, key, contentType string, body []byte) (objectReference, error) {
	trimmed := strings.TrimLeft(key, "/")
	prefix := strings.Trim(f.prefix, "/")
	finalKey := trimmed
	if prefix != "" {
		if finalKey != "" {
			finalKey = prefix + "/" + finalKey
		} else {
			finalKey = prefix
		}
	}
	copyBody := append([]byte(nil), body...)
	upload := fakeUpload{Key: finalKey, ContentType: contentType, Body: copyBody}
	url := ""
	if f.baseURL != "" {
		base := strings.TrimRight(f.baseURL, "/")
		if finalKey != "" {
			url = base + "/" + finalKey
		} else {
			url = base
		}
	}
	upload.URL = url
	f.uploads = append(f.uploads, upload)
	return objectReference{Key: finalKey, URL: url}, nil
}

func (f *fakeObjectStorage) Delete(ctx context.Context, key string) error {
	f.deletes = append(f.deletes, key)
	return nil
}

func firstRecordingID(store *Storage) string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	for id := range store.data.Recordings {
		return id
	}
	return ""
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

func TestAuthenticateOAuthCreatesUser(t *testing.T) {
	store := newTestStore(t)

	user, err := store.AuthenticateOAuth(OAuthLoginParams{
		Provider:    "example",
		Subject:     "subject-1",
		Email:       "viewer@example.com",
		DisplayName: "Viewer",
	})
	if err != nil {
		t.Fatalf("AuthenticateOAuth returned error: %v", err)
	}
	if user.ID == "" {
		t.Fatal("expected user id to be assigned")
	}
	if user.Email != "viewer@example.com" {
		t.Fatalf("expected normalized email, got %s", user.Email)
	}
	if !user.SelfSignup {
		t.Fatal("expected OAuth-created user to be marked as self signup")
	}
	if len(user.Roles) != 1 || user.Roles[0] != "viewer" {
		t.Fatalf("expected viewer role for OAuth user, got %v", user.Roles)
	}

	fetched, ok := store.FindUserByEmail("viewer@example.com")
	if !ok || fetched.ID != user.ID {
		t.Fatalf("expected user to be persisted, got %+v", fetched)
	}

	again, err := store.AuthenticateOAuth(OAuthLoginParams{Provider: "example", Subject: "subject-1"})
	if err != nil {
		t.Fatalf("AuthenticateOAuth second call returned error: %v", err)
	}
	if again.ID != user.ID {
		t.Fatalf("expected existing account to be reused, got %s", again.ID)
	}
}

func TestAuthenticateOAuthLinksExistingUser(t *testing.T) {
	store := newTestStore(t)

	existing, err := store.CreateUser(CreateUserParams{DisplayName: "Existing", Email: "linked@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	linked, err := store.AuthenticateOAuth(OAuthLoginParams{Provider: "example", Subject: "subject-2", Email: "linked@example.com", DisplayName: "Viewer"})
	if err != nil {
		t.Fatalf("AuthenticateOAuth returned error: %v", err)
	}
	if linked.ID != existing.ID {
		t.Fatalf("expected OAuth login to link to existing user, got %s", linked.ID)
	}
}

func TestAuthenticateOAuthGeneratesFallbackEmail(t *testing.T) {
	store := newTestStore(t)

	user, err := store.AuthenticateOAuth(OAuthLoginParams{Provider: "acme", Subject: "unique"})
	if err != nil {
		t.Fatalf("AuthenticateOAuth returned error: %v", err)
	}
	if !strings.HasSuffix(user.Email, "@acme.oauth") {
		t.Fatalf("expected fallback email with domain, got %s", user.Email)
	}
	if user.DisplayName == "" {
		t.Fatal("expected fallback display name to be set")
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

func TestRecordingRetentionPurgesExpired(t *testing.T) {
	RunRepositoryRecordingRetention(t, jsonRepositoryFactory)
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
	RunRepositoryChatReportsLifecycle(t, jsonRepositoryFactory)
}

func TestCreateTipAndList(t *testing.T) {
	RunRepositoryTipsLifecycle(t, jsonRepositoryFactory)
}

func TestCreateSubscriptionAndCancel(t *testing.T) {
	RunRepositorySubscriptionsLifecycle(t, jsonRepositoryFactory)
}

func TestRepositoryStreamKeyRotation(t *testing.T) {
	RunRepositoryStreamKeyRotation(t, jsonRepositoryFactory)
}

func TestRepositoryOAuthLinking(t *testing.T) {
	RunRepositoryOAuthLinking(t, jsonRepositoryFactory)
}

func TestCloneDatasetCopiesModerationMetadata(t *testing.T) {
	now := time.Now().UTC()
	resolvedAt := now
	cancelledAt := now

	src := dataset{
		ChatBanActors: map[string]map[string]string{
			"chan": {"user": "actor"},
		},
		ChatBanReasons: map[string]map[string]string{
			"chan": {"user": "spam"},
		},
		ChatTimeoutActors: map[string]map[string]string{
			"chan": {"user": "actor"},
		},
		ChatTimeoutReasons: map[string]map[string]string{
			"chan": {"user": "caps"},
		},
		ChatTimeoutIssuedAt: map[string]map[string]time.Time{
			"chan": {"user": now},
		},
		ChatReports: map[string]models.ChatReport{
			"rep": {ID: "rep", ResolvedAt: &resolvedAt},
		},
		Tips: map[string]models.Tip{
			"tip": {ID: "tip", ChannelID: "chan", FromUserID: "supporter", CreatedAt: now},
		},
		Subscriptions: map[string]models.Subscription{
			"sub": {ID: "sub", ChannelID: "chan", UserID: "viewer", StartedAt: now, ExpiresAt: now.Add(time.Hour), Status: "active", CancelledAt: &cancelledAt},
		},
	}

	clone := cloneDataset(src)

	clone.ChatBanActors["chan"]["user"] = "other"
	if src.ChatBanActors["chan"]["user"] != "actor" {
		t.Fatalf("expected ban actors to be deep copied")
	}

	clone.ChatTimeoutReasons["chan"]["user"] = "other"
	if src.ChatTimeoutReasons["chan"]["user"] != "caps" {
		t.Fatalf("expected timeout reasons to be deep copied")
	}

	clone.ChatTimeoutIssuedAt["chan"]["user"] = now.Add(time.Minute)
	if !src.ChatTimeoutIssuedAt["chan"]["user"].Equal(now) {
		t.Fatalf("expected timeout issued timestamps to be deep copied")
	}

	newResolved := clone.ChatReports["rep"]
	*newResolved.ResolvedAt = newResolved.ResolvedAt.Add(time.Hour)
	if !src.ChatReports["rep"].ResolvedAt.Equal(resolvedAt) {
		t.Fatalf("expected report resolvedAt pointer to be cloned")
	}

	newSub := clone.Subscriptions["sub"]
	*newSub.CancelledAt = newSub.CancelledAt.Add(time.Hour)
	if !src.Subscriptions["sub"].CancelledAt.Equal(cancelledAt) {
		t.Fatalf("expected subscription cancelledAt pointer to be cloned")
	}
}

func TestMain(m *testing.M) {
	// ensure tests do not leave temp files behind by relying on testing package cleanup
	code := m.Run()
	os.Exit(code)
}
