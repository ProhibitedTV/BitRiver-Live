//go:build postgres

package storage_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/ingest"
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

func TestPostgresUserLifecycle(t *testing.T) {
	storage.RunRepositoryUserLifecycle(t, postgresRepositoryFactory)
}

func TestPostgresChatRestrictionsLifecycle(t *testing.T) {
	storage.RunRepositoryChatRestrictionsLifecycle(t, postgresRepositoryFactory)
}

func TestPostgresChatReportsLifecycle(t *testing.T) {
	storage.RunRepositoryChatReportsLifecycle(t, postgresRepositoryFactory)
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

func TestPostgresSubscriptionsLifecycle(t *testing.T) {
	storage.RunRepositorySubscriptionsLifecycle(t, postgresRepositoryFactory)
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

var postgresTables = []string{
	"chat_messages",
	"chat_bans",
	"chat_timeouts",
	"chat_reports",
	"tips",
	"subscriptions",
	"clip_exports",
	"recording_thumbnails",
	"recording_renditions",
	"recordings",
	"stream_session_manifests",
	"stream_sessions",
	"follows",
	"channels",
	"profiles",
	"users",
}

func applyPostgresMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	entries, err := os.ReadDir("deploy/migrations")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		path := filepath.Join("deploy/migrations", entry.Name())
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
	if len(postgresTables) == 0 {
		return nil
	}
	query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(postgresTables, ", "))
	_, err := pool.Exec(ctx, query)
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
