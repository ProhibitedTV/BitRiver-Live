package server

import (
	"net/http"
	"strings"
)

const (
	defaultFrameAncestors     = "'none'"
	defaultFrameOptions       = "DENY"
	defaultReferrerPolicy     = "no-referrer"
	defaultPermissionsPolicy  = "camera=(), microphone=(), geolocation=()"
	defaultContentTypeOptions = "nosniff"
)

// SecurityConfig controls the HTTP response headers that harden the server
// against clickjacking, MIME sniffing, referrer leakage, and unintended
// resource loading. Zero-valued fields fall back to safe defaults; override the
// ContentSecurityPolicy directive when embedding the app in a trusted host.
type SecurityConfig struct {
	ContentSecurityPolicy string
	FrameAncestors        string
	FrameOptions          string
	ReferrerPolicy        string
	PermissionsPolicy     string
	ContentTypeOptions    string
}

// defaultSecurityConfig returns the default security-header configuration.
func defaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		ContentSecurityPolicy: defaultContentSecurityPolicy(defaultFrameAncestors),
		FrameAncestors:        defaultFrameAncestors,
		FrameOptions:          defaultFrameOptions,
		ReferrerPolicy:        defaultReferrerPolicy,
		PermissionsPolicy:     defaultPermissionsPolicy,
		ContentTypeOptions:    defaultContentTypeOptions,
	}
}

// withDefaults applies default header values for unset fields.
func (cfg SecurityConfig) withDefaults() SecurityConfig {
	defaults := defaultSecurityConfig()

	if cfg.FrameAncestors == "" {
		cfg.FrameAncestors = defaults.FrameAncestors
	}
	if cfg.FrameOptions == "" {
		cfg.FrameOptions = defaults.FrameOptions
	}
	if cfg.ReferrerPolicy == "" {
		cfg.ReferrerPolicy = defaults.ReferrerPolicy
	}
	if cfg.PermissionsPolicy == "" {
		cfg.PermissionsPolicy = defaults.PermissionsPolicy
	}
	if cfg.ContentTypeOptions == "" {
		cfg.ContentTypeOptions = defaults.ContentTypeOptions
	}

	return cfg
}

// defaultContentSecurityPolicy builds the default Content-Security-Policy value.
func defaultContentSecurityPolicy(frameAncestors string) string {
	value := frameAncestors
	if value == "" {
		value = defaultFrameAncestors
	}

	return "default-src 'self'; " +
		"connect-src 'self'; " +
		"img-src 'self' data:; " +
		"script-src 'self'; " +
		"style-src 'self'; " +
		"font-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"frame-ancestors " + value + "; " +
		"form-action 'self'"
}

func defaultViewerContentSecurityPolicy(frameAncestors string) string {
	value := frameAncestors
	if value == "" {
		value = defaultFrameAncestors
	}

	return "default-src 'self'; " +
		"connect-src 'self'; " +
		"img-src 'self' data:; " +
		"script-src 'self' 'unsafe-inline'; " +
		"style-src 'self'; " +
		"font-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"frame-ancestors " + value + "; " +
		"form-action 'self'"
}

func isViewerProxyPath(path string) bool {
	return path == "/viewer" || strings.HasPrefix(path, "/viewer/")
}

func contentSecurityPolicyForRequest(cfg SecurityConfig, path string) string {
	if cfg.ContentSecurityPolicy != "" {
		return cfg.ContentSecurityPolicy
	}
	if isViewerProxyPath(path) {
		return defaultViewerContentSecurityPolicy(cfg.FrameAncestors)
	}
	return defaultContentSecurityPolicy(cfg.FrameAncestors)
}

// securityHeadersMiddleware sets security headers before invoking the next handler.
func securityHeadersMiddleware(cfg SecurityConfig, next http.Handler) http.Handler {
	effective := cfg.withDefaults()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if policy := contentSecurityPolicyForRequest(effective, r.URL.Path); policy != "" {
			w.Header().Set("Content-Security-Policy", policy)
		}
		if effective.FrameOptions != "" {
			w.Header().Set("X-Frame-Options", effective.FrameOptions)
		}
		if effective.ContentTypeOptions != "" {
			w.Header().Set("X-Content-Type-Options", effective.ContentTypeOptions)
		}
		if effective.ReferrerPolicy != "" {
			w.Header().Set("Referrer-Policy", effective.ReferrerPolicy)
		}
		if effective.PermissionsPolicy != "" {
			w.Header().Set("Permissions-Policy", effective.PermissionsPolicy)
		}

		next.ServeHTTP(w, r)
	})
}
