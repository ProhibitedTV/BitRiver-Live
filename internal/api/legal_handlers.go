package api

import (
	"net/http"
	"strings"

	"bitriver-live/internal/models"
	"bitriver-live/internal/storage"
)

type dmcaCaseRequest struct {
	ReporterName  string `json:"reporterName"`
	ReporterEmail string `json:"reporterEmail"`
	ContentURL    string `json:"contentUrl"`
	Description   string `json:"description"`
}

type dmcaUpdateRequest struct {
	Status string `json:"status"`
	Notes  string `json:"notes"`
}

type dataSubjectRequestPayload struct {
	SubjectEmail string `json:"subjectEmail"`
	RequestType  string `json:"requestType"`
	Notes        string `json:"notes"`
}

type dataSubjectUpdatePayload struct {
	Status string `json:"status"`
	Notes  string `json:"notes"`
}

type dataSubjectAuditPayload struct {
	Action      string `json:"action"`
	Details     string `json:"details"`
	EvidenceRef string `json:"evidenceRef"`
}

func (h *Handler) LegalDMCA(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req dmcaCaseRequest
		if !DecodeAndValidate(w, r, &req) {
			return
		}
		rec, err := h.LegalService.CreateDMCACase(storage.CreateDMCACaseParams{ReporterName: req.ReporterName, ReporterEmail: req.ReporterEmail, ContentURL: req.ContentURL, Description: req.Description})
		if err != nil {
			WriteRequestError(w, err)
			return
		}
		WriteJSON(w, http.StatusCreated, rec)
	case http.MethodGet:
		if _, ok := h.requireRole(w, r, roleAdmin); !ok {
			return
		}
		rows, err := h.Store.ListDMCACases()
		if err != nil {
			WriteError(w, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, rows)
	default:
		WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) LegalDMCAByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireRole(w, r, roleAdmin); !ok {
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/legal/dmca/")
	if id == "" {
		WriteError(w, http.StatusNotFound, errNotFound("dmca"))
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var req dmcaUpdateRequest
		if !DecodeAndValidate(w, r, &req) {
			return
		}
		actor, _ := h.requireAuthenticatedUser(w, r)
		rec, err := h.LegalService.UpdateDMCACase(id, req.Status, req.Notes, actor.ID)
		if err != nil {
			WriteRequestError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, rec)
	case http.MethodGet:
		rec, ok := h.Store.GetDMCACase(id)
		if !ok {
			WriteError(w, http.StatusNotFound, errNotFound("dmca"))
			return
		}
		history, _ := h.Store.ListLegalStateHistory("dmca", id)
		WriteJSON(w, http.StatusOK, map[string]any{"case": rec, "history": history})
	default:
		WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPatch)
	}
}

func (h *Handler) LegalDataSubject(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireRole(w, r, roleAdmin); !ok {
		return
	}
	switch r.Method {
	case http.MethodPost:
		var req dataSubjectRequestPayload
		if !DecodeAndValidate(w, r, &req) {
			return
		}
		rec, err := h.LegalService.CreateDataSubjectRequest(storage.CreateDataSubjectRequestParams{SubjectEmail: req.SubjectEmail, RequestType: req.RequestType, Notes: req.Notes})
		if err != nil {
			WriteRequestError(w, err)
			return
		}
		WriteJSON(w, http.StatusCreated, rec)
	case http.MethodGet:
		rows, err := h.Store.ListDataSubjectRequests()
		if err != nil {
			WriteError(w, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, rows)
	default:
		WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) LegalDataSubjectByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireRole(w, r, roleAdmin); !ok {
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/legal/data-subject/")
	id = strings.Trim(id, "/")
	if id == "" {
		WriteError(w, http.StatusNotFound, errNotFound("request"))
		return
	}
	if strings.HasSuffix(r.URL.Path, "/audit") {
		requestID := strings.TrimSuffix(id, "/audit")
		if r.Method == http.MethodPost {
			var req dataSubjectAuditPayload
			if !DecodeAndValidate(w, r, &req) {
				return
			}
			actor, _ := h.requireAuthenticatedUser(w, r)
			evt, err := h.Store.AddDataSubjectAuditEvent(requestID, storage.CreateDataSubjectAuditEventParams{ActorUserID: actor.ID, Action: req.Action, Details: req.Details, EvidenceRef: req.EvidenceRef})
			if err != nil {
				WriteRequestError(w, err)
				return
			}
			WriteJSON(w, http.StatusCreated, evt)
			return
		}
		if r.Method == http.MethodGet {
			events, err := h.Store.ListDataSubjectAuditEvents(requestID)
			if err != nil {
				WriteError(w, http.StatusInternalServerError, err)
				return
			}
			WriteJSON(w, http.StatusOK, events)
			return
		}
	}
	switch r.Method {
	case http.MethodPatch:
		var req dataSubjectUpdatePayload
		if !DecodeAndValidate(w, r, &req) {
			return
		}
		actor, _ := h.requireAuthenticatedUser(w, r)
		rec, err := h.LegalService.UpdateDataSubjectRequest(id, req.Status, req.Notes, actor.ID)
		if err != nil {
			WriteRequestError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, rec)
	case http.MethodGet:
		rec, ok := h.Store.GetDataSubjectRequest(id)
		if !ok {
			WriteError(w, http.StatusNotFound, errNotFound("request"))
			return
		}
		history, _ := h.Store.ListLegalStateHistory("data_subject", id)
		audit, _ := h.Store.ListDataSubjectAuditEvents(id)
		WriteJSON(w, http.StatusOK, map[string]any{"request": rec, "history": history, "audit": audit})
	default:
		WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPatch)
	}
}

func errNotFound(resource string) error {
	return &RequestError{Status: http.StatusNotFound, Message: resource + " not found"}
}

var _ = models.DMCACase{}
