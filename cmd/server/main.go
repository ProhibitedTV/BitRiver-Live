// Command server starts the BitRiver API HTTP service. Flag values take
// precedence over environment variables unless explicitly overridden by the
// OAuth env helpers below.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"bitriver-live/internal/app"
	"bitriver-live/internal/auth/oauth"
	"bitriver-live/internal/config"
	"bitriver-live/internal/envutil/pgdsn"
	"bitriver-live/internal/observability/logging"
	"bitriver-live/internal/observability/metrics"
	"bitriver-live/internal/observability/tracing"
	"bitriver-live/internal/server"
	"bitriver-live/internal/stringsutil"
)

// keyValueFlag captures key=value flag inputs for per-provider OAuth overrides.
// CLI values populate the map first and are later merged with environment
// variables, allowing env-specific secrets to replace flag values when both are
// provided.
type keyValueFlag map[string]string

var processEnv = config.LoadEnvironment()

// String returns a stable string form for flag and log output.
func (kv *keyValueFlag) String() string {
	if kv == nil || len(*kv) == 0 {
		return ""
	}
	parts := make([]string, 0, len(*kv))
	for key, value := range *kv {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// Set parses and stores a flag assignment, returning an error when the format is invalid.
func (kv *keyValueFlag) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid format %q, expected provider=value", value)
	}
	name := strings.ToLower(strings.TrimSpace(parts[0]))
	if name == "" {
		return fmt.Errorf("provider name is required")
	}
	if *kv == nil {
		*kv = make(map[string]string)
	}
	(*kv)[name] = strings.TrimSpace(parts[1])
	return nil
}

