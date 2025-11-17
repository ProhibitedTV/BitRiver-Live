package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"bitriver-live/internal/auth"
	"bitriver-live/internal/auth/oauth"
	"bitriver-live/internal/chat"
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

type ingestUnavailableRepo struct {
	storage.Repository
}

type pingFunc func(context.Context) error

func (f pingFunc) Ping(ctx context.Context) error {
	return f(ctx)
}

type failingRepository struct {
	storage.Repository
	err error
}

func (r failingRepository) Ping(context.Context) error {
	return r.err
}

func (r ingestUnavailableRepo) StartStream(channelID string, renditions []string) (models.StreamSession, error) {
	return models.StreamSession{}, storage.ErrIngestControllerUnavailable
}

func (r ingestUnavailableRepo) StopStream(channelID string, peakConcurrent int) (models.StreamSession, error) {
	return models.StreamSession{}, storage.ErrIngestControllerUnavailable
}

func withUser(req *http.Request, user models.User) *http.Request {
	return req.WithContext(ContextWithUser(req.Context(), user))
}

type oauthStub struct {
	providers      []oauth.ProviderInfo
	beginResult    oauth.BeginResult
	beginError     error
	completeResult oauth.Completion
	completeError  error
	cancelResult   string
	cancelError    error
	lastBegin      struct {
		provider string
		returnTo string
	}
	lastComplete struct {
		provider string
		state    string
		code     string
	}
	lastCancel string
}

type profileRepositoryWithOrphan struct {
	storage.Repository
	orphan models.Profile
}

func (r profileRepositoryWithOrphan) ListProfiles() []models.Profile {
	profiles := r.Repository.ListProfiles()
	return append(profiles, r.orphan)
}

func (s *oauthStub) Providers() []oauth.ProviderInfo {
	if s.providers != nil {
		return s.providers
	}
	return []oauth.ProviderInfo{}
}

func (s *oauthStub) Begin(provider, returnTo string) (oauth.BeginResult, error) {
	s.lastBegin.provider = provider
	s.lastBegin.returnTo = returnTo
	if s.beginError != nil {
		return oauth.BeginResult{}, s.beginError
	}
	return s.beginResult, nil
}

func (s *oauthStub) Complete(provider, state, code string) (oauth.Completion, error) {
	s.lastComplete.provider = provider
	s.lastComplete.state = state
	s.lastComplete.code = code
	if s.completeError != nil {
		return oauth.Completion{}, s.completeError
	}
	return s.completeResult, nil
}

func (s *oauthStub) Cancel(state string) (string, error) {
	s.lastCancel = state
	if s.cancelError != nil {
		return "", s.cancelError
	}
	return s.cancelResult, nil
}

