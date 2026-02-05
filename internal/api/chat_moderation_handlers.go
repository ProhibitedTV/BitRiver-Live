package api

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/models"
	"bitriver-live/internal/storage"
)

// Chat request/response DTOs.
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

type chatFilterRequest struct {
	Kind    string `json:"kind"`
	Pattern string `json:"pattern"`
	Enabled bool   `json:"enabled"`
}

type chatFilterUpdateRequest struct {
	Kind    *string `json:"kind,omitempty"`
	Pattern *string `json:"pattern,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
}

type chatFilterResponse struct {
	ID        string `json:"id"`
	ChannelID string `json:"channelId"`
	Kind      string `json:"kind"`
	Pattern   string `json:"pattern"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
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

type moderationAutoModResponse struct {
	ID            string                  `json:"id"`
	ChannelID     string                  `json:"channelId"`
	ChannelTitle  string                  `json:"channelTitle,omitempty"`
	UserID        string                  `json:"userId,omitempty"`
	User          *moderationUserResponse `json:"user,omitempty"`
	FilterID      string                  `json:"filterId,omitempty"`
	FilterKind    string                  `json:"filterKind,omitempty"`
	FilterPattern string                  `json:"filterPattern,omitempty"`
	Message       string                  `json:"message,omitempty"`
	Action        string                  `json:"action,omitempty"`
	CreatedAt     string                  `json:"createdAt,omitempty"`
}

type moderationQueueResponse struct {
	Queue       []moderationFlagResponse   `json:"queue"`
	Actions     []moderationActionResponse `json:"actions"`
	QueueMeta   moderationPageInfo         `json:"queueMeta"`
	ActionsMeta moderationPageInfo         `json:"actionsMeta"`
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

type moderationPageInfo struct {
	NextCursor string `json:"nextCursor,omitempty"`
	Limit      int    `json:"limit"`
	HasMore    bool   `json:"hasMore"`
}

type moderationAutoModPageResponse struct {
	Actions []moderationAutoModResponse `json:"actions"`
	Meta    moderationPageInfo          `json:"meta"`
}

// newChatMessageResponse builds and returns chat message response using the supplied dependencies.
func newChatMessageResponse(message models.ChatMessage) chatMessageResponse {
	return chatMessageResponse{
		ID:        message.ID,
		ChannelID: message.ChannelID,
		UserID:    message.UserID,
		Content:   message.Content,
		CreatedAt: message.CreatedAt.Format(time.RFC3339Nano),
	}
}

// newChatRestrictionResponse builds and returns chat restriction response using the supplied dependencies.
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

// newChatFilterResponse builds and returns chat filter response using the supplied dependencies.
func newChatFilterResponse(filter models.ChatFilter) chatFilterResponse {
	return chatFilterResponse{
		ID:        filter.ID,
		ChannelID: filter.ChannelID,
		Kind:      filter.Kind,
		Pattern:   filter.Pattern,
		Enabled:   filter.Enabled,
		CreatedAt: filter.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt: filter.UpdatedAt.Format(time.RFC3339Nano),
	}
}

// newChatReportResponse builds and returns chat report response using the supplied dependencies.
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

// newModerationUser builds and returns moderation user using the supplied dependencies.
func newModerationUser(user models.User) moderationUserResponse {
	resp := moderationUserResponse{ID: user.ID}
	if user.DisplayName != "" {
		resp.DisplayName = user.DisplayName
	}
	return resp
}

// ChatWebsocket performs chat websocket and returns an error when dependent systems reject the operation.
func (h *Handler) ChatWebsocket(w http.ResponseWriter, r *http.Request) {
	if h.ChatGateway == nil {
		WriteRequestError(w, ServiceUnavailableError("chat gateway unavailable"))
		return
	}
	user, ok := h.requireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	h.ChatGateway.HandleConnection(w, r, user)
}

// handleChatRoutes routes and serves chat routes requests, writing HTTP errors for invalid input or backend failures.
func (h *Handler) handleChatRoutes(channelID string, remaining []string, w http.ResponseWriter, r *http.Request) {
	channel, exists := h.Store.GetChannel(channelID)
	if !exists {
		WriteError(w, http.StatusNotFound, fmt.Errorf("channel %s not found", channelID))
		return
	}

	if len(remaining) > 0 && remaining[0] != "" {
		switch remaining[0] {
		case "moderation":
			actor, ok := h.requireAuthenticatedUser(w, r)
			if !ok {
				return
			}
			h.handleChatModeration(actor, channel, remaining[1:], w, r)
			return
		case "reports":
			actor, ok := h.requireAuthenticatedUser(w, r)
			if !ok {
				return
			}
			h.handleChatReports(actor, channel, remaining[1:], w, r)
			return
		default:
			messageID := remaining[0]
			if len(remaining) > 1 {
				WriteError(w, http.StatusNotFound, fmt.Errorf("unknown chat path"))
				return
			}
			if r.Method != http.MethodDelete {
				WriteMethodNotAllowed(w, r, http.MethodDelete)
				return
			}
			actor, ok := h.requireAuthenticatedUser(w, r)
			if !ok {
				return
			}
			if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
				WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
				return
			}
			if err := h.Store.DeleteChatMessage(channelID, messageID); err != nil {
				WriteError(w, http.StatusBadRequest, err)
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
				WriteRequestError(w, ValidationError("invalid limit value"))
				return
			}
			limit = parsed
		}
		messages, err := h.Store.ListChatMessages(channelID, limit)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		response := make([]chatMessageResponse, 0, len(messages))
		for _, message := range messages {
			response = append(response, newChatMessageResponse(message))
		}
		WriteJSON(w, http.StatusOK, response)
	case http.MethodPost:
		actor, ok := h.requireAuthenticatedUser(w, r)
		if !ok {
			return
		}
		var req createChatRequest
		if !DecodeAndValidate(w, r, &req) {
			return
		}
		if req.UserID != actor.ID && !actor.HasRole(roleAdmin) {
			WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
			return
		}
		if h.ChatGateway != nil {
			author, ok := h.Store.GetUser(req.UserID)
			if !ok {
				WriteRequestError(w, ValidationError(fmt.Sprintf("user %s not found", req.UserID)))
				return
			}
			messageEvt, err := h.ChatGateway.CreateMessage(r.Context(), author, channelID, req.Content)
			if err != nil {
				WriteError(w, http.StatusBadRequest, err)
				return
			}
			chatMessage := models.ChatMessage{
				ID:        messageEvt.ID,
				ChannelID: messageEvt.ChannelID,
				UserID:    messageEvt.UserID,
				Content:   messageEvt.Content,
				CreatedAt: messageEvt.CreatedAt,
			}
			WriteJSON(w, http.StatusCreated, newChatMessageResponse(chatMessage))
			return
		}
		message, err := h.Store.CreateChatMessage(channelID, req.UserID, req.Content)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		WriteJSON(w, http.StatusCreated, newChatMessageResponse(message))
	default:
		WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPost)
	}
}

