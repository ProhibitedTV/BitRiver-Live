package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"bitriver-live/internal/auth"
	"bitriver-live/internal/auth/oauth"
	"bitriver-live/internal/chat"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
	"bitriver-live/internal/observability/metrics"
	"bitriver-live/internal/storage"
)

// Handler aggregates the HTTP endpoints exposed by the BitRiver API along with
// the shared services they depend on, such as persistence, chat, and upload
// processing.
type Handler struct {
	Store               storage.Repository
	Sessions            *auth.SessionManager
	ChatGateway         *chat.Gateway
	OAuth               oauth.Service
	UploadProcessor     *UploadProcessor
	DefaultRenditions   []string
	SRSHookToken        string
	AllowSelfSignup     bool
	RateLimiter         healthPinger
	ChatQueue           healthPinger
	UploadMediaDir      string
	uploadDirOnce       sync.Once
	uploadDir           string
	SessionCookiePolicy SessionCookiePolicy
	srsViewers          *srsViewerTracker
	Logger              *slog.Logger
}

type healthPinger interface {
	Ping(context.Context) error
}

// NewHandler wires the core API dependencies together, ensuring a session
// manager is available by creating a default manager when none is provided.
func NewHandler(store storage.Repository, sessions *auth.SessionManager) *Handler {
	if sessions == nil {
		sessions = auth.NewSessionManager(0)
	}
	return &Handler{
		Store:               store,
		Sessions:            sessions,
		DefaultRenditions:   []string{"1080p", "720p", "480p"},
		AllowSelfSignup:     true,
		SessionCookiePolicy: DefaultSessionCookiePolicy(),
		Logger:              slog.Default(),
	}
}

func (h *Handler) sessionManager() *auth.SessionManager {
	if h.Sessions == nil {
		h.Sessions = auth.NewSessionManager(0)
	}
	return h.Sessions
}

func (h *Handler) logger() *slog.Logger {
	if h.Logger == nil {
		h.Logger = slog.Default()
	}
	return h.Logger
}

func (h *Handler) srsTracker() *srsViewerTracker {
	if h.srsViewers == nil {
		h.srsViewers = newSRSViewerTracker()
	}
	return h.srsViewers
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	components, overallStatus, statusCode := h.componentHealth(ctx)
	checks := []ingest.HealthStatus{}
	if h.Store != nil {
		checks = h.Store.IngestHealth(ctx)
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

func newSessionResponse(session models.StreamSession) sessionResponse {
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
