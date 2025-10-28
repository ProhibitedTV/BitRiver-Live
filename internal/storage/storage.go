package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
	"golang.org/x/crypto/pbkdf2"
)

const (
	passwordHashSaltLength = 16
	passwordHashKeyLength  = 32
	passwordHashIterations = 120000

	metadataManifestPrefix  = "object:manifest:"
	metadataThumbnailPrefix = "object:thumbnail:"
)

type dataset struct {
	Users               map[string]models.User          `json:"users"`
	Channels            map[string]models.Channel       `json:"channels"`
	StreamSessions      map[string]models.StreamSession `json:"streamSessions"`
	ChatMessages        map[string]models.ChatMessage   `json:"chatMessages"`
	ChatBans            map[string]map[string]time.Time `json:"chatBans"`
	ChatTimeouts        map[string]map[string]time.Time `json:"chatTimeouts"`
	ChatBanActors       map[string]map[string]string    `json:"chatBanActors"`
	ChatBanReasons      map[string]map[string]string    `json:"chatBanReasons"`
	ChatTimeoutActors   map[string]map[string]string    `json:"chatTimeoutActors"`
	ChatTimeoutReasons  map[string]map[string]string    `json:"chatTimeoutReasons"`
	ChatTimeoutIssuedAt map[string]map[string]time.Time `json:"chatTimeoutIssuedAt"`
	ChatReports         map[string]models.ChatReport    `json:"chatReports"`
	Tips                map[string]models.Tip           `json:"tips"`
	Subscriptions       map[string]models.Subscription  `json:"subscriptions"`
	Profiles            map[string]models.Profile       `json:"profiles"`
	Follows             map[string]map[string]time.Time `json:"follows"`
	Recordings          map[string]models.Recording     `json:"recordings"`
	ClipExports         map[string]models.ClipExport    `json:"clipExports"`
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
	ingestHealth        []ingest.HealthStatus
	ingestHealthUpdated time.Time
	recordingRetention  RecordingRetentionPolicy
	objectStorage       ObjectStorageConfig
	objectClient        objectStorageClient
}

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

type noopObjectStorageClient struct{}

func (noopObjectStorageClient) Enabled() bool { return false }

func (noopObjectStorageClient) Upload(ctx context.Context, key, contentType string, body []byte) (objectReference, error) {
	return objectReference{}, nil
}

func (noopObjectStorageClient) Delete(ctx context.Context, key string) error {
	return nil
}

type prefixedObjectStorageClient struct {
	cfg ObjectStorageConfig
}

func (c *prefixedObjectStorageClient) Enabled() bool { return true }

func (c *prefixedObjectStorageClient) Upload(ctx context.Context, key, contentType string, body []byte) (objectReference, error) {
	finalKey := c.withPrefix(key)
	return objectReference{Key: finalKey, URL: c.publicURL(finalKey)}, nil
}

func (c *prefixedObjectStorageClient) Delete(ctx context.Context, key string) error {
	return nil
}

func (c *prefixedObjectStorageClient) withPrefix(key string) string {
	trimmed := strings.TrimLeft(key, "/")
	prefix := strings.Trim(c.cfg.Prefix, "/")
	if prefix == "" {
		return trimmed
	}
	if trimmed == "" {
		return prefix
	}
	return prefix + "/" + trimmed
}

func (c *prefixedObjectStorageClient) publicURL(key string) string {
	base := strings.TrimSpace(c.cfg.PublicEndpoint)
	if base == "" {
		return ""
	}
	trimmedBase := strings.TrimRight(base, "/")
	trimmedKey := strings.TrimLeft(key, "/")
	if trimmedKey == "" {
		return trimmedBase
	}
	return trimmedBase + "/" + trimmedKey
}

func newObjectStorageClient(cfg ObjectStorageConfig) objectStorageClient {
	if strings.TrimSpace(cfg.Bucket) == "" && strings.TrimSpace(cfg.PublicEndpoint) == "" {
		return noopObjectStorageClient{}
	}
	return &prefixedObjectStorageClient{cfg: cfg}
}

// ClipExportParams captures the request to generate a recording clip.
type ClipExportParams struct {
	Title        string
	StartSeconds int
	EndSeconds   int
}

func newDataset() dataset {
	return dataset{
		Users:               make(map[string]models.User),
		Channels:            make(map[string]models.Channel),
		StreamSessions:      make(map[string]models.StreamSession),
		ChatMessages:        make(map[string]models.ChatMessage),
		ChatBans:            make(map[string]map[string]time.Time),
		ChatTimeouts:        make(map[string]map[string]time.Time),
		ChatBanActors:       make(map[string]map[string]string),
		ChatBanReasons:      make(map[string]map[string]string),
		ChatTimeoutActors:   make(map[string]map[string]string),
		ChatTimeoutReasons:  make(map[string]map[string]string),
		ChatTimeoutIssuedAt: make(map[string]map[string]time.Time),
		ChatReports:         make(map[string]models.ChatReport),
		Tips:                make(map[string]models.Tip),
		Subscriptions:       make(map[string]models.Subscription),
		Profiles:            make(map[string]models.Profile),
		Follows:             make(map[string]map[string]time.Time),
		Recordings:          make(map[string]models.Recording),
		ClipExports:         make(map[string]models.ClipExport),
	}
}

func (s *Storage) ensureDatasetInitializedLocked() {
	if s.data.Users == nil {
		s.data.Users = make(map[string]models.User)
	}
	if s.data.Channels == nil {
		s.data.Channels = make(map[string]models.Channel)
	}
	if s.data.StreamSessions == nil {
		s.data.StreamSessions = make(map[string]models.StreamSession)
	}
	if s.data.ChatMessages == nil {
		s.data.ChatMessages = make(map[string]models.ChatMessage)
	}
	if s.data.ChatBans == nil {
		s.data.ChatBans = make(map[string]map[string]time.Time)
	}
	if s.data.ChatTimeouts == nil {
		s.data.ChatTimeouts = make(map[string]map[string]time.Time)
	}
	if s.data.ChatBanActors == nil {
		s.data.ChatBanActors = make(map[string]map[string]string)
	}
	if s.data.ChatBanReasons == nil {
		s.data.ChatBanReasons = make(map[string]map[string]string)
	}
	if s.data.ChatTimeoutActors == nil {
		s.data.ChatTimeoutActors = make(map[string]map[string]string)
	}
	if s.data.ChatTimeoutReasons == nil {
		s.data.ChatTimeoutReasons = make(map[string]map[string]string)
	}
	if s.data.ChatTimeoutIssuedAt == nil {
		s.data.ChatTimeoutIssuedAt = make(map[string]map[string]time.Time)
	}
	if s.data.ChatReports == nil {
		s.data.ChatReports = make(map[string]models.ChatReport)
	}
	if s.data.Tips == nil {
		s.data.Tips = make(map[string]models.Tip)
	}
	if s.data.Subscriptions == nil {
		s.data.Subscriptions = make(map[string]models.Subscription)
	}
	if s.data.Profiles == nil {
		s.data.Profiles = make(map[string]models.Profile)
	}
	if s.data.Follows == nil {
		s.data.Follows = make(map[string]map[string]time.Time)
	}
	if s.data.Recordings == nil {
		s.data.Recordings = make(map[string]models.Recording)
	}
	if s.data.ClipExports == nil {
		s.data.ClipExports = make(map[string]models.ClipExport)
	}
}

func buildObjectKey(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.Trim(part, "/")
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return strings.Join(normalized, "/")
}

