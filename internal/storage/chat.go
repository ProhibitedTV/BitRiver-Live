package storage

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"bitriver-live/internal/models"
)

// initChatDataset performs init chat dataset and propagates validation or dependency failures to the caller.
func initChatDataset(ds *dataset) {
	ds.ChatMessages = make(map[string]models.ChatMessage)
	ds.ChatBans = make(map[string]map[string]time.Time)
	ds.ChatTimeouts = make(map[string]map[string]time.Time)
	ds.ChatBanActors = make(map[string]map[string]string)
	ds.ChatBanReasons = make(map[string]map[string]string)
	ds.ChatTimeoutActors = make(map[string]map[string]string)
	ds.ChatTimeoutReasons = make(map[string]map[string]string)
	ds.ChatTimeoutIssuedAt = make(map[string]map[string]time.Time)
	ds.ChatReports = make(map[string]models.ChatReport)
	ds.Appeals = make(map[string]models.Appeal)
	ds.AppealEvents = make(map[string][]models.AppealEvent)
	ds.ChatFilters = make(map[string]models.ChatFilter)
	ds.ChatAutoModActions = make(map[string]models.ChatAutoModAction)
}

// ensureChatDatasetInitializedLocked performs ensure chat dataset initialized locked and propagates validation or dependency failures to the caller.
func (s *Storage) ensureChatDatasetInitializedLocked() {
	if s.data.ChatMessages == nil {
		s.data.ChatMessages = make(map[string]models.ChatMessage)
	}
	if s.data.ChatBans == nil {
		s.data.ChatBans = make(map[string]map[string]time.Time)
	}
	if s.data.ChatTimeouts == nil {
		s.data.ChatTimeouts = make(map[string]map[string]time.Time)
	}
	if s.data.ChatBanActors == nil {
		s.data.ChatBanActors = make(map[string]map[string]string)
	}
	if s.data.ChatBanReasons == nil {
		s.data.ChatBanReasons = make(map[string]map[string]string)
	}
	if s.data.ChatTimeoutActors == nil {
		s.data.ChatTimeoutActors = make(map[string]map[string]string)
	}
	if s.data.ChatTimeoutReasons == nil {
		s.data.ChatTimeoutReasons = make(map[string]map[string]string)
	}
	if s.data.ChatTimeoutIssuedAt == nil {
		s.data.ChatTimeoutIssuedAt = make(map[string]map[string]time.Time)
	}
	if s.data.ChatReports == nil {
		s.data.ChatReports = make(map[string]models.ChatReport)
	}
	if s.data.Appeals == nil {
		s.data.Appeals = make(map[string]models.Appeal)
	}
	if s.data.AppealEvents == nil {
		s.data.AppealEvents = make(map[string][]models.AppealEvent)
	}
	if s.data.ChatFilters == nil {
		s.data.ChatFilters = make(map[string]models.ChatFilter)
	}
	if s.data.ChatAutoModActions == nil {
		s.data.ChatAutoModActions = make(map[string]models.ChatAutoModAction)
	}
}

