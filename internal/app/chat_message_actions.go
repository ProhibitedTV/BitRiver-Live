package app

import (
	"fmt"
	"strings"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/domain"
	"bitriver-live/internal/service"
)

type chatDeleteBroadcaster interface {
	BroadcastMessageDelete(channelID, messageID, actorID string) bool
}

type chatMessageActionAdapter struct {
	service.ChatModerationUseCase
	broadcaster chatDeleteBroadcaster
}

func newChatMessageActionAdapter(base service.ChatModerationUseCase, broadcaster chatDeleteBroadcaster) service.ChatModerationUseCase {
	if base == nil {
		return nil
	}
	return &chatMessageActionAdapter{ChatModerationUseCase: base, broadcaster: broadcaster}
}

func (a *chatMessageActionAdapter) ListChatMessages(channelID string, limit int) ([]domain.ChatMessage, error) {
	messages, err := a.ChatModerationUseCase.ListChatMessages(channelID, limit)
	if err != nil {
		return nil, err
	}
	for index := range messages {
		messages[index] = chatMessageForViewer(messages[index])
	}
	return messages, nil
}

func (a *chatMessageActionAdapter) CreateChatMessage(channelID, userID, content string) (domain.ChatMessage, error) {
	message, err := a.ChatModerationUseCase.CreateChatMessage(channelID, userID, content)
	if err != nil {
		return domain.ChatMessage{}, err
	}
	return chatMessageForViewer(message), nil
}

func (a *chatMessageActionAdapter) DeleteChatMessage(channelID, messageID string) error {
	channelID = strings.TrimSpace(channelID)
	messageID = strings.TrimSpace(messageID)
	if channelID == "" || messageID == "" {
		return fmt.Errorf("channel id and message id are required")
	}

	messages, err := a.ChatModerationUseCase.ListChatMessages(channelID, 0)
	if err != nil {
		return err
	}
	found := false
	for _, message := range messages {
		if message.ID == messageID && message.ChannelID == channelID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("chat message %s not found in channel %s", messageID, channelID)
	}

	if err := a.ChatModerationUseCase.DeleteChatMessage(channelID, messageID); err != nil {
		return err
	}
	if a.broadcaster != nil {
		a.broadcaster.BroadcastMessageDelete(channelID, messageID, "")
	}
	return nil
}

func chatMessageForViewer(message domain.ChatMessage) domain.ChatMessage {
	kind, _ := chat.ClassifyMessageContent(message.Content)
	if kind != chat.MessageKindAction {
		message.Content = strings.TrimSpace(message.Content)
		return message
	}
	message.Content = chat.ActionDisplayContent(message.Content, message.UserID)
	return message
}