func normalizeObjectComponent(input string) string {
	lowered := strings.ToLower(strings.TrimSpace(input))
	if lowered == "" {
		return "item"
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range lowered {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	normalized := strings.Trim(builder.String(), "-")
	if normalized == "" {
		return "item"
	}
	return normalized
}

func manifestMetadataKey(name string) string {
	return metadataManifestPrefix + normalizeObjectComponent(name)
}

func thumbnailMetadataKey(id string) string {
	return metadataThumbnailPrefix + id
}

func (s *Storage) ensureBanMetadata(channelID string) {
	if s.data.ChatBanActors == nil {
		s.data.ChatBanActors = make(map[string]map[string]string)
	}
	if s.data.ChatBanActors[channelID] == nil {
		s.data.ChatBanActors[channelID] = make(map[string]string)
	}
	if s.data.ChatBanReasons == nil {
		s.data.ChatBanReasons = make(map[string]map[string]string)
	}
	if s.data.ChatBanReasons[channelID] == nil {
		s.data.ChatBanReasons[channelID] = make(map[string]string)
	}
}

func (s *Storage) ensureTimeoutMetadata(channelID string) {
	if s.data.ChatTimeoutActors == nil {
		s.data.ChatTimeoutActors = make(map[string]map[string]string)
	}
	if s.data.ChatTimeoutActors[channelID] == nil {
		s.data.ChatTimeoutActors[channelID] = make(map[string]string)
	}
	if s.data.ChatTimeoutReasons == nil {
		s.data.ChatTimeoutReasons = make(map[string]map[string]string)
	}
	if s.data.ChatTimeoutReasons[channelID] == nil {
		s.data.ChatTimeoutReasons[channelID] = make(map[string]string)
	}
	if s.data.ChatTimeoutIssuedAt == nil {
		s.data.ChatTimeoutIssuedAt = make(map[string]map[string]time.Time)
	}
	if s.data.ChatTimeoutIssuedAt[channelID] == nil {
		s.data.ChatTimeoutIssuedAt[channelID] = make(map[string]time.Time)
	}
}

var (
	ErrInvalidCredentials       = errors.New("invalid credentials")
	ErrPasswordLoginUnsupported = errors.New("account does not support password login")
)

// CreateUserParams captures the attributes that can be set when creating a user.
type CreateUserParams struct {
	DisplayName string
	Email       string
	Password    string
	Roles       []string
	SelfSignup  bool
}

// CreateTipParams captures the information required to record a tip.
type CreateTipParams struct {
	ChannelID     string
	FromUserID    string
	Amount        float64
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
	Amount            float64
	Currency          string
	Duration          time.Duration
	AutoRenew         bool
	ExternalReference string
}

func normalizeRoles(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	roles := make([]string, 0, len(input))
	seen := make(map[string]struct{})
	for _, role := range input {
		trimmed := strings.TrimSpace(role)
		if trimmed == "" {
			continue
		}
		normalized := strings.ToLower(trimmed)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		roles = append(roles, normalized)
	}
	if len(roles) == 0 {
		return nil
	}
	sort.Strings(roles)
	return roles
}

func NewStorage(path string, opts ...Option) (*Storage, error) {
	store := &Storage{
		filePath:            path,
		ingestController:    ingest.NoopController{},
		ingestMaxAttempts:   1,
		ingestHealth:        []ingest.HealthStatus{{Component: "ingest", Status: "disabled"}},
		ingestHealthUpdated: time.Now().UTC(),
		recordingRetention: RecordingRetentionPolicy{
			Published:   90 * 24 * time.Hour,
			Unpublished: 14 * 24 * time.Hour,
		},
		objectClient: noopObjectStorageClient{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt.applyJSON(store)
		}
	}
	if store.ingestController == nil {
		store.ingestController = ingest.NoopController{}
	}
	if store.ingestMaxAttempts <= 0 {
		store.ingestMaxAttempts = 1
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	store.objectClient = newObjectStorageClient(store.objectStorage)
	return store, nil
}

func (s *Storage) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	file, err := os.Open(s.filePath)
	if errors.Is(err, os.ErrNotExist) {
		s.data = newDataset()
		return nil
	} else if err != nil {
		return fmt.Errorf("open store file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&s.data); err != nil {
		if errors.Is(err, io.EOF) {
			s.data = newDataset()
			return nil
		}
		return fmt.Errorf("decode store file: %w", err)
	}

	s.ensureDatasetInitializedLocked()

	return nil
}

func (s *Storage) persist() error {
	return s.persistDataset(s.data)
}

func (s *Storage) persistDataset(data dataset) error {
	if s.persistOverride != nil {
		if err := s.persistOverride(data); err != nil {
			return err
		}
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "store-*.json")
	if err != nil {
		return fmt.Errorf("create temp store file: %w", err)
	}
	tmpPath := tmpFile.Name()
	success := false
	defer func() {
		if !success {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("encode store file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("flush store file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp store file: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		return fmt.Errorf("replace store file: %w", err)
	}
	success = true
	return nil
}

func cloneDataset(src dataset) dataset {
	clone := dataset{}

	if src.Users != nil {
		clone.Users = make(map[string]models.User, len(src.Users))
		for id, user := range src.Users {
			cloned := user
			if user.Roles != nil {
				cloned.Roles = append([]string(nil), user.Roles...)
			}
			clone.Users[id] = cloned
		}
	}

	if src.Channels != nil {
		clone.Channels = make(map[string]models.Channel, len(src.Channels))
		for id, channel := range src.Channels {
			cloned := channel
			if channel.Tags != nil {
				cloned.Tags = append([]string(nil), channel.Tags...)
			}
			if channel.CurrentSessionID != nil {
				current := *channel.CurrentSessionID
				cloned.CurrentSessionID = &current
			}
			clone.Channels[id] = cloned
		}
	}

	if src.StreamSessions != nil {
		clone.StreamSessions = make(map[string]models.StreamSession, len(src.StreamSessions))
		for id, session := range src.StreamSessions {
			cloned := session
			if session.Renditions != nil {
				cloned.Renditions = append([]string(nil), session.Renditions...)
			}
			if session.EndedAt != nil {
				ended := *session.EndedAt
				cloned.EndedAt = &ended
			}
			clone.StreamSessions[id] = cloned
		}
	}

	if src.ChatMessages != nil {
		clone.ChatMessages = make(map[string]models.ChatMessage, len(src.ChatMessages))
		for id, message := range src.ChatMessages {
			clone.ChatMessages[id] = message
		}
	}

	if src.ChatBans != nil {
		clone.ChatBans = make(map[string]map[string]time.Time, len(src.ChatBans))
		for channelID, bans := range src.ChatBans {
			if bans == nil {
				clone.ChatBans[channelID] = nil
				continue
			}
			cloned := make(map[string]time.Time, len(bans))
			for userID, issuedAt := range bans {
				cloned[userID] = issuedAt
			}
			clone.ChatBans[channelID] = cloned
		}
	}

	if src.ChatTimeouts != nil {
		clone.ChatTimeouts = make(map[string]map[string]time.Time, len(src.ChatTimeouts))
		for channelID, timeouts := range src.ChatTimeouts {
			if timeouts == nil {
				clone.ChatTimeouts[channelID] = nil
				continue
			}
			cloned := make(map[string]time.Time, len(timeouts))
			for userID, expiry := range timeouts {
				cloned[userID] = expiry
			}
			clone.ChatTimeouts[channelID] = cloned
		}
	}

	if src.Recordings != nil {
		clone.Recordings = make(map[string]models.Recording, len(src.Recordings))
		for id, recording := range src.Recordings {
			clone.Recordings[id] = cloneRecording(recording)
		}
	}

	if src.ClipExports != nil {
		clone.ClipExports = make(map[string]models.ClipExport, len(src.ClipExports))
		for id, clip := range src.ClipExports {
			clone.ClipExports[id] = cloneClipExport(clip)
		}
	}

	if src.Profiles != nil {
		clone.Profiles = make(map[string]models.Profile, len(src.Profiles))
		for id, profile := range src.Profiles {
			cloned := profile
			if profile.TopFriends != nil {
				cloned.TopFriends = append([]string(nil), profile.TopFriends...)
			}
			if profile.DonationAddresses != nil {
				cloned.DonationAddresses = append([]models.CryptoAddress(nil), profile.DonationAddresses...)
			}
			if profile.FeaturedChannelID != nil {
				featured := *profile.FeaturedChannelID
				cloned.FeaturedChannelID = &featured
			}
			clone.Profiles[id] = cloned
		}
	}

	if src.Follows != nil {
		clone.Follows = make(map[string]map[string]time.Time, len(src.Follows))
		for userID, channels := range src.Follows {
			if channels == nil {
				clone.Follows[userID] = nil
				continue
			}
			followed := make(map[string]time.Time, len(channels))
			for channelID, followedAt := range channels {
				followed[channelID] = followedAt
			}
			clone.Follows[userID] = followed
		}
	}

	return clone
}

func cloneRecording(recording models.Recording) models.Recording {
	cloned := recording
	if recording.Renditions != nil {
		cloned.Renditions = append([]models.RecordingRendition(nil), recording.Renditions...)
	}
	if recording.Thumbnails != nil {
		cloned.Thumbnails = append([]models.RecordingThumbnail(nil), recording.Thumbnails...)
	}
	if recording.Metadata != nil {
		meta := make(map[string]string, len(recording.Metadata))
		for k, v := range recording.Metadata {
			meta[k] = v
		}
		cloned.Metadata = meta
	}
	if recording.PublishedAt != nil {
		published := *recording.PublishedAt
		cloned.PublishedAt = &published
	}
	if recording.RetainUntil != nil {
		retain := *recording.RetainUntil
		cloned.RetainUntil = &retain
	}
	if recording.Clips != nil {
		cloned.Clips = append([]models.ClipExportSummary(nil), recording.Clips...)
	}
	return cloned
}

func cloneClipExport(clip models.ClipExport) models.ClipExport {
	cloned := clip
	if clip.CompletedAt != nil {
		completed := *clip.CompletedAt
		cloned.CompletedAt = &completed
	}
	return cloned
}

func (s *Storage) recordingDeadline(now time.Time, published bool) *time.Time {
	var window time.Duration
	if published {
		window = s.recordingRetention.Published
	} else {
		window = s.recordingRetention.Unpublished
	}
	if window <= 0 {
		return nil
	}
	deadline := now.Add(window)
	return &deadline
}

func (s *Storage) purgeExpiredRecordingsLocked(now time.Time) (bool, dataset, error) {
	if len(s.data.Recordings) == 0 {
		return false, dataset{}, nil
	}
	removed := false
	snapshotTaken := false
	var snapshot dataset
	for id, recording := range s.data.Recordings {
		if recording.RetainUntil == nil || now.Before(*recording.RetainUntil) {
			continue
		}
		if !snapshotTaken {
			snapshot = cloneDataset(s.data)
			snapshotTaken = true
		}
		if err := s.deleteRecordingArtifactsLocked(recording); err != nil {
			if snapshotTaken {
				s.data = snapshot
			}
			return false, dataset{}, err
		}
		for clipID, clip := range s.data.ClipExports {
			if clip.RecordingID != id {
				continue
			}
			if err := s.deleteClipArtifactsLocked(clip); err != nil {
				if snapshotTaken {
					s.data = snapshot
				}
				return false, dataset{}, err
			}
			delete(s.data.ClipExports, clipID)
		}
		delete(s.data.Recordings, id)
		removed = true
	}
	if !removed {
		return false, dataset{}, nil
	}
	return true, snapshot, nil
}

func (s *Storage) recordingWithClipsLocked(recording models.Recording) models.Recording {
	cloned := cloneRecording(recording)
	if len(s.data.ClipExports) == 0 {
		return cloned
	}
	var clips []models.ClipExportSummary
	for _, clip := range s.data.ClipExports {
		if clip.RecordingID != recording.ID {
			continue
		}
		clips = append(clips, models.ClipExportSummary{
			ID:           clip.ID,
			Title:        clip.Title,
			StartSeconds: clip.StartSeconds,
			EndSeconds:   clip.EndSeconds,
			Status:       clip.Status,
		})
	}
	if len(clips) == 0 {
		return cloned
	}
	sort.Slice(clips, func(i, j int) bool {
		if clips[i].StartSeconds == clips[j].StartSeconds {
			return clips[i].ID < clips[j].ID
		}
		return clips[i].StartSeconds < clips[j].StartSeconds
	})
	cloned.Clips = clips
	return cloned
}

func (s *Storage) generateID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func (s *Storage) generateStreamKey() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate stream key: %w", err)
	}
	return strings.ToUpper(hex.EncodeToString(bytes)), nil
}

// User operations

func (s *Storage) CreateUser(params CreateUserParams) (models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedEmail := strings.TrimSpace(strings.ToLower(params.Email))
	if normalizedEmail == "" {
		return models.User{}, errors.New("email is required")
	}
	for _, user := range s.data.Users {
		if user.Email == normalizedEmail {
			return models.User{}, fmt.Errorf("email %s already in use", params.Email)
		}
	}

	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		return models.User{}, errors.New("displayName is required")
	}

	roles := normalizeRoles(params.Roles)
	if params.SelfSignup {
		if params.Password == "" {
			return models.User{}, errors.New("password is required for self-service signup")
		}
		if len(roles) == 0 {
			roles = []string{"viewer"}
		}
	}

	id, err := s.generateID()
	if err != nil {
		return models.User{}, err
	}

	var passwordHash string
	if params.Password != "" {
		hashed, hashErr := hashPassword(params.Password)
		if hashErr != nil {
			return models.User{}, fmt.Errorf("hash password: %w", hashErr)
		}
		passwordHash = hashed
	}

	now := time.Now().UTC()
	user := models.User{
		ID:           id,
		DisplayName:  displayName,
		Email:        normalizedEmail,
		Roles:        roles,
		PasswordHash: passwordHash,
		SelfSignup:   params.SelfSignup,
		CreatedAt:    now,
	}

	s.data.Users[id] = user
	if err := s.persist(); err != nil {
		delete(s.data.Users, id)
		return models.User{}, err
	}

	return user, nil
}

func (s *Storage) ListUsers() []models.User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]models.User, 0, len(s.data.Users))
	for _, user := range s.data.Users {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].CreatedAt.Before(users[j].CreatedAt)
	})
	return users
}

