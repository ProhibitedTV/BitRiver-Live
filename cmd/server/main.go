package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"bitriver-live/internal/api"
	"bitriver-live/internal/auth"
	"bitriver-live/internal/chat"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/observability/logging"
	"bitriver-live/internal/observability/metrics"
	"bitriver-live/internal/server"
	"bitriver-live/internal/storage"
)

func main() {
	addr := flag.String("addr", "", "HTTP listen address")
	dataPath := flag.String("data", "", "path to JSON datastore")
	storageDriver := flag.String("storage-driver", "", "datastore driver (json or postgres)")
	postgresDSN := flag.String("postgres-dsn", "", "Postgres connection string")
	postgresMaxConns := flag.Int("postgres-max-conns", 0, "maximum connections in the Postgres pool")
	postgresMinConns := flag.Int("postgres-min-conns", 0, "minimum idle connections maintained by the Postgres pool")
	postgresMaxConnLifetime := flag.Duration("postgres-max-conn-lifetime", 0, "maximum lifetime for a pooled Postgres connection")
	postgresMaxConnIdle := flag.Duration("postgres-max-conn-idle", 0, "maximum idle time for a pooled Postgres connection")
	postgresHealthInterval := flag.Duration("postgres-health-interval", 0, "interval between Postgres health checks")
	postgresAcquireTimeout := flag.Duration("postgres-acquire-timeout", 0, "timeout when acquiring a Postgres connection from the pool")
	postgresAppName := flag.String("postgres-app-name", "", "application_name reported to Postgres")
	sessionStoreDriver := flag.String("session-store", "", "session store driver (memory or postgres)")
	sessionPostgresDSN := flag.String("session-postgres-dsn", "", "Postgres DSN for the session store")
	mode := flag.String("mode", "", "server runtime mode (development or production)")
	tlsCert := flag.String("tls-cert", "", "path to TLS certificate file")
	tlsKey := flag.String("tls-key", "", "path to TLS private key file")
	logLevel := flag.String("log-level", "info", "log level (debug, info, warn, error)")
	globalRPS := flag.Float64("rate-global-rps", 0, "global request rate limit in requests per second")
	globalBurst := flag.Int("rate-global-burst", 0, "global rate limit burst allowance")
	loginLimit := flag.Int("rate-login-limit", 0, "maximum login attempts per window for a single IP")
	loginWindow := flag.Duration("rate-login-window", 0, "window for counting login attempts")
	redisAddr := flag.String("rate-redis-addr", "", "Redis address for distributed login throttling")
	redisAddrs := flag.String("rate-redis-addrs", "", "comma separated Redis addresses for distributed login throttling")
	redisUsername := flag.String("rate-redis-username", "", "Redis username for distributed login throttling")
	redisPassword := flag.String("rate-redis-password", "", "Redis password for distributed login throttling")
	redisMasterName := flag.String("rate-redis-master-name", "", "Redis sentinel master name for distributed login throttling")
	redisPoolSize := flag.Int("rate-redis-pool-size", 0, "maximum Redis connections for distributed login throttling")
	redisTLSCA := flag.String("rate-redis-tls-ca", "", "path to Redis TLS CA certificate for distributed login throttling")
	redisTLSCert := flag.String("rate-redis-tls-cert", "", "path to Redis TLS client certificate for distributed login throttling")
	redisTLSKey := flag.String("rate-redis-tls-key", "", "path to Redis TLS client key for distributed login throttling")
	redisTLSServerName := flag.String("rate-redis-tls-server-name", "", "override Redis TLS server name for distributed login throttling")
	redisTLSSkipVerify := flag.Bool("rate-redis-tls-skip-verify", false, "skip Redis TLS verification for distributed login throttling")
	redisTimeout := flag.Duration("rate-redis-timeout", 0, "timeout for Redis operations")
	chatQueueDriver := flag.String("chat-queue-driver", "", "chat queue driver (memory or redis)")
	chatRedisAddr := flag.String("chat-queue-redis-addr", "", "Redis address for chat queue transport")
	chatRedisAddrs := flag.String("chat-queue-redis-addrs", "", "comma separated Redis addresses for chat queue transport")
	chatRedisUsername := flag.String("chat-queue-redis-username", "", "Redis username for chat queue")
	chatRedisPassword := flag.String("chat-queue-redis-password", "", "Redis password for chat queue")
	chatRedisStream := flag.String("chat-queue-redis-stream", "", "Redis stream key for chat queue events")
	chatRedisGroup := flag.String("chat-queue-redis-group", "", "Redis consumer group for chat queue")
	chatRedisMasterName := flag.String("chat-queue-redis-sentinel-master", "", "Redis sentinel master name for chat queue")
	chatRedisPoolSize := flag.Int("chat-queue-redis-pool-size", 0, "maximum Redis connections for chat queue")
	chatRedisTLSCA := flag.String("chat-queue-redis-tls-ca", "", "path to Redis TLS CA certificate for chat queue")
	chatRedisTLSCert := flag.String("chat-queue-redis-tls-cert", "", "path to Redis TLS client certificate for chat queue")
	chatRedisTLSKey := flag.String("chat-queue-redis-tls-key", "", "path to Redis TLS client key for chat queue")
	chatRedisTLSServerName := flag.String("chat-queue-redis-tls-server-name", "", "override Redis TLS server name for chat queue")
	chatRedisTLSSkipVerify := flag.Bool("chat-queue-redis-tls-skip-verify", false, "skip Redis TLS verification for chat queue")
	viewerOrigin := flag.String("viewer-origin", "", "URL of the Next.js viewer runtime to proxy (e.g. http://127.0.0.1:3000)")
	objectEndpoint := flag.String("object-endpoint", "", "object storage endpoint (e.g. http://127.0.0.1:9000)")
	objectRegion := flag.String("object-region", "", "object storage region")
	objectAccessKey := flag.String("object-access-key", "", "object storage access key")
	objectSecretKey := flag.String("object-secret-key", "", "object storage secret key")
	objectBucket := flag.String("object-bucket", "", "object storage bucket name")
	objectUseSSL := flag.Bool("object-use-ssl", false, "enable TLS for object storage requests")
	objectPrefix := flag.String("object-prefix", "", "object storage key prefix for recordings")
	objectPublicEndpoint := flag.String("object-public-endpoint", "", "public endpoint used for playback URLs")
	objectLifecycleDays := flag.Int("object-lifecycle-days", 0, "lifecycle policy in days for archived objects")
	recordingRetentionPublished := flag.String("recording-retention-published", "", "retention duration for published recordings (e.g. 720h, 0 disables expiry)")
	recordingRetentionUnpublished := flag.String("recording-retention-unpublished", "", "retention duration for unpublished recordings")
	flag.Parse()

	logger := logging.New(logging.Config{Level: firstNonEmpty(*logLevel, os.Getenv("BITRIVER_LIVE_LOG_LEVEL"))})
	auditLogger := logging.WithComponent(logger, "audit")
	recorder := metrics.Default()

	serverMode := modeValue(*mode, os.Getenv("BITRIVER_LIVE_MODE"))
	listenAddr := resolveListenAddr(*addr, serverMode, os.Getenv("BITRIVER_LIVE_ADDR"))

	tlsCertPath := firstNonEmpty(*tlsCert, os.Getenv("BITRIVER_LIVE_TLS_CERT"))
	tlsKeyPath := firstNonEmpty(*tlsKey, os.Getenv("BITRIVER_LIVE_TLS_KEY"))

	viewerURL, err := resolveViewerOrigin(*viewerOrigin, os.Getenv("BITRIVER_VIEWER_ORIGIN"))
	if err != nil {
		logger.Error("invalid viewer origin", "error", err)
		os.Exit(1)
	}

	ingestConfig, err := ingest.LoadConfigFromEnv()
	if err != nil {
		logger.Error("failed to load ingest configuration", "error", err)
		os.Exit(1)
	}

	var options []storage.Option
	if ingestConfig.RetryInterval > 0 || ingestConfig.MaxBootAttempts > 0 {
		options = append(options, storage.WithIngestRetries(ingestConfig.MaxBootAttempts, ingestConfig.RetryInterval))
	}
	if ingestConfig.Enabled() {
		controller, err := ingestConfig.NewHTTPController()
		if err != nil {
			logger.Error("failed to initialise ingest controller", "error", err)
			os.Exit(1)
		}
		controller.SetLogger(logging.WithComponent(logger, "ingest"))
		options = append(options, storage.WithIngestController(controller))
	}

	publishedRetention, publishedSet, err := resolveDurationSetting(*recordingRetentionPublished, "BITRIVER_LIVE_RECORDING_RETENTION_PUBLISHED")
	if err != nil {
		logger.Error("invalid published retention", "error", err)
		os.Exit(1)
	}
	unpublishedRetention, unpublishedSet, err := resolveDurationSetting(*recordingRetentionUnpublished, "BITRIVER_LIVE_RECORDING_RETENTION_UNPUBLISHED")
	if err != nil {
		logger.Error("invalid unpublished retention", "error", err)
		os.Exit(1)
	}
	if publishedSet || unpublishedSet {
		policy := storage.RecordingRetentionPolicy{Published: -1, Unpublished: -1}
		if publishedSet {
			policy.Published = publishedRetention
		}
		if unpublishedSet {
			policy.Unpublished = unpublishedRetention
		}
		options = append(options, storage.WithRecordingRetention(policy))
	}

	objectCfg := storage.ObjectStorageConfig{
		Endpoint:       firstNonEmpty(*objectEndpoint, os.Getenv("BITRIVER_LIVE_OBJECT_ENDPOINT")),
		Region:         firstNonEmpty(*objectRegion, os.Getenv("BITRIVER_LIVE_OBJECT_REGION")),
		AccessKey:      firstNonEmpty(*objectAccessKey, os.Getenv("BITRIVER_LIVE_OBJECT_ACCESS_KEY")),
		SecretKey:      firstNonEmpty(*objectSecretKey, os.Getenv("BITRIVER_LIVE_OBJECT_SECRET_KEY")),
		Bucket:         firstNonEmpty(*objectBucket, os.Getenv("BITRIVER_LIVE_OBJECT_BUCKET")),
		UseSSL:         resolveBool(*objectUseSSL, "BITRIVER_LIVE_OBJECT_USE_SSL"),
		Prefix:         strings.TrimSpace(firstNonEmpty(*objectPrefix, os.Getenv("BITRIVER_LIVE_OBJECT_PREFIX"))),
		PublicEndpoint: firstNonEmpty(*objectPublicEndpoint, os.Getenv("BITRIVER_LIVE_OBJECT_PUBLIC_ENDPOINT")),
		LifecycleDays:  resolveInt(*objectLifecycleDays, "BITRIVER_LIVE_OBJECT_LIFECYCLE_DAYS"),
	}
	if objectCfg.Endpoint != "" || objectCfg.Bucket != "" || objectCfg.PublicEndpoint != "" || objectCfg.Prefix != "" || objectCfg.Region != "" || objectCfg.AccessKey != "" || objectCfg.SecretKey != "" || objectCfg.LifecycleDays > 0 || objectCfg.UseSSL {
		options = append(options, storage.WithObjectStorage(objectCfg))
	}

	driver := resolveStorageDriver(*storageDriver, os.Getenv("BITRIVER_LIVE_STORAGE_DRIVER"))
	var (
		store              storage.Repository
		storagePostgresDSN string
	)
	switch driver {
	case "json":
		dataFile := resolveDataPath(*dataPath)
		store, err = storage.NewJSONRepository(dataFile, options...)
	case "postgres":
		storagePostgresDSN = strings.TrimSpace(firstNonEmpty(*postgresDSN, os.Getenv("BITRIVER_LIVE_POSTGRES_DSN")))
		if storagePostgresDSN == "" {
			logger.Error("postgres storage selected without DSN")
			os.Exit(1)
		}
		pgOptions := append([]storage.Option(nil), options...)
		maxConns := resolveInt(*postgresMaxConns, "BITRIVER_LIVE_POSTGRES_MAX_CONNS")
		minConns := resolveInt(*postgresMinConns, "BITRIVER_LIVE_POSTGRES_MIN_CONNS")
		if maxConns > 0 || minConns > 0 {
			pgOptions = append(pgOptions, storage.WithPostgresPoolLimits(int32(maxConns), int32(minConns)))
		}
		maxLifetime := resolveDuration(*postgresMaxConnLifetime, "BITRIVER_LIVE_POSTGRES_MAX_CONN_LIFETIME", 0)
		maxIdle := resolveDuration(*postgresMaxConnIdle, "BITRIVER_LIVE_POSTGRES_MAX_CONN_IDLE", 0)
		healthInterval := resolveDuration(*postgresHealthInterval, "BITRIVER_LIVE_POSTGRES_HEALTH_INTERVAL", 0)
		if maxLifetime > 0 || maxIdle > 0 || healthInterval > 0 {
			pgOptions = append(pgOptions, storage.WithPostgresPoolDurations(maxLifetime, maxIdle, healthInterval))
		}
		acquireTimeout := resolveDuration(*postgresAcquireTimeout, "BITRIVER_LIVE_POSTGRES_ACQUIRE_TIMEOUT", 0)
		if acquireTimeout > 0 {
			pgOptions = append(pgOptions, storage.WithPostgresAcquireTimeout(acquireTimeout))
		}
		appName := firstNonEmpty(*postgresAppName, os.Getenv("BITRIVER_LIVE_POSTGRES_APP_NAME"))
		if appName != "" {
			pgOptions = append(pgOptions, storage.WithPostgresApplicationName(appName))
		}
		store, err = storage.NewPostgresRepository(storagePostgresDSN, pgOptions...)
	default:
		logger.Error("unsupported storage driver", "driver", driver)
		os.Exit(1)
	}
	if err != nil {
		logger.Error("failed to open datastore", "error", err)
		os.Exit(1)
	}

	sessionDriver := strings.ToLower(strings.TrimSpace(firstNonEmpty(*sessionStoreDriver, os.Getenv("BITRIVER_LIVE_SESSION_STORE"))))
	if sessionDriver == "" {
		sessionDriver = "memory"
	}

	var (
		sessionStore  auth.SessionStore
		sessionCloser func(context.Context) error
	)

	switch sessionDriver {
	case "memory":
		sessionStore = auth.NewMemorySessionStore()
	case "postgres":
		sessionDSN := firstNonEmpty(*sessionPostgresDSN, os.Getenv("BITRIVER_LIVE_SESSION_POSTGRES_DSN"))
		if sessionDSN == "" {
			sessionDSN = storagePostgresDSN
		}
		if strings.TrimSpace(sessionDSN) == "" {
			logger.Error("postgres session store selected without DSN")
			os.Exit(1)
		}
		pgStore, err := auth.NewPostgresSessionStore(sessionDSN)
		if err != nil {
			logger.Error("failed to open session store", "error", err)
			os.Exit(1)
		}
		sessionStore = pgStore
		sessionCloser = func(ctx context.Context) error { return pgStore.Close(ctx) }
	default:
		logger.Error("unsupported session store driver", "driver", sessionDriver)
		os.Exit(1)
	}

	sessions := auth.NewSessionManager(24*time.Hour, auth.WithStore(sessionStore))
	chatQueueCfg := chat.RedisQueueConfig{
		Addr:       firstNonEmpty(*chatRedisAddr, os.Getenv("BITRIVER_LIVE_CHAT_QUEUE_REDIS_ADDR")),
		Addrs:      splitAndTrim(firstNonEmpty(*chatRedisAddrs, os.Getenv("BITRIVER_LIVE_CHAT_QUEUE_REDIS_ADDRS"))),
		Username:   firstNonEmpty(*chatRedisUsername, os.Getenv("BITRIVER_LIVE_CHAT_QUEUE_REDIS_USERNAME")),
		Password:   firstNonEmpty(*chatRedisPassword, os.Getenv("BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD")),
		Stream:     firstNonEmpty(*chatRedisStream, os.Getenv("BITRIVER_LIVE_CHAT_QUEUE_REDIS_STREAM")),
		Group:      firstNonEmpty(*chatRedisGroup, os.Getenv("BITRIVER_LIVE_CHAT_QUEUE_REDIS_GROUP")),
		MasterName: firstNonEmpty(*chatRedisMasterName, os.Getenv("BITRIVER_LIVE_CHAT_QUEUE_REDIS_SENTINEL_MASTER")),
		PoolSize:   resolveInt(*chatRedisPoolSize, "BITRIVER_LIVE_CHAT_QUEUE_REDIS_POOL_SIZE"),
		TLS: chat.RedisTLSConfig{
			CAFile:             firstNonEmpty(*chatRedisTLSCA, os.Getenv("BITRIVER_LIVE_CHAT_QUEUE_REDIS_TLS_CA")),
			CertFile:           firstNonEmpty(*chatRedisTLSCert, os.Getenv("BITRIVER_LIVE_CHAT_QUEUE_REDIS_TLS_CERT")),
			KeyFile:            firstNonEmpty(*chatRedisTLSKey, os.Getenv("BITRIVER_LIVE_CHAT_QUEUE_REDIS_TLS_KEY")),
			ServerName:         firstNonEmpty(*chatRedisTLSServerName, os.Getenv("BITRIVER_LIVE_CHAT_QUEUE_REDIS_TLS_SERVER_NAME")),
			InsecureSkipVerify: resolveBool(*chatRedisTLSSkipVerify, "BITRIVER_LIVE_CHAT_QUEUE_REDIS_TLS_SKIP_VERIFY"),
		},
	}
	queue, err := configureChatQueue(*chatQueueDriver, chatQueueCfg, logger)
	if err != nil {
		logger.Error("failed to configure chat queue", "error", err)
		os.Exit(1)
	}
	gateway := chat.NewGateway(chat.GatewayConfig{
		Queue:  queue,
		Store:  store,
		Logger: logging.WithComponent(logger, "chat"),
	})
	handler := api.NewHandler(store, sessions)
	handler.ChatGateway = gateway
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	go storage.NewChatWorker(store, queue, logging.WithComponent(logger, "chat-worker")).Run(workerCtx)

	rateCfg := server.RateLimitConfig{
		GlobalRPS:       resolveFloat(*globalRPS, "BITRIVER_LIVE_RATE_GLOBAL_RPS"),
		GlobalBurst:     resolveInt(*globalBurst, "BITRIVER_LIVE_RATE_GLOBAL_BURST"),
		LoginLimit:      resolveInt(*loginLimit, "BITRIVER_LIVE_RATE_LOGIN_LIMIT"),
		LoginWindow:     resolveDuration(*loginWindow, "BITRIVER_LIVE_RATE_LOGIN_WINDOW", time.Minute),
		RedisAddr:       firstNonEmpty(*redisAddr, os.Getenv("BITRIVER_LIVE_RATE_REDIS_ADDR")),
		RedisAddrs:      splitAndTrim(firstNonEmpty(*redisAddrs, os.Getenv("BITRIVER_LIVE_RATE_REDIS_ADDRS"))),
		RedisUsername:   firstNonEmpty(*redisUsername, os.Getenv("BITRIVER_LIVE_RATE_REDIS_USERNAME")),
		RedisPassword:   firstNonEmpty(*redisPassword, os.Getenv("BITRIVER_LIVE_RATE_REDIS_PASSWORD")),
		RedisMasterName: firstNonEmpty(*redisMasterName, os.Getenv("BITRIVER_LIVE_RATE_REDIS_MASTER_NAME")),
		RedisTimeout:    resolveDuration(*redisTimeout, "BITRIVER_LIVE_RATE_REDIS_TIMEOUT", 2*time.Second),
		RedisPoolSize:   resolveInt(*redisPoolSize, "BITRIVER_LIVE_RATE_REDIS_POOL_SIZE"),
		RedisTLS: server.RedisTLSConfig{
			CAFile:             firstNonEmpty(*redisTLSCA, os.Getenv("BITRIVER_LIVE_RATE_REDIS_TLS_CA")),
			CertFile:           firstNonEmpty(*redisTLSCert, os.Getenv("BITRIVER_LIVE_RATE_REDIS_TLS_CERT")),
			KeyFile:            firstNonEmpty(*redisTLSKey, os.Getenv("BITRIVER_LIVE_RATE_REDIS_TLS_KEY")),
			ServerName:         firstNonEmpty(*redisTLSServerName, os.Getenv("BITRIVER_LIVE_RATE_REDIS_TLS_SERVER_NAME")),
			InsecureSkipVerify: resolveBool(*redisTLSSkipVerify, "BITRIVER_LIVE_RATE_REDIS_TLS_SKIP_VERIFY"),
		},
	}

	tlsCfg := server.TLSConfig{
		CertFile: tlsCertPath,
		KeyFile:  tlsKeyPath,
	}

	srv, err := server.New(handler, server.Config{
		Addr:         listenAddr,
		TLS:          tlsCfg,
		RateLimit:    rateCfg,
		Logger:       logger,
		AuditLogger:  auditLogger,
		Metrics:      recorder,
		ViewerOrigin: viewerURL,
	})
	if err != nil {
		logger.Error("failed to initialise server", "error", err)
		os.Exit(1)
	}

	errs := make(chan error, 1)
	go func() {
		logger.Info("BitRiver Live API listening", "addr", listenAddr, "mode", serverMode)
		if tlsCfg.CertFile != "" && tlsCfg.KeyFile != "" {
			logger.Info("TLS enabled", "cert_file", tlsCfg.CertFile)
		}
		logger.Info("metrics endpoint available", "path", "/metrics")
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.Info("received shutdown signal", "signal", sig.String())
	case err := <-errs:
		logger.Error("server error", "error", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Warn("graceful shutdown failed", "error", err)
	}

	if closer, ok := store.(interface{ Close(context.Context) error }); ok {
		if err := closer.Close(ctx); err != nil {
			logger.Warn("failed to close datastore", "error", err)
		}
	} else if closer, ok := store.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			logger.Warn("failed to close datastore", "error", err)
		}
	}

	if sessionCloser != nil {
		if err := sessionCloser(ctx); err != nil {
			logger.Warn("failed to close session store", "error", err)
		}
	} else if closer, ok := sessionStore.(interface{ Close(context.Context) error }); ok {
		if err := closer.Close(ctx); err != nil {
			logger.Warn("failed to close session store", "error", err)
		}
	} else if closer, ok := sessionStore.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			logger.Warn("failed to close session store", "error", err)
		}
	}

	logger.Info("server stopped")
}

