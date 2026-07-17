package chat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	redis "github.com/redis/go-redis/v9"

	"bitriver-live/internal/testsupport/redisstub"
)

func TestRedisQueueRequeuesOnCancellation(t *testing.T) {
	requeues := make(chan Event, 3)
	deliveries := make(chan []string, 1)

	srv, err := redisstub.Start(redisstub.Options{
		Password: "secret",
		Hooks: &redisstub.Hooks{
			XAdd: func(_ string, values map[string]string) {
				payload := values["payload"]
				if payload == "" {
					return
				}
				var evt Event
				if err := json.Unmarshal([]byte(payload), &evt); err != nil {
					return
				}
				requeues <- evt
			},
			XReadGroup: func(_ string, _ string, ids []string) {
				deliveries <- ids
			},
		},
	})
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
	rs, ok := sub.(*redisSubscription)
	if !ok {
		t.Fatalf("unexpected subscription type %T", sub)
	}

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

	waitForRead(t, deliveries, rs, 1)
	waitForBufferFill(t, rs, 1)

	var drained []Event

	sub.Close()

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

	waitForRequeue(t, requeues, event2.Message.ID)

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
	deliveries := make(chan []string, 1)

	srv, err := redisstub.Start(redisstub.Options{
		Password: "secret",
		Hooks: &redisstub.Hooks{
			XReadGroup: func(_ string, _ string, ids []string) {
				deliveries <- ids
			},
		},
	})
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
	rs, ok := sub.(*redisSubscription)
	if !ok {
		t.Fatalf("unexpected subscription type %T", sub)
	}

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

	waitForRead(t, deliveries, rs, 1)
	waitForBufferFill(t, rs, 1)

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

	ackIDs := failClient.AckIDs()
	if len(ackIDs) != 1 {
		t.Fatalf("expected exactly 1 acked entry, got %d", len(ackIDs))
	}
	if ackIDs[0] == "" {
		t.Fatalf("recorded ack id should not be empty")
	}
}

func waitForBufferFill(t *testing.T, sub *redisSubscription, expected int) {
	t.Helper()

	ticker := time.NewTicker(10 * time.Millisecond)
	timer := time.NewTimer(5 * time.Second)
	defer ticker.Stop()
	defer timer.Stop()

	for {
		select {
		case <-ticker.C:
			if len(sub.ch) >= expected {
				return
			}
		case <-timer.C:
			t.Fatalf("subscription buffer did not reach %d entries", expected)
		}
	}
}

func waitForRead(t *testing.T, deliveries <-chan []string, sub *redisSubscription, expected int) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer timer.Stop()
	defer ticker.Stop()

	for {
		select {
		case ids := <-deliveries:
			if len(ids) >= expected {
				return
			}
		case <-ticker.C:
			if sub != nil && len(sub.ch) >= expected {
				return
			}
		case <-timer.C:
			buffered := 0
			if sub != nil {
				buffered = len(sub.ch)
			}
			t.Fatalf("timed out waiting for %d deliveries (buffered=%d)", expected, buffered)
		}
	}
}

func waitForRequeue(t *testing.T, events <-chan Event, id string) {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	seen := 0
	for {
		select {
		case evt := <-events:
			if evt.Message != nil && evt.Message.ID == id {
				seen++
				if seen >= 1 {
					return
				}
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for requeue of %s (seen %d)", id, seen)
		}
	}
}

func TestBackoffIncreasesAndResets(t *testing.T) {
	backoff := newBackoff(10*time.Millisecond, 40*time.Millisecond)
	backoff.jitter = 0
	waits := make(chan time.Duration, 5)
	backoff.waitFn = func(d time.Duration) <-chan time.Time {
		waits <- d
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	}

	ctx := context.Background()
	for i := 0; i < 4; i++ {
		if err := backoff.Sleep(ctx); err != nil {
			t.Fatalf("sleep %d: %v", i, err)
		}
	}

	expected := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		40 * time.Millisecond,
		40 * time.Millisecond,
	}
	for i, want := range expected {
		select {
		case got := <-waits:
			if got != want {
				t.Fatalf("wait %d: expected %v, got %v", i, want, got)
			}
		default:
			t.Fatalf("wait %d: no delay recorded", i)
		}
	}

	backoff.Reset()
	if err := backoff.Sleep(ctx); err != nil {
		t.Fatalf("sleep after reset: %v", err)
	}
	select {
	case got := <-waits:
		if got != 10*time.Millisecond {
			t.Fatalf("expected base delay after reset, got %v", got)
		}
	default:
		t.Fatalf("expected delay after reset")
	}
}

func TestBackoffSleepRespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	backoff := newBackoff(10*time.Millisecond, time.Second)
	called := false
	backoff.waitFn = func(d time.Duration) <-chan time.Time {
		called = true
		return time.After(d)
	}

	if err := backoff.Sleep(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation error, got %v", err)
	}
	if called {
		t.Fatalf("waitFn should not be invoked when context is already canceled")
	}
}

func TestRedisSubscriptionExitsWhenContextCanceled(t *testing.T) {
	sub := &redisSubscription{
		queue: &redisQueue{},
		ch:    make(chan Event, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		sub.run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("subscription did not exit promptly after cancellation")
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

	delegate := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{srv.Addr()},
		Password: "secret",
	})
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

func TestAsBytes(t *testing.T) {
	t.Run("returns raw bytes without conversion", func(t *testing.T) {
		raw := []byte(`{"id":"evt-1"}`)
		got, ok := asBytes(raw)
		if !ok {
			t.Fatalf("expected ok for []byte input")
		}
		if len(got) != len(raw) {
			t.Fatalf("expected matching length, got %d want %d", len(got), len(raw))
		}
		if len(got) > 0 && &got[0] != &raw[0] {
			t.Fatalf("expected asBytes to return original []byte backing array")
		}
	})

	t.Run("encodes string to bytes", func(t *testing.T) {
		got, ok := asBytes("payload")
		if !ok {
			t.Fatalf("expected ok for string input")
		}
		if string(got) != "payload" {
			t.Fatalf("unexpected bytes: %q", string(got))
		}
	})

	t.Run("rejects unsupported type", func(t *testing.T) {
		got, ok := asBytes(123)
		if ok {
			t.Fatalf("expected unsupported input to return ok=false")
		}
		if got != nil {
			t.Fatalf("expected nil bytes for unsupported type")
		}
	})
}

func TestExtractPayload(t *testing.T) {
	t.Run("matches payload key case-insensitively and preserves byte value", func(t *testing.T) {
		payload := []byte(`{"message":"hello"}`)
		fields := []interface{}{[]byte("PAYLOAD"), payload}

		got := extractPayload(fields)
		if string(got) != string(payload) {
			t.Fatalf("unexpected payload: %q", string(got))
		}
		if len(got) > 0 && &got[0] != &payload[0] {
			t.Fatalf("expected byte payload to be returned without conversion")
		}
	})

	t.Run("converts string payload to bytes", func(t *testing.T) {
		fields := []interface{}{"payload", `{"event":"chat"}`}

		got := extractPayload(fields)
		if string(got) != `{"event":"chat"}` {
			t.Fatalf("unexpected payload: %q", string(got))
		}
	})

	t.Run("ignores non-matching and empty payload values", func(t *testing.T) {
		cases := []struct {
			name   string
			fields []interface{}
		}{
			{name: "missing payload key", fields: []interface{}{"other", "value"}},
			{name: "empty string payload", fields: []interface{}{"payload", ""}},
			{name: "empty bytes payload", fields: []interface{}{[]byte("payload"), []byte{}}},
			{name: "unsupported payload type", fields: []interface{}{"payload", 1}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := extractPayload(tc.fields); got != nil {
					t.Fatalf("expected nil payload, got %q", string(got))
				}
			})
		}
	})
}

var errTransientGroupCreate = errors.New("transient group creation failure")
var errSimulatedXAddFailure = errors.New("simulated xadd failure")

type flakyGroupClient struct {
	delegate redisCommandClient
	mu       sync.Mutex
	failNext bool
}

func newFlakyGroupClient(delegate redisCommandClient) *flakyGroupClient {
	return &flakyGroupClient{delegate: delegate, failNext: true}
}

func (c *flakyGroupClient) Close() error {
	return nil
}

func (c *flakyGroupClient) Do(ctx context.Context, args ...interface{}) *redis.Cmd {
	if c.shouldFail(args) {
		cmd := redis.NewCmd(ctx, args...)
		cmd.SetErr(errTransientGroupCreate)
		return cmd
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
	delegate redisCommandClient
	mu       sync.Mutex
	allowed  int
	acks     []string
}

func newFailingXAddClient(delegate redisCommandClient, allowed int) *failingXAddClient {
	return &failingXAddClient{delegate: delegate, allowed: allowed}
}

func (c *failingXAddClient) Do(ctx context.Context, args ...interface{}) *redis.Cmd {
	if len(args) > 0 {
		if cmd, _ := args[0].(string); strings.EqualFold(cmd, "xadd") {
			c.mu.Lock()
			c.allowed--
			fail := c.allowed < 0
			c.mu.Unlock()
			if fail {
				cmd := redis.NewCmd(ctx, args...)
				cmd.SetErr(errSimulatedXAddFailure)
				return cmd
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
	return c.delegate.Do(ctx, args...)
}

func (c *failingXAddClient) AckIDs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := make([]string, len(c.acks))
	copy(ids, c.acks)
	return ids
}
