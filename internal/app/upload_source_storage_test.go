package app

import (
	"testing"
	"time"

	"bitriver-live/internal/api"
	"bitriver-live/internal/storage"
)

func TestUploadSourceStorageConfigFromObjectStorage(t *testing.T) {
	t.Parallel()

	input := storage.ObjectStorageConfig{
		Endpoint:       "s3.internal.example:9000",
		Bucket:         "uploads-source",
		Prefix:         "recordings/raw",
		PublicEndpoint: "https://cdn.example.com",
		UseSSL:         true,
		RequestTimeout: 17 * time.Second,
	}

	got := uploadSourceStorageConfigFromObjectStorage(input)
	want := api.UploadSourceStorageConfig{
		Endpoint:       input.Endpoint,
		Bucket:         input.Bucket,
		Prefix:         input.Prefix,
		PublicEndpoint: input.PublicEndpoint,
		UseSSL:         input.UseSSL,
		RequestTimeout: input.RequestTimeout,
	}

	if got != want {
		t.Fatalf("upload source storage config mismatch: got %+v, want %+v", got, want)
	}
}
