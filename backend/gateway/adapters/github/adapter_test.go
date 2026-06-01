package github

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

func TestVerifySignature_Valid(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"action":"opened"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	a := &Adapter{cfg: types.ChannelConfig{
		Extra: map[string]any{"webhook_secret": secret},
	}}

	if err := a.verifySignature(body, sig); err != nil {
		t.Errorf("expected valid signature, got error: %v", err)
	}
}

func TestVerifySignature_Invalid(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"action":"opened"}`)

	mac := hmac.New(sha256.New, []byte("wrong-secret"))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	a := &Adapter{cfg: types.ChannelConfig{
		Extra: map[string]any{"webhook_secret": secret},
	}}

	if err := a.verifySignature(body, sig); err == nil {
		t.Error("expected invalid signature error")
	}
}

func TestVerifySignature_MissingHeader(t *testing.T) {
	a := &Adapter{cfg: types.ChannelConfig{
		Extra: map[string]any{"webhook_secret": "secret"},
	}}

	if err := a.verifySignature([]byte("body"), ""); err == nil {
		t.Error("expected error for missing signature header")
	}
}

func TestVerifySignature_NoSecret(t *testing.T) {
	// Without a configured secret, verification should pass
	a := &Adapter{cfg: types.ChannelConfig{
		Extra: map[string]any{},
	}}

	if err := a.verifySignature([]byte("body"), ""); err != nil {
		t.Errorf("expected no error when secret is empty: %v", err)
	}
}

func TestVerifySignature_BadFormat(t *testing.T) {
	a := &Adapter{cfg: types.ChannelConfig{
		Extra: map[string]any{"webhook_secret": "secret"},
	}}

	if err := a.verifySignature([]byte("body"), "not-hex"); err == nil {
		t.Error("expected error for non-hex signature")
	}
}

func TestBuildEnvelope_Basic(t *testing.T) {
	a := NewAdapter(types.ChannelConfig{Code: "github"})

	payload := json.RawMessage(`{"sender":{"login":"test-user"},"repository":{"full_name":"org/repo"}}`)
	env := a.buildEnvelope("pull_request", payload, "delivery-123")

	if env.Channel != "github" {
		t.Errorf("expected channel github, got %s", env.Channel)
	}
	if env.MessageID != "delivery-123" {
		t.Errorf("expected message_id delivery-123, got %s", env.MessageID)
	}
	if env.Sender.UserID != "test-user" {
		t.Errorf("expected sender test-user, got %s", env.Sender.UserID)
	}
	if env.SessionKey.ChatID != "org/repo" {
		t.Errorf("expected chat_id org/repo, got %s", env.SessionKey.ChatID)
	}
	if env.MessageType != types.MessageTypeEvent {
		t.Errorf("expected event type, got %s", env.MessageType)
	}
}

func TestBuildEnvelope_MissingDeliveryID(t *testing.T) {
	a := NewAdapter(types.ChannelConfig{Code: "github"})

	payload := json.RawMessage(`{"sender":{"login":"user"},"repository":{"full_name":"org/repo"}}`)
	env := a.buildEnvelope("push", payload, "")

	if env.MessageID == "" {
		t.Error("expected generated message_id when delivery ID is missing")
	}
	if env.TraceID == "" {
		t.Error("expected trace_id to be populated")
	}
}

func TestBuildEnvelope_MissingActor(t *testing.T) {
	a := NewAdapter(types.ChannelConfig{Code: "github"})

	payload := json.RawMessage(`{}`)
	env := a.buildEnvelope("issues", payload, "id-1")

	if env.Sender.UserID != "unknown" {
		t.Errorf("expected unknown actor, got %s", env.Sender.UserID)
	}
	if env.SessionKey.ChatID != "unknown" {
		t.Errorf("expected unknown repo, got %s", env.SessionKey.ChatID)
	}
}

func TestToMessageType(t *testing.T) {
	tests := []struct {
		githubEvent string
		want        types.MessageType
	}{
		{"issues", types.MessageTypeEvent},
		{"issue_comment", types.MessageTypeEvent},
		{"pull_request", types.MessageTypeEvent},
		{"pull_request_review", types.MessageTypeEvent},
		{"pull_request_review_comment", types.MessageTypeEvent},
		{"push", types.MessageTypeEvent},
		{"ping", types.MessageTypeEvent},
		{"unknown_type", types.MessageTypeEvent},
	}

	for _, tt := range tests {
		t.Run(tt.githubEvent, func(t *testing.T) {
			got := toMessageType(tt.githubEvent)
			if got != tt.want {
				t.Errorf("toMessageType(%s) = %s, want %s", tt.githubEvent, got, tt.want)
			}
		})
	}
}

func TestNewAdapter(t *testing.T) {
	cfg := types.ChannelConfig{
		Code: "github",
		Extra: map[string]any{
			"webhook_secret": "secret",
			"bind_port":      "9090",
		},
	}
	a := NewAdapter(cfg)

	if a.cfg.Code != "github" {
		t.Errorf("expected code github, got %s", a.cfg.Code)
	}
	if a.callback != nil {
		t.Error("callback should be nil after construction")
	}
	if a.server != nil {
		t.Error("server should be nil before Connect")
	}
}

func TestOnMessage_DoubleRegistration(t *testing.T) {
	a := NewAdapter(types.ChannelConfig{Code: "github"})

	err := a.OnMessage(func(ctx context.Context, env *types.MessageEnvelope) error { return nil })
	if err != nil {
		t.Fatalf("first OnMessage should succeed: %v", err)
	}

	// Second call should fail
	err = a.OnMessage(func(ctx context.Context, env *types.MessageEnvelope) error { return nil })
	if err == nil {
		t.Error("second OnMessage should fail")
	}
}

func TestAdapterInfo(t *testing.T) {
	a := NewAdapter(types.ChannelConfig{Code: "github"})
	info := a.Info()

	if info.Code != "github" {
		t.Errorf("expected code github, got %s", info.Code)
	}
	if info.Label != "GitHub" {
		t.Errorf("expected label GitHub, got %s", info.Label)
	}
	if info.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", info.Version)
	}
	if !info.Capabilities.SupportsWebhook {
		t.Error("GitHub adapter should support webhooks")
	}
	if info.Capabilities.SupportsIM {
		t.Error("GitHub adapter should NOT support IM")
	}
}

func TestBuildEnvelope_ContentExtra(t *testing.T) {
	a := NewAdapter(types.ChannelConfig{Code: "github"})

	payload := json.RawMessage(`{"sender":{"login":"u"},"repository":{"full_name":"o/r"}}`)
	env := a.buildEnvelope("pull_request", payload, "id-1")

	if env.Content.Extra == nil {
		t.Fatal("expected Extra in content")
	}
	evt, ok := env.Content.Extra["github_event"].(string)
	if !ok || evt != "pull_request" {
		t.Errorf("expected github_event=pull_request in Extra, got %v", env.Content.Extra["github_event"])
	}

	// Text should contain the event type
	if !strings.Contains(env.Content.Text, "pull_request") {
		t.Errorf("expected Text to contain pull_request, got %s", env.Content.Text)
	}
}
