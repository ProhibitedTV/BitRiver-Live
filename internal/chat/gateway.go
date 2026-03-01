package chat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/observability/metrics"
)

// Store exposes the read-only operations the gateway requires from the backing
// datastore.
type Store interface {
	GetChannel(id string) (domain.Channel, bool)
	GetUser(id string) (domain.User, bool)
	ChatRestrictions() RestrictionsSnapshot
	IsChatBanned(channelID, userID string) bool
	ChatTimeout(channelID, userID string) (time.Time, bool)
	ListChatFilters(channelID string) ([]domain.ChatFilter, error)
}

// GatewayConfig configures a chat Gateway.
type GatewayConfig struct {
	Queue  Queue
	Store  Store
	Logger *slog.Logger
	// HeartbeatInterval controls how often the gateway sends WebSocket ping
	// frames to connected clients. A zero value disables heartbeats.
	HeartbeatInterval time.Duration
}

// Gateway coordinates live chat fan-out, managing WebSocket clients and
// publishing persistence events to the configured queue.
type Gateway struct {
	queue  Queue
	store  Store
	logger *slog.Logger

	heartbeatInterval time.Duration

	mu       sync.RWMutex
	rooms    map[string]map[*client]struct{}
	bans     map[string]map[string]struct{}
	timeouts map[string]map[string]time.Time

	regexMu      sync.RWMutex
	regexCache   map[string]*regexp.Regexp
	regexCompile func(string) (*regexp.Regexp, error)
}

// NewGateway initialises a gateway using the provided configuration.
func NewGateway(cfg GatewayConfig) *Gateway {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	snapshot := RestrictionsSnapshot{}
	if cfg.Store != nil {
		snapshot = cfg.Store.ChatRestrictions().Copy()
	}
	return &Gateway{
		queue:             cfg.Queue,
		store:             cfg.Store,
		logger:            logger,
		heartbeatInterval: cfg.HeartbeatInterval,
		rooms:             make(map[string]map[*client]struct{}),
		bans:              snapshot.Bans,
		timeouts:          snapshot.Timeouts,
		regexCache:        make(map[string]*regexp.Regexp),
		regexCompile:      regexp.Compile,
	}
}

