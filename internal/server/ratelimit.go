package server

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type RateLimitConfig struct {
	GlobalRPS              float64
	GlobalBurst            int
	LoginLimit             int
	LoginWindow            time.Duration
	RequireLoginProtection bool
	TrustForwardedHeaders  bool
	TrustedProxies         []string
	RedisAddr              string
	RedisAddrs             []string
	RedisUsername          string
	RedisPassword          string
	RedisMasterName        string
	RedisTimeout           time.Duration
	RedisPoolSize          int
	RedisTLS               RedisTLSConfig
}

type rateLimiter struct {
	global           *tokenBucket
	loginLimit       int
	loginWindow      time.Duration
	lastLoginCleanup time.Time
	loginMu          sync.Mutex
	loginBuckets     map[string]*ipLimiter
	store            tokenStore
	now              func() time.Time
}

type ipLimiter struct {
	bucket   *tokenBucket
	lastSeen time.Time
}

type tokenStore interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error)
}

// newRateLimiter builds a rate limiter from config.
func newRateLimiter(cfg RateLimitConfig) (*rateLimiter, error) {
	if cfg.RequireLoginProtection && cfg.LoginLimit <= 0 {
		return nil, errors.New("login rate limiting required in production; set BITRIVER_LIVE_RATE_LOGIN_LIMIT or --rate-login-limit")
	}

	rl := &rateLimiter{
		loginLimit:   cfg.LoginLimit,
		loginWindow:  cfg.LoginWindow,
		loginBuckets: make(map[string]*ipLimiter),
		now:          time.Now,
	}
	if cfg.GlobalRPS > 0 {
		burst := cfg.GlobalBurst
		if burst <= 0 {
			burst = int(cfg.GlobalRPS)
			if burst < 1 {
				burst = 1
			}
		}
		rl.global = newTokenBucket(cfg.GlobalRPS, burst)
	}
	if rl.loginLimit <= 0 {
		rl.loginLimit = 0
	}
	if rl.loginWindow <= 0 {
		rl.loginWindow = time.Minute
	}
	if rl.loginLimit > 0 && (cfg.RedisAddr != "" || len(cfg.RedisAddrs) > 0) {
		storeCfg := redisStoreConfig{
			Addr:       cfg.RedisAddr,
			Addrs:      cfg.RedisAddrs,
			Username:   cfg.RedisUsername,
			Password:   cfg.RedisPassword,
			MasterName: cfg.RedisMasterName,
			Timeout:    cfg.RedisTimeout,
			PoolSize:   cfg.RedisPoolSize,
			TLS:        cfg.RedisTLS,
		}
		store, err := newRedisStore(storeCfg)
		if err != nil {
			return nil, err
		}
		rl.store = store
	}
	return rl, nil
}

// AllowRequest checks the global request token bucket.
func (r *rateLimiter) AllowRequest() bool {
	if r == nil || r.global == nil {
		return true
	}
	return r.global.Allow()
}

// AllowLogin enforces per-key login limits.
func (r *rateLimiter) AllowLogin(ctx context.Context, key string) (bool, time.Duration, error) {
	if r == nil || r.loginLimit <= 0 {
		return true, 0, nil
	}
	if r.store != nil {
		allowed, retryAfter, err := r.store.Allow(ctx, fmt.Sprintf("bitriver:login:%s", key), r.loginLimit, r.loginWindow)
		return allowed, retryAfter, err
	}
	if key == "" {
		key = "unknown"
	}
	r.loginMu.Lock()
	now := r.currentTime()
	bucket, exists := r.loginBuckets[key]
	if !exists {
		rate := float64(r.loginLimit) / r.loginWindow.Seconds()
		if rate <= 0 {
			rate = 1 / r.loginWindow.Seconds()
		}
		bucket = &ipLimiter{bucket: newTokenBucket(rate, r.loginLimit)}
		r.loginBuckets[key] = bucket
	}
	bucket.lastSeen = now
	cleanupInterval := r.loginWindow / 2
	if cleanupInterval < time.Second {
		cleanupInterval = time.Second
	}
	if r.lastLoginCleanup.IsZero() || now.Sub(r.lastLoginCleanup) >= cleanupInterval {
		r.cleanupLocked(now)
		r.lastLoginCleanup = now
	}
	r.loginMu.Unlock()

	if bucket.bucket.Allow() {
		return true, 0, nil
	}
	return false, time.Second, nil
}

func (r *rateLimiter) currentTime() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

// cleanupLocked removes stale in-memory login buckets.
func (r *rateLimiter) cleanupLocked(now time.Time) {
	if len(r.loginBuckets) == 0 {
		return
	}
	cutoff := now.Add(-2 * r.loginWindow)
	for key, bucket := range r.loginBuckets {
		if bucket.lastSeen.Before(cutoff) {
			delete(r.loginBuckets, key)
		}
	}
}

// Ping checks health of the backing distributed store, when configured.
func (r *rateLimiter) Ping(ctx context.Context) error {
	if r == nil || r.store == nil {
		return nil
	}
	if pinger, ok := r.store.(interface{ Ping(context.Context) error }); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

type tokenBucket struct {
	mu        sync.Mutex
	rate      float64
	capacity  float64
	tokens    float64
	lastCheck time.Time
}

// newTokenBucket creates a token bucket with sane minimums.
func newTokenBucket(rate float64, burst int) *tokenBucket {
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = 1
	}
	now := time.Now()
	return &tokenBucket{
		rate:      rate,
		capacity:  float64(burst),
		tokens:    float64(burst),
		lastCheck: now,
	}
}

// Allow consumes one token when available.
func (tb *tokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(tb.lastCheck).Seconds()
	tb.lastCheck = now
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	if tb.tokens < 1 {
		return false
	}
	tb.tokens -= 1
	return true
}
