package api

import (
	"testing"
	"time"

	"bitriver-live/internal/models"
	"bitriver-live/internal/storage"
)

type analyticsRepository struct {
	storage.Repository
	channels       []models.Channel
	sessions       map[string][]models.StreamSession
	chatMessages   map[string][]models.ChatMessage
	followerCounts map[string]int
	current        map[string]models.StreamSession
}

func (r analyticsRepository) ListChannels(_, _ string) []models.Channel {
	return append([]models.Channel(nil), r.channels...)
}

func (r analyticsRepository) CountFollowers(channelID string) int {
	return r.followerCounts[channelID]
}

func (r analyticsRepository) CurrentStreamSession(channelID string) (models.StreamSession, bool) {
	session, ok := r.current[channelID]
	return session, ok
}

func (r analyticsRepository) ListStreamSessions(channelID string) ([]models.StreamSession, error) {
	sessions := r.sessions[channelID]
	clones := make([]models.StreamSession, len(sessions))
	copy(clones, sessions)
	return clones, nil
}

func (r analyticsRepository) ListChatMessages(channelID string, _ int) ([]models.ChatMessage, error) {
	messages := r.chatMessages[channelID]
	clones := make([]models.ChatMessage, len(messages))
	copy(clones, messages)
	return clones, nil
}

func TestComputeAnalyticsOverviewStreamWindows(t *testing.T) {
	handler, _ := newTestHandler(t)

	now := time.Date(2024, time.March, 15, 12, 0, 0, 0, time.UTC)
	offset := func(d time.Duration) time.Time { return now.Add(d) }
	tt := func(t time.Time) *time.Time { return &t }

	channelA := models.Channel{ID: "channel-a", Title: "Alpha"}
	channelB := models.Channel{ID: "channel-b", Title: "Beta"}

	handler.Store = analyticsRepository{
		Repository: handler.Store,
		channels:   []models.Channel{channelA, channelB},
		followerCounts: map[string]int{
			channelA.ID: 5,
			channelB.ID: 3,
		},
		sessions: map[string][]models.StreamSession{
			channelA.ID: {
				{ID: "session-a1", ChannelID: channelA.ID, StartedAt: offset(-2 * time.Hour), EndedAt: tt(offset(-1 * time.Hour))},
				{ID: "session-a2", ChannelID: channelA.ID, StartedAt: offset(-26 * time.Hour), EndedAt: tt(offset(-25 * time.Hour))},
				{ID: "session-a3", ChannelID: channelA.ID, StartedAt: offset(-30 * time.Minute)},
			},
			channelB.ID: {
				{ID: "session-b1", ChannelID: channelB.ID, StartedAt: offset(-25 * time.Hour), EndedAt: tt(offset(-20 * time.Hour))},
				{ID: "session-b2", ChannelID: channelB.ID, StartedAt: offset(1 * time.Hour), EndedAt: tt(offset(2 * time.Hour))},
				{ID: "session-b3", ChannelID: channelB.ID, StartedAt: offset(-1 * time.Hour), EndedAt: tt(offset(-2 * time.Hour))},
			},
		},
		chatMessages: map[string][]models.ChatMessage{},
		current:      map[string]models.StreamSession{},
	}

	resp, err := handler.computeAnalyticsOverview(now)
	if err != nil {
		t.Fatalf("computeAnalyticsOverview returned error: %v", err)
	}

	if resp.Summary == nil {
		t.Fatalf("expected summary to be populated")
	}
	const expectedWatchMinutes = 330.0
	if resp.Summary.WatchTimeMinutes != expectedWatchMinutes {
		t.Fatalf("expected watch time %.1f, got %.1f", expectedWatchMinutes, resp.Summary.WatchTimeMinutes)
	}

	if len(resp.PerChannel) != 2 {
		t.Fatalf("expected analytics for 2 channels, got %d", len(resp.PerChannel))
	}
	perChannel := make(map[string]analyticsChannelResponse)
	for _, entry := range resp.PerChannel {
		perChannel[entry.ChannelID] = entry
	}

	channelAResp := perChannel[channelA.ID]
	if channelAResp.AvgWatchMinutes != 50 {
		t.Fatalf("expected channel A avg watch 50, got %.1f", channelAResp.AvgWatchMinutes)
	}

	channelBResp := perChannel[channelB.ID]
	if channelBResp.AvgWatchMinutes != 100 {
		t.Fatalf("expected channel B avg watch 100, got %.1f", channelBResp.AvgWatchMinutes)
	}
}
