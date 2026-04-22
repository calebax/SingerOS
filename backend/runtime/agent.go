// Package runtime defines the unified agent.run boundary for SingerOS.
package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
	auth "github.com/insmtx/SingerOS/backend/auth"
	"github.com/insmtx/SingerOS/backend/config"
	runtimeeino "github.com/insmtx/SingerOS/backend/runtime/eino"
	runtimeevents "github.com/insmtx/SingerOS/backend/runtime/events"
	runtimeprompt "github.com/insmtx/SingerOS/backend/runtime/prompt"
	"github.com/insmtx/SingerOS/backend/toolruntime"
	"github.com/insmtx/SingerOS/backend/tools"
	"github.com/ygpkg/yg-go/logs"
)

const defaultAgentSystemPrompt = "You are the SingerOS agent runtime. Use available skills and tools to analyze incoming events and respond with concrete, evidence-based actions."

// Agent is the SingerOS runtime agent entrypoint.
type Agent struct {
	chatModel    einomodel.ToolCallingChatModel
	toolAdapter  *runtimeeino.ToolAdapter
	toolRegistry *tools.Registry
	skills       *runtimeprompt.SkillsContext
	tools        *runtimeprompt.ToolsContext
	systemPrompt string
}

// NewAgent creates the SingerOS agent backed by the Eino flow framework.
func NewAgent(ctx context.Context, llmConfig *config.LLMConfig, runtimeConfig Config) (*Agent, error) {
	if llmConfig == nil {
		return nil, fmt.Errorf("llm config is required")
	}
	if runtimeConfig.ToolRegistry == nil {
		return nil, fmt.Errorf("tool registry is required")
	}
	toolRuntime := toolruntime.New(runtimeConfig.ToolRegistry, nil)

	chatModel, err := runtimeeino.NewOpenAIChatModel(ctx, llmConfig)
	if err != nil {
		return nil, err
	}

	skillsContext, err := runtimeprompt.BuildSkillsContext(runtimeConfig.SkillsCatalog)
	if err != nil {
		return nil, err
	}

	return &Agent{
		chatModel:    chatModel,
		toolAdapter:  runtimeeino.NewToolAdapter(runtimeConfig.ToolRegistry, toolRuntime),
		toolRegistry: runtimeConfig.ToolRegistry,
		skills:       skillsContext,
		tools:        runtimeprompt.BuildToolsContext(runtimeConfig.ToolRegistry),
		systemPrompt: defaultAgentSystemPrompt,
	}, nil
}

// Run executes one normalized request through the SingerOS agent.
func (a *Agent) Run(ctx context.Context, req *RequestContext) (*RunResult, error) {
	startedAt := time.Now().UTC()
	if a == nil || a.chatModel == nil {
		return nil, fmt.Errorf("eino chat model is not initialized")
	}

	state, err := a.buildRunState(req)
	if err != nil {
		return nil, err
	}
	req = state.req

	if err := emitRunEvent(ctx, state.emitter, req, RunEventStarted, nil); err != nil {
		return nil, err
	}

	flow, err := runtimeeino.NewFlow(ctx, &runtimeeino.FlowConfig{
		Model:        a.chatModel,
		ToolAdapter:  a.toolAdapter,
		Binding:      state.toolBinding,
		SystemPrompt: state.systemPrompt,
		Skills:       a.skills,
		Tools:        state.tools,
		MaxStep:      state.maxStep,
	})
	if err != nil {
		emitRunError(ctx, state.emitter, req, err)
		return nil, err
	}

	var message interface {
		String() string
	}
	var resultMessage string
	var usage *UsagePayload
	if req.EventSink != nil {
		streamedMessage, streamErr := flow.Stream(ctx, state.userInput, state.emitter)
		err = streamErr
		if streamedMessage != nil {
			message = streamedMessage
			resultMessage = strings.TrimSpace(streamedMessage.Content)
			usage = usageFromResponseMeta(streamedMessage.ResponseMeta)
		}
	} else {
		generatedMessage, generateErr := flow.Generate(ctx, state.userInput)
		err = generateErr
		if generatedMessage != nil {
			message = generatedMessage
			resultMessage = strings.TrimSpace(generatedMessage.Content)
			usage = usageFromResponseMeta(generatedMessage.ResponseMeta)
		}
	}
	if err != nil {
		emitRunError(ctx, state.emitter, req, err)
		return nil, err
	}
	if resultMessage == "" && message != nil {
		resultMessage = formatLLMResultForLog(message)
	}

	result := &RunResult{
		RunID:       req.RunID,
		TraceID:     req.TraceID,
		Status:      RunStatusCompleted,
		Message:     resultMessage,
		Usage:       usage,
		StartedAt:   startedAt,
		CompletedAt: time.Now().UTC(),
	}

	if usage != nil {
		_ = state.emitter.Emit(ctx, &RunEvent{
			Type:    RunEventUsage,
			Content: eventContentJSON(usage),
		})
	}
	if err := emitRunEvent(ctx, state.emitter, req, RunEventCompleted, result); err != nil {
		return nil, err
	}

	logs.InfoContextf(ctx, "SingerOS runtime final LLM result: run_id=%s actor=%s result=%s",
		req.RunID, req.Actor.UserID, formatLLMResultForLog(message))

	return result, nil
}

