package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"bitriver-live/internal/auth"
	"bitriver-live/internal/models"
)

const (
	mfaIssuer = "BitRiver Live"
)

type mfaStatusResponse struct {
	Enabled                bool    `json:"enabled"`
	Pending                bool    `json:"pending"`
	EnrolledAt             *string `json:"enrolledAt,omitempty"`
	LastUsedAt             *string `json:"lastUsedAt,omitempty"`
	RecoveryCodesRemaining int     `json:"recoveryCodesRemaining"`
}

type mfaEnrollmentResponse struct {
	Secret        string   `json:"secret"`
	OTPAuthURL    string   `json:"otpauthUrl"`
	RecoveryCodes []string `json:"recoveryCodes"`
}

type mfaChallengeResponse struct {
	MFARequired bool                   `json:"mfaRequired"`
	MFAToken    string                 `json:"mfaToken"`
	MFAExpires  string                 `json:"mfaExpiresAt"`
	Enrollment  *mfaEnrollmentResponse `json:"enrollment,omitempty"`
}

type mfaEnrollRequest struct {
	Token string `json:"token,omitempty"`
}

type mfaVerifyRequest struct {
	Token string `json:"token,omitempty"`
	Code  string `json:"code"`
}

type mfaDisableRequest struct {
	Code string `json:"code"`
}

// MFAStatus performs mfastatus and returns an error when dependent systems reject the operation.
func (h *Handler) MFAStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}
	user, ok := h.requireSessionUser(w, r)
	if !ok {
		return
	}
	settings, exists, err := h.Store.GetMFASettings(user.ID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}
	WriteJSON(w, http.StatusOK, buildMFAStatus(settings, exists))
}

// MFAEnroll performs mfaenroll and returns an error when dependent systems reject the operation.
func (h *Handler) MFAEnroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteMethodNotAllowed(w, r, http.MethodPost)
		return
	}
	var req mfaEnrollRequest
	if !DecodeAndValidate(w, r, &req) {
		return
	}

	user, err := h.resolveMFAUser(req.Token, r)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, err)
		return
	}

	settings, exists, err := h.Store.GetMFASettings(user.ID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}
	if exists && settings.Enabled {
		WriteError(w, http.StatusConflict, fmt.Errorf("mfa already enabled"))
		return
	}

	enrollment, updated, err := h.prepareMFAEnrollment(user)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	if _, err := h.Store.UpsertMFASettings(updated); err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}
	WriteJSON(w, http.StatusOK, enrollment)
}

// MFAVerify performs mfaverify and returns an error when dependent systems reject the operation.
func (h *Handler) MFAVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteMethodNotAllowed(w, r, http.MethodPost)
		return
	}
	var req mfaVerifyRequest
	if !DecodeAndValidate(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		WriteRequestError(w, ValidationError("code is required"))
		return
	}

	userID, _, tokenUsed, err := h.resolveMFAChallenge(req.Token, r)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, err)
		return
	}

	settings, exists, err := h.Store.GetMFASettings(userID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}
	if !exists || strings.TrimSpace(settings.Secret) == "" {
		WriteError(w, http.StatusBadRequest, fmt.Errorf("mfa not enrolled"))
		return
	}

	now := time.Now().UTC()
	verified, err := auth.VerifyTOTP(settings.Secret, req.Code, now)
	if err != nil {
		WriteRequestError(w, ValidationError(err.Error()))
		return
	}
	if !verified {
		consumed := false
		for i, hash := range settings.RecoveryCodes {
			if auth.MatchRecoveryCode(hash, req.Code) {
				settings.RecoveryCodes = append(settings.RecoveryCodes[:i], settings.RecoveryCodes[i+1:]...)
				consumed = true
				break
			}
		}
		if !consumed {
			WriteError(w, http.StatusUnauthorized, fmt.Errorf("invalid mfa code"))
			return
		}
	}

	if !settings.Enabled {
		settings.Enabled = true
		settings.EnabledAt = &now
	}
	settings.LastUsedAt = &now

	updated, err := h.Store.UpsertMFASettings(settings)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	if tokenUsed {
		if err := h.mfaChallengeManager().Revoke(req.Token); err != nil {
			WriteError(w, http.StatusInternalServerError, err)
			return
		}
		token, expiresAt, err := h.sessionManager().Create(userID)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, err)
			return
		}
		user, ok := h.Store.GetUser(userID)
		if !ok {
			WriteError(w, http.StatusUnauthorized, fmt.Errorf("account not found"))
			return
		}
		h.setSessionCookie(w, r, token, expiresAt)
		WriteJSON(w, http.StatusOK, newAuthResponse(user, expiresAt))
		return
	}

	WriteJSON(w, http.StatusOK, buildMFAStatus(updated, true))
}