// main parses configuration, initializes runtime dependencies, and runs the process until shutdown.
func main() {
	processEnv = config.LoadEnvironment()
	addr := flag.String("addr", "", "HTTP listen address")
	mode := flag.String("mode", "", "server runtime mode (development or production, required; production enforces /metrics protection)")
	envFile := flag.String("env-file", "", "path to the environment file written by the setup wizard")
	allowSelfSignup := flag.Bool("allow-self-signup", false, "allow unauthenticated viewers to register accounts")
	sessionCookieCrossSite := flag.Bool("session-cookie-cross-site", false, "emit SameSite=None; Secure session cookies for cross-site viewer deployments")
	adminCORSOrigins := flag.String("admin-cors-origins", "", "comma separated origins allowed to access the control centre APIs")
	viewerCORSOrigins := flag.String("viewer-cors-origins", "", "comma separated origins allowed to access viewer APIs")
	securityCSP := flag.String("security-csp", "", "override the Content-Security-Policy header (empty uses the secure default)")
	securityFrameAncestors := flag.String("security-frame-ancestors", "", "frame-ancestors directive used in the default Content-Security-Policy")
	securityFrameOptions := flag.String("security-frame-options", "", "X-Frame-Options header value")
	securityReferrerPolicy := flag.String("security-referrer-policy", "", "Referrer-Policy header value")
	securityPermissionsPolicy := flag.String("security-permissions-policy", "", "Permissions-Policy header value")
	securityContentTypeOptions := flag.String("security-content-type-options", "", "X-Content-Type-Options header value")

	// Storage flags (env: BITRIVER_LIVE_STORAGE_DRIVER, BITRIVER_LIVE_POSTGRES_DSN, DATABASE_URL, BITRIVER_LIVE_POSTGRES_*).
	storageDriver := flag.String("storage-driver", "", "datastore driver (postgres only)")
	postgresDSN := flag.String("postgres-dsn", "", "Postgres connection string")
	postgresMaxConns := flag.Int("postgres-max-conns", 0, "maximum connections in the Postgres pool")
	postgresMinConns := flag.Int("postgres-min-conns", 0, "minimum idle connections maintained by the Postgres pool")
	postgresMaxConnLifetime := flag.Duration("postgres-max-conn-lifetime", 0, "maximum lifetime for a pooled Postgres connection")
	postgresMaxConnIdle := flag.Duration("postgres-max-conn-idle", 0, "maximum idle time for a pooled Postgres connection")
	postgresHealthInterval := flag.Duration("postgres-health-interval", 0, "interval between Postgres health checks")
	postgresAcquireTimeout := flag.Duration("postgres-acquire-timeout", 0, "timeout when acquiring a Postgres connection from the pool")
	postgresAppName := flag.String("postgres-app-name", "", "application_name reported to Postgres")

	// Session flags (env: BITRIVER_LIVE_SESSION_STORE, BITRIVER_LIVE_SESSION_POSTGRES_DSN, BITRIVER_LIVE_SESSION_TTL, BITRIVER_LIVE_SESSION_IDLE_TIMEOUT, BITRIVER_LIVE_SESSION_COOKIE_CROSS_SITE, BITRIVER_LIVE_ALLOW_SELF_SIGNUP).
	sessionStoreDriver := flag.String("session-store", "", "session store driver (memory or postgres)")
	sessionPostgresDSN := flag.String("session-postgres-dsn", "", "Postgres DSN for the session store")
	sessionTTL := flag.Duration("session-ttl", 0, "absolute session lifetime (e.g. 168h)")
	sessionIdleTimeout := flag.Duration("session-idle-timeout", 0, "idle timeout that refreshes session expiry on activity")

	// TLS flags (env: BITRIVER_LIVE_TLS_CERT, BITRIVER_LIVE_TLS_KEY).
	tlsCert := flag.String("tls-cert", "", "path to TLS certificate file")
	tlsKey := flag.String("tls-key", "", "path to TLS private key file")
	logLevel := flag.String("log-level", "info", "log level (debug, info, warn, error)")
	otelEndpoint := flag.String("otel-endpoint", "", "OTLP endpoint for OpenTelemetry traces (e.g. http://collector:4318)")
	otelSampleRatio := flag.Float64("otel-sample-ratio", 1, "OpenTelemetry trace sampling ratio (0.0-1.0)")
	metricsToken := flag.String("metrics-token", "", "token required to scrape /metrics (Authorization bearer or X-Metrics-Token); production requires this or --metrics-allow-networks/BITRIVER_LIVE_METRICS_ALLOW_NETWORKS")
	metricsAllowNetworks := flag.String("metrics-allow-networks", "", "comma separated CIDR blocks or IPs allowed to scrape /metrics; production requires this or --metrics-token/BITRIVER_LIVE_METRICS_TOKEN")

	// Rate limiting flags (env: BITRIVER_LIVE_RATE_*).
	globalRPS := flag.Float64("rate-global-rps", 0, "global request rate limit in requests per second")
	globalBurst := flag.Int("rate-global-burst", 0, "global rate limit burst allowance")
	loginLimit := flag.Int("rate-login-limit", 0, "maximum login attempts per window for a single IP")
	loginWindow := flag.Duration("rate-login-window", 0, "window for counting login attempts")
	trustForwarded := flag.Bool("rate-trust-forwarded-headers", false, "trust proxy-provided client IP headers")
	trustedProxies := flag.String("rate-trusted-proxies", "", "comma separated CIDR blocks or IPs of trusted proxies")
	uploadsTrustForwarded := flag.Bool("uploads-trust-forwarded-headers", false, "trust proxy-provided forwarded headers when building upload media URLs")
	uploadMediaBaseURL := flag.String("upload-media-base-url", "", "canonical externally reachable base URL for upload media source URLs")
	uploadMaxBytes := flag.Int64("upload-max-bytes", 0, "maximum multipart upload size in bytes")
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

	// Chat queue flags (env: BITRIVER_LIVE_CHAT_QUEUE_DRIVER, BITRIVER_LIVE_CHAT_QUEUE_REDIS_*).
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
	omeLLHLSOrigin := flag.String("ome-llhls-origin", "", "internal OvenMediaEngine LL-HLS origin proxied at /live (e.g. http://ome:8080)")
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
	chatRetentionMessages := flag.String("chat-retention-messages", "", "retention duration for chat messages (e.g. 720h, 0 disables expiry)")
	chatRetentionModeration := flag.String("chat-retention-moderation-logs", "", "retention duration for chat moderation logs")
	// OAuth flags (env: BITRIVER_LIVE_OAUTH_CONFIG, BITRIVER_LIVE_OAUTH_PROVIDERS, BITRIVER_LIVE_OAUTH_* overrides).
	oauthProvidersFlag := flag.String("oauth-providers", "", "JSON array or path describing OAuth providers")
	var oauthClientIDs keyValueFlag
	var oauthClientSecrets keyValueFlag
	var oauthRedirects keyValueFlag
	flag.Var(&oauthClientIDs, "oauth-client-id", "override OAuth client ID (provider=value)")
	flag.Var(&oauthClientSecrets, "oauth-client-secret", "override OAuth client secret (provider=value)")
	flag.Var(&oauthRedirects, "oauth-redirect-url", "override OAuth redirect URL (provider=value)")
	flag.Parse()

	logger := logging.Init(logging.Config{Level: stringsutil.FirstNonEmpty(*logLevel, envGet("BITRIVER_LIVE_LOG_LEVEL")), Format: string(logging.FormatJSON)})
	auditLogger := logging.WithComponent(logger, "audit")
	registry := metrics.NewRegistry()
	recorder := registry.Recorder

	allowSelfSignupValue := *allowSelfSignup
	if env, ok := envLookup("BITRIVER_LIVE_ALLOW_SELF_SIGNUP"); ok {
		if value, err := strconv.ParseBool(strings.TrimSpace(env)); err == nil {
			allowSelfSignupValue = value
		} else {
			logger.Warn("invalid BITRIVER_LIVE_ALLOW_SELF_SIGNUP", "value", env, "error", err)
		}
	}

	_, oauthManager, err := oauth.LoadFromFlagsAndEnv(oauth.LoadInput{
		Source:        *oauthProvidersFlag,
		ClientIDs:     oauthClientIDs,
		ClientSecrets: oauthClientSecrets,
		RedirectURLs:  oauthRedirects,
		Env:           processEnv,
	})
	if err != nil {
		logger.Error("failed to configure oauth", "error", err)
		os.Exit(1)
	}

	serverMode, err := resolveMode(*mode, envGet("BITRIVER_LIVE_MODE"))
	if err != nil {
		flag.Usage()
		logger.Error("invalid server mode", "error", err)
		os.Exit(2)
	}
	otelEndpointValue := stringsutil.FirstNonEmpty(*otelEndpoint, envGet("BITRIVER_LIVE_OTEL_EXPORTER_OTLP_ENDPOINT"), envGet("OTEL_EXPORTER_OTLP_ENDPOINT"))
	otelSampleRatioValue := resolveSampleRatio(*otelSampleRatio, envGet("BITRIVER_LIVE_OTEL_SAMPLE_RATIO"), envGet("OTEL_TRACES_SAMPLER_ARG"), logger)
	environmentValue := stringsutil.FirstNonEmpty(envGet("BITRIVER_LIVE_ENVIRONMENT"), string(serverMode))
	tracingProvider := tracing.NewProvider(tracing.Config{
		ServiceName: "bitriver-live-api",
		Environment: environmentValue,
		Endpoint:    otelEndpointValue,
		SampleRatio: otelSampleRatioValue,
		Logger:      logger,
	})
	tracing.SetDefault(tracingProvider.Tracer())
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracingProvider.Shutdown(shutdownCtx); err != nil {
			logger.Warn("trace exporter shutdown failed", "error", err)
		}
	}()
	loginLimitValue, err := resolveLoginLimit(serverMode, *loginLimit, "BITRIVER_LIVE_RATE_LOGIN_LIMIT")
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	sessionCookieCrossSiteValue := resolveBool(*sessionCookieCrossSite, "BITRIVER_LIVE_SESSION_COOKIE_CROSS_SITE")
	listenAddr := resolveListenAddr(*addr, serverMode, envGet("BITRIVER_LIVE_ADDR"))
	envFilePath := strings.TrimSpace(stringsutil.FirstNonEmpty(*envFile, envGet("BITRIVER_LIVE_ENV_FILE")))
	datastoreConfig := config.ResolveServerDatastoreConfig(*postgresDSN, *sessionPostgresDSN, processEnv)

	tlsCertPath := stringsutil.FirstNonEmpty(*tlsCert, envGet("BITRIVER_LIVE_TLS_CERT"))
	tlsKeyPath := stringsutil.FirstNonEmpty(*tlsKey, envGet("BITRIVER_LIVE_TLS_KEY"))

	viewerURL, err := resolveViewerOrigin(*viewerOrigin, envGet("BITRIVER_VIEWER_ORIGIN"))
	if err != nil {
		logger.Error("invalid viewer origin", "error", err)
		os.Exit(1)
	}
	omeLLHLSOriginURL, err := resolveOMEPlaybackOrigin(*omeLLHLSOrigin, envGet("BITRIVER_OME_LLHLS_ORIGIN"))
	if err != nil {
		logger.Error("invalid OME LL-HLS origin", "error", err)
		os.Exit(1)
	}
	uploadMediaBaseURLValue, err := resolveUploadMediaBaseURL(*uploadMediaBaseURL, envGet("BITRIVER_LIVE_UPLOAD_MEDIA_BASE_URL"))
	if err != nil {
		logger.Error("invalid upload media base URL", "error", err)
		os.Exit(1)
	}

	corsConfig := server.CORSConfig{
		AdminOrigins:  splitAndTrim(stringsutil.FirstNonEmpty(*adminCORSOrigins, envGet("BITRIVER_LIVE_ADMIN_CORS_ORIGINS"))),
		ViewerOrigins: splitAndTrim(stringsutil.FirstNonEmpty(*viewerCORSOrigins, envGet("BITRIVER_LIVE_VIEWER_CORS_ORIGINS"))),
	}

	securityCfg := server.SecurityConfig{
		ContentSecurityPolicy: stringsutil.FirstNonEmpty(*securityCSP, envGet("BITRIVER_LIVE_SECURITY_CSP")),
		FrameAncestors:        stringsutil.FirstNonEmpty(*securityFrameAncestors, envGet("BITRIVER_LIVE_SECURITY_FRAME_ANCESTORS")),
		FrameOptions:          stringsutil.FirstNonEmpty(*securityFrameOptions, envGet("BITRIVER_LIVE_SECURITY_FRAME_OPTIONS")),
		ReferrerPolicy:        stringsutil.FirstNonEmpty(*securityReferrerPolicy, envGet("BITRIVER_LIVE_SECURITY_REFERRER_POLICY")),
		PermissionsPolicy:     stringsutil.FirstNonEmpty(*securityPermissionsPolicy, envGet("BITRIVER_LIVE_SECURITY_PERMISSIONS_POLICY")),
		ContentTypeOptions:    stringsutil.FirstNonEmpty(*securityContentTypeOptions, envGet("BITRIVER_LIVE_SECURITY_CONTENT_TYPE_OPTIONS")),
	}

	ingestConfig, err := app.LoadIngestConfig(logger)
	if err != nil {
		logger.Error("failed to load ingest config", "error", err)
		os.Exit(1)
	}

	runtime, err := app.NewServerRuntime(app.ServerRuntimeInput{
		Mode:                          serverMode,
		ListenAddr:                    listenAddr,
		Logger:                        logger,
		AuditLogger:                   auditLogger,
		MetricsRecorder:               recorder,
		TracingProvider:               tracingProvider,
		AllowSelfSignup:               allowSelfSignupValue,
		SetupManager:                  newSetupManager(envFilePath, nil),
		UploadsTrustForwarded:         resolveBool(*uploadsTrustForwarded, "BITRIVER_LIVE_UPLOADS_TRUST_FORWARDED_HEADERS"),
		UploadMediaBaseURL:            uploadMediaBaseURLValue,
		UploadMaxBytes:                resolveInt64(*uploadMaxBytes, "BITRIVER_LIVE_UPLOAD_MAX_BYTES"),
		LoginLimit:                    loginLimitValue,
		LoginWindow:                   resolveDuration(*loginWindow, "BITRIVER_LIVE_RATE_LOGIN_WINDOW", time.Minute),
		RequireLoginProtection:        requiresLoginProtection(serverMode),
		TrustForwardedHeaders:         resolveBool(*trustForwarded, "BITRIVER_LIVE_RATE_TRUST_FORWARDED_HEADERS"),
		TrustedProxies:                splitAndTrim(stringsutil.FirstNonEmpty(*trustedProxies, envGet("BITRIVER_LIVE_RATE_TRUSTED_PROXIES"))),
		GlobalRPS:                     resolveFloat(*globalRPS, "BITRIVER_LIVE_RATE_GLOBAL_RPS"),
		GlobalBurst:                   resolveInt(*globalBurst, "BITRIVER_LIVE_RATE_GLOBAL_BURST"),
		RateRedisAddr:                 stringsutil.FirstNonEmpty(*redisAddr, envGet("BITRIVER_LIVE_RATE_REDIS_ADDR")),
		RateRedisAddrs:                splitAndTrim(stringsutil.FirstNonEmpty(*redisAddrs, envGet("BITRIVER_LIVE_RATE_REDIS_ADDRS"))),
		RateRedisUsername:             stringsutil.FirstNonEmpty(*redisUsername, envGet("BITRIVER_LIVE_RATE_REDIS_USERNAME")),
		RateRedisPassword:             stringsutil.FirstNonEmpty(*redisPassword, envGet("BITRIVER_LIVE_RATE_REDIS_PASSWORD")),
		RateRedisMasterName:           stringsutil.FirstNonEmpty(*redisMasterName, envGet("BITRIVER_LIVE_RATE_REDIS_MASTER_NAME")),
		RateRedisTimeout:              resolveDuration(*redisTimeout, "BITRIVER_LIVE_RATE_REDIS_TIMEOUT", 2*time.Second),
		RateRedisPoolSize:             resolveInt(*redisPoolSize, "BITRIVER_LIVE_RATE_REDIS_POOL_SIZE"),
		RateRedisTLS:                  server.RedisTLSConfig{CAFile: stringsutil.FirstNonEmpty(*redisTLSCA, envGet("BITRIVER_LIVE_RATE_REDIS_TLS_CA")), CertFile: stringsutil.FirstNonEmpty(*redisTLSCert, envGet("BITRIVER_LIVE_RATE_REDIS_TLS_CERT")), KeyFile: stringsutil.FirstNonEmpty(*redisTLSKey, envGet("BITRIVER_LIVE_RATE_REDIS_TLS_KEY")), ServerName: stringsutil.FirstNonEmpty(*redisTLSServerName, envGet("BITRIVER_LIVE_RATE_REDIS_TLS_SERVER_NAME")), InsecureSkipVerify: resolveBool(*redisTLSSkipVerify, "BITRIVER_LIVE_RATE_REDIS_TLS_SKIP_VERIFY")},
		CORS:                          corsConfig,
		Security:                      securityCfg,
		MetricsAccess:                 server.MetricsAccessConfig{Token: stringsutil.FirstNonEmpty(*metricsToken, envGet("BITRIVER_LIVE_METRICS_TOKEN")), AllowedNetworks: splitAndTrim(stringsutil.FirstNonEmpty(*metricsAllowNetworks, envGet("BITRIVER_LIVE_METRICS_ALLOW_NETWORKS")))},
		RequireMetricsProtection:      requiresMetricsProtection(serverMode),
		ViewerOrigin:                  viewerURL,
		ViewerMediaProxyOrigin:        omeLLHLSOriginURL,
		OAuth:                         oauthManager,
		SessionCookieSecureMode:       app.ResolveSessionCookieSecureMode(serverMode),
		SessionCookieCrossSite:        sessionCookieCrossSiteValue,
		TLS:                           server.TLSConfig{CertFile: tlsCertPath, KeyFile: tlsKeyPath},
		StorageDriverFlag:             *storageDriver,
		PostgresDSN:                   datastoreConfig.PostgresDSN,
		PostgresMaxConns:              *postgresMaxConns,
		PostgresMinConns:              *postgresMinConns,
		PostgresMaxConnLifetime:       *postgresMaxConnLifetime,
		PostgresMaxConnIdle:           *postgresMaxConnIdle,
		PostgresHealthInterval:        *postgresHealthInterval,
		PostgresAcquireTimeout:        *postgresAcquireTimeout,
		PostgresAppName:               *postgresAppName,
		SessionStoreDriverFlag:        *sessionStoreDriver,
		SessionPostgresDSN:            datastoreConfig.SessionPostgresDSN,
		SessionTTL:                    resolveDuration(*sessionTTL, "BITRIVER_LIVE_SESSION_TTL", 0),
		SessionIdleTimeout:            resolveDuration(*sessionIdleTimeout, "BITRIVER_LIVE_SESSION_IDLE_TIMEOUT", 0),
		ObjectEndpoint:                stringsutil.FirstNonEmpty(*objectEndpoint, envGet("BITRIVER_LIVE_OBJECT_ENDPOINT")),
		ObjectRegion:                  stringsutil.FirstNonEmpty(*objectRegion, envGet("BITRIVER_LIVE_OBJECT_REGION")),
		ObjectAccessKey:               stringsutil.FirstNonEmpty(*objectAccessKey, envGet("BITRIVER_LIVE_OBJECT_ACCESS_KEY")),
		ObjectSecretKey:               stringsutil.FirstNonEmpty(*objectSecretKey, envGet("BITRIVER_LIVE_OBJECT_SECRET_KEY")),
		ObjectBucket:                  stringsutil.FirstNonEmpty(*objectBucket, envGet("BITRIVER_LIVE_OBJECT_BUCKET")),
		ObjectUseSSL:                  resolveBool(*objectUseSSL, "BITRIVER_LIVE_OBJECT_USE_SSL"),
		ObjectPrefix:                  strings.TrimSpace(stringsutil.FirstNonEmpty(*objectPrefix, envGet("BITRIVER_LIVE_OBJECT_PREFIX"))),
		ObjectPublicEndpoint:          stringsutil.FirstNonEmpty(*objectPublicEndpoint, envGet("BITRIVER_LIVE_OBJECT_PUBLIC_ENDPOINT")),
		ObjectLifecycleDays:           resolveInt(*objectLifecycleDays, "BITRIVER_LIVE_OBJECT_LIFECYCLE_DAYS"),
		RecordingRetentionPublished:   *recordingRetentionPublished,
		RecordingRetentionUnpublished: *recordingRetentionUnpublished,
		ChatRetentionMessages:         *chatRetentionMessages,
		ChatRetentionModeration:       *chatRetentionModeration,
		IngestConfig:                  ingestConfig,
		IngestEnabled:                 true,
		ChatQueueDriver:               stringsutil.FirstNonEmpty(*chatQueueDriver, envGet("BITRIVER_LIVE_CHAT_QUEUE_DRIVER")),
		ChatRedisAddr:                 stringsutil.FirstNonEmpty(*chatRedisAddr, envGet("BITRIVER_LIVE_CHAT_QUEUE_REDIS_ADDR")),
		ChatRedisAddrs:                splitAndTrim(stringsutil.FirstNonEmpty(*chatRedisAddrs, envGet("BITRIVER_LIVE_CHAT_QUEUE_REDIS_ADDRS"))),
		ChatRedisUsername:             stringsutil.FirstNonEmpty(*chatRedisUsername, envGet("BITRIVER_LIVE_CHAT_QUEUE_REDIS_USERNAME")),
		ChatRedisPassword:             stringsutil.FirstNonEmpty(*chatRedisPassword, envGet("BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD")),
		ChatRedisStream:               stringsutil.FirstNonEmpty(*chatRedisStream, envGet("BITRIVER_LIVE_CHAT_QUEUE_REDIS_STREAM")),
		ChatRedisGroup:                stringsutil.FirstNonEmpty(*chatRedisGroup, envGet("BITRIVER_LIVE_CHAT_QUEUE_REDIS_GROUP")),
		ChatRedisMasterName:           stringsutil.FirstNonEmpty(*chatRedisMasterName, envGet("BITRIVER_LIVE_CHAT_QUEUE_REDIS_SENTINEL_MASTER")),
		ChatRedisPoolSize:             resolveInt(*chatRedisPoolSize, "BITRIVER_LIVE_CHAT_QUEUE_REDIS_POOL_SIZE"),
		ChatRedisTLSCA:                stringsutil.FirstNonEmpty(*chatRedisTLSCA, envGet("BITRIVER_LIVE_CHAT_QUEUE_REDIS_TLS_CA")),
		ChatRedisTLSCert:              stringsutil.FirstNonEmpty(*chatRedisTLSCert, envGet("BITRIVER_LIVE_CHAT_QUEUE_REDIS_TLS_CERT")),
		ChatRedisTLSKey:               stringsutil.FirstNonEmpty(*chatRedisTLSKey, envGet("BITRIVER_LIVE_CHAT_QUEUE_REDIS_TLS_KEY")),
		ChatRedisTLSServerName:        stringsutil.FirstNonEmpty(*chatRedisTLSServerName, envGet("BITRIVER_LIVE_CHAT_QUEUE_REDIS_TLS_SERVER_NAME")),
		ChatRedisTLSSkipVerify:        resolveBool(*chatRedisTLSSkipVerify, "BITRIVER_LIVE_CHAT_QUEUE_REDIS_TLS_SKIP_VERIFY"),
	})
	if err != nil {
		logger.Error("failed to initialise runtime", "error", err)
		os.Exit(1)
	}

	runtime.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	restartRequested := false

	select {
	case sig := <-quit:
		logger.Info("received shutdown signal", "signal", sig.String())
	case err := <-runtime.Errors():
		logger.Error("server error", "error", err)
	case <-runtime.RestartSignals():
		logger.Info("setup wizard requested restart")
		restartRequested = true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	runtime.Shutdown(ctx)

	logger.Info("server stopped")
	if restartRequested {
		logger.Info("exiting for restart after setup wizard")
	}
}

