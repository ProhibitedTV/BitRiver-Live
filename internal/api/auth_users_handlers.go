package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bitriver-live/internal/auth/oauth"
	"bitriver-live/internal/models"
	"bitriver-live/internal/storage"
)

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if !h.AllowSelfSignup {
		writeError(w, http.StatusForbidden, errors.New("public self-signup is disabled"))
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
		slog.Error("signup create user failed", "email", req.Email, "error", err)
		writeError(w, http.StatusBadRequest, errors.New("unable to create account"))
		return
	}

	token, expiresAt, err := h.sessionManager().Create(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	h.setSessionCookie(w, r, token, expiresAt)
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

	h.setSessionCookie(w, r, token, expiresAt)
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
	h.setSessionCookie(w, r, token, expiresAt)
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
		h.ClearSessionCookie(w, r)
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
