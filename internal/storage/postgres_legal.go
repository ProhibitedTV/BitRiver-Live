package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"bitriver-live/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

const dmcaCaseSelectColumns = `id, reporter_name, reporter_email, content_url, description, status, notes, actioned_at, restored_at, rejected_at, created_at, updated_at`

func trimLegalText(value string) string {
	return strings.TrimSpace(value)
}

func normalizeLegalStatus(value string) string {
	return strings.ToLower(trimLegalText(value))
}

func trimLegalUpdateValue(value *string) string {
	if value == nil {
		return ""
	}
	return trimLegalText(*value)
}

func normalizeLegalUpdateStatus(value *string) string {
	if value == nil {
		return ""
	}
	return normalizeLegalStatus(*value)
}

func scanDMCACase(row interface{ Scan(dest ...any) error }) (domain.DMCACase, error) {
	var rec domain.DMCACase
	err := row.Scan(&rec.ID, &rec.ReporterName, &rec.ReporterEmail, &rec.ContentURL, &rec.Description, &rec.Status, &rec.Notes, &rec.ActionedAt, &rec.RestoredAt, &rec.RejectedAt, &rec.CreatedAt, &rec.UpdatedAt)
	return rec, err
}

func scanDataSubjectRequest(row interface{ Scan(dest ...any) error }) (domain.DataSubjectRequest, error) {
	var rec domain.DataSubjectRequest
	err := row.Scan(&rec.ID, &rec.SubjectEmail, &rec.RequestType, &rec.Status, &rec.Notes, &rec.CreatedAt, &rec.UpdatedAt)
	return rec, err
}

