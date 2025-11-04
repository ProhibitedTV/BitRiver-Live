package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"bitriver-live/internal/auth"
	"bitriver-live/internal/auth/oauth"
	"bitriver-live/internal/chat"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
	"bitriver-live/internal/observability/metrics"
	"bitriver-live/internal/storage"
)

type Handler struct {
	Store           storage.Repository
	Sessions        *auth.SessionManager
	ChatGateway     *chat.Gateway
	OAuth           oauth.Service
	UploadProcessor *UploadProcessor
}

func NewHandler(store storage.Repository, sessions *auth.SessionManager) *Handler {
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

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
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
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "bitriver_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSessionCookie removes the BitRiver session cookie from the response.
func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w, r)
}

func isSecureRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		for _, p := range strings.Split(proto, ",") {
			if strings.EqualFold(strings.TrimSpace(p), "https") {
				return true
			}
		}
	}
	if r.URL != nil && strings.EqualFold(r.URL.Scheme, "https") {
		return true
	}
	return false
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

	setSessionCookie(w, r, token, expiresAt)
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

	setSessionCookie(w, r, token, expiresAt)
	writeJSON(w, http.StatusOK, newAuthResponse(user, expiresAt))
}

type oauthStartRequest struct {
	ReturnTo string `json:"returnTo"`
}

func (h *Handler) OAuthProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}
	providers := []oauth.ProviderInfo{}
	if h.OAuth != nil {
		providers = h.OAuth.Providers()
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": providers})
}

func (h *Handler) OAuthByProvider(w http.ResponseWriter, r *http.Request) {
	if h.OAuth == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("oauth providers not configured"))
		return
	}
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/auth/oauth/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, fmt.Errorf("invalid oauth path"))
		return
	}
	provider := parts[0]
	action := parts[1]
	switch action {
	case "start":
		h.oauthStart(w, r, provider)
	case "callback":
		h.oauthCallback(w, r, provider)
	default:
		writeError(w, http.StatusNotFound, fmt.Errorf("invalid oauth action"))
	}
}

func (h *Handler) oauthStart(w http.ResponseWriter, r *http.Request, provider string) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}
	var req oauthStartRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	begin, err := h.OAuth.Begin(provider, sanitizeReturnPath(req.ReturnTo))
	if errors.Is(err, oauth.ErrProviderNotConfigured) {
		writeError(w, http.StatusNotFound, fmt.Errorf("oauth provider %s not configured", provider))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": begin.URL})
}

func (h *Handler) oauthCallback(w http.ResponseWriter, r *http.Request, provider string) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	query := r.URL.Query()
	state := query.Get("state")
	if errParam := query.Get("error"); errParam != "" {
		redirectTarget := "/"
		if dest, err := h.OAuth.Cancel(state); err == nil {
			redirectTarget = dest
		}
		http.Redirect(w, r, appendQueryParam(sanitizeReturnPath(redirectTarget), "oauth", "error"), http.StatusSeeOther)
		return
	}

	if state == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("state parameter is required"))
		return
	}
	code := query.Get("code")
	if strings.TrimSpace(code) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("authorization code is required"))
		return
	}

	completion, err := h.OAuth.Complete(provider, state, code)
	returnPath := sanitizeReturnPath(completion.ReturnTo)
	if returnPath == "" {
		returnPath = "/"
	}
	if err != nil {
		if errors.Is(err, oauth.ErrProviderNotConfigured) {
			writeError(w, http.StatusNotFound, fmt.Errorf("oauth provider %s not configured", provider))
			return
		}
		http.Redirect(w, r, appendQueryParam(returnPath, "oauth", "error"), http.StatusSeeOther)
		return
	}

	user, err := h.Store.AuthenticateOAuth(storage.OAuthLoginParams{
		Provider:    completion.Profile.Provider,
		Subject:     completion.Profile.Subject,
		Email:       completion.Profile.Email,
		DisplayName: completion.Profile.DisplayName,
	})
	if err != nil {
		http.Redirect(w, r, appendQueryParam(returnPath, "oauth", "error"), http.StatusSeeOther)
		return
	}

	token, expiresAt, err := h.sessionManager().Create(user.ID)
	if err != nil {
		http.Redirect(w, r, appendQueryParam(returnPath, "oauth", "error"), http.StatusSeeOther)
		return
	}
	setSessionCookie(w, r, token, expiresAt)
	http.Redirect(w, r, appendQueryParam(returnPath, "oauth", "success"), http.StatusSeeOther)
}

func sanitizeReturnPath(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "/"
	}
	parsed, err := url.Parse(trimmed)
	if err == nil {
		if parsed.IsAbs() {
			trimmed = parsed.Path
			if parsed.RawQuery != "" {
				trimmed = trimmed + "?" + parsed.RawQuery
			}
		} else {
			trimmed = parsed.RequestURI()
		}
	}
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" || strings.HasPrefix(trimmed, "//") {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return trimmed
}

