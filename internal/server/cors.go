package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// CORSConfig declares the origins allowed to access the API across domains.
// AdminOrigins authorise requests from the control centre, while ViewerOrigins
// cover the public viewer UI. When both lists are empty, only same-origin
// requests are permitted.
type CORSConfig struct {
	AdminOrigins  []string
	ViewerOrigins []string
}

type corsPolicy struct {
	allowed map[string]struct{}
}

func newCORSPolicy(cfg CORSConfig) (corsPolicy, error) {
	policy := corsPolicy{allowed: make(map[string]struct{})}
	origins := append([]string{}, cfg.AdminOrigins...)
	origins = append(origins, cfg.ViewerOrigins...)
	for _, origin := range origins {
		normalized, err := normalizeOrigin(origin)
		if err != nil {
			return corsPolicy{}, fmt.Errorf("parse origin %q: %w", origin, err)
		}
		if normalized != "" {
			policy.allowed[normalized] = struct{}{}
		}
	}
	return policy, nil
}

func normalizeOrigin(origin string) (string, error) {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return "", nil
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("origin must include scheme and host")
	}
	return fmt.Sprintf("%s://%s", strings.ToLower(parsed.Scheme), strings.ToLower(parsed.Host)), nil
}

func corsMiddleware(policy corsPolicy, logger *slog.Logger, resolver *clientIPResolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		reqOrigin := originForRequest(r, resolver)
		if !policy.allows(origin, reqOrigin) {
			if logger != nil {
				logger.Warn("blocked CORS origin", "origin", origin, "path", r.URL.Path)
			}
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")

		if r.Method == http.MethodOptions {
			requestedMethod := r.Header.Get("Access-Control-Request-Method")
			if requestedMethod == "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			requestedHeaders := r.Header.Get("Access-Control-Request-Headers")
			if requestedHeaders != "" {
				w.Header().Set("Access-Control-Allow-Headers", requestedHeaders)
			} else {
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (p corsPolicy) allows(origin string, requestOrigin string) bool {
	normalizedOrigin, err := normalizeOrigin(origin)
	if err != nil {
		return false
	}
	if normalizedOrigin == "" {
		return false
	}
	if _, ok := p.allowed[normalizedOrigin]; ok {
		return true
	}

	if requestOrigin == "" {
		return false
	}

	return normalizedOrigin == requestOrigin
}

func originForRequest(r *http.Request, resolver *clientIPResolver) string {
	host := strings.ToLower(strings.TrimSpace(r.Host))
	if host == "" {
		return ""
	}

	scheme := requestScheme(r, resolver)
	if scheme == "" {
		return ""
	}

	return fmt.Sprintf("%s://%s", scheme, host)
}

func requestScheme(r *http.Request, resolver *clientIPResolver) string {
	if resolver != nil && resolver.shouldTrust(r.RemoteAddr) {
		if proto := forwardedProto(r); proto != "" {
			return proto
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func forwardedProto(r *http.Request) string {
	if r == nil {
		return ""
	}
	if proto := headerForwardedProto(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		return proto
	}
	values := r.Header.Values("Forwarded")
	for _, value := range values {
		if proto := forwardedProtoFromHeader(value); proto != "" {
			return proto
		}
	}
	return ""
}

func headerForwardedProto(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.Split(header, ",")
	if len(parts) == 0 {
		return ""
	}
	proto := strings.ToLower(strings.TrimSpace(parts[0]))
	if proto == "http" || proto == "https" {
		return proto
	}
	return ""
}

func forwardedProtoFromHeader(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	entries := strings.Split(header, ",")
	for _, entry := range entries {
		params := strings.Split(entry, ";")
		for _, param := range params {
			param = strings.TrimSpace(param)
			if param == "" {
				continue
			}
			kv := strings.SplitN(param, "=", 2)
			if len(kv) != 2 {
				continue
			}
			if strings.ToLower(strings.TrimSpace(kv[0])) != "proto" {
				continue
			}
			value := strings.Trim(strings.TrimSpace(kv[1]), "\"")
			value = strings.ToLower(value)
			if value == "http" || value == "https" {
				return value
			}
		}
	}
	return ""
}
