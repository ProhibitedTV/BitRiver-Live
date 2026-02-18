package domain

import (
	"time"

	"bitriver-live/internal/models"
)

const (
	DMCACaseStatusOpen     = models.DMCACaseStatusOpen
	DMCACaseStatusActioned = models.DMCACaseStatusActioned
	DMCACaseStatusRestored = models.DMCACaseStatusRestored
	DMCACaseStatusRejected = models.DMCACaseStatusRejected

	DataSubjectRequestTypeExport = models.DataSubjectRequestTypeExport
	DataSubjectRequestTypeDelete = models.DataSubjectRequestTypeDelete

	DataSubjectRequestStatusOpen     = models.DataSubjectRequestStatusOpen
	DataSubjectRequestStatusActioned = models.DataSubjectRequestStatusActioned
	DataSubjectRequestStatusRejected = models.DataSubjectRequestStatusRejected

	PaymentStatePending   = models.PaymentStatePending
	PaymentStateConfirmed = models.PaymentStateConfirmed
	PaymentStateFailed    = models.PaymentStateFailed
	PaymentStateRefunded  = models.PaymentStateRefunded
)

type (
	Money                 = models.Money
	User                  = models.User
	MFASettings           = models.MFASettings
	OAuthAccount          = models.OAuthAccount
	Channel               = models.Channel
	StreamSession         = models.StreamSession
	RenditionManifest     = models.RenditionManifest
	Recording             = models.Recording
	RecordingRendition    = models.RecordingRendition
	RecordingThumbnail    = models.RecordingThumbnail
	Upload                = models.Upload
	DMCACase              = models.DMCACase
	LegalStateHistory     = models.LegalStateHistory
	DataSubjectRequest    = models.DataSubjectRequest
	DataSubjectAuditEvent = models.DataSubjectAuditEvent
	ClipExport            = models.ClipExport
	ClipExportSummary     = models.ClipExportSummary
	ChatMessage           = models.ChatMessage
	ChatReport            = models.ChatReport
	Appeal                = models.Appeal
	AppealEvent           = models.AppealEvent
	ChatRestriction       = models.ChatRestriction
	ChatFilter            = models.ChatFilter
	ChatAutoModAction     = models.ChatAutoModAction
	Tip                   = models.Tip
	Subscription          = models.Subscription
	PaymentTransaction    = models.PaymentTransaction
	CryptoAddress         = models.CryptoAddress
	SocialLink            = models.SocialLink
	Profile               = models.Profile
)

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

func NewMoneyFromMinorUnits(units int64) Money { return models.NewMoneyFromMinorUnits(units) }
func ParseMoney(value string) (Money, error)   { return models.ParseMoney(value) }
func MustParseMoney(value string) Money        { return models.MustParseMoney(value) }
