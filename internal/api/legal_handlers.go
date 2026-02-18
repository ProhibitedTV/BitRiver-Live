package api

import (
	"net/http"
	"strings"

	"bitriver-live/internal/domain"
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
		rec, err := h.legalService().CreateDMCACase(domain.DMCACaseCreateParams{ReporterName: req.ReporterName, ReporterEmail: req.ReporterEmail, ContentURL: req.ContentURL, Description: req.Description})
		if err != nil {
			WriteRequestError(w, err)
			return
		}
		WriteJSON(w, http.StatusCreated, rec)
	case http.MethodGet:
		if _, ok := h.requireRole(w, r, roleAdmin); !ok {
			return
		}
		rows, err := h.legalService().ListDMCACases()
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
		rec, err := h.legalService().UpdateDMCACase(id, req.Status, req.Notes, actor.ID)
		if err != nil {
			WriteRequestError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, rec)
	case http.MethodGet:
		rec, ok := h.legalService().GetDMCACase(id)
		if !ok {
			WriteError(w, http.StatusNotFound, errNotFound("dmca"))
			return
		}
		history, _ := h.legalService().ListLegalStateHistory("dmca", id)
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
		rec, err := h.legalService().CreateDataSubjectRequest(domain.DataSubjectRequestCreateParams{SubjectEmail: req.SubjectEmail, RequestType: req.RequestType, Notes: req.Notes})
		if err != nil {
			WriteRequestError(w, err)
			return
		}
		WriteJSON(w, http.StatusCreated, rec)
	case http.MethodGet:
		rows, err := h.legalService().ListDataSubjectRequests()
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
			evt, err := h.legalService().AddDataSubjectAuditEvent(requestID, domain.DataSubjectAuditEventCreateParams{ActorUserID: actor.ID, Action: req.Action, Details: req.Details, EvidenceRef: req.EvidenceRef})
			if err != nil {
				WriteRequestError(w, err)
				return
			}
			WriteJSON(w, http.StatusCreated, evt)
			return
		}
		if r.Method == http.MethodGet {
			events, err := h.legalService().ListDataSubjectAuditEvents(requestID)
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
		rec, err := h.legalService().UpdateDataSubjectRequest(id, req.Status, req.Notes, actor.ID)
		if err != nil {
			WriteRequestError(w, err)
			return
		}
		WriteJSON(w, http.StatusOK, rec)
	case http.MethodGet:
		rec, ok := h.legalService().GetDataSubjectRequest(id)
		if !ok {
			WriteError(w, http.StatusNotFound, errNotFound("request"))
			return
		}
		history, _ := h.legalService().ListLegalStateHistory("data_subject", id)
		audit, _ := h.legalService().ListDataSubjectAuditEvents(id)
		WriteJSON(w, http.StatusOK, map[string]any{"request": rec, "history": history, "audit": audit})
	default:
		WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPatch)
	}
}

func errNotFound(resource string) error {
	return &RequestError{Status: http.StatusNotFound, Message: resource + " not found"}
}

var _ = domain.DMCACase{}
