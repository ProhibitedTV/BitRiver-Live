package storage

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"bitriver-live/internal/models"
)

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
}

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
}

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
}

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

func (s *Storage) isChatBannedLocked(channelID, userID string) bool {
	if bans := s.data.ChatBans[channelID]; bans != nil {
		if _, exists := bans[userID]; exists {
			return true
		}
	}
	return false
}

func (s *Storage) chatTimeoutLocked(channelID, userID string) (time.Time, bool) {
	if timeouts := s.data.ChatTimeouts[channelID]; timeouts != nil {
		expiry, ok := timeouts[userID]
		if ok {
			return expiry, true
		}
	}
	return time.Time{}, false
}

func (s *Storage) ListChatMessages(channelID string, limit int) ([]models.ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
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

func (s *Storage) lookupBanActor(channelID, userID string) string {
	if actors := s.data.ChatBanActors[channelID]; actors != nil {
		return actors[userID]
	}
	return ""
}

func (s *Storage) lookupBanReason(channelID, userID string) string {
	if reasons := s.data.ChatBanReasons[channelID]; reasons != nil {
		return reasons[userID]
	}
	return ""
}

func (s *Storage) lookupTimeoutActor(channelID, userID string) string {
	if actors := s.data.ChatTimeoutActors[channelID]; actors != nil {
		return actors[userID]
	}
	return ""
}

func (s *Storage) lookupTimeoutReason(channelID, userID string) string {
	if reasons := s.data.ChatTimeoutReasons[channelID]; reasons != nil {
		return reasons[userID]
	}
	return ""
}

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
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
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