func appendQueryParam(path, key, value string) string {
	parsed, err := url.Parse(path)
	if err != nil {
		parsed = &url.URL{Path: path}
	}
	if parsed.Scheme != "" && parsed.Host != "" {
		parsed.Scheme = ""
		parsed.Host = ""
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	parsed.Fragment = ""
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String()
}

func (h *Handler) Session(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		token := ExtractToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("missing session token"))
			return
		}
		userID, expiresAt, ok, err := h.sessionManager().Validate(token)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
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
		if err := h.sessionManager().Revoke(token); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		clearSessionCookie(w, r)
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
		if requester.ID != id && !requester.HasRole(roleAdmin) {
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

type channelResponse struct {
	channelPublicResponse
	StreamKey string `json:"streamKey"`
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

type subscriptionStateResponse struct {
	Subscribers int     `json:"subscribers"`
	Subscribed  bool    `json:"subscribed"`
	Tier        string  `json:"tier,omitempty"`
	RenewsAt    *string `json:"renewsAt,omitempty"`
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
	Channel      channelPublicResponse      `json:"channel"`
	Owner        channelOwnerResponse       `json:"owner"`
	Profile      profileSummaryResponse     `json:"profile"`
	Live         bool                       `json:"live"`
	Follow       followStateResponse        `json:"follow"`
	Subscription *subscriptionStateResponse `json:"subscription,omitempty"`
	Playback     *playbackStreamResponse    `json:"playback,omitempty"`
}

type vodItemResponse struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	DurationSeconds int    `json:"durationSeconds"`
	PublishedAt     string `json:"publishedAt"`
	ThumbnailURL    string `json:"thumbnailUrl,omitempty"`
	PlaybackURL     string `json:"playbackUrl,omitempty"`
}

type vodCollectionResponse struct {
	ChannelID string            `json:"channelId"`
	Items     []vodItemResponse `json:"items"`
}

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

func (h *Handler) Directory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	query := ""
	if r.URL != nil {
		query = strings.TrimSpace(r.URL.Query().Get("q"))
	}
	channels := h.Store.ListChannels("", query)
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

func buildChannelResponse(channel models.Channel, includeStreamKey bool) channelResponse {
	resp := channelResponse{
		channelPublicResponse: channelPublicResponse{
			ID:        channel.ID,
			OwnerID:   channel.OwnerID,
			Title:     channel.Title,
			Category:  channel.Category,
			Tags:      append([]string{}, channel.Tags...),
			LiveState: channel.LiveState,
			CreatedAt: channel.CreatedAt.Format(time.RFC3339Nano),
			UpdatedAt: channel.UpdatedAt.Format(time.RFC3339Nano),
		},
	}
	if channel.CurrentSessionID != nil {
		sessionID := *channel.CurrentSessionID
		resp.CurrentSessionID = &sessionID
	}
	if includeStreamKey {
		resp.StreamKey = channel.StreamKey
	}
	return resp
}

func newChannelResponse(channel models.Channel) channelResponse {
	return buildChannelResponse(channel, true)
}

func newChannelPublicResponse(channel models.Channel) channelPublicResponse {
	return buildChannelResponse(channel, false).channelPublicResponse
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

func (h *Handler) subscriptionState(channelID string, actor *models.User) (subscriptionStateResponse, error) {
	subs, err := h.Store.ListSubscriptions(channelID, false)
	if err != nil {
		return subscriptionStateResponse{}, err
	}
	state := subscriptionStateResponse{Subscribers: len(subs)}
	if actor == nil {
		return state, nil
	}
	for _, sub := range subs {
		if sub.UserID != actor.ID {
			continue
		}
		state.Subscribed = true
		tier := strings.TrimSpace(sub.Tier)
		if tier != "" {
			state.Tier = tier
		}
		renews := sub.ExpiresAt.Format(time.RFC3339Nano)
		state.RenewsAt = &renews
		break
	}
	return state, nil
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

func (h *Handler) Channels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		actor, ok := h.requireAuthenticatedUser(w, r)
		if !ok {
			return
		}
		ownerID := strings.TrimSpace(r.URL.Query().Get("ownerId"))
		if ownerID == "" {
			if !actor.HasRole(roleAdmin) {
				ownerID = actor.ID
			}
		} else if ownerID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}

		channels := h.Store.ListChannels(ownerID, "")
		if ownerID == actor.ID || actor.HasRole(roleAdmin) {
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
		if req.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
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
			if actor, ok := UserFromContext(r.Context()); ok && (channel.OwnerID == actor.ID || actor.HasRole(roleAdmin)) {
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
			var viewer *models.User
			if actor, ok := UserFromContext(r.Context()); ok {
				follow.Following = h.Store.IsFollowingChannel(actor.ID, channel.ID)
				viewer = &actor
			}
			response := channelPlaybackResponse{
				Channel: newChannelPublicResponse(channel),
				Owner:   newOwnerResponse(owner, profile),
				Profile: newProfileSummaryResponse(profile),
				Live:    channel.LiveState == "live" || channel.LiveState == "starting",
				Follow:  follow,
			}
			if state, err := h.subscriptionState(channel.ID, viewer); err == nil {
				response.Subscription = &state
			} else {
				writeError(w, http.StatusInternalServerError, err)
				return
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
		case "subscribe":
			if len(parts) > 2 {
				writeError(w, http.StatusNotFound, fmt.Errorf("unknown channel path"))
				return
			}
			channel, ok := h.Store.GetChannel(channelID)
			if !ok {
				writeError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			switch r.Method {
			case http.MethodGet:
				var viewer *models.User
				if actor, ok := UserFromContext(r.Context()); ok {
					viewer = &actor
				}
				state, err := h.subscriptionState(channel.ID, viewer)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				writeJSON(w, http.StatusOK, state)
			case http.MethodPost:
				actor, ok := h.requireAuthenticatedUser(w, r)
				if !ok {
					return
				}
				subs, err := h.Store.ListSubscriptions(channel.ID, false)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				alreadySubscribed := false
				for _, sub := range subs {
					if sub.UserID == actor.ID {
						alreadySubscribed = true
						break
					}
				}
				if !alreadySubscribed {
					params := storage.CreateSubscriptionParams{
						ChannelID: channel.ID,
						UserID:    actor.ID,
						Tier:      "supporter",
						Provider:  "internal",
						Amount:    0,
						Currency:  "USD",
						Duration:  30 * 24 * time.Hour,
						AutoRenew: true,
					}
					sub, err := h.Store.CreateSubscription(params)
					if err != nil {
						writeError(w, http.StatusBadRequest, err)
						return
					}
					metrics.Default().ObserveMonetization("subscription", sub.Amount)
				}
				state, err := h.subscriptionState(channel.ID, &actor)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				writeJSON(w, http.StatusOK, state)
			case http.MethodDelete:
				actor, ok := h.requireAuthenticatedUser(w, r)
				if !ok {
					return
				}
				subs, err := h.Store.ListSubscriptions(channel.ID, false)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				subscriptionID := ""
				for _, sub := range subs {
					if sub.UserID == actor.ID {
						subscriptionID = sub.ID
						break
					}
				}
				if subscriptionID != "" {
					if _, err := h.Store.CancelSubscription(subscriptionID, actor.ID, ""); err != nil {
						writeError(w, http.StatusBadRequest, err)
						return
					}
				}
				state, err := h.subscriptionState(channel.ID, &actor)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				writeJSON(w, http.StatusOK, state)
			default:
				w.Header().Set("Allow", "GET, POST, DELETE")
				writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
			}
			return
		case "vods":
			if len(parts) > 2 {
				writeError(w, http.StatusNotFound, fmt.Errorf("unknown channel path"))
				return
			}
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
			recordings, err := h.Store.ListRecordings(channel.ID, false)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			items := make([]vodItemResponse, 0, len(recordings))
			for _, recording := range recordings {
				if recording.PublishedAt == nil {
					continue
				}
				item := newVodItemResponse(recording)
				if item.PublishedAt == "" {
					continue
				}
				items = append(items, item)
			}
			payload := vodCollectionResponse{ChannelID: channel.ID, Items: items}
			writeJSON(w, http.StatusOK, payload)
			return
		case "chat":
			h.handleChatRoutes(channelID, parts[2:], w, r)
			return
		case "monetization":
			channel, ok := h.Store.GetChannel(channelID)
			if !ok {
				writeError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
				return
			}
			h.handleMonetizationRoutes(channel, parts[2:], w, r)
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

type clipExportRequest struct {
	Title        string `json:"title"`
	StartSeconds int    `json:"startSeconds"`
	EndSeconds   int    `json:"endSeconds"`
}

type createUploadRequest struct {
	ChannelID   string            `json:"channelId"`
	Title       string            `json:"title"`
	Filename    string            `json:"filename"`
	SizeBytes   int64             `json:"sizeBytes"`
	PlaybackURL string            `json:"playbackUrl"`
	Metadata    map[string]string `json:"metadata"`
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
	case "rotate":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
			return
		}
		updated, err := h.Store.RotateChannelStreamKey(channel.ID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, newChannelResponse(updated))
	default:
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown stream action %s", action))
	}
}

type createChatRequest struct {
	UserID  string `json:"userId"`
	Content string `json:"content"`
}

type chatModerationRequest struct {
	Action     string `json:"action"`
	TargetID   string `json:"targetId"`
	DurationMs int    `json:"durationMs"`
	Reason     string `json:"reason,omitempty"`
}

type chatModerationResponse struct {
	Action    string  `json:"action"`
	ChannelID string  `json:"channelId"`
	TargetID  string  `json:"targetId"`
	ExpiresAt *string `json:"expiresAt,omitempty"`
}

type moderationUserResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName,omitempty"`
}

type moderationFlagResponse struct {
	ID           string                  `json:"id"`
	ChannelID    string                  `json:"channelId"`
	ChannelTitle string                  `json:"channelTitle,omitempty"`
	Reporter     *moderationUserResponse `json:"reporter,omitempty"`
	Target       *moderationUserResponse `json:"target,omitempty"`
	Reason       string                  `json:"reason,omitempty"`
	Message      string                  `json:"message,omitempty"`
	MessageID    string                  `json:"messageId,omitempty"`
	EvidenceURL  string                  `json:"evidenceUrl,omitempty"`
	CreatedAt    string                  `json:"createdAt,omitempty"`
	FlaggedAt    string                  `json:"flaggedAt,omitempty"`
}

type moderationActionResponse struct {
	ID           string                  `json:"id"`
	ChannelID    string                  `json:"channelId"`
	ChannelTitle string                  `json:"channelTitle,omitempty"`
	Action       string                  `json:"action,omitempty"`
	TargetID     string                  `json:"targetId,omitempty"`
	Moderator    *moderationUserResponse `json:"moderator,omitempty"`
	CreatedAt    string                  `json:"createdAt,omitempty"`
}

type moderationQueueResponse struct {
	Queue   []moderationFlagResponse   `json:"queue"`
	Actions []moderationActionResponse `json:"actions"`
}

type analyticsSummaryResponse struct {
	LiveViewers      int     `json:"liveViewers"`
	StreamsLive      int     `json:"streamsLive"`
	WatchTimeMinutes float64 `json:"watchTimeMinutes"`
	ChatMessages     int     `json:"chatMessages"`
}

type analyticsChannelResponse struct {
	ChannelID       string  `json:"channelId"`
	Title           string  `json:"title,omitempty"`
	LiveViewers     int     `json:"liveViewers"`
	Followers       int     `json:"followers"`
	AvgWatchMinutes float64 `json:"avgWatchMinutes"`
	ChatMessages    int     `json:"chatMessages"`
}

type analyticsOverviewResponse struct {
	Summary    *analyticsSummaryResponse  `json:"summary,omitempty"`
	PerChannel []analyticsChannelResponse `json:"perChannel"`
}

type chatRestrictionResponse struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	TargetID  string  `json:"targetId"`
	ActorID   string  `json:"actorId,omitempty"`
	Reason    string  `json:"reason,omitempty"`
	IssuedAt  string  `json:"issuedAt"`
	ExpiresAt *string `json:"expiresAt,omitempty"`
}

type chatReportRequest struct {
	TargetID    string `json:"targetId"`
	Reason      string `json:"reason"`
	MessageID   string `json:"messageId,omitempty"`
	EvidenceURL string `json:"evidenceUrl,omitempty"`
}

type resolveModerationRequest struct {
	Resolution string `json:"resolution"`
}

type chatReportResponse struct {
	ID          string  `json:"id"`
	ChannelID   string  `json:"channelId"`
	ReporterID  string  `json:"reporterId"`
	TargetID    string  `json:"targetId"`
	Reason      string  `json:"reason"`
	Status      string  `json:"status"`
	Resolution  string  `json:"resolution,omitempty"`
	MessageID   string  `json:"messageId,omitempty"`
	EvidenceURL string  `json:"evidenceUrl,omitempty"`
	CreatedAt   string  `json:"createdAt"`
	ResolvedAt  *string `json:"resolvedAt,omitempty"`
	ResolverID  string  `json:"resolverId,omitempty"`
}

type resolveChatReportRequest struct {
	Resolution string `json:"resolution"`
}

type chatMessageResponse struct {
	ID        string `json:"id"`
	ChannelID string `json:"channelId"`
	UserID    string `json:"userId"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

type createTipRequest struct {
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Provider      string  `json:"provider"`
	Reference     string  `json:"reference,omitempty"`
	WalletAddress string  `json:"walletAddress,omitempty"`
	Message       string  `json:"message,omitempty"`
}

type tipResponse struct {
	ID            string  `json:"id"`
	ChannelID     string  `json:"channelId"`
	FromUserID    string  `json:"fromUserId"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Provider      string  `json:"provider"`
	Reference     string  `json:"reference"`
	WalletAddress string  `json:"walletAddress,omitempty"`
	Message       string  `json:"message,omitempty"`
	CreatedAt     string  `json:"createdAt"`
}

type createSubscriptionRequest struct {
	Tier              string  `json:"tier"`
	Provider          string  `json:"provider"`
	Reference         string  `json:"reference,omitempty"`
	ExternalReference string  `json:"externalReference,omitempty"`
	Amount            float64 `json:"amount"`
	Currency          string  `json:"currency"`
	DurationDays      int     `json:"durationDays"`
	AutoRenew         bool    `json:"autoRenew"`
}

type subscriptionResponse struct {
	ID                string  `json:"id"`
	ChannelID         string  `json:"channelId"`
	UserID            string  `json:"userId"`
	Tier              string  `json:"tier"`
	Provider          string  `json:"provider"`
	Reference         string  `json:"reference"`
	ExternalReference string  `json:"externalReference,omitempty"`
	Amount            float64 `json:"amount"`
	Currency          string  `json:"currency"`
	StartedAt         string  `json:"startedAt"`
	ExpiresAt         string  `json:"expiresAt"`
	AutoRenew         bool    `json:"autoRenew"`
	Status            string  `json:"status"`
	CancelledBy       string  `json:"cancelledBy,omitempty"`
	CancelledReason   string  `json:"cancelledReason,omitempty"`
	CancelledAt       *string `json:"cancelledAt,omitempty"`
}

type cancelSubscriptionRequest struct {
	Reason string `json:"reason"`
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

func newChatRestrictionResponse(r models.ChatRestriction) chatRestrictionResponse {
	resp := chatRestrictionResponse{
		ID:       r.ID,
		Type:     r.Type,
		TargetID: r.TargetID,
		ActorID:  r.ActorID,
		Reason:   r.Reason,
		IssuedAt: r.IssuedAt.Format(time.RFC3339Nano),
	}
	if r.ExpiresAt != nil {
		expires := r.ExpiresAt.Format(time.RFC3339Nano)
		resp.ExpiresAt = &expires
	}
	if resp.ActorID == "" {
		resp.ActorID = r.ActorID
	}
	return resp
}

func newChatReportResponse(report models.ChatReport) chatReportResponse {
	resp := chatReportResponse{
		ID:          report.ID,
		ChannelID:   report.ChannelID,
		ReporterID:  report.ReporterID,
		TargetID:    report.TargetID,
		Reason:      report.Reason,
		Status:      report.Status,
		Resolution:  report.Resolution,
		MessageID:   report.MessageID,
		EvidenceURL: report.EvidenceURL,
		CreatedAt:   report.CreatedAt.Format(time.RFC3339Nano),
		ResolverID:  report.ResolverID,
	}
	if report.ResolvedAt != nil {
		resolved := report.ResolvedAt.Format(time.RFC3339Nano)
		resp.ResolvedAt = &resolved
	}
	return resp
}

func newModerationUser(user models.User) moderationUserResponse {
	resp := moderationUserResponse{ID: user.ID}
	if user.DisplayName != "" {
		resp.DisplayName = user.DisplayName
	}
	return resp
}

func newTipResponse(tip models.Tip) tipResponse {
	return tipResponse{
		ID:            tip.ID,
		ChannelID:     tip.ChannelID,
		FromUserID:    tip.FromUserID,
		Amount:        tip.Amount,
		Currency:      tip.Currency,
		Provider:      tip.Provider,
		Reference:     tip.Reference,
		WalletAddress: tip.WalletAddress,
		Message:       tip.Message,
		CreatedAt:     tip.CreatedAt.Format(time.RFC3339Nano),
	}
}

func newSubscriptionResponse(sub models.Subscription) subscriptionResponse {
	resp := subscriptionResponse{
		ID:                sub.ID,
		ChannelID:         sub.ChannelID,
		UserID:            sub.UserID,
		Tier:              sub.Tier,
		Provider:          sub.Provider,
		Reference:         sub.Reference,
		ExternalReference: sub.ExternalReference,
		Amount:            sub.Amount,
		Currency:          sub.Currency,
		StartedAt:         sub.StartedAt.Format(time.RFC3339Nano),
		ExpiresAt:         sub.ExpiresAt.Format(time.RFC3339Nano),
		AutoRenew:         sub.AutoRenew,
		Status:            sub.Status,
		CancelledBy:       sub.CancelledBy,
		CancelledReason:   sub.CancelledReason,
	}
	if sub.CancelledAt != nil {
		cancelled := sub.CancelledAt.Format(time.RFC3339Nano)
		resp.CancelledAt = &cancelled
	}
	return resp
}

func (h *Handler) ChatWebsocket(w http.ResponseWriter, r *http.Request) {
	if h.ChatGateway == nil {
		http.Error(w, "chat gateway unavailable", http.StatusServiceUnavailable)
		return
	}
	user, ok := h.requireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	h.ChatGateway.HandleConnection(w, r, user)
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
			writeError(w, http.StatusBadRequest, fmt.Errorf("channelId is required"))
			return
		}
		channel, exists := h.Store.GetChannel(channelID)
		if !exists {
			writeError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
			return
		}
		if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		uploads, err := h.Store.ListUploads(channelID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		response := make([]uploadResponse, 0, len(uploads))
		for _, upload := range uploads {
			response = append(response, newUploadResponse(upload))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		actor, ok := h.requireAuthenticatedUser(w, r)
		if !ok {
			return
		}
		var req createUploadRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		channelID := strings.TrimSpace(req.ChannelID)
		if channelID == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("channelId is required"))
			return
		}
		channel, exists := h.Store.GetChannel(channelID)
		if !exists {
			writeError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
			return
		}
		if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		var metadata map[string]string
		if len(req.Metadata) > 0 {
			metadata = make(map[string]string, len(req.Metadata))
			for k, v := range req.Metadata {
				metadata[k] = v
			}
		}
		playbackURL := strings.TrimSpace(req.PlaybackURL)
		if playbackURL != "" {
			if metadata == nil {
				metadata = make(map[string]string, 1)
			}
			metadata["sourceUrl"] = playbackURL
		}

		params := storage.CreateUploadParams{
			ChannelID:   channelID,
			Title:       req.Title,
			Filename:    req.Filename,
			SizeBytes:   req.SizeBytes,
			Metadata:    metadata,
			PlaybackURL: playbackURL,
		}
		upload, err := h.Store.CreateUpload(params)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if h.UploadProcessor != nil {
			h.UploadProcessor.Enqueue(upload.ID)
		}
		writeJSON(w, http.StatusCreated, newUploadResponse(upload))
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}

func (h *Handler) UploadByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/uploads/")
	if path == "" {
		writeError(w, http.StatusNotFound, fmt.Errorf("upload id missing"))
		return
	}
	parts := strings.Split(path, "/")
	uploadID := strings.TrimSpace(parts[0])
	upload, ok := h.Store.GetUpload(uploadID)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("upload %s not found", uploadID))
		return
	}
	channel, exists := h.Store.GetChannel(upload.ChannelID)
	if !exists {
		writeError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", upload.ChannelID))
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
		writeJSON(w, http.StatusOK, newUploadResponse(upload))
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
			writeError(w, http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, DELETE")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}

func (h *Handler) Recordings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	channelID := strings.TrimSpace(r.URL.Query().Get("channelId"))
	if channelID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("channelId is required"))
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
		writeError(w, http.StatusBadRequest, err)
		return
	}
	response := make([]recordingResponse, 0, len(recordings))
	for _, recording := range recordings {
		response = append(response, newRecordingResponse(recording))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) RecordingByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/recordings/")
	if path == "" {
		writeError(w, http.StatusNotFound, fmt.Errorf("recording id missing"))
		return
	}
	parts := strings.Split(path, "/")
	recordingID := strings.TrimSpace(parts[0])
	remaining := parts[1:]

	recording, ok := h.Store.GetRecording(recordingID)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("recording %s not found", recordingID))
		return
	}
	channel, channelExists := h.Store.GetChannel(recording.ChannelID)
	if !channelExists {
		writeError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", recording.ChannelID))
		return
	}
	actor, hasActor := UserFromContext(r.Context())

	if len(remaining) > 0 && remaining[0] != "" {
		action := remaining[0]
		switch action {
		case "publish":
			if len(remaining) > 1 {
				writeError(w, http.StatusNotFound, fmt.Errorf("unknown recording path"))
				return
			}
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", "POST")
				writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
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
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, newRecordingResponse(updated))
			return
		case "clips":
			if len(remaining) > 1 {
				writeError(w, http.StatusNotFound, fmt.Errorf("unknown recording path"))
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
					writeError(w, http.StatusBadRequest, err)
					return
				}
				response := make([]clipExportResponse, 0, len(clips))
				for _, clip := range clips {
					response = append(response, newClipExportResponse(clip))
				}
				writeJSON(w, http.StatusOK, response)
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
				if err := decodeJSON(r, &req); err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				clip, err := h.Store.CreateClipExport(recordingID, storage.ClipExportParams{
					Title:        req.Title,
					StartSeconds: req.StartSeconds,
					EndSeconds:   req.EndSeconds,
				})
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				writeJSON(w, http.StatusCreated, newClipExportResponse(clip))
			default:
				w.Header().Set("Allow", "GET, POST")
				writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
			}
			return
		default:
			writeError(w, http.StatusNotFound, fmt.Errorf("unknown recording path"))
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
		writeJSON(w, http.StatusOK, newRecordingResponse(recording))
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
			writeError(w, http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, DELETE")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
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
	channel, exists := h.Store.GetChannel(channelID)
	if !exists {
		writeError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
		return
	}

	if len(remaining) > 0 && remaining[0] != "" {
		actor, ok := h.requireAuthenticatedUser(w, r)
		if !ok {
			return
		}
		switch remaining[0] {
		case "moderation":
			h.handleChatModeration(actor, channel, remaining[1:], w, r)
			return
		default:
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
			if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
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
		actor, ok := h.requireAuthenticatedUser(w, r)
		if !ok {
			return
		}
		var req createChatRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if req.UserID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		if h.ChatGateway != nil {
			author, ok := h.Store.GetUser(req.UserID)
			if !ok {
				writeError(w, http.StatusBadRequest, fmt.Errorf("user %s not found", req.UserID))
				return
			}
			messageEvt, err := h.ChatGateway.CreateMessage(r.Context(), author, channelID, req.Content)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			chatMessage := models.ChatMessage{
				ID:        messageEvt.ID,
				ChannelID: messageEvt.ChannelID,
				UserID:    messageEvt.UserID,
				Content:   messageEvt.Content,
				CreatedAt: messageEvt.CreatedAt,
			}
			writeJSON(w, http.StatusCreated, newChatMessageResponse(chatMessage))
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

func (h *Handler) handleChatModeration(actor models.User, channel models.Channel, remaining []string, w http.ResponseWriter, r *http.Request) {
	if h.ChatGateway == nil {
		http.Error(w, "chat gateway unavailable", http.StatusServiceUnavailable)
		return
	}
	if len(remaining) > 0 {
		switch remaining[0] {
		case "restrictions":
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", "GET")
				writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
				return
			}
			if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
				WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
				return
			}
			restrictions := h.Store.ListChatRestrictions(channel.ID)
			response := make([]chatRestrictionResponse, 0, len(restrictions))
			for _, restriction := range restrictions {
				response = append(response, newChatRestrictionResponse(restriction))
			}
			writeJSON(w, http.StatusOK, response)
			return
		case "reports":
			h.handleChatReports(actor, channel, remaining[1:], w, r)
			return
		}
	}
	if len(remaining) > 0 {
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown chat moderation path"))
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}
	var req chatModerationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.TargetID) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("targetId is required"))
		return
	}
	if _, ok := h.Store.GetUser(req.TargetID); !ok {
		writeError(w, http.StatusBadRequest, fmt.Errorf("user %s not found", req.TargetID))
		return
	}
	var evt chat.ModerationEvent
	evt.ChannelID = channel.ID
	evt.ActorID = actor.ID
	evt.TargetID = req.TargetID
	evt.Reason = strings.TrimSpace(req.Reason)

	switch strings.ToLower(strings.TrimSpace(req.Action)) {
	case "timeout":
		duration := time.Duration(req.DurationMs) * time.Millisecond
		if duration <= 0 {
			writeError(w, http.StatusBadRequest, fmt.Errorf("durationMs must be positive"))
			return
		}
		expires := time.Now().Add(duration).UTC()
		evt.Action = chat.ModerationActionTimeout
		evt.ExpiresAt = &expires
	case "remove_timeout", "untimeout":
		evt.Action = chat.ModerationActionRemoveTimeout
	case "ban":
		evt.Action = chat.ModerationActionBan
	case "unban":
		evt.Action = chat.ModerationActionUnban
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown moderation action"))
		return
	}

	if err := h.ChatGateway.ApplyModeration(r.Context(), actor, evt); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var expires *string
	if evt.ExpiresAt != nil {
		formatted := evt.ExpiresAt.Format(time.RFC3339Nano)
		expires = &formatted
	}
	writeJSON(w, http.StatusAccepted, chatModerationResponse{
		Action:    string(evt.Action),
		ChannelID: evt.ChannelID,
		TargetID:  evt.TargetID,
		ExpiresAt: expires,
	})
}

