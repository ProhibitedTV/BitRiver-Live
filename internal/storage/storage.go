package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/ingest"
)

// Ping always reports success for the JSON-backed repository.
func (s *Storage) Ping(context.Context) error {
	return nil
}

// NewStorage initializes the JSON-backed store with defaults, options, and persisted data.
func NewStorage(path string, opts ...Option) (*Storage, error) {
	store := &Storage{
		filePath:            path,
		ingestController:    ingest.NoopController{},
		ingestMaxAttempts:   1,
		ingestTimeout:       defaultIngestOperationTimeout,
		ingestHealth:        []ingest.HealthStatus{{Component: "ingest", Status: "disabled"}},
		ingestHealthUpdated: time.Now().UTC(),
		recordingRetention: RecordingRetentionPolicy{
			Published:   90 * 24 * time.Hour,
			Unpublished: 14 * 24 * time.Hour,
		},
		chatRetention: ChatRetentionPolicy{
			Messages:       0,
			ModerationLogs: 0,
		},
		objectClient: noopObjectStorageClient{},
		retentionNow: func() time.Time { return time.Now().UTC() },
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
	store.ingestTimeout = normalizeIngestTimeout(store.ingestTimeout)
	if err := store.load(); err != nil {
		return nil, err
	}
	store.objectStorage = applyObjectStorageDefaults(store.objectStorage)
	store.objectClient = newObjectStorageClient(store.objectStorage)
	return store, nil
}

// load reads the on-disk dataset into memory, creating an empty dataset when missing or empty.
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
	defer func() {
		_ = file.Close()
	}()

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

// persist writes the current in-memory dataset to durable storage.
func (s *Storage) persist() error {
	return s.persistDataset(s.data)
}

// persistDataset atomically writes a dataset snapshot to disk.
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

// User operations

func (s *Storage) CreateUser(params CreateUserParams) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedEmail := strings.TrimSpace(strings.ToLower(params.Email))
	if normalizedEmail == "" {
		return domain.User{}, errors.New("email is required")
	}
	for _, user := range s.data.Users {
		if user.Email == normalizedEmail {
			return domain.User{}, fmt.Errorf("email %s already in use", params.Email)
		}
	}

	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		return domain.User{}, errors.New("displayName is required")
	}

	roles := normalizeRoles(params.Roles)
	if params.SelfSignup {
		if params.Password == "" {
			return domain.User{}, errors.New("password is required for self-service signup")
		}
		if len(roles) == 0 {
			roles = []string{"viewer"}
		}
	}

	id, err := generateID()
	if err != nil {
		return domain.User{}, err
	}

	var passwordHash string
	if params.Password != "" {
		hashed, hashErr := hashPassword(params.Password)
		if hashErr != nil {
			return domain.User{}, fmt.Errorf("hash password: %w", hashErr)
		}
		passwordHash = hashed
	}

	now := time.Now().UTC()
	user := domain.User{
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
		return domain.User{}, err
	}

	return user, nil
}

// ListUsers returns users ordered by creation time.
func (s *Storage) ListUsers() []domain.User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]domain.User, 0, len(s.data.Users))
	for _, user := range s.data.Users {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].CreatedAt.Before(users[j].CreatedAt)
	})
	return users
}

// GetUser executes GetUser.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this signature does not return `error`; not-found/absence is represented by the
// boolean return value.
// Transactions/connections: no external transaction is required; it coordinates access with
// the in-memory mutex and persists snapshots to disk/object storage as needed.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (s *Storage) GetUser(id string) (domain.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.data.Users[id]
	return user, ok
}

// FindUserByEmail looks up a user by their normalized email address.
func (s *Storage) FindUserByEmail(email string) (domain.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	normalizedEmail := strings.TrimSpace(strings.ToLower(email))
	for _, user := range s.data.Users {
		if user.Email == normalizedEmail {
			return user, true
		}
	}
	return domain.User{}, false
}

