package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"bitriver-live/internal/api"
	"bitriver-live/internal/auth"
	"bitriver-live/internal/auth/oauth"
	"bitriver-live/internal/chat"
	"bitriver-live/internal/config"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/observability/logging"
	"bitriver-live/internal/observability/metrics"
	"bitriver-live/internal/observability/tracing"
	"bitriver-live/internal/server"
	"bitriver-live/internal/service"
	serviceuploads "bitriver-live/internal/service/uploads"
	"bitriver-live/internal/storage"
)

type ServerRuntimeInput struct {
	Mode                     string
	ListenAddr               string
	Logger                   *slog.Logger
	AuditLogger              *slog.Logger
	MetricsRecorder          *metrics.Recorder
	TracingProvider          *tracing.Provider
	AllowSelfSignup          bool
	SetupManager             api.SetupManager
	UploadsTrustForwarded    bool
	UploadMediaBaseURL       string
	UploadMaxBytes           int64
	LoginLimit               int
	LoginWindow              time.Duration
	RequireLoginProtection   bool
	TrustForwardedHeaders    bool
	TrustedProxies           []string
	GlobalRPS                float64
	GlobalBurst              int
	RateRedisAddr            string
	RateRedisAddrs           []string
	RateRedisUsername        string
	RateRedisPassword        string
	RateRedisMasterName      string
	RateRedisTimeout         time.Duration
	RateRedisPoolSize        int
	RateRedisTLS             server.RedisTLSConfig
	CORS                     server.CORSConfig
	Security                 server.SecurityConfig
	MetricsAccess            server.MetricsAccessConfig
	RequireMetricsProtection bool
	ViewerOrigin             *url.URL
	OAuth                    oauth.Service
	SessionCookieSecureMode  api.SessionCookieSecureMode
	SessionCookieCrossSite   bool
	TLS                      server.TLSConfig

	StorageDriverFlag       string
	PostgresDSN             string
	PostgresMaxConns        int
	PostgresMinConns        int
	PostgresMaxConnLifetime time.Duration
	PostgresMaxConnIdle     time.Duration
	PostgresHealthInterval  time.Duration
	PostgresAcquireTimeout  time.Duration
	PostgresAppName         string

	SessionStoreDriverFlag string
	SessionPostgresDSN     string
	SessionTTL             time.Duration
	SessionIdleTimeout     time.Duration

	ObjectEndpoint                string
	ObjectRegion                  string
	ObjectAccessKey               string
	ObjectSecretKey               string
	ObjectBucket                  string
	ObjectUseSSL                  bool
	ObjectPrefix                  string
	ObjectPublicEndpoint          string
	ObjectLifecycleDays           int
	RecordingRetentionPublished   string
	RecordingRetentionUnpublished string
	ChatRetentionMessages         string
	ChatRetentionModeration       string

	IngestConfig  ingest.Config
	IngestEnabled bool

	ChatQueueDriver        string
	ChatRedisAddr          string
	ChatRedisAddrs         []string
	ChatRedisUsername      string
	ChatRedisPassword      string
	ChatRedisStream        string
	ChatRedisGroup         string
	ChatRedisMasterName    string
	ChatRedisPoolSize      int
	ChatRedisTLSCA         string
	ChatRedisTLSCert       string
	ChatRedisTLSKey        string
	ChatRedisTLSServerName string
	ChatRedisTLSSkipVerify bool
}

type ServerRuntime struct {
	server           *server.Server
	errs             chan error
	restartChan      chan struct{}
	logger           *slog.Logger
	listenAddr       string
	mode             string
	tlsCfg           server.TLSConfig
	uploadProcessor  *serviceuploads.UploadProcessor
	store            storage.Repository
	sessionStore     auth.SessionStore
	mfaStore         auth.MFAChallengeStore
	sessionCloser    func(context.Context) error
	mfaCloser        func(context.Context) error
	workerCancel     context.CancelFunc
	sessionPurgeStop func()
}

