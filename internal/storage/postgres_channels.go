package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// loadStreamSession performs load stream session and propagates validation or dependency failures to the caller.
func (r *postgresRepository) loadStreamSession(ctx context.Context, id string) (models.StreamSession, bool) {
	if strings.TrimSpace(id) == "" {
		return models.StreamSession{}, false
	}
	var (
		channelID       string
		startedAt       time.Time
		endedAt         pgtype.Timestamptz
		renditions      []string
		peak            int
		originURL       string
		playbackURL     string
		ingestEndpoints []string
		ingestJobIDs    []string
	)
	err := r.pool.QueryRow(ctx, "SELECT channel_id, started_at, ended_at, renditions, peak_concurrent, origin_url, playback_url, ingest_endpoints, ingest_job_ids FROM stream_sessions WHERE id = $1", id).
		Scan(&channelID, &startedAt, &endedAt, &renditions, &peak, &originURL, &playbackURL, &ingestEndpoints, &ingestJobIDs)
	if err != nil {
		return models.StreamSession{}, false
	}
	manifestsRows, err := r.pool.Query(ctx, "SELECT name, manifest_url, bitrate FROM stream_session_manifests WHERE session_id = $1", id)
	if err != nil {
		return models.StreamSession{}, false
	}
	defer manifestsRows.Close()
	manifests := make([]models.RenditionManifest, 0)
	for manifestsRows.Next() {
		var name, url string
		var bitrate pgtype.Int4
		if err := manifestsRows.Scan(&name, &url, &bitrate); err != nil {
			return models.StreamSession{}, false
		}
		entry := models.RenditionManifest{Name: name, ManifestURL: url}
		if bitrate.Valid {
			entry.Bitrate = int(bitrate.Int32)
		}
		manifests = append(manifests, entry)
	}
	if err := manifestsRows.Err(); err != nil {
		return models.StreamSession{}, false
	}
	session := models.StreamSession{
		ID:                 id,
		ChannelID:          channelID,
		StartedAt:          startedAt.UTC(),
		Renditions:         append([]string{}, renditions...),
		PeakConcurrent:     peak,
		OriginURL:          originURL,
		PlaybackURL:        playbackURL,
		IngestEndpoints:    append([]string{}, ingestEndpoints...),
		IngestJobIDs:       append([]string{}, ingestJobIDs...),
		RenditionManifests: manifests,
	}
	if endedAt.Valid {
		ts := endedAt.Time.UTC()
		session.EndedAt = &ts
	}
	if session.Renditions == nil {
		session.Renditions = []string{}
	}
	if session.RenditionManifests == nil {
		session.RenditionManifests = []models.RenditionManifest{}
	}
	if session.IngestEndpoints == nil {
		session.IngestEndpoints = []string{}
	}
	if session.IngestJobIDs == nil {
		session.IngestJobIDs = []string{}
	}
	return session, true
}

// recordingDeadline performs recording deadline and propagates validation or dependency failures to the caller.
func (r *postgresRepository) recordingDeadline(now time.Time, published bool) *time.Time {
	var window time.Duration
	if published {
		window = r.recordingRetention.Published
	} else {
		window = r.recordingRetention.Unpublished
	}
	if window < 0 {
		return nil
	}
	deadline := now.Add(window)
	return &deadline
}

// createRecording creates recording and returns an error when validation or persistence fails.
func (r *postgresRepository) createRecording(session models.StreamSession, channel models.Channel, ended time.Time) (models.Recording, error) {
	recordingID, err := generateID()
	if err != nil {
		return models.Recording{}, err
	}
	duration := int(ended.Sub(session.StartedAt).Round(time.Second).Seconds())
	if duration < 0 {
		duration = 0
	}
	title := strings.TrimSpace(channel.Title)
	if title == "" {
		title = fmt.Sprintf("Recording %s", session.ID)
	}
	metadata := map[string]string{
		"channelId":  channel.ID,
		"sessionId":  session.ID,
		"startedAt":  session.StartedAt.UTC().Format(time.RFC3339Nano),
		"endedAt":    ended.UTC().Format(time.RFC3339Nano),
		"renditions": strconv.Itoa(len(session.RenditionManifests)),
	}
	if session.PeakConcurrent > 0 {
		metadata["peakConcurrent"] = strconv.Itoa(session.PeakConcurrent)
	}
	recording := models.Recording{
		ID:              recordingID,
		ChannelID:       channel.ID,
		SessionID:       session.ID,
		Title:           title,
		DurationSeconds: duration,
		PlaybackBaseURL: session.PlaybackURL,
		Metadata:        metadata,
		CreatedAt:       ended,
	}
	if deadline := r.recordingDeadline(ended, false); deadline != nil {
		recording.RetainUntil = deadline
	}
	if len(session.RenditionManifests) > 0 {
		renditions := make([]models.RecordingRendition, 0, len(session.RenditionManifests))
		for _, manifest := range session.RenditionManifests {
			renditions = append(renditions, models.RecordingRendition(manifest))
		}
		recording.Renditions = renditions
	}
	if err := r.populateRecordingArtifacts(&recording, session); err != nil {
		return models.Recording{}, err
	}
	return recording, nil
}

// populateRecordingArtifacts performs populate recording artifacts and propagates validation or dependency failures to the caller.
func (r *postgresRepository) populateRecordingArtifacts(recording *models.Recording, session models.StreamSession) error {
	client := r.objectClient
	if client == nil || !client.Enabled() {
		return nil
	}
	if recording.Metadata == nil {
		recording.Metadata = make(map[string]string)
	}

	createdAt := recording.CreatedAt.UTC().Format(time.RFC3339Nano)
	if len(session.RenditionManifests) > 0 {
		for idx, manifest := range session.RenditionManifests {
			key := buildObjectKey("recordings", recording.ID, "manifests", normalizeObjectComponent(manifest.Name)+".json")
			payload := map[string]any{
				"recordingId": recording.ID,
				"sessionId":   recording.SessionID,
				"name":        manifest.Name,
				"source":      manifest.ManifestURL,
				"createdAt":   createdAt,
			}
			if manifest.Bitrate > 0 {
				payload["bitrate"] = manifest.Bitrate
			}
			data, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("encode manifest payload: %w", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), r.objectStorage.requestTimeout())
			ref, err := client.Upload(ctx, key, "application/json", data)
			cancel()
			if err != nil {
				return fmt.Errorf("upload manifest %s: %w", manifest.Name, err)
			}
			if ref.Key != "" {
				recording.Metadata[manifestMetadataKey(manifest.Name)] = ref.Key
			}
			if ref.URL != "" && idx < len(recording.Renditions) {
				recording.Renditions[idx].ManifestURL = ref.URL
			}
		}
	}

	thumbID, err := generateID()
	if err != nil {
		return fmt.Errorf("generate thumbnail id: %w", err)
	}
	thumbKey := buildObjectKey("recordings", recording.ID, "thumbnails", thumbID+".json")
	thumbPayload := map[string]any{
		"recordingId": recording.ID,
		"sessionId":   recording.SessionID,
		"createdAt":   createdAt,
	}
	thumbData, err := json.Marshal(thumbPayload)
	if err != nil {
		return fmt.Errorf("encode thumbnail payload: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.objectStorage.requestTimeout())
	ref, err := client.Upload(ctx, thumbKey, "application/json", thumbData)
	cancel()
	if err != nil {
		return fmt.Errorf("upload thumbnail: %w", err)
	}
	if ref.Key != "" {
		recording.Metadata[thumbnailMetadataKey(thumbID)] = ref.Key
	}
	thumbnail := models.RecordingThumbnail{
		ID:          thumbID,
		RecordingID: recording.ID,
		URL:         ref.URL,
		CreatedAt:   recording.CreatedAt,
	}
	recording.Thumbnails = append(recording.Thumbnails, thumbnail)
	return nil
}

// insertRecording performs insert recording and propagates validation or dependency failures to the caller.
func (r *postgresRepository) insertRecording(ctx context.Context, tx pgx.Tx, recording models.Recording) error {
	metadata := recording.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encode recording metadata: %w", err)
	}
	var publishedAt any
	if recording.PublishedAt != nil {
		publishedAt = recording.PublishedAt
	}
	var retainUntil any
	if recording.RetainUntil != nil {
		retainUntil = recording.RetainUntil
	}
	_, err = tx.Exec(ctx, "INSERT INTO recordings (id, channel_id, session_id, title, duration_seconds, playback_base_url, metadata, published_at, created_at, retain_until) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
		recording.ID,
		recording.ChannelID,
		recording.SessionID,
		recording.Title,
		recording.DurationSeconds,
		recording.PlaybackBaseURL,
		metadataJSON,
		publishedAt,
		recording.CreatedAt,
		retainUntil,
	)
	if err != nil {
		return fmt.Errorf("insert recording %s: %w", recording.ID, err)
	}
	for _, rendition := range recording.Renditions {
		if _, err := tx.Exec(ctx, "INSERT INTO recording_renditions (recording_id, name, manifest_url, bitrate) VALUES ($1, $2, $3, $4)", recording.ID, rendition.Name, rendition.ManifestURL, rendition.Bitrate); err != nil {
			return fmt.Errorf("insert recording rendition %s: %w", rendition.Name, err)
		}
	}
	for _, thumb := range recording.Thumbnails {
		if _, err := tx.Exec(ctx, "INSERT INTO recording_thumbnails (id, recording_id, url, width, height, created_at) VALUES ($1, $2, $3, $4, $5, $6)", thumb.ID, recording.ID, thumb.URL, thumb.Width, thumb.Height, thumb.CreatedAt); err != nil {
			return fmt.Errorf("insert recording thumbnail %s: %w", thumb.ID, err)
		}
	}
	return nil
}

