package storage

import (
	"errors"
	"time"

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

// RecordingRetentionPolicy specifies how long recordings are kept before being
// purged when unpublished or published.
type RecordingRetentionPolicy struct {
	Published   time.Duration
	Unpublished time.Duration
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

const defaultObjectStorageRequestTimeout = 30 * time.Second

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
	ChannelID     string
	FromUserID    string
	Amount        models.Money
	Currency      string
	Provider      string
	Reference     string
	WalletAddress string
	Message       string
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
}
