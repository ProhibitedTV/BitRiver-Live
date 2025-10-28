package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bitriver-live/internal/auth"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
	"bitriver-live/internal/observability/metrics"
	"bitriver-live/internal/storage"
)

type Handler struct {
	Store    *storage.Storage
	Sessions *auth.SessionManager
}

func NewHandler(store *storage.Storage, sessions *auth.SessionManager) *Handler {
	if sessions == nil {
		sessions = auth.NewSessionManager(24 * time.Hour)
	}
	return &Handler{Store: store, Sessions: sessions}
}

func (h *Handler) sessionManager() *auth.SessionManager {
	if h.Sessions == nil {
		h.Sessions = auth.NewSessionManager(24 * time.Hour)
	}
	return h.Sessions
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// WriteError is an exported helper for returning JSON API errors.
func WriteError(w http.ResponseWriter, status int, err error) {
	writeError(w, status, err)
}

func setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	if token == "" {
		return
	}
	maxAge := int(time.Until(expires).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "bitriver_session",
		Value:    token,
		Path:     "/",
		Expires:  expires.UTC(),
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "bitriver_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

func decodeJSON(r *http.Request, dest interface{}) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	return nil
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	var checks []ingest.HealthStatus
	if h.Store != nil {
		checks = h.Store.IngestHealth(r.Context())
	}
	status := "ok"
	for _, check := range checks {
		switch strings.ToLower(check.Status) {
		case "ok", "disabled":
			continue
		default:
			status = "degraded"
		}
	}
	payload := map[string]interface{}{
		"status":   status,
		"services": checks,
	}
	for _, check := range checks {
		metrics.SetIngestHealth(check.Component, check.Status)
	}
	writeJSON(w, http.StatusOK, payload)
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	var req signupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("password must be at least 8 characters"))
		return
	}

	user, err := h.Store.CreateUser(storage.CreateUserParams{
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Password:    req.Password,
		SelfSignup:  true,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	token, expiresAt, err := h.sessionManager().Create(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	setSessionCookie(w, token, expiresAt)
	writeJSON(w, http.StatusCreated, newAuthResponse(user, expiresAt))
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	user, err := h.Store.AuthenticateUser(req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid credentials"))
		return
	}

	token, expiresAt, err := h.sessionManager().Create(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	setSessionCookie(w, token, expiresAt)
	writeJSON(w, http.StatusOK, newAuthResponse(user, expiresAt))
}

func (h *Handler) Session(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		token := ExtractToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing session token"))
			return
		}
		userID, expiresAt, ok := h.sessionManager().Validate(token)
		if !ok {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid or expired session"))
			return
		}
		user, exists := h.Store.GetUser(userID)
		if !exists {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("account not found"))
			return
		}
		writeJSON(w, http.StatusOK, newAuthResponse(user, expiresAt))
	case http.MethodDelete:
		token := ExtractToken(r)
		if token == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("missing session token"))
			return
		}
		h.sessionManager().Revoke(token)
		clearSessionCookie(w)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, DELETE")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}

func ExtractToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header != "" {
		parts := strings.SplitN(header, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	if cookie, err := r.Cookie("bitriver_session"); err == nil {
		return cookie.Value
	}
	return ""
}

// Users

type createUserRequest struct {
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	Roles       []string `json:"roles"`
	Password    string   `json:"password,omitempty"`
}

type updateUserRequest struct {
	DisplayName *string   `json:"displayName"`
	Email       *string   `json:"email"`
	Roles       *[]string `json:"roles"`
}

type signupRequest struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	ExpiresAt string       `json:"expiresAt"`
	User      userResponse `json:"user"`
}

type userResponse struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	Roles       []string `json:"roles"`
	SelfSignup  bool     `json:"selfSignup"`
	HasPassword bool     `json:"hasPassword"`
	CreatedAt   string   `json:"createdAt"`
}

func newUserResponse(user models.User) userResponse {
	return userResponse{
		ID:          user.ID,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Roles:       append([]string{}, user.Roles...),
		SelfSignup:  user.SelfSignup,
		HasPassword: user.PasswordHash != "",
		CreatedAt:   user.CreatedAt.Format(time.RFC3339Nano),
	}
}

func newAuthResponse(user models.User, expires time.Time) authResponse {
	return authResponse{
		ExpiresAt: expires.UTC().Format(time.RFC3339Nano),
		User:      newUserResponse(user),
	}
}