// deleteRecordingArtifacts deletes recording artifacts and returns an error when cleanup or persistence fails.
func (r *postgresRepository) deleteRecordingArtifacts(recording models.Recording) error {
	client := r.objectClient
	if client == nil || !client.Enabled() {
		return nil
	}
	if len(recording.Metadata) == 0 {
		return nil
	}
	deleted := make(map[string]struct{})
	for key, objectKey := range recording.Metadata {
		if !strings.HasPrefix(key, metadataManifestPrefix) && !strings.HasPrefix(key, metadataThumbnailPrefix) {
			continue
		}
		trimmed := strings.TrimSpace(objectKey)
		if trimmed == "" {
			continue
		}
		if _, exists := deleted[trimmed]; exists {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), r.objectStorage.requestTimeout())
		err := client.Delete(ctx, trimmed)
		cancel()
		if err != nil {
			return fmt.Errorf("delete object %s: %w", trimmed, err)
		}
		deleted[trimmed] = struct{}{}
	}
	return nil
}

// deleteClipArtifacts deletes clip artifacts and returns an error when cleanup or persistence fails.
func (r *postgresRepository) deleteClipArtifacts(clip models.ClipExport) error {
	client := r.objectClient
	if client == nil || !client.Enabled() {
		return nil
	}
	trimmed := strings.TrimSpace(clip.StorageObject)
	if trimmed == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.objectStorage.requestTimeout())
	defer cancel()
	if err := client.Delete(ctx, trimmed); err != nil {
		return fmt.Errorf("delete clip object %s: %w", trimmed, err)
	}
	return nil
}

// retentionTime performs retention time and propagates validation or dependency failures to the caller.
func (r *postgresRepository) retentionTime() time.Time {
	if r.retentionNow != nil {
		return r.retentionNow()
	}
	return time.Now().UTC()
}

// runRecordingRetention runs recording retention and exits when the work completes or a dependency fails.
func (r *postgresRepository) runRecordingRetention(ctx context.Context) error {
	return r.purgeExpiredRecordings(ctx, r.retentionTime())
}

// purgeExpiredRecordings performs purge expired recordings and propagates validation or dependency failures to the caller.
func (r *postgresRepository) purgeExpiredRecordings(ctx context.Context, now time.Time) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	rows, err := r.pool.Query(ctx, "SELECT id, metadata FROM recordings WHERE retain_until IS NOT NULL AND retain_until <= $1", now)
	if err != nil {
		return err
	}
	defer rows.Close()
	ids := make([]string, 0)
	recordings := make(map[string]models.Recording)
	for rows.Next() {
		var id string
		var metadataBytes []byte
		if err := rows.Scan(&id, &metadataBytes); err != nil {
			return err
		}
		meta := make(map[string]string)
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &meta); err != nil {
				return fmt.Errorf("decode recording metadata: %w", err)
			}
		}
		recordings[id] = models.Recording{ID: id, Metadata: meta}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		recording := recordings[id]
		if err := r.deleteRecordingArtifacts(recording); err != nil {
			slog.Default().Warn("failed to delete recording artifacts", "recording_id", id, "error", err)
			continue
		}
		clipRows, err := r.pool.Query(ctx, "SELECT id, storage_object FROM clip_exports WHERE recording_id = $1", id)
		if err != nil {
			return fmt.Errorf("load clip exports for recording %s: %w", id, err)
		}
		clips := make([]models.ClipExport, 0)
		for clipRows.Next() {
			var clip models.ClipExport
			var storageObject pgtype.Text
			if err := clipRows.Scan(&clip.ID, &storageObject); err != nil {
				clipRows.Close()
				return fmt.Errorf("scan clip export: %w", err)
			}
			if storageObject.Valid {
				clip.StorageObject = storageObject.String
			}
			clips = append(clips, clip)
		}
		clipRows.Close()
		if err := clipRows.Err(); err != nil {
			return fmt.Errorf("read clip exports for recording %s: %w", id, err)
		}
		failed := false
		for _, clip := range clips {
			if err := r.deleteClipArtifacts(clip); err != nil {
				slog.Default().Warn("failed to delete clip artifacts", "recording_id", id, "clip_id", clip.ID, "error", err)
				failed = true
			}
		}
		if failed {
			continue
		}
		if _, err := r.pool.Exec(ctx, "DELETE FROM recordings WHERE id = $1", id); err != nil {
			return fmt.Errorf("delete recording %s: %w", id, err)
		}
	}
	return nil
}

// loadRecording performs load recording and propagates validation or dependency failures to the caller.
func (r *postgresRepository) loadRecording(ctx context.Context, id string) (models.Recording, bool, error) {
	var (
		channelID       string
		sessionID       string
		title           string
		duration        int
		playbackBaseURL string
		metadataBytes   []byte
		publishedAt     pgtype.Timestamptz
		createdAt       time.Time
		retainUntil     pgtype.Timestamptz
	)
	err := r.pool.QueryRow(ctx, "SELECT channel_id, session_id, title, duration_seconds, playback_base_url, metadata, published_at, created_at, retain_until FROM recordings WHERE id = $1", id).
		Scan(&channelID, &sessionID, &title, &duration, &playbackBaseURL, &metadataBytes, &publishedAt, &createdAt, &retainUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Recording{}, false, nil
	}
	if err != nil {
		return models.Recording{}, false, err
	}
	metadata := make(map[string]string)
	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			return models.Recording{}, false, fmt.Errorf("decode recording metadata: %w", err)
		}
	}
	recording := models.Recording{
		ID:              id,
		ChannelID:       channelID,
		SessionID:       sessionID,
		Title:           title,
		DurationSeconds: duration,
		PlaybackBaseURL: playbackBaseURL,
		Metadata:        metadata,
		CreatedAt:       createdAt.UTC(),
	}
	if publishedAt.Valid {
		ts := publishedAt.Time.UTC()
		recording.PublishedAt = &ts
	}
	if retainUntil.Valid {
		ts := retainUntil.Time.UTC()
		recording.RetainUntil = &ts
	}
	renditionsRows, err := r.pool.Query(ctx, "SELECT name, manifest_url, bitrate FROM recording_renditions WHERE recording_id = $1", id)
	if err != nil {
		return models.Recording{}, false, fmt.Errorf("load recording renditions: %w", err)
	}
	renditions := make([]models.RecordingRendition, 0)
	for renditionsRows.Next() {
		var name, url string
		var bitrate pgtype.Int4
		if err := renditionsRows.Scan(&name, &url, &bitrate); err != nil {
			renditionsRows.Close()
			return models.Recording{}, false, fmt.Errorf("scan recording rendition: %w", err)
		}
		entry := models.RecordingRendition{Name: name, ManifestURL: url}
		if bitrate.Valid {
			entry.Bitrate = int(bitrate.Int32)
		}
		renditions = append(renditions, entry)
	}
	renditionsRows.Close()
	if err := renditionsRows.Err(); err != nil {
		return models.Recording{}, false, fmt.Errorf("read recording renditions: %w", err)
	}
	recording.Renditions = renditions

	thumbRows, err := r.pool.Query(ctx, "SELECT id, url, width, height, created_at FROM recording_thumbnails WHERE recording_id = $1", id)
	if err != nil {
		return models.Recording{}, false, fmt.Errorf("load recording thumbnails: %w", err)
	}
	thumbnails := make([]models.RecordingThumbnail, 0)
	for thumbRows.Next() {
		var thumb models.RecordingThumbnail
		thumb.RecordingID = id
		if err := thumbRows.Scan(&thumb.ID, &thumb.URL, &thumb.Width, &thumb.Height, &thumb.CreatedAt); err != nil {
			thumbRows.Close()
			return models.Recording{}, false, fmt.Errorf("scan recording thumbnail: %w", err)
		}
		thumbnails = append(thumbnails, thumb)
	}
	thumbRows.Close()
	if err := thumbRows.Err(); err != nil {
		return models.Recording{}, false, fmt.Errorf("read recording thumbnails: %w", err)
	}
	recording.Thumbnails = thumbnails

	clipRows, err := r.pool.Query(ctx, "SELECT id, title, start_seconds, end_seconds, status FROM clip_exports WHERE recording_id = $1", id)
	if err != nil {
		return models.Recording{}, false, fmt.Errorf("load clip exports: %w", err)
	}
	clips := make([]models.ClipExportSummary, 0)
	for clipRows.Next() {
		var clip models.ClipExportSummary
		if err := clipRows.Scan(&clip.ID, &clip.Title, &clip.StartSeconds, &clip.EndSeconds, &clip.Status); err != nil {
			clipRows.Close()
			return models.Recording{}, false, fmt.Errorf("scan clip export: %w", err)
		}
		clips = append(clips, clip)
	}
	clipRows.Close()
	if err := clipRows.Err(); err != nil {
		return models.Recording{}, false, fmt.Errorf("read clip exports: %w", err)
	}
	if len(clips) > 0 {
		sort.Slice(clips, func(i, j int) bool {
			if clips[i].StartSeconds == clips[j].StartSeconds {
				return clips[i].ID < clips[j].ID
			}
			return clips[i].StartSeconds < clips[j].StartSeconds
		})
		recording.Clips = clips
	}
	return recording, true, nil
}

