package storage

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"bitriver-live/internal/models"
)

func (s *Storage) CreateDMCACase(params CreateDMCACaseParams) (models.DMCACase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDatasetInitializedLocked()
	if strings.TrimSpace(params.ReporterName) == "" || strings.TrimSpace(params.ReporterEmail) == "" || strings.TrimSpace(params.ContentURL) == "" {
		return models.DMCACase{}, fmt.Errorf("reporter name, reporter email, and content url are required")
	}
	id, err := generateID()
	if err != nil {
		return models.DMCACase{}, err
	}
	now := time.Now().UTC()
	rec := models.DMCACase{ID: id, ReporterName: strings.TrimSpace(params.ReporterName), ReporterEmail: strings.TrimSpace(params.ReporterEmail), ContentURL: strings.TrimSpace(params.ContentURL), Description: strings.TrimSpace(params.Description), Status: models.DMCACaseStatusOpen, CreatedAt: now, UpdatedAt: now}
	updated := cloneDataset(s.data)
	updated.DMCACases[id] = rec
	s.appendLegalHistoryLocked(&updated, "dmca", id, "", rec.Status, "", "case submitted")
	if err := s.persistDataset(updated); err != nil {
		return models.DMCACase{}, err
	}
	s.data = updated
	return rec, nil
}

func (s *Storage) ListDMCACases() ([]models.DMCACase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.ensureDatasetInitializedLocked()
	res := make([]models.DMCACase, 0, len(s.data.DMCACases))
	for _, item := range s.data.DMCACases {
		res = append(res, item)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].CreatedAt.After(res[j].CreatedAt) })
	return res, nil
}

func (s *Storage) GetDMCACase(id string) (models.DMCACase, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.ensureDatasetInitializedLocked()
	rec, ok := s.data.DMCACases[strings.TrimSpace(id)]
	return rec, ok
}

func (s *Storage) UpdateDMCACase(id string, update DMCACaseUpdate, actorUserID string) (models.DMCACase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDatasetInitializedLocked()
	id = strings.TrimSpace(id)
	rec, ok := s.data.DMCACases[id]
	if !ok {
		return models.DMCACase{}, fmt.Errorf("dmca case not found")
	}
	updated := cloneDataset(s.data)
	if update.Notes != nil {
		rec.Notes = strings.TrimSpace(*update.Notes)
	}
	if update.Status != nil {
		to := strings.ToLower(strings.TrimSpace(*update.Status))
		from := rec.Status
		rec.Status = to
		now := time.Now().UTC()
		rec.UpdatedAt = now
		switch to {
		case models.DMCACaseStatusActioned:
			rec.ActionedAt = &now
		case models.DMCACaseStatusRestored:
			rec.RestoredAt = &now
		case models.DMCACaseStatusRejected:
			rec.RejectedAt = &now
		}
		s.appendLegalHistoryLocked(&updated, "dmca", id, from, to, actorUserID, rec.Notes)
	}
	rec.UpdatedAt = time.Now().UTC()
	updated.DMCACases[id] = rec
	if err := s.persistDataset(updated); err != nil {
		return models.DMCACase{}, err
	}
	s.data = updated
	return rec, nil
}

func (s *Storage) CreateDataSubjectRequest(params CreateDataSubjectRequestParams) (models.DataSubjectRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDatasetInitializedLocked()
	if strings.TrimSpace(params.SubjectEmail) == "" {
		return models.DataSubjectRequest{}, fmt.Errorf("subject email is required")
	}
	reqType := strings.ToLower(strings.TrimSpace(params.RequestType))
	if reqType != models.DataSubjectRequestTypeExport && reqType != models.DataSubjectRequestTypeDelete {
		return models.DataSubjectRequest{}, fmt.Errorf("invalid request type")
	}
	id, err := generateID()
	if err != nil {
		return models.DataSubjectRequest{}, err
	}
	now := time.Now().UTC()
	rec := models.DataSubjectRequest{ID: id, SubjectEmail: strings.TrimSpace(params.SubjectEmail), RequestType: reqType, Status: models.DataSubjectRequestStatusOpen, Notes: strings.TrimSpace(params.Notes), CreatedAt: now, UpdatedAt: now}
	updated := cloneDataset(s.data)
	updated.DataSubjectRequests[id] = rec
	s.appendLegalHistoryLocked(&updated, "data_subject", id, "", rec.Status, "", "request submitted")
	if err := s.persistDataset(updated); err != nil {
		return models.DataSubjectRequest{}, err
	}
	s.data = updated
	return rec, nil
}

