package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/storage"
)

func uploadSecurityOwner(t *testing.T, store *storage.Storage, name string) (domain.User, domain.Channel) {
	t.Helper()
	owner, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: name, Email: name + "@example.com", Roles: []string{"creator"},
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, name, "gaming", nil)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	return owner, channel
}

type uploadSecurityEnqueuer struct{ ids []string }

func (e *uploadSecurityEnqueuer) Enqueue(id string) { e.ids = append(e.ids, id) }

func uploadSecurityMultipart(t *testing.T, channelID string, metadata map[string]string, fileFirst bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writeFile := func() {
		part, err := writer.CreateFormFile("file", "clip.mp4")
		if err != nil {
			t.Fatalf("create file part: %v", err)
		}
		if _, err := part.Write(validMP4Sample()); err != nil {
			t.Fatalf("write file part: %v", err)
		}
	}
	if fileFirst {
		writeFile()
	}
	if err := writer.WriteField("channelId", channelID); err != nil {
		t.Fatalf("write channel field: %v", err)
	}
	for key, value := range metadata {
		if err := writer.WriteField("metadata["+key+"]", value); err != nil {
			t.Fatalf("write metadata: %v", err)
		}
	}
	if !fileFirst {
		writeFile()
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/uploads", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestUploadRejectsServerManagedMetadata(t *testing.T) {
	var objectCalls atomic.Int32
	objects := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		objectCalls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(objects.Close)
	for _, key := range []string{"mediaPath", "mediaToken", "sourceObjectKey", "sourceObjectURL"} {
		for _, variant := range []string{key, " " + key + " ", strings.ToUpper(key), " " + strings.ToUpper(key) + " "} {
			for _, format := range []string{"json", "multipart_file_first", "multipart_file_last"} {
				t.Run(format+"/"+variant, func(t *testing.T) {
					h, store := newTestHandler(t)
					h.UploadMediaDir = t.TempDir()
					h.UploadSourceStorage = UploadSourceStorageConfig{
						Endpoint: objects.URL, Bucket: "bucket", Prefix: "upload-sources",
					}
					enqueue := &uploadSecurityEnqueuer{}
					h.UploadProcessor = enqueue
					owner, channel := uploadSecurityOwner(t, store, "owner")
					metadata := map[string]string{variant: "fixture-untrusted-value"}
					var req *http.Request
					if format == "json" {
						payload, err := json.Marshal(createUploadRequest{ChannelID: channel.ID, Metadata: metadata})
						if err != nil {
							t.Fatalf("encode upload: %v", err)
						}
						req = httptest.NewRequest(http.MethodPost, "/api/uploads", bytes.NewReader(payload))
					} else {
						req = uploadSecurityMultipart(t, channel.ID, metadata, format == "multipart_file_first")
					}
					rec := httptest.NewRecorder()
					h.Uploads(rec, withUser(req, owner))
					if rec.Code != http.StatusBadRequest {
						t.Fatalf("status=%d, want 400", rec.Code)
					}
					if !strings.Contains(rec.Body.String(), "server-managed") || strings.Contains(rec.Body.String(), "fixture-untrusted-value") {
						t.Fatal("expected actionable error without metadata value disclosure")
					}
					uploads, err := store.ListUploads(channel.ID)
					if err != nil || len(uploads) != 0 || len(enqueue.ids) != 0 {
						t.Fatal("rejected upload was persisted or enqueued")
					}
					files, err := os.ReadDir(h.UploadMediaDir)
					if err != nil || len(files) != 0 {
						t.Fatal("rejected multipart upload left a temporary file")
					}
					if objectCalls.Load() != 0 {
						t.Fatal("rejected request touched durable storage")
					}
				})
			}
		}
	}
}

func TestUploadCustomMetadataAndGeneratedMediaRemainUsable(t *testing.T) {
	for _, format := range []string{"json", "multipart"} {
		t.Run(format, func(t *testing.T) {
			h, store := newTestHandler(t)
			h.UploadMediaDir = t.TempDir()
			enqueue := &uploadSecurityEnqueuer{}
			h.UploadProcessor = enqueue
			owner, channel := uploadSecurityOwner(t, store, "owner")
			metadata := map[string]string{"label": "custom", "sourceUrl": "https://example.com/source.mp4"}
			var req *http.Request
			if format == "json" {
				payload, err := json.Marshal(createUploadRequest{ChannelID: channel.ID, Metadata: metadata})
				if err != nil {
					t.Fatalf("encode upload: %v", err)
				}
				req = httptest.NewRequest(http.MethodPost, "/api/uploads", bytes.NewReader(payload))
			} else {
				req = uploadSecurityMultipart(t, channel.ID, metadata, true)
			}
			rec := httptest.NewRecorder()
			h.Uploads(rec, withUser(req, owner))
			if rec.Code != http.StatusCreated {
				t.Fatalf("status=%d, want 201", rec.Code)
			}
			var upload uploadResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &upload); err != nil {
				t.Fatalf("decode upload: %v", err)
			}
			if upload.Metadata["label"] != "custom" || !reflect.DeepEqual(enqueue.ids, []string{upload.ID}) {
				t.Fatal("custom metadata or normal enqueue lost")
			}
			if format == "json" {
				if upload.Metadata["sourceUrl"] != metadata["sourceUrl"] {
					t.Fatal("external source URL changed")
				}
				return
			}
			if upload.Metadata["sourceObjectKey"] != upload.ID+".mp4" || upload.Metadata["mediaToken"] == "" {
				t.Fatal("server source reference/token missing")
			}
			media := httptest.NewRecorder()
			h.UploadByID(media, httptest.NewRequest(http.MethodGet, upload.Metadata["sourceUrl"], nil))
			if media.Code != http.StatusOK || !bytes.Equal(media.Body.Bytes(), validMP4Sample()) {
				t.Fatal("server-generated media capability unusable")
			}
			deleted := httptest.NewRecorder()
			h.UploadByID(deleted, withUser(httptest.NewRequest(http.MethodDelete, "/api/uploads/"+upload.ID, nil), owner))
			files, err := os.ReadDir(h.UploadMediaDir)
			if deleted.Code != http.StatusNoContent || err != nil || len(files) != 0 {
				t.Fatal("normal delete failed to remove source/temp files")
			}
		})
	}
}

