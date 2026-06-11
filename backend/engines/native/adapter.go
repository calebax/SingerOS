// Package native implements the built-in Eino-backed Leros engine.
package native

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/insmtx/Leros/backend/engines"
	"github.com/insmtx/Leros/backend/internal/runtime/deps"
	"github.com/insmtx/Leros/backend/pkg/leros"
)

// EngineName is the registry name for the native engine.
const EngineName = engines.EngineNative

// Adapter implements engines.Engine for the in-process Eino runtime.
type Adapter struct {
	runner *Runner
	env    *deps.Container

	mu       sync.RWMutex
	skillDir string
}

// NewAdapter creates a native engine adapter.
func NewAdapter(env *deps.Container) (*Adapter, error) {
	runner, err := NewRunner(context.Background(), env)
	if err != nil {
		return nil, fmt.Errorf("create native runner: %w", err)
	}

	skillDir, err := leros.SkillsDir()
	if err != nil {
		skillDir = "" // best-effort
	}

	return &Adapter{
		runner:   runner,
		env:      env,
		skillDir: skillDir,
	}, nil
}

// Prepare satisfies engines.Engine.
func (a *Adapter) Prepare(_ context.Context, _ engines.PrepareRequest) error {
	return nil
}

// RegisterMCP satisfies engines.Engine.
func (a *Adapter) RegisterMCP(_ context.Context, _ engines.MCPServerConfig) error {
	return nil
}

// GetSkillDir satisfies engines.Engine.
func (a *Adapter) GetSkillDir() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.skillDir
}

// Run satisfies engines.Engine by delegating to the native Runner.
func (a *Adapter) Run(ctx context.Context, req engines.RunRequest) (*engines.RunHandle, error) {
	if a == nil || a.runner == nil {
		return nil, fmt.Errorf("native engine is not initialized")
	}

	eventsCh, err := a.runner.Run(ctx, req)
	if err != nil {
		return nil, err
	}

	return &engines.RunHandle{
		Process:   &noopProcess{pid: os.Getpid()},
		Events:    eventsCh,
		Responder: &noopResponder{},
	}, nil
}

// noopProcess satisfies engines.Process for the in-process engine.
type noopProcess struct {
	pid int
}

func (p *noopProcess) PID() int    { return p.pid }
func (p *noopProcess) Stop() error { return nil }

// noopResponder satisfies engines.ApprovalResponder.
type noopResponder struct{}

func (r *noopResponder) WriteDecision(string, string) error { return nil }

var _ engines.Engine = (*Adapter)(nil)
