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

func (h *Handler) ModerationQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	if _, ok := h.requireRole(w, r, roleAdmin); !ok {
		return
	}

	payload, err := h.moderationQueuePayload()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}
	WriteJSON(w, http.StatusOK, payload)
}

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
