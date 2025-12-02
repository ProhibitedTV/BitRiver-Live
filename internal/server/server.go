package server

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"bitriver-live/internal/api"
	"bitriver-live/internal/auth/oauth"
	"bitriver-live/internal/observability/logging"
	"bitriver-live/internal/observability/metrics"
	"bitriver-live/web"
)

// TLSConfig defines certificate files that enable TLS for the HTTP listener
// created by Server. When both CertFile and KeyFile are provided the server
// starts with TLS; otherwise it falls back to plain HTTP on Config.Addr.
type TLSConfig struct {
	CertFile string
	KeyFile  string
}

// MetricsAccessConfig defines the authentication and network allowlist used to
// guard the Prometheus scrape endpoint.
type MetricsAccessConfig struct {
	Token           string
	AllowedNetworks []string
}

// Config aggregates the dependencies and settings required to construct a
// Server. Addr determines the listen address for the HTTP server, TLS controls
// whether HTTPS is enabled, RateLimit configures per-client throttling, CORS
// whitelists cross-site admin and viewer origins, Security sets the HTTP
// hardening headers, Logger and AuditLogger provide structured logging, Metrics
// records request metrics (defaulting to metrics.Default when nil), MetricsAccess
// restricts the Prometheus scrape endpoint, ViewerOrigin configures reverse
// proxying for viewer traffic, OAuth is injected into the supplied API handler,
// SessionCookieSecureMode forces HTTPS-only session cookies when set to
// SessionCookieSecureAlways, and SessionCookieCrossSite enables SameSite=None
// cookies for cross-site viewer deployments.
type Config struct {
	Addr                    string
	TLS                     TLSConfig
	RateLimit               RateLimitConfig
	CORS                    CORSConfig
	Security                SecurityConfig
	Logger                  *slog.Logger
	AuditLogger             *slog.Logger
	Metrics                 *metrics.Recorder
	MetricsAccess           MetricsAccessConfig
	ViewerOrigin            *url.URL
	OAuth                   oauth.Service
	AllowSelfSignup         *bool
	SessionCookieSecureMode api.SessionCookieSecureMode
	SessionCookieCrossSite  bool
	SRSHookToken            string
}

// Server wraps the configured http.Server alongside observability, rate
// limiting, and TLS metadata derived from Config. It exposes lifecycle methods
// for starting and gracefully shutting down the listener created by New.
type Server struct {
	httpServer  *http.Server
	logger      *slog.Logger
	auditLogger *slog.Logger
	metrics     *metrics.Recorder
	rateLimiter *rateLimiter
	ipResolver  *clientIPResolver
	tlsCertFile string
	tlsKeyFile  string
}

