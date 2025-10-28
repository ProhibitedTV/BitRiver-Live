-- 0001_initial.sql
--
-- Establishes the relational schema that mirrors the entities defined in
-- internal/models. The layout keeps identifiers as text so existing snowflake
-- and ULID generators used by the JSON repository continue to work without
-- translation when importing data into Postgres.

BEGIN;

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    roles TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    password_hash TEXT,
    self_signup BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS profiles (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    bio TEXT NOT NULL DEFAULT '',
    avatar_url TEXT,
    banner_url TEXT,
    featured_channel_id TEXT,
    top_friends TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    donation_addresses JSONB NOT NULL DEFAULT '[]'::JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS channels (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stream_key TEXT NOT NULL,
    title TEXT NOT NULL,
    category TEXT,
    tags TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    live_state TEXT NOT NULL,
    current_session_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS follows (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    followed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, channel_id)
);

CREATE TABLE IF NOT EXISTS stream_sessions (
    id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    renditions TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    peak_concurrent INTEGER NOT NULL DEFAULT 0,
    origin_url TEXT,
    playback_url TEXT,
    ingest_endpoints TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    ingest_job_ids TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[]
);

CREATE TABLE IF NOT EXISTS stream_session_manifests (
    session_id TEXT NOT NULL REFERENCES stream_sessions(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    manifest_url TEXT NOT NULL,
    bitrate INTEGER,
    PRIMARY KEY (session_id, name)
);

CREATE TABLE IF NOT EXISTS recordings (
    id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL REFERENCES stream_sessions(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    duration_seconds INTEGER NOT NULL,
    playback_base_url TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::JSONB,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    retain_until TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS recording_renditions (
    recording_id TEXT NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    manifest_url TEXT NOT NULL,
    bitrate INTEGER,
    PRIMARY KEY (recording_id, name)
);

CREATE TABLE IF NOT EXISTS recording_thumbnails (
    id TEXT PRIMARY KEY,
    recording_id TEXT NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    width INTEGER,
    height INTEGER,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS clip_exports (
    id TEXT PRIMARY KEY,
    recording_id TEXT NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    session_id TEXT NOT NULL REFERENCES stream_sessions(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    start_seconds INTEGER NOT NULL,
    end_seconds INTEGER NOT NULL,
    status TEXT NOT NULL,
    playback_url TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    storage_object TEXT
);

CREATE TABLE IF NOT EXISTS chat_messages (
    id TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS chat_bans (
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    banned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (channel_id, user_id)
);

CREATE TABLE IF NOT EXISTS chat_timeouts (
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (channel_id, user_id)
);

COMMIT;