func (a *Agent) buildRunState(req *RequestContext) (*runState, error) {
	if req == nil {
		return nil, errors.New("request context is required")
	}
	ensureRunDefaults(req)

	userInput := buildUserInput(req)
	if userInput == "" {
		userInput = string(req.Input.Type)
	}

	emitter := runtimeevents.NewEmitter(req.RunID, req.TraceID, sinkForRequest(req))
	toolsContext := a.tools
	if len(req.Capability.AllowedTools) > 0 {
		var err error
		toolsContext, err = runtimeprompt.BuildToolsContextForNames(a.toolRegistry, req.Capability.AllowedTools)
		if err != nil {
			return nil, err
		}
	}

	return &runState{
		req:          req,
		emitter:      emitter,
		userInput:    userInput,
		systemPrompt: a.systemPromptForRequest(req),
		toolBinding: runtimeeino.ToolBinding{
			Selector:     authSelectorFromRequest(req),
			UserID:       req.Actor.UserID,
			AccountID:    req.Actor.AccountID,
			AllowedTools: req.Capability.AllowedTools,
			Emitter:      emitter,
			EmitToolIO:   req.EventSink != nil,
		},
		tools:   toolsContext,
		maxStep: maxStepForRequest(req),
	}, nil
}

func buildUserInput(req *RequestContext) string {
	if req == nil {
		return ""
	}

	switch {
	case strings.TrimSpace(req.Input.Text) != "":
		return strings.TrimSpace(req.Input.Text)
	case len(req.Input.Messages) > 0:
		lines := make([]string, 0, len(req.Input.Messages))
		for _, message := range req.Input.Messages {
			if strings.TrimSpace(message.Content) == "" {
				continue
			}
			role := message.Role
			if role == "" {
				role = "user"
			}
			lines = append(lines, fmt.Sprintf("%s: %s", role, message.Content))
		}
		return strings.Join(lines, "\n")
	default:
		return string(req.Input.Type)
	}
}

func authSelectorFromRequest(req *RequestContext) *auth.AuthSelector {
	selector := &auth.AuthSelector{
		ScopeType: auth.ScopeTypeEvent,
	}
	if req == nil {
		return selector
	}
	selector.ScopeID = req.RunID

	externalRefs := make(map[string]string)
	if provider := metadataString(req.Metadata, "provider"); provider != "" {
		selector.Provider = provider
	}
	eventContext := metadataMap(req.Metadata, "event_context")
	if provider := metadataString(eventContext, "provider"); selector.Provider == "" && provider != "" {
		selector.Provider = provider
	}
	for key, value := range eventContext {
		if stringValue, ok := value.(string); ok && stringValue != "" {
			externalRefs[fmt.Sprintf("context.%s", key)] = stringValue
		}
	}
	eventPayload := metadataMap(req.Metadata, "event_payload")
	if installationID := nestedString(eventPayload, "installation", "id"); installationID != "" {
		externalRefs["github.installation_id"] = installationID
	}
	if senderID := nestedString(eventPayload, "sender", "id"); senderID != "" {
		externalRefs["github.sender_id"] = senderID
	}
	if senderLogin := nestedString(eventPayload, "sender", "login"); senderLogin != "" {
		externalRefs["github.sender_login"] = senderLogin
		selector.SubjectType = auth.SubjectTypeUser
		selector.SubjectID = senderLogin
	}
	if selector.SubjectID == "" && req.Actor.UserID != "" {
		selector.SubjectType = auth.SubjectTypeUser
		selector.SubjectID = req.Actor.UserID
	}
	if req.Actor.AccountID != "" {
		selector.ExplicitProfileID = req.Actor.AccountID
	}
	if len(externalRefs) > 0 {
		selector.ExternalRefs = externalRefs
	}
	return selector
}

