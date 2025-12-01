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
	"bitriver-live/internal/storage"
)

func normalizeSRSAction(action string) string {
	normalized := strings.ToLower(strings.TrimSpace(action))
	normalized = strings.TrimPrefix(normalized, "on_")
	return normalized
}

func (h *Handler) channelForStream(stream string) (models.Channel, bool) {
	trimmed := strings.TrimSpace(stream)
	if trimmed == "" || h.Store == nil {
		return models.Channel{}, false
	}
	channels := h.Store.ListChannels("", "")
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

func newSRSViewerTracker() *srsViewerTracker {
	return &srsViewerTracker{entries: make(map[string]viewerCount)}
}

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

func (t *srsViewerTracker) peak(channelID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.entries[channelID].peak
}

func (t *srsViewerTracker) clear(channelID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, channelID)
}

func (h *Handler) SRSHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}
	if !h.srsHookAuthorized(r) {
		if logger := h.logger(); logger != nil {
			logger.Warn("srs hook rejected token", "path", r.URL.Path, "remote", r.RemoteAddr)
		}
		writeError(w, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
		return
	}

	var req srsHookRequest
	if r.Body != nil && r.Body != http.NoBody {
		if err := decodeJSONAllowUnknown(r, &req); err != nil {
			if !errors.Is(err, io.EOF) {
				writeError(w, http.StatusBadRequest, err)
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
		writeError(w, http.StatusBadRequest, fmt.Errorf("action is required"))
		return
	}

	channel, ok := h.channelForStream(req.Stream)
	if !ok {
		if logger := h.logger(); logger != nil {
			logger.Warn("srs hook stream rejected", "stream", strings.TrimSpace(req.Stream), "action", action)
		}
		writeError(w, http.StatusNotFound, fmt.Errorf("stream %s not recognized", strings.TrimSpace(req.Stream)))
		return
	}

	tracker := h.srsTracker()

	switch action {
	case "publish":
		h.handleSRSPublish(channel, w, r)
	case "play":
		counts := tracker.increment(channel.ID)
		writeJSON(w, http.StatusOK, map[string]int{"currentViewers": counts.current})
	case "stop":
		counts := tracker.decrement(channel.ID)
		writeJSON(w, http.StatusOK, map[string]int{"currentViewers": counts.current})
	case "unpublish":
		peak := tracker.peak(channel.ID)
		h.handleSRSUnpublish(channel, peak, tracker, w)
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown action %s", req.Action))
	}
}

func (h *Handler) handleSRSPublish(channel models.Channel, w http.ResponseWriter, r *http.Request) {
	if current, ok := h.Store.CurrentStreamSession(channel.ID); ok {
		writeJSON(w, http.StatusOK, srsHookResponse{Status: "ok", Action: "on_publish", ChannelID: channel.ID, SessionID: current.ID})
		return
	}

	session, err := h.Store.StartStream(channel.ID, h.srsRenditions())
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, storage.ErrIngestControllerUnavailable) {
			status = http.StatusServiceUnavailable
		}
		writeError(w, status, err)
		return
	}
	metrics.StreamStarted()
	writeJSON(w, http.StatusOK, srsHookResponse{Status: "ok", Action: "on_publish", ChannelID: channel.ID, SessionID: session.ID})
}

func (h *Handler) handleSRSUnpublish(channel models.Channel, peak int, tracker *srsViewerTracker, w http.ResponseWriter) {
	if _, ok := h.Store.CurrentStreamSession(channel.ID); ok {
		session, err := h.Store.StopStream(channel.ID, peak)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, storage.ErrIngestControllerUnavailable) {
				status = http.StatusServiceUnavailable
			}
			writeError(w, status, err)
			return
		}
		if tracker != nil {
			tracker.clear(channel.ID)
		}
		metrics.StreamStopped()
		writeJSON(w, http.StatusOK, newSessionResponse(session))
		return
	}

	if tracker != nil {
		tracker.clear(channel.ID)
	}

	offline := "offline"
	if _, err := h.Store.UpdateChannel(channel.ID, storage.ChannelUpdate{LiveState: &offline}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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

func (h *Handler) handleStreamRoutes(channel models.Channel, remaining []string, w http.ResponseWriter, r *http.Request) {
	if len(remaining) == 0 {
		writeError(w, http.StatusNotFound, fmt.Errorf("stream action missing"))
		return
	}
	if _, ok := h.ensureChannelAccess(w, r, channel); !ok {
		return
	}
	action := remaining[0]
	switch action {
	case "start":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
			return
		}
		var req startStreamRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		session, err := h.Store.StartStream(channel.ID, req.Renditions)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, storage.ErrIngestControllerUnavailable) {
				status = http.StatusServiceUnavailable
			}
			writeError(w, status, err)
			return
		}
		metrics.StreamStarted()
		writeJSON(w, http.StatusCreated, newSessionResponse(session))
	case "stop":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
			return
		}
		var req stopStreamRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		session, err := h.Store.StopStream(channel.ID, req.PeakConcurrent)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, storage.ErrIngestControllerUnavailable) {
				status = http.StatusServiceUnavailable
			}
			writeError(w, status, err)
			return
		}
		metrics.StreamStopped()
		writeJSON(w, http.StatusOK, newSessionResponse(session))
	case "rotate":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
			return
		}
		updated, err := h.Store.RotateChannelStreamKey(channel.ID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, newChannelResponse(updated))
	default:
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown stream action %s", action))
	}
}

// SRSHook processes callbacks from SRS http_hooks to validate publish/play
// events and update stream session state accordingly.
