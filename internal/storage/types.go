package storage

import (
	"context"
	"errors"
	"sync"
	"time"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/ingest"
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
	Users               map[string]domain.User                    `json:"users"`
	MFASettings         map[string]domain.MFASettings             `json:"mfaSettings"`
	OAuthAccounts       map[string]domain.OAuthAccount            `json:"oauthAccounts"`
	Channels            map[string]domain.Channel                 `json:"channels"`
	StreamSessions      map[string]domain.StreamSession           `json:"streamSessions"`
	ChatMessages        map[string]domain.ChatMessage             `json:"chatMessages"`
	ChatBans            map[string]map[string]time.Time           `json:"chatBans"`
	ChatTimeouts        map[string]map[string]time.Time           `json:"chatTimeouts"`
	ChatBanActors       map[string]map[string]string              `json:"chatBanActors"`
	ChatBanReasons      map[string]map[string]string              `json:"chatBanReasons"`
	ChatTimeoutActors   map[string]map[string]string              `json:"chatTimeoutActors"`
	ChatTimeoutReasons  map[string]map[string]string              `json:"chatTimeoutReasons"`
	ChatTimeoutIssuedAt map[string]map[string]time.Time           `json:"chatTimeoutIssuedAt"`
	ChatReports         map[string]domain.ChatReport              `json:"chatReports"`
	Appeals             map[string]domain.Appeal                  `json:"appeals"`
	AppealEvents        map[string][]domain.AppealEvent           `json:"appealEvents"`
	ChatFilters         map[string]domain.ChatFilter              `json:"chatFilters"`
	ChatAutoModActions  map[string]domain.ChatAutoModAction       `json:"chatAutoModActions"`
	Tips                map[string]domain.Tip                     `json:"tips"`
	Subscriptions       map[string]domain.Subscription            `json:"subscriptions"`
	PaymentTransactions map[string]domain.PaymentTransaction      `json:"paymentTransactions"`
	Profiles            map[string]domain.Profile                 `json:"profiles"`
	Follows             map[string]map[string]time.Time           `json:"follows"`
	Recordings          map[string]domain.Recording               `json:"recordings"`
	Uploads             map[string]domain.Upload                  `json:"uploads"`
	ClipExports         map[string]domain.ClipExport              `json:"clipExports"`
	DMCACases           map[string]domain.DMCACase                `json:"dmcaCases"`
	DataSubjectRequests map[string]domain.DataSubjectRequest      `json:"dataSubjectRequests"`
	DataSubjectAudit    map[string][]domain.DataSubjectAuditEvent `json:"dataSubjectAudit"`
	LegalStateHistory   []domain.LegalStateHistory                `json:"legalStateHistory"`
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

type chatFilterParams struct {
	Kind    string
	Pattern string
	Enabled bool
}

type chatFilterUpdate struct {
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

type ClipExportParams = domain.ClipExportParams
type CreateUploadParams = domain.UploadCreateParams
type UploadUpdate = domain.UploadUpdate
type CreateUserParams = domain.UserCreateParams
type OAuthLoginParams = domain.OAuthLoginParams
type CreateTipParams = domain.TipCreateParams
type CreateSubscriptionParams = domain.SubscriptionCreateParams
type ProcessPaymentWebhookParams = domain.ProcessPaymentWebhookParams
type CreateDMCACaseParams = domain.DMCACaseCreateParams
type DMCACaseUpdate = domain.DMCACaseUpdate
type CreateDataSubjectRequestParams = domain.DataSubjectRequestCreateParams
type DataSubjectRequestUpdate = domain.DataSubjectRequestUpdate
type CreateDataSubjectAuditEventParams = domain.DataSubjectAuditEventCreateParams
