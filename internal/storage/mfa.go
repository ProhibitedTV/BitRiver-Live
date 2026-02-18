package storage

import (
	"fmt"
	"strings"
	"time"

	"bitriver-live/internal/domain"
)

// GetMFASettings returns stored MFA settings for the provided user.
func (s *Storage) GetMFASettings(userID string) (domain.MFASettings, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	settings, ok := s.data.MFASettings[userID]
	if !ok {
		return domain.MFASettings{}, false, nil
	}
	return settings, true, nil
}

// UpsertMFASettings creates or updates MFA settings for the provided user.
func (s *Storage) UpsertMFASettings(settings domain.MFASettings) (domain.MFASettings, error) {
	userID := strings.TrimSpace(settings.UserID)
	if userID == "" {
		return domain.MFASettings{}, fmt.Errorf("userID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Users[userID]; !ok {
		return domain.MFASettings{}, fmt.Errorf("user %s not found", userID)
	}

	updatedData := cloneDataset(s.data)
	now := time.Now().UTC()
	existing, ok := updatedData.MFASettings[userID]
	if ok && settings.CreatedAt.IsZero() {
		settings.CreatedAt = existing.CreatedAt
	}
	if settings.CreatedAt.IsZero() {
		settings.CreatedAt = now
	}
	settings.UpdatedAt = now
	settings.UserID = userID
	updatedData.MFASettings[userID] = settings

	if err := s.persistDataset(updatedData); err != nil {
		return domain.MFASettings{}, err
	}

	s.data = updatedData
	return settings, nil
}

// DeleteMFASettings removes MFA settings for the provided user.
func (s *Storage) DeleteMFASettings(userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.MFASettings[userID]; !ok {
		return nil
	}

	updatedData := cloneDataset(s.data)
	delete(updatedData.MFASettings, userID)

	if err := s.persistDataset(updatedData); err != nil {
		return err
	}

	s.data = updatedData
	return nil
}
