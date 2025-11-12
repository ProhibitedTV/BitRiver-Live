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
	"bitriver-live/internal/models"
	"bitriver-live/internal/storage"
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
	go storage.NewChatWorker(store, queue, nil).Run(ctx)

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
	defer viewerAConn.Close()
	viewerBConn := mustDial(t, wsURL+"?user="+viewerB.ID)
	defer viewerBConn.Close()

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

	waitUntil(t, 2*time.Second, func() bool {
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
	go storage.NewChatWorker(store, queue, nil).Run(ctx)

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
	defer ownerConn.Close()
	viewerConn := mustDial(t, wsURL+"?user="+viewer.ID)
	defer viewerConn.Close()

	joinPayload := map[string]string{"type": "join", "channelId": channel.ID}
	sendJSON(t, ownerConn, joinPayload)
	waitForType(t, ownerConn, "ack")
	sendJSON(t, viewerConn, joinPayload)
	waitForType(t, viewerConn, "ack")

	sendJSON(t, ownerConn, map[string]any{
		"type":       "timeout",
		"channelId":  channel.ID,
		"targetId":   viewer.ID,
		"durationMs": 200,
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

	waitUntil(t, time.Second, func() bool {
		_, ok := store.ChatTimeout(channel.ID, viewer.ID)
		return ok
	})
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

func mustCreateUser(t *testing.T, store *storage.Storage, params storage.CreateUserParams) models.User {
	t.Helper()
	user, err := store.CreateUser(params)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return user
}

func mustCreateChannel(t *testing.T, store *storage.Storage, ownerID, title string) models.Channel {
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

func expectEvent(t *testing.T, conn *chat.Conn, expectedType string) {
	t.Helper()
	message := waitForType(t, conn, "event")
	event, ok := message["event"].(map[string]interface{})
	if !ok {
		t.Fatalf("malformed event payload: %v", message)
	}
	if event["type"] != expectedType {
		t.Fatalf("expected event %s, got %v", expectedType, event["type"])
	}
}

func expectError(t *testing.T, conn *chat.Conn) {
	t.Helper()
	waitForType(t, conn, "error")
}

func readJSON(t *testing.T, conn *chat.Conn) map[string]interface{} {
	t.Helper()
	data, err := conn.ReadMessage(context.Background())
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return payload
}

func waitUntil(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
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