// cloneChatData performs clone chat data and propagates validation or dependency failures to the caller.
func cloneChatData(src dataset, clone *dataset) {
	if src.ChatMessages != nil {
		clone.ChatMessages = make(map[string]models.ChatMessage, len(src.ChatMessages))
		for id, message := range src.ChatMessages {
			clone.ChatMessages[id] = message
		}
	}

	if src.ChatBans != nil {
		clone.ChatBans = make(map[string]map[string]time.Time, len(src.ChatBans))
		for channelID, bans := range src.ChatBans {
			if bans == nil {
				clone.ChatBans[channelID] = nil
				continue
			}
			cloned := make(map[string]time.Time, len(bans))
			for userID, issuedAt := range bans {
				cloned[userID] = issuedAt
			}
			clone.ChatBans[channelID] = cloned
		}
	}

	if src.ChatTimeouts != nil {
		clone.ChatTimeouts = make(map[string]map[string]time.Time, len(src.ChatTimeouts))
		for channelID, timeouts := range src.ChatTimeouts {
			if timeouts == nil {
				clone.ChatTimeouts[channelID] = nil
				continue
			}
			cloned := make(map[string]time.Time, len(timeouts))
			for userID, expiry := range timeouts {
				cloned[userID] = expiry
			}
			clone.ChatTimeouts[channelID] = cloned
		}
	}

	if src.ChatBanActors != nil {
		clone.ChatBanActors = make(map[string]map[string]string, len(src.ChatBanActors))
		for channelID, actors := range src.ChatBanActors {
			if actors == nil {
				clone.ChatBanActors[channelID] = nil
				continue
			}
			cloned := make(map[string]string, len(actors))
			for userID, actorID := range actors {
				cloned[userID] = actorID
			}
			clone.ChatBanActors[channelID] = cloned
		}
	}

	if src.ChatBanReasons != nil {
		clone.ChatBanReasons = make(map[string]map[string]string, len(src.ChatBanReasons))
		for channelID, reasons := range src.ChatBanReasons {
			if reasons == nil {
				clone.ChatBanReasons[channelID] = nil
				continue
			}
			cloned := make(map[string]string, len(reasons))
			for userID, reason := range reasons {
				cloned[userID] = reason
			}
			clone.ChatBanReasons[channelID] = cloned
		}
	}

	if src.ChatTimeoutActors != nil {
		clone.ChatTimeoutActors = make(map[string]map[string]string, len(src.ChatTimeoutActors))
		for channelID, actors := range src.ChatTimeoutActors {
			if actors == nil {
				clone.ChatTimeoutActors[channelID] = nil
				continue
			}
			cloned := make(map[string]string, len(actors))
			for userID, actorID := range actors {
				cloned[userID] = actorID
			}
			clone.ChatTimeoutActors[channelID] = cloned
		}
	}

	if src.ChatTimeoutReasons != nil {
		clone.ChatTimeoutReasons = make(map[string]map[string]string, len(src.ChatTimeoutReasons))
		for channelID, reasons := range src.ChatTimeoutReasons {
			if reasons == nil {
				clone.ChatTimeoutReasons[channelID] = nil
				continue
			}
			cloned := make(map[string]string, len(reasons))
			for userID, reason := range reasons {
				cloned[userID] = reason
			}
			clone.ChatTimeoutReasons[channelID] = cloned
		}
	}

	if src.ChatTimeoutIssuedAt != nil {
		clone.ChatTimeoutIssuedAt = make(map[string]map[string]time.Time, len(src.ChatTimeoutIssuedAt))
		for channelID, issued := range src.ChatTimeoutIssuedAt {
			if issued == nil {
				clone.ChatTimeoutIssuedAt[channelID] = nil
				continue
			}
			cloned := make(map[string]time.Time, len(issued))
			for userID, ts := range issued {
				cloned[userID] = ts
			}
			clone.ChatTimeoutIssuedAt[channelID] = cloned
		}
	}

	if src.ChatReports != nil {
		clone.ChatReports = make(map[string]models.ChatReport, len(src.ChatReports))
		for id, report := range src.ChatReports {
			cloned := report
			if report.ResolvedAt != nil {
				resolved := *report.ResolvedAt
				cloned.ResolvedAt = &resolved
			}
			clone.ChatReports[id] = cloned
		}
	}

	if src.ChatFilters != nil {
		clone.ChatFilters = make(map[string]models.ChatFilter, len(src.ChatFilters))
		for id, filter := range src.ChatFilters {
			clone.ChatFilters[id] = filter
		}
	}

	if src.ChatAutoModActions != nil {
		clone.ChatAutoModActions = make(map[string]models.ChatAutoModAction, len(src.ChatAutoModActions))
		for id, action := range src.ChatAutoModActions {
			clone.ChatAutoModActions[id] = action
		}
	}
}

// ensureBanMetadata performs ensure ban metadata and propagates validation or dependency failures to the caller.
func (s *Storage) ensureBanMetadata(channelID string) {
	if s.data.ChatBanActors == nil {
		s.data.ChatBanActors = make(map[string]map[string]string)
	}
	if s.data.ChatBanActors[channelID] == nil {
		s.data.ChatBanActors[channelID] = make(map[string]string)
	}
	if s.data.ChatBanReasons == nil {
		s.data.ChatBanReasons = make(map[string]map[string]string)
	}
	if s.data.ChatBanReasons[channelID] == nil {
		s.data.ChatBanReasons[channelID] = make(map[string]string)
	}
}

// ensureTimeoutMetadata performs ensure timeout metadata and propagates validation or dependency failures to the caller.
func (s *Storage) ensureTimeoutMetadata(channelID string) {
	if s.data.ChatTimeoutActors == nil {
		s.data.ChatTimeoutActors = make(map[string]map[string]string)
	}
	if s.data.ChatTimeoutActors[channelID] == nil {
		s.data.ChatTimeoutActors[channelID] = make(map[string]string)
	}
	if s.data.ChatTimeoutReasons == nil {
		s.data.ChatTimeoutReasons = make(map[string]map[string]string)
	}
	if s.data.ChatTimeoutReasons[channelID] == nil {
		s.data.ChatTimeoutReasons[channelID] = make(map[string]string)
	}
	if s.data.ChatTimeoutIssuedAt == nil {
		s.data.ChatTimeoutIssuedAt = make(map[string]map[string]time.Time)
	}
	if s.data.ChatTimeoutIssuedAt[channelID] == nil {
		s.data.ChatTimeoutIssuedAt[channelID] = make(map[string]time.Time)
	}
}

