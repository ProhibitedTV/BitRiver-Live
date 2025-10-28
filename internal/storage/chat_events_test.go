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
	RunRepositoryChatRestrictionsLifecycle(t, jsonRepositoryFactory)
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
