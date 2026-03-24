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
	"bitriver-live/internal/observability/tracing"
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

// MetricsAccessConfigured reports whether a metrics token or allowlisted network
// is configured. Empty and whitespace-only values do not count as configured
// protection.
func MetricsAccessConfigured(cfg MetricsAccessConfig) bool {
	if strings.TrimSpace(cfg.Token) != "" {
		return true
	}
	for _, network := range cfg.AllowedNetworks {
		if strings.TrimSpace(network) != "" {
			return true
		}
	}
	return false
}

// Config aggregates the dependencies and settings required to construct a
// Server. Addr determines the listen address for the HTTP server, TLS controls
// whether HTTPS is enabled, RateLimit configures per-client throttling, CORS
// whitelists cross-site admin and viewer origins, Security sets the HTTP
// hardening headers, Logger and AuditLogger provide structured logging, Metrics
// records request metrics (defaulting to metrics.Default when nil), Tracer
// injects OpenTelemetry spans for HTTP requests, MetricsAccess restricts the
// Prometheus scrape endpoint, ViewerOrigin configures reverse proxying for
// viewer traffic, OAuth is injected into the supplied API handler,
// SessionCookieSecureMode forces HTTPS-only session cookies when set to
// SessionCookieSecureAlways, and SessionCookieCrossSite enables SameSite=None
// cookies for cross-site viewer deployments.
type Config struct {
	Addr                     string
	TLS                      TLSConfig
	RateLimit                RateLimitConfig
	CORS                     CORSConfig
	Security                 SecurityConfig
	Logger                   *slog.Logger
	AuditLogger              *slog.Logger
	Metrics                  *metrics.Recorder
	Tracer                   *tracing.Tracer
	MetricsAccess            MetricsAccessConfig
	RequireMetricsProtection bool
	ViewerOrigin             *url.URL
	OAuth                    oauth.Service
	AllowSelfSignup          *bool
	SessionCookieSecureMode  api.SessionCookieSecureMode
	SessionCookieCrossSite   bool
	SRSHookToken             string
	PaymentWebhookSecrets    map[string]string
	UploadMaxBytes           int64
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
	handler.Tracer = cfg.Tracer

	corsPolicy, err := newCORSPolicy(cfg.CORS)
	if err != nil {
		return nil, fmt.Errorf("configure CORS: %w", err)
	}

	recorder := cfg.Metrics
	if recorder == nil {
		recorder = metrics.Default()
	}
	configureAPIHandler(handler, cfg)

	rl, err := newRateLimiter(cfg.RateLimit)
	if err != nil {
		return nil, fmt.Errorf("configure rate limiter: %w", err)
	}
	handler.RateLimiter = rl
	ipResolver, err := newClientIPResolver(cfg.RateLimit)
	if err != nil {
		return nil, fmt.Errorf("configure client ip resolver: %w", err)
	}
	if cfg.RequireMetricsProtection && !MetricsAccessConfigured(cfg.MetricsAccess) {
		return nil, errors.New("metrics protection required: configure metrics token or allowlisted networks")
	}
	metricsAccess, err := newMetricsAccessController(cfg.MetricsAccess, ipResolver, cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("configure metrics access: %w", err)
	}

	mux := http.NewServeMux()
	if err := registerRoutes(mux, handler, cfg, recorder, metricsAccess, ipResolver); err != nil {
		return nil, err
	}

	handlerChain := buildMiddlewareChain(mux, handler, cfg, recorder, corsPolicy, rl, ipResolver)

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

func configureAPIHandler(handler *api.Handler, cfg Config) {
	handler.OAuth = cfg.OAuth
	if cfg.AllowSelfSignup != nil {
		handler.AllowSelfSignup = *cfg.AllowSelfSignup
	}
	handler.SRSHookToken = cfg.SRSHookToken
	if cfg.PaymentWebhookSecrets != nil {
		handler.WebhookSecrets = cfg.PaymentWebhookSecrets
	}
	if cfg.UploadMaxBytes > 0 {
		handler.UploadMaxBytes = cfg.UploadMaxBytes
	}
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
}

func registerRoutes(mux *http.ServeMux, handler *api.Handler, cfg Config, recorder *metrics.Recorder, metricsAccess *metricsAccessController, ipResolver *clientIPResolver) error {
	mux.HandleFunc("/healthz", handler.Health)
	mux.HandleFunc("/readyz", handler.Ready)
	mux.HandleFunc("/api/status", handler.Status)
	metricsHandler := recorder.Handler()
	metricsHandler = metricsAccess.handler(metricsHandler)
	mux.Handle("/metrics", metricsHandler)
	mux.HandleFunc("/api/auth/signup", handler.Signup)
	mux.HandleFunc("/api/auth/login", handler.Login)
	mux.HandleFunc("/api/auth/oauth/providers", handler.OAuthProviders)
	mux.HandleFunc("/api/auth/oauth/", handler.OAuthByProvider)
	mux.HandleFunc("/api/auth/session", handler.Session)
	mux.HandleFunc("/api/viewer/me", handler.ViewerMe)
	mux.HandleFunc("/api/auth/mfa", handler.MFAStatus)
	mux.HandleFunc("/api/auth/mfa/enroll", handler.MFAEnroll)
	mux.HandleFunc("/api/auth/mfa/verify", handler.MFAVerify)
	mux.HandleFunc("/api/auth/mfa/disable", handler.MFADisable)
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
	mux.HandleFunc("/api/moderation/automod", handler.ModerationAutoMod)
	mux.HandleFunc("/api/analytics/overview", handler.AnalyticsOverview)
	mux.HandleFunc("/api/metrics/qoe", handler.ViewerQoE)
	mux.HandleFunc("/api/setup", handler.SetupWizard)
	mux.HandleFunc("/api/legal/dmca", handler.LegalDMCA)
	mux.HandleFunc("/api/legal/dmca/", handler.LegalDMCAByID)
	mux.HandleFunc("/api/legal/data-subject", handler.LegalDataSubject)
	mux.HandleFunc("/api/legal/data-subject/", handler.LegalDataSubjectByID)
	mux.HandleFunc("/api/ingest/srs-hook", handler.SRSHook)
	mux.HandleFunc("/api/payments/webhooks/", handler.PaymentWebhook)

	staticFS, err := web.Static()
	if err != nil {
		return fmt.Errorf("load web assets: %w", err)
	}
	index, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		return fmt.Errorf("read web index: %w", err)
	}
	signupDocument, err := fs.ReadFile(staticFS, "signup.html")
	if err != nil {
		return fmt.Errorf("read signup page: %w", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	adminHandler := embeddedHTMLHandler(index)
	signupHandler := embeddedHTMLHandler(withBodyDataAttribute(signupDocument, "data-allow-self-signup", fmt.Sprintf("%t", handler.AllowSelfSignup)))
	compatSignupHandler := http.Handler(signupHandler)
	if cfg.ViewerOrigin != nil {
		compatSignupHandler = viewerSignupRedirectHandler()
	}
	mux.Handle("/signup", compatSignupHandler)
	mux.Handle("/signup/", compatSignupHandler)
	mux.Handle("/signup.html", compatSignupHandler)
	mux.Handle("/admin", adminHandler)
	mux.Handle("/admin/", adminHandler)

	if cfg.ViewerOrigin != nil {
		viewerProxy := httputil.NewSingleHostReverseProxy(cfg.ViewerOrigin)
		viewerProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			if requestLogger := loggingWithRequest(cfg.Logger, ipResolver, r); requestLogger != nil {
				requestLogger.Error("viewer proxy error", "error", err)
			}
			writeMiddlewareError(w, http.StatusBadGateway, "viewer temporarily unavailable")
		}
		viewerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			viewerProxy.ServeHTTP(w, r)
		})
		mux.Handle("/viewer", viewerHandler)
		mux.Handle("/viewer/", viewerHandler)
	}

	rootRedirectPath := ""
	if cfg.ViewerOrigin != nil {
		rootRedirectPath = "/viewer"
	}
	mux.HandleFunc("/", spaHandler(staticFS, index, fileServer, cfg.Logger, ipResolver, rootRedirectPath))
	return nil
}