func metadataString(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	value, _ := metadata[key].(string)
	return value
}

func metadataMap(metadata map[string]any, key string) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	if typed, ok := metadata[key].(map[string]any); ok {
		return typed
	}
	return nil
}

func nestedString(payload map[string]any, path ...string) string {
	var current interface{} = payload
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = object[key]
	}
	switch value := current.(type) {
	case string:
		return value
	case float64:
		return fmt.Sprintf("%.0f", value)
	case int:
		return fmt.Sprintf("%d", value)
	case int64:
		return fmt.Sprintf("%d", value)
	default:
		return ""
	}
}

func (a *Agent) systemPromptForRequest(req *RequestContext) string {
	prompt := strings.TrimSpace(a.systemPrompt)
	if req != nil && strings.TrimSpace(req.Assistant.SystemPrompt) != "" {
		if prompt == "" {
			prompt = strings.TrimSpace(req.Assistant.SystemPrompt)
		} else {
			prompt += "\n\n" + strings.TrimSpace(req.Assistant.SystemPrompt)
		}
	}
	if req == nil {
		return prompt
	}

	switch metadataString(req.Metadata, "event_type") {
	case "pull_request", "github.pull_request", "github.pull_request.opened":
		extra := "For GitHub pull request events, start from the event payload, then use GitHub tools to inspect metadata, changed files, and only the most relevant files before deciding whether to publish a review. Prefer COMMENT by default. Do not auto-approve. Use REQUEST_CHANGES only when you have concrete merge-blocking evidence."
		if prompt == "" {
			return extra
		}
		return prompt + "\n\n" + extra
	case "push", "github.push":
		extra := "For GitHub push events, apply the same code review conventions used for pull requests. Start from the raw payload, use compare-commits style GitHub tools to inspect the diff, then read only the most relevant files before writing findings. If there is no PR review target, still produce a concise code review assessment."
		if prompt == "" {
			return extra
		}
		return prompt + "\n\n" + extra
	default:
		return prompt
	}
}

func ensureRunDefaults(req *RequestContext) {
	if req.RunID == "" {
		req.RunID = fmt.Sprintf("run_%d", time.Now().UTC().UnixNano())
	}
	if req.Input.Type == "" {
		req.Input.Type = InputTypeMessage
	}
}

func maxStepForRequest(req *RequestContext) int {
	if req != nil && req.Runtime.MaxStep > 0 {
		return req.Runtime.MaxStep
	}
	return 12
}

func sinkForRequest(req *RequestContext) runtimeevents.EventSink {
	if req == nil || req.EventSink == nil {
		return runtimeevents.NewNoopSink()
	}
	return req.EventSink
}

func emitRunEvent(ctx context.Context, emitter *runtimeevents.Emitter, req *RequestContext, eventType RunEventType, result *RunResult) error {
	event := &RunEvent{Type: eventType}
	if result != nil {
		event.Content = result.Message
	}
	_ = emitter.Emit(ctx, event)
	return nil
}

func emitRunError(ctx context.Context, emitter *runtimeevents.Emitter, req *RequestContext, err error) {
	if err == nil {
		return
	}
	eventType := RunEventFailed
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		eventType = RunEventCancelled
	}
	_ = emitter.Emit(ctx, &RunEvent{
		Type:    eventType,
		Content: err.Error(),
	})
}

func eventContentJSON(value interface{}) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(encoded)
}

func usageFromResponseMeta(meta *einoschema.ResponseMeta) *UsagePayload {
	if meta == nil || meta.Usage == nil {
		return nil
	}
	return &UsagePayload{
		InputTokens:  meta.Usage.PromptTokens,
		OutputTokens: meta.Usage.CompletionTokens,
		TotalTokens:  meta.Usage.TotalTokens,
	}
}

func formatLLMResultForLog(message interface{ String() string }) string {
	if message == nil {
		return "<nil>"
	}

	formatted := strings.TrimSpace(message.String())
	if formatted == "" {
		return "<empty>"
	}
	if len(formatted) > 2000 {
		return formatted[:2000] + "...(truncated)"
	}
	return formatted
}
