package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
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

// Config aggregates the dependencies and settings required to construct a
// Server. Addr determines the listen address for the HTTP server, TLS controls
// whether HTTPS is enabled, RateLimit configures per-client throttling, Logger
// and AuditLogger provide structured logging, Metrics records request metrics
// (defaulting to metrics.Default when nil), ViewerOrigin configures reverse
// proxying for viewer traffic, and OAuth is injected into the supplied API
// handler.
type Config struct {
	Addr                   string
	TLS                    TLSConfig
	RateLimit              RateLimitConfig
	Logger                 *slog.Logger
	AuditLogger            *slog.Logger
	Metrics                *metrics.Recorder
	ViewerOrigin           *url.URL
	OAuth                  oauth.Service
	AllowSelfSignup        *bool
	SessionCookieCrossSite bool
	SRSHookToken           string
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
	if cfg.SessionCookieCrossSite {
		handler.SessionCookiePolicy = api.SessionCookiePolicy{
			SameSite:   http.SameSiteNoneMode,
			SecureMode: api.SessionCookieSecureAlways,
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.Health)
	mux.HandleFunc("/readyz", handler.Ready)
	mux.Handle("/metrics", recorder.Handler())
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
			if cfg.Logger != nil {
				cfg.Logger.Error("viewer proxy error", "error", err, "path", r.URL.Path)
			}
			http.Error(w, "viewer temporarily unavailable", http.StatusBadGateway)
		}
		viewerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			viewerProxy.ServeHTTP(w, r)
		})
		mux.Handle("/viewer", viewerHandler)
		mux.Handle("/viewer/", viewerHandler)
	}

	mux.HandleFunc("/", spaHandler(staticFS, index, fileServer))

	rl, err := newRateLimiter(cfg.RateLimit)
	if err != nil {
		return nil, fmt.Errorf("configure rate limiter: %w", err)
	}
	handler.RateLimiter = rl
	ipResolver, err := newClientIPResolver(cfg.RateLimit)
	if err != nil {
		return nil, fmt.Errorf("configure client ip resolver: %w", err)
	}
	handlerChain := http.Handler(mux)
	handlerChain = authMiddleware(handler, handlerChain)
	handlerChain = rateLimitMiddleware(rl, ipResolver, cfg.Logger, handlerChain)
	handlerChain = metricsMiddleware(recorder, handlerChain)
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

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (sr *statusRecorder) WriteHeader(status int) {
	sr.status = status
	sr.ResponseWriter.WriteHeader(status)
}

func (sr *statusRecorder) Flush() {
	if flusher, ok := sr.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (sr *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := sr.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

func (sr *statusRecorder) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := sr.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

func (sr *statusRecorder) CloseNotify() <-chan bool {
	if notifier, ok := sr.ResponseWriter.(http.CloseNotifier); ok {
		return notifier.CloseNotify()
	}
	return nil
}

func (sr *statusRecorder) ReadFrom(r io.Reader) (int64, error) {
	if readerFrom, ok := sr.ResponseWriter.(io.ReaderFrom); ok {
		return readerFrom.ReadFrom(r)
	}
	return io.Copy(sr.ResponseWriter, r)
}

func loggingMiddleware(logger *slog.Logger, resolver *clientIPResolver, next http.Handler) http.Handler {
	if logger == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := newStatusRecorder(w)
		start := time.Now()
		next.ServeHTTP(recorder, r)
		duration := time.Since(start)
		ip, source := resolveClientIP(r, resolver)
		logger.Info("request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", duration.Milliseconds(),
			"remote_ip", ip,
			"ip_source", source)
	})
}

func metricsMiddleware(recorder *metrics.Recorder, next http.Handler) http.Handler {
	if recorder == nil {
		recorder = metrics.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sr := newStatusRecorder(w)
		start := time.Now()
		next.ServeHTTP(sr, r)
		recorder.ObserveRequest(r.Method, r.URL.Path, sr.status, time.Since(start))
	})
}

func rateLimitMiddleware(rl *rateLimiter, resolver *clientIPResolver, logger *slog.Logger, next http.Handler) http.Handler {
	if rl == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.AllowRequest() {
			http.Error(w, "global rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/auth/login" {
			ip, source := resolveClientIP(r, resolver)
			allowed, retryAfter, err := rl.AllowLogin(ip)
			if err != nil {
				if logger != nil {
					logger.Error("rate limiter failure", "error", err, "remote_ip", ip, "ip_source", source)
				}
				http.Error(w, "rate limit failure", http.StatusServiceUnavailable)
				return
			}
			if !allowed {
				if logger != nil {
					logger.Warn("login rate limited", "remote_ip", ip, "ip_source", source)
				}
				if retryAfter > 0 {
					w.Header().Set("Retry-After", fmt.Sprintf("%.0f", retryAfter.Seconds()))
				}
				http.Error(w, "too many login attempts", http.StatusTooManyRequests)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func auditMiddleware(logger *slog.Logger, resolver *clientIPResolver, next http.Handler) http.Handler {
	if logger == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sr := newStatusRecorder(w)
		start := time.Now()
		next.ServeHTTP(sr, r)
		if !shouldAudit(r) {
			return
		}
		duration := time.Since(start)
		user, ok := api.UserFromContext(r.Context())
		ip, source := resolveClientIP(r, resolver)
		fields := []interface{}{
			"method", r.Method,
			"path", r.URL.Path,
			"status", sr.status,
			"duration_ms", duration.Milliseconds(),
			"remote_ip", ip,
			"ip_source", source,
		}
		if ok {
			fields = append(fields, "user_id", user.ID)
		}
		logger.Info("audit", fields...)
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

type clientIPResolver struct {
	trustForwarded bool
	trustedNets    []*net.IPNet
}

func newClientIPResolver(cfg RateLimitConfig) (*clientIPResolver, error) {
	resolver := &clientIPResolver{trustForwarded: cfg.TrustForwardedHeaders}
	for _, raw := range cfg.TrustedProxies {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, network, err := net.ParseCIDR(trimmed); err == nil {
			resolver.trustedNets = append(resolver.trustedNets, network)
			continue
		}
		ip := net.ParseIP(trimmed)
		if ip == nil {
			return nil, fmt.Errorf("parse trusted proxy %q: invalid address", trimmed)
		}
		maskSize := 128
		if ip.To4() != nil {
			maskSize = 32
		}
		resolver.trustedNets = append(resolver.trustedNets, &net.IPNet{IP: ip, Mask: net.CIDRMask(maskSize, maskSize)})
	}
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
		if path == "/healthz" || path == "/metrics" || strings.HasPrefix(path, "/api/auth/") || !strings.HasPrefix(path, "/api/") {
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
		user, err := handler.AuthenticateRequest(r)
		if err != nil {
			if optionalAuth {
				handler.ClearSessionCookie(w, r)
				next.ServeHTTP(w, r)
				return
			}
			api.WriteError(w, http.StatusUnauthorized, err)
			return
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
