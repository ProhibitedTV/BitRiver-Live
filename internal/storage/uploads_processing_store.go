package storage

import (
	"context"
	"fmt"
	"strings"

	"bitriver-live/internal/domain"
	serviceuploads "bitriver-live/internal/service/uploads"
)

// UploadProcessingStore adapts repository operations for the upload processing service.
type UploadProcessingStore struct {
	repo interface {
		ListChannels(ownerID, query string) []domain.Channel
		ListUploads(channelID string) ([]domain.Upload, error)
		GetUpload(id string) (domain.Upload, bool)
		UpdateUpload(id string, update domain.UploadUpdate) (domain.Upload, error)
	}
}

// NewUploadProcessingStore adapts a repository-capable service to the upload processor store contract.
func NewUploadProcessingStore(repo interface {
	ListChannels(ownerID, query string) []domain.Channel
	ListUploads(channelID string) ([]domain.Upload, error)
	GetUpload(id string) (domain.Upload, bool)
	UpdateUpload(id string, update domain.UploadUpdate) (domain.Upload, error)
}) serviceuploads.Store {
	return UploadProcessingStore{repo: repo}
}

// ListPendingUploads returns pending uploads from the configured backing services.
func (s UploadProcessingStore) ListPendingUploads(ctx context.Context, limit int) ([]domain.Upload, error) {
	if s.repo == nil {
		return nil, nil
	}

	var (
		pending  []domain.Upload
		firstErr error
	)

	for _, channel := range s.repo.ListChannels("", "") {
		if limit > 0 && len(pending) >= limit {
			break
		}
		select {
		case <-ctx.Done():
			return pending, ctx.Err()
		default:
		}

		uploads, err := s.repo.ListUploads(channel.ID)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, upload := range uploads {
			status := strings.ToLower(strings.TrimSpace(upload.Status))
			if status != "pending" && status != "processing" {
				continue
			}
			pending = append(pending, upload)
			if limit > 0 && len(pending) >= limit {
				break
			}
		}
	}

	return pending, firstErr
}

// GetUpload returns upload from the configured backing services.
func (s UploadProcessingStore) GetUpload(ctx context.Context, id string) (domain.Upload, bool) {
	if s.repo == nil {
		return domain.Upload{}, false
	}
	select {
	case <-ctx.Done():
		return domain.Upload{}, false
	default:
	}

	return s.repo.GetUpload(id)
}

// UpdateUpload updates upload and returns an error when persistence or validation fails.
func (s UploadProcessingStore) UpdateUpload(ctx context.Context, id string, update domain.UploadUpdate) (domain.Upload, error) {
	if s.repo == nil {
		return domain.Upload{}, fmt.Errorf("upload store unavailable")
	}
	select {
	case <-ctx.Done():
		return domain.Upload{}, ctx.Err()
	default:
	}

	return s.repo.UpdateUpload(id, update)
}

var _ serviceuploads.Store = UploadProcessingStore{}
