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
		"reason":     "cool down",
	})
	moderationEvent := waitForEventType(t, ownerConn, "moderation")
	moderation := eventSection(t, moderationEvent, "moderation")
	if moderation["reason"] != "cool down" {
		t.Fatalf("expected moderation reason, got %#v", moderation["reason"])
	}
	waitForEventType(t, viewerConn, "moderation")

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

func TestGatewayRejectsUnauthorizedModeration(t *testing.T) {
	store := newTestStorage(t)
	owner := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "owner", Email: "owner@example.com", Roles: []string{"admin"}})
	viewer := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "viewer", Email: "viewer@example.com"})
	target := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "target", Email: "target@example.com"})
	channel := mustCreateChannel(t, store, owner.ID, "Main")

	gateway := chat.NewGateway(chat.GatewayConfig{Store: store})
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
	viewerConn := mustDial(t, wsURL+"?user="+viewer.ID)
	defer func() {
		_ = viewerConn.Close()
	}()

	sendJSON(t, viewerConn, map[string]string{"type": "join", "channelId": channel.ID})
	waitForType(t, viewerConn, "ack")
	waitForEventType(t, viewerConn, "presence_snapshot")

	sendJSON(t, viewerConn, map[string]any{
		"type":       "timeout",
		"channelId":  channel.ID,
		"targetId":   target.ID,
		"durationMs": 5000,
		"reason":     "attempted overreach",
	})

	payload := waitForType(t, viewerConn, "error")
	if payload["error"] != "forbidden" {
		t.Fatalf("expected forbidden error, got %#v", payload["error"])
	}
	if _, ok := store.ChatTimeout(channel.ID, target.ID); ok {
		t.Fatalf("expected unauthorized timeout to be rejected")
	}
}

func TestGatewayAllowsModeratorRole(t *testing.T) {
	store := newTestStorage(t)
	owner := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "owner", Email: "owner@example.com"})
	moderator := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "mod", Email: "mod@example.com", Roles: []string{"moderator"}})
	target := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "target", Email: "target@example.com"})
	channel := mustCreateChannel(t, store, owner.ID, "Main")

	gateway := chat.NewGateway(chat.GatewayConfig{Store: store})
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
	moderatorConn := mustDial(t, wsURL+"?user="+moderator.ID)
	defer func() {
		_ = moderatorConn.Close()
	}()

	sendJSON(t, moderatorConn, map[string]string{"type": "join", "channelId": channel.ID})
	waitForType(t, moderatorConn, "ack")
	waitForEventType(t, moderatorConn, "presence_snapshot")

	sendJSON(t, moderatorConn, map[string]any{
		"type":      "ban",
		"channelId": channel.ID,
		"targetId":  target.ID,
		"reason":    "spam raid",
	})

	event := waitForEventType(t, moderatorConn, "moderation")
	moderation := eventSection(t, event, "moderation")
	if moderation["action"] != "ban" {
		t.Fatalf("expected ban action, got %#v", moderation["action"])
	}
	if moderation["reason"] != "spam raid" {
		t.Fatalf("expected ban reason, got %#v", moderation["reason"])
	}
}

func TestGatewayPresenceSnapshotAndConnectionDeduping(t *testing.T) {
	store := newTestStorage(t)
	owner := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "Owner", Email: "owner@example.com", Roles: []string{"creator"}})
	viewer := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "Viewer", Email: "viewer@example.com"})
	channel := mustCreateChannel(t, store, owner.ID, "Main")

	gateway := chat.NewGateway(chat.GatewayConfig{Store: store})
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
	viewerConnOne := mustDial(t, wsURL+"?user="+viewer.ID)
	defer func() {
		_ = viewerConnOne.Close()
	}()
	viewerConnTwo := mustDial(t, wsURL+"?user="+viewer.ID)
	defer func() {
		_ = viewerConnTwo.Close()
	}()
	ownerConn := mustDial(t, wsURL+"?user="+owner.ID)
	defer func() {
		_ = ownerConn.Close()
	}()

	joinPayload := map[string]string{"type": "join", "channelId": channel.ID}
	sendJSON(t, viewerConnOne, joinPayload)
	waitForType(t, viewerConnOne, "ack")
	snapshotOne := waitForEventType(t, viewerConnOne, "presence_snapshot")
	assertPresenceCounts(t, snapshotOne, 1, 1)
	assertPresenceUsers(t, snapshotOne, []string{viewer.ID})

	sendJSON(t, viewerConnTwo, joinPayload)
	waitForType(t, viewerConnTwo, "ack")
	snapshotTwo := waitForEventType(t, viewerConnTwo, "presence_snapshot")
	assertPresenceCounts(t, snapshotTwo, 2, 1)
	assertPresenceUsers(t, snapshotTwo, []string{viewer.ID})

	sendJSON(t, ownerConn, joinPayload)
	waitForType(t, ownerConn, "ack")
	ownerJoin := waitForEventType(t, viewerConnOne, "presence_join")
	assertPresenceCounts(t, ownerJoin, 3, 2)
	assertPresenceUser(t, ownerJoin, owner.ID, "owner")

	sendJSON(t, viewerConnTwo, map[string]string{"type": "leave", "channelId": channel.ID})
	sendJSON(t, viewerConnOne, map[string]string{"type": "leave", "channelId": channel.ID})
	viewerLeave := waitForEventType(t, ownerConn, "presence_leave")
	assertPresenceCounts(t, viewerLeave, 1, 1)
	assertPresenceUser(t, viewerLeave, viewer.ID, "viewer")
}

