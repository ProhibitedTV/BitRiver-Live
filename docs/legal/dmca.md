# DMCA Policy and Operations

DMCA notices are tracked in `/api/legal/dmca` with states:

- `open`
- `actioned`
- `restored`
- `rejected`

State transitions are persisted with audit history in `legal_state_history`.
