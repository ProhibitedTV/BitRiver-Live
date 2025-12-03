package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"bitriver-live/internal/models"
)

func cloneRecording(recording models.Recording) models.Recording {
	cloned := recording
	if recording.Renditions != nil {
		cloned.Renditions = append([]models.RecordingRendition(nil), recording.Renditions...)
	}
	if recording.Thumbnails != nil {
		cloned.Thumbnails = append([]models.RecordingThumbnail(nil), recording.Thumbnails...)
	}
	if recording.Metadata != nil {
		meta := make(map[string]string, len(recording.Metadata))
		for k, v := range recording.Metadata {
			meta[k] = v
		}
		cloned.Metadata = meta
	}
	if recording.PublishedAt != nil {
		published := *recording.PublishedAt
		cloned.PublishedAt = &published
	}
	if recording.RetainUntil != nil {
		retain := *recording.RetainUntil
		cloned.RetainUntil = &retain
	}
	if recording.Clips != nil {
		cloned.Clips = append([]models.ClipExportSummary(nil), recording.Clips...)
	}
	return cloned
}

func cloneUpload(upload models.Upload) models.Upload {
	cloned := upload
	if upload.Metadata != nil {
		meta := make(map[string]string, len(upload.Metadata))
		for k, v := range upload.Metadata {
			meta[k] = v
		}
		cloned.Metadata = meta
	}
	if upload.RecordingID != nil {
		recording := *upload.RecordingID
		cloned.RecordingID = &recording
	}
	if upload.CompletedAt != nil {
		completed := *upload.CompletedAt
		cloned.CompletedAt = &completed
	}
	return cloned
}

func cloneClipExport(clip models.ClipExport) models.ClipExport {
	cloned := clip
	if clip.CompletedAt != nil {
		completed := *clip.CompletedAt
		cloned.CompletedAt = &completed
	}
	return cloned
}

func (s *Storage) recordingDeadline(now time.Time, published bool) *time.Time {
	var window time.Duration
	if published {
		window = s.recordingRetention.Published
	} else {
		window = s.recordingRetention.Unpublished
	}
	if window <= 0 {
		return nil
	}
	deadline := now.Add(window)
	return &deadline
}

func (s *Storage) purgeExpiredRecordingsLocked(now time.Time) (bool, dataset, error) {
	if len(s.data.Recordings) == 0 {
		return false, dataset{}, nil
	}
	removed := false
	snapshotTaken := false
	var snapshot dataset
	for id, recording := range s.data.Recordings {
		if recording.RetainUntil == nil || now.Before(*recording.RetainUntil) {
			continue
		}
		if !snapshotTaken {
			snapshot = cloneDataset(s.data)
			snapshotTaken = true
		}
		if err := s.deleteRecordingArtifactsLocked(recording); err != nil {
			if snapshotTaken {
				s.data = snapshot
			}
			return false, dataset{}, err
		}
		for clipID, clip := range s.data.ClipExports {
			if clip.RecordingID != id {
				continue
			}
			if err := s.deleteClipArtifactsLocked(clip); err != nil {
				if snapshotTaken {
					s.data = snapshot
				}
				return false, dataset{}, err
			}
			delete(s.data.ClipExports, clipID)
		}
		delete(s.data.Recordings, id)
		removed = true
	}
	if !removed {
		return false, dataset{}, nil
	}
	return true, snapshot, nil
}

func (s *Storage) recordingWithClipsLocked(recording models.Recording) models.Recording {
	cloned := cloneRecording(recording)
	if len(s.data.ClipExports) == 0 {
		return cloned
	}
	var clips []models.ClipExportSummary
	for _, clip := range s.data.ClipExports {
		if clip.RecordingID != recording.ID {
			continue
		}
		clips = append(clips, models.ClipExportSummary{
			ID:           clip.ID,
			Title:        clip.Title,
			StartSeconds: clip.StartSeconds,
			EndSeconds:   clip.EndSeconds,
			Status:       clip.Status,
		})
	}
	if len(clips) == 0 {
		return cloned
	}
	sort.Slice(clips, func(i, j int) bool {
		if clips[i].StartSeconds == clips[j].StartSeconds {
			return clips[i].ID < clips[j].ID
		}
		return clips[i].StartSeconds < clips[j].StartSeconds
	})
	cloned.Clips = clips
	return cloned
}