// normalizeChatFilter performs normalize chat filter and propagates validation or dependency failures to the caller.
func normalizeChatFilter(kind, pattern string) (string, string, error) {
	normalizedKind := strings.ToLower(strings.TrimSpace(kind))
	trimmedPattern := strings.TrimSpace(pattern)
	if trimmedPattern == "" {
		return "", "", fmt.Errorf("pattern is required")
	}
	switch normalizedKind {
	case "word", "regex":
	default:
		return "", "", fmt.Errorf("filter kind must be word or regex")
	}
	if normalizedKind == "regex" {
		if _, err := regexp.Compile(trimmedPattern); err != nil {
			return "", "", fmt.Errorf("invalid regex pattern: %w", err)
		}
	}
	return normalizedKind, trimmedPattern, nil
}

// Chat operations

func (s *Storage) CreateChatMessage(channelID, userID, content string) (models.ChatMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return models.ChatMessage{}, fmt.Errorf("channel %s not found", channelID)
	}
	if _, ok := s.data.Users[userID]; !ok {
		return models.ChatMessage{}, fmt.Errorf("user %s not found", userID)
	}

	if err := s.ensureChatAccessLocked(channelID, userID); err != nil {
		return models.ChatMessage{}, err
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return models.ChatMessage{}, errors.New("message content cannot be empty")
	}
	if len([]rune(trimmed)) > MaxChatMessageLength {
		return models.ChatMessage{}, fmt.Errorf("message content exceeds %d characters", MaxChatMessageLength)
	}

	id, err := generateID()
	if err != nil {
		return models.ChatMessage{}, err
	}

	message := models.ChatMessage{
		ID:        id,
		ChannelID: channelID,
		UserID:    userID,
		Content:   trimmed,
		CreatedAt: time.Now().UTC(),
	}

	s.data.ChatMessages[id] = message
	if err := s.persist(); err != nil {
		delete(s.data.ChatMessages, id)
		return models.ChatMessage{}, err
	}

	return message, nil
}

// ensureChatAccessLocked performs ensure chat access locked and propagates validation or dependency failures to the caller.
func (s *Storage) ensureChatAccessLocked(channelID, userID string) error {
	if s.isChatBannedLocked(channelID, userID) {
		return fmt.Errorf("user is banned")
	}
	if expiry, ok := s.chatTimeoutLocked(channelID, userID); ok {
		if time.Now().UTC().Before(expiry) {
			return fmt.Errorf("user is timed out")
		}
		if err := s.removeChatTimeoutLocked(channelID, userID); err != nil {
			return err
		}
	}
	return nil
}

// removeChatTimeoutLocked performs remove chat timeout locked and propagates validation or dependency failures to the caller.
func (s *Storage) removeChatTimeoutLocked(channelID, userID string) error {
	var (
		previousExpiry time.Time
		hadExpiry      bool
		previousIssued time.Time
		hadIssued      bool
		previousActor  string
		hadActor       bool
		previousReason string
		hadReason      bool
	)

	if timeouts := s.data.ChatTimeouts[channelID]; timeouts != nil {
		if expiry, ok := timeouts[userID]; ok {
			previousExpiry = expiry
			hadExpiry = true
			delete(timeouts, userID)
			if len(timeouts) == 0 {
				delete(s.data.ChatTimeouts, channelID)
			}
		}
	}
	if issued := s.data.ChatTimeoutIssuedAt[channelID]; issued != nil {
		if ts, ok := issued[userID]; ok {
			previousIssued = ts
			hadIssued = true
			delete(issued, userID)
			if len(issued) == 0 {
				delete(s.data.ChatTimeoutIssuedAt, channelID)
			}
		}
	}
	if actors := s.data.ChatTimeoutActors[channelID]; actors != nil {
		if actor, ok := actors[userID]; ok {
			previousActor = actor
			hadActor = true
			delete(actors, userID)
			if len(actors) == 0 {
				delete(s.data.ChatTimeoutActors, channelID)
			}
		}
	}
	if reasons := s.data.ChatTimeoutReasons[channelID]; reasons != nil {
		if reason, ok := reasons[userID]; ok {
			previousReason = reason
			hadReason = true
			delete(reasons, userID)
			if len(reasons) == 0 {
				delete(s.data.ChatTimeoutReasons, channelID)
			}
		}
	}

	if !hadExpiry && !hadIssued && !hadActor && !hadReason {
		return nil
	}

	if err := s.persist(); err != nil {
		if hadExpiry {
			if s.data.ChatTimeouts == nil {
				s.data.ChatTimeouts = make(map[string]map[string]time.Time)
			}
			if s.data.ChatTimeouts[channelID] == nil {
				s.data.ChatTimeouts[channelID] = make(map[string]time.Time)
			}
			s.data.ChatTimeouts[channelID][userID] = previousExpiry
		}
		if hadIssued {
			if s.data.ChatTimeoutIssuedAt == nil {
				s.data.ChatTimeoutIssuedAt = make(map[string]map[string]time.Time)
			}
			if s.data.ChatTimeoutIssuedAt[channelID] == nil {
				s.data.ChatTimeoutIssuedAt[channelID] = make(map[string]time.Time)
			}
			s.data.ChatTimeoutIssuedAt[channelID][userID] = previousIssued
		}
		if hadActor {
			if s.data.ChatTimeoutActors == nil {
				s.data.ChatTimeoutActors = make(map[string]map[string]string)
			}
			if s.data.ChatTimeoutActors[channelID] == nil {
				s.data.ChatTimeoutActors[channelID] = make(map[string]string)
			}
			s.data.ChatTimeoutActors[channelID][userID] = previousActor
		}
		if hadReason {
			if s.data.ChatTimeoutReasons == nil {
				s.data.ChatTimeoutReasons = make(map[string]map[string]string)
			}
			if s.data.ChatTimeoutReasons[channelID] == nil {
				s.data.ChatTimeoutReasons[channelID] = make(map[string]string)
			}
			s.data.ChatTimeoutReasons[channelID][userID] = previousReason
		}
		return err
	}

	return nil
}

