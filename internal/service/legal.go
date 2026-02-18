package service

import (
	"fmt"
	"strings"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/storage"
)

type LegalService struct {
	repo storage.Repository
}

func NewLegalService(repo storage.Repository) *LegalService {
	return &LegalService{repo: repo}
}

func (s *LegalService) CreateDMCACase(params storage.CreateDMCACaseParams) (domain.DMCACase, error) {
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
	return s.repo.UpdateDMCACase(id, storage.DMCACaseUpdate{Status: &status, Notes: &notes}, actorUserID)
}

func (s *LegalService) CreateDataSubjectRequest(params storage.CreateDataSubjectRequestParams) (domain.DataSubjectRequest, error) {
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
	return s.repo.UpdateDataSubjectRequest(id, storage.DataSubjectRequestUpdate{Status: &status, Notes: &notes}, actorUserID)
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
