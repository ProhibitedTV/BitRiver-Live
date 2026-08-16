INSERT INTO users (id, display_name, email, roles, password_hash, self_signup) VALUES
  ('admin', 'Upgrade admin', 'admin@example.invalid', ARRAY['admin'], 'hash-admin', false),
  ('creator', 'Upgrade creator', 'creator@example.invalid', ARRAY['creator'], 'hash-creator', false),
  ('moderator', 'Upgrade moderator', 'moderator@example.invalid', ARRAY['moderator'], 'hash-moderator', false),
  ('viewer', 'Upgrade viewer', 'viewer@example.invalid', ARRAY['viewer'], 'hash-viewer', true);
INSERT INTO profiles (user_id, bio, featured_channel_id, created_at, updated_at, social_links)
VALUES ('creator', 'upgrade fixture', 'channel-1', now(), now(), '[{"platform":"website","url":"https://example.invalid/creator"}]');
INSERT INTO oauth_accounts (provider, subject, user_id, email, display_name)
VALUES ('fixture', 'creator-subject', 'creator', 'creator@example.invalid', 'Upgrade creator');
INSERT INTO auth_sessions (token, user_id, expires_at, hashed_token, absolute_expires_at)
VALUES (repeat('a', 64), 'admin', now() + interval '1 hour', repeat('a', 64), now() + interval '8 hours');
INSERT INTO auth_mfa (user_id, secret, recovery_codes, enabled, enabled_at)
VALUES ('admin', 'JBSWY3DPEHPK3PXP', ARRAY['fixture-recovery-hash'], true, now());
INSERT INTO channels (id, owner_id, stream_key, title, category, tags, schedule, live_state)
VALUES ('channel-1', 'creator', 'fixture-stream-key', 'Upgrade channel', 'technology', ARRAY['upgrade','fixture'], '[{"title":"Upgrade event","startsAt":"2026-08-16T02:00:00Z"}]', 'offline');
INSERT INTO follows (user_id, channel_id) VALUES ('viewer', 'channel-1');
INSERT INTO stream_sessions (id, channel_id, started_at, ended_at, renditions, peak_concurrent, origin_url, playback_url, ingest_endpoints, ingest_job_ids)
VALUES ('session-1', 'channel-1', now() - interval '10 minutes', now() - interval '5 minutes', ARRAY['720p'], 7, 'rtmp://origin.invalid/live/fixture', 'https://media.invalid/live/fixture/index.m3u8', ARRAY['rtmp://ingest.invalid/live'], ARRAY['job-1']);
INSERT INTO stream_session_manifests (session_id, name, manifest_url, bitrate)
VALUES ('session-1', '720p', 'https://media.invalid/live/fixture/720p/index.m3u8', 2800000);
INSERT INTO recordings (id, channel_id, session_id, title, duration_seconds, playback_base_url, metadata, published_at, created_at)
VALUES ('recording-1', 'channel-1', 'session-1', 'Upgrade recording', 300, 'https://objects.invalid/recordings/fixture', '{"objectKey":"recordings/fixture/index.m3u8","sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}', now(), now());
INSERT INTO recording_renditions (recording_id, name, manifest_url, bitrate)
VALUES ('recording-1', '720p', 'https://objects.invalid/recordings/fixture/720p.m3u8', 2800000);
INSERT INTO recording_thumbnails (id, recording_id, url, width, height, created_at)
VALUES ('thumbnail-1', 'recording-1', 'https://objects.invalid/recordings/fixture/thumb.jpg', 1280, 720, now());
INSERT INTO uploads (id, channel_id, title, filename, size_bytes, status, progress, recording_id, playback_url, metadata, completed_at)
VALUES ('upload-1', 'channel-1', 'Upgrade upload', 'fixture.mp4', 4096, 'completed', 100, 'recording-1', 'https://objects.invalid/uploads/fixture.mp4', '{"objectKey":"uploads/fixture.mp4","sha256":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"}', now());
INSERT INTO chat_messages (id, channel_id, user_id, content, created_at)
VALUES ('message-1', 'channel-1', 'viewer', 'upgrade fixture message', now());
INSERT INTO chat_bans (channel_id, user_id, actor_id, reason)
VALUES ('channel-1', 'moderator', 'admin', 'upgrade fixture ban');
INSERT INTO chat_timeouts (channel_id, user_id, actor_id, reason, expires_at)
VALUES ('channel-1', 'viewer', 'moderator', 'upgrade fixture timeout', now() + interval '5 minutes');
INSERT INTO chat_filters (id, channel_id, kind, pattern)
VALUES ('filter-1', 'channel-1', 'literal', 'fixture-blocked-term');
INSERT INTO chat_automod_actions (id, channel_id, user_id, filter_id, filter_kind, filter_pattern, message, action)
VALUES ('automod-1', 'channel-1', 'viewer', 'filter-1', 'literal', 'fixture-blocked-term', 'blocked fixture', 'blocked');
INSERT INTO chat_reports (id, channel_id, reporter_id, target_id, reason, message_id)
VALUES ('report-1', 'channel-1', 'viewer', 'moderator', 'upgrade fixture report', 'message-1');
INSERT INTO appeals (id, report_id, channel_id, reporter_id, reason)
VALUES ('appeal-1', 'report-1', 'channel-1', 'moderator', 'upgrade fixture appeal');
INSERT INTO appeal_events (id, appeal_id, actor_id, action, note)
VALUES ('appeal-event-1', 'appeal-1', 'admin', 'opened', 'upgrade fixture event');
INSERT INTO legal_dmca_cases (id, reporter_name, reporter_email, content_url, description, status)
VALUES ('dmca-1', 'Fixture reporter', 'reporter@example.invalid', 'https://example.invalid/content', 'upgrade fixture case', 'open');
INSERT INTO legal_data_subject_requests (id, subject_email, request_type, status)
VALUES ('dsr-1', 'viewer@example.invalid', 'export', 'open');
INSERT INTO legal_data_subject_audit_events (id, request_id, actor_user_id, action, details, evidence_ref)
VALUES ('dsr-event-1', 'dsr-1', 'admin', 'created', 'upgrade fixture audit', 'ticket-1');
INSERT INTO legal_state_history (id, entity_type, entity_id, to_state, actor_user_id, reason)
VALUES ('legal-history-1', 'dmca', 'dmca-1', 'open', 'admin', 'upgrade fixture history');
INSERT INTO tips (id, channel_id, from_user_id, amount, currency, provider, reference, message, status, idempotency_key)
VALUES ('tip-1', 'channel-1', 'viewer', 1.25, 'USD', 'fixture', 'tip-reference-1', 'upgrade fixture tip', 'succeeded', 'tip-key-1');
INSERT INTO subscriptions (id, channel_id, user_id, tier, provider, reference, amount, currency, started_at, expires_at, auto_renew, status, idempotency_key)
VALUES ('subscription-1', 'channel-1', 'viewer', 'supporter', 'fixture', 'subscription-reference-1', 5, 'USD', now(), now() + interval '30 days', true, 'active', 'subscription-key-1');
INSERT INTO payment_transactions (id, provider, event_id, entity_type, entity_id, reference, status, idempotency_key)
VALUES ('payment-1', 'fixture', 'event-1', 'tip', 'tip-1', 'payment-reference-1', 'succeeded', 'payment-key-1');
