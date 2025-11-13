package models

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const (
	moneyFractionDigits = 8
	moneyScale          = int64(100000000)
)

// Money represents a currency amount stored in minor units (1e-8 of the major
// currency) to avoid floating point rounding issues. JSON encoding and string
// formatting expose the canonical decimal representation while all internal
// operations use the fixed-precision integer value.
type Money struct {
	minorUnits int64
}

// NewMoneyFromMinorUnits constructs a Money value from its minor-unit
// representation.
func NewMoneyFromMinorUnits(units int64) Money {
	return Money{minorUnits: units}
}

// MinorUnits exposes the internal integer representation scaled by 1e-8.
func (m Money) MinorUnits() int64 {
	return m.minorUnits
}

// Add returns the sum of two Money values.
func (m Money) Add(other Money) Money {
	return Money{minorUnits: m.minorUnits + other.minorUnits}
}

// IsZero reports whether the amount is exactly zero.
func (m Money) IsZero() bool {
	return m.minorUnits == 0
}

// DecimalString returns the canonical decimal representation with up to eight
// fractional digits.
func (m Money) DecimalString() string {
	return formatMinorUnits(m.minorUnits)
}

// String implements fmt.Stringer.
func (m Money) String() string {
	return m.DecimalString()
}

// MarshalJSON encodes the fixed-precision amount as a JSON number.
func (m Money) MarshalJSON() ([]byte, error) {
	return []byte(m.DecimalString()), nil
}

// UnmarshalJSON decodes a JSON number or string into the fixed-precision minor
// unit representation. A JSON null resets the value to zero.
func (m *Money) UnmarshalJSON(data []byte) error {
	if m == nil {
		return fmt.Errorf("models: cannot decode into nil Money pointer")
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*m = Money{}
		return nil
	}
	var raw string
	if data[0] == '"' {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("decode money string: %w", err)
		}
	} else {
		raw = trimmed
	}
	money, err := ParseMoney(raw)
	if err != nil {
		return err
	}
	*m = money
	return nil
}

// ParseMoney parses a human-readable decimal string into a Money value with up
// to eight fractional digits.
func ParseMoney(value string) (Money, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return Money{}, fmt.Errorf("invalid money amount")
	}
	rat, ok := new(big.Rat).SetString(trimmed)
	if !ok {
		return Money{}, fmt.Errorf("invalid money amount")
	}
	rat.Mul(rat, big.NewRat(moneyScale, 1))
	if !rat.IsInt() {
		return Money{}, fmt.Errorf("amount supports up to %d decimal places", moneyFractionDigits)
	}
	numerator := rat.Num()
	if !numerator.IsInt64() {
		return Money{}, fmt.Errorf("money amount out of range")
	}
	return Money{minorUnits: numerator.Int64()}, nil
}

// MustParseMoney panics if the value cannot be parsed. It is intended for
// tests and static initialisation.
func MustParseMoney(value string) Money {
	money, err := ParseMoney(value)
	if err != nil {
		panic(err)
	}
	return money
}

func formatMinorUnits(units int64) string {
	negative := units < 0
	if negative {
		units = -units
	}
	major := units / moneyScale
	minor := units % moneyScale
	var builder strings.Builder
	if negative {
		builder.WriteByte('-')
	}
	builder.WriteString(fmt.Sprintf("%d", major))
	if minor == 0 {
		return builder.String()
	}
	builder.WriteByte('.')
	fraction := fmt.Sprintf("%0*d", moneyFractionDigits, minor)
	fraction = strings.TrimRight(fraction, "0")
	builder.WriteString(fraction)
	return builder.String()
}

type User struct {
	ID           string    `json:"id"`
	DisplayName  string    `json:"displayName"`
	Email        string    `json:"email"`
	Roles        []string  `json:"roles"`
	PasswordHash string    `json:"passwordHash,omitempty"`
	SelfSignup   bool      `json:"selfSignup"`
	CreatedAt    time.Time `json:"createdAt"`
}

// HasRole reports whether the user has the provided role, ignoring case.
func (u User) HasRole(role string) bool {
	for _, existing := range u.Roles {
		if strings.EqualFold(existing, role) {
			return true
		}
	}
	return false
}

type OAuthAccount struct {
	Provider    string    `json:"provider"`
	Subject     string    `json:"subject"`
	UserID      string    `json:"userId"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	LinkedAt    time.Time `json:"linkedAt"`
}

type Channel struct {
	ID               string    `json:"id"`
	OwnerID          string    `json:"ownerId"`
	StreamKey        string    `json:"streamKey"`
	Title            string    `json:"title"`
	Category         string    `json:"category,omitempty"`
	Tags             []string  `json:"tags"`
	LiveState        string    `json:"liveState"`
	CurrentSessionID *string   `json:"currentSessionId,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type StreamSession struct {
	ID                 string              `json:"id"`
	ChannelID          string              `json:"channelId"`
	StartedAt          time.Time           `json:"startedAt"`
	EndedAt            *time.Time          `json:"endedAt,omitempty"`
	Renditions         []string            `json:"renditions"`
	PeakConcurrent     int                 `json:"peakConcurrent"`
	OriginURL          string              `json:"originUrl,omitempty"`
	PlaybackURL        string              `json:"playbackUrl,omitempty"`
	IngestEndpoints    []string            `json:"ingestEndpoints,omitempty"`
	IngestJobIDs       []string            `json:"ingestJobIds,omitempty"`
	RenditionManifests []RenditionManifest `json:"renditionManifests,omitempty"`
}

type RenditionManifest struct {
	Name        string `json:"name"`
	ManifestURL string `json:"manifestUrl"`
	Bitrate     int    `json:"bitrate,omitempty"`
}

