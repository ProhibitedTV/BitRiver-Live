package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
)

// Ping always reports success for the JSON-backed repository.
func (s *Storage) Ping(context.Context) error {
	return nil
}

func newDataset() dataset {
	ds := dataset{
		Users:          make(map[string]models.User),
		OAuthAccounts:  make(map[string]models.OAuthAccount),
		Channels:       make(map[string]models.Channel),
		StreamSessions: make(map[string]models.StreamSession),
		Tips:           make(map[string]models.Tip),
		Subscriptions:  make(map[string]models.Subscription),
		Profiles:       make(map[string]models.Profile),
		Follows:        make(map[string]map[string]time.Time),
		Recordings:     make(map[string]models.Recording),
		ClipExports:    make(map[string]models.ClipExport),
	}
	initChatDataset(&ds)
	return ds
}

func (s *Storage) ensureDatasetInitializedLocked() {
	if s.data.Users == nil {
		s.data.Users = make(map[string]models.User)
	}
	if s.data.OAuthAccounts == nil {
		s.data.OAuthAccounts = make(map[string]models.OAuthAccount)
	}
	if s.data.Channels == nil {
		s.data.Channels = make(map[string]models.Channel)
	}
	if s.data.StreamSessions == nil {
		s.data.StreamSessions = make(map[string]models.StreamSession)
	}
	s.ensureChatDatasetInitializedLocked()
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
	if s.data.Uploads == nil {
		s.data.Uploads = make(map[string]models.Upload)
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

func oauthAccountKey(provider, subject string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	subject = strings.TrimSpace(subject)
	return provider + "|" + subject
}

func fallbackOAuthEmail(provider, subject string) string {
	domain := strings.ToLower(strings.TrimSpace(provider))
	if domain == "" {
		domain = "provider"
	}
	domain = sanitizeOAuthComponent(domain)
	hash := sha256.Sum256([]byte(provider + ":" + subject))
	local := hex.EncodeToString(hash[:8])
	return fmt.Sprintf("%s@%s.oauth", local, domain)
}

func defaultOAuthDisplayName(provider, email, subject string) string {
	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail != "" {
		local := strings.SplitN(trimmedEmail, "@", 2)[0]
		local = strings.ReplaceAll(local, ".", " ")
		local = strings.TrimSpace(local)
		if local != "" {
			return capitalizeWord(local)
		}
	}
	sanitized := sanitizeOAuthComponent(subject)
	if sanitized != "" {
		return sanitized
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return "Viewer"
	}
	return capitalizeWord(provider) + " user"
}

func sanitizeOAuthComponent(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == ' ':
			builder.WriteRune(' ')
		default:
			builder.WriteRune('-')
		}
	}
	return strings.TrimSpace(builder.String())
}