// New wires the HTTP router, middlewares, and instrumentation required for the
// BitRiver API. It registers health, metrics, authentication, user, channel,
// directory, profile, chat, recording, upload, moderation, and analytics
// endpoints on a mux alongside static asset and optional viewer proxy handlers.
// The supplied Config drives listener address selection, TLS activation,
// logging, auditing, rate limiting, and metrics recording (falling back to
// metrics.Default when Metrics is nil). The handler's OAuth field is populated
// from Config before being used by auth middleware, and the resulting Server
// retains references for lifecycle management.
func New(handler *api.Handler, cfg Config) (*Server, error) {
	if handler == nil {
		return nil, errors.New("handler is required")
	}

	if cfg.Logger != nil {
		handler.Logger = cfg.Logger
	}

	corsPolicy, err := newCORSPolicy(cfg.CORS)
	if err != nil {
		return nil, fmt.Errorf("configure CORS: %w", err)
	}

	recorder := cfg.Metrics
	if recorder == nil {
		recorder = metrics.Default()
	}
	handler.OAuth = cfg.OAuth
	if cfg.AllowSelfSignup != nil {
		handler.AllowSelfSignup = *cfg.AllowSelfSignup
	}
	handler.SRSHookToken = cfg.SRSHookToken
	handler.SessionCookiePolicy = api.DefaultSessionCookiePolicy()
	if cfg.SessionCookieSecureMode != 0 {
		handler.SessionCookiePolicy.SecureMode = cfg.SessionCookieSecureMode
	}
	if cfg.SessionCookieCrossSite {
		handler.SessionCookiePolicy = api.SessionCookiePolicy{
			SameSite:   http.SameSiteNoneMode,
			SecureMode: api.SessionCookieSecureAlways,
		}
	}

	rl, err := newRateLimiter(cfg.RateLimit)
	if err != nil {
		return nil, fmt.Errorf("configure rate limiter: %w", err)
	}
	handler.RateLimiter = rl
	ipResolver, err := newClientIPResolver(cfg.RateLimit)
	if err != nil {
		return nil, fmt.Errorf("configure client ip resolver: %w", err)
	}
	metricsAccess, err := newMetricsAccessController(cfg.MetricsAccess, ipResolver, cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("configure metrics access: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.Health)
	mux.HandleFunc("/readyz", handler.Ready)
	metricsHandler := recorder.Handler()
	metricsHandler = metricsAccess.handler(metricsHandler)
	mux.Handle("/metrics", metricsHandler)
	mux.HandleFunc("/api/auth/signup", handler.Signup)
	mux.HandleFunc("/api/auth/login", handler.Login)
	mux.HandleFunc("/api/auth/oauth/providers", handler.OAuthProviders)
	mux.HandleFunc("/api/auth/oauth/", handler.OAuthByProvider)
	mux.HandleFunc("/api/auth/session", handler.Session)
	mux.HandleFunc("/api/users", handler.Users)
	mux.HandleFunc("/api/users/", handler.UserByID)
	mux.HandleFunc("/api/directory", handler.Directory)
	mux.HandleFunc("/api/directory/featured", handler.DirectoryFeatured)
	mux.HandleFunc("/api/directory/recommended", handler.DirectoryRecommended)
	mux.HandleFunc("/api/directory/following", handler.DirectoryFollowing)
	mux.HandleFunc("/api/directory/live", handler.DirectoryLive)
	mux.HandleFunc("/api/directory/trending", handler.DirectoryTrending)
	mux.HandleFunc("/api/directory/categories", handler.DirectoryCategories)
	mux.HandleFunc("/api/channels", handler.Channels)
	mux.HandleFunc("/api/channels/", handler.ChannelByID)
	mux.HandleFunc("/api/profiles", handler.Profiles)
	mux.HandleFunc("/api/profiles/", handler.ProfileByID)
	mux.HandleFunc("/api/chat/ws", handler.ChatWebsocket)
	mux.HandleFunc("/api/recordings", handler.Recordings)
	mux.HandleFunc("/api/recordings/", handler.RecordingByID)
	mux.HandleFunc("/api/uploads", handler.Uploads)
	mux.HandleFunc("/api/uploads/", handler.UploadByID)
	mux.HandleFunc("/api/moderation/queue", handler.ModerationQueue)
	mux.HandleFunc("/api/moderation/queue/", handler.ModerationQueueByID)
	mux.HandleFunc("/api/analytics/overview", handler.AnalyticsOverview)
	mux.HandleFunc("/api/ingest/srs-hook", handler.SRSHook)

	staticFS, err := web.Static()
	if err != nil {
		return nil, fmt.Errorf("load web assets: %w", err)
	}
	index, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		return nil, fmt.Errorf("read web index: %w", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	if cfg.ViewerOrigin != nil {
		viewerProxy := httputil.NewSingleHostReverseProxy(cfg.ViewerOrigin)
		viewerProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			if requestLogger := loggerWithRequestContext(r.Context(), cfg.Logger); requestLogger != nil {
				requestLogger.Error("viewer proxy error", "error", err, "path", r.URL.Path)
			}
			writeMiddlewareError(w, http.StatusBadGateway, "viewer temporarily unavailable")
		}
		viewerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			viewerProxy.ServeHTTP(w, r)
		})
		mux.Handle("/viewer", viewerHandler)
		mux.Handle("/viewer/", viewerHandler)
	}

	mux.HandleFunc("/", spaHandler(staticFS, index, fileServer))

	handlerChain := http.Handler(mux)
	handlerChain = corsMiddleware(corsPolicy, cfg.Logger, handlerChain)
	securityCfg := cfg.Security.withDefaults()
	handlerChain = securityHeadersMiddleware(securityCfg, handlerChain)
	handlerChain = requestIDMiddleware(cfg.Logger, handlerChain)
	handlerChain = authMiddleware(handler, handlerChain)
	handlerChain = rateLimitMiddleware(rl, ipResolver, cfg.Logger, handlerChain)
	handlerChain = metrics.HTTPMiddleware(recorder, handlerChain)
	handlerChain = auditMiddleware(cfg.AuditLogger, ipResolver, handlerChain)
	handlerChain = loggingMiddleware(cfg.Logger, ipResolver, handlerChain)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handlerChain,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	srv := &Server{
		httpServer:  httpServer,
		logger:      cfg.Logger,
		auditLogger: cfg.AuditLogger,
		metrics:     recorder,
		rateLimiter: rl,
		ipResolver:  ipResolver,
		tlsCertFile: strings.TrimSpace(cfg.TLS.CertFile),
		tlsKeyFile:  strings.TrimSpace(cfg.TLS.KeyFile),
	}

	if srv.tlsCertFile != "" && srv.tlsKeyFile != "" {
		httpServer.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	return srv, nil
}

func (s *Server) Start() error {
	if s.httpServer == nil {
		return fmt.Errorf("http server is not configured")
	}

	if s.tlsCertFile != "" && s.tlsKeyFile != "" {
		return s.httpServer.ListenAndServeTLS(s.tlsCertFile, s.tlsKeyFile)
	}

	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func loggingMiddleware(logger *slog.Logger, resolver *clientIPResolver, next http.Handler) http.Handler {
	return logging.RequestLogger(logging.RequestLoggerConfig{
		Logger:            logger,
		DisableRemoteAddr: true,
		AdditionalFields: func(r *http.Request, _ int, _ time.Duration) []any {
			ip, source := resolveClientIP(r, resolver)
			if ip == "" && source == "" {
				return nil
			}
			return []any{"remote_ip", ip, "ip_source", source}
		},
	})(next)
}

type metricsAccessController struct {
	token    string
	networks []*net.IPNet
	resolver *clientIPResolver
	logger   *slog.Logger
}

func newMetricsAccessController(cfg MetricsAccessConfig, resolver *clientIPResolver, logger *slog.Logger) (*metricsAccessController, error) {
	networks, err := parseNetworks(cfg.AllowedNetworks, "metrics network")
	if err != nil {
		return nil, err
	}
	return &metricsAccessController{
		token:    strings.TrimSpace(cfg.Token),
		networks: networks,
		resolver: resolver,
		logger:   logger,
	}, nil
}

func (m *metricsAccessController) handler(next http.Handler) http.Handler {
	if m == nil || (m.token == "" && len(m.networks) == 0) {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, source := resolveClientIP(r, m.resolver)

		if m.token != "" && subtle.ConstantTimeCompare([]byte(m.token), []byte(metricsTokenFromRequest(r))) == 1 {
			next.ServeHTTP(w, r)
			return
		}

		if len(m.networks) > 0 && ipAllowed(ip, m.networks) {
			next.ServeHTTP(w, r)
			return
		}

		if requestLogger := loggerWithRequestContext(r.Context(), m.logger); requestLogger != nil {
			requestLogger.Warn("metrics access denied", "remote_ip", ip, "ip_source", source)
		}
		writeMiddlewareError(w, http.StatusForbidden, "metrics access denied")
	})
}

func metricsTokenFromRequest(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("bearer "):])
	}

	if token := strings.TrimSpace(r.Header.Get("X-Metrics-Token")); token != "" {
		return token
	}

	return ""
}

