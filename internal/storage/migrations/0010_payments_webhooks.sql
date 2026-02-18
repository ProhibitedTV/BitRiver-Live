BEGIN;

ALTER TABLE tips
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

ALTER TABLE subscriptions
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

CREATE TABLE IF NOT EXISTS payment_transactions (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    event_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    reference TEXT NOT NULL,
    status TEXT NOT NULL,
    idempotency_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS payment_transactions_provider_event_unique ON payment_transactions (provider, event_id);
CREATE INDEX IF NOT EXISTS payment_transactions_entity_idx ON payment_transactions (entity_type, entity_id, created_at DESC);

COMMIT;
