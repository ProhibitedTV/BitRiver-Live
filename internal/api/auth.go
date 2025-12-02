package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"bitriver-live/internal/models"
)

// contextKey is a private type used to avoid collisions when storing values
// in context.Context.
type contextKey string

const (
	// userContextKey is the context key under which the authenticated user is stored.
	userContextKey contextKey = "authenticatedUser"

	// roleAdmin represents a site-wide administrator with full access.
	roleAdmin = "admin"
	// roleCreator represents a content creator who owns one or more channels.
	roleCreator = "creator"
)

// ContextWithUser stores the authenticated user in the provided context.
//
// This is typically called after AuthenticateRequest has successfully
// resolved the current user from a session token, and before the request
// is passed down to application handlers.
func ContextWithUser(ctx context.Context, user models.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext retrieves the authenticated user from context, if present.
//
// It returns the user and a boolean indicating whether a user was found.
func UserFromContext(ctx context.Context) (models.User, bool) {
	user, ok := ctx.Value(userContextKey).(models.User)
	return user, ok
}

// AuthenticateRequest validates the session token on the request and returns
// the associated user alongside the refreshed session expiry when available.
//
// The token is extracted using ExtractToken (e.g., from cookies or headers)
// and validated via the sessionManager. If the token is missing, invalid,
// expired, or the user no longer exists, an error is returned.
func (h *Handler) AuthenticateRequest(r *http.Request) (models.User, time.Time, error) {
	token := ExtractToken(r)
	if token == "" {
		return models.User{}, time.Time{}, fmt.Errorf("missing session token")
	}

	userID, expiresAt, ok, err := h.sessionManager().Validate(token)
	if err != nil {
		return models.User{}, time.Time{}, fmt.Errorf("session validation failed: %w", err)
	}
	if !ok {
		return models.User{}, time.Time{}, fmt.Errorf("invalid or expired session")
	}

	user, exists := h.Store.GetUser(userID)
	if !exists {
		return models.User{}, time.Time{}, fmt.Errorf("account not found")
	}

	return user, expiresAt, nil
}

// requireAuthenticatedUser ensures that a request has an authenticated user
// attached to its context.
//
// If no user is present, it writes a 401 Unauthorized response and returns
// false. On success, it returns the user and true.
func (h *Handler) requireAuthenticatedUser(w http.ResponseWriter, r *http.Request) (models.User, bool) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, fmt.Errorf("authentication required"))
		return models.User{}, false
	}
	return user, true
}

// requireRole ensures that the current user has at least one of the provided
// roles before allowing the request to proceed.
//
// If the user is not authenticated, a 401 is returned. If the user does not
// have any of the required roles, a 403 Forbidden response is written.
// When roles is empty, any authenticated user is allowed.
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

// userHasAnyRole reports whether the user has at least one of the provided
// roles. If roles is empty, it returns true.
func userHasAnyRole(user models.User, roles ...string) bool {
	if len(roles) == 0 {
		return true
	}
	for _, required := range roles {
		if user.HasRole(required) {
			return true
		}
	}
	return false
}

// ensureChannelAccess verifies that the current user has permission to access
// the given channel.
//
// Access rules:
//   - The user must be authenticated and have either the admin or creator role.
//   - Admins may access any channel.
//   - Creators may only access channels where channel.OwnerID matches their ID.
//
// On failure, a 401 or 403 response is written and false is returned.
func (h *Handler) ensureChannelAccess(w http.ResponseWriter, r *http.Request, channel models.Channel) (models.User, bool) {
	user, ok := h.requireRole(w, r, roleAdmin, roleCreator)
	if !ok {
		return models.User{}, false
	}
	if channel.OwnerID != user.ID && !user.HasRole(roleAdmin) {
		WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
		return models.User{}, false
	}
	return user, true
}
