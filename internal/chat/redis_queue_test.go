package chat

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	redis "github.com/redis/go-redis/v9"

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

func TestRedisQueueRequeueFailureLeavesPending(t *testing.T) {
	srv, err := redisstub.Start(redisstub.Options{Password: "secret"})
	if err != nil {
		t.Fatalf("failed to start redis stub: %v", err)
	}
	t.Cleanup(func() {
		_ = srv.Close()
	})

	queueIface, err := NewRedisQueue(RedisQueueConfig{
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

	rq, ok := queueIface.(*redisQueue)
	if !ok {
		t.Fatalf("unexpected queue implementation %T", queueIface)
	}
	failClient := newFailingXAddClient(rq.client, 2)
	rq.client = failClient

	sub := queueIface.Subscribe()

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

	if err := queueIface.Publish(context.Background(), event1); err != nil {
		t.Fatalf("publish event1: %v", err)
	}
	if err := queueIface.Publish(context.Background(), event2); err != nil {
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

	// Wait for the requeue attempts to complete before inspecting ack behaviour.
	time.Sleep(100 * time.Millisecond)

	acks := failClient.AckIDs()
	if len(acks) != 1 {
		t.Fatalf("expected exactly 1 acked entry, got %d", len(acks))
	}
	if acks[0] == "" {
		t.Fatalf("recorded ack id should not be empty")
	}
}

func TestRedisQueueEnsureGroupRecoversAfterTransientFailure(t *testing.T) {
	srv, err := redisstub.Start(redisstub.Options{Password: "secret"})
	if err != nil {
		t.Fatalf("failed to start redis stub: %v", err)
	}
	t.Cleanup(func() {
		_ = srv.Close()
	})

	delegate, err := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{srv.Addr()},
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("create redis client: %v", err)
	}
	client := newFlakyGroupClient(delegate)

	queue := &redisQueue{
		client:       client,
		stream:       "test-stream",
		group:        "test-group",
		blockTimeout: 50 * time.Millisecond,
		buffer:       4,
	}

	ctx := context.Background()
	event := Event{
		Type: EventTypeMessage,
		Message: &MessageEvent{
			ID:        "evt-flaky",
			ChannelID: "channel-1",
			UserID:    "user-1",
			Content:   "hello",
			CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		},
		OccurredAt: time.Now().UTC(),
	}

	if err := queue.Publish(ctx, event); !errors.Is(err, errTransientGroupCreate) {
		t.Fatalf("expected transient error on first publish, got %v", err)
	}

	if err := queue.Publish(ctx, event); err != nil {
		t.Fatalf("publish after transient failure: %v", err)
	}

	sub := queue.Subscribe()
	t.Cleanup(func() {
		sub.Close()
	})

	select {
	case got := <-sub.Events():
		if got.Message == nil || got.Message.ID != event.Message.ID {
			t.Fatalf("unexpected event: %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for event after recovery")
	}
}

var errTransientGroupCreate = errors.New("transient group creation failure")
var errSimulatedXAddFailure = errors.New("simulated xadd failure")

type flakyGroupClient struct {
	delegate redis.UniversalClient
	mu       sync.Mutex
	failNext bool
}

func newFlakyGroupClient(delegate redis.UniversalClient) *flakyGroupClient {
	return &flakyGroupClient{delegate: delegate, failNext: true}
}

func (c *flakyGroupClient) Close() error {
	return nil
}

func (c *flakyGroupClient) Do(ctx context.Context, args ...interface{}) (interface{}, error) {
	if c.shouldFail(args) {
		return nil, errTransientGroupCreate
	}
	return c.delegate.Do(ctx, args...)
}

func (c *flakyGroupClient) shouldFail(args []interface{}) bool {
	if len(args) == 0 {
		return false
	}
	cmd, _ := args[0].(string)
	if !strings.EqualFold(cmd, "xgroup") {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failNext {
		c.failNext = false
		return true
	}
	return false
}

type failingXAddClient struct {
	redis.UniversalClient
	mu      sync.Mutex
	allowed int
	acks    []string
}

func newFailingXAddClient(delegate redis.UniversalClient, allowed int) *failingXAddClient {
	return &failingXAddClient{UniversalClient: delegate, allowed: allowed}
}

func (c *failingXAddClient) Do(ctx context.Context, args ...interface{}) (interface{}, error) {
	if len(args) > 0 {
		if cmd, _ := args[0].(string); strings.EqualFold(cmd, "xadd") {
			c.mu.Lock()
			c.allowed--
			fail := c.allowed < 0
			c.mu.Unlock()
			if fail {
				return nil, errSimulatedXAddFailure
			}
		} else if cmd, _ := args[0].(string); strings.EqualFold(cmd, "xack") {
			if len(args) > 3 {
				c.mu.Lock()
				for _, arg := range args[3:] {
					if id, ok := arg.(string); ok {
						c.acks = append(c.acks, id)
						continue
					}
					if raw, ok := arg.([]byte); ok {
						c.acks = append(c.acks, string(raw))
					}
				}
				c.mu.Unlock()
			}
		}
	}
	return c.UniversalClient.Do(ctx, args...)
}

func (c *failingXAddClient) AckIDs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := make([]string, len(c.acks))
	copy(ids, c.acks)
	return ids
}
