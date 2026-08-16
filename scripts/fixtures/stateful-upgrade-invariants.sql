WITH fixture_values AS (
  SELECT concat_ws(E'\n',
    (SELECT string_agg(id || ':' || array_to_string(roles, ','), ',' ORDER BY id)
       FROM users WHERE id IN ('admin', 'creator', 'moderator', 'viewer')),
    (SELECT user_id || ':' || bio || ':' || social_links::text FROM profiles WHERE user_id = 'creator'),
    (SELECT id || ':' || owner_id || ':' || title || ':' || schedule::text FROM channels WHERE id = 'channel-1'),
    (SELECT id || ':' || channel_id || ':' || coalesce(playback_url, '') || ':' || metadata::text FROM uploads WHERE id = 'upload-1'),
    (SELECT id || ':' || channel_id || ':' || coalesce(playback_base_url, '') || ':' || metadata::text FROM recordings WHERE id = 'recording-1'),
    (SELECT id || ':' || status FROM chat_reports WHERE id = 'report-1'),
    (SELECT id || ':' || status FROM appeals WHERE id = 'appeal-1'),
    (SELECT id || ':' || status FROM legal_dmca_cases WHERE id = 'dmca-1'),
    (SELECT id || ':' || action FROM chat_automod_actions WHERE id = 'automod-1'),
    (SELECT provider || ':' || event_id || ':' || status FROM payment_transactions WHERE id = 'payment-1')
  ) AS value
)
SELECT jsonb_build_object(
  'rowCounts', jsonb_build_object(
    'users', (SELECT count(*) FROM users),
    'profiles', (SELECT count(*) FROM profiles),
    'authSessions', (SELECT count(*) FROM auth_sessions),
    'authMfa', (SELECT count(*) FROM auth_mfa),
    'channels', (SELECT count(*) FROM channels),
    'follows', (SELECT count(*) FROM follows),
    'streamSessions', (SELECT count(*) FROM stream_sessions),
    'recordings', (SELECT count(*) FROM recordings),
    'uploads', (SELECT count(*) FROM uploads),
    'chatMessages', (SELECT count(*) FROM chat_messages),
    'chatFilters', (SELECT count(*) FROM chat_filters),
    'chatReports', (SELECT count(*) FROM chat_reports),
    'appeals', (SELECT count(*) FROM appeals),
    'legalCases', (SELECT count(*) FROM legal_dmca_cases),
    'paymentTransactions', (SELECT count(*) FROM payment_transactions)
  ),
  'valueFingerprintSha256', encode(digest(value::bytea, 'sha256'), 'hex')
)::text
FROM fixture_values;
