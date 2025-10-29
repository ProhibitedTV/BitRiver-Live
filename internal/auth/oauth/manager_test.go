package oauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestManagerBeginAndComplete(t *testing.T) {
	tokenRequests := 0
	userinfoRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenRequests++
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse token form: %v", err)
			}
			if got := r.Form.Get("code"); got != "code-xyz" {
				t.Fatalf("expected code-xyz, got %q", got)
			}
			payload := map[string]string{"access_token": "token-123", "token_type": "Bearer"}
			_ = json.NewEncoder(w).Encode(payload)
		case "/userinfo":
			userinfoRequests++
			if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer token-123") {
				t.Fatalf("expected bearer token, got %q", got)
			}
			payload := map[string]any{
				"sub":   "user-1",
				"email": "viewer@example.com",
				"name":  "Viewer",
			}
			_ = json.NewEncoder(w).Encode(payload)
		default:
			t.Fatalf("unexpected request path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := ProviderConfig{
		Name:         "test",
		DisplayName:  "Test Provider",
		AuthorizeURL: server.URL + "/authorize",
		TokenURL:     server.URL + "/token",
		UserInfoURL:  server.URL + "/userinfo",
		ClientID:     "client-1",
		ClientSecret: "secret-1",
		RedirectURL:  "https://example.com/callback",
		Scopes:       []string{"profile", "email"},
		Profile: ProfileMapping{
			IDField:    "sub",
			EmailField: "email",
			NameField:  "name",
		},
	}

	mgr, err := NewManager([]ProviderConfig{cfg})
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	begin, err := mgr.Begin("test", "/dashboard")
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}
	if begin.State == "" {
		t.Fatal("expected state token to be generated")
	}
	parsed, err := url.Parse(begin.URL)
	if err != nil {
		t.Fatalf("parse authorize url: %v", err)
	}
	if parsed.Query().Get("state") != begin.State {
		t.Fatalf("expected state %q in authorize url", begin.State)
	}
	if parsed.Query().Get("client_id") != cfg.ClientID {
		t.Fatalf("expected client id in authorize url")
	}

	completion, err := mgr.Complete("test", begin.State, "code-xyz")
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if completion.ReturnTo != "/dashboard" {
		t.Fatalf("expected return path to be preserved, got %q", completion.ReturnTo)
	}
	if completion.Profile.Provider != "test" {
		t.Fatalf("expected provider test, got %q", completion.Profile.Provider)
	}
	if completion.Profile.Subject != "user-1" {
		t.Fatalf("expected subject user-1, got %q", completion.Profile.Subject)
	}
	if completion.Profile.Email != "viewer@example.com" {
		t.Fatalf("expected email viewer@example.com, got %q", completion.Profile.Email)
	}
	if completion.Profile.DisplayName != "Viewer" {
		t.Fatalf("expected display name Viewer, got %q", completion.Profile.DisplayName)
	}
	if tokenRequests != 1 {
		t.Fatalf("expected 1 token request, got %d", tokenRequests)
	}
	if userinfoRequests != 1 {
		t.Fatalf("expected 1 userinfo request, got %d", userinfoRequests)
	}
}

func TestManagerCancel(t *testing.T) {
	mgr, err := NewManager([]ProviderConfig{exampleProviderConfig()})
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	begin, err := mgr.Begin("example", "/viewer")
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}
	redirected, err := mgr.Cancel(begin.State)
	if err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	if redirected != "/viewer" {
		t.Fatalf("expected redirect /viewer, got %q", redirected)
	}
	if _, err := mgr.Cancel(begin.State); !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("expected ErrStateInvalid on reused state, got %v", err)
	}
}

func exampleProviderConfig() ProviderConfig {
	return ProviderConfig{
		Name:         "example",
		DisplayName:  "Example",
		AuthorizeURL: "https://auth.example.com/oauth/authorize",
		TokenURL:     "https://auth.example.com/oauth/token",
		UserInfoURL:  "https://auth.example.com/oauth/userinfo",
		ClientID:     "client",
		ClientSecret: "secret",
		RedirectURL:  "https://app.example.com/oauth/callback",
		Scopes:       []string{"profile"},
		Profile:      ProfileMapping{IDField: "id", EmailField: "email", NameField: "name"},
	}
}
