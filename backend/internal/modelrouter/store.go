package modelrouter

import (
	"context"
	"fmt"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/insmtx/Leros/backend/internal/llm"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// ModelStore — minimal in-handler model config resolution
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// BusinessIDs holds business identifiers for a single run.
type BusinessIDs struct {
	ProjectID   uint
	SessionID   uint
	AssistantID uint
	Uin         uint
}

// ModelStore holds UpstreamConfig entries keyed by model name.
// It is safe for concurrent use.
type ModelStore struct {
	configs map[string]*UpstreamConfig
	caller  llm.Caller
	orgID   uint
	biz     *BusinessIDs
	mu      sync.RWMutex
}

// NewModelStore creates an isolated model routing store.
func NewModelStore() *ModelStore {
	return &ModelStore{configs: make(map[string]*UpstreamConfig)}
}

// SetCaller sets the llm.Caller used for upstream LLM calls.
func (s *ModelStore) SetCaller(caller llm.Caller) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.caller = caller
}

// SetOrgID sets the org ID used for call recording.
func (s *ModelStore) SetOrgID(orgID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orgID = orgID
}

// PutBiz stores business identifiers for the current run.
func (s *ModelStore) PutBiz(b BusinessIDs) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.biz = &b
}

// GetBiz returns a copy of the stored business identifiers, or nil if not set.
func (s *ModelStore) GetBiz() *BusinessIDs {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.biz == nil {
		return nil
	}
	cp := *s.biz
	return &cp
}

// Put registers an upstream configuration for a model.
// If cfg.Protocol is not explicitly set and cfg.Provider is non-empty,
// the protocol is inferred from the provider.
func (s *ModelStore) Put(cfg UpstreamConfig) {
	if cfg.Protocol == "" && cfg.Provider != "" {
		cfg.Protocol = protocolForProvider(cfg.Provider)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.configs == nil {
		s.configs = make(map[string]*UpstreamConfig)
	}
	cp := cfg
	s.configs[cfg.ModelName] = &cp
}

// Resolve returns the UpstreamConfig for the given model name.
func (s *ModelStore) Resolve(model string) (*UpstreamConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.configs[model]
	if !ok {
		return nil, fmt.Errorf("modelrouter: no upstream config for model %q", model)
	}
	cp := *cfg
	return &cp, nil
}

func (s *ModelStore) getCaller() llm.Caller {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.caller
}

func (s *ModelStore) getOrgID() uint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.orgID
}

// injectBusinessIDs 从 ModelStore 中提取当前 run 的业务 ID，注入到 context 中。
func injectBusinessIDs(ctx context.Context, store *ModelStore, c *gin.Context) context.Context {
	biz := store.GetBiz()
	if biz != nil {
		ctx = llm.WithCtxUint(ctx, llm.CtxProjectID, biz.ProjectID)
		ctx = llm.WithCtxUint(ctx, llm.CtxSessionID, biz.SessionID)
		ctx = llm.WithCtxUint(ctx, llm.CtxAssistantID, biz.AssistantID)
		ctx = llm.WithCtxUint(ctx, llm.CtxUin, biz.Uin)
	}
	if clientIP := c.ClientIP(); clientIP != "" {
		ctx = llm.WithCtxString(ctx, llm.CtxClientIP, clientIP)
	}
	return ctx
}
