package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/storage"
)

// This matrix exercises handler authorization over real private test storage.
// Session/middleware and production media qualification are separate gates.
func TestChannelAuthorizationMatrix(t *testing.T) {
	principals := []struct {
		name  string
		roles []string
		owner bool
		admin bool
	}{
		{name: "guest"},
		{name: "viewer", roles: []string{"viewer"}},
		{name: "unrelated_creator", roles: []string{"creator"}},
		{name: "moderator", roles: []string{"moderator"}},
		{name: "owner", roles: []string{"creator"}, owner: true},
		{name: "admin", roles: []string{"admin"}, admin: true},
	}
	actions := []struct {
		name    string
		method  string
		suffix  string
		body    string
		status  int
		public  bool
		listing bool
	}{
		{name: "detail", method: http.MethodGet, status: http.StatusOK, public: true},
		{name: "playback", method: http.MethodGet, suffix: "/playback", status: http.StatusOK, public: true},
		{name: "owner_list", method: http.MethodGet, status: http.StatusOK, listing: true},
		{name: "update", method: http.MethodPatch, body: `{"title":"Authorized title"}`, status: http.StatusOK},
		{name: "delete", method: http.MethodDelete, status: http.StatusNoContent},
		{name: "start", method: http.MethodPost, suffix: "/stream/start", body: `{"renditions":["1080p"]}`, status: http.StatusCreated},
		{name: "stop", method: http.MethodPost, suffix: "/stream/stop", body: `{"peakConcurrent":2}`, status: http.StatusOK},
		{name: "rotate", method: http.MethodPost, suffix: "/stream/rotate", status: http.StatusOK},
	}

	for _, principal := range principals {
		for _, action := range actions {
			t.Run(principal.name+"/"+action.name, func(t *testing.T) {
				handler, store := newTestHandler(t)
				createUser := func(name string, roles []string) domain.User {
					t.Helper()
					user, err := store.CreateUser(storage.CreateUserParams{
						DisplayName: name, Email: name + "@example.com", Roles: roles,
					})
					if err != nil {
						t.Fatalf("create fixture user: %v", err)
					}
					return user
				}
				owner := createUser("owner", []string{"creator"})
				other := createUser("other", []string{"creator"})
				channel, err := store.CreateChannel(owner.ID, "Protected channel", "gaming", nil)
				if err != nil {
					t.Fatalf("create protected channel: %v", err)
				}
				otherChannel, err := store.CreateChannel(other.ID, "Other channel", "gaming", nil)
				if err != nil {
					t.Fatalf("create unrelated channel: %v", err)
				}
				if action.name == "stop" {
					if _, err := store.StartStream(channel.ID, []string{"1080p"}); err != nil {
						t.Fatalf("start fixture session: %v", err)
					}
				}
				before, _ := store.GetChannel(channel.ID)
				sessionBefore, liveBefore := store.CurrentStreamSession(channel.ID)
				path := "/api/channels/" + channel.ID + action.suffix
				if action.listing {
					path = "/api/channels?ownerId=" + owner.ID
				}
				req := httptest.NewRequest(action.method, path, strings.NewReader(action.body))
				if principal.name != "guest" {
					actor := owner
					if principal.name == "unrelated_creator" {
						actor = other
					} else if !principal.owner {
						actor = createUser("actor", principal.roles)
					}
					req = withUser(req, actor)
				}
				rec := httptest.NewRecorder()
				if action.listing {
					handler.Channels(rec, req)
				} else {
					handler.ChannelByID(rec, req)
				}

				allowed := action.public || principal.owner || principal.admin
				wantStatus := action.status
				if !allowed {
					wantStatus = http.StatusForbidden
					if principal.name == "guest" {
						wantStatus = http.StatusUnauthorized
					}
				}
				if rec.Code != wantStatus {
					t.Fatalf("status=%d, want %d", rec.Code, wantStatus)
				}
				// Even owners/admins receive only public metadata on playback.
				privateResponse := allowed && (principal.owner || principal.admin) &&
					(action.name == "detail" || action.listing || action.name == "update" || action.name == "rotate")
				if strings.Contains(rec.Body.String(), `"streamKey"`) != privateResponse {
					t.Fatal("unexpected private stream-key field exposure")
				}
				if privateResponse && action.name != "rotate" && !strings.Contains(rec.Body.String(), before.StreamKey) {
					t.Fatal("authorized response omitted the channel's actual private key")
				}
				if !privateResponse && strings.Contains(rec.Body.String(), before.StreamKey) {
					t.Fatal("private stream-key value leaked")
				}
				if strings.Contains(rec.Body.String(), otherChannel.StreamKey) {
					t.Fatal("unrelated channel stream key leaked")
				}

				after, exists := store.GetChannel(channel.ID)
				sessionAfter, liveAfter := store.CurrentStreamSession(channel.ID)
				if !allowed || action.public || action.listing {
					if !exists || !reflect.DeepEqual(before, after) || liveBefore != liveAfter || !reflect.DeepEqual(sessionBefore, sessionAfter) {
						t.Fatal("denied or read-only action changed channel/session state")
					}
				} else {
					switch action.name {
					case "delete":
						if exists {
							t.Fatal("authorized delete left channel present")
						}
					case "update":
						if !exists || after.Title != "Authorized title" {
							t.Fatal("authorized update did not persist title")
						}
					case "rotate":
						var response channelResponse
						if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
							t.Fatalf("decode rotated channel: %v", err)
						}
						if !exists || after.StreamKey == before.StreamKey || response.StreamKey != after.StreamKey {
							t.Fatal("authorized rotation did not return the persisted new key")
						}
					case "start":
						if !exists || !liveAfter || sessionAfter.ChannelID != channel.ID {
							t.Fatal("authorized start did not establish channel session")
						}
					case "stop":
						if !exists || liveAfter || after.LiveState != "offline" {
							t.Fatal("authorized stop did not end channel session")
						}
					}
				}
				if otherAfter, ok := store.GetChannel(otherChannel.ID); !ok || !reflect.DeepEqual(otherChannel, otherAfter) {
					t.Fatal("action changed unrelated channel")
				}
			})
		}
	}
}
