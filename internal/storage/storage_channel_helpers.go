package storage

import (
	"sort"
	"strings"

	"bitriver-live/internal/domain"
)

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]struct{})
	for _, tag := range tags {
		trimmed := strings.TrimSpace(strings.ToLower(tag))
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	sort.Strings(normalized)
	return normalized
}

// UpdateChannel executes UpdateChannel.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: no external transaction is required; it coordinates access with
// the in-memory mutex and persists snapshots to disk/object storage as needed.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.

func channelMatchesQuery(channel domain.Channel, owner domain.User, normalizedQuery string) bool {
	if normalizedQuery == "" {
		return true
	}
	if strings.Contains(strings.ToLower(channel.Title), normalizedQuery) {
		return true
	}
	if strings.Contains(strings.ToLower(channel.Category), normalizedQuery) {
		return true
	}
	if owner.DisplayName != "" && strings.Contains(strings.ToLower(owner.DisplayName), normalizedQuery) {
		return true
	}
	for _, tag := range channel.Tags {
		if strings.Contains(strings.ToLower(tag), normalizedQuery) {
			return true
		}
	}
	return false
}

// FollowChannel records that a viewer is following the channel. The operation is idempotent.
