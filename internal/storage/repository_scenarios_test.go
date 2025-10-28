package storage

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/ingest"
)

// RepositoryFactory constructs a repository backed by either the JSON store or
// Postgres implementation for cross-datastore scenario assertions.
type RepositoryFactory func(t *testing.T, opts ...Option) (Repository, func(), error)

func runRepository(t *testing.T, factory RepositoryFactory, opts ...Option) Repository {
	t.Helper()
	if factory == nil {
		t.Fatal("repository factory is required")
	}
	repo, cleanup, err := factory(t, opts...)
	if errors.Is(err, ErrPostgresUnavailable) {
		t.Skip("postgres repository unavailable")
	}
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	if repo == nil {
		t.Fatal("repository factory returned nil repository")
	}
	if cleanup != nil {
		t.Cleanup(cleanup)
	}
	return repo
}

func requireAvailable(t *testing.T, err error, operation string) {
	t.Helper()
	if errors.Is(err, ErrPostgresUnavailable) {
		t.Skip("postgres repository unavailable")
	}
	if err != nil {
		t.Fatalf("%s: %v", operation, err)
	}
}

// RunRepositoryChatRestrictionsLifecycle replays the moderation scenario
// exercised in chat_events_test.go against the provided repository.
func RunRepositoryChatRestrictionsLifecycle(t *testing.T, factory RepositoryFactory) {
	repo := runRepository(t, factory)

	owner, err := repo.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create owner")
	target, err := repo.CreateUser(CreateUserParams{DisplayName: "target", Email: "target@example.com"})
	requireAvailable(t, err, "create target")
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	requireAvailable(t, err, "create channel")

	expiry := time.Now().Add(time.Minute)
	events := []chat.Event{
		{
			Type: chat.EventTypeModeration,
			Moderation: &chat.ModerationEvent{
				Action:    chat.ModerationActionBan,
				ChannelID: channel.ID,
				ActorID:   owner.ID,
				TargetID:  target.ID,
				Reason:    "spam",
			},
			OccurredAt: time.Now().UTC(),
		},
		{
			Type: chat.EventTypeModeration,
			Moderation: &chat.ModerationEvent{
				Action:    chat.ModerationActionTimeout,
				ChannelID: channel.ID,
				ActorID:   owner.ID,
				TargetID:  target.ID,
				ExpiresAt: &expiry,
				Reason:    "caps",
			},
			OccurredAt: time.Now().UTC(),
		},
	}
	for _, evt := range events {
		requireAvailable(t, repo.ApplyChatEvent(evt), "apply moderation event")
	}

	snapshot := repo.ChatRestrictions()
	if _, banned := snapshot.Bans[channel.ID][target.ID]; !banned {
		t.Fatalf("expected target %q to be banned", target.ID)
	}
	if actor := snapshot.BanActors[channel.ID][target.ID]; actor != owner.ID {
		t.Fatalf("expected ban actor %q, got %q", owner.ID, actor)
	}
	if reason := snapshot.BanReasons[channel.ID][target.ID]; reason != "spam" {
		t.Fatalf("expected ban reason to persist, got %q", reason)
	}
	timeoutExpiry, ok := snapshot.Timeouts[channel.ID][target.ID]
	if !ok || timeoutExpiry.Before(expiry.Add(-time.Second)) {
		t.Fatalf("expected timeout to record expiry")
	}
	if actor := snapshot.TimeoutActors[channel.ID][target.ID]; actor != owner.ID {
		t.Fatalf("expected timeout actor %q, got %q", owner.ID, actor)
	}
	if reason := snapshot.TimeoutReasons[channel.ID][target.ID]; reason != "caps" {
		t.Fatalf("expected timeout reason to persist, got %q", reason)
	}
	if issued := snapshot.TimeoutIssuedAt[channel.ID][target.ID]; issued.IsZero() {
		t.Fatalf("expected timeout issued timestamp to be set")
	}

	clearEvents := []chat.Event{
		{
			Type: chat.EventTypeModeration,
			Moderation: &chat.ModerationEvent{
				Action:    chat.ModerationActionRemoveTimeout,
				ChannelID: channel.ID,
				ActorID:   owner.ID,
				TargetID:  target.ID,
				Reason:    "resolved",
			},
			OccurredAt: time.Now().UTC(),
		},
		{
			Type: chat.EventTypeModeration,
			Moderation: &chat.ModerationEvent{
				Action:    chat.ModerationActionUnban,
				ChannelID: channel.ID,
				ActorID:   owner.ID,
				TargetID:  target.ID,
				Reason:    "appeal",
			},
			OccurredAt: time.Now().UTC(),
		},
	}
	for _, evt := range clearEvents {
		requireAvailable(t, repo.ApplyChatEvent(evt), "clear moderation event")
	}

	snapshot = repo.ChatRestrictions()
	if _, banned := snapshot.Bans[channel.ID][target.ID]; banned {
		t.Fatalf("expected ban removal for %q", target.ID)
	}
	if _, muted := snapshot.Timeouts[channel.ID][target.ID]; muted {
		t.Fatalf("expected timeout removal for %q", target.ID)
	}
}

