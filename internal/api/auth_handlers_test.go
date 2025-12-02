package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bitriver-live/internal/auth/oauth"
	"bitriver-live/internal/storage"
)

func TestLoginSessionCookieAttributes(t *testing.T) {
	cases := []struct {
		name         string
		configure    func(req *http.Request)
		policy       SessionCookiePolicy
		wantSecure   bool
		wantSameSite http.SameSite
	}{
		{
			name:         "insecure localhost defaults to non secure",
			configure:    func(req *http.Request) {},
			policy:       SessionCookiePolicy{},
			wantSecure:   false,
			wantSameSite: http.SameSiteStrictMode,
		},
		{
			name: "forwarded https enables secure cookie",
			configure: func(req *http.Request) {
				req.Header.Set("X-Forwarded-Proto", "https")
			},
			policy:       SessionCookiePolicy{},
			wantSecure:   true,
			wantSameSite: http.SameSiteStrictMode,
		},
		{
			name:      "secure policy forces secure flag",
			configure: func(req *http.Request) {},
			policy: SessionCookiePolicy{
				SameSite:   http.SameSiteLaxMode,
				SecureMode: SessionCookieSecureAlways,
			},
			wantSecure:   true,
			wantSameSite: http.SameSiteLaxMode,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, store := newTestHandler(t)
			handler.SessionCookiePolicy = tc.policy
			_, err := store.CreateUser(storage.CreateUserParams{
				DisplayName: "Viewer",
				Email:       "viewer@example.com",
				Password:    "supersecret",
			})
			if err != nil {
				t.Fatalf("CreateUser: %v", err)
			}

			payload, _ := json.Marshal(loginRequest{Email: "viewer@example.com", Password: "supersecret"})
			req := httptest.NewRequest(http.MethodPost, "http://localhost/api/auth/login", bytes.NewReader(payload))
			tc.configure(req)
			rec := httptest.NewRecorder()

			handler.Login(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", rec.Code)
			}
			cookie := findCookie(t, rec.Result().Cookies(), "bitriver_session")
			if cookie.Value == "" {
				t.Fatal("expected login to issue session cookie")
			}
			if cookie.Path != "/" {
				t.Fatalf("expected cookie path /, got %q", cookie.Path)
			}
			if !cookie.HttpOnly {
				t.Fatal("expected HttpOnly cookie")
			}
			if cookie.Secure != tc.wantSecure {
				t.Fatalf("expected Secure=%v, got %v", tc.wantSecure, cookie.Secure)
			}
			if cookie.SameSite != tc.wantSameSite {
				t.Fatalf("expected SameSite %v, got %v", tc.wantSameSite, cookie.SameSite)
			}

			expiresIn := time.Until(cookie.Expires)
			if expiresIn < 23*time.Hour || expiresIn > 24*time.Hour+time.Minute {
				t.Fatalf("unexpected cookie expiry duration: %v", expiresIn)
			}
			if cookie.MaxAge < int(23*time.Hour/time.Second) || cookie.MaxAge > int((24*time.Hour+time.Minute)/time.Second) {
				t.Fatalf("unexpected cookie MaxAge: %d", cookie.MaxAge)
			}
		})
	}
}

func TestDeleteSessionClearsCookieAttributes(t *testing.T) {
	cases := []struct {
		name       string
		configure  func(req *http.Request)
		wantSecure bool
	}{
		{
			name:       "http request clears insecure cookie",
			configure:  func(req *http.Request) {},
			wantSecure: false,
		},
		{
			name: "tls request clears secure cookie",
			configure: func(req *http.Request) {
				req.TLS = &tls.ConnectionState{}
			},
			wantSecure: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, store := newTestHandler(t)
			user, err := store.CreateUser(storage.CreateUserParams{
				DisplayName: "Viewer",
				Email:       "viewer@example.com",
				Password:    "supersecret",
			})
			if err != nil {
				t.Fatalf("CreateUser: %v", err)
			}
			token, _, err := handler.sessionManager().Create(user.ID)
			if err != nil {
				t.Fatalf("Create session: %v", err)
			}

			req := httptest.NewRequest(http.MethodDelete, "/api/auth/session", nil)
			req.AddCookie(&http.Cookie{Name: "bitriver_session", Value: token})
			tc.configure(req)
			rec := httptest.NewRecorder()

			handler.Session(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Fatalf("expected status 204, got %d", rec.Code)
			}
			cookie := findCookie(t, rec.Result().Cookies(), "bitriver_session")
			if cookie.Value != "" {
				t.Fatal("expected cookie to be cleared")
			}
			if cookie.MaxAge != -1 {
				t.Fatalf("expected MaxAge=-1, got %d", cookie.MaxAge)
			}
			if !cookie.Expires.Equal(time.Unix(0, 0).UTC()) {
				t.Fatalf("expected expires at unix epoch, got %v", cookie.Expires)
			}
			if cookie.Path != "/" {
				t.Fatalf("expected path /, got %q", cookie.Path)
			}
			if !cookie.HttpOnly {
				t.Fatal("expected HttpOnly cookie")
			}
			if cookie.Secure != tc.wantSecure {
				t.Fatalf("expected Secure=%v, got %v", tc.wantSecure, cookie.Secure)
			}
			if cookie.SameSite != http.SameSiteStrictMode {
				t.Fatalf("expected SameSite Strict, got %v", cookie.SameSite)
			}
		})
	}
}

func TestOAuthCallbackSessionCookiePolicy(t *testing.T) {
	handler, _ := newTestHandler(t)
	handler.SessionCookiePolicy = SessionCookiePolicy{
		SameSite:   http.SameSiteNoneMode,
		SecureMode: SessionCookieSecureAlways,
	}
	handler.OAuth = &oauthStub{completeResult: oauth.Completion{
		ReturnTo: "/dashboard",
		Profile: oauth.UserProfile{
			Provider:    "test",
			Subject:     "sub-1",
			Email:       "viewer@example.com",
			DisplayName: "Viewer",
		},
	}}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/test/callback?state=abc&code=xyz", nil)
	rec := httptest.NewRecorder()

	handler.OAuthByProvider(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", rec.Code)
	}
	cookie := findCookie(t, rec.Result().Cookies(), "bitriver_session")
	if cookie.Value == "" {
		t.Fatal("expected session cookie to be set")
	}
	if cookie.Path != "/" {
		t.Fatalf("expected path /, got %q", cookie.Path)
	}
	if !cookie.HttpOnly {
		t.Fatal("expected HttpOnly cookie")
	}
	if !cookie.Secure {
		t.Fatal("expected Secure cookie due to forced policy")
	}
	if cookie.SameSite != http.SameSiteNoneMode {
		t.Fatalf("expected SameSite=None, got %v", cookie.SameSite)
	}
	expiresIn := time.Until(cookie.Expires)
	if expiresIn <= 0 {
		t.Fatalf("expected positive expiry, got %v", expiresIn)
	}
	if cookie.MaxAge <= 0 {
		t.Fatalf("expected positive MaxAge, got %d", cookie.MaxAge)
	}
}
