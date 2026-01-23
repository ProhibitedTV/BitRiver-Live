package auth

import (
	"sync"
	"time"
)

// MemoryMFAChallengeStore stores MFA challenges in memory for local development.
type MemoryMFAChallengeStore struct {
	mu      sync.RWMutex
	records map[string]MFAChallengeRecord
}

// NewMemoryMFAChallengeStore constructs an in-memory MFA challenge store.
func NewMemoryMFAChallengeStore() *MemoryMFAChallengeStore {
	return &MemoryMFAChallengeStore{
		records: make(map[string]MFAChallengeRecord),
	}
}

// Save stores or updates the MFA challenge token.
func (s *MemoryMFAChallengeStore) Save(token, userID string, expiresAt time.Time) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.records == nil {
		s.records = make(map[string]MFAChallengeRecord)
	}
	s.records[token] = MFAChallengeRecord{
		Token:     token,
		UserID:    userID,
		ExpiresAt: expiresAt.UTC(),
	}
	return nil
}

// Get fetches the MFA challenge details for the provided token.
func (s *MemoryMFAChallengeStore) Get(token string) (MFAChallengeRecord, bool, error) {
	if s == nil {
		return MFAChallengeRecord{}, false, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[token]
	if !ok {
		return MFAChallengeRecord{}, false, nil
	}
	return record, true, nil
}

// Delete removes the MFA challenge token.
func (s *MemoryMFAChallengeStore) Delete(token string) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, token)
	return nil
}

// PurgeExpired deletes expired MFA challenges from the table.
func (s *MemoryMFAChallengeStore) PurgeExpired(now time.Time) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, record := range s.records {
		if now.After(record.ExpiresAt) {
			delete(s.records, token)
		}
	}
	return nil
}