// RunRepositoryChatReportsLifecycle executes the chat report workflow from
// storage_test.go with the provided repository implementation.
func RunRepositoryChatReportsLifecycle(t *testing.T, factory RepositoryFactory) {
	repo := runRepository(t, factory)

	owner, err := repo.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create owner")
	reporter, err := repo.CreateUser(CreateUserParams{DisplayName: "reporter", Email: "reporter@example.com"})
	requireAvailable(t, err, "create reporter")
	target, err := repo.CreateUser(CreateUserParams{DisplayName: "target", Email: "target@example.com"})
	requireAvailable(t, err, "create target")
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	requireAvailable(t, err, "create channel")

	report, err := repo.CreateChatReport(channel.ID, reporter.ID, target.ID, "spam", "msg-1", "")
	requireAvailable(t, err, "create chat report")
	if report.Status != "open" {
		t.Fatalf("expected new report to be open, got %q", report.Status)
	}

	pending, err := repo.ListChatReports(channel.ID, false)
	requireAvailable(t, err, "list pending chat reports")
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending report, got %d", len(pending))
	}

	resolved, err := repo.ResolveChatReport(report.ID, owner.ID, "handled")
	requireAvailable(t, err, "resolve chat report")
	if resolved.Status != "resolved" || resolved.Resolution != "handled" {
		t.Fatalf("unexpected resolved payload: %+v", resolved)
	}

	pending, err = repo.ListChatReports(channel.ID, false)
	requireAvailable(t, err, "list pending chat reports after resolve")
	if len(pending) != 0 {
		t.Fatalf("expected no pending reports, got %d", len(pending))
	}

	all, err := repo.ListChatReports(channel.ID, true)
	requireAvailable(t, err, "list all chat reports")
	if len(all) != 1 {
		t.Fatalf("expected resolved report to be listed, got %d", len(all))
	}
}

// RunRepositoryTipsLifecycle asserts tip creation and listing behaviour against
// a repository implementation.
func RunRepositoryTipsLifecycle(t *testing.T, factory RepositoryFactory) {
	repo := runRepository(t, factory)

	owner, err := repo.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create owner")
	supporter, err := repo.CreateUser(CreateUserParams{DisplayName: "fan", Email: "fan@example.com"})
	requireAvailable(t, err, "create supporter")
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	requireAvailable(t, err, "create channel")

	tip, err := repo.CreateTip(CreateTipParams{
		ChannelID:  channel.ID,
		FromUserID: supporter.ID,
		Amount:     5.5,
		Currency:   "usd",
		Provider:   "stripe",
		Reference:  "ref-1",
		Message:    "keep it up",
	})
	requireAvailable(t, err, "create tip")
	if tip.ID == "" {
		t.Fatalf("expected tip id to be set")
	}

	tips, err := repo.ListTips(channel.ID, 10)
	requireAvailable(t, err, "list tips")
	if len(tips) != 1 || tips[0].ID != tip.ID {
		t.Fatalf("expected persisted tip, got %+v", tips)
	}
}

