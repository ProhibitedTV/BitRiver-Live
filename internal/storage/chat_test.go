package storage

import (
	"testing"
	"time"

	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
)

func TestListChatMessagesOrdering(t *testing.T) {
	store := newTestStore(t)
	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Alice",
		Email:       "alice@example.com",
	})
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

func TestDeleteChatMessage(t *testing.T) {
	store := newTestStore(t)
	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Alice",
		Email:       "alice@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(user.ID, "My Channel", "", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	msg, err := store.CreateChatMessage(channel.ID, user.ID, "hello")
	if err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}

	if err := store.DeleteChatMessage(channel.ID, msg.ID); err != nil {
		t.Fatalf("DeleteChatMessage: %v", err)
	}
	if err := store.DeleteChatMessage(channel.ID, msg.ID); err != nil {
		t.Fatalf("expected deleting missing message to be a no-op, got %v", err)
	}
}

func TestListFollowedChannelIDsOrdersByRecency(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{DisplayName: "Creator", Email: "creator@example.com"})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	viewer, err := store.CreateUser(CreateUserParams{DisplayName: "Viewer", Email: "viewer@example.com"})
	if err != nil {
		t.Fatalf("CreateUser viewer: %v", err)
	}
	first, err := store.CreateChannel(owner.ID, "Alpha", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel alpha: %v", err)
	}
	second, err := store.CreateChannel(owner.ID, "Beta", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel beta: %v", err)
	}

	if err := store.FollowChannel(viewer.ID, first.ID); err != nil {
		t.Fatalf("FollowChannel alpha: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := store.FollowChannel(viewer.ID, second.ID); err != nil {
		t.Fatalf("FollowChannel beta: %v", err)
	}

	followed := store.ListFollowedChannelIDs(viewer.ID)
	if len(followed) != 2 || followed[0] != second.ID || followed[1] != first.ID {
		t.Fatalf("expected channels ordered by recency, got %v", followed)
	}
}

func TestListChatRestrictionsSkipsExpiredTimeouts(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{DisplayName: "Owner", Email: "owner@example.com"})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Main", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	active, err := store.CreateUser(CreateUserParams{DisplayName: "Active", Email: "active@example.com"})
	if err != nil {
		t.Fatalf("CreateUser active: %v", err)
	}
	expired, err := store.CreateUser(CreateUserParams{DisplayName: "Expired", Email: "expired@example.com"})
	if err != nil {
		t.Fatalf("CreateUser expired: %v", err)
	}

	now := time.Now().UTC()
	store.mu.Lock()
	store.ensureDatasetInitializedLocked()
	if store.data.ChatTimeouts == nil {
		store.data.ChatTimeouts = make(map[string]map[string]time.Time)
	}
	store.data.ChatTimeouts[channel.ID] = map[string]time.Time{
		active.ID:  now.Add(10 * time.Minute),
		expired.ID: now.Add(-5 * time.Minute),
	}
	store.ensureTimeoutMetadata(channel.ID)
	store.data.ChatTimeoutIssuedAt[channel.ID][active.ID] = now.Add(-time.Minute)
	store.data.ChatTimeoutIssuedAt[channel.ID][expired.ID] = now.Add(-2 * time.Minute)
	store.data.ChatTimeoutActors[channel.ID][active.ID] = owner.ID
	store.data.ChatTimeoutActors[channel.ID][expired.ID] = owner.ID
	store.data.ChatTimeoutReasons[channel.ID][active.ID] = "active"
	store.data.ChatTimeoutReasons[channel.ID][expired.ID] = "expired"
	store.mu.Unlock()

	restrictions := store.ListChatRestrictions(channel.ID)
	if len(restrictions) != 1 {
		t.Fatalf("expected 1 restriction, got %d", len(restrictions))
	}
	if restrictions[0].TargetID != active.ID {
		t.Fatalf("expected restriction for %q, got %+v", active.ID, restrictions[0])
	}
	if restrictions[0].ExpiresAt == nil || restrictions[0].ExpiresAt.Before(now.Add(5*time.Minute)) {
		t.Fatalf("expected active timeout expiry to be retained, got %+v", restrictions[0].ExpiresAt)
	}

	store.mu.RLock()
	if timeouts := store.data.ChatTimeouts[channel.ID]; timeouts != nil {
		if _, ok := timeouts[expired.ID]; ok {
			t.Fatalf("expected expired timeout to be pruned")
		}
	}
	if actors := store.data.ChatTimeoutActors[channel.ID]; actors != nil {
		if _, ok := actors[expired.ID]; ok {
			t.Fatalf("expected expired timeout actor metadata to be pruned")
		}
	}
	if reasons := store.data.ChatTimeoutReasons[channel.ID]; reasons != nil {
		if _, ok := reasons[expired.ID]; ok {
			t.Fatalf("expected expired timeout reason metadata to be pruned")
		}
	}
	if issued := store.data.ChatTimeoutIssuedAt[channel.ID]; issued != nil {
		if _, ok := issued[expired.ID]; ok {
			t.Fatalf("expected expired timeout issued metadata to be pruned")
		}
	}
	store.mu.RUnlock()
}

