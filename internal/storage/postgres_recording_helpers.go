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

	"bitriver-live/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// loadStreamSession executes loadStreamSession.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this signature does not return `error`; not-found/absence is represented by the
// boolean return value.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) loadStreamSession(ctx context.Context, id string) (domain.StreamSession, bool) {
	if strings.TrimSpace(id) == "" {
		return domain.StreamSession{}, false
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
		return domain.StreamSession{}, false
	}
	manifestsRows, err := r.pool.Query(ctx, "SELECT name, manifest_url, bitrate FROM stream_session_manifests WHERE session_id = $1", id)
	if err != nil {
		return domain.StreamSession{}, false
	}
	defer manifestsRows.Close()
	manifests := make([]domain.RenditionManifest, 0)
	for manifestsRows.Next() {
		var name, url string
		var bitrate pgtype.Int4
		if err := manifestsRows.Scan(&name, &url, &bitrate); err != nil {
			return domain.StreamSession{}, false
		}
		entry := domain.RenditionManifest{Name: name, ManifestURL: url}
		if bitrate.Valid {
			entry.Bitrate = int(bitrate.Int32)
		}
		manifests = append(manifests, entry)
	}
	if err := manifestsRows.Err(); err != nil {
		return domain.StreamSession{}, false
	}
	session := domain.StreamSession{
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
		session.RenditionManifests = []domain.RenditionManifest{}
	}
	if session.IngestEndpoints == nil {
		session.IngestEndpoints = []string{}
	}
	if session.IngestJobIDs == nil {
		session.IngestJobIDs = []string{}
	}
	return session, true
}

// recordingDeadline executes recordingDeadline.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
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

// createRecording executes createRecording.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) createRecording(session domain.StreamSession, channel domain.Channel, ended time.Time) (domain.Recording, error) {
	recordingID, err := generateID()
	if err != nil {
		return domain.Recording{}, err
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
	recording := domain.Recording{
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
		renditions := make([]domain.RecordingRendition, 0, len(session.RenditionManifests))
		for _, manifest := range session.RenditionManifests {
			renditions = append(renditions, domain.RecordingRendition(manifest))
		}
		recording.Renditions = renditions
	}
	if err := r.populateRecordingArtifacts(&recording, session); err != nil {
		return domain.Recording{}, err
	}
	return recording, nil
}

// populateRecordingArtifacts executes populateRecordingArtifacts.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) populateRecordingArtifacts(recording *domain.Recording, session domain.StreamSession) error {
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
	thumbnail := domain.RecordingThumbnail{
		ID:          thumbID,
		RecordingID: recording.ID,
		URL:         ref.URL,
		CreatedAt:   recording.CreatedAt,
	}
	recording.Thumbnails = append(recording.Thumbnails, thumbnail)
	return nil
}

// insertRecording executes insertRecording.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: requires an already-open pgx transaction supplied by the caller;
// it does not commit/rollback and does not retry automatically.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) insertRecording(ctx context.Context, tx pgx.Tx, recording domain.Recording) error {
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

// deleteRecordingArtifacts executes deleteRecordingArtifacts.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) deleteRecordingArtifacts(recording domain.Recording) error {
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

// deleteClipArtifacts executes deleteClipArtifacts.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) deleteClipArtifacts(clip domain.ClipExport) error {
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

// retentionTime executes retentionTime.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) retentionTime() time.Time {
	if r.retentionNow != nil {
		return r.retentionNow()
	}
	return time.Now().UTC()
}

// runRecordingRetention executes runRecordingRetention.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) runRecordingRetention(ctx context.Context) error {
	return r.purgeExpiredRecordings(ctx, r.retentionTime())
}

