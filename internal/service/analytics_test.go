package service

import (
	"sort"
	"strings"
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

func (s analyticsStoreStub) CountFollowersByChannelIDs(channelIDs []string) map[string]int {
	counts := make(map[string]int, len(channelIDs))
	for _, channelID := range channelIDs {
		counts[channelID] = s.followerCounts[channelID]
	}
	return counts
}

func (s analyticsStoreStub) CurrentStreamSessionsByChannelIDs(channelIDs []string) map[string]domain.StreamSession {
	grouped := make(map[string]domain.StreamSession)
	for _, channelID := range channelIDs {
		if session, ok := s.current[channelID]; ok {
			grouped[channelID] = session
		}
	}
	return grouped
}

func (s analyticsStoreStub) ListStreamSessionsByChannelIDs(channelIDs []string) (map[string][]domain.StreamSession, error) {
	grouped := make(map[string][]domain.StreamSession, len(channelIDs))
	for _, channelID := range channelIDs {
		sessions := s.sessions[channelID]
		clones := make([]domain.StreamSession, len(sessions))
		copy(clones, sessions)
		grouped[channelID] = clones
	}
	return grouped, nil
}

func (s analyticsStoreStub) CountChatMessagesSinceByChannelIDs(channelIDs []string, since time.Time) (map[string]int, error) {
	counts := make(map[string]int, len(channelIDs))
	for _, channelID := range channelIDs {
		total := 0
		for _, message := range s.chatMessages[channelID] {
			if !message.CreatedAt.Before(since) {
				total++
			}
		}
		counts[channelID] = total
	}
	return counts, nil
}

func computeAnalyticsOverviewLegacy(store analyticsStoreStub, now time.Time) (AnalyticsOverview, error) {
	channels := store.ListChannels("", "")
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	windowStart := now.Add(-24 * time.Hour)
	summary := AnalyticsSummary{}
	perChannel := make([]AnalyticsChannelOverview, 0, len(channels))
	for _, channel := range channels {
		entry := AnalyticsChannelOverview{ChannelID: channel.ID, Title: channel.Title, Followers: store.followerCounts[channel.ID]}
		if current, ok := store.current[channel.ID]; ok {
			entry.LiveViewers = current.PeakConcurrent
		}
		sessions := store.sessions[channel.ID]
		if len(sessions) > 0 {
			totalMinutes := 0.0
			for _, session := range sessions {
				totalMinutes += sessionDurationMinutes(session, now)
				summary.WatchTimeMinutes += streamWatchOverlapMinutes(session, windowStart, now)
			}
			entry.AvgWatchMinutes = totalMinutes / float64(len(sessions))
		}
		today := 0
		for _, message := range store.chatMessages[channel.ID] {
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
	resp := AnalyticsOverview{PerChannel: perChannel}
	if len(perChannel) > 0 || summary.LiveViewers > 0 || summary.StreamsLive > 0 || summary.WatchTimeMinutes > 0 || summary.ChatMessages > 0 {
		resp.Summary = &summary
	}
	return resp, nil
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

func TestComputeAnalyticsOverviewMatchesLegacyMultiChannelFixture(t *testing.T) {
	originalMetrics := metrics.Default()
	metrics.SetDefault(metrics.New())
	t.Cleanup(func() { metrics.SetDefault(originalMetrics) })

	now := time.Date(2024, time.March, 20, 18, 0, 0, 0, time.UTC)
	toPtr := func(ts time.Time) *time.Time { return &ts }
	fixture := analyticsStoreStub{
		channels: []domain.Channel{
			{ID: "ch-1", Title: "Alpha", LiveState: "live"},
			{ID: "ch-2", Title: "Beta", LiveState: " offline "},
			{ID: "ch-3", Title: "Gamma", LiveState: "starting"},
		},
		followerCounts: map[string]int{"ch-1": 7, "ch-2": 7, "ch-3": 2},
		current: map[string]domain.StreamSession{
			"ch-1": {ID: "cur-1", ChannelID: "ch-1", PeakConcurrent: 22, StartedAt: now.Add(-45 * time.Minute)},
			"ch-3": {ID: "cur-3", ChannelID: "ch-3", PeakConcurrent: 22, StartedAt: now.Add(-30 * time.Minute)},
		},
		sessions: map[string][]domain.StreamSession{
			"ch-1": {
				{ID: "s-1", ChannelID: "ch-1", StartedAt: now.Add(-3 * time.Hour), EndedAt: toPtr(now.Add(-2 * time.Hour))},
				{ID: "s-2", ChannelID: "ch-1", StartedAt: now.Add(-90 * time.Minute)},
			},
			"ch-2": {
				{ID: "s-3", ChannelID: "ch-2", StartedAt: now.Add(-30 * time.Hour), EndedAt: toPtr(now.Add(-28 * time.Hour))},
			},
			"ch-3": {},
		},
		chatMessages: map[string][]domain.ChatMessage{
			"ch-1": {
				{ID: "m-1", ChannelID: "ch-1", CreatedAt: now.Add(-1 * time.Hour)},
				{ID: "m-2", ChannelID: "ch-1", CreatedAt: now.Add(-2 * time.Hour)},
				{ID: "m-3", ChannelID: "ch-1", CreatedAt: now.Add(-30 * time.Hour)},
			},
			"ch-2": {
				{ID: "m-4", ChannelID: "ch-2", CreatedAt: now.Add(-10 * time.Minute)},
			},
			"ch-3": {
				{ID: "m-5", ChannelID: "ch-3", CreatedAt: now.Add(-2 * time.Hour)},
			},
		},
	}

	legacy, err := computeAnalyticsOverviewLegacy(fixture, now)
	if err != nil {
		t.Fatalf("computeAnalyticsOverviewLegacy returned error: %v", err)
	}
	current, err := computeAnalyticsOverview(fixture, now)
	if err != nil {
		t.Fatalf("computeAnalyticsOverview returned error: %v", err)
	}

	if !analyticsOverviewEqual(legacy, current) {
		t.Fatalf("expected refactored overview to match legacy output\nlegacy: %#v\ncurrent: %#v", legacy, current)
	}
}

func analyticsOverviewEqual(left, right AnalyticsOverview) bool {
	if (left.Summary == nil) != (right.Summary == nil) {
		return false
	}
	if left.Summary != nil && *left.Summary != *right.Summary {
		return false
	}
	if len(left.PerChannel) != len(right.PerChannel) {
		return false
	}
	for i := range left.PerChannel {
		if left.PerChannel[i] != right.PerChannel[i] {
			return false
		}
	}
	return true
}
