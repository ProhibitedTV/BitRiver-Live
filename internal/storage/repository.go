package storage

import (
	"context"
	"time"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/domain"
	"bitriver-live/internal/ingest"
)

// Repository is a compatibility aggregate kept for adapter wiring while
// callers migrate to narrower domain-owned repository interfaces.
type Repository interface {
	Ping(ctx context.Context) error
	IngestHealth(ctx context.Context) []ingest.HealthStatus
	LastIngestHealth() ([]ingest.HealthStatus, time.Time)

	domain.AuthRepository
	domain.ChannelsRepository
	domain.UploadsRepository
	domain.RecordingsRepository
	domain.PaymentsRepository
	domain.LegalRepository

	SetUserPassword(id, password string) (domain.User, error)
	GetMFASettings(userID string) (domain.MFASettings, bool, error)
	DeleteMFASettings(userID string) error

	UpsertProfile(userID string, update ProfileUpdate) (domain.Profile, error)
	RotateChannelStreamKey(id string) (domain.Channel, error)
	GetChannelByStreamKey(streamKey string) (domain.Channel, bool)

	StartStream(channelID string, renditions []string) (domain.StreamSession, error)
	StopStream(channelID string, peakConcurrent int) (domain.StreamSession, error)

	CreateChatMessage(channelID, userID, content string) (domain.ChatMessage, error)
	DeleteChatMessage(channelID, messageID string) error
	ListChatMessages(channelID string, limit int) ([]domain.ChatMessage, error)
	ChatRestrictions() chat.RestrictionsSnapshot
	IsChatBanned(channelID, userID string) bool
	ChatTimeout(channelID, userID string) (time.Time, bool)
	ApplyChatEvent(evt chat.Event) error

	ListChatRestrictions(channelID string) []domain.ChatRestriction
	CreateChatReport(channelID, reporterID, targetID, reason, messageID, evidenceURL string) (domain.ChatReport, error)
	ListChatReports(channelID string, includeResolved bool) ([]domain.ChatReport, error)
	ResolveChatReport(reportID, resolverID, resolution string) (domain.ChatReport, error)
	CreateAppeal(reportID, reporterID, reason string) (domain.Appeal, error)
	ListAppeals(channelID, requesterID string, includeClosed bool) ([]domain.Appeal, error)
	ResolveAppeal(appealID, resolverID, resolution string) (domain.Appeal, error)
	ReopenAppeal(appealID, actorID, note string) (domain.Appeal, error)
	ListChatFilters(channelID string) ([]domain.ChatFilter, error)
	CreateChatFilter(channelID string, params domain.ChatFilterCreateParams) (domain.ChatFilter, error)
	UpdateChatFilter(id string, update domain.ChatFilterUpdate) (domain.ChatFilter, error)
	DeleteChatFilter(id string) error
	ListChatAutoModActions(channelID string, limit int) ([]domain.ChatAutoModAction, error)

	ListTips(channelID string, limit int) ([]domain.Tip, error)
	ListSubscriptions(channelID string, includeInactive bool) ([]domain.Subscription, error)
	GetSubscription(id string) (domain.Subscription, bool)
	CancelSubscription(id, cancelledBy, reason string) (domain.Subscription, error)

	ListDMCACases() ([]domain.DMCACase, error)

	ListDataSubjectRequests() ([]domain.DataSubjectRequest, error)
	AddDataSubjectAuditEvent(requestID string, params CreateDataSubjectAuditEventParams) (domain.DataSubjectAuditEvent, error)
	ListDataSubjectAuditEvents(requestID string) ([]domain.DataSubjectAuditEvent, error)
	ListLegalStateHistory(entityType, entityID string) ([]domain.LegalStateHistory, error)
}

var _ Repository = (*Storage)(nil)