func (h *Handler) Users(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := h.requireRole(w, r, roleAdmin); !ok {
			return
		}
		users := h.Store.ListUsers()
		response := make([]userResponse, 0, len(users))
		for _, user := range users {
			response = append(response, newUserResponse(user))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		if _, ok := h.requireRole(w, r, roleAdmin); !ok {
			return
		}
		var req createUserRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		user, err := h.Store.CreateUser(storage.CreateUserParams{
			DisplayName: req.DisplayName,
			Email:       req.Email,
			Roles:       req.Roles,
			Password:    req.Password,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, newUserResponse(user))
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}

func (h *Handler) UserByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/users/")
	if id == "" {
		writeError(w, http.StatusNotFound, fmt.Errorf("user id missing"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		requester, ok := h.requireAuthenticatedUser(w, r)
		if !ok {
			return
		}
		if requester.ID != id && !userHasRole(requester, roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		user, ok := h.Store.GetUser(id)
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("user %s not found", id))
			return
		}
		writeJSON(w, http.StatusOK, newUserResponse(user))
	case http.MethodPatch:
		if _, ok := h.requireRole(w, r, roleAdmin); !ok {
			return
		}
		var req updateUserRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		update := storage.UserUpdate{}
		if req.DisplayName != nil {
			update.DisplayName = req.DisplayName
		}
		if req.Email != nil {
			update.Email = req.Email
		}
		if req.Roles != nil {
			rolesCopy := append([]string{}, (*req.Roles)...)
			update.Roles = &rolesCopy
		}
		user, err := h.Store.UpdateUser(id, update)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, newUserResponse(user))
	case http.MethodDelete:
		if _, ok := h.requireRole(w, r, roleAdmin); !ok {
			return
		}
		if err := h.Store.DeleteUser(id); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, PATCH, DELETE")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}

// Channels

type createChannelRequest struct {
	OwnerID  string   `json:"ownerId"`
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
}

type updateChannelRequest struct {
	Title    *string   `json:"title"`
	Category *string   `json:"category"`
	Tags     *[]string `json:"tags"`
}

type channelResponse struct {
	ID               string   `json:"id"`
	OwnerID          string   `json:"ownerId"`
	StreamKey        string   `json:"streamKey"`
	Title            string   `json:"title"`
	Category         string   `json:"category,omitempty"`
	Tags             []string `json:"tags"`
	LiveState        string   `json:"liveState"`
	CurrentSessionID *string  `json:"currentSessionId,omitempty"`
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
}

type channelPublicResponse struct {
	ID               string   `json:"id"`
	OwnerID          string   `json:"ownerId"`
	Title            string   `json:"title"`
	Category         string   `json:"category,omitempty"`
	Tags             []string `json:"tags"`
	LiveState        string   `json:"liveState"`
	CurrentSessionID *string  `json:"currentSessionId,omitempty"`
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
}

type channelOwnerResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
}

type profileSummaryResponse struct {
	Bio       string `json:"bio,omitempty"`
	AvatarURL string `json:"avatarUrl,omitempty"`
	BannerURL string `json:"bannerUrl,omitempty"`
}

type directoryChannelResponse struct {
	Channel       channelPublicResponse  `json:"channel"`
	Owner         channelOwnerResponse   `json:"owner"`
	Profile       profileSummaryResponse `json:"profile"`
	Live          bool                   `json:"live"`
	FollowerCount int                    `json:"followerCount"`
}

type directoryResponse struct {
	Channels    []directoryChannelResponse `json:"channels"`
	GeneratedAt string                     `json:"generatedAt"`
}

type followStateResponse struct {
	Followers int  `json:"followers"`
	Following bool `json:"following"`
}

type playbackStreamResponse struct {
	SessionID   string                      `json:"sessionId"`
	StartedAt   string                      `json:"startedAt"`
	PlaybackURL string                      `json:"playbackUrl,omitempty"`
	OriginURL   string                      `json:"originUrl,omitempty"`
	Protocol    string                      `json:"protocol,omitempty"`
	PlayerHint  string                      `json:"playerHint,omitempty"`
	LatencyMode string                      `json:"latencyMode,omitempty"`
	Renditions  []renditionManifestResponse `json:"renditions,omitempty"`
}

type channelPlaybackResponse struct {
	Channel  channelPublicResponse   `json:"channel"`
	Owner    channelOwnerResponse    `json:"owner"`
	Profile  profileSummaryResponse  `json:"profile"`
	Live     bool                    `json:"live"`
	Follow   followStateResponse     `json:"follow"`
	Playback *playbackStreamResponse `json:"playback,omitempty"`
}

func (h *Handler) Directory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	channels := h.Store.ListChannels("")
	response := make([]directoryChannelResponse, 0, len(channels))
	for _, channel := range channels {
		owner, exists := h.Store.GetUser(channel.OwnerID)
		if !exists {
			continue
		}
		profile, _ := h.Store.GetProfile(owner.ID)
		followerCount := h.Store.CountFollowers(channel.ID)
		response = append(response, directoryChannelResponse{
			Channel:       newChannelPublicResponse(channel),
			Owner:         newOwnerResponse(owner, profile),
			Profile:       newProfileSummaryResponse(profile),
			Live:          channel.LiveState == "live" || channel.LiveState == "starting",
			FollowerCount: followerCount,
		})
	}

	payload := directoryResponse{
		Channels:    response,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	writeJSON(w, http.StatusOK, payload)
}

func newChannelResponse(channel models.Channel) channelResponse {
	resp := channelResponse{
		ID:        channel.ID,
		OwnerID:   channel.OwnerID,
		StreamKey: channel.StreamKey,
		Title:     channel.Title,
		Category:  channel.Category,
		Tags:      append([]string{}, channel.Tags...),
		LiveState: channel.LiveState,
		CreatedAt: channel.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt: channel.UpdatedAt.Format(time.RFC3339Nano),
	}
	if channel.CurrentSessionID != nil {
		sessionID := *channel.CurrentSessionID
		resp.CurrentSessionID = &sessionID
	}
	return resp
}

func newChannelPublicResponse(channel models.Channel) channelPublicResponse {
	resp := channelPublicResponse{
		ID:        channel.ID,
		OwnerID:   channel.OwnerID,
		Title:     channel.Title,
		Category:  channel.Category,
		Tags:      append([]string{}, channel.Tags...),
		LiveState: channel.LiveState,
		CreatedAt: channel.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt: channel.UpdatedAt.Format(time.RFC3339Nano),
	}
	if channel.CurrentSessionID != nil {
		sessionID := *channel.CurrentSessionID
		resp.CurrentSessionID = &sessionID
	}
	return resp
}

func newOwnerResponse(user models.User, profile models.Profile) channelOwnerResponse {
	owner := channelOwnerResponse{ID: user.ID, DisplayName: user.DisplayName}
	if profile.AvatarURL != "" {
		owner.AvatarURL = profile.AvatarURL
	}
	return owner
}

func newProfileSummaryResponse(profile models.Profile) profileSummaryResponse {
	summary := profileSummaryResponse{}
	if profile.Bio != "" {
		summary.Bio = profile.Bio
	}
	if profile.AvatarURL != "" {
		summary.AvatarURL = profile.AvatarURL
	}
	if profile.BannerURL != "" {
		summary.BannerURL = profile.BannerURL
	}
	return summary
}

func (h *Handler) Channels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		actor, ok := h.requireAuthenticatedUser(w, r)
		if !ok {
			return
		}
		ownerID := strings.TrimSpace(r.URL.Query().Get("ownerId"))
		if ownerID == "" {
			if !userHasRole(actor, roleAdmin) {
				ownerID = actor.ID
			}
		} else if ownerID != actor.ID && !userHasRole(actor, roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}

		channels := h.Store.ListChannels(ownerID)
		if ownerID == actor.ID || userHasRole(actor, roleAdmin) {
			response := make([]channelResponse, 0, len(channels))
			for _, channel := range channels {
				response = append(response, newChannelResponse(channel))
			}
			writeJSON(w, http.StatusOK, response)
			return
		}

		response := make([]channelPublicResponse, 0, len(channels))
		for _, channel := range channels {
			response = append(response, newChannelPublicResponse(channel))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		actor, ok := h.requireRole(w, r, roleAdmin, roleCreator)
		if !ok {
			return
		}
		var req createChannelRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if req.OwnerID == "" {
			req.OwnerID = actor.ID
		}
		if req.OwnerID != actor.ID && !userHasRole(actor, roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		channel, err := h.Store.CreateChannel(req.OwnerID, req.Title, req.Category, req.Tags)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, newChannelResponse(channel))
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}

func (h *Handler) ChannelByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/channels/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, fmt.Errorf("channel id missing"))
		return
	}
	channelID := parts[0]

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			channel, ok := h.Store.GetChannel(channelID)
			if !ok {
				writeError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			if actor, ok := UserFromContext(r.Context()); ok && (channel.OwnerID == actor.ID || userHasRole(actor, roleAdmin)) {
				writeJSON(w, http.StatusOK, newChannelResponse(channel))
				return
			}
			writeJSON(w, http.StatusOK, newChannelPublicResponse(channel))
		case http.MethodPatch:
			channel, ok := h.Store.GetChannel(channelID)
			if !ok {
				writeError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			if _, ok := h.ensureChannelAccess(w, r, channel); !ok {
				return
			}
			var req updateChannelRequest
			if err := decodeJSON(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			update := storage.ChannelUpdate{}
			if req.Title != nil {
				update.Title = req.Title
			}
			if req.Category != nil {
				update.Category = req.Category
			}
			if req.Tags != nil {
				tagsCopy := append([]string{}, (*req.Tags)...)
				update.Tags = &tagsCopy
			}
			channel, err := h.Store.UpdateChannel(channelID, update)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, newChannelResponse(channel))
		case http.MethodDelete:
			channel, ok := h.Store.GetChannel(channelID)
			if !ok {
				writeError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			if _, ok := h.ensureChannelAccess(w, r, channel); !ok {
				return
			}
			if err := h.Store.DeleteChannel(channelID); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("Allow", "GET, PATCH, DELETE")
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		}
		return
	}

	if len(parts) >= 2 {
		switch parts[1] {
		case "playback":
			channel, ok := h.Store.GetChannel(channelID)
			if !ok {
				writeError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", "GET")
				writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
				return
			}
			owner, exists := h.Store.GetUser(channel.OwnerID)
			if !exists {
				writeError(w, http.StatusInternalServerError, fmt.Errorf("channel owner %s not found", channel.OwnerID))
				return
			}
			profile, _ := h.Store.GetProfile(owner.ID)
			follow := followStateResponse{Followers: h.Store.CountFollowers(channel.ID)}
			if actor, ok := UserFromContext(r.Context()); ok {
				follow.Following = h.Store.IsFollowingChannel(actor.ID, channel.ID)
			}
			response := channelPlaybackResponse{
				Channel: newChannelPublicResponse(channel),
				Owner:   newOwnerResponse(owner, profile),
				Profile: newProfileSummaryResponse(profile),
				Live:    channel.LiveState == "live" || channel.LiveState == "starting",
				Follow:  follow,
			}
			if session, live := h.Store.CurrentStreamSession(channel.ID); live {
				playback := playbackStreamResponse{
					SessionID: session.ID,
					StartedAt: session.StartedAt.Format(time.RFC3339Nano),
				}
				if session.PlaybackURL != "" {
					playback.PlaybackURL = session.PlaybackURL
				}
				if session.OriginURL != "" {
					playback.OriginURL = session.OriginURL
				}
				if len(session.RenditionManifests) > 0 {
					manifests := make([]renditionManifestResponse, 0, len(session.RenditionManifests))
					for _, manifest := range session.RenditionManifests {
						manifests = append(manifests, renditionManifestResponse{
							Name:        manifest.Name,
							ManifestURL: manifest.ManifestURL,
							Bitrate:     manifest.Bitrate,
						})
					}
					playback.Renditions = manifests
				}
				protocol := "ll-hls"
				player := "hls.js"
				latency := "low-latency"
				url := strings.ToLower(playback.PlaybackURL)
				if strings.HasPrefix(url, "webrtc") || strings.HasPrefix(url, "wss") {
					protocol = "webrtc"
					player = "ovenplayer"
					latency = "ultra-low"
				}
				playback.Protocol = protocol
				playback.PlayerHint = player
				playback.LatencyMode = latency
				response.Playback = &playback
			}
			writeJSON(w, http.StatusOK, response)
			return
		case "stream":
			channel, ok := h.Store.GetChannel(channelID)
			if !ok {
				writeError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			h.handleStreamRoutes(channel, parts[2:], w, r)
			return
		case "sessions":
			channel, ok := h.Store.GetChannel(channelID)
			if !ok {
				writeError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			if _, ok := h.ensureChannelAccess(w, r, channel); !ok {
				return
			}
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", "GET")
				writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
				return
			}
			sessions, err := h.Store.ListStreamSessions(channelID)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			response := make([]sessionResponse, 0, len(sessions))
			for _, session := range sessions {
				response = append(response, newSessionResponse(session))
			}
			writeJSON(w, http.StatusOK, response)
			return
		case "follow":
			if len(parts) > 2 {
				writeError(w, http.StatusNotFound, fmt.Errorf("unknown channel path"))
				return
			}
			if _, ok := h.Store.GetChannel(channelID); !ok {
				writeError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			actor, ok := h.requireAuthenticatedUser(w, r)
			if !ok {
				return
			}
			switch r.Method {
			case http.MethodPost:
				if err := h.Store.FollowChannel(actor.ID, channelID); err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
			case http.MethodDelete:
				if err := h.Store.UnfollowChannel(actor.ID, channelID); err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
			default:
				w.Header().Set("Allow", "POST, DELETE")
				writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
				return
			}
			state := followStateResponse{
				Followers: h.Store.CountFollowers(channelID),
				Following: h.Store.IsFollowingChannel(actor.ID, channelID),
			}
			writeJSON(w, http.StatusOK, state)
			return
		case "chat":
			h.handleChatRoutes(channelID, parts[2:], w, r)
			return
		}
	}

	writeError(w, http.StatusNotFound, fmt.Errorf("unknown channel path"))
}

type startStreamRequest struct {
	Renditions []string `json:"renditions"`
}

type stopStreamRequest struct {
	PeakConcurrent int `json:"peakConcurrent"`
}

type sessionResponse struct {
	ID                 string                      `json:"id"`
	ChannelID          string                      `json:"channelId"`
	StartedAt          string                      `json:"startedAt"`
	EndedAt            *string                     `json:"endedAt,omitempty"`
	Renditions         []string                    `json:"renditions"`
	PeakConcurrent     int                         `json:"peakConcurrent"`
	OriginURL          string                      `json:"originUrl,omitempty"`
	PlaybackURL        string                      `json:"playbackUrl,omitempty"`
	IngestEndpoints    []string                    `json:"ingestEndpoints,omitempty"`
	IngestJobIDs       []string                    `json:"ingestJobIds,omitempty"`
	RenditionManifests []renditionManifestResponse `json:"renditionManifests,omitempty"`
}

func newSessionResponse(session models.StreamSession) sessionResponse {
	resp := sessionResponse{
		ID:             session.ID,
		ChannelID:      session.ChannelID,
		StartedAt:      session.StartedAt.Format(time.RFC3339Nano),
		Renditions:     append([]string{}, session.Renditions...),
		PeakConcurrent: session.PeakConcurrent,
	}
	if session.EndedAt != nil {
		ended := session.EndedAt.Format(time.RFC3339Nano)
		resp.EndedAt = &ended
	}
	if session.OriginURL != "" {
		resp.OriginURL = session.OriginURL
	}
	if session.PlaybackURL != "" {
		resp.PlaybackURL = session.PlaybackURL
	}
	if len(session.IngestEndpoints) > 0 {
		resp.IngestEndpoints = append([]string{}, session.IngestEndpoints...)
	}
	if len(session.IngestJobIDs) > 0 {
		resp.IngestJobIDs = append([]string{}, session.IngestJobIDs...)
	}
	if len(session.RenditionManifests) > 0 {
		manifests := make([]renditionManifestResponse, 0, len(session.RenditionManifests))
		for _, manifest := range session.RenditionManifests {
			manifests = append(manifests, renditionManifestResponse{
				Name:        manifest.Name,
				ManifestURL: manifest.ManifestURL,
				Bitrate:     manifest.Bitrate,
			})
		}
		resp.RenditionManifests = manifests
	}
	return resp
}

type renditionManifestResponse struct {
	Name        string `json:"name"`
	ManifestURL string `json:"manifestUrl"`
	Bitrate     int    `json:"bitrate,omitempty"`
}

func (h *Handler) handleStreamRoutes(channel models.Channel, remaining []string, w http.ResponseWriter, r *http.Request) {
	if len(remaining) == 0 {
		writeError(w, http.StatusNotFound, fmt.Errorf("stream action missing"))
		return
	}
	if _, ok := h.ensureChannelAccess(w, r, channel); !ok {
		return
	}
	action := remaining[0]
	switch action {
	case "start":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
			return
		}
		var req startStreamRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		session, err := h.Store.StartStream(channel.ID, req.Renditions)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		metrics.StreamStarted()
		writeJSON(w, http.StatusCreated, newSessionResponse(session))
	case "stop":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
			return
		}
		var req stopStreamRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		session, err := h.Store.StopStream(channel.ID, req.PeakConcurrent)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		metrics.StreamStopped()
		writeJSON(w, http.StatusOK, newSessionResponse(session))
	default:
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown stream action %s", action))
	}
}