func (s *Storage) GetUser(id string) (models.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.data.Users[id]
	return user, ok
}

// FindUserByEmail looks up a user by their normalized email address.
func (s *Storage) FindUserByEmail(email string) (models.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	normalizedEmail := strings.TrimSpace(strings.ToLower(email))
	for _, user := range s.data.Users {
		if user.Email == normalizedEmail {
			return user, true
		}
	}
	return models.User{}, false
}

// AuthenticateUser verifies credentials and returns the matching user on success.
func (s *Storage) AuthenticateUser(email, password string) (models.User, error) {
	if password == "" {
		return models.User{}, errors.New("password is required")
	}
	user, ok := s.FindUserByEmail(email)
	if !ok {
		return models.User{}, ErrInvalidCredentials
	}
	if user.PasswordHash == "" {
		return models.User{}, ErrPasswordLoginUnsupported
	}
	if err := verifyPassword(user.PasswordHash, password); err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return models.User{}, ErrInvalidCredentials
		}
		return models.User{}, err
	}
	return user, nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, passwordHashSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	derived := pbkdf2.Key([]byte(password), salt, passwordHashIterations, passwordHashKeyLength, sha256.New)
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedKey := base64.RawStdEncoding.EncodeToString(derived)
	return fmt.Sprintf("pbkdf2$sha256$%d$%s$%s", passwordHashIterations, encodedSalt, encodedKey), nil
}

func verifyPassword(encodedHash, candidate string) error {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 5 {
		return fmt.Errorf("verify password: invalid hash format")
	}
	if parts[0] != "pbkdf2" || parts[1] != "sha256" {
		return fmt.Errorf("verify password: unsupported hash identifier")
	}
	iterations, err := strconv.Atoi(parts[2])
	if err != nil || iterations <= 0 {
		return fmt.Errorf("verify password: invalid iteration count")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return fmt.Errorf("verify password: decode salt: %w", err)
	}
	storedKey, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return fmt.Errorf("verify password: decode hash: %w", err)
	}
	derived := pbkdf2.Key([]byte(candidate), salt, iterations, len(storedKey), sha256.New)
	if len(derived) != len(storedKey) || subtle.ConstantTimeCompare(derived, storedKey) != 1 {
		return ErrInvalidCredentials
	}
	return nil
}

// UserUpdate represents the fields that can be modified for an existing user.
type UserUpdate struct {
	DisplayName *string
	Email       *string
	Roles       *[]string
}

// UpdateUser mutates user metadata while enforcing uniqueness constraints.
func (s *Storage) UpdateUser(id string, update UserUpdate) (models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	user, ok := updatedData.Users[id]
	if !ok {
		return models.User{}, fmt.Errorf("user %s not found", id)
	}

	if update.DisplayName != nil {
		name := strings.TrimSpace(*update.DisplayName)
		if name == "" {
			return models.User{}, errors.New("displayName cannot be empty")
		}
		user.DisplayName = name
	}

	if update.Email != nil {
		email := strings.TrimSpace(strings.ToLower(*update.Email))
		if email == "" {
			return models.User{}, errors.New("email cannot be empty")
		}
		for existingID, existing := range updatedData.Users {
			if existingID == user.ID {
				continue
			}
			if existing.Email == email {
				return models.User{}, fmt.Errorf("email %s already in use", email)
			}
		}
		user.Email = email
	}

	if update.Roles != nil {
		user.Roles = normalizeRoles(*update.Roles)
	}

	updatedData.Users[id] = user
	if err := s.persistDataset(updatedData); err != nil {
		return models.User{}, err
	}

	s.data = updatedData

	return user, nil
}

// DeleteUser removes the user, related profile, and chat history.
func (s *Storage) DeleteUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	if _, ok := updatedData.Users[id]; !ok {
		return fmt.Errorf("user %s not found", id)
	}

	for _, channel := range updatedData.Channels {
		if channel.OwnerID == id {
			return fmt.Errorf("user %s owns channel %s; transfer or delete the channel first", id, channel.ID)
		}
	}

	delete(updatedData.Users, id)
	delete(updatedData.Profiles, id)
	delete(updatedData.Follows, id)

	now := time.Now().UTC()
	for profileID, profile := range updatedData.Profiles {
		filtered := make([]string, 0, len(profile.TopFriends))
		for _, friend := range profile.TopFriends {
			if friend == id {
				continue
			}
			filtered = append(filtered, friend)
		}
		if len(filtered) != len(profile.TopFriends) {
			profile.TopFriends = filtered
			profile.UpdatedAt = now
			updatedData.Profiles[profileID] = profile
		}
	}

	for messageID, message := range updatedData.ChatMessages {
		if message.UserID == id {
			delete(updatedData.ChatMessages, messageID)
		}
	}

	if err := s.persistDataset(updatedData); err != nil {
		return err
	}

	s.data = updatedData

	return nil
}

// Profile operations

type ProfileUpdate struct {
	Bio               *string
	AvatarURL         *string
	BannerURL         *string
	FeaturedChannelID *string
	TopFriends        *[]string
	DonationAddresses *[]models.CryptoAddress
}