func (s *Storage) createRecordingLocked(session models.StreamSession, channel models.Channel, ended time.Time) (models.Recording, error) {
	s.ensureDatasetInitializedLocked()
	id, err := generateID()
	if err != nil {
		return models.Recording{}, err
	}
	duration := int(ended.Sub(session.StartedAt).Round(time.Second).Seconds())
	if duration < 0 {
		duration = 0
	}
	title := channel.Title
	if title == "" {
		title = fmt.Sprintf("Recording %s", session.ID)
	}
	title = strings.TrimSpace(title)
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
		ID:              id,
		ChannelID:       channel.ID,
		SessionID:       session.ID,
		Title:           title,
		DurationSeconds: duration,
		PlaybackBaseURL: session.PlaybackURL,
		Metadata:        metadata,
		CreatedAt:       ended,
	}
	if deadline := s.recordingDeadline(ended, false); deadline != nil {
		recording.RetainUntil = deadline
	}
	if len(session.RenditionManifests) > 0 {
		renditions := make([]models.RecordingRendition, 0, len(session.RenditionManifests))
		for _, manifest := range session.RenditionManifests {
			renditions = append(renditions, models.RecordingRendition(manifest))
		}
		recording.Renditions = renditions
	}
	if err := s.populateRecordingArtifactsLocked(&recording, session); err != nil {
		return models.Recording{}, err
	}
	return recording, nil
}

func (s *Storage) populateRecordingArtifactsLocked(recording *models.Recording, session models.StreamSession) error {
	client := s.objectClient
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
			ctx, cancel := context.WithTimeout(context.Background(), s.objectStorage.requestTimeout())
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
	ctx, cancel := context.WithTimeout(context.Background(), s.objectStorage.requestTimeout())
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

func (s *Storage) deleteRecordingArtifactsLocked(recording models.Recording) error {
	client := s.objectClient
	if client == nil || !client.Enabled() {
		return nil
	}
	if len(recording.Metadata) == 0 {
		return nil
	}
	deleted := make(map[string]struct{})
	for metaKey, objectKey := range recording.Metadata {
		if !strings.HasPrefix(metaKey, metadataManifestPrefix) && !strings.HasPrefix(metaKey, metadataThumbnailPrefix) {
			continue
		}
		trimmed := strings.TrimSpace(objectKey)
		if trimmed == "" {
			continue
		}
		if _, exists := deleted[trimmed]; exists {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), s.objectStorage.requestTimeout())
		err := client.Delete(ctx, trimmed)
		cancel()
		if err != nil {
			return fmt.Errorf("delete object %s: %w", trimmed, err)
		}
		deleted[trimmed] = struct{}{}
	}
	return nil
}

func (s *Storage) deleteClipArtifactsLocked(clip models.ClipExport) error {
	client := s.objectClient
	if client == nil || !client.Enabled() || strings.TrimSpace(clip.StorageObject) == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.objectStorage.requestTimeout())
	err := client.Delete(ctx, clip.StorageObject)
	cancel()
	if err != nil {
		return fmt.Errorf("delete clip object %s: %w", clip.StorageObject, err)
	}
	return nil
}

