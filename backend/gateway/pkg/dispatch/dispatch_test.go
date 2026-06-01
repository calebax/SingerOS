package dispatch

import (
	"context"
	"testing"

	"github.com/insmtx/Leros/backend/gateway/pkg/types"
)

func TestAuthGate_Empty(t *testing.T) {
	gate := NewAuthGate()
	env := &types.MessageEnvelope{}
	// Empty gate allows everything
	if err := gate.Authorize(env); err != nil {
		t.Errorf("empty gate should allow: %v", err)
	}
}

func TestAuthGate_Allowlist(t *testing.T) {
	gate := NewAuthGate()
	gate.AddRule(AllowlistUsers("alice", "bob"))

	tests := []struct {
		name    string
		userID  string
		wantErr bool
	}{
		{"allowed user", "alice", false},
		{"another allowed", "bob", false},
		{"denied user", "charlie", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := &types.MessageEnvelope{
				Sender: types.SenderInfo{UserID: tt.userID},
			}
			err := gate.Authorize(env)
			if (err != nil) != tt.wantErr {
				t.Errorf("Authorize() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuthGate_Denylist(t *testing.T) {
	gate := NewAuthGate()
	gate.AddRule(DenylistUsers("spammer"))

	env := &types.MessageEnvelope{
		Sender: types.SenderInfo{UserID: "spammer"},
	}
	if err := gate.Authorize(env); err == nil {
		t.Error("denylist should deny spammer")
	}

	env.Sender.UserID = "good_user"
	if err := gate.Authorize(env); err != nil {
		t.Errorf("denylist should allow non-spammer: %v", err)
	}
}

func TestAuthGate_DenyFirst(t *testing.T) {
	// If a deny rule fires first, allowlist should never be reached
	gate := NewAuthGate()
	gate.AddRule(DenylistUsers("alice")) // deny first
	gate.AddRule(AllowlistUsers("alice")) // allow later — should not matter

	env := &types.MessageEnvelope{
		Sender: types.SenderInfo{UserID: "alice"},
	}
	if err := gate.Authorize(env); err == nil {
		t.Error("deny rule should take precedence over later allow rule")
	}
}

func TestAuthGate_RequireMention(t *testing.T) {
	gate := NewAuthGate()
	gate.AddRule(RequireMention("@bot"))

	tests := []struct {
		name     string
		chatType types.ChatType
		text     string
		wantErr  bool
	}{
		{"dm always passes", types.ChatTypeDM, "", false},
		{"group with mention", types.ChatTypeGroup, "hey @bot help", false},
		{"group without mention", types.ChatTypeGroup, "hello world", true},
		{"group empty text", types.ChatTypeGroup, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := &types.MessageEnvelope{
				SessionKey: types.SessionKey{ChatType: tt.chatType},
				Content:    types.MessageContent{Text: tt.text},
			}
			err := gate.Authorize(env)
			if (err != nil) != tt.wantErr {
				t.Errorf("Authorize() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuthGate_AllowBots(t *testing.T) {
	gate := NewAuthGate()
	gate.AddRule(AllowBots(false)) // deny bots

	botEnv := &types.MessageEnvelope{Sender: types.SenderInfo{IsBot: true}}
	if err := gate.Authorize(botEnv); err == nil {
		t.Error("should deny bot when AllowBots(false)")
	}

	humanEnv := &types.MessageEnvelope{Sender: types.SenderInfo{IsBot: false}}
	if err := gate.Authorize(humanEnv); err != nil {
		t.Errorf("should allow human: %v", err)
	}
}

func TestAuthGate_PrependRule(t *testing.T) {
	gate := NewAuthGate()
	gate.AddRule(AllowlistUsers("alice"))      // alice allowed
	gate.PrependRule(DenylistUsers("alice")) // prepend deny

	env := &types.MessageEnvelope{Sender: types.SenderInfo{UserID: "alice"}}
	if err := gate.Authorize(env); err == nil {
		t.Error("prepended deny rule should fire first and deny alice")
	}
}

func TestAuthGate_AllowlistChats(t *testing.T) {
	gate := NewAuthGate()
	gate.AddRule(AllowlistChats(
		types.SessionKey{Channel: "feishu", ChatType: types.ChatTypeDM, ChatID: "dm_123", UserID: ""},
	))

	allowed := &types.MessageEnvelope{
		SessionKey: types.SessionKey{Channel: "feishu", ChatType: types.ChatTypeDM, ChatID: "dm_123"},
	}
	if err := gate.Authorize(allowed); err != nil {
		t.Errorf("should allow matched chat: %v", err)
	}

	denied := &types.MessageEnvelope{
		SessionKey: types.SessionKey{Channel: "feishu", ChatType: types.ChatTypeDM, ChatID: "dm_456"},
	}
	if err := gate.Authorize(denied); err == nil {
		t.Error("should deny unmatched chat")
	}
}

func TestAuthGate_DenylistChats(t *testing.T) {
	gate := NewAuthGate()
	gate.AddRule(DenylistChats(
		types.SessionKey{Channel: "feishu", ChatType: types.ChatTypeGroup, ChatID: "grp_evil", UserID: ""},
	))

	denied := &types.MessageEnvelope{
		SessionKey: types.SessionKey{Channel: "feishu", ChatType: types.ChatTypeGroup, ChatID: "grp_evil"},
	}
	if err := gate.Authorize(denied); err == nil {
		t.Error("should deny listed chat")
	}

	allowed := &types.MessageEnvelope{
		SessionKey: types.SessionKey{Channel: "feishu", ChatType: types.ChatTypeGroup, ChatID: "grp_good"},
	}
	if err := gate.Authorize(allowed); err != nil {
		t.Errorf("should allow non-listed chat: %v", err)
	}
}

// --- DeliveryRouter Tests ---

type mockSender struct {
	sent       []string
	sendErr    error
	typingSent []string
	typingErr  error
}

func (m *mockSender) Send(ctx context.Context, target string, msg types.OutboundMessage) error {
	m.sent = append(m.sent, target)
	return m.sendErr
}

func (m *mockSender) SendTyping(ctx context.Context, target string) error {
	m.typingSent = append(m.typingSent, target)
	return m.typingErr
}

func TestDeliveryRouter_Send(t *testing.T) {
	router := NewDeliveryRouter()
	mock := &mockSender{}
	router.Register("feishu", mock)

	err := router.Send(context.Background(), []types.DeliveryTarget{
		{Channel: "feishu", ChatID: "chat_1"},
		{Channel: "feishu", ChatID: "chat_2"},
	}, types.OutboundMessage{Text: "hello"})
	if err != nil {
		t.Fatalf("Send() failed: %v", err)
	}

	if len(mock.sent) != 2 {
		t.Errorf("expected 2 sends, got %d", len(mock.sent))
	}
	if mock.sent[0] != "chat_1" || mock.sent[1] != "chat_2" {
		t.Errorf("unexpected targets: %v", mock.sent)
	}
}

func TestDeliveryRouter_SendUnregistered(t *testing.T) {
	router := NewDeliveryRouter()
	err := router.Send(context.Background(), []types.DeliveryTarget{
		{Channel: "unknown", ChatID: "x"},
	}, types.OutboundMessage{Text: "hi"})
	if err == nil {
		t.Error("expected error for unregistered channel")
	}
}

func TestDeliveryRouter_SendTyping(t *testing.T) {
	router := NewDeliveryRouter()
	mock := &mockSender{}
	router.Register("feishu", mock)

	err := router.SendTyping(context.Background(), "feishu", "chat_1")
	if err != nil {
		t.Fatalf("SendTyping() failed: %v", err)
	}
	if len(mock.typingSent) != 1 || mock.typingSent[0] != "chat_1" {
		t.Errorf("unexpected typing target: %v", mock.typingSent)
	}
}

func TestDeliveryRouter_HasChannel(t *testing.T) {
	router := NewDeliveryRouter()
	if router.HasChannel("feishu") {
		t.Error("should not have unregistered channel")
	}
	mock := &mockSender{}
	router.Register("feishu", mock)
	if !router.HasChannel("feishu") {
		t.Error("should have registered channel")
	}
}

func TestDeliveryRouter_Unregister(t *testing.T) {
	router := NewDeliveryRouter()
	mock := &mockSender{}
	router.Register("feishu", mock)
	router.Unregister("feishu")
	if router.HasChannel("feishu") {
		t.Error("should not have unregistered channel")
	}
}

// --- MessageGateway Tests ---

type mockPublisher struct {
	published []any
}

func (m *mockPublisher) Publish(ctx context.Context, topic string, event any) error {
	m.published = append(m.published, event)
	return nil
}

func TestMessageGateway_HandleDedup(t *testing.T) {
	router := NewDeliveryRouter()
	pub := &mockPublisher{}
	gw := NewMessageGateway(router, pub, WithDedupTTL(DefaultDedupTTL))

	env := &types.MessageEnvelope{MessageID: "msg-1", Channel: "feishu", MessageType: types.MessageTypeText}

	result, err := gw.Handle(context.Background(), env)
	if err != nil {
		t.Fatalf("first Handle() failed: %v", err)
	}
	if !result.Published {
		t.Error("expected first message to be published")
	}

	// Duplicate should be silently skipped
	result, err = gw.Handle(context.Background(), env)
	if err != nil {
		t.Fatalf("duplicate Handle() failed: %v", err)
	}
	if result.Published {
		t.Error("duplicate should not be published")
	}
	if !result.Dropped {
		t.Error("duplicate should be marked Dropped")
	}
}

func TestMessageGateway_HandleEmptyMessageID(t *testing.T) {
	router := NewDeliveryRouter()
	pub := &mockPublisher{}
	gw := NewMessageGateway(router, pub)

	env := &types.MessageEnvelope{MessageID: "", Channel: "feishu", MessageType: types.MessageTypeText}

	result, err := gw.Handle(context.Background(), env)
	if err != nil {
		t.Fatalf("Handle() with empty ID failed: %v", err)
	}
	if !result.Published {
		t.Error("expected publish for empty-ID message")
	}
}

func TestMessageGateway_HandleAuthDeny(t *testing.T) {
	router := NewDeliveryRouter()
	pub := &mockPublisher{}
	authGate := NewAuthGate()
	authGate.AddRule(DenylistUsers("evil"))
	gw := NewMessageGateway(router, pub, WithAuthGate(authGate))

	env := &types.MessageEnvelope{MessageID: "msg-1", Sender: types.SenderInfo{UserID: "evil"}}

	result, err := gw.Handle(context.Background(), env)
	if err != nil {
		t.Fatalf("Handle() unexpected error: %v", err)
	}
	if result.Published {
		t.Error("denied message should not be published")
	}
	if !result.Dropped {
		t.Error("denied message should be marked Dropped")
	}
}

func TestMessageGateway_SendWithTargets(t *testing.T) {
	router := NewDeliveryRouter()
	mock := &mockSender{}
	router.Register("feishu", mock)

	gw := NewMessageGateway(router, nil)

	source := types.SessionKey{Channel: "feishu", ChatType: types.ChatTypeDM, ChatID: "dm_123"}
	msg := types.OutboundMessage{
		Text: "hello",
		DeliveryTargets: []types.DeliveryTarget{
			{Channel: "feishu", ChatID: "dm_456"},
		},
	}

	if err := gw.Send(context.Background(), source, msg); err != nil {
		t.Fatalf("Send() failed: %v", err)
	}
	if len(mock.sent) != 1 || mock.sent[0] != "dm_456" {
		t.Errorf("expected send to dm_456, got %v", mock.sent)
	}
}

func TestMessageGateway_SendSourceFallback(t *testing.T) {
	router := NewDeliveryRouter()
	mock := &mockSender{}
	router.Register("feishu", mock)

	gw := NewMessageGateway(router, nil)

	source := types.SessionKey{Channel: "feishu", ChatType: types.ChatTypeDM, ChatID: "dm_123"}
	msg := types.OutboundMessage{Text: "hello"} // no DeliveryTargets set

	if err := gw.Send(context.Background(), source, msg); err != nil {
		t.Fatalf("Send() failed: %v", err)
	}
	if len(mock.sent) != 1 || mock.sent[0] != "dm_123" {
		t.Errorf("expected send to source chat dm_123, got %v", mock.sent)
	}
}

func TestMessageGateway_SendTyping(t *testing.T) {
	router := NewDeliveryRouter()
	mock := &mockSender{}
	router.Register("feishu", mock)

	gw := NewMessageGateway(router, nil)
	source := types.SessionKey{Channel: "feishu", ChatType: types.ChatTypeDM, ChatID: "dm_123"}

	if err := gw.SendTyping(context.Background(), source); err != nil {
		t.Fatalf("SendTyping() failed: %v", err)
	}
	if len(mock.typingSent) != 1 || mock.typingSent[0] != "dm_123" {
		t.Errorf("expected typing to dm_123, got %v", mock.typingSent)
	}
}

func TestMessageGateway_TopicFormat(t *testing.T) {
	router := NewDeliveryRouter()
	pub := &mockPublisher{}
	gw := NewMessageGateway(router, pub)

	env := &types.MessageEnvelope{
		MessageID:   "msg-1",
		Channel:     "github",
		MessageType: types.MessageTypeEvent,
	}
	result, err := gw.Handle(context.Background(), env)
	if err != nil {
		t.Fatalf("Handle() failed: %v", err)
	}
	if !result.Published {
		t.Error("expected publish for topic formation test")
	}
	if result.Topic != "interaction.github.event" {
		t.Errorf("expected topic interaction.github.event, got %s", result.Topic)
	}
}
