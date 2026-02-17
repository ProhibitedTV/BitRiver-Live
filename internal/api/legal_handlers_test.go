package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bitriver-live/internal/storage"
)

func TestLegalDMCAWorkflow(t *testing.T) {
	h, store := newTestHandler(t)
	body := bytes.NewBufferString(`{"reporterName":"Alice","reporterEmail":"alice@example.com","contentUrl":"https://example.com/vod/1","description":"unauthorized"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/legal/dmca", body)
	h.LegalDMCA(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var created map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("expected dmca id")
	}

	admin, _ := store.CreateUser(storage.CreateUserParams{DisplayName: "Admin", Email: "admin@example.com", Password: "password123", Roles: []string{"admin"}})
	patch := httptest.NewRecorder()
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/legal/dmca/"+id, bytes.NewBufferString(`{"status":"actioned","notes":"removed"}`))
	patchReq = withUser(patchReq, admin)
	h.LegalDMCAByID(patch, patchReq)
	if patch.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", patch.Code)
	}
}

func TestLegalDataSubjectRequiresAdmin(t *testing.T) {
	h, store := newTestHandler(t)
	viewer, _ := store.CreateUser(storage.CreateUserParams{DisplayName: "Viewer", Email: "viewer@example.com", Password: "password123", Roles: []string{"viewer"}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/legal/data-subject", nil)
	req = withUser(req, viewer)
	h.LegalDataSubject(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}
