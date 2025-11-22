package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"bitriver-live/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (r *postgresRepository) importSnapshot(ctx context.Context, snapshot *Snapshot) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	return r.withConn(func(_ context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin snapshot transaction: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if err := r.importSnapshotUsers(ctx, tx, snapshot.Users); err != nil {
			return err
		}
		if err := r.importSnapshotProfiles(ctx, tx, snapshot.Profiles); err != nil {
			return err
		}
		if err := r.importSnapshotChannels(ctx, tx, snapshot.Channels); err != nil {
			return err
		}
		if err := r.importSnapshotFollows(ctx, tx, snapshot.Follows); err != nil {
			return err
		}
		if err := r.importSnapshotStreamSessions(ctx, tx, snapshot.StreamSessions); err != nil {
			return err
		}
		if err := r.importSnapshotRecordings(ctx, tx, snapshot.Recordings); err != nil {
			return err
		}
		if err := r.importSnapshotUploads(ctx, tx, snapshot.Uploads); err != nil {
			return err
		}
		if err := r.importSnapshotClipExports(ctx, tx, snapshot.ClipExports); err != nil {
			return err
		}
		if err := r.importSnapshotChatMessages(ctx, tx, snapshot.ChatMessages); err != nil {
			return err
		}
		if err := r.importSnapshotChatModeration(ctx, tx, snapshot); err != nil {
			return err
		}
		if err := r.importSnapshotChatReports(ctx, tx, snapshot.ChatReports); err != nil {
			return err
		}
		if err := r.importSnapshotTips(ctx, tx, snapshot.Tips); err != nil {
			return err
		}
		if err := r.importSnapshotSubscriptions(ctx, tx, snapshot.Subscriptions); err != nil {
			return err
		}
		if err := r.importSnapshotOAuthAccounts(ctx, tx, snapshot.OAuthAccounts); err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit snapshot import: %w", err)
		}
		return nil
	})
}

func (r *postgresRepository) importSnapshotUsers(ctx context.Context, tx pgx.Tx, users map[string]models.User) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]string, 0, len(users))
	for id := range users {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, key := range ids {
		user := users[key]
		id := strings.TrimSpace(user.ID)
		if id == "" {
			id = key
		}
		createdAt := user.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		} else {
			createdAt = createdAt.UTC()
		}
		roles := append([]string(nil), user.Roles...)
		if roles == nil {
			roles = []string{}
		}
		_, err := tx.Exec(ctx, "INSERT INTO users (id, display_name, email, roles, password_hash, self_signup, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO NOTHING", id, strings.TrimSpace(user.DisplayName), strings.TrimSpace(user.Email), roles, strings.TrimSpace(user.PasswordHash), user.SelfSignup, createdAt)
		if err != nil {
			return fmt.Errorf("insert user %s: %w", id, err)
		}
	}
	return nil
}

func (r *postgresRepository) importSnapshotProfiles(ctx context.Context, tx pgx.Tx, profiles map[string]models.Profile) error {
	if len(profiles) == 0 {
		return nil
	}
	ids := make([]string, 0, len(profiles))
	for id := range profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, userID := range ids {
		profile := profiles[userID]
		created := profile.CreatedAt
		if created.IsZero() {
			created = time.Now().UTC()
		} else {
			created = created.UTC()
		}
		updated := profile.UpdatedAt
		if updated.IsZero() {
			updated = created
		} else {
			updated = updated.UTC()
		}
		socialLinks, err := encodeSocialLinks(profile.SocialLinks)
		if err != nil {
			return err
		}
		donation, err := encodeDonationAddresses(profile.DonationAddresses)
		if err != nil {
			return err
		}
		topFriends := append([]string(nil), profile.TopFriends...)
		if topFriends == nil {
			topFriends = []string{}
		}
		var featured any
		if profile.FeaturedChannelID != nil && strings.TrimSpace(*profile.FeaturedChannelID) != "" {
			featured = strings.TrimSpace(*profile.FeaturedChannelID)
		}
		_, err = tx.Exec(ctx, "INSERT INTO profiles (user_id, bio, avatar_url, banner_url, featured_channel_id, top_friends, social_links, donation_addresses, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT (user_id) DO NOTHING", userID, profile.Bio, strings.TrimSpace(profile.AvatarURL), strings.TrimSpace(profile.BannerURL), featured, topFriends, socialLinks, donation, created, updated)
		if err != nil {
			return fmt.Errorf("insert profile %s: %w", userID, err)
		}
	}
	return nil
}

