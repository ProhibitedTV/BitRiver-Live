package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/models"
	"bitriver-live/internal/observability/metrics"
	"bitriver-live/internal/service"
)

type createTipRequest struct {
	Amount        json.Number `json:"amount"`
	Currency      string      `json:"currency"`
	Provider      string      `json:"provider"`
	Reference     string      `json:"reference,omitempty"`
	WalletAddress string      `json:"walletAddress,omitempty"`
	Message       string      `json:"message,omitempty"`
}

type tipResponse struct {
	ID             string       `json:"id"`
	ChannelID      string       `json:"channelId"`
	FromUserID     string       `json:"fromUserId"`
	Amount         models.Money `json:"amount"`
	Currency       string       `json:"currency"`
	Provider       string       `json:"provider"`
	Reference      string       `json:"reference"`
	WalletAddress  string       `json:"walletAddress,omitempty"`
	Message        string       `json:"message,omitempty"`
	Status         string       `json:"status"`
	IdempotencyKey string       `json:"idempotencyKey,omitempty"`
	CreatedAt      string       `json:"createdAt"`
}

type createSubscriptionRequest struct {
	Tier              string      `json:"tier"`
	Provider          string      `json:"provider"`
	Reference         string      `json:"reference,omitempty"`
	ExternalReference string      `json:"externalReference,omitempty"`
	Amount            json.Number `json:"amount"`
	Currency          string      `json:"currency"`
	DurationDays      int         `json:"durationDays"`
	AutoRenew         bool        `json:"autoRenew"`
}

type subscriptionResponse struct {
	ID                string       `json:"id"`
	ChannelID         string       `json:"channelId"`
	UserID            string       `json:"userId"`
	Tier              string       `json:"tier"`
	Provider          string       `json:"provider"`
	Reference         string       `json:"reference"`
	ExternalReference string       `json:"externalReference,omitempty"`
	Amount            models.Money `json:"amount"`
	Currency          string       `json:"currency"`
	StartedAt         string       `json:"startedAt"`
	ExpiresAt         string       `json:"expiresAt"`
	AutoRenew         bool         `json:"autoRenew"`
	Status            string       `json:"status"`
	CancelledBy       string       `json:"cancelledBy,omitempty"`
	CancelledReason   string       `json:"cancelledReason,omitempty"`
	CancelledAt       *string      `json:"cancelledAt,omitempty"`
	IdempotencyKey    string       `json:"idempotencyKey,omitempty"`
}

// parseMoneyNumber parses money number and returns an error when the input is malformed.
func parseMoneyNumber(number json.Number, field string) (models.Money, error) {
	raw := strings.TrimSpace(number.String())
	if raw == "" {
		return models.Money{}, fmt.Errorf("%s is required", field)
	}
	money, err := models.ParseMoney(raw)
	if err != nil {
		return models.Money{}, fmt.Errorf("invalid %s: %w", field, err)
	}
	return money, nil
}

// newTipResponse builds and returns tip response using the supplied dependencies.
func newTipResponse(tip models.Tip) tipResponse {
	return tipResponse{
		ID:             tip.ID,
		ChannelID:      tip.ChannelID,
		FromUserID:     tip.FromUserID,
		Amount:         tip.Amount,
		Currency:       tip.Currency,
		Provider:       tip.Provider,
		Reference:      tip.Reference,
		WalletAddress:  tip.WalletAddress,
		Message:        tip.Message,
		Status:         tip.Status,
		IdempotencyKey: tip.IdempotencyKey,
		CreatedAt:      tip.CreatedAt.Format(time.RFC3339Nano),
	}
}

