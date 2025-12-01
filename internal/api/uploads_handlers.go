package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bitriver-live/internal/models"
	"bitriver-live/internal/storage"
)

type uploadResponse struct {
	ID          string            `json:"id"`
	ChannelID   string            `json:"channelId"`
	Title       string            `json:"title"`
	Filename    string            `json:"filename"`
	SizeBytes   int64             `json:"sizeBytes"`
	Status      string            `json:"status"`
	Progress    int               `json:"progress"`
	RecordingID *string           `json:"recordingId,omitempty"`
	PlaybackURL string            `json:"playbackUrl,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   string            `json:"createdAt"`
	UpdatedAt   string            `json:"updatedAt"`
	CompletedAt *string           `json:"completedAt,omitempty"`
}

type uploadedMedia struct {
	tempPath     string
	size         int64
	originalName string
	contentType  string
}

type createUploadRequest struct {
	ChannelID   string            `json:"channelId"`
	Title       string            `json:"title"`
	Filename    string            `json:"filename"`
	SizeBytes   int64             `json:"sizeBytes"`
	PlaybackURL string            `json:"playbackUrl"`
	Metadata    map[string]string `json:"metadata"`
}

func newUploadResponse(upload models.Upload) uploadResponse {
	resp := uploadResponse{
		ID:        upload.ID,
		ChannelID: upload.ChannelID,
		Title:     upload.Title,
		Filename:  upload.Filename,
		SizeBytes: upload.SizeBytes,
		Status:    upload.Status,
		Progress:  upload.Progress,
		Metadata:  nil,
		Error:     upload.Error,
		CreatedAt: upload.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt: upload.UpdatedAt.Format(time.RFC3339Nano),
	}
	if upload.Metadata != nil {
		meta := make(map[string]string, len(upload.Metadata))
		for k, v := range upload.Metadata {
			meta[k] = v
		}
		resp.Metadata = meta
	}
	if upload.RecordingID != nil {
		id := *upload.RecordingID
		resp.RecordingID = &id
	}
	if upload.PlaybackURL != "" {
		resp.PlaybackURL = upload.PlaybackURL
	}
	if upload.CompletedAt != nil {
		completed := upload.CompletedAt.Format(time.RFC3339Nano)
		resp.CompletedAt = &completed
	}
	if strings.TrimSpace(resp.Error) == "" {
		resp.Error = ""
	}
	return resp
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (h *Handler) Uploads(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		actor, ok := h.requireAuthenticatedUser(w, r)
		if !ok {
			return
		}
		channelID := strings.TrimSpace(r.URL.Query().Get("channelId"))
		if channelID == "" {
			WriteError(w, http.StatusBadRequest, fmt.Errorf("channelId is required"))
			return
		}
		channel, exists := h.Store.GetChannel(channelID)
		if !exists {
			WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
			return
		}
		if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		uploads, err := h.Store.ListUploads(channelID)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		response := make([]uploadResponse, 0, len(uploads))
		for _, upload := range uploads {
			response = append(response, newUploadResponse(upload))
		}
		WriteJSON(w, http.StatusOK, response)
	case http.MethodPost:
		actor, ok := h.requireAuthenticatedUser(w, r)
		if !ok {
			return
		}
		contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
		if strings.HasPrefix(contentType, "multipart/form-data") {
			h.createUploadFromMultipart(w, r, actor)
			return
		}
		h.createUploadFromJSON(w, r, actor)
	default:
		w.Header().Set("Allow", "GET, POST")
		WriteError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}

func (h *Handler) UploadByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/uploads/")
	if path == "" {
		WriteError(w, http.StatusNotFound, fmt.Errorf("upload id missing"))
		return
	}
	parts := strings.Split(path, "/")
	uploadID := strings.TrimSpace(parts[0])
	upload, ok := h.Store.GetUpload(uploadID)
	if !ok {
		WriteError(w, http.StatusNotFound, fmt.Errorf("upload %s not found", uploadID))
		return
	}
	channel, exists := h.Store.GetChannel(upload.ChannelID)
	if !exists {
		WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", upload.ChannelID))
		return
	}
	if len(parts) > 1 && strings.TrimSpace(parts[1]) == "media" {
		h.serveUploadMedia(w, r, upload)
		return
	}
	actor, hasActor := UserFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		if !hasActor {
			WriteError(w, http.StatusUnauthorized, fmt.Errorf("authentication required"))
			return
		}
		if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		WriteJSON(w, http.StatusOK, newUploadResponse(upload))
	case http.MethodDelete:
		if !hasActor {
			WriteError(w, http.StatusUnauthorized, fmt.Errorf("authentication required"))
			return
		}
		if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		if err := h.Store.DeleteUpload(uploadID); err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		h.deleteUploadMedia(upload)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, DELETE")
		WriteError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}

func (h *Handler) createUploadFromJSON(w http.ResponseWriter, r *http.Request, actor models.User) {
	var req createUploadRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteDecodeError(w, err)
		return
	}
	upload, status, err := h.createUploadEntry(r, actor, req, nil)
	if err != nil {
		WriteError(w, status, err)
		return
	}
	WriteJSON(w, http.StatusCreated, newUploadResponse(upload))
}

func (h *Handler) createUploadFromMultipart(w http.ResponseWriter, r *http.Request, actor models.User) {
	reader, err := r.MultipartReader()
	if err != nil {
		WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid multipart payload"))
		return
	}
	req := createUploadRequest{}
	metadata := make(map[string]string)
	var media *uploadedMedia
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			WriteError(w, http.StatusBadRequest, fmt.Errorf("read multipart data: %w", err))
			return
		}
		name := part.FormName()
		if name == "" {
			_ = part.Close()
			continue
		}
		if name == "file" {
			if media != nil {
				_ = part.Close()
				continue
			}
			saved, saveErr := h.saveMultipartFile(part)
			if saveErr != nil {
				WriteError(w, http.StatusBadRequest, saveErr)
				return
			}
			media = saved
			continue
		}
		payload, readErr := io.ReadAll(part)
		_ = part.Close()
		if readErr != nil {
			WriteError(w, http.StatusBadRequest, fmt.Errorf("read form field: %w", readErr))
			return
		}
		value := strings.TrimSpace(string(payload))
		switch name {
		case "channelId":
			req.ChannelID = value
		case "title":
			req.Title = value
		case "filename":
			req.Filename = value
		case "playbackUrl":
			req.PlaybackURL = value
		case "sizeBytes":
			if value != "" {
				if size, parseErr := strconv.ParseInt(value, 10, 64); parseErr == nil {
					req.SizeBytes = size
				}
			}
		default:
			if strings.HasPrefix(name, "metadata[") && strings.HasSuffix(name, "]") {
				key := strings.TrimSpace(name[len("metadata[") : len(name)-1])
				if key != "" && value != "" {
					metadata[key] = value
				}
			}
		}
	}
	if len(metadata) > 0 {
		req.Metadata = metadata
	}
	if media != nil {
		if strings.TrimSpace(req.Filename) == "" {
			req.Filename = media.originalName
		}
		if strings.TrimSpace(req.Title) == "" {
			name := media.originalName
			if ext := filepath.Ext(name); ext != "" {
				name = strings.TrimSuffix(name, ext)
			}
			req.Title = name
		}
		if media.size > 0 {
			req.SizeBytes = media.size
		}
	}
	upload, status, err := h.createUploadEntry(r, actor, req, media)
	if err != nil {
		WriteError(w, status, err)
		return
	}
	WriteJSON(w, http.StatusCreated, newUploadResponse(upload))
}

func (h *Handler) createUploadEntry(r *http.Request, actor models.User, req createUploadRequest, media *uploadedMedia) (models.Upload, int, error) {
	channelID := strings.TrimSpace(req.ChannelID)
	if channelID == "" {
		return models.Upload{}, http.StatusBadRequest, fmt.Errorf("channelId is required")
	}
	channel, exists := h.Store.GetChannel(channelID)
	if !exists {
		return models.Upload{}, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID)
	}
	if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
		return models.Upload{}, http.StatusForbidden, fmt.Errorf("forbidden")
	}
	metadata := cloneStringMap(req.Metadata)
	playbackURL := strings.TrimSpace(req.PlaybackURL)
	if playbackURL != "" {
		if metadata == nil {
			metadata = make(map[string]string, 1)
		}
		metadata["sourceUrl"] = playbackURL
	}
	sizeBytes := req.SizeBytes
	if media != nil && media.size > 0 {
		sizeBytes = media.size
	}
	params := storage.CreateUploadParams{
		ChannelID:   channelID,
		Title:       req.Title,
		Filename:    req.Filename,
		SizeBytes:   sizeBytes,
		Metadata:    metadata,
		PlaybackURL: playbackURL,
	}
	upload, err := h.Store.CreateUpload(params)
	if err != nil {
		return models.Upload{}, http.StatusBadRequest, err
	}
	if media != nil {
		updated, attachErr := h.attachMediaToUpload(r, upload, metadata, media)
		if attachErr != nil {
			return models.Upload{}, http.StatusInternalServerError, attachErr
		}
		upload = updated
	}
	if h.UploadProcessor != nil {
		h.UploadProcessor.Enqueue(upload.ID)
	}
	return upload, 0, nil
}

func (h *Handler) saveMultipartFile(part *multipart.Part) (*uploadedMedia, error) {
	defer part.Close()
	dir := h.uploadMediaDir()
	tmp, err := os.CreateTemp(dir, "pending-upload-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	defer tmp.Close()
	written, err := io.Copy(tmp, part)
	if err != nil {
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("save upload: %w", err)
	}
	return &uploadedMedia{
		tempPath:     tmp.Name(),
		size:         written,
		originalName: part.FileName(),
		contentType:  part.Header.Get("Content-Type"),
	}, nil
}

func (h *Handler) attachMediaToUpload(r *http.Request, upload models.Upload, baseMetadata map[string]string, media *uploadedMedia) (models.Upload, error) {
	if media == nil {
		return upload, nil
	}
	storedName, err := h.persistUploadMedia(upload.ID, media)
	if err != nil {
		_ = h.Store.DeleteUpload(upload.ID)
		return models.Upload{}, err
	}
	metadata := cloneStringMap(baseMetadata)
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["source"] = "upload"
	metadata["mediaPath"] = storedName
	if media.originalName != "" {
		metadata["uploadedFilename"] = media.originalName
	}
	if media.contentType != "" {
		metadata["contentType"] = media.contentType
	}
	token := generateUploadMediaToken()
	metadata["mediaToken"] = token
	metadata["sourceUrl"] = h.uploadMediaURL(r, upload.ID, token)
	update := storage.UploadUpdate{Metadata: metadata}
	if _, err := h.Store.UpdateUpload(upload.ID, update); err != nil {
		_ = os.Remove(filepath.Join(h.uploadMediaDir(), storedName))
		_ = h.Store.DeleteUpload(upload.ID)
		return models.Upload{}, err
	}
	upload.Metadata = metadata
	return upload, nil
}

func (h *Handler) persistUploadMedia(uploadID string, media *uploadedMedia) (string, error) {
	if media == nil || media.tempPath == "" {
		return "", fmt.Errorf("media payload missing")
	}
	defer func() {
		if media.tempPath != "" {
			_ = os.Remove(media.tempPath)
		}
	}()
	dir := h.uploadMediaDir()
	ext := strings.ToLower(filepath.Ext(media.originalName))
	if ext == "" {
		ext = ".bin"
	}
	storedName := fmt.Sprintf("%s%s", uploadID, ext)
	finalPath := filepath.Join(dir, storedName)
	_ = os.Remove(finalPath)
	if err := os.Rename(media.tempPath, finalPath); err != nil {
		return "", fmt.Errorf("store upload media: %w", err)
	}
	media.tempPath = ""
	return storedName, nil
}

func (h *Handler) serveUploadMedia(w http.ResponseWriter, r *http.Request, upload models.Upload) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		WriteError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}
	if upload.Metadata == nil {
		WriteError(w, http.StatusNotFound, fmt.Errorf("media not found"))
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	expected := strings.TrimSpace(upload.Metadata["mediaToken"])
	if token == "" || expected == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
		WriteError(w, http.StatusForbidden, fmt.Errorf("invalid token"))
		return
	}
	mediaPath := strings.TrimSpace(upload.Metadata["mediaPath"])
	if mediaPath == "" {
		WriteError(w, http.StatusNotFound, fmt.Errorf("media not found"))
		return
	}
	fullPath := filepath.Join(h.uploadMediaDir(), filepath.Base(mediaPath))
	file, err := os.Open(fullPath)
	if err != nil {
		WriteError(w, http.StatusNotFound, fmt.Errorf("media unavailable"))
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, fmt.Errorf("media stat failed"))
		return
	}
	contentType := strings.TrimSpace(upload.Metadata["contentType"])
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	http.ServeContent(w, r, upload.Metadata["uploadedFilename"], stat.ModTime(), file)
}

func (h *Handler) deleteUploadMedia(upload models.Upload) {
	if upload.Metadata == nil {
		return
	}
	mediaPath := strings.TrimSpace(upload.Metadata["mediaPath"])
	if mediaPath == "" {
		return
	}
	fullPath := filepath.Join(h.uploadMediaDir(), filepath.Base(mediaPath))
	_ = os.Remove(fullPath)
}

func (h *Handler) uploadMediaDir() string {
	h.uploadDirOnce.Do(func() {
		dir := strings.TrimSpace(h.UploadMediaDir)
		if dir == "" {
			dir = filepath.Join(os.TempDir(), "bitriver-uploads")
		}
		dir = filepath.Clean(dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			dir = filepath.Join(os.TempDir(), "bitriver-uploads")
			_ = os.MkdirAll(dir, 0o755)
		}
		h.uploadDir = dir
	})
	if h.uploadDir == "" {
		return filepath.Join(os.TempDir(), "bitriver-uploads")
	}
	return h.uploadDir
}

func (h *Handler) uploadMediaURL(r *http.Request, uploadID, token string) string {
	if r == nil {
		return ""
	}
	scheme := requestScheme(r)
	base := r.Host
	if base == "" && r.URL != nil {
		base = r.URL.Host
	}
	if base == "" {
		base = "localhost"
	}
	mediaURL := url.URL{
		Scheme: scheme,
		Host:   base,
		Path:   fmt.Sprintf("/api/uploads/%s/media", uploadID),
	}
	if token != "" {
		q := mediaURL.Query()
		q.Set("token", token)
		mediaURL.RawQuery = q.Encode()
	}
	return mediaURL.String()
}

func requestScheme(r *http.Request) string {
	if r == nil {
		return "http"
	}
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		parts := strings.Split(proto, ",")
		return strings.TrimSpace(parts[0])
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func generateUploadMediaToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
