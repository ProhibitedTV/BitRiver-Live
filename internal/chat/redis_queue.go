package chat

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// RedisTLSConfig controls TLS behaviour for Redis connections.
type RedisTLSConfig struct {
	CAFile             string
	CertFile           string
	KeyFile            string
	ServerName         string
	InsecureSkipVerify bool
}

// RedisQueueConfig configures the Redis-backed chat queue implementation.
type RedisQueueConfig struct {
	Addr         string
	Addrs        []string
	Username     string
	Password     string
	Stream       string
	Group        string
	Logger       *slog.Logger
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	BlockTimeout time.Duration
	Buffer       int
	PoolSize     int
	MasterName   string
	TLS          RedisTLSConfig
}

// NewRedisQueue initialises a queue backed by Redis Streams. The caller is
// responsible for ensuring the Redis instance is reachable.
func NewRedisQueue(cfg RedisQueueConfig) (Queue, error) {
	addrs := make([]string, 0, len(cfg.Addrs)+1)
	for _, addr := range cfg.Addrs {
		if trimmed := strings.TrimSpace(addr); trimmed != "" {
			addrs = append(addrs, trimmed)
		}
	}
	if addr := strings.TrimSpace(cfg.Addr); addr != "" {
		addrs = append(addrs, addr)
	}
	if len(addrs) == 0 {
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
	tlsConfig, err := buildTLSConfig(cfg.TLS)
	if err != nil {
		return nil, err
	}
	client, err := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        addrs,
		MasterName:   strings.TrimSpace(cfg.MasterName),
		Username:     strings.TrimSpace(cfg.Username),
		Password:     cfg.Password,
		TLSConfig:    tlsConfig,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
		MaxRetries:   2,
	})
	if err != nil {
		return nil, err
	}
	queue := &redisQueue{
		client:       client,
		stream:       stream,
		group:        group,
		blockTimeout: cfg.BlockTimeout,
		logger:       cfg.Logger,
		buffer:       cfg.Buffer,
	}
	if queue.logger == nil {
		queue.logger = slog.Default()
	}
	if queue.blockTimeout <= 0 {
		queue.blockTimeout = 2 * time.Second
	}
	if err := queue.ensureGroup(context.Background()); err != nil {
		client.Close()
		return nil, err
	}
	return queue, nil
}

type redisQueue struct {
	client       redis.UniversalClient
	stream       string
	group        string
	blockTimeout time.Duration
	logger       *slog.Logger
	buffer       int

	groupMu    sync.Mutex
	groupReady atomic.Bool
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
	_, err = q.client.Do(ctx, "XADD", q.stream, "*", "payload", string(payload))
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
	sub := &redisSubscription{
		queue:    q,
		consumer: consumer,
		cancel:   cancel,
		ch:       make(chan Event, q.buffer),
	}
	go sub.run(ctx)
	return sub
}

func (q *redisQueue) ensureGroup(ctx context.Context) error {
	if q.groupReady.Load() {
		return nil
	}
	q.groupMu.Lock()
	defer q.groupMu.Unlock()
	if q.groupReady.Load() {
		return nil
	}
	_, err := q.client.Do(ctx, "XGROUP", "CREATE", q.stream, q.group, "$", "MKSTREAM")
	if err != nil {
		if isBusyGroup(err) {
			q.groupReady.Store(true)
			return nil
		}
		return err
	}
	q.groupReady.Store(true)
	return nil
}

type redisSubscription struct {
	queue    *redisQueue
	consumer string
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
		if err := s.queue.ensureGroup(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			if s.queue.logger != nil {
				s.queue.logger.Warn("redis queue group ensure failed", "error", err)
			}
			time.Sleep(200 * time.Millisecond)
			continue
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
				s.requeueEntry(entry)
				return
			}
		}
	}
}

func (s *redisSubscription) ack(ctx context.Context, id string) {
	if id == "" {
		return
	}
	if _, err := s.queue.client.Do(ctx, "XACK", s.queue.stream, s.queue.group, id); err != nil && s.queue.logger != nil {
		s.queue.logger.Warn("redis ack failed", "id", id, "error", err)
	}
}

func (s *redisSubscription) requeueEntry(entry redisStreamEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	s.ack(ctx, entry.ID)
	if len(entry.Payload) == 0 {
		return
	}
	if _, err := s.queue.client.Do(ctx, "XADD", s.queue.stream, "*", "payload", string(entry.Payload)); err != nil && s.queue.logger != nil {
		s.queue.logger.Warn("redis requeue failed", "id", entry.ID, "error", err)
	}
}

type redisStreamEntry struct {
	ID      string
	Payload []byte
}

func (s *redisSubscription) read(ctx context.Context) ([]redisStreamEntry, error) {
	blockMs := int(math.Max(float64(s.queue.blockTimeout.Milliseconds()), 1))
	reply, err := s.queue.client.Do(
		ctx,
		"XREADGROUP",
		"GROUP",
		s.queue.group,
		s.consumer,
		"COUNT",
		"32",
		"BLOCK",
		strconv.Itoa(blockMs),
		"STREAMS",
		s.queue.stream,
		">",
	)
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
			value, _ := asString(fields[i+1])
			if value != "" {
				return []byte(value)
			}
		}
	}
	return nil
}

func asString(v interface{}) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case []byte:
		return string(val), true
	default:
		return "", false
	}
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
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "nil reply") || strings.Contains(msg, "timeout")
}

func randomConsumerID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("consumer-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("consumer-%s", hex.EncodeToString(buf))
}

func buildTLSConfig(cfg RedisTLSConfig) (*tls.Config, error) {
	if cfg.CAFile == "" && cfg.CertFile == "" && cfg.KeyFile == "" && !cfg.InsecureSkipVerify {
		return nil, nil
	}
	tlsCfg := &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify}
	if cfg.ServerName != "" {
		tlsCfg.ServerName = cfg.ServerName
	}
	if cfg.CAFile != "" {
		caPath := filepath.Clean(cfg.CAFile)
		pemData, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("read redis tls ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemData) {
			return nil, fmt.Errorf("redis tls ca is invalid")
		}
		tlsCfg.RootCAs = pool
	}
	if cfg.CertFile != "" || cfg.KeyFile != "" {
		certPath := filepath.Clean(cfg.CertFile)
		keyPath := filepath.Clean(cfg.KeyFile)
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("load redis tls certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return tlsCfg, nil
}
