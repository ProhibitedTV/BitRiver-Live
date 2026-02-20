package api

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"bitriver-live/internal/domain"
	serviceuploads "bitriver-live/internal/service/uploads"
)

// NewUploadSourceCleaner builds a SourceArtifactCleaner backed by the same
// upload source storage configuration used by upload handlers.
func NewUploadSourceCleaner(cfg UploadSourceStorageConfig, uploadMediaDir string, logger *slog.Logger) serviceuploads.SourceArtifactCleaner {
	return &uploadSourceCleaner{
		store:          newUploadSourceStorage(cfg),
		uploadMediaDir: strings.TrimSpace(uploadMediaDir),
		logger:         logger,
	}
}

type uploadSourceCleaner struct {
	store          uploadSourceStorage
	uploadMediaDir string
	logger         *slog.Logger
}

func (c *uploadSourceCleaner) Delete(ctx context.Context, upload domain.Upload, sourceKey string) error {
	key := strings.TrimSpace(sourceKey)
	if key == "" {
		return nil
	}
	if c.store != nil && c.store.Enabled() {
		return c.store.Delete(ctx, key)
	}
	dir := strings.TrimSpace(c.uploadMediaDir)
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "bitriver-uploads")
	}
	fullPath := filepath.Join(dir, filepath.Base(key))
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if c.logger != nil {
		c.logger.Info("cleanup upload source artifact", "upload_id", upload.ID, "source_key", key, "path", fullPath)
	}
	return nil
}

var _ serviceuploads.SourceArtifactCleaner = (*uploadSourceCleaner)(nil)