func configureChatQueue(driver string, cfg chat.RedisQueueConfig, logger *slog.Logger) (chat.Queue, error) {
	driver = strings.ToLower(strings.TrimSpace(driver))
	switch driver {
	case "redis":
		if len(cfg.Addrs) == 0 && strings.TrimSpace(cfg.Addr) == "" {
			return nil, fmt.Errorf("redis addr is required for chat queue")
		}
		cfg.Logger = logging.WithComponent(logger, "chat-queue")
		queue, err := chat.NewRedisQueue(cfg)
		if err != nil {
			return nil, err
		}
		return queue, nil
	case "", "memory":
		return chat.NewMemoryQueue(128), nil
	default:
		return nil, fmt.Errorf("unsupported chat queue driver %q", driver)
	}
}

func resolveListenAddr(flagValue, mode, envAddr string) string {
	listenAddr := strings.TrimSpace(flagValue)
	if listenAddr == "" {
		listenAddr = strings.TrimSpace(envAddr)
	}
	if listenAddr == "" {
		listenAddr = defaultListenForMode(mode)
	}
	return listenAddr
}

func modeValue(flagMode, envMode string) string {
	mode := strings.ToLower(strings.TrimSpace(flagMode))
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(envMode))
	}
	if mode == "" {
		mode = "development"
	}
	return mode
}

