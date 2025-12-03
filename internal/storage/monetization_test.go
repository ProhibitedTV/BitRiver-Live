package storage

import (
	"fmt"
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

func TestUpsertProfileCreatesProfile(t *testing.T) {
	store := newTestStore(t)
	owner, err := store.CreateUser(CreateUserParams{
		DisplayName: "Streamer",
		Email:       "streamer@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	friend, err := store.CreateUser(CreateUserParams{
		DisplayName: "Friend",
		Email:       "friend@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser friend: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Main Stage", "music", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	bio := "Welcome to my river stage"
	avatar := "https://cdn.example.com/avatar.png"
	banner := "https://cdn.example.com/banner.png"
	featured := channel.ID
	topFriends := []string{friend.ID}
	donation := []models.CryptoAddress{{Currency: "eth", Address: "0xabc", Note: "Primary"}}

	profile, err := store.UpsertProfile(owner.ID, ProfileUpdate{
		Bio:               &bio,
		AvatarURL:         &avatar,
		BannerURL:         &banner,
		FeaturedChannelID: &featured,
		TopFriends:        &topFriends,
		DonationAddresses: &donation,
	})
	if err != nil {
		t.Fatalf("UpsertProfile: %v", err)
	}

	if profile.Bio != bio {
		t.Fatalf("expected bio %q, got %q", bio, profile.Bio)
	}
	if profile.FeaturedChannelID == nil || *profile.FeaturedChannelID != channel.ID {
		t.Fatalf("expected featured channel %s", channel.ID)
	}
	if len(profile.TopFriends) != 1 || profile.TopFriends[0] != friend.ID {
		t.Fatalf("expected top friends to include %s", friend.ID)
	}
	if len(profile.DonationAddresses) != 1 {
		t.Fatalf("expected 1 donation address, got %d", len(profile.DonationAddresses))
	}
	if profile.DonationAddresses[0].Currency != "ETH" {
		t.Fatalf("expected currency to be normalized to ETH, got %s", profile.DonationAddresses[0].Currency)
	}
	if profile.CreatedAt.IsZero() || profile.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps to be populated")
	}

	loaded, ok := store.GetProfile(owner.ID)
	if !ok {
		t.Fatalf("expected persisted profile")
	}
	if loaded.UpdatedAt.Before(profile.UpdatedAt) {
		t.Fatalf("expected loaded profile updated at >= stored profile")
	}

	// second update clears top friends and replaces donation details
	topFriends = []string{}
	donation = []models.CryptoAddress{{Currency: "btc", Address: "bc1xyz"}}
	updated, err := store.UpsertProfile(owner.ID, ProfileUpdate{
		TopFriends:        &topFriends,
		DonationAddresses: &donation,
	})
	if err != nil {
		t.Fatalf("UpsertProfile second update: %v", err)
	}
	if len(updated.TopFriends) != 0 {
		t.Fatalf("expected top friends cleared")
	}
	if len(updated.DonationAddresses) != 1 || updated.DonationAddresses[0].Currency != "BTC" {
		t.Fatalf("expected BTC donation address")
	}

	_, existing := store.GetProfile(friend.ID)
	if existing {
		t.Fatalf("expected friend to have no explicit profile yet")
	}
}

func TestUpsertProfileDonationValidation(t *testing.T) {
	store := newTestStore(t)
	owner, err := store.CreateUser(CreateUserParams{
		DisplayName: "Creator",
		Email:       "creator@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}

	valid := []models.CryptoAddress{{Currency: "eth", Address: "0xabc123"}}
	if _, err := store.UpsertProfile(owner.ID, ProfileUpdate{DonationAddresses: &valid}); err != nil {
		t.Fatalf("expected valid donation addresses to succeed: %v", err)
	}

	testCases := []struct {
		name     string
		donation []models.CryptoAddress
	}{
		{
			name:     "invalid currency",
			donation: []models.CryptoAddress{{Currency: "et1", Address: "0xabc123"}},
		},
		{
			name:     "too short",
			donation: []models.CryptoAddress{{Currency: "ETH", Address: "abc"}},
		},
		{
			name:     "invalid characters",
			donation: []models.CryptoAddress{{Currency: "ETH", Address: "bad address"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := store.UpsertProfile(owner.ID, ProfileUpdate{DonationAddresses: &tc.donation})
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestUpsertProfileTopFriendsLimit(t *testing.T) {
	store := newTestStore(t)
	owner, err := store.CreateUser(CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}

	friendIDs := make([]string, 0, 9)
	for i := 0; i < 9; i++ {
		friend, err := store.CreateUser(CreateUserParams{
			DisplayName: "Friend",
			Email:       fmt.Sprintf("friend%d@example.com", i),
		})
		if err != nil {
			t.Fatalf("CreateUser friend %d: %v", i, err)
		}
		friendIDs = append(friendIDs, friend.ID)
	}

	if _, err := store.UpsertProfile(owner.ID, ProfileUpdate{TopFriends: &friendIDs}); err == nil {
		t.Fatalf("expected error for more than eight top friends")
	}
}