type closeableSessionStore interface {
	auth.SessionStore
	Close(context.Context) error
}

type closeableMFAStore interface {
	auth.MFAChallengeStore
	Close(context.Context) error
}

var (
	newPostgresRepository = func(dsn string, opts ...storage.Option) (storage.Repository, error) {
		return storage.NewPostgresRepository(dsn, opts...)
	}
	newPostgresSessionStore = func(dsn string, opts ...auth.PostgresSessionStoreOption) (closeableSessionStore, error) {
		return auth.NewPostgresSessionStore(dsn, opts...)
	}
	newPostgresMFAChallengeStore = func(dsn string, opts ...auth.PostgresMFAChallengeStoreOption) (closeableMFAStore, error) {
		return auth.NewPostgresMFAChallengeStore(dsn, opts...)
	}
	chatQueueFactory = configureChatQueue
)

func (r *ServerRuntime) Errors() <-chan error            { return r.errs }
func (r *ServerRuntime) RestartSignals() <-chan struct{} { return r.restartChan }

func (r *ServerRuntime) Start() {
	go func() {
		r.logger.Info("BitRiver Live API listening", "addr", r.listenAddr, "mode", r.mode)
		if r.tlsCfg.CertFile != "" && r.tlsCfg.KeyFile != "" {
			r.logger.Info("TLS enabled", "cert_file", r.tlsCfg.CertFile)
		}
		r.logger.Info("metrics endpoint available", "path", "/metrics")
		if err := r.server.Start(); err != nil {
			r.errs <- err
		}
	}()
}

func (r *ServerRuntime) Shutdown(ctx context.Context) {
	if r.workerCancel != nil {
		r.workerCancel()
	}
	if r.sessionPurgeStop != nil {
		r.sessionPurgeStop()
	}
	if err := r.server.Shutdown(ctx); err != nil {
		r.logger.Warn("graceful shutdown failed", "error", err)
	}
	if r.uploadProcessor != nil {
		if err := r.uploadProcessor.Shutdown(ctx); err != nil {
			r.logger.Warn("failed to stop upload processor", "error", err)
		}
	}
	if closer, ok := r.store.(interface{ Close(context.Context) error }); ok {
		_ = closer.Close(ctx)
	}
	if r.sessionCloser != nil {
		_ = r.sessionCloser(ctx)
	}
	if r.mfaCloser != nil {
		_ = r.mfaCloser(ctx)
	}
}

