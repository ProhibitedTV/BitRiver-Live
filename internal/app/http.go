package app

import (
	"context"

	"bitriver-live/internal/api"
	"bitriver-live/internal/auth"
	"bitriver-live/internal/chat"
	"bitriver-live/internal/server"
	"bitriver-live/internal/service"
	"bitriver-live/internal/storage"
)

type healthPinger interface {
	Ping(context.Context) error
}

// HandlerConfig collects runtime dependencies for constructing api.Handler.
type HandlerConfig struct {
	Sessions              *auth.SessionManager
	MFAChallenges         *auth.MFAChallengeManager
	AllowSelfSignup       bool
	ChatGateway           *chat.Gateway
	Setup                 api.SetupManager
	DefaultRenditions     []string
	SRSHookToken          string
	TrustForwardedHeaders bool
	UploadMediaBaseURL    string
	UploadMaxBytes        int64
	UploadSourceStorage   storage.ObjectStorageConfig
	ChatQueue             healthPinger
	AuthUsersService      service.AuthUsersUseCase
	ChannelsService       service.ChannelsDirectoryUseCase
	UploadsService        service.UploadsUseCase
	RecordingsService     service.RecordingsVODUseCase
	ChatModerationService service.ChatModerationUseCase
	LegalService          service.LegalComplianceUseCase
	StreamsService        service.StreamsUseCase
	ProfilesService       service.ProfilesUseCase
	AnalyticsService      service.AnalyticsUseCase
	SystemService         service.SystemHealthUseCase
	MonetizationService   service.MonetizationUseCase
	PaymentService        *service.PaymentService
}

// NewHandler composes API transport with services and infrastructure adapters.
func NewHandler(cfg HandlerConfig) *api.Handler {
	handler := api.NewHandler(api.Dependencies{Sessions: cfg.Sessions, MFAChallenges: cfg.MFAChallenges, AuthUsersService: cfg.AuthUsersService, ChannelsService: cfg.ChannelsService, UploadsService: cfg.UploadsService, RecordingsService: cfg.RecordingsService, ChatModerationService: cfg.ChatModerationService, LegalService: cfg.LegalService, StreamsService: cfg.StreamsService, ProfilesService: cfg.ProfilesService, AnalyticsService: cfg.AnalyticsService, SystemService: cfg.SystemService, MonetizationService: cfg.MonetizationService, PaymentService: cfg.PaymentService})
	handler.AllowSelfSignup = cfg.AllowSelfSignup
	handler.ChatGateway = cfg.ChatGateway
	handler.Setup = cfg.Setup
	handler.DefaultRenditions = cfg.DefaultRenditions
	handler.SRSHookToken = cfg.SRSHookToken
	handler.TrustForwardedHeaders = cfg.TrustForwardedHeaders
	handler.UploadMediaBaseURL = cfg.UploadMediaBaseURL
	handler.UploadMaxBytes = cfg.UploadMaxBytes
	handler.UploadSourceStorage = api.UploadSourceStorageConfig{
		Endpoint:       cfg.UploadSourceStorage.Endpoint,
		Bucket:         cfg.UploadSourceStorage.Bucket,
		Prefix:         cfg.UploadSourceStorage.Prefix,
		PublicEndpoint: cfg.UploadSourceStorage.PublicEndpoint,
		UseSSL:         cfg.UploadSourceStorage.UseSSL,
		RequestTimeout: cfg.UploadSourceStorage.RequestTimeout,
	}
	if cfg.ChatQueue != nil {
		handler.ChatQueue = cfg.ChatQueue
	}
	return handler
}

// NewHTTPServer builds the runnable HTTP server from composed application parts.
func NewHTTPServer(handler *api.Handler, cfg server.Config) (*server.Server, error) {
	return server.New(handler, cfg)
}
