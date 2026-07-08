package domain

import (
	"strings"
	"testing"
)

func TestBuildUserInputPrefersSenderName(t *testing.T) {
	req := &RunRequest{Input: InputContext{Type: InputTypeMessage, Messages: []InputMessage{
		{Role: "user", Content: "帮我写一个 HTTP server", SenderName: "A"},
		{Role: "assistant", Content: "好的，以下是代码...", SenderName: "AI队友Alpha"},
		{Role: "user", Content: "加上 /health 端点", SenderName: "B"},
	}}}
	got := BuildUserInput(req)
	want := "【用户问题】\n[1] 用户 「A」发送：「帮我写一个 HTTP server」\n【AI 队友回复】\n[2] AI 队友 「AI队友Alpha」发送：「好的，以下是代码...」\n【用户问题】\n[3] 用户 「B」发送：「加上 /health 端点」"
	if got != want {
		t.Fatalf("BuildUserInput = %q, want %q", got, want)
	}
}

func TestBuildUserInputFallsBackToRole(t *testing.T) {
	req := &RunRequest{Input: InputContext{Type: InputTypeMessage, Messages: []InputMessage{
		{Role: "user", Content: "hello"},
		{Content: "no role"},
	}}}
	got := BuildUserInput(req)
	if !strings.Contains(got, "用户 「user」发送：「hello」") || !strings.Contains(got, "用户 「user」发送：「no role」") {
		t.Fatalf("expected role fallback in %q", got)
	}
}