func (s *Storage) UpsertProfile(userID string, update ProfileUpdate) (models.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	if _, ok := updatedData.Users[userID]; !ok {
		return models.Profile{}, fmt.Errorf("user %s not found", userID)
	}

	profile, exists := updatedData.Profiles[userID]
	now := time.Now().UTC()
	if !exists {
		profile = models.Profile{
			UserID:            userID,
			TopFriends:        []string{},
			DonationAddresses: []models.CryptoAddress{},
			CreatedAt:         now,
		}
	}

	if update.Bio != nil {
		profile.Bio = strings.TrimSpace(*update.Bio)
	}
	if update.AvatarURL != nil {
		profile.AvatarURL = strings.TrimSpace(*update.AvatarURL)
	}
	if update.BannerURL != nil {
		profile.BannerURL = strings.TrimSpace(*update.BannerURL)
	}
	if update.FeaturedChannelID != nil {
		trimmed := strings.TrimSpace(*update.FeaturedChannelID)
		if trimmed == "" {
			profile.FeaturedChannelID = nil
		} else {
			channel, ok := updatedData.Channels[trimmed]
			if !ok {
				return models.Profile{}, fmt.Errorf("featured channel %s not found", trimmed)
			}
			if channel.OwnerID != userID {
				return models.Profile{}, errors.New("featured channel must belong to profile owner")
			}
			id := channel.ID
			profile.FeaturedChannelID = &id
		}
	}
	if update.TopFriends != nil {
		if len(*update.TopFriends) > 8 {
			return models.Profile{}, errors.New("top friends cannot exceed eight entries")
		}
		seen := make(map[string]struct{})
		ordered := make([]string, 0, len(*update.TopFriends))
		for _, friendID := range *update.TopFriends {
			trimmed := strings.TrimSpace(friendID)
			if trimmed == "" {
				return models.Profile{}, errors.New("top friends must reference valid users")
			}
			if trimmed == userID {
				return models.Profile{}, errors.New("cannot add profile owner as a top friend")
			}
			if _, friendExists := updatedData.Users[trimmed]; !friendExists {
				return models.Profile{}, fmt.Errorf("top friend %s not found", trimmed)
			}
			if _, duplicate := seen[trimmed]; duplicate {
				return models.Profile{}, errors.New("duplicate user in top friends list")
			}
			seen[trimmed] = struct{}{}
			ordered = append(ordered, trimmed)
		}
		profile.TopFriends = ordered
	}
	if update.DonationAddresses != nil {
		addresses := make([]models.CryptoAddress, 0, len(*update.DonationAddresses))
		for _, addr := range *update.DonationAddresses {
			currency := strings.ToUpper(strings.TrimSpace(addr.Currency))
			if currency == "" {
				return models.Profile{}, errors.New("donation currency is required")
			}
			address := strings.TrimSpace(addr.Address)
			if address == "" {
				return models.Profile{}, errors.New("donation address is required")
			}
			note := strings.TrimSpace(addr.Note)
			addresses = append(addresses, models.CryptoAddress{Currency: currency, Address: address, Note: note})
		}
		profile.DonationAddresses = addresses
	}

	profile.UpdatedAt = now
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}

	updatedData.Profiles[userID] = profile
	if err := s.persistDataset(updatedData); err != nil {
		return models.Profile{}, err
	}

	s.data = updatedData

	return profile, nil
}

func (s *Storage) GetProfile(userID string) (models.Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profile, ok := s.data.Profiles[userID]
	if !ok {
		user, userExists := s.data.Users[userID]
		if !userExists {
			return models.Profile{}, false
		}
		profile = models.Profile{
			UserID:            userID,
			TopFriends:        []string{},
			DonationAddresses: []models.CryptoAddress{},
			CreatedAt:         user.CreatedAt,
			UpdatedAt:         user.CreatedAt,
		}
		return profile, false
	}

	if profile.TopFriends == nil {
		profile.TopFriends = []string{}
	}
	if profile.DonationAddresses == nil {
		profile.DonationAddresses = []models.CryptoAddress{}
	}

	return profile, true
}

func (s *Storage) ListProfiles() []models.Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profiles := make([]models.Profile, 0, len(s.data.Profiles))
	for _, profile := range s.data.Profiles {
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].CreatedAt.Before(profiles[j].CreatedAt)
	})
	return profiles
}

// Channel operations

type ChannelUpdate struct {
	Title     *string
	Category  *string
	Tags      *[]string
	LiveState *string
}

func (s *Storage) CreateChannel(ownerID, title, category string, tags []string) (models.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Users[ownerID]; !ok {
		return models.Channel{}, fmt.Errorf("owner %s not found", ownerID)
	}
	if title = strings.TrimSpace(title); title == "" {
		return models.Channel{}, errors.New("title is required")
	}

	id, err := s.generateID()
	if err != nil {
		return models.Channel{}, err
	}
	streamKey, err := s.generateStreamKey()
	if err != nil {
		return models.Channel{}, err
	}

	now := time.Now().UTC()
	channel := models.Channel{
		ID:        id,
		OwnerID:   ownerID,
		StreamKey: streamKey,
		Title:     title,
		Category:  strings.TrimSpace(category),
		Tags:      normalizeTags(tags),
		LiveState: "offline",
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.data.Channels[id] = channel
	if err := s.persist(); err != nil {
		delete(s.data.Channels, id)
		return models.Channel{}, err
	}

	return channel, nil
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]struct{})
	for _, tag := range tags {
		trimmed := strings.TrimSpace(strings.ToLower(tag))
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	sort.Strings(normalized)
	return normalized
}

func (s *Storage) UpdateChannel(id string, update ChannelUpdate) (models.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	channel, ok := updatedData.Channels[id]
	if !ok {
		return models.Channel{}, fmt.Errorf("channel %s not found", id)
	}

	if update.Title != nil {
		if title := strings.TrimSpace(*update.Title); title != "" {
			channel.Title = title
		} else {
			return models.Channel{}, errors.New("title cannot be empty")
		}
	}
	if update.Category != nil {
		channel.Category = strings.TrimSpace(*update.Category)
	}
	if update.Tags != nil {
		channel.Tags = normalizeTags(*update.Tags)
	}
	if update.LiveState != nil {
		state := strings.ToLower(strings.TrimSpace(*update.LiveState))
		if state != "offline" && state != "live" && state != "starting" && state != "ended" {
			return models.Channel{}, fmt.Errorf("invalid liveState %s", state)
		}
		channel.LiveState = state
	}

	channel.UpdatedAt = time.Now().UTC()
	updatedData.Channels[id] = channel
	if err := s.persistDataset(updatedData); err != nil {
		return models.Channel{}, err
	}

	s.data = updatedData

	return channel, nil
}

func (s *Storage) GetChannel(id string) (models.Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	channel, ok := s.data.Channels[id]
	return channel, ok
}

func (s *Storage) ListChannels(ownerID string) []models.Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	channels := make([]models.Channel, 0, len(s.data.Channels))
	for _, channel := range s.data.Channels {
		if ownerID != "" && channel.OwnerID != ownerID {
			continue
		}
		channels = append(channels, channel)
	}
	sort.Slice(channels, func(i, j int) bool {
		if channels[i].LiveState == channels[j].LiveState {
			return channels[i].CreatedAt.Before(channels[j].CreatedAt)
		}
		return channels[i].LiveState == "live"
	})
	return channels
}

// FollowChannel records that a viewer is following the channel. The operation is idempotent.
func (s *Storage) FollowChannel(userID, channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	if _, ok := updatedData.Users[userID]; !ok {
		return fmt.Errorf("user %s not found", userID)
	}
	if _, ok := updatedData.Channels[channelID]; !ok {
		return fmt.Errorf("channel %s not found", channelID)
	}

	if updatedData.Follows == nil {
		updatedData.Follows = make(map[string]map[string]time.Time)
	}
	follows := updatedData.Follows[userID]
	if follows == nil {
		follows = make(map[string]time.Time)
	}
	if _, exists := follows[channelID]; !exists {
		follows[channelID] = time.Now().UTC()
	}
	updatedData.Follows[userID] = follows

	if err := s.persistDataset(updatedData); err != nil {
		return err
	}

	s.data = updatedData

	return nil
}

// UnfollowChannel removes the follow association if present. The operation is idempotent.
func (s *Storage) UnfollowChannel(userID, channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	if _, ok := updatedData.Users[userID]; !ok {
		return fmt.Errorf("user %s not found", userID)
	}
	if _, ok := updatedData.Channels[channelID]; !ok {
		return fmt.Errorf("channel %s not found", channelID)
	}

	if follows, ok := updatedData.Follows[userID]; ok {
		if _, exists := follows[channelID]; exists {
			delete(follows, channelID)
			if len(follows) == 0 {
				delete(updatedData.Follows, userID)
			} else {
				updatedData.Follows[userID] = follows
			}
		}
	}

	if err := s.persistDataset(updatedData); err != nil {
		return err
	}

	s.data = updatedData

	return nil
}

// IsFollowingChannel reports whether the given user follows the channel.
func (s *Storage) IsFollowingChannel(userID, channelID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	follows, ok := s.data.Follows[userID]
	if !ok {
		return false
	}
	_, exists := follows[channelID]
	return exists
}

// CountFollowers returns the number of viewers following the channel.
func (s *Storage) CountFollowers(channelID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, follows := range s.data.Follows {
		if follows == nil {
			continue
		}
		if _, ok := follows[channelID]; ok {
			count++
		}
	}
	return count
}