func (r *postgresRepository) CreateDMCACase(params CreateDMCACaseParams) (domain.DMCACase, error) {
	reporterName := trimLegalText(params.ReporterName)
	reporterEmail := trimLegalText(params.ReporterEmail)
	contentURL := trimLegalText(params.ContentURL)
	description := trimLegalText(params.Description)
	if reporterName == "" || reporterEmail == "" || contentURL == "" {
		return domain.DMCACase{}, fmt.Errorf("reporter name, reporter email, and content url are required")
	}
	id, err := generateID()
	if err != nil {
		return domain.DMCACase{}, err
	}
	now := time.Now().UTC()
	err = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		_, err := conn.Exec(ctx, `INSERT INTO legal_dmca_cases (id, reporter_name, reporter_email, content_url, description, status, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, id, reporterName, reporterEmail, contentURL, description, domain.DMCACaseStatusOpen, now, now)
		if err != nil {
			return err
		}
		_, err = conn.Exec(ctx, `INSERT INTO legal_state_history (id, entity_type, entity_id, from_state, to_state, actor_user_id, reason, created_at) VALUES ($1,'dmca',$2,'',$3,'',$4,$5)`, mustID(), id, domain.DMCACaseStatusOpen, "case submitted", now)
		return err
	})
	if err != nil {
		return domain.DMCACase{}, err
	}
	return domain.DMCACase{ID: id, ReporterName: reporterName, ReporterEmail: reporterEmail, ContentURL: contentURL, Description: description, Status: domain.DMCACaseStatusOpen, CreatedAt: now, UpdatedAt: now}, nil
}

func mustID() string { id, _ := generateID(); return id }

func (r *postgresRepository) ListDMCACases() ([]domain.DMCACase, error) {
	out := []domain.DMCACase{}
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		rows, err := conn.Query(ctx, `SELECT `+dmcaCaseSelectColumns+` FROM legal_dmca_cases ORDER BY created_at DESC`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			rec, err := scanDMCACase(rows)
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *postgresRepository) GetDMCACase(id string) (domain.DMCACase, bool) {
	id = trimLegalText(id)
	if id == "" {
		return domain.DMCACase{}, false
	}
	var rec domain.DMCACase
	var found bool
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		loaded, err := scanDMCACase(conn.QueryRow(ctx, `SELECT `+dmcaCaseSelectColumns+` FROM legal_dmca_cases WHERE id=$1`, id))
		if err != nil {
			return nil
		}
		rec = loaded
		found = true
		return nil
	})
	if err != nil || !found {
		return domain.DMCACase{}, false
	}
	return rec, true
}

func (r *postgresRepository) UpdateDMCACase(id string, update DMCACaseUpdate, actorUserID string) (domain.DMCACase, error) {
	id = trimLegalText(id)
	if id == "" {
		return domain.DMCACase{}, fmt.Errorf("dmca case id required")
	}
	status := normalizeLegalUpdateStatus(update.Status)
	notes := trimLegalUpdateValue(update.Notes)
	var rec domain.DMCACase
	var found bool
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		_, err := conn.Exec(ctx, `UPDATE legal_dmca_cases SET status=COALESCE(NULLIF($2,''),status), notes=COALESCE($3,notes), updated_at=NOW(), actioned_at=CASE WHEN $2='actioned' THEN NOW() ELSE actioned_at END, restored_at=CASE WHEN $2='restored' THEN NOW() ELSE restored_at END, rejected_at=CASE WHEN $2='rejected' THEN NOW() ELSE rejected_at END WHERE id=$1`, id, status, notes)
		if err != nil {
			return err
		}
		if status != "" {
			_, _ = conn.Exec(ctx, `INSERT INTO legal_state_history (id, entity_type, entity_id, from_state, to_state, actor_user_id, reason, created_at) VALUES ($1,'dmca',$2,'',$3,$4,$5,NOW())`, mustID(), id, status, trimLegalText(actorUserID), notes)
		}
		loaded, loadErr := scanDMCACase(conn.QueryRow(ctx, `SELECT `+dmcaCaseSelectColumns+` FROM legal_dmca_cases WHERE id=$1`, id))
		if loadErr != nil {
			return nil
		}
		rec = loaded
		found = true
		return nil
	})
	if err != nil {
		return domain.DMCACase{}, err
	}
	if !found {
		return domain.DMCACase{}, fmt.Errorf("dmca case not found")
	}
	return rec, nil
}

func (r *postgresRepository) CreateDataSubjectRequest(params CreateDataSubjectRequestParams) (domain.DataSubjectRequest, error) {
	id, err := generateID()
	if err != nil {
		return domain.DataSubjectRequest{}, err
	}
	subjectEmail := trimLegalText(params.SubjectEmail)
	reqType := normalizeLegalStatus(params.RequestType)
	notes := trimLegalText(params.Notes)
	now := time.Now().UTC()
	err = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		_, err := conn.Exec(ctx, `INSERT INTO legal_data_subject_requests (id, subject_email, request_type, status, notes, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, id, subjectEmail, reqType, domain.DataSubjectRequestStatusOpen, notes, now, now)
		if err != nil {
			return err
		}
		_, _ = conn.Exec(ctx, `INSERT INTO legal_state_history (id, entity_type, entity_id, from_state, to_state, actor_user_id, reason, created_at) VALUES ($1,'data_subject',$2,'',$3,'',$4,$5)`, mustID(), id, domain.DataSubjectRequestStatusOpen, "request submitted", now)
		return nil
	})
	if err != nil {
		return domain.DataSubjectRequest{}, err
	}
	return domain.DataSubjectRequest{ID: id, SubjectEmail: subjectEmail, RequestType: reqType, Status: domain.DataSubjectRequestStatusOpen, Notes: notes, CreatedAt: now, UpdatedAt: now}, nil
}

func (r *postgresRepository) ListDataSubjectRequests() ([]domain.DataSubjectRequest, error) {
	out := []domain.DataSubjectRequest{}
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		rows, err := conn.Query(ctx, `SELECT id, subject_email, request_type, status, notes, created_at, updated_at FROM legal_data_subject_requests ORDER BY created_at DESC`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			rec, err := scanDataSubjectRequest(rows)
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *postgresRepository) GetDataSubjectRequest(id string) (domain.DataSubjectRequest, bool) {
	id = trimLegalText(id)
	if id == "" {
		return domain.DataSubjectRequest{}, false
	}
	var rec domain.DataSubjectRequest
	var found bool
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		loaded, err := scanDataSubjectRequest(conn.QueryRow(ctx, `SELECT id, subject_email, request_type, status, notes, created_at, updated_at FROM legal_data_subject_requests WHERE id=$1`, id))
		if err != nil {
			return nil
		}
		rec = loaded
		found = true
		return nil
	})
	if err != nil || !found {
		return domain.DataSubjectRequest{}, false
	}
	return rec, true
}

