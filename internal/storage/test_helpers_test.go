package storage

import (
	"os"
	"path/filepath"
	"testing"

	"bitriver-live/internal/ingest"
)

func newTestStore(t *testing.T) *Storage {
	return newTestStoreWithController(t, ingest.NoopController{})
}

func newTestStoreWithController(t *testing.T, controller ingest.Controller, extra ...Option) *Storage {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	if controller == nil {
		controller = ingest.NoopController{}
	}
	opts := []Option{WithIngestController(controller), WithIngestRetries(1, 0)}
	opts = append(opts, extra...)
	store, err := NewStorage(path, opts...)
	if err != nil {
		t.Fatalf("NewStorage error: %v", err)
	}
	return store
}

func jsonRepositoryFactory(t *testing.T, opts ...Option) (Repository, func(), error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	defaults := []Option{WithIngestController(ingest.NoopController{}), WithIngestRetries(1, 0)}
	opts = append(defaults, opts...)
	store, err := NewStorage(path, opts...)
	if err != nil {
		return nil, nil, err
	}
	return store, func() {}, nil
}

func firstRecordingID(store *Storage) string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	for id := range store.data.Recordings {
		return id
	}
	return ""
}

func TestMain(m *testing.M) {
	// ensure tests do not leave temp files behind by relying on testing package cleanup
	code := m.Run()
	os.Exit(code)
}
