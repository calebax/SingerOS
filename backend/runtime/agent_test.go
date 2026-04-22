package runtime

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/insmtx/SingerOS/backend/config"
	runtimeevents "github.com/insmtx/SingerOS/backend/runtime/events"
	"github.com/insmtx/SingerOS/backend/tools"
	nodetools "github.com/insmtx/SingerOS/backend/tools/node"
	"github.com/ygpkg/yg-go/logs"
	"go.uber.org/zap/zapcore"
)

func TestAgentRunRealModel(t *testing.T) {
	logs.SetLevel(zapcore.DebugLevel)

	apiKey := firstNonEmptyEnv("SINGEROS_LLM_API_KEY")
	if apiKey == "" {
		t.Skip("set SINGEROS_LLM_API_KEY to run the real model agent test")
	}

	ctx, cancel := realModelTestContext(t)
	defer cancel()

	registry := tools.NewRegistry()
	agent, err := NewAgent(ctx, &config.LLMConfig{
		Provider: "openai",
		APIKey:   apiKey,
		Model:    firstNonEmptyEnv("SINGEROS_LLM_MODEL"),
		BaseURL:  firstNonEmptyEnv("SINGEROS_LLM_BASE_URL"),
	}, Config{
		ToolRegistry: registry,
	})
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}

	result, err := agent.Run(ctx, &RequestContext{
		RunID: "run_real_model_message",
		Actor: ActorContext{
			UserID:  "test-user",
			Channel: "test",
		},
		Input: InputContext{
			Type: InputTypeMessage,
			Text: "Reply with exactly this text: SingerOS agent runtime ok",
		},
		Runtime:   RuntimeOptions{MaxStep: 2},
		EventSink: runtimeevents.NewLogSink(),
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	if result == nil {
		t.Fatalf("expected result")
	}
	if result.Status != RunStatusCompleted {
		t.Fatalf("expected completed result, got %+v", result)
	}
	if strings.TrimSpace(result.Message) == "" {
		t.Fatalf("expected non-empty model response")
	}
	if !strings.Contains(result.Message, "SingerOS agent runtime ok") {
		t.Fatalf("unexpected model response: %s", result.Message)
	}
}

func TestAgentRunNodeTool(t *testing.T) {
	logs.SetLevel(zapcore.DebugLevel)

	apiKey := firstNonEmptyEnv("SINGEROS_LLM_API_KEY")
	if apiKey == "" {
		t.Skip("set SINGEROS_LLM_API_KEY to run the real model agent tool-call test")
	}

	ctx, cancel := realModelTestContext(t)
	defer cancel()

	registry := tools.NewRegistry()
	if err := nodetools.Register(registry); err != nil {
		t.Fatalf("register node tools: %v", err)
	}

	agent, err := NewAgent(ctx, &config.LLMConfig{
		Provider: "openai",
		APIKey:   apiKey,
		Model:    firstNonEmptyEnv("SINGEROS_LLM_MODEL"),
		BaseURL:  firstNonEmptyEnv("SINGEROS_LLM_BASE_URL"),
	}, Config{
		ToolRegistry: registry,
	})
	if err != nil {
		t.Fatalf("new agent: %v", err)
	}

	sink := &recordingEventSink{}
	result, err := agent.Run(ctx, &RequestContext{
		RunID: "run_real_model_node_shell_time",
		Assistant: AssistantContext{
			ID:   "test-assistant",
			Name: "Tool Test Assistant",
			SystemPrompt: strings.Join([]string{
				"你必须使用工具完成用户任务，不能凭空回答。",
				"查询当前系统时间时必须调用 node_shell。",
				"node_shell 的 container_id 必须使用 b327e241316c2a2f62cbee986edd0e71235205f0fde5dc7a4543f5344396b351。",
				"调用命令使用 date '+%Y-%m-%d %H:%M:%S %Z'。",
				"最终答案只需要总结工具返回的当前时间。",
			}, "\n"),
		},
		Actor: ActorContext{
			UserID:  "test-user",
			Channel: "test",
		},
		Input: InputContext{
			Type: InputTypeTaskInstruction,
			Text: "使用工具查询当前系统时间。",
		},
		Runtime: RuntimeOptions{MaxStep: 6},
		Capability: CapabilityContext{
			AllowedTools: []string{
				nodetools.ToolNameNodeShell,
				nodetools.ToolNameNodeFileRead,
				nodetools.ToolNameNodeFileWrite,
			},
		},
		EventSink: sink,
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	if result == nil {
		t.Fatalf("expected result")
	}
	if result.Status != RunStatusCompleted {
		t.Fatalf("expected completed result, got %+v", result)
	}
	if strings.TrimSpace(result.Message) == "" {
		t.Fatalf("expected non-empty model response")
	}

	toolEvent := sink.firstToolEvent(runtimeevents.RunEventToolCallCompleted, nodetools.ToolNameNodeShell)
	if toolEvent == nil {
		t.Fatalf("expected completed %s tool call, events=%s", nodetools.ToolNameNodeShell, sink.eventSummary())
	}
	if !strings.Contains(toolEvent.Content, nodetools.ToolNameNodeShell) {
		t.Fatalf("expected %s tool event content, got %s", nodetools.ToolNameNodeShell, toolEvent.Content)
	}
	if !strings.Contains(toolEvent.Content, "[exit_code=0]") {
		t.Fatalf("expected %s content to contain exit_code=0, got %s", nodetools.ToolNameNodeShell, toolEvent.Content)
	}
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func realModelTestContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	timeoutValue := strings.TrimSpace(os.Getenv("SINGEROS_TEST_TIMEOUT"))
	if timeoutValue == "" {
		timeoutValue = "3m"
	}
	if timeoutValue == "0" || strings.EqualFold(timeoutValue, "none") {
		return context.Background(), func() {}
	}

	timeout, err := time.ParseDuration(timeoutValue)
	if err != nil {
		t.Fatalf("parse SINGEROS_TEST_TIMEOUT: %v", err)
	}
	return context.WithTimeout(context.Background(), timeout)
}

type recordingEventSink struct {
	mu     sync.Mutex
	events []*runtimeevents.RunEvent
}

func (s *recordingEventSink) Emit(ctx context.Context, event *runtimeevents.RunEvent) error {
	if event == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	copied := *event
	s.events = append(s.events, &copied)
	return nil
}

func (s *recordingEventSink) firstToolEvent(eventType runtimeevents.RunEventType, toolName string) *runtimeevents.RunEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, event := range s.events {
		if event == nil || event.Type != eventType {
			continue
		}
		if strings.Contains(event.Content, toolName) {
			return event
		}
	}
	return nil
}

func (s *recordingEventSink) eventSummary() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	parts := make([]string, 0, len(s.events))
	for _, event := range s.events {
		if event == nil {
			continue
		}
		if event.Content != "" {
			parts = append(parts, string(event.Type)+":"+event.Content)
			continue
		}
		parts = append(parts, string(event.Type))
	}
	return strings.Join(parts, ", ")
}
