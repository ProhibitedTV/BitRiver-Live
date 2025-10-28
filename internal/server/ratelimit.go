package server

import (
	"fmt"
	"sync"
	"time"
)

type RateLimitConfig struct {
	GlobalRPS     float64
	GlobalBurst   int
	LoginLimit    int
	LoginWindow   time.Duration
	RedisAddr     string
	RedisPassword string
	RedisTimeout  time.Duration
}

type rateLimiter struct {
	global       *tokenBucket
	loginLimit   int
	loginWindow  time.Duration
	loginMu      sync.Mutex
	loginBuckets map[string]*ipLimiter
	store        tokenStore
}

type ipLimiter struct {
	bucket   *tokenBucket
	lastSeen time.Time
}

type tokenStore interface {
	Allow(key string, limit int, window time.Duration) (bool, time.Duration, error)
}

func newRateLimiter(cfg RateLimitConfig) *rateLimiter {
	rl := &rateLimiter{
		loginLimit:   cfg.LoginLimit,
		loginWindow:  cfg.LoginWindow,
		loginBuckets: make(map[string]*ipLimiter),
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
	if cfg.RedisAddr != "" && rl.loginLimit > 0 {
		timeout := cfg.RedisTimeout
		if timeout <= 0 {
			timeout = 2 * time.Second
		}
		rl.store = newRedisStore(cfg.RedisAddr, cfg.RedisPassword, timeout)
	}
	return rl
}

func (r *rateLimiter) AllowRequest() bool {
	if r == nil || r.global == nil {
		return true
	}
	return r.global.Allow()
}

func (r *rateLimiter) AllowLogin(key string) (bool, time.Duration, error) {
	if r == nil || r.loginLimit <= 0 {
		return true, 0, nil
	}
	if r.store != nil {
		allowed, retryAfter, err := r.store.Allow(fmt.Sprintf("bitriver:login:%s", key), r.loginLimit, r.loginWindow)
		return allowed, retryAfter, err
	}
	if key == "" {
		key = "unknown"
	}
	r.loginMu.Lock()
	bucket, exists := r.loginBuckets[key]
	if !exists {
		rate := float64(r.loginLimit) / r.loginWindow.Seconds()
		if rate <= 0 {
			rate = 1 / r.loginWindow.Seconds()
		}
		bucket = &ipLimiter{bucket: newTokenBucket(rate, r.loginLimit)}
		r.loginBuckets[key] = bucket
	}
	bucket.lastSeen = time.Now()
	r.cleanupLocked()
	r.loginMu.Unlock()

	if bucket.bucket.Allow() {
		return true, 0, nil
	}
	return false, time.Second, nil
}

func (r *rateLimiter) cleanupLocked() {
	if len(r.loginBuckets) == 0 {
		return
	}
	cutoff := time.Now().Add(-2 * r.loginWindow)
	for key, bucket := range r.loginBuckets {
		if bucket.lastSeen.Before(cutoff) {
			delete(r.loginBuckets, key)
		}
	}
}

type tokenBucket struct {
	mu        sync.Mutex
	rate      float64
	capacity  float64
	tokens    float64
	lastCheck time.Time
}

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
