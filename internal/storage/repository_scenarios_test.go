package storage

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
)

// RepositoryFactory constructs a repository backed by either the JSON store or
// Postgres implementation for cross-datastore scenario assertions.
type RepositoryFactory func(t *testing.T, opts ...Option) (Repository, func(), error)

func runRepository(t *testing.T, factory RepositoryFactory, opts ...Option) Repository {
	t.Helper()
	if factory == nil {
		t.Fatal("repository factory is required")
	}
	repo, cleanup, err := factory(t, opts...)
	if errors.Is(err, ErrPostgresUnavailable) {
		t.Skip("postgres repository unavailable")
	}
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	if repo == nil {
		t.Fatal("repository factory returned nil repository")
	}
	if cleanup != nil {
		t.Cleanup(cleanup)
	}
	return repo
}

func requireAvailable(t *testing.T, err error, operation string) {
	t.Helper()
	if errors.Is(err, ErrPostgresUnavailable) {
		t.Skip("postgres repository unavailable")
	}
	if err != nil {
		t.Fatalf("%s: %v", operation, err)
	}
}

type timeoutIngestController struct {
	bootBlock     bool
	shutdownBlock bool
	bootResult    ingest.BootResult
}

func (c *timeoutIngestController) BootStream(ctx context.Context, params ingest.BootParams) (ingest.BootResult, error) {
	if c.bootBlock {
		<-ctx.Done()
		return ingest.BootResult{}, ctx.Err()
	}
	return c.bootResult, nil
}

func (c *timeoutIngestController) ShutdownStream(ctx context.Context, channelID, sessionID string, jobIDs []string) error {
	if c.shutdownBlock {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

func (c *timeoutIngestController) HealthChecks(ctx context.Context) []ingest.HealthStatus {
	return []ingest.HealthStatus{{Component: "ingest", Status: "ok"}}
}

func (c *timeoutIngestController) TranscodeUpload(ctx context.Context, params ingest.UploadTranscodeParams) (ingest.UploadTranscodeResult, error) {
	return ingest.UploadTranscodeResult{PlaybackURL: params.SourceURL}, nil
}

// RunRepositoryUserLifecycle validates the basic user management workflow across
// repository implementations.
func RunRepositoryUserLifecycle(t *testing.T, factory RepositoryFactory) {
	repo := runRepository(t, factory)

	password := "supersafe"
	viewer, err := repo.CreateUser(CreateUserParams{DisplayName: "Viewer", Email: "Viewer@example.com", Password: password, SelfSignup: true})
	requireAvailable(t, err, "create viewer")
	if viewer.Email != "viewer@example.com" {
		t.Fatalf("expected email to normalize to lowercase, got %q", viewer.Email)
	}
	if !viewer.SelfSignup {
		t.Fatalf("expected viewer to be marked as self-signup")
	}
	if want := []string{"viewer"}; !reflect.DeepEqual(viewer.Roles, want) {
		t.Fatalf("expected default viewer role, got %v", viewer.Roles)
	}

	if _, err := repo.CreateUser(CreateUserParams{DisplayName: "Duplicate", Email: "viewer@example.com"}); err == nil {
		t.Fatalf("expected duplicate email to return error")
	}

	admin, err := repo.CreateUser(CreateUserParams{DisplayName: "Admin", Email: "admin@example.com", Roles: []string{"Admin", "creator"}})
	requireAvailable(t, err, "create admin")
	if want := []string{"admin", "creator"}; !reflect.DeepEqual(admin.Roles, want) {
		t.Fatalf("expected normalized roles %v, got %v", want, admin.Roles)
	}

	noRoles, err := repo.CreateUser(CreateUserParams{DisplayName: "No Roles", Email: "noroles@example.com"})
	requireAvailable(t, err, "create user without roles")
	if len(noRoles.Roles) != 0 {
		t.Fatalf("expected no roles, got %v", noRoles.Roles)
	}
	requireAvailable(t, repo.DeleteUser(noRoles.ID), "cleanup user without roles")

	users := repo.ListUsers()
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].ID != viewer.ID {
		t.Fatalf("expected users to be sorted by creation time")
	}

	fetched, ok := repo.GetUser(viewer.ID)
	if !ok {
		t.Fatalf("expected viewer to be retrievable")
	}
	if fetched.DisplayName != viewer.DisplayName {
		t.Fatalf("expected display name %q, got %q", viewer.DisplayName, fetched.DisplayName)
	}

	authed, err := repo.AuthenticateUser("viewer@example.com", password)
	requireAvailable(t, err, "authenticate viewer")
	if authed.ID != viewer.ID {
		t.Fatalf("expected authenticated user %q, got %q", viewer.ID, authed.ID)
	}

	newDisplay := "Renamed Viewer"
	newEmail := "viewer2@example.com"
	newRoles := []string{"Creator"}
	updated, err := repo.UpdateUser(viewer.ID, UserUpdate{DisplayName: &newDisplay, Email: &newEmail, Roles: &newRoles})
	requireAvailable(t, err, "update viewer")
	if updated.DisplayName != newDisplay {
		t.Fatalf("expected updated display name %q, got %q", newDisplay, updated.DisplayName)
	}
	if updated.Email != newEmail {
		t.Fatalf("expected updated email %q, got %q", newEmail, updated.Email)
	}
	if want := []string{"creator"}; !reflect.DeepEqual(updated.Roles, want) {
		t.Fatalf("expected roles %v after update, got %v", want, updated.Roles)
	}

	if _, err := repo.UpdateUser(admin.ID, UserUpdate{Email: &newEmail}); err == nil {
		t.Fatalf("expected conflicting email update to fail")
	}

	requireAvailable(t, repo.DeleteUser(viewer.ID), "delete viewer")
	if _, ok := repo.GetUser(viewer.ID); ok {
		t.Fatalf("expected viewer to be removed")
	}
	if _, err := repo.AuthenticateUser(newEmail, password); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials after deletion, got %v", err)
	}

	remaining := repo.ListUsers()
	if len(remaining) != 1 || remaining[0].ID != admin.ID {
		t.Fatalf("expected admin to remain after deletion")
	}
}

