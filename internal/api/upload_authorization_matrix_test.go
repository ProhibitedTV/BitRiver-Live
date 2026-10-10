package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/storage"
)

type uploadAuthorizationPrincipal struct {
	name  string
	roles []string
	owner bool
	admin bool
}

var uploadAuthorizationPrincipals = []uploadAuthorizationPrincipal{
	{name: "guest"},
	{name: "viewer", roles: []string{"viewer"}},
	{name: "unrelated_creator", roles: []string{"creator"}},
	{name: "moderator", roles: []string{"moderator"}},
	{name: "owner", roles: []string{"creator"}, owner: true},
	{name: "admin", roles: []string{"admin"}, admin: true},
}

type uploadAuthorizationFixture struct {
	handler               *Handler
	store                 *storage.Storage
	owner, otherOwner     domain.User
	channel, otherChannel domain.Channel
	upload, otherUpload   domain.Upload
	source, otherSource   []byte
}

func newUploadAuthorizationFixture(t *testing.T) uploadAuthorizationFixture {
	t.Helper()
	h, store := newTestHandler(t)
	h.UploadMediaDir = t.TempDir()
	owner, channel := uploadSecurityOwner(t, store, "owner")
	otherOwner, otherChannel := uploadSecurityOwner(t, store, "other")
	source := validMP4Sample()
	otherSource := append(append([]byte(nil), source...), byte(42))
	create := func(actor domain.User, channel domain.Channel, data []byte) domain.Upload {
		t.Helper()
		rec := httptest.NewRecorder()
		h.Uploads(rec, withUser(newMultipartUploadRequest(t, channel.ID, "private.mp4", data, false), actor))
		if rec.Code != http.StatusCreated {
			t.Fatalf("create fixture upload: status=%d", rec.Code)
		}
		var response uploadResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode fixture upload: %v", err)
		}
		upload, ok := store.GetUpload(response.ID)
		if !ok || upload.Metadata["mediaToken"] == "" || upload.Metadata["sourceObjectKey"] == "" {
			t.Fatal("fixture upload/source capability missing")
		}
		return upload
	}
	return uploadAuthorizationFixture{
		handler: h, store: store, owner: owner, otherOwner: otherOwner,
		channel: channel, otherChannel: otherChannel,
		upload: create(owner, channel, source), otherUpload: create(otherOwner, otherChannel, otherSource),
		source: source, otherSource: otherSource,
	}
}

func (f uploadAuthorizationFixture) requestAs(t *testing.T, req *http.Request, principal uploadAuthorizationPrincipal) *http.Request {
	t.Helper()
	if principal.name == "guest" {
		return req
	}
	actor := f.owner
	if principal.name == "unrelated_creator" {
		actor = f.otherOwner
	} else if !principal.owner {
		var err error
		actor, err = f.store.CreateUser(storage.CreateUserParams{
			DisplayName: "actor", Email: "actor@example.com", Roles: principal.roles,
		})
		if err != nil {
			t.Fatalf("create principal: %v", err)
		}
	}
	return withUser(req, actor)
}

func (f uploadAuthorizationFixture) requireSource(t *testing.T, upload domain.Upload, expected []byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.handler.UploadMediaDir, upload.Metadata["sourceObjectKey"]))
	if err != nil || !bytes.Equal(data, expected) {
		t.Fatal("source bytes changed or disappeared")
	}
}

func (f uploadAuthorizationFixture) requireUnchanged(t *testing.T, upload domain.Upload, source []byte) {
	t.Helper()
	stored, ok := f.store.GetUpload(upload.ID)
	if !ok || !reflect.DeepEqual(upload, stored) {
		t.Fatal("upload changed or disappeared")
	}
	f.requireSource(t, upload, source)
}

