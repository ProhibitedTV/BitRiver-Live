package service

import (
	"errors"

	"bitriver-live/internal/storage"
)

// IsIngestUnavailable reports whether an error indicates ingest controller unavailability.
func IsIngestUnavailable(err error) bool {
	return errors.Is(err, storage.ErrIngestControllerUnavailable)
}
