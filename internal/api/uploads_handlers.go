package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/security/tokenauth"
)

var (
	openUploadMediaFile = os.Open
	statUploadMediaFile = func(file *os.File) (os.FileInfo, error) {
		return file.Stat()
	}
)

var allowedUploadMediaTypes = map[string]map[string]struct{}{
	".mp4": {
		"video/mp4": {},
	},
	".m4v": {
		"video/mp4": {},
	},
	".mov": {
		"video/mp4":       {},
		"video/quicktime": {},
	},
	".webm": {
		"video/webm": {},
	},
}

const defaultUploadMaxBytes int64 = 512 << 20 // 512 MiB

var errUploadTooLarge = errors.New("upload exceeds maximum allowed size")

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

// newUploadResponse builds and returns upload response using the supplied dependencies.
func newUploadResponse(upload domain.Upload) uploadResponse {
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

// cloneStringMap performs clone string map and propagates validation or dependency failures to the caller.
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

// Uploads performs uploads and returns an error when dependent systems reject the operation.
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
		channel, exists := h.uploadsService().GetChannel(channelID)
		if !exists {
			WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
			return
		}
		if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		uploads, err := h.uploadsService().ListUploads(channelID)
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
		WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPost)
	}
}

// UploadByID performs upload by id and returns an error when dependent systems reject the operation.
func (h *Handler) UploadByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/uploads/")
	if path == "" {
		WriteError(w, http.StatusNotFound, fmt.Errorf("upload id missing"))
		return
	}
	parts := strings.Split(path, "/")
	uploadID := strings.TrimSpace(parts[0])
	upload, ok := h.uploadsService().GetUpload(uploadID)
	if !ok {
		WriteError(w, http.StatusNotFound, fmt.Errorf("upload %s not found", uploadID))
		return
	}
	channel, exists := h.uploadsService().GetChannel(upload.ChannelID)
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
		if err := h.uploadsService().DeleteUpload(uploadID); err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		h.deleteUploadMedia(upload)
		w.WriteHeader(http.StatusNoContent)
	default:
		WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodDelete)
	}
}

// createUploadFromJSON creates upload from json and returns an error when validation or persistence fails.
func (h *Handler) createUploadFromJSON(w http.ResponseWriter, r *http.Request, actor domain.User) {
	var req createUploadRequest
	if !DecodeAndValidate(w, r, &req) {
		return
	}
	upload, status, err := h.createUploadEntry(r, actor, req, nil)
	if err != nil {
		WriteError(w, status, err)
		return
	}
	WriteJSON(w, http.StatusCreated, newUploadResponse(upload))
}