func (h *Handler) handleChatReports(actor models.User, channel models.Channel, remaining []string, w http.ResponseWriter, r *http.Request) {
	if len(remaining) > 0 && strings.TrimSpace(remaining[0]) != "" {
		reportID := remaining[0]
		if len(remaining) == 2 && remaining[1] == "resolve" {
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", "POST")
				writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
				return
			}
			if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
				WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
				return
			}
			var req resolveChatReportRequest
			if err := decodeJSON(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			report, err := h.Store.ResolveChatReport(reportID, actor.ID, req.Resolution)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, newChatReportResponse(report))
			return
		}
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown chat report path"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		includeResolved := false
		status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
		if status == "all" || status == "resolved" {
			includeResolved = true
		}
		reports, err := h.Store.ListChatReports(channel.ID, includeResolved)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		response := make([]chatReportResponse, 0, len(reports))
		for _, report := range reports {
			response = append(response, newChatReportResponse(report))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		var req chatReportRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		targetID := strings.TrimSpace(req.TargetID)
		if targetID == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("targetId is required"))
			return
		}
		if _, ok := h.Store.GetUser(targetID); !ok {
			writeError(w, http.StatusBadRequest, fmt.Errorf("user %s not found", targetID))
			return
		}
		reason := strings.TrimSpace(req.Reason)
		if reason == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("reason is required"))
			return
		}
		messageID := strings.TrimSpace(req.MessageID)
		evidence := strings.TrimSpace(req.EvidenceURL)
		if h.ChatGateway != nil {
			reporter, ok := h.Store.GetUser(actor.ID)
			if !ok {
				writeError(w, http.StatusInternalServerError, fmt.Errorf("reporter %s not found", actor.ID))
				return
			}
			evt, err := h.ChatGateway.SubmitReport(r.Context(), reporter, channel.ID, targetID, reason, messageID, evidence)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			report := models.ChatReport{
				ID:          evt.ID,
				ChannelID:   evt.ChannelID,
				ReporterID:  evt.ReporterID,
				TargetID:    evt.TargetID,
				Reason:      evt.Reason,
				MessageID:   evt.MessageID,
				EvidenceURL: evt.EvidenceURL,
				Status:      evt.Status,
				CreatedAt:   evt.CreatedAt,
			}
			writeJSON(w, http.StatusAccepted, newChatReportResponse(report))
			return
		}
		report, err := h.Store.CreateChatReport(channel.ID, actor.ID, targetID, reason, messageID, evidence)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusAccepted, newChatReportResponse(report))
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}