func ipAllowed(ip string, networks []*net.IPNet) bool {
	if ip == "" {
		return false
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, network := range networks {
		if network.Contains(parsed) {
			return true
		}
	}
	return false
}

func rateLimitMiddleware(rl *rateLimiter, resolver *clientIPResolver, logger *slog.Logger, next http.Handler) http.Handler {
	if rl == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.AllowRequest() {
			writeMiddlewareError(w, http.StatusTooManyRequests, "global rate limit exceeded")
			return
		}
		if shouldRateLimitAuthRequest(r) {
			ip, source := resolveClientIP(r, resolver)
			requestLogger := loggerWithRequestContext(r.Context(), logger)
			allowed, retryAfter, err := rl.AllowLogin(ip)
			if err != nil {
				if requestLogger != nil {
					requestLogger.Error("rate limiter failure", "error", err, "remote_ip", ip, "ip_source", source)
				}
				writeMiddlewareError(w, http.StatusServiceUnavailable, "rate limit failure")
				return
			}
			if !allowed {
				if requestLogger != nil {
					requestLogger.Warn("login rate limited", "remote_ip", ip, "ip_source", source)
				}
				if retryAfter > 0 {
					w.Header().Set("Retry-After", fmt.Sprintf("%.0f", retryAfter.Seconds()))
				}
				writeMiddlewareError(w, http.StatusTooManyRequests, "too many login attempts")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func shouldRateLimitAuthRequest(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}
	switch r.URL.Path {
	case "/api/auth/login", "/api/auth/signup":
		return r.Method == http.MethodPost
	case "/api/auth/session":
		return r.Method == http.MethodGet || r.Method == http.MethodDelete
	}

	if strings.HasPrefix(r.URL.Path, "/api/auth/oauth/") {
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/auth/oauth/")
		parts := strings.Split(strings.Trim(trimmed, "/"), "/")
		if len(parts) >= 2 {
			action := parts[1]
			switch action {
			case "start":
				return r.Method == http.MethodPost
			case "callback":
				return r.Method == http.MethodGet
			}
		}
	}

	return false
}

func auditMiddleware(logger *slog.Logger, resolver *clientIPResolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sr := metrics.NewResponseRecorder(w)
		start := time.Now()
		next.ServeHTTP(sr, r)
		if !shouldAudit(r) {
			return
		}
		duration := time.Since(start)
		user, ok := api.UserFromContext(r.Context())
		ip, source := resolveClientIP(r, resolver)
		requestLogger := loggerWithRequestContext(r.Context(), logger)
		if requestLogger == nil {
			return
		}
		fields := []interface{}{
			"method", r.Method,
			"path", r.URL.Path,
			"status", sr.Status(),
			"duration_ms", duration.Milliseconds(),
			"remote_ip", ip,
			"ip_source", source,
		}
		if ok {
			fields = append(fields, "user_id", user.ID)
		}
		requestLogger.Info("audit", fields...)
	})
}

func shouldAudit(r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return false
	}
	switch {
	case strings.HasPrefix(r.URL.Path, "/api/"):
		return true
	default:
		return false
	}
}