// createUploadFromMultipart creates upload from multipart and returns an error when validation or persistence fails.
func (h *Handler) createUploadFromMultipart(w http.ResponseWriter, r *http.Request, actor domain.User) {
	// Current server-side upload flow:
	//  1) Stream the multipart body and persist the first `file` part to a temp file.
	//  2) Create an uploads row (`status=pending`) through uploadsService().
	//  3) Move the temp file into UploadMediaDir and store media path/token/url in metadata.
	//  4) Enqueue the background upload processor, which later asks ingest/transcoder to process sourceUrl.
	// The API is intentionally single-file: additional `file` parts are ignored.
	maxUploadBytes := h.uploadMaxBytes()
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
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
			if uploadTooLarge(err) {
				h.writeUploadTooLarge(w, maxUploadBytes)
				return
			}
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
				if errors.Is(saveErr, errUploadTooLarge) {
					h.writeUploadTooLarge(w, maxUploadBytes)
					return
				}
				WriteError(w, http.StatusBadRequest, saveErr)
				return
			}
			media = saved
			continue
		}
		payload, readErr := io.ReadAll(part)
		_ = part.Close()
		if readErr != nil {
			if uploadTooLarge(readErr) {
				h.writeUploadTooLarge(w, maxUploadBytes)
				return
			}
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

// createUploadEntry creates upload entry and returns an error when validation or persistence fails.
func (h *Handler) createUploadEntry(r *http.Request, actor domain.User, req createUploadRequest, media *uploadedMedia) (domain.Upload, int, error) {
	channelID := strings.TrimSpace(req.ChannelID)
	if channelID == "" {
		return domain.Upload{}, http.StatusBadRequest, fmt.Errorf("channelId is required")
	}
	channel, exists := h.uploadsService().GetChannel(channelID)
	if !exists {
		return domain.Upload{}, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID)
	}
	if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
		return domain.Upload{}, http.StatusForbidden, fmt.Errorf("forbidden")
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
	params := domain.UploadCreateParams{
		ChannelID:   channelID,
		Title:       req.Title,
		Filename:    req.Filename,
		SizeBytes:   sizeBytes,
		Metadata:    metadata,
		PlaybackURL: playbackURL,
	}
	upload, err := h.uploadsService().CreateUpload(params)
	if err != nil {
		return domain.Upload{}, http.StatusBadRequest, err
	}
	if media != nil {
		updated, attachErr := h.attachMediaToUpload(r, upload, metadata, media)
		if attachErr != nil {
			return domain.Upload{}, http.StatusInternalServerError, attachErr
		}
		upload = updated
	}
	if h.UploadProcessor != nil {
		h.UploadProcessor.Enqueue(upload.ID)
	}
	return upload, 0, nil
}

// saveMultipartFile performs save multipart file and propagates validation or dependency failures to the caller.
func (h *Handler) saveMultipartFile(part *multipart.Part) (*uploadedMedia, error) {
	defer func() {
		_ = part.Close()
	}()
	dir := h.uploadMediaDir()
	tmp, err := os.CreateTemp(dir, "pending-upload-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		_ = tmp.Close()
	}()
	maxUploadBytes := h.uploadMaxBytes()
	limited := &io.LimitedReader{R: part, N: maxUploadBytes + 1}
	written, err := io.Copy(tmp, limited)
	if err != nil {
		_ = os.Remove(tmp.Name())
		if uploadTooLarge(err) {
			return nil, errUploadTooLarge
		}
		return nil, fmt.Errorf("save upload: %w", err)
	}
	if written > maxUploadBytes || limited.N == 0 {
		_ = os.Remove(tmp.Name())
		return nil, errUploadTooLarge
	}

	ext := strings.ToLower(filepath.Ext(part.FileName()))
	if _, ok := allowedUploadMediaTypes[ext]; !ok {
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("unsupported media extension %q", ext)
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("rewind upload: %w", err)
	}
	header := make([]byte, 512)
	read, err := io.ReadFull(tmp, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("read upload header: %w", err)
	}
	detectedType := http.DetectContentType(header[:read])
	if _, ok := allowedUploadMediaTypes[ext][detectedType]; !ok {
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("unsupported media type %q for extension %q", detectedType, ext)
	}

	return &uploadedMedia{
		tempPath:     tmp.Name(),
		size:         written,
		originalName: part.FileName(),
		contentType:  detectedType,
	}, nil
}

// uploadMaxBytes resolves the multipart upload size policy in bytes.
func (h *Handler) uploadMaxBytes() int64 {
	if h != nil && h.UploadMaxBytes > 0 {
		return h.UploadMaxBytes
	}
	return defaultUploadMaxBytes
}

// writeUploadTooLarge writes a standard request-too-large response for uploads.
func (h *Handler) writeUploadTooLarge(w http.ResponseWriter, maxUploadBytes int64) {
	WriteError(w, http.StatusRequestEntityTooLarge, RequestError{
		Status:  http.StatusRequestEntityTooLarge,
		CodeVal: "request_too_large",
		Message: fmt.Sprintf("upload size exceeds limit of %d bytes", maxUploadBytes),
		Err:     errUploadTooLarge,
	})
}

// uploadTooLarge reports whether the provided error indicates a max-body-size violation.
func uploadTooLarge(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errUploadTooLarge) {
		return true
	}
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