// AuthenticateOAuth executes AuthenticateOAuth.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: no external transaction is required; it coordinates access with
// the in-memory mutex and persists snapshots to disk/object storage as needed.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (s *Storage) AuthenticateOAuth(params OAuthLoginParams) (domain.User, error) {
	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	subject := strings.TrimSpace(params.Subject)
	if provider == "" {
		return domain.User{}, errors.New("provider is required")
	}
	if subject == "" {
		return domain.User{}, errors.New("subject is required")
	}

	normalizedEmail := strings.TrimSpace(strings.ToLower(params.Email))
	if normalizedEmail == "" {
		normalizedEmail = fallbackOAuthEmail(provider, subject)
	}

	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		displayName = defaultOAuthDisplayName(provider, normalizedEmail, subject)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDatasetInitializedLocked()

	if s.data.OAuthAccounts == nil {
		s.data.OAuthAccounts = make(map[string]domain.OAuthAccount)
	}

	key := oauthAccountKey(provider, subject)
	if account, ok := s.data.OAuthAccounts[key]; ok {
		if user, ok := s.data.Users[account.UserID]; ok {
			return user, nil
		}
		delete(s.data.OAuthAccounts, key)
	}

	var (
		user   domain.User
		exists bool
	)
	for _, existing := range s.data.Users {
		if existing.Email == normalizedEmail {
			user = existing
			exists = true
			break
		}
	}

	now := time.Now().UTC()
	if !exists {
		id, err := generateID()
		if err != nil {
			return domain.User{}, err
		}
		user = domain.User{
			ID:          id,
			DisplayName: displayName,
			Email:       normalizedEmail,
			Roles:       []string{"viewer"},
			SelfSignup:  true,
			CreatedAt:   now,
		}
	} else {
		if strings.TrimSpace(user.DisplayName) == "" {
			user.DisplayName = displayName
		}
	}

	s.data.Users[user.ID] = user
	s.data.OAuthAccounts[key] = domain.OAuthAccount{
		Provider:    provider,
		Subject:     subject,
		UserID:      user.ID,
		Email:       normalizedEmail,
		DisplayName: displayName,
		LinkedAt:    now,
	}

	if err := s.persist(); err != nil {
		if !exists {
			delete(s.data.Users, user.ID)
		} else {
			s.data.Users[user.ID] = user
		}
		delete(s.data.OAuthAccounts, key)
		return domain.User{}, err
	}

	return user, nil
}

type UserUpdate = domain.UserUpdate

// UpdateUser mutates user metadata while enforcing uniqueness constraints.
func (s *Storage) UpdateUser(id string, update UserUpdate) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	user, ok := updatedData.Users[id]
	if !ok {
		return domain.User{}, fmt.Errorf("user %s not found", id)
	}

	if update.DisplayName != nil {
		name := strings.TrimSpace(*update.DisplayName)
		if name == "" {
			return domain.User{}, errors.New("displayName cannot be empty")
		}
		user.DisplayName = name
	}

	if update.Email != nil {
		email := strings.TrimSpace(strings.ToLower(*update.Email))
		if email == "" {
			return domain.User{}, errors.New("email cannot be empty")
		}
		for existingID, existing := range updatedData.Users {
			if existingID == user.ID {
				continue
			}
			if existing.Email == email {
				return domain.User{}, fmt.Errorf("email %s already in use", email)
			}
		}
		user.Email = email
	}

	if update.Roles != nil {
		user.Roles = normalizeRoles(*update.Roles)
	}

	updatedData.Users[id] = user
	if err := s.persistDataset(updatedData); err != nil {
		return domain.User{}, err
	}

	s.data = updatedData

	return user, nil
}

// SetUserPassword replaces the stored password hash for the provided user.
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
	delete(updatedData.MFASettings, id)
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

type ProfileUpdate = domain.ProfileUpdate

