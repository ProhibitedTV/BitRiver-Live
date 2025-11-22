package api

import (
	"net/http"
	"strings"
	"time"
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

func DefaultSessionCookiePolicy() SessionCookiePolicy {
	return SessionCookiePolicy{
		SameSite:   http.SameSiteStrictMode,
		SecureMode: SessionCookieSecureAuto,
	}
}

func (p SessionCookiePolicy) secure(r *http.Request) bool {
	if p.SecureMode == SessionCookieSecureAlways {
		return true
	}
	return isSecureRequest(r)
}

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

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time, policy SessionCookiePolicy) {
	if token == "" {
		return
	}
	maxAge := int(time.Until(expires).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "bitriver_session",
		Value:    token,
		Path:     "/",
		Expires:  expires.UTC(),
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   policy.secure(r),
		SameSite: policy.SameSite,
	})
}

func (h *Handler) setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	setSessionCookie(w, r, token, expires, h.sessionCookiePolicy())
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request, policy SessionCookiePolicy) {
	http.SetCookie(w, &http.Cookie{
		Name:     "bitriver_session",
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