// loadUpload performs load upload and propagates validation or dependency failures to the caller.
func (r *postgresRepository) loadUpload(ctx context.Context, id string) (models.Upload, bool, error) {
	var (
		channelID     string
		title         string
		filename      string
		sizeBytes     int64
		status        string
		progress      int
		recordingID   pgtype.Text
		playbackURL   pgtype.Text
		metadataBytes []byte
		errorText     pgtype.Text
		createdAt     time.Time
		updatedAt     time.Time
		completedAt   pgtype.Timestamptz
	)
	err := r.pool.QueryRow(ctx, "SELECT channel_id, title, filename, size_bytes, status, progress, recording_id, playback_url, metadata, error, created_at, updated_at, completed_at FROM uploads WHERE id = $1", id).
		Scan(&channelID, &title, &filename, &sizeBytes, &status, &progress, &recordingID, &playbackURL, &metadataBytes, &errorText, &createdAt, &updatedAt, &completedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Upload{}, false, nil
	}
	if err != nil {
		return models.Upload{}, false, err
	}
	metadata := make(map[string]string)
	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			return models.Upload{}, false, fmt.Errorf("decode upload metadata: %w", err)
		}
	}
	upload := models.Upload{
		ID:        id,
		ChannelID: channelID,
		Title:     title,
		Filename:  filename,
		SizeBytes: sizeBytes,
		Status:    status,
		Progress:  progress,
		Metadata:  metadata,
		CreatedAt: createdAt.UTC(),
		UpdatedAt: updatedAt.UTC(),
	}
	if recordingID.Valid {
		value := strings.TrimSpace(recordingID.String)
		if value != "" {
			upload.RecordingID = &value
		}
	}
	if playbackURL.Valid {
		upload.PlaybackURL = playbackURL.String
	}
	if errorText.Valid {
		upload.Error = errorText.String
	}
	if completedAt.Valid {
		ts := completedAt.Time.UTC()
		upload.CompletedAt = &ts
	}
	return upload, true, nil
}

// CreateChannel creates channel and returns an error when persistence or validation fails.
func (r *postgresRepository) CreateChannel(ownerID, title, category string, tags []string) (models.Channel, error) {
	if r == nil || r.pool == nil {
		return models.Channel{}, ErrPostgresUnavailable
	}
	if strings.TrimSpace(ownerID) == "" {
		return models.Channel{}, fmt.Errorf("owner %s not found", ownerID)
	}
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return models.Channel{}, errors.New("title is required")
	}

	var (
		channel           models.Channel
		insertedCreatedAt time.Time
		insertedUpdatedAt time.Time
		streamKey         string
		id                string
		normalizedTags    []string
		trimmedCategory   string
	)
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin create channel tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)", ownerID).Scan(&exists); err != nil {
			return fmt.Errorf("check owner %s: %w", ownerID, err)
		}
		if !exists {
			return fmt.Errorf("owner %s not found", ownerID)
		}

		id, err = generateID()
		if err != nil {
			return err
		}
		streamKey, err = generateStreamKey()
		if err != nil {
			return err
		}
		normalizedTags = normalizeTags(tags)
		trimmedCategory = strings.TrimSpace(category)
		now := time.Now().UTC()

		err = tx.QueryRow(ctx, "INSERT INTO channels (id, owner_id, stream_key, title, category, tags, live_state, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, 'offline', $7, $8) RETURNING created_at, updated_at",
			id,
			ownerID,
			streamKey,
			trimmedTitle,
			trimmedCategory,
			normalizedTags,
			now,
			now,
		).Scan(&insertedCreatedAt, &insertedUpdatedAt)
		if err != nil {
			return fmt.Errorf("insert channel: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit create channel: %w", err)
		}
		return nil
	})
	if err != nil {
		return models.Channel{}, err
	}

	channel = models.Channel{
		ID:        id,
		OwnerID:   ownerID,
		StreamKey: streamKey,
		Title:     trimmedTitle,
		Category:  trimmedCategory,
		Tags:      normalizedTags,
		LiveState: "offline",
		CreatedAt: insertedCreatedAt.UTC(),
		UpdatedAt: insertedUpdatedAt.UTC(),
	}
	return channel, nil
}

// UpdateChannel updates channel and returns an error when persistence or validation fails.
func (r *postgresRepository) UpdateChannel(id string, update ChannelUpdate) (models.Channel, error) {
	if r == nil || r.pool == nil {
		return models.Channel{}, ErrPostgresUnavailable
	}
	var channel models.Channel
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin update channel tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var (
			channelID, ownerID, streamKey, title string
			category                             pgtype.Text
			tags                                 []string
			liveState                            string
			currentSession                       pgtype.Text
			createdAt, updatedAt                 time.Time
		)
		row := tx.QueryRow(ctx, "SELECT id, owner_id, stream_key, title, category, tags, live_state, current_session_id, created_at, updated_at FROM channels WHERE id = $1 FOR UPDATE", id)
		if err := row.Scan(&channelID, &ownerID, &streamKey, &title, &category, &tags, &liveState, &currentSession, &createdAt, &updatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("channel %s not found", id)
			}
			return fmt.Errorf("load channel %s: %w", id, err)
		}

		channel = models.Channel{
			ID:        channelID,
			OwnerID:   ownerID,
			StreamKey: streamKey,
			Title:     title,
			Tags:      append([]string{}, tags...),
			LiveState: liveState,
			CreatedAt: createdAt.UTC(),
			UpdatedAt: updatedAt.UTC(),
		}
		if category.Valid {
			channel.Category = category.String
		}
		if currentSession.Valid {
			id := currentSession.String
			channel.CurrentSessionID = &id
		}

		if update.Title != nil {
			trimmed := strings.TrimSpace(*update.Title)
			if trimmed == "" {
				return errors.New("title cannot be empty")
			}
			channel.Title = trimmed
		}
		if update.Category != nil {
			channel.Category = strings.TrimSpace(*update.Category)
		}
		if update.Tags != nil {
			channel.Tags = normalizeTags(*update.Tags)
		}
		if update.LiveState != nil {
			state := strings.ToLower(strings.TrimSpace(*update.LiveState))
			switch state {
			case "offline", "live", "starting", "ended":
				channel.LiveState = state
			default:
				return fmt.Errorf("invalid liveState %s", state)
			}
		}

		channel.UpdatedAt = time.Now().UTC()
		_, err = tx.Exec(ctx, "UPDATE channels SET title = $1, category = $2, tags = $3, live_state = $4, updated_at = $5 WHERE id = $6",
			channel.Title,
			channel.Category,
			channel.Tags,
			channel.LiveState,
			channel.UpdatedAt,
			channel.ID,
		)
		if err != nil {
			return fmt.Errorf("update channel %s: %w", id, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit update channel: %w", err)
		}
		return nil
	})
	if err != nil {
		return models.Channel{}, err
	}
	if channel.Tags == nil {
		channel.Tags = []string{}
	}
	return channel, nil
}

