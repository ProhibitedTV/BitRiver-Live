package api

import (
	"context"
	"net/http"
)

type componentStatus struct {
	Component string `json:"component"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

func (h *Handler) componentHealth(ctx context.Context) ([]componentStatus, string, int) {
	overallStatus := "ok"
	statusCode := http.StatusOK
	recordComponent := func(component string, err error) componentStatus {
		status := "ok"
		message := ""
		if err != nil {
			status = "degraded"
			message = err.Error()
			overallStatus = "degraded"
			statusCode = http.StatusServiceUnavailable
		}
		return componentStatus{Component: component, Status: status, Error: message}
	}

	components := make([]componentStatus, 0, 4)
	if h.Store != nil {
		components = append(components, recordComponent("datastore", h.Store.Ping(ctx)))
	}

	components = append(components, recordComponent("sessions", h.sessionManager().Ping(ctx)))

	if h.RateLimiter != nil {
		components = append(components, recordComponent("rate_limiter", h.RateLimiter.Ping(ctx)))
	}

	if h.ChatQueue != nil {
		components = append(components, recordComponent("chat_queue", h.ChatQueue.Ping(ctx)))
	}

	return components, overallStatus, statusCode
}
