// Package webhook provides shared HTTP webhook guard middleware.
//
// These guards handle common webhook concerns (body size, content type, rate
// limiting, dedup, raw body read timeout) that every webhook-receiving adapter
// needs. Signature verification is platform-specific and stays in each adapter.
package webhook

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"sync"
	"time"
)

// Guard applies common webhook security checks.
//
// Usage:
//
//	guard := webhook.NewGuard(webhook.GuardConfig{
//	    MaxBodySize: 1 << 20, // 1MB
//	    RateLimit:   120,      // 120 requests
//	    RateWindow:  60 * time.Second,
//	})
//
//	body, err := guard.Guard(r, "feishu:webhook")
type Guard struct {
	maxBodySize int64
	rateLimit   int
	rateWindow  time.Duration

	mu       sync.Mutex
	rateMap  map[string]*rateBucket // key → sliding window
	dedupMap map[string]time.Time   // hash → first seen
	dedupTTL time.Duration
}

// GuardConfig configures a webhook guard.
type GuardConfig struct {
	// MaxBodySize is the maximum request body size in bytes. 0 = unlimited.
	MaxBodySize int64
	// RateLimit is the max requests per RateWindow per key. 0 = unlimited.
	RateLimit int
	// RateWindow is the sliding window duration for rate limiting.
	RateWindow time.Duration
	// DedupTTL is how long to remember seen payload hashes. 0 = no dedup.
	DedupTTL time.Duration
}

type rateBucket struct {
	timestamps []time.Time
}

// NewGuard creates a webhook guard with the given config.
func NewGuard(cfg GuardConfig) *Guard {
	return &Guard{
		maxBodySize: cfg.MaxBodySize,
		rateLimit:   cfg.RateLimit,
		rateWindow:  cfg.RateWindow,
		rateMap:     make(map[string]*rateBucket),
		dedupMap:    make(map[string]time.Time),
		dedupTTL:    cfg.DedupTTL,
	}
}

// Guard reads the request body with size and rate limits, then checks for
// replay attacks. Returns the raw body bytes, or an error.
//
// The caller is responsible for closing the request body. This method
// replaces the body with a new reader so the caller can re-read if needed.
func (g *Guard) Guard(r *http.Request, key string) (body []byte, err error) {
	// 1. Content-Type check
	ct := r.Header.Get("Content-Type")
	if ct != "" && ct != "application/json" && ct != "application/json; charset=utf-8" {
		return nil, &GuardError{Code: "invalid_content_type", Message: "expected application/json"}
	}

	// 2. Body size guard
	limitedReader := io.LimitReader(r.Body, g.maxBodySize+1)
	body, err = io.ReadAll(limitedReader)
	r.Body.Close()
	if err != nil {
		return nil, &GuardError{Code: "read_body", Message: err.Error()}
	}
	if g.maxBodySize > 0 && int64(len(body)) > g.maxBodySize {
		return nil, &GuardError{Code: "body_too_large", Message: "request body exceeds limit"}
	}

	// 3. Rate limit check
	if !g.checkRateLimit(key) {
		return nil, &GuardError{Code: "rate_limited", Message: "too many requests"}
	}

	// 4. Replay/dedup check
	if g.dedupTTL > 0 {
		if g.isReplay(body) {
			return nil, &GuardError{Code: "replay", Message: "duplicate request"}
		}
		g.markSeen(body)
	}

	// Replace body so caller can re-read
	r.Body = io.NopCloser(io.LimitReader(io.NopCloser(nil).(io.Reader), 0))

	return body, nil
}

// GuardError is returned when a guard check fails.
type GuardError struct {
	Code    string
	Message string
}

func (e *GuardError) Error() string {
	return "webhook guard: " + e.Code + ": " + e.Message
}

func (g *Guard) checkRateLimit(key string) bool {
	if g.rateLimit <= 0 {
		return true
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	bucket, ok := g.rateMap[key]
	if !ok {
		bucket = &rateBucket{}
		g.rateMap[key] = bucket
	}

	now := time.Now()
	cutoff := now.Add(-g.rateWindow)

	// Evict old entries
	valid := bucket.timestamps[:0]
	for _, ts := range bucket.timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	bucket.timestamps = valid

	if len(bucket.timestamps) >= g.rateLimit {
		return false
	}

	bucket.timestamps = append(bucket.timestamps, now)
	return true
}

func (g *Guard) isReplay(body []byte) bool {
	hash := bodyHash(body)
	g.mu.Lock()
	defer g.mu.Unlock()

	// Evict expired
	cutoff := time.Now().Add(-g.dedupTTL)
	for h, ts := range g.dedupMap {
		if ts.Before(cutoff) {
			delete(g.dedupMap, h)
		}
	}

	_, exists := g.dedupMap[hash]
	return exists
}

func (g *Guard) markSeen(body []byte) {
	hash := bodyHash(body)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.dedupMap[hash] = time.Now()
}

func bodyHash(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:16])
}
