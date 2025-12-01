package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetSessionCookieDefaults(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	req.TLS = &tls.ConnectionState{}

	setSessionCookie(rec, req, "token", time.Now().Add(time.Hour), DefaultSessionCookiePolicy())

	cookie := findCookie(t, rec.Result().Cookies(), "bitriver_session")
	if cookie.Path != "/" {
		t.Fatalf("expected session cookie Path=/, got %q", cookie.Path)
	}
	if !cookie.HttpOnly {
		t.Fatal("expected session cookie to be HttpOnly by default")
	}
	if !cookie.Secure {
		t.Fatal("expected HTTPS request to set Secure on session cookie")
	}
}

func TestSetSessionCookieRespectsForwardedProto(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	req.Header.Set("X-Forwarded-Proto", "https")

	setSessionCookie(rec, req, "token", time.Now().Add(time.Hour), DefaultSessionCookiePolicy())

	cookie := findCookie(t, rec.Result().Cookies(), "bitriver_session")
	if !cookie.Secure {
		t.Fatal("expected Secure cookie when X-Forwarded-Proto includes HTTPS")
	}
}
