package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"bitriver-live/internal/auth"
	"bitriver-live/internal/auth/oauth"
	"bitriver-live/internal/chat"
	"bitriver-live/internal/domain"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/observability/metrics"
	"bitriver-live/internal/observability/tracing"
	"bitriver-live/internal/service"
	serviceuploads "bitriver-live/internal/service/uploads"
)

// Handler aggregates the HTTP endpoints exposed by the BitRiver API along with
// the shared services they depend on, such as persistence, chat, and upload
// processing.
type Handler struct {
	Sessions              *auth.SessionManager
	MFAChallenges         *auth.MFAChallengeManager
	ChatGateway           *chat.Gateway
	OAuth                 oauth.Service
	UploadProcessor       serviceuploads.Enqueuer
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
	WebhookSecrets        map[string]string
	Setup                 SetupManager
	DefaultRenditions     []string
	SRSHookToken          string
	AllowSelfSignup       bool
	RateLimiter           healthPinger
	ChatQueue             healthPinger
	UploadMediaDir        string
	TrustForwardedHeaders bool
	TrustedProxies        []string
	uploadDirOnce         sync.Once
	uploadDir             string
	trustedProxyOnce      sync.Once
	trustedProxyNets      []*net.IPNet
	SessionCookiePolicy   SessionCookiePolicy
	srsViewers            *srsViewerTracker
	Logger                *slog.Logger
	Tracer                *tracing.Tracer
}

type healthPinger interface {
	Ping(context.Context) error
}

// NewHandler wires the core API dependencies together, ensuring a session
// manager is available by creating a default manager when none is provided.
type Dependencies struct {
	Sessions              *auth.SessionManager
	MFAChallenges         *auth.MFAChallengeManager
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

func NewHandler(deps Dependencies) *Handler {
	if deps.Sessions == nil {
		deps.Sessions = auth.NewSessionManager(0)
	}
	if deps.MFAChallenges == nil {
		deps.MFAChallenges = auth.NewMFAChallengeManager(0)
	}
	return &Handler{
		Sessions:              deps.Sessions,
		MFAChallenges:         deps.MFAChallenges,
		DefaultRenditions:     []string{"1080p", "720p", "480p"},
		AllowSelfSignup:       true,
		SessionCookiePolicy:   DefaultSessionCookiePolicy(),
		AuthUsersService:      deps.AuthUsersService,
		ChannelsService:       deps.ChannelsService,
		UploadsService:        deps.UploadsService,
		RecordingsService:     deps.RecordingsService,
		ChatModerationService: deps.ChatModerationService,
		LegalService:          deps.LegalService,
		StreamsService:        deps.StreamsService,
		ProfilesService:       deps.ProfilesService,
		AnalyticsService:      deps.AnalyticsService,
		SystemService:         deps.SystemService,
		MonetizationService:   deps.MonetizationService,
		PaymentService:        deps.PaymentService,
		WebhookSecrets:        map[string]string{},
		Logger:                slog.Default(),
	}
}

// sessionManager performs session manager and propagates validation or dependency failures to the caller.

func (h *Handler) authUsersService() service.AuthUsersUseCase {
	return h.AuthUsersService
}

func (h *Handler) channelsService() service.ChannelsDirectoryUseCase {
	return h.ChannelsService
}

func (h *Handler) uploadsService() service.UploadsUseCase {
	return h.UploadsService
}

func (h *Handler) recordingsService() service.RecordingsVODUseCase {
	return h.RecordingsService
}

func (h *Handler) chatModerationService() service.ChatModerationUseCase {
	return h.ChatModerationService
}

func (h *Handler) legalService() service.LegalComplianceUseCase {
	return h.LegalService
}

func (h *Handler) streamsService() service.StreamsUseCase {
	return h.StreamsService
}

func (h *Handler) profilesService() service.ProfilesUseCase {
	return h.ProfilesService
}

func (h *Handler) analyticsService() service.AnalyticsUseCase {
	return h.AnalyticsService
}

func (h *Handler) systemService() service.SystemHealthUseCase {
	return h.SystemService
}

func (h *Handler) monetizationService() service.MonetizationUseCase {
	return h.MonetizationService
}

func (h *Handler) sessionManager() *auth.SessionManager {
	if h.Sessions == nil {
		h.Sessions = auth.NewSessionManager(0)
	}
	return h.Sessions
}

// mfaChallengeManager performs mfa challenge manager and propagates validation or dependency failures to the caller.
func (h *Handler) mfaChallengeManager() *auth.MFAChallengeManager {
	if h.MFAChallenges == nil {
		h.MFAChallenges = auth.NewMFAChallengeManager(0)
	}
	return h.MFAChallenges
}

// logger performs logger and propagates validation or dependency failures to the caller.
func (h *Handler) logger() *slog.Logger {
	if h.Logger == nil {
		h.Logger = slog.Default()
	}
	return h.Logger
}

// tracer performs tracer and propagates validation or dependency failures to the caller.
func (h *Handler) tracer() *tracing.Tracer {
	if h.Tracer == nil {
		h.Tracer = tracing.Default()
	}
	return h.Tracer
}

// startSpan starts span and returns an error when startup or dependency checks fail.
func (h *Handler) startSpan(r *http.Request, name string, attrs ...tracing.Attribute) (*http.Request, *tracing.Span) {
	if r == nil {
		return r, nil
	}
	ctx, span := h.tracer().StartSpan(r.Context(), name, attrs...)
	return r.WithContext(ctx), span
}

// srsTracker performs srs tracker and propagates validation or dependency failures to the caller.
func (h *Handler) srsTracker() *srsViewerTracker {
	if h.srsViewers == nil {
		h.srsViewers = newSRSViewerTracker()
	}
	return h.srsViewers
}

// Health performs health and returns an error when dependent systems reject the operation.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	r, span := h.startSpan(r, "api.health")
	if span != nil {
		defer span.End()
	}
	ctx := r.Context()

	components, overallStatus, statusCode := h.componentHealth(ctx)
	checks := []ingest.HealthStatus{}
	if svc := h.systemService(); svc != nil {
		checks = svc.IngestHealth(ctx)
	}

	for _, check := range checks {
		switch strings.ToLower(check.Status) {
		case "ok", "disabled":
		// no-op
		default:
			overallStatus = "degraded"
		}
	}

	payload := map[string]interface{}{
		"status":     overallStatus,
		"services":   checks,
		"components": components,
	}
	for _, check := range checks {
		metrics.SetIngestHealth(check.Component, check.Status)
	}
	WriteJSON(w, statusCode, payload)
}

