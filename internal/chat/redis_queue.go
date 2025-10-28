package chat

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RedisQueueConfig configures the Redis-backed chat queue implementation.
type RedisQueueConfig struct {
	Addr         string
	Password     string
	Stream       string
	Group        string
	Logger       *slog.Logger
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	BlockTimeout time.Duration
	Buffer       int
}

// NewRedisQueue initialises a queue backed by Redis Streams. The caller is
// responsible for ensuring the Redis instance is reachable.
func NewRedisQueue(cfg RedisQueueConfig) (Queue, error) {
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		return nil, fmt.Errorf("redis addr is required")
	}
	stream := strings.TrimSpace(cfg.Stream)
	if stream == "" {
		stream = "bitriver:chat"
	}
	group := strings.TrimSpace(cfg.Group)
	if group == "" {
		group = "chat-workers"
	}
	if cfg.Buffer <= 0 {
		cfg.Buffer = 128
	}
	queue := &redisQueue{
		addr:         addr,
		password:     cfg.Password,
		stream:       stream,
		group:        group,
		dialTimeout:  cfg.DialTimeout,
		readTimeout:  cfg.ReadTimeout,
		writeTimeout: cfg.WriteTimeout,
		blockTimeout: cfg.BlockTimeout,
		logger:       cfg.Logger,
		buffer:       cfg.Buffer,
	}
	if queue.logger == nil {
		queue.logger = slog.Default()
	}
	if queue.dialTimeout <= 0 {
		queue.dialTimeout = 5 * time.Second
	}
	if queue.readTimeout <= 0 {
		queue.readTimeout = 10 * time.Second
	}
	if queue.writeTimeout <= 0 {
		queue.writeTimeout = 5 * time.Second
	}
	if queue.blockTimeout <= 0 {
		queue.blockTimeout = 2 * time.Second
	}
	queue.publisher = newRedisConn(queue)
	if err := queue.ensureGroup(context.Background()); err != nil {
		return nil, err
	}
	return queue, nil
}

type redisQueue struct {
	addr         string
	password     string
	stream       string
	group        string
	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
	blockTimeout time.Duration
	logger       *slog.Logger
	buffer       int

	publisher *redisConn
	groupOnce sync.Once
	groupErr  error
}

func (q *redisQueue) Publish(ctx context.Context, event Event) error {
	if event.Type == "" {
		return errors.New("event type is required")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if err := q.ensureGroup(ctx); err != nil {
		return err
	}
	args := []string{"XADD", q.stream, "*", "payload", string(payload)}
	_, err = q.publisher.Do(ctx, args...)
	return err
}

func (q *redisQueue) Subscribe() Subscription {
	ctx, cancel := context.WithCancel(context.Background())
	if err := q.ensureGroup(ctx); err != nil {
		if q.logger != nil {
			q.logger.Error("redis queue group setup failed", "error", err)
		}
	}
	consumer := randomConsumerID()
	conn := newRedisConn(q)
	sub := &redisSubscription{
		queue:    q,
		consumer: consumer,
		conn:     conn,
		cancel:   cancel,
		ch:       make(chan Event, q.buffer),
	}
	go sub.run(ctx)
	return sub
}

func (q *redisQueue) ensureGroup(ctx context.Context) error {
	q.groupOnce.Do(func() {
		conn := q.publisher
		if conn == nil {
			conn = newRedisConn(q)
			q.publisher = conn
		}
		args := []string{"XGROUP", "CREATE", q.stream, q.group, "$", "MKSTREAM"}
		if _, err := conn.Do(ctx, args...); err != nil {
			if !isBusyGroup(err) {
				q.groupErr = err
			}
		}
	})
	return q.groupErr
}

type redisSubscription struct {
	queue    *redisQueue
	consumer string
	conn     *redisConn
	cancel   context.CancelFunc

	once sync.Once
	ch   chan Event
}

func (s *redisSubscription) Events() <-chan Event {
	return s.ch
}

func (s *redisSubscription) Close() {
	s.once.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.conn != nil {
			s.conn.Close()
		}
		close(s.ch)
	})
}

func (s *redisSubscription) run(ctx context.Context) {
	defer s.Close()
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		entries, err := s.read(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			if s.queue.logger != nil {
				s.queue.logger.Warn("redis queue read failed", "error", err)
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}
		for _, entry := range entries {
			var event Event
			if err := json.Unmarshal(entry.Payload, &event); err != nil {
				if s.queue.logger != nil {
					s.queue.logger.Error("redis queue decode failed", "error", err)
				}
				s.ack(ctx, entry.ID)
				continue
			}
			select {
			case s.ch <- event:
				s.ack(ctx, entry.ID)
			case <-ctx.Done():
				return
			}
		}
	}
}

func (s *redisSubscription) ack(ctx context.Context, id string) {
	if id == "" {
		return
	}
	_, err := s.conn.Do(ctx, "XACK", s.queue.stream, s.queue.group, id)
	if err != nil && s.queue.logger != nil {
		s.queue.logger.Warn("redis ack failed", "id", id, "error", err)
	}
}

type redisStreamEntry struct {
	ID      string
	Payload []byte
}