// HandleConnection upgrades the HTTP request to a WebSocket connection for the
// authenticated user.
func (g *Gateway) HandleConnection(w http.ResponseWriter, r *http.Request, user domain.User) {
	conn, err := Accept(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &client{
		gateway: g,
		conn:    conn,
		user:    user,
		send:    make(chan outboundMessage, 16),
		rooms:   make(map[string]struct{}),
		cancel:  cancel,
	}

	go c.writeLoop()
	if g.heartbeatInterval > 0 {
		go c.heartbeatLoop(ctx, g.heartbeatInterval)
	}
	go c.readLoop(ctx)
}

// CreateMessage generates a new chat message authored by the given user.
func (g *Gateway) CreateMessage(ctx context.Context, author domain.User, channelID, content string) (MessageEvent, error) {
	if err := g.ensureChannelAccessible(channelID, author.ID); err != nil {
		return MessageEvent{}, err
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return MessageEvent{}, fmt.Errorf("message cannot be empty")
	}
	if len([]rune(trimmed)) > 500 {
		return MessageEvent{}, fmt.Errorf("message exceeds 500 characters")
	}
	if g.store != nil {
		filter, err := g.matchChatFilter(channelID, trimmed)
		if err != nil {
			if g.logger != nil {
				g.logger.Warn("failed to evaluate chat filters", "error", err)
			}
		} else if filter != nil {
			return MessageEvent{}, g.emitAutoMod(ctx, author, channelID, trimmed, *filter)
		}
	}
	id, err := generateID()
	if err != nil {
		return MessageEvent{}, err
	}
	message := MessageEvent{
		ID:        id,
		ChannelID: channelID,
		UserID:    author.ID,
		Content:   trimmed,
		CreatedAt: time.Now().UTC(),
	}
	event := Event{Type: EventTypeMessage, Message: &message, OccurredAt: time.Now().UTC()}
	g.broadcast(event)
	g.publish(ctx, event)
	metrics.Default().ObserveChatEvent("message")
	return message, nil
}

// matchChatFilter performs match chat filter and propagates validation or dependency failures to the caller.
func (g *Gateway) matchChatFilter(channelID, content string) (*domain.ChatFilter, error) {
	filters, err := g.store.ListChatFilters(channelID)
	if err != nil {
		return nil, err
	}
	if len(filters) == 0 {
		return nil, nil
	}
	lowerContent := strings.ToLower(content)
	for _, filter := range filters {
		if !filter.Enabled {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(filter.Kind)) {
		case "word":
			pattern := strings.ToLower(strings.TrimSpace(filter.Pattern))
			if pattern != "" && strings.Contains(lowerContent, pattern) {
				return &filter, nil
			}
		case "regex":
			pattern := strings.TrimSpace(filter.Pattern)
			if pattern == "" {
				continue
			}
			re, err := g.compiledRegex(filter.ID, pattern)
			if err != nil {
				if g.logger != nil {
					g.logger.Warn("invalid chat filter regex", "filter_id", filter.ID, "error", err)
				}
				continue
			}
			if re.MatchString(content) {
				return &filter, nil
			}
		}
	}
	return nil, nil
}

func (g *Gateway) compiledRegex(filterID, pattern string) (*regexp.Regexp, error) {
	cacheKey := filterID + "\n" + pattern

	g.regexMu.RLock()
	if cached, ok := g.regexCache[cacheKey]; ok {
		g.regexMu.RUnlock()
		return cached, nil
	}
	g.regexMu.RUnlock()

	compiled, err := g.regexCompile(pattern)
	if err != nil {
		return nil, err
	}

	g.regexMu.Lock()
	if cached, ok := g.regexCache[cacheKey]; ok {
		g.regexMu.Unlock()
		return cached, nil
	}
	g.regexCache[cacheKey] = compiled
	g.regexMu.Unlock()

	return compiled, nil
}

// emitAutoMod performs emit auto mod and propagates validation or dependency failures to the caller.
func (g *Gateway) emitAutoMod(ctx context.Context, author domain.User, channelID, content string, filter domain.ChatFilter) error {
	id, err := generateID()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	event := AutoModEvent{
		ID:            id,
		ChannelID:     channelID,
		UserID:        author.ID,
		FilterID:      filter.ID,
		FilterKind:    filter.Kind,
		FilterPattern: filter.Pattern,
		Message:       content,
		Action:        "blocked",
		CreatedAt:     now,
	}
	evt := Event{Type: EventTypeAutoMod, AutoMod: &event, OccurredAt: now}
	g.broadcast(evt)
	g.publish(ctx, evt)
	metrics.Default().ObserveChatEvent("automod")
	return fmt.Errorf("message blocked by automated moderation")
}

// ApplyModeration emits a moderation event into the chat stream.
func (g *Gateway) ApplyModeration(ctx context.Context, actor domain.User, event ModerationEvent) error {
	if err := g.validateModeration(actor, event); err != nil {
		return err
	}
	now := time.Now().UTC()
	if event.Action == ModerationActionTimeout && event.ExpiresAt == nil {
		return fmt.Errorf("timeout expiry required")
	}
	if event.Action == ModerationActionTimeout && event.ExpiresAt != nil && event.ExpiresAt.Before(now) {
		return fmt.Errorf("timeout expiry must be in the future")
	}
	evt := Event{Type: EventTypeModeration, Moderation: &event, OccurredAt: now}
	g.applyModeration(event)
	g.broadcast(evt)
	g.publish(ctx, evt)
	metrics.Default().ObserveChatEvent("moderation:" + string(event.Action))
	return nil
}

// SubmitReport emits a viewer report into the chat stream and persistence layer.
func (g *Gateway) SubmitReport(ctx context.Context, reporter domain.User, channelID, targetID, reason, messageID, evidenceURL string) (ReportEvent, error) {
	if err := g.ensureChannelAccessible(channelID, reporter.ID); err != nil {
		return ReportEvent{}, err
	}
	if strings.TrimSpace(targetID) == "" {
		return ReportEvent{}, fmt.Errorf("target is required")
	}
	if g.store != nil {
		if _, ok := g.store.GetChannel(channelID); !ok {
			return ReportEvent{}, fmt.Errorf("channel %s not found", channelID)
		}
		if _, ok := g.store.GetUser(targetID); !ok {
			return ReportEvent{}, fmt.Errorf("user %s not found", targetID)
		}
	}
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return ReportEvent{}, fmt.Errorf("reason is required")
	}
	id, err := generateID()
	if err != nil {
		return ReportEvent{}, err
	}
	now := time.Now().UTC()
	report := ReportEvent{
		ID:          id,
		ChannelID:   channelID,
		ReporterID:  reporter.ID,
		TargetID:    targetID,
		Reason:      trimmedReason,
		MessageID:   strings.TrimSpace(messageID),
		EvidenceURL: strings.TrimSpace(evidenceURL),
		Status:      "open",
		CreatedAt:   now,
	}
	evt := Event{Type: EventTypeReport, Report: &report, OccurredAt: now}
	g.broadcast(evt)
	g.publish(ctx, evt)
	metrics.Default().ObserveChatEvent("report")
	return report, nil
}

