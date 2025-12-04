package storage

import (
	"context"
	"errors"
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
	store := &recordingStore{Repository: newTestStore(t), applied: make(chan chat.Event, 1)}
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

	started := make(chan struct{})
	queue := newRecordingQueue()
	worker := NewChatWorker(store, queue, nil).WithStartedChannel(started)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go worker.Run(ctx)
	waitForSignal(t, started)

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

	waitForEvent(t, store.applied)

	messages, err := store.ListChatMessages(channel.ID, 0)
	if err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].ID != messageEvt.Message.ID || messages[0].Content != messageEvt.Message.Content {
		t.Fatalf("unexpected message payload: %+v", messages[0])
	}
}

func TestChatWorkerSkipsFailedStoreApply(t *testing.T) {
	baseStore := newTestStore(t)
	store := &recordingStore{
		Repository: baseStore,
		applyErr:   errors.New("cannot persist"),
		applied:    make(chan chat.Event, 1),
	}
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

	started := make(chan struct{})
	queue := newRecordingQueue()
	worker := NewChatWorker(store, queue, nil).WithStartedChannel(started)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go worker.Run(ctx)
	waitForSignal(t, started)

	messageEvt := chat.Event{
		Type: chat.EventTypeMessage,
		Message: &chat.MessageEvent{
			ID:        "evt-2",
			ChannelID: channel.ID,
			UserID:    viewer.ID,
			Content:   "hello",
			CreatedAt: time.Now().UTC(),
		},
		OccurredAt: time.Now().UTC(),
	}
	if err := queue.Publish(ctx, messageEvt); err != nil {
		t.Fatalf("Publish message: %v", err)
	}

	waitForEvent(t, store.applied)

	messages, err := baseStore.ListChatMessages(channel.ID, 0)
	if err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected no messages after failed apply, got %d", len(messages))
	}
}

func TestChatWorkerPublishError(t *testing.T) {
	queue := newRecordingQueue()
	queue.publishErr = errors.New("queue offline")

	err := queue.Publish(context.Background(), chat.Event{Type: chat.EventTypeMessage})
	if err == nil {
		t.Fatal("expected publish error")
	}
	if err.Error() != "queue offline" {
		t.Fatalf("unexpected publish error: %v", err)
	}
}

func waitForSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for worker start")
	}
}

func waitForEvent(t *testing.T, ch <-chan chat.Event) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

type recordingQueue struct {
	publishErr error
	events     chan chat.Event
}

func newRecordingQueue() *recordingQueue {
	return &recordingQueue{events: make(chan chat.Event, 4)}
}

func (q *recordingQueue) Publish(ctx context.Context, event chat.Event) error {
	if q.publishErr != nil {
		return q.publishErr
	}
	select {
	case q.events <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *recordingQueue) Subscribe() chat.Subscription {
	return &recordingSubscription{events: q.events}
}

type recordingSubscription struct {
	events chan chat.Event
	once   bool
}

func (s *recordingSubscription) Events() <-chan chat.Event {
	return s.events
}

func (s *recordingSubscription) Close() {
	if !s.once {
		close(s.events)
		s.once = true
	}
}

type recordingStore struct {
	Repository
	applyErr error
	applied  chan chat.Event
}

func (s *recordingStore) ApplyChatEvent(evt chat.Event) error {
	if s.applied != nil {
		s.applied <- evt
	}
	if s.applyErr != nil {
		return s.applyErr
	}
	return s.Repository.ApplyChatEvent(evt)
}
