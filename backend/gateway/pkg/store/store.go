// Package store provides the adapter state persistence layer.
//
// The StateStore interface is the single contract for persisting adapter runtime
// state: credentials, tokens, cursors, context tokens, dedup ids, process PIDs.
// The default implementation uses the local filesystem under a configurable directory.
//
// Key format: {channel}/{identity}/{kind}
//
// Examples:
//   - qqbot/{app_id}/token
//   - feishu/{app_id}/dedup
//   - whatsapp/{session_hash}/pid
//   - wechat/{account_id}/context_tokens
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StateStore persists and retrieves adapter runtime state.
type StateStore interface {
	Get(ctx context.Context, key string, dest any) error
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// entry is the on-disk representation with optional TTL.
type entry struct {
	Value  json.RawMessage `json:"value"`
	Expiry int64           `json:"expiry,omitempty"` // unix timestamp, 0 = no expiry
}

// FileStore implements StateStore backed by the local filesystem.
//
// Each key is stored as a JSON file at {dir}/{key}.json. TTL is enforced
// at read time — expired entries are treated as non-existent.
type FileStore struct {
	dir string
	mu  sync.RWMutex
}

// NewFileStore creates a filesystem-backed state store.
//
// The directory is created if it does not exist. Permissions are 0700.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create state dir %s: %w", dir, err)
	}
	return &FileStore{dir: dir}, nil
}

// DefaultStateDir returns the default state directory.
func DefaultStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".singeros-state"
	}
	return filepath.Join(home, ".singeros", "state")
}

func (s *FileStore) filePath(key string) string {
	return filepath.Join(s.dir, key+".json")
}

// Get reads and decodes the value for a key. Returns nil if the key does not
// exist or has expired.
func (s *FileStore) Get(ctx context.Context, key string, dest any) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.filePath(key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read state %s: %w", key, err)
	}

	var ent entry
	if err := json.Unmarshal(data, &ent); err != nil {
		return fmt.Errorf("unmarshal state %s: %w", key, err)
	}

	// Check TTL
	if ent.Expiry > 0 && time.Now().Unix() > ent.Expiry {
		return nil // expired
	}

	if dest == nil {
		return nil
	}
	return json.Unmarshal(ent.Value, dest)
}

// Set writes a value with an optional TTL. A TTL of 0 means no expiry.
func (s *FileStore) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal state %s: %w", key, err)
	}

	var expiry int64
	if ttl > 0 {
		expiry = time.Now().Add(ttl).Unix()
	}

	ent := entry{Value: raw, Expiry: expiry}
	data, err := json.Marshal(ent)
	if err != nil {
		return fmt.Errorf("marshal entry %s: %w", key, err)
	}

	dir := filepath.Dir(s.filePath(key))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create state subdir: %w", err)
	}

	// Atomic write
	tmp := s.filePath(key) + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write temp state %s: %w", key, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return os.Rename(tmp, s.filePath(key))
}

// Delete removes the state entry for a key. No error if the key does not exist.
func (s *FileStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.filePath(key)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete state %s: %w", key, err)
	}
	return nil
}
