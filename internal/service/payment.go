package service

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"bitriver-live/internal/models"
	"bitriver-live/internal/observability"
	"bitriver-live/internal/storage"
)

type PaymentService struct {
	repo   storage.Repository
	logger *slog.Logger
}

func NewPaymentService(repo storage.Repository, logger *slog.Logger) *PaymentService {
	if logger == nil {
		logger = slog.Default()
	}
	return &PaymentService{repo: repo, logger: logger}
}

func (s *PaymentService) CreatePendingTip(params storage.CreateTipParams) (models.Tip, error) {
	if s == nil || s.repo == nil {
		return models.Tip{}, fmt.Errorf("payment service unavailable")
	}
	return s.repo.CreateTip(params)
}

func (s *PaymentService) CreatePendingSubscription(params storage.CreateSubscriptionParams) (models.Subscription, error) {
	if s == nil || s.repo == nil {
		return models.Subscription{}, fmt.Errorf("payment service unavailable")
	}
	return s.repo.CreateSubscription(params)
}

func (s *PaymentService) ProcessWebhook(provider string, params storage.ProcessPaymentWebhookParams) (models.PaymentTransaction, error) {
	if s == nil || s.repo == nil {
		return models.PaymentTransaction{}, fmt.Errorf("payment service unavailable")
	}
	params.Provider = provider
	tx, err := s.repo.ProcessPaymentWebhook(params)
	if err != nil {
		return models.PaymentTransaction{}, err
	}
	observability.RecordPaymentTransition(s.logger, strings.ToLower(tx.EntityType), models.PaymentStatePending, strings.ToLower(tx.Status), models.NewMoneyFromMinorUnits(0))
	return tx, nil
}

func BuildIdempotencyKey(userID, reference string) string {
	return strings.TrimSpace(userID) + ":" + strings.TrimSpace(reference) + ":" + time.Now().UTC().Format("20060102")
}
