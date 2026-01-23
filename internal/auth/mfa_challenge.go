package auth

import (
	"context"
	"errors"
	"time"
)

// MFAChallengeStore defines the persistence contract for MFA challenge tokens.
type MFAChallengeStore interface {
	Save(token, userID string, expiresAt time.Time) error
	Get(token string) (MFAChallengeRecord, bool, error)
	Delete(token string) error
	PurgeExpired(now time.Time) error
}

// MFAChallengeRecord captures a stored MFA challenge entry.
type MFAChallengeRecord struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
}

// MFAChallengeOption configures a MFAChallengeManager instance.
type MFAChallengeOption func(*MFAChallengeManager)

// WithMFAChallengeStore injects a custom MFAChallengeStore implementation.
func WithMFAChallengeStore(store MFAChallengeStore) MFAChallengeOption {
	return func(m *MFAChallengeManager) {
		m.store = store
	}
}

// WithMFAChallengeTokenLength sets the token length used for MFA challenges.
func WithMFAChallengeTokenLength(length int) MFAChallengeOption {
	return func(m *MFAChallengeManager) {
		if length > 0 {
			m.tokenLength = length
		}
	}
}

// MFAChallengeManager coordinates MFA challenge creation and validation.
type MFAChallengeManager struct {
	store        MFAChallengeStore
	ttl          time.Duration
	tokenLength  int
	tokenFactory func(int) (string, error)
}

// NewMFAChallengeManager constructs a manager with the provided TTL and options.
func NewMFAChallengeManager(ttl time.Duration, opts ...MFAChallengeOption) *MFAChallengeManager {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	manager := &MFAChallengeManager{
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
		manager.store = NewMemoryMFAChallengeStore()
	}
	return manager
}

// Create issues a new MFA challenge token for the provided user identifier.
func (m *MFAChallengeManager) Create(userID string) (string, time.Time, error) {
	if userID == "" {
		return "", time.Time{}, ErrInvalidUserID
	}
	token, err := m.tokenFactory(m.tokenLength)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(m.ttl).UTC()
	if err := m.store.Save(token, userID, expiresAt); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// Validate checks the backing store for the provided token and returns the associated user when valid.
func (m *MFAChallengeManager) Validate(token string) (string, time.Time, bool, error) {
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

// Revoke deletes the MFA challenge token from the backing store.
func (m *MFAChallengeManager) Revoke(token string) error {
	if token == "" {
		return nil
	}
	return m.store.Delete(token)
}

// PurgeExpired removes any expired challenges from the backing store.
func (m *MFAChallengeManager) PurgeExpired() error {
	return m.store.PurgeExpired(time.Now())
}

// Ping verifies the underlying store is reachable when it exposes a ping method.
func (m *MFAChallengeManager) Ping(ctx context.Context) error {
	if m == nil || m.store == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if pinger, ok := m.store.(interface{ Ping(context.Context) error }); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

var ErrInvalidMFAChallenge = errors.New("invalid or expired mfa challenge")
