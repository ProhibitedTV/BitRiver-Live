package chat_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/domain"
	"bitriver-live/internal/storage"
	"bitriver-live/internal/testsupport"
)

func TestGatewayMessageFlow(t *testing.T) {
	store := newTestStorage(t)
	owner := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"admin"}})
	viewerA := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "viewer-a", Email: "viewer-a@example.com"})
	viewerB := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "viewer-b", Email: "viewer-b@example.com"})
	channel := mustCreateChannel(t, store, owner.ID, "Main")

	queue := chat.NewMemoryQueue(32)
	gateway := chat.NewGateway(chat.GatewayConfig{Queue: queue, Store: store})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	go storage.NewChatWorker(store, queue, nil).WithStartedChannel(started).Run(ctx)
	<-started

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user")
		user, ok := store.GetUser(userID)
		if !ok {
			http.Error(w, "unknown user", http.StatusUnauthorized)
			return
		}
		gateway.HandleConnection(w, r, user)
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1)
	viewerAConn := mustDial(t, wsURL+"?user="+viewerA.ID)
	defer func() {
		_ = viewerAConn.Close()
	}()
	viewerBConn := mustDial(t, wsURL+"?user="+viewerB.ID)
	defer func() {
		_ = viewerBConn.Close()
	}()

	joinPayload := map[string]string{"type": "join", "channelId": channel.ID}
	sendJSON(t, viewerAConn, joinPayload)
	waitForType(t, viewerAConn, "ack")
	sendJSON(t, viewerBConn, joinPayload)
	waitForType(t, viewerBConn, "ack")

	sendJSON(t, viewerAConn, map[string]string{
		"type":      "message",
		"channelId": channel.ID,
		"content":   "hello world",
	})

	waitForType(t, viewerAConn, "event")
	waitForType(t, viewerBConn, "event")

	testsupport.WaitUntil(t, 2*time.Second, "chat message persisted", func() bool {
		messages, err := store.ListChatMessages(channel.ID, 0)
		if err != nil {
			return false
		}
		return len(messages) == 1 && messages[0].Content == "hello world"
	})
}

func TestGatewayModerationFlow(t *testing.T) {
	store := newTestStorage(t)
	owner := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"admin"}})
	viewer := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "viewer", Email: "viewer@example.com"})
	channel := mustCreateChannel(t, store, owner.ID, "Main")

	queue := chat.NewMemoryQueue(32)
	gateway := chat.NewGateway(chat.GatewayConfig{Queue: queue, Store: store})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	go storage.NewChatWorker(store, queue, nil).WithStartedChannel(started).Run(ctx)
	<-started

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user")
		user, ok := store.GetUser(userID)
		if !ok {
			http.Error(w, "unknown user", http.StatusUnauthorized)
			return
		}
		gateway.HandleConnection(w, r, user)
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1)
	ownerConn := mustDial(t, wsURL+"?user="+owner.ID)
	defer func() {
		_ = ownerConn.Close()
	}()
	viewerConn := mustDial(t, wsURL+"?user="+viewer.ID)
	defer func() {
		_ = viewerConn.Close()
	}()

	joinPayload := map[string]string{"type": "join", "channelId": channel.ID}
	sendJSON(t, ownerConn, joinPayload)
	waitForType(t, ownerConn, "ack")
	sendJSON(t, viewerConn, joinPayload)
	waitForType(t, viewerConn, "ack")

	sendJSON(t, ownerConn, map[string]any{
		"type":       "timeout",
		"channelId":  channel.ID,
		"targetId":   viewer.ID,
		"durationMs": 5000,
	})
	waitForType(t, ownerConn, "event")
	waitForType(t, viewerConn, "event")

	// Attempt to speak while timed out
	sendJSON(t, viewerConn, map[string]string{
		"type":      "message",
		"channelId": channel.ID,
		"content":   "should fail",
	})
	expectError(t, viewerConn)

	testsupport.WaitUntil(t, time.Second, "chat timeout persisted", func() bool {
		_, ok := store.ChatTimeout(channel.ID, viewer.ID)
		return ok
	})
}