func (h *Handler) ModerationQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if _, ok := h.requireRole(w, r, roleAdmin); !ok {
		return
	}

	payload, err := h.moderationQueuePayload()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func (h *Handler) ModerationQueueByID(w http.ResponseWriter, r *http.Request) {
	flagID := strings.TrimPrefix(r.URL.Path, "/api/moderation/queue/")
	if flagID == "" {
		writeError(w, http.StatusNotFound, fmt.Errorf("flag id missing"))
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	actor, ok := h.requireRole(w, r, roleAdmin)
	if !ok {
		return
	}

	var req resolveModerationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	resolution := strings.TrimSpace(req.Resolution)
	if resolution == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("resolution is required"))
		return
	}

	report, err := h.Store.ResolveChatReport(flagID, actor.ID, resolution)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, newChatReportResponse(report))
}

func (h *Handler) moderationQueuePayload() (moderationQueueResponse, error) {
	channels := h.Store.ListChannels("", "")
	type flaggedItem struct {
		payload moderationFlagResponse
		created time.Time
	}
	type actionItem struct {
		payload moderationActionResponse
		created time.Time
	}
	flags := make([]flaggedItem, 0)
	actions := make([]actionItem, 0)
	for _, channel := range channels {
		reports, err := h.Store.ListChatReports(channel.ID, true)
		if err != nil {
			return moderationQueueResponse{}, err
		}
		for _, report := range reports {
			reporter, hasReporter := h.Store.GetUser(report.ReporterID)
			target, hasTarget := h.Store.GetUser(report.TargetID)
			createdAt := report.CreatedAt
			flag := moderationFlagResponse{
				ID:           report.ID,
				ChannelID:    report.ChannelID,
				ChannelTitle: channel.Title,
				Reason:       report.Reason,
				MessageID:    report.MessageID,
				EvidenceURL:  report.EvidenceURL,
				CreatedAt:    createdAt.Format(time.RFC3339Nano),
				FlaggedAt:    createdAt.Format(time.RFC3339Nano),
			}
			if hasReporter {
				reporterResp := newModerationUser(reporter)
				flag.Reporter = &reporterResp
			}
			if hasTarget {
				targetResp := newModerationUser(target)
				flag.Target = &targetResp
			}
			if strings.EqualFold(report.Status, "open") {
				flags = append(flags, flaggedItem{payload: flag, created: createdAt})
				continue
			}
			if strings.EqualFold(report.Status, "resolved") {
				resolvedAt := createdAt
				if report.ResolvedAt != nil {
					resolvedAt = report.ResolvedAt.UTC()
				}
				moderatorResp := (*moderationUserResponse)(nil)
				if resolverID := strings.TrimSpace(report.ResolverID); resolverID != "" {
					if moderator, exists := h.Store.GetUser(resolverID); exists {
						value := newModerationUser(moderator)
						moderatorResp = &value
					}
				}
				action := moderationActionResponse{
					ID:           report.ID,
					ChannelID:    report.ChannelID,
					ChannelTitle: channel.Title,
					Action:       strings.TrimSpace(report.Resolution),
					TargetID:     report.TargetID,
					Moderator:    moderatorResp,
					CreatedAt:    resolvedAt.Format(time.RFC3339Nano),
				}
				actions = append(actions, actionItem{payload: action, created: resolvedAt})
			}
		}
	}
	sort.Slice(flags, func(i, j int) bool {
		return flags[i].created.After(flags[j].created)
	})
	queue := make([]moderationFlagResponse, len(flags))
	for i, item := range flags {
		queue[i] = item.payload
	}
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].created.After(actions[j].created)
	})
	limit := len(actions)
	if limit > 20 {
		limit = 20
	}
	resolved := make([]moderationActionResponse, limit)
	for i := 0; i < limit; i++ {
		resolved[i] = actions[i].payload
	}
	return moderationQueueResponse{Queue: queue, Actions: resolved}, nil
}

