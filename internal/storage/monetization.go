package storage

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"bitriver-live/internal/models"
)

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
	if amount.MinorUnits() <= 0 {
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
	if utf8.RuneCountInString(reference) > MaxTipReferenceLength {
		return models.Tip{}, fmt.Errorf("reference exceeds %d characters", MaxTipReferenceLength)
	}
	wallet := strings.TrimSpace(params.WalletAddress)
	if utf8.RuneCountInString(wallet) > MaxTipWalletAddressLength {
		return models.Tip{}, fmt.Errorf("wallet address exceeds %d characters", MaxTipWalletAddressLength)
	}
	message := strings.TrimSpace(params.Message)
	if utf8.RuneCountInString(message) > MaxTipMessageLength {
		return models.Tip{}, fmt.Errorf("message exceeds %d characters", MaxTipMessageLength)
	}
	if s.tipExists(provider, reference) {
		return models.Tip{}, errors.New(duplicateTipReferenceError)
	}
	id, err := generateID()
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
		WalletAddress: wallet,
		Message:       message,
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

// tipExists reports whether a tip with the given provider/reference pair is
// already persisted. Callers must hold s.mu.
func (s *Storage) tipExists(provider, reference string) bool {
	if len(s.data.Tips) == 0 {
		return false
	}
	for _, tip := range s.data.Tips {
		if tip.Provider == provider && tip.Reference == reference {
			return true
		}
	}
	return false
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
	if amount.MinorUnits() < 0 {
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
	for _, existing := range s.data.Subscriptions {
		if existing.Provider == provider && existing.Reference == reference {
			return models.Subscription{}, fmt.Errorf("subscription reference %s/%s already exists", provider, reference)
		}
	}
	id, err := generateID()
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