type createChatRequest struct {
	UserID  string `json:"userId"`
	Content string `json:"content"`
}

type chatMessageResponse struct {
	ID        string `json:"id"`
	ChannelID string `json:"channelId"`
	UserID    string `json:"userId"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

func newChatMessageResponse(message models.ChatMessage) chatMessageResponse {
	return chatMessageResponse{
		ID:        message.ID,
		ChannelID: message.ChannelID,
		UserID:    message.UserID,
		Content:   message.Content,
		CreatedAt: message.CreatedAt.Format(time.RFC3339Nano),
	}
}

type cryptoAddressPayload struct {
	Currency string `json:"currency"`
	Address  string `json:"address"`
	Note     string `json:"note,omitempty"`
}

type upsertProfileRequest struct {
	Bio               *string                 `json:"bio"`
	AvatarURL         *string                 `json:"avatarUrl"`
	BannerURL         *string                 `json:"bannerUrl"`
	FeaturedChannelID *string                 `json:"featuredChannelId"`
	TopFriends        *[]string               `json:"topFriends"`
	DonationAddresses *[]cryptoAddressPayload `json:"donationAddresses"`
}

type cryptoAddressResponse struct {
	Currency string `json:"currency"`
	Address  string `json:"address"`
	Note     string `json:"note,omitempty"`
}

type friendSummaryResponse struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
}

type profileViewResponse struct {
	UserID            string                  `json:"userId"`
	DisplayName       string                  `json:"displayName"`
	Bio               string                  `json:"bio"`
	AvatarURL         string                  `json:"avatarUrl"`
	BannerURL         string                  `json:"bannerUrl"`
	FeaturedChannelID *string                 `json:"featuredChannelId,omitempty"`
	TopFriends        []friendSummaryResponse `json:"topFriends"`
	DonationAddresses []cryptoAddressResponse `json:"donationAddresses"`
	Channels          []channelPublicResponse `json:"channels"`
	LiveChannels      []channelPublicResponse `json:"liveChannels"`
	CreatedAt         string                  `json:"createdAt"`
	UpdatedAt         string                  `json:"updatedAt"`
}

func (h *Handler) handleChatRoutes(channelID string, remaining []string, w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	channel, exists := h.Store.GetChannel(channelID)
	if !exists {
		writeError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
		return
	}
	if len(remaining) > 0 && remaining[0] != "" {
		messageID := remaining[0]
		if len(remaining) > 1 {
			writeError(w, http.StatusNotFound, fmt.Errorf("unknown chat path"))
			return
		}
		if r.Method != http.MethodDelete {
			w.Header().Set("Allow", "DELETE")
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
			return
		}
		if channel.OwnerID != actor.ID && !userHasRole(actor, roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		if err := h.Store.DeleteChatMessage(channelID, messageID); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.Method {
	case http.MethodGet:
		limitStr := r.URL.Query().Get("limit")
		limit := 0
		if limitStr != "" {
			parsed, err := strconv.Atoi(limitStr)
			if err != nil || parsed < 0 {
				writeError(w, http.StatusBadRequest, fmt.Errorf("invalid limit value"))
				return
			}
			limit = parsed
		}
		messages, err := h.Store.ListChatMessages(channelID, limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		response := make([]chatMessageResponse, 0, len(messages))
		for _, message := range messages {
			response = append(response, newChatMessageResponse(message))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		var req createChatRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if req.UserID != actor.ID && !userHasRole(actor, roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		message, err := h.Store.CreateChatMessage(channelID, req.UserID, req.Content)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, newChatMessageResponse(message))
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}

// Profiles

func (h *Handler) Profiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := h.requireAuthenticatedUser(w, r); !ok {
			return
		}
		profiles := h.Store.ListProfiles()
		response := make([]profileViewResponse, 0, len(profiles))
		for _, profile := range profiles {
			user, ok := h.Store.GetUser(profile.UserID)
			if !ok {
				continue
			}
			response = append(response, h.buildProfileViewResponse(user, profile))
		}
		writeJSON(w, http.StatusOK, response)
	default:
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}

func (h *Handler) ProfileByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/profiles/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, fmt.Errorf("profile id missing"))
		return
	}
	userID := parts[0]

	switch r.Method {
	case http.MethodGet:
		if _, ok := h.requireAuthenticatedUser(w, r); !ok {
			return
		}
		h.handleGetProfile(userID, w, r)
	case http.MethodPut:
		actor, ok := h.requireAuthenticatedUser(w, r)
		if !ok {
			return
		}
		if actor.ID != userID {
			if !userHasRole(actor, roleAdmin) {
				WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
				return
			}
		} else if !userHasAnyRole(actor, roleAdmin, roleCreator) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		h.handleUpsertProfile(userID, w, r)
	default:
		w.Header().Set("Allow", "GET, PUT")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}

func (h *Handler) handleGetProfile(userID string, w http.ResponseWriter, r *http.Request) {
	user, ok := h.Store.GetUser(userID)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("user %s not found", userID))
		return
	}
	profile, _ := h.Store.GetProfile(userID)
	writeJSON(w, http.StatusOK, h.buildProfileViewResponse(user, profile))
}

func (h *Handler) handleUpsertProfile(userID string, w http.ResponseWriter, r *http.Request) {
	var req upsertProfileRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	user, ok := h.Store.GetUser(userID)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("user %s not found", userID))
		return
	}

	update := storage.ProfileUpdate{}
	if req.Bio != nil {
		update.Bio = req.Bio
	}
	if req.AvatarURL != nil {
		update.AvatarURL = req.AvatarURL
	}
	if req.BannerURL != nil {
		update.BannerURL = req.BannerURL
	}
	if req.FeaturedChannelID != nil {
		update.FeaturedChannelID = req.FeaturedChannelID
	}
	if req.TopFriends != nil {
		friendsCopy := append([]string{}, (*req.TopFriends)...)
		update.TopFriends = &friendsCopy
	}
	if req.DonationAddresses != nil {
		addresses := make([]models.CryptoAddress, 0, len(*req.DonationAddresses))
		for _, addr := range *req.DonationAddresses {
			addresses = append(addresses, models.CryptoAddress{
				Currency: addr.Currency,
				Address:  addr.Address,
				Note:     addr.Note,
			})
		}
		update.DonationAddresses = &addresses
	}

	profile, err := h.Store.UpsertProfile(userID, update)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, h.buildProfileViewResponse(user, profile))
}

func (h *Handler) buildProfileViewResponse(user models.User, profile models.Profile) profileViewResponse {
	channels := h.Store.ListChannels(user.ID)
	channelResponses := make([]channelPublicResponse, 0, len(channels))
	liveResponses := make([]channelPublicResponse, 0)
	for _, channel := range channels {
		resp := newChannelPublicResponse(channel)
		channelResponses = append(channelResponses, resp)
		if channel.LiveState == "live" {
			liveResponses = append(liveResponses, resp)
		}
	}

	friends := make([]friendSummaryResponse, 0, len(profile.TopFriends))
	for _, friendID := range profile.TopFriends {
		friendUser, ok := h.Store.GetUser(friendID)
		if !ok {
			continue
		}
		friendProfile, _ := h.Store.GetProfile(friendID)
		friends = append(friends, friendSummaryResponse{
			UserID:      friendUser.ID,
			DisplayName: friendUser.DisplayName,
			AvatarURL:   friendProfile.AvatarURL,
		})
	}

	donations := make([]cryptoAddressResponse, 0, len(profile.DonationAddresses))
	for _, addr := range profile.DonationAddresses {
		donations = append(donations, cryptoAddressResponse{
			Currency: addr.Currency,
			Address:  addr.Address,
			Note:     addr.Note,
		})
	}

	response := profileViewResponse{
		UserID:            user.ID,
		DisplayName:       user.DisplayName,
		Bio:               profile.Bio,
		AvatarURL:         profile.AvatarURL,
		BannerURL:         profile.BannerURL,
		TopFriends:        friends,
		DonationAddresses: donations,
		Channels:          channelResponses,
		LiveChannels:      liveResponses,
		CreatedAt:         profile.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt:         profile.UpdatedAt.Format(time.RFC3339Nano),
	}
	if profile.FeaturedChannelID != nil {
		id := *profile.FeaturedChannelID
		response.FeaturedChannelID = &id
	}
	return response
}
