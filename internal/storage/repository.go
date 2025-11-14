package storage

import (
	"context"
	"time"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
)

// Repository exposes the datastore operations required by API handlers,
// chat infrastructure, and ingest orchestration.
type Repository interface {
	Ping(ctx context.Context) error
	IngestHealth(ctx context.Context) []ingest.HealthStatus
	LastIngestHealth() ([]ingest.HealthStatus, time.Time)

	CreateUser(params CreateUserParams) (models.User, error)
	AuthenticateUser(email, password string) (models.User, error)
	AuthenticateOAuth(params OAuthLoginParams) (models.User, error)
	ListUsers() []models.User
	GetUser(id string) (models.User, bool)
	UpdateUser(id string, update UserUpdate) (models.User, error)
	SetUserPassword(id, password string) (models.User, error)
	DeleteUser(id string) error

	UpsertProfile(userID string, update ProfileUpdate) (models.Profile, error)
	GetProfile(userID string) (models.Profile, bool)
	ListProfiles() []models.Profile

	CreateChannel(ownerID, title, category string, tags []string) (models.Channel, error)
	UpdateChannel(id string, update ChannelUpdate) (models.Channel, error)
	RotateChannelStreamKey(id string) (models.Channel, error)
	DeleteChannel(id string) error
	GetChannel(id string) (models.Channel, bool)
	ListChannels(ownerID, query string) []models.Channel

	FollowChannel(userID, channelID string) error
	UnfollowChannel(userID, channelID string) error
	IsFollowingChannel(userID, channelID string) bool
	CountFollowers(channelID string) int
	ListFollowedChannelIDs(userID string) []string

	StartStream(channelID string, renditions []string) (models.StreamSession, error)
	StopStream(channelID string, peakConcurrent int) (models.StreamSession, error)
	CurrentStreamSession(channelID string) (models.StreamSession, bool)
	ListStreamSessions(channelID string) ([]models.StreamSession, error)

	ListRecordings(channelID string, includeUnpublished bool) ([]models.Recording, error)
	GetRecording(id string) (models.Recording, bool)
	PublishRecording(id string) (models.Recording, error)
	DeleteRecording(id string) error

	CreateUpload(params CreateUploadParams) (models.Upload, error)
	ListUploads(channelID string) ([]models.Upload, error)
	GetUpload(id string) (models.Upload, bool)
	UpdateUpload(id string, update UploadUpdate) (models.Upload, error)
	DeleteUpload(id string) error

	CreateClipExport(recordingID string, params ClipExportParams) (models.ClipExport, error)
	ListClipExports(recordingID string) ([]models.ClipExport, error)

	CreateChatMessage(channelID, userID, content string) (models.ChatMessage, error)
	DeleteChatMessage(channelID, messageID string) error
	ListChatMessages(channelID string, limit int) ([]models.ChatMessage, error)
	ChatRestrictions() chat.RestrictionsSnapshot
	IsChatBanned(channelID, userID string) bool
	ChatTimeout(channelID, userID string) (time.Time, bool)
	ApplyChatEvent(evt chat.Event) error

	ListChatRestrictions(channelID string) []models.ChatRestriction
	CreateChatReport(channelID, reporterID, targetID, reason, messageID, evidenceURL string) (models.ChatReport, error)
	ListChatReports(channelID string, includeResolved bool) ([]models.ChatReport, error)
	ResolveChatReport(reportID, resolverID, resolution string) (models.ChatReport, error)

	CreateTip(params CreateTipParams) (models.Tip, error)
	ListTips(channelID string, limit int) ([]models.Tip, error)

	CreateSubscription(params CreateSubscriptionParams) (models.Subscription, error)
	ListSubscriptions(channelID string, includeInactive bool) ([]models.Subscription, error)
	GetSubscription(id string) (models.Subscription, bool)
	CancelSubscription(id, cancelledBy, reason string) (models.Subscription, error)
}

var _ Repository = (*Storage)(nil)
