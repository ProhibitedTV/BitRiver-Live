package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"bitriver-live/internal/models"
	"bitriver-live/internal/storage"
)

type clipExportRequest struct {
	Title        string `json:"title"`
	StartSeconds int    `json:"startSeconds"`
	EndSeconds   int    `json:"endSeconds"`
}

type recordingResponse struct {
	ID              string                       `json:"id"`
	ChannelID       string                       `json:"channelId"`
	SessionID       string                       `json:"sessionId"`
	Title           string                       `json:"title"`
	DurationSeconds int                          `json:"durationSeconds"`
	PlaybackBaseURL string                       `json:"playbackBaseUrl,omitempty"`
	Renditions      []recordingRenditionResponse `json:"renditions,omitempty"`
	Thumbnails      []recordingThumbnailResponse `json:"thumbnails,omitempty"`
	Metadata        map[string]string            `json:"metadata,omitempty"`
	PublishedAt     *string                      `json:"publishedAt,omitempty"`
	CreatedAt       string                       `json:"createdAt"`
	RetainUntil     *string                      `json:"retainUntil,omitempty"`
	Clips           []clipExportSummaryResponse  `json:"clips,omitempty"`
}

type recordingRenditionResponse struct {
	Name        string `json:"name"`
	ManifestURL string `json:"manifestUrl"`
	Bitrate     int    `json:"bitrate,omitempty"`
}

type recordingThumbnailResponse struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	CreatedAt string `json:"createdAt"`
}

type clipExportSummaryResponse struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	StartSeconds int    `json:"startSeconds"`
	EndSeconds   int    `json:"endSeconds"`
	Status       string `json:"status"`
}

type clipExportResponse struct {
	ID           string  `json:"id"`
	RecordingID  string  `json:"recordingId"`
	ChannelID    string  `json:"channelId"`
	SessionID    string  `json:"sessionId"`
	Title        string  `json:"title"`
	StartSeconds int     `json:"startSeconds"`
	EndSeconds   int     `json:"endSeconds"`
	Status       string  `json:"status"`
	PlaybackURL  string  `json:"playbackUrl,omitempty"`
	CreatedAt    string  `json:"createdAt"`
	CompletedAt  *string `json:"completedAt,omitempty"`
}

func newVodItemResponse(recording models.Recording) vodItemResponse {
	item := vodItemResponse{
		ID:              recording.ID,
		Title:           recording.Title,
		DurationSeconds: recording.DurationSeconds,
	}
	if recording.PublishedAt != nil {
		item.PublishedAt = recording.PublishedAt.Format(time.RFC3339Nano)
	}
	if len(recording.Thumbnails) > 0 {
		thumb := recording.Thumbnails[0]
		if thumb.URL != "" {
			item.ThumbnailURL = thumb.URL
		}
	}
	if len(recording.Renditions) > 0 {
		rendition := recording.Renditions[0]
		if rendition.ManifestURL != "" {
			item.PlaybackURL = rendition.ManifestURL
		}
	}
	if item.PlaybackURL == "" && recording.PlaybackBaseURL != "" {
		item.PlaybackURL = recording.PlaybackBaseURL
	}
	return item
}

func newRecordingResponse(recording models.Recording) recordingResponse {
	resp := recordingResponse{
		ID:              recording.ID,
		ChannelID:       recording.ChannelID,
		SessionID:       recording.SessionID,
		Title:           recording.Title,
		DurationSeconds: recording.DurationSeconds,
		CreatedAt:       recording.CreatedAt.Format(time.RFC3339Nano),
	}
	if recording.PlaybackBaseURL != "" {
		resp.PlaybackBaseURL = recording.PlaybackBaseURL
	}
	if recording.Metadata != nil {
		meta := make(map[string]string, len(recording.Metadata))
		for k, v := range recording.Metadata {
			meta[k] = v
		}
		resp.Metadata = meta
	}
	if recording.PublishedAt != nil {
		published := recording.PublishedAt.Format(time.RFC3339Nano)
		resp.PublishedAt = &published
	}
	if recording.RetainUntil != nil {
		retain := recording.RetainUntil.Format(time.RFC3339Nano)
		resp.RetainUntil = &retain
	}
	if len(recording.Renditions) > 0 {
		manifests := make([]recordingRenditionResponse, 0, len(recording.Renditions))
		for _, rendition := range recording.Renditions {
			manifests = append(manifests, recordingRenditionResponse{
				Name:        rendition.Name,
				ManifestURL: rendition.ManifestURL,
				Bitrate:     rendition.Bitrate,
			})
		}
		resp.Renditions = manifests
	}
	if len(recording.Thumbnails) > 0 {
		thumbs := make([]recordingThumbnailResponse, 0, len(recording.Thumbnails))
		for _, thumb := range recording.Thumbnails {
			thumbs = append(thumbs, recordingThumbnailResponse{
				ID:        thumb.ID,
				URL:       thumb.URL,
				Width:     thumb.Width,
				Height:    thumb.Height,
				CreatedAt: thumb.CreatedAt.Format(time.RFC3339Nano),
			})
		}
		resp.Thumbnails = thumbs
	}
	if len(recording.Clips) > 0 {
		clips := make([]clipExportSummaryResponse, 0, len(recording.Clips))
		for _, clip := range recording.Clips {
			clips = append(clips, clipExportSummaryResponse{
				ID:           clip.ID,
				Title:        clip.Title,
				StartSeconds: clip.StartSeconds,
				EndSeconds:   clip.EndSeconds,
				Status:       clip.Status,
			})
		}
		resp.Clips = clips
	}
	return resp
}

