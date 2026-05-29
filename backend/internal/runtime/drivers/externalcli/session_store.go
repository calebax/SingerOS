package externalcli

import (
	"context"
	"sync"
	"time"

	"github.com/ygpkg/yg-go/logs"
)

const (
	externalCLISessionsMetadataKey = "external_cli_sessions"

	providerSessionStatusActive = "active"
	providerSessionStatusFailed = "failed"
)

// ProviderSessionKey 标识一个外部 CLI 会话绑定。
type ProviderSessionKey struct {
	InternalSessionID string
	Provider          string
	WorkDir           string
	AssistantID       string
}

// ProviderSessionBinding 将 Leros 会话映射到提供者原生 CLI 会话。
type ProviderSessionBinding struct {
	InternalSessionID string
	Provider          string
	ProviderSessionID string
	WorkDir           string
	AssistantID       string
	Status            string
	LastError         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ProviderSessionMetadata 在 Session.Metadata 中存储提供者原生会话信息。
type ProviderSessionMetadata struct {
	Provider          string    `json:"provider"`
	ProviderSessionID string    `json:"provider_session_id"`
	CreatedAt         time.Time `json:"created_at"`
}

// ProviderSessionStore 持久化提供者会话绑定，用于外部 CLI 恢复。
type ProviderSessionStore interface {
	Get(ctx context.Context, key ProviderSessionKey) (*ProviderSessionBinding, error)
	Upsert(ctx context.Context, binding *ProviderSessionBinding) error
	MarkFailed(ctx context.Context, key ProviderSessionKey, reason string) error
}

var (
	defaultStoreMu    sync.Mutex
	defaultStore      ProviderSessionStore
	defaultStoreReady bool
)

// DefaultProviderSessionStore 返回包级提供者会话存储，首次调用时自动初始化 SQLite。
func DefaultProviderSessionStore() ProviderSessionStore {
	defaultStoreMu.Lock()
	defer defaultStoreMu.Unlock()

	if defaultStoreReady {
		return defaultStore
	}
	defaultStoreReady = true

	var store ProviderSessionStore
	sqliteStore, err := newSQLiteProviderSessionStore()
	if err != nil {
		logs.Warnf("Provider session store unavailable, falling back to in-memory: %v", err)
		store = NewInMemoryProviderSessionStore()
	} else {
		store = sqliteStore
	}
	defaultStore = store
	return defaultStore
}

// SetDefaultProviderSessionStore 替换包级提供者会话存储（主要用于测试）。
func SetDefaultProviderSessionStore(store ProviderSessionStore) {
	if store == nil {
		return
	}
	defaultStoreMu.Lock()
	defer defaultStoreMu.Unlock()
	defaultStore = store
	defaultStoreReady = true
}
