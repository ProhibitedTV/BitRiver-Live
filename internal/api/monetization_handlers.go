package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bitriver-live/internal/models"
	"bitriver-live/internal/observability/metrics"
	"bitriver-live/internal/storage"
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
	ID            string       `json:"id"`
	ChannelID     string       `json:"channelId"`
	FromUserID    string       `json:"fromUserId"`
	Amount        models.Money `json:"amount"`
	Currency      string       `json:"currency"`
	Provider      string       `json:"provider"`
	Reference     string       `json:"reference"`
	WalletAddress string       `json:"walletAddress,omitempty"`
	Message       string       `json:"message,omitempty"`
	CreatedAt     string       `json:"createdAt"`
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
}

type cancelSubscriptionRequest struct {
	Reason string `json:"reason"`
}

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

func newTipResponse(tip models.Tip) tipResponse {
	return tipResponse{
		ID:            tip.ID,
		ChannelID:     tip.ChannelID,
		FromUserID:    tip.FromUserID,
		Amount:        tip.Amount,
		Currency:      tip.Currency,
		Provider:      tip.Provider,
		Reference:     tip.Reference,
		WalletAddress: tip.WalletAddress,
		Message:       tip.Message,
		CreatedAt:     tip.CreatedAt.Format(time.RFC3339Nano),
	}
}

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
	}
	if sub.CancelledAt != nil {
		cancelled := sub.CancelledAt.Format(time.RFC3339Nano)
		resp.CancelledAt = &cancelled
	}
	return resp
}

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
		if err := DecodeJSON(r, &req); err != nil {
			WriteDecodeError(w, err)
			return
		}
		amount, err := parseMoneyNumber(req.Amount, "amount")
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		params := storage.CreateTipParams{
			ChannelID:     channel.ID,
			FromUserID:    actor.ID,
			Amount:        amount,
			Currency:      req.Currency,
			Provider:      req.Provider,
			Reference:     req.Reference,
			WalletAddress: req.WalletAddress,
			Message:       req.Message,
		}
		tip, err := h.Store.CreateTip(params)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		metrics.Default().ObserveMonetization("tip", tip.Amount)
		WriteJSON(w, http.StatusCreated, newTipResponse(tip))
	default:
		w.Header().Set("Allow", "GET, POST")
		WriteError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}

func (h *Handler) handleSubscriptionsRoutes(channel models.Channel, remaining []string, w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	if len(remaining) > 0 && strings.TrimSpace(remaining[0]) != "" {
		subscriptionID := remaining[0]
		if len(remaining) == 1 {
			if r.Method != http.MethodDelete {
				w.Header().Set("Allow", "DELETE")
				WriteError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
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
		if err := DecodeJSON(r, &req); err != nil {
			WriteDecodeError(w, err)
			return
		}
		durationDays := req.DurationDays
		if durationDays <= 0 {
			WriteError(w, http.StatusBadRequest, fmt.Errorf("durationDays must be positive"))
			return
		}
		amount, err := parseMoneyNumber(req.Amount, "amount")
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		params := storage.CreateSubscriptionParams{
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
		}
		sub, err := h.Store.CreateSubscription(params)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		metrics.Default().ObserveMonetization("subscription", sub.Amount)
		WriteJSON(w, http.StatusCreated, newSubscriptionResponse(sub))
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		WriteError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}