type sessionStoreConfig struct {
	Driver      string
	DSN         string
	AbsoluteTTL time.Duration
	IdleTimeout time.Duration
}

// resolveListenAddr resolves listen addr from flags and environment values, returning validation errors when incompatible settings are provided.
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

// resolveMode resolves mode from flags and environment values, returning validation errors when incompatible settings are provided.
func resolveMode(flagMode, envMode string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(flagMode))
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(envMode))
	}

	switch mode {
	case "development", "production":
		return mode, nil
	case "":
		return "", errors.New("server mode is required; set --mode or BITRIVER_LIVE_MODE to development or production")
	default:
		return "", fmt.Errorf("invalid mode %q: must be development or production", mode)
	}
}

// resolveSampleRatio resolves sample ratio from flags and environment values, returning validation errors when incompatible settings are provided.
func resolveSampleRatio(flagValue float64, envValue string, otelSamplerArg string, logger *slog.Logger) float64 {
	value := flagValue
	if envValue != "" {
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(envValue), 64); err == nil {
			value = parsed
		} else if logger != nil {
			logger.Warn("invalid BITRIVER_LIVE_OTEL_SAMPLE_RATIO", "value", envValue, "error", err)
		}
	} else if otelSamplerArg != "" {
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(otelSamplerArg), 64); err == nil {
			value = parsed
		} else if logger != nil {
			logger.Warn("invalid OTEL_TRACES_SAMPLER_ARG", "value", otelSamplerArg, "error", err)
		}
	}
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	if value == 0 {
		return 1
	}
	return value
}

