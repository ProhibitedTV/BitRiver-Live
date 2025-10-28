package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
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
	mode := flag.String("mode", "", "server runtime mode (development or production)")
	tlsCert := flag.String("tls-cert", "", "path to TLS certificate file")
	tlsKey := flag.String("tls-key", "", "path to TLS private key file")
	logLevel := flag.String("log-level", "info", "log level (debug, info, warn, error)")
	globalRPS := flag.Float64("rate-global-rps", 0, "global request rate limit in requests per second")
	globalBurst := flag.Int("rate-global-burst", 0, "global rate limit burst allowance")
	loginLimit := flag.Int("rate-login-limit", 0, "maximum login attempts per window for a single IP")
	loginWindow := flag.Duration("rate-login-window", 0, "window for counting login attempts")
	redisAddr := flag.String("rate-redis-addr", "", "Redis address for distributed login throttling")
	redisPassword := flag.String("rate-redis-password", "", "Redis password for distributed login throttling")
	redisTimeout := flag.Duration("rate-redis-timeout", 0, "timeout for Redis operations")
	viewerOrigin := flag.String("viewer-origin", "", "URL of the Next.js viewer runtime to proxy (e.g. http://127.0.0.1:3000)")
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

	driver := resolveStorageDriver(*storageDriver, os.Getenv("BITRIVER_LIVE_STORAGE_DRIVER"))
	var store storage.Repository
	switch driver {
	case "json":
		dataFile := resolveDataPath(*dataPath)
		store, err = storage.NewJSONRepository(dataFile, options...)
	case "postgres":
		dsn := firstNonEmpty(*postgresDSN, os.Getenv("BITRIVER_LIVE_POSTGRES_DSN"))
		if strings.TrimSpace(dsn) == "" {
			logger.Error("postgres storage selected without DSN")
			os.Exit(1)
		}
		store, err = storage.NewPostgresRepository(dsn, options...)
	default:
		logger.Error("unsupported storage driver", "driver", driver)
		os.Exit(1)
	}
	if err != nil {
		logger.Error("failed to open datastore", "error", err)
		os.Exit(1)
	}

	sessions := auth.NewSessionManager(24 * time.Hour)
	queue := chat.NewMemoryQueue(128)
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
		GlobalRPS:     resolveFloat(*globalRPS, "BITRIVER_LIVE_RATE_GLOBAL_RPS"),
		GlobalBurst:   resolveInt(*globalBurst, "BITRIVER_LIVE_RATE_GLOBAL_BURST"),
		LoginLimit:    resolveInt(*loginLimit, "BITRIVER_LIVE_RATE_LOGIN_LIMIT"),
		LoginWindow:   resolveDuration(*loginWindow, "BITRIVER_LIVE_RATE_LOGIN_WINDOW", time.Minute),
		RedisAddr:     firstNonEmpty(*redisAddr, os.Getenv("BITRIVER_LIVE_RATE_REDIS_ADDR")),
		RedisPassword: firstNonEmpty(*redisPassword, os.Getenv("BITRIVER_LIVE_RATE_REDIS_PASSWORD")),
		RedisTimeout:  resolveDuration(*redisTimeout, "BITRIVER_LIVE_RATE_REDIS_TIMEOUT", 2*time.Second),
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

	logger.Info("server stopped")
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
