package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"bitriver-live/internal/domain"
)

func TestUploadSourceCleanerDeleteMissingFileNoop(t *testing.T) {
	dir := t.TempDir()
	cleaner := NewUploadSourceCleaner(UploadSourceStorageConfig{}, dir, nil)
	upload := domain.Upload{ID: "upload-1"}
	if err := cleaner.Delete(context.Background(), upload, "missing.mp4"); err != nil {
		t.Fatalf("Delete returned error for missing file: %v", err)
	}
}

func TestUploadSourceCleanerDeleteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.mp4")
	if err := os.WriteFile(path, []byte("payload"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cleaner := NewUploadSourceCleaner(UploadSourceStorageConfig{}, dir, nil)
	upload := domain.Upload{ID: "upload-2"}
	if err := cleaner.Delete(context.Background(), upload, "exists.mp4"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file to be deleted, stat err=%v", err)
	}
}
