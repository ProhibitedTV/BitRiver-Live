package chat

import (
	"context"
	"errors"
	"sync"
)

// Queue fan-outs chat events to interested subscribers. The implementation is
// intentionally minimal to support in-memory deployments and fakes used in
// integration tests.
type Queue interface {
	Publish(ctx context.Context, event Event) error
	Subscribe() Subscription
}

// Subscription represents an active event stream.
type Subscription interface {
	Events() <-chan Event
	Close()
}

// NewMemoryQueue initialises an in-memory fan-out queue suitable for tests and
// single-process deployments.
func NewMemoryQueue(buffer int) Queue {
	if buffer <= 0 {
		buffer = 32
	}
	return &memoryQueue{
		subs:   make(map[*memorySubscription]struct{}),
		buffer: buffer,
	}
}

type memoryQueue struct {
	mu     sync.RWMutex
	subs   map[*memorySubscription]struct{}
	buffer int
}

func (q *memoryQueue) Publish(ctx context.Context, event Event) error {
	if event.Type == "" {
		return errors.New("event type is required")
	}
	q.mu.RLock()
	defer q.mu.RUnlock()
	for sub := range q.subs {
		select {
		case sub.ch <- event:
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Drop instead of blocking to keep the live path
			// responsive. Consumers are expected to drain promptly.
		}
	}
	return nil
}

func (q *memoryQueue) Subscribe() Subscription {
	sub := &memorySubscription{
		queue: q,
		ch:    make(chan Event, q.buffer),
	}
	q.mu.Lock()
	q.subs[sub] = struct{}{}
	q.mu.Unlock()
	return sub
}

type memorySubscription struct {
	once  sync.Once
	queue *memoryQueue
	ch    chan Event
}

func (s *memorySubscription) Events() <-chan Event {
	return s.ch
}

func (s *memorySubscription) Close() {
	s.once.Do(func() {
		s.queue.mu.Lock()
		delete(s.queue.subs, s)
		s.queue.mu.Unlock()
		close(s.ch)
	})
}
