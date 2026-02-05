package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// constantTimeEqual performs constant time equal and propagates validation or dependency failures to the caller.
func constantTimeEqual(expected, provided string) bool {
	if expected == "" || provided == "" {
		return false
	}
	if len(expected) != len(provided) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}

// srsHookAuthorized performs srs hook authorized and propagates validation or dependency failures to the caller.
func (h *Handler) srsHookAuthorized(r *http.Request) bool {
	token := strings.TrimSpace(h.SRSHookToken)
	if token == "" || r == nil {
		return false
	}

	if authHeader := strings.TrimSpace(r.Header.Get("Authorization")); authHeader != "" {
		if parts := strings.SplitN(authHeader, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			if constantTimeEqual(token, strings.TrimSpace(parts[1])) {
				return true
			}
		}
	}

	if queryToken := strings.TrimSpace(r.URL.Query().Get("token")); queryToken != "" {
		if constantTimeEqual(token, queryToken) {
			return true
		}
	}

	return false
}

// srsRenditions performs srs renditions and propagates validation or dependency failures to the caller.
func (h *Handler) srsRenditions() []string {
	if len(h.DefaultRenditions) == 0 {
		return []string{"1080p", "720p", "480p"}
	}
	rends := make([]string, len(h.DefaultRenditions))
	copy(rends, h.DefaultRenditions)
	return rends
}
