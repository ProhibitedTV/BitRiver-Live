package service

import (
	"testing"
	"time"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/observability/metrics"
)

type analyticsStoreStub struct {
	channels       []domain.Channel
	sessions       map[string][]domain.StreamSession
	chatMessages   map[string][]domain.ChatMessage
	followerCounts map[string]int
	current        map[string]domain.StreamSession
}

func (s analyticsStoreStub) ListChannels(_, _ string) []domain.Channel {
	return append([]domain.Channel(nil), s.channels...)
}

func (s analyticsStoreStub) CountFollowers(channelID string) int {
	return s.followerCounts[channelID]
}

func (s analyticsStoreStub) CurrentStreamSession(channelID string) (domain.StreamSession, bool) {
	session, ok := s.current[channelID]
	return session, ok
}

func (s analyticsStoreStub) ListStreamSessions(channelID string) ([]domain.StreamSession, error) {
	sessions := s.sessions[channelID]
	clones := make([]domain.StreamSession, len(sessions))
	copy(clones, sessions)
	return clones, nil
}

func (s analyticsStoreStub) ListChatMessages(channelID string, _ int) ([]domain.ChatMessage, error) {
	messages := s.chatMessages[channelID]
	clones := make([]domain.ChatMessage, len(messages))
	copy(clones, messages)
	return clones, nil
}

func TestComputeAnalyticsOverviewStreamWindows(t *testing.T) {
	now := time.Date(2024, time.March, 15, 12, 0, 0, 0, time.UTC)
	offset := func(d time.Duration) time.Time { return now.Add(d) }
	tt := func(t time.Time) *time.Time { return &t }

	channelA := domain.Channel{ID: "channel-a", Title: "Alpha"}
	channelB := domain.Channel{ID: "channel-b", Title: "Beta"}

	overview, err := computeAnalyticsOverview(analyticsStoreStub{
		channels: []domain.Channel{channelA, channelB},
		followerCounts: map[string]int{
			channelA.ID: 5,
			channelB.ID: 3,
		},
		sessions: map[string][]domain.StreamSession{
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
		chatMessages: map[string][]domain.ChatMessage{},
		current:      map[string]domain.StreamSession{},
	}, now)
	if err != nil {
		t.Fatalf("computeAnalyticsOverview returned error: %v", err)
	}

	if overview.Summary == nil {
		t.Fatalf("expected summary to be populated")
	}
	const expectedWatchMinutes = 330.0
	if overview.Summary.WatchTimeMinutes != expectedWatchMinutes {
		t.Fatalf("expected watch time %.1f, got %.1f", expectedWatchMinutes, overview.Summary.WatchTimeMinutes)
	}

	if len(overview.PerChannel) != 2 {
		t.Fatalf("expected analytics for 2 channels, got %d", len(overview.PerChannel))
	}
	perChannel := make(map[string]AnalyticsChannelOverview)
	for _, entry := range overview.PerChannel {
		perChannel[entry.ChannelID] = entry
	}

	channelAOverview := perChannel[channelA.ID]
	if channelAOverview.AvgWatchMinutes != 50 {
		t.Fatalf("expected channel A avg watch 50, got %.1f", channelAOverview.AvgWatchMinutes)
	}

	channelBOverview := perChannel[channelB.ID]
	if channelBOverview.AvgWatchMinutes != 100 {
		t.Fatalf("expected channel B avg watch 100, got %.1f", channelBOverview.AvgWatchMinutes)
	}
}

func TestComputeAnalyticsOverviewStreamsLiveFallback(t *testing.T) {
	originalMetrics := metrics.Default()
	metrics.SetDefault(metrics.New())
	t.Cleanup(func() { metrics.SetDefault(originalMetrics) })

	now := time.Date(2024, time.March, 15, 12, 0, 0, 0, time.UTC)
	liveState := "live"
	startingState := "starting"

	overview, err := computeAnalyticsOverview(analyticsStoreStub{
		channels: []domain.Channel{
			{ID: "live-1", Title: "Live One", LiveState: liveState},
			{ID: "start-1", Title: "Starting One", LiveState: startingState},
		},
		followerCounts: map[string]int{},
		sessions:       map[string][]domain.StreamSession{},
		chatMessages:   map[string][]domain.ChatMessage{},
		current:        map[string]domain.StreamSession{},
	}, now)
	if err != nil {
		t.Fatalf("computeAnalyticsOverview returned error: %v", err)
	}
	if overview.Summary == nil {
		t.Fatalf("expected summary")
	}
	if overview.Summary.StreamsLive != 2 {
		t.Fatalf("expected streams live fallback 2, got %d", overview.Summary.StreamsLive)
	}
}