// isChatBannedLocked reports whether chat banned locked is satisfied for the current input.
func (s *Storage) isChatBannedLocked(channelID, userID string) bool {
	if bans := s.data.ChatBans[channelID]; bans != nil {
		if _, exists := bans[userID]; exists {
			return true
		}
	}
	return false
}

// chatTimeoutLocked performs chat timeout locked and propagates validation or dependency failures to the caller.
func (s *Storage) chatTimeoutLocked(channelID, userID string) (time.Time, bool) {
	if timeouts := s.data.ChatTimeouts[channelID]; timeouts != nil {
		expiry, ok := timeouts[userID]
		if ok {
			return expiry, true
		}
	}
	return time.Time{}, false
}

// purgeExpiredChatMessagesLocked performs purge expired chat messages locked and propagates validation or dependency failures to the caller.
func (s *Storage) purgeExpiredChatMessagesLocked(now time.Time) (bool, dataset, error) {
	retention := s.chatRetention.Messages
	if retention <= 0 || len(s.data.ChatMessages) == 0 {
		return false, dataset{}, nil
	}
	cutoff := now.Add(-retention)
	removed := false
	snapshotTaken := false
	var snapshot dataset
	for id, message := range s.data.ChatMessages {
		if message.CreatedAt.After(cutoff) {
			continue
		}
		if !snapshotTaken {
			snapshot = cloneDataset(s.data)
			snapshotTaken = true
		}
		delete(s.data.ChatMessages, id)
		removed = true
	}
	if !removed {
		return false, dataset{}, nil
	}
	return true, snapshot, nil
}

// purgeExpiredChatReportsLocked performs purge expired chat reports locked and propagates validation or dependency failures to the caller.
func (s *Storage) purgeExpiredChatReportsLocked(now time.Time) (bool, dataset, error) {
	retention := s.chatRetention.ModerationLogs
	if retention <= 0 || len(s.data.ChatReports) == 0 {
		return false, dataset{}, nil
	}
	cutoff := now.Add(-retention)
	removed := false
	snapshotTaken := false
	var snapshot dataset
	for id, report := range s.data.ChatReports {
		reportTime := report.CreatedAt
		if report.ResolvedAt != nil && report.ResolvedAt.After(reportTime) {
			reportTime = report.ResolvedAt.UTC()
		}
		if reportTime.After(cutoff) {
			continue
		}
		if !snapshotTaken {
			snapshot = cloneDataset(s.data)
			snapshotTaken = true
		}
		delete(s.data.ChatReports, id)
		removed = true
	}
	if !removed {
		return false, dataset{}, nil
	}
	return true, snapshot, nil
}

// purgeExpiredChatAutoModActionsLocked performs purge expired chat auto mod actions locked and propagates validation or dependency failures to the caller.
func (s *Storage) purgeExpiredChatAutoModActionsLocked(now time.Time) (bool, dataset, error) {
	retention := s.chatRetention.ModerationLogs
	if retention <= 0 || len(s.data.ChatAutoModActions) == 0 {
		return false, dataset{}, nil
	}
	cutoff := now.Add(-retention)
	removed := false
	snapshotTaken := false
	var snapshot dataset
	for id, action := range s.data.ChatAutoModActions {
		if action.CreatedAt.After(cutoff) {
			continue
		}
		if !snapshotTaken {
			snapshot = cloneDataset(s.data)
			snapshotTaken = true
		}
		delete(s.data.ChatAutoModActions, id)
		removed = true
	}
	if !removed {
		return false, dataset{}, nil
	}
	return true, snapshot, nil
}

