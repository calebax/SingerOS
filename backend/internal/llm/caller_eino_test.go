package llm

import (
	"context"
	"testing"

	einoschema "github.com/cloudwego/eino/schema"
)

func TestBuildEinoMessagesWithSystemPrompt(t *testing.T) {
	req := &CallRequest{
		SystemPrompt: "You are a helpful assistant.",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
			{Role: "user", Content: "How are you?"},
		},
	}

	msgs := buildEinoMessages(req)
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages (1 system + 3 user), got %d", len(msgs))
	}

	if msgs[0].Role != einoschema.System {
		t.Fatalf("expected first message role=system, got %q", msgs[0].Role)
	}
	if msgs[0].Content != "You are a helpful assistant." {
		t.Fatalf("expected system prompt content, got %q", msgs[0].Content)
	}
	if msgs[1].Role != einoschema.User {
		t.Fatalf("expected second message role=user, got %q", msgs[1].Role)
	}
	if msgs[1].Content != "Hello" {
		t.Fatalf("expected 'Hello', got %q", msgs[1].Content)
	}
	if msgs[2].Role != einoschema.Assistant {
		t.Fatalf("expected third message role=assistant, got %q", msgs[2].Role)
	}
	if msgs[3].Role != einoschema.User {
		t.Fatalf("expected fourth message role=user, got %q", msgs[3].Role)
	}
}

func TestBuildEinoMessagesWithoutSystemPrompt(t *testing.T) {
	req := &CallRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	msgs := buildEinoMessages(req)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != einoschema.User {
		t.Fatalf("expected role=user, got %q", msgs[0].Role)
	}
}

func TestBuildEinoMessagesEmpty(t *testing.T) {
	req := &CallRequest{}
	msgs := buildEinoMessages(req)
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(msgs))
	}
}

func TestMapRole(t *testing.T) {
	tests := []struct {
		input string
		want  einoschema.RoleType
	}{
		{"system", einoschema.System},
		{"SYSTEM", einoschema.System},
		{"  system  ", einoschema.System},
		{"assistant", einoschema.Assistant},
		{"tool", einoschema.Tool},
		{"user", einoschema.User},
		{"unknown", einoschema.User},
		{"", einoschema.User},
	}

	for _, tt := range tests {
		if got := mapRole(tt.input); got != tt.want {
			t.Fatalf("mapRole(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractUsageWithResponseMeta(t *testing.T) {
	prompt := 100
	completion := 50
	total := 150
	resp := &einoschema.Message{
		ResponseMeta: &einoschema.ResponseMeta{
			Usage: &einoschema.TokenUsage{
				PromptTokens:     prompt,
				CompletionTokens: completion,
				TotalTokens:      total,
			},
		},
	}

	usage := extractUsage(resp)
	if usage == nil {
		t.Fatal("expected non-nil usage")
	}
	if usage.InputTokens != 100 {
		t.Fatalf("expected InputTokens=100, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 50 {
		t.Fatalf("expected OutputTokens=50, got %d", usage.OutputTokens)
	}
	if usage.TotalTokens != 150 {
		t.Fatalf("expected TotalTokens=150, got %d", usage.TotalTokens)
	}
}

func TestExtractUsageWithoutResponseMeta(t *testing.T) {
	resp := &einoschema.Message{}
	usage := extractUsage(resp)
	if usage != nil {
		t.Fatalf("expected nil usage for nil ResponseMeta, got %+v", usage)
	}
}

func TestExtractUsageNilInput(t *testing.T) {
	usage := extractUsage(nil)
	if usage != nil {
		t.Fatalf("expected nil usage for nil message, got %+v", usage)
	}
}

func TestExtractUsageZeroTotalComputesSum(t *testing.T) {
	resp := &einoschema.Message{
		ResponseMeta: &einoschema.ResponseMeta{
			Usage: &einoschema.TokenUsage{
				PromptTokens:     30,
				CompletionTokens: 20,
				TotalTokens:      0,
			},
		},
	}
	usage := extractUsage(resp)
	if usage == nil {
		t.Fatal("expected non-nil usage")
	}
	if usage.TotalTokens != 50 {
		t.Fatalf("expected TotalTokens=50 (30+20), got %d", usage.TotalTokens)
	}
}

func TestStreamSinkAdapter(t *testing.T) {
	ctx := context.Background()
	sink := &mockStreamSink{}
	adapter := &streamSinkAdapter{sink: sink}

	if err := adapter.EmitMessageDelta(ctx, "msg-1", "hello"); err != nil {
		t.Fatalf("EmitMessageDelta failed: %v", err)
	}
	if sink.messageDeltas != "hello" {
		t.Fatalf("expected messageDeltas='hello', got %q", sink.messageDeltas)
	}

	if err := adapter.EmitReasoningDelta(ctx, "msg-1", "thinking"); err != nil {
		t.Fatalf("EmitReasoningDelta failed: %v", err)
	}
	if sink.reasoningDeltas != "thinking" {
		t.Fatalf("expected reasoningDeltas='thinking', got %q", sink.reasoningDeltas)
	}
}

func TestStreamSinkAdapterNilSink(t *testing.T) {
	ctx := context.Background()
	adapter := &streamSinkAdapter{sink: nil}

	if err := adapter.EmitMessageDelta(ctx, "msg-1", "hello"); err != nil {
		t.Fatalf("EmitMessageDelta with nil sink should not error, got: %v", err)
	}
	if err := adapter.EmitReasoningDelta(ctx, "msg-1", "thinking"); err != nil {
		t.Fatalf("EmitReasoningDelta with nil sink should not error, got: %v", err)
	}
}

func TestFlowUsageToDomainNil(t *testing.T) {
	if u := flowUsageToDomain(nil); u != nil {
		t.Fatal("expected nil for nil input")
	}
}

type mockStreamSink struct {
	messageDeltas   string
	reasoningDeltas string
}

func (m *mockStreamSink) EmitMessageDelta(_ context.Context, content string) error {
	m.messageDeltas += content
	return nil
}

func (m *mockStreamSink) EmitReasoningDelta(_ context.Context, content string) error {
	m.reasoningDeltas += content
	return nil
}