// UpsertProfile executes UpsertProfile.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: no external transaction is required; it coordinates access with
// the in-memory mutex and persists snapshots to disk/object storage as needed.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (s *Storage) UpsertProfile(userID string, update ProfileUpdate) (domain.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	if _, ok := updatedData.Users[userID]; !ok {
		return domain.Profile{}, fmt.Errorf("user %s not found", userID)
	}

	profile, exists := updatedData.Profiles[userID]
	now := time.Now().UTC()
	if !exists {
		profile = domain.Profile{
			UserID:            userID,
			SocialLinks:       []domain.SocialLink{},
			TopFriends:        []string{},
			DonationAddresses: []domain.CryptoAddress{},
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
	if update.SocialLinks != nil {
		normalized, err := NormalizeSocialLinks(*update.SocialLinks)
		if err != nil {
			return domain.Profile{}, err
		}
		profile.SocialLinks = normalized
	}
	if update.FeaturedChannelID != nil {
		trimmed := strings.TrimSpace(*update.FeaturedChannelID)
		if trimmed == "" {
			profile.FeaturedChannelID = nil
		} else {
			channel, ok := updatedData.Channels[trimmed]
			if !ok {
				return domain.Profile{}, fmt.Errorf("featured channel %s not found", trimmed)
			}
			if channel.OwnerID != userID {
				return domain.Profile{}, errors.New("featured channel must belong to profile owner")
			}
			id := channel.ID
			profile.FeaturedChannelID = &id
		}
	}
	if update.TopFriends != nil {
		if len(*update.TopFriends) > 8 {
			return domain.Profile{}, errors.New("top friends cannot exceed eight entries")
		}
		seen := make(map[string]struct{})
		ordered := make([]string, 0, len(*update.TopFriends))
		for _, friendID := range *update.TopFriends {
			trimmed := strings.TrimSpace(friendID)
			if trimmed == "" {
				return domain.Profile{}, errors.New("top friends must reference valid users")
			}
			if trimmed == userID {
				return domain.Profile{}, errors.New("cannot add profile owner as a top friend")
			}
			if _, friendExists := updatedData.Users[trimmed]; !friendExists {
				return domain.Profile{}, fmt.Errorf("top friend %s not found", trimmed)
			}
			if _, duplicate := seen[trimmed]; duplicate {
				return domain.Profile{}, errors.New("duplicate user in top friends list")
			}
			seen[trimmed] = struct{}{}
			ordered = append(ordered, trimmed)
		}
		profile.TopFriends = ordered
	}
	if update.DonationAddresses != nil {
		addresses := make([]domain.CryptoAddress, 0, len(*update.DonationAddresses))
		for _, addr := range *update.DonationAddresses {
			normalized, err := NormalizeDonationAddress(addr)
			if err != nil {
				return domain.Profile{}, err
			}
			addresses = append(addresses, normalized)
		}
		profile.DonationAddresses = addresses
	}

	profile.UpdatedAt = now
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}

	updatedData.Profiles[userID] = profile
	if err := s.persistDataset(updatedData); err != nil {
		return domain.Profile{}, err
	}

	s.data = updatedData

	return profile, nil
}

// GetProfile executes GetProfile.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this signature does not return `error`; not-found/absence is represented by the
// boolean return value.
// Transactions/connections: no external transaction is required; it coordinates access with
// the in-memory mutex and persists snapshots to disk/object storage as needed.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (s *Storage) GetProfile(userID string) (domain.Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profile, ok := s.data.Profiles[userID]
	if !ok {
		user, userExists := s.data.Users[userID]
		if !userExists {
			return domain.Profile{}, false
		}
		profile = domain.Profile{
			UserID:            userID,
			SocialLinks:       []domain.SocialLink{},
			TopFriends:        []string{},
			DonationAddresses: []domain.CryptoAddress{},
			CreatedAt:         user.CreatedAt,
			UpdatedAt:         user.CreatedAt,
		}
		return profile, false
	}

	if profile.SocialLinks == nil {
		profile.SocialLinks = []domain.SocialLink{}
	}
	if profile.TopFriends == nil {
		profile.TopFriends = []string{}
	}
	if profile.DonationAddresses == nil {
		profile.DonationAddresses = []domain.CryptoAddress{}
	}

	return profile, true
}

// ListProfiles executes ListProfiles.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: no external transaction is required; it coordinates access with
// the in-memory mutex and persists snapshots to disk/object storage as needed.
// Ordering/pagination: returns a full result set (no cursor/offset pagination contract).
// Ordering is whatever this implementation explicitly enforces; otherwise it is unspecified.
func (s *Storage) ListProfiles() []domain.Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profiles := make([]domain.Profile, 0, len(s.data.Profiles))
	for _, profile := range s.data.Profiles {
		if profile.SocialLinks == nil {
			profile.SocialLinks = []domain.SocialLink{}
		}
		if profile.TopFriends == nil {
			profile.TopFriends = []string{}
		}
		if profile.DonationAddresses == nil {
			profile.DonationAddresses = []domain.CryptoAddress{}
		}
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].CreatedAt.Before(profiles[j].CreatedAt)
	})
	return profiles
}

// Channel operations

type ChannelUpdate = domain.ChannelUpdate