// handleChatModeration routes and serves chat moderation requests, writing HTTP errors for invalid input or backend failures.
func (h *Handler) handleChatModeration(actor models.User, channel models.Channel, remaining []string, w http.ResponseWriter, r *http.Request) {
	if h.ChatGateway == nil {
		WriteRequestError(w, ServiceUnavailableError("chat gateway unavailable"))
		return
	}
	if len(remaining) > 0 {
		switch remaining[0] {
		case "restrictions":
			if r.Method != http.MethodGet {
				WriteMethodNotAllowed(w, r, http.MethodGet)
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
			WriteJSON(w, http.StatusOK, response)
			return
		case "filters":
			h.handleChatFilters(actor, channel, remaining[1:], w, r)
			return
		case "reports":
			h.handleChatReports(actor, channel, remaining[1:], w, r)
			return
		}
	}
	if len(remaining) > 0 {
		WriteError(w, http.StatusNotFound, fmt.Errorf("unknown chat moderation path"))
		return
	}
	if r.Method != http.MethodPost {
		WriteMethodNotAllowed(w, r, http.MethodPost)
		return
	}
	if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
		WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
		return
	}
	var req chatModerationRequest
	if !DecodeAndValidate(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.TargetID) == "" {
		WriteRequestError(w, ValidationError("targetId is required"))
		return
	}
	if _, ok := h.Store.GetUser(req.TargetID); !ok {
		WriteRequestError(w, ValidationError(fmt.Sprintf("user %s not found", req.TargetID)))
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
			WriteRequestError(w, ValidationError("durationMs must be positive"))
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
		WriteRequestError(w, ValidationError("unknown moderation action"))
		return
	}

	if err := h.ChatGateway.ApplyModeration(r.Context(), actor, evt); err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return
	}
	var expires *string
	if evt.ExpiresAt != nil {
		formatted := evt.ExpiresAt.Format(time.RFC3339Nano)
		expires = &formatted
	}
	WriteJSON(w, http.StatusAccepted, chatModerationResponse{
		Action:    string(evt.Action),
		ChannelID: evt.ChannelID,
		TargetID:  evt.TargetID,
		ExpiresAt: expires,
	})
}