func (h *Handler) AnalyticsOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}
	if _, ok := h.requireRole(w, r, roleAdmin); !ok {
		return
	}
	payload, err := h.computeAnalyticsOverview(time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func (h *Handler) computeAnalyticsOverview(now time.Time) (analyticsOverviewResponse, error) {
	channels := h.Store.ListChannels("", "")
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	windowStart := now.Add(-24 * time.Hour)
	summary := analyticsSummaryResponse{}
	perChannel := make([]analyticsChannelResponse, 0, len(channels))
	for _, channel := range channels {
		entry := analyticsChannelResponse{
			ChannelID: channel.ID,
			Title:     channel.Title,
			Followers: h.Store.CountFollowers(channel.ID),
		}
		if current, ok := h.Store.CurrentStreamSession(channel.ID); ok {
			entry.LiveViewers = current.PeakConcurrent
		}
		sessions, err := h.Store.ListStreamSessions(channel.ID)
		if err != nil {
			return analyticsOverviewResponse{}, err
		}
		if len(sessions) > 0 {
			totalMinutes := 0.0
			for _, session := range sessions {
				totalMinutes += sessionDurationMinutes(session, now)
				summary.WatchTimeMinutes += streamWatchOverlapMinutes(session, windowStart, now)
			}
			entry.AvgWatchMinutes = totalMinutes / float64(len(sessions))
		}
		messages, err := h.Store.ListChatMessages(channel.ID, 0)
		if err != nil {
			return analyticsOverviewResponse{}, err
		}
		today := 0
		for _, message := range messages {
			if message.CreatedAt.Before(startOfDay) {
				break
			}
			today++
		}
		entry.ChatMessages = today
		summary.ChatMessages += today
		summary.LiveViewers += entry.LiveViewers
		perChannel = append(perChannel, entry)
	}
	streamsLive := int(metrics.Default().ActiveStreams())
	if streamsLive <= 0 {
		count := 0
		for _, channel := range channels {
			state := strings.ToLower(strings.TrimSpace(channel.LiveState))
			if state == "live" || state == "starting" {
				count++
			}
		}
		streamsLive = count
	}
	summary.StreamsLive = streamsLive
	sort.Slice(perChannel, func(i, j int) bool {
		if perChannel[i].LiveViewers != perChannel[j].LiveViewers {
			return perChannel[i].LiveViewers > perChannel[j].LiveViewers
		}
		if perChannel[i].Followers != perChannel[j].Followers {
			return perChannel[i].Followers > perChannel[j].Followers
		}
		return perChannel[i].Title < perChannel[j].Title
	})
	resp := analyticsOverviewResponse{PerChannel: perChannel}
	if len(perChannel) > 0 || summary.LiveViewers > 0 || summary.StreamsLive > 0 || summary.WatchTimeMinutes > 0 || summary.ChatMessages > 0 {
		resp.Summary = &summary
	}
	return resp, nil
}

func sessionDurationMinutes(session models.StreamSession, now time.Time) float64 {
	end := now
	if session.EndedAt != nil && session.EndedAt.Before(end) {
		end = *session.EndedAt
	}
	if end.Before(session.StartedAt) {
		return 0
	}
	return end.Sub(session.StartedAt).Minutes()
}

func streamWatchOverlapMinutes(session models.StreamSession, windowStart, windowEnd time.Time) float64 {
	start := session.StartedAt
	if start.Before(windowStart) {
		start = windowStart
	}
	end := windowEnd
	if session.EndedAt != nil && session.EndedAt.Before(end) {
		end = *session.EndedAt
	}
	if end.Before(windowStart) {
		return 0
	}
	if end.After(windowEnd) {
		end = windowEnd
	}
	if !end.After(start) {
		return 0
	}
	return end.Sub(start).Minutes()
}

func (h *Handler) handleMonetizationRoutes(channel models.Channel, remaining []string, w http.ResponseWriter, r *http.Request) {
	if len(remaining) == 0 {
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown monetization path"))
		return
	}
	switch remaining[0] {
	case "tips":
		h.handleTipsRoutes(channel, remaining[1:], w, r)
	case "subscriptions":
		h.handleSubscriptionsRoutes(channel, remaining[1:], w, r)
	default:
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown monetization path"))
	}
}

func (h *Handler) handleTipsRoutes(channel models.Channel, remaining []string, w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	if len(remaining) > 0 && strings.TrimSpace(remaining[0]) != "" {
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown tips path"))
		return
	}
	switch r.Method {
	case http.MethodGet:
		if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		limit := 0
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			if value, err := strconv.Atoi(raw); err == nil && value > 0 {
				limit = value
			}
		}
		tips, err := h.Store.ListTips(channel.ID, limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		response := make([]tipResponse, 0, len(tips))
		for _, tip := range tips {
			response = append(response, newTipResponse(tip))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		var req createTipRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		params := storage.CreateTipParams{
			ChannelID:     channel.ID,
			FromUserID:    actor.ID,
			Amount:        req.Amount,
			Currency:      req.Currency,
			Provider:      req.Provider,
			Reference:     req.Reference,
			WalletAddress: req.WalletAddress,
			Message:       req.Message,
		}
		tip, err := h.Store.CreateTip(params)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		metrics.Default().ObserveMonetization("tip", tip.Amount)
		writeJSON(w, http.StatusCreated, newTipResponse(tip))
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	}
}

func (h *Handler) handleSubscriptionsRoutes(channel models.Channel, remaining []string, w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	if len(remaining) > 0 && strings.TrimSpace(remaining[0]) != "" {
		subscriptionID := remaining[0]
		if len(remaining) == 1 {
			if r.Method != http.MethodDelete {
				w.Header().Set("Allow", "DELETE")
				writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
				return
			}
			sub, ok := h.Store.GetSubscription(subscriptionID)
			if !ok {
				writeError(w, http.StatusNotFound, fmt.Errorf("subscription %s not found", subscriptionID))
				return
			}
			if sub.UserID != actor.ID && channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
				WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
				return
			}
			reason := strings.TrimSpace(r.URL.Query().Get("reason"))
			updated, err := h.Store.CancelSubscription(subscriptionID, actor.ID, reason)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, newSubscriptionResponse(updated))
			return
		}
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown subscription path"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		includeInactive := false
		status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
		if status == "all" || status == "inactive" {
			includeInactive = true
		}
		subs, err := h.Store.ListSubscriptions(channel.ID, includeInactive)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		response := make([]subscriptionResponse, 0, len(subs))
		for _, sub := range subs {
			response = append(response, newSubscriptionResponse(sub))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		var req createSubscriptionRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		durationDays := req.DurationDays
		if durationDays <= 0 {
			writeError(w, http.StatusBadRequest, fmt.Errorf("durationDays must be positive"))
			return
		}
		params := storage.CreateSubscriptionParams{
			ChannelID:         channel.ID,
			UserID:            actor.ID,
			Tier:              req.Tier,
			Provider:          req.Provider,
			Reference:         req.Reference,
			Amount:            req.Amount,
			Currency:          req.Currency,
			Duration:          time.Duration(durationDays) * 24 * time.Hour,
			AutoRenew:         req.AutoRenew,
			ExternalReference: req.ExternalReference,
		}
		sub, err := h.Store.CreateSubscription(params)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		metrics.Default().ObserveMonetization("subscription", sub.Amount)
		writeJSON(w, http.StatusCreated, newSubscriptionResponse(sub))
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
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
			if !actor.HasRole(roleAdmin) {
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
	channels := h.Store.ListChannels(user.ID, "")
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
