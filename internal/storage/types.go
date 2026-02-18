package storage

import (
	"context"
	"errors"
	"sync"
	"time"

	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
)

const (
	passwordHashSaltLength = 16
	passwordHashKeyLength  = 32
	passwordHashIterations = 120000

	metadataManifestPrefix  = "object:manifest:"
	metadataThumbnailPrefix = "object:thumbnail:"

	// MaxTipReferenceLength defines the maximum number of characters allowed for
	// a tip reference identifier.
	MaxTipReferenceLength = 256
	// MaxTipWalletAddressLength defines the maximum number of characters allowed
	// for a tip wallet address.
	MaxTipWalletAddressLength = 256
	// MaxTipMessageLength defines the maximum number of characters allowed for a
	// tip message payload.
	MaxTipMessageLength = 512

	// MaxChatMessageLength defines the maximum number of characters allowed for a
	// chat message.
	MaxChatMessageLength = 500

	ChatReportStatusOpen     = "open"
	ChatReportStatusResolved = "resolved"
	AppealStatusOpen         = "open"
	AppealStatusResolved     = "resolved"

	duplicateTipReferenceError = "pq: duplicate key value violates unique constraint \"tips_reference_unique\""
)

var (
	// ErrIngestControllerUnavailable indicates that stream lifecycle
	// operations cannot be performed because no ingest controller has been
	// configured.
	ErrIngestControllerUnavailable = errors.New("ingest controller unavailable")

	ErrInvalidCredentials       = errors.New("invalid credentials")
	ErrPasswordLoginUnsupported = errors.New("account does not support password login")
)

type dataset struct {
	Users               map[string]models.User                    `json:"users"`
	MFASettings         map[string]models.MFASettings             `json:"mfaSettings"`
	OAuthAccounts       map[string]models.OAuthAccount            `json:"oauthAccounts"`
	Channels            map[string]models.Channel                 `json:"channels"`
	StreamSessions      map[string]models.StreamSession           `json:"streamSessions"`
	ChatMessages        map[string]models.ChatMessage             `json:"chatMessages"`
	ChatBans            map[string]map[string]time.Time           `json:"chatBans"`
	ChatTimeouts        map[string]map[string]time.Time           `json:"chatTimeouts"`
	ChatBanActors       map[string]map[string]string              `json:"chatBanActors"`
	ChatBanReasons      map[string]map[string]string              `json:"chatBanReasons"`
	ChatTimeoutActors   map[string]map[string]string              `json:"chatTimeoutActors"`
	ChatTimeoutReasons  map[string]map[string]string              `json:"chatTimeoutReasons"`
	ChatTimeoutIssuedAt map[string]map[string]time.Time           `json:"chatTimeoutIssuedAt"`
	ChatReports         map[string]models.ChatReport              `json:"chatReports"`
	Appeals             map[string]models.Appeal                  `json:"appeals"`
	AppealEvents        map[string][]models.AppealEvent           `json:"appealEvents"`
	ChatFilters         map[string]models.ChatFilter              `json:"chatFilters"`
	ChatAutoModActions  map[string]models.ChatAutoModAction       `json:"chatAutoModActions"`
	Tips                map[string]models.Tip                     `json:"tips"`
	Subscriptions       map[string]models.Subscription            `json:"subscriptions"`
	PaymentTransactions map[string]models.PaymentTransaction      `json:"paymentTransactions"`
	Profiles            map[string]models.Profile                 `json:"profiles"`
	Follows             map[string]map[string]time.Time           `json:"follows"`
	Recordings          map[string]models.Recording               `json:"recordings"`
	Uploads             map[string]models.Upload                  `json:"uploads"`
	ClipExports         map[string]models.ClipExport              `json:"clipExports"`
	DMCACases           map[string]models.DMCACase                `json:"dmcaCases"`
	DataSubjectRequests map[string]models.DataSubjectRequest      `json:"dataSubjectRequests"`
	DataSubjectAudit    map[string][]models.DataSubjectAuditEvent `json:"dataSubjectAudit"`
	LegalStateHistory   []models.LegalStateHistory                `json:"legalStateHistory"`
}

