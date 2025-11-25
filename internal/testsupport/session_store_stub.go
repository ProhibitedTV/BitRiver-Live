package testsupport

import (
	"context"
	"sync"
	"time"

	"bitriver-live/internal/auth"
)

// SessionStoreStub is an in-memory auth.SessionStore implementation intended for tests.
// It allows seeding records with custom expirations and inspecting stored tokens.
type SessionStoreStub struct {
	mu       sync.RWMutex
	sessions map[string]auth.SessionRecord
}

// NewSessionStoreStub constructs a SessionStoreStub with empty state.
func NewSessionStoreStub() *SessionStoreStub {
	return &SessionStoreStub{sessions: make(map[string]auth.SessionRecord)}
}

// Save records the session details for the provided token.
func (s *SessionStoreStub) Save(token, userID string, expiresAt time.Time) error {
	s.mu.Lock()
	s.sessions[token] = auth.SessionRecord{Token: token, UserID: userID, ExpiresAt: expiresAt.UTC()}
	s.mu.Unlock()
	return nil
}

// Get retrieves the session record for the provided token.
func (s *SessionStoreStub) Get(token string) (auth.SessionRecord, bool, error) {
	s.mu.RLock()
	record, ok := s.sessions[token]
	s.mu.RUnlock()
	return record, ok, nil
}

// Delete removes the session token from the store.
func (s *SessionStoreStub) Delete(token string) error {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
	return nil
}

// PurgeExpired removes sessions that have passed their expiration.
func (s *SessionStoreStub) PurgeExpired(now time.Time) error {
	s.mu.Lock()
	for token, record := range s.sessions {
		if now.After(record.ExpiresAt) {
			delete(s.sessions, token)
		}
	}
	s.mu.Unlock()
	return nil
}

// Seed inserts a session record with the provided values, overriding any existing entry.
func (s *SessionStoreStub) Seed(token, userID string, expiresAt time.Time) {
	s.mu.Lock()
	s.sessions[token] = auth.SessionRecord{Token: token, UserID: userID, ExpiresAt: expiresAt.UTC()}
	s.mu.Unlock()
}

// Record looks up a token and returns the stored SessionRecord when present.
func (s *SessionStoreStub) Record(token string) (auth.SessionRecord, bool) {
	s.mu.RLock()
	record, ok := s.sessions[token]
	s.mu.RUnlock()
	return record, ok
}

// Ping reports success for compatibility with SessionManager health checks.
func (s *SessionStoreStub) Ping(context.Context) error { return nil }