// RotateChannelStreamKey performs rotate channel stream key and returns an error when dependent systems reject the operation.
func (r *postgresRepository) RotateChannelStreamKey(id string) (models.Channel, error) {
	if r == nil || r.pool == nil {
		return models.Channel{}, ErrPostgresUnavailable
	}
	var channel models.Channel
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin rotate stream key tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var (
			channelID, ownerID, streamKey, title string
			category                             pgtype.Text
			tags                                 []string
			liveState                            string
			currentSession                       pgtype.Text
			createdAt, updatedAt                 time.Time
		)
		row := tx.QueryRow(ctx, "SELECT id, owner_id, stream_key, title, category, tags, live_state, current_session_id, created_at, updated_at FROM channels WHERE id = $1 FOR UPDATE", id)
		if err := row.Scan(&channelID, &ownerID, &streamKey, &title, &category, &tags, &liveState, &currentSession, &createdAt, &updatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("channel %s not found", id)
			}
			return fmt.Errorf("load channel %s: %w", id, err)
		}

		newKey, err := generateStreamKey()
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if _, err := tx.Exec(ctx, "UPDATE channels SET stream_key = $1, updated_at = $2 WHERE id = $3", newKey, now, id); err != nil {
			return fmt.Errorf("update stream key: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit rotate stream key: %w", err)
		}

		channel = models.Channel{
			ID:        channelID,
			OwnerID:   ownerID,
			StreamKey: newKey,
			Title:     title,
			Tags:      append([]string{}, tags...),
			LiveState: liveState,
			CreatedAt: createdAt.UTC(),
			UpdatedAt: now,
		}
		if category.Valid {
			channel.Category = category.String
		}
		if currentSession.Valid {
			current := currentSession.String
			channel.CurrentSessionID = &current
		}
		return nil
	})
	if err != nil {
		return models.Channel{}, err
	}
	if channel.Tags == nil {
		channel.Tags = []string{}
	}
	return channel, nil
}

// DeleteChannel deletes channel and returns an error when persistence or validation fails.
func (r *postgresRepository) DeleteChannel(id string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	return r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin delete channel tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var currentSession pgtype.Text
		if err := tx.QueryRow(ctx, "SELECT current_session_id FROM channels WHERE id = $1 FOR UPDATE", id).Scan(&currentSession); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("channel %s not found", id)
			}
			return fmt.Errorf("load channel %s: %w", id, err)
		}
		if currentSession.Valid {
			return errors.New("cannot delete a channel with an active stream")
		}

		if _, err := tx.Exec(ctx, "UPDATE profiles SET featured_channel_id = NULL WHERE featured_channel_id = $1", id); err != nil {
			return fmt.Errorf("clear featured channel references: %w", err)
		}
		if _, err := tx.Exec(ctx, "DELETE FROM channels WHERE id = $1", id); err != nil {
			return fmt.Errorf("delete channel %s: %w", id, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit delete channel: %w", err)
		}
		return nil
	})
}

// GetChannel returns channel from the configured backing services.
func (r *postgresRepository) GetChannel(id string) (models.Channel, bool) {
	if r == nil || r.pool == nil {
		return models.Channel{}, false
	}
	var channel models.Channel
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var (
			channelID, ownerID, streamKey, title string
			category                             pgtype.Text
			tags                                 []string
			liveState                            string
			currentSession                       pgtype.Text
			createdAt, updatedAt                 time.Time
		)
		err := conn.QueryRow(ctx, "SELECT id, owner_id, stream_key, title, category, tags, live_state, current_session_id, created_at, updated_at FROM channels WHERE id = $1", id).
			Scan(&channelID, &ownerID, &streamKey, &title, &category, &tags, &liveState, &currentSession, &createdAt, &updatedAt)
		if err != nil {
			return err
		}
		channel = models.Channel{
			ID:        channelID,
			OwnerID:   ownerID,
			StreamKey: streamKey,
			Title:     title,
			Tags:      append([]string{}, tags...),
			LiveState: liveState,
			CreatedAt: createdAt.UTC(),
			UpdatedAt: updatedAt.UTC(),
		}
		if category.Valid {
			channel.Category = category.String
		}
		if currentSession.Valid {
			current := currentSession.String
			channel.CurrentSessionID = &current
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) || err != nil {
		return models.Channel{}, false
	}
	if channel.Tags == nil {
		channel.Tags = []string{}
	}
	return channel, true
}

// GetChannelByStreamKey returns channel by stream key from the configured backing services.
func (r *postgresRepository) GetChannelByStreamKey(streamKey string) (models.Channel, bool) {
	if r == nil || r.pool == nil {
		return models.Channel{}, false
	}
	key := strings.TrimSpace(streamKey)
	if key == "" {
		return models.Channel{}, false
	}

	var channel models.Channel
	found := false
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var (
			category       pgtype.Text
			tags           []string
			currentSession pgtype.Text
			createdAt      time.Time
			updatedAt      time.Time
		)
		row := conn.QueryRow(ctx, "SELECT id, owner_id, stream_key, title, category, tags, live_state, current_session_id, created_at, updated_at FROM channels WHERE stream_key = $1", key)
		if err := row.Scan(&channel.ID, &channel.OwnerID, &channel.StreamKey, &channel.Title, &category, &tags, &channel.LiveState, &currentSession, &createdAt, &updatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return fmt.Errorf("load channel by stream key: %w", err)
		}
		channel.Tags = append([]string{}, tags...)
		if category.Valid {
			channel.Category = category.String
		}
		if currentSession.Valid {
			id := currentSession.String
			channel.CurrentSessionID = &id
		}
		channel.CreatedAt = createdAt.UTC()
		channel.UpdatedAt = updatedAt.UTC()
		found = true
		return nil
	})
	if err != nil || !found {
		return models.Channel{}, false
	}
	return channel, true
}

// ListChannels returns channels from the configured backing services.
func (r *postgresRepository) ListChannels(ownerID, query string) []models.Channel {
	if r == nil || r.pool == nil {
		return nil
	}
	ctx, cancel := r.acquireContext()
	defer cancel()
	baseQuery := "SELECT c.id, c.owner_id, c.stream_key, c.title, c.category, c.tags, c.live_state, c.current_session_id, c.created_at, c.updated_at FROM channels c JOIN users u ON u.id = c.owner_id"
	trimmedOwner := strings.TrimSpace(ownerID)
	trimmedQuery := strings.TrimSpace(query)
	var (
		args    []interface{}
		clauses []string
	)
	if trimmedOwner != "" {
		args = append(args, trimmedOwner)
		clauses = append(clauses, fmt.Sprintf("c.owner_id = $%d", len(args)))
	}
	if trimmedQuery != "" {
		args = append(args, "%"+trimmedQuery+"%")
		argPos := len(args)
		clauses = append(clauses, fmt.Sprintf("(c.title ILIKE $%[1]d OR u.display_name ILIKE $%[1]d OR EXISTS (SELECT 1 FROM unnest(c.tags) AS tag WHERE tag ILIKE $%[1]d))", argPos))
	}
	if len(clauses) > 0 {
		baseQuery += " WHERE " + strings.Join(clauses, " AND ")
	}
	baseQuery += " ORDER BY CASE WHEN c.live_state = 'live' THEN 0 ELSE 1 END, c.created_at ASC"
	rows, err := r.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	channels := make([]models.Channel, 0)
	for rows.Next() {
		var (
			channelID, ownerIDVal, streamKey, title string
			category                                pgtype.Text
			tags                                    []string
			liveState                               string
			currentSession                          pgtype.Text
			createdAt, updatedAt                    time.Time
		)
		if err := rows.Scan(&channelID, &ownerIDVal, &streamKey, &title, &category, &tags, &liveState, &currentSession, &createdAt, &updatedAt); err != nil {
			return nil
		}
		channel := models.Channel{
			ID:        channelID,
			OwnerID:   ownerIDVal,
			StreamKey: streamKey,
			Title:     title,
			Tags:      append([]string{}, tags...),
			LiveState: liveState,
			CreatedAt: createdAt.UTC(),
			UpdatedAt: updatedAt.UTC(),
		}
		if category.Valid {
			channel.Category = category.String
		}
		if currentSession.Valid {
			current := currentSession.String
			channel.CurrentSessionID = &current
		}
		if channel.Tags == nil {
			channel.Tags = []string{}
		}
		channels = append(channels, channel)
	}
	if err := rows.Err(); err != nil {
		return nil
	}
	return channels
}