// ListChatMessages returns chat messages from the configured backing services.
func (s *Storage) ListChatMessages(channelID string, limit int) ([]models.ChatMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	now := s.retentionTime()
	removed, snapshot, err := s.purgeExpiredChatMessagesLocked(now)
	if err != nil {
		return nil, err
	}
	if removed {
		if err := s.persist(); err != nil {
			s.data = snapshot
			return nil, err
		}
	}

	messages := make([]models.ChatMessage, 0)
	for _, message := range s.data.ChatMessages {
		if message.ChannelID == channelID {
			messages = append(messages, message)
		}
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].CreatedAt.After(messages[j].CreatedAt)
	})

	if limit > 0 && len(messages) > limit {
		messages = messages[:limit]
	}
	return messages, nil
}

// DeleteChatMessage removes a single chat message from the transcript.
func (s *Storage) DeleteChatMessage(channelID, messageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return fmt.Errorf("channel %s not found", channelID)
	}

	message, ok := s.data.ChatMessages[messageID]
	if !ok {
		return nil
	}
	if message.ChannelID != channelID {
		return nil
	}

	delete(s.data.ChatMessages, messageID)
	if err := s.persist(); err != nil {
		s.data.ChatMessages[messageID] = message
		return err
	}
	return nil
}

// pruneExpiredTimeoutsLocked performs prune expired timeouts locked and propagates validation or dependency failures to the caller.
func (s *Storage) pruneExpiredTimeoutsLocked(channelID string, now time.Time) bool {
	timeouts := s.data.ChatTimeouts[channelID]
	if len(timeouts) == 0 {
		return false
	}
	pruned := false
	for userID, expiry := range timeouts {
		if !expiry.Before(now) {
			continue
		}
		pruned = true
		delete(timeouts, userID)
		if len(timeouts) == 0 {
			delete(s.data.ChatTimeouts, channelID)
		}
		if issued := s.data.ChatTimeoutIssuedAt[channelID]; issued != nil {
			delete(issued, userID)
			if len(issued) == 0 {
				delete(s.data.ChatTimeoutIssuedAt, channelID)
			}
		}
		if actors := s.data.ChatTimeoutActors[channelID]; actors != nil {
			delete(actors, userID)
			if len(actors) == 0 {
				delete(s.data.ChatTimeoutActors, channelID)
			}
		}
		if reasons := s.data.ChatTimeoutReasons[channelID]; reasons != nil {
			delete(reasons, userID)
			if len(reasons) == 0 {
				delete(s.data.ChatTimeoutReasons, channelID)
			}
		}
	}
	return pruned
}

// ListChatRestrictions returns the current bans and timeouts for a channel.
func (s *Storage) ListChatRestrictions(channelID string) []models.ChatRestriction {
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	pruned := s.pruneExpiredTimeoutsLocked(channelID, now)
	restrictions := make([]models.ChatRestriction, 0)
	if bans := s.data.ChatBans[channelID]; bans != nil {
		for userID, issued := range bans {
			restriction := models.ChatRestriction{
				ID:        fmt.Sprintf("ban:%s:%s", channelID, userID),
				Type:      "ban",
				ChannelID: channelID,
				TargetID:  userID,
				IssuedAt:  issued,
				ActorID:   s.lookupBanActor(channelID, userID),
				Reason:    s.lookupBanReason(channelID, userID),
			}
			restrictions = append(restrictions, restriction)
		}
	}
	if timeouts := s.data.ChatTimeouts[channelID]; timeouts != nil {
		for userID, expiry := range timeouts {
			if !expiry.After(now) {
				continue
			}
			expiryUTC := expiry.UTC()
			issued := s.lookupTimeoutIssuedAt(channelID, userID, expiryUTC)
			expCopy := expiryUTC
			restriction := models.ChatRestriction{
				ID:        fmt.Sprintf("timeout:%s:%s", channelID, userID),
				Type:      "timeout",
				ChannelID: channelID,
				TargetID:  userID,
				IssuedAt:  issued,
				ExpiresAt: &expCopy,
				ActorID:   s.lookupTimeoutActor(channelID, userID),
				Reason:    s.lookupTimeoutReason(channelID, userID),
			}
			restrictions = append(restrictions, restriction)
		}
	}
	sort.Slice(restrictions, func(i, j int) bool {
		if restrictions[i].IssuedAt.Equal(restrictions[j].IssuedAt) {
			return restrictions[i].ID < restrictions[j].ID
		}
		return restrictions[i].IssuedAt.After(restrictions[j].IssuedAt)
	})
	if pruned {
		if err := s.persist(); err != nil {
			slog.Error("persist pruned chat timeouts", "err", err)
		}
	}
	return restrictions
}

