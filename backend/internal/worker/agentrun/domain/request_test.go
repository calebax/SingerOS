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

func TestBuildAttachmentText_SingleAttachment(t *testing.T) {
	attachments := []Attachment{
		{Name: "foo.txt", URL: "http://example.com/foo.txt", MimeType: "text/plain"},
	}
	got := BuildAttachmentText(attachments)

	if !strings.Contains(got, "attached by the user in this message") {
		t.Fatalf("expected 'attached by the user in this message' in %q", got)
	}
	if !strings.Contains(got, "- foo.txt") {
		t.Fatalf("expected '- foo.txt' in %q", got)
	}
	if !strings.Contains(got, "URL: http://example.com/foo.txt") {
		t.Fatalf("expected 'URL: http://example.com/foo.txt' in %q", got)
	}
	if !strings.Contains(got, "Type: text/plain") {
		t.Fatalf("expected 'Type: text/plain' in %q", got)
	}
}

func TestBuildAttachmentText_MultipleAttachments(t *testing.T) {
	attachments := []Attachment{
		{Name: "a.txt", URL: "http://a", MimeType: "text/plain"},
		{Name: "b.png", URL: "http://b", MimeType: "image/png"},
	}
	got := BuildAttachmentText(attachments)

	if !strings.Contains(got, "- a.txt") {
		t.Fatalf("expected '- a.txt' in %q", got)
	}
	if !strings.Contains(got, "- b.png") {
		t.Fatalf("expected '- b.png' in %q", got)
	}
	lineCount := strings.Count(got, "\n")
	if lineCount < 6 {
		t.Fatalf("expected at least 6 newlines for 2 attachments, got %d in %q", lineCount, got)
	}
}

func TestBuildAttachmentText_EmptyAttachment(t *testing.T) {
	got := BuildAttachmentText(nil)
	if got != "" {
		t.Fatalf("expected empty string for nil, got %q", got)
	}

	got = BuildAttachmentText([]Attachment{})
	if got != "" {
		t.Fatalf("expected empty string for empty slice, got %q", got)
	}
}

func TestBuildAttachmentText_NoURL(t *testing.T) {
	attachments := []Attachment{
		{Name: "foo.txt", MimeType: "text/plain"},
	}
	got := BuildAttachmentText(attachments)

	if !strings.Contains(got, "- foo.txt") {
		t.Fatalf("expected '- foo.txt' in %q", got)
	}
	if !strings.Contains(got, "Type: text/plain") {
		t.Fatalf("expected 'Type: text/plain' in %q", got)
	}
	if strings.Contains(got, "URL:") {
		t.Fatalf("expected no URL line when URL is empty, got %q", got)
	}
}

func TestBuildAttachmentText_NoMimeType(t *testing.T) {
	attachments := []Attachment{
		{Name: "foo.txt", URL: "http://example.com"},
	}
	got := BuildAttachmentText(attachments)

	if !strings.Contains(got, "URL: http://example.com") {
		t.Fatalf("expected 'URL: http://example.com' in %q", got)
	}
	if strings.Contains(got, "Type:") {
		t.Fatalf("expected no Type line when MimeType is empty, got %q", got)
	}
}
