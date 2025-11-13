//go:build postgres

package storage_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
	"unsafe"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
	"bitriver-live/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

// postgresRepositoryFactory opens a Postgres-backed repository for integration
// scenarios, applying migrations and ensuring tables are truncated between
// tests. The factory requires BITRIVER_TEST_POSTGRES_DSN to point at a clean
// database dedicated to automated runs.
func postgresRepositoryFactory(t *testing.T, opts ...storage.Option) (storage.Repository, func(), error) {
	t.Helper()
	dsn := os.Getenv("BITRIVER_TEST_POSTGRES_DSN")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("BITRIVER_TEST_POSTGRES_DSN not set")
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse postgres config: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("open postgres pool: %v", err)
	}

	applyPostgresMigrations(t, ctx, pool)
	if err := truncatePostgresTables(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("truncate tables: %v", err)
	}

	defaults := []storage.Option{storage.WithIngestController(ingest.NoopController{}), storage.WithIngestRetries(1, 0)}
	opts = append(defaults, opts...)
	repo, err := storage.NewPostgresRepository(dsn, opts...)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}

	t.Cleanup(func() {
		if err := truncatePostgresTables(context.Background(), pool); err != nil {
			t.Fatalf("truncate tables: %v", err)
		}
	})
	t.Cleanup(func() {
		switch closer := repo.(type) {
		case interface{ Close(context.Context) error }:
			if err := closer.Close(context.Background()); err != nil {
				t.Fatalf("close repository: %v", err)
			}
		case interface{ Close() error }:
			if err := closer.Close(); err != nil {
				t.Fatalf("close repository: %v", err)
			}
		}
	})
	t.Cleanup(func() { pool.Close() })

	return repo, nil, nil
}

func postgresPoolFromRepository(t *testing.T, repo storage.Repository) *pgxpool.Pool {
	t.Helper()
	val := reflect.ValueOf(repo)
	if val.Kind() != reflect.Pointer {
		t.Fatalf("expected repository pointer, got %T", repo)
	}
	elem := val.Elem()
	field := elem.FieldByName("pool")
	if !field.IsValid() {
		t.Fatal("postgres repository pool field missing")
	}
	if field.IsNil() {
		t.Fatal("postgres repository pool is nil")
	}
	return (*pgxpool.Pool)(unsafe.Pointer(field.UnsafePointer()))
}

func openPostgresRepository(t *testing.T) storage.Repository {
	t.Helper()
	repo, _, err := postgresRepositoryFactory(t)
	if errors.Is(err, storage.ErrPostgresUnavailable) {
		t.Skip("postgres repository unavailable in this build")
	}
	if err != nil {
		t.Fatalf("failed to open postgres repository: %v", err)
	}
	if repo == nil {
		t.Fatal("expected postgres repository instance")
	}
	return repo
}

func TestPostgresRepositoryConnection(t *testing.T) {
	repo, _, err := postgresRepositoryFactory(t)
	if errors.Is(err, storage.ErrPostgresUnavailable) {
		t.Skip("postgres repository unavailable in this build")
	}
	if err != nil {
		t.Fatalf("failed to open postgres repository: %v", err)
	}
	if repo == nil {
		t.Fatalf("expected postgres repository instance")
	}
}

func TestPostgresRepositoryAcquireTimeoutCoversQueries(t *testing.T) {
	repo, cleanup, err := postgresRepositoryFactory(t,
		storage.WithPostgresAcquireTimeout(50*time.Millisecond),
	)
	if errors.Is(err, storage.ErrPostgresUnavailable) {
		t.Skip("postgres repository unavailable in this build")
	}
	if err != nil {
		t.Fatalf("failed to open postgres repository: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	type withConnInvoker interface {
		withConn(func(context.Context, *pgxpool.Conn) error) error
	}

	invoker, ok := repo.(withConnInvoker)
	if !ok {
		t.Fatalf("expected postgres repository implementation, got %T", repo)
	}

	start := time.Now()
	err = invoker.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		_, execErr := conn.Exec(ctx, "SELECT pg_sleep(0.1)")
		return execErr
	})
	if err == nil {
		t.Fatal("expected query to fail due to context deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded; got %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("query exceeded expected timeout: %v", time.Since(start))
	}
}

