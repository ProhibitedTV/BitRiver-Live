package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/models"
)

// ApplyChatEvent mutates the in-memory dataset based on the supplied chat
// event and persists the change to disk.
func (s *Storage) ApplyChatEvent(evt chat.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureDatasetInitializedLocked()

	switch evt.Type {
	case chat.EventTypeMessage:
		if evt.Message == nil {
			return fmt.Errorf("message payload missing")
		}
		message := models.ChatMessage{
			ID:        evt.Message.ID,
			ChannelID: evt.Message.ChannelID,
			UserID:    evt.Message.UserID,
			Content:   evt.Message.Content,
			CreatedAt: evt.Message.CreatedAt.UTC(),
		}
		if message.ID == "" || message.ChannelID == "" || message.UserID == "" {
			return fmt.Errorf("invalid message event")
		}
		s.data.ChatMessages[message.ID] = message
	case chat.EventTypeModeration:
		if evt.Moderation == nil {
			return fmt.Errorf("moderation payload missing")
		}
		s.applyModerationLocked(*evt.Moderation, evt.OccurredAt)
	case chat.EventTypeReport:
		if evt.Report == nil {
			return fmt.Errorf("report payload missing")
		}
		if err := s.applyReportLocked(*evt.Report); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported chat event %q", evt.Type)
	}

	return s.persist()
}

func (s *Storage) applyModerationLocked(evt chat.ModerationEvent, occurredAt time.Time) {
	switch evt.Action {
	case chat.ModerationActionBan:
		if s.data.ChatBans == nil {
			s.data.ChatBans = make(map[string]map[string]time.Time)
		}
		if s.data.ChatBans[evt.ChannelID] == nil {
			s.data.ChatBans[evt.ChannelID] = make(map[string]time.Time)
		}
		issued := occurredAt.UTC()
		if issued.IsZero() {
			issued = time.Now().UTC()
		}
		s.data.ChatBans[evt.ChannelID][evt.TargetID] = issued
		s.ensureBanMetadata(evt.ChannelID)
		s.data.ChatBanActors[evt.ChannelID][evt.TargetID] = evt.ActorID
		s.data.ChatBanReasons[evt.ChannelID][evt.TargetID] = strings.TrimSpace(evt.Reason)
	case chat.ModerationActionUnban:
		if bans := s.data.ChatBans[evt.ChannelID]; bans != nil {
			delete(bans, evt.TargetID)
			if len(bans) == 0 {
				delete(s.data.ChatBans, evt.ChannelID)
			}
		}
		if actors := s.data.ChatBanActors[evt.ChannelID]; actors != nil {
			delete(actors, evt.TargetID)
			if len(actors) == 0 {
				delete(s.data.ChatBanActors, evt.ChannelID)
			}
		}
		if reasons := s.data.ChatBanReasons[evt.ChannelID]; reasons != nil {
			delete(reasons, evt.TargetID)
			if len(reasons) == 0 {
				delete(s.data.ChatBanReasons, evt.ChannelID)
			}
		}
	case chat.ModerationActionTimeout:
		if s.data.ChatTimeouts == nil {
			s.data.ChatTimeouts = make(map[string]map[string]time.Time)
		}
		if s.data.ChatTimeouts[evt.ChannelID] == nil {
			s.data.ChatTimeouts[evt.ChannelID] = make(map[string]time.Time)
		}
		if evt.ExpiresAt != nil {
			expiry := evt.ExpiresAt.UTC()
			s.data.ChatTimeouts[evt.ChannelID][evt.TargetID] = expiry
			s.ensureTimeoutMetadata(evt.ChannelID)
			issued := occurredAt.UTC()
			if issued.IsZero() {
				issued = time.Now().UTC()
			}
			s.data.ChatTimeoutIssuedAt[evt.ChannelID][evt.TargetID] = issued
			s.data.ChatTimeoutActors[evt.ChannelID][evt.TargetID] = evt.ActorID
			s.data.ChatTimeoutReasons[evt.ChannelID][evt.TargetID] = strings.TrimSpace(evt.Reason)
		}
	case chat.ModerationActionRemoveTimeout:
		if timeouts := s.data.ChatTimeouts[evt.ChannelID]; timeouts != nil {
			delete(timeouts, evt.TargetID)
			if len(timeouts) == 0 {
				delete(s.data.ChatTimeouts, evt.ChannelID)
			}
		}
		if issued := s.data.ChatTimeoutIssuedAt[evt.ChannelID]; issued != nil {
			delete(issued, evt.TargetID)
			if len(issued) == 0 {
				delete(s.data.ChatTimeoutIssuedAt, evt.ChannelID)
			}
		}
		if actors := s.data.ChatTimeoutActors[evt.ChannelID]; actors != nil {
			delete(actors, evt.TargetID)
			if len(actors) == 0 {
				delete(s.data.ChatTimeoutActors, evt.ChannelID)
			}
		}
		if reasons := s.data.ChatTimeoutReasons[evt.ChannelID]; reasons != nil {
			delete(reasons, evt.TargetID)
			if len(reasons) == 0 {
				delete(s.data.ChatTimeoutReasons, evt.ChannelID)
			}
		}
	}
}

