// Command migrate-json-to-postgres migrates stored data from JSON into Postgres.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"bitriver-live/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	jsonPath := flag.String("json", "data/store.json", "path to the JSON datastore to migrate")
	postgresDSN := flag.String("postgres-dsn", "", "Postgres connection string")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	dsn := strings.TrimSpace(*postgresDSN)
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("BITRIVER_LIVE_POSTGRES_DSN"))
	}
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		logger.Error("postgres DSN required", "hint", "set --postgres-dsn, BITRIVER_LIVE_POSTGRES_DSN, or DATABASE_URL")
		os.Exit(1)
	}

	snapshot, err := storage.LoadSnapshotFromJSON(*jsonPath)
	if err != nil {
		logger.Error("failed to load JSON snapshot", "error", err)
		os.Exit(1)
	}
	counts := snapshot.Counts()
	logger.Info("loaded JSON snapshot", "path", *jsonPath, "users", counts.Users, "channels", counts.Channels)

	repo, err := storage.NewPostgresRepository(dsn)
	if err != nil {
		logger.Error("failed to open postgres repository", "error", err)
		os.Exit(1)
	}
	defer func() {
		if closer, ok := repo.(interface{ Close(context.Context) error }); ok {
			_ = closer.Close(context.Background())
		}
	}()

	if err := storage.ImportSnapshotToPostgres(context.Background(), repo, snapshot); err != nil {
		logger.Error("failed to import snapshot", "error", err)
		os.Exit(1)
	}

	if err := verifyCounts(context.Background(), dsn, counts); err != nil {
		logger.Error("verification failed", "error", err)
		os.Exit(1)
	}

	logger.Info("migration completed", "users", counts.Users, "channels", counts.Channels, "recordings", counts.Recordings)
}

func verifyCounts(ctx context.Context, dsn string, counts storage.SnapshotCounts) error {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse verification config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("open verification connection: %w", err)
	}
	defer pool.Close()

	checks := []struct {
		name     string
		query    string
		expected int
	}{
		{"users", "SELECT COUNT(*) FROM users", counts.Users},
		{"profiles", "SELECT COUNT(*) FROM profiles", counts.Profiles},
		{"channels", "SELECT COUNT(*) FROM channels", counts.Channels},
		{"follows", "SELECT COUNT(*) FROM follows", counts.Follows},
		{"stream_sessions", "SELECT COUNT(*) FROM stream_sessions", counts.StreamSessions},
		{"stream_session_manifests", "SELECT COUNT(*) FROM stream_session_manifests", counts.StreamSessionManifests},
		{"recordings", "SELECT COUNT(*) FROM recordings", counts.Recordings},
		{"recording_renditions", "SELECT COUNT(*) FROM recording_renditions", counts.RecordingRenditions},
		{"recording_thumbnails", "SELECT COUNT(*) FROM recording_thumbnails", counts.RecordingThumbnails},
		{"uploads", "SELECT COUNT(*) FROM uploads", counts.Uploads},
		{"clip_exports", "SELECT COUNT(*) FROM clip_exports", counts.ClipExports},
		{"chat_messages", "SELECT COUNT(*) FROM chat_messages", counts.ChatMessages},
		{"chat_bans", "SELECT COUNT(*) FROM chat_bans", counts.ChatBans},
		{"chat_timeouts", "SELECT COUNT(*) FROM chat_timeouts", counts.ChatTimeouts},
		{"chat_reports", "SELECT COUNT(*) FROM chat_reports", counts.ChatReports},
		{"tips", "SELECT COUNT(*) FROM tips", counts.Tips},
		{"subscriptions", "SELECT COUNT(*) FROM subscriptions", counts.Subscriptions},
		{"oauth_accounts", "SELECT COUNT(*) FROM oauth_accounts", counts.OAuthAccounts},
	}

	for _, check := range checks {
		var actual int
		if err := pool.QueryRow(ctx, check.query).Scan(&actual); err != nil {
			return fmt.Errorf("query %s: %w", check.name, err)
		}
		if actual != check.expected {
			return fmt.Errorf("mismatch for %s: expected %d, got %d", check.name, check.expected, actual)
		}
	}
	return nil
}