// attachMediaToUpload performs attach media to upload and propagates validation or dependency failures to the caller.
func (h *Handler) attachMediaToUpload(r *http.Request, upload domain.Upload, baseMetadata map[string]string, media *uploadedMedia) (domain.Upload, error) {
	if media == nil {
		return upload, nil
	}
	stored, err := h.persistUploadMedia(r.Context(), upload.ID, media)
	if err != nil {
		_ = h.uploadsService().DeleteUpload(upload.ID)
		return domain.Upload{}, err
	}
	metadata := cloneStringMap(baseMetadata)
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["source"] = "upload"
	metadata["mediaPath"] = stored.Key
	metadata["sourceObjectKey"] = stored.Key
	if stored.PublicURL != "" {
		metadata["sourceObjectURL"] = stored.PublicURL
	}
	if media.originalName != "" {
		metadata["uploadedFilename"] = media.originalName
	}
	contentType := strings.TrimSpace(media.contentType)
	if contentType == "" {
		contentType = strings.TrimSpace(stored.ContentType)
	}
	if contentType != "" {
		metadata["contentType"] = contentType
	}
	token := generateUploadMediaToken()
	metadata["mediaToken"] = token
	if stored.PublicURL != "" {
		metadata["sourceUrl"] = stored.PublicURL
	} else {
		metadata["sourceUrl"] = h.uploadMediaURL(r, upload.ID, token)
	}
	update := domain.UploadUpdate{Metadata: metadata}
	if _, err := h.uploadsService().UpdateUpload(upload.ID, update); err != nil {
		h.deleteStoredUploadSource(stored.Key)
		_ = h.uploadsService().DeleteUpload(upload.ID)
		return domain.Upload{}, err
	}
	upload.Metadata = metadata
	return upload, nil
}

// persistUploadMedia performs persist upload media and propagates validation or dependency failures to the caller.
func (h *Handler) persistUploadMedia(ctx context.Context, uploadID string, media *uploadedMedia) (storedUploadSource, error) {
	if media == nil || media.tempPath == "" {
		return storedUploadSource{}, fmt.Errorf("media payload missing")
	}
	defer func() {
		if media.tempPath != "" {
			_ = os.Remove(media.tempPath)
		}
	}()
	payload, err := readUploadMediaFile(media.tempPath)
	if err != nil {
		return storedUploadSource{}, fmt.Errorf("read upload media: %w", err)
	}
	if store := h.uploadSourceStore(); store.Enabled() {
		stored, storeErr := store.Store(ctx, uploadID, media.originalName, media.contentType, payload)
		if storeErr != nil {
			return storedUploadSource{}, fmt.Errorf("store upload media: %w", storeErr)
		}
		media.tempPath = ""
		return stored, nil
	}
	dir := h.uploadMediaDir()
	ext := strings.ToLower(filepath.Ext(media.originalName))
	if ext == "" {
		ext = ".bin"
	}
	storedName := fmt.Sprintf("%s%s", uploadID, ext)
	finalPath := filepath.Join(dir, storedName)
	_ = os.Remove(finalPath)
	if err := os.Rename(media.tempPath, finalPath); err != nil {
		return storedUploadSource{}, fmt.Errorf("store upload media: %w", err)
	}
	media.tempPath = ""
	return storedUploadSource{Key: storedName, ContentType: media.contentType}, nil
}

