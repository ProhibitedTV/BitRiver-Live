BEGIN;

CREATE TABLE IF NOT EXISTS appeals (
    id TEXT PRIMARY KEY,
    report_id TEXT NOT NULL REFERENCES chat_reports(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    reporter_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    resolution TEXT,
    resolver_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS appeals_channel_status_idx ON appeals (channel_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS appeals_reporter_idx ON appeals (reporter_id, created_at DESC);

CREATE TABLE IF NOT EXISTS appeal_events (
    id TEXT PRIMARY KEY,
    appeal_id TEXT NOT NULL REFERENCES appeals(id) ON DELETE CASCADE,
    actor_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS appeal_events_appeal_created_idx ON appeal_events (appeal_id, created_at ASC);

COMMIT;