// CreateChannel executes CreateChannel.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: no external transaction is required; it coordinates access with
// the in-memory mutex and persists snapshots to disk/object storage as needed.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (s *Storage) CreateChannel(ownerID, title, category string, tags []string) (domain.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Users[ownerID]; !ok {
		return domain.Channel{}, fmt.Errorf("owner %s not found", ownerID)
	}
	if title = strings.TrimSpace(title); title == "" {
		return domain.Channel{}, errors.New("title is required")
	}

	id, err := generateID()
	if err != nil {
		return domain.Channel{}, err
	}
	streamKey, err := generateStreamKey()
	if err != nil {
		return domain.Channel{}, err
	}

	now := time.Now().UTC()
	channel := domain.Channel{
		ID:        id,
		OwnerID:   ownerID,
		StreamKey: streamKey,
		Title:     title,
		Category:  strings.TrimSpace(category),
		Tags:      normalizeTags(tags),
		Schedule:  []domain.ChannelScheduleEntry{},
		LiveState: "offline",
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.data.Channels[id] = channel
	if err := s.persist(); err != nil {
		delete(s.data.Channels, id)
		return domain.Channel{}, err
	}

	return channel, nil
}

// UpdateChannel executes UpdateChannel.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: no external transaction is required; it coordinates access with
// the in-memory mutex and persists snapshots to disk/object storage as needed.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (s *Storage) UpdateChannel(id string, update ChannelUpdate) (domain.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	channel, ok := updatedData.Channels[id]
	if !ok {
		return domain.Channel{}, fmt.Errorf("channel %s not found", id)
	}

	if update.Title != nil {
		if title := strings.TrimSpace(*update.Title); title != "" {
			channel.Title = title
		} else {
			return domain.Channel{}, errors.New("title cannot be empty")
		}
	}
	if update.Category != nil {
		channel.Category = strings.TrimSpace(*update.Category)
	}
	if update.Tags != nil {
		channel.Tags = normalizeTags(*update.Tags)
	}
	if update.Schedule != nil {
		schedule, err := normalizeChannelSchedule(*update.Schedule, channel.Schedule, time.Now().UTC())
		if err != nil {
			return domain.Channel{}, err
		}
		channel.Schedule = schedule
	}
	if update.LiveState != nil {
		state := strings.ToLower(strings.TrimSpace(*update.LiveState))
		if state != "offline" && state != "live" && state != "starting" && state != "ended" {
			return domain.Channel{}, fmt.Errorf("invalid liveState %s", state)
		}
		channel.LiveState = state
	}

	channel.UpdatedAt = time.Now().UTC()
	updatedData.Channels[id] = channel
	if err := s.persistDataset(updatedData); err != nil {
		return domain.Channel{}, err
	}

	s.data = updatedData

	channel.Tags = append([]string{}, channel.Tags...)
	channel.Schedule = cloneChannelSchedule(channel.Schedule)
	return channel, nil
}

// RotateChannelStreamKey executes RotateChannelStreamKey.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: no external transaction is required; it coordinates access with
// the in-memory mutex and persists snapshots to disk/object storage as needed.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (s *Storage) RotateChannelStreamKey(id string) (domain.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	channel, ok := updatedData.Channels[id]
	if !ok {
		return domain.Channel{}, fmt.Errorf("channel %s not found", id)
	}

	streamKey, err := generateStreamKey()
	if err != nil {
		return domain.Channel{}, err
	}

	channel.StreamKey = streamKey
	channel.UpdatedAt = time.Now().UTC()
	updatedData.Channels[id] = channel

	if err := s.persistDataset(updatedData); err != nil {
		return domain.Channel{}, err
	}

	s.data = updatedData

	channel.Tags = append([]string{}, channel.Tags...)
	channel.Schedule = cloneChannelSchedule(channel.Schedule)
	return channel, nil
}

// GetChannel executes GetChannel.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this signature does not return `error`; not-found/absence is represented by the
// boolean return value.
// Transactions/connections: no external transaction is required; it coordinates access with
// the in-memory mutex and persists snapshots to disk/object storage as needed.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (s *Storage) GetChannel(id string) (domain.Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	channel, ok := s.data.Channels[id]
	if ok {
		channel.Tags = append([]string{}, channel.Tags...)
		channel.Schedule = cloneChannelSchedule(channel.Schedule)
	}
	return channel, ok
}

