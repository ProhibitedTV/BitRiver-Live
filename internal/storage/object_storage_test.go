package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type fakeObjectStorage struct {
	uploads []fakeUpload
	deletes []string
	prefix  string
	baseURL string
}

type hangingDeleteObjectStorage struct{}

type memoryS3Server struct {
	mu       sync.Mutex
	objects  map[string]map[string][]byte
	requests []memoryS3Request
}

type memoryS3Request struct {
	Method        string
	Authorization string
	ContentSHA    string
}

func newMemoryS3Server() *memoryS3Server {
	return &memoryS3Server{objects: make(map[string]map[string][]byte)}
}

func (m *memoryS3Server) addBucket(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.objects[name]; !exists {
		m.objects[name] = make(map[string][]byte)
	}
}

func (m *memoryS3Server) getObject(bucket, key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	objs, ok := m.objects[bucket]
	if !ok {
		return nil, false
	}
	data, ok := objs[key]
	if !ok {
		return nil, false
	}
	copyData := append([]byte(nil), data...)
	return copyData, true
}

func (m *memoryS3Server) lastRequest() memoryS3Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.requests) == 0 {
		return memoryS3Request{}
	}
	return m.requests[len(m.requests)-1]
}

func (m *memoryS3Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()
	bucket, key, err := parseS3Path(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusInternalServerError)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, memoryS3Request{
		Method:        r.Method,
		Authorization: r.Header.Get("Authorization"),
		ContentSHA:    r.Header.Get("X-Amz-Content-Sha256"),
	})
	bucketObjects, exists := m.objects[bucket]
	if !exists {
		http.Error(w, "bucket not found", http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodPut:
		bucketObjects[key] = append([]byte(nil), body...)
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		delete(bucketObjects, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func parseS3Path(path string) (string, string, error) {
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" {
		return "", "", fmt.Errorf("missing bucket")
	}
	parts := strings.SplitN(trimmed, "/", 2)
	bucket := parts[0]
	key := ""
	if len(parts) == 2 {
		key = parts[1]
	}
	if bucket == "" {
		return "", "", fmt.Errorf("missing bucket")
	}
	return bucket, key, nil
}

func TestS3ObjectStorageClientUploadDelete(t *testing.T) {
	server := newMemoryS3Server()
	server.addBucket("vod")
	ts := httptest.NewServer(server)
	defer ts.Close()

	cfg := ObjectStorageConfig{
		Endpoint:       strings.TrimPrefix(ts.URL, "http://"),
		Region:         "us-east-1",
		AccessKey:      "AKIAEXAMPLE",
		SecretKey:      "secretKeyExample",
		Bucket:         "vod",
		UseSSL:         false,
		Prefix:         "vod/assets",
		PublicEndpoint: "https://cdn.example.com/content",
	}

	client := newObjectStorageClient(cfg)
	s3Client, ok := client.(*s3ObjectStorageClient)
	if !ok {
		t.Fatalf("expected s3ObjectStorageClient, got %T", client)
	}

	ctx := context.Background()
	payload := []byte("stream manifest data")
	ref, err := s3Client.Upload(ctx, "manifests/stream.m3u8", "application/vnd.apple.mpegurl", payload)
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	expectedKey := "vod/assets/manifests/stream.m3u8"
	if ref.Key != expectedKey {
		t.Fatalf("expected key %s, got %s", expectedKey, ref.Key)
	}
	expectedURL := "https://cdn.example.com/content/" + expectedKey
	if ref.URL != expectedURL {
		t.Fatalf("expected url %s, got %s", expectedURL, ref.URL)
	}
	stored, ok := server.getObject("vod", expectedKey)
	if !ok {
		t.Fatalf("expected object %s to be stored", expectedKey)
	}
	if !bytes.Equal(stored, payload) {
		t.Fatalf("stored payload mismatch: got %q", stored)
	}
	uploadReq := server.lastRequest()
	if uploadReq.Method != http.MethodPut {
		t.Fatalf("expected PUT request, got %s", uploadReq.Method)
	}
	if uploadReq.Authorization == "" || !strings.Contains(uploadReq.Authorization, cfg.AccessKey) {
		t.Fatal("expected authorization header to include access key")
	}
	if uploadReq.ContentSHA == "" {
		t.Fatal("expected content hash header to be set")
	}

	if err := s3Client.Delete(ctx, ref.Key); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, ok := server.getObject("vod", expectedKey); ok {
		t.Fatalf("expected object %s to be removed", expectedKey)
	}
	deleteReq := server.lastRequest()
	if deleteReq.Method != http.MethodDelete {
		t.Fatalf("expected DELETE request, got %s", deleteReq.Method)
	}
	if deleteReq.Authorization == "" || !strings.Contains(deleteReq.Authorization, cfg.AccessKey) {
		t.Fatal("expected delete request to include authorization header")
	}
}

type fakeUpload struct {
	Key         string
	ContentType string
	Body        []byte
	URL         string
}

func (f *fakeObjectStorage) Enabled() bool { return true }

func (f *fakeObjectStorage) Upload(ctx context.Context, key, contentType string, body []byte) (objectReference, error) {
	trimmed := strings.TrimLeft(key, "/")
	prefix := strings.Trim(f.prefix, "/")
	finalKey := trimmed
	if prefix != "" {
		if finalKey != "" {
			finalKey = prefix + "/" + finalKey
		} else {
			finalKey = prefix
		}
	}
	copyBody := append([]byte(nil), body...)
	upload := fakeUpload{Key: finalKey, ContentType: contentType, Body: copyBody}
	url := ""
	if f.baseURL != "" {
		base := strings.TrimRight(f.baseURL, "/")
		if finalKey != "" {
			url = base + "/" + finalKey
		} else {
			url = base
		}
	}
	upload.URL = url
	f.uploads = append(f.uploads, upload)
	return objectReference{Key: finalKey, URL: url}, nil
}

func (f *fakeObjectStorage) Delete(ctx context.Context, key string) error {
	f.deletes = append(f.deletes, key)
	return nil
}

func (h *hangingDeleteObjectStorage) Enabled() bool { return true }

func (h *hangingDeleteObjectStorage) Upload(ctx context.Context, key, contentType string, body []byte) (objectReference, error) {
	return objectReference{}, nil
}

func (h *hangingDeleteObjectStorage) Delete(ctx context.Context, key string) error {
	<-ctx.Done()
	return ctx.Err()
}