func TestExpiredTimeoutsClearedAndPersisted(t *testing.T) {
	store := newTestStore(t)

	owner, err := store.CreateUser(CreateUserParams{DisplayName: "Owner", Email: "owner@example.com"})
	if err != nil {
		t.Fatalf("CreateUser owner: %v", err)
	}
	viewer, err := store.CreateUser(CreateUserParams{DisplayName: "Viewer", Email: "viewer@example.com"})
	if err != nil {
		t.Fatalf("CreateUser viewer: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Main", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	now := time.Now().UTC()
	store.mu.Lock()
	store.ensureDatasetInitializedLocked()
	if store.data.ChatTimeouts[channel.ID] == nil {
		store.data.ChatTimeouts[channel.ID] = make(map[string]time.Time)
	}
	store.data.ChatTimeouts[channel.ID][viewer.ID] = now.Add(-time.Minute)
	store.ensureTimeoutMetadata(channel.ID)
	store.data.ChatTimeoutIssuedAt[channel.ID][viewer.ID] = now.Add(-2 * time.Minute)
	store.data.ChatTimeoutActors[channel.ID][viewer.ID] = owner.ID
	store.data.ChatTimeoutReasons[channel.ID][viewer.ID] = "expired"
	store.mu.Unlock()

	if _, err := store.CreateChatMessage(channel.ID, viewer.ID, "hello"); err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}

	reloaded, err := NewStorage(store.filePath, WithIngestController(ingest.NoopController{}), WithIngestRetries(1, 0))
	if err != nil {
		t.Fatalf("NewStorage reload: %v", err)
	}
	if _, ok := reloaded.ChatTimeout(channel.ID, viewer.ID); ok {
		t.Fatalf("expected timeout to remain cleared after reload")
	}

	reloaded.mu.RLock()
	defer reloaded.mu.RUnlock()
	if timeouts := reloaded.data.ChatTimeouts[channel.ID]; timeouts != nil {
		if _, exists := timeouts[viewer.ID]; exists {
			t.Fatalf("timeout entry persisted unexpectedly")
		}
	}
	if issued := reloaded.data.ChatTimeoutIssuedAt[channel.ID]; issued != nil {
		if _, exists := issued[viewer.ID]; exists {
			t.Fatalf("timeout issued-at metadata persisted unexpectedly")
		}
	}
	if actors := reloaded.data.ChatTimeoutActors[channel.ID]; actors != nil {
		if _, exists := actors[viewer.ID]; exists {
			t.Fatalf("timeout actor metadata persisted unexpectedly")
		}
	}
	if reasons := reloaded.data.ChatTimeoutReasons[channel.ID]; reasons != nil {
		if _, exists := reasons[viewer.ID]; exists {
			t.Fatalf("timeout reason metadata persisted unexpectedly")
		}
	}
}

func TestChatReportsLifecycle(t *testing.T) {
	RunRepositoryChatReportsLifecycle(t, jsonRepositoryFactory)
}

func TestRepositoryChannelSearch(t *testing.T) {
	RunRepositoryChannelSearch(t, jsonRepositoryFactory)
}

func TestRepositoryChannelLookupByStreamKey(t *testing.T) {
	RunRepositoryChannelLookupByStreamKey(t, jsonRepositoryFactory)
}

func TestCloneDatasetCopiesModerationMetadata(t *testing.T) {
	now := time.Now().UTC()
	resolvedAt := now
	cancelledAt := now

	src := dataset{
		ChatBanActors: map[string]map[string]string{
			"chan": {"user": "actor"},
		},
		ChatBanReasons: map[string]map[string]string{
			"chan": {"user": "spam"},
		},
		ChatTimeoutActors: map[string]map[string]string{
			"chan": {"user": "actor"},
		},
		ChatTimeoutReasons: map[string]map[string]string{
			"chan": {"user": "caps"},
		},
		ChatTimeoutIssuedAt: map[string]map[string]time.Time{
			"chan": {"user": now},
		},
		ChatReports: map[string]models.ChatReport{
			"rep": {ID: "rep", ResolvedAt: &resolvedAt},
		},
		Tips: map[string]models.Tip{
			"tip": {ID: "tip", ChannelID: "chan", FromUserID: "supporter", CreatedAt: now},
		},
		Subscriptions: map[string]models.Subscription{
			"sub": {ID: "sub", ChannelID: "chan", UserID: "viewer", StartedAt: now, ExpiresAt: now.Add(time.Hour), Status: "active", CancelledAt: &cancelledAt},
		},
	}

	clone := cloneDataset(src)

	clone.ChatBanActors["chan"]["user"] = "other"
	if src.ChatBanActors["chan"]["user"] != "actor" {
		t.Fatalf("expected ban actors to be deep copied")
	}

	clone.ChatTimeoutReasons["chan"]["user"] = "other"
	if src.ChatTimeoutReasons["chan"]["user"] != "caps" {
		t.Fatalf("expected timeout reasons to be deep copied")
	}

	clone.ChatTimeoutIssuedAt["chan"]["user"] = now.Add(time.Minute)
	if !src.ChatTimeoutIssuedAt["chan"]["user"].Equal(now) {
		t.Fatalf("expected timeout issued timestamps to be deep copied")
	}

	newResolved := clone.ChatReports["rep"]
	*newResolved.ResolvedAt = newResolved.ResolvedAt.Add(time.Hour)
	if !src.ChatReports["rep"].ResolvedAt.Equal(resolvedAt) {
		t.Fatalf("expected report resolvedAt pointer to be cloned")
	}

	newSub := clone.Subscriptions["sub"]
	*newSub.CancelledAt = newSub.CancelledAt.Add(time.Hour)
	if !src.Subscriptions["sub"].CancelledAt.Equal(cancelledAt) {
		t.Fatalf("expected subscription cancelledAt pointer to be cloned")
	}
}