func (s *redisSubscription) read(ctx context.Context) ([]redisStreamEntry, error) {
	blockMs := int(math.Max(float64(s.queue.blockTimeout.Milliseconds()), 1))
	reply, err := s.conn.Do(ctx, "XREADGROUP", "GROUP", s.queue.group, s.consumer, "COUNT", "32", "BLOCK", strconv.Itoa(blockMs), "STREAMS", s.queue.stream, ">")
	if err != nil {
		if isNilReply(err) {
			return nil, nil
		}
		return nil, err
	}
	streams, ok := reply.([]interface{})
	if !ok || len(streams) == 0 {
		return nil, nil
	}
	var entries []redisStreamEntry
	for _, stream := range streams {
		parts, ok := stream.([]interface{})
		if !ok || len(parts) != 2 {
			continue
		}
		records, _ := parts[1].([]interface{})
		for _, record := range records {
			tuple, ok := record.([]interface{})
			if !ok || len(tuple) != 2 {
				continue
			}
			id, _ := asString(tuple[0])
			fields, _ := tuple[1].([]interface{})
			payload := extractPayload(fields)
			if id == "" || len(payload) == 0 {
				continue
			}
			entries = append(entries, redisStreamEntry{ID: id, Payload: payload})
		}
	}
	return entries, nil
}

func extractPayload(fields []interface{}) []byte {
	for i := 0; i < len(fields); i += 2 {
		key, _ := asString(fields[i])
		if strings.EqualFold(key, "payload") && i+1 < len(fields) {
			value, _ := fields[i+1].(string)
			return []byte(value)
		}
	}
	return nil
}

func isBusyGroup(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "busygrou")
}

func isNilReply(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "nil reply")
}

func randomConsumerID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("consumer-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("consumer-%s", hex.EncodeToString(buf))
}

type redisConn struct {
	queue *redisQueue

	mu     sync.Mutex
	conn   net.Conn
	reader *bufio.Reader
}

func newRedisConn(q *redisQueue) *redisConn {
	return &redisConn{queue: q}
}

func (c *redisConn) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = nil
	c.reader = nil
}

func (c *redisConn) Do(ctx context.Context, args ...string) (interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("redis command missing")
	}
	if err := c.ensureConnection(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil, fmt.Errorf("redis connection unavailable")
	}
	deadline := time.Now().Add(c.queue.writeTimeout)
	if ctx != nil {
		if dl, ok := ctx.Deadline(); ok {
			deadline = dl
		}
	}
	_ = c.conn.SetWriteDeadline(deadline)
	if err := writeCommand(c.conn, args); err != nil {
		c.resetLocked()
		return nil, err
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(c.queue.readTimeout))
	reply, err := parseRESP(c.reader)
	if err != nil {
		if errors.Is(err, io.EOF) {
			c.resetLocked()
		}
		return nil, err
	}
	return reply, nil
}

func (c *redisConn) ensureConnection(ctx context.Context) error {
	c.mu.Lock()
	if c.conn != nil {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	dialer := net.Dialer{Timeout: c.queue.dialTimeout}
	if ctx == nil {
		ctx = context.Background()
	}
	conn, err := dialer.DialContext(ctx, "tcp", c.queue.addr)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(conn)

	c.mu.Lock()
	c.conn = conn
	c.reader = reader
	c.mu.Unlock()

	if strings.TrimSpace(c.queue.password) != "" {
		if _, err := c.Do(ctx, "AUTH", c.queue.password); err != nil {
			conn.Close()
			c.mu.Lock()
			c.conn = nil
			c.reader = nil
			c.mu.Unlock()
			return fmt.Errorf("redis auth failed: %w", err)
		}
	}
	return nil
}

func (c *redisConn) resetLocked() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = nil
	c.reader = nil
}

func writeCommand(w io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("redis command missing")
	}
	if _, err := fmt.Fprintf(w, "*%d\r\n", len(args)); err != nil {
		return err
	}
	for _, arg := range args {
		if arg == "" {
			if _, err := io.WriteString(w, "$-1\r\n"); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(w, "$%d\r\n%s\r\n", len(arg), arg); err != nil {
			return err
		}
	}
	if flusher, ok := w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

func parseRESP(r *bufio.Reader) (interface{}, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	switch prefix {
	case '+':
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		return line, nil
	case '-':
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		return nil, errors.New(line)
	case ':':
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		value, convErr := strconv.ParseInt(line, 10, 64)
		if convErr != nil {
			return nil, convErr
		}
		return value, nil
	case '$':
		lengthStr, err := readLine(r)
		if err != nil {
			return nil, err
		}
		length, convErr := strconv.Atoi(lengthStr)
		if convErr != nil {
			return nil, convErr
		}
		if length == -1 {
			return nil, nil
		}
		buf := make([]byte, length+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return string(buf[:length]), nil
	case '*':
		lengthStr, err := readLine(r)
		if err != nil {
			return nil, err
		}
		length, convErr := strconv.Atoi(lengthStr)
		if convErr != nil {
			return nil, convErr
		}
		if length == -1 {
			return nil, nil
		}
		items := make([]interface{}, 0, length)
		for i := 0; i < length; i++ {
			item, err := parseRESP(r)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	default:
		return nil, fmt.Errorf("unexpected redis prefix %q", prefix)
	}
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}

func asString(v interface{}) (string, bool) {
	switch value := v.(type) {
	case string:
		return value, true
	case []byte:
		return string(value), true
	default:
		return "", false
	}
}
