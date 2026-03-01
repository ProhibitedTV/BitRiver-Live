package app

import (
	"bitriver-live/internal/api"
	"bitriver-live/internal/storage"
)

func uploadSourceStorageConfigFromObjectStorage(cfg storage.ObjectStorageConfig) api.UploadSourceStorageConfig {
	return api.UploadSourceStorageConfig{
		Endpoint:       cfg.Endpoint,
		Bucket:         cfg.Bucket,
		Prefix:         cfg.Prefix,
		PublicEndpoint: cfg.PublicEndpoint,
		UseSSL:         cfg.UseSSL,
		RequestTimeout: cfg.RequestTimeout,
	}
}
