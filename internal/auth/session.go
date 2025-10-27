package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]session
	ttl      time.Duration
}

type session struct {
	userID    string
	expiresAt time.Time
}

func NewSessionManager(ttl time.Duration) *SessionManager {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &SessionManager{
		sessions: make(map[string]session),
		ttl:      ttl,
	}
}

func (m *SessionManager) Create(userID string) (string, time.Time, error) {
	if userID == "" {
		return "", time.Time{}, ErrInvalidUserID
	}
	token, err := generateToken(32)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(m.ttl)
	m.mu.Lock()
	m.sessions[token] = session{userID: userID, expiresAt: expiresAt}
	m.mu.Unlock()
	return token, expiresAt, nil
}

func (m *SessionManager) Validate(token string) (string, time.Time, bool) {
	if token == "" {
		return "", time.Time{}, false
	}
	m.mu.RLock()
	s, ok := m.sessions[token]
	m.mu.RUnlock()
	if !ok {
		return "", time.Time{}, false
	}
	if time.Now().After(s.expiresAt) {
		m.mu.Lock()
		delete(m.sessions, token)
		m.mu.Unlock()
		return "", time.Time{}, false
	}
	return s.userID, s.expiresAt, true
}

func (m *SessionManager) Revoke(token string) {
	if token == "" {
		return
	}
	m.mu.Lock()
	delete(m.sessions, token)
	m.mu.Unlock()
}

func (m *SessionManager) PurgeExpired() {
	now := time.Now()
	m.mu.Lock()
	for token, s := range m.sessions {
		if now.After(s.expiresAt) {
			delete(m.sessions, token)
		}
	}
	m.mu.Unlock()
}

func generateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

var ErrInvalidUserID = errors.New("userID is required")
