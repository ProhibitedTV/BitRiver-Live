package app

import (
	"errors"
	"testing"
	"time"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/service"
)

type chatModerationActionStub struct {
	service.ChatModerationUseCase
	messages  []domain.ChatMessage
	deleteErr error
	deleted   []string
}

func (s *chatModerationActionStub) ListChatMessages(channelID string, limit int) ([]domain.ChatMessage, error) {
	out := make([]domain.ChatMessage, 0, len(s.messages))
	for _, message := range s.messages {
		if message.ChannelID == channelID {
			out = append(out, message)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *chatModerationActionStub) CreateChatMessage(channelID, userID, content string) (domain.ChatMessage, error) {
	message := domain.ChatMessage{ID: "created", ChannelID: channelID, UserID: userID, Content: content, CreatedAt: time.Now().UTC()}
	s.messages = append(s.messages, message)
	return message, nil
}

func (s *chatModerationActionStub) DeleteChatMessage(channelID, messageID string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = append(s.deleted, channelID+":"+messageID)
	return nil
}

type deleteBroadcastStub struct {
	calls []string
}

func (s *deleteBroadcastStub) BroadcastMessageDelete(channelID, messageID, actorID string) bool {
	s.calls = append(s.calls, channelID+":"+messageID+":"+actorID)
	return true
}

func TestChatMessageActionAdapterFormatsActionHistory(t *testing.T) {
	base := &chatModerationActionStub{messages: []domain.ChatMessage{
		{ID: "plain", ChannelID: "channel-1", UserID: "alice", Content: " hello "},
		{ID: "action", ChannelID: "channel-1", UserID: "alice", Content: "/me waves hello"},
	}}
	adapter := newChatMessageActionAdapter(base, nil)

	messages, err := adapter.ListChatMessages("channel-1", 0)
	if err != nil {
		t.Fatalf("list chat messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Content != "hello" {
		t.Fatalf("unexpected plain content %q", messages[0].Content)
	}
	if messages[1].Content != "* alice waves hello" {
		t.Fatalf("unexpected action display content %q", messages[1].Content)
	}
	if base.messages[1].Content != "/me waves hello" {
		t.Fatalf("history formatting mutated canonical stored content: %q", base.messages[1].Content)
	}
}

func TestChatMessageActionAdapterDeletePersistsBeforeBroadcast(t *testing.T) {
	base := &chatModerationActionStub{messages: []domain.ChatMessage{{ID: "message-1", ChannelID: "channel-1", UserID: "alice", Content: "hello"}}}
	broadcaster := &deleteBroadcastStub{}
	adapter := newChatMessageActionAdapter(base, broadcaster)

	if err := adapter.DeleteChatMessage("channel-1", "message-1"); err != nil {
		t.Fatalf("delete chat message: %v", err)
	}
	if len(base.deleted) != 1 || base.deleted[0] != "channel-1:message-1" {
		t.Fatalf("expected persistence delete before fanout, got %#v", base.deleted)
	}
	if len(broadcaster.calls) != 1 || broadcaster.calls[0] != "channel-1:message-1:" {
		t.Fatalf("expected one delete broadcast, got %#v", broadcaster.calls)
	}
}

func TestChatMessageActionAdapterDeleteRejectsMissingOrWrongChannel(t *testing.T) {
	base := &chatModerationActionStub{messages: []domain.ChatMessage{{ID: "message-1", ChannelID: "channel-1", UserID: "alice", Content: "hello"}}}
	broadcaster := &deleteBroadcastStub{}
	adapter := newChatMessageActionAdapter(base, broadcaster)

	if err := adapter.DeleteChatMessage("channel-1", "missing"); err == nil {
		t.Fatal("expected missing message deletion to fail")
	}
	if err := adapter.DeleteChatMessage("channel-2", "message-1"); err == nil {
		t.Fatal("expected wrong-channel deletion to fail")
	}
	if len(base.deleted) != 0 || len(broadcaster.calls) != 0 {
		t.Fatalf("failed preflight must not delete or broadcast: deletes=%#v broadcasts=%#v", base.deleted, broadcaster.calls)
	}
}

func TestChatMessageActionAdapterDoesNotBroadcastFailedPersistence(t *testing.T) {
	base := &chatModerationActionStub{
		messages:  []domain.ChatMessage{{ID: "message-1", ChannelID: "channel-1", UserID: "alice", Content: "hello"}},
		deleteErr: errors.New("database unavailable"),
	}
	broadcaster := &deleteBroadcastStub{}
	adapter := newChatMessageActionAdapter(base, broadcaster)

	if err := adapter.DeleteChatMessage("channel-1", "message-1"); err == nil {
		t.Fatal("expected persistence failure")
	}
	if len(broadcaster.calls) != 0 {
		t.Fatalf("persistence failure must not broadcast deletion: %#v", broadcaster.calls)
	}
}