// requiresMetricsProtection reports whether metrics protection is satisfied for the current input.
func requiresMetricsProtection(mode string) bool {
	return isProductionMode(mode)
}

// requiresLoginProtection reports whether login protection is satisfied for the current input.
func requiresLoginProtection(mode string) bool {
	return isProductionMode(mode)
}

// validateMetricsProtection validates metrics protection and reports an error when required invariants are not met.
func validateMetricsProtection(mode string, cfg server.MetricsAccessConfig) error {
	if requiresMetricsProtection(mode) && !server.MetricsAccessConfigured(cfg) {
		return errors.New("production mode requires protecting /metrics with BITRIVER_LIVE_METRICS_TOKEN or BITRIVER_LIVE_METRICS_ALLOW_NETWORKS")
	}

	return nil
}

// defaultListenForMode returns the default listen for mode for the current runtime mode.
func defaultListenForMode(mode string) string {
	if isProductionMode(mode) {
		return ":80"
	}
	return ":8080"
}

// isProductionMode reports whether production mode is satisfied for the current input.
func isProductionMode(mode string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), "production")
}

// resolveStorageDriver resolves storage driver from flags and environment values, returning validation errors when incompatible settings are provided.
func resolveStorageDriver(flagValue, envValue, postgresDSN string) (string, bool, error) {
	if driver := strings.ToLower(strings.TrimSpace(flagValue)); driver != "" {
		if driver != "postgres" {
			return "", true, fmt.Errorf("unsupported datastore driver %q: only postgres is supported", driver)
		}
		return driver, true, nil
	}
	if driver := strings.ToLower(strings.TrimSpace(envValue)); driver != "" {
		if driver != "postgres" {
			return "", true, fmt.Errorf("unsupported datastore driver %q: only postgres is supported", driver)
		}
		return driver, true, nil
	}
	if strings.TrimSpace(postgresDSN) != "" {
		return "postgres", false, nil
	}
	return "", false, fmt.Errorf("no datastore configured: configure Postgres via BITRIVER_LIVE_POSTGRES_DSN, DATABASE_URL, or --postgres-dsn")
}