// RunRepositorySubscriptionsLifecycle validates the subscription lifecycle for
// a repository implementation.
func RunRepositorySubscriptionsLifecycle(t *testing.T, factory RepositoryFactory) {
	repo := runRepository(t, factory)

	owner, err := repo.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create owner")
	viewer, err := repo.CreateUser(CreateUserParams{DisplayName: "viewer", Email: "viewer@example.com"})
	requireAvailable(t, err, "create viewer")
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	requireAvailable(t, err, "create channel")

	sub, err := repo.CreateSubscription(CreateSubscriptionParams{
		ChannelID: channel.ID,
		UserID:    viewer.ID,
		Tier:      "tier1",
		Provider:  "stripe",
		Reference: "sub-1",
		Amount:    4.99,
		Currency:  "usd",
		Duration:  time.Hour,
		AutoRenew: true,
	})
	requireAvailable(t, err, "create subscription")
	if sub.ID == "" {
		t.Fatalf("expected subscription id to be set")
	}

	subs, err := repo.ListSubscriptions(channel.ID, false)
	requireAvailable(t, err, "list subscriptions")
	if len(subs) != 1 || subs[0].ID != sub.ID {
		t.Fatalf("expected subscription listing to include created subscription, got %+v", subs)
	}

	stored, ok := repo.GetSubscription(sub.ID)
	if !ok {
		t.Fatalf("expected GetSubscription to find %q", sub.ID)
	}
	if stored.ID != sub.ID || stored.Status != "active" {
		t.Fatalf("unexpected stored subscription: %+v", stored)
	}

	cancelled, err := repo.CancelSubscription(sub.ID, owner.ID, "fraud")
	requireAvailable(t, err, "cancel subscription")
	if cancelled.Status != "cancelled" {
		t.Fatalf("expected subscription to be cancelled, got status %q", cancelled.Status)
	}

	all, err := repo.ListSubscriptions(channel.ID, true)
	requireAvailable(t, err, "list all subscriptions")
	if len(all) != 1 || all[0].Status != "cancelled" {
		t.Fatalf("expected cancelled subscription to be returned, got %+v", all)
	}
}

// RunRepositoryIngestHealthSnapshots verifies that repositories persist ingest
// health snapshots provided by the configured ingest controller.
func RunRepositoryIngestHealthSnapshots(t *testing.T, factory RepositoryFactory) {
	responses := [][]ingest.HealthStatus{
		{{Component: "srs", Status: "ok"}},
		{{Component: "transcoder", Status: "error", Detail: "timeout"}},
	}
	fake := &fakeIngestController{healthResponses: responses}
	repo := runRepository(t, factory, WithIngestController(fake))

	first := repo.IngestHealth(context.Background())
	if fake.healthCalls == 0 {
		if _, isPostgres := repo.(*postgresRepository); isPostgres {
			t.Skip("postgres repository ingest health not yet implemented")
		}
		t.Fatalf("expected ingest controller to be queried")
	}
	if !reflect.DeepEqual(first, responses[0]) {
		t.Fatalf("unexpected health payload: %+v", first)
	}
	recorded, ts1 := repo.LastIngestHealth()
	if !reflect.DeepEqual(recorded, first) {
		t.Fatalf("expected last health to match recorded snapshot")
	}
	if ts1.IsZero() {
		t.Fatal("expected health timestamp to be set")
	}

	second := repo.IngestHealth(context.Background())
	if fake.healthCalls < 2 {
		t.Fatalf("expected subsequent health call to increment counter, got %d", fake.healthCalls)
	}
	if !reflect.DeepEqual(second, responses[1]) {
		t.Fatalf("unexpected second health payload: %+v", second)
	}
	recorded, ts2 := repo.LastIngestHealth()
	if !reflect.DeepEqual(recorded, second) {
		t.Fatalf("expected snapshot to update on subsequent call")
	}
	if ts2.Before(ts1) {
		t.Fatal("expected subsequent health timestamp to be >= initial timestamp")
	}
}

