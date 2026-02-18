package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"bitriver-live/internal/models"
	"bitriver-live/internal/observability/metrics"
	"bitriver-live/internal/observability/tracing"
	"bitriver-live/internal/storage"
)

// normalizeSRSAction performs normalize srsaction and propagates validation or dependency failures to the caller.
func normalizeSRSAction(action string) string {
	normalized := strings.ToLower(strings.TrimSpace(action))
	normalized = strings.TrimPrefix(normalized, "on_")
	return normalized
}

// channelForStream performs channel for stream and propagates validation or dependency failures to the caller.
func (h *Handler) channelForStream(stream string) (models.Channel, bool) {
	trimmed := strings.TrimSpace(stream)
	if trimmed == "" || h.streamsService() == nil {
		return models.Channel{}, false
	}
	channels := h.streamsService().ListChannels("", "")
	for _, channel := range channels {
		if channel.StreamKey == trimmed || channel.ID == trimmed {
			return channel, true
		}
	}
	return models.Channel{}, false
}

type srsHookRequest struct {
	Action   string `json:"action"`
	Stream   string `json:"stream"`
	ClientID string `json:"client_id,omitempty"`
	Param    string `json:"param,omitempty"`
}

type srsViewerTracker struct {
	mu      sync.Mutex
	entries map[string]viewerCount
}

type viewerCount struct {
	current int
	peak    int
}

// newSRSViewerTracker builds and returns srsviewer tracker using the supplied dependencies.
func newSRSViewerTracker() *srsViewerTracker {
	return &srsViewerTracker{entries: make(map[string]viewerCount)}
}

// increment performs increment and propagates validation or dependency failures to the caller.
func (t *srsViewerTracker) increment(channelID string) viewerCount {
	t.mu.Lock()
	defer t.mu.Unlock()
	counts := t.entries[channelID]
	counts.current++
	if counts.current > counts.peak {
		counts.peak = counts.current
	}
	t.entries[channelID] = counts
	return counts
}

// decrement performs decrement and propagates validation or dependency failures to the caller.
func (t *srsViewerTracker) decrement(channelID string) viewerCount {
	t.mu.Lock()
	defer t.mu.Unlock()
	counts := t.entries[channelID]
	if counts.current > 0 {
		counts.current--
	}
	t.entries[channelID] = counts
	return counts
}

// peak performs peak and propagates validation or dependency failures to the caller.
func (t *srsViewerTracker) peak(channelID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.entries[channelID].peak
}

// clear performs clear and propagates validation or dependency failures to the caller.
func (t *srsViewerTracker) clear(channelID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, channelID)
}

// SRSHook performs srshook and returns an error when dependent systems reject the operation.
func (h *Handler) SRSHook(w http.ResponseWriter, r *http.Request) {
	r, span := h.startSpan(r, "api.srs_hook")
	if span != nil {
		defer span.End()
	}
	if r.Method != http.MethodPost {
		WriteMethodNotAllowed(w, r, http.MethodPost)
		return
	}
	if !h.srsHookAuthorized(r) {
		if logger := h.logger(); logger != nil {
			logger.Warn("srs hook rejected token", "path", r.URL.Path, "remote", r.RemoteAddr)
		}
		WriteError(w, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	var req srsHookRequest
	if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
		if err := DecodeJSONAllowUnknown(r, &req); err != nil {
			if !errors.Is(err, io.EOF) {
				WriteDecodeError(w, err)
				return
			}
		}
	}
	if req.Action == "" {
		req.Action = r.URL.Query().Get("action")
	}
	if req.Stream == "" {
		req.Stream = r.URL.Query().Get("stream")
	}

	action := normalizeSRSAction(req.Action)
	if action == "" {
		WriteError(w, http.StatusBadRequest, fmt.Errorf("action is required"))
		return
	}

	channel, ok := h.channelForStream(req.Stream)
	if !ok {
		if logger := h.logger(); logger != nil {
			logger.Warn("srs hook stream rejected", "stream", strings.TrimSpace(req.Stream), "action", action)
		}
		WriteError(w, http.StatusNotFound, fmt.Errorf("stream %s not recognized", strings.TrimSpace(req.Stream)))
		return
	}
	if span != nil {
		span.AddAttributes(
			tracing.StringAttr("channel.id", channel.ID),
			tracing.StringAttr("srs.action", action),
		)
	}

	tracker := h.srsTracker()

	switch action {
	case "publish":
		h.handleSRSPublish(channel, w, r)
	case "play":
		counts := tracker.increment(channel.ID)
		WriteJSON(w, http.StatusOK, map[string]int{"currentViewers": counts.current})
	case "stop":
		counts := tracker.decrement(channel.ID)
		WriteJSON(w, http.StatusOK, map[string]int{"currentViewers": counts.current})
	case "unpublish":
		peak := tracker.peak(channel.ID)
		h.handleSRSUnpublish(channel, peak, tracker, w)
	default:
		WriteError(w, http.StatusBadRequest, fmt.Errorf("unknown action %s", req.Action))
	}
}

// handleSRSPublish routes and serves srspublish requests, writing HTTP errors for invalid input or backend failures.
func (h *Handler) handleSRSPublish(channel models.Channel, w http.ResponseWriter, r *http.Request) {
	if current, ok := h.streamsService().CurrentStreamSession(channel.ID); ok {
		WriteJSON(w, http.StatusOK, srsHookResponse{Status: "ok", Action: "on_publish", ChannelID: channel.ID, SessionID: current.ID})
		return
	}

	session, err := h.streamsService().StartStream(channel.ID, h.srsRenditions())
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, storage.ErrIngestControllerUnavailable) {
			status = http.StatusServiceUnavailable
		}
		WriteError(w, status, err)
		return
	}
	metrics.StreamStarted()
	WriteJSON(w, http.StatusOK, srsHookResponse{Status: "ok", Action: "on_publish", ChannelID: channel.ID, SessionID: session.ID})
}

