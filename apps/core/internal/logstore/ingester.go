package logstore

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	mrand "math/rand/v2"
	"orion/core/internal/db"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// KeyPrefix starts every app ingest key.
const KeyPrefix = "orion_ik_"

var (
	// ErrInvalidKey is returned for a missing, unknown or revoked ingest key.
	ErrInvalidKey = errors.New("invalid ingest key")
	// ErrRateLimited carries how long the caller should wait.
	ErrRateLimited = errors.New("app ingest rate limit exceeded")
)

// RateLimitedError wraps ErrRateLimited with a Retry-After.
type RateLimitedError struct{ RetryAfter time.Duration }

func (e *RateLimitedError) Error() string { return ErrRateLimited.Error() }
func (e *RateLimitedError) Unwrap() error { return ErrRateLimited }

// GenerateKey returns a new ingest key and its stored hash and display prefix.
func GenerateKey() (key string, hash string, prefix string, err error) {
	var raw [20]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return "", "", "", err
	}
	key = KeyPrefix + hex.EncodeToString(raw[:])
	return key, HashKey(key), key[:len(KeyPrefix)+8], nil
}

// HashKey is the SHA-256 stored for a key.
func HashKey(key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return hex.EncodeToString(sum[:])
}

// AppContext is an authenticated app with its compiled redaction rules.
type AppContext struct {
	App      db.App
	KeyID    string
	Redactor *Redactor
	loadedAt time.Time
}

type cachedKey struct {
	ctx     *AppContext
	expires time.Time
}

// Ingester authenticates ingest keys and runs the ingest pipeline:
// parse, normalise, redact, sample, rate limit, enqueue.
type Ingester struct {
	main    *gorm.DB
	store   *Store
	limiter *AppLimiter

	mu       sync.Mutex
	cache    map[string]cachedKey // by key hash
	lastUsed map[string]time.Time // key id -> last persisted last_used_at
	ttl      time.Duration
	now      func() time.Time
}

func NewIngester(main *gorm.DB, store *Store, limiter *AppLimiter) *Ingester {
	return &Ingester{main: main, store: store, limiter: limiter, cache: map[string]cachedKey{}, lastUsed: map[string]time.Time{}, ttl: 5 * time.Second, now: time.Now}
}

// Invalidate drops cached key lookups so a revoked key or edited app takes effect immediately.
func (i *Ingester) Invalidate() {
	i.mu.Lock()
	i.cache = map[string]cachedKey{}
	i.mu.Unlock()
}

// Authenticate resolves an Authorization bearer value to an app.
func (i *Ingester) Authenticate(authorization string) (*AppContext, error) {
	token := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	if !strings.HasPrefix(token, KeyPrefix) {
		return nil, ErrInvalidKey
	}
	hash := HashKey(token)

	i.mu.Lock()
	if hit, ok := i.cache[hash]; ok && i.now().Before(hit.expires) {
		i.mu.Unlock()
		return hit.ctx, nil
	}
	i.mu.Unlock()

	var key db.AppIngestKey
	if err := i.main.Where("key_hash = ? AND revoked_at IS NULL", hash).First(&key).Error; err != nil {
		return nil, ErrInvalidKey
	}
	if subtle.ConstantTimeCompare([]byte(key.KeyHash), []byte(hash)) != 1 {
		return nil, ErrInvalidKey
	}
	var app db.App
	if err := i.main.Where("id = ?", key.AppID).First(&app).Error; err != nil {
		return nil, ErrInvalidKey
	}
	var redaction struct {
		ExtraKeys       []string `json:"extra_keys"`
		DisableDefaults bool     `json:"disable_defaults"`
	}
	_ = json.Unmarshal([]byte(app.RedactionJSON), &redaction)
	ctx := &AppContext{App: app, KeyID: key.ID, Redactor: NewRedactor(redaction.ExtraKeys, redaction.DisableDefaults), loadedAt: i.now()}

	i.mu.Lock()
	i.cache[hash] = cachedKey{ctx: ctx, expires: i.now().Add(i.ttl)}
	i.mu.Unlock()
	return ctx, nil
}

// Result summarises one ingest request.
type Result struct {
	Accepted int
	Rejected int
	Errors   []EntryError
}

// Ingest runs the pipeline for one authenticated request body.
func (i *Ingester) Ingest(app *AppContext, contentType string, body []byte) (Result, error) {
	now := i.now().UTC()
	parsed, err := ParseRequest(contentType, body, now)
	if err != nil {
		return Result{}, err
	}
	result := Result{Rejected: len(parsed.Errors), Errors: parsed.Errors}
	if len(parsed.Entries) == 0 {
		return result, nil
	}

	if ok, wait := i.limiter.Allow(app.App.ID, len(parsed.Entries)); !ok {
		return Result{}, &RateLimitedError{RetryAfter: wait}
	}

	kept := parsed.Entries[:0]
	for _, entry := range parsed.Entries {
		entry.AppID = app.App.ID
		entry.ID = NewEntryID(now)
		app.Redactor.Apply(&entry)
		if entry.SeverityNumber <= 8 && app.App.DebugSampleRate > 0 && app.App.DebugSampleRate < 1 && mrand.Float64() >= app.App.DebugSampleRate {
			result.Accepted++ // sampled out: accepted from the caller's point of view
			continue
		}
		kept = append(kept, entry)
	}
	if err := i.store.Enqueue(kept); err != nil {
		return Result{}, &RateLimitedError{RetryAfter: time.Second}
	}
	result.Accepted += len(kept)
	i.touch(app, now)
	return result, nil
}

// touch records last use at most once a minute per key and app.
func (i *Ingester) touch(app *AppContext, now time.Time) {
	i.mu.Lock()
	last := i.lastUsed[app.KeyID]
	due := now.Sub(last) > time.Minute
	if due {
		i.lastUsed[app.KeyID] = now
	}
	i.mu.Unlock()
	if !due {
		return
	}
	i.main.Model(&db.AppIngestKey{}).Where("id = ?", app.KeyID).Update("last_used_at", now)
	i.main.Model(&db.App{}).Where("id = ?", app.App.ID).Update("last_ingest_at", now)
}
