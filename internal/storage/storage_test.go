package storage

import (
	"errors"
	"testing"
)

func TestDeleteUserPersistFailureLeavesDataUntouched(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}

	target, err := store.CreateUser(CreateUserParams{
		DisplayName: "Target",
		Email:       "target@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser target: %v", err)
	}

	channel, err := store.CreateChannel(owner.ID, "Main", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if _, err := store.CreateChatMessage(channel.ID, target.ID, "hello"); err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}

	bio := "Hi"
	if _, err := store.UpsertProfile(target.ID, ProfileUpdate{Bio: &bio}); err != nil {
		t.Fatalf("UpsertProfile: %v", err)
	}

	friendBio := "Friend"
	topFriends := []string{target.ID}
	if _, err := store.UpsertProfile(owner.ID, ProfileUpdate{Bio: &friendBio, TopFriends: &topFriends}); err != nil {
		t.Fatalf("UpsertProfile friend: %v", err)
	}

	store.persistOverride = func(dataset) error {
		return errors.New("persist failed")
	}

	if err := store.DeleteUser(target.ID); err == nil {
		t.Fatalf("expected DeleteUser error when persist fails")
	}

	store.persistOverride = nil

	if _, ok := store.GetUser(target.ID); !ok {
		t.Fatalf("expected user %s to remain", target.ID)
	}

	profile, ok := store.GetProfile(target.ID)
	if !ok {
		t.Fatalf("expected profile for %s to remain", target.ID)
	}
	if profile.Bio != bio {
		t.Fatalf("expected profile bio %q, got %q", bio, profile.Bio)
	}

	friendProfile, ok := store.GetProfile(owner.ID)
	if !ok {
		t.Fatalf("expected owner profile to exist")
	}
	if len(friendProfile.TopFriends) != 1 || friendProfile.TopFriends[0] != target.ID {
		t.Fatalf("expected top friends to remain unchanged, got %v", friendProfile.TopFriends)
	}

	if messages, err := store.ListChatMessages(channel.ID, 0); err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	} else if len(messages) != 1 {
		t.Fatalf("expected chat messages to remain, got %d", len(messages))
	}
}

func TestUpsertProfilePersistFailureLeavesDataUntouched(t *testing.T) {
	store := newTestStore(t)

	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "User",
		Email:       "user@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	initialBio := "initial"
	if _, err := store.UpsertProfile(user.ID, ProfileUpdate{Bio: &initialBio}); err != nil {
		t.Fatalf("UpsertProfile initial: %v", err)
	}

	updatedBio := "updated"
	store.persistOverride = func(dataset) error {
		return errors.New("persist failed")
	}

	if _, err := store.UpsertProfile(user.ID, ProfileUpdate{Bio: &updatedBio}); err == nil {
		t.Fatalf("expected UpsertProfile error when persist fails")
	}

	store.persistOverride = nil

	profile, _ := store.GetProfile(user.ID)
	if profile.Bio != initialBio {
		t.Fatalf("expected bio %q, got %q", initialBio, profile.Bio)
	}
}

func TestUpdateChannelPersistFailureLeavesDataUntouched(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	channel, err := store.CreateChannel(owner.ID, "Title", "gaming", []string{"fun"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	newTitle := "Updated"
	store.persistOverride = func(dataset) error {
		return errors.New("persist failed")
	}

	if _, err := store.UpdateChannel(channel.ID, ChannelUpdate{Title: &newTitle}); err == nil {
		t.Fatalf("expected UpdateChannel error when persist fails")
	}

	store.persistOverride = nil

	current, ok := store.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("expected channel to remain")
	}
	if current.Title != channel.Title {
		t.Fatalf("expected title %q, got %q", channel.Title, current.Title)
	}
}

func TestDeleteChannelPersistFailureLeavesDataUntouched(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}

	channel, err := store.CreateChannel(owner.ID, "Main", "gaming", []string{"retro"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	session, err := store.StartStream(channel.ID, []string{"1080p"})
	if err != nil {
		t.Fatalf("StartStream: %v", err)
	}
	if _, err := store.StopStream(channel.ID, 10); err != nil {
		t.Fatalf("StopStream: %v", err)
	}
	if _, err := store.CreateChatMessage(channel.ID, owner.ID, "hello"); err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}

	store.persistOverride = func(dataset) error {
		return errors.New("persist failed")
	}

	if err := store.DeleteChannel(channel.ID); err == nil {
		t.Fatalf("expected DeleteChannel error when persist fails")
	}

	store.persistOverride = nil

	if _, ok := store.GetChannel(channel.ID); !ok {
		t.Fatalf("expected channel to remain")
	}
	if _, ok := store.data.StreamSessions[session.ID]; !ok {
		t.Fatalf("expected stream session to remain")
	}
	if messages, err := store.ListChatMessages(channel.ID, 0); err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	} else if len(messages) != 1 {
		t.Fatalf("expected chat message to remain, got %d", len(messages))
	}
}

func TestFollowChannelLifecycle(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{DisplayName: "Creator", Email: "creator@example.com"})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	viewer, err := store.CreateUser(CreateUserParams{DisplayName: "Viewer", Email: "viewer@example.com"})
	if err != nil {
		t.Fatalf("CreateUser viewer: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Workshop", "maker", []string{"cnc"})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	if count := store.CountFollowers(channel.ID); count != 0 {
		t.Fatalf("expected zero followers, got %d", count)
	}
	if store.IsFollowingChannel(viewer.ID, channel.ID) {
		t.Fatal("expected viewer to not follow channel")
	}
	if followed := store.ListFollowedChannelIDs(viewer.ID); followed != nil {
		t.Fatalf("expected no followed channels, got %v", followed)
	}

	if err := store.FollowChannel(viewer.ID, channel.ID); err != nil {
		t.Fatalf("FollowChannel: %v", err)
	}
	if err := store.FollowChannel(viewer.ID, channel.ID); err != nil {
		t.Fatalf("FollowChannel idempotency: %v", err)
	}
	if count := store.CountFollowers(channel.ID); count != 1 {
		t.Fatalf("expected one follower, got %d", count)
	}
	if !store.IsFollowingChannel(viewer.ID, channel.ID) {
		t.Fatal("expected viewer to follow channel")
	}
	followed := store.ListFollowedChannelIDs(viewer.ID)
	if len(followed) != 1 || followed[0] != channel.ID {
		t.Fatalf("unexpected followed list: %v", followed)
	}

	if err := store.UnfollowChannel(viewer.ID, channel.ID); err != nil {
		t.Fatalf("UnfollowChannel: %v", err)
	}
	if err := store.UnfollowChannel(viewer.ID, channel.ID); err != nil {
		t.Fatalf("UnfollowChannel idempotency: %v", err)
	}
	if count := store.CountFollowers(channel.ID); count != 0 {
		t.Fatalf("expected zero followers after unfollow, got %d", count)
	}
	if store.IsFollowingChannel(viewer.ID, channel.ID) {
		t.Fatal("expected viewer to not follow channel after unfollow")
	}
}