// lookupBanActor performs lookup ban actor and propagates validation or dependency failures to the caller.
func (s *Storage) lookupBanActor(channelID, userID string) string {
	if actors := s.data.ChatBanActors[channelID]; actors != nil {
		return actors[userID]
	}
	return ""
}

// lookupBanReason performs lookup ban reason and propagates validation or dependency failures to the caller.
func (s *Storage) lookupBanReason(channelID, userID string) string {
	if reasons := s.data.ChatBanReasons[channelID]; reasons != nil {
		return reasons[userID]
	}
	return ""
}

// lookupTimeoutActor performs lookup timeout actor and propagates validation or dependency failures to the caller.
func (s *Storage) lookupTimeoutActor(channelID, userID string) string {
	if actors := s.data.ChatTimeoutActors[channelID]; actors != nil {
		return actors[userID]
	}
	return ""
}

// lookupTimeoutReason performs lookup timeout reason and propagates validation or dependency failures to the caller.
func (s *Storage) lookupTimeoutReason(channelID, userID string) string {
	if reasons := s.data.ChatTimeoutReasons[channelID]; reasons != nil {
		return reasons[userID]
	}
	return ""
}

// lookupTimeoutIssuedAt performs lookup timeout issued at and propagates validation or dependency failures to the caller.
func (s *Storage) lookupTimeoutIssuedAt(channelID, userID string, fallback time.Time) time.Time {
	if issued := s.data.ChatTimeoutIssuedAt[channelID]; issued != nil {
		if ts, ok := issued[userID]; ok {
			return ts
		}
	}
	return fallback
}

// CreateChatReport persists a moderation report filed by a viewer.
func (s *Storage) CreateChatReport(channelID, reporterID, targetID, reason, messageID, evidenceURL string) (models.ChatReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return models.ChatReport{}, fmt.Errorf("channel %s not found", channelID)
	}
	if _, ok := s.data.Users[reporterID]; !ok {
		return models.ChatReport{}, fmt.Errorf("reporter %s not found", reporterID)
	}
	if _, ok := s.data.Users[targetID]; !ok {
		return models.ChatReport{}, fmt.Errorf("target %s not found", targetID)
	}
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return models.ChatReport{}, fmt.Errorf("reason is required")
	}
	id, err := generateID()
	if err != nil {
		return models.ChatReport{}, err
	}
	now := time.Now().UTC()
	report := models.ChatReport{
		ID:          id,
		ChannelID:   channelID,
		ReporterID:  reporterID,
		TargetID:    targetID,
		Reason:      trimmedReason,
		MessageID:   strings.TrimSpace(messageID),
		EvidenceURL: strings.TrimSpace(evidenceURL),
		Status:      ChatReportStatusOpen,
		CreatedAt:   now,
	}
	if s.data.ChatReports == nil {
		s.data.ChatReports = make(map[string]models.ChatReport)
	}
	s.data.ChatReports[id] = report
	if err := s.persist(); err != nil {
		delete(s.data.ChatReports, id)
		return models.ChatReport{}, err
	}
	return report, nil
}

// ListChatReports lists reports for a channel.
func (s *Storage) ListChatReports(channelID string, includeResolved bool) ([]models.ChatReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	now := s.retentionTime()
	removed, snapshot, err := s.purgeExpiredChatReportsLocked(now)
	if err != nil {
		return nil, err
	}
	if removed {
		if err := s.persist(); err != nil {
			s.data = snapshot
			return nil, err
		}
	}

	reports := make([]models.ChatReport, 0)
	for _, report := range s.data.ChatReports {
		if report.ChannelID != channelID {
			continue
		}
		if !includeResolved && strings.EqualFold(report.Status, ChatReportStatusResolved) {
			continue
		}
		reports = append(reports, report)
	}
	sort.Slice(reports, func(i, j int) bool {
		if reports[i].CreatedAt.Equal(reports[j].CreatedAt) {
			return reports[i].ID < reports[j].ID
		}
		return reports[i].CreatedAt.After(reports[j].CreatedAt)
	})
	return reports, nil
}

// ResolveChatReport marks a report as addressed.
func (s *Storage) ResolveChatReport(reportID, resolverID, resolution string) (models.ChatReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	report, ok := s.data.ChatReports[reportID]
	if !ok {
		return models.ChatReport{}, fmt.Errorf("report %s not found", reportID)
	}
	if _, ok := s.data.Users[resolverID]; !ok {
		return models.ChatReport{}, fmt.Errorf("resolver %s not found", resolverID)
	}
	if strings.EqualFold(report.Status, ChatReportStatusResolved) {
		return report, nil
	}
	now := time.Now().UTC()
	trimmed := strings.TrimSpace(resolution)
	if trimmed == "" {
		trimmed = ChatReportStatusResolved
	}
	report.Status = ChatReportStatusResolved
	report.Resolution = trimmed
	report.ResolverID = resolverID
	report.ResolvedAt = &now
	s.data.ChatReports[reportID] = report
	if err := s.persist(); err != nil {
		return models.ChatReport{}, err
	}
	return report, nil
}