const (
	ipSourceRemoteAddr    = "remote_addr"
	ipSourceXForwardedFor = "x_forwarded_for"
	ipSourceXRealIP       = "x_real_ip"
)

func parseNetworks(raw []string, descriptor string) ([]*net.IPNet, error) {
	var networks []*net.IPNet
	for _, value := range raw {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, network, err := net.ParseCIDR(trimmed); err == nil {
			networks = append(networks, network)
			continue
		}
		ip := net.ParseIP(trimmed)
		if ip == nil {
			return nil, fmt.Errorf("parse %s %q: invalid address", descriptor, trimmed)
		}
		maskSize := 128
		if ip.To4() != nil {
			maskSize = 32
		}
		networks = append(networks, &net.IPNet{IP: ip, Mask: net.CIDRMask(maskSize, maskSize)})
	}
	return networks, nil
}

type clientIPResolver struct {
	trustForwarded bool
	trustedNets    []*net.IPNet
}

func newClientIPResolver(cfg RateLimitConfig) (*clientIPResolver, error) {
	resolver := &clientIPResolver{trustForwarded: cfg.TrustForwardedHeaders}
	trusted, err := parseNetworks(cfg.TrustedProxies, "trusted proxy")
	if err != nil {
		return nil, err
	}
	resolver.trustedNets = trusted
	if !resolver.trustForwarded && len(resolver.trustedNets) == 0 {
		return resolver, nil
	}
	return resolver, nil
}