func buildMiddlewareChain(mux *http.ServeMux, handler *api.Handler, cfg Config, recorder *metrics.Recorder, corsPolicy corsPolicy, rl *rateLimiter, ipResolver *clientIPResolver) http.Handler {
	handlerChain := http.Handler(mux)
	handlerChain = corsMiddleware(corsPolicy, cfg.Logger, ipResolver, handlerChain)
	securityCfg := cfg.Security.withDefaults()
	handlerChain = securityHeadersMiddleware(securityCfg, handlerChain)
	handlerChain = requestIDMiddleware(cfg.Logger, handlerChain)
	handlerChain = tracing.HTTPMiddleware(cfg.Tracer, handlerChain)
	handlerChain = authMiddleware(handler, handlerChain)
	handlerChain = csrfMiddleware(handler, cfg.Logger, ipResolver, handlerChain)
	handlerChain = rateLimitMiddleware(rl, ipResolver, cfg.Logger, handlerChain)
	handlerChain = metrics.HTTPMiddleware(recorder, handlerChain)
	handlerChain = auditMiddleware(cfg.AuditLogger, ipResolver, handlerChain)
	handlerChain = loggingMiddleware(cfg.Logger, ipResolver, handlerChain)
	return handlerChain
}

// Start performs start and returns an error when dependent systems reject the operation.
func (s *Server) Start() error {
	if s.httpServer == nil {
		return fmt.Errorf("http server is not configured")
	}

	if s.tlsCertFile != "" && s.tlsKeyFile != "" {
		return s.httpServer.ListenAndServeTLS(s.tlsCertFile, s.tlsKeyFile)
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown performs shutdown and returns an error when dependent systems reject the operation.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

// loggingMiddleware performs logging middleware and propagates validation or dependency failures to the caller.
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

// newMetricsAccessController builds and returns metrics access controller using the supplied dependencies.
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

// handler performs handler and propagates validation or dependency failures to the caller.
func (m *metricsAccessController) handler(next http.Handler) http.Handler {
	if m == nil || (m.token == "" && len(m.networks) == 0) {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _ := resolveClientIP(r, m.resolver)

		if m.token != "" && subtle.ConstantTimeCompare([]byte(m.token), []byte(metricsTokenFromRequest(r))) == 1 {
			next.ServeHTTP(w, r)
			return
		}

		if len(m.networks) > 0 && ipAllowed(ip, m.networks) {
			next.ServeHTTP(w, r)
			return
		}

		if requestLogger := loggingWithRequest(m.logger, m.resolver, r); requestLogger != nil {
			requestLogger.Warn("metrics access denied")
		}
		writeMiddlewareError(w, http.StatusForbidden, "metrics access denied")
	})
}

// metricsTokenFromRequest performs metrics token from request and propagates validation or dependency failures to the caller.
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

// ipAllowed performs ip allowed and propagates validation or dependency failures to the caller.
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

// rateLimitMiddleware performs rate limit middleware and propagates validation or dependency failures to the caller.
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
			ip, _ := resolveClientIP(r, resolver)
			key := authRateLimitKey(r, ip)
			requestLogger := loggingWithRequest(logger, resolver, r)
			allowed, retryAfter, err := rl.AllowLogin(r.Context(), key)
			if err != nil {
				if requestLogger != nil {
					requestLogger.Error("rate limiter failure", "error", err)
				}
				writeMiddlewareError(w, http.StatusServiceUnavailable, "rate limit failure")
				return
			}
			if !allowed {
				if requestLogger != nil {
					requestLogger.Warn("login rate limited")
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

// shouldRateLimitAuthRequest performs should rate limit auth request and propagates validation or dependency failures to the caller.
func shouldRateLimitAuthRequest(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}
	switch r.URL.Path {
	case "/api/auth/login", "/api/auth/signup":
		return r.Method == http.MethodPost
	case "/api/auth/mfa/verify", "/api/auth/mfa/enroll":
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

const mfaChallengeRateLimitCookie = "bitriver_mfa_challenge"

// authRateLimitKey performs auth rate limit key and propagates validation or dependency failures to the caller.
func authRateLimitKey(r *http.Request, clientIP string) string {
	if r == nil || r.URL == nil {
		return clientIP
	}

	if r.URL.Path == "/api/auth/mfa/verify" {
		if cookie, err := r.Cookie(mfaChallengeRateLimitCookie); err == nil {
			if challengeID := strings.TrimSpace(cookie.Value); challengeID != "" {
				return fmt.Sprintf("%s|challenge:%s", clientIP, challengeID)
			}
		}
	}

	return clientIP
}

// auditMiddleware performs audit middleware and propagates validation or dependency failures to the caller.
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

// shouldAudit performs should audit and propagates validation or dependency failures to the caller.
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

// parseNetworks parses networks and returns an error when the input is malformed.
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

// newClientIPResolver builds and returns client ipresolver using the supplied dependencies.
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

// ClientIPFromRequest performs client ipfrom request and returns an error when dependent systems reject the operation.
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

// shouldTrust performs should trust and propagates validation or dependency failures to the caller.
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

// resolveClientIP resolves client ip from flags and environment values, returning validation errors when incompatible settings are provided.
func resolveClientIP(r *http.Request, resolver *clientIPResolver) (string, string) {
	if resolver == nil {
		return clientIP(r.RemoteAddr), ipSourceRemoteAddr
	}
	return resolver.ClientIPFromRequest(r)
}

// clientIP performs client ip and propagates validation or dependency failures to the caller.
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

// authMiddleware performs auth middleware and propagates validation or dependency failures to the caller.
func authMiddleware(handler *api.Handler, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/healthz" || path == "/metrics" || path == "/api/ingest/srs-hook" || path == "/api/metrics/qoe" || strings.HasPrefix(path, "/api/auth/") || (path == "/api/legal/dmca" && r.Method == http.MethodPost) || !strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		optionalAuth := false
		if path == "/api/viewer/me" && (r.Method == http.MethodGet || r.Method == http.MethodDelete) {
			optionalAuth = true
		}
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

// spaHandler performs spa handler and propagates validation or dependency failures to the caller.
func embeddedHTMLHandler(document []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, fmt.Sprintf("method %s not allowed", r.Method), http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write(document)
	}
}

func viewerSignupRedirectHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, fmt.Sprintf("method %s not allowed", r.Method), http.StatusMethodNotAllowed)
			return
		}
		http.Redirect(w, r, buildViewerSignupRedirect(r), http.StatusTemporaryRedirect)
	}
}