func newClipExportResponse(clip models.ClipExport) clipExportResponse {
	resp := clipExportResponse{
		ID:           clip.ID,
		RecordingID:  clip.RecordingID,
		ChannelID:    clip.ChannelID,
		SessionID:    clip.SessionID,
		Title:        clip.Title,
		StartSeconds: clip.StartSeconds,
		EndSeconds:   clip.EndSeconds,
		Status:       clip.Status,
		CreatedAt:    clip.CreatedAt.Format(time.RFC3339Nano),
	}
	if clip.PlaybackURL != "" {
		resp.PlaybackURL = clip.PlaybackURL
	}
	if clip.CompletedAt != nil {
		completed := clip.CompletedAt.Format(time.RFC3339Nano)
		resp.CompletedAt = &completed
	}
	return resp
}

func (h *Handler) Recordings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		WriteError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	channelID := strings.TrimSpace(r.URL.Query().Get("channelId"))
	if channelID == "" {
		WriteError(w, http.StatusBadRequest, fmt.Errorf("channelId is required"))
		return
	}

	includeUnpublished := false
	if actor, ok := UserFromContext(r.Context()); ok {
		if channel, exists := h.Store.GetChannel(channelID); exists {
			if channel.OwnerID == actor.ID || actor.HasRole(roleAdmin) {
				includeUnpublished = true
			}
		}
	}

	recordings, err := h.Store.ListRecordings(channelID, includeUnpublished)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return
	}
	response := make([]recordingResponse, 0, len(recordings))
	for _, recording := range recordings {
		response = append(response, newRecordingResponse(recording))
	}
	WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) RecordingByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/recordings/")
	if path == "" {
		WriteError(w, http.StatusNotFound, fmt.Errorf("recording id missing"))
		return
	}
	parts := strings.Split(path, "/")
	recordingID := strings.TrimSpace(parts[0])
	remaining := parts[1:]

	recording, ok := h.Store.GetRecording(recordingID)
	if !ok {
		WriteError(w, http.StatusNotFound, fmt.Errorf("recording %s not found", recordingID))
		return
	}
	channel, channelExists := h.Store.GetChannel(recording.ChannelID)
	if !channelExists {
		WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", recording.ChannelID))
		return
	}
	actor, hasActor := UserFromContext(r.Context())

	if len(remaining) > 0 && remaining[0] != "" {
		action := remaining[0]
		switch action {
		case "publish":
			if len(remaining) > 1 {
				WriteError(w, http.StatusNotFound, fmt.Errorf("unknown recording path"))
				return
			}
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", "POST")
				WriteError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
				return
			}
			if !hasActor {
				WriteError(w, http.StatusUnauthorized, fmt.Errorf("authentication required"))
				return
			}
			if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
				WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
				return
			}
			updated, err := h.Store.PublishRecording(recordingID)
			if err != nil {
				WriteError(w, http.StatusBadRequest, err)
				return
			}
			WriteJSON(w, http.StatusOK, newRecordingResponse(updated))
			return
		case "clips":
			if len(remaining) > 1 {
				WriteError(w, http.StatusNotFound, fmt.Errorf("unknown recording path"))
				return
			}
			switch r.Method {
			case http.MethodGet:
				if recording.PublishedAt == nil {
					if !hasActor || (channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin)) {
						WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
						return
					}
				}
				clips, err := h.Store.ListClipExports(recordingID)
				if err != nil {
					WriteError(w, http.StatusBadRequest, err)
					return
				}
				response := make([]clipExportResponse, 0, len(clips))
				for _, clip := range clips {
					response = append(response, newClipExportResponse(clip))
				}
				WriteJSON(w, http.StatusOK, response)
			case http.MethodPost:
				if !hasActor {
					WriteError(w, http.StatusUnauthorized, fmt.Errorf("authentication required"))
					return
				}
				if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
					WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
					return
				}
				var req clipExportRequest
				if err := DecodeJSON(r, &req); err != nil {
					WriteDecodeError(w, err)
					return
				}
				clip, err := h.Store.CreateClipExport(recordingID, storage.ClipExportParams{
					Title:        req.Title,
					StartSeconds: req.StartSeconds,
					EndSeconds:   req.EndSeconds,
				})
				if err != nil {
					WriteError(w, http.StatusBadRequest, err)
					return
				}
				WriteJSON(w, http.StatusCreated, newClipExportResponse(clip))
			default:
				w.Header().Set("Allow", "GET, POST")
				WriteError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
			}
			return
		default:
			WriteError(w, http.StatusNotFound, fmt.Errorf("unknown recording path"))
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		if recording.PublishedAt == nil {
			if !hasActor || (channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin)) {
				WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
				return
			}
		}
		WriteJSON(w, http.StatusOK, newRecordingResponse(recording))
	case http.MethodDelete:
		if !hasActor {
			WriteError(w, http.StatusUnauthorized, fmt.Errorf("authentication required"))
			return
		}
		if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		if err := h.Store.DeleteRecording(recordingID); err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, DELETE")
		WriteError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}