func TestProfilesList(t *testing.T) {
	handler, store := newTestHandler(t)

	viewer, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Viewer", Email: "viewer@example.com"})
	if err != nil {
		t.Fatalf("CreateUser viewer: %v", err)
	}

	creatorOne, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Creator One", Email: "creator1@example.com"})
	if err != nil {
		t.Fatalf("CreateUser creatorOne: %v", err)
	}
	creatorTwo, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Creator Two", Email: "creator2@example.com"})
	if err != nil {
		t.Fatalf("CreateUser creatorTwo: %v", err)
	}

	channel, err := store.CreateChannel(creatorOne.ID, "Channel One", "Gaming", []string{"play"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	featured := channel.ID
	topFriends := []string{creatorTwo.ID}
	bioOne := "Streaming adventures"
	avatarOne := "https://example.com/avatar.png"
	bannerOne := "https://example.com/banner.png"
	if _, err := store.UpsertProfile(creatorOne.ID, storage.ProfileUpdate{
		Bio:               &bioOne,
		AvatarURL:         &avatarOne,
		BannerURL:         &bannerOne,
		FeaturedChannelID: &featured,
		TopFriends:        &topFriends,
	}); err != nil {
		t.Fatalf("UpsertProfile creatorOne: %v", err)
	}

	bioTwo := "Chill streams"
	if _, err := store.UpsertProfile(creatorTwo.ID, storage.ProfileUpdate{Bio: &bioTwo}); err != nil {
		t.Fatalf("UpsertProfile creatorTwo: %v", err)
	}

	orphan := models.Profile{
		UserID:    "missing-user",
		Bio:       "ghost profile",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	handler.Store = profileRepositoryWithOrphan{Repository: handler.Store, orphan: orphan}

	assertProfilesResponse := func(body []byte) map[string]profileViewResponse {
		var payload []profileViewResponse
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(payload) != 2 {
			t.Fatalf("expected 2 profiles because orphaned entries should be omitted, got %d", len(payload))
		}

		profilesByUser := make(map[string]profileViewResponse)
		for _, p := range payload {
			profilesByUser[p.UserID] = p
		}

		profileOne, ok := profilesByUser[creatorOne.ID]
		if !ok {
			t.Fatalf("missing profile for %s", creatorOne.ID)
		}
		if profileOne.DisplayName != creatorOne.DisplayName {
			t.Fatalf("expected display name %q, got %q", creatorOne.DisplayName, profileOne.DisplayName)
		}
		if profileOne.FeaturedChannelID == nil || *profileOne.FeaturedChannelID != channel.ID {
			t.Fatalf("expected featured channel %s, got %v", channel.ID, profileOne.FeaturedChannelID)
		}
		if len(profileOne.TopFriends) != 1 || profileOne.TopFriends[0].UserID != creatorTwo.ID {
			t.Fatalf("expected top friend %s, got %+v", creatorTwo.ID, profileOne.TopFriends)
		}
		if len(profileOne.Channels) != 1 || profileOne.Channels[0].ID != channel.ID {
			t.Fatalf("expected channel %s, got %+v", channel.ID, profileOne.Channels)
		}

		profileTwo, ok := profilesByUser[creatorTwo.ID]
		if !ok {
			t.Fatalf("missing profile for %s", creatorTwo.ID)
		}
		if profileTwo.FeaturedChannelID != nil {
			t.Fatalf("expected no featured channel for %s", creatorTwo.ID)
		}

		return profilesByUser
	}

	req := httptest.NewRequest(http.MethodGet, "/api/profiles", nil)
	req = withUser(req, viewer)
	rec := httptest.NewRecorder()
	handler.Profiles(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	profilesAuthenticated := assertProfilesResponse(rec.Body.Bytes())

	unauthReq := httptest.NewRequest(http.MethodGet, "/api/profiles", nil)
	unauthRec := httptest.NewRecorder()
	handler.Profiles(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for unauthenticated request, got %d", unauthRec.Code)
	}

	profilesPublic := assertProfilesResponse(unauthRec.Body.Bytes())
	if !reflect.DeepEqual(profilesAuthenticated, profilesPublic) {
		t.Fatalf("expected public and authenticated profile responses to match")
	}
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

func TestUserByID(t *testing.T) {
	updatedName := "Updated Creator"
	updatedRoles := []string{"creator", "moderator"}

	cases := []struct {
		name       string
		method     string
		setup      func(t *testing.T, store *storage.Storage) (requester models.User, target models.User, body []byte)
		wantStatus int
		assert     func(t *testing.T, rec *httptest.ResponseRecorder, store *storage.Storage, target models.User)
	}{
		{
			name:   "owner gets own record",
			method: http.MethodGet,
			setup: func(t *testing.T, store *storage.Storage) (models.User, models.User, []byte) {
				owner, err := store.CreateUser(storage.CreateUserParams{
					DisplayName: "Owner",
					Email:       "owner@example.com",
					Roles:       []string{"creator"},
				})
				if err != nil {
					t.Fatalf("CreateUser owner: %v", err)
				}
				return owner, owner, nil
			},
			wantStatus: http.StatusOK,
			assert: func(t *testing.T, rec *httptest.ResponseRecorder, store *storage.Storage, target models.User) {
				var resp userResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if resp.ID != target.ID {
					t.Fatalf("expected id %s, got %s", target.ID, resp.ID)
				}
				if resp.DisplayName != target.DisplayName {
					t.Fatalf("expected display name %q, got %q", target.DisplayName, resp.DisplayName)
				}
				if resp.Email != target.Email {
					t.Fatalf("expected email %q, got %q", target.Email, resp.Email)
				}
				if !reflect.DeepEqual(resp.Roles, target.Roles) {
					t.Fatalf("expected roles %v, got %v", target.Roles, resp.Roles)
				}

				persisted, ok := store.GetUser(target.ID)
				if !ok {
					t.Fatalf("expected user %s to exist", target.ID)
				}
				if persisted.Email != target.Email {
					t.Fatalf("expected persisted email %q, got %q", target.Email, persisted.Email)
				}
				if !reflect.DeepEqual(persisted.Roles, target.Roles) {
					t.Fatalf("expected persisted roles %v, got %v", target.Roles, persisted.Roles)
				}
			},
		},
		{
			name:   "non-admin forbidden from viewing others",
			method: http.MethodGet,
			setup: func(t *testing.T, store *storage.Storage) (models.User, models.User, []byte) {
				viewer, err := store.CreateUser(storage.CreateUserParams{
					DisplayName: "Viewer",
					Email:       "viewer@example.com",
				})
				if err != nil {
					t.Fatalf("CreateUser viewer: %v", err)
				}
				creator, err := store.CreateUser(storage.CreateUserParams{
					DisplayName: "Creator",
					Email:       "creator@example.com",
					Roles:       []string{"creator"},
				})
				if err != nil {
					t.Fatalf("CreateUser creator: %v", err)
				}
				return viewer, creator, nil
			},
			wantStatus: http.StatusForbidden,
			assert: func(t *testing.T, rec *httptest.ResponseRecorder, store *storage.Storage, target models.User) {
				var resp map[string]string
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if resp["error"] == "" {
					t.Fatal("expected error message in response")
				}
				if _, ok := store.GetUser(target.ID); !ok {
					t.Fatalf("expected user %s to remain in store", target.ID)
				}
			},
		},
		{
			name:   "admin patches another user",
			method: http.MethodPatch,
			setup: func(t *testing.T, store *storage.Storage) (models.User, models.User, []byte) {
				admin, err := store.CreateUser(storage.CreateUserParams{
					DisplayName: "Admin",
					Email:       "admin@example.com",
					Roles:       []string{"admin"},
				})
				if err != nil {
					t.Fatalf("CreateUser admin: %v", err)
				}
				target, err := store.CreateUser(storage.CreateUserParams{
					DisplayName: "Original Creator",
					Email:       "creator2@example.com",
					Roles:       []string{"creator"},
				})
				if err != nil {
					t.Fatalf("CreateUser target: %v", err)
				}
				payload := map[string]interface{}{
					"displayName": updatedName,
					"roles":       updatedRoles,
				}
				body, err := json.Marshal(payload)
				if err != nil {
					t.Fatalf("marshal payload: %v", err)
				}
				return admin, target, body
			},
			wantStatus: http.StatusOK,
			assert: func(t *testing.T, rec *httptest.ResponseRecorder, store *storage.Storage, target models.User) {
				var resp userResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if resp.ID != target.ID {
					t.Fatalf("expected id %s, got %s", target.ID, resp.ID)
				}
				if resp.DisplayName != updatedName {
					t.Fatalf("expected display name %q, got %q", updatedName, resp.DisplayName)
				}
				if !reflect.DeepEqual(resp.Roles, updatedRoles) {
					t.Fatalf("expected roles %v, got %v", updatedRoles, resp.Roles)
				}
				persisted, ok := store.GetUser(target.ID)
				if !ok {
					t.Fatalf("expected user %s to exist", target.ID)
				}
				if persisted.DisplayName != updatedName {
					t.Fatalf("expected persisted display name %q, got %q", updatedName, persisted.DisplayName)
				}
				if !reflect.DeepEqual(persisted.Roles, updatedRoles) {
					t.Fatalf("expected persisted roles %v, got %v", updatedRoles, persisted.Roles)
				}
			},
		},
		{
			name:   "admin deletes user",
			method: http.MethodDelete,
			setup: func(t *testing.T, store *storage.Storage) (models.User, models.User, []byte) {
				admin, err := store.CreateUser(storage.CreateUserParams{
					DisplayName: "Admin",
					Email:       "delete-admin@example.com",
					Roles:       []string{"admin"},
				})
				if err != nil {
					t.Fatalf("CreateUser admin: %v", err)
				}
				target, err := store.CreateUser(storage.CreateUserParams{
					DisplayName: "Deletable",
					Email:       "delete-me@example.com",
				})
				if err != nil {
					t.Fatalf("CreateUser target: %v", err)
				}
				return admin, target, nil
			},
			wantStatus: http.StatusNoContent,
			assert: func(t *testing.T, rec *httptest.ResponseRecorder, store *storage.Storage, target models.User) {
				if body := strings.TrimSpace(rec.Body.String()); body != "" {
					t.Fatalf("expected empty body, got %q", body)
				}
				if _, ok := store.GetUser(target.ID); ok {
					t.Fatalf("expected user %s to be deleted", target.ID)
				}
				if err := store.DeleteUser(target.ID); err == nil {
					t.Fatalf("expected deleting removed user to error")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, store := newTestHandler(t)
			requester, target, body := tc.setup(t, store)

			var reader io.Reader
			if body != nil {
				reader = bytes.NewReader(body)
			}
			req := httptest.NewRequest(tc.method, "/api/users/"+target.ID, reader)
			if requester.ID != "" {
				req = withUser(req, requester)
			}

			rec := httptest.NewRecorder()
			handler.UserByID(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, rec.Code)
			}

			tc.assert(t, rec, store, target)
		})
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
	if signupCookie.Secure {
		t.Fatal("expected HTTP signup to issue non-secure cookie")
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
	if clearedCookie.Secure {
		t.Fatal("expected HTTP logout to issue non-secure cookie")
	}

	if _, _, ok, err := handler.sessionManager().Validate(loginCookie.Value); err != nil || ok {
		if err != nil {
			t.Fatalf("Validate returned error: %v", err)
		}
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

func TestSignupRejectsShortPassword(t *testing.T) {
	handler, store := newTestHandler(t)

	signupPayload := map[string]string{
		"displayName": "Viewer",
		"email":       "viewer@example.com",
		"password":    "shortpw",
	}
	body, _ := json.Marshal(signupPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Signup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected signup status 400, got %d", rec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "password must be at least 8 characters" {
		t.Fatalf("unexpected error message: %q", payload["error"])
	}

	if _, ok := store.FindUserByEmail("viewer@example.com"); ok {
		t.Fatal("unexpected user created for short password")
	}

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "bitriver_session" {
			t.Fatal("unexpected session cookie issued for short password")
		}
	}
}

func TestSignupDisabled(t *testing.T) {
	handler, _ := newTestHandler(t)
	handler.AllowSelfSignup = false

	signupPayload := map[string]string{
		"displayName": "Viewer",
		"email":       "viewer@example.com",
		"password":    "supersecret",
	}
	body, _ := json.Marshal(signupPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Signup(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 when self-signup disabled, got %d", rec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] == "" {
		t.Fatal("expected error message in response")
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "bitriver_session" {
			t.Fatal("unexpected session cookie issued when signup disabled")
		}
	}
}

func TestDirectoryFiltersChannelsByQuery(t *testing.T) {
	handler, store := newTestHandler(t)

	creatorOne, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Coder One", Email: "coder1@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create first creator: %v", err)
	}
	creatorTwo, err := store.CreateUser(storage.CreateUserParams{DisplayName: "RetroMaster", Email: "retro@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create second creator: %v", err)
	}
	creatorThree, err := store.CreateUser(storage.CreateUserParams{DisplayName: "DJ Night", Email: "dj@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create third creator: %v", err)
	}

	lounge, err := store.CreateChannel(creatorOne.ID, "Coding Lounge", "technology", []string{"GoLang", "Backend"})
	if err != nil {
		t.Fatalf("create coding lounge: %v", err)
	}
	arcade, err := store.CreateChannel(creatorTwo.ID, "Arcade Stars", "gaming", []string{"retro", "speedrun"})
	if err != nil {
		t.Fatalf("create arcade stars: %v", err)
	}
	beats, err := store.CreateChannel(creatorThree.ID, "Midnight Beats", "music", []string{"Live", "Music"})
	if err != nil {
		t.Fatalf("create midnight beats: %v", err)
	}

	cases := []struct {
		name    string
		query   string
		wantIDs []string
	}{
		{name: "no filter", query: "", wantIDs: []string{lounge.ID, arcade.ID, beats.ID}},
		{name: "title filter", query: "lounge", wantIDs: []string{lounge.ID}},
		{name: "owner filter", query: "RETROMASTER", wantIDs: []string{arcade.ID}},
		{name: "tag filter", query: "MuSiC", wantIDs: []string{beats.ID}},
		{name: "no matches", query: "unknown", wantIDs: []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := "/api/directory"
			if strings.TrimSpace(tc.query) != "" {
				path = fmt.Sprintf("/api/directory?q=%s", tc.query)
			}
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			handler.Directory(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", rec.Code)
			}
			var resp directoryResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(resp.Channels) != len(tc.wantIDs) {
				t.Fatalf("expected %d channels, got %d", len(tc.wantIDs), len(resp.Channels))
			}
			for i, id := range tc.wantIDs {
				if resp.Channels[i].Channel.ID != id {
					t.Fatalf("expected channel %s at index %d, got %s", id, i, resp.Channels[i].Channel.ID)
				}
			}
		})
	}
}

func TestDirectoryFollowingRequiresAuthentication(t *testing.T) {
	handler, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/directory/following", nil)
	rec := httptest.NewRecorder()

	handler.DirectoryFollowing(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for anonymous request, got %d", rec.Code)
	}
}

func TestDirectoryFollowingListsLiveFollowedChannels(t *testing.T) {
	handler, store := newTestHandler(t)

	viewer, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Viewer", Email: "viewer@example.com"})
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	creator, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Creator", Email: "creator@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create creator: %v", err)
	}

	liveChannel, err := store.CreateChannel(creator.ID, "Live Now", "gaming", []string{"speedrun"})
	if err != nil {
		t.Fatalf("create live channel: %v", err)
	}
	startingChannel, err := store.CreateChannel(creator.ID, "Starting Soon", "music", []string{"dj"})
	if err != nil {
		t.Fatalf("create starting channel: %v", err)
	}
	offlineChannel, err := store.CreateChannel(creator.ID, "Offline Show", "tech", []string{"coding"})
	if err != nil {
		t.Fatalf("create offline channel: %v", err)
	}

	liveState := "live"
	if _, err := store.UpdateChannel(liveChannel.ID, storage.ChannelUpdate{LiveState: &liveState}); err != nil {
		t.Fatalf("set live state: %v", err)
	}
	startingState := "starting"
	if _, err := store.UpdateChannel(startingChannel.ID, storage.ChannelUpdate{LiveState: &startingState}); err != nil {
		t.Fatalf("set starting state: %v", err)
	}

	if err := store.FollowChannel(viewer.ID, offlineChannel.ID); err != nil {
		t.Fatalf("follow offline channel: %v", err)
	}
	if err := store.FollowChannel(viewer.ID, liveChannel.ID); err != nil {
		t.Fatalf("follow live channel: %v", err)
	}
	if err := store.FollowChannel(viewer.ID, startingChannel.ID); err != nil {
		t.Fatalf("follow starting channel: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/directory/following", nil)
	req = withUser(req, viewer)
	rec := httptest.NewRecorder()

	handler.DirectoryFollowing(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp directoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	wantIDs := []string{startingChannel.ID, liveChannel.ID}
	if len(resp.Channels) != len(wantIDs) {
		t.Fatalf("expected %d channels, got %d", len(wantIDs), len(resp.Channels))
	}
	for i, id := range wantIDs {
		if resp.Channels[i].Channel.ID != id {
			t.Fatalf("expected channel %s at index %d, got %s", id, i, resp.Channels[i].Channel.ID)
		}
	}
}

func TestOAuthProvidersEndpoint(t *testing.T) {
	handler, _ := newTestHandler(t)
	stub := &oauthStub{providers: []oauth.ProviderInfo{{Name: "test", DisplayName: "Test"}}}
	handler.OAuth = stub

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/providers", nil)
	rec := httptest.NewRecorder()
	handler.OAuthProviders(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var payload struct {
		Providers []oauth.ProviderInfo `json:"providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Providers) != 1 || payload.Providers[0].Name != "test" {
		t.Fatalf("unexpected providers payload: %+v", payload.Providers)
	}
}

func TestOAuthStartEndpoint(t *testing.T) {
	handler, _ := newTestHandler(t)
	stub := &oauthStub{beginResult: oauth.BeginResult{URL: "https://auth.example.com", State: "state-123"}}
	handler.OAuth = stub

	body, _ := json.Marshal(oauthStartRequest{ReturnTo: "/control"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/test/start", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.OAuthByProvider(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if stub.lastBegin.provider != "test" {
		t.Fatalf("expected provider to be forwarded to stub, got %s", stub.lastBegin.provider)
	}
	if stub.lastBegin.returnTo != "/control" {
		t.Fatalf("expected return path /control, got %q", stub.lastBegin.returnTo)
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["url"] != "https://auth.example.com" {
		t.Fatalf("expected auth url in response, got %q", payload["url"])
	}
}

func TestOAuthCallbackCreatesSession(t *testing.T) {
	handler, store := newTestHandler(t)
	stub := &oauthStub{completeResult: oauth.Completion{
		ReturnTo: "/dashboard",
		Profile: oauth.UserProfile{
			Provider:    "test",
			Subject:     "sub-1",
			Email:       "viewer@example.com",
			DisplayName: "Viewer",
		},
	}}
	handler.OAuth = stub

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/test/callback?state=abc&code=xyz", nil)
	rec := httptest.NewRecorder()
	handler.OAuthByProvider(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect status, got %d", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/dashboard?oauth=success" {
		t.Fatalf("expected success redirect, got %q", location)
	}
	cookie := findCookie(t, rec.Result().Cookies(), "bitriver_session")
	if cookie.Value == "" {
		t.Fatal("expected session cookie to be issued")
	}
	user, ok := store.FindUserByEmail("viewer@example.com")
	if !ok {
		t.Fatalf("expected user to be created via oauth")
	}
	if user.DisplayName != "Viewer" {
		t.Fatalf("expected display name Viewer, got %q", user.DisplayName)
	}
}

func TestSignupIssuesSecureCookieForTLSRequests(t *testing.T) {
	handler, _ := newTestHandler(t)

	signupPayload := map[string]string{
		"displayName": "Viewer",
		"email":       "secure@example.com",
		"password":    "supersecret",
	}
	body, _ := json.Marshal(signupPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", bytes.NewReader(body))
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()

	handler.Signup(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected signup status 201, got %d", rec.Code)
	}

	cookie := findCookie(t, rec.Result().Cookies(), "bitriver_session")
	if !cookie.Secure {
		t.Fatal("expected TLS signup to issue secure cookie")
	}
}

func TestRecordingEndpointsEndToEnd(t *testing.T) {
	handler, store := newTestHandler(t)

	creator, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Creator",
		Email:       "creator@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser creator: %v", err)
	}
	channel, err := store.CreateChannel(creator.ID, "Playthrough", "gaming", []string{"rpg"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	session, err := store.StartStream(channel.ID, []string{"1080p"})
	if err != nil {
		t.Fatalf("StartStream: %v", err)
	}
	if _, err := store.StopStream(channel.ID, 20); err != nil {
		t.Fatalf("StopStream: %v", err)
	}

	// Anonymous listing should hide unpublished recordings.
	req := httptest.NewRequest(http.MethodGet, "/api/recordings?channelId="+channel.ID, nil)
	rec := httptest.NewRecorder()
	handler.Recordings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var anonymousList []recordingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &anonymousList); err != nil {
		t.Fatalf("decode anonymous list: %v", err)
	}
	if len(anonymousList) != 0 {
		t.Fatalf("expected unpublished recordings to be hidden, got %d", len(anonymousList))
	}

	// Owner should see the recording.
	req = httptest.NewRequest(http.MethodGet, "/api/recordings?channelId="+channel.ID, nil)
	req = withUser(req, creator)
	rec = httptest.NewRecorder()
	handler.Recordings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for owner, got %d", rec.Code)
	}
	var ownerList []recordingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &ownerList); err != nil {
		t.Fatalf("decode owner list: %v", err)
	}
	if len(ownerList) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(ownerList))
	}
	recordingID := ownerList[0].ID

	// Publish the recording.
	req = httptest.NewRequest(http.MethodPost, "/api/recordings/"+recordingID+"/publish", nil)
	req = withUser(req, creator)
	rec = httptest.NewRecorder()
	handler.RecordingByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected publish status 200, got %d", rec.Code)
	}
	var published recordingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &published); err != nil {
		t.Fatalf("decode publish response: %v", err)
	}
	if published.PublishedAt == nil {
		t.Fatalf("expected publish timestamp")
	}

	// Anonymous listing should now include the recording.
	req = httptest.NewRequest(http.MethodGet, "/api/recordings?channelId="+channel.ID, nil)
	rec = httptest.NewRecorder()
	handler.Recordings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 after publish, got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &anonymousList); err != nil {
		t.Fatalf("decode anonymous list after publish: %v", err)
	}
	if len(anonymousList) != 1 {
		t.Fatalf("expected 1 public recording, got %d", len(anonymousList))
	}

	// Create a clip export.
	clipPayload := clipExportRequest{Title: "Intro", StartSeconds: 0, EndSeconds: 5}
	body, _ := json.Marshal(clipPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/recordings/"+recordingID+"/clips", bytes.NewReader(body))
	req = withUser(req, creator)
	rec = httptest.NewRecorder()
	handler.RecordingByID(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected clip create status 201, got %d", rec.Code)
	}
	var clip clipExportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &clip); err != nil {
		t.Fatalf("decode clip response: %v", err)
	}
	if clip.RecordingID != recordingID {
		t.Fatalf("expected clip recording id %s, got %s", recordingID, clip.RecordingID)
	}

	// Fetch recording details anonymously (should include clip summary).
	req = httptest.NewRequest(http.MethodGet, "/api/recordings/"+recordingID, nil)
	rec = httptest.NewRecorder()
	handler.RecordingByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected get status 200, got %d", rec.Code)
	}
	var fetched recordingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode recording: %v", err)
	}
	if len(fetched.Clips) != 1 || fetched.Clips[0].ID != clip.ID {
		t.Fatalf("expected clip summary in recording response")
	}

	// Anonymous clip listing should work for published recording.
	req = httptest.NewRequest(http.MethodGet, "/api/recordings/"+recordingID+"/clips", nil)
	rec = httptest.NewRecorder()
	handler.RecordingByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected clip list status 200, got %d", rec.Code)
	}
	var clipList []clipExportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &clipList); err != nil {
		t.Fatalf("decode clip list: %v", err)
	}
	if len(clipList) != 1 {
		t.Fatalf("expected 1 clip, got %d", len(clipList))
	}

	// Owner deletes the recording.
	req = httptest.NewRequest(http.MethodDelete, "/api/recordings/"+recordingID, nil)
	req = withUser(req, creator)
	rec = httptest.NewRecorder()
	handler.RecordingByID(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected delete status 204, got %d", rec.Code)
	}

	// Ensure recording is gone from API.
	req = httptest.NewRequest(http.MethodGet, "/api/recordings?channelId="+channel.ID, nil)
	rec = httptest.NewRecorder()
	handler.Recordings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 after delete, got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &anonymousList); err != nil {
		t.Fatalf("decode final list: %v", err)
	}
	if len(anonymousList) != 0 {
		t.Fatalf("expected no recordings after delete")
	}

	// Ensure session still intact
	sessions, err := store.ListStreamSessions(channel.ID)
	if err != nil {
		t.Fatalf("ListStreamSessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != session.ID {
		t.Fatalf("expected original session to remain")
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

	components, ok := payload["components"].([]interface{})
	if !ok {
		t.Fatalf("expected components array in response")
	}
	if len(components) == 0 {
		t.Fatalf("expected component health entries")
	}
}

func TestHealthIncludesDependencyStatuses(t *testing.T) {
	handler, _ := newTestHandler(t)
	handler.RateLimiter = pingFunc(func(context.Context) error { return nil })
	handler.ChatQueue = pingFunc(func(context.Context) error { return nil })

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

	components, ok := payload["components"].([]interface{})
	if !ok {
		t.Fatalf("expected components array in response")
	}
	if len(components) != 4 {
		t.Fatalf("expected 4 components, got %d", len(components))
	}

	statuses := make(map[string]map[string]interface{})
	for _, raw := range components {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected component entry type %T", raw)
		}
		name, _ := entry["component"].(string)
		statuses[name] = entry
	}

	expected := []string{"datastore", "sessions", "rate_limiter", "chat_queue"}
	for _, name := range expected {
		entry, ok := statuses[name]
		if !ok {
			t.Fatalf("missing component %s", name)
		}
		if status, _ := entry["status"].(string); status != "ok" {
			t.Fatalf("expected component %s to be ok, got %s", name, status)
		}
		if errVal, hasErr := entry["error"]; hasErr && errVal != "" {
			t.Fatalf("expected no error for component %s, got %v", name, errVal)
		}
	}
}

func TestHealthDegradedWhenRepositoryPingFails(t *testing.T) {
	handler, _ := newTestHandler(t)
	failing := failingRepository{Repository: handler.Store, err: errors.New("datastore unreachable")}
	handler.Store = failing
	handler.RateLimiter = pingFunc(func(context.Context) error { return nil })
	handler.ChatQueue = pingFunc(func(context.Context) error { return nil })

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.Health(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health payload: %v", err)
	}
	if payload["status"] != "degraded" {
		t.Fatalf("expected overall degraded status, got %v", payload["status"])
	}

	components, ok := payload["components"].([]interface{})
	if !ok {
		t.Fatalf("expected components array in response")
	}

	foundDatastore := false
	for _, raw := range components {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if entry["component"] == "datastore" {
			foundDatastore = true
			if entry["status"] != "degraded" {
				t.Fatalf("expected datastore status degraded, got %v", entry["status"])
			}
			if !strings.Contains(fmt.Sprint(entry["error"]), "datastore unreachable") {
				t.Fatalf("expected datastore error message, got %v", entry["error"])
			}
		}
	}
	if !foundDatastore {
		t.Fatalf("expected datastore component entry")
	}
}

func TestHealthDegradedWhenRedisDependencyFails(t *testing.T) {
	handler, _ := newTestHandler(t)
	redisErr := errors.New("redis offline")
	handler.RateLimiter = pingFunc(func(context.Context) error { return redisErr })
	handler.ChatQueue = pingFunc(func(context.Context) error { return nil })

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.Health(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health payload: %v", err)
	}
	if payload["status"] != "degraded" {
		t.Fatalf("expected overall degraded status, got %v", payload["status"])
	}

	components, ok := payload["components"].([]interface{})
	if !ok {
		t.Fatalf("expected components array in response")
	}

	for _, raw := range components {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if entry["component"] == "rate_limiter" {
			if entry["status"] != "degraded" {
				t.Fatalf("expected rate limiter degraded, got %v", entry["status"])
			}
			if !strings.Contains(fmt.Sprint(entry["error"]), redisErr.Error()) {
				t.Fatalf("expected redis error message, got %v", entry["error"])
			}
			return
		}
	}
	t.Fatalf("expected rate limiter component entry")
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

func TestChannelStreamEndpointsUnavailableWithoutIngest(t *testing.T) {
	handler, store := newTestHandler(t)
	handler.Store = ingestUnavailableRepo{Repository: store}

	creator, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Streamer",
		Email:       "streamer@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	channel, err := store.CreateChannel(creator.ID, "No Ingest", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	startPayload := map[string]any{"renditions": []string{"720p"}}
	body, _ := json.Marshal(startPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/channels/"+channel.ID+"/stream/start", bytes.NewReader(body))
	req = withUser(req, creator)
	rec := httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected start status 503, got %d", rec.Code)
	}

	stored, ok := store.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("expected to reload channel %s", channel.ID)
	}
	if stored.LiveState != "offline" {
		t.Fatalf("expected offline channel after failed start, got %s", stored.LiveState)
	}
	if stored.CurrentSessionID != nil {
		t.Fatalf("expected current session to remain nil, got %v", stored.CurrentSessionID)
	}

	session, err := store.StartStream(channel.ID, []string{"720p"})
	if err != nil {
		t.Fatalf("StartStream: %v", err)
	}

	stopPayload := map[string]any{"peakConcurrent": 15}
	body, _ = json.Marshal(stopPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/channels/"+channel.ID+"/stream/stop", bytes.NewReader(body))
	req = withUser(req, creator)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected stop status 503, got %d", rec.Code)
	}

	stored, ok = store.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("expected to reload channel %s after stop", channel.ID)
	}
	if stored.CurrentSessionID == nil || *stored.CurrentSessionID != session.ID {
		t.Fatalf("expected current session to remain %s, got %v", session.ID, stored.CurrentSessionID)
	}
	if stored.LiveState != "live" {
		t.Fatalf("expected channel to remain live, got %s", stored.LiveState)
	}
}

func TestRotateStreamKeyEndpoint(t *testing.T) {
	handler, store := newTestHandler(t)

	owner, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
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

	channel, err := store.CreateChannel(owner.ID, "Studio", "gaming", []string{"retro"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	originalKey := channel.StreamKey

	// Owner rotates key successfully.
	req := httptest.NewRequest(http.MethodPost, "/api/channels/"+channel.ID+"/stream/rotate", nil)
	req = withUser(req, owner)
	rec := httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected owner rotation status 200, got %d", rec.Code)
	}
	var resp channelResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode rotate response: %v", err)
	}
	if resp.StreamKey == "" {
		t.Fatal("expected rotated stream key in response")
	}
	if resp.StreamKey == originalKey {
		t.Fatalf("expected rotated stream key to differ from original %s", originalKey)
	}

	updated, ok := store.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("channel %s missing after rotation", channel.ID)
	}
	if updated.StreamKey != resp.StreamKey {
		t.Fatalf("expected store stream key %s, got %s", resp.StreamKey, updated.StreamKey)
	}

	// Viewer without access is forbidden.
	req = httptest.NewRequest(http.MethodPost, "/api/channels/"+channel.ID+"/stream/rotate", nil)
	req = withUser(req, viewer)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected viewer rotation status 403, got %d", rec.Code)
	}

	// Admin can rotate even when not owner.
	req = httptest.NewRequest(http.MethodPost, "/api/channels/"+channel.ID+"/stream/rotate", nil)
	req = withUser(req, admin)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected admin rotation status 200, got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode admin rotate response: %v", err)
	}
	if resp.StreamKey == updated.StreamKey {
		t.Fatalf("expected admin rotation to change stream key from %s", updated.StreamKey)
	}

	latest, ok := store.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("channel %s missing after admin rotation", channel.ID)
	}
	if latest.StreamKey != resp.StreamKey {
		t.Fatalf("expected final stream key %s, got %s", resp.StreamKey, latest.StreamKey)
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

func TestChannelByIDTrailingSlashMatchesBaseRoute(t *testing.T) {
	handler, store := newTestHandler(t)

	owner, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}

	channel, err := store.CreateChannel(owner.ID, "Studio", "gaming", []string{"retro"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	baseReq := httptest.NewRequest(http.MethodGet, "/api/channels/"+channel.ID, nil)
	baseRec := httptest.NewRecorder()
	handler.ChannelByID(baseRec, baseReq)
	if baseRec.Code != http.StatusOK {
		t.Fatalf("expected base route status 200, got %d", baseRec.Code)
	}

	slashReq := httptest.NewRequest(http.MethodGet, "/api/channels/"+channel.ID+"/", nil)
	slashRec := httptest.NewRecorder()
	handler.ChannelByID(slashRec, slashReq)
	if slashRec.Code != http.StatusOK {
		t.Fatalf("expected trailing slash status 200, got %d", slashRec.Code)
	}

	var basePayload, slashPayload channelPublicResponse
	if err := json.Unmarshal(baseRec.Body.Bytes(), &basePayload); err != nil {
		t.Fatalf("decode base response: %v", err)
	}
	if err := json.Unmarshal(slashRec.Body.Bytes(), &slashPayload); err != nil {
		t.Fatalf("decode trailing slash response: %v", err)
	}
	if !reflect.DeepEqual(basePayload, slashPayload) {
		t.Fatalf("expected trailing slash response to match base route\nbase: %#v\nslash: %#v", basePayload, slashPayload)
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

func TestChatRoutesAuthorization(t *testing.T) {
	handler, store := newTestHandler(t)
	owner, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Test Channel", "", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	message, err := store.CreateChatMessage(channel.ID, owner.ID, "hello world")
	if err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/channels/"+channel.ID+"/chat", nil)
	rec := httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected guest chat status 200, got %d", rec.Code)
	}
	var messages []chatMessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &messages); err != nil {
		t.Fatalf("decode guest chat response: %v", err)
	}
	if len(messages) == 0 {
		t.Fatalf("expected messages in guest chat response")
	}

	payload := map[string]interface{}{
		"userId":  owner.ID,
		"content": "Unauthorized",
	}
	body, _ := json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPost, "/api/channels/"+channel.ID+"/chat", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected anonymous chat post status 401, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/channels/"+channel.ID+"/chat/"+message.ID, nil)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected anonymous chat delete status 401, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/channels/"+channel.ID+"/chat/"+message.ID, nil)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected chat message GET status 405, got %d", rec.Code)
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
	viewer, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Viewer",
		Email:       "viewer@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser viewer: %v", err)
	}
	admin, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Admin",
		Email:       "admin@example.com",
		Roles:       []string{"admin"},
	})
	if err != nil {
		t.Fatalf("CreateUser admin: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Main Stage", "music", []string{"live"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if _, err := store.StartStream(channel.ID, []string{"1080p"}); err != nil {
		t.Fatalf("StartStream: %v", err)
	}

	payload := map[string]interface{}{
		"displayName":       "Streamer Deluxe",
		"email":             "streamer+updates@example.com",
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
	if response.DisplayName != payload["displayName"] {
		t.Fatalf("expected display name %s, got %s", payload["displayName"], response.DisplayName)
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

	req = httptest.NewRequest(http.MethodGet, "/api/profiles/"+owner.ID, nil)
	rec = httptest.NewRecorder()
	handler.ProfileByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected public profile status 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/profiles/missing", nil)
	req = withUser(req, owner)
	rec = httptest.NewRecorder()
	handler.ProfileByID(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected missing profile status 404, got %d", rec.Code)
	}

	storedProfile, ok := store.GetProfile(owner.ID)
	if !ok {
		t.Fatalf("expected persisted profile for %s", owner.ID)
	}
	updatedOwner, ok := store.GetUser(owner.ID)
	if !ok {
		t.Fatalf("expected stored user for %s", owner.ID)
	}
	if updatedOwner.DisplayName != payload["displayName"] {
		t.Fatalf("expected user display name updated to %s", payload["displayName"])
	}
	if updatedOwner.Email != payload["email"] {
		t.Fatalf("expected user email updated to %s", payload["email"])
	}

	viewerPayload := map[string]interface{}{
		"bio": "viewer cannot edit others",
	}
	viewerBody, _ := json.Marshal(viewerPayload)
	req = httptest.NewRequest(http.MethodPut, "/api/profiles/"+owner.ID, bytes.NewReader(viewerBody))
	req = withUser(req, viewer)
	rec = httptest.NewRecorder()
	handler.ProfileByID(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected viewer forbidden status 403, got %d", rec.Code)
	}
	afterForbidden, ok := store.GetProfile(owner.ID)
	if !ok {
		t.Fatalf("expected profile to remain after forbidden update")
	}
	if !reflect.DeepEqual(storedProfile, afterForbidden) {
		t.Fatalf("expected profile to remain unchanged after forbidden update")
	}

	adminPayload := map[string]interface{}{
		"bio":       "Updated by admin",
		"avatarUrl": "https://cdn.example.com/admin-updated.png",
	}
	adminBody, _ := json.Marshal(adminPayload)
	req = httptest.NewRequest(http.MethodPut, "/api/profiles/"+owner.ID, bytes.NewReader(adminBody))
	req = withUser(req, admin)
	rec = httptest.NewRecorder()
	handler.ProfileByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected admin profile update status 200, got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode admin profile response: %v", err)
	}
	if response.Bio != "Updated by admin" {
		t.Fatalf("expected bio updated by admin, got %s", response.Bio)
	}
	if response.AvatarURL != "https://cdn.example.com/admin-updated.png" {
		t.Fatalf("expected avatar updated by admin, got %s", response.AvatarURL)
	}
	adminUpdated, ok := store.GetProfile(owner.ID)
	if !ok {
		t.Fatalf("expected stored profile after admin update")
	}
	if adminUpdated.Bio != "Updated by admin" {
		t.Fatalf("expected stored profile bio updated by admin")
	}
	if adminUpdated.AvatarURL != "https://cdn.example.com/admin-updated.png" {
		t.Fatalf("expected stored profile avatar updated by admin")
	}

	viewerPayload = map[string]interface{}{
		"bio": "Just a viewer",
	}
	viewerBody, _ = json.Marshal(viewerPayload)
	req = httptest.NewRequest(http.MethodPut, "/api/profiles/"+viewer.ID, bytes.NewReader(viewerBody))
	req = withUser(req, viewer)
	rec = httptest.NewRecorder()
	handler.ProfileByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected viewer profile update status 200, got %d", rec.Code)
	}
}

func TestHandleUpsertProfileDonationValidation(t *testing.T) {
	setup := func(t *testing.T) (*Handler, *storage.Storage, models.User) {
		t.Helper()
		handler, store := newTestHandler(t)
		owner, err := store.CreateUser(storage.CreateUserParams{
			DisplayName: "Owner",
			Email:       "owner@example.com",
			Roles:       []string{"creator"},
		})
		if err != nil {
			t.Fatalf("CreateUser owner: %v", err)
		}
		return handler, store, owner
	}

	t.Run("valid donation payload", func(t *testing.T) {
		handler, _, owner := setup(t)
		payload := map[string]interface{}{
			"donationAddresses": []map[string]string{{
				"currency": "eth",
				"address":  "0xabc123",
				"note":     "primary wallet",
			}},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, "/api/profiles/"+owner.ID, bytes.NewReader(body))
		req = withUser(req, owner)
		rec := httptest.NewRecorder()
		handler.ProfileByID(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		var resp profileViewResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp.DonationAddresses) != 1 {
			t.Fatalf("expected 1 donation address, got %d", len(resp.DonationAddresses))
		}
		if resp.DonationAddresses[0].Currency != "ETH" {
			t.Fatalf("expected currency to normalize to ETH, got %s", resp.DonationAddresses[0].Currency)
		}
	})

	t.Run("invalid donation currency", func(t *testing.T) {
		handler, _, owner := setup(t)
		payload := map[string]interface{}{
			"donationAddresses": []map[string]string{{
				"currency": "et1",
				"address":  "0xabc123",
			}},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, "/api/profiles/"+owner.ID, bytes.NewReader(body))
		req = withUser(req, owner)
		rec := httptest.NewRecorder()
		handler.ProfileByID(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("invalid donation address", func(t *testing.T) {
		handler, _, owner := setup(t)
		payload := map[string]interface{}{
			"donationAddresses": []map[string]string{{
				"currency": "eth",
				"address":  "bad address",
			}},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPut, "/api/profiles/"+owner.ID, bytes.NewReader(body))
		req = withUser(req, owner)
		rec := httptest.NewRecorder()
		handler.ProfileByID(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})
}

func TestChatReportsAPI(t *testing.T) {
	handler, store := newTestHandler(t)
	owner, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	reporter, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Reporter", Email: "reporter@example.com"})
	if err != nil {
		t.Fatalf("create reporter: %v", err)
	}
	target, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Target", Email: "target@example.com"})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Arena", "gaming", nil)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	queue := chat.NewMemoryQueue(8)
	handler.ChatGateway = chat.NewGateway(chat.GatewayConfig{Queue: queue, Store: store})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go storage.NewChatWorker(store, queue, nil).Run(ctx)

	// Apply a ban to populate restrictions endpoint.
	if err := store.ApplyChatEvent(chat.Event{Type: chat.EventTypeModeration, Moderation: &chat.ModerationEvent{Action: chat.ModerationActionBan, ChannelID: channel.ID, ActorID: owner.ID, TargetID: target.ID, Reason: "spam"}, OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatalf("apply ban: %v", err)
	}

	payload := chatReportRequest{TargetID: target.ID, Reason: "abuse"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/channels/"+channel.ID+"/chat/reports", bytes.NewReader(body))
	req = withUser(req, reporter)
	rec := httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected report submit 202, got %d", rec.Code)
	}
	var reportResp chatReportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &reportResp); err != nil {
		t.Fatalf("decode report response: %v", err)
	}

	// Wait for worker to persist report.
	deadline := time.After(2 * time.Second)
	for {
		reports, err := store.ListChatReports(channel.ID, true)
		if err == nil && len(reports) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for report persistence")
		case <-time.After(20 * time.Millisecond):
		}
	}
	req = httptest.NewRequest(http.MethodGet, "/api/channels/"+channel.ID+"/chat/reports", nil)
	req = withUser(req, owner)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected open reports status 200, got %d", rec.Code)
	}
	var openReports []chatReportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &openReports); err != nil {
		t.Fatalf("decode open reports response: %v", err)
	}
	if len(openReports) != 1 || openReports[0].Status != "open" {
		t.Fatalf("expected open report, got %+v", openReports)
	}

	resolveBody, _ := json.Marshal(resolveChatReportRequest{Resolution: "handled"})
	req = httptest.NewRequest(http.MethodPost, "/api/channels/"+channel.ID+"/chat/moderation/reports/"+reportResp.ID+"/resolve", bytes.NewReader(resolveBody))
	req = withUser(req, owner)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected resolve status 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/channels/"+channel.ID+"/chat/moderation/restrictions", nil)
	req = withUser(req, owner)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected restrictions status 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/channels/"+channel.ID+"/chat/reports?status=all", nil)
	req = withUser(req, owner)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected list reports status 200, got %d", rec.Code)
	}
	var reports []chatReportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &reports); err != nil {
		t.Fatalf("decode reports response: %v", err)
	}
	if len(reports) != 1 || reports[0].Status != "resolved" {
		t.Fatalf("expected resolved report, got %+v", reports)
	}
}

func TestChatModerationPostRequiresOwnerOrAdmin(t *testing.T) {
	handler, store := newTestHandler(t)
	owner, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	moderator, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Mod", Email: "mod@example.com"})
	if err != nil {
		t.Fatalf("create moderator: %v", err)
	}
	target, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Target", Email: "target@example.com"})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Arena", "gaming", nil)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	queue := chat.NewMemoryQueue(4)
	handler.ChatGateway = chat.NewGateway(chat.GatewayConfig{Queue: queue, Store: store})

	payload := chatModerationRequest{Action: "ban", TargetID: target.ID}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/channels/"+channel.ID+"/chat/moderation", bytes.NewReader(body))
	req = withUser(req, moderator)
	rec := httptest.NewRecorder()

	handler.ChannelByID(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected moderation post status 403, got %d", rec.Code)
	}
}

func TestChatModerationRestrictionsOmitExpiredTimeouts(t *testing.T) {
	handler, store := newTestHandler(t)

	owner, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Arena", "gaming", nil)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	queue := chat.NewMemoryQueue(4)
	handler.ChatGateway = chat.NewGateway(chat.GatewayConfig{Queue: queue, Store: store})
	active, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Active", Email: "active@example.com"})
	if err != nil {
		t.Fatalf("create active user: %v", err)
	}
	expired, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Expired", Email: "expired@example.com"})
	if err != nil {
		t.Fatalf("create expired user: %v", err)
	}

	now := time.Now().UTC()
	activeExpiry := now.Add(20 * time.Minute)
	expiredExpiry := now.Add(-5 * time.Minute)

	if err := store.ApplyChatEvent(chat.Event{
		Type: chat.EventTypeModeration,
		Moderation: &chat.ModerationEvent{
			Action:    chat.ModerationActionTimeout,
			ChannelID: channel.ID,
			ActorID:   owner.ID,
			TargetID:  active.ID,
			Reason:    "active",
			ExpiresAt: &activeExpiry,
		},
		OccurredAt: now.Add(-2 * time.Minute),
	}); err != nil {
		t.Fatalf("apply active timeout: %v", err)
	}

	if err := store.ApplyChatEvent(chat.Event{
		Type: chat.EventTypeModeration,
		Moderation: &chat.ModerationEvent{
			Action:    chat.ModerationActionTimeout,
			ChannelID: channel.ID,
			ActorID:   owner.ID,
			TargetID:  expired.ID,
			Reason:    "expired",
			ExpiresAt: &expiredExpiry,
		},
		OccurredAt: now.Add(-10 * time.Minute),
	}); err != nil {
		t.Fatalf("apply expired timeout: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/channels/"+channel.ID+"/chat/moderation/restrictions", nil)
	req = withUser(req, owner)
	rec := httptest.NewRecorder()

	handler.ChannelByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected restrictions status 200, got %d", rec.Code)
	}

	var resp []chatRestrictionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode restrictions response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 restriction, got %+v", resp)
	}
	if resp[0].TargetID != active.ID {
		t.Fatalf("expected restriction for active user, got %+v", resp[0])
	}
	if resp[0].ExpiresAt == nil {
		t.Fatalf("expected expiry timestamp for active timeout")
	}
	if _, err := time.Parse(time.RFC3339Nano, *resp[0].ExpiresAt); err != nil {
		t.Fatalf("expected RFC3339 expiry, got %q", *resp[0].ExpiresAt)
	}
}

func TestMonetizationEndpoints(t *testing.T) {
	handler, store := newTestHandler(t)
	owner, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	supporter, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Supporter", Email: "supporter@example.com"})
	if err != nil {
		t.Fatalf("create supporter: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Arena", "gaming", nil)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	longMessage := strings.Repeat("m", storage.MaxTipMessageLength+1)
	badTipReq := createTipRequest{Amount: json.Number("10"), Currency: "USD", Provider: "stripe", Message: longMessage}
	body, _ := json.Marshal(badTipReq)
	req := httptest.NewRequest(http.MethodPost, "/api/channels/"+channel.ID+"/monetization/tips", bytes.NewReader(body))
	req = withUser(req, supporter)
	rec := httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected tip status 400 for long message, got %d", rec.Code)
	}

	tipReq := createTipRequest{Amount: json.Number("10"), Currency: "USD", Provider: "stripe", Message: "gg"}
	body, _ = json.Marshal(tipReq)
	req = httptest.NewRequest(http.MethodPost, "/api/channels/"+channel.ID+"/monetization/tips", bytes.NewReader(body))
	req = withUser(req, supporter)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected tip status 201, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/channels/"+channel.ID+"/monetization/tips", nil)
	req = withUser(req, owner)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected tip list status 200, got %d", rec.Code)
	}

	subReq := createSubscriptionRequest{Tier: "gold", Provider: "stripe", Amount: json.Number("9.99"), Currency: "usd", DurationDays: 30, AutoRenew: true}
	body, _ = json.Marshal(subReq)
	req = httptest.NewRequest(http.MethodPost, "/api/channels/"+channel.ID+"/monetization/subscriptions", bytes.NewReader(body))
	req = withUser(req, supporter)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected subscription status 201, got %d", rec.Code)
	}
	var subResp subscriptionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &subResp); err != nil {
		t.Fatalf("decode subscription: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/channels/"+channel.ID+"/monetization/subscriptions", nil)
	req = withUser(req, owner)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected subscription list status 200, got %d", rec.Code)
	}

	cancelURL := "/api/channels/" + channel.ID + "/monetization/subscriptions/" + subResp.ID
	req = httptest.NewRequest(http.MethodDelete, cancelURL, nil)
	req = withUser(req, supporter)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected cancel status 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/channels/"+channel.ID+"/monetization/subscriptions?status=all", nil)
	req = withUser(req, owner)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected subscription history status 200, got %d", rec.Code)
	}
	var subs []subscriptionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &subs); err != nil {
		t.Fatalf("decode subscriptions response: %v", err)
	}
	if len(subs) != 1 || subs[0].Status != "cancelled" {
		t.Fatalf("expected cancelled subscription, got %+v", subs)
	}
}

func TestChannelSubscribeEndpointTogglesState(t *testing.T) {
	handler, store := newTestHandler(t)

	owner, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Chill", "music", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	viewer, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Viewer",
		Email:       "viewer@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser viewer: %v", err)
	}

	path := fmt.Sprintf("/api/channels/%s/subscribe", channel.ID)

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected GET status 200, got %d", rec.Code)
	}
	var initial subscriptionStateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &initial); err != nil {
		t.Fatalf("decode initial subscription state: %v", err)
	}
	if initial.Subscribers != 0 || initial.Subscribed {
		t.Fatalf("expected zero subscribers and unsubscribed state, got %+v", initial)
	}

	req = httptest.NewRequest(http.MethodPost, path, nil)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected POST without auth to return 401, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, path, nil)
	req = withUser(req, viewer)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected POST status 200, got %d", rec.Code)
	}
	var subscribed subscriptionStateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &subscribed); err != nil {
		t.Fatalf("decode subscribed state: %v", err)
	}
	if !subscribed.Subscribed || subscribed.Subscribers != 1 {
		t.Fatalf("expected subscribed state with one subscriber, got %+v", subscribed)
	}
	if subscribed.RenewsAt == nil || *subscribed.RenewsAt == "" {
		t.Fatalf("expected renewsAt timestamp, got %+v", subscribed)
	}

	req = httptest.NewRequest(http.MethodDelete, path, nil)
	req = withUser(req, viewer)
	rec = httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected DELETE status 200, got %d", rec.Code)
	}
	var unsubscribed subscriptionStateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &unsubscribed); err != nil {
		t.Fatalf("decode unsubscribed state: %v", err)
	}
	if unsubscribed.Subscribed || unsubscribed.Subscribers != 0 {
		t.Fatalf("expected unsubscribed state with zero subscribers, got %+v", unsubscribed)
	}
}

func TestChannelPlaybackIncludesSubscriptionState(t *testing.T) {
	handler, store := newTestHandler(t)

	owner, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	donation := []models.CryptoAddress{{Currency: "eth", Address: "0xabc123", Note: "Main"}}
	if _, err := store.UpsertProfile(owner.ID, storage.ProfileUpdate{DonationAddresses: &donation}); err != nil {
		t.Fatalf("UpsertProfile donation: %v", err)
	}
	if profile, ok := handler.Store.GetProfile(owner.ID); !ok || len(profile.DonationAddresses) == 0 {
		t.Fatalf("handler store missing donation addresses for %s", owner.ID)
	}
	channel, err := store.CreateChannel(owner.ID, "Ambient", "music", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	viewer, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Viewer",
		Email:       "viewer@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser viewer: %v", err)
	}

	_, err = store.CreateSubscription(storage.CreateSubscriptionParams{
		ChannelID: channel.ID,
		UserID:    viewer.ID,
		Tier:      "VIP",
		Provider:  "internal",
		Amount:    models.MustParseMoney("5"),
		Currency:  "USD",
		Duration:  30 * 24 * time.Hour,
		AutoRenew: true,
	})
	if err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}

	path := fmt.Sprintf("/api/channels/%s/playback", channel.ID)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req = withUser(req, viewer)
	rec := httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected playback status 200, got %d", rec.Code)
	}

	var payload channelPlaybackResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode playback response: %v", err)
	}
	if payload.Subscription == nil {
		t.Fatal("expected playback to include subscription block")
	}
	if !payload.Subscription.Subscribed {
		t.Fatalf("expected subscribed state, got %+v", payload.Subscription)
	}
	if payload.Subscription.Subscribers != 1 {
		t.Fatalf("expected subscriber count of 1, got %+v", payload.Subscription)
	}
	if payload.Subscription.Tier != "VIP" {
		t.Fatalf("expected tier VIP, got %+v", payload.Subscription)
	}
	if payload.Subscription.RenewsAt == nil || *payload.Subscription.RenewsAt == "" {
		t.Fatalf("expected renewsAt timestamp, got %+v", payload.Subscription)
	}
	if len(payload.DonationAddresses) != 1 {
		t.Fatalf("expected 1 donation address, got %d", len(payload.DonationAddresses))
	}
	donationResp := payload.DonationAddresses[0]
	if donationResp.Currency != "ETH" {
		t.Fatalf("expected donation currency ETH, got %s", donationResp.Currency)
	}
	if donationResp.Address != "0xabc123" {
		t.Fatalf("expected donation address 0xabc123, got %s", donationResp.Address)
	}
	if donationResp.Note != "Main" {
		t.Fatalf("expected donation note Main, got %s", donationResp.Note)
	}
}

func TestChannelVodsReturnPublishedRecordings(t *testing.T) {
	handler, store := newTestHandler(t)

	owner, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Archive", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	if _, err := store.StartStream(channel.ID, []string{"1080p"}); err != nil {
		t.Fatalf("StartStream: %v", err)
	}
	if _, err := store.StopStream(channel.ID, 42); err != nil {
		t.Fatalf("StopStream: %v", err)
	}
	recordings, err := store.ListRecordings(channel.ID, true)
	if err != nil {
		t.Fatalf("ListRecordings: %v", err)
	}
	if len(recordings) == 0 {
		t.Fatal("expected at least one recording")
	}
	published, err := store.PublishRecording(recordings[0].ID)
	if err != nil {
		t.Fatalf("PublishRecording: %v", err)
	}

	if _, err := store.StartStream(channel.ID, []string{"720p"}); err != nil {
		t.Fatalf("StartStream second: %v", err)
	}
	if _, err := store.StopStream(channel.ID, 24); err != nil {
		t.Fatalf("StopStream second: %v", err)
	}

	path := fmt.Sprintf("/api/channels/%s/vods", channel.ID)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ChannelByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected VOD status 200, got %d", rec.Code)
	}

	var payload vodCollectionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode VOD payload: %v", err)
	}
	if payload.ChannelID != channel.ID {
		t.Fatalf("expected channelId %s, got %s", channel.ID, payload.ChannelID)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected exactly one published VOD, got %d", len(payload.Items))
	}
	item := payload.Items[0]
	if item.ID != published.ID {
		t.Fatalf("expected VOD %s, got %s", published.ID, item.ID)
	}
	if item.PublishedAt == "" {
		t.Fatal("expected publishedAt to be populated")
	}
	if item.DurationSeconds != published.DurationSeconds {
		t.Fatalf("expected duration %d, got %d", published.DurationSeconds, item.DurationSeconds)
	}
}

func TestModerationQueueLifecycle(t *testing.T) {
	handler, store := newTestHandler(t)

	admin, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Admin",
		Email:       "admin@example.com",
		Roles:       []string{"admin"},
	})
	if err != nil {
		t.Fatalf("CreateUser admin: %v", err)
	}
	reporter, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Reporter",
		Email:       "reporter@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser reporter: %v", err)
	}
	target, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Target",
		Email:       "target@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser target: %v", err)
	}
	channel, err := store.CreateChannel(admin.ID, "Studio", "", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	message, err := store.CreateChatMessage(channel.ID, target.ID, "spam message")
	if err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}
	report, err := store.CreateChatReport(channel.ID, reporter.ID, target.ID, "spam", message.ID, "")
	if err != nil {
		t.Fatalf("CreateChatReport: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/moderation/queue", nil)
	req = withUser(req, admin)
	rec := httptest.NewRecorder()
	handler.ModerationQueue(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected moderation queue status 200, got %d", rec.Code)
	}
	var initial moderationQueueResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &initial); err != nil {
		t.Fatalf("decode moderation queue: %v", err)
	}
	if len(initial.Queue) != 1 {
		t.Fatalf("expected one queued flag, got %d", len(initial.Queue))
	}
	flag := initial.Queue[0]
	if flag.ID != report.ID {
		t.Fatalf("expected flag id %s, got %s", report.ID, flag.ID)
	}
	if flag.Reporter == nil || flag.Reporter.DisplayName != reporter.DisplayName {
		t.Fatalf("expected reporter display name %q", reporter.DisplayName)
	}
	if flag.ChannelTitle != "Studio" {
		t.Fatalf("expected channel title Studio, got %s", flag.ChannelTitle)
	}

	body, _ := json.Marshal(map[string]string{"resolution": "dismiss"})
	req = httptest.NewRequest(http.MethodPost, "/api/moderation/queue/"+report.ID, bytes.NewReader(body))
	req = withUser(req, admin)
	rec = httptest.NewRecorder()
	handler.ModerationQueueByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected resolve status 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/moderation/queue", nil)
	req = withUser(req, admin)
	rec = httptest.NewRecorder()
	handler.ModerationQueue(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected moderation queue status 200, got %d", rec.Code)
	}
	var resolved moderationQueueResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resolved); err != nil {
		t.Fatalf("decode resolved queue: %v", err)
	}
	if len(resolved.Queue) != 0 {
		t.Fatalf("expected empty queue after resolution, got %d", len(resolved.Queue))
	}
	if len(resolved.Actions) == 0 {
		t.Fatal("expected recent moderation actions")
	}
	action := resolved.Actions[0]
	if action.Action != "dismiss" {
		t.Fatalf("expected action dismiss, got %s", action.Action)
	}
	if action.Moderator == nil || action.Moderator.ID != admin.ID {
		t.Fatalf("expected moderator id %s", admin.ID)
	}
}

func TestAnalyticsOverview(t *testing.T) {
	handler, store := newTestHandler(t)

	admin, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Admin",
		Email:       "admin@example.com",
		Roles:       []string{"admin"},
	})
	if err != nil {
		t.Fatalf("CreateUser admin: %v", err)
	}
	creator, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Creator",
		Email:       "creator@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser creator: %v", err)
	}
	viewer, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Viewer",
		Email:       "viewer@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser viewer: %v", err)
	}
	channel, err := store.CreateChannel(creator.ID, "Main Stage", "", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if err := store.FollowChannel(viewer.ID, channel.ID); err != nil {
		t.Fatalf("FollowChannel: %v", err)
	}
	if _, err := store.CreateChatMessage(channel.ID, viewer.ID, "Hello world"); err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}
	if _, err := store.CreateChatMessage(channel.ID, viewer.ID, "Another message"); err != nil {
		t.Fatalf("CreateChatMessage second: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/analytics/overview", nil)
	req = withUser(req, admin)
	rec := httptest.NewRecorder()
	handler.AnalyticsOverview(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected analytics status 200, got %d", rec.Code)
	}
	var payload analyticsOverviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode analytics payload: %v", err)
	}
	if payload.Summary == nil {
		t.Fatal("expected analytics summary")
	}
	if payload.Summary.ChatMessages < 2 {
		t.Fatalf("expected chat messages >= 2, got %d", payload.Summary.ChatMessages)
	}
	if len(payload.PerChannel) != 1 {
		t.Fatalf("expected single channel entry, got %d", len(payload.PerChannel))
	}
	entry := payload.PerChannel[0]
	if entry.ChannelID != channel.ID {
		t.Fatalf("expected channel id %s, got %s", channel.ID, entry.ChannelID)
	}
	if entry.Followers != 1 {
		t.Fatalf("expected followers 1, got %d", entry.Followers)
	}
	if entry.ChatMessages < 2 {
		t.Fatalf("expected channel chat messages >= 2, got %d", entry.ChatMessages)
	}
}
