package pgdsn

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var sslModeDisablePattern = regexp.MustCompile(`(?i)(^|[?&\s;])sslmode=disable([&#;\s]|$)`)

// SSLModeDisable reports whether the DSN explicitly sets sslmode=disable.
func SSLModeDisable(dsn string) bool {
	return sslModeDisablePattern.MatchString(dsn)
}

// IsComposePostgresDSN reports whether the DSN points to the local Compose postgres service.
func IsComposePostgresDSN(dsn string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(dsn))
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "host=postgres") {
		return true
	}
	if strings.HasPrefix(trimmed, "postgres://") || strings.HasPrefix(trimmed, "postgresql://") {
		parsed, err := url.Parse(trimmed)
		if err == nil && strings.EqualFold(parsed.Hostname(), "postgres") {
			return true
		}
	}
	return false
}

// ValidateTLSPolicy validates Postgres TLS policy for a DSN tied to an environment variable.
func ValidateTLSPolicy(dsn, envVar string) error {
	if dsn == "" {
		return nil
	}
	if SSLModeDisable(dsn) && !IsComposePostgresDSN(dsn) {
		return fmt.Errorf("%s must enable TLS (set sslmode=require or provide a CA with verify-full); sslmode=disable is only allowed for the local Compose postgres service", envVar)
	}
	return nil
}