// FollowChannel performs follow channel and returns an error when dependent systems reject the operation.
func (r *postgresRepository) FollowChannel(userID, channelID string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	return r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin follow channel tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if err := ensureUserExists(ctx, tx, userID); err != nil {
			return err
		}
		if err := ensureChannelExists(ctx, tx, channelID); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, "INSERT INTO follows (user_id, channel_id, followed_at) VALUES ($1, $2, NOW()) ON CONFLICT DO NOTHING", userID, channelID); err != nil {
			return fmt.Errorf("follow channel %s: %w", channelID, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit follow channel: %w", err)
		}
		return nil
	})
}

// UnfollowChannel performs unfollow channel and returns an error when dependent systems reject the operation.
func (r *postgresRepository) UnfollowChannel(userID, channelID string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	return r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin unfollow channel tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if err := ensureUserExists(ctx, tx, userID); err != nil {
			return err
		}
		if err := ensureChannelExists(ctx, tx, channelID); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, "DELETE FROM follows WHERE user_id = $1 AND channel_id = $2", userID, channelID); err != nil {
			return fmt.Errorf("unfollow channel %s: %w", channelID, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit unfollow channel: %w", err)
		}
		return nil
	})
}

// IsFollowingChannel reports whether following channel is satisfied for the current input.
func (r *postgresRepository) IsFollowingChannel(userID, channelID string) bool {
	if r == nil || r.pool == nil {
		return false
	}
	var exists bool
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		return conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM follows WHERE user_id = $1 AND channel_id = $2)", userID, channelID).Scan(&exists)
	})
	if err != nil {
		return false
	}
	return exists
}

// CountFollowers performs count followers and returns an error when dependent systems reject the operation.
func (r *postgresRepository) CountFollowers(channelID string) int {
	if r == nil || r.pool == nil {
		return 0
	}
	var count int
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		return conn.QueryRow(ctx, "SELECT COUNT(*) FROM follows WHERE channel_id = $1", channelID).Scan(&count)
	})
	if err != nil {
		return 0
	}
	return count
}

// ListFollowedChannelIDs returns followed channel ids from the configured backing services.
func (r *postgresRepository) ListFollowedChannelIDs(userID string) []string {
	if r == nil || r.pool == nil {
		return nil
	}
	ids := make([]string, 0)
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		rows, err := conn.Query(ctx, "SELECT channel_id FROM follows WHERE user_id = $1 ORDER BY followed_at DESC", userID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var channelID string
			if err := rows.Scan(&channelID); err != nil {
				return err
			}
			ids = append(ids, channelID)
		}
		return rows.Err()
	})
	if err != nil {
		return nil
	}
	return ids
}

// StartStream starts stream and returns when setup fails or shutdown is requested.
func (r *postgresRepository) StartStream(channelID string, renditions []string) (models.StreamSession, error) {
	if r == nil || r.pool == nil {
		return models.StreamSession{}, ErrPostgresUnavailable
	}
	var (
		streamKey      string
		sessionID      string
		startedAt      time.Time
		currentSession pgtype.Text
	)
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin start stream tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var (
			ownerID, title, category pgtype.Text
			tags                     []string
		)
		row := tx.QueryRow(ctx, "SELECT stream_key, current_session_id, owner_id, title, category, tags FROM channels WHERE id = $1 FOR UPDATE", channelID)
		if err := row.Scan(&streamKey, &currentSession, &ownerID, &title, &category, &tags); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("channel %s not found", channelID)
			}
			return fmt.Errorf("load channel %s: %w", channelID, err)
		}
		if currentSession.Valid {
			return errors.New("channel already live")
		}

		sessionID, err = generateID()
		if err != nil {
			return err
		}
		startedAt = time.Now().UTC()
		if _, err := tx.Exec(ctx, "UPDATE channels SET current_session_id = $1, live_state = 'starting', updated_at = $2 WHERE id = $3", sessionID, startedAt, channelID); err != nil {
			return fmt.Errorf("mark channel starting: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit mark channel starting: %w", err)
		}
		return nil
	})
	if err != nil {
		return models.StreamSession{}, err
	}

	attempts := r.ingestMaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	controller := r.ingestController
	if controller == nil {
		_ = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
			_, err := conn.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = NOW() WHERE id = $1", channelID)
			return err
		})
		return models.StreamSession{}, ErrIngestControllerUnavailable
	}
	deadline := normalizeIngestTimeout(r.ingestTimeout)
	var boot ingest.BootResult
	var bootErr error
	for attempt := 0; attempt < attempts; attempt++ {
		bootCtx, cancel := context.WithTimeout(context.Background(), deadline)
		boot, bootErr = controller.BootStream(bootCtx, ingest.BootParams{
			ChannelID:  channelID,
			SessionID:  sessionID,
			StreamKey:  streamKey,
			Renditions: append([]string{}, renditions...),
		})
		cancel()
		if bootErr == nil {
			break
		}
		if attempt < attempts-1 && r.ingestRetryInterval > 0 {
			time.Sleep(r.ingestRetryInterval)
		}
	}
	if bootErr != nil {
		_ = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
			_, err := conn.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = NOW() WHERE id = $1", channelID)
			return err
		})
		return models.StreamSession{}, fmt.Errorf("boot ingest: %w", bootErr)
	}

	session := models.StreamSession{
		ID:             sessionID,
		ChannelID:      channelID,
		StartedAt:      startedAt,
		Renditions:     append([]string{}, renditions...),
		PeakConcurrent: 0,
		OriginURL:      boot.OriginURL,
		PlaybackURL:    boot.PlaybackURL,
		IngestJobIDs:   append([]string{}, boot.JobIDs...),
	}
	ingestEndpoints := make([]string, 0, 2)
	if boot.PrimaryIngest != "" {
		ingestEndpoints = append(ingestEndpoints, boot.PrimaryIngest)
	}
	if boot.BackupIngest != "" {
		ingestEndpoints = append(ingestEndpoints, boot.BackupIngest)
	}
	session.IngestEndpoints = ingestEndpoints
	if len(boot.Renditions) > 0 {
		manifests := make([]models.RenditionManifest, 0, len(boot.Renditions))
		for _, rendition := range boot.Renditions {
			manifests = append(manifests, models.RenditionManifest{
				Name:        rendition.Name,
				ManifestURL: rendition.ManifestURL,
				Bitrate:     rendition.Bitrate,
			})
		}
		session.RenditionManifests = manifests
	}

	revertChannel := func() {
		_ = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
			_, err := conn.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = NOW() WHERE id = $1", channelID)
			return err
		})
	}
	shutdownIngest := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), deadline)
		_ = controller.ShutdownStream(shutdownCtx, channelID, sessionID, append([]string{}, session.IngestJobIDs...))
		cancel()
		revertChannel()
	}

	persistErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin persist stream session: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if _, err := tx.Exec(ctx, "INSERT INTO stream_sessions (id, channel_id, started_at, renditions, peak_concurrent, origin_url, playback_url, ingest_endpoints, ingest_job_ids) VALUES ($1, $2, $3, $4, 0, $5, $6, $7, $8)",
			session.ID,
			session.ChannelID,
			session.StartedAt,
			session.Renditions,
			session.OriginURL,
			session.PlaybackURL,
			session.IngestEndpoints,
			session.IngestJobIDs,
		); err != nil {
			return fmt.Errorf("insert stream session: %w", err)
		}
		for _, manifest := range session.RenditionManifests {
			if _, err := tx.Exec(ctx, "INSERT INTO stream_session_manifests (session_id, name, manifest_url, bitrate) VALUES ($1, $2, $3, $4)", session.ID, manifest.Name, manifest.ManifestURL, manifest.Bitrate); err != nil {
				return fmt.Errorf("insert rendition manifest: %w", err)
			}
		}
		if _, err := tx.Exec(ctx, "UPDATE channels SET current_session_id = $1, live_state = 'live', updated_at = $2 WHERE id = $3", session.ID, session.StartedAt, channelID); err != nil {
			return fmt.Errorf("mark channel live: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit start stream: %w", err)
		}
		return nil
	})
	if persistErr != nil {
		shutdownIngest()
		return models.StreamSession{}, persistErr
	}

	return session, nil
}

