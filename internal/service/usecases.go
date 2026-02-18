package service

import (
	"bitriver-live/internal/domain"
)

// AuthUsersUseCase encapsulates user and authentication persistence operations
// needed by the API auth/user handlers.
type AuthUsersUseCase interface {
	CreateUser(params domain.UserCreateParams) (domain.User, error)
	AuthenticateUser(email, password string) (domain.User, error)
	AuthenticateOAuth(params domain.OAuthLoginParams) (domain.User, error)
	GetUser(id string) (domain.User, bool)
	ListUsers() []domain.User
	UpdateUser(id string, update domain.UserUpdate) (domain.User, error)
	DeleteUser(id string) error
	UpsertMFASettings(settings domain.MFASettings) (domain.MFASettings, error)
}

// ChannelsDirectoryUseCase encapsulates channel directory, follow,
// subscription, and playback-oriented persistence operations used by channel
// handlers.
type ChannelsDirectoryUseCase interface {
	ListChannels(ownerID, query string) []domain.Channel
	ListProfiles() []domain.Profile
	GetChannel(id string) (domain.Channel, bool)
	GetUser(id string) (domain.User, bool)
	GetProfile(userID string) (domain.Profile, bool)
	CountFollowers(channelID string) int
	ListFollowedChannelIDs(userID string) []string
	ListSubscriptions(channelID string, includeInactive bool) ([]domain.Subscription, error)
	CreateChannel(ownerID, title, category string, tags []string) (domain.Channel, error)
	UpdateChannel(id string, update domain.ChannelUpdate) (domain.Channel, error)
	DeleteChannel(id string) error
	CurrentStreamSession(channelID string) (domain.StreamSession, bool)
	ListStreamSessions(channelID string) ([]domain.StreamSession, error)
	FollowChannel(userID, channelID string) error
	UnfollowChannel(userID, channelID string) error
	IsFollowingChannel(userID, channelID string) bool
	CreateSubscription(params domain.SubscriptionCreateParams) (domain.Subscription, error)
	CancelSubscription(id, cancelledBy, reason string) (domain.Subscription, error)
	ListUploads(channelID string) ([]domain.Upload, error)
	GetRecording(id string) (domain.Recording, bool)
}

// UploadsUseCase encapsulates upload lifecycle operations.
type UploadsUseCase interface {
	GetChannel(id string) (domain.Channel, bool)
	ListUploads(channelID string) ([]domain.Upload, error)
	GetUpload(id string) (domain.Upload, bool)
	DeleteUpload(id string) error
	CreateUpload(params domain.UploadCreateParams) (domain.Upload, error)
	UpdateUpload(id string, update domain.UploadUpdate) (domain.Upload, error)
}

// RecordingsVODUseCase encapsulates recording and clip operations.
type RecordingsVODUseCase interface {
	GetChannel(id string) (domain.Channel, bool)
	ListRecordings(channelID string, includeUnpublished bool) ([]domain.Recording, error)
	GetRecording(id string) (domain.Recording, bool)
	PublishRecording(id string) (domain.Recording, error)
	ListClipExports(recordingID string) ([]domain.ClipExport, error)
	CreateClipExport(recordingID string, params domain.ClipExportParams) (domain.ClipExport, error)
	DeleteRecording(id string) error
}

type storeUseCases struct {
	store interface {
		domain.AuthRepository
		domain.ChannelsRepository
		domain.UploadsRepository
		domain.RecordingsRepository
	}
}

func NewStoreUseCases(store interface {
	domain.AuthRepository
	domain.ChannelsRepository
	domain.UploadsRepository
	domain.RecordingsRepository
}) *storeUseCases {
	return &storeUseCases{store: store}
}

func (s *storeUseCases) CreateUser(params domain.UserCreateParams) (domain.User, error) {
	return s.store.CreateUser(params)
}
func (s *storeUseCases) AuthenticateUser(email, password string) (domain.User, error) {
	return s.store.AuthenticateUser(email, password)
}
func (s *storeUseCases) AuthenticateOAuth(params domain.OAuthLoginParams) (domain.User, error) {
	return s.store.AuthenticateOAuth(params)
}
func (s *storeUseCases) GetUser(id string) (domain.User, bool) { return s.store.GetUser(id) }
func (s *storeUseCases) ListUsers() []domain.User              { return s.store.ListUsers() }
func (s *storeUseCases) UpdateUser(id string, update domain.UserUpdate) (domain.User, error) {
	return s.store.UpdateUser(id, update)
}
func (s *storeUseCases) DeleteUser(id string) error { return s.store.DeleteUser(id) }
func (s *storeUseCases) UpsertMFASettings(settings domain.MFASettings) (domain.MFASettings, error) {
	return s.store.UpsertMFASettings(settings)
}

