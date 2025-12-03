package storage

import (
	"testing"
	"time"

	"bitriver-live/internal/models"
)

func TestCreateTipAndList(t *testing.T) {
	RunRepositoryTipsLifecycle(t, jsonRepositoryFactory)
}

func TestStorageTipReferenceUniqueness(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	supporter, err := store.CreateUser(CreateUserParams{DisplayName: "supporter", Email: "supporter@example.com"})
	if err != nil {
		t.Fatalf("create supporter: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	params := CreateTipParams{
		ChannelID:  channel.ID,
		FromUserID: supporter.ID,
		Amount:     models.MustParseMoney("5"),
		Currency:   "usd",
		Provider:   "stripe",
		Reference:  "dup-ref",
	}
	if _, err := store.CreateTip(params); err != nil {
		t.Fatalf("create tip: %v", err)
	}
	if _, err := store.CreateTip(params); err == nil {
		t.Fatal("expected duplicate tip creation to fail")
	} else if err.Error() != duplicateTipReferenceError {
		t.Fatalf("unexpected duplicate error: %v", err)
	}
}

func TestCreateSubscriptionAndCancel(t *testing.T) {
	RunRepositorySubscriptionsLifecycle(t, jsonRepositoryFactory)
}

func TestSubscriptionReferenceUniquenessJSON(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	viewer, err := store.CreateUser(CreateUserParams{DisplayName: "viewer", Email: "viewer@example.com"})
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	params := CreateSubscriptionParams{
		ChannelID: channel.ID,
		UserID:    viewer.ID,
		Tier:      "tier1",
		Provider:  "stripe",
		Reference: "dup-sub",
		Amount:    models.MustParseMoney("4.99"),
		Currency:  "usd",
		Duration:  time.Hour,
	}
	if _, err := store.CreateSubscription(params); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	_, err = store.CreateSubscription(params)
	if err == nil {
		t.Fatal("expected duplicate subscription reference to fail")
	}
	if got, want := err.Error(), "subscription reference stripe/dup-sub already exists"; got != want {
		t.Fatalf("unexpected error: got %q want %q", got, want)
	}
}

func TestRepositoryMonetizationPrecision(t *testing.T) {
	RunRepositoryMonetizationPrecision(t, jsonRepositoryFactory)
}
