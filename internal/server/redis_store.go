package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bitriver-live/internal/ctxutil"

	redis "github.com/redis/go-redis/v9"
)

type RedisTLSConfig struct {
	CAFile             string
	CertFile           string
	KeyFile            string
	ServerName         string
	InsecureSkipVerify bool
}

type redisStoreConfig struct {
	Addr       string
	Addrs      []string
	Username   string
	Password   string
	MasterName string
	Timeout    time.Duration
	PoolSize   int
	TLS        RedisTLSConfig
}

type redisStore struct {
	client  redis.UniversalClient
	timeout time.Duration
}

// newRedisStore builds and returns redis store using the supplied dependencies.
func newRedisStore(cfg redisStoreConfig) (*redisStore, error) {
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
		return nil, errors.New("redis addr required")
	}
	tlsConfig, err := buildRedisTLSConfig(cfg.TLS)
	if err != nil {
		return nil, err
	}
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        addrs,
		MasterName:   strings.TrimSpace(cfg.MasterName),
		Username:     strings.TrimSpace(cfg.Username),
		Password:     cfg.Password,
		TLSConfig:    tlsConfig,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  cfg.Timeout,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
		MaxRetries:   2,
	})
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &redisStore{client: client, timeout: timeout}, nil
}

// Allow performs allow and returns an error when dependent systems reject the operation.
func (s *redisStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	ctx = ctxutil.Normalize(ctx)
	timeout := s.timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	countReply, err := s.client.Do(ctx, "INCR", key).Result()
	if err != nil {
		return false, 0, err
	}
	count, err := toInt(countReply)
	if err != nil {
		return false, 0, err
	}
	if count == 1 {
		seconds := int64(window / time.Second)
		if seconds <= 0 {
			seconds = 1
		}
		if err := s.client.Do(ctx, "EXPIRE", key, seconds).Err(); err != nil {
			return false, 0, err
		}
	}
	if count <= int64(limit) {
		return true, 0, nil
	}
	ttlReply, err := s.client.Do(ctx, "TTL", key).Result()
	if err != nil {
		return false, 0, err
	}
	ttl, err := toInt(ttlReply)
	if err != nil {
		return false, 0, err
	}
	if ttl < 0 {
		return false, window, nil
	}
	return false, time.Duration(ttl) * time.Second, nil
}

// Close performs close and returns an error when dependent systems reject the operation.
func (s *redisStore) Close(context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

// Ping performs ping and returns an error when dependent systems reject the operation.
func (s *redisStore) Ping(ctx context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}
	ctx = ctxutil.Normalize(ctx)
	timeout := s.timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return s.client.Do(ctx, "PING").Err()
}

// toInt performs to int and propagates validation or dependency failures to the caller.
func toInt(v interface{}) (int64, error) {
	switch val := v.(type) {
	case int64:
		return val, nil
	case string:
		return strconv.ParseInt(val, 10, 64)
	case []byte:
		return strconv.ParseInt(string(val), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected redis reply type %T", v)
	}
}

// buildRedisTLSConfig builds redis tlsconfig from runtime state used by downstream handlers.
func buildRedisTLSConfig(cfg RedisTLSConfig) (*tls.Config, error) {
	if cfg.CAFile == "" && cfg.CertFile == "" && cfg.KeyFile == "" && !cfg.InsecureSkipVerify {
		return nil, nil
	}
	tlsCfg := &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify}
	if cfg.ServerName != "" {
		tlsCfg.ServerName = cfg.ServerName
	}
	if cfg.CAFile != "" {
		path := filepath.Clean(cfg.CAFile)
		pemData, err := os.ReadFile(path)
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