// publish performs publish and propagates validation or dependency failures to the caller.
func (g *Gateway) publish(ctx context.Context, event Event) {
	if g.queue == nil {
		return
	}
	if err := g.queue.Publish(ctx, event); err != nil && g.logger != nil {
		g.logger.Warn("failed to publish chat event", "error", err)
	}
}

// ensureChannelAccessible performs ensure channel accessible and propagates validation or dependency failures to the caller.
func (g *Gateway) ensureChannelAccessible(channelID, userID string) error {
	if g.store != nil {
		if _, ok := g.store.GetChannel(channelID); !ok {
			return fmt.Errorf("channel %s not found", channelID)
		}
		if _, ok := g.store.GetUser(userID); !ok {
			return fmt.Errorf("user %s not found", userID)
		}
	}
	if g.isBanned(channelID, userID) {
		return fmt.Errorf("user is banned")
	}
	if expiry, ok := g.timeoutExpiry(channelID, userID); ok {
		if time.Now().UTC().Before(expiry) {
			return fmt.Errorf("user is timed out")
		}
		g.clearTimeout(channelID, userID)
	}
	return nil
}

// validateModeration validates moderation and reports an error when required invariants are not met.
func (g *Gateway) validateModeration(actor domain.User, evt ModerationEvent) error {
	if evt.ChannelID == "" || evt.TargetID == "" {
		return fmt.Errorf("channel and target are required")
	}
	if g.store == nil {
		return fmt.Errorf("chat store unavailable")
	}
	channel, exists := g.store.GetChannel(evt.ChannelID)
	if !exists {
		return fmt.Errorf("channel %s not found", evt.ChannelID)
	}
	if actor.ID != channel.OwnerID && !actor.HasRole("admin") {
		return fmt.Errorf("forbidden")
	}
	if evt.Action == ModerationActionTimeout && evt.ExpiresAt == nil {
		return fmt.Errorf("timeout expiry required")
	}
	if evt.Action == ModerationActionTimeout && evt.TargetID == actor.ID {
		return fmt.Errorf("cannot timeout yourself")
	}
	return nil
}

// broadcast performs broadcast and propagates validation or dependency failures to the caller.
func (g *Gateway) broadcast(event Event) {
	if event.Type == EventTypeModeration {
		if event.Moderation != nil {
			g.applyModeration(*event.Moderation)
		}
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	var channelID string
	if event.Message != nil {
		channelID = event.Message.ChannelID
	} else if event.Moderation != nil {
		channelID = event.Moderation.ChannelID
	} else if event.Report != nil {
		channelID = event.Report.ChannelID
	}
	if channelID == "" {
		return
	}
	recipients := g.rooms[channelID]
	if len(recipients) == 0 {
		return
	}
	payload, err := json.Marshal(outboundMessage{Type: "event", Event: &event})
	if err != nil {
		if g.logger != nil {
			g.logger.Error("failed to marshal chat event", "error", err)
		}
		return
	}
	for client := range recipients {
		select {
		case client.send <- outboundMessage{Raw: payload}:
		default:
		}
	}
}

// applyModeration performs apply moderation and propagates validation or dependency failures to the caller.
func (g *Gateway) applyModeration(evt ModerationEvent) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch evt.Action {
	case ModerationActionBan:
		if g.bans == nil {
			g.bans = make(map[string]map[string]struct{})
		}
		if g.bans[evt.ChannelID] == nil {
			g.bans[evt.ChannelID] = make(map[string]struct{})
		}
		g.bans[evt.ChannelID][evt.TargetID] = struct{}{}
	case ModerationActionUnban:
		if g.bans != nil {
			delete(g.bans[evt.ChannelID], evt.TargetID)
		}
	case ModerationActionTimeout:
		if g.timeouts == nil {
			g.timeouts = make(map[string]map[string]time.Time)
		}
		if g.timeouts[evt.ChannelID] == nil {
			g.timeouts[evt.ChannelID] = make(map[string]time.Time)
		}
		if evt.ExpiresAt != nil {
			g.timeouts[evt.ChannelID][evt.TargetID] = evt.ExpiresAt.UTC()
		}
	case ModerationActionRemoveTimeout:
		if g.timeouts != nil {
			delete(g.timeouts[evt.ChannelID], evt.TargetID)
		}
	}
}

