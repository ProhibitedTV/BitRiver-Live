package service

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"bitriver-live/internal/domain"
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

func (s *PaymentService) CreatePendingTip(params storage.CreateTipParams) (domain.Tip, error) {
	if s == nil || s.repo == nil {
		return domain.Tip{}, fmt.Errorf("payment service unavailable")
	}
	return s.repo.CreateTip(params)
}

func (s *PaymentService) CreatePendingSubscription(params storage.CreateSubscriptionParams) (domain.Subscription, error) {
	if s == nil || s.repo == nil {
		return domain.Subscription{}, fmt.Errorf("payment service unavailable")
	}
	return s.repo.CreateSubscription(params)
}

func (s *PaymentService) ProcessWebhook(provider string, params storage.ProcessPaymentWebhookParams) (domain.PaymentTransaction, error) {
	if s == nil || s.repo == nil {
		return domain.PaymentTransaction{}, fmt.Errorf("payment service unavailable")
	}
	params.Provider = provider
	tx, err := s.repo.ProcessPaymentWebhook(params)
	if err != nil {
		return domain.PaymentTransaction{}, err
	}
	observability.RecordPaymentTransition(s.logger, strings.ToLower(tx.EntityType), domain.PaymentStatePending, strings.ToLower(tx.Status), domain.NewMoneyFromMinorUnits(0))
	return tx, nil
}

func BuildIdempotencyKey(userID, reference string) string {
	return strings.TrimSpace(userID) + ":" + strings.TrimSpace(reference) + ":" + time.Now().UTC().Format("20060102")
}