func (s *Storage) appendAppealEventLocked(appealID, actorID, action, note string, createdAt time.Time) error {
	eventID, err := generateID()
	if err != nil {
		return err
	}
	evt := models.AppealEvent{ID: eventID, AppealID: appealID, ActorID: actorID, Action: action, Note: strings.TrimSpace(note), CreatedAt: createdAt}
	s.data.AppealEvents[appealID] = append(s.data.AppealEvents[appealID], evt)
	appeal := s.data.Appeals[appealID]
	appeal.Events = append(appeal.Events, evt)
	s.data.Appeals[appealID] = appeal
	return nil
}

func (s *Storage) CreateAppeal(reportID, reporterID, reason string) (models.Appeal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	report, ok := s.data.ChatReports[reportID]
	if !ok {
		return models.Appeal{}, fmt.Errorf("report %s not found", reportID)
	}
	if _, ok := s.data.Users[reporterID]; !ok {
		return models.Appeal{}, fmt.Errorf("reporter %s not found", reporterID)
	}
	if report.ReporterID != reporterID {
		return models.Appeal{}, fmt.Errorf("forbidden")
	}
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return models.Appeal{}, fmt.Errorf("reason is required")
	}
	id, err := generateID()
	if err != nil {
		return models.Appeal{}, err
	}
	now := time.Now().UTC()
	appeal := models.Appeal{ID: id, ReportID: reportID, ChannelID: report.ChannelID, ReporterID: reporterID, Reason: trimmedReason, Status: AppealStatusOpen, CreatedAt: now}
	if s.data.Appeals == nil {
		s.data.Appeals = make(map[string]models.Appeal)
	}
	if s.data.AppealEvents == nil {
		s.data.AppealEvents = make(map[string][]models.AppealEvent)
	}
	s.data.Appeals[id] = appeal
	if err := s.appendAppealEventLocked(id, reporterID, "submitted", trimmedReason, now); err != nil {
		return models.Appeal{}, err
	}
	if err := s.persist(); err != nil {
		delete(s.data.Appeals, id)
		delete(s.data.AppealEvents, id)
		return models.Appeal{}, err
	}
	return s.data.Appeals[id], nil
}

func (s *Storage) ListAppeals(channelID, requesterID string, includeClosed bool) ([]models.Appeal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	appeals := make([]models.Appeal, 0)
	for _, appeal := range s.data.Appeals {
		if appeal.ChannelID != channelID {
			continue
		}
		if requesterID != "" && appeal.ReporterID != requesterID {
			continue
		}
		if !includeClosed && strings.EqualFold(appeal.Status, AppealStatusResolved) {
			continue
		}
		appeal.Events = append([]models.AppealEvent(nil), s.data.AppealEvents[appeal.ID]...)
		appeals = append(appeals, appeal)
	}
	sort.Slice(appeals, func(i, j int) bool {
		if appeals[i].CreatedAt.Equal(appeals[j].CreatedAt) {
			return appeals[i].ID < appeals[j].ID
		}
		return appeals[i].CreatedAt.After(appeals[j].CreatedAt)
	})
	return appeals, nil
}

func (s *Storage) ResolveAppeal(appealID, resolverID, resolution string) (models.Appeal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	appeal, ok := s.data.Appeals[appealID]
	if !ok {
		return models.Appeal{}, fmt.Errorf("appeal %s not found", appealID)
	}
	if _, ok := s.data.Users[resolverID]; !ok {
		return models.Appeal{}, fmt.Errorf("resolver %s not found", resolverID)
	}
	trimmed := strings.TrimSpace(resolution)
	if trimmed == "" {
		trimmed = AppealStatusResolved
	}
	now := time.Now().UTC()
	appeal.Status = AppealStatusResolved
	appeal.Resolution = trimmed
	appeal.ResolverID = resolverID
	appeal.ResolvedAt = &now
	s.data.Appeals[appealID] = appeal
	if err := s.appendAppealEventLocked(appealID, resolverID, "resolved", trimmed, now); err != nil {
		return models.Appeal{}, err
	}
	if err := s.persist(); err != nil {
		return models.Appeal{}, err
	}
	appeal.Events = append([]models.AppealEvent(nil), s.data.AppealEvents[appealID]...)
	return appeal, nil
}