// handleSRSUnpublish routes and serves srsunpublish requests, writing HTTP errors for invalid input or backend failures.
func (h *Handler) handleSRSUnpublish(channel models.Channel, peak int, tracker *srsViewerTracker, w http.ResponseWriter) {
	if _, ok := h.streamsService().CurrentStreamSession(channel.ID); ok {
		session, err := h.streamsService().StopStream(channel.ID, peak)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, storage.ErrIngestControllerUnavailable) {
				status = http.StatusServiceUnavailable
			}
			WriteError(w, status, err)
			return
		}
		if tracker != nil {
			tracker.clear(channel.ID)
		}
		metrics.StreamStopped()
		WriteJSON(w, http.StatusOK, newSessionResponse(session))
		return
	}

	if tracker != nil {
		tracker.clear(channel.ID)
	}

	offline := "offline"
	if _, err := h.streamsService().UpdateChannel(channel.ID, storage.ChannelUpdate{LiveState: &offline}); err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type startStreamRequest struct {
	Renditions []string `json:"renditions"`
}

type stopStreamRequest struct {
	PeakConcurrent int `json:"peakConcurrent"`
}

type srsHookResponse struct {
	Status    string `json:"status"`
	Action    string `json:"action"`
	ChannelID string `json:"channelId,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
}

type renditionManifestResponse struct {
	Name        string `json:"name"`
	ManifestURL string `json:"manifestUrl"`
	Bitrate     int    `json:"bitrate,omitempty"`
}

// handleStreamRoutes routes and serves stream routes requests, writing HTTP errors for invalid input or backend failures.
func (h *Handler) handleStreamRoutes(channel models.Channel, remaining []string, w http.ResponseWriter, r *http.Request) {
	if len(remaining) == 0 {
		WriteError(w, http.StatusNotFound, fmt.Errorf("stream action missing"))
		return
	}
	if _, ok := h.ensureChannelAccess(w, r, channel); !ok {
		return
	}
	action := remaining[0]
	switch action {
	case "start":
		if r.Method != http.MethodPost {
			WriteMethodNotAllowed(w, r, http.MethodPost)
			return
		}
		var req startStreamRequest
		if !DecodeAndValidate(w, r, &req) {
			return
		}
		session, err := h.streamsService().StartStream(channel.ID, req.Renditions)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, storage.ErrIngestControllerUnavailable) {
				status = http.StatusServiceUnavailable
			}
			WriteError(w, status, err)
			return
		}
		metrics.StreamStarted()
		WriteJSON(w, http.StatusCreated, newSessionResponse(session))
	case "stop":
		if r.Method != http.MethodPost {
			WriteMethodNotAllowed(w, r, http.MethodPost)
			return
		}
		var req stopStreamRequest
		if !DecodeAndValidate(w, r, &req) {
			return
		}
		session, err := h.streamsService().StopStream(channel.ID, req.PeakConcurrent)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, storage.ErrIngestControllerUnavailable) {
				status = http.StatusServiceUnavailable
			}
			WriteError(w, status, err)
			return
		}
		metrics.StreamStopped()
		WriteJSON(w, http.StatusOK, newSessionResponse(session))
	case "rotate":
		if r.Method != http.MethodPost {
			WriteMethodNotAllowed(w, r, http.MethodPost)
			return
		}
		updated, err := h.streamsService().RotateChannelStreamKey(channel.ID)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		WriteJSON(w, http.StatusOK, newChannelResponse(updated))
	default:
		WriteError(w, http.StatusNotFound, fmt.Errorf("unknown stream action %s", action))
	}
}

// SRSHook processes callbacks from SRS http_hooks to validate publish/play
// events and update stream session state accordingly.
