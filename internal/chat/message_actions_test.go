package chat

import (
	"encoding/json"
	"testing"
	"time"
)

func TestClassifyMessageContent(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		kind    MessageKind
		content string
	}{
		{name: "plain", input: " hello ", kind: MessageKindMessage, content: "hello"},
		{name: "action", input: " /me waves hello ", kind: MessageKindAction, content: "waves hello"},
		{name: "case insensitive action", input: "/ME dances", kind: MessageKindAction, content: "dances"},
		{name: "empty action remains plain", input: "/me ", kind: MessageKindMessage, content: "/me"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, content := ClassifyMessageContent(tt.input)
			if kind != tt.kind || content != tt.content {
				t.Fatalf("ClassifyMessageContent(%q) = (%q, %q), want (%q, %q)", tt.input, kind, content, tt.kind, tt.content)
			}
		})
	}
}

func TestMessageEventMarshalJSONExposesActionKindAndPlainText(t *testing.T) {
	created := time.Date(2026, time.September, 13, 2, 0, 0, 0, time.UTC)
	event := MessageEvent{
		ID:        "msg-1",
		ChannelID: "channel-1",
		UserID:    "user-1",
		User:      &UserMetadata{ID: "user-1", DisplayName: "Alice", Role: "viewer"},
		Content:   "/me waves <b>hello</b>",
		CreatedAt: created,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal message event: %v", err)
	}
	var decoded struct {
		Kind    MessageKind `json:"kind"`
		Content string      `json:"content"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode message event: %v", err)
	}
	if decoded.Kind != MessageKindAction {
		t.Fatalf("expected action kind, got %q", decoded.Kind)
	}
	if decoded.Content != "* Alice waves <b>hello</b>" {
		t.Fatalf("expected plain action display content, got %q", decoded.Content)
	}
	if event.Content != "/me waves <b>hello</b>" {
		t.Fatalf("marshal mutated canonical persisted content: %q", event.Content)
	}
}

func TestMessageEventMarshalJSONDefaultsToMessageKind(t *testing.T) {
	payload, err := json.Marshal(MessageEvent{ID: "msg-1", ChannelID: "channel-1", UserID: "user-1", Content: "hello"})
	if err != nil {
		t.Fatalf("marshal message event: %v", err)
	}
	var decoded struct {
		Kind    MessageKind `json:"kind"`
		Content string      `json:"content"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode message event: %v", err)
	}
	if decoded.Kind != MessageKindMessage || decoded.Content != "hello" {
		t.Fatalf("unexpected ordinary message wire form: kind=%q content=%q", decoded.Kind, decoded.Content)
	}
}

func TestBroadcastMessageDeleteRejectsMissingIdentifiers(t *testing.T) {
	gateway := NewGateway(GatewayConfig{})
	if gateway.BroadcastMessageDelete("", "message-1", "actor-1") {
		t.Fatal("expected missing channel id to be rejected")
	}
	if gateway.BroadcastMessageDelete("channel-1", "", "actor-1") {
		t.Fatal("expected missing message id to be rejected")
	}
}
