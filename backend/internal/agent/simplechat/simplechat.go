package simplechat

import (
	"context"
	"fmt"

	"github.com/insmtx/SingerOS/backend/config"
	"github.com/insmtx/SingerOS/backend/internal/agent"
	"github.com/insmtx/SingerOS/backend/tools"
	skilltools "github.com/insmtx/SingerOS/backend/tools/skill"
)

type Runner struct {
	agent *agent.Agent
}

type Config struct {
	LLMProvider string
	APIKey      string
	Model       string
	BaseURL     string
}

func LoadFromEnv() *Config {
	return &Config{
		LLMProvider: "openai",
		APIKey:      "",
		Model:       "gpt-4",
		BaseURL:     "",
	}
}

func NewRunner(ctx context.Context, cfg *Config) (*Runner, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	llmConfig := &config.LLMConfig{
		Provider: cfg.LLMProvider,
		APIKey:   cfg.APIKey,
		Model:    cfg.Model,
		BaseURL:  cfg.BaseURL,
	}

	catalog := skilltools.NewEmptyCatalog()
	toolRegistry := tools.NewRegistry()

	runtimeConfig := agent.Config{
		SkillsCatalog: catalog,
		ToolRegistry:  toolRegistry,
	}

	agentInstance, err := agent.NewAgent(ctx, llmConfig, runtimeConfig)
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}

	return &Runner{
		agent: agentInstance,
	}, nil
}

type ChatRequest struct {
	Question string
	Sink     agent.RunEventSink
}

func (r *Runner) Ask(ctx context.Context, question string) (*agent.RunResult, error) {
	if question == "" {
		return nil, fmt.Errorf("question is required")
	}

	req := &agent.RequestContext{
		Input: agent.InputContext{
			Type: agent.InputTypeMessage,
			Text: question,
		},
	}

	return r.agent.Run(ctx, req)
}

func (r *Runner) ChatStream(ctx context.Context, req *ChatRequest) (*agent.RunResult, error) {
	if req.Question == "" {
		return nil, fmt.Errorf("question is required")
	}

	requestCtx := &agent.RequestContext{
		Input: agent.InputContext{
			Type: agent.InputTypeMessage,
			Text: req.Question,
		},
		EventSink: req.Sink,
	}

	return r.agent.Run(ctx, requestCtx)
}