// ListFollowedChannelIDs returns the identifiers of channels the user follows ordered by recency.
func (s *Storage) ListFollowedChannelIDs(userID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	follows, ok := s.data.Follows[userID]
	if !ok || len(follows) == 0 {
		return nil
	}

	type pair struct {
		id   string
		when time.Time
	}

	pairs := make([]pair, 0, len(follows))
	for channelID, followedAt := range follows {
		pairs = append(pairs, pair{id: channelID, when: followedAt})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].when.After(pairs[j].when)
	})

	ids := make([]string, 0, len(pairs))
	for _, p := range pairs {
		ids = append(ids, p.id)
	}
	return ids
}

// DeleteChannel removes a channel and its associated sessions and chat transcripts.
func (s *Storage) DeleteChannel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	channel, ok := updatedData.Channels[id]
	if !ok {
		return fmt.Errorf("channel %s not found", id)
	}
	if channel.CurrentSessionID != nil {
		return errors.New("cannot delete a channel with an active stream")
	}

	delete(updatedData.Channels, id)

	for sessionID, session := range updatedData.StreamSessions {
		if session.ChannelID == id {
			delete(updatedData.StreamSessions, sessionID)
		}
	}
	for messageID, message := range updatedData.ChatMessages {
		if message.ChannelID == id {
			delete(updatedData.ChatMessages, messageID)
		}
	}
	for userID, follows := range updatedData.Follows {
		if follows == nil {
			continue
		}
		if _, exists := follows[id]; exists {
			delete(follows, id)
			if len(follows) == 0 {
				delete(updatedData.Follows, userID)
			} else {
				updatedData.Follows[userID] = follows
			}
		}
	}

	for profileID, profile := range updatedData.Profiles {
		if profile.FeaturedChannelID != nil && *profile.FeaturedChannelID == id {
			profile.FeaturedChannelID = nil
			updatedData.Profiles[profileID] = profile
		}
	}

	if err := s.persistDataset(updatedData); err != nil {
		return err
	}

	s.data = updatedData

	return nil
}

// Streaming operations

func (s *Storage) StartStream(channelID string, renditions []string) (models.StreamSession, error) {
	s.mu.Lock()
	channel, ok := s.data.Channels[channelID]
	if !ok {
		s.mu.Unlock()
		return models.StreamSession{}, fmt.Errorf("channel %s not found", channelID)
	}
	if channel.CurrentSessionID != nil {
		s.mu.Unlock()
		return models.StreamSession{}, errors.New("channel already live")
	}

	sessionID, err := s.generateID()
	if err != nil {
		s.mu.Unlock()
		return models.StreamSession{}, err
	}

	channel.CurrentSessionID = &sessionID
	channel.LiveState = "starting"
	s.data.Channels[channelID] = channel
	s.mu.Unlock()

	attempts := s.ingestMaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	ctx := context.Background()
	var boot ingest.BootResult
	var bootErr error
	for attempt := 0; attempt < attempts; attempt++ {
		boot, bootErr = s.ingestController.BootStream(ctx, ingest.BootParams{
			ChannelID:  channelID,
			SessionID:  sessionID,
			StreamKey:  channel.StreamKey,
			Renditions: append([]string{}, renditions...),
		})
		if bootErr == nil {
			break
		}
		if attempt < attempts-1 && s.ingestRetryInterval > 0 {
			time.Sleep(s.ingestRetryInterval)
		}
	}
	if bootErr != nil {
		s.mu.Lock()
		if updated, exists := s.data.Channels[channelID]; exists {
			updated.CurrentSessionID = nil
			updated.LiveState = "offline"
			s.data.Channels[channelID] = updated
		}
		s.mu.Unlock()
		return models.StreamSession{}, fmt.Errorf("boot ingest: %w", bootErr)
	}

	now := time.Now().UTC()
	session := models.StreamSession{
		ID:             sessionID,
		ChannelID:      channelID,
		StartedAt:      now,
		Renditions:     append([]string{}, renditions...),
		PeakConcurrent: 0,
		OriginURL:      boot.OriginURL,
		PlaybackURL:    boot.PlaybackURL,
		IngestJobIDs:   append([]string{}, boot.JobIDs...),
	}
	ingestEndpoints := make([]string, 0, 2)
	if boot.PrimaryIngest != "" {
		ingestEndpoints = append(ingestEndpoints, boot.PrimaryIngest)
	}
	if boot.BackupIngest != "" {
		ingestEndpoints = append(ingestEndpoints, boot.BackupIngest)
	}
	if len(ingestEndpoints) > 0 {
		session.IngestEndpoints = ingestEndpoints
	}
	if len(boot.Renditions) > 0 {
		manifests := make([]models.RenditionManifest, 0, len(boot.Renditions))
		for _, rendition := range boot.Renditions {
			manifests = append(manifests, models.RenditionManifest{
				Name:        rendition.Name,
				ManifestURL: rendition.ManifestURL,
				Bitrate:     rendition.Bitrate,
			})
		}
		session.RenditionManifests = manifests
	}

	s.mu.Lock()
	s.data.StreamSessions[sessionID] = session
	channel = s.data.Channels[channelID]
	channel.CurrentSessionID = &sessionID
	channel.LiveState = "live"
	channel.UpdatedAt = now
	s.data.Channels[channelID] = channel

	if err := s.persist(); err != nil {
		delete(s.data.StreamSessions, sessionID)
		channel.CurrentSessionID = nil
		channel.LiveState = "offline"
		s.data.Channels[channelID] = channel
		jobIDs := append([]string{}, session.IngestJobIDs...)
		s.mu.Unlock()
		_ = s.ingestController.ShutdownStream(context.Background(), channelID, sessionID, jobIDs)
		return models.StreamSession{}, err
	}
	s.mu.Unlock()

	return session, nil
}

func (s *Storage) StopStream(channelID string, peakConcurrent int) (models.StreamSession, error) {
	s.mu.Lock()
	channel, ok := s.data.Channels[channelID]
	if !ok {
		s.mu.Unlock()
		return models.StreamSession{}, fmt.Errorf("channel %s not found", channelID)
	}
	if channel.CurrentSessionID == nil {
		s.mu.Unlock()
		return models.StreamSession{}, errors.New("channel is not live")
	}

	sessionID := *channel.CurrentSessionID
	session, ok := s.data.StreamSessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return models.StreamSession{}, fmt.Errorf("session %s missing", sessionID)
	}

	originalChannel := channel
	originalSession := session
	jobIDs := append([]string{}, session.IngestJobIDs...)
	s.mu.Unlock()

	if err := s.ingestController.ShutdownStream(context.Background(), channelID, sessionID, jobIDs); err != nil {
		return models.StreamSession{}, fmt.Errorf("shutdown ingest: %w", err)
	}

	now := time.Now().UTC()
	session.EndedAt = &now
	if peakConcurrent > session.PeakConcurrent {
		session.PeakConcurrent = peakConcurrent
	}

	s.mu.Lock()
	channel, ok = s.data.Channels[channelID]
	if !ok {
		s.mu.Unlock()
		return models.StreamSession{}, fmt.Errorf("channel %s not found", channelID)
	}
	s.data.StreamSessions[sessionID] = session
	channel.CurrentSessionID = nil
	channel.LiveState = "offline"
	channel.UpdatedAt = now
	s.data.Channels[channelID] = channel

	recording, recErr := s.createRecordingLocked(session, channel, now)
	if recErr != nil {
		s.data.StreamSessions[sessionID] = originalSession
		s.data.Channels[channelID] = originalChannel
		s.mu.Unlock()
		return models.StreamSession{}, recErr
	}
	if recording.ID != "" {
		s.data.Recordings[recording.ID] = recording
	}

	if err := s.persist(); err != nil {
		s.data.StreamSessions[sessionID] = originalSession
		s.data.Channels[channelID] = originalChannel
		if recording.ID != "" {
			delete(s.data.Recordings, recording.ID)
		}
		s.mu.Unlock()
		return models.StreamSession{}, err
	}
	s.mu.Unlock()

	return session, nil
}

func (s *Storage) createRecordingLocked(session models.StreamSession, channel models.Channel, ended time.Time) (models.Recording, error) {
	s.ensureDatasetInitializedLocked()
	id, err := s.generateID()
	if err != nil {
		return models.Recording{}, err
	}
	duration := int(ended.Sub(session.StartedAt).Round(time.Second).Seconds())
	if duration < 0 {
		duration = 0
	}
	title := channel.Title
	if title == "" {
		title = fmt.Sprintf("Recording %s", session.ID)
	}
	title = strings.TrimSpace(title)
	metadata := map[string]string{
		"channelId":  channel.ID,
		"sessionId":  session.ID,
		"startedAt":  session.StartedAt.UTC().Format(time.RFC3339Nano),
		"endedAt":    ended.UTC().Format(time.RFC3339Nano),
		"renditions": strconv.Itoa(len(session.RenditionManifests)),
	}
	if session.PeakConcurrent > 0 {
		metadata["peakConcurrent"] = strconv.Itoa(session.PeakConcurrent)
	}
	recording := models.Recording{
		ID:              id,
		ChannelID:       channel.ID,
		SessionID:       session.ID,
		Title:           title,
		DurationSeconds: duration,
		PlaybackBaseURL: session.PlaybackURL,
		Metadata:        metadata,
		CreatedAt:       ended,
	}
	if deadline := s.recordingDeadline(ended, false); deadline != nil {
		recording.RetainUntil = deadline
	}
	if len(session.RenditionManifests) > 0 {
		renditions := make([]models.RecordingRendition, 0, len(session.RenditionManifests))
		for _, manifest := range session.RenditionManifests {
			renditions = append(renditions, models.RecordingRendition{
				Name:        manifest.Name,
				ManifestURL: manifest.ManifestURL,
				Bitrate:     manifest.Bitrate,
			})
		}
		recording.Renditions = renditions
	}
	if err := s.populateRecordingArtifactsLocked(&recording, session); err != nil {
		return models.Recording{}, err
	}
	return recording, nil
}

