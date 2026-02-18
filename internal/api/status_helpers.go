package api

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type statusCheck struct {
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Status      string    `json:"status"`
	Detail      string    `json:"detail,omitempty"`
	Remediation string    `json:"remediation"`
	CheckedAt   time.Time `json:"checkedAt"`
}

type logHint struct {
	Label   string `json:"label"`
	Command string `json:"command"`
}

type statusResponse struct {
	Status         string        `json:"status"`
	CheckedAt      time.Time     `json:"checkedAt"`
	Checks         []statusCheck `json:"checks"`
	RecentFailures []statusCheck `json:"recentFailures"`
	LogHints       []logHint     `json:"logHints"`
}

// Status combines readiness, ingest health, and backing store/queue checks into
// a single operator-friendly payload.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	r, span := h.startSpan(r, "api.status")
	if span != nil {
		defer span.End()
	}
	ctx := r.Context()
	checkedAt := time.Now().UTC()

	components, _, _ := h.componentHealth(ctx)
	checks := make([]statusCheck, 0, len(components)+3)
	for _, component := range components {
		checks = append(checks, componentStatusCheck(component, checkedAt))
	}

	ingestChecks, ingestCheckedAt := h.ingestStatusChecks(ctx, checkedAt)
	checks = append(checks, ingestChecks...)
	if !ingestCheckedAt.IsZero() {
		checkedAt = ingestCheckedAt
	}

	payload := statusResponse{
		Status:         aggregateStatus(checks),
		CheckedAt:      checkedAt,
		Checks:         checks,
		RecentFailures: recentFailures(checks),
		LogHints:       defaultLogHints(),
	}

	WriteJSON(w, http.StatusOK, payload)
}

// ingestStatusChecks performs ingest status checks and propagates validation or dependency failures to the caller.
func (h *Handler) ingestStatusChecks(ctx context.Context, fallback time.Time) ([]statusCheck, time.Time) {
	if h.systemService() == nil {
		return nil, time.Time{}
	}

	snapshot := h.systemService().IngestHealth(ctx)
	_, recordedAt := h.systemService().LastIngestHealth()
	checkedAt := fallback
	if !recordedAt.IsZero() {
		checkedAt = recordedAt
	}

	checks := make([]statusCheck, 0, len(snapshot))
	for _, entry := range snapshot {
		checks = append(checks, statusCheck{
			Name:        entry.Component,
			Category:    "ingest",
			Status:      normalizeStatus(entry.Status, entry.Detail),
			Detail:      entry.Detail,
			Remediation: remediationFor(entry.Component),
			CheckedAt:   checkedAt,
		})
	}

	return checks, checkedAt
}

// componentStatusCheck performs component status check and propagates validation or dependency failures to the caller.
func componentStatusCheck(component componentStatus, checkedAt time.Time) statusCheck {
	return statusCheck{
		Name:        component.Component,
		Category:    "core",
		Status:      normalizeStatus(component.Status, component.Error),
		Detail:      component.Error,
		Remediation: remediationFor(component.Component),
		CheckedAt:   checkedAt,
	}
}

// normalizeStatus performs normalize status and propagates validation or dependency failures to the caller.
func normalizeStatus(status, detail string) string {
	switch strings.ToLower(status) {
	case "ok", "ready":
		return "ready"
	case "disabled", "unknown":
		return "degraded"
	case "degraded", "warning":
		if detail != "" {
			return "down"
		}
		return "degraded"
	default:
		if detail != "" {
			return "down"
		}
		return "degraded"
	}
}

// aggregateStatus performs aggregate status and propagates validation or dependency failures to the caller.
func aggregateStatus(checks []statusCheck) string {
	overall := "ready"
	degraded := false
	for _, check := range checks {
		switch check.Status {
		case "down":
			return "down"
		case "degraded":
			degraded = true
		}
	}
	if degraded {
		return "degraded"
	}
	return overall
}

// recentFailures performs recent failures and propagates validation or dependency failures to the caller.
func recentFailures(checks []statusCheck) []statusCheck {
	degraded := make([]statusCheck, 0)
	for _, check := range checks {
		if check.Status != "ready" {
			degraded = append(degraded, check)
		}
	}
	return degraded
}

// remediationFor performs remediation for and propagates validation or dependency failures to the caller.
func remediationFor(component string) string {
	key := strings.ToLower(strings.TrimSpace(component))
	switch key {
	case "datastore":
		return "Verify Postgres is reachable from the API container and that DATABASE_URL matches .env, then restart the server."
	case "sessions":
		return "Confirm session secrets are configured and system clocks are in sync across nodes."
	case "rate_limiter":
		return "Ensure Redis is running and RATE_LIMIT_REDIS_URL points to it before retrying."
	case "chat_queue":
		return "Check the chat queue Redis broker and restart the chat worker if it is offline."
	case "srs":
		return "Confirm SRS is listening on the ingest port and reachable from the API network."
	case "ovenmediaengine":
		return "Verify OvenMediaEngine control and playback endpoints are up, then restart the encoder stack."
	case "transcoder":
		return "Restart the transcoder worker and confirm it can reach the media directory."
	case "ingest":
		return "Set up ingest endpoints in .env or disable ingest if you do not need live streaming."
	default:
		return "Inspect service logs from the dashboard and docker compose logs to recover the component."
	}
}

// defaultLogHints returns the default log hints for the current runtime mode.
func defaultLogHints() []logHint {
	return []logHint{
		{Label: "API server", Command: "docker compose logs -f server"},
		{Label: "Ingest pipeline", Command: "docker compose logs -f srs ome transcoder"},
		{Label: "Database + Redis", Command: "docker compose logs -f postgres redis"},
	}
}
