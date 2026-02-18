package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"bitriver-live/internal/api"
)

const (
	csrfCookieName  = "bitriver_csrf"
	csrfHeaderName  = "X-CSRF-Token"
	csrfTokenLength = 32
)

// csrfExemptPaths is the explicit allowlist of machine-to-machine endpoints
// that are protected by independent shared-secret signatures/tokens.
var csrfExemptPaths = []string{
	"/api/ingest/srs-hook",
	"/api/payments/webhooks/",
}

func csrfMiddleware(handler *api.Handler, logger *slog.Logger, resolver *clientIPResolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !csrfShouldInspect(r) {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := api.UserFromContext(r.Context()); !ok {
			next.ServeHTTP(w, r)
			return
		}
		if csrfPathExempt(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if !cookieAuthWithoutBearer(r) {
			next.ServeHTTP(w, r)
			return
		}

		csrfCookie, err := r.Cookie(csrfCookieName)
		if err != nil || strings.TrimSpace(csrfCookie.Value) == "" {
			csrfCookie = setCSRFCookie(w)
		}

		provided := strings.TrimSpace(r.Header.Get(csrfHeaderName))
		if subtle.ConstantTimeCompare([]byte(provided), []byte(csrfCookie.Value)) != 1 {
			if requestLogger := loggingWithRequest(logger, resolver, r); requestLogger != nil {
				requestLogger.Warn("csrf token validation failed", "path", r.URL.Path)
			}
			api.WriteError(w, http.StatusForbidden, fmt.Errorf("invalid csrf token"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func csrfShouldInspect(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func csrfPathExempt(path string) bool {
	for _, exempt := range csrfExemptPaths {
		if strings.HasSuffix(exempt, "/") {
			if strings.HasPrefix(path, exempt) {
				return true
			}
			continue
		}
		if path == exempt {
			return true
		}
	}
	return false
}

func cookieAuthWithoutBearer(r *http.Request) bool {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return false
	}
	cookie, err := r.Cookie("bitriver_session")
	if err != nil {
		return false
	}
	return strings.TrimSpace(cookie.Value) != ""
}

func setCSRFCookie(w http.ResponseWriter) *http.Cookie {
	token, err := generateCSRFToken()
	if err != nil {
		token = ""
	}
	cookie := &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
	return cookie
}

func generateCSRFToken() (string, error) {
	buf := make([]byte, csrfTokenLength)
	if _, err := rand.Read(buf); err != nil {
		return "", errors.New("generate csrf token")
	}
	return hex.EncodeToString(buf), nil
}