// validateProductionDatastore validates production datastore and reports an error when required invariants are not met.
func validateProductionDatastore(driver, resolvedPostgresDSN, envPostgresDSN string) error {
	if driver != "postgres" {
		if driver == "" {
			return fmt.Errorf("production mode requires the postgres datastore driver")
		}
		return fmt.Errorf("production mode requires the postgres datastore driver, got %q", driver)
	}
	if strings.TrimSpace(envPostgresDSN) == "" {
		return fmt.Errorf("production mode requires BITRIVER_LIVE_POSTGRES_DSN to be set")
	}
	if strings.TrimSpace(resolvedPostgresDSN) == "" {
		return fmt.Errorf("postgres storage selected without DSN")
	}
	if err := validatePostgresTLS(resolvedPostgresDSN, "BITRIVER_LIVE_POSTGRES_DSN"); err != nil {
		return err
	}
	return nil
}

// resolvePostgresDSN resolves postgres dsn from flags and environment values, returning validation errors when incompatible settings are provided.
func resolvePostgresDSN(flagValue string) string {
	return strings.TrimSpace(config.ResolveServerDatastoreConfig(flagValue, "", processEnv).PostgresDSN)
}

// resolveViewerOrigin resolves viewer origin from flags and environment values, returning validation errors when incompatible settings are provided.
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