func (r *postgresRepository) UpdateDataSubjectRequest(id string, update DataSubjectRequestUpdate, actorUserID string) (domain.DataSubjectRequest, error) {
	id = trimLegalText(id)
	status := normalizeLegalUpdateStatus(update.Status)
	notes := trimLegalUpdateValue(update.Notes)
	var rec domain.DataSubjectRequest
	var found bool
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		_, err := conn.Exec(ctx, `UPDATE legal_data_subject_requests SET status=COALESCE(NULLIF($2,''),status), notes=COALESCE($3,notes), updated_at=NOW() WHERE id=$1`, id, status, notes)
		if err != nil {
			return err
		}
		if status != "" {
			_, _ = conn.Exec(ctx, `INSERT INTO legal_state_history (id, entity_type, entity_id, from_state, to_state, actor_user_id, reason, created_at) VALUES ($1,'data_subject',$2,'',$3,$4,$5,NOW())`, mustID(), id, status, trimLegalText(actorUserID), notes)
		}
		loaded, loadErr := scanDataSubjectRequest(conn.QueryRow(ctx, `SELECT id, subject_email, request_type, status, notes, created_at, updated_at FROM legal_data_subject_requests WHERE id=$1`, id))
		if loadErr != nil {
			return nil
		}
		rec = loaded
		found = true
		return nil
	})
	if err != nil {
		return domain.DataSubjectRequest{}, err
	}
	if !found {
		return domain.DataSubjectRequest{}, fmt.Errorf("data subject request not found")
	}
	return rec, nil
}

func (r *postgresRepository) AddDataSubjectAuditEvent(requestID string, params CreateDataSubjectAuditEventParams) (domain.DataSubjectAuditEvent, error) {
	id, err := generateID()
	if err != nil {
		return domain.DataSubjectAuditEvent{}, err
	}
	requestID = trimLegalText(requestID)
	actorUserID := trimLegalText(params.ActorUserID)
	action := trimLegalText(params.Action)
	details := trimLegalText(params.Details)
	evidenceRef := trimLegalText(params.EvidenceRef)
	now := time.Now().UTC()
	err = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		_, err := conn.Exec(ctx, `INSERT INTO legal_data_subject_audit_events (id, request_id, actor_user_id, action, details, evidence_ref, occurred_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, id, requestID, actorUserID, action, details, evidenceRef, now)
		return err
	})
	if err != nil {
		return domain.DataSubjectAuditEvent{}, err
	}
	return domain.DataSubjectAuditEvent{ID: id, RequestID: requestID, ActorUserID: actorUserID, Action: action, Details: details, EvidenceRef: evidenceRef, OccurredAtUTC: now}, nil
}

func (r *postgresRepository) ListDataSubjectAuditEvents(requestID string) ([]domain.DataSubjectAuditEvent, error) {
	requestID = trimLegalText(requestID)
	out := []domain.DataSubjectAuditEvent{}
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		rows, err := conn.Query(ctx, `SELECT id, request_id, actor_user_id, action, details, evidence_ref, occurred_at FROM legal_data_subject_audit_events WHERE request_id=$1 ORDER BY occurred_at ASC`, requestID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var rec domain.DataSubjectAuditEvent
			if err := rows.Scan(&rec.ID, &rec.RequestID, &rec.ActorUserID, &rec.Action, &rec.Details, &rec.EvidenceRef, &rec.OccurredAtUTC); err != nil {
				return err
			}
			out = append(out, rec)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *postgresRepository) ListLegalStateHistory(entityType, entityID string) ([]domain.LegalStateHistory, error) {
	entityType = normalizeLegalStatus(entityType)
	entityID = trimLegalText(entityID)
	query := `SELECT id, entity_type, entity_id, from_state, to_state, actor_user_id, reason, created_at FROM legal_state_history WHERE ($1='' OR entity_type=$1) AND ($2='' OR entity_id=$2) ORDER BY created_at ASC`
	out := []domain.LegalStateHistory{}
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		rows, err := conn.Query(ctx, query, entityType, entityID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var rec domain.LegalStateHistory
			if err := rows.Scan(&rec.ID, &rec.EntityType, &rec.EntityID, &rec.FromState, &rec.ToState, &rec.ActorUserID, &rec.Reason, &rec.CreatedAt); err != nil {
				return err
			}
			out = append(out, rec)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
