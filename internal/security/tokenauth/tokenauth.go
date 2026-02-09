package tokenauth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// BearerHeaderStatus describes bearer-header parsing results.
type BearerHeaderStatus int

const (
	BearerHeaderMissing BearerHeaderStatus = iota
	BearerHeaderInvalid
	BearerHeaderTokenMissing
	BearerHeaderValid
)

// ConstantTimeEqual compares non-empty strings using constant-time comparison.
func ConstantTimeEqual(expected, provided string) bool {
	if expected == "" || provided == "" {
		return false
	}
	if len(expected) != len(provided) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}

// ParseBearerHeader parses an Authorization header and returns the bearer token when present.
func ParseBearerHeader(header string) (string, BearerHeaderStatus) {
	if strings.TrimSpace(header) == "" {
		return "", BearerHeaderMissing
	}
	header = strings.TrimLeft(header, " \t\r\n")
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", BearerHeaderInvalid
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", BearerHeaderTokenMissing
	}
	return token, BearerHeaderValid
}

// QueryToken returns a trimmed token query parameter value.
func QueryToken(r *http.Request, key string) (string, bool) {
	if r == nil || r.URL == nil {
		return "", false
	}
	token := strings.TrimSpace(r.URL.Query().Get(key))
	if token == "" {
		return "", false
	}
	return token, true
}
