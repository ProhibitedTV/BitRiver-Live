package storage

import (
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

	"bitriver-live/internal/models"
	"golang.org/x/crypto/pbkdf2"
)

const (
	passwordHashSaltLength = 16
	passwordHashKeyLength  = 32
	passwordHashIterations = 120000
)

type dataset struct {
	Users          map[string]models.User          `json:"users"`
	Channels       map[string]models.Channel       `json:"channels"`
	StreamSessions map[string]models.StreamSession `json:"streamSessions"`
	ChatMessages   map[string]models.ChatMessage   `json:"chatMessages"`
	Profiles       map[string]models.Profile       `json:"profiles"`
}

type Storage struct {
	mu       sync.RWMutex
	filePath string
	data     dataset
	// persistOverride allows tests to intercept persist operations.
	persistOverride func(dataset) error
}

func newDataset() dataset {
	return dataset{
		Users:          make(map[string]models.User),
		Channels:       make(map[string]models.Channel),
		StreamSessions: make(map[string]models.StreamSession),
		ChatMessages:   make(map[string]models.ChatMessage),
		Profiles:       make(map[string]models.Profile),
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
	if s.data.Profiles == nil {
		s.data.Profiles = make(map[string]models.Profile)
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

func NewStorage(path string) (*Storage, error) {
	store := &Storage{filePath: path}
	if err := store.load(); err != nil {
		return nil, err
	}
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

	return clone
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
	defer s.mu.Unlock()

	channel, ok := s.data.Channels[channelID]
	if !ok {
		return models.StreamSession{}, fmt.Errorf("channel %s not found", channelID)
	}
	if channel.CurrentSessionID != nil {
		return models.StreamSession{}, errors.New("channel already live")
	}

	sessionID, err := s.generateID()
	if err != nil {
		return models.StreamSession{}, err
	}

	now := time.Now().UTC()
	session := models.StreamSession{
		ID:             sessionID,
		ChannelID:      channelID,
		StartedAt:      now,
		Renditions:     append([]string{}, renditions...),
		PeakConcurrent: 0,
	}

	s.data.StreamSessions[sessionID] = session
	channel.CurrentSessionID = &sessionID
	channel.LiveState = "live"
	channel.UpdatedAt = now
	s.data.Channels[channelID] = channel

	if err := s.persist(); err != nil {
		delete(s.data.StreamSessions, sessionID)
		channel.CurrentSessionID = nil
		channel.LiveState = "offline"
		s.data.Channels[channelID] = channel
		return models.StreamSession{}, err
	}

	return session, nil
}

func (s *Storage) StopStream(channelID string, peakConcurrent int) (models.StreamSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	channel, ok := s.data.Channels[channelID]
	if !ok {
		return models.StreamSession{}, fmt.Errorf("channel %s not found", channelID)
	}
	if channel.CurrentSessionID == nil {
		return models.StreamSession{}, errors.New("channel is not live")
	}

	sessionID := *channel.CurrentSessionID
	session, ok := s.data.StreamSessions[sessionID]
	if !ok {
		return models.StreamSession{}, fmt.Errorf("session %s missing", sessionID)
	}

	now := time.Now().UTC()
	session.EndedAt = &now
	if peakConcurrent > session.PeakConcurrent {
		session.PeakConcurrent = peakConcurrent
	}
	s.data.StreamSessions[sessionID] = session

	channel.CurrentSessionID = nil
	channel.LiveState = "offline"
	channel.UpdatedAt = now
	s.data.Channels[channelID] = channel

	if err := s.persist(); err != nil {
		return models.StreamSession{}, err
	}

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