// purgeExpiredRecordings executes purgeExpiredRecordings.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
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
	recordings := make(map[string]domain.Recording)
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
		recordings[id] = domain.Recording{ID: id, Metadata: meta}
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
		clips := make([]domain.ClipExport, 0)
		for clipRows.Next() {
			var clip domain.ClipExport
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

// loadRecording executes loadRecording.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns infrastructure/persistence errors as wrapped `error` values; not-found is
// represented by the boolean return when provided by this signature.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) loadRecording(ctx context.Context, id string) (domain.Recording, bool, error) {
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
		return domain.Recording{}, false, nil
	}
	if err != nil {
		return domain.Recording{}, false, err
	}
	metadata := make(map[string]string)
	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			return domain.Recording{}, false, fmt.Errorf("decode recording metadata: %w", err)
		}
	}
	recording := domain.Recording{
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
		return domain.Recording{}, false, fmt.Errorf("load recording renditions: %w", err)
	}
	renditions := make([]domain.RecordingRendition, 0)
	for renditionsRows.Next() {
		var name, url string
		var bitrate pgtype.Int4
		if err := renditionsRows.Scan(&name, &url, &bitrate); err != nil {
			renditionsRows.Close()
			return domain.Recording{}, false, fmt.Errorf("scan recording rendition: %w", err)
		}
		entry := domain.RecordingRendition{Name: name, ManifestURL: url}
		if bitrate.Valid {
			entry.Bitrate = int(bitrate.Int32)
		}
		renditions = append(renditions, entry)
	}
	renditionsRows.Close()
	if err := renditionsRows.Err(); err != nil {
		return domain.Recording{}, false, fmt.Errorf("read recording renditions: %w", err)
	}
	recording.Renditions = renditions

	thumbRows, err := r.pool.Query(ctx, "SELECT id, url, width, height, created_at FROM recording_thumbnails WHERE recording_id = $1", id)
	if err != nil {
		return domain.Recording{}, false, fmt.Errorf("load recording thumbnails: %w", err)
	}
	thumbnails := make([]domain.RecordingThumbnail, 0)
	for thumbRows.Next() {
		var thumb domain.RecordingThumbnail
		thumb.RecordingID = id
		if err := thumbRows.Scan(&thumb.ID, &thumb.URL, &thumb.Width, &thumb.Height, &thumb.CreatedAt); err != nil {
			thumbRows.Close()
			return domain.Recording{}, false, fmt.Errorf("scan recording thumbnail: %w", err)
		}
		thumbnails = append(thumbnails, thumb)
	}
	thumbRows.Close()
	if err := thumbRows.Err(); err != nil {
		return domain.Recording{}, false, fmt.Errorf("read recording thumbnails: %w", err)
	}
	recording.Thumbnails = thumbnails

	clipRows, err := r.pool.Query(ctx, "SELECT id, title, start_seconds, end_seconds, status FROM clip_exports WHERE recording_id = $1", id)
	if err != nil {
		return domain.Recording{}, false, fmt.Errorf("load clip exports: %w", err)
	}
	clips := make([]domain.ClipExportSummary, 0)
	for clipRows.Next() {
		var clip domain.ClipExportSummary
		if err := clipRows.Scan(&clip.ID, &clip.Title, &clip.StartSeconds, &clip.EndSeconds, &clip.Status); err != nil {
			clipRows.Close()
			return domain.Recording{}, false, fmt.Errorf("scan clip export: %w", err)
		}
		clips = append(clips, clip)
	}
	clipRows.Close()
	if err := clipRows.Err(); err != nil {
		return domain.Recording{}, false, fmt.Errorf("read clip exports: %w", err)
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

// loadUpload executes loadUpload.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns infrastructure/persistence errors as wrapped `error` values; not-found is
// represented by the boolean return when provided by this signature.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) loadUpload(ctx context.Context, id string) (domain.Upload, bool, error) {
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
		return domain.Upload{}, false, nil
	}
	if err != nil {
		return domain.Upload{}, false, err
	}
	metadata := make(map[string]string)
	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			return domain.Upload{}, false, fmt.Errorf("decode upload metadata: %w", err)
		}
	}
	upload := domain.Upload{
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

// CreateChannel executes CreateChannel.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
