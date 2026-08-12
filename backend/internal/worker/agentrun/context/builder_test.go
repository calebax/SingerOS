package agentruncontext

import (
	"context"
	"strings"
	"testing"

	agentrundomain "github.com/insmtx/Leros/backend/internal/worker/agentrun/domain"
)

func TestContextBuilderBuildSystemPromptLayers(t *testing.T) {
	builder := NewContextBuilder(ContextBuilder{})
	prompt, err := builder.BuildSystemPrompt(context.Background(), &agentrundomain.RunRequest{
		Assistant: agentrundomain.AssistantContext{
			Name:         "合同审查专家",
			SystemPrompt: "Assistant-specific prompt.",
		},
		Conversation: agentrundomain.ConversationContext{
			ID: "conv-123",
			Messages: []agentrundomain.InputMessage{
				{Role: "user", Content: "hello"},
			},
		},
		Model: agentrundomain.ModelOptions{
			Provider: "openai",
			Model:    "gpt-4",
		},
		Actor: agentrundomain.ActorContext{
			Channel: "wechat",
		},
	})
	if err != nil {
		t.Fatalf("build system prompt: %v", err)
	}

	for _, expected := range []string{
		"当前对用户展示和执行任务的第一身份是被召唤的 AI 队友",
		"队友名称：合同审查专家",
		"Assistant-specific prompt.",
		"<identity_constraints>",
		"原样引用“队友名称”作为你的名称",
		"禁止改写、音译、拼写变形或自创名号",
		"不要自行编造平台名、公司名、版本或与系统无关的身份信息",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected prompt to contain %q", expected)
		}
	}
	if strings.Contains(prompt, "我是 lework，你工作和生活中的 AI 队友") {
		t.Fatal("expected teammate prompt not to contain default lework self-introduction")
	}

	if !strings.Contains(prompt, "Memory 工具使用指导") {
		t.Fatal("expected prompt to contain Layer 5 memory guidance")
	}

	if strings.Contains(prompt, "Skill 工具使用指导") {
		t.Fatal("expected prompt NOT to contain standalone 'Skill 工具使用指导' section (merged into skill loading)")
	}

	for _, expected := range []string{
		"没有维护的 skill 会变成负担",
		"不要等用户要求",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected merged skill guidance to contain %q", expected)
		}
	}

	if !strings.Contains(prompt, "运行信息") {
		t.Fatal("expected prompt to contain Layer 9 run meta")
	}
	if !strings.Contains(prompt, "conv-123") {
		t.Fatal("expected prompt to contain session ID")
	}
	if !strings.Contains(prompt, "gpt-4") {
		t.Fatal("expected prompt to contain model name")
	}

	if !strings.Contains(prompt, "微信") {
		t.Fatal("expected prompt to contain Layer 10 platform guidance for wechat")
	}

	for _, unexpected := range []string{
		"<session-summary>",
		"Self-learning rules",
		"Available skills:",
	} {
		if strings.Contains(prompt, unexpected) {
			t.Fatalf("expected prompt NOT to contain %q", unexpected)
		}
	}
}

func TestBuildProjectContextSection(t *testing.T) {
	builder := NewContextBuilder(ContextBuilder{})
	prompt, err := builder.BuildSystemPrompt(context.Background(), &agentrundomain.RunRequest{
		Assistant: agentrundomain.AssistantContext{
			Name: "投标策略师",
		},
		Project: agentrundomain.ProjectContext{
			Name:        "投标协作项目",
			Description: "自动化投标文件生成与审查",
			Objective:   "提升投标效率",
			Members: []agentrundomain.MemberBrief{
				{MemberID: 1, MemberType: "user", MemberRole: "owner", Name: "张三", IsCurrentUser: true},
				{MemberID: 10, MemberType: "assistant", MemberRole: "member", Name: "投标策略师", IsCurrentExec: true},
				{MemberID: 11, MemberType: "assistant", MemberRole: "member", Name: "合同审查专家", IsDefault: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("build system prompt: %v", err)
	}

	for _, expected := range []string{
		"## 协作成员",
		"用户：张三（owner）",
		"AI 队友：投标策略师（member）",
		"AI 队友：合同审查专家（member）",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected prompt to contain %q", expected)
		}
	}

	projectIdx := strings.Index(prompt, "## 协作成员")
	workspaceIdx := strings.Index(prompt, "## 工作区信息")
	if projectIdx >= 0 && workspaceIdx >= 0 && projectIdx > workspaceIdx {
		t.Fatal("expected project section to appear before workspace section")
	}
}
