package chat

import (
	"context"
	"testing"
	"time"

	"bitriver-live/internal/testsupport/redisstub"
)

func TestRedisQueueRequeuesOnCancellation(t *testing.T) {
	srv, err := redisstub.Start(redisstub.Options{Password: "secret"})
	if err != nil {
		t.Fatalf("failed to start redis stub: %v", err)
	}
	t.Cleanup(func() {
		_ = srv.Close()
	})

	queue, err := NewRedisQueue(RedisQueueConfig{
		Addr:         srv.Addr(),
		Password:     "secret",
		Stream:       "test-stream",
		Group:        "test-group",
		BlockTimeout: 50 * time.Millisecond,
		Buffer:       1,
	})
	if err != nil {
		t.Fatalf("create queue: %v", err)
	}

	sub := queue.Subscribe()

	event1 := Event{
		Type: EventTypeMessage,
		Message: &MessageEvent{
			ID:        "evt-1",
			ChannelID: "channel-1",
			UserID:    "user-1",
			Content:   "buffer-fill",
			CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		},
		OccurredAt: time.Now().UTC(),
	}
	event2 := Event{
		Type: EventTypeMessage,
		Message: &MessageEvent{
			ID:        "evt-2",
			ChannelID: "channel-1",
			UserID:    "user-2",
			Content:   "needs-requeue",
			CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		},
		OccurredAt: time.Now().UTC(),
	}

	if err := queue.Publish(context.Background(), event1); err != nil {
		t.Fatalf("publish event1: %v", err)
	}
	if err := queue.Publish(context.Background(), event2); err != nil {
		t.Fatalf("publish event2: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	sub.Close()

	var drained []Event
	for evt := range sub.Events() {
		drained = append(drained, evt)
	}
	if len(drained) != 1 {
		t.Fatalf("expected 1 drained event, got %d", len(drained))
	}
	if drained[0].Message == nil || drained[0].Message.ID != event1.Message.ID {
		t.Fatalf("unexpected drained event: %+v", drained[0])
	}

	replacement := queue.Subscribe()
	t.Cleanup(func() {
		replacement.Close()
	})

	select {
	case got := <-replacement.Events():
		if got.Message == nil || got.Message.ID != event2.Message.ID {
			t.Fatalf("unexpected event: %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for requeued event")
	}
}