func TestPostgresUserLifecycle(t *testing.T) {
	storage.RunRepositoryUserLifecycle(t, postgresRepositoryFactory)
}

func TestPostgresChatRestrictionsLifecycle(t *testing.T) {
	storage.RunRepositoryChatRestrictionsLifecycle(t, postgresRepositoryFactory)
}

func TestPostgresChatReportsLifecycle(t *testing.T) {
	storage.RunRepositoryChatReportsLifecycle(t, postgresRepositoryFactory)
}

func TestPostgresChannelSearch(t *testing.T) {
	storage.RunRepositoryChannelSearch(t, postgresRepositoryFactory)
}

func TestPostgresSetUserPassword(t *testing.T) {
	repo := openPostgresRepository(t)

	email := "admin@example.com"
	original := "initialP@ss"
	user, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "Admin", Email: email, Password: original})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}

	if _, err := repo.AuthenticateUser(email, original); err != nil {
		t.Fatalf("authenticate original password: %v", err)
	}

	updated, err := repo.SetUserPassword(user.ID, "Sup3rSecret!")
	if err != nil {
		t.Fatalf("set user password: %v", err)
	}
	if updated.PasswordHash == "" {
		t.Fatalf("expected password hash to be set")
	}

	if _, err := repo.AuthenticateUser(email, original); !errors.Is(err, storage.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials for old password, got %v", err)
	}
	if _, err := repo.AuthenticateUser(email, "Sup3rSecret!"); err != nil {
		t.Fatalf("authenticate with new password: %v", err)
	}
}