// serveUploadMedia performs serve upload media and propagates validation or dependency failures to the caller.
func (h *Handler) serveUploadMedia(w http.ResponseWriter, r *http.Request, upload domain.Upload) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}
	if upload.Metadata == nil {
		WriteError(w, http.StatusNotFound, fmt.Errorf("media not found"))
		return
	}
	token, ok := tokenauth.QueryToken(r, "token")
	expected := strings.TrimSpace(upload.Metadata["mediaToken"])
	if !ok || !tokenauth.ConstantTimeEqual(expected, token) {
		WriteError(w, http.StatusForbidden, fmt.Errorf("invalid token"))
		return
	}
	key := strings.TrimSpace(upload.Metadata["sourceObjectKey"])
	if key == "" {
		key = strings.TrimSpace(upload.Metadata["mediaPath"])
	}
	if key == "" {
		WriteError(w, http.StatusNotFound, fmt.Errorf("media not found"))
		return
	}
	if store := h.uploadSourceStore(); store.Enabled() {
		obj, err := store.Get(r.Context(), key)
		if err != nil {
			err = fmt.Errorf("get upload %s media (%s): %w", upload.ID, key, err)
			h.logger().Error("serve upload media get", "uploadId", upload.ID, "key", key, "err", err)
			WriteError(w, http.StatusNotFound, RequestError{Err: err, Status: http.StatusNotFound, Message: "media unavailable"})
			return
		}
		contentType := strings.TrimSpace(upload.Metadata["contentType"])
		if contentType == "" {
			contentType = strings.TrimSpace(obj.ContentType)
		}
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "private, max-age=300")
		http.ServeContent(w, r, upload.Metadata["uploadedFilename"], obj.ModTime, bytes.NewReader(obj.Body))
		return
	}
	fullPath := filepath.Join(h.uploadMediaDir(), filepath.Base(key))
	file, err := openUploadMediaFile(fullPath)
	if err != nil {
		err = fmt.Errorf("open upload %s media (%s): %w", upload.ID, fullPath, err)
		h.logger().Error("serve upload media open", "uploadId", upload.ID, "path", fullPath, "err", err)
		WriteError(w, http.StatusNotFound, RequestError{Err: err, Status: http.StatusNotFound, Message: "media unavailable"})
		return
	}
	defer func() { _ = file.Close() }()
	stat, err := statUploadMediaFile(file)
	if err != nil {
		err = fmt.Errorf("stat upload %s media (%s): %w", upload.ID, fullPath, err)
		h.logger().Error("serve upload media stat", "uploadId", upload.ID, "path", fullPath, "err", err)
		WriteError(w, http.StatusInternalServerError, RequestError{Err: err, Status: http.StatusInternalServerError, Message: "unable to serve media"})
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

// deleteUploadMedia deletes upload media and returns an error when cleanup or persistence fails.
func (h *Handler) deleteUploadMedia(upload domain.Upload) {
	if upload.Metadata == nil {
		return
	}
	key := strings.TrimSpace(upload.Metadata["sourceObjectKey"])
	if key == "" {
		key = strings.TrimSpace(upload.Metadata["mediaPath"])
	}
	if key == "" {
		return
	}
	h.deleteStoredUploadSource(key)
}

func (h *Handler) deleteStoredUploadSource(key string) {
	if strings.TrimSpace(key) == "" {
		return
	}
	if store := h.uploadSourceStore(); store.Enabled() {
		if err := store.Delete(context.Background(), key); err != nil {
			h.logger().Warn("delete upload source", "key", key, "err", err)
			return
		}
		h.logger().Info("deleted upload source", "key", key, "backend", "object")
		return
	}
	fullPath := filepath.Join(h.uploadMediaDir(), filepath.Base(key))
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		h.logger().Warn("delete upload source", "key", key, "path", fullPath, "err", err)
		return
	}
	h.logger().Info("deleted upload source", "key", key, "path", fullPath, "backend", "filesystem")
}

// uploadMediaDir performs upload media dir and propagates validation or dependency failures to the caller.
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