// RunRepositoryRecordingRetention validates the retention workflow that purges
// expired recordings and associated artefacts.
func RunRepositoryRecordingRetention(t *testing.T, factory RepositoryFactory) {
	policy := RecordingRetentionPolicy{Published: 200 * time.Millisecond, Unpublished: 200 * time.Millisecond}
	controller := &fakeIngestController{bootResponses: []bootResponse{{result: ingest.BootResult{
		Renditions: []ingest.Rendition{{Name: "720p", ManifestURL: "https://origin/720p.m3u8"}},
	}}}}
	objectConfig := WithObjectStorage(ObjectStorageConfig{
		Bucket:         "vod",
		Prefix:         "vod/assets",
		PublicEndpoint: "https://cdn.example.com/content",
	})

	repo := runRepository(t, factory, WithRecordingRetention(policy), WithIngestController(controller), objectConfig)
	fakeStorage := &fakeObjectStorage{prefix: "vod/assets", baseURL: "https://cdn.example.com/content"}
	switch r := repo.(type) {
	case *Storage:
		r.objectClient = fakeStorage
	case *postgresRepository:
		r.objectClient = fakeStorage
	}

	owner, err := repo.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create owner")
	channel, err := repo.CreateChannel(owner.ID, "Speedrun", "gaming", nil)
	requireAvailable(t, err, "create channel")
	_, err = repo.StartStream(channel.ID, []string{"720p"})
	requireAvailable(t, err, "start stream")
	_, err = repo.StopStream(channel.ID, 10)
	requireAvailable(t, err, "stop stream")

	recordings, err := repo.ListRecordings(channel.ID, true)
	requireAvailable(t, err, "list recordings before retention")
	if len(recordings) != 1 {
		t.Fatalf("expected one recording before retention purge, got %d", len(recordings))
	}
	recordingID := recordings[0].ID

	clip, err := repo.CreateClipExport(recordingID, ClipExportParams{Title: "Intro", StartSeconds: 0, EndSeconds: 5})
	requireAvailable(t, err, "create clip export")

	clipObject := ""
	if store, ok := repo.(*Storage); ok {
		store.mu.Lock()
		stored := store.data.ClipExports[clip.ID]
		stored.StorageObject = buildObjectKey("clips", clip.ID+".mp4")
		store.data.ClipExports[clip.ID] = stored
		store.mu.Unlock()
		clipObject = stored.StorageObject
	}

	expectedDeletes := make(map[string]struct{})
	for _, upload := range fakeStorage.uploads {
		if strings.Contains(upload.Key, "/manifests/") || strings.Contains(upload.Key, "/thumbnails/") {
			expectedDeletes[upload.Key] = struct{}{}
		}
	}
	if clipObject != "" {
		expectedDeletes[clipObject] = struct{}{}
	}

	fakeStorage.deletes = nil
	time.Sleep(250 * time.Millisecond)

	recordings, err = repo.ListRecordings(channel.ID, true)
	requireAvailable(t, err, "list recordings after retention")
	if len(recordings) != 0 {
		t.Fatalf("expected retention to purge recordings, got %d", len(recordings))
	}

	for _, key := range fakeStorage.deletes {
		delete(expectedDeletes, key)
	}
	if len(expectedDeletes) != 0 {
		t.Fatalf("expected storage deletes for manifests, thumbnails, and clips; missing %v", expectedDeletes)
	}
}
