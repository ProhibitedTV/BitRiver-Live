package auth

import (
	"context"
	"sync"
	"time"
)

// MemorySessionStore keeps session state in-memory. It is safe for concurrent use
// and primarily intended for development or single-instance deployments.
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]SessionRecord
}

// NewMemorySessionStore constructs an in-memory store implementation.
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{sessions: make(map[string]SessionRecord)}
}

// Save records the session details for the provided token.
func (s *MemorySessionStore) Save(token, userID string, expiresAt time.Time) error {
	s.mu.Lock()
	s.sessions[token] = SessionRecord{Token: token, UserID: userID, ExpiresAt: expiresAt}
	s.mu.Unlock()
	return nil
}

// Get retrieves the session record for the provided token.
func (s *MemorySessionStore) Get(token string) (SessionRecord, bool, error) {
	s.mu.RLock()
	record, ok := s.sessions[token]
	s.mu.RUnlock()
	return record, ok, nil
}

// Delete removes the session token from the store.
func (s *MemorySessionStore) Delete(token string) error {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
	return nil
}

// PurgeExpired removes any expired sessions from the store.
func (s *MemorySessionStore) PurgeExpired(now time.Time) error {
	s.mu.Lock()
	for token, record := range s.sessions {
		if now.After(record.ExpiresAt) {
			delete(s.sessions, token)
		}
	}
	s.mu.Unlock()
	return nil
}

// Ping always reports success for the in-memory session store.
func (s *MemorySessionStore) Ping(context.Context) error {
	return nil
}