func buildViewerSignupRedirect(r *http.Request) string {
	query := url.Values{}
	if r != nil && r.URL != nil {
		query = r.URL.Query()
	}

	redirectPath, nextPath := resolveViewerSignupTargets(query.Get("next"))
	parsed, err := url.Parse(redirectPath)
	if err != nil || parsed == nil || parsed.IsAbs() {
		parsed = &url.URL{Path: "/viewer"}
	}
	if parsed.Path == "" {
		parsed.Path = "/viewer"
	}

	values := parsed.Query()
	values.Set("auth", normalizeViewerAuthMode(query.Get("auth"), query.Get("mode")))
	if mfaMode := normalizeViewerMFAMode(query.Get("mfa")); mfaMode != "" {
		values.Set("mfa", mfaMode)
		values.Set("auth", "signin")
	}
	if nextPath != "" {
		values.Set("next", nextPath)
	} else {
		values.Del("next")
	}
	parsed.RawQuery = values.Encode()
	parsed.Fragment = ""
	return parsed.String()
}

func resolveViewerSignupTargets(rawNext string) (string, string) {
	nextPath := sanitizeViewerSignupPath(rawNext)
	if nextPath == "" || nextPath == "/" {
		return "/viewer", ""
	}
	if nextPath == "/viewer" || strings.HasPrefix(nextPath, "/viewer/") {
		return nextPath, nextPath
	}
	return "/viewer", ""
}