// StopStream stops stream and returns an error when cleanup operations fail.
func (r *postgresRepository) StopStream(channelID string, peakConcurrent int) (session models.StreamSession, err error) {
	if r == nil || r.pool == nil {
		return models.StreamSession{}, ErrPostgresUnavailable
	}

	var (
		channelTitle         string
		channelCategory      pgtype.Text
		channelTags          []string
		channelWasLive       bool
		cleanupAfterShutdown bool
		stopTimestamp        time.Time
	)
	defer func() {
		if err == nil || !channelWasLive || !cleanupAfterShutdown {
			return
		}
		timestamp := stopTimestamp
		if timestamp.IsZero() {
			timestamp = time.Now().UTC()
		}
		cleanupErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
			if _, execErr := conn.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = $1 WHERE id = $2", timestamp, channelID); execErr != nil {
				return fmt.Errorf("update channel %s: %w", channelID, execErr)
			}
			return nil
		})
		if cleanupErr != nil {
			err = fmt.Errorf("%w; cleanup stop stream: %v", err, cleanupErr)
		}
	}()

	err = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin stop stream tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var (
			streamKey       string
			currentSession  pgtype.Text
			renditions      []string
			ingestEndpoints []string
			ingestJobIDs    []string
			peak            int
			startedAt       time.Time
			endedAt         pgtype.Timestamptz
			originURL       string
			playbackURL     string
		)
		row := tx.QueryRow(ctx, "SELECT stream_key, current_session_id, title, category, tags FROM channels WHERE id = $1 FOR UPDATE", channelID)
		if err := row.Scan(&streamKey, &currentSession, &channelTitle, &channelCategory, &channelTags); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("channel %s not found", channelID)
			}
			return fmt.Errorf("load channel %s: %w", channelID, err)
		}
		if !currentSession.Valid {
			return errors.New("channel is not live")
		}
		channelWasLive = true
		sessionID := currentSession.String

		sessRow := tx.QueryRow(ctx, "SELECT started_at, ended_at, renditions, peak_concurrent, origin_url, playback_url, ingest_endpoints, ingest_job_ids FROM stream_sessions WHERE id = $1 FOR UPDATE", sessionID)
		if err := sessRow.Scan(&startedAt, &endedAt, &renditions, &peak, &originURL, &playbackURL, &ingestEndpoints, &ingestJobIDs); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("session %s missing", sessionID)
			}
			return fmt.Errorf("load session %s: %w", sessionID, err)
		}
		manifestsRows, err := tx.Query(ctx, "SELECT name, manifest_url, bitrate FROM stream_session_manifests WHERE session_id = $1", sessionID)
		if err != nil {
			return fmt.Errorf("load session manifests: %w", err)
		}
		manifests := make([]models.RenditionManifest, 0)
		for manifestsRows.Next() {
			var name, url string
			var bitrate pgtype.Int4
			if err := manifestsRows.Scan(&name, &url, &bitrate); err != nil {
				manifestsRows.Close()
				return fmt.Errorf("scan session manifest: %w", err)
			}
			entry := models.RenditionManifest{Name: name, ManifestURL: url}
			if bitrate.Valid {
				entry.Bitrate = int(bitrate.Int32)
			}
			manifests = append(manifests, entry)
		}
		manifestsRows.Close()
		if err := manifestsRows.Err(); err != nil {
			return fmt.Errorf("read session manifests: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit load session: %w", err)
		}

		session = models.StreamSession{
			ID:                 sessionID,
			ChannelID:          channelID,
			StartedAt:          startedAt.UTC(),
			Renditions:         append([]string{}, renditions...),
			PeakConcurrent:     peak,
			OriginURL:          originURL,
			PlaybackURL:        playbackURL,
			IngestEndpoints:    append([]string{}, ingestEndpoints...),
			IngestJobIDs:       append([]string{}, ingestJobIDs...),
			RenditionManifests: append([]models.RenditionManifest{}, manifests...),
		}
		if endedAt.Valid {
			ts := endedAt.Time.UTC()
			session.EndedAt = &ts
		}
		return nil
	})
	if err != nil {
		return models.StreamSession{}, err
	}

	deadline := normalizeIngestTimeout(r.ingestTimeout)
	controller := r.ingestController
	if controller == nil {
		return models.StreamSession{}, ErrIngestControllerUnavailable
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	if err := controller.ShutdownStream(shutdownCtx, channelID, session.ID, append([]string{}, session.IngestJobIDs...)); err != nil {
		return models.StreamSession{}, fmt.Errorf("shutdown ingest: %w", err)
	}
	cleanupAfterShutdown = true

	stopTimestamp = time.Now().UTC()
	session.EndedAt = &stopTimestamp
	if peakConcurrent > session.PeakConcurrent {
		session.PeakConcurrent = peakConcurrent
	}

	channel := models.Channel{ID: channelID, Title: channelTitle}
	if channelCategory.Valid {
		channel.Category = channelCategory.String
	}
	if len(channelTags) > 0 {
		channel.Tags = append([]string{}, channelTags...)
	}

	recording, recErr := r.createRecording(session, channel, stopTimestamp)
	if recErr != nil {
		return models.StreamSession{}, recErr
	}

	err = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin finalize stop stream tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if _, err := tx.Exec(ctx, "UPDATE stream_sessions SET ended_at = $1, peak_concurrent = $2 WHERE id = $3", session.EndedAt, session.PeakConcurrent, session.ID); err != nil {
			return fmt.Errorf("update stream session %s: %w", session.ID, err)
		}
		if _, err := tx.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = $1 WHERE id = $2", stopTimestamp, channelID); err != nil {
			return fmt.Errorf("update channel %s: %w", channelID, err)
		}
		if recording.ID != "" {
			if err := r.insertRecording(ctx, tx, recording); err != nil {
				return err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit stop stream: %w", err)
		}
		return nil
	})
	if err != nil {
		return models.StreamSession{}, err
	}

	return session, nil
}

// CurrentStreamSession performs current stream session and returns an error when dependent systems reject the operation.
func (r *postgresRepository) CurrentStreamSession(channelID string) (models.StreamSession, bool) {
	if r == nil || r.pool == nil {
		return models.StreamSession{}, false
	}
	var current pgtype.Text
	if err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		return conn.QueryRow(ctx, "SELECT current_session_id FROM channels WHERE id = $1", channelID).Scan(&current)
	}); err != nil {
		return models.StreamSession{}, false
	}
	if !current.Valid {
		return models.StreamSession{}, false
	}
	loadCtx, cancel := r.acquireContext()
	defer cancel()
	session, ok := r.loadStreamSession(loadCtx, current.String)
	if !ok {
		return models.StreamSession{}, false
	}
	return session, true
}

