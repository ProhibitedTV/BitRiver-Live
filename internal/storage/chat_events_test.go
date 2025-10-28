package storage

import (
	"context"
	"testing"
	"time"

	"bitriver-live/internal/chat"
)

func TestApplyChatEventPersistsMessage(t *testing.T) {
	store := newTestStore(t)
	user, err := store.CreateUser(CreateUserParams{DisplayName: "viewer", Email: "viewer@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channelOwner, err := store.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	channel, err := store.CreateChannel(channelOwner.ID, "Lobby", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	evt := chat.Event{
		Type: chat.EventTypeMessage,
		Message: &chat.MessageEvent{
			ID:        "msg-1",
			ChannelID: channel.ID,
			UserID:    user.ID,
			Content:   "hello",
			CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		},
		OccurredAt: time.Now().UTC(),
	}
	if err := store.ApplyChatEvent(evt); err != nil {
		t.Fatalf("ApplyChatEvent: %v", err)
	}

	messages, err := store.ListChatMessages(channel.ID, 0)
	if err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].ID != evt.Message.ID || messages[0].Content != evt.Message.Content {
		t.Fatalf("unexpected message payload: %+v", messages[0])
	}
}

func TestChatRestrictionsReflectModeration(t *testing.T) {
	store := newTestStore(t)
	owner, err := store.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	target, err := store.CreateUser(CreateUserParams{DisplayName: "target", Email: "target@example.com"})
	if err != nil {
		t.Fatalf("CreateUser target: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

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
		if err := store.ApplyChatEvent(evt); err != nil {
			t.Fatalf("ApplyChatEvent(%s): %v", evt.Type, err)
		}
	}

	snapshot := store.ChatRestrictions()
	if _, banned := snapshot.Bans[channel.ID][target.ID]; !banned {
		t.Fatalf("expected target to be banned")
	}
	if actor := snapshot.BanActors[channel.ID][target.ID]; actor != owner.ID {
		t.Fatalf("expected ban actor %q, got %q", owner.ID, actor)
	}
	if reason := snapshot.BanReasons[channel.ID][target.ID]; reason != "spam" {
		t.Fatalf("expected ban reason to persist, got %q", reason)
	}
	timeoutExpiry, ok := snapshot.Timeouts[channel.ID][target.ID]
	if !ok || timeoutExpiry.Before(expiry.Add(-time.Second)) {
		t.Fatalf("expected timeout to be recorded")
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
		if err := store.ApplyChatEvent(evt); err != nil {
			t.Fatalf("ApplyChatEvent(%s): %v", evt.Type, err)
		}
	}

	snapshot = store.ChatRestrictions()
	if _, banned := snapshot.Bans[channel.ID][target.ID]; banned {
		t.Fatalf("expected ban removal")
	}
	if _, muted := snapshot.Timeouts[channel.ID][target.ID]; muted {
		t.Fatalf("expected timeout removal")
	}
}

func TestApplyChatEventPersistsReport(t *testing.T) {
	store := newTestStore(t)
	owner, err := store.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	reporter, err := store.CreateUser(CreateUserParams{DisplayName: "reporter", Email: "reporter@example.com"})
	if err != nil {
		t.Fatalf("CreateUser reporter: %v", err)
	}
	target, err := store.CreateUser(CreateUserParams{DisplayName: "target", Email: "target@example.com"})
	if err != nil {
		t.Fatalf("CreateUser target: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	createdAt := time.Now().UTC().Truncate(time.Millisecond)
	evt := chat.Event{
		Type: chat.EventTypeReport,
		Report: &chat.ReportEvent{
			ID:         "rep-1",
			ChannelID:  channel.ID,
			ReporterID: reporter.ID,
			TargetID:   target.ID,
			Reason:     "harassment",
			CreatedAt:  createdAt,
		},
		OccurredAt: createdAt,
	}
	if err := store.ApplyChatEvent(evt); err != nil {
		t.Fatalf("ApplyChatEvent(report): %v", err)
	}

	reports, err := store.ListChatReports(channel.ID, false)
	if err != nil {
		t.Fatalf("ListChatReports: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	report := reports[0]
	if report.ID != evt.Report.ID || report.ReporterID != reporter.ID || report.Status != "open" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestChatWorkerProcessesQueue(t *testing.T) {
	store := newTestStore(t)
	owner, err := store.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	viewer, err := store.CreateUser(CreateUserParams{DisplayName: "viewer", Email: "viewer@example.com"})
	if err != nil {
		t.Fatalf("CreateUser viewer: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	queue := chat.NewMemoryQueue(8)
	worker := NewChatWorker(store, queue, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go worker.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	messageEvt := chat.Event{
		Type: chat.EventTypeMessage,
		Message: &chat.MessageEvent{
			ID:        "evt-1",
			ChannelID: channel.ID,
			UserID:    viewer.ID,
			Content:   "hey",
			CreatedAt: time.Now().UTC(),
		},
		OccurredAt: time.Now().UTC(),
	}
	if err := queue.Publish(ctx, messageEvt); err != nil {
		t.Fatalf("Publish message: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		messages, err := store.ListChatMessages(channel.ID, 0)
		if err != nil {
			t.Fatalf("ListChatMessages: %v", err)
		}
		if len(messages) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for chat persistence")
		case <-time.After(20 * time.Millisecond):
		}
	}
}
