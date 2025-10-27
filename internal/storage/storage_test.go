package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"bitriver-live/internal/models"
)

func newTestStore(t *testing.T) *Storage {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	store, err := NewStorage(path)
	if err != nil {
		t.Fatalf("NewStorage error: %v", err)
	}
	return store
}

func TestCreateAndListUser(t *testing.T) {
	store := newTestStore(t)

	user, err := store.CreateUser("Alice", "alice@example.com", []string{"creator"})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if user.ID == "" {
		t.Fatal("expected user ID to be set")
	}

	users := store.ListUsers()
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Email != "alice@example.com" {
		t.Fatalf("expected email alice@example.com, got %s", users[0].Email)
	}
}

func TestCreateChannelAndStartStopStream(t *testing.T) {
	store := newTestStore(t)
	user, err := store.CreateUser("Alice", "alice@example.com", []string{"creator"})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	channel, err := store.CreateChannel(user.ID, "My Channel", "gaming", []string{"Action", "Indie"})
	if err != nil {
		t.Fatalf("CreateChannel returned error: %v", err)
	}
	if channel.StreamKey == "" {
		t.Fatal("expected stream key to be generated")
	}
	if channel.LiveState != "offline" {
		t.Fatalf("expected liveState offline, got %s", channel.LiveState)
	}

	session, err := store.StartStream(channel.ID, []string{"1080p", "720p"})
	if err != nil {
		t.Fatalf("StartStream returned error: %v", err)
	}
	if session.ChannelID != channel.ID {
		t.Fatalf("session channel mismatch: %s", session.ChannelID)
	}
	updated, ok := store.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("channel %s not found after start", channel.ID)
	}
	if updated.LiveState != "live" {
		t.Fatalf("expected live state live, got %s", updated.LiveState)
	}
	if updated.CurrentSessionID == nil || *updated.CurrentSessionID != session.ID {
		t.Fatal("expected current session ID to be set")
	}

	ended, err := store.StopStream(channel.ID, 42)
	if err != nil {
		t.Fatalf("StopStream returned error: %v", err)
	}
	if ended.EndedAt == nil {
		t.Fatal("expected session to have end time")
	}
	updated, ok = store.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("channel %s not found after stop", channel.ID)
	}
	if updated.LiveState != "offline" {
		t.Fatalf("expected live state offline, got %s", updated.LiveState)
	}
	if updated.CurrentSessionID != nil {
		t.Fatal("expected current session to be cleared")
	}
}

func TestStoragePersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")

	store, err := NewStorage(path)
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	user, err := store.CreateUser("Alice", "alice@example.com", nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := store.persist(); err != nil {
		t.Fatalf("persist: %v", err)
	}

	// reopen store and ensure data is present
	reopened, err := NewStorage(path)
	if err != nil {
		t.Fatalf("NewStorage reopen: %v", err)
	}
	if got, ok := reopened.GetUser(user.ID); !ok {
		t.Fatalf("expected to find persisted user %s", user.ID)
	} else if got.Email != user.Email {
		t.Fatalf("expected email %s, got %s", user.Email, got.Email)
	}
}

func TestListChatMessagesOrdering(t *testing.T) {
	store := newTestStore(t)
	user, err := store.CreateUser("Alice", "alice@example.com", nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(user.ID, "My Channel", "", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	msg1, err := store.CreateChatMessage(channel.ID, user.ID, "first")
	if err != nil {
		t.Fatalf("CreateChatMessage #1: %v", err)
	}
	msg2, err := store.CreateChatMessage(channel.ID, user.ID, "second")
	if err != nil {
		t.Fatalf("CreateChatMessage #2: %v", err)
	}

	msgs, err := store.ListChatMessages(channel.ID, 0)
	if err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].ID != msg2.ID {
		t.Fatalf("expected newest message first, got %s", msgs[0].ID)
	}
	if msgs[1].ID != msg1.ID {
		t.Fatalf("expected oldest message last, got %s", msgs[1].ID)
	}
}

func TestUpsertProfileCreatesProfile(t *testing.T) {
	store := newTestStore(t)
	owner, err := store.CreateUser("Streamer", "streamer@example.com", []string{"creator"})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	friend, err := store.CreateUser("Friend", "friend@example.com", nil)
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

func TestUpsertProfileTopFriendsLimit(t *testing.T) {
	store := newTestStore(t)
	owner, err := store.CreateUser("Owner", "owner@example.com", nil)
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}

	friendIDs := make([]string, 0, 9)
	for i := 0; i < 9; i++ {
		friend, err := store.CreateUser("Friend", fmt.Sprintf("friend%d@example.com", i), nil)
		if err != nil {
			t.Fatalf("CreateUser friend %d: %v", i, err)
		}
		friendIDs = append(friendIDs, friend.ID)
	}

	if _, err := store.UpsertProfile(owner.ID, ProfileUpdate{TopFriends: &friendIDs}); err == nil {
		t.Fatalf("expected error for more than eight top friends")
	}
}

func TestMain(m *testing.M) {
	// ensure tests do not leave temp files behind by relying on testing package cleanup
	code := m.Run()
	os.Exit(code)
}
