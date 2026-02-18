package api

import (
	"net/http"
	"time"

	"bitriver-live/internal/service"
)

type analyticsSummaryResponse struct {
	LiveViewers      int     `json:"liveViewers"`
	StreamsLive      int     `json:"streamsLive"`
	WatchTimeMinutes float64 `json:"watchTimeMinutes"`
	ChatMessages     int     `json:"chatMessages"`
}

type analyticsChannelResponse struct {
	ChannelID       string  `json:"channelId"`
	Title           string  `json:"title,omitempty"`
	LiveViewers     int     `json:"liveViewers"`
	Followers       int     `json:"followers"`
	AvgWatchMinutes float64 `json:"avgWatchMinutes"`
	ChatMessages    int     `json:"chatMessages"`
}

type analyticsOverviewResponse struct {
	Summary    *analyticsSummaryResponse  `json:"summary,omitempty"`
	PerChannel []analyticsChannelResponse `json:"perChannel"`
}

// AnalyticsOverview performs analytics overview and returns an error when dependent systems reject the operation.
func (h *Handler) AnalyticsOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}
	if _, ok := h.requireRole(w, r, roleAdmin); !ok {
		return
	}
	overview, err := h.analyticsService().ComputeAnalyticsOverview(time.Now().UTC())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}
	WriteJSON(w, http.StatusOK, mapAnalyticsOverviewResponse(overview))
}

func mapAnalyticsOverviewResponse(overview service.AnalyticsOverview) analyticsOverviewResponse {
	response := analyticsOverviewResponse{PerChannel: make([]analyticsChannelResponse, 0, len(overview.PerChannel))}
	for _, channel := range overview.PerChannel {
		response.PerChannel = append(response.PerChannel, analyticsChannelResponse{
			ChannelID:       channel.ChannelID,
			Title:           channel.Title,
			LiveViewers:     channel.LiveViewers,
			Followers:       channel.Followers,
			AvgWatchMinutes: channel.AvgWatchMinutes,
			ChatMessages:    channel.ChatMessages,
		})
	}
	if overview.Summary != nil {
		response.Summary = &analyticsSummaryResponse{
			LiveViewers:      overview.Summary.LiveViewers,
			StreamsLive:      overview.Summary.StreamsLive,
			WatchTimeMinutes: overview.Summary.WatchTimeMinutes,
			ChatMessages:     overview.Summary.ChatMessages,
		}
	}
	return response
}