// ListStreamSessions returns stream sessions from the configured backing services.
func (r *postgresRepository) ListStreamSessions(channelID string) ([]models.StreamSession, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	ids := make([]string, 0)
	if err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
			return fmt.Errorf("check channel %s: %w", channelID, err)
		}
		if !exists {
			return fmt.Errorf("channel %s not found", channelID)
		}
		rows, err := conn.Query(ctx, "SELECT id FROM stream_sessions WHERE channel_id = $1 ORDER BY started_at DESC", channelID)
		if err != nil {
			return fmt.Errorf("list sessions: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("scan session id: %w", err)
			}
			ids = append(ids, id)
		}
		return rows.Err()
	}); err != nil {
		return nil, err
	}

	sessions := make([]models.StreamSession, 0, len(ids))
	for _, id := range ids {
		loadCtx, cancel := r.acquireContext()
		session, ok := r.loadStreamSession(loadCtx, id)
		cancel()
		if !ok {
			continue
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

// ListRecordings returns recordings from the configured backing services.
func (r *postgresRepository) ListRecordings(channelID string, includeUnpublished bool) ([]models.Recording, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	ids := make([]string, 0)
	if err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
			return fmt.Errorf("check channel %s: %w", channelID, err)
		}
		if !exists {
			return fmt.Errorf("channel %s not found", channelID)
		}
		if err := r.purgeExpiredRecordings(ctx, r.retentionTime()); err != nil {
			slog.Default().Warn("purge expired recordings failed", "channel_id", channelID, "error", err)
		}
		query := "SELECT id FROM recordings WHERE channel_id = $1"
		if !includeUnpublished {
			query += " AND published_at IS NOT NULL"
		}
		query += " ORDER BY created_at DESC"
		rows, err := conn.Query(ctx, query, channelID)
		if err != nil {
			return fmt.Errorf("list recordings: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("scan recording id: %w", err)
			}
			ids = append(ids, id)
		}
		return rows.Err()
	}); err != nil {
		return nil, err
	}

	recordings := make([]models.Recording, 0, len(ids))
	for _, id := range ids {
		loadCtx, cancel := r.acquireContext()
		recording, ok, loadErr := r.loadRecording(loadCtx, id)
		cancel()
		if loadErr != nil {
			return nil, loadErr
		}
		if !ok {
			continue
		}
		recordings = append(recordings, recording)
	}
	return recordings, nil
}

// CreateUpload creates upload and returns an error when persistence or validation fails.
func (r *postgresRepository) CreateUpload(params CreateUploadParams) (models.Upload, error) {
	if r == nil || r.pool == nil {
		return models.Upload{}, ErrPostgresUnavailable
	}
	channelID := strings.TrimSpace(params.ChannelID)
	if channelID == "" {
		return models.Upload{}, fmt.Errorf("channelId is required")
	}
	title := strings.TrimSpace(params.Title)
	if title == "" {
		title = "Uploaded video"
	}
	filename := strings.TrimSpace(params.Filename)
	if filename == "" {
		filename = "upload.mp4"
	}
	metadata := make(map[string]string, len(params.Metadata))
	for k, v := range params.Metadata {
		if strings.TrimSpace(k) == "" {
			continue
		}
		metadata[k] = v
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return models.Upload{}, fmt.Errorf("encode metadata: %w", err)
	}
	playbackURL := strings.TrimSpace(params.PlaybackURL)

	upload := models.Upload{}
	err = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
			return fmt.Errorf("check channel %s: %w", channelID, err)
		}
		if !exists {
			return fmt.Errorf("channel %s not found", channelID)
		}

		id, err := generateID()
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if _, err := conn.Exec(ctx, "INSERT INTO uploads (id, channel_id, title, filename, size_bytes, status, progress, playback_url, metadata, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'pending', 0, $6, $7, $8, $9)",
			id,
			channelID,
			title,
			filename,
			params.SizeBytes,
			playbackURL,
			metadataJSON,
			now,
			now,
		); err != nil {
			return fmt.Errorf("insert upload: %w", err)
		}
		upload = models.Upload{
			ID:          id,
			ChannelID:   channelID,
			Title:       title,
			Filename:    filename,
			SizeBytes:   params.SizeBytes,
			Status:      "pending",
			Progress:    0,
			Metadata:    metadata,
			PlaybackURL: playbackURL,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		return nil
	})
	if err != nil {
		return models.Upload{}, err
	}
	return upload, nil
}

// ListUploads returns uploads from the configured backing services.
func (r *postgresRepository) ListUploads(channelID string) ([]models.Upload, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	ids := make([]string, 0)
	if err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
			return fmt.Errorf("check channel %s: %w", channelID, err)
		}
		if !exists {
			return fmt.Errorf("channel %s not found", channelID)
		}
		rows, err := conn.Query(ctx, "SELECT id FROM uploads WHERE channel_id = $1 ORDER BY created_at DESC", channelID)
		if err != nil {
			return fmt.Errorf("list uploads: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("scan upload id: %w", err)
			}
			ids = append(ids, id)
		}
		return rows.Err()
	}); err != nil {
		return nil, err
	}

	uploads := make([]models.Upload, 0, len(ids))
	for _, id := range ids {
		loadCtx, cancel := r.acquireContext()
		upload, ok, loadErr := r.loadUpload(loadCtx, id)
		cancel()
		if loadErr != nil {
			return nil, loadErr
		}
		if !ok {
			continue
		}
		uploads = append(uploads, upload)
	}
	return uploads, nil
}

// GetUpload returns upload from the configured backing services.
func (r *postgresRepository) GetUpload(id string) (models.Upload, bool) {
	if r == nil || r.pool == nil {
		return models.Upload{}, false
	}
	ctx, cancel := r.acquireContext()
	upload, ok, err := r.loadUpload(ctx, id)
	cancel()
	if err != nil || !ok {
		return models.Upload{}, false
	}
	return upload, true
}

// UpdateUpload updates upload and returns an error when persistence or validation fails.
func (r *postgresRepository) UpdateUpload(id string, update UploadUpdate) (models.Upload, error) {
	if r == nil || r.pool == nil {
		return models.Upload{}, ErrPostgresUnavailable
	}
	var result models.Upload
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin update upload tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		upload, ok, err := r.loadUpload(ctx, id)
		if err != nil {
			return fmt.Errorf("load upload %s: %w", id, err)
		}
		if !ok {
			return fmt.Errorf("upload %s not found", id)
		}

		if update.Title != nil {
			if trimmed := strings.TrimSpace(*update.Title); trimmed != "" {
				upload.Title = trimmed
			}
		}
		if update.Status != nil {
			upload.Status = strings.TrimSpace(*update.Status)
		}
		if update.Progress != nil {
			progress := *update.Progress
			if progress < 0 {
				progress = 0
			}
			if progress > 100 {
				progress = 100
			}
			upload.Progress = progress
		}
		if update.RecordingID != nil {
			trimmed := strings.TrimSpace(*update.RecordingID)
			if trimmed == "" {
				upload.RecordingID = nil
			} else {
				upload.RecordingID = &trimmed
			}
		}
		if update.PlaybackURL != nil {
			upload.PlaybackURL = strings.TrimSpace(*update.PlaybackURL)
		}
		if update.Metadata != nil {
			if upload.Metadata == nil {
				upload.Metadata = make(map[string]string, len(update.Metadata))
			}
			for k, v := range update.Metadata {
				if strings.TrimSpace(k) == "" {
					continue
				}
				if v == "" {
					delete(upload.Metadata, k)
					continue
				}
				upload.Metadata[k] = v
			}
		}
		if update.Error != nil {
			upload.Error = strings.TrimSpace(*update.Error)
		}
		if update.CompletedAt != nil {
			if update.CompletedAt.IsZero() {
				upload.CompletedAt = nil
			} else {
				ts := update.CompletedAt.UTC()
				upload.CompletedAt = &ts
			}
		}

		upload.UpdatedAt = time.Now().UTC()

		metadataJSON, err := json.Marshal(upload.Metadata)
		if err != nil {
			return fmt.Errorf("encode metadata: %w", err)
		}
		var recordingID interface{}
		if upload.RecordingID != nil {
			recordingID = *upload.RecordingID
		}
		var completedAt interface{}
		if upload.CompletedAt != nil {
			completedAt = *upload.CompletedAt
		}
		if _, err := tx.Exec(ctx, "UPDATE uploads SET title = $1, status = $2, progress = $3, recording_id = $4, playback_url = $5, metadata = $6, error = $7, completed_at = $8, updated_at = $9 WHERE id = $10",
			upload.Title,
			upload.Status,
			upload.Progress,
			recordingID,
			upload.PlaybackURL,
			metadataJSON,
			upload.Error,
			completedAt,
			upload.UpdatedAt,
			id,
		); err != nil {
			return fmt.Errorf("update upload %s: %w", id, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit update upload: %w", err)
		}
		result = upload
		return nil
	})
	if err != nil {
		return models.Upload{}, err
	}
	return result, nil
}

// DeleteUpload deletes upload and returns an error when persistence or validation fails.
func (r *postgresRepository) DeleteUpload(id string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	ctx, cancel := r.acquireContext()
	command, err := r.pool.Exec(ctx, "DELETE FROM uploads WHERE id = $1", id)
	cancel()
	if err != nil {
		return fmt.Errorf("delete upload %s: %w", id, err)
	}
	if command.RowsAffected() == 0 {
		return fmt.Errorf("upload %s not found", id)
	}
	return nil
}