// isBanned reports whether banned is satisfied for the current input.
func (g *Gateway) isBanned(channelID, userID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if bans := g.bans[channelID]; bans != nil {
		if _, exists := bans[userID]; exists {
			return true
		}
	}
	if g.store != nil {
		return g.store.IsChatBanned(channelID, userID)
	}
	return false
}

// timeoutExpiry performs timeout expiry and propagates validation or dependency failures to the caller.
func (g *Gateway) timeoutExpiry(channelID, userID string) (time.Time, bool) {
	g.mu.RLock()
	if timeouts := g.timeouts[channelID]; timeouts != nil {
		if expiry, ok := timeouts[userID]; ok {
			g.mu.RUnlock()
			return expiry, true
		}
	}
	g.mu.RUnlock()
	if g.store != nil {
		return g.store.ChatTimeout(channelID, userID)
	}
	return time.Time{}, false
}

// clearTimeout performs clear timeout and propagates validation or dependency failures to the caller.
func (g *Gateway) clearTimeout(channelID, userID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if timeouts := g.timeouts[channelID]; timeouts != nil {
		delete(timeouts, userID)
	}
}

// generateID performs generate id and propagates validation or dependency failures to the caller.
func generateID() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

type client struct {
	gateway *Gateway
	conn    *Conn
	user    domain.User
	send    chan outboundMessage
	rooms   map[string]struct{}
	closed  sync.Once
	cancel  context.CancelFunc
}

type inboundMessage struct {
	Type       string `json:"type"`
	ChannelID  string `json:"channelId"`
	Content    string `json:"content"`
	TargetID   string `json:"targetId"`
	DurationMs int    `json:"durationMs"`
	Reason     string `json:"reason"`
	MessageID  string `json:"messageId"`
	Evidence   string `json:"evidenceUrl"`
}

type outboundMessage struct {
	Type  string `json:"type,omitempty"`
	Error string `json:"error,omitempty"`
	Event *Event `json:"event,omitempty"`
	Raw   []byte `json:"-"`
}

// writeLoop writes loop to the active response or stream and surfaces encode or I/O failures.
func (c *client) writeLoop() {
	defer c.close()
	for msg := range c.send {
		payload := msg.Raw
		if payload == nil {
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			payload = data
		}
		if err := c.conn.WriteText(payload); err != nil {
			return
		}
	}
}

// heartbeatLoop performs heartbeat loop and propagates validation or dependency failures to the caller.
func (c *client) heartbeatLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.conn.Ping(nil); err != nil {
				c.close()
				return
			}
		}
	}
}

// readLoop reads loop from the underlying source and returns decode or I/O errors.
func (c *client) readLoop(ctx context.Context) {
	defer c.close()
	for {
		payload, err := c.conn.ReadMessage(ctx)
		if err != nil {
			return
		}
		var msg inboundMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			c.sendError("invalid payload")
			continue
		}
		switch msg.Type {
		case "join":
			c.handleJoin(msg.ChannelID)
		case "leave":
			c.handleLeave(msg.ChannelID)
		case "message":
			c.handleMessage(ctx, msg)
		case "timeout":
			c.handleModeration(ctx, msg, ModerationActionTimeout)
		case "remove_timeout":
			c.handleModeration(ctx, msg, ModerationActionRemoveTimeout)
		case "ban":
			c.handleModeration(ctx, msg, ModerationActionBan)
		case "unban":
			c.handleModeration(ctx, msg, ModerationActionUnban)
		case "report":
			c.handleReport(ctx, msg)
		default:
			c.sendError("unknown command")
		}
	}
}

