package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"bitriver-live/internal/models"
)

type contextKey string

const (
	userContextKey contextKey = "authenticatedUser"

	roleAdmin   = "admin"
	roleCreator = "creator"
)

// ContextWithUser stores the authenticated user in the provided context.
func ContextWithUser(ctx context.Context, user models.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext retrieves the authenticated user from context if present.
func UserFromContext(ctx context.Context) (models.User, bool) {
	user, ok := ctx.Value(userContextKey).(models.User)
	return user, ok
}

// AuthenticateRequest validates the session token on the request and returns the user.
func (h *Handler) AuthenticateRequest(r *http.Request) (models.User, error) {
	token := ExtractToken(r)
	if token == "" {
		return models.User{}, fmt.Errorf("missing session token")
	}
	userID, _, ok := h.sessionManager().Validate(token)
	if !ok {
		return models.User{}, fmt.Errorf("invalid or expired session")
	}
	user, exists := h.Store.GetUser(userID)
	if !exists {
		return models.User{}, fmt.Errorf("account not found")
	}
	return user, nil
}

func (h *Handler) requireAuthenticatedUser(w http.ResponseWriter, r *http.Request) (models.User, bool) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, fmt.Errorf("authentication required"))
		return models.User{}, false
	}
	return user, true
}

func (h *Handler) requireRole(w http.ResponseWriter, r *http.Request, roles ...string) (models.User, bool) {
	user, ok := h.requireAuthenticatedUser(w, r)
	if !ok {
		return models.User{}, false
	}
	if len(roles) == 0 {
		return user, true
	}
	if !userHasAnyRole(user, roles...) {
		WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
		return models.User{}, false
	}
	return user, true
}

func userHasAnyRole(user models.User, roles ...string) bool {
	if len(roles) == 0 {
		return true
	}
	for _, required := range roles {
		if userHasRole(user, required) {
			return true
		}
	}
	return false
}

func userHasRole(user models.User, role string) bool {
	for _, existing := range user.Roles {
		if strings.EqualFold(existing, role) {
			return true
		}
	}
	return false
}

func (h *Handler) ensureChannelAccess(w http.ResponseWriter, r *http.Request, channel models.Channel) (models.User, bool) {
	user, ok := h.requireRole(w, r, roleAdmin, roleCreator)
	if !ok {
		return models.User{}, false
	}
	if channel.OwnerID != user.ID && !userHasRole(user, roleAdmin) {
		WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
		return models.User{}, false
	}
	return user, true
}
