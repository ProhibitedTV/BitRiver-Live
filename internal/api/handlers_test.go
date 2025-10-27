package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"bitriver-live/internal/auth"
	"bitriver-live/internal/models"
	"bitriver-live/internal/storage"
)

func newTestHandler(t *testing.T) (*Handler, *storage.Storage) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	store, err := storage.NewStorage(path)
	if err != nil {
		t.Fatalf("NewStorage error: %v", err)
	}
	sessions := auth.NewSessionManager(24 * time.Hour)
	return NewHandler(store, sessions), store
}

func withUser(req *http.Request, user models.User) *http.Request {
	return req.WithContext(ContextWithUser(req.Context(), user))
}

func TestUsersEndpointCreatesAndListsUsers(t *testing.T) {
	handler, store := newTestHandler(t)

	admin, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Admin",
		Email:       "admin@example.com",
		Roles:       []string{"admin"},
	})
	if err != nil {
		t.Fatalf("CreateUser admin: %v", err)
	}

	payload := map[string]interface{}{
		"displayName": "Alice",
		"email":       "alice@example.com",
		"roles":       []string{"creator"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req = withUser(req, admin)
	rec := httptest.NewRecorder()

	handler.Users(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req = withUser(req, admin)
	rec = httptest.NewRecorder()
	handler.Users(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response []userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response) < 2 {
		t.Fatalf("expected at least 2 users, got %d", len(response))
	}
	found := false
	for _, u := range response {
		if u.Email == "alice@example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find alice@example.com in response")
	}
}

func TestAuthorizationEnforced(t *testing.T) {
	handler, store := newTestHandler(t)

	payload := map[string]interface{}{
		"displayName": "Bob",
		"email":       "bob@example.com",
		"roles":       []string{"creator"},
	}
	body, _ := json.Marshal(payload)

	// Anonymous request should be rejected with 401.
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.Users(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for anonymous request, got %d", rec.Code)
	}

	viewer, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Viewer",
		Email:       "viewer@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser viewer: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req = withUser(req, viewer)
	rec = httptest.NewRecorder()
	handler.Users(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for viewer, got %d", rec.Code)
	}

	admin, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Admin",
		Email:       "admin@example.com",
		Roles:       []string{"admin"},
	})
	if err != nil {
		t.Fatalf("CreateUser admin: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req = withUser(req, admin)
	rec = httptest.NewRecorder()
	handler.Users(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201 for admin, got %d", rec.Code)
	}
}

func TestSignupAndLoginFlow(t *testing.T) {
	handler, _ := newTestHandler(t)

	signupPayload := map[string]string{
		"displayName": "Viewer",
		"email":       "viewer@example.com",
		"password":    "supersecret",
	}
	body, _ := json.Marshal(signupPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.Signup(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected signup status 201, got %d", rec.Code)
	}

	var signed authResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &signed); err != nil {
		t.Fatalf("decode signup response: %v", err)
	}
	if signed.ExpiresAt == "" {
		t.Fatal("expected signup response to include expiry")
	}
	if !signed.User.SelfSignup {
		t.Fatal("expected user to be marked as self-signup")
	}

	signupCookie := findCookie(t, rec.Result().Cookies(), "bitriver_session")
	if signupCookie.Value == "" {
		t.Fatal("expected signup to set session cookie")
	}
	if !signupCookie.HttpOnly {
		t.Fatal("expected session cookie to be HttpOnly")
	}
	if !signupCookie.Secure {
		t.Fatal("expected session cookie to be Secure")
	}
	if signupCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("expected SameSite=Strict, got %v", signupCookie.SameSite)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	req.AddCookie(signupCookie)
	rec = httptest.NewRecorder()
	handler.Session(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected session status 200, got %d", rec.Code)
	}

	loginPayload := map[string]string{
		"email":    "viewer@example.com",
		"password": "supersecret",
	}
	body, _ = json.Marshal(loginPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.Login(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected login status 200, got %d", rec.Code)
	}

	var loggedIn authResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &loggedIn); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loggedIn.ExpiresAt == "" {
		t.Fatal("expected login response to include expiry")
	}

	loginCookie := findCookie(t, rec.Result().Cookies(), "bitriver_session")
	if loginCookie.Value == "" {
		t.Fatal("expected login to refresh session cookie")
	}
	if loginCookie.Value == signupCookie.Value {
		t.Fatal("expected login to issue a new session token")
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/auth/session", nil)
	req.AddCookie(loginCookie)
	rec = httptest.NewRecorder()
	handler.Session(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected delete session status 204, got %d", rec.Code)
	}

	clearedCookie := findCookie(t, rec.Result().Cookies(), "bitriver_session")
	if clearedCookie.Value != "" {
		t.Fatal("expected logout cookie to be cleared")
	}
	if clearedCookie.MaxAge != -1 {
		t.Fatalf("expected cleared cookie to have MaxAge=-1, got %d", clearedCookie.MaxAge)
	}

	if _, _, ok := handler.sessionManager().Validate(loginCookie.Value); ok {
		t.Fatal("expected logout to revoke session token")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	req.AddCookie(loginCookie)
	rec = httptest.NewRecorder()
	handler.Session(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected session to be revoked, got status %d", rec.Code)
	}
}

func TestHealthReportsIngestStatus(t *testing.T) {
	handler, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.Health(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health payload: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("expected overall status ok, got %v", payload["status"])
	}
	services, ok := payload["services"].([]interface{})
	if !ok {
		t.Fatalf("expected services array in response")
	}
	if len(services) == 0 {
		t.Fatalf("expected at least one health service entry")
	}
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %q not found", name)
	return nil
}

func TestChannelStreamLifecycle(t *testing.T) {
	handler, store := newTestHandler(t)
	user, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Alice",
		Email:       "alice@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Create channel via HTTP
	payload := map[string]interface{}{
		"ownerId":  user.ID,
		"title":    "My Channel",
		"category": "gaming",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/channels", bytes.NewReader(body))
	req = withUser(req, user)
	rec := httptest.NewRecorder()
	handler.Channels(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected channel create status 201, got %d", rec.Code)
	}
	var channel channelResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &channel); err != nil {
		t.Fatalf("decode channel response: %v", err)
	}
	if channel.LiveState != "offline" {
		t.Fatalf("expected offline channel, got %s", channel.LiveState)
	}

	// Start stream
	startPayload := map[string]interface{}{"renditions": []string{"1080p"}}
	body, _ = json.Marshal(startPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/channels/"+channel.ID+"/stream/start", bytes.NewReader(body))
	req = withUser(req, user)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected start status 201, got %d", rec.Code)
	}
	var session sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if session.ChannelID != channel.ID {
		t.Fatalf("expected session channel %s, got %s", channel.ID, session.ChannelID)
	}

	// Stop stream
	stopPayload := map[string]interface{}{"peakConcurrent": 10}
	body, _ = json.Marshal(stopPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/channels/"+channel.ID+"/stream/stop", bytes.NewReader(body))
	req = withUser(req, user)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected stop status 200, got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode stop session: %v", err)
	}
	if session.PeakConcurrent != 10 {
		t.Fatalf("expected peak concurrent 10, got %d", session.PeakConcurrent)
	}
}

func TestChannelsListPermissions(t *testing.T) {
	handler, store := newTestHandler(t)

	creator, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Creator",
		Email:       "creator@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser creator: %v", err)
	}

	admin, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Admin",
		Email:       "admin@example.com",
		Roles:       []string{"admin"},
	})
	if err != nil {
		t.Fatalf("CreateUser admin: %v", err)
	}

	viewer, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Viewer",
		Email:       "viewer@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser viewer: %v", err)
	}

	channel, err := store.CreateChannel(creator.ID, "Creator Channel", "gaming", []string{"retro"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/channels", nil)
	req = withUser(req, creator)
	rec := httptest.NewRecorder()
	handler.Channels(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected creator list status 200, got %d", rec.Code)
	}
	var creatorResponse []channelResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &creatorResponse); err != nil {
		t.Fatalf("decode creator response: %v", err)
	}
	if len(creatorResponse) != 1 {
		t.Fatalf("expected one channel for creator, got %d", len(creatorResponse))
	}
	if creatorResponse[0].StreamKey == "" {
		t.Fatal("expected stream key for creator-owned channel")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/channels?ownerId="+creator.ID, nil)
	req = withUser(req, viewer)
	rec = httptest.NewRecorder()
	handler.Channels(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected viewer status 403, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/channels?ownerId="+creator.ID, nil)
	req = withUser(req, admin)
	rec = httptest.NewRecorder()
	handler.Channels(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected admin list status 200, got %d", rec.Code)
	}
	var adminResponse []channelResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &adminResponse); err != nil {
		t.Fatalf("decode admin response: %v", err)
	}
	if len(adminResponse) != 1 {
		t.Fatalf("expected one channel for admin, got %d", len(adminResponse))
	}
	if adminResponse[0].StreamKey != channel.StreamKey {
		t.Fatalf("expected admin to receive stream key %s, got %s", channel.StreamKey, adminResponse[0].StreamKey)
	}
}

func TestChatEndpointsLimit(t *testing.T) {
	handler, store := newTestHandler(t)
	user, err := store.CreateUser(storage.CreateUserParams{
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

	for i := 0; i < 3; i++ {
		payload := map[string]interface{}{
			"userId":  user.ID,
			"content": "message",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/channels/"+channel.ID+"/chat", bytes.NewReader(body))
		req = withUser(req, user)
		rec := httptest.NewRecorder()
		handler.ChannelByID(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected chat status 201, got %d", rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/channels/"+channel.ID+"/chat?limit=2", nil)
	req = withUser(req, user)
	rec := httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected chat list status 200, got %d", rec.Code)
	}
	var messages []chatMessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &messages); err != nil {
		t.Fatalf("decode chat response: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
}

func TestProfileEndpoints(t *testing.T) {
	handler, store := newTestHandler(t)
	owner, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Streamer",
		Email:       "streamer@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	friend, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Friend",
		Email:       "friend@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser friend: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Main Stage", "music", []string{"live"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if _, err := store.StartStream(channel.ID, []string{"1080p"}); err != nil {
		t.Fatalf("StartStream: %v", err)
	}

	payload := map[string]interface{}{
		"bio":               "Welcome to the cascade",
		"avatarUrl":         "https://cdn.example.com/avatar.png",
		"bannerUrl":         "https://cdn.example.com/banner.png",
		"featuredChannelId": channel.ID,
		"topFriends":        []string{friend.ID},
		"donationAddresses": []map[string]string{{"currency": "eth", "address": "0xabc", "note": "Main"}},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/api/profiles/"+owner.ID, bytes.NewReader(body))
	req = withUser(req, owner)
	rec := httptest.NewRecorder()
	handler.ProfileByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected profile upsert status 200, got %d", rec.Code)
	}

	var response profileViewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	if response.UserID != owner.ID {
		t.Fatalf("expected user id %s, got %s", owner.ID, response.UserID)
	}
	if response.DisplayName != owner.DisplayName {
		t.Fatalf("expected display name %s, got %s", owner.DisplayName, response.DisplayName)
	}
	if response.FeaturedChannelID == nil || *response.FeaturedChannelID != channel.ID {
		t.Fatalf("expected featured channel %s", channel.ID)
	}
	if len(response.TopFriends) != 1 || response.TopFriends[0].UserID != friend.ID {
		t.Fatalf("expected top friend %s", friend.ID)
	}
	if len(response.DonationAddresses) != 1 || response.DonationAddresses[0].Currency != "ETH" {
		t.Fatalf("expected donation currency ETH")
	}
	if len(response.LiveChannels) != 1 || response.LiveChannels[0].ID != channel.ID {
		t.Fatalf("expected live channel %s", channel.ID)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/profiles/"+owner.ID, nil)
	req = withUser(req, owner)
	rec = httptest.NewRecorder()
	handler.ProfileByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected profile get status 200, got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode profile get response: %v", err)
	}
	if len(response.Channels) != 1 {
		t.Fatalf("expected single channel on profile, got %d", len(response.Channels))
	}

	req = httptest.NewRequest(http.MethodGet, "/api/profiles/missing", nil)
	req = withUser(req, owner)
	rec = httptest.NewRecorder()
	handler.ProfileByID(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected missing profile status 404, got %d", rec.Code)
	}
}