// uploadMediaURL performs upload media url and propagates validation or dependency failures to the caller.
func (h *Handler) uploadMediaURL(r *http.Request, uploadID, token string) string {
	if r == nil {
		return ""
	}
	mediaURL := url.URL{Path: fmt.Sprintf("/api/uploads/%s/media", uploadID)}
	if token != "" {
		q := mediaURL.Query()
		q.Set("token", token)
		mediaURL.RawQuery = q.Encode()
	}
	if baseURL := strings.TrimSpace(h.UploadMediaBaseURL); baseURL != "" {
		if parsed, err := parseUploadMediaBaseURL(baseURL); err == nil {
			canonicalMediaURL := mediaURL
			canonicalMediaURL.Path = strings.TrimPrefix(canonicalMediaURL.Path, "/")
			return parsed.ResolveReference(&canonicalMediaURL).String()
		}
	}
	trustForwarded := h.shouldTrustForwarded(r)
	scheme := requestScheme(r, trustForwarded)
	base := ""
	if trustForwarded {
		base = forwardedHost(r)
	}
	if base == "" {
		base = r.Host
	}
	if base == "" && r.URL != nil {
		base = r.URL.Host
	}
	if base == "" {
		base = "localhost"
	}
	mediaURL.Scheme = scheme
	mediaURL.Host = base
	return mediaURL.String()
}

func parseUploadMediaBaseURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("upload media base URL is empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parse upload media base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("upload media base URL scheme must be http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("upload media base URL host is required")
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	return parsed, nil
}

// forwardedHost performs forwarded host and propagates validation or dependency failures to the caller.
func forwardedHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	if host := firstForwardedValue(r.Header.Get("X-Forwarded-Host")); host != "" {
		return host
	}
	forwarded := strings.TrimSpace(r.Header.Get("Forwarded"))
	if forwarded == "" {
		return ""
	}
	entries := strings.Split(forwarded, ",")
	if len(entries) == 0 {
		return ""
	}
	for _, param := range strings.Split(strings.TrimSpace(entries[0]), ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(param), "=")
		if !ok {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(key), "host") {
			continue
		}
		host := strings.TrimSpace(value)
		host = strings.Trim(host, "\"")
		if host != "" {
			return host
		}
	}
	return ""
}

// firstForwardedValue returns the first non-empty value from the provided candidates.
func firstForwardedValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

// requestScheme performs request scheme and propagates validation or dependency failures to the caller.
func requestScheme(r *http.Request, trustForwarded bool) string {
	if r == nil {
		return "http"
	}
	if trustForwarded {
		if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
			parts := strings.Split(proto, ",")
			return strings.TrimSpace(parts[0])
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// shouldTrustForwarded performs should trust forwarded and propagates validation or dependency failures to the caller.
func (h *Handler) shouldTrustForwarded(r *http.Request) bool {
	if h == nil || r == nil {
		return false
	}
	if h.TrustForwardedHeaders {
		return true
	}
	if len(h.TrustedProxies) == 0 {
		return false
	}
	h.trustedProxyOnce.Do(func() {
		networks, err := parseTrustedProxyNetworks(h.TrustedProxies)
		if err != nil {
			h.logger().Error("parse trusted proxies", "error", err)
			return
		}
		h.trustedProxyNets = networks
	})
	if len(h.trustedProxyNets) == 0 {
		return false
	}
	host := clientIPFromRemoteAddr(r.RemoteAddr)
	if host == "" {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, network := range h.trustedProxyNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// parseTrustedProxyNetworks parses trusted proxy networks and returns an error when the input is malformed.
func parseTrustedProxyNetworks(raw []string) ([]*net.IPNet, error) {
	var networks []*net.IPNet
	for _, value := range raw {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, network, err := net.ParseCIDR(trimmed); err == nil {
			networks = append(networks, network)
			continue
		}
		ip := net.ParseIP(trimmed)
		if ip == nil {
			return nil, fmt.Errorf("parse trusted proxy %q: invalid address", trimmed)
		}
		maskSize := 128
		if ip.To4() != nil {
			maskSize = 32
		}
		networks = append(networks, &net.IPNet{IP: ip, Mask: net.CIDRMask(maskSize, maskSize)})
	}
	return networks, nil
}

// clientIPFromRemoteAddr performs client ipfrom remote addr and propagates validation or dependency failures to the caller.
func clientIPFromRemoteAddr(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

// generateUploadMediaToken performs generate upload media token and propagates validation or dependency failures to the caller.
func generateUploadMediaToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
