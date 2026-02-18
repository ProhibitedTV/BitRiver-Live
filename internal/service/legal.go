package service

import (
	"fmt"
	"strings"

	"bitriver-live/internal/domain"
)

type legalRepository interface {
	domain.LegalRepository
	ListDMCACases() ([]domain.DMCACase, error)
	ListDataSubjectRequests() ([]domain.DataSubjectRequest, error)
	AddDataSubjectAuditEvent(requestID string, params domain.DataSubjectAuditEventCreateParams) (domain.DataSubjectAuditEvent, error)
	ListDataSubjectAuditEvents(requestID string) ([]domain.DataSubjectAuditEvent, error)
	ListLegalStateHistory(entityType, entityID string) ([]domain.LegalStateHistory, error)
}

type LegalService struct {
	repo legalRepository
}

func NewLegalService(repo legalRepository) *LegalService {
	return &LegalService{repo: repo}
}

func (s *LegalService) CreateDMCACase(params domain.DMCACaseCreateParams) (domain.DMCACase, error) {
	if s == nil || s.repo == nil {
		return domain.DMCACase{}, fmt.Errorf("legal service unavailable")
	}
	return s.repo.CreateDMCACase(params)
}

func (s *LegalService) UpdateDMCACase(id, status, notes, actorUserID string) (domain.DMCACase, error) {
	if s == nil || s.repo == nil {
		return domain.DMCACase{}, fmt.Errorf("legal service unavailable")
	}
	status = strings.TrimSpace(strings.ToLower(status))
	existing, ok := s.repo.GetDMCACase(id)
	if !ok {
		return domain.DMCACase{}, fmt.Errorf("dmca case not found")
	}
	if !validDMCAStateTransition(existing.Status, status) {
		return domain.DMCACase{}, fmt.Errorf("invalid dmca status transition from %s to %s", existing.Status, status)
	}
	return s.repo.UpdateDMCACase(id, domain.DMCACaseUpdate{Status: &status, Notes: &notes}, actorUserID)
}

func (s *LegalService) CreateDataSubjectRequest(params domain.DataSubjectRequestCreateParams) (domain.DataSubjectRequest, error) {
	if s == nil || s.repo == nil {
		return domain.DataSubjectRequest{}, fmt.Errorf("legal service unavailable")
	}
	return s.repo.CreateDataSubjectRequest(params)
}

func (s *LegalService) UpdateDataSubjectRequest(id, status, notes, actorUserID string) (domain.DataSubjectRequest, error) {
	if s == nil || s.repo == nil {
		return domain.DataSubjectRequest{}, fmt.Errorf("legal service unavailable")
	}
	status = strings.TrimSpace(strings.ToLower(status))
	existing, ok := s.repo.GetDataSubjectRequest(id)
	if !ok {
		return domain.DataSubjectRequest{}, fmt.Errorf("data subject request not found")
	}
	if !validDataSubjectTransition(existing.Status, status) {
		return domain.DataSubjectRequest{}, fmt.Errorf("invalid request status transition from %s to %s", existing.Status, status)
	}
	return s.repo.UpdateDataSubjectRequest(id, domain.DataSubjectRequestUpdate{Status: &status, Notes: &notes}, actorUserID)
}

func (s *LegalService) ListDMCACases() ([]domain.DMCACase, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("legal service unavailable")
	}
	return s.repo.ListDMCACases()
}

func (s *LegalService) GetDMCACase(id string) (domain.DMCACase, bool) {
	if s == nil || s.repo == nil {
		return domain.DMCACase{}, false
	}
	return s.repo.GetDMCACase(id)
}

func (s *LegalService) ListDataSubjectRequests() ([]domain.DataSubjectRequest, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("legal service unavailable")
	}
	return s.repo.ListDataSubjectRequests()
}

func (s *LegalService) GetDataSubjectRequest(id string) (domain.DataSubjectRequest, bool) {
	if s == nil || s.repo == nil {
		return domain.DataSubjectRequest{}, false
	}
	return s.repo.GetDataSubjectRequest(id)
}

func (s *LegalService) AddDataSubjectAuditEvent(requestID string, params domain.DataSubjectAuditEventCreateParams) (domain.DataSubjectAuditEvent, error) {
	if s == nil || s.repo == nil {
		return domain.DataSubjectAuditEvent{}, fmt.Errorf("legal service unavailable")
	}
	return s.repo.AddDataSubjectAuditEvent(requestID, params)
}

func (s *LegalService) ListDataSubjectAuditEvents(requestID string) ([]domain.DataSubjectAuditEvent, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("legal service unavailable")
	}
	return s.repo.ListDataSubjectAuditEvents(requestID)
}

func (s *LegalService) ListLegalStateHistory(entityType, entityID string) ([]domain.LegalStateHistory, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("legal service unavailable")
	}
	return s.repo.ListLegalStateHistory(entityType, entityID)
}

func validDMCAStateTransition(from, to string) bool {
	if to == "" {
		return false
	}
	from = strings.ToLower(strings.TrimSpace(from))
	to = strings.ToLower(strings.TrimSpace(to))
	if from == to {
		return true
	}
	switch from {
	case domain.DMCACaseStatusOpen:
		return to == domain.DMCACaseStatusActioned || to == domain.DMCACaseStatusRejected
	case domain.DMCACaseStatusActioned:
		return to == domain.DMCACaseStatusRestored
	case domain.DMCACaseStatusRejected, domain.DMCACaseStatusRestored:
		return false
	default:
		return false
	}
}

func validDataSubjectTransition(from, to string) bool {
	if to == "" {
		return false
	}
	from = strings.ToLower(strings.TrimSpace(from))
	to = strings.ToLower(strings.TrimSpace(to))
	if from == to {
		return true
	}
	if from == domain.DataSubjectRequestStatusOpen {
		return to == domain.DataSubjectRequestStatusActioned || to == domain.DataSubjectRequestStatusRejected
	}
	return false
}