// Ready reports the status of core API dependencies without considering ingest
// services so load balancers can gate traffic on database and session readiness
// alone.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	r, span := h.startSpan(r, "api.ready")
	if span != nil {
		defer span.End()
	}
	components, overallStatus, statusCode := h.componentHealth(r.Context())
	payload := map[string]interface{}{
		"status":     overallStatus,
		"components": components,
	}
	WriteJSON(w, statusCode, payload)
}

type sessionResponse struct {
	ID                 string                      `json:"id"`
	ChannelID          string                      `json:"channelId"`
	StartedAt          string                      `json:"startedAt"`
	EndedAt            *string                     `json:"endedAt,omitempty"`
	Renditions         []string                    `json:"renditions"`
	PeakConcurrent     int                         `json:"peakConcurrent"`
	OriginURL          string                      `json:"originUrl,omitempty"`
	PlaybackURL        string                      `json:"playbackUrl,omitempty"`
	IngestEndpoints    []string                    `json:"ingestEndpoints,omitempty"`
	IngestJobIDs       []string                    `json:"ingestJobIds,omitempty"`
	RenditionManifests []renditionManifestResponse `json:"renditionManifests,omitempty"`
}

// newSessionResponse builds and returns session response using the supplied dependencies.
func newSessionResponse(session domain.StreamSession) sessionResponse {
	resp := sessionResponse{
		ID:             session.ID,
		ChannelID:      session.ChannelID,
		StartedAt:      session.StartedAt.Format(time.RFC3339Nano),
		Renditions:     append([]string{}, session.Renditions...),
		PeakConcurrent: session.PeakConcurrent,
	}
	if session.EndedAt != nil {
		ended := session.EndedAt.Format(time.RFC3339Nano)
		resp.EndedAt = &ended
	}
	if session.OriginURL != "" {
		resp.OriginURL = session.OriginURL
	}
	if session.PlaybackURL != "" {
		resp.PlaybackURL = session.PlaybackURL
	}
	if len(session.IngestEndpoints) > 0 {
		resp.IngestEndpoints = append([]string{}, session.IngestEndpoints...)
	}
	if len(session.IngestJobIDs) > 0 {
		resp.IngestJobIDs = append([]string{}, session.IngestJobIDs...)
	}
	if len(session.RenditionManifests) > 0 {
		manifests := make([]renditionManifestResponse, 0, len(session.RenditionManifests))
		for _, manifest := range session.RenditionManifests {
			manifests = append(manifests, renditionManifestResponse{
				Name:        manifest.Name,
				ManifestURL: manifest.ManifestURL,
				Bitrate:     manifest.Bitrate,
			})
		}
		resp.RenditionManifests = manifests
	}
	return resp
}