func (s *Storage) ListRecordings(channelID string, includeUnpublished bool) ([]models.Recording, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	now := time.Now().UTC()
	removed, snapshot, err := s.purgeExpiredRecordingsLocked(now)
	if err != nil {
		return nil, err
	}
	if removed {
		if err := s.persist(); err != nil {
			s.data = snapshot
			return nil, err
		}
	}

	recordings := make([]models.Recording, 0)
	for _, recording := range s.data.Recordings {
		if recording.ChannelID != channelID {
			continue
		}
		if !includeUnpublished && recording.PublishedAt == nil {
			continue
		}
		recordings = append(recordings, s.recordingWithClipsLocked(recording))
	}
	sort.Slice(recordings, func(i, j int) bool {
		return recordings[i].CreatedAt.After(recordings[j].CreatedAt)
	})
	return recordings, nil
}

func (s *Storage) CreateUpload(params CreateUploadParams) (models.Upload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureDatasetInitializedLocked()

	channelID := strings.TrimSpace(params.ChannelID)
	channel, ok := s.data.Channels[channelID]
	if !ok {
		return models.Upload{}, fmt.Errorf("channel %s not found", channelID)
	}

	title := strings.TrimSpace(params.Title)
	if title == "" {
		if channel.Title != "" {
			title = fmt.Sprintf("%s upload", channel.Title)
		} else {
			title = "Uploaded video"
		}
	}

	filename := strings.TrimSpace(params.Filename)
	if filename == "" {
		filename = fmt.Sprintf("upload-%s.mp4", time.Now().UTC().Format("20060102-150405"))
	}

	id, err := generateID()
	if err != nil {
		return models.Upload{}, err
	}

	now := time.Now().UTC()
	metadata := make(map[string]string, len(params.Metadata))
	for k, v := range params.Metadata {
		if strings.TrimSpace(k) == "" {
			continue
		}
		metadata[k] = v
	}

	playbackURL := strings.TrimSpace(params.PlaybackURL)

	upload := models.Upload{
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

	s.data.Uploads[id] = upload
	if err := s.persist(); err != nil {
		delete(s.data.Uploads, id)
		return models.Upload{}, err
	}

	return upload, nil
}

func (s *Storage) ListUploads(channelID string) ([]models.Upload, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.data.Channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	uploads := make([]models.Upload, 0)
	for _, upload := range s.data.Uploads {
		if upload.ChannelID != channelID {
			continue
		}
		uploads = append(uploads, cloneUpload(upload))
	}
	sort.Slice(uploads, func(i, j int) bool {
		return uploads[i].CreatedAt.After(uploads[j].CreatedAt)
	})
	return uploads, nil
}

func (s *Storage) GetUpload(id string) (models.Upload, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	upload, ok := s.data.Uploads[id]
	if !ok {
		return models.Upload{}, false
	}
	return cloneUpload(upload), true
}

func (s *Storage) UpdateUpload(id string, update UploadUpdate) (models.Upload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	upload, ok := s.data.Uploads[id]
	if !ok {
		return models.Upload{}, fmt.Errorf("upload %s not found", id)
	}

	original := upload

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
			completed := update.CompletedAt.UTC()
			upload.CompletedAt = &completed
		}
	}

	upload.UpdatedAt = time.Now().UTC()

	s.data.Uploads[id] = upload
	if err := s.persist(); err != nil {
		s.data.Uploads[id] = original
		return models.Upload{}, err
	}
	return cloneUpload(upload), nil
}

func (s *Storage) DeleteUpload(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	upload, ok := s.data.Uploads[id]
	if !ok {
		return fmt.Errorf("upload %s not found", id)
	}

	delete(s.data.Uploads, id)
	if err := s.persist(); err != nil {
		s.data.Uploads[id] = upload
		return err
	}
	return nil
}

func (s *Storage) GetRecording(id string) (models.Recording, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		return models.Recording{}, false
	}
	now := time.Now().UTC()
	removed, snapshot, err := s.purgeExpiredRecordingsLocked(now)
	if err != nil {
		return models.Recording{}, false
	}
	if removed {
		if err := s.persist(); err != nil {
			s.data = snapshot
			return models.Recording{}, false
		}
	}
	recording, ok := s.data.Recordings[id]
	if !ok {
		return models.Recording{}, false
	}
	return s.recordingWithClipsLocked(recording), true
}

