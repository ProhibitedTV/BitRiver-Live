package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"bitriver-live/internal/auth/oauth"
	"bitriver-live/internal/storage"
)

func TestOAuthMFARedirectOmitsMFATokenFromQuery(t *testing.T) {
	handler, store := newTestHandler(t)
	if _, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Admin",
		Email:       "admin@example.com",
		Password:    "supersecret",
		Roles:       []string{"admin"},
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	handler.OAuth = &oauthStub{completeResult: oauth.Completion{
		ReturnTo: "/dashboard",
		Profile: oauth.UserProfile{
			Provider:    "test",
			Subject:     "sub-1",
			Email:       "admin@example.com",
			DisplayName: "Admin",
		},
	}}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/test/callback?state=abc&code=xyz", nil)
	rec := httptest.NewRecorder()

	handler.OAuthByProvider(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("expected redirect location")
	}
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse location: %v", err)
	}
	if parsed.Path != "/signup" {
		t.Fatalf("expected redirect to /signup, got %q", parsed.Path)
	}
	query := parsed.Query()
	if token := query.Get("mfaToken"); token != "" {
		t.Fatalf("expected no mfaToken in query, got %q", token)
	}
	if query.Get("mfa") == "" {
		t.Fatal("expected mfa mode query parameter")
	}

	challengeCookie := findCookie(t, rec.Result().Cookies(), mfaChallengeCookieName)
	if challengeCookie.Value == "" {
		t.Fatal("expected MFA challenge cookie")
	}
	if !challengeCookie.HttpOnly {
		t.Fatal("expected MFA challenge cookie to be HttpOnly")
	}
	if challengeCookie.Path != "/api/auth/mfa" {
		t.Fatalf("expected MFA challenge cookie path /api/auth/mfa, got %q", challengeCookie.Path)
	}
}

func TestOAuthMFAChallengeCookieSupportsVerifyFlow(t *testing.T) {
	handler, store := newTestHandler(t)
	if _, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Admin",
		Email:       "admin@example.com",
		Password:    "supersecret",
		Roles:       []string{"admin"},
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	handler.OAuth = &oauthStub{completeResult: oauth.Completion{
		ReturnTo: "/dashboard",
		Profile: oauth.UserProfile{
			Provider:    "test",
			Subject:     "sub-2",
			Email:       "admin@example.com",
			DisplayName: "Admin",
		},
	}}

	callbackReq := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/test/callback?state=abc&code=xyz", nil)
	callbackRec := httptest.NewRecorder()
	handler.OAuthByProvider(callbackRec, callbackReq)
	if callbackRec.Code != http.StatusSeeOther {
		t.Fatalf("expected callback redirect, got %d", callbackRec.Code)
	}
	challengeCookie := findCookie(t, callbackRec.Result().Cookies(), mfaChallengeCookieName)
	if challengeCookie.Value == "" {
		t.Fatal("expected MFA challenge cookie")
	}

	enrollPayload, _ := json.Marshal(mfaEnrollRequest{})
	enrollReq := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/enroll", bytes.NewReader(enrollPayload))
	enrollReq.AddCookie(challengeCookie)
	enrollRec := httptest.NewRecorder()
	handler.MFAEnroll(enrollRec, enrollReq)
	if enrollRec.Code != http.StatusOK {
		t.Fatalf("expected enroll status 200, got %d", enrollRec.Code)
	}
	var enrollment mfaEnrollmentResponse
	if err := json.NewDecoder(enrollRec.Body).Decode(&enrollment); err != nil {
		t.Fatalf("decode enrollment: %v", err)
	}
	if len(enrollment.RecoveryCodes) == 0 {
		t.Fatal("expected recovery codes")
	}

	verifyPayload, _ := json.Marshal(mfaVerifyRequest{Code: enrollment.RecoveryCodes[0]})
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/verify", bytes.NewReader(verifyPayload))
	verifyReq.AddCookie(challengeCookie)
	verifyRec := httptest.NewRecorder()
	handler.MFAVerify(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("expected verify status 200, got %d", verifyRec.Code)
	}
	if findCookie(t, verifyRec.Result().Cookies(), sessionCookieName).Value == "" {
		t.Fatal("expected session cookie after MFA verification")
	}
	cleared := findCookie(t, verifyRec.Result().Cookies(), mfaChallengeCookieName)
	if cleared.MaxAge != -1 {
		t.Fatalf("expected MFA challenge cookie to be cleared, got max age %d", cleared.MaxAge)
	}
}
