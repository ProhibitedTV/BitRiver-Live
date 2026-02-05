package api

import (
	"errors"
	"net/http"
	"strings"

	"bitriver-live/internal/observability/metrics"
	"bitriver-live/internal/observability/tracing"
)

type viewerQoERequest struct {
	ChannelID       string  `json:"channelId"`
	SessionID       string  `json:"sessionId,omitempty"`
	Event           string  `json:"event"`
	Player          string  `json:"player,omitempty"`
	Protocol        string  `json:"protocol,omitempty"`
	LatencyMode     string  `json:"latencyMode,omitempty"`
	Rendition       string  `json:"rendition,omitempty"`
	PlaybackURL     string  `json:"playbackUrl,omitempty"`
	CurrentTime     float64 `json:"currentTime,omitempty"`
	Duration        float64 `json:"duration,omitempty"`
	BufferedSeconds float64 `json:"bufferedSeconds,omitempty"`
	DroppedFrames   int     `json:"droppedFrames,omitempty"`
	Error           string  `json:"error,omitempty"`
}

// ViewerQoE performs viewer qo e and returns an error when dependent systems reject the operation.
func (h *Handler) ViewerQoE(w http.ResponseWriter, r *http.Request) {
	r, span := h.startSpan(r, "api.viewer_qoe")
	if span != nil {
		defer span.End()
	}
	if r.Method != http.MethodPost {
		WriteMethodNotAllowed(w, r, http.MethodPost)
		return
	}

	var req viewerQoERequest
	if err := DecodeJSONAllowUnknown(r, &req); err != nil {
		WriteDecodeError(w, err)
		return
	}

	req.ChannelID = strings.TrimSpace(req.ChannelID)
	req.Event = strings.TrimSpace(req.Event)
	if req.ChannelID == "" || req.Event == "" {
		WriteError(w, http.StatusBadRequest, errors.New("channelId and event are required"))
		return
	}

	if span != nil {
		span.AddAttributes(
			tracing.StringAttr("channel.id", req.ChannelID),
			tracing.StringAttr("session.id", req.SessionID),
			tracing.StringAttr("qoe.event", req.Event),
			tracing.StringAttr("qoe.player", req.Player),
			tracing.StringAttr("qoe.protocol", req.Protocol),
			tracing.StringAttr("qoe.rendition", req.Rendition),
		)
		if req.Error != "" {
			span.RecordError(errors.New(req.Error))
		}
	}

	metrics.Default().ObserveViewerQoE(metrics.ViewerQoELabel{
		Event:       req.Event,
		Player:      req.Player,
		Protocol:    req.Protocol,
		Rendition:   req.Rendition,
		LatencyMode: req.LatencyMode,
	})

	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}