func resolveOMEPlaybackOrigin(flagValue, envValue string) (*url.URL, error) {
	raw := strings.TrimSpace(stringsutil.FirstNonEmpty(flagValue, envValue))
	if raw == "" {
		return nil, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse OME LL-HLS origin: %w", err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("OME LL-HLS origin must use http or https and include a host")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, fmt.Errorf("OME LL-HLS origin must be an origin URL without credentials, path, query, or fragment")
	}
	parsed.Path = ""
	return parsed, nil
}

// validatePostgresTLS validates postgres tls and reports an error when required invariants are not met.
func validatePostgresTLS(dsn, envVar string) error {
	return pgdsn.ValidateTLSPolicy(dsn, envVar)
}

// splitAndTrim splits and normalizes and trim values for downstream validation.
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

// resolveFloat resolves float from flags and environment values, returning validation errors when incompatible settings are provided.
func resolveFloat(flagValue float64, envKey string) float64 {
	if flagValue > 0 {
		return flagValue
	}
	if env := envGet(envKey); env != "" {
		if value, err := parseFloat(env); err == nil {
			return value
		}
	}
	return 0
}

const defaultLoginLimitNonProduction = 10

// resolveLoginLimit resolves login limit from flags and environment values, returning validation errors when incompatible settings are provided.
func resolveLoginLimit(mode string, flagValue int, envKey string) (int, error) {
	limit := resolveInt(flagValue, envKey)
	if requiresLoginProtection(mode) {
		if limit <= 0 {
			return 0, fmt.Errorf("production mode requires non-zero login throttling; set --rate-login-limit or BITRIVER_LIVE_RATE_LOGIN_LIMIT")
		}
		return limit, nil
	}
	if limit <= 0 {
		return defaultLoginLimitNonProduction, nil
	}
	return limit, nil
}