func TestPostgresChatMessageLifecycle(t *testing.T) {
	repo := openPostgresRepository(t)

	owner, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	message, err := repo.CreateChatMessage(channel.ID, owner.ID, "hello world")
	if err != nil {
		t.Fatalf("create chat message: %v", err)
	}
	if message.ID == "" {
		t.Fatalf("expected message id to be set")
	}

	history, err := repo.ListChatMessages(channel.ID, 0)
	if err != nil {
		t.Fatalf("list chat messages: %v", err)
	}
	if len(history) != 1 || history[0].ID != message.ID {
		t.Fatalf("expected stored message in history, got %+v", history)
	}

	if err := repo.DeleteChatMessage(channel.ID, message.ID); err != nil {
		t.Fatalf("delete chat message: %v", err)
	}
	history, err = repo.ListChatMessages(channel.ID, 0)
	if err != nil {
		t.Fatalf("list chat messages after delete: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("expected empty history after deletion, got %d", len(history))
	}
}

func TestPostgresOAuthLinking(t *testing.T) {
	storage.RunRepositoryOAuthLinking(t, postgresRepositoryFactory)
}

func TestPostgresChatMessageHistoryPaging(t *testing.T) {
	repo := openPostgresRepository(t)

	owner, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	msg1, err := repo.CreateChatMessage(channel.ID, owner.ID, "first")
	if err != nil {
		t.Fatalf("create chat message #1: %v", err)
	}
	msg2, err := repo.CreateChatMessage(channel.ID, owner.ID, "second")
	if err != nil {
		t.Fatalf("create chat message #2: %v", err)
	}
	msg3, err := repo.CreateChatMessage(channel.ID, owner.ID, "third")
	if err != nil {
		t.Fatalf("create chat message #3: %v", err)
	}

	history, err := repo.ListChatMessages(channel.ID, 0)
	if err != nil {
		t.Fatalf("list chat messages: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(history))
	}
	if history[0].ID != msg3.ID || history[1].ID != msg2.ID || history[2].ID != msg1.ID {
		t.Fatalf("unexpected message ordering: %v", history)
	}

	paged, err := repo.ListChatMessages(channel.ID, 2)
	if err != nil {
		t.Fatalf("list chat messages with limit: %v", err)
	}
	if len(paged) != 2 || paged[0].ID != msg3.ID || paged[1].ID != msg2.ID {
		t.Fatalf("unexpected paged results: %v", paged)
	}
}

func TestPostgresReadHelpersRespectAcquireTimeout(t *testing.T) {
	repo, cleanup, err := postgresRepositoryFactory(t,
		storage.WithPostgresPoolLimits(1, 1),
		storage.WithPostgresAcquireTimeout(50*time.Millisecond),
	)
	if errors.Is(err, storage.ErrPostgresUnavailable) {
		t.Skip("postgres repository unavailable in this build")
	}
	if err != nil {
		t.Fatalf("failed to open postgres repository: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	owner, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "owner", Email: "owner-timeout@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	target, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "target", Email: "target-timeout@example.com"})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	channel, err := repo.CreateChannel(owner.ID, "Timeout Lobby", "gaming", nil)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	msg, err := repo.CreateChatMessage(channel.ID, owner.ID, "ping")
	if err != nil {
		t.Fatalf("create chat message: %v", err)
	}
	banEvent := chat.Event{
		Type: chat.EventTypeModeration,
		Moderation: &chat.ModerationEvent{
			Action:    chat.ModerationActionBan,
			ChannelID: channel.ID,
			ActorID:   owner.ID,
			TargetID:  target.ID,
			Reason:    "timeout-check",
		},
		OccurredAt: time.Now().UTC(),
	}
	if err := repo.ApplyChatEvent(banEvent); err != nil {
		t.Fatalf("apply ban event: %v", err)
	}

	pool := postgresPoolFromRepository(t, repo)
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire pool connection: %v", err)
	}
	defer func() {
		if conn != nil {
			conn.Release()
		}
	}()

	deadline := 250 * time.Millisecond
	expectQuick := func(name string, fn func() error) {
		t.Helper()
		done := make(chan error, 1)
		go func() {
			done <- fn()
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("%s failed: %v", name, err)
			}
		case <-time.After(deadline):
			t.Fatalf("%s did not respect acquire timeout", name)
		}
	}

	expectQuick("ListChannels", func() error {
		if channels := repo.ListChannels("", ""); channels != nil {
			return fmt.Errorf("expected nil channel list while pool is exhausted, got %d", len(channels))
		}
		return nil
	})

	expectQuick("ListChatMessages", func() error {
		_, err := repo.ListChatMessages(channel.ID, 0)
		if err == nil {
			return fmt.Errorf("expected chat message error while pool is exhausted")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("expected deadline exceeded, got %v", err)
		}
		return nil
	})

	expectQuick("ChatRestrictions", func() error {
		snapshot := repo.ChatRestrictions()
		if len(snapshot.Bans[channel.ID]) != 0 {
			return fmt.Errorf("expected no bans returned while pool is exhausted")
		}
		return nil
	})

	expectQuick("IsChatBanned", func() error {
		if repo.IsChatBanned(channel.ID, target.ID) {
			return fmt.Errorf("expected ban lookup to fail while pool is exhausted")
		}
		return nil
	})

	conn.Release()
	conn = nil

	channels := repo.ListChannels("", "")
	if len(channels) != 1 || channels[0].ID != channel.ID {
		t.Fatalf("expected channel to be listed after releasing pool connection, got %+v", channels)
	}

	history, err := repo.ListChatMessages(channel.ID, 0)
	if err != nil {
		t.Fatalf("list chat messages after releasing connection: %v", err)
	}
	if len(history) != 1 || history[0].ID != msg.ID {
		t.Fatalf("expected stored message after releasing connection, got %+v", history)
	}

	snapshot := repo.ChatRestrictions()
	if _, ok := snapshot.Bans[channel.ID][target.ID]; !ok {
		t.Fatalf("expected ban to be visible after releasing connection")
	}

	if !repo.IsChatBanned(channel.ID, target.ID) {
		t.Fatalf("expected ban lookup to succeed after releasing connection")
	}
}