// newSubscriptionResponse builds and returns subscription response using the supplied dependencies.
func newSubscriptionResponse(sub models.Subscription) subscriptionResponse {
	resp := subscriptionResponse{
		ID:                sub.ID,
		ChannelID:         sub.ChannelID,
		UserID:            sub.UserID,
		Tier:              sub.Tier,
		Provider:          sub.Provider,
		Reference:         sub.Reference,
		ExternalReference: sub.ExternalReference,
		Amount:            sub.Amount,
		Currency:          sub.Currency,
		StartedAt:         sub.StartedAt.Format(time.RFC3339Nano),
		ExpiresAt:         sub.ExpiresAt.Format(time.RFC3339Nano),
		AutoRenew:         sub.AutoRenew,
		Status:            sub.Status,
		CancelledBy:       sub.CancelledBy,
		CancelledReason:   sub.CancelledReason,
		IdempotencyKey:    sub.IdempotencyKey,
	}
	if sub.CancelledAt != nil {
		cancelled := sub.CancelledAt.Format(time.RFC3339Nano)
		resp.CancelledAt = &cancelled
	}
	return resp
}

// handleMonetizationRoutes routes and serves monetization routes requests, writing HTTP errors for invalid input or backend failures.
func (h *Handler) handleMonetizationRoutes(channel models.Channel, remaining []string, w http.ResponseWriter, r *http.Request) {
	if len(remaining) == 0 {
		WriteError(w, http.StatusNotFound, fmt.Errorf("unknown monetization path"))
		return
	}
	switch remaining[0] {
	case "tips":
		h.handleTipsRoutes(channel, remaining[1:], w, r)
	case "subscriptions":
		h.handleSubscriptionsRoutes(channel, remaining[1:], w, r)
	default:
		WriteError(w, http.StatusNotFound, fmt.Errorf("unknown monetization path"))
	}
}

// handleTipsRoutes routes and serves tips routes requests, writing HTTP errors for invalid input or backend failures.
func (h *Handler) handleTipsRoutes(channel models.Channel, remaining []string, w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	if len(remaining) > 0 && strings.TrimSpace(remaining[0]) != "" {
		WriteError(w, http.StatusNotFound, fmt.Errorf("unknown tips path"))
		return
	}
	switch r.Method {
	case http.MethodGet:
		if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		limit := 0
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			if value, err := strconv.Atoi(raw); err == nil && value > 0 {
				limit = value
			}
		}
		tips, err := h.Store.ListTips(channel.ID, limit)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		response := make([]tipResponse, 0, len(tips))
		for _, tip := range tips {
			response = append(response, newTipResponse(tip))
		}
		WriteJSON(w, http.StatusOK, response)
	case http.MethodPost:
		var req createTipRequest
		if !DecodeAndValidate(w, r, &req) {
			return
		}
		amount, err := parseMoneyNumber(req.Amount, "amount")
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if idempotencyKey == "" {
			idempotencyKey = service.BuildIdempotencyKey(actor.ID, req.Reference)
		}
		params := domain.TipCreateParams{
			ChannelID:      channel.ID,
			FromUserID:     actor.ID,
			Amount:         amount,
			Currency:       req.Currency,
			Provider:       req.Provider,
			Reference:      req.Reference,
			WalletAddress:  req.WalletAddress,
			Message:        req.Message,
			IdempotencyKey: idempotencyKey,
		}
		tip, err := h.PaymentService.CreatePendingTip(params)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		metrics.Default().ObserveMonetization("tip_pending", tip.Amount)
		WriteJSON(w, http.StatusCreated, newTipResponse(tip))
	default:
		WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPost)
	}
}