func (r *clientIPResolver) ClientIPFromRequest(req *http.Request) (string, string) {
	if req == nil {
		return "", ipSourceRemoteAddr
	}
	if r != nil && r.shouldTrust(req.RemoteAddr) {
		if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					return trimmed, ipSourceXForwardedFor
				}
			}
		}
		if xrip := strings.TrimSpace(req.Header.Get("X-Real-IP")); xrip != "" {
			return xrip, ipSourceXRealIP
		}
	}
	return clientIP(req.RemoteAddr), ipSourceRemoteAddr
}

func (r *clientIPResolver) shouldTrust(remoteAddr string) bool {
	if r == nil {
		return false
	}
	if r.trustForwarded {
		return true
	}
	if len(r.trustedNets) == 0 {
		return false
	}
	host := clientIP(remoteAddr)
	if host == "" {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, network := range r.trustedNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func resolveClientIP(r *http.Request, resolver *clientIPResolver) (string, string) {
	if resolver == nil {
		return clientIP(r.RemoteAddr), ipSourceRemoteAddr
	}
	return resolver.ClientIPFromRequest(r)
}

func clientIP(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func authMiddleware(handler *api.Handler, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/healthz" || path == "/metrics" || path == "/api/ingest/srs-hook" || strings.HasPrefix(path, "/api/auth/") || !strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		optionalAuth := false
		if r.Method == http.MethodGet {
			switch {
			case path == "/api/directory":
				optionalAuth = true
			case strings.HasPrefix(path, "/api/channels/"):
				optionalAuth = true
			case strings.HasPrefix(path, "/api/recordings"):
				optionalAuth = true
			case path == "/api/profiles":
				optionalAuth = true
			case strings.HasPrefix(path, "/api/profiles/"):
				optionalAuth = true
			}
		}
		token := api.ExtractToken(r)
		if token == "" {
			if optionalAuth {
				next.ServeHTTP(w, r)
				return
			}
			api.WriteError(w, http.StatusUnauthorized, fmt.Errorf("missing session token"))
			return
		}
		user, expiresAt, err := handler.AuthenticateRequest(r)
		if err != nil {
			if optionalAuth {
				handler.ClearSessionCookie(w, r)
				next.ServeHTTP(w, r)
				return
			}
			api.WriteError(w, http.StatusUnauthorized, err)
			return
		}
		if _, err := r.Cookie("bitriver_session"); err == nil {
			handler.RefreshSessionCookie(w, r, token, expiresAt)
		}
		ctx := api.ContextWithUser(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func spaHandler(staticFS fs.FS, index []byte, fileServer http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, fmt.Sprintf("method %s not allowed", r.Method), http.StatusMethodNotAllowed)
			return
		}

		requested := strings.TrimPrefix(r.URL.Path, "/")
		if requested != "" {
			servePath := requested
			file, err := staticFS.Open(servePath)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					trimmed := strings.TrimSuffix(requested, "/")
					if trimmed != "" {
						aliasPath := trimmed + ".html"
						file, err = staticFS.Open(aliasPath)
						if err == nil {
							servePath = aliasPath
						}
					}
				}
			}

			switch {
			case err == nil:
				info, statErr := file.Stat()
				file.Close()
				if statErr == nil && !info.IsDir() {
					reqToServe := r
					if servePath != requested {
						cloned := r.Clone(r.Context())
						clonedURL := *r.URL
						clonedURL.Path = "/" + servePath
						clonedURL.RawPath = ""
						cloned.URL = &clonedURL
						reqToServe = cloned
					}
					fileServer.ServeHTTP(w, reqToServe)
					return
				}
				if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
					http.Error(w, statErr.Error(), http.StatusInternalServerError)
					return
				}
			case err != nil && !errors.Is(err, fs.ErrNotExist):
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write(index)
	}
}