// handleChatFilters routes and serves chat filters requests, writing HTTP errors for invalid input or backend failures.
func (h *Handler) handleChatFilters(actor models.User, channel models.Channel, remaining []string, w http.ResponseWriter, r *http.Request) {
	if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
		WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
		return
	}

	if len(remaining) > 0 && strings.TrimSpace(remaining[0]) != "" {
		filterID := remaining[0]
		switch r.Method {
		case http.MethodPatch:
			var req chatFilterUpdateRequest
			if !DecodeAndValidate(w, r, &req) {
				return
			}
			if req.Kind == nil && req.Pattern == nil && req.Enabled == nil {
				WriteRequestError(w, ValidationError("at least one field is required"))
				return
			}
			update := storage.ChatFilterUpdate{
				Kind:    req.Kind,
				Pattern: req.Pattern,
				Enabled: req.Enabled,
			}
			filter, err := h.Store.UpdateChatFilter(filterID, update)
			if err != nil {
				WriteError(w, http.StatusBadRequest, err)
				return
			}
			WriteJSON(w, http.StatusOK, newChatFilterResponse(filter))
			return
		case http.MethodDelete:
			if err := h.Store.DeleteChatFilter(filterID); err != nil {
				WriteError(w, http.StatusBadRequest, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			WriteMethodNotAllowed(w, r, http.MethodPatch, http.MethodDelete)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		filters, err := h.Store.ListChatFilters(channel.ID)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		response := make([]chatFilterResponse, 0, len(filters))
		for _, filter := range filters {
			response = append(response, newChatFilterResponse(filter))
		}
		WriteJSON(w, http.StatusOK, response)
	case http.MethodPost:
		var req chatFilterRequest
		if !DecodeAndValidate(w, r, &req) {
			return
		}
		filter, err := h.Store.CreateChatFilter(channel.ID, storage.ChatFilterParams{
			Kind:    req.Kind,
			Pattern: req.Pattern,
			Enabled: req.Enabled,
		})
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		WriteJSON(w, http.StatusCreated, newChatFilterResponse(filter))
	default:
		WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPost)
	}
}

// handleChatReports routes and serves chat reports requests, writing HTTP errors for invalid input or backend failures.
func (h *Handler) handleChatReports(actor models.User, channel models.Channel, remaining []string, w http.ResponseWriter, r *http.Request) {
	if len(remaining) > 0 && strings.TrimSpace(remaining[0]) != "" {
		reportID := remaining[0]
		if len(remaining) == 2 && remaining[1] == "resolve" {
			if r.Method != http.MethodPost {
				WriteMethodNotAllowed(w, r, http.MethodPost)
				return
			}
			if channel.OwnerID != actor.ID && !actor.HasRole(roleAdmin) {
				WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden"))
				return
			}
			var req resolveChatReportRequest
			if !DecodeAndValidate(w, r, &req) {
				return
			}
			report, err := h.Store.ResolveChatReport(reportID, actor.ID, req.Resolution)
			if err != nil {
				WriteError(w, http.StatusBadRequest, err)
				return
			}
			WriteJSON(w, http.StatusOK, newChatReportResponse(report))
			return
		}
		WriteError(w, http.StatusNotFound, fmt.Errorf("unknown chat report path"))
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
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		response := make([]chatReportResponse, 0, len(reports))
		for _, report := range reports {
			response = append(response, newChatReportResponse(report))
		}
		WriteJSON(w, http.StatusOK, response)
	case http.MethodPost:
		var req chatReportRequest
		if !DecodeAndValidate(w, r, &req) {
			return
		}
		targetID := strings.TrimSpace(req.TargetID)
		if targetID == "" {
			WriteRequestError(w, ValidationError("targetId is required"))
			return
		}
		if _, ok := h.Store.GetUser(targetID); !ok {
			WriteRequestError(w, ValidationError(fmt.Sprintf("user %s not found", targetID)))
			return
		}
		reason := strings.TrimSpace(req.Reason)
		if reason == "" {
			WriteRequestError(w, ValidationError("reason is required"))
			return
		}
		messageID := strings.TrimSpace(req.MessageID)
		evidence := strings.TrimSpace(req.EvidenceURL)
		if h.ChatGateway != nil {
			reporter, ok := h.Store.GetUser(actor.ID)
			if !ok {
				WriteError(w, http.StatusInternalServerError, fmt.Errorf("reporter %s not found", actor.ID))
				return
			}
			evt, err := h.ChatGateway.SubmitReport(r.Context(), reporter, channel.ID, targetID, reason, messageID, evidence)
			if err != nil {
				WriteError(w, http.StatusBadRequest, err)
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
			WriteJSON(w, http.StatusAccepted, newChatReportResponse(report))
			return
		}
		report, err := h.Store.CreateChatReport(channel.ID, actor.ID, targetID, reason, messageID, evidence)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}
		WriteJSON(w, http.StatusAccepted, newChatReportResponse(report))
	default:
		WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPost)
	}
}