// GetChannelByStreamKey looks up a channel by its stream key.
func (s *Storage) GetChannelByStreamKey(streamKey string) (domain.Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := strings.TrimSpace(streamKey)
	if key == "" {
		return domain.Channel{}, false
	}

	for _, channel := range s.data.Channels {
		if channel.StreamKey == key {
			channel.Tags = append([]string{}, channel.Tags...)
			channel.Schedule = cloneChannelSchedule(channel.Schedule)
			return channel, true
		}
	}

	return domain.Channel{}, false
}

// ListChannels executes ListChannels.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: no external transaction is required; it coordinates access with
// the in-memory mutex and persists snapshots to disk/object storage as needed.
// Ordering/pagination: returns a full result set (no cursor/offset pagination contract).
// Ordering is whatever this implementation explicitly enforces; otherwise it is unspecified.
func (s *Storage) ListChannels(ownerID, query string) []domain.Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	channels := make([]domain.Channel, 0, len(s.data.Channels))
	for _, channel := range s.data.Channels {
		if ownerID != "" && channel.OwnerID != ownerID {
			continue
		}
		if normalizedQuery != "" {
			owner := s.data.Users[channel.OwnerID]
			if !channelMatchesQuery(channel, owner, normalizedQuery) {
				continue
			}
		}
		channel.Tags = append([]string{}, channel.Tags...)
		channel.Schedule = cloneChannelSchedule(channel.Schedule)
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

// CountFollowersByChannelIDs returns follower totals for each requested channel.
func (s *Storage) CountFollowersByChannelIDs(channelIDs []string) map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	channelSet := make(map[string]struct{}, len(channelIDs))
	counts := make(map[string]int, len(channelIDs))
	for _, channelID := range channelIDs {
		channelSet[channelID] = struct{}{}
		counts[channelID] = 0
	}

	for _, follows := range s.data.Follows {
		if follows == nil {
			continue
		}
		for channelID := range follows {
			if _, ok := channelSet[channelID]; ok {
				counts[channelID]++
			}
		}
	}
	return counts
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

func (s *Storage) StartStream(channelID string, renditions []string) (domain.StreamSession, error) {
	s.mu.Lock()
	channel, ok := s.data.Channels[channelID]
	if !ok {
		s.mu.Unlock()
		return domain.StreamSession{}, fmt.Errorf("channel %s not found", channelID)
	}
	if channel.CurrentSessionID != nil {
		s.mu.Unlock()
		return domain.StreamSession{}, errors.New("channel already live")
	}

	sessionID, err := generateID()
	if err != nil {
		s.mu.Unlock()
		return domain.StreamSession{}, err
	}

	channel.CurrentSessionID = &sessionID
	channel.LiveState = "starting"
	s.data.Channels[channelID] = channel
	s.mu.Unlock()

	controller := s.ingestController
	if controller == nil {
		s.mu.Lock()
		if updated, exists := s.data.Channels[channelID]; exists {
			updated.CurrentSessionID = nil
			updated.LiveState = "offline"
			s.data.Channels[channelID] = updated
		}
		s.mu.Unlock()
		return domain.StreamSession{}, ErrIngestControllerUnavailable
	}

	timeout := normalizeIngestTimeout(s.ingestTimeout)
	boot, bootErr := runIngestBootWithRetry(controller, ingest.BootParams{
		ChannelID:  channelID,
		SessionID:  sessionID,
		StreamKey:  channel.StreamKey,
		Renditions: append([]string{}, renditions...),
	}, timeout, s.ingestMaxAttempts, s.ingestRetryInterval)
	if bootErr != nil {
		s.mu.Lock()
		if updated, exists := s.data.Channels[channelID]; exists {
			updated.CurrentSessionID = nil
			updated.LiveState = "offline"
			s.data.Channels[channelID] = updated
		}
		s.mu.Unlock()
		return domain.StreamSession{}, fmt.Errorf("boot ingest: %w", bootErr)
	}

	now := time.Now().UTC()
	session := domain.StreamSession{
		ID:             sessionID,
		ChannelID:      channelID,
		StartedAt:      now,
		Renditions:     append([]string{}, renditions...),
		PeakConcurrent: 0,
	}
	applyBootResultToSession(&session, boot, false)

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
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		_ = controller.ShutdownStream(ctx, channelID, sessionID, jobIDs)
		cancel()
		return domain.StreamSession{}, err
	}
	s.mu.Unlock()

	return session, nil
}

