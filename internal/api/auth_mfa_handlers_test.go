package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bitriver-live/internal/storage"
)

func TestLoginRequiresMFAForPrivilegedRoles(t *testing.T) {
	handler, store := newTestHandler(t)
	user, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Admin",
		Email:       "admin@example.com",
		Password:    "supersecret",
		Roles:       []string{"admin"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	payload, _ := json.Marshal(loginRequest{Email: "admin@example.com", Password: "supersecret"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(payload))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp mfaChallengeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.MFARequired {
		t.Fatal("expected MFA to be required")
	}
	if resp.MFAToken == "" {
		t.Fatal("expected MFA token")
	}
	if resp.Enrollment == nil || len(resp.Enrollment.RecoveryCodes) == 0 {
		t.Fatal("expected enrollment recovery codes")
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("did not expect session cookie during MFA challenge")
	}

	verifyPayload, _ := json.Marshal(mfaVerifyRequest{
		Token: resp.MFAToken,
		Code:  resp.Enrollment.RecoveryCodes[0],
	})
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/verify", bytes.NewReader(verifyPayload))
	verifyRec := httptest.NewRecorder()

	handler.MFAVerify(verifyRec, verifyReq)

	if verifyRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", verifyRec.Code)
	}
	if findCookie(t, verifyRec.Result().Cookies(), "bitriver_session").Value == "" {
		t.Fatal("expected session cookie after MFA verification")
	}

	settings, exists, err := store.GetMFASettings(user.ID)
	if err != nil {
		t.Fatalf("GetMFASettings error: %v", err)
	}
	if !exists || !settings.Enabled {
		t.Fatal("expected MFA settings to be enabled")
	}
}

func TestMFAVerifyUsesChallengeCookieWhenTokenOmitted(t *testing.T) {
	handler, store := newTestHandler(t)
	user, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Admin",
		Email:       "admin2@example.com",
		Password:    "supersecret",
		Roles:       []string{"admin"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	token, _, err := handler.mfaChallengeManager().Create(user.ID)
	if err != nil {
		t.Fatalf("Create challenge: %v", err)
	}

	enrollPayload, _ := json.Marshal(mfaEnrollRequest{})
	enrollReq := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/enroll", bytes.NewReader(enrollPayload))
	enrollReq.AddCookie(&http.Cookie{Name: mfaChallengeCookieName, Value: token, Path: "/api/auth/mfa"})
	enrollRec := httptest.NewRecorder()
	handler.MFAEnroll(enrollRec, enrollReq)
	if enrollRec.Code != http.StatusOK {
		t.Fatalf("expected enroll status 200, got %d", enrollRec.Code)
	}
	var enrollment mfaEnrollmentResponse
	if err := json.NewDecoder(enrollRec.Body).Decode(&enrollment); err != nil {
		t.Fatalf("decode enrollment: %v", err)
	}

	verifyPayload, _ := json.Marshal(mfaVerifyRequest{Code: enrollment.RecoveryCodes[0]})
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/verify", bytes.NewReader(verifyPayload))
	verifyReq.AddCookie(&http.Cookie{Name: mfaChallengeCookieName, Value: token, Path: "/api/auth/mfa"})
	verifyRec := httptest.NewRecorder()
	handler.MFAVerify(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("expected verify status 200, got %d", verifyRec.Code)
	}
	if findCookie(t, verifyRec.Result().Cookies(), sessionCookieName).Value == "" {
		t.Fatal("expected session cookie after MFA verification")
	}
}