func defaultListenForMode(mode string) string {
	if mode == "production" {
		return ":80"
	}
	return ":8080"
}

func resolveStorageDriver(flagValue, envValue string) string {
	driver := strings.ToLower(strings.TrimSpace(flagValue))
	if driver == "" {
		driver = strings.ToLower(strings.TrimSpace(envValue))
	}
	if driver == "" {
		driver = "json"
	}
	return driver
}

func resolveDataPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := strings.TrimSpace(os.Getenv("BITRIVER_LIVE_DATA")); env != "" {
		return env
	}
	return "data/store.json"
}

func resolveViewerOrigin(flagValue, envValue string) (*url.URL, error) {
	raw := strings.TrimSpace(flagValue)
	if raw == "" {
		raw = strings.TrimSpace(envValue)
	}
	if raw == "" {
		return nil, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse viewer origin: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("viewer origin must include scheme and host")
	}
	return parsed, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func splitAndTrim(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func resolveFloat(flagValue float64, envKey string) float64 {
	if flagValue > 0 {
		return flagValue
	}
	if env := os.Getenv(envKey); env != "" {
		if value, err := parseFloat(env); err == nil {
			return value
		}
	}
	return 0
}

func resolveInt(flagValue int, envKey string) int {
	if flagValue > 0 {
		return flagValue
	}
	if env := os.Getenv(envKey); env != "" {
		if value, err := parseInt(env); err == nil {
			return value
		}
	}
	return 0
}

func resolveDuration(flagValue time.Duration, envKey string, fallback time.Duration) time.Duration {
	if flagValue > 0 {
		return flagValue
	}
	if env := os.Getenv(envKey); env != "" {
		if value, err := time.ParseDuration(env); err == nil {
			return value
		}
	}
	if fallback > 0 {
		return fallback
	}
	return 0
}

func resolveBool(flagValue bool, envKey string) bool {
	if flagValue {
		return true
	}
	if env, ok := os.LookupEnv(envKey); ok {
		if value, err := strconv.ParseBool(strings.TrimSpace(env)); err == nil {
			return value
		}
	}
	return false
}

func resolveDurationSetting(flagValue string, envKey string) (time.Duration, bool, error) {
	raw := strings.TrimSpace(flagValue)
	if raw == "" {
		if env, ok := os.LookupEnv(envKey); ok {
			raw = strings.TrimSpace(env)
		}
	}
	if raw == "" {
		return 0, false, nil
	}
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return 0, false, err
	}
	return duration, true, nil
}

func parseFloat(value string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(value), 64)
}

func parseInt(value string) (int, error) {
	v, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return v, nil
}