func NewServerRuntime(in ServerRuntimeInput) (*ServerRuntime, error) {
	var options []storage.Option
	if in.IngestConfig.RetryInterval > 0 || in.IngestConfig.MaxBootAttempts > 0 {
		options = append(options, storage.WithIngestRetries(in.IngestConfig.MaxBootAttempts, in.IngestConfig.RetryInterval))
	}
	var ingestController ingest.Controller
	if in.IngestEnabled && in.IngestConfig.Enabled() {
		controller, err := in.IngestConfig.NewHTTPController()
		if err != nil {
			return nil, fmt.Errorf("initialise ingest controller: %w", err)
		}
		controller.SetLogger(logging.WithComponent(in.Logger, "ingest"))
		ingestController = controller
		options = append(options, storage.WithIngestController(controller))
	}
	postgresDefaultDSN := strings.TrimSpace(in.PostgresDSN)
	objectCfg := storage.ObjectStorageConfig{Endpoint: in.ObjectEndpoint, Region: in.ObjectRegion, AccessKey: in.ObjectAccessKey, SecretKey: in.ObjectSecretKey, Bucket: in.ObjectBucket, UseSSL: in.ObjectUseSSL, Prefix: in.ObjectPrefix, PublicEndpoint: in.ObjectPublicEndpoint, LifecycleDays: in.ObjectLifecycleDays}
	if objectCfg.Endpoint != "" || objectCfg.Bucket != "" || objectCfg.PublicEndpoint != "" || objectCfg.Prefix != "" || objectCfg.Region != "" || objectCfg.AccessKey != "" || objectCfg.SecretKey != "" || objectCfg.LifecycleDays > 0 || objectCfg.UseSSL {
		options = append(options, storage.WithObjectStorage(objectCfg))
	}
	store, err := newPostgresRepository(postgresDefaultDSN, options...)
	if err != nil {
		return nil, fmt.Errorf("initialise storage repository: %w", err)
	}
	storeCleanupEnabled := true
	if closer, ok := store.(interface{ Close(context.Context) error }); ok {
		defer func() {
			if storeCleanupEnabled {
				_ = closer.Close(context.Background())
			}
		}()
	}

	sessionDSN := strings.TrimSpace(in.SessionPostgresDSN)
	var sessionStore auth.SessionStore
	var mfaStore auth.MFAChallengeStore
	var sessionCloser func(context.Context) error
	var mfaCloser func(context.Context) error
	sessionCleanupEnabled := false
	mfaCleanupEnabled := false
	if strings.EqualFold(in.SessionStoreDriverFlag, "memory") {
		sessionStore = auth.NewMemorySessionStore()
		mfaStore = auth.NewMemoryMFAChallengeStore()
	} else {
		pgStore, err := newPostgresSessionStore(sessionDSN, auth.WithTimeout(in.PostgresAcquireTimeout))
		if err != nil {
			return nil, fmt.Errorf("initialise session store: %w", err)
		}
		sessionStore = pgStore
		sessionCloser = func(ctx context.Context) error { return pgStore.Close(ctx) }
		sessionCleanupEnabled = true
		defer func() {
			if sessionCleanupEnabled {
				_ = pgStore.Close(context.Background())
			}
		}()
		mfaPGStore, err := newPostgresMFAChallengeStore(sessionDSN, auth.WithMFAChallengeTimeout(in.PostgresAcquireTimeout))
		if err != nil {
			return nil, fmt.Errorf("initialise mfa challenge store: %w", err)
		}
		mfaStore = mfaPGStore
		mfaCloser = func(ctx context.Context) error { return mfaPGStore.Close(ctx) }
		mfaCleanupEnabled = true
		defer func() {
			if mfaCleanupEnabled {
				_ = mfaPGStore.Close(context.Background())
			}
		}()
	}
	sessionOptions := []auth.SessionOption{auth.WithStore(sessionStore)}
	if in.SessionIdleTimeout > 0 {
		sessionOptions = append(sessionOptions, auth.WithIdleTimeout(in.SessionIdleTimeout))
	}
	sessions := auth.NewSessionManager(in.SessionTTL, sessionOptions...)
	mfaChallenges := auth.NewMFAChallengeManager(0, auth.WithMFAChallengeStore(mfaStore))

	chatCfg := chat.RedisQueueConfig{Addr: in.ChatRedisAddr, Addrs: in.ChatRedisAddrs, Username: in.ChatRedisUsername, Password: in.ChatRedisPassword, Stream: in.ChatRedisStream, Group: in.ChatRedisGroup, MasterName: in.ChatRedisMasterName, PoolSize: in.ChatRedisPoolSize, TLS: chat.RedisTLSConfig{CAFile: in.ChatRedisTLSCA, CertFile: in.ChatRedisTLSCert, KeyFile: in.ChatRedisTLSKey, ServerName: in.ChatRedisTLSServerName, InsecureSkipVerify: in.ChatRedisTLSSkipVerify}}
	queue, err := chatQueueFactory(in.ChatQueueDriver, chatCfg, in.Logger)
	if err != nil {
		return nil, fmt.Errorf("initialise chat queue: %w", err)
	}
	gateway := chat.NewGateway(chat.GatewayConfig{Queue: queue, Store: store, Logger: logging.WithComponent(in.Logger, "chat")})

	restartChan := make(chan struct{}, 1)
	var chatQueuePinger interface{ Ping(context.Context) error }
	if pingable, ok := queue.(interface{ Ping(context.Context) error }); ok {
		chatQueuePinger = pingable
	}
	useCases := service.NewStoreUseCases(store)
	handler := NewHandler(HandlerConfig{Sessions: sessions, MFAChallenges: mfaChallenges, AllowSelfSignup: in.AllowSelfSignup, ChatGateway: gateway, Setup: in.SetupManager, DefaultRenditions: ladderProfileNames(in.IngestConfig.LadderProfiles), SRSHookToken: in.IngestConfig.SRSToken, TrustForwardedHeaders: in.UploadsTrustForwarded, UploadMediaBaseURL: in.UploadMediaBaseURL, UploadMaxBytes: in.UploadMaxBytes, UploadSourceStorage: objectCfg, ChatQueue: chatQueuePinger, AuthUsersService: useCases, ChannelsService: useCases, UploadsService: useCases, RecordingsService: useCases, ChatModerationService: useCases, LegalService: service.NewLegalService(store), StreamsService: useCases, ProfilesService: useCases, AnalyticsService: useCases, SystemService: useCases, MonetizationService: useCases, PaymentService: service.NewPaymentService(store, in.Logger)})

	var uploadProcessor *serviceuploads.UploadProcessor
	if ingestController != nil {
		uploadSourceCfg := api.UploadSourceStorageConfig{
			Endpoint:       objectCfg.Endpoint,
			Bucket:         objectCfg.Bucket,
			Prefix:         objectCfg.Prefix,
			PublicEndpoint: objectCfg.PublicEndpoint,
			UseSSL:         objectCfg.UseSSL,
			RequestTimeout: objectCfg.RequestTimeout,
		}
		uploadProcessor = serviceuploads.NewUploadProcessor(serviceuploads.UploadProcessorConfig{
			Store:                 storage.NewUploadProcessingStore(useCases),
			Ingest:                ingestController,
			Cleaner:               api.NewUploadSourceCleaner(uploadSourceCfg, "", logging.WithComponent(in.Logger, "uploads")),
			Renditions:            in.IngestConfig.LadderProfiles,
			ReadySourceRetention:  0,
			FailedSourceRetention: 24 * time.Hour,
			Logger:                logging.WithComponent(in.Logger, "uploads"),
		})
		uploadProcessor.Start()
		handler.UploadProcessor = uploadProcessor
	}
	workerCtx, workerCancel := context.WithCancel(context.Background())
	sessionPurgeStop := startSessionPurgeWorker(workerCtx, logging.WithComponent(in.Logger, "session-purger"), sessions, 15*time.Minute)
	go storage.NewChatWorker(store, queue, logging.WithComponent(in.Logger, "chat-worker")).Run(workerCtx)

	rateCfg := server.RateLimitConfig{GlobalRPS: in.GlobalRPS, GlobalBurst: in.GlobalBurst, LoginLimit: in.LoginLimit, LoginWindow: in.LoginWindow, RequireLoginProtection: in.RequireLoginProtection, TrustForwardedHeaders: in.TrustForwardedHeaders, TrustedProxies: in.TrustedProxies, RedisAddr: in.RateRedisAddr, RedisAddrs: in.RateRedisAddrs, RedisUsername: in.RateRedisUsername, RedisPassword: in.RateRedisPassword, RedisMasterName: in.RateRedisMasterName, RedisTimeout: in.RateRedisTimeout, RedisPoolSize: in.RateRedisPoolSize, RedisTLS: in.RateRedisTLS}
	handler.TrustedProxies = rateCfg.TrustedProxies
	srv, err := NewHTTPServer(handler, server.Config{Addr: in.ListenAddr, TLS: in.TLS, RateLimit: rateCfg, CORS: in.CORS, Security: in.Security, Logger: in.Logger, AuditLogger: in.AuditLogger, Metrics: in.MetricsRecorder, Tracer: in.TracingProvider.Tracer(), MetricsAccess: in.MetricsAccess, RequireMetricsProtection: in.RequireMetricsProtection, ViewerOrigin: in.ViewerOrigin, OAuth: in.OAuth, AllowSelfSignup: &in.AllowSelfSignup, SessionCookieSecureMode: in.SessionCookieSecureMode, SessionCookieCrossSite: in.SessionCookieCrossSite, SRSHookToken: in.IngestConfig.SRSToken, UploadMaxBytes: in.UploadMaxBytes})
	if err != nil {
		return nil, fmt.Errorf("initialise http server: %w", err)
	}
	storeCleanupEnabled = false
	sessionCleanupEnabled = false
	mfaCleanupEnabled = false
	return &ServerRuntime{server: srv, errs: make(chan error, 1), restartChan: restartChan, logger: in.Logger, listenAddr: in.ListenAddr, mode: in.Mode, tlsCfg: in.TLS, uploadProcessor: uploadProcessor, store: store, sessionStore: sessionStore, mfaStore: mfaStore, sessionCloser: sessionCloser, mfaCloser: mfaCloser, workerCancel: workerCancel, sessionPurgeStop: sessionPurgeStop}, nil
}