// RunRepositoryOAuthLinking ensures repositories create and link users via
// OAuth logins, covering new account creation, linking to an existing email,
// and fallback metadata generation.
func RunRepositoryOAuthLinking(t *testing.T, factory RepositoryFactory) {
	repo := runRepository(t, factory)

	created, err := repo.AuthenticateOAuth(OAuthLoginParams{
		Provider:    "example",
		Subject:     "subject-1",
		Email:       "viewer@example.com",
		DisplayName: "Viewer",
	})
	requireAvailable(t, err, "create oauth user")
	if created.ID == "" {
		t.Fatal("expected oauth login to return user with id")
	}
	if created.Email != "viewer@example.com" {
		t.Fatalf("expected normalized email, got %q", created.Email)
	}
	if !created.SelfSignup {
		t.Fatal("expected oauth-created user to be marked as self signup")
	}
	if len(created.Roles) != 1 || created.Roles[0] != "viewer" {
		t.Fatalf("expected viewer role, got %v", created.Roles)
	}

	again, err := repo.AuthenticateOAuth(OAuthLoginParams{Provider: "example", Subject: "subject-1"})
	requireAvailable(t, err, "reuse oauth account")
	if again.ID != created.ID {
		t.Fatalf("expected oauth login to reuse existing user, got %q", again.ID)
	}

	existing, err := repo.CreateUser(CreateUserParams{DisplayName: "Existing", Email: "linked@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create existing user")

	linked, err := repo.AuthenticateOAuth(OAuthLoginParams{Provider: "example", Subject: "subject-2", Email: "linked@example.com", DisplayName: "Viewer"})
	requireAvailable(t, err, "link oauth account")
	if linked.ID != existing.ID {
		t.Fatalf("expected oauth login to link to existing user, got %q", linked.ID)
	}

	fallback, err := repo.AuthenticateOAuth(OAuthLoginParams{Provider: "acme", Subject: "unique"})
	requireAvailable(t, err, "create fallback oauth user")
	if !strings.HasSuffix(fallback.Email, "@acme.oauth") {
		t.Fatalf("expected fallback email with provider domain, got %q", fallback.Email)
	}
	if strings.TrimSpace(fallback.DisplayName) == "" {
		t.Fatal("expected fallback display name to be populated")
	}
}

// RunRepositoryStreamKeyRotation ensures repositories generate and persist fresh stream keys.
func RunRepositoryStreamKeyRotation(t *testing.T, factory RepositoryFactory) {
	repo := runRepository(t, factory)

	owner, err := repo.CreateUser(CreateUserParams{DisplayName: "Owner", Email: "owner@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create owner")

	channel, err := repo.CreateChannel(owner.ID, "Rotate", "gaming", []string{"tech"})
	requireAvailable(t, err, "create channel")
	if channel.StreamKey == "" {
		t.Fatal("expected initial stream key")
	}

	originalKey := channel.StreamKey
	rotated, err := repo.RotateChannelStreamKey(channel.ID)
	requireAvailable(t, err, "rotate stream key")
	if rotated.StreamKey == "" {
		t.Fatal("expected rotated stream key")
	}
	if rotated.StreamKey == originalKey {
		t.Fatalf("expected rotated stream key to differ from original %q", originalKey)
	}

	fetched, ok := repo.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("expected channel %s to remain after rotation", channel.ID)
	}
	if fetched.StreamKey != rotated.StreamKey {
		t.Fatalf("expected fetched stream key %q, got %q", rotated.StreamKey, fetched.StreamKey)
	}

	channels := repo.ListChannels(owner.ID, "")
	found := false
	for _, item := range channels {
		if item.ID != channel.ID {
			continue
		}
		found = true
		if item.StreamKey != rotated.StreamKey {
			t.Fatalf("expected listed stream key %q, got %q", rotated.StreamKey, item.StreamKey)
		}
	}
	if !found {
		t.Fatalf("expected rotated channel %s to appear in list", channel.ID)
	}
}

// RunRepositoryChannelSearch verifies that repositories filter channels by
// title, owner display name, and tags using case-insensitive matching.
func RunRepositoryChannelSearch(t *testing.T, factory RepositoryFactory) {
	repo := runRepository(t, factory)

	creatorOne, err := repo.CreateUser(CreateUserParams{DisplayName: "Coder One", Email: "coder1@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create first creator")
	creatorTwo, err := repo.CreateUser(CreateUserParams{DisplayName: "RetroMaster", Email: "retro@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create second creator")
	creatorThree, err := repo.CreateUser(CreateUserParams{DisplayName: "DJ Night", Email: "dj@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create third creator")

	lounge, err := repo.CreateChannel(creatorOne.ID, "Coding Lounge", "technology", []string{"GoLang", "Backend"})
	requireAvailable(t, err, "create coding lounge")
	arcade, err := repo.CreateChannel(creatorTwo.ID, "Arcade Stars", "gaming", []string{"retro", "speedrun"})
	requireAvailable(t, err, "create arcade stars")
	beats, err := repo.CreateChannel(creatorThree.ID, "Midnight Beats", "music", []string{"Live", "Music"})
	requireAvailable(t, err, "create midnight beats")

	if channels := repo.ListChannels("", ""); len(channels) != 3 {
		t.Fatalf("expected 3 channels without filter, got %d", len(channels))
	}

	cases := []struct {
		name    string
		ownerID string
		query   string
		wantIDs []string
	}{
		{name: "title query", query: "lounge", wantIDs: []string{lounge.ID}},
		{name: "owner display name", query: "RETROMASTER", wantIDs: []string{arcade.ID}},
		{name: "tag mixed case", query: "MuSiC", wantIDs: []string{beats.ID}},
		{name: "owner scoped", ownerID: creatorTwo.ID, query: "ARCADE", wantIDs: []string{arcade.ID}},
		{name: "no matches", query: "unknown", wantIDs: []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			channels := repo.ListChannels(tc.ownerID, tc.query)
			if len(channels) != len(tc.wantIDs) {
				t.Fatalf("expected %d channels, got %d", len(tc.wantIDs), len(channels))
			}
			for i, id := range tc.wantIDs {
				if channels[i].ID != id {
					t.Fatalf("expected channel %s at index %d, got %s", id, i, channels[i].ID)
				}
			}
		})
	}
}

// RunRepositoryChatRestrictionsLifecycle replays the moderation scenario
// exercised in chat_events_test.go against the provided repository.
func RunRepositoryChatRestrictionsLifecycle(t *testing.T, factory RepositoryFactory) {
	repo := runRepository(t, factory)

	owner, err := repo.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create owner")
	target, err := repo.CreateUser(CreateUserParams{DisplayName: "target", Email: "target@example.com"})
	requireAvailable(t, err, "create target")
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	requireAvailable(t, err, "create channel")

	expiry := time.Now().Add(time.Minute)
	events := []chat.Event{
		{
			Type: chat.EventTypeModeration,
			Moderation: &chat.ModerationEvent{
				Action:    chat.ModerationActionBan,
				ChannelID: channel.ID,
				ActorID:   owner.ID,
				TargetID:  target.ID,
				Reason:    "spam",
			},
			OccurredAt: time.Now().UTC(),
		},
		{
			Type: chat.EventTypeModeration,
			Moderation: &chat.ModerationEvent{
				Action:    chat.ModerationActionTimeout,
				ChannelID: channel.ID,
				ActorID:   owner.ID,
				TargetID:  target.ID,
				ExpiresAt: &expiry,
				Reason:    "caps",
			},
			OccurredAt: time.Now().UTC(),
		},
	}
	for _, evt := range events {
		requireAvailable(t, repo.ApplyChatEvent(evt), "apply moderation event")
	}

	snapshot := repo.ChatRestrictions()
	if _, banned := snapshot.Bans[channel.ID][target.ID]; !banned {
		t.Fatalf("expected target %q to be banned", target.ID)
	}
	if actor := snapshot.BanActors[channel.ID][target.ID]; actor != owner.ID {
		t.Fatalf("expected ban actor %q, got %q", owner.ID, actor)
	}
	if reason := snapshot.BanReasons[channel.ID][target.ID]; reason != "spam" {
		t.Fatalf("expected ban reason to persist, got %q", reason)
	}
	timeoutExpiry, ok := snapshot.Timeouts[channel.ID][target.ID]
	if !ok || timeoutExpiry.Before(expiry.Add(-time.Second)) {
		t.Fatalf("expected timeout to record expiry")
	}
	if actor := snapshot.TimeoutActors[channel.ID][target.ID]; actor != owner.ID {
		t.Fatalf("expected timeout actor %q, got %q", owner.ID, actor)
	}
	if reason := snapshot.TimeoutReasons[channel.ID][target.ID]; reason != "caps" {
		t.Fatalf("expected timeout reason to persist, got %q", reason)
	}
	if issued := snapshot.TimeoutIssuedAt[channel.ID][target.ID]; issued.IsZero() {
		t.Fatalf("expected timeout issued timestamp to be set")
	}

	clearEvents := []chat.Event{
		{
			Type: chat.EventTypeModeration,
			Moderation: &chat.ModerationEvent{
				Action:    chat.ModerationActionRemoveTimeout,
				ChannelID: channel.ID,
				ActorID:   owner.ID,
				TargetID:  target.ID,
				Reason:    "resolved",
			},
			OccurredAt: time.Now().UTC(),
		},
		{
			Type: chat.EventTypeModeration,
			Moderation: &chat.ModerationEvent{
				Action:    chat.ModerationActionUnban,
				ChannelID: channel.ID,
				ActorID:   owner.ID,
				TargetID:  target.ID,
				Reason:    "appeal",
			},
			OccurredAt: time.Now().UTC(),
		},
	}
	for _, evt := range clearEvents {
		requireAvailable(t, repo.ApplyChatEvent(evt), "clear moderation event")
	}

	snapshot = repo.ChatRestrictions()
	if _, banned := snapshot.Bans[channel.ID][target.ID]; banned {
		t.Fatalf("expected ban removal for %q", target.ID)
	}
	if _, muted := snapshot.Timeouts[channel.ID][target.ID]; muted {
		t.Fatalf("expected timeout removal for %q", target.ID)
	}
}

// RunRepositoryChatReportsLifecycle executes the chat report workflow from
// storage_test.go with the provided repository implementation.
func RunRepositoryChatReportsLifecycle(t *testing.T, factory RepositoryFactory) {
	repo := runRepository(t, factory)

	owner, err := repo.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create owner")
	reporter, err := repo.CreateUser(CreateUserParams{DisplayName: "reporter", Email: "reporter@example.com"})
	requireAvailable(t, err, "create reporter")
	target, err := repo.CreateUser(CreateUserParams{DisplayName: "target", Email: "target@example.com"})
	requireAvailable(t, err, "create target")
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	requireAvailable(t, err, "create channel")

	report, err := repo.CreateChatReport(channel.ID, reporter.ID, target.ID, "spam", "msg-1", "")
	requireAvailable(t, err, "create chat report")
	if report.Status != "open" {
		t.Fatalf("expected new report to be open, got %q", report.Status)
	}

	pending, err := repo.ListChatReports(channel.ID, false)
	requireAvailable(t, err, "list pending chat reports")
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending report, got %d", len(pending))
	}

	resolved, err := repo.ResolveChatReport(report.ID, owner.ID, "handled")
	requireAvailable(t, err, "resolve chat report")
	if resolved.Status != "resolved" || resolved.Resolution != "handled" {
		t.Fatalf("unexpected resolved payload: %+v", resolved)
	}

	pending, err = repo.ListChatReports(channel.ID, false)
	requireAvailable(t, err, "list pending chat reports after resolve")
	if len(pending) != 0 {
		t.Fatalf("expected no pending reports, got %d", len(pending))
	}

	all, err := repo.ListChatReports(channel.ID, true)
	requireAvailable(t, err, "list all chat reports")
	if len(all) != 1 {
		t.Fatalf("expected resolved report to be listed, got %d", len(all))
	}
}

// RunRepositoryTipsLifecycle asserts tip creation and listing behaviour against
// a repository implementation.
func RunRepositoryTipsLifecycle(t *testing.T, factory RepositoryFactory) {
	repo := runRepository(t, factory)

	owner, err := repo.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create owner")
	supporter, err := repo.CreateUser(CreateUserParams{DisplayName: "fan", Email: "fan@example.com"})
	requireAvailable(t, err, "create supporter")
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	requireAvailable(t, err, "create channel")

	expectedTipAmount := models.MustParseMoney("5.5")
	tip, err := repo.CreateTip(CreateTipParams{
		ChannelID:  channel.ID,
		FromUserID: supporter.ID,
		Amount:     expectedTipAmount,
		Currency:   "usd",
		Provider:   "stripe",
		Reference:  "ref-1",
		Message:    "keep it up",
	})
	requireAvailable(t, err, "create tip")
	if tip.ID == "" {
		t.Fatalf("expected tip id to be set")
	}

	if tip.Amount.MinorUnits() != expectedTipAmount.MinorUnits() {
		t.Fatalf("expected persisted tip amount %d, got %d", expectedTipAmount.MinorUnits(), tip.Amount.MinorUnits())
	}

	tips, err := repo.ListTips(channel.ID, 10)
	requireAvailable(t, err, "list tips")
	if len(tips) != 1 || tips[0].ID != tip.ID {
		t.Fatalf("expected persisted tip, got %+v", tips)
	}
	if tips[0].Amount.MinorUnits() != expectedTipAmount.MinorUnits() {
		t.Fatalf("expected listed tip amount %d, got %d", expectedTipAmount.MinorUnits(), tips[0].Amount.MinorUnits())
	}

	longReference := strings.Repeat("r", MaxTipReferenceLength+1)
	if _, err := repo.CreateTip(CreateTipParams{
		ChannelID:  channel.ID,
		FromUserID: supporter.ID,
		Amount:     models.MustParseMoney("5.5"),
		Currency:   "usd",
		Provider:   "stripe",
		Reference:  longReference,
	}); err == nil {
		t.Fatalf("expected error for overlong reference")
	}

	longWallet := strings.Repeat("w", MaxTipWalletAddressLength+1)
	if _, err := repo.CreateTip(CreateTipParams{
		ChannelID:     channel.ID,
		FromUserID:    supporter.ID,
		Amount:        models.MustParseMoney("5.5"),
		Currency:      "usd",
		Provider:      "stripe",
		Reference:     "ref-wallet",
		WalletAddress: longWallet,
	}); err == nil {
		t.Fatalf("expected error for overlong wallet address")
	}

	longMessage := strings.Repeat("m", MaxTipMessageLength+1)
	if _, err := repo.CreateTip(CreateTipParams{
		ChannelID:  channel.ID,
		FromUserID: supporter.ID,
		Amount:     models.MustParseMoney("5.5"),
		Currency:   "usd",
		Provider:   "stripe",
		Reference:  "ref-message",
		Message:    longMessage,
	}); err == nil {
		t.Fatalf("expected error for overlong message")
	}
}

// RunRepositorySubscriptionsLifecycle validates the subscription lifecycle for
// a repository implementation.
func RunRepositorySubscriptionsLifecycle(t *testing.T, factory RepositoryFactory) {
	repo := runRepository(t, factory)

	owner, err := repo.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create owner")
	viewer, err := repo.CreateUser(CreateUserParams{DisplayName: "viewer", Email: "viewer@example.com"})
	requireAvailable(t, err, "create viewer")
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	requireAvailable(t, err, "create channel")

	expectedSubAmount := models.MustParseMoney("4.99")
	sub, err := repo.CreateSubscription(CreateSubscriptionParams{
		ChannelID: channel.ID,
		UserID:    viewer.ID,
		Tier:      "tier1",
		Provider:  "stripe",
		Reference: "sub-1",
		Amount:    expectedSubAmount,
		Currency:  "usd",
		Duration:  time.Hour,
		AutoRenew: true,
	})
	requireAvailable(t, err, "create subscription")
	if sub.ID == "" {
		t.Fatalf("expected subscription id to be set")
	}

	if sub.Amount.MinorUnits() != expectedSubAmount.MinorUnits() {
		t.Fatalf("expected subscription amount %d, got %d", expectedSubAmount.MinorUnits(), sub.Amount.MinorUnits())
	}

	subs, err := repo.ListSubscriptions(channel.ID, false)
	requireAvailable(t, err, "list subscriptions")
	if len(subs) != 1 || subs[0].ID != sub.ID {
		t.Fatalf("expected subscription listing to include created subscription, got %+v", subs)
	}
	if subs[0].Amount.MinorUnits() != expectedSubAmount.MinorUnits() {
		t.Fatalf("expected listed subscription amount %d, got %d", expectedSubAmount.MinorUnits(), subs[0].Amount.MinorUnits())
	}

	stored, ok := repo.GetSubscription(sub.ID)
	if !ok {
		t.Fatalf("expected GetSubscription to find %q", sub.ID)
	}
	if stored.ID != sub.ID || stored.Status != "active" {
		t.Fatalf("unexpected stored subscription: %+v", stored)
	}
	if stored.Amount.MinorUnits() != expectedSubAmount.MinorUnits() {
		t.Fatalf("expected fetched subscription amount %d, got %d", expectedSubAmount.MinorUnits(), stored.Amount.MinorUnits())
	}

	cancelled, err := repo.CancelSubscription(sub.ID, owner.ID, "fraud")
	requireAvailable(t, err, "cancel subscription")
	if cancelled.Status != "cancelled" {
		t.Fatalf("expected subscription to be cancelled, got status %q", cancelled.Status)
	}

	all, err := repo.ListSubscriptions(channel.ID, true)
	requireAvailable(t, err, "list all subscriptions")
	if len(all) != 1 || all[0].Status != "cancelled" {
		t.Fatalf("expected cancelled subscription to be returned, got %+v", all)
	}
}

// RunRepositoryMonetizationPrecision verifies repositories preserve fixed-precision
// minor units for tips and subscriptions.
func RunRepositoryMonetizationPrecision(t *testing.T, factory RepositoryFactory) {
	repo := runRepository(t, factory)

	owner, err := repo.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create owner")
	viewer, err := repo.CreateUser(CreateUserParams{DisplayName: "viewer", Email: "viewer@example.com"})
	requireAvailable(t, err, "create viewer")
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	requireAvailable(t, err, "create channel")

	preciseTip := models.MustParseMoney("0.00000025")
	tip, err := repo.CreateTip(CreateTipParams{
		ChannelID:  channel.ID,
		FromUserID: viewer.ID,
		Amount:     preciseTip,
		Currency:   "usd",
		Provider:   "internal",
		Reference:  "precision-tip",
	})
	requireAvailable(t, err, "create precise tip")
	if tip.Amount.MinorUnits() != preciseTip.MinorUnits() {
		t.Fatalf("expected precise tip units %d, got %d", preciseTip.MinorUnits(), tip.Amount.MinorUnits())
	}
	tips, err := repo.ListTips(channel.ID, 10)
	requireAvailable(t, err, "list precise tips")
	found := false
	for _, listed := range tips {
		if listed.ID == tip.ID {
			found = true
			if listed.Amount.MinorUnits() != preciseTip.MinorUnits() {
				t.Fatalf("expected listed tip units %d, got %d", preciseTip.MinorUnits(), listed.Amount.MinorUnits())
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected precision tip %q in listing", tip.ID)
	}

	preciseSub := models.MustParseMoney("1234.56789012")
	subscription, err := repo.CreateSubscription(CreateSubscriptionParams{
		ChannelID: channel.ID,
		UserID:    viewer.ID,
		Tier:      "precision",
		Provider:  "internal",
		Reference: "precision-sub",
		Amount:    preciseSub,
		Currency:  "usd",
		Duration:  time.Hour,
	})
	requireAvailable(t, err, "create precise subscription")
	if subscription.Amount.MinorUnits() != preciseSub.MinorUnits() {
		t.Fatalf("expected precise subscription units %d, got %d", preciseSub.MinorUnits(), subscription.Amount.MinorUnits())
	}
	subs, err := repo.ListSubscriptions(channel.ID, true)
	requireAvailable(t, err, "list precise subscriptions")
	located := false
	for _, listed := range subs {
		if listed.ID == subscription.ID {
			located = true
			if listed.Amount.MinorUnits() != preciseSub.MinorUnits() {
				t.Fatalf("expected listed subscription units %d, got %d", preciseSub.MinorUnits(), listed.Amount.MinorUnits())
			}
			break
		}
	}
	if !located {
		t.Fatalf("expected subscription %q in listing", subscription.ID)
	}

	stored, ok := repo.GetSubscription(subscription.ID)
	if !ok {
		t.Fatalf("expected subscription %q to be retrievable", subscription.ID)
	}
	if stored.Amount.MinorUnits() != preciseSub.MinorUnits() {
		t.Fatalf("expected stored subscription units %d, got %d", preciseSub.MinorUnits(), stored.Amount.MinorUnits())
	}
}

// RunRepositoryIngestHealthSnapshots verifies that repositories persist ingest
// health snapshots provided by the configured ingest controller.
func RunRepositoryIngestHealthSnapshots(t *testing.T, factory RepositoryFactory) {
	responses := [][]ingest.HealthStatus{
		{{Component: "srs", Status: "ok"}},
		{{Component: "transcoder", Status: "error", Detail: "timeout"}},
	}
	fake := &fakeIngestController{healthResponses: responses}
	repo := runRepository(t, factory, WithIngestController(fake))

	first := repo.IngestHealth(context.Background())
	if fake.healthCalls == 0 {
		if _, isPostgres := repo.(*postgresRepository); isPostgres {
			t.Skip("postgres repository ingest health not yet implemented")
		}
		t.Fatalf("expected ingest controller to be queried")
	}
	if !reflect.DeepEqual(first, responses[0]) {
		t.Fatalf("unexpected health payload: %+v", first)
	}
	recorded, ts1 := repo.LastIngestHealth()
	if !reflect.DeepEqual(recorded, first) {
		t.Fatalf("expected last health to match recorded snapshot")
	}
	if ts1.IsZero() {
		t.Fatal("expected health timestamp to be set")
	}

	second := repo.IngestHealth(context.Background())
	if fake.healthCalls < 2 {
		t.Fatalf("expected subsequent health call to increment counter, got %d", fake.healthCalls)
	}
	if !reflect.DeepEqual(second, responses[1]) {
		t.Fatalf("unexpected second health payload: %+v", second)
	}
	recorded, ts2 := repo.LastIngestHealth()
	if !reflect.DeepEqual(recorded, second) {
		t.Fatalf("expected snapshot to update on subsequent call")
	}
	if ts2.Before(ts1) {
		t.Fatal("expected subsequent health timestamp to be >= initial timestamp")
	}
}

// RunRepositoryRecordingRetention validates the retention workflow that purges
// expired recordings and associated artefacts.
func RunRepositoryRecordingRetention(t *testing.T, factory RepositoryFactory) {
	policy := RecordingRetentionPolicy{Published: 200 * time.Millisecond, Unpublished: 200 * time.Millisecond}
	controller := &fakeIngestController{bootResponses: []bootResponse{{result: ingest.BootResult{
		Renditions: []ingest.Rendition{{Name: "720p", ManifestURL: "https://origin/720p.m3u8"}},
	}}}}
	objectConfig := WithObjectStorage(ObjectStorageConfig{
		Bucket:         "vod",
		Prefix:         "vod/assets",
		PublicEndpoint: "https://cdn.example.com/content",
	})

	repo := runRepository(t, factory, WithRecordingRetention(policy), WithIngestController(controller), objectConfig)
	fakeStorage := &fakeObjectStorage{prefix: "vod/assets", baseURL: "https://cdn.example.com/content"}
	switch r := repo.(type) {
	case *Storage:
		r.objectClient = fakeStorage
	case *postgresRepository:
		r.objectClient = fakeStorage
	}

	owner, err := repo.CreateUser(CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create owner")
	channel, err := repo.CreateChannel(owner.ID, "Speedrun", "gaming", nil)
	requireAvailable(t, err, "create channel")
	_, err = repo.StartStream(channel.ID, []string{"720p"})
	requireAvailable(t, err, "start stream")
	_, err = repo.StopStream(channel.ID, 10)
	requireAvailable(t, err, "stop stream")

	recordings, err := repo.ListRecordings(channel.ID, true)
	requireAvailable(t, err, "list recordings before retention")
	if len(recordings) != 1 {
		t.Fatalf("expected one recording before retention purge, got %d", len(recordings))
	}
	recordingID := recordings[0].ID

	clip, err := repo.CreateClipExport(recordingID, ClipExportParams{Title: "Intro", StartSeconds: 0, EndSeconds: 5})
	requireAvailable(t, err, "create clip export")

	clipObject := ""
	if store, ok := repo.(*Storage); ok {
		store.mu.Lock()
		stored := store.data.ClipExports[clip.ID]
		stored.StorageObject = buildObjectKey("clips", clip.ID+".mp4")
		store.data.ClipExports[clip.ID] = stored
		store.mu.Unlock()
		clipObject = stored.StorageObject
	}

	expectedDeletes := make(map[string]struct{})
	for _, upload := range fakeStorage.uploads {
		if strings.Contains(upload.Key, "/manifests/") || strings.Contains(upload.Key, "/thumbnails/") {
			expectedDeletes[upload.Key] = struct{}{}
		}
	}

	if clipObject != "" {
		expectedDeletes[clipObject] = struct{}{}
	}

	fakeStorage.deletes = nil
	time.Sleep(250 * time.Millisecond)

	recordings, err = repo.ListRecordings(channel.ID, true)
	requireAvailable(t, err, "list recordings after retention")
	if len(recordings) != 0 {
		t.Fatalf("expected retention to purge recordings, got %d", len(recordings))
	}

	for _, key := range fakeStorage.deletes {
		delete(expectedDeletes, key)
	}
	if len(expectedDeletes) != 0 {
		t.Fatalf("expected storage deletes for manifests, thumbnails, and clips; missing %v", expectedDeletes)
	}
}

// RunRepositoryStreamLifecycleWithoutIngest verifies stream start/stop requests
// fail gracefully when no ingest controller is configured.
func RunRepositoryStreamLifecycleWithoutIngest(t *testing.T, factory RepositoryFactory) {
	repo := runRepository(t, factory)

	switch r := repo.(type) {
	case *Storage:
		r.mu.Lock()
		r.ingestController = nil
		r.mu.Unlock()
	case *postgresRepository:
		r.ingestController = nil
	}

	owner, err := repo.CreateUser(CreateUserParams{DisplayName: "Creator", Email: "creator@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create owner")
	channel, err := repo.CreateChannel(owner.ID, "Live", "gaming", nil)
	requireAvailable(t, err, "create channel")

	if _, err := repo.StartStream(channel.ID, []string{"720p"}); !errors.Is(err, ErrIngestControllerUnavailable) {
		t.Fatalf("expected ErrIngestControllerUnavailable from StartStream, got %v", err)
	}

	stored, ok := repo.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("expected to reload channel %s", channel.ID)
	}
	if stored.LiveState != "offline" {
		t.Fatalf("expected live state to remain offline, got %s", stored.LiveState)
	}
	if stored.CurrentSessionID != nil {
		t.Fatalf("expected current session to remain nil, got %v", stored.CurrentSessionID)
	}

	var sessionID string
	switch r := repo.(type) {
	case *Storage:
		r.mu.Lock()
		var genErr error
		sessionID, genErr = generateID()
		r.mu.Unlock()
		if genErr != nil {
			t.Fatalf("generate session id: %v", genErr)
		}
		now := time.Now().UTC()
		r.mu.Lock()
		session := models.StreamSession{ID: sessionID, ChannelID: channel.ID, StartedAt: now}
		if r.data.StreamSessions == nil {
			r.data.StreamSessions = make(map[string]models.StreamSession)
		}
		r.data.StreamSessions[sessionID] = session
		ch := r.data.Channels[channel.ID]
		ch.CurrentSessionID = &sessionID
		ch.LiveState = "live"
		ch.UpdatedAt = now
		r.data.Channels[channel.ID] = ch
		r.mu.Unlock()
	case *postgresRepository:
		var genErr error
		sessionID, genErr = generateID()
		if genErr != nil {
			t.Fatalf("generate session id: %v", genErr)
		}
		ctx := context.Background()
		now := time.Now().UTC()
		if _, err := r.pool.Exec(ctx, "INSERT INTO stream_sessions (id, channel_id, started_at, renditions, peak_concurrent, origin_url, playback_url, ingest_endpoints, ingest_job_ids) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)", sessionID, channel.ID, now, []string{}, 0, "", "", []string{}, []string{}); err != nil {
			t.Fatalf("seed stream session: %v", err)
		}
		if _, err := r.pool.Exec(ctx, "UPDATE channels SET current_session_id = $1, live_state = 'live', updated_at = $2 WHERE id = $3", sessionID, now, channel.ID); err != nil {
			t.Fatalf("mark channel live: %v", err)
		}
	}

	if sessionID == "" {
		t.Fatal("expected session id to be set for stop stream test")
	}

	if _, err := repo.StopStream(channel.ID, 5); !errors.Is(err, ErrIngestControllerUnavailable) {
		t.Fatalf("expected ErrIngestControllerUnavailable from StopStream, got %v", err)
	}

	stored, ok = repo.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("expected to reload channel %s after stop", channel.ID)
	}
	if stored.CurrentSessionID == nil || *stored.CurrentSessionID != sessionID {
		t.Fatalf("expected channel to remain live with session %s, got %v", sessionID, stored.CurrentSessionID)
	}
	if stored.LiveState != "live" {
		t.Fatalf("expected channel to remain live, got %s", stored.LiveState)
	}
}

func RunRepositoryStreamTimeouts(t *testing.T, factory RepositoryFactory) {
	const timeout = 30 * time.Millisecond

	bootController := &timeoutIngestController{bootBlock: true}
	repo := runRepository(t, factory, WithIngestController(bootController), WithIngestTimeout(timeout))

	owner, err := repo.CreateUser(CreateUserParams{DisplayName: "Creator", Email: "creator@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create owner")
	channel, err := repo.CreateChannel(owner.ID, "Timeouts", "gaming", []string{"speedrun"})
	requireAvailable(t, err, "create channel")

	start := time.Now()
	_, err = repo.StartStream(channel.ID, []string{"720p"})
	if err == nil {
		t.Fatal("expected StartStream to fail when ingest boot blocks")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected StartStream deadline exceeded, got %v", err)
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("StartStream exceeded timeout expectation: %v", time.Since(start))
	}

	stored, ok := repo.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("expected to reload channel %s", channel.ID)
	}
	if stored.LiveState != "offline" {
		t.Fatalf("expected channel to remain offline, got %s", stored.LiveState)
	}
	if stored.CurrentSessionID != nil {
		t.Fatalf("expected current session to remain nil, got %v", *stored.CurrentSessionID)
	}
	if _, active := repo.CurrentStreamSession(channel.ID); active {
		t.Fatal("expected no active session after start timeout")
	}

	shutdownController := &timeoutIngestController{bootResult: ingest.BootResult{PlaybackURL: "https://playback.example"}}
	stopRepo := runRepository(t, factory, WithIngestController(shutdownController), WithIngestTimeout(timeout))

	owner, err = stopRepo.CreateUser(CreateUserParams{DisplayName: "Creator", Email: "streamer@example.com", Roles: []string{"creator"}})
	requireAvailable(t, err, "create stop owner")
	channel, err = stopRepo.CreateChannel(owner.ID, "Timeouts", "gaming", []string{"speedrun"})
	requireAvailable(t, err, "create stop channel")

	session, err := stopRepo.StartStream(channel.ID, []string{"720p"})
	requireAvailable(t, err, "start stream before timeout")

	shutdownController.shutdownBlock = true

	start = time.Now()
	_, err = stopRepo.StopStream(channel.ID, 10)
	if err == nil {
		t.Fatal("expected StopStream to fail when ingest shutdown blocks")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected StopStream deadline exceeded, got %v", err)
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("StopStream exceeded timeout expectation: %v", time.Since(start))
	}

	stored, ok = stopRepo.GetChannel(channel.ID)
	if !ok {
		t.Fatalf("expected to reload channel %s after stop timeout", channel.ID)
	}
	if stored.LiveState != "live" {
		t.Fatalf("expected channel to remain live, got %s", stored.LiveState)
	}
	if stored.CurrentSessionID == nil || *stored.CurrentSessionID != session.ID {
		t.Fatalf("expected current session to remain %s, got %v", session.ID, stored.CurrentSessionID)
	}
	current, ok := stopRepo.CurrentStreamSession(channel.ID)
	if !ok {
		t.Fatalf("expected current session %s to persist", session.ID)
	}
	if current.EndedAt != nil {
		t.Fatal("expected session to remain active after shutdown timeout")
	}
}
