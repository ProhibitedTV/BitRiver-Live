package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

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
	return NewHandler(store), store
}

func TestUsersEndpointCreatesAndListsUsers(t *testing.T) {
	handler, _ := newTestHandler(t)

	payload := map[string]interface{}{
		"displayName": "Alice",
		"email":       "alice@example.com",
		"roles":       []string{"creator"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Users(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec = httptest.NewRecorder()
	handler.Users(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response []userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response) != 1 {
		t.Fatalf("expected 1 user, got %d", len(response))
	}
	if response[0].Email != "alice@example.com" {
		t.Fatalf("expected email alice@example.com, got %s", response[0].Email)
	}
}

func TestChannelStreamLifecycle(t *testing.T) {
	handler, store := newTestHandler(t)
	user, err := store.CreateUser("Alice", "alice@example.com", nil)
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

func TestChatEndpointsLimit(t *testing.T) {
	handler, store := newTestHandler(t)
	user, err := store.CreateUser("Alice", "alice@example.com", nil)
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
		rec := httptest.NewRecorder()
		handler.ChannelByID(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected chat status 201, got %d", rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/channels/"+channel.ID+"/chat?limit=2", nil)
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
	owner, err := store.CreateUser("Streamer", "streamer@example.com", []string{"creator"})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	friend, err := store.CreateUser("Friend", "friend@example.com", nil)
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
	rec = httptest.NewRecorder()
	handler.ProfileByID(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected missing profile status 404, got %d", rec.Code)
	}
}