func TestPostgresChatBanTimeoutLifecycle(t *testing.T) {
	repo := openPostgresRepository(t)

	owner, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	target, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "target", Email: "target@example.com"})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	expiry := time.Now().UTC().Add(5 * time.Minute)
	timeoutEvent := chat.Event{
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
	}
	if err := repo.ApplyChatEvent(timeoutEvent); err != nil {
		t.Fatalf("apply timeout event: %v", err)
	}

	if _, err := repo.CreateChatMessage(channel.ID, target.ID, "should fail"); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}

	banEvent := chat.Event{
		Type: chat.EventTypeModeration,
		Moderation: &chat.ModerationEvent{
			Action:    chat.ModerationActionBan,
			ChannelID: channel.ID,
			ActorID:   owner.ID,
			TargetID:  target.ID,
			Reason:    "spam",
		},
		OccurredAt: time.Now().UTC(),
	}
	if err := repo.ApplyChatEvent(banEvent); err != nil {
		t.Fatalf("apply ban event: %v", err)
	}

	if _, err := repo.CreateChatMessage(channel.ID, target.ID, "still banned"); err == nil || !strings.Contains(err.Error(), "banned") {
		t.Fatalf("expected ban error, got %v", err)
	}

	if !repo.IsChatBanned(channel.ID, target.ID) {
		t.Fatalf("expected user to be banned")
	}
	if timeout, ok := repo.ChatTimeout(channel.ID, target.ID); !ok || timeout.Before(expiry.Add(-time.Second)) {
		t.Fatalf("expected timeout expiry to be recorded")
	}

	restrictions := repo.ListChatRestrictions(channel.ID)
	if len(restrictions) != 2 {
		t.Fatalf("expected ban and timeout restrictions, got %v", restrictions)
	}

	snapshot := repo.ChatRestrictions()
	if _, ok := snapshot.Bans[channel.ID][target.ID]; !ok {
		t.Fatalf("expected ban snapshot entry")
	}
	if actor := snapshot.BanActors[channel.ID][target.ID]; actor != owner.ID {
		t.Fatalf("expected ban actor %q, got %q", owner.ID, actor)
	}
	if reason := snapshot.BanReasons[channel.ID][target.ID]; reason != "spam" {
		t.Fatalf("expected ban reason 'spam', got %q", reason)
	}
	timeoutExpiry, ok := snapshot.Timeouts[channel.ID][target.ID]
	if !ok || timeoutExpiry.Before(expiry.Add(-time.Second)) {
		t.Fatalf("expected timeout snapshot entry")
	}
	if actor := snapshot.TimeoutActors[channel.ID][target.ID]; actor != owner.ID {
		t.Fatalf("expected timeout actor %q, got %q", owner.ID, actor)
	}
	if reason := snapshot.TimeoutReasons[channel.ID][target.ID]; reason != "caps" {
		t.Fatalf("expected timeout reason 'caps', got %q", reason)
	}
	if issued := snapshot.TimeoutIssuedAt[channel.ID][target.ID]; issued.IsZero() {
		t.Fatalf("expected timeout issued timestamp to be set")
	}

	clearTimeout := chat.Event{
		Type: chat.EventTypeModeration,
		Moderation: &chat.ModerationEvent{
			Action:    chat.ModerationActionRemoveTimeout,
			ChannelID: channel.ID,
			ActorID:   owner.ID,
			TargetID:  target.ID,
			Reason:    "resolved",
		},
		OccurredAt: time.Now().UTC(),
	}
	if err := repo.ApplyChatEvent(clearTimeout); err != nil {
		t.Fatalf("apply remove-timeout event: %v", err)
	}
	unbanEvent := chat.Event{
		Type: chat.EventTypeModeration,
		Moderation: &chat.ModerationEvent{
			Action:    chat.ModerationActionUnban,
			ChannelID: channel.ID,
			ActorID:   owner.ID,
			TargetID:  target.ID,
			Reason:    "appeal",
		},
		OccurredAt: time.Now().UTC(),
	}
	if err := repo.ApplyChatEvent(unbanEvent); err != nil {
		t.Fatalf("apply unban event: %v", err)
	}

	if repo.IsChatBanned(channel.ID, target.ID) {
		t.Fatalf("expected user to be unbanned")
	}
	if _, ok := repo.ChatTimeout(channel.ID, target.ID); ok {
		t.Fatalf("expected timeout to be cleared")
	}
	if remaining := repo.ListChatRestrictions(channel.ID); len(remaining) != 0 {
		t.Fatalf("expected no remaining restrictions, got %v", remaining)
	}
}

