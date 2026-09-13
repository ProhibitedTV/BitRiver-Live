package chat

import (
	"encoding/json"
	"strings"
	"time"
)

const actionPrefix = "/me "

// ClassifyMessageContent converts the canonical stored chat source into the
// semantic wire representation. Action messages intentionally remain stored as
// `/me <text>` so existing persistence backends and older readers stay valid.
func ClassifyMessageContent(content string) (MessageKind, string) {
	trimmed := strings.TrimSpace(content)
	if len(trimmed) > len(actionPrefix) && strings.EqualFold(trimmed[:len(actionPrefix)], actionPrefix) {
		body := strings.TrimSpace(trimmed[len(actionPrefix):])
		if body != "" {
			return MessageKindAction, body
		}
	}
	return MessageKindMessage, trimmed
}

// ActionDisplayContent returns a plain-text display form for REST/history
// clients that do not yet consume the additive message kind field.
func ActionDisplayContent(content, author string) string {
	kind, body := ClassifyMessageContent(content)
	if kind != MessageKindAction {
		return body
	}
	label := strings.TrimSpace(author)
	if label == "" {
		return "* " + body
	}
	return "* " + label + " " + body
}

// MarshalJSON exposes additive kind semantics without changing the canonical
// MessageEvent value consumed by the persistence queue. Action content remains
// `/me ...` internally but is emitted as plain action body text on the wire.
func (m MessageEvent) MarshalJSON() ([]byte, error) {
	kind, body := ClassifyMessageContent(m.Content)
	if kind == MessageKindAction {
		author := m.UserID
		if m.User != nil && strings.TrimSpace(m.User.DisplayName) != "" {
			author = m.User.DisplayName
		}
		body = ActionDisplayContent(m.Content, author)
	}
	return json.Marshal(struct {
		ID        string        `json:"id"`
		ChannelID string        `json:"channelId"`
		UserID    string        `json:"userId"`
		User      *UserMetadata `json:"user,omitempty"`
		Kind      MessageKind   `json:"kind"`
		Content   string        `json:"content"`
		CreatedAt time.Time     `json:"createdAt"`
	}{
		ID:        m.ID,
		ChannelID: m.ChannelID,
		UserID:    m.UserID,
		User:      m.User,
		Kind:      kind,
		Content:   body,
		CreatedAt: m.CreatedAt,
	})
}

// BroadcastMessageDelete fans out a durable transcript deletion after the
// caller has successfully removed the message from the system of record. It is
// intentionally not queued for persistence because the delete already happened.
func (g *Gateway) BroadcastMessageDelete(channelID, messageID, actorID string) bool {
	if g == nil {
		return false
	}
	channelID = strings.TrimSpace(channelID)
	messageID = strings.TrimSpace(messageID)
	actorID = strings.TrimSpace(actorID)
	if channelID == "" || messageID == "" {
		return false
	}
	deletedAt := time.Now().UTC()
	g.broadcast(Event{
		Type: EventTypeMessageDelete,
		MessageDelete: &MessageDeleteEvent{
			ChannelID: channelID,
			MessageID: messageID,
			ActorID:   actorID,
			DeletedAt: deletedAt,
		},
		OccurredAt: deletedAt,
	})
	return true
}