type Recording struct {
	ID              string               `json:"id"`
	ChannelID       string               `json:"channelId"`
	SessionID       string               `json:"sessionId"`
	Title           string               `json:"title"`
	DurationSeconds int                  `json:"durationSeconds"`
	PlaybackBaseURL string               `json:"playbackBaseUrl,omitempty"`
	Renditions      []RecordingRendition `json:"renditions,omitempty"`
	Thumbnails      []RecordingThumbnail `json:"thumbnails,omitempty"`
	Metadata        map[string]string    `json:"metadata,omitempty"`
	PublishedAt     *time.Time           `json:"publishedAt,omitempty"`
	CreatedAt       time.Time            `json:"createdAt"`
	RetainUntil     *time.Time           `json:"retainUntil,omitempty"`
	Clips           []ClipExportSummary  `json:"clips,omitempty"`
}

type RecordingRendition struct {
	Name        string `json:"name"`
	ManifestURL string `json:"manifestUrl"`
	Bitrate     int    `json:"bitrate,omitempty"`
}

type RecordingThumbnail struct {
	ID          string    `json:"id"`
	RecordingID string    `json:"recordingId"`
	URL         string    `json:"url"`
	Width       int       `json:"width,omitempty"`
	Height      int       `json:"height,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Upload struct {
	ID          string            `json:"id"`
	ChannelID   string            `json:"channelId"`
	Title       string            `json:"title"`
	Filename    string            `json:"filename"`
	SizeBytes   int64             `json:"sizeBytes"`
	Status      string            `json:"status"`
	Progress    int               `json:"progress"`
	RecordingID *string           `json:"recordingId,omitempty"`
	PlaybackURL string            `json:"playbackUrl,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	CompletedAt *time.Time        `json:"completedAt,omitempty"`
}

type ClipExport struct {
	ID            string     `json:"id"`
	RecordingID   string     `json:"recordingId"`
	ChannelID     string     `json:"channelId"`
	SessionID     string     `json:"sessionId"`
	Title         string     `json:"title"`
	StartSeconds  int        `json:"startSeconds"`
	EndSeconds    int        `json:"endSeconds"`
	Status        string     `json:"status"`
	PlaybackURL   string     `json:"playbackUrl,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
	StorageObject string     `json:"storageObject,omitempty"`
}

type ClipExportSummary struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	StartSeconds int    `json:"startSeconds"`
	EndSeconds   int    `json:"endSeconds"`
	Status       string `json:"status"`
}

type ChatMessage struct {
	ID        string    `json:"id"`
	ChannelID string    `json:"channelId"`
	UserID    string    `json:"userId"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type ChatReport struct {
	ID          string     `json:"id"`
	ChannelID   string     `json:"channelId"`
	ReporterID  string     `json:"reporterId"`
	TargetID    string     `json:"targetId"`
	Reason      string     `json:"reason"`
	MessageID   string     `json:"messageId,omitempty"`
	EvidenceURL string     `json:"evidenceUrl,omitempty"`
	Status      string     `json:"status"`
	Resolution  string     `json:"resolution,omitempty"`
	ResolverID  string     `json:"resolverId,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`
}

type ChatRestriction struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	ChannelID string     `json:"channelId"`
	TargetID  string     `json:"targetId"`
	ActorID   string     `json:"actorId,omitempty"`
	Reason    string     `json:"reason,omitempty"`
	IssuedAt  time.Time  `json:"issuedAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// Tip describes a viewer tip recorded for a channel. Amount uses the fixed
// precision Money type (1e-8 minor units) while the public JSON API continues to
// expose human-readable decimal values.
type Tip struct {
	ID            string    `json:"id"`
	ChannelID     string    `json:"channelId"`
	FromUserID    string    `json:"fromUserId"`
	Amount        Money     `json:"amount"`
	Currency      string    `json:"currency"`
	Provider      string    `json:"provider"`
	Reference     string    `json:"reference"`
	WalletAddress string    `json:"walletAddress,omitempty"`
	Message       string    `json:"message,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Subscription represents a recurring or fixed-term monetization commitment.
// Amount uses the Money type to preserve precision; clients continue to see
// decimal values over JSON.
type Subscription struct {
	ID                string     `json:"id"`
	ChannelID         string     `json:"channelId"`
	UserID            string     `json:"userId"`
	Tier              string     `json:"tier"`
	Provider          string     `json:"provider"`
	Reference         string     `json:"reference"`
	Amount            Money      `json:"amount"`
	Currency          string     `json:"currency"`
	StartedAt         time.Time  `json:"startedAt"`
	ExpiresAt         time.Time  `json:"expiresAt"`
	AutoRenew         bool       `json:"autoRenew"`
	Status            string     `json:"status"`
	CancelledBy       string     `json:"cancelledBy,omitempty"`
	CancelledReason   string     `json:"cancelledReason,omitempty"`
	CancelledAt       *time.Time `json:"cancelledAt,omitempty"`
	ExternalReference string     `json:"externalReference,omitempty"`
}

type CryptoAddress struct {
	Currency string `json:"currency"`
	Address  string `json:"address"`
	Note     string `json:"note,omitempty"`
}

type Profile struct {
	UserID            string          `json:"userId"`
	Bio               string          `json:"bio"`
	AvatarURL         string          `json:"avatarUrl"`
	BannerURL         string          `json:"bannerUrl"`
	FeaturedChannelID *string         `json:"featuredChannelId,omitempty"`
	TopFriends        []string        `json:"topFriends"`
	DonationAddresses []CryptoAddress `json:"donationAddresses"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}