func TestPostgresChatReportResolution(t *testing.T) {
	repo := openPostgresRepository(t)

	owner, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	reporter, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "reporter", Email: "reporter@example.com"})
	if err != nil {
		t.Fatalf("create reporter: %v", err)
	}
	target, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "target", Email: "target@example.com"})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	report, err := repo.CreateChatReport(channel.ID, reporter.ID, target.ID, "spam", "", "")
	if err != nil {
		t.Fatalf("create chat report: %v", err)
	}

	resolved, err := repo.ResolveChatReport(report.ID, owner.ID, "handled")
	if err != nil {
		t.Fatalf("resolve chat report: %v", err)
	}
	if resolved.Status != "resolved" || resolved.Resolution != "handled" {
		t.Fatalf("unexpected resolved payload: %+v", resolved)
	}
	if resolved.ResolverID != owner.ID {
		t.Fatalf("expected resolver to be %q, got %q", owner.ID, resolved.ResolverID)
	}
	if resolved.ResolvedAt == nil {
		t.Fatalf("expected resolved timestamp to be set")
	}

	pending, err := repo.ListChatReports(channel.ID, false)
	if err != nil {
		t.Fatalf("list pending chat reports: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending reports, got %d", len(pending))
	}
	all, err := repo.ListChatReports(channel.ID, true)
	if err != nil {
		t.Fatalf("list all chat reports: %v", err)
	}
	if len(all) != 1 || all[0].ID != report.ID {
		t.Fatalf("expected resolved report to be listed, got %+v", all)
	}
}

func TestPostgresTipsLifecycle(t *testing.T) {
	storage.RunRepositoryTipsLifecycle(t, postgresRepositoryFactory)
}

func TestPostgresTipReferenceUniqueness(t *testing.T) {
	repo := openPostgresRepository(t)

	owner, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	supporter, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "fan", Email: "fan@example.com"})
	if err != nil {
		t.Fatalf("create supporter: %v", err)
	}
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	_, err = repo.CreateTip(storage.CreateTipParams{
		ChannelID:  channel.ID,
		FromUserID: supporter.ID,
		Amount:     models.MustParseMoney("5"),
		Currency:   "usd",
		Provider:   "stripe",
		Reference:  "dup-ref",
	})
	if err != nil {
		t.Fatalf("create tip: %v", err)
	}

	_, err = repo.CreateTip(storage.CreateTipParams{
		ChannelID:  channel.ID,
		FromUserID: supporter.ID,
		Amount:     models.MustParseMoney("5"),
		Currency:   "usd",
		Provider:   "stripe",
		Reference:  "dup-ref",
	})
	if err == nil {
		t.Fatal("expected duplicate tip reference to fail")
	}
}

func TestPostgresSubscriptionsLifecycle(t *testing.T) {
	storage.RunRepositorySubscriptionsLifecycle(t, postgresRepositoryFactory)
}

func TestPostgresMonetizationPrecision(t *testing.T) {
	storage.RunRepositoryMonetizationPrecision(t, postgresRepositoryFactory)
}

