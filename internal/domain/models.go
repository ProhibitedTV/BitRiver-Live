package domain

import "time"

type UserCreateParams struct {
	DisplayName string
	Email       string
	Password    string
	Roles       []string
	SelfSignup  bool
}

type OAuthLoginParams struct {
	Provider    string
	Subject     string
	Email       string
	DisplayName string
}

type UserUpdate struct {
	DisplayName *string
	Email       *string
	Roles       *[]string
}

type ProfileUpdate struct {
	Bio               *string
	AvatarURL         *string
	BannerURL         *string
	SocialLinks       *[]SocialLink
	FeaturedChannelID *string
	TopFriends        *[]string
	DonationAddresses *[]CryptoAddress
}

type ChannelUpdate struct {
	Title     *string
	Category  *string
	Tags      *[]string
	LiveState *string
}

type UploadCreateParams struct {
	ChannelID   string
	Title       string
	Filename    string
	SizeBytes   int64
	Metadata    map[string]string
	PlaybackURL string
}

type UploadUpdate struct {
	Title       *string
	Status      *string
	Progress    *int
	RecordingID *string
	PlaybackURL *string
	Metadata    map[string]string
	Error       *string
	CompletedAt *time.Time
}

type ClipExportParams struct {
	Title        string
	StartSeconds int
	EndSeconds   int
}

type ChatFilterCreateParams struct {
	Kind    string
	Pattern string
	Enabled bool
}

type ChatFilterUpdate struct {
	Kind    *string
	Pattern *string
	Enabled *bool
}

type TipCreateParams struct {
	ChannelID      string
	FromUserID     string
	Amount         Money
	Currency       string
	Provider       string
	Reference      string
	WalletAddress  string
	Message        string
	IdempotencyKey string
}

type SubscriptionCreateParams struct {
	ChannelID         string
	UserID            string
	Tier              string
	Provider          string
	Reference         string
	Amount            Money
	Currency          string
	Duration          time.Duration
	AutoRenew         bool
	ExternalReference string
	IdempotencyKey    string
}

type ProcessPaymentWebhookParams struct {
	Provider       string
	EventID        string
	EntityType     string
	Reference      string
	Status         string
	IdempotencyKey string
}

type DMCACaseCreateParams struct {
	ReporterName  string
	ReporterEmail string
	ContentURL    string
	Description   string
}

type DMCACaseUpdate struct {
	Status *string
	Notes  *string
}

type DataSubjectRequestCreateParams struct {
	SubjectEmail string
	RequestType  string
	Notes        string
}

type DataSubjectRequestUpdate struct {
	Status *string
	Notes  *string
}

type DataSubjectAuditEventCreateParams struct {
	ActorUserID string
	Action      string
	Details     string
	EvidenceRef string
}
