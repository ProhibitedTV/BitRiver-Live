package api

import (
	"net/http"
	"strings"
	"time"
)

const (
	sessionCookieName      = "bitriver_session"
	mfaChallengeCookieName = "bitriver_mfa_challenge"
	mfaChallengeCookieTTL  = 10 * time.Minute
)

type SessionCookieSecureMode int

const (
	SessionCookieSecureAuto SessionCookieSecureMode = iota
	SessionCookieSecureAlways
)

type SessionCookiePolicy struct {
	SameSite   http.SameSite
	SecureMode SessionCookieSecureMode
}

// DefaultSessionCookiePolicy returns the default session cookie policy for the current runtime mode.
func DefaultSessionCookiePolicy() SessionCookiePolicy {
	return SessionCookiePolicy{
		SameSite:   http.SameSiteStrictMode,
		SecureMode: SessionCookieSecureAuto,
	}
}

// secure performs secure and propagates validation or dependency failures to the caller.
func (p SessionCookiePolicy) secure(r *http.Request) bool {
	if p.SecureMode == SessionCookieSecureAlways {
		return true
	}
	return isSecureRequest(r)
}

// sessionCookiePolicy performs session cookie policy and propagates validation or dependency failures to the caller.
func (h *Handler) sessionCookiePolicy() SessionCookiePolicy {
	policy := h.SessionCookiePolicy
	if policy.SameSite == 0 {
		policy.SameSite = http.SameSiteStrictMode
	}
	if policy.SecureMode == 0 {
		policy.SecureMode = SessionCookieSecureAuto
	}
	return policy
}

// setSessionCookie parses and stores a flag assignment, returning an error when the format is invalid.
func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time, policy SessionCookiePolicy) {
	if token == "" {
		return
	}
	maxAge := int(time.Until(expires).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires.UTC(),
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   policy.secure(r),
		SameSite: policy.SameSite,
	})
}

// setSessionCookie parses and stores a flag assignment, returning an error when the format is invalid.
func (h *Handler) setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	setSessionCookie(w, r, token, expires, h.sessionCookiePolicy())
}

// RefreshSessionCookie updates the session cookie using the handler's configured policy.
// It is intended for middleware that extends session lifetimes when idle timeouts are enabled.
func (h *Handler) RefreshSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	h.setSessionCookie(w, r, token, expires)
}

// clearSessionCookie performs clear session cookie and propagates validation or dependency failures to the caller.
func clearSessionCookie(w http.ResponseWriter, r *http.Request, policy SessionCookiePolicy) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   policy.secure(r),
		SameSite: policy.SameSite,
	})
}

// ClearSessionCookie removes the BitRiver session cookie from the response.
func ClearSessionCookie(w http.ResponseWriter, r *http.Request, policy SessionCookiePolicy) {
	clearSessionCookie(w, r, policy)
}

// ClearSessionCookie removes the BitRiver session cookie from the response using the handler's configured policy.
func (h *Handler) ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w, r, h.sessionCookiePolicy())
}

// setMFAChallengeCookie stores a short-lived cookie carrying the MFA challenge token.
func (h *Handler) setMFAChallengeCookie(w http.ResponseWriter, r *http.Request, token string) {
	if token == "" {
		return
	}
	expires := time.Now().Add(mfaChallengeCookieTTL).UTC()
	http.SetCookie(w, &http.Cookie{
		Name:     mfaChallengeCookieName,
		Value:    token,
		Path:     "/api/auth/mfa",
		Expires:  expires,
		MaxAge:   int(mfaChallengeCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.sessionCookiePolicy().secure(r),
		SameSite: h.sessionCookiePolicy().SameSite,
	})
}

// clearMFAChallengeCookie removes the MFA challenge cookie from the response.
func (h *Handler) clearMFAChallengeCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     mfaChallengeCookieName,
		Value:    "",
		Path:     "/api/auth/mfa",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.sessionCookiePolicy().secure(r),
		SameSite: h.sessionCookiePolicy().SameSite,
	})
}

// mfaChallengeTokenFromRequest extracts the MFA challenge token from body token or cookie.
func mfaChallengeTokenFromRequest(r *http.Request, bodyToken string) string {
	if token := strings.TrimSpace(bodyToken); token != "" {
		return token
	}
	if r == nil {
		return ""
	}
	cookie, err := r.Cookie(mfaChallengeCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

// isSecureRequest reports whether secure request is satisfied for the current input.
func isSecureRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		for _, p := range strings.Split(proto, ",") {
			if strings.EqualFold(strings.TrimSpace(p), "https") {
				return true
			}
		}
	}
	if r.URL != nil && strings.EqualFold(r.URL.Scheme, "https") {
		return true
	}
	return false
}