func TestPostgresSubscriptionReferenceUniqueness(t *testing.T) {
	repo := openPostgresRepository(t)

	owner, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	viewer, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "viewer", Email: "viewer@example.com"})
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	_, err = repo.CreateSubscription(storage.CreateSubscriptionParams{
		ChannelID: channel.ID,
		UserID:    viewer.ID,
		Tier:      "tier1",
		Provider:  "stripe",
		Reference: "dup-sub",
		Amount:    models.MustParseMoney("4.99"),
		Currency:  "usd",
		Duration:  time.Hour,
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	_, err = repo.CreateSubscription(storage.CreateSubscriptionParams{
		ChannelID: channel.ID,
		UserID:    viewer.ID,
		Tier:      "tier1",
		Provider:  "stripe",
		Reference: "dup-sub",
		Amount:    models.MustParseMoney("4.99"),
		Currency:  "usd",
		Duration:  time.Hour,
	})
	if err == nil {
		t.Fatal("expected duplicate subscription reference to fail")
	}
}

func TestPostgresSubscriptionCancellationMetadata(t *testing.T) {
	repo := openPostgresRepository(t)

	owner, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	viewer, err := repo.CreateUser(storage.CreateUserParams{DisplayName: "viewer", Email: "viewer@example.com"})
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	channel, err := repo.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	sub, err := repo.CreateSubscription(storage.CreateSubscriptionParams{
		ChannelID: channel.ID,
		UserID:    viewer.ID,
		Tier:      "tier1",
		Provider:  "stripe",
		Reference: "cancel-me",
		Amount:    models.MustParseMoney("4.99"),
		Currency:  "usd",
		Duration:  time.Hour,
		AutoRenew: true,
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	cancelled, err := repo.CancelSubscription(sub.ID, viewer.ID, "")
	if err != nil {
		t.Fatalf("cancel subscription: %v", err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %q", cancelled.Status)
	}
	if cancelled.AutoRenew {
		t.Fatalf("expected auto renew disabled after cancellation")
	}
	if cancelled.CancelledBy != viewer.ID {
		t.Fatalf("expected cancelledBy %q, got %q", viewer.ID, cancelled.CancelledBy)
	}
	if cancelled.CancelledReason != "user_cancelled" {
		t.Fatalf("expected default cancellation reason, got %q", cancelled.CancelledReason)
	}
	if cancelled.CancelledAt == nil {
		t.Fatal("expected cancellation timestamp to be set")
	}

	stored, ok := repo.GetSubscription(sub.ID)
	if !ok {
		t.Fatalf("expected to load subscription %q", sub.ID)
	}
	if stored.Status != "cancelled" || stored.CancelledBy != viewer.ID || stored.CancelledReason != "user_cancelled" {
		t.Fatalf("unexpected stored subscription after cancellation: %+v", stored)
	}
	if stored.AutoRenew {
		t.Fatalf("expected stored subscription auto renew disabled")
	}
	if stored.CancelledAt == nil {
		t.Fatal("expected stored cancellation timestamp")
	}
}

func TestPostgresStreamKeyRotation(t *testing.T) {
	storage.RunRepositoryStreamKeyRotation(t, postgresRepositoryFactory)
}

func TestPostgresIngestHealthSnapshots(t *testing.T) {
	storage.RunRepositoryIngestHealthSnapshots(t, postgresRepositoryFactory)
}

func TestPostgresRecordingRetention(t *testing.T) {
	storage.RunRepositoryRecordingRetention(t, postgresRepositoryFactory)
}

func TestPostgresStreamLifecycleWithoutIngest(t *testing.T) {
	storage.RunRepositoryStreamLifecycleWithoutIngest(t, postgresRepositoryFactory)
}

func TestPostgresStreamTimeouts(t *testing.T) {
	storage.RunRepositoryStreamTimeouts(t, postgresRepositoryFactory)
}

func applyPostgresMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("determine repository root: runtime.Caller failed")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	migrationsDir := filepath.Join(repoRoot, "deploy", "migrations")

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		path := filepath.Join(migrationsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", entry.Name(), err)
		}
		for _, stmt := range splitSQLStatements(string(data)) {
			if stmt == "" {
				continue
			}
			if _, err := pool.Exec(ctx, stmt); err != nil {
				t.Fatalf("apply migration %s: %v", entry.Name(), err)
			}
		}
	}
}

func truncatePostgresTables(ctx context.Context, pool *pgxpool.Pool) error {
	tables, err := storage.PostgresTablesForTest(ctx, pool)
	if err != nil {
		return err
	}

	if len(tables) == 0 {
		return nil
	}
	query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(tables, ", "))
	_, err = pool.Exec(ctx, query)
	return err
}

func splitSQLStatements(script string) []string {
	parts := strings.Split(script, ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		statements = append(statements, trimmed)
	}
	return statements
}
