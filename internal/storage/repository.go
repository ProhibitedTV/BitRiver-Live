package storage

import (
	"context"
	"time"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/domain"
	"bitriver-live/internal/ingest"
)

// Repository exposes the datastore operations required by API handlers,
// chat infrastructure, and ingest orchestration.
type Repository interface {
	Ping(ctx context.Context) error
	IngestHealth(ctx context.Context) []ingest.HealthStatus
	LastIngestHealth() ([]ingest.HealthStatus, time.Time)

	CreateUser(params CreateUserParams) (domain.User, error)
	AuthenticateUser(email, password string) (domain.User, error)
	AuthenticateOAuth(params OAuthLoginParams) (domain.User, error)
	ListUsers() []domain.User
	GetUser(id string) (domain.User, bool)
	UpdateUser(id string, update UserUpdate) (domain.User, error)
	SetUserPassword(id, password string) (domain.User, error)
	DeleteUser(id string) error
	GetMFASettings(userID string) (domain.MFASettings, bool, error)
	UpsertMFASettings(settings domain.MFASettings) (domain.MFASettings, error)
	DeleteMFASettings(userID string) error

	UpsertProfile(userID string, update ProfileUpdate) (domain.Profile, error)
	GetProfile(userID string) (domain.Profile, bool)
	ListProfiles() []domain.Profile

	CreateChannel(ownerID, title, category string, tags []string) (domain.Channel, error)
	UpdateChannel(id string, update ChannelUpdate) (domain.Channel, error)
	RotateChannelStreamKey(id string) (domain.Channel, error)
	DeleteChannel(id string) error
	GetChannel(id string) (domain.Channel, bool)
	GetChannelByStreamKey(streamKey string) (domain.Channel, bool)
	ListChannels(ownerID, query string) []domain.Channel

	FollowChannel(userID, channelID string) error
	UnfollowChannel(userID, channelID string) error
	IsFollowingChannel(userID, channelID string) bool
	CountFollowers(channelID string) int
	ListFollowedChannelIDs(userID string) []string

	StartStream(channelID string, renditions []string) (domain.StreamSession, error)
	StopStream(channelID string, peakConcurrent int) (domain.StreamSession, error)
	CurrentStreamSession(channelID string) (domain.StreamSession, bool)
	ListStreamSessions(channelID string) ([]domain.StreamSession, error)

	ListRecordings(channelID string, includeUnpublished bool) ([]domain.Recording, error)
	GetRecording(id string) (domain.Recording, bool)
	PublishRecording(id string) (domain.Recording, error)
	DeleteRecording(id string) error

	CreateUpload(params CreateUploadParams) (domain.Upload, error)
	ListUploads(channelID string) ([]domain.Upload, error)
	GetUpload(id string) (domain.Upload, bool)
	UpdateUpload(id string, update UploadUpdate) (domain.Upload, error)
	DeleteUpload(id string) error

	CreateClipExport(recordingID string, params ClipExportParams) (domain.ClipExport, error)
	ListClipExports(recordingID string) ([]domain.ClipExport, error)

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
	CreateChatFilter(channelID string, params ChatFilterParams) (domain.ChatFilter, error)
	UpdateChatFilter(id string, update ChatFilterUpdate) (domain.ChatFilter, error)
	DeleteChatFilter(id string) error
	ListChatAutoModActions(channelID string, limit int) ([]domain.ChatAutoModAction, error)

	CreateTip(params CreateTipParams) (domain.Tip, error)
	ListTips(channelID string, limit int) ([]domain.Tip, error)

	CreateSubscription(params CreateSubscriptionParams) (domain.Subscription, error)
	ProcessPaymentWebhook(params ProcessPaymentWebhookParams) (domain.PaymentTransaction, error)
	ListSubscriptions(channelID string, includeInactive bool) ([]domain.Subscription, error)
	GetSubscription(id string) (domain.Subscription, bool)
	CancelSubscription(id, cancelledBy, reason string) (domain.Subscription, error)

	CreateDMCACase(params CreateDMCACaseParams) (domain.DMCACase, error)
	ListDMCACases() ([]domain.DMCACase, error)
	GetDMCACase(id string) (domain.DMCACase, bool)
	UpdateDMCACase(id string, update DMCACaseUpdate, actorUserID string) (domain.DMCACase, error)

	CreateDataSubjectRequest(params CreateDataSubjectRequestParams) (domain.DataSubjectRequest, error)
	ListDataSubjectRequests() ([]domain.DataSubjectRequest, error)
	GetDataSubjectRequest(id string) (domain.DataSubjectRequest, bool)
	UpdateDataSubjectRequest(id string, update DataSubjectRequestUpdate, actorUserID string) (domain.DataSubjectRequest, error)
	AddDataSubjectAuditEvent(requestID string, params CreateDataSubjectAuditEventParams) (domain.DataSubjectAuditEvent, error)
	ListDataSubjectAuditEvents(requestID string) ([]domain.DataSubjectAuditEvent, error)
	ListLegalStateHistory(entityType, entityID string) ([]domain.LegalStateHistory, error)
}

var _ Repository = (*Storage)(nil)