// handleSubscriptionsRoutes routes and serves subscriptions routes requests, writing HTTP errors for invalid input or backend failures.
func (h *Handler) handleSubscriptionsRoutes(channel models.Channel, remaining []string, w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	if len(remaining) > 0 && strings.TrimSpace(remaining[0]) != "" {
		subscriptionID := remaining[0]
		if len(remaining) == 1 {
			if r.Method != http.MethodDelete {
				WriteMethodNotAllowed(w, r, http.MethodDelete)
				return
			}
			sub, ok := h.Store.GetSubscription(subscriptionID)
			if !ok {
				WriteError(w, http.StatusNotFound, fmt.Errorf("subscription %s not found", subscriptionID))
				return
			}
			if sub.UserID != actor.ID && channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
				WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
				return
			}
			reason := strings.TrimSpace(r.URL.Query().Get("reason"))
			updated, err := h.Store.CancelSubscription(subscriptionID, actor.ID, reason)
			if err != nil {
				WriteError(w, http.StatusBadRequest, err)
				return
			}
			WriteJSON(w, http.StatusOK, newSubscriptionResponse(updated))
			return
		}
		WriteError(w, http.StatusNotFound, fmt.Errorf("unknown subscription path"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		includeInactive := false
		status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
		if status == "all" || status == "inactive" {
			includeInactive = true
		}
		subs, err := h.Store.ListSubscriptions(channel.ID, includeInactive)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		response := make([]subscriptionResponse, 0, len(subs))
		for _, sub := range subs {
			response = append(response, newSubscriptionResponse(sub))
		}
		WriteJSON(w, http.StatusOK, response)
	case http.MethodPost:
		var req createSubscriptionRequest
		if !DecodeAndValidate(w, r, &req) {
			return
		}
		durationDays := req.DurationDays
		if durationDays <= 0 {
			WriteRequestError(w, ValidationError("durationDays must be positive"))
			return
		}
		amount, err := parseMoneyNumber(req.Amount, "amount")
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if idempotencyKey == "" {
			idempotencyKey = service.BuildIdempotencyKey(actor.ID, req.Reference)
		}
		params := domain.SubscriptionCreateParams{
			ChannelID:         channel.ID,
			UserID:            actor.ID,
			Tier:              req.Tier,
			Provider:          req.Provider,
			Reference:         req.Reference,
			Amount:            amount,
			Currency:          req.Currency,
			Duration:          time.Duration(durationDays) * 24 * time.Hour,
			AutoRenew:         req.AutoRenew,
			ExternalReference: req.ExternalReference,
			IdempotencyKey:    idempotencyKey,
		}
		sub, err := h.PaymentService.CreatePendingSubscription(params)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		metrics.Default().ObserveMonetization("subscription_pending", sub.Amount)
		WriteJSON(w, http.StatusCreated, newSubscriptionResponse(sub))
	default:
		WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPost, http.MethodDelete)
	}
}

type paymentWebhookRequest struct {
	EventID        string `json:"eventId"`
	EntityType     string `json:"entityType"`
	Reference      string `json:"reference"`
	Status         string `json:"status"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

func (h *Handler) PaymentWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteMethodNotAllowed(w, r, http.MethodPost)
		return
	}
	provider := strings.TrimPrefix(r.URL.Path, "/api/payments/webhooks/")
	provider = strings.ToLower(strings.TrimSpace(provider))
	secret := strings.TrimSpace(h.WebhookSecrets[provider])
	if provider == "" || secret == "" {
		WriteError(w, http.StatusNotFound, fmt.Errorf("unsupported provider"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Errorf("read webhook body: %w", err))
		return
	}
	sig := strings.TrimSpace(r.Header.Get("X-Bitriver-Signature"))
	if !verifyWebhookSignature(secret, body, sig) {
		WriteError(w, http.StatusUnauthorized, fmt.Errorf("invalid webhook signature"))
		return
	}
	var req paymentWebhookRequest
	if err := json.Unmarshal(body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Errorf("decode webhook body: %w", err))
		return
	}
	tx, err := h.PaymentService.ProcessWebhook(provider, domain.ProcessPaymentWebhookParams{
		EventID: req.EventID, EntityType: req.EntityType, Reference: req.Reference, Status: req.Status, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return
	}
	WriteJSON(w, http.StatusOK, tx)
}

func verifyWebhookSignature(secret string, body []byte, provided string) bool {
	provided = strings.TrimSpace(strings.ToLower(strings.TrimPrefix(provided, "sha256=")))
	if provided == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(provided))
}