func TestGatewayAutoModBlocksMessage(t *testing.T) {
	store := newTestStorage(t)
	owner := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"admin"}})
	viewer := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "viewer", Email: "viewer@example.com"})
	channel := mustCreateChannel(t, store, owner.ID, "Main")

	_, err := store.CreateChatFilter(channel.ID, domain.ChatFilterCreateParams{
		Kind:    "word",
		Pattern: "spoiler",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateChatFilter: %v", err)
	}

	queue := chat.NewMemoryQueue(32)
	gateway := chat.NewGateway(chat.GatewayConfig{Queue: queue, Store: store})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	go storage.NewChatWorker(store, queue, nil).WithStartedChannel(started).Run(ctx)
	<-started

	if _, err := gateway.CreateMessage(ctx, viewer, channel.ID, "no spoilers please"); err == nil {
		t.Fatalf("expected automod rejection, got nil")
	}

	testsupport.WaitUntil(t, time.Second, "automod action recorded", func() bool {
		actions, err := store.ListChatAutoModActions(channel.ID, 0)
		if err != nil || len(actions) == 0 {
			return false
		}
		return actions[0].UserID == viewer.ID
	})

	messages, err := store.ListChatMessages(channel.ID, 0)
	if err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected zero messages, got %d", len(messages))
	}
}

func TestGatewayCreateMessageRuneLimitCountsMultibyteCharacters(t *testing.T) {
	gateway := chat.NewGateway(chat.GatewayConfig{})
	author := domain.User{ID: "viewer"}
	channelID := "channel-1"
	ctx := context.Background()

	atLimit := strings.Repeat("🙂", 500)
	if _, err := gateway.CreateMessage(ctx, author, channelID, atLimit); err != nil {
		t.Fatalf("expected 500-rune message to pass, got %v", err)
	}

	overLimit := strings.Repeat("🙂", 501)
	if _, err := gateway.CreateMessage(ctx, author, channelID, overLimit); err == nil {
		t.Fatalf("expected over-limit error, got nil")
	} else if err.Error() != "message exceeds 500 characters" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGatewayApplyModerationWithoutStore(t *testing.T) {
	gateway := chat.NewGateway(chat.GatewayConfig{})
	actor := domain.User{ID: "moderator", Roles: []string{"admin"}}
	expiresAt := time.Now().Add(time.Minute)
	err := gateway.ApplyModeration(context.Background(), actor, chat.ModerationEvent{
		ChannelID: "missing-channel",
		TargetID:  "user",
		Action:    chat.ModerationActionTimeout,
		ExpiresAt: &expiresAt,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "chat store unavailable" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newTestStorage(t *testing.T) *storage.Storage {
	t.Helper()
	tempDir := t.TempDir()
	store, err := storage.NewStorage(tempDir + "/store.json")
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	return store
}

func mustCreateUser(t *testing.T, store *storage.Storage, params storage.CreateUserParams) domain.User {
	t.Helper()
	user, err := store.CreateUser(params)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return user
}

func mustCreateChannel(t *testing.T, store *storage.Storage, ownerID, title string) domain.Channel {
	t.Helper()
	channel, err := store.CreateChannel(ownerID, title, "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	return channel
}

func mustDial(t *testing.T, url string) *chat.Conn {
	t.Helper()
	conn, err := chat.Dial(context.Background(), url, http.Header{}, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	return conn
}

func sendJSON(t *testing.T, conn *chat.Conn, payload interface{}) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := conn.WriteText(data); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
}

func expectError(t *testing.T, conn *chat.Conn) {
	t.Helper()
	waitForType(t, conn, "error")
}

func readJSON(t *testing.T, conn *chat.Conn) map[string]interface{} {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data, err := conn.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return payload
}

func waitForType(t *testing.T, conn *chat.Conn, expected string) map[string]interface{} {
	t.Helper()
	for i := 0; i < 8; i++ {
		message := readJSON(t, conn)
		if message["type"] == expected {
			return message
		}
	}
	t.Fatalf("expected %s message", expected)
	return nil
}
