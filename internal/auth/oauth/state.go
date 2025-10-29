package oauth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// StateData stores metadata associated with an OAuth state value.
type StateData struct {
	Provider string
	ReturnTo string
	Expires  time.Time
}

// StateStore tracks OAuth state parameters until they are redeemed.
type StateStore interface {
	Put(state string, data StateData, ttl time.Duration) error
	Take(state string) (StateData, bool)
}

// memoryStateStore keeps state in memory with expiry.
type memoryStateStore struct {
	mu    sync.Mutex
	items map[string]StateData
}

// NewMemoryStateStore constructs an in-memory store for state parameters.
func NewMemoryStateStore() StateStore {
	return &memoryStateStore{items: make(map[string]StateData)}
}

func (s *memoryStateStore) Put(state string, data StateData, ttl time.Duration) error {
	if state == "" {
		return fmt.Errorf("state token is required")
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data.Expires = time.Now().Add(ttl)
	s.items[state] = data
	s.pruneLocked()
	return nil
}

func (s *memoryStateStore) Take(state string) (StateData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	data, ok := s.items[state]
	if ok {
		delete(s.items, state)
	}
	if !ok {
		return StateData{}, false
	}
	if !data.Expires.IsZero() && time.Now().After(data.Expires) {
		return StateData{}, false
	}
	return data, true
}

func (s *memoryStateStore) pruneLocked() {
	now := time.Now()
	for key, item := range s.items {
		if !item.Expires.IsZero() && now.After(item.Expires) {
			delete(s.items, key)
		}
	}
}

// GenerateState creates a cryptographically random state string.
func GenerateState() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