func capitalizeWord(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	return strings.ToUpper(lower[:1]) + lower[1:]
}

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
	store.ingestTimeout = normalizeIngestTimeout(store.ingestTimeout)
	if err := store.load(); err != nil {
		return nil, err
	}
	store.objectStorage = applyObjectStorageDefaults(store.objectStorage)
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

	if src.OAuthAccounts != nil {
		clone.OAuthAccounts = make(map[string]models.OAuthAccount, len(src.OAuthAccounts))
		for key, account := range src.OAuthAccounts {
			clone.OAuthAccounts[key] = account
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

	cloneChatData(src, &clone)

	if src.Tips != nil {
		clone.Tips = make(map[string]models.Tip, len(src.Tips))
		for id, tip := range src.Tips {
			clone.Tips[id] = tip
		}
	}

	if src.Subscriptions != nil {
		clone.Subscriptions = make(map[string]models.Subscription, len(src.Subscriptions))
		for id, subscription := range src.Subscriptions {
			cloned := subscription
			if subscription.CancelledAt != nil {
				cancelled := *subscription.CancelledAt
				cloned.CancelledAt = &cancelled
			}
			clone.Subscriptions[id] = cloned
		}
	}

	if src.Recordings != nil {
		clone.Recordings = make(map[string]models.Recording, len(src.Recordings))
		for id, recording := range src.Recordings {
			clone.Recordings[id] = cloneRecording(recording)
		}
	}

	if src.Uploads != nil {
		clone.Uploads = make(map[string]models.Upload, len(src.Uploads))
		for id, upload := range src.Uploads {
			clone.Uploads[id] = cloneUpload(upload)
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
			if profile.SocialLinks != nil {
				cloned.SocialLinks = append([]models.SocialLink(nil), profile.SocialLinks...)
			}
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

	id, err := generateID()
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

func (s *Storage) AuthenticateOAuth(params OAuthLoginParams) (models.User, error) {
	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	subject := strings.TrimSpace(params.Subject)
	if provider == "" {
		return models.User{}, errors.New("provider is required")
	}
	if subject == "" {
		return models.User{}, errors.New("subject is required")
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
		s.data.OAuthAccounts = make(map[string]models.OAuthAccount)
	}

	key := oauthAccountKey(provider, subject)
	if account, ok := s.data.OAuthAccounts[key]; ok {
		if user, ok := s.data.Users[account.UserID]; ok {
			return user, nil
		}
		delete(s.data.OAuthAccounts, key)
	}

	var (
		user   models.User
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
			return models.User{}, err
		}
		user = models.User{
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
	s.data.OAuthAccounts[key] = models.OAuthAccount{
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
		return models.User{}, err
	}

	return user, nil
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
	SocialLinks       *[]models.SocialLink
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
			SocialLinks:       []models.SocialLink{},
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
	if update.SocialLinks != nil {
		normalized, err := NormalizeSocialLinks(*update.SocialLinks)
		if err != nil {
			return models.Profile{}, err
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
			normalized, err := NormalizeDonationAddress(addr)
			if err != nil {
				return models.Profile{}, err
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
			SocialLinks:       []models.SocialLink{},
			TopFriends:        []string{},
			DonationAddresses: []models.CryptoAddress{},
			CreatedAt:         user.CreatedAt,
			UpdatedAt:         user.CreatedAt,
		}
		return profile, false
	}

	if profile.SocialLinks == nil {
		profile.SocialLinks = []models.SocialLink{}
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
		if profile.SocialLinks == nil {
			profile.SocialLinks = []models.SocialLink{}
		}
		if profile.TopFriends == nil {
			profile.TopFriends = []string{}
		}
		if profile.DonationAddresses == nil {
			profile.DonationAddresses = []models.CryptoAddress{}
		}
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

	id, err := generateID()
	if err != nil {
		return models.Channel{}, err
	}
	streamKey, err := generateStreamKey()
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

func (s *Storage) RotateChannelStreamKey(id string) (models.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedData := cloneDataset(s.data)

	channel, ok := updatedData.Channels[id]
	if !ok {
		return models.Channel{}, fmt.Errorf("channel %s not found", id)
	}

	streamKey, err := generateStreamKey()
	if err != nil {
		return models.Channel{}, err
	}

	channel.StreamKey = streamKey
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

// GetChannelByStreamKey looks up a channel by its stream key.
func (s *Storage) GetChannelByStreamKey(streamKey string) (models.Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := strings.TrimSpace(streamKey)
	if key == "" {
		return models.Channel{}, false
	}

	for _, channel := range s.data.Channels {
		if channel.StreamKey == key {
			return channel, true
		}
	}

	return models.Channel{}, false
}

func (s *Storage) ListChannels(ownerID, query string) []models.Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	channels := make([]models.Channel, 0, len(s.data.Channels))
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

func channelMatchesQuery(channel models.Channel, owner models.User, normalizedQuery string) bool {
	if normalizedQuery == "" {
		return true
	}
	if strings.Contains(strings.ToLower(channel.Title), normalizedQuery) {
		return true
	}
	if owner.DisplayName != "" && strings.Contains(strings.ToLower(owner.DisplayName), normalizedQuery) {
		return true
	}
	for _, tag := range channel.Tags {
		if strings.Contains(strings.ToLower(tag), normalizedQuery) {
			return true
		}
	}
	return false
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

	sessionID, err := generateID()
	if err != nil {
		s.mu.Unlock()
		return models.StreamSession{}, err
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
		return models.StreamSession{}, ErrIngestControllerUnavailable
	}

	attempts := s.ingestMaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	timeout := normalizeIngestTimeout(s.ingestTimeout)
	var boot ingest.BootResult
	var bootErr error
	for attempt := 0; attempt < attempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		boot, bootErr = controller.BootStream(ctx, ingest.BootParams{
			ChannelID:  channelID,
			SessionID:  sessionID,
			StreamKey:  channel.StreamKey,
			Renditions: append([]string{}, renditions...),
		})
		cancel()
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
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		_ = controller.ShutdownStream(ctx, channelID, sessionID, jobIDs)
		cancel()
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

	controller := s.ingestController
	if controller == nil {
		return models.StreamSession{}, ErrIngestControllerUnavailable
	}

	timeout := normalizeIngestTimeout(s.ingestTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := controller.ShutdownStream(ctx, channelID, sessionID, jobIDs); err != nil {
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