// ModerationQueue performs moderation queue and returns an error when dependent systems reject the operation.
func (h *Handler) ModerationQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	if _, ok := h.requireRole(w, r, roleAdmin); !ok {
		return
	}

	query := r.URL.Query()
	queueCursor, err := parseModerationCursor(query.Get("cursor"))
	if err != nil {
		WriteRequestError(w, ValidationError("cursor must be RFC3339"))
		return
	}
	queueLimit, err := parseModerationLimit(query.Get("limit"), 50)
	if err != nil {
		WriteRequestError(w, ValidationError("limit must be a positive integer"))
		return
	}
	actionsCursor, err := parseModerationCursor(query.Get("actionsCursor"))
	if err != nil {
		WriteRequestError(w, ValidationError("actionsCursor must be RFC3339"))
		return
	}
	actionsLimit, err := parseModerationLimit(query.Get("actionsLimit"), 20)
	if err != nil {
		WriteRequestError(w, ValidationError("actionsLimit must be a positive integer"))
		return
	}

	payload, err := h.moderationQueuePayload(queueCursor, queueLimit, actionsCursor, actionsLimit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}
	WriteJSON(w, http.StatusOK, payload)
}

// ModerationQueueByID performs moderation queue by id and returns an error when dependent systems reject the operation.
func (h *Handler) ModerationQueueByID(w http.ResponseWriter, r *http.Request) {
	flagID := strings.TrimPrefix(r.URL.Path, "/api/moderation/queue/")
	if flagID == "" {
		WriteError(w, http.StatusNotFound, fmt.Errorf("flag id missing"))
		return
	}

	if r.Method != http.MethodPost {
		WriteMethodNotAllowed(w, r, http.MethodPost)
		return
	}

	actor, ok := h.requireRole(w, r, roleAdmin)
	if !ok {
		return
	}

	var req resolveModerationRequest
	if !DecodeAndValidate(w, r, &req) {
		return
	}
	resolution := strings.TrimSpace(req.Resolution)
	if resolution == "" {
		WriteRequestError(w, ValidationError("resolution is required"))
		return
	}

	report, err := h.Store.ResolveChatReport(flagID, actor.ID, resolution)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return
	}
	WriteJSON(w, http.StatusOK, newChatReportResponse(report))
}

// ModerationAutoMod performs moderation auto mod and returns an error when dependent systems reject the operation.
func (h *Handler) ModerationAutoMod(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	if _, ok := h.requireRole(w, r, roleAdmin); !ok {
		return
	}

	query := r.URL.Query()
	cursor, err := parseModerationCursor(query.Get("cursor"))
	if err != nil {
		WriteRequestError(w, ValidationError("cursor must be RFC3339"))
		return
	}
	limit, err := parseModerationLimit(query.Get("limit"), 50)
	if err != nil {
		WriteRequestError(w, ValidationError("limit must be a positive integer"))
		return
	}

	payload, err := h.moderationAutoModPayload(cursor, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}
	WriteJSON(w, http.StatusOK, payload)
}