// MFADisable performs mfadisable and returns an error when dependent systems reject the operation.
func (h *Handler) MFADisable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteMethodNotAllowed(w, r, http.MethodPost)
		return
	}
	var req mfaDisableRequest
	if !DecodeAndValidate(w, r, &req) {
		return
	}
	user, ok := h.requireSessionUser(w, r)
	if !ok {
		return
	}
	settings, exists, err := h.Store.GetMFASettings(user.ID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}
	if !exists || !settings.Enabled {
		WriteError(w, http.StatusBadRequest, fmt.Errorf("mfa not enabled"))
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		WriteRequestError(w, ValidationError("code is required"))
		return
	}
	verified, err := auth.VerifyTOTP(settings.Secret, req.Code, time.Now())
	if err != nil {
		WriteRequestError(w, ValidationError(err.Error()))
		return
	}
	if !verified {
		consumed := false
		for _, hash := range settings.RecoveryCodes {
			if auth.MatchRecoveryCode(hash, req.Code) {
				consumed = true
				break
			}
		}
		if !consumed {
			WriteError(w, http.StatusUnauthorized, fmt.Errorf("invalid mfa code"))
			return
		}
	}
	if err := h.Store.DeleteMFASettings(user.ID); err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}
	WriteJSON(w, http.StatusOK, mfaStatusResponse{Enabled: false, Pending: false})
}

// resolveMFAUser resolves mfauser from flags and environment values, returning validation errors when incompatible settings are provided.
func (h *Handler) resolveMFAUser(token string, r *http.Request) (models.User, error) {
	if strings.TrimSpace(token) != "" {
		userID, _, ok, err := h.mfaChallengeManager().Validate(token)
		if err != nil {
			return models.User{}, err
		}
		if !ok {
			return models.User{}, auth.ErrInvalidMFAChallenge
		}
		user, ok := h.Store.GetUser(userID)
		if !ok {
			return models.User{}, fmt.Errorf("account not found")
		}
		return user, nil
	}
	user, _, err := h.AuthenticateRequest(r)
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}

// resolveMFAChallenge resolves mfachallenge from flags and environment values, returning validation errors when incompatible settings are provided.
func (h *Handler) resolveMFAChallenge(token string, r *http.Request) (string, time.Time, bool, error) {
	if strings.TrimSpace(token) != "" {
		userID, expiresAt, ok, err := h.mfaChallengeManager().Validate(token)
		if err != nil {
			return "", time.Time{}, true, err
		}
		if !ok {
			return "", time.Time{}, true, auth.ErrInvalidMFAChallenge
		}
		return userID, expiresAt, true, nil
	}
	user, _, err := h.AuthenticateRequest(r)
	if err != nil {
		return "", time.Time{}, false, err
	}
	return user.ID, time.Time{}, false, nil
}

// prepareMFAEnrollment performs prepare mfaenrollment and propagates validation or dependency failures to the caller.
func (h *Handler) prepareMFAEnrollment(user models.User) (mfaEnrollmentResponse, models.MFASettings, error) {
	secret, err := auth.GenerateMFASecret()
	if err != nil {
		return mfaEnrollmentResponse{}, models.MFASettings{}, err
	}
	codes, err := auth.GenerateRecoveryCodes(8)
	if err != nil {
		return mfaEnrollmentResponse{}, models.MFASettings{}, err
	}
	hashedCodes := make([]string, 0, len(codes))
	for _, code := range codes {
		hashed, err := auth.HashRecoveryCode(code)
		if err != nil {
			return mfaEnrollmentResponse{}, models.MFASettings{}, err
		}
		hashedCodes = append(hashedCodes, hashed)
	}
	accountLabel := user.Email
	if strings.TrimSpace(accountLabel) == "" {
		accountLabel = user.ID
	}
	otpURL, err := auth.BuildOTPAuthURL(mfaIssuer, accountLabel, secret)
	if err != nil {
		return mfaEnrollmentResponse{}, models.MFASettings{}, err
	}
	enrollment := mfaEnrollmentResponse{
		Secret:        secret,
		OTPAuthURL:    otpURL,
		RecoveryCodes: codes,
	}
	settings := models.MFASettings{
		UserID:        user.ID,
		Secret:        secret,
		RecoveryCodes: hashedCodes,
		Enabled:       false,
	}
	return enrollment, settings, nil
}

// buildMFAStatus builds mfastatus from runtime state used by downstream handlers.
func buildMFAStatus(settings models.MFASettings, exists bool) mfaStatusResponse {
	if !exists {
		return mfaStatusResponse{Enabled: false, Pending: false}
	}
	response := mfaStatusResponse{
		Enabled:                settings.Enabled,
		Pending:                !settings.Enabled,
		RecoveryCodesRemaining: len(settings.RecoveryCodes),
	}
	if settings.EnabledAt != nil {
		enabledAt := settings.EnabledAt.Format(time.RFC3339Nano)
		response.EnrolledAt = &enabledAt
	}
	if settings.LastUsedAt != nil {
		lastUsed := settings.LastUsedAt.Format(time.RFC3339Nano)
		response.LastUsedAt = &lastUsed
	}
	return response
}

// mfaRequirement performs mfa requirement and propagates validation or dependency failures to the caller.
func (h *Handler) mfaRequirement(user models.User) (models.MFASettings, bool, bool, error) {
	settings, exists, err := h.Store.GetMFASettings(user.ID)
	if err != nil {
		return models.MFASettings{}, false, false, err
	}
	requires := (exists && settings.Enabled) || userHasAnyRole(user, roleAdmin, roleCreator)
	return settings, exists, requires, nil
}
