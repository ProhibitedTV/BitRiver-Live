package api

import (
	"net/http"
	"strings"

	"bitriver-live/internal/security/tokenauth"
)

// srsHookAuthorized performs srs hook authorized and propagates validation or dependency failures to the caller.
func (h *Handler) srsHookAuthorized(r *http.Request) bool {
	token := strings.TrimSpace(h.SRSHookToken)
	if token == "" || r == nil {
		return false
	}

	if bearerToken, status := tokenauth.ParseBearerHeader(r.Header.Get("Authorization")); status == tokenauth.BearerHeaderValid {
		if tokenauth.ConstantTimeEqual(token, bearerToken) {
			return true
		}
	}

	if queryToken, ok := tokenauth.QueryToken(r, "token"); ok {
		if tokenauth.ConstantTimeEqual(token, queryToken) {
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
