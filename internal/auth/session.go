package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// SessionStore defines the persistence contract for session tokens.
type SessionStore interface {
	Save(token, userID string, expiresAt time.Time) error
	Get(token string) (SessionRecord, bool, error)
	Delete(token string) error
	PurgeExpired(now time.Time) error
}

// SessionRecord captures a session row retrieved from the backing store.
type SessionRecord struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
}

// SessionOption configures a SessionManager instance.
type SessionOption func(*SessionManager)

// WithStore injects a custom SessionStore implementation.
func WithStore(store SessionStore) SessionOption {
	return func(m *SessionManager) {
		m.store = store
	}
}

// WithTokenLength sets the token length used for newly created sessions.
func WithTokenLength(length int) SessionOption {
	return func(m *SessionManager) {
		if length > 0 {
			m.tokenLength = length
		}
	}
}

// SessionManager coordinates session creation and validation against a backing store.
type SessionManager struct {
	store        SessionStore
	ttl          time.Duration
	tokenLength  int
	tokenFactory func(int) (string, error)
}

// NewSessionManager constructs a SessionManager with the provided TTL and options.
// The manager defaults to an in-memory store for local development when no store is supplied.
func NewSessionManager(ttl time.Duration, opts ...SessionOption) *SessionManager {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	manager := &SessionManager{
		ttl:          ttl,
		tokenLength:  32,
		tokenFactory: generateToken,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(manager)
		}
	}
	if manager.store == nil {
		manager.store = NewMemorySessionStore()
	}
	return manager
}

// Create issues a new session token for the provided user identifier.
func (m *SessionManager) Create(userID string) (string, time.Time, error) {
	if userID == "" {
		return "", time.Time{}, ErrInvalidUserID
	}
	token, err := m.tokenFactory(m.tokenLength)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(m.ttl)
	if err := m.store.Save(token, userID, expiresAt.UTC()); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// Validate checks the backing store for the provided token and returns the associated user when valid.
func (m *SessionManager) Validate(token string) (string, time.Time, bool, error) {
	if token == "" {
		return "", time.Time{}, false, nil
	}
	record, ok, err := m.store.Get(token)
	if err != nil {
		return "", time.Time{}, false, err
	}
	if !ok {
		return "", time.Time{}, false, nil
	}
	if time.Now().After(record.ExpiresAt) {
		_ = m.store.Delete(token)
		return "", time.Time{}, false, nil
	}
	return record.UserID, record.ExpiresAt, true, nil
}

// Revoke deletes the session token from the backing store.
func (m *SessionManager) Revoke(token string) error {
	if token == "" {
		return nil
	}
	return m.store.Delete(token)
}

// PurgeExpired removes any expired sessions from the backing store.
func (m *SessionManager) PurgeExpired() error {
	return m.store.PurgeExpired(time.Now())
}

// Ping verifies the underlying session store is reachable when it exposes a ping method.
func (m *SessionManager) Ping(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if m.store == nil {
		return nil
	}
	if pinger, ok := m.store.(interface{ Ping(context.Context) error }); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func generateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// ErrInvalidUserID is returned when attempting to create a session without a user identifier.
var ErrInvalidUserID = errors.New("userID is required")