type Storage struct {
	mu       sync.RWMutex
	filePath string
	data     dataset
	// persistOverride allows tests to intercept persist operations.
	persistOverride     func(dataset) error
	ingestController    ingest.Controller
	ingestMaxAttempts   int
	ingestRetryInterval time.Duration
	ingestTimeout       time.Duration
	ingestHealth        []ingest.HealthStatus
	ingestHealthUpdated time.Time
	recordingRetention  RecordingRetentionPolicy
	chatRetention       ChatRetentionPolicy
	objectStorage       ObjectStorageConfig
	objectClient        objectStorageClient
	retentionNow        func() time.Time
}

// RecordingRetentionPolicy specifies how long recordings are kept before being
// purged when unpublished or published.
type RecordingRetentionPolicy struct {
	Published   time.Duration
	Unpublished time.Duration
}

// ChatRetentionPolicy specifies how long chat messages and moderation logs are
// retained before they are purged.
type ChatRetentionPolicy struct {
	Messages       time.Duration
	ModerationLogs time.Duration
}

// ChatFilterParams describes a moderation filter to enforce on chat messages.
type ChatFilterParams struct {
	Kind    string
	Pattern string
	Enabled bool
}

// ChatFilterUpdate allows partial updates for existing chat filters.
type ChatFilterUpdate struct {
	Kind    *string
	Pattern *string
	Enabled *bool
}

// ObjectStorageConfig describes the external storage bucket used for
// persisting VOD artefacts.
type ObjectStorageConfig struct {
	Endpoint       string
	Region         string
	AccessKey      string
	SecretKey      string
	Bucket         string
	UseSSL         bool
	Prefix         string
	LifecycleDays  int
	PublicEndpoint string
	RequestTimeout time.Duration
}

type objectStorageClient interface {
	Enabled() bool
	Upload(ctx context.Context, key, contentType string, body []byte) (objectReference, error)
	Delete(ctx context.Context, key string) error
}

type objectReference struct {
	Key string
	URL string
}

const defaultObjectStorageRequestTimeout = 30 * time.Second

// ClipExportParams captures the request to generate a recording clip.
type ClipExportParams struct {
	Title        string
	StartSeconds int
	EndSeconds   int
}

// CreateUploadParams captures the information required to store an uploaded asset.
type CreateUploadParams struct {
	ChannelID   string
	Title       string
	Filename    string
	SizeBytes   int64
	Metadata    map[string]string
	PlaybackURL string
}

// UploadUpdate describes the mutable fields of an upload entry.
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

// CreateUserParams captures the attributes that can be set when creating a user.
type CreateUserParams struct {
	DisplayName string
	Email       string
	Password    string
	Roles       []string
	SelfSignup  bool
}

// OAuthLoginParams represents the identity information returned by an OAuth
// provider used to authenticate or provision a user account.
type OAuthLoginParams struct {
	Provider    string
	Subject     string
	Email       string
	DisplayName string
}

// CreateTipParams captures the information required to record a tip.
type CreateTipParams struct {
	ChannelID      string
	FromUserID     string
	Amount         models.Money
	Currency       string
	Provider       string
	Reference      string
	WalletAddress  string
	Message        string
	IdempotencyKey string
}

// CreateSubscriptionParams captures the data needed to start a subscription.
type CreateSubscriptionParams struct {
	ChannelID         string
	UserID            string
	Tier              string
	Provider          string
	Reference         string
	Amount            models.Money
	Currency          string
	Duration          time.Duration
	AutoRenew         bool
	ExternalReference string
	IdempotencyKey    string
}

// ProcessPaymentWebhookParams captures a verified webhook event from a provider.
type ProcessPaymentWebhookParams struct {
	Provider       string
	EventID        string
	EntityType     string
	Reference      string
	Status         string
	IdempotencyKey string
}

type CreateDMCACaseParams struct {
	ReporterName  string
	ReporterEmail string
	ContentURL    string
	Description   string
}

type DMCACaseUpdate struct {
	Status *string
	Notes  *string
}

type CreateDataSubjectRequestParams struct {
	SubjectEmail string
	RequestType  string
	Notes        string
}

type DataSubjectRequestUpdate struct {
	Status *string
	Notes  *string
}

type CreateDataSubjectAuditEventParams struct {
	ActorUserID string
	Action      string
	Details     string
	EvidenceRef string
}
