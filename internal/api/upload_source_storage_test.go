package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestObjectUploadSourceStorageDeleteNotFoundIsNoop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	store := newUploadSourceStorage(UploadSourceStorageConfig{
		Endpoint: strings.TrimPrefix(server.URL, "http://"),
		Bucket:   "bucket",
	})
	if !store.Enabled() {
		t.Fatal("expected object store to be enabled")
	}
	if err := store.Delete(context.Background(), "missing.mp4"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}
