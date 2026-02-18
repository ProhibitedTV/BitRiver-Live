package app

import (
	"context"

	"bitriver-live/internal/api"
	"bitriver-live/internal/auth"
	"bitriver-live/internal/chat"
	"bitriver-live/internal/server"
	"bitriver-live/internal/storage"
)

type healthPinger interface {
	Ping(context.Context) error
}

// HandlerConfig collects runtime dependencies for constructing api.Handler.
type HandlerConfig struct {
	Store                 storage.Repository
	Sessions              *auth.SessionManager
	MFAChallenges         *auth.MFAChallengeManager
	AllowSelfSignup       bool
	ChatGateway           *chat.Gateway
	Setup                 api.SetupManager
	DefaultRenditions     []string
	SRSHookToken          string
	TrustForwardedHeaders bool
	ChatQueue             healthPinger
}

// NewHandler composes API transport with services and infrastructure adapters.
func NewHandler(cfg HandlerConfig) *api.Handler {
	handler := api.NewHandler(cfg.Store, cfg.Sessions)
	handler.MFAChallenges = cfg.MFAChallenges
	handler.AllowSelfSignup = cfg.AllowSelfSignup
	handler.ChatGateway = cfg.ChatGateway
	handler.Setup = cfg.Setup
	handler.DefaultRenditions = cfg.DefaultRenditions
	handler.SRSHookToken = cfg.SRSHookToken
	handler.TrustForwardedHeaders = cfg.TrustForwardedHeaders
	if cfg.ChatQueue != nil {
		handler.ChatQueue = cfg.ChatQueue
	}
	return handler
}

// NewHTTPServer builds the runnable HTTP server from composed application parts.
func NewHTTPServer(handler *api.Handler, cfg server.Config) (*server.Server, error) {
	return server.New(handler, cfg)
}
