package service

import (
	"fmt"
	"strings"

	"bitriver-live/internal/models"
	"bitriver-live/internal/storage"
)

type LegalService struct {
	repo storage.Repository
}

func NewLegalService(repo storage.Repository) *LegalService {
	return &LegalService{repo: repo}
}

func (s *LegalService) CreateDMCACase(params storage.CreateDMCACaseParams) (models.DMCACase, error) {
	if s == nil || s.repo == nil {
		return models.DMCACase{}, fmt.Errorf("legal service unavailable")
	}
	return s.repo.CreateDMCACase(params)
}

func (s *LegalService) UpdateDMCACase(id, status, notes, actorUserID string) (models.DMCACase, error) {
	if s == nil || s.repo == nil {
		return models.DMCACase{}, fmt.Errorf("legal service unavailable")
	}
	status = strings.TrimSpace(strings.ToLower(status))
	existing, ok := s.repo.GetDMCACase(id)
	if !ok {
		return models.DMCACase{}, fmt.Errorf("dmca case not found")
	}
	if !validDMCAStateTransition(existing.Status, status) {
		return models.DMCACase{}, fmt.Errorf("invalid dmca status transition from %s to %s", existing.Status, status)
	}
	return s.repo.UpdateDMCACase(id, storage.DMCACaseUpdate{Status: &status, Notes: &notes}, actorUserID)
}

func (s *LegalService) CreateDataSubjectRequest(params storage.CreateDataSubjectRequestParams) (models.DataSubjectRequest, error) {
	if s == nil || s.repo == nil {
		return models.DataSubjectRequest{}, fmt.Errorf("legal service unavailable")
	}
	return s.repo.CreateDataSubjectRequest(params)
}

func (s *LegalService) UpdateDataSubjectRequest(id, status, notes, actorUserID string) (models.DataSubjectRequest, error) {
	if s == nil || s.repo == nil {
		return models.DataSubjectRequest{}, fmt.Errorf("legal service unavailable")
	}
	status = strings.TrimSpace(strings.ToLower(status))
	existing, ok := s.repo.GetDataSubjectRequest(id)
	if !ok {
		return models.DataSubjectRequest{}, fmt.Errorf("data subject request not found")
	}
	if !validDataSubjectTransition(existing.Status, status) {
		return models.DataSubjectRequest{}, fmt.Errorf("invalid request status transition from %s to %s", existing.Status, status)
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
	case models.DMCACaseStatusOpen:
		return to == models.DMCACaseStatusActioned || to == models.DMCACaseStatusRejected
	case models.DMCACaseStatusActioned:
		return to == models.DMCACaseStatusRestored
	case models.DMCACaseStatusRejected, models.DMCACaseStatusRestored:
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
	if from == models.DataSubjectRequestStatusOpen {
		return to == models.DataSubjectRequestStatusActioned || to == models.DataSubjectRequestStatusRejected
	}
	return false
}