// StopStream executes StopStream.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: no external transaction is required; it coordinates access with
// the in-memory mutex and persists snapshots to disk/object storage as needed.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (s *Storage) StopStream(channelID string, peakConcurrent int) (domain.StreamSession, error) {
	s.mu.Lock()
	channel, ok := s.data.Channels[channelID]
	if !ok {
		s.mu.Unlock()
		return domain.StreamSession{}, fmt.Errorf("channel %s not found", channelID)
	}
	if channel.CurrentSessionID == nil {
		s.mu.Unlock()
		return domain.StreamSession{}, errors.New("channel is not live")
	}

	sessionID := *channel.CurrentSessionID
	session, ok := s.data.StreamSessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return domain.StreamSession{}, fmt.Errorf("session %s missing", sessionID)
	}

	originalChannel := channel
	originalSession := session
	jobIDs := append([]string{}, session.IngestJobIDs...)
	s.mu.Unlock()

	controller := s.ingestController
	if controller == nil {
		return domain.StreamSession{}, ErrIngestControllerUnavailable
	}

	timeout := normalizeIngestTimeout(s.ingestTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := controller.ShutdownStream(ctx, channelID, sessionID, jobIDs); err != nil {
		return domain.StreamSession{}, fmt.Errorf("shutdown ingest: %w", err)
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
		return domain.StreamSession{}, fmt.Errorf("channel %s not found", channelID)
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
		return domain.StreamSession{}, recErr
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
		return domain.StreamSession{}, err
	}
	s.mu.Unlock()

	return session, nil
}

// ListStreamSessions executes ListStreamSessions.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: no external transaction is required; it coordinates access with
// the in-memory mutex and persists snapshots to disk/object storage as needed.
// Ordering/pagination: returns a full result set (no cursor/offset pagination contract).
// Ordering is whatever this implementation explicitly enforces; otherwise it is unspecified.
func (s *Storage) ListStreamSessions(channelID string) ([]domain.StreamSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	sessions := make([]domain.StreamSession, 0)
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

// ListStreamSessionsByChannelIDs returns stream sessions grouped by channel.
func (s *Storage) ListStreamSessionsByChannelIDs(channelIDs []string) (map[string][]domain.StreamSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	channelSet := make(map[string]struct{}, len(channelIDs))
	grouped := make(map[string][]domain.StreamSession, len(channelIDs))
	for _, channelID := range channelIDs {
		if _, ok := s.data.Channels[channelID]; !ok {
			return nil, fmt.Errorf("channel %s not found", channelID)
		}
		channelSet[channelID] = struct{}{}
		grouped[channelID] = []domain.StreamSession{}
	}

	for _, session := range s.data.StreamSessions {
		if _, ok := channelSet[session.ChannelID]; ok {
			grouped[session.ChannelID] = append(grouped[session.ChannelID], session)
		}
	}

	for channelID := range grouped {
		sessions := grouped[channelID]
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].StartedAt.After(sessions[j].StartedAt)
		})
		grouped[channelID] = sessions
	}

	return grouped, nil
}

// CurrentStreamSession returns the active stream session for the channel if present.
func (s *Storage) CurrentStreamSession(channelID string) (domain.StreamSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	channel, ok := s.data.Channels[channelID]
	if !ok || channel.CurrentSessionID == nil {
		return domain.StreamSession{}, false
	}
	session, exists := s.data.StreamSessions[*channel.CurrentSessionID]
	if !exists {
		return domain.StreamSession{}, false
	}
	return session, true
}

// CurrentStreamSessionsByChannelIDs returns active sessions keyed by channel.
func (s *Storage) CurrentStreamSessionsByChannelIDs(channelIDs []string) map[string]domain.StreamSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	grouped := make(map[string]domain.StreamSession, len(channelIDs))
	for _, channelID := range channelIDs {
		channel, ok := s.data.Channels[channelID]
		if !ok || channel.CurrentSessionID == nil {
			continue
		}
		session, exists := s.data.StreamSessions[*channel.CurrentSessionID]
		if !exists {
			continue
		}
		grouped[channelID] = session
	}
	return grouped
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

// recordIngestHealth executes recordIngestHealth.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: no external transaction is required; it coordinates access with
// the in-memory mutex and persists snapshots to disk/object storage as needed.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
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