// GetRecording returns recording from the configured backing services.
func (r *postgresRepository) GetRecording(id string) (models.Recording, bool) {
	if r == nil || r.pool == nil {
		return models.Recording{}, false
	}
	ctx, cancel := r.acquireContext()
	if err := r.purgeExpiredRecordings(ctx, r.retentionTime()); err != nil {
		slog.Default().Warn("purge expired recordings failed", "recording_id", id, "error", err)
	}
	recording, ok, err := r.loadRecording(ctx, id)
	cancel()
	if err != nil || !ok {
		return models.Recording{}, false
	}
	return recording, true
}

// PublishRecording performs publish recording and returns an error when dependent systems reject the operation.
func (r *postgresRepository) PublishRecording(id string) (models.Recording, error) {
	if r == nil || r.pool == nil {
		return models.Recording{}, ErrPostgresUnavailable
	}

	var recording models.Recording
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin publish recording tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var (
			channelID       string
			sessionID       string
			title           string
			duration        int
			playbackBaseURL string
			metadataBytes   []byte
			createdAt       time.Time
			retainUntil     pgtype.Timestamptz
			publishedAt     pgtype.Timestamptz
		)
		err = tx.QueryRow(ctx, "SELECT channel_id, session_id, title, duration_seconds, playback_base_url, metadata, created_at, retain_until, published_at FROM recordings WHERE id = $1 FOR UPDATE", id).
			Scan(&channelID, &sessionID, &title, &duration, &playbackBaseURL, &metadataBytes, &createdAt, &retainUntil, &publishedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("recording %s not found", id)
		}
		if err != nil {
			return fmt.Errorf("load recording %s: %w", id, err)
		}
		if publishedAt.Valid {
			rec, _, loadErr := r.loadRecording(ctx, id)
			if loadErr != nil {
				return loadErr
			}
			recording = rec
			return nil
		}
		now := time.Now().UTC()
		if _, err := tx.Exec(ctx, "UPDATE recordings SET published_at = $1 WHERE id = $2", now, id); err != nil {
			return fmt.Errorf("publish recording %s: %w", id, err)
		}
		if deadline := r.recordingDeadline(now, true); deadline != nil {
			if _, err := tx.Exec(ctx, "UPDATE recordings SET retain_until = $1 WHERE id = $2", deadline, id); err != nil {
				return fmt.Errorf("update recording retention: %w", err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit publish recording: %w", err)
		}
		rec, _, loadErr := r.loadRecording(ctx, id)
		if loadErr != nil {
			return loadErr
		}
		if rec.ID == "" {
			return fmt.Errorf("recording %s not found", id)
		}
		recording = rec
		return nil
	})
	if err != nil {
		return models.Recording{}, err
	}
	return recording, nil
}

// DeleteRecording deletes recording and returns an error when persistence or validation fails.
func (r *postgresRepository) DeleteRecording(id string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	ctx, cancel := r.acquireContext()
	recording, ok, err := r.loadRecording(ctx, id)
	if err != nil {
		cancel()
		return err
	}
	if !ok {
		cancel()
		return fmt.Errorf("recording %s not found", id)
	}
	if err := r.deleteRecordingArtifacts(recording); err != nil {
		cancel()
		return err
	}
	clipRows, err := r.pool.Query(ctx, "SELECT id, storage_object FROM clip_exports WHERE recording_id = $1", id)
	if err != nil {
		cancel()
		return fmt.Errorf("load clip exports: %w", err)
	}
	clips := make([]models.ClipExport, 0)
	for clipRows.Next() {
		var clip models.ClipExport
		var storageObject pgtype.Text
		if err := clipRows.Scan(&clip.ID, &storageObject); err != nil {
			clipRows.Close()
			return fmt.Errorf("scan clip export: %w", err)
		}
		if storageObject.Valid {
			clip.StorageObject = storageObject.String
		}
		clips = append(clips, clip)
	}
	clipRows.Close()
	for _, clip := range clips {
		if err := r.deleteClipArtifacts(clip); err != nil {
			cancel()
			return err
		}
	}
	_, err = r.pool.Exec(ctx, "DELETE FROM recordings WHERE id = $1", id)
	cancel()
	if err != nil {
		return fmt.Errorf("delete recording %s: %w", id, err)
	}
	return nil
}

// CreateClipExport creates clip export and returns an error when persistence or validation fails.
func (r *postgresRepository) CreateClipExport(recordingID string, params ClipExportParams) (models.ClipExport, error) {
	if r == nil || r.pool == nil {
		return models.ClipExport{}, ErrPostgresUnavailable
	}
	if strings.TrimSpace(recordingID) == "" {
		return models.ClipExport{}, fmt.Errorf("recording id is required")
	}
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return models.ClipExport{}, fmt.Errorf("title is required")
	}
	clip := models.ClipExport{}
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var (
			channelID string
			sessionID string
			duration  int
		)
		if err := conn.QueryRow(ctx, "SELECT channel_id, session_id, duration_seconds FROM recordings WHERE id = $1", recordingID).
			Scan(&channelID, &sessionID, &duration); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("recording %s not found", recordingID)
			}
			return fmt.Errorf("load recording %s: %w", recordingID, err)
		}
		if params.EndSeconds <= params.StartSeconds {
			return fmt.Errorf("endSeconds must be greater than startSeconds")
		}
		if params.StartSeconds < 0 {
			return fmt.Errorf("startSeconds must be non-negative")
		}
		if duration > 0 && params.EndSeconds > duration {
			return fmt.Errorf("clip exceeds recording duration")
		}
		id, err := generateID()
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		newClip := models.ClipExport{
			ID:           id,
			RecordingID:  recordingID,
			ChannelID:    channelID,
			SessionID:    sessionID,
			Title:        title,
			StartSeconds: params.StartSeconds,
			EndSeconds:   params.EndSeconds,
			Status:       "pending",
			CreatedAt:    now,
		}
		if _, err := conn.Exec(ctx, "INSERT INTO clip_exports (id, recording_id, channel_id, session_id, title, start_seconds, end_seconds, status, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			newClip.ID,
			newClip.RecordingID,
			newClip.ChannelID,
			newClip.SessionID,
			newClip.Title,
			newClip.StartSeconds,
			newClip.EndSeconds,
			newClip.Status,
			newClip.CreatedAt,
		); err != nil {
			return fmt.Errorf("insert clip export: %w", err)
		}
		clip = newClip
		return nil
	})
	if err != nil {
		return models.ClipExport{}, err
	}
	return clip, nil
}

// ListClipExports returns clip exports from the configured backing services.
func (r *postgresRepository) ListClipExports(recordingID string) ([]models.ClipExport, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	if strings.TrimSpace(recordingID) == "" {
		return nil, fmt.Errorf("recording id is required")
	}
	clips := make([]models.ClipExport, 0)
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM recordings WHERE id = $1)", recordingID).Scan(&exists); err != nil {
			return fmt.Errorf("check recording %s: %w", recordingID, err)
		}
		if !exists {
			return fmt.Errorf("recording %s not found", recordingID)
		}
		rows, err := conn.Query(ctx, "SELECT id, recording_id, channel_id, session_id, title, start_seconds, end_seconds, status, playback_url, created_at, completed_at, storage_object FROM clip_exports WHERE recording_id = $1 ORDER BY created_at DESC", recordingID)
		if err != nil {
			return fmt.Errorf("list clip exports: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var clip models.ClipExport
			var completedAt pgtype.Timestamptz
			var playbackURL pgtype.Text
			var storageObject pgtype.Text
			if err := rows.Scan(&clip.ID, &clip.RecordingID, &clip.ChannelID, &clip.SessionID, &clip.Title, &clip.StartSeconds, &clip.EndSeconds, &clip.Status, &playbackURL, &clip.CreatedAt, &completedAt, &storageObject); err != nil {
				return fmt.Errorf("scan clip export: %w", err)
			}
			if completedAt.Valid {
				ts := completedAt.Time.UTC()
				clip.CompletedAt = &ts
			}
			if playbackURL.Valid {
				clip.PlaybackURL = playbackURL.String
			}
			if storageObject.Valid {
				clip.StorageObject = storageObject.String
			}
			clips = append(clips, clip)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return clips, nil
}