func sanitizeViewerSignupPath(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err == nil {
		if parsed.IsAbs() {
			trimmed = parsed.Path
			if parsed.RawQuery != "" {
				trimmed = trimmed + "?" + parsed.RawQuery
			}
			if parsed.Fragment != "" {
				trimmed = trimmed + "#" + parsed.Fragment
			}
		} else {
			trimmed = parsed.RequestURI()
			if parsed.Fragment != "" {
				trimmed = trimmed + "#" + parsed.Fragment
			}
		}
	}
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" || strings.HasPrefix(trimmed, "//") {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	parsed, err = url.Parse(trimmed)
	if err != nil || parsed == nil {
		return ""
	}
	switch {
	case parsed.Path == "", parsed.Path == "/", parsed.Path == "/viewer", strings.HasPrefix(parsed.Path, "/viewer/"):
		query := parsed.Query()
		query.Del("auth")
		query.Del("next")
		query.Del("mfa")
		parsed.RawQuery = query.Encode()
		return parsed.String()
	case strings.HasPrefix(parsed.Path, "/signup"), strings.HasPrefix(parsed.Path, "/admin"), strings.HasPrefix(parsed.Path, "/api/"):
		return ""
	default:
		return ""
	}
}

func normalizeViewerAuthMode(rawValues ...string) string {
	for _, raw := range rawValues {
		if strings.EqualFold(strings.TrimSpace(raw), "signup") {
			return "signup"
		}
	}
	return "signin"
}

func normalizeViewerMFAMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "enroll":
		return "enroll"
	case "verify":
		return "verify"
	default:
		return ""
	}
}

func withBodyDataAttribute(document []byte, name, value string) []byte {
	if strings.TrimSpace(name) == "" {
		return document
	}
	bodyTag := `<body class="auth-page">`
	replacement := fmt.Sprintf(`<body class="auth-page" %s="%s">`, name, value)
	return []byte(strings.Replace(string(document), bodyTag, replacement, 1))
}

func spaHandler(staticFS fs.FS, index []byte, fileServer http.Handler, logger *slog.Logger, resolver *clientIPResolver, rootRedirectPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, fmt.Sprintf("method %s not allowed", r.Method), http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path == "/" && strings.TrimSpace(rootRedirectPath) != "" {
			http.Redirect(w, r, rootRedirectPath, http.StatusTemporaryRedirect)
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
				_ = file.Close()
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
					log := loggingWithRequest(logger, resolver, r)
					if log != nil {
						log.Error("serve static asset failed", "path", r.URL.Path, "servePath", servePath, "reason", statErr)
					}
					api.WriteError(w, http.StatusInternalServerError, statErr)
					return
				}
			case err != nil && !errors.Is(err, fs.ErrNotExist):
				log := loggingWithRequest(logger, resolver, r)
				if log != nil {
					log.Error("serve static asset failed", "path", r.URL.Path, "servePath", servePath, "reason", err)
				}
				api.WriteError(w, http.StatusInternalServerError, err)
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
