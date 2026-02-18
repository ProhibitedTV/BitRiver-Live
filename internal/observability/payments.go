package observability

import (
	"log/slog"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/observability/metrics"
)

// RecordPaymentTransition emits audit logs and metrics for payment state transitions.
func RecordPaymentTransition(logger *slog.Logger, entityType, fromState, toState string, amount domain.Money) {
	metrics.Default().ObserveMonetization(entityType+"_"+toState, amount)
	if logger == nil {
		return
	}
	logger.Info("payment_state_transition", "entityType", entityType, "from", fromState, "to", toState)
}
