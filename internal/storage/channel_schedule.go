package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"bitriver-live/internal/domain"
)

const (
	defaultScheduleDurationMinutes = 60
	maxChannelScheduleEntries      = 24
)

func normalizeChannelSchedule(entries []domain.ChannelScheduleEntry, existing []domain.ChannelScheduleEntry, now time.Time) ([]domain.ChannelScheduleEntry, error) {
	if len(entries) == 0 {
		return []domain.ChannelScheduleEntry{}, nil
	}
	if len(entries) > maxChannelScheduleEntries {
		return nil, fmt.Errorf("schedule cannot contain more than %d entries", maxChannelScheduleEntries)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	existingByID := make(map[string]domain.ChannelScheduleEntry, len(existing))
	for _, entry := range existing {
		id := strings.TrimSpace(entry.ID)
		if id != "" {
			existingByID[id] = entry
		}
	}
	seen := make(map[string]struct{}, len(entries))
	normalized := make([]domain.ChannelScheduleEntry, 0, len(entries))
	for _, entry := range entries {
		title := strings.TrimSpace(entry.Title)
		if title == "" {
			return nil, errors.New("schedule title is required")
		}
		startsAt := entry.StartsAt
		if startsAt.IsZero() {
			return nil, errors.New("schedule startsAt is required")
		}
		startsAt = startsAt.UTC()
		duration := entry.DurationMinutes
		if duration < 0 {
			return nil, errors.New("schedule durationMinutes cannot be negative")
		}
		if duration == 0 {
			duration = defaultScheduleDurationMinutes
		}
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			generated, err := generateID()
			if err != nil {
				return nil, err
			}
			id = generated
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("duplicate schedule id %s", id)
		}
		seen[id] = struct{}{}
		createdAt := entry.CreatedAt
		if existingEntry, ok := existingByID[id]; ok && !existingEntry.CreatedAt.IsZero() {
			createdAt = existingEntry.CreatedAt
		}
		if createdAt.IsZero() {
			createdAt = now
		} else {
			createdAt = createdAt.UTC()
		}
		normalized = append(normalized, domain.ChannelScheduleEntry{
			ID:              id,
			Title:           title,
			StartsAt:        startsAt,
			DurationMinutes: duration,
			Description:     strings.TrimSpace(entry.Description),
			CreatedAt:       createdAt,
			UpdatedAt:       now,
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].StartsAt.Equal(normalized[j].StartsAt) {
			return normalized[i].ID < normalized[j].ID
		}
		return normalized[i].StartsAt.Before(normalized[j].StartsAt)
	})
	return normalized, nil
}

func cloneChannelSchedule(entries []domain.ChannelScheduleEntry) []domain.ChannelScheduleEntry {
	if len(entries) == 0 {
		return []domain.ChannelScheduleEntry{}
	}
	cloned := make([]domain.ChannelScheduleEntry, len(entries))
	copy(cloned, entries)
	return cloned
}

func encodeChannelSchedule(entries []domain.ChannelScheduleEntry) ([]byte, error) {
	if entries == nil {
		entries = []domain.ChannelScheduleEntry{}
	}
	encoded, err := json.Marshal(entries)
	if err != nil {
		return nil, fmt.Errorf("encode channel schedule: %w", err)
	}
	return encoded, nil
}

func decodeChannelSchedule(encoded []byte) ([]domain.ChannelScheduleEntry, error) {
	if len(encoded) == 0 {
		return []domain.ChannelScheduleEntry{}, nil
	}
	var entries []domain.ChannelScheduleEntry
	if err := json.Unmarshal(encoded, &entries); err != nil {
		return nil, fmt.Errorf("decode channel schedule: %w", err)
	}
	for i := range entries {
		entries[i].StartsAt = entries[i].StartsAt.UTC()
		entries[i].CreatedAt = entries[i].CreatedAt.UTC()
		entries[i].UpdatedAt = entries[i].UpdatedAt.UTC()
	}
	return entries, nil
}