func (s *Storage) populateRecordingArtifactsLocked(recording *models.Recording, session models.StreamSession) error {
	client := s.objectClient
	if client == nil || !client.Enabled() {
		return nil
	}
	if recording.Metadata == nil {
		recording.Metadata = make(map[string]string)
	}

	ctx := context.Background()
	createdAt := recording.CreatedAt.UTC().Format(time.RFC3339Nano)
	if len(session.RenditionManifests) > 0 {
		for idx, manifest := range session.RenditionManifests {
			key := buildObjectKey("recordings", recording.ID, "manifests", normalizeObjectComponent(manifest.Name)+".json")
			payload := map[string]any{
				"recordingId": recording.ID,
				"sessionId":   recording.SessionID,
				"name":        manifest.Name,
				"source":      manifest.ManifestURL,
				"createdAt":   createdAt,
			}
			if manifest.Bitrate > 0 {
				payload["bitrate"] = manifest.Bitrate
			}
			data, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("encode manifest payload: %w", err)
			}
			ref, err := client.Upload(ctx, key, "application/json", data)
			if err != nil {
				return fmt.Errorf("upload manifest %s: %w", manifest.Name, err)
			}
			if ref.Key != "" {
				recording.Metadata[manifestMetadataKey(manifest.Name)] = ref.Key
			}
			if ref.URL != "" && idx < len(recording.Renditions) {
				recording.Renditions[idx].ManifestURL = ref.URL
			}
		}
	}

	thumbID, err := s.generateID()
	if err != nil {
		return fmt.Errorf("generate thumbnail id: %w", err)
	}
	thumbKey := buildObjectKey("recordings", recording.ID, "thumbnails", thumbID+".json")
	thumbPayload := map[string]any{
		"recordingId": recording.ID,
		"sessionId":   recording.SessionID,
		"createdAt":   createdAt,
	}
	thumbData, err := json.Marshal(thumbPayload)
	if err != nil {
		return fmt.Errorf("encode thumbnail payload: %w", err)
	}
	ref, err := client.Upload(ctx, thumbKey, "application/json", thumbData)
	if err != nil {
		return fmt.Errorf("upload thumbnail: %w", err)
	}
	if ref.Key != "" {
		recording.Metadata[thumbnailMetadataKey(thumbID)] = ref.Key
	}
	thumbnail := models.RecordingThumbnail{
		ID:          thumbID,
		RecordingID: recording.ID,
		URL:         ref.URL,
		CreatedAt:   recording.CreatedAt,
	}
	recording.Thumbnails = append(recording.Thumbnails, thumbnail)
	return nil
}

func (s *Storage) deleteRecordingArtifactsLocked(recording models.Recording) error {
	client := s.objectClient
	if client == nil || !client.Enabled() {
		return nil
	}
	if len(recording.Metadata) == 0 {
		return nil
	}
	ctx := context.Background()
	deleted := make(map[string]struct{})
	for key, objectKey := range recording.Metadata {
		if !strings.HasPrefix(key, metadataManifestPrefix) && !strings.HasPrefix(key, metadataThumbnailPrefix) {
			continue
		}
		trimmed := strings.TrimSpace(objectKey)
		if trimmed == "" {
			continue
		}
		if _, exists := deleted[trimmed]; exists {
			continue
		}
		if err := client.Delete(ctx, trimmed); err != nil {
			return fmt.Errorf("delete object %s: %w", trimmed, err)
		}
		deleted[trimmed] = struct{}{}
	}
	return nil
}

func (s *Storage) deleteClipArtifactsLocked(clip models.ClipExport) error {
	client := s.objectClient
	if client == nil || !client.Enabled() || strings.TrimSpace(clip.StorageObject) == "" {
		return nil
	}
	if err := client.Delete(context.Background(), clip.StorageObject); err != nil {
		return fmt.Errorf("delete clip object %s: %w", clip.StorageObject, err)
	}
	return nil
}

func (s *Storage) ListStreamSessions(channelID string) ([]models.StreamSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	sessions := make([]models.StreamSession, 0)
	for _, session := range s.data.StreamSessions {
		if session.ChannelID == channelID {
			sessions = append(sessions, session)
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartedAt.After(sessions[j].StartedAt)
	})
	return sessions, nil
}

func (s *Storage) ListRecordings(channelID string, includeUnpublished bool) ([]models.Recording, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	now := time.Now().UTC()
	removed, snapshot, err := s.purgeExpiredRecordingsLocked(now)
	if err != nil {
		return nil, err
	}
	if removed {
		if err := s.persist(); err != nil {
			s.data = snapshot
			return nil, err
		}
	}

	recordings := make([]models.Recording, 0)
	for _, recording := range s.data.Recordings {
		if recording.ChannelID != channelID {
			continue
		}
		if !includeUnpublished && recording.PublishedAt == nil {
			continue
		}
		recordings = append(recordings, s.recordingWithClipsLocked(recording))
	}
	sort.Slice(recordings, func(i, j int) bool {
		return recordings[i].CreatedAt.After(recordings[j].CreatedAt)
	})
	return recordings, nil
}

func (s *Storage) GetRecording(id string) (models.Recording, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		return models.Recording{}, false
	}
	now := time.Now().UTC()
	removed, snapshot, err := s.purgeExpiredRecordingsLocked(now)
	if err != nil {
		return models.Recording{}, false
	}
	if removed {
		if err := s.persist(); err != nil {
			s.data = snapshot
			return models.Recording{}, false
		}
	}
	recording, ok := s.data.Recordings[id]
	if !ok {
		return models.Recording{}, false
	}
	return s.recordingWithClipsLocked(recording), true
}

func (s *Storage) PublishRecording(id string) (models.Recording, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		return models.Recording{}, fmt.Errorf("recording id is required")
	}

	recording, ok := s.data.Recordings[id]
	if !ok {
		return models.Recording{}, fmt.Errorf("recording %s not found", id)
	}
	if recording.PublishedAt != nil {
		return s.recordingWithClipsLocked(recording), nil
	}

	now := time.Now().UTC()
	updated := cloneRecording(recording)
	updated.PublishedAt = &now
	if deadline := s.recordingDeadline(now, true); deadline != nil {
		updated.RetainUntil = deadline
	} else {
		updated.RetainUntil = nil
	}

	snapshot := cloneDataset(s.data)
	s.data.Recordings[id] = updated
	if err := s.persist(); err != nil {
		s.data = snapshot
		return models.Recording{}, err
	}
	return s.recordingWithClipsLocked(updated), nil
}

func (s *Storage) DeleteRecording(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		return fmt.Errorf("recording id is required")
	}
	recording, ok := s.data.Recordings[id]
	if !ok {
		return fmt.Errorf("recording %s not found", id)
	}
	if err := s.deleteRecordingArtifactsLocked(recording); err != nil {
		return err
	}
	snapshot := cloneDataset(s.data)
	for clipID, clip := range s.data.ClipExports {
		if clip.RecordingID != id {
			continue
		}
		if err := s.deleteClipArtifactsLocked(clip); err != nil {
			s.data = snapshot
			return err
		}
		delete(s.data.ClipExports, clipID)
	}
	delete(s.data.Recordings, id)
	if err := s.persist(); err != nil {
		s.data = snapshot
		return err
	}
	return nil
}

func (s *Storage) CreateClipExport(recordingID string, params ClipExportParams) (models.ClipExport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if recordingID == "" {
		return models.ClipExport{}, fmt.Errorf("recording id is required")
	}
	recording, ok := s.data.Recordings[recordingID]
	if !ok {
		return models.ClipExport{}, fmt.Errorf("recording %s not found", recordingID)
	}
	if params.EndSeconds <= params.StartSeconds {
		return models.ClipExport{}, fmt.Errorf("endSeconds must be greater than startSeconds")
	}
	if params.StartSeconds < 0 {
		return models.ClipExport{}, fmt.Errorf("startSeconds must be non-negative")
	}
	if recording.DurationSeconds > 0 && params.EndSeconds > recording.DurationSeconds {
		return models.ClipExport{}, fmt.Errorf("clip exceeds recording duration")
	}
	id, err := s.generateID()
	if err != nil {
		return models.ClipExport{}, err
	}
	now := time.Now().UTC()
	clip := models.ClipExport{
		ID:           id,
		RecordingID:  recordingID,
		ChannelID:    recording.ChannelID,
		SessionID:    recording.SessionID,
		Title:        strings.TrimSpace(params.Title),
		StartSeconds: params.StartSeconds,
		EndSeconds:   params.EndSeconds,
		Status:       "pending",
		CreatedAt:    now,
	}
	snapshot := cloneDataset(s.data)
	s.data.ClipExports[id] = clip
	if err := s.persist(); err != nil {
		s.data = snapshot
		return models.ClipExport{}, err
	}
	return clip, nil
}