func (r *postgresRepository) importSnapshotChannels(ctx context.Context, tx pgx.Tx, channels map[string]models.Channel) error {
	if len(channels) == 0 {
		return nil
	}
	ids := make([]string, 0, len(channels))
	for id := range channels {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, key := range ids {
		channel := channels[key]
		id := strings.TrimSpace(channel.ID)
		if id == "" {
			id = key
		}
		created := channel.CreatedAt
		if created.IsZero() {
			created = time.Now().UTC()
		} else {
			created = created.UTC()
		}
		updated := channel.UpdatedAt
		if updated.IsZero() {
			updated = created
		} else {
			updated = updated.UTC()
		}
		tags := append([]string(nil), channel.Tags...)
		if tags == nil {
			tags = []string{}
		}
		var current any
		if channel.CurrentSessionID != nil && strings.TrimSpace(*channel.CurrentSessionID) != "" {
			current = strings.TrimSpace(*channel.CurrentSessionID)
		}
		_, err := tx.Exec(ctx, "INSERT INTO channels (id, owner_id, stream_key, title, category, tags, live_state, current_session_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT (id) DO NOTHING", id, strings.TrimSpace(channel.OwnerID), strings.TrimSpace(channel.StreamKey), strings.TrimSpace(channel.Title), strings.TrimSpace(channel.Category), tags, strings.TrimSpace(channel.LiveState), current, created, updated)
		if err != nil {
			return fmt.Errorf("insert channel %s: %w", id, err)
		}
	}
	return nil
}

func (r *postgresRepository) importSnapshotFollows(ctx context.Context, tx pgx.Tx, follows map[string]map[string]time.Time) error {
	for userID, entries := range follows {
		for channelID, followedAt := range entries {
			_, err := tx.Exec(ctx, "INSERT INTO follows (user_id, channel_id, followed_at) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING", strings.TrimSpace(userID), strings.TrimSpace(channelID), followedAt.UTC())
			if err != nil {
				return fmt.Errorf("insert follow %s->%s: %w", userID, channelID, err)
			}
		}
	}
	return nil
}

func (r *postgresRepository) importSnapshotStreamSessions(ctx context.Context, tx pgx.Tx, sessions map[string]models.StreamSession) error {
	if len(sessions) == 0 {
		return nil
	}
	ids := make([]string, 0, len(sessions))
	for id := range sessions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, key := range ids {
		session := sessions[key]
		id := strings.TrimSpace(session.ID)
		if id == "" {
			id = key
		}
		started := session.StartedAt
		if started.IsZero() {
			started = time.Now().UTC()
		} else {
			started = started.UTC()
		}
		var ended any
		if session.EndedAt != nil && !session.EndedAt.IsZero() {
			ended = session.EndedAt.UTC()
		}
		renditions := append([]string(nil), session.Renditions...)
		if renditions == nil {
			renditions = []string{}
		}
		ingestEndpoints := append([]string(nil), session.IngestEndpoints...)
		if ingestEndpoints == nil {
			ingestEndpoints = []string{}
		}
		ingestJobIDs := append([]string(nil), session.IngestJobIDs...)
		if ingestJobIDs == nil {
			ingestJobIDs = []string{}
		}
		_, err := tx.Exec(ctx, "INSERT INTO stream_sessions (id, channel_id, started_at, ended_at, renditions, peak_concurrent, origin_url, playback_url, ingest_endpoints, ingest_job_ids) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT (id) DO NOTHING", id, strings.TrimSpace(session.ChannelID), started, ended, renditions, session.PeakConcurrent, strings.TrimSpace(session.OriginURL), strings.TrimSpace(session.PlaybackURL), ingestEndpoints, ingestJobIDs)
		if err != nil {
			return fmt.Errorf("insert stream session %s: %w", id, err)
		}
		for _, manifest := range session.RenditionManifests {
			_, err := tx.Exec(ctx, "INSERT INTO stream_session_manifests (session_id, name, manifest_url, bitrate) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING", id, strings.TrimSpace(manifest.Name), strings.TrimSpace(manifest.ManifestURL), manifest.Bitrate)
			if err != nil {
				return fmt.Errorf("insert stream manifest %s/%s: %w", id, manifest.Name, err)
			}
		}
	}
	return nil
}

func (r *postgresRepository) importSnapshotRecordings(ctx context.Context, tx pgx.Tx, recordings map[string]models.Recording) error {
	if len(recordings) == 0 {
		return nil
	}
	ids := make([]string, 0, len(recordings))
	for id := range recordings {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, key := range ids {
		recording := recordings[key]
		if strings.TrimSpace(recording.ID) == "" {
			recording.ID = key
		}
		recording.CreatedAt = recording.CreatedAt.UTC()
		if recording.PublishedAt != nil {
			ts := recording.PublishedAt.UTC()
			recording.PublishedAt = &ts
		}
		if recording.RetainUntil != nil {
			ts := recording.RetainUntil.UTC()
			recording.RetainUntil = &ts
		}
		for i := range recording.Thumbnails {
			recording.Thumbnails[i].CreatedAt = recording.Thumbnails[i].CreatedAt.UTC()
		}
		if err := r.insertRecording(ctx, tx, recording); err != nil {
			return err
		}
	}
	return nil
}

func (r *postgresRepository) importSnapshotUploads(ctx context.Context, tx pgx.Tx, uploads map[string]models.Upload) error {
	if len(uploads) == 0 {
		return nil
	}
	ids := make([]string, 0, len(uploads))
	for id := range uploads {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, key := range ids {
		upload := uploads[key]
		id := strings.TrimSpace(upload.ID)
		if id == "" {
			id = key
		}
		metadata := upload.Metadata
		if metadata == nil {
			metadata = make(map[string]string)
		}
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("encode upload metadata %s: %w", id, err)
		}
		created := upload.CreatedAt.UTC()
		if created.IsZero() {
			created = time.Now().UTC()
		}
		updated := upload.UpdatedAt.UTC()
		if updated.IsZero() {
			updated = created
		}
		var recordingID any
		if upload.RecordingID != nil && strings.TrimSpace(*upload.RecordingID) != "" {
			recordingID = strings.TrimSpace(*upload.RecordingID)
		}
		var completedAt any
		if upload.CompletedAt != nil && !upload.CompletedAt.IsZero() {
			completedAt = upload.CompletedAt.UTC()
		}
		var errorText any
		if strings.TrimSpace(upload.Error) != "" {
			errorText = strings.TrimSpace(upload.Error)
		}
		_, err = tx.Exec(ctx, "INSERT INTO uploads (id, channel_id, title, filename, size_bytes, status, progress, recording_id, playback_url, metadata, error, created_at, updated_at, completed_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) ON CONFLICT (id) DO NOTHING", id, strings.TrimSpace(upload.ChannelID), strings.TrimSpace(upload.Title), strings.TrimSpace(upload.Filename), upload.SizeBytes, strings.TrimSpace(upload.Status), upload.Progress, recordingID, strings.TrimSpace(upload.PlaybackURL), metadataJSON, errorText, created, updated, completedAt)
		if err != nil {
			return fmt.Errorf("insert upload %s: %w", id, err)
		}
	}
	return nil
}

func (r *postgresRepository) importSnapshotClipExports(ctx context.Context, tx pgx.Tx, clips map[string]models.ClipExport) error {
	if len(clips) == 0 {
		return nil
	}
	ids := make([]string, 0, len(clips))
	for id := range clips {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, key := range ids {
		clip := clips[key]
		id := strings.TrimSpace(clip.ID)
		if id == "" {
			id = key
		}
		created := clip.CreatedAt.UTC()
		if created.IsZero() {
			created = time.Now().UTC()
		}
		var completed any
		if clip.CompletedAt != nil && !clip.CompletedAt.IsZero() {
			completed = clip.CompletedAt.UTC()
		}
		var storageObject any
		if strings.TrimSpace(clip.StorageObject) != "" {
			storageObject = strings.TrimSpace(clip.StorageObject)
		}
		_, err := tx.Exec(ctx, "INSERT INTO clip_exports (id, recording_id, channel_id, session_id, title, start_seconds, end_seconds, status, playback_url, created_at, completed_at, storage_object) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) ON CONFLICT (id) DO NOTHING", id, strings.TrimSpace(clip.RecordingID), strings.TrimSpace(clip.ChannelID), strings.TrimSpace(clip.SessionID), strings.TrimSpace(clip.Title), clip.StartSeconds, clip.EndSeconds, strings.TrimSpace(clip.Status), strings.TrimSpace(clip.PlaybackURL), created, completed, storageObject)
		if err != nil {
			return fmt.Errorf("insert clip export %s: %w", id, err)
		}
	}
	return nil
}

func (r *postgresRepository) importSnapshotChatMessages(ctx context.Context, tx pgx.Tx, messages map[string]models.ChatMessage) error {
	if len(messages) == 0 {
		return nil
	}
	ids := make([]string, 0, len(messages))
	for id := range messages {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, key := range ids {
		msg := messages[key]
		id := strings.TrimSpace(msg.ID)
		if id == "" {
			id = key
		}
		created := msg.CreatedAt.UTC()
		if created.IsZero() {
			created = time.Now().UTC()
		}
		_, err := tx.Exec(ctx, "INSERT INTO chat_messages (id, channel_id, user_id, content, created_at) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO NOTHING", id, strings.TrimSpace(msg.ChannelID), strings.TrimSpace(msg.UserID), msg.Content, created)
		if err != nil {
			return fmt.Errorf("insert chat message %s: %w", id, err)
		}
	}
	return nil
}

func (r *postgresRepository) importSnapshotChatModeration(ctx context.Context, tx pgx.Tx, snapshot *Snapshot) error {
	for channelID, entries := range snapshot.ChatBans {
		for userID, issuedAt := range entries {
			actor := lookupString(snapshot.ChatBanActors, channelID, userID)
			var actorParam any
			if actor != "" {
				actorParam = actor
			}
			reason := lookupString(snapshot.ChatBanReasons, channelID, userID)
			_, err := tx.Exec(ctx, "INSERT INTO chat_bans (channel_id, user_id, actor_id, reason, issued_at) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (channel_id, user_id) DO NOTHING", strings.TrimSpace(channelID), strings.TrimSpace(userID), actorParam, reason, issuedAt.UTC())
			if err != nil {
				return fmt.Errorf("insert chat ban %s/%s: %w", channelID, userID, err)
			}
		}
	}
	for channelID, entries := range snapshot.ChatTimeouts {
		for userID, expiresAt := range entries {
			actor := lookupString(snapshot.ChatTimeoutActors, channelID, userID)
			var actorParam any
			if actor != "" {
				actorParam = actor
			}
			reason := lookupString(snapshot.ChatTimeoutReasons, channelID, userID)
			issuedAt := lookupTime(snapshot.ChatTimeoutIssuedAt, channelID, userID)
			if issuedAt.IsZero() {
				issuedAt = expiresAt
			}
			_, err := tx.Exec(ctx, "INSERT INTO chat_timeouts (channel_id, user_id, actor_id, reason, issued_at, expires_at) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (channel_id, user_id) DO NOTHING", strings.TrimSpace(channelID), strings.TrimSpace(userID), actorParam, reason, issuedAt.UTC(), expiresAt.UTC())
			if err != nil {
				return fmt.Errorf("insert chat timeout %s/%s: %w", channelID, userID, err)
			}
		}
	}
	return nil
}

func (r *postgresRepository) importSnapshotChatReports(ctx context.Context, tx pgx.Tx, reports map[string]models.ChatReport) error {
	if len(reports) == 0 {
		return nil
	}
	ids := make([]string, 0, len(reports))
	for id := range reports {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, key := range ids {
		report := reports[key]
		id := strings.TrimSpace(report.ID)
		if id == "" {
			id = key
		}
		created := report.CreatedAt.UTC()
		if created.IsZero() {
			created = time.Now().UTC()
		}
		var resolvedAt any
		if report.ResolvedAt != nil && !report.ResolvedAt.IsZero() {
			resolvedAt = report.ResolvedAt.UTC()
		}
		var resolver any
		if strings.TrimSpace(report.ResolverID) != "" {
			resolver = strings.TrimSpace(report.ResolverID)
		}
		var messageID any
		if strings.TrimSpace(report.MessageID) != "" {
			messageID = strings.TrimSpace(report.MessageID)
		}
		var evidence any
		if strings.TrimSpace(report.EvidenceURL) != "" {
			evidence = strings.TrimSpace(report.EvidenceURL)
		}
		_, err := tx.Exec(ctx, "INSERT INTO chat_reports (id, channel_id, reporter_id, target_id, reason, message_id, evidence_url, status, resolution, resolver_id, created_at, resolved_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) ON CONFLICT (id) DO NOTHING", id, strings.TrimSpace(report.ChannelID), strings.TrimSpace(report.ReporterID), strings.TrimSpace(report.TargetID), strings.TrimSpace(report.Reason), messageID, evidence, strings.TrimSpace(report.Status), strings.TrimSpace(report.Resolution), resolver, created, resolvedAt)
		if err != nil {
			return fmt.Errorf("insert chat report %s: %w", id, err)
		}
	}
	return nil
}

func (r *postgresRepository) importSnapshotTips(ctx context.Context, tx pgx.Tx, tips map[string]models.Tip) error {
	if len(tips) == 0 {
		return nil
	}
	ids := make([]string, 0, len(tips))
	for id := range tips {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, key := range ids {
		tip := tips[key]
		id := strings.TrimSpace(tip.ID)
		if id == "" {
			id = key
		}
		created := tip.CreatedAt.UTC()
		if created.IsZero() {
			created = time.Now().UTC()
		}
		var wallet any
		if strings.TrimSpace(tip.WalletAddress) != "" {
			wallet = strings.TrimSpace(tip.WalletAddress)
		}
		var message any
		if strings.TrimSpace(tip.Message) != "" {
			message = strings.TrimSpace(tip.Message)
		}
		_, err := tx.Exec(ctx, "INSERT INTO tips (id, channel_id, from_user_id, amount, currency, provider, reference, wallet_address, message, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT (id) DO NOTHING", id, strings.TrimSpace(tip.ChannelID), strings.TrimSpace(tip.FromUserID), tip.Amount.DecimalString(), strings.TrimSpace(tip.Currency), strings.TrimSpace(tip.Provider), strings.TrimSpace(tip.Reference), wallet, message, created)
		if err != nil {
			return fmt.Errorf("insert tip %s: %w", id, err)
		}
	}
	return nil
}

func (r *postgresRepository) importSnapshotSubscriptions(ctx context.Context, tx pgx.Tx, subs map[string]models.Subscription) error {
	if len(subs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(subs))
	for id := range subs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, key := range ids {
		sub := subs[key]
		id := strings.TrimSpace(sub.ID)
		if id == "" {
			id = key
		}
		started := sub.StartedAt.UTC()
		if started.IsZero() {
			started = time.Now().UTC()
		}
		expires := sub.ExpiresAt.UTC()
		if expires.IsZero() {
			expires = started
		}
		var cancelledBy any
		if strings.TrimSpace(sub.CancelledBy) != "" {
			cancelledBy = strings.TrimSpace(sub.CancelledBy)
		}
		var cancelledReason any
		if strings.TrimSpace(sub.CancelledReason) != "" {
			cancelledReason = strings.TrimSpace(sub.CancelledReason)
		}
		var cancelledAt any
		if sub.CancelledAt != nil && !sub.CancelledAt.IsZero() {
			cancelledAt = sub.CancelledAt.UTC()
		}
		var externalRef any
		if strings.TrimSpace(sub.ExternalReference) != "" {
			externalRef = strings.TrimSpace(sub.ExternalReference)
		}
		_, err := tx.Exec(ctx, "INSERT INTO subscriptions (id, channel_id, user_id, tier, provider, reference, amount, currency, started_at, expires_at, auto_renew, status, cancelled_by, cancelled_reason, cancelled_at, external_reference) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16) ON CONFLICT (id) DO NOTHING", id, strings.TrimSpace(sub.ChannelID), strings.TrimSpace(sub.UserID), strings.TrimSpace(sub.Tier), strings.TrimSpace(sub.Provider), strings.TrimSpace(sub.Reference), sub.Amount.DecimalString(), strings.TrimSpace(sub.Currency), started, expires, sub.AutoRenew, strings.TrimSpace(sub.Status), cancelledBy, cancelledReason, cancelledAt, externalRef)
		if err != nil {
			return fmt.Errorf("insert subscription %s: %w", id, err)
		}
	}
	return nil
}

func (r *postgresRepository) importSnapshotOAuthAccounts(ctx context.Context, tx pgx.Tx, accounts map[string]models.OAuthAccount) error {
	if len(accounts) == 0 {
		return nil
	}
	keys := make([]string, 0, len(accounts))
	for key := range accounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		account := accounts[key]
		linked := account.LinkedAt.UTC()
		if linked.IsZero() {
			linked = time.Now().UTC()
		}
		_, err := tx.Exec(ctx, "INSERT INTO oauth_accounts (provider, subject, user_id, email, display_name, linked_at) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (provider, subject) DO NOTHING", strings.TrimSpace(account.Provider), strings.TrimSpace(account.Subject), strings.TrimSpace(account.UserID), strings.TrimSpace(account.Email), strings.TrimSpace(account.DisplayName), linked)
		if err != nil {
			return fmt.Errorf("insert oauth account %s: %w", key, err)
		}
	}
	return nil
}

func lookupString(container map[string]map[string]string, channelID, userID string) string {
	if container == nil {
		return ""
	}
	if users, ok := container[channelID]; ok {
		if value, ok := users[userID]; ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func lookupTime(container map[string]map[string]time.Time, channelID, userID string) time.Time {
	if container == nil {
		return time.Time{}
	}
	if users, ok := container[channelID]; ok {
		if value, ok := users[userID]; ok {
			return value
		}
	}
	return time.Time{}
}