// handleJoin routes and serves join requests, writing HTTP errors for invalid input or backend failures.
func (c *client) handleJoin(channelID string) {
	if channelID == "" {
		c.sendError("channel required")
		return
	}
	if err := c.gateway.ensureChannelAccessible(channelID, c.user.ID); err != nil {
		c.sendError(err.Error())
		return
	}
	c.gateway.mu.Lock()
	if c.gateway.rooms[channelID] == nil {
		c.gateway.rooms[channelID] = make(map[*client]struct{})
	}
	c.gateway.rooms[channelID][c] = struct{}{}
	c.gateway.mu.Unlock()
	c.rooms[channelID] = struct{}{}

	payload, _ := json.Marshal(outboundMessage{Type: "ack"})
	c.send <- outboundMessage{Raw: payload}
}

// handleLeave routes and serves leave requests, writing HTTP errors for invalid input or backend failures.
func (c *client) handleLeave(channelID string) {
	if channelID == "" {
		return
	}
	c.gateway.mu.Lock()
	if clients := c.gateway.rooms[channelID]; clients != nil {
		delete(clients, c)
		if len(clients) == 0 {
			delete(c.gateway.rooms, channelID)
		}
	}
	c.gateway.mu.Unlock()
	delete(c.rooms, channelID)
}

// handleMessage routes and serves message requests, writing HTTP errors for invalid input or backend failures.
func (c *client) handleMessage(ctx context.Context, msg inboundMessage) {
	if msg.ChannelID == "" {
		c.sendError("channel required")
		return
	}
	if _, joined := c.rooms[msg.ChannelID]; !joined {
		c.sendError("join channel first")
		return
	}
	event, err := c.gateway.CreateMessage(ctx, c.user, msg.ChannelID, msg.Content)
	if err != nil {
		c.sendError(err.Error())
		return
	}
	ack := Event{Type: EventTypeMessage, Message: &event, OccurredAt: time.Now().UTC()}
	payload, _ := json.Marshal(outboundMessage{Type: "ack", Event: &ack})
	c.send <- outboundMessage{Raw: payload}
}

// handleModeration routes and serves moderation requests, writing HTTP errors for invalid input or backend failures.
func (c *client) handleModeration(ctx context.Context, msg inboundMessage, action ModerationAction) {
	if msg.ChannelID == "" || msg.TargetID == "" {
		c.sendError("channel and target required")
		return
	}
	if _, joined := c.rooms[msg.ChannelID]; !joined {
		c.sendError("join channel first")
		return
	}
	evt := ModerationEvent{
		Action:    action,
		ChannelID: msg.ChannelID,
		ActorID:   c.user.ID,
		TargetID:  msg.TargetID,
	}
	if action == ModerationActionTimeout {
		duration := time.Duration(msg.DurationMs) * time.Millisecond
		if duration <= 0 {
			c.sendError("duration must be positive")
			return
		}
		expires := time.Now().Add(duration).UTC()
		evt.ExpiresAt = &expires
	}
	if err := c.gateway.ApplyModeration(ctx, c.user, evt); err != nil {
		c.sendError(err.Error())
		return
	}
}

// handleReport routes and serves report requests, writing HTTP errors for invalid input or backend failures.
func (c *client) handleReport(ctx context.Context, msg inboundMessage) {
	if msg.ChannelID == "" || msg.TargetID == "" {
		c.sendError("channel and target required")
		return
	}
	if _, joined := c.rooms[msg.ChannelID]; !joined {
		c.sendError("join channel first")
		return
	}
	report, err := c.gateway.SubmitReport(ctx, c.user, msg.ChannelID, msg.TargetID, msg.Reason, msg.MessageID, msg.Evidence)
	if err != nil {
		c.sendError(err.Error())
		return
	}
	evt := Event{Type: EventTypeReport, Report: &report, OccurredAt: report.CreatedAt}
	payload, _ := json.Marshal(outboundMessage{Type: "ack", Event: &evt})
	c.send <- outboundMessage{Raw: payload}
}

// sendError performs send error and propagates validation or dependency failures to the caller.
func (c *client) sendError(message string) {
	payload, _ := json.Marshal(outboundMessage{Type: "error", Error: message})
	select {
	case c.send <- outboundMessage{Raw: payload}:
	default:
	}
}

// close performs close and propagates validation or dependency failures to the caller.
func (c *client) close() {
	c.closed.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		for channel := range c.rooms {
			c.handleLeave(channel)
		}
		close(c.send)
		_ = c.conn.Close()
	})
}