// parseModerationCursor parses moderation cursor and returns an error when the input is malformed.
func parseModerationCursor(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

// parseModerationLimit parses moderation limit and returns an error when the input is malformed.
func parseModerationLimit(value string, defaultLimit int) (int, error) {
	if strings.TrimSpace(value) == "" {
		return defaultLimit, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("invalid limit")
	}
	return parsed, nil
}

type moderationTimedItem[T any] struct {
	payload T
	created time.Time
}

// paginateModerationItems slices moderation items by cursor and limit while preserving descending timestamp order.
// It returns page metadata that callers use to expose stable next-page cursors.
func paginateModerationItems[T any](items []moderationTimedItem[T], cursor *time.Time, limit int) ([]T, moderationPageInfo) {
	filtered := items
	if cursor != nil {
		filtered = make([]moderationTimedItem[T], 0, len(items))
		for _, item := range items {
			if item.created.Before(*cursor) {
				filtered = append(filtered, item)
			}
		}
	}
	pageSize := limit
	if pageSize > len(filtered) {
		pageSize = len(filtered)
	}
	hasMore := len(filtered) > pageSize
	result := make([]T, pageSize)
	for i := 0; i < pageSize; i++ {
		result[i] = filtered[i].payload
	}
	nextCursor := ""
	if pageSize > 0 {
		nextCursor = filtered[pageSize-1].created.Format(time.RFC3339Nano)
	}
	return result, moderationPageInfo{
		NextCursor: nextCursor,
		Limit:      limit,
		HasMore:    hasMore,
	}
}

// moderationQueuePayload performs moderation queue payload and propagates validation or dependency failures to the caller.
func (h *Handler) moderationQueuePayload(queueCursor *time.Time, queueLimit int, actionsCursor *time.Time, actionsLimit int) (moderationQueueResponse, error) {
	channels := h.Store.ListChannels("", "")
	flags := make([]moderationTimedItem[moderationFlagResponse], 0)
	actions := make([]moderationTimedItem[moderationActionResponse], 0)
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
				flags = append(flags, moderationTimedItem[moderationFlagResponse]{payload: flag, created: createdAt})
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
				actions = append(actions, moderationTimedItem[moderationActionResponse]{payload: action, created: resolvedAt})
			}
		}
	}
	sort.Slice(flags, func(i, j int) bool {
		return flags[i].created.After(flags[j].created)
	})
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].created.After(actions[j].created)
	})
	queue, queueMeta := paginateModerationItems(flags, queueCursor, queueLimit)
	resolved, actionsMeta := paginateModerationItems(actions, actionsCursor, actionsLimit)
	return moderationQueueResponse{
		Queue:       queue,
		Actions:     resolved,
		QueueMeta:   queueMeta,
		ActionsMeta: actionsMeta,
	}, nil
}

// moderationAutoModPayload performs moderation auto mod payload and propagates validation or dependency failures to the caller.
func (h *Handler) moderationAutoModPayload(cursor *time.Time, limit int) (moderationAutoModPageResponse, error) {
	channels := h.Store.ListChannels("", "")
	actions := make([]moderationTimedItem[moderationAutoModResponse], 0)
	for _, channel := range channels {
		items, err := h.Store.ListChatAutoModActions(channel.ID, 0)
		if err != nil {
			return moderationAutoModPageResponse{}, err
		}
		for _, item := range items {
			createdAt := item.CreatedAt
			resp := moderationAutoModResponse{
				ID:            item.ID,
				ChannelID:     item.ChannelID,
				ChannelTitle:  channel.Title,
				UserID:        item.UserID,
				FilterID:      item.FilterID,
				FilterKind:    item.FilterKind,
				FilterPattern: item.FilterPattern,
				Message:       item.Message,
				Action:        item.Action,
				CreatedAt:     createdAt.Format(time.RFC3339Nano),
			}
			if user, ok := h.Store.GetUser(item.UserID); ok {
				userResp := newModerationUser(user)
				resp.User = &userResp
			}
			actions = append(actions, moderationTimedItem[moderationAutoModResponse]{payload: resp, created: createdAt})
		}
	}
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].created.After(actions[j].created)
	})
	output, meta := paginateModerationItems(actions, cursor, limit)
	return moderationAutoModPageResponse{
		Actions: output,
		Meta:    meta,
	}, nil
}
