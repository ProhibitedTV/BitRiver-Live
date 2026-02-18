package storage

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"bitriver-live/internal/domain"
)

// CreateTip records a tip event for a channel.
func (s *Storage) CreateTip(params CreateTipParams) (domain.Tip, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[params.ChannelID]; !ok {
		return domain.Tip{}, fmt.Errorf("channel %s not found", params.ChannelID)
	}
	if _, ok := s.data.Users[params.FromUserID]; !ok {
		return domain.Tip{}, fmt.Errorf("user %s not found", params.FromUserID)
	}
	amount := params.Amount
	if amount.MinorUnits() <= 0 {
		return domain.Tip{}, fmt.Errorf("amount must be positive")
	}
	currency := strings.ToUpper(strings.TrimSpace(params.Currency))
	if currency == "" {
		return domain.Tip{}, fmt.Errorf("currency is required")
	}
	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	if provider == "" {
		return domain.Tip{}, fmt.Errorf("provider is required")
	}
	reference := strings.TrimSpace(params.Reference)
	if reference == "" {
		reference = fmt.Sprintf("tip-%d", time.Now().UnixNano())
	}
	if utf8.RuneCountInString(reference) > MaxTipReferenceLength {
		return domain.Tip{}, fmt.Errorf("reference exceeds %d characters", MaxTipReferenceLength)
	}
	wallet := strings.TrimSpace(params.WalletAddress)
	if utf8.RuneCountInString(wallet) > MaxTipWalletAddressLength {
		return domain.Tip{}, fmt.Errorf("wallet address exceeds %d characters", MaxTipWalletAddressLength)
	}
	message := strings.TrimSpace(params.Message)
	if utf8.RuneCountInString(message) > MaxTipMessageLength {
		return domain.Tip{}, fmt.Errorf("message exceeds %d characters", MaxTipMessageLength)
	}
	if s.tipExists(provider, reference) {
		return domain.Tip{}, errors.New(duplicateTipReferenceError)
	}
	id, err := generateID()
	if err != nil {
		return domain.Tip{}, err
	}
	now := time.Now().UTC()
	idempotencyKey := strings.TrimSpace(params.IdempotencyKey)
	tip := domain.Tip{
		ID:             id,
		ChannelID:      params.ChannelID,
		FromUserID:     params.FromUserID,
		Amount:         amount,
		Currency:       currency,
		Provider:       provider,
		Reference:      reference,
		WalletAddress:  wallet,
		Message:        message,
		Status:         domain.PaymentStatePending,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      now,
	}
	if s.data.Tips == nil {
		s.data.Tips = make(map[string]domain.Tip)
	}
	s.data.Tips[id] = tip
	if err := s.persist(); err != nil {
		delete(s.data.Tips, id)
		return domain.Tip{}, err
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
func (s *Storage) ListTips(channelID string, limit int) ([]domain.Tip, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	tips := make([]domain.Tip, 0)
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
func (s *Storage) CreateSubscription(params CreateSubscriptionParams) (domain.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[params.ChannelID]; !ok {
		return domain.Subscription{}, fmt.Errorf("channel %s not found", params.ChannelID)
	}
	if _, ok := s.data.Users[params.UserID]; !ok {
		return domain.Subscription{}, fmt.Errorf("user %s not found", params.UserID)
	}
	if params.Duration <= 0 {
		return domain.Subscription{}, fmt.Errorf("duration must be positive")
	}
	amount := params.Amount
	if amount.MinorUnits() < 0 {
		return domain.Subscription{}, fmt.Errorf("amount cannot be negative")
	}
	currency := strings.ToUpper(strings.TrimSpace(params.Currency))
	if currency == "" {
		return domain.Subscription{}, fmt.Errorf("currency is required")
	}
	tier := strings.TrimSpace(params.Tier)
	if tier == "" {
		tier = "supporter"
	}
	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	if provider == "" {
		return domain.Subscription{}, fmt.Errorf("provider is required")
	}
	reference := strings.TrimSpace(params.Reference)
	if reference == "" {
		reference = fmt.Sprintf("sub-%d", time.Now().UnixNano())
	}
	for _, existing := range s.data.Subscriptions {
		if existing.Provider == provider && existing.Reference == reference {
			return domain.Subscription{}, fmt.Errorf("subscription reference %s/%s already exists", provider, reference)
		}
	}
	id, err := generateID()
	if err != nil {
		return domain.Subscription{}, err
	}
	started := time.Now().UTC()
	expires := started.Add(params.Duration)
	idempotencyKey := strings.TrimSpace(params.IdempotencyKey)
	subscription := domain.Subscription{
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
		Status:            domain.PaymentStatePending,
		ExternalReference: strings.TrimSpace(params.ExternalReference),
		IdempotencyKey:    idempotencyKey,
	}
	if s.data.Subscriptions == nil {
		s.data.Subscriptions = make(map[string]domain.Subscription)
	}
	s.data.Subscriptions[id] = subscription
	if err := s.persist(); err != nil {
		delete(s.data.Subscriptions, id)
		return domain.Subscription{}, err
	}
	return subscription, nil
}

// ListSubscriptions lists subscriptions for a channel.
func (s *Storage) ListSubscriptions(channelID string, includeInactive bool) ([]domain.Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	subs := make([]domain.Subscription, 0)
	for _, sub := range s.data.Subscriptions {
		if sub.ChannelID != channelID {
			continue
		}
		if !includeInactive && !(strings.EqualFold(sub.Status, "active") || strings.EqualFold(sub.Status, domain.PaymentStatePending) || strings.EqualFold(sub.Status, domain.PaymentStateConfirmed)) {
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
func (s *Storage) GetSubscription(id string) (domain.Subscription, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.data.Subscriptions[id]
	return sub, ok
}

// CancelSubscription marks a subscription as cancelled.
func (s *Storage) CancelSubscription(id, cancelledBy, reason string) (domain.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subscription, ok := s.data.Subscriptions[id]
	if !ok {
		return domain.Subscription{}, fmt.Errorf("subscription %s not found", id)
	}
	if subscription.Status == "cancelled" {
		return subscription, nil
	}
	if _, ok := s.data.Users[cancelledBy]; !ok {
		return domain.Subscription{}, fmt.Errorf("user %s not found", cancelledBy)
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
		return domain.Subscription{}, err
	}
	return subscription, nil
}

// ProcessPaymentWebhook applies a verified provider event with duplicate-event protection.
func (s *Storage) ProcessPaymentWebhook(params ProcessPaymentWebhookParams) (domain.PaymentTransaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	eventID := strings.TrimSpace(params.EventID)
	entityType := strings.ToLower(strings.TrimSpace(params.EntityType))
	reference := strings.TrimSpace(params.Reference)
	status := strings.ToLower(strings.TrimSpace(params.Status))
	idempotencyKey := strings.TrimSpace(params.IdempotencyKey)
	if provider == "" || eventID == "" || entityType == "" || reference == "" || status == "" {
		return domain.PaymentTransaction{}, fmt.Errorf("provider, eventID, entityType, reference and status are required")
	}
	for _, tx := range s.data.PaymentTransactions {
		if tx.Provider == provider && tx.EventID == eventID {
			return tx, nil
		}
	}
	var entityID string
	switch entityType {
	case "tip":
		for id, tip := range s.data.Tips {
			if tip.Provider == provider && tip.Reference == reference {
				tip.Status = status
				if idempotencyKey != "" {
					tip.IdempotencyKey = idempotencyKey
				}
				s.data.Tips[id] = tip
				entityID = id
				break
			}
		}
	case "subscription":
		for id, sub := range s.data.Subscriptions {
			if sub.Provider == provider && sub.Reference == reference {
				sub.Status = status
				if idempotencyKey != "" {
					sub.IdempotencyKey = idempotencyKey
				}
				s.data.Subscriptions[id] = sub
				entityID = id
				break
			}
		}
	default:
		return domain.PaymentTransaction{}, fmt.Errorf("unsupported entity type %s", entityType)
	}
	if entityID == "" {
		return domain.PaymentTransaction{}, fmt.Errorf("payment entity %s/%s not found", entityType, reference)
	}
	id, err := generateID()
	if err != nil {
		return domain.PaymentTransaction{}, err
	}
	now := time.Now().UTC()
	transaction := domain.PaymentTransaction{
		ID: id, Provider: provider, EventID: eventID, EntityType: entityType, EntityID: entityID,
		Reference: reference, Status: status, IdempotencyKey: idempotencyKey, CreatedAt: now,
	}
	if s.data.PaymentTransactions == nil {
		s.data.PaymentTransactions = make(map[string]domain.PaymentTransaction)
	}
	s.data.PaymentTransactions[id] = transaction
	if err := s.persist(); err != nil {
		delete(s.data.PaymentTransactions, id)
		return domain.PaymentTransaction{}, err
	}
	return transaction, nil
}
