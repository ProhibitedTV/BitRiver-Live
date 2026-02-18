package domain

// AuthRepository defines user and authentication persistence required by auth flows.
type AuthRepository interface {
	CreateUser(params UserCreateParams) (User, error)
	AuthenticateUser(email, password string) (User, error)
	AuthenticateOAuth(params OAuthLoginParams) (User, error)
	ListUsers() []User
	GetUser(id string) (User, bool)
	UpdateUser(id string, update UserUpdate) (User, error)
	DeleteUser(id string) error
	UpsertMFASettings(settings MFASettings) (MFASettings, error)
}

// ChannelsRepository defines channel directory, follow, and subscription operations.
type ChannelsRepository interface {
	ListChannels(ownerID, query string) []Channel
	ListProfiles() []Profile
	GetChannel(id string) (Channel, bool)
	GetUser(id string) (User, bool)
	GetProfile(userID string) (Profile, bool)
	CountFollowers(channelID string) int
	ListFollowedChannelIDs(userID string) []string
	ListSubscriptions(channelID string, includeInactive bool) ([]Subscription, error)
	CreateChannel(ownerID, title, category string, tags []string) (Channel, error)
	UpdateChannel(id string, update ChannelUpdate) (Channel, error)
	DeleteChannel(id string) error
	CurrentStreamSession(channelID string) (StreamSession, bool)
	ListStreamSessions(channelID string) ([]StreamSession, error)
	FollowChannel(userID, channelID string) error
	UnfollowChannel(userID, channelID string) error
	IsFollowingChannel(userID, channelID string) bool
	CreateSubscription(params SubscriptionCreateParams) (Subscription, error)
	CancelSubscription(id, cancelledBy, reason string) (Subscription, error)
	ListUploads(channelID string) ([]Upload, error)
	GetRecording(id string) (Recording, bool)
}

// UploadsRepository defines upload lifecycle operations.
type UploadsRepository interface {
	GetChannel(id string) (Channel, bool)
	ListUploads(channelID string) ([]Upload, error)
	GetUpload(id string) (Upload, bool)
	DeleteUpload(id string) error
	CreateUpload(params UploadCreateParams) (Upload, error)
	UpdateUpload(id string, update UploadUpdate) (Upload, error)
}

// RecordingsRepository defines recording and clip operations.
type RecordingsRepository interface {
	GetChannel(id string) (Channel, bool)
	ListRecordings(channelID string, includeUnpublished bool) ([]Recording, error)
	GetRecording(id string) (Recording, bool)
	PublishRecording(id string) (Recording, error)
	ListClipExports(recordingID string) ([]ClipExport, error)
	CreateClipExport(recordingID string, params ClipExportParams) (ClipExport, error)
	DeleteRecording(id string) error
}

// PaymentsRepository defines monetization operations.
type PaymentsRepository interface {
	CreateTip(params TipCreateParams) (Tip, error)
	CreateSubscription(params SubscriptionCreateParams) (Subscription, error)
	ProcessPaymentWebhook(params ProcessPaymentWebhookParams) (PaymentTransaction, error)
}

// LegalRepository defines legal workflow operations.
type LegalRepository interface {
	CreateDMCACase(params DMCACaseCreateParams) (DMCACase, error)
	GetDMCACase(id string) (DMCACase, bool)
	UpdateDMCACase(id string, update DMCACaseUpdate, actorUserID string) (DMCACase, error)
	CreateDataSubjectRequest(params DataSubjectRequestCreateParams) (DataSubjectRequest, error)
	GetDataSubjectRequest(id string) (DataSubjectRequest, bool)
	UpdateDataSubjectRequest(id string, update DataSubjectRequestUpdate, actorUserID string) (DataSubjectRequest, error)
}