// resolveInt resolves int from flags and environment values, returning validation errors when incompatible settings are provided.
func resolveInt(flagValue int, envKey string) int {
	if flagValue > 0 {
		return flagValue
	}
	if env := envGet(envKey); env != "" {
		if value, err := parseInt(env); err == nil {
			return value
		}
	}
	return 0
}

// resolveInt64 resolves int64 from flags and environment values, returning validation errors when incompatible settings are provided.
func resolveInt64(flagValue int64, envKey string) int64 {
	if flagValue > 0 {
		return flagValue
	}
	if env := envGet(envKey); env != "" {
		if value, err := parseInt64(env); err == nil {
			return value
		}
	}
	return 0
}

// resolveDuration resolves duration from flags and environment values, returning validation errors when incompatible settings are provided.
func resolveDuration(flagValue time.Duration, envKey string, fallback time.Duration) time.Duration {
	if flagValue > 0 {
		return flagValue
	}
	if env := envGet(envKey); env != "" {
		if value, err := time.ParseDuration(env); err == nil {
			return value
		}
	}
	if fallback > 0 {
		return fallback
	}
	return 0
}

// resolveBool resolves bool from flags and environment values, returning validation errors when incompatible settings are provided.
func resolveBool(flagValue bool, envKey string) bool {
	if flagValue {
		return true
	}
	if env, ok := envLookup(envKey); ok {
		if value, err := strconv.ParseBool(strings.TrimSpace(env)); err == nil {
			return value
		}
	}
	return false
}

// resolveDurationSetting resolves duration setting from flags and environment values, returning validation errors when incompatible settings are provided.
func resolveDurationSetting(flagValue string, envKey string) (time.Duration, bool, error) {
	raw := strings.TrimSpace(flagValue)
	if raw == "" {
		if env, ok := envLookup(envKey); ok {
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

// parseFloat parses float and returns an error when the input is malformed.
func parseFloat(value string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(value), 64)
}

// parseInt parses int and returns an error when the input is malformed.
func parseInt(value string) (int, error) {
	v, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return v, nil
}

// parseInt64 parses int64 and returns an error when the input is malformed.
func parseInt64(value string) (int64, error) {
	v, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func resolveUploadMediaBaseURL(flagValue, envValue string) (string, error) {
	raw := strings.TrimSpace(flagValue)
	if raw == "" {
		raw = strings.TrimSpace(envValue)
	}
	if raw == "" {
		return "", nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse upload media base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("upload media base URL scheme must be http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("upload media base URL host is required")
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	return parsed.String(), nil
}

func envGet(key string) string {
	return processEnv.Get(key)
}

func envLookup(key string) (string, bool) {
	return processEnv.Lookup(key)
}
