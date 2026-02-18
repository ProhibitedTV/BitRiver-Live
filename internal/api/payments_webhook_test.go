package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	models "bitriver-live/internal/domain"
	"bitriver-live/internal/storage"
)

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestPaymentWebhookVerifiesSignatureAndAppliesState(t *testing.T) {
	h, store := newTestHandler(t)
	h.WebhookSecrets = map[string]string{"stripe": "top-secret"}

	owner, _ := store.CreateUser(storage.CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Password: "password123"})
	viewer, _ := store.CreateUser(storage.CreateUserParams{DisplayName: "viewer", Email: "viewer@example.com", Password: "password123"})
	channel, _ := store.CreateChannel(owner.ID, "Live", "gaming", nil)
	_, err := store.CreateTip(storage.CreateTipParams{ChannelID: channel.ID, FromUserID: viewer.ID, Amount: models.MustParseMoney("1"), Currency: "USD", Provider: "stripe", Reference: "tip-ref-1", IdempotencyKey: "key-1"})
	if err != nil {
		t.Fatalf("CreateTip: %v", err)
	}

	payload := map[string]string{"eventId": "evt_1", "entityType": "tip", "reference": "tip-ref-1", "status": models.PaymentStateConfirmed, "idempotencyKey": "key-1"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/payments/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("X-Bitriver-Signature", sign("top-secret", body))
	rec := httptest.NewRecorder()
	h.PaymentWebhook(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rec.Code, rec.Body.String())
	}

	tips, err := store.ListTips(channel.ID, 10)
	if err != nil {
		t.Fatalf("ListTips: %v", err)
	}
	if len(tips) != 1 || tips[0].Status != models.PaymentStateConfirmed {
		t.Fatalf("expected confirmed tip, got %+v", tips)
	}
}

func TestPaymentWebhookRejectsInvalidSignature(t *testing.T) {
	h, _ := newTestHandler(t)
	h.WebhookSecrets = map[string]string{"stripe": "top-secret"}
	body := []byte(`{"eventId":"evt_1","entityType":"tip","reference":"tip-ref-1","status":"confirmed"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/payments/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("X-Bitriver-Signature", sign("wrong", body))
	rec := httptest.NewRecorder()
	h.PaymentWebhook(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", rec.Code)
	}
}
