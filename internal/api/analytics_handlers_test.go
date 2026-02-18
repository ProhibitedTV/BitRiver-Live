package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bitriver-live/internal/service"
	"bitriver-live/internal/storage"
)

type analyticsUseCaseStub struct {
	overview service.AnalyticsOverview
	err      error
}

func (s analyticsUseCaseStub) ComputeAnalyticsOverview(_ time.Time) (service.AnalyticsOverview, error) {
	return s.overview, s.err
}

func TestMapAnalyticsOverviewResponse(t *testing.T) {
	mapped := mapAnalyticsOverviewResponse(service.AnalyticsOverview{
		Summary: &service.AnalyticsSummary{
			LiveViewers:      12,
			StreamsLive:      3,
			WatchTimeMinutes: 420.5,
			ChatMessages:     18,
		},
		PerChannel: []service.AnalyticsChannelOverview{{
			ChannelID:       "channel-1",
			Title:           "Main",
			LiveViewers:     7,
			Followers:       101,
			AvgWatchMinutes: 33.3,
			ChatMessages:    11,
		}},
	})

	if mapped.Summary == nil {
		t.Fatal("expected summary")
	}
	if mapped.Summary.StreamsLive != 3 {
		t.Fatalf("expected streamsLive 3, got %d", mapped.Summary.StreamsLive)
	}
	if len(mapped.PerChannel) != 1 {
		t.Fatalf("expected one channel row, got %d", len(mapped.PerChannel))
	}
	if mapped.PerChannel[0].ChannelID != "channel-1" {
		t.Fatalf("expected channel-1, got %s", mapped.PerChannel[0].ChannelID)
	}
}

func TestAnalyticsOverviewHandlerMapsServicePayload(t *testing.T) {
	handler, store := newTestHandler(t)
	admin, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Admin", Email: "admin@example.com", Roles: []string{"admin"}})
	if err != nil {
		t.Fatalf("CreateUser admin: %v", err)
	}

	handler.Store = nil
	handler.AnalyticsService = analyticsUseCaseStub{overview: service.AnalyticsOverview{
		Summary: &service.AnalyticsSummary{StreamsLive: 2},
		PerChannel: []service.AnalyticsChannelOverview{{
			ChannelID:       "ch-1",
			Title:           "One",
			LiveViewers:     4,
			Followers:       6,
			AvgWatchMinutes: 10,
			ChatMessages:    3,
		}},
	}}

	req := httptest.NewRequest(http.MethodGet, "/api/analytics/overview", nil)
	req = withUser(req, admin)
	rec := httptest.NewRecorder()
	handler.AnalyticsOverview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload analyticsOverviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Summary == nil || payload.Summary.StreamsLive != 2 {
		t.Fatalf("expected summary streams live 2, got %+v", payload.Summary)
	}
	if len(payload.PerChannel) != 1 || payload.PerChannel[0].ChannelID != "ch-1" {
		t.Fatalf("unexpected perChannel payload: %+v", payload.PerChannel)
	}
}
