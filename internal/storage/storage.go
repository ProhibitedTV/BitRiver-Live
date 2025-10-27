package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"bitriver-live/internal/models"
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
		s.data = dataset{
			Users:          make(map[string]models.User),
			Channels:       make(map[string]models.Channel),
			StreamSessions: make(map[string]models.StreamSession),
			ChatMessages:   make(map[string]models.ChatMessage),
			Profiles:       make(map[string]models.Profile),
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("open store file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&s.data); err != nil {
		return fmt.Errorf("decode store file: %w", err)
	}

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

	return nil
}

func (s *Storage) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	file, err := os.Create(s.filePath)
	if err != nil {
		return fmt.Errorf("create store file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(s.data); err != nil {
		return fmt.Errorf("encode store file: %w", err)
	}
	return nil
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

func (s *Storage) CreateUser(displayName, email string, roles []string) (models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedEmail := strings.TrimSpace(strings.ToLower(email))
	if normalizedEmail == "" {
		return models.User{}, errors.New("email is required")
	}
	for _, user := range s.data.Users {
		if user.Email == normalizedEmail {
			return models.User{}, fmt.Errorf("email %s already in use", email)
		}
	}

	if displayName = strings.TrimSpace(displayName); displayName == "" {
		return models.User{}, errors.New("displayName is required")
	}

	id, err := s.generateID()
	if err != nil {
		return models.User{}, err
	}

	now := time.Now().UTC()
	user := models.User{
		ID:          id,
		DisplayName: displayName,
		Email:       normalizedEmail,
		Roles:       append([]string{}, roles...),
		CreatedAt:   now,
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

	if _, ok := s.data.Users[userID]; !ok {
		return models.Profile{}, fmt.Errorf("user %s not found", userID)
	}

	profile, exists := s.data.Profiles[userID]
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
			channel, ok := s.data.Channels[trimmed]
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
			if _, friendExists := s.data.Users[trimmed]; !friendExists {
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

	s.data.Profiles[userID] = profile
	if err := s.persist(); err != nil {
		return models.Profile{}, err
	}

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

	channel, ok := s.data.Channels[id]
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
	s.data.Channels[id] = channel
	if err := s.persist(); err != nil {
		return models.Channel{}, err
	}

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