func (s *Storage) ListDataSubjectRequests() ([]models.DataSubjectRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.ensureDatasetInitializedLocked()
	out := make([]models.DataSubjectRequest, 0, len(s.data.DataSubjectRequests))
	for _, v := range s.data.DataSubjectRequests {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Storage) GetDataSubjectRequest(id string) (models.DataSubjectRequest, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.ensureDatasetInitializedLocked()
	rec, ok := s.data.DataSubjectRequests[strings.TrimSpace(id)]
	return rec, ok
}

func (s *Storage) UpdateDataSubjectRequest(id string, update DataSubjectRequestUpdate, actorUserID string) (models.DataSubjectRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDatasetInitializedLocked()
	id = strings.TrimSpace(id)
	rec, ok := s.data.DataSubjectRequests[id]
	if !ok {
		return models.DataSubjectRequest{}, fmt.Errorf("data subject request not found")
	}
	updated := cloneDataset(s.data)
	if update.Notes != nil {
		rec.Notes = strings.TrimSpace(*update.Notes)
	}
	if update.Status != nil {
		from := rec.Status
		rec.Status = strings.ToLower(strings.TrimSpace(*update.Status))
		s.appendLegalHistoryLocked(&updated, "data_subject", id, from, rec.Status, actorUserID, rec.Notes)
	}
	rec.UpdatedAt = time.Now().UTC()
	updated.DataSubjectRequests[id] = rec
	if err := s.persistDataset(updated); err != nil {
		return models.DataSubjectRequest{}, err
	}
	s.data = updated
	return rec, nil
}

func (s *Storage) AddDataSubjectAuditEvent(requestID string, params CreateDataSubjectAuditEventParams) (models.DataSubjectAuditEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDatasetInitializedLocked()
	requestID = strings.TrimSpace(requestID)
	if _, ok := s.data.DataSubjectRequests[requestID]; !ok {
		return models.DataSubjectAuditEvent{}, fmt.Errorf("data subject request not found")
	}
	if strings.TrimSpace(params.Action) == "" {
		return models.DataSubjectAuditEvent{}, fmt.Errorf("action is required")
	}
	id, err := generateID()
	if err != nil {
		return models.DataSubjectAuditEvent{}, err
	}
	evt := models.DataSubjectAuditEvent{ID: id, RequestID: requestID, ActorUserID: strings.TrimSpace(params.ActorUserID), Action: strings.TrimSpace(params.Action), Details: strings.TrimSpace(params.Details), EvidenceRef: strings.TrimSpace(params.EvidenceRef), OccurredAtUTC: time.Now().UTC()}
	updated := cloneDataset(s.data)
	updated.DataSubjectAudit[requestID] = append(updated.DataSubjectAudit[requestID], evt)
	if err := s.persistDataset(updated); err != nil {
		return models.DataSubjectAuditEvent{}, err
	}
	s.data = updated
	return evt, nil
}

func (s *Storage) ListDataSubjectAuditEvents(requestID string) ([]models.DataSubjectAuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.ensureDatasetInitializedLocked()
	requestID = strings.TrimSpace(requestID)
	items := append([]models.DataSubjectAuditEvent(nil), s.data.DataSubjectAudit[requestID]...)
	sort.Slice(items, func(i, j int) bool { return items[i].OccurredAtUTC.Before(items[j].OccurredAtUTC) })
	return items, nil
}

func (s *Storage) ListLegalStateHistory(entityType, entityID string) ([]models.LegalStateHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.ensureDatasetInitializedLocked()
	entityType = strings.TrimSpace(strings.ToLower(entityType))
	entityID = strings.TrimSpace(entityID)
	out := make([]models.LegalStateHistory, 0)
	for _, item := range s.data.LegalStateHistory {
		if entityType != "" && item.EntityType != entityType {
			continue
		}
		if entityID != "" && item.EntityID != entityID {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *Storage) appendLegalHistoryLocked(updated *dataset, entityType, entityID, fromState, toState, actorUserID, reason string) {
	if updated == nil {
		return
	}
	id, err := generateID()
	if err != nil {
		return
	}
	updated.LegalStateHistory = append(updated.LegalStateHistory, models.LegalStateHistory{ID: id, EntityType: entityType, EntityID: entityID, FromState: fromState, ToState: toState, ActorUserID: strings.TrimSpace(actorUserID), Reason: strings.TrimSpace(reason), CreatedAt: time.Now().UTC()})
}
