package api

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"bitriver-live/internal/models"
	"bitriver-live/internal/observability/metrics"
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

func (h *Handler) AnalyticsOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}
	if _, ok := h.requireRole(w, r, roleAdmin); !ok {
		return
	}
	payload, err := h.computeAnalyticsOverview(time.Now().UTC())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}
	WriteJSON(w, http.StatusOK, payload)
}

func (h *Handler) computeAnalyticsOverview(now time.Time) (analyticsOverviewResponse, error) {
	channels := h.Store.ListChannels("", "")
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	windowStart := now.Add(-24 * time.Hour)
	summary := analyticsSummaryResponse{}
	perChannel := make([]analyticsChannelResponse, 0, len(channels))
	for _, channel := range channels {
		entry := analyticsChannelResponse{
			ChannelID: channel.ID,
			Title:     channel.Title,
			Followers: h.Store.CountFollowers(channel.ID),
		}
		if current, ok := h.Store.CurrentStreamSession(channel.ID); ok {
			entry.LiveViewers = current.PeakConcurrent
		}
		sessions, err := h.Store.ListStreamSessions(channel.ID)
		if err != nil {
			return analyticsOverviewResponse{}, err
		}
		if len(sessions) > 0 {
			totalMinutes := 0.0
			for _, session := range sessions {
				totalMinutes += sessionDurationMinutes(session, now)
				summary.WatchTimeMinutes += streamWatchOverlapMinutes(session, windowStart, now)
			}
			entry.AvgWatchMinutes = totalMinutes / float64(len(sessions))
		}
		messages, err := h.Store.ListChatMessages(channel.ID, 0)
		if err != nil {
			return analyticsOverviewResponse{}, err
		}
		today := 0
		for _, message := range messages {
			if message.CreatedAt.Before(startOfDay) {
				break
			}
			today++
		}
		entry.ChatMessages = today
		summary.ChatMessages += today
		summary.LiveViewers += entry.LiveViewers
		perChannel = append(perChannel, entry)
	}
	streamsLive := int(metrics.Default().ActiveStreams())
	if streamsLive <= 0 {
		count := 0
		for _, channel := range channels {
			state := strings.ToLower(strings.TrimSpace(channel.LiveState))
			if state == "live" || state == "starting" {
				count++
			}
		}
		streamsLive = count
	}
	summary.StreamsLive = streamsLive
	sort.Slice(perChannel, func(i, j int) bool {
		if perChannel[i].LiveViewers != perChannel[j].LiveViewers {
			return perChannel[i].LiveViewers > perChannel[j].LiveViewers
		}
		if perChannel[i].Followers != perChannel[j].Followers {
			return perChannel[i].Followers > perChannel[j].Followers
		}
		return perChannel[i].Title < perChannel[j].Title
	})
	resp := analyticsOverviewResponse{PerChannel: perChannel}
	if len(perChannel) > 0 || summary.LiveViewers > 0 || summary.StreamsLive > 0 || summary.WatchTimeMinutes > 0 || summary.ChatMessages > 0 {
		resp.Summary = &summary
	}
	return resp, nil
}

func sessionDurationMinutes(session models.StreamSession, now time.Time) float64 {
	end := now
	if session.EndedAt != nil && session.EndedAt.Before(end) {
		end = *session.EndedAt
	}
	if end.Before(session.StartedAt) {
		return 0
	}
	return end.Sub(session.StartedAt).Minutes()
}

func streamWatchOverlapMinutes(session models.StreamSession, windowStart, windowEnd time.Time) float64 {
	start := session.StartedAt
	if start.Before(windowStart) {
		start = windowStart
	}
	end := windowEnd
	if session.EndedAt != nil && session.EndedAt.Before(end) {
		end = *session.EndedAt
	}
	if end.Before(windowStart) {
		return 0
	}
	if end.After(windowEnd) {
		end = windowEnd
	}
	if !end.After(start) {
		return 0
	}
	return end.Sub(start).Minutes()
}