func (s *Storage) ReopenAppeal(appealID, actorID, note string) (models.Appeal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	appeal, ok := s.data.Appeals[appealID]
	if !ok {
		return models.Appeal{}, fmt.Errorf("appeal %s not found", appealID)
	}
	if _, ok := s.data.Users[actorID]; !ok {
		return models.Appeal{}, fmt.Errorf("actor %s not found", actorID)
	}
	now := time.Now().UTC()
	appeal.Status = AppealStatusOpen
	appeal.Resolution = ""
	appeal.ResolverID = ""
	appeal.ResolvedAt = nil
	s.data.Appeals[appealID] = appeal
	if err := s.appendAppealEventLocked(appealID, actorID, "reopened", note, now); err != nil {
		return models.Appeal{}, err
	}
	if err := s.persist(); err != nil {
		return models.Appeal{}, err
	}
	appeal.Events = append([]models.AppealEvent(nil), s.data.AppealEvents[appealID]...)
	return appeal, nil
}

// ListChatFilters returns the configured auto-moderation filters for a channel.
func (s *Storage) ListChatFilters(channelID string) ([]models.ChatFilter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	filters := make([]models.ChatFilter, 0)
	for _, filter := range s.data.ChatFilters {
		if filter.ChannelID != channelID {
			continue
		}
		filters = append(filters, filter)
	}
	sort.Slice(filters, func(i, j int) bool {
		if filters[i].CreatedAt.Equal(filters[j].CreatedAt) {
			return filters[i].ID < filters[j].ID
		}
		return filters[i].CreatedAt.After(filters[j].CreatedAt)
	})
	return filters, nil
}

// CreateChatFilter registers a new auto-moderation filter for a channel.
func (s *Storage) CreateChatFilter(channelID string, params ChatFilterParams) (models.ChatFilter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return models.ChatFilter{}, fmt.Errorf("channel %s not found", channelID)
	}
	kind, pattern, err := normalizeChatFilter(params.Kind, params.Pattern)
	if err != nil {
		return models.ChatFilter{}, err
	}
	id, err := generateID()
	if err != nil {
		return models.ChatFilter{}, err
	}
	now := time.Now().UTC()
	filter := models.ChatFilter{
		ID:        id,
		ChannelID: channelID,
		Kind:      kind,
		Pattern:   pattern,
		Enabled:   params.Enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if s.data.ChatFilters == nil {
		s.data.ChatFilters = make(map[string]models.ChatFilter)
	}
	s.data.ChatFilters[id] = filter
	if err := s.persist(); err != nil {
		delete(s.data.ChatFilters, id)
		return models.ChatFilter{}, err
	}
	return filter, nil
}

// UpdateChatFilter updates an existing auto-moderation filter.
func (s *Storage) UpdateChatFilter(id string, update ChatFilterUpdate) (models.ChatFilter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filter, ok := s.data.ChatFilters[id]
	if !ok {
		return models.ChatFilter{}, fmt.Errorf("filter %s not found", id)
	}
	kind := filter.Kind
	pattern := filter.Pattern
	if update.Kind != nil {
		kind = *update.Kind
	}
	if update.Pattern != nil {
		pattern = *update.Pattern
	}
	if update.Enabled != nil {
		filter.Enabled = *update.Enabled
	}
	normalizedKind, normalizedPattern, err := normalizeChatFilter(kind, pattern)
	if err != nil {
		return models.ChatFilter{}, err
	}
	filter.Kind = normalizedKind
	filter.Pattern = normalizedPattern
	filter.UpdatedAt = time.Now().UTC()
	s.data.ChatFilters[id] = filter
	if err := s.persist(); err != nil {
		return models.ChatFilter{}, err
	}
	return filter, nil
}

// DeleteChatFilter removes a filter from the channel's automod configuration.
func (s *Storage) DeleteChatFilter(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filter, ok := s.data.ChatFilters[id]
	if !ok {
		return fmt.Errorf("filter %s not found", id)
	}
	delete(s.data.ChatFilters, id)
	if err := s.persist(); err != nil {
		s.data.ChatFilters[id] = filter
		return err
	}
	return nil
}

// ListChatAutoModActions returns recent auto-moderation actions for a channel.
func (s *Storage) ListChatAutoModActions(channelID string, limit int) ([]models.ChatAutoModAction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	now := s.retentionTime()
	removed, snapshot, err := s.purgeExpiredChatAutoModActionsLocked(now)
	if err != nil {
		return nil, err
	}
	if removed {
		if err := s.persist(); err != nil {
			s.data = snapshot
			return nil, err
		}
	}

	actions := make([]models.ChatAutoModAction, 0)
	for _, action := range s.data.ChatAutoModActions {
		if action.ChannelID != channelID {
			continue
		}
		actions = append(actions, action)
	}
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].CreatedAt.Equal(actions[j].CreatedAt) {
			return actions[i].ID < actions[j].ID
		}
		return actions[i].CreatedAt.After(actions[j].CreatedAt)
	})
	if limit > 0 && len(actions) > limit {
		actions = actions[:limit]
	}
	return actions, nil
}
