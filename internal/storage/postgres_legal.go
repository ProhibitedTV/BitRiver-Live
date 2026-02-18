package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"bitriver-live/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

func (r *postgresRepository) CreateDMCACase(params CreateDMCACaseParams) (domain.DMCACase, error) {
	if strings.TrimSpace(params.ReporterName) == "" || strings.TrimSpace(params.ReporterEmail) == "" || strings.TrimSpace(params.ContentURL) == "" {
		return domain.DMCACase{}, fmt.Errorf("reporter name, reporter email, and content url are required")
	}
	id, err := generateID()
	if err != nil {
		return domain.DMCACase{}, err
	}
	now := time.Now().UTC()
	err = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		_, err := conn.Exec(ctx, `INSERT INTO legal_dmca_cases (id, reporter_name, reporter_email, content_url, description, status, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, id, strings.TrimSpace(params.ReporterName), strings.TrimSpace(params.ReporterEmail), strings.TrimSpace(params.ContentURL), strings.TrimSpace(params.Description), domain.DMCACaseStatusOpen, now, now)
		if err != nil {
			return err
		}
		_, err = conn.Exec(ctx, `INSERT INTO legal_state_history (id, entity_type, entity_id, from_state, to_state, actor_user_id, reason, created_at) VALUES ($1,'dmca',$2,'',$3,'',$4,$5)`, mustID(), id, domain.DMCACaseStatusOpen, "case submitted", now)
		return err
	})
	if err != nil {
		return domain.DMCACase{}, err
	}
	return domain.DMCACase{ID: id, ReporterName: strings.TrimSpace(params.ReporterName), ReporterEmail: strings.TrimSpace(params.ReporterEmail), ContentURL: strings.TrimSpace(params.ContentURL), Description: strings.TrimSpace(params.Description), Status: domain.DMCACaseStatusOpen, CreatedAt: now, UpdatedAt: now}, nil
}

func mustID() string { id, _ := generateID(); return id }

func (r *postgresRepository) ListDMCACases() ([]domain.DMCACase, error) {
	rows, err := r.pool.Query(context.Background(), `SELECT id, reporter_name, reporter_email, content_url, description, status, notes, actioned_at, restored_at, rejected_at, created_at, updated_at FROM legal_dmca_cases ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.DMCACase{}
	for rows.Next() {
		var rec domain.DMCACase
		if err := rows.Scan(&rec.ID, &rec.ReporterName, &rec.ReporterEmail, &rec.ContentURL, &rec.Description, &rec.Status, &rec.Notes, &rec.ActionedAt, &rec.RestoredAt, &rec.RejectedAt, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *postgresRepository) GetDMCACase(id string) (domain.DMCACase, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.DMCACase{}, false
	}
	row := r.pool.QueryRow(context.Background(), `SELECT id, reporter_name, reporter_email, content_url, description, status, notes, actioned_at, restored_at, rejected_at, created_at, updated_at FROM legal_dmca_cases WHERE id=$1`, id)
	var rec domain.DMCACase
	if err := row.Scan(&rec.ID, &rec.ReporterName, &rec.ReporterEmail, &rec.ContentURL, &rec.Description, &rec.Status, &rec.Notes, &rec.ActionedAt, &rec.RestoredAt, &rec.RejectedAt, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return domain.DMCACase{}, false
	}
	return rec, true
}

func (r *postgresRepository) UpdateDMCACase(id string, update DMCACaseUpdate, actorUserID string) (domain.DMCACase, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.DMCACase{}, fmt.Errorf("dmca case id required")
	}
	status := ""
	if update.Status != nil {
		status = strings.ToLower(strings.TrimSpace(*update.Status))
	}
	notes := ""
	if update.Notes != nil {
		notes = strings.TrimSpace(*update.Notes)
	}
	_, err := r.pool.Exec(context.Background(), `UPDATE legal_dmca_cases SET status=COALESCE(NULLIF($2,''),status), notes=COALESCE($3,notes), updated_at=NOW(), actioned_at=CASE WHEN $2='actioned' THEN NOW() ELSE actioned_at END, restored_at=CASE WHEN $2='restored' THEN NOW() ELSE restored_at END, rejected_at=CASE WHEN $2='rejected' THEN NOW() ELSE rejected_at END WHERE id=$1`, id, status, notes)
	if err != nil {
		return domain.DMCACase{}, err
	}
	if status != "" {
		_, _ = r.pool.Exec(context.Background(), `INSERT INTO legal_state_history (id, entity_type, entity_id, from_state, to_state, actor_user_id, reason, created_at) VALUES ($1,'dmca',$2,'',$3,$4,$5,NOW())`, mustID(), id, status, strings.TrimSpace(actorUserID), notes)
	}
	rec, ok := r.GetDMCACase(id)
	if !ok {
		return domain.DMCACase{}, fmt.Errorf("dmca case not found")
	}
	return rec, nil
}

func (r *postgresRepository) CreateDataSubjectRequest(params CreateDataSubjectRequestParams) (domain.DataSubjectRequest, error) {
	id, err := generateID()
	if err != nil {
		return domain.DataSubjectRequest{}, err
	}
	reqType := strings.ToLower(strings.TrimSpace(params.RequestType))
	now := time.Now().UTC()
	_, err = r.pool.Exec(context.Background(), `INSERT INTO legal_data_subject_requests (id, subject_email, request_type, status, notes, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, id, strings.TrimSpace(params.SubjectEmail), reqType, domain.DataSubjectRequestStatusOpen, strings.TrimSpace(params.Notes), now, now)
	if err != nil {
		return domain.DataSubjectRequest{}, err
	}
	_, _ = r.pool.Exec(context.Background(), `INSERT INTO legal_state_history (id, entity_type, entity_id, from_state, to_state, actor_user_id, reason, created_at) VALUES ($1,'data_subject',$2,'',$3,'',$4,$5)`, mustID(), id, domain.DataSubjectRequestStatusOpen, "request submitted", now)
	return domain.DataSubjectRequest{ID: id, SubjectEmail: strings.TrimSpace(params.SubjectEmail), RequestType: reqType, Status: domain.DataSubjectRequestStatusOpen, Notes: strings.TrimSpace(params.Notes), CreatedAt: now, UpdatedAt: now}, nil
}

func (r *postgresRepository) ListDataSubjectRequests() ([]domain.DataSubjectRequest, error) {
	rows, err := r.pool.Query(context.Background(), `SELECT id, subject_email, request_type, status, notes, created_at, updated_at FROM legal_data_subject_requests ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.DataSubjectRequest{}
	for rows.Next() {
		var rec domain.DataSubjectRequest
		if err := rows.Scan(&rec.ID, &rec.SubjectEmail, &rec.RequestType, &rec.Status, &rec.Notes, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *postgresRepository) GetDataSubjectRequest(id string) (domain.DataSubjectRequest, bool) {
	row := r.pool.QueryRow(context.Background(), `SELECT id, subject_email, request_type, status, notes, created_at, updated_at FROM legal_data_subject_requests WHERE id=$1`, strings.TrimSpace(id))
	var rec domain.DataSubjectRequest
	if err := row.Scan(&rec.ID, &rec.SubjectEmail, &rec.RequestType, &rec.Status, &rec.Notes, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return domain.DataSubjectRequest{}, false
	}
	return rec, true
}

func (r *postgresRepository) UpdateDataSubjectRequest(id string, update DataSubjectRequestUpdate, actorUserID string) (domain.DataSubjectRequest, error) {
	status := ""
	if update.Status != nil {
		status = strings.ToLower(strings.TrimSpace(*update.Status))
	}
	notes := ""
	if update.Notes != nil {
		notes = strings.TrimSpace(*update.Notes)
	}
	_, err := r.pool.Exec(context.Background(), `UPDATE legal_data_subject_requests SET status=COALESCE(NULLIF($2,''),status), notes=COALESCE($3,notes), updated_at=NOW() WHERE id=$1`, strings.TrimSpace(id), status, notes)
	if err != nil {
		return domain.DataSubjectRequest{}, err
	}
	if status != "" {
		_, _ = r.pool.Exec(context.Background(), `INSERT INTO legal_state_history (id, entity_type, entity_id, from_state, to_state, actor_user_id, reason, created_at) VALUES ($1,'data_subject',$2,'',$3,$4,$5,NOW())`, mustID(), strings.TrimSpace(id), status, strings.TrimSpace(actorUserID), notes)
	}
	rec, ok := r.GetDataSubjectRequest(id)
	if !ok {
		return domain.DataSubjectRequest{}, fmt.Errorf("data subject request not found")
	}
	return rec, nil
}

func (r *postgresRepository) AddDataSubjectAuditEvent(requestID string, params CreateDataSubjectAuditEventParams) (domain.DataSubjectAuditEvent, error) {
	id, err := generateID()
	if err != nil {
		return domain.DataSubjectAuditEvent{}, err
	}
	now := time.Now().UTC()
	_, err = r.pool.Exec(context.Background(), `INSERT INTO legal_data_subject_audit_events (id, request_id, actor_user_id, action, details, evidence_ref, occurred_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, id, strings.TrimSpace(requestID), strings.TrimSpace(params.ActorUserID), strings.TrimSpace(params.Action), strings.TrimSpace(params.Details), strings.TrimSpace(params.EvidenceRef), now)
	if err != nil {
		return domain.DataSubjectAuditEvent{}, err
	}
	return domain.DataSubjectAuditEvent{ID: id, RequestID: strings.TrimSpace(requestID), ActorUserID: strings.TrimSpace(params.ActorUserID), Action: strings.TrimSpace(params.Action), Details: strings.TrimSpace(params.Details), EvidenceRef: strings.TrimSpace(params.EvidenceRef), OccurredAtUTC: now}, nil
}

func (r *postgresRepository) ListDataSubjectAuditEvents(requestID string) ([]domain.DataSubjectAuditEvent, error) {
	rows, err := r.pool.Query(context.Background(), `SELECT id, request_id, actor_user_id, action, details, evidence_ref, occurred_at FROM legal_data_subject_audit_events WHERE request_id=$1 ORDER BY occurred_at ASC`, strings.TrimSpace(requestID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.DataSubjectAuditEvent{}
	for rows.Next() {
		var rec domain.DataSubjectAuditEvent
		if err := rows.Scan(&rec.ID, &rec.RequestID, &rec.ActorUserID, &rec.Action, &rec.Details, &rec.EvidenceRef, &rec.OccurredAtUTC); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *postgresRepository) ListLegalStateHistory(entityType, entityID string) ([]domain.LegalStateHistory, error) {
	entityType = strings.ToLower(strings.TrimSpace(entityType))
	entityID = strings.TrimSpace(entityID)
	query := `SELECT id, entity_type, entity_id, from_state, to_state, actor_user_id, reason, created_at FROM legal_state_history WHERE ($1='' OR entity_type=$1) AND ($2='' OR entity_id=$2) ORDER BY created_at ASC`
	rows, err := r.pool.Query(context.Background(), query, entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.LegalStateHistory{}
	for rows.Next() {
		var rec domain.LegalStateHistory
		if err := rows.Scan(&rec.ID, &rec.EntityType, &rec.EntityID, &rec.FromState, &rec.ToState, &rec.ActorUserID, &rec.Reason, &rec.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}
