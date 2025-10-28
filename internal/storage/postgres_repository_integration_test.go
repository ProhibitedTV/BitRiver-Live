//go:build postgres

package storage_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"bitriver-live/internal/ingest"
	"bitriver-live/internal/storage"
)

func TestPostgresRepositoryConnection(t *testing.T) {
	dsn := os.Getenv("BITRIVER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("BITRIVER_TEST_POSTGRES_DSN not set")
	}

	repo, err := storage.NewPostgresRepository(dsn, storage.WithIngestController(ingest.NoopController{}))
	if err != nil {
		if errors.Is(err, storage.ErrPostgresUnavailable) {
			t.Skip("postgres repository unavailable in this build")
		}
		t.Fatalf("failed to open postgres repository: %v", err)
	}
	t.Cleanup(func() {
		if closer, ok := repo.(interface{ Close(context.Context) error }); ok {
			_ = closer.Close(context.Background())
			return
		}
		if closer, ok := repo.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	})
}