func (s *Storage) ListClipExports(recordingID string) ([]models.ClipExport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if recordingID == "" {
		return nil, fmt.Errorf("recording id is required")
	}
	if _, ok := s.data.Recordings[recordingID]; !ok {
		return nil, fmt.Errorf("recording %s not found", recordingID)
	}
	clips := make([]models.ClipExport, 0)
	for _, clip := range s.data.ClipExports {
		if clip.RecordingID != recordingID {
			continue
		}
		clips = append(clips, cloneClipExport(clip))
	}
	sort.Slice(clips, func(i, j int) bool {
		return clips[i].CreatedAt.After(clips[j].CreatedAt)
	})
	return clips, nil
}

// CurrentStreamSession returns the active stream session for the channel if present.
func (s *Storage) CurrentStreamSession(channelID string) (models.StreamSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	channel, ok := s.data.Channels[channelID]
	if !ok || channel.CurrentSessionID == nil {
		return models.StreamSession{}, false
	}
	session, exists := s.data.StreamSessions[*channel.CurrentSessionID]
	if !exists {
		return models.StreamSession{}, false
	}
	return session, true
}

// IngestHealth reports the status of configured ingest dependencies.
func (s *Storage) IngestHealth(ctx context.Context) []ingest.HealthStatus {
	controller := s.ingestController
	if controller == nil {
		status := []ingest.HealthStatus{{Component: "ingest", Status: "disabled"}}
		s.recordIngestHealth(status)
		return status
	}
	checks := controller.HealthChecks(ctx)
	if len(checks) == 0 {
		checks = []ingest.HealthStatus{{Component: "ingest", Status: "unknown"}}
	}
	s.recordIngestHealth(checks)
	return checks
}

func (s *Storage) recordIngestHealth(statuses []ingest.HealthStatus) {
	snapshot := append([]ingest.HealthStatus(nil), statuses...)
	s.mu.Lock()
	s.ingestHealth = snapshot
	s.ingestHealthUpdated = time.Now().UTC()
	s.mu.Unlock()
}

// LastIngestHealth returns the most recently recorded ingest health snapshot.
func (s *Storage) LastIngestHealth() ([]ingest.HealthStatus, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.ingestHealth) == 0 {
		return nil, time.Time{}
	}
	snapshot := append([]ingest.HealthStatus(nil), s.ingestHealth...)
	return snapshot, s.ingestHealthUpdated
}

// Chat operations

func (s *Storage) CreateChatMessage(channelID, userID, content string) (models.ChatMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return models.ChatMessage{}, fmt.Errorf("channel %s not found", channelID)
	}
	if _, ok := s.data.Users[userID]; !ok {
		return models.ChatMessage{}, fmt.Errorf("user %s not found", userID)
	}

	if err := s.ensureChatAccessLocked(channelID, userID); err != nil {
		return models.ChatMessage{}, err
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return models.ChatMessage{}, errors.New("message content cannot be empty")
	}
	if len([]rune(trimmed)) > 500 {
		return models.ChatMessage{}, errors.New("message content exceeds 500 characters")
	}

	id, err := s.generateID()
	if err != nil {
		return models.ChatMessage{}, err
	}

	message := models.ChatMessage{
		ID:        id,
		ChannelID: channelID,
		UserID:    userID,
		Content:   trimmed,
		CreatedAt: time.Now().UTC(),
	}

	s.data.ChatMessages[id] = message
	if err := s.persist(); err != nil {
		delete(s.data.ChatMessages, id)
		return models.ChatMessage{}, err
	}

	return message, nil
}

func (s *Storage) ensureChatAccessLocked(channelID, userID string) error {
	if s.isChatBannedLocked(channelID, userID) {
		return fmt.Errorf("user is banned")
	}
	if expiry, ok := s.chatTimeoutLocked(channelID, userID); ok {
		if time.Now().UTC().Before(expiry) {
			return fmt.Errorf("user is timed out")
		}
		delete(s.data.ChatTimeouts[channelID], userID)
	}
	return nil
}

func (s *Storage) isChatBannedLocked(channelID, userID string) bool {
	if bans := s.data.ChatBans[channelID]; bans != nil {
		if _, exists := bans[userID]; exists {
			return true
		}
	}
	return false
}

func (s *Storage) chatTimeoutLocked(channelID, userID string) (time.Time, bool) {
	if timeouts := s.data.ChatTimeouts[channelID]; timeouts != nil {
		expiry, ok := timeouts[userID]
		if ok {
			return expiry, true
		}
	}
	return time.Time{}, false
}

func (s *Storage) ListChatMessages(channelID string, limit int) ([]models.ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	messages := make([]models.ChatMessage, 0)
	for _, message := range s.data.ChatMessages {
		if message.ChannelID == channelID {
			messages = append(messages, message)
		}
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].CreatedAt.After(messages[j].CreatedAt)
	})

	if limit > 0 && len(messages) > limit {
		messages = messages[:limit]
	}
	return messages, nil
}

// DeleteChatMessage removes a single chat message from the transcript.
func (s *Storage) DeleteChatMessage(channelID, messageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return fmt.Errorf("channel %s not found", channelID)
	}

	message, ok := s.data.ChatMessages[messageID]
	if !ok || message.ChannelID != channelID {
		return fmt.Errorf("message %s not found for channel %s", messageID, channelID)
	}

	delete(s.data.ChatMessages, messageID)
	if err := s.persist(); err != nil {
		return err
	}
	return nil
}

// ListChatRestrictions returns the current bans and timeouts for a channel.
func (s *Storage) ListChatRestrictions(channelID string) []models.ChatRestriction {
	s.mu.RLock()
	defer s.mu.RUnlock()

	restrictions := make([]models.ChatRestriction, 0)
	if bans := s.data.ChatBans[channelID]; bans != nil {
		for userID, issued := range bans {
			restriction := models.ChatRestriction{
				ID:        fmt.Sprintf("ban:%s:%s", channelID, userID),
				Type:      "ban",
				ChannelID: channelID,
				TargetID:  userID,
				IssuedAt:  issued,
				ActorID:   s.lookupBanActor(channelID, userID),
				Reason:    s.lookupBanReason(channelID, userID),
			}
			restrictions = append(restrictions, restriction)
		}
	}
	if timeouts := s.data.ChatTimeouts[channelID]; timeouts != nil {
		for userID, expiry := range timeouts {
			issued := s.lookupTimeoutIssuedAt(channelID, userID, expiry)
			restriction := models.ChatRestriction{
				ID:        fmt.Sprintf("timeout:%s:%s", channelID, userID),
				Type:      "timeout",
				ChannelID: channelID,
				TargetID:  userID,
				IssuedAt:  issued,
				ExpiresAt: &expiry,
				ActorID:   s.lookupTimeoutActor(channelID, userID),
				Reason:    s.lookupTimeoutReason(channelID, userID),
			}
			restrictions = append(restrictions, restriction)
		}
	}
	sort.Slice(restrictions, func(i, j int) bool {
		if restrictions[i].IssuedAt.Equal(restrictions[j].IssuedAt) {
			return restrictions[i].ID < restrictions[j].ID
		}
		return restrictions[i].IssuedAt.After(restrictions[j].IssuedAt)
	})
	return restrictions
}

func (s *Storage) lookupBanActor(channelID, userID string) string {
	if actors := s.data.ChatBanActors[channelID]; actors != nil {
		return actors[userID]
	}
	return ""
}

func (s *Storage) lookupBanReason(channelID, userID string) string {
	if reasons := s.data.ChatBanReasons[channelID]; reasons != nil {
		return reasons[userID]
	}
	return ""
}

func (s *Storage) lookupTimeoutActor(channelID, userID string) string {
	if actors := s.data.ChatTimeoutActors[channelID]; actors != nil {
		return actors[userID]
	}
	return ""
}

func (s *Storage) lookupTimeoutReason(channelID, userID string) string {
	if reasons := s.data.ChatTimeoutReasons[channelID]; reasons != nil {
		return reasons[userID]
	}
	return ""
}

func (s *Storage) lookupTimeoutIssuedAt(channelID, userID string, fallback time.Time) time.Time {
	if issued := s.data.ChatTimeoutIssuedAt[channelID]; issued != nil {
		if ts, ok := issued[userID]; ok {
			return ts
		}
	}
	return fallback
}