func ladderProfileNames(profiles []ingest.Rendition) []string {
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		if strings.TrimSpace(profile.Name) == "" {
			continue
		}
		names = append(names, profile.Name)
	}
	return names
}

func configureChatQueue(driver string, cfg chat.RedisQueueConfig, logger *slog.Logger) (chat.Queue, error) {
	driver = strings.ToLower(strings.TrimSpace(driver))
	switch driver {
	case "redis":
		if len(cfg.Addrs) == 0 && strings.TrimSpace(cfg.Addr) == "" {
			return nil, fmt.Errorf("redis addr is required for chat queue")
		}
		cfg.Logger = logging.WithComponent(logger, "chat-queue")
		return chat.NewRedisQueue(cfg)
	case "", "memory":
		return chat.NewMemoryQueue(128), nil
	default:
		return nil, fmt.Errorf("unsupported chat queue driver %q", driver)
	}
}

func ResolveSessionCookieSecureMode(mode string) api.SessionCookieSecureMode {
	if strings.EqualFold(strings.TrimSpace(mode), "production") {
		return api.SessionCookieSecureAlways
	}
	return api.SessionCookieSecureAuto
}

func LoadIngestConfig(logger *slog.Logger) (ingest.Config, error) {
	parsed, err := config.LoadIngestFromEnv(config.LoadEnvironment())
	cfg := ingest.Config{
		SRSBaseURL:        parsed.SRSBaseURL,
		SRSToken:          parsed.SRSToken,
		OMEBaseURL:        parsed.OMEBaseURL,
		OMEUsername:       parsed.OMEUsername,
		OMEPassword:       parsed.OMEPassword,
		JobBaseURL:        parsed.JobBaseURL,
		JobToken:          parsed.JobToken,
		HealthEndpoint:    parsed.HealthEndpoint,
		HealthTimeout:     parsed.HealthTimeout,
		MaxBootAttempts:   parsed.MaxBootAttempts,
		RetryInterval:     parsed.RetryInterval,
		HTTPMaxAttempts:   parsed.HTTPMaxAttempts,
		HTTPRetryInterval: parsed.HTTPRetryInterval,
	}
	for _, profile := range parsed.LadderProfiles {
		cfg.LadderProfiles = append(cfg.LadderProfiles, ingest.Rendition{Name: profile.Name, Bitrate: profile.Bitrate})
	}
	if err == nil {
		return cfg, nil
	}
	if err == config.ErrIngestConfigDisabled {
		if logger != nil {
			logger.Warn("ingest control integration disabled; uploads will skip SRS checks unless BITRIVER_SRS_API and BITRIVER_SRS_TOKEN are set")
		}
		return cfg, nil
	}
	var missing config.MissingIngestConfigError
	if errors.As(err, &missing) {
		return cfg, ingest.MissingConfigError{Missing: missing.Missing}
	}
	return cfg, err
}