func (s *storeUseCases) ListChannels(ownerID, query string) []domain.Channel {
	return s.store.ListChannels(ownerID, query)
}
func (s *storeUseCases) ListProfiles() []domain.Profile { return s.store.ListProfiles() }
func (s *storeUseCases) GetChannel(id string) (domain.Channel, bool) {
	return s.store.GetChannel(id)
}
func (s *storeUseCases) GetProfile(userID string) (domain.Profile, bool) {
	return s.store.GetProfile(userID)
}
func (s *storeUseCases) CountFollowers(channelID string) int {
	return s.store.CountFollowers(channelID)
}
func (s *storeUseCases) ListFollowedChannelIDs(userID string) []string {
	return s.store.ListFollowedChannelIDs(userID)
}
func (s *storeUseCases) ListSubscriptions(channelID string, includeInactive bool) ([]domain.Subscription, error) {
	return s.store.ListSubscriptions(channelID, includeInactive)
}
func (s *storeUseCases) CreateChannel(ownerID, title, category string, tags []string) (domain.Channel, error) {
	return s.store.CreateChannel(ownerID, title, category, tags)
}
func (s *storeUseCases) UpdateChannel(id string, update domain.ChannelUpdate) (domain.Channel, error) {
	return s.store.UpdateChannel(id, update)
}
func (s *storeUseCases) DeleteChannel(id string) error { return s.store.DeleteChannel(id) }
func (s *storeUseCases) CurrentStreamSession(channelID string) (domain.StreamSession, bool) {
	return s.store.CurrentStreamSession(channelID)
}
func (s *storeUseCases) ListStreamSessions(channelID string) ([]domain.StreamSession, error) {
	return s.store.ListStreamSessions(channelID)
}
func (s *storeUseCases) FollowChannel(userID, channelID string) error {
	return s.store.FollowChannel(userID, channelID)
}
func (s *storeUseCases) UnfollowChannel(userID, channelID string) error {
	return s.store.UnfollowChannel(userID, channelID)
}
func (s *storeUseCases) IsFollowingChannel(userID, channelID string) bool {
	return s.store.IsFollowingChannel(userID, channelID)
}
func (s *storeUseCases) CreateSubscription(params domain.SubscriptionCreateParams) (domain.Subscription, error) {
	return s.store.CreateSubscription(params)
}
func (s *storeUseCases) CancelSubscription(id, cancelledBy, reason string) (domain.Subscription, error) {
	return s.store.CancelSubscription(id, cancelledBy, reason)
}
func (s *storeUseCases) ListUploads(channelID string) ([]domain.Upload, error) {
	return s.store.ListUploads(channelID)
}
func (s *storeUseCases) GetRecording(id string) (domain.Recording, bool) {
	return s.store.GetRecording(id)
}
func (s *storeUseCases) GetUpload(id string) (domain.Upload, bool) { return s.store.GetUpload(id) }
func (s *storeUseCases) DeleteUpload(id string) error              { return s.store.DeleteUpload(id) }
func (s *storeUseCases) CreateUpload(params domain.UploadCreateParams) (domain.Upload, error) {
	return s.store.CreateUpload(params)
}
func (s *storeUseCases) UpdateUpload(id string, update domain.UploadUpdate) (domain.Upload, error) {
	return s.store.UpdateUpload(id, update)
}
func (s *storeUseCases) ListRecordings(channelID string, includeUnpublished bool) ([]domain.Recording, error) {
	return s.store.ListRecordings(channelID, includeUnpublished)
}
func (s *storeUseCases) PublishRecording(id string) (domain.Recording, error) {
	return s.store.PublishRecording(id)
}
func (s *storeUseCases) ListClipExports(recordingID string) ([]domain.ClipExport, error) {
	return s.store.ListClipExports(recordingID)
}
func (s *storeUseCases) CreateClipExport(recordingID string, params domain.ClipExportParams) (domain.ClipExport, error) {
	return s.store.CreateClipExport(recordingID, params)
}
func (s *storeUseCases) DeleteRecording(id string) error { return s.store.DeleteRecording(id) }

var (
	_ AuthUsersUseCase         = (*storeUseCases)(nil)
	_ ChannelsDirectoryUseCase = (*storeUseCases)(nil)
	_ UploadsUseCase           = (*storeUseCases)(nil)
	_ RecordingsVODUseCase     = (*storeUseCases)(nil)
)