func (s *Storage) applyReportLocked(evt chat.ReportEvent) error {
	if strings.TrimSpace(evt.ID) == "" {
		return fmt.Errorf("report id missing")
	}
	if s.data.ChatReports == nil {
		s.data.ChatReports = make(map[string]models.ChatReport)
	}
	report := models.ChatReport{
		ID:          evt.ID,
		ChannelID:   evt.ChannelID,
		ReporterID:  evt.ReporterID,
		TargetID:    evt.TargetID,
		Reason:      evt.Reason,
		MessageID:   evt.MessageID,
		EvidenceURL: evt.EvidenceURL,
		Status:      strings.ToLower(evt.Status),
		CreatedAt:   evt.CreatedAt.UTC(),
	}
	if report.Status == "" {
		report.Status = ChatReportStatusOpen
	}
	s.data.ChatReports[report.ID] = report
	return nil
}

// ChatRestrictions returns the current moderation snapshot for all channels.
func (s *Storage) ChatRestrictions() chat.RestrictionsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := chat.RestrictionsSnapshot{
		Bans:            make(map[string]map[string]struct{}, len(s.data.ChatBans)),
		Timeouts:        make(map[string]map[string]time.Time, len(s.data.ChatTimeouts)),
		BanActors:       make(map[string]map[string]string, len(s.data.ChatBanActors)),
		BanReasons:      make(map[string]map[string]string, len(s.data.ChatBanReasons)),
		TimeoutActors:   make(map[string]map[string]string, len(s.data.ChatTimeoutActors)),
		TimeoutReasons:  make(map[string]map[string]string, len(s.data.ChatTimeoutReasons)),
		TimeoutIssuedAt: make(map[string]map[string]time.Time, len(s.data.ChatTimeoutIssuedAt)),
	}
	for channelID, bans := range s.data.ChatBans {
		if len(bans) == 0 {
			continue
		}
		snapshot.Bans[channelID] = make(map[string]struct{}, len(bans))
		for userID := range bans {
			snapshot.Bans[channelID][userID] = struct{}{}
		}
	}
	now := time.Now().UTC()
	for channelID, timeouts := range s.data.ChatTimeouts {
		if len(timeouts) == 0 {
			continue
		}
		pruned := make(map[string]time.Time)
		for userID, expiry := range timeouts {
			if expiry.After(now) {
				pruned[userID] = expiry
			}
		}
		if len(pruned) > 0 {
			snapshot.Timeouts[channelID] = pruned
		}
	}
	for channelID, actors := range s.data.ChatBanActors {
		if len(actors) == 0 {
			continue
		}
		snapshot.BanActors[channelID] = make(map[string]string, len(actors))
		for userID, actor := range actors {
			snapshot.BanActors[channelID][userID] = actor
		}
	}
	for channelID, reasons := range s.data.ChatBanReasons {
		if len(reasons) == 0 {
			continue
		}
		snapshot.BanReasons[channelID] = make(map[string]string, len(reasons))
		for userID, reason := range reasons {
			snapshot.BanReasons[channelID][userID] = reason
		}
	}
	for channelID, actors := range s.data.ChatTimeoutActors {
		if len(actors) == 0 {
			continue
		}
		snapshot.TimeoutActors[channelID] = make(map[string]string, len(actors))
		for userID, actor := range actors {
			snapshot.TimeoutActors[channelID][userID] = actor
		}
	}
	for channelID, reasons := range s.data.ChatTimeoutReasons {
		if len(reasons) == 0 {
			continue
		}
		snapshot.TimeoutReasons[channelID] = make(map[string]string, len(reasons))
		for userID, reason := range reasons {
			snapshot.TimeoutReasons[channelID][userID] = reason
		}
	}
	for channelID, issued := range s.data.ChatTimeoutIssuedAt {
		if len(issued) == 0 {
			continue
		}
		snapshot.TimeoutIssuedAt[channelID] = make(map[string]time.Time, len(issued))
		for userID, ts := range issued {
			snapshot.TimeoutIssuedAt[channelID][userID] = ts
		}
	}
	return snapshot
}

// IsChatBanned reports whether the user is currently banned from the given channel.
func (s *Storage) IsChatBanned(channelID, userID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isChatBannedLocked(channelID, userID)
}

// ChatTimeout returns the timeout expiry if the user is muted in the channel.
func (s *Storage) ChatTimeout(channelID, userID string) (time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.chatTimeoutLocked(channelID, userID)
}

// ChatWorker consumes queue events and applies them to storage.
type ChatWorker struct {
	queue  chat.Queue
	store  Repository
	logger *slog.Logger
}

// NewChatWorker prepares a worker that will persist chat events delivered via the queue.
func NewChatWorker(store Repository, queue chat.Queue, logger *slog.Logger) *ChatWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &ChatWorker{queue: queue, store: store, logger: logger}
}

// Run blocks until the context is cancelled, persisting chat events as they arrive.
func (w *ChatWorker) Run(ctx context.Context) {
	if w.queue == nil || w.store == nil {
		return
	}
	sub := w.queue.Subscribe()
	defer sub.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-sub.Events():
			if !ok {
				return
			}
			if err := w.store.ApplyChatEvent(evt); err != nil && w.logger != nil {
				w.logger.Error("failed to apply chat event", "error", err)
			}
		}
	}
}