func (s *Storage) PublishRecording(id string) (models.Recording, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		return models.Recording{}, fmt.Errorf("recording id is required")
	}

	recording, ok := s.data.Recordings[id]
	if !ok {
		return models.Recording{}, fmt.Errorf("recording %s not found", id)
	}
	if recording.PublishedAt != nil {
		return s.recordingWithClipsLocked(recording), nil
	}

	now := time.Now().UTC()
	updated := cloneRecording(recording)
	updated.PublishedAt = &now
	if deadline := s.recordingDeadline(now, true); deadline != nil {
		updated.RetainUntil = deadline
	} else {
		updated.RetainUntil = nil
	}

	snapshot := cloneDataset(s.data)
	s.data.Recordings[id] = updated
	if err := s.persist(); err != nil {
		s.data = snapshot
		return models.Recording{}, err
	}
	return s.recordingWithClipsLocked(updated), nil
}

func (s *Storage) DeleteRecording(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		return fmt.Errorf("recording id is required")
	}
	recording, ok := s.data.Recordings[id]
	if !ok {
		return fmt.Errorf("recording %s not found", id)
	}
	if err := s.deleteRecordingArtifactsLocked(recording); err != nil {
		return err
	}
	snapshot := cloneDataset(s.data)
	for clipID, clip := range s.data.ClipExports {
		if clip.RecordingID != id {
			continue
		}
		if err := s.deleteClipArtifactsLocked(clip); err != nil {
			s.data = snapshot
			return err
		}
		delete(s.data.ClipExports, clipID)
	}
	delete(s.data.Recordings, id)
	if err := s.persist(); err != nil {
		s.data = snapshot
		return err
	}
	return nil
}

func (s *Storage) CreateClipExport(recordingID string, params ClipExportParams) (models.ClipExport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if recordingID == "" {
		return models.ClipExport{}, fmt.Errorf("recording id is required")
	}
	recording, ok := s.data.Recordings[recordingID]
	if !ok {
		return models.ClipExport{}, fmt.Errorf("recording %s not found", recordingID)
	}
	if params.EndSeconds <= params.StartSeconds {
		return models.ClipExport{}, fmt.Errorf("endSeconds must be greater than startSeconds")
	}
	if params.StartSeconds < 0 {
		return models.ClipExport{}, fmt.Errorf("startSeconds must be non-negative")
	}
	if recording.DurationSeconds > 0 && params.EndSeconds > recording.DurationSeconds {
		return models.ClipExport{}, fmt.Errorf("clip exceeds recording duration")
	}
	id, err := generateID()
	if err != nil {
		return models.ClipExport{}, err
	}
	now := time.Now().UTC()
	clip := models.ClipExport{
		ID:           id,
		RecordingID:  recordingID,
		ChannelID:    recording.ChannelID,
		SessionID:    recording.SessionID,
		Title:        strings.TrimSpace(params.Title),
		StartSeconds: params.StartSeconds,
		EndSeconds:   params.EndSeconds,
		Status:       "pending",
		CreatedAt:    now,
	}
	snapshot := cloneDataset(s.data)
	s.data.ClipExports[id] = clip
	if err := s.persist(); err != nil {
		s.data = snapshot
		return models.ClipExport{}, err
	}
	return clip, nil
}

func (s *Storage) ListClipExports(recordingID string) ([]models.ClipExport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if recordingID == "" {
		return nil, fmt.Errorf("recording id is required")
	}
	if _, ok := s.data.Recordings[recordingID]; !ok {
		return nil, fmt.Errorf("recording %s not found", recordingID)
	}
	clips := make([]models.ClipExport, 0)
	for _, clip := range s.data.ClipExports {
		if clip.RecordingID != recordingID {
			continue
		}
		clips = append(clips, cloneClipExport(clip))
	}
	sort.Slice(clips, func(i, j int) bool {
		return clips[i].CreatedAt.After(clips[j].CreatedAt)
	})
	return clips, nil
}