func TestUploadRejectsCrossOwnerSourceReference(t *testing.T) {
	for _, reference := range []string{"mediaPath", "sourceObjectKey"} {
		t.Run(reference, func(t *testing.T) {
			h, store := newTestHandler(t)
			h.UploadMediaDir = t.TempDir()
			victim, victimChannel := uploadSecurityOwner(t, store, "victim")
			attacker, attackerChannel := uploadSecurityOwner(t, store, "attacker")
			source := validMP4Sample()
			rec := httptest.NewRecorder()
			h.Uploads(rec, withUser(newMultipartUploadRequest(t, victimChannel.ID, "private.mp4", source, false), victim))
			if rec.Code != http.StatusCreated {
				t.Fatalf("create victim upload: status %d", rec.Code)
			}
			var victimUpload uploadResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &victimUpload); err != nil {
				t.Fatalf("decode victim upload: %v", err)
			}
			before, ok := store.GetUpload(victimUpload.ID)
			if !ok {
				t.Fatal("victim upload missing")
			}
			key := victimUpload.Metadata["sourceObjectKey"]
			payload, err := json.Marshal(createUploadRequest{
				ChannelID: attackerChannel.ID, Filename: "forged.mp4",
				Metadata: map[string]string{reference: key, "mediaToken": "fixture-forged-capability"},
			})
			if err != nil {
				t.Fatalf("encode attack fixture: %v", err)
			}
			rec = httptest.NewRecorder()
			h.Uploads(rec, withUser(httptest.NewRequest(http.MethodPost, "/api/uploads", bytes.NewReader(payload)), attacker))
			if rec.Code == http.StatusCreated {
				// Fail-before diagnostic: all reads/deletes target only t.TempDir.
				var forged uploadResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &forged); err != nil {
					t.Fatalf("decode forged upload: %v", err)
				}
				read := httptest.NewRecorder()
				h.UploadByID(read, httptest.NewRequest(http.MethodGet, "/api/uploads/"+forged.ID+"/media?token=fixture-forged-capability", nil))
				deleted := httptest.NewRecorder()
				h.UploadByID(deleted, withUser(httptest.NewRequest(http.MethodDelete, "/api/uploads/"+forged.ID, nil), attacker))
				_, statErr := os.Stat(filepath.Join(h.UploadMediaDir, key))
				t.Fatalf("forged upload accepted: raw GET=%d leaked victim bytes=%t; DELETE=%d removed victim source=%t",
					read.Code, bytes.Equal(read.Body.Bytes(), source), deleted.Code, os.IsNotExist(statErr))
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d, want 400", rec.Code)
			}
			after, ok := store.GetUpload(victimUpload.ID)
			if !ok || !reflect.DeepEqual(before, after) {
				t.Fatal("rejected request changed victim upload")
			}
			remaining, err := os.ReadFile(filepath.Join(h.UploadMediaDir, key))
			if err != nil || !bytes.Equal(remaining, source) {
				t.Fatal("rejected request changed victim source")
			}
			uploads, err := store.ListUploads(attackerChannel.ID)
			if err != nil || len(uploads) != 0 {
				t.Fatal("rejected request persisted an attacker upload")
			}
		})
	}
}