// CreateChatReport persists a moderation report filed by a viewer.
func (s *Storage) CreateChatReport(channelID, reporterID, targetID, reason, messageID, evidenceURL string) (models.ChatReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return models.ChatReport{}, fmt.Errorf("channel %s not found", channelID)
	}
	if _, ok := s.data.Users[reporterID]; !ok {
		return models.ChatReport{}, fmt.Errorf("reporter %s not found", reporterID)
	}
	if _, ok := s.data.Users[targetID]; !ok {
		return models.ChatReport{}, fmt.Errorf("target %s not found", targetID)
	}
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return models.ChatReport{}, fmt.Errorf("reason is required")
	}
	id, err := s.generateID()
	if err != nil {
		return models.ChatReport{}, err
	}
	now := time.Now().UTC()
	report := models.ChatReport{
		ID:          id,
		ChannelID:   channelID,
		ReporterID:  reporterID,
		TargetID:    targetID,
		Reason:      trimmedReason,
		MessageID:   strings.TrimSpace(messageID),
		EvidenceURL: strings.TrimSpace(evidenceURL),
		Status:      "open",
		CreatedAt:   now,
	}
	if s.data.ChatReports == nil {
		s.data.ChatReports = make(map[string]models.ChatReport)
	}
	s.data.ChatReports[id] = report
	if err := s.persist(); err != nil {
		delete(s.data.ChatReports, id)
		return models.ChatReport{}, err
	}
	return report, nil
}

// ListChatReports lists reports for a channel.
func (s *Storage) ListChatReports(channelID string, includeResolved bool) ([]models.ChatReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	reports := make([]models.ChatReport, 0)
	for _, report := range s.data.ChatReports {
		if report.ChannelID != channelID {
			continue
		}
		if !includeResolved && strings.EqualFold(report.Status, "resolved") {
			continue
		}
		reports = append(reports, report)
	}
	sort.Slice(reports, func(i, j int) bool {
		if reports[i].CreatedAt.Equal(reports[j].CreatedAt) {
			return reports[i].ID < reports[j].ID
		}
		return reports[i].CreatedAt.After(reports[j].CreatedAt)
	})
	return reports, nil
}

// ResolveChatReport marks a report as addressed.
func (s *Storage) ResolveChatReport(reportID, resolverID, resolution string) (models.ChatReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	report, ok := s.data.ChatReports[reportID]
	if !ok {
		return models.ChatReport{}, fmt.Errorf("report %s not found", reportID)
	}
	if _, ok := s.data.Users[resolverID]; !ok {
		return models.ChatReport{}, fmt.Errorf("resolver %s not found", resolverID)
	}
	if strings.EqualFold(report.Status, "resolved") {
		return report, nil
	}
	now := time.Now().UTC()
	trimmed := strings.TrimSpace(resolution)
	if trimmed == "" {
		trimmed = "resolved"
	}
	report.Status = "resolved"
	report.Resolution = trimmed
	report.ResolverID = resolverID
	report.ResolvedAt = &now
	s.data.ChatReports[reportID] = report
	if err := s.persist(); err != nil {
		return models.ChatReport{}, err
	}
	return report, nil
}

// CreateTip records a tip event for a channel.
func (s *Storage) CreateTip(params CreateTipParams) (models.Tip, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[params.ChannelID]; !ok {
		return models.Tip{}, fmt.Errorf("channel %s not found", params.ChannelID)
	}
	if _, ok := s.data.Users[params.FromUserID]; !ok {
		return models.Tip{}, fmt.Errorf("user %s not found", params.FromUserID)
	}
	amount := params.Amount
	if amount <= 0 {
		return models.Tip{}, fmt.Errorf("amount must be positive")
	}
	currency := strings.ToUpper(strings.TrimSpace(params.Currency))
	if currency == "" {
		return models.Tip{}, fmt.Errorf("currency is required")
	}
	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	if provider == "" {
		return models.Tip{}, fmt.Errorf("provider is required")
	}
	reference := strings.TrimSpace(params.Reference)
	if reference == "" {
		reference = fmt.Sprintf("tip-%d", time.Now().UnixNano())
	}
	id, err := s.generateID()
	if err != nil {
		return models.Tip{}, err
	}
	now := time.Now().UTC()
	tip := models.Tip{
		ID:            id,
		ChannelID:     params.ChannelID,
		FromUserID:    params.FromUserID,
		Amount:        amount,
		Currency:      currency,
		Provider:      provider,
		Reference:     reference,
		WalletAddress: strings.TrimSpace(params.WalletAddress),
		Message:       strings.TrimSpace(params.Message),
		CreatedAt:     now,
	}
	if s.data.Tips == nil {
		s.data.Tips = make(map[string]models.Tip)
	}
	s.data.Tips[id] = tip
	if err := s.persist(); err != nil {
		delete(s.data.Tips, id)
		return models.Tip{}, err
	}
	return tip, nil
}

// ListTips returns recent tips for a channel.
func (s *Storage) ListTips(channelID string, limit int) ([]models.Tip, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	tips := make([]models.Tip, 0)
	for _, tip := range s.data.Tips {
		if tip.ChannelID == channelID {
			tips = append(tips, tip)
		}
	}
	sort.Slice(tips, func(i, j int) bool {
		return tips[i].CreatedAt.After(tips[j].CreatedAt)
	})
	if limit > 0 && len(tips) > limit {
		tips = tips[:limit]
	}
	return tips, nil
}

// CreateSubscription records a new channel subscription.
func (s *Storage) CreateSubscription(params CreateSubscriptionParams) (models.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[params.ChannelID]; !ok {
		return models.Subscription{}, fmt.Errorf("channel %s not found", params.ChannelID)
	}
	if _, ok := s.data.Users[params.UserID]; !ok {
		return models.Subscription{}, fmt.Errorf("user %s not found", params.UserID)
	}
	if params.Duration <= 0 {
		return models.Subscription{}, fmt.Errorf("duration must be positive")
	}
	amount := params.Amount
	if amount < 0 {
		return models.Subscription{}, fmt.Errorf("amount cannot be negative")
	}
	currency := strings.ToUpper(strings.TrimSpace(params.Currency))
	if currency == "" {
		return models.Subscription{}, fmt.Errorf("currency is required")
	}
	tier := strings.TrimSpace(params.Tier)
	if tier == "" {
		tier = "supporter"
	}
	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	if provider == "" {
		return models.Subscription{}, fmt.Errorf("provider is required")
	}
	reference := strings.TrimSpace(params.Reference)
	if reference == "" {
		reference = fmt.Sprintf("sub-%d", time.Now().UnixNano())
	}
	id, err := s.generateID()
	if err != nil {
		return models.Subscription{}, err
	}
	started := time.Now().UTC()
	expires := started.Add(params.Duration)
	subscription := models.Subscription{
		ID:                id,
		ChannelID:         params.ChannelID,
		UserID:            params.UserID,
		Tier:              tier,
		Provider:          provider,
		Reference:         reference,
		Amount:            amount,
		Currency:          currency,
		StartedAt:         started,
		ExpiresAt:         expires,
		AutoRenew:         params.AutoRenew,
		Status:            "active",
		ExternalReference: strings.TrimSpace(params.ExternalReference),
	}
	if s.data.Subscriptions == nil {
		s.data.Subscriptions = make(map[string]models.Subscription)
	}
	s.data.Subscriptions[id] = subscription
	if err := s.persist(); err != nil {
		delete(s.data.Subscriptions, id)
		return models.Subscription{}, err
	}
	return subscription, nil
}

// ListSubscriptions lists subscriptions for a channel.
func (s *Storage) ListSubscriptions(channelID string, includeInactive bool) ([]models.Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	subs := make([]models.Subscription, 0)
	for _, sub := range s.data.Subscriptions {
		if sub.ChannelID != channelID {
			continue
		}
		if !includeInactive && !strings.EqualFold(sub.Status, "active") {
			continue
		}
		subs = append(subs, sub)
	}
	sort.Slice(subs, func(i, j int) bool {
		if subs[i].StartedAt.Equal(subs[j].StartedAt) {
			return subs[i].ID < subs[j].ID
		}
		return subs[i].StartedAt.After(subs[j].StartedAt)
	})
	return subs, nil
}

// GetSubscription returns a subscription by id.
func (s *Storage) GetSubscription(id string) (models.Subscription, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.data.Subscriptions[id]
	return sub, ok
}

// CancelSubscription marks a subscription as cancelled.
func (s *Storage) CancelSubscription(id, cancelledBy, reason string) (models.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subscription, ok := s.data.Subscriptions[id]
	if !ok {
		return models.Subscription{}, fmt.Errorf("subscription %s not found", id)
	}
	if subscription.Status == "cancelled" {
		return subscription, nil
	}
	if _, ok := s.data.Users[cancelledBy]; !ok {
		return models.Subscription{}, fmt.Errorf("user %s not found", cancelledBy)
	}
	now := time.Now().UTC()
	subscription.Status = "cancelled"
	subscription.AutoRenew = false
	subscription.CancelledBy = cancelledBy
	subscription.CancelledAt = &now
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		if cancelledBy == subscription.UserID {
			trimmed = "user_cancelled"
		} else {
			trimmed = "cancelled_by_admin"
		}
	}
	subscription.CancelledReason = trimmed
	s.data.Subscriptions[id] = subscription
	if err := s.persist(); err != nil {
		return models.Subscription{}, err
	}
	return subscription, nil
}
