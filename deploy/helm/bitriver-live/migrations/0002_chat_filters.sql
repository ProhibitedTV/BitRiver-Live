CREATE TABLE IF NOT EXISTS chat_filters (
    id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    pattern TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS chat_filters_channel_idx ON chat_filters (channel_id, created_at DESC);

CREATE TABLE IF NOT EXISTS chat_automod_actions (
    id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filter_id TEXT REFERENCES chat_filters(id) ON DELETE SET NULL,
    filter_kind TEXT NOT NULL DEFAULT '',
    filter_pattern TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL,
    action TEXT NOT NULL DEFAULT 'blocked',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS chat_automod_actions_channel_idx ON chat_automod_actions (channel_id, created_at DESC);
CREATE INDEX IF NOT EXISTS chat_automod_actions_user_idx ON chat_automod_actions (user_id, created_at DESC);