func TestGatewayMessageMetadataIncludesRoleAndBadges(t *testing.T) {
	store := newTestStorage(t)
	owner := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "Owner", Email: "owner@example.com", Roles: []string{"creator"}})
	channel := mustCreateChannel(t, store, owner.ID, "Main")

	gateway := chat.NewGateway(chat.GatewayConfig{Store: store})
	message, err := gateway.CreateMessage(context.Background(), owner, channel.ID, "hello room")
	if err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	if message.User == nil {
		t.Fatalf("expected message user metadata")
	}
	if message.User.ID != owner.ID {
		t.Fatalf("expected metadata user %q, got %q", owner.ID, message.User.ID)
	}
	if message.User.DisplayName != "Owner" {
		t.Fatalf("expected display name Owner, got %q", message.User.DisplayName)
	}
	if message.User.Role != "owner" {
		t.Fatalf("expected owner role, got %q", message.User.Role)
	}
	if !hasBadge(message.User.Badges, "owner") || !hasBadge(message.User.Badges, "broadcaster") {
		t.Fatalf("expected owner and broadcaster badges, got %#v", message.User.Badges)
	}
}

func TestGatewaySystemEventBroadcastIncludesDisplayMetadata(t *testing.T) {
	store := newTestStorage(t)
	owner := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "Owner", Email: "owner@example.com", Roles: []string{"creator"}})
	viewer := mustCreateUser(t, store, storage.CreateUserParams{DisplayName: "Viewer", Email: "viewer@example.com"})
	channel := mustCreateChannel(t, store, owner.ID, "Main")

	gateway := chat.NewGateway(chat.GatewayConfig{Store: store})
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
	viewerConn := mustDial(t, wsURL+"?user="+viewer.ID)
	defer func() {
		_ = viewerConn.Close()
	}()
	sendJSON(t, viewerConn, map[string]string{"type": "join", "channelId": channel.ID})
	waitForType(t, viewerConn, "ack")
	waitForEventType(t, viewerConn, "presence_snapshot")

	systemEvent, err := gateway.BroadcastSystemEvent(chat.SystemEvent{
		ChannelID: channel.ID,
		Kind:      "stream_live",
		Message:   "Stream went live.",
		ActorID:   owner.ID,
		TargetID:  viewer.ID,
		Metadata:  map[string]string{"state": "live"},
	})
	if err != nil {
		t.Fatalf("BroadcastSystemEvent: %v", err)
	}
	if systemEvent.ID == "" {
		t.Fatalf("expected generated system event id")
	}

	envelope := waitForEventType(t, viewerConn, "system")
	system := eventSection(t, envelope, "system")
	if system["kind"] != "stream_live" {
		t.Fatalf("expected stream_live system kind, got %#v", system["kind"])
	}
	if system["message"] != "Stream went live." {
		t.Fatalf("expected system message, got %#v", system["message"])
	}
	actor := sectionMap(t, system, "actor")
	if actor["id"] != owner.ID || actor["role"] != "owner" {
		t.Fatalf("expected owner actor metadata, got %#v", actor)
	}
	target := sectionMap(t, system, "target")
	if target["id"] != viewer.ID || target["role"] != "viewer" {
		t.Fatalf("expected viewer target metadata, got %#v", target)
	}
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

func waitForEventType(t *testing.T, conn *chat.Conn, expected string) map[string]interface{} {
	t.Helper()
	for i := 0; i < 16; i++ {
		message := readJSON(t, conn)
		event, ok := message["event"].(map[string]interface{})
		if !ok {
			continue
		}
		if event["type"] == expected {
			return event
		}
	}
	t.Fatalf("expected event %s", expected)
	return nil
}

func assertPresenceCounts(t *testing.T, event map[string]interface{}, viewerCount, chatterCount int) {
	t.Helper()
	presence := eventSection(t, event, "presence")
	if got := intValue(t, presence["viewerCount"]); got != viewerCount {
		t.Fatalf("expected viewerCount %d, got %d", viewerCount, got)
	}
	if got := intValue(t, presence["chatterCount"]); got != chatterCount {
		t.Fatalf("expected chatterCount %d, got %d", chatterCount, got)
	}
}

func assertPresenceUsers(t *testing.T, event map[string]interface{}, want []string) {
	t.Helper()
	presence := eventSection(t, event, "presence")
	rawUsers, ok := presence["users"].([]interface{})
	if !ok {
		t.Fatalf("expected presence users, got %#v", presence["users"])
	}
	if len(rawUsers) != len(want) {
		t.Fatalf("expected %d users, got %d", len(want), len(rawUsers))
	}
	for index, rawUser := range rawUsers {
		user, ok := rawUser.(map[string]interface{})
		if !ok {
			t.Fatalf("expected user object, got %#v", rawUser)
		}
		if user["id"] != want[index] {
			t.Fatalf("expected user %q at index %d, got %#v", want[index], index, user["id"])
		}
	}
}

func assertPresenceUser(t *testing.T, event map[string]interface{}, userID, role string) {
	t.Helper()
	presence := eventSection(t, event, "presence")
	user := sectionMap(t, presence, "user")
	if user["id"] != userID {
		t.Fatalf("expected presence user %q, got %#v", userID, user["id"])
	}
	if user["role"] != role {
		t.Fatalf("expected presence role %q, got %#v", role, user["role"])
	}
}

func eventSection(t *testing.T, event map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	return sectionMap(t, event, key)
}

func sectionMap(t *testing.T, parent map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	section, ok := parent[key].(map[string]interface{})
	if !ok {
		t.Fatalf("expected %s object, got %#v", key, parent[key])
	}
	return section
}

func intValue(t *testing.T, value interface{}) int {
	t.Helper()
	number, ok := value.(float64)
	if !ok {
		t.Fatalf("expected number, got %#v", value)
	}
	return int(number)
}

func hasBadge(badges []chat.UserBadge, id string) bool {
	for _, badge := range badges {
		if badge.ID == id {
			return true
		}
	}
	return false
}