// The matrix exercises handler policy, not session/middleware enforcement.
func TestUploadAuthorizationMatrix(t *testing.T) {
	for _, principal := range uploadAuthorizationPrincipals {
		for _, action := range []string{"list", "detail", "create_json", "create_multipart", "delete"} {
			t.Run(principal.name+"/"+action, func(t *testing.T) {
				f := newUploadAuthorizationFixture(t)
				enqueue := &uploadSecurityEnqueuer{}
				f.handler.UploadProcessor = enqueue
				var req *http.Request
				wantStatus := http.StatusOK
				switch action {
				case "list":
					req = httptest.NewRequest(http.MethodGet, "/api/uploads?channelId="+f.channel.ID, nil)
				case "detail":
					req = httptest.NewRequest(http.MethodGet, "/api/uploads/"+f.upload.ID, nil)
				case "create_json":
					payload, err := json.Marshal(createUploadRequest{ChannelID: f.channel.ID, Title: "Authorized upload", Filename: "new.mp4",
						Metadata: map[string]string{"label": "authorized", "sourceUrl": "https://example.com/source.mp4"}})
					if err != nil {
						t.Fatalf("encode request: %v", err)
					}
					req = httptest.NewRequest(http.MethodPost, "/api/uploads", bytes.NewReader(payload))
					req.Header.Set("Content-Type", "application/json")
					wantStatus = http.StatusCreated
				case "create_multipart":
					req = uploadSecurityMultipart(t, f.channel.ID, map[string]string{"label": "authorized"}, true)
					wantStatus = http.StatusCreated
				case "delete":
					req = httptest.NewRequest(http.MethodDelete, "/api/uploads/"+f.upload.ID, nil)
					wantStatus = http.StatusNoContent
				}
				allowed := principal.owner || principal.admin
				if !allowed {
					wantStatus = http.StatusForbidden
					if principal.name == "guest" {
						wantStatus = http.StatusUnauthorized
					}
				}
				rec := httptest.NewRecorder()
				req = f.requestAs(t, req, principal)
				if action == "detail" || action == "delete" {
					f.handler.UploadByID(rec, req)
				} else {
					f.handler.Uploads(rec, req)
				}
				if rec.Code != wantStatus {
					t.Fatalf("status=%d, want %d", rec.Code, wantStatus)
				}
				for _, private := range []string{f.otherUpload.ID, f.otherUpload.Metadata["mediaToken"], f.otherUpload.Metadata["sourceObjectKey"]} {
					if strings.Contains(rec.Body.String(), private) {
						t.Fatal("unrelated upload/reference/token leaked")
					}
				}
				wantRows, wantFiles := 1, 2
				if !allowed {
					for _, private := range []string{f.upload.Metadata["mediaToken"], f.upload.Metadata["sourceObjectKey"], `"metadata"`} {
						if strings.Contains(rec.Body.String(), private) {
							t.Fatal("denied response disclosed upload metadata")
						}
					}
				}
				if allowed && action == "delete" {
					wantRows, wantFiles = 0, 1
					if _, ok := f.store.GetUpload(f.upload.ID); ok {
						t.Fatal("authorized delete left upload")
					}
					if _, err := os.Stat(filepath.Join(f.handler.UploadMediaDir, f.upload.Metadata["sourceObjectKey"])); !os.IsNotExist(err) {
						t.Fatal("authorized delete left source")
					}
				} else {
					f.requireUnchanged(t, f.upload, f.source)
				}
				if allowed && (action == "list" || action == "detail") {
					var response uploadResponse
					if action == "list" {
						var listed []uploadResponse
						if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil || len(listed) != 1 {
							t.Fatal("authorized list did not return exactly its upload")
						}
						response = listed[0]
					} else if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
						t.Fatalf("decode detail: %v", err)
					}
					if response.ID != f.upload.ID || response.Metadata["mediaToken"] != f.upload.Metadata["mediaToken"] || response.Metadata["sourceObjectKey"] != f.upload.Metadata["sourceObjectKey"] {
						t.Fatal("authorized response omitted its actual source capability")
					}
				}
				if allowed && strings.HasPrefix(action, "create_") {
					wantRows = 2
					var response uploadResponse
					if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
						t.Fatalf("decode created upload: %v", err)
					}
					created, ok := f.store.GetUpload(response.ID)
					if !ok || response.ID == f.upload.ID || created.ChannelID != f.channel.ID || created.Metadata["label"] != "authorized" || !reflect.DeepEqual(enqueue.ids, []string{response.ID}) {
						t.Fatal("authorized create did not persist/enqueue the requested upload")
					}
					if action == "create_multipart" {
						wantFiles = 3
						if created.Metadata["mediaToken"] == "" || created.Metadata["mediaToken"] == f.upload.Metadata["mediaToken"] {
							t.Fatal("new upload capability missing or reused")
						}
						f.requireSource(t, created, validMP4Sample())
					} else if created.Metadata["sourceUrl"] != "https://example.com/source.mp4" {
						t.Fatal("authorized external source changed")
					}
				} else if len(enqueue.ids) != 0 {
					t.Fatal("denied/read-only/delete action enqueued work")
				}
				rows, err := f.store.ListUploads(f.channel.ID)
				if err != nil || len(rows) != wantRows {
					t.Fatal("unexpected upload row count")
				}
				files, err := os.ReadDir(f.handler.UploadMediaDir)
				if err != nil || len(files) != wantFiles {
					t.Fatal("unexpected source count or orphaned temporary file")
				}
				for _, file := range files {
					if strings.HasPrefix(file.Name(), "pending-upload-") {
						t.Fatal("request left a pending file")
					}
				}
				for _, channel := range []domain.Channel{f.channel, f.otherChannel} {
					if after, ok := f.store.GetChannel(channel.ID); !ok || !reflect.DeepEqual(channel, after) {
						t.Fatal("upload action changed channel")
					}
				}
				f.requireUnchanged(t, f.otherUpload, f.otherSource)
			})
		}
	}
}

func TestUploadMediaCapabilityMatrix(t *testing.T) {
	for _, principal := range uploadAuthorizationPrincipals {
		for _, capability := range []string{"missing", "wrong", "other_upload", "valid"} {
			t.Run(principal.name+"/"+capability, func(t *testing.T) {
				f := newUploadAuthorizationFixture(t)
				path := "/api/uploads/" + f.upload.ID + "/media"
				token := ""
				switch capability {
				case "wrong":
					token = "fixture-invalid-capability"
				case "other_upload":
					token = f.otherUpload.Metadata["mediaToken"]
				case "valid":
					token = f.upload.Metadata["mediaToken"]
				}
				if token != "" {
					path += "?token=" + url.QueryEscape(token)
				}
				rec := httptest.NewRecorder()
				f.handler.UploadByID(rec, f.requestAs(t, httptest.NewRequest(http.MethodGet, path, nil), principal))
				wantStatus := http.StatusForbidden
				if capability == "valid" {
					wantStatus = http.StatusOK
				}
				if rec.Code != wantStatus {
					t.Fatalf("status=%d, want %d", rec.Code, wantStatus)
				}
				if capability == "valid" {
					if !bytes.Equal(rec.Body.Bytes(), f.source) {
						t.Fatal("valid bearer did not return the correct source")
					}
				} else {
					if bytes.Contains(rec.Body.Bytes(), f.source) || strings.Contains(rec.Body.String(), f.upload.Metadata["mediaToken"]) || strings.Contains(rec.Body.String(), f.otherUpload.Metadata["mediaToken"]) {
						t.Fatal("denied capability leaked source bytes or token")
					}
				}
				f.requireUnchanged(t, f.upload, f.source)
				f.requireUnchanged(t, f.otherUpload, f.otherSource)
			})
		}
	}
}
